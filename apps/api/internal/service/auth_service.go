package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authService struct {
	staffRepo  domain.StaffRepository
	sessionRepo domain.SessionRepository
	configRepo  domain.SystemConfigRepository
	jwtSecret   []byte
}

func NewAuthService(
	staffRepo domain.StaffRepository,
	sessionRepo domain.SessionRepository,
	configRepo domain.SystemConfigRepository,
	jwtSecret string,
) domain.AuthService {
	return &authService{
		staffRepo:  staffRepo,
		sessionRepo: sessionRepo,
		configRepo:  configRepo,
		jwtSecret:   []byte(jwtSecret),
	}
}

func (s *authService) getConfigInt(ctx context.Context, key string, defaultVal int) int {
	v, err := s.configRepo.Get(ctx, key)
	if err != nil {
		return defaultVal
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return defaultVal
	}
	return n
}

func (s *authService) Login(ctx context.Context, input domain.LoginInput) (*domain.LoginResponse, error) {
	user, err := s.staffRepo.GetByUsername(ctx, input.Username)
	if err != nil {
		// Use generic error to prevent username enumeration
		return nil, domain.ErrInvalidCredentials
	}

	if !user.IsActive {
		return nil, domain.ErrAccountInactiveUser
	}

	if user.IsLocked() {
		return nil, domain.ErrAccountLocked
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		maxFails := s.getConfigInt(ctx, "auth.max_failed_logins", 5)
		lockMins := s.getConfigInt(ctx, "auth.lockout_minutes", 15)

		_ = s.staffRepo.IncrementFailedLogin(ctx, user.ID)
		if user.FailedLoginCount+1 >= maxFails {
			lockUntil := time.Now().Add(time.Duration(lockMins) * time.Minute)
			_ = s.staffRepo.LockAccount(ctx, user.ID, lockUntil)
			return nil, domain.ErrAccountLocked
		}
		return nil, domain.ErrInvalidCredentials
	}

	// Successful login — reset failure counter
	_ = s.staffRepo.ResetFailedLogin(ctx, user.ID)
	_ = s.staffRepo.UpdateLastLogin(ctx, user.ID)

	// Generate tokens
	atTTL := s.getConfigInt(ctx, "auth.access_token_ttl_minutes", 15)
	rtTTL := s.getConfigInt(ctx, "auth.refresh_token_ttl_hours", 8)

	sessionID := uuid.New()
	accessToken, err := s.generateAccessToken(user, sessionID, atTTL)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	session := &domain.StaffSession{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: postgres.HashToken(refreshToken),
		IPAddress:        input.IPAddress,
		UserAgent:        input.UserAgent,
		ExpiresAt:        time.Now().Add(time.Duration(rtTTL) * time.Hour),
		CreatedAt:        time.Now().UTC(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    atTTL * 60,
		User:         user,
	}, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*domain.LoginResponse, error) {
	hash := postgres.HashToken(refreshToken)
	session, err := s.sessionRepo.GetByTokenHash(ctx, hash)
	if err != nil {
		return nil, domain.ErrInvalidToken
	}

	if !session.IsValid() {
		if session.RevokedAt != nil {
			return nil, domain.ErrSessionRevoked
		}
		return nil, domain.ErrSessionExpired
	}

	user, err := s.staffRepo.GetByID(ctx, session.UserID)
	if err != nil || !user.IsActive {
		return nil, domain.ErrAccountInactiveUser
	}

	// Revoke old session (rotation)
	_ = s.sessionRepo.RevokeByID(ctx, session.ID)

	atTTL := s.getConfigInt(ctx, "auth.access_token_ttl_minutes", 15)
	rtTTL := s.getConfigInt(ctx, "auth.refresh_token_ttl_hours", 8)

	newSessionID := uuid.New()
	accessToken, err := s.generateAccessToken(user, newSessionID, atTTL)
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := generateOpaqueToken()
	if err != nil {
		return nil, err
	}

	newSession := &domain.StaffSession{
		ID:               newSessionID,
		UserID:           user.ID,
		RefreshTokenHash: postgres.HashToken(newRefreshToken),
		IPAddress:        session.IPAddress,
		UserAgent:        session.UserAgent,
		ExpiresAt:        time.Now().Add(time.Duration(rtTTL) * time.Hour),
		CreatedAt:        time.Now().UTC(),
	}
	if err := s.sessionRepo.Create(ctx, newSession); err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    atTTL * 60,
		User:         user,
	}, nil
}

func (s *authService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.RevokeByID(ctx, sessionID)
}

func (s *authService) ValidateAccessToken(_ context.Context, tokenString string) (*domain.JWTClaims, error) {
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.jwtSecret, nil
	}, jwt.WithExpirationRequired())

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, domain.ErrSessionExpired
		}
		return nil, domain.ErrInvalidToken
	}

	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, domain.ErrInvalidToken
	}

	userID, _ := uuid.Parse(mc["uid"].(string))
	sessionID, _ := uuid.Parse(mc["sid"].(string))

	return &domain.JWTClaims{
		UserID:     userID,
		Username:   mc["username"].(string),
		Role:       domain.StaffRole(mc["role"].(string)),
		BranchCode: mc["branch"].(string),
		SessionID:  sessionID,
	}, nil
}

func (s *authService) generateAccessToken(user *domain.StaffUser, sessionID uuid.UUID, ttlMinutes int) (string, error) {
	claims := jwt.MapClaims{
		"uid":      user.ID.String(),
		"username": user.Username,
		"role":     string(user.Role),
		"branch":   user.BranchCode,
		"sid":      sessionID.String(),
		"exp":      time.Now().Add(time.Duration(ttlMinutes) * time.Minute).Unix(),
		"iat":      time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

// generateOpaqueToken generates a cryptographically-random 32-byte base64 token.
func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
