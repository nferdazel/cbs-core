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
	staffRepo   domain.StaffRepository
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
		staffRepo:   staffRepo,
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

// isPasswordExpired menghitung umur kata sandi terhadap kebijakan
// auth.password_expiry_days. Nilai 0 atau negatif berarti penegakan nonaktif
// (bawaan produksi; bank mengisi N hari untuk mengaktifkannya). Tanggal ubah yang
// kosong diperlakukan belum kedaluwarsa supaya akun lama yang datanya belum
// lengkap tidak langsung terkunci.
func (s *authService) isPasswordExpired(ctx context.Context, user *domain.StaffUser) bool {
	days := s.getConfigInt(ctx, "auth.password_expiry_days", 0)
	if days <= 0 || user.PasswordChangedAt.IsZero() {
		return false
	}
	return time.Since(user.PasswordChangedAt) > time.Duration(days)*24*time.Hour
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

	// Kata sandi yang kedaluwarsa TIDAK menolak login: pengguna tetap menerima
	// token, tetapi tokennya ditandai dan middleware membatasinya hanya untuk
	// mengganti kata sandi. Menolak login akan mengunci akun tanpa jalur pulih.
	passwordExpired := s.isPasswordExpired(ctx, user)

	sessionID := uuid.New()
	accessToken, err := s.generateAccessToken(user, sessionID, atTTL, passwordExpired)
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
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		ExpiresIn:        atTTL * 60,
		RefreshExpiresIn: rtTTL * 3600,
		PasswordExpired:  passwordExpired,
		User:             user,
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
			// Refresh token sudah pernah dirotasi tetapi dipakai lagi: indikasi
			// pencurian token. Cabut seluruh sesi user agar token curian mati.
			_ = s.sessionRepo.RevokeAllForUser(ctx, session.UserID)
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

	// Penanda kedaluwarsa dihitung ulang saat refresh: bila kata sandi sudah
	// kedaluwarsa, token baru tetap terbatas dan tidak menjadi jalan pintas
	// menembus pembatasan.
	passwordExpired := s.isPasswordExpired(ctx, user)

	newSessionID := uuid.New()
	accessToken, err := s.generateAccessToken(user, newSessionID, atTTL, passwordExpired)
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
		AccessToken:      accessToken,
		RefreshToken:     newRefreshToken,
		ExpiresIn:        atTTL * 60,
		RefreshExpiresIn: rtTTL * 3600,
		PasswordExpired:  passwordExpired,
		User:             user,
	}, nil
}

func (s *authService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.RevokeByID(ctx, sessionID)
}

// ValidateAccessToken memverifikasi tanda tangan dan masa berlaku token, lalu
// mencocokkannya dengan keadaan sesi dan akun di basis data.
//
// Tanda tangan saja tidak cukup. Token dibuat sekali dan tetap sah sampai kedaluwarsa,
// sedangkan sesinya dapat dicabut kapan saja: keluar, penguncian akun karena percobaan
// masuk gagal, penonaktifan pengguna, atau pencabutan seluruh sesi karena token refresh
// dipakai ulang. Tanpa pencocokan ini, token lama tetap dapat dipakai sampai masa
// berlakunya habis. Peran dan cabang juga dibaca dari basis data, sehingga perubahan
// wewenang berlaku pada permintaan berikutnya, bukan setelah token ditukar.
func (s *authService) ValidateAccessToken(ctx context.Context, tokenString string) (*domain.JWTClaims, error) {
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
	if userID == uuid.Nil || sessionID == uuid.Nil {
		return nil, domain.ErrInvalidToken
	}

	identity, err := s.sessionRepo.GetIdentity(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	// Token yang sesinya sudah dicabut atau kedaluwarsa tidak boleh menjadi jalan masuk
	// ke akun yang dinonaktifkan atau dikunci.
	if identity.UserID != userID {
		return nil, domain.ErrInvalidToken
	}
	if !identity.IsActive {
		return nil, domain.ErrAccountInactiveUser
	}
	if identity.IsLocked() {
		return nil, domain.ErrAccountLocked
	}

	// mc adalah jwt.MapClaims; nilai bool diambil dengan aman (klaim lama mungkin
	// belum memuat penanda ini).
	pwdExpired, _ := mc["pwd_expired"].(bool)

	return &domain.JWTClaims{
		UserID:     identity.UserID,
		Username:   identity.Username,
		Role:       identity.Role,
		BranchCode: identity.BranchCode,
		Book:       identity.Book,
		SessionID:  identity.SessionID,
		// Penanda ini dibaca dari klaim, bukan dari basis data: status kedaluwarsa
		// dibekukan saat token diterbitkan agar konsisten dengan keputusan login.
		PasswordExpired: pwdExpired,
		// Izin & menu efektif dibaca dari database (lihat SessionIdentity) dan
		// diisi ulang setiap permintaan, sehingga tidak dari token dan tidak dari
		// pemetaan peran di kode.
		Permissions:       identity.Permissions,
		PermissionsLoaded: true,
		Menus:             identity.Menus,
	}, nil
}

func (s *authService) generateAccessToken(user *domain.StaffUser, sessionID uuid.UUID, ttlMinutes int, passwordExpired bool) (string, error) {
	claims := jwt.MapClaims{
		"uid":         user.ID.String(),
		"username":    user.Username,
		"role":        string(user.Role),
		"branch":      user.BranchCode,
		"book":        string(user.Book),
		"sid":         sessionID.String(),
		"pwd_expired": passwordExpired,
		"exp":         time.Now().Add(time.Duration(ttlMinutes) * time.Minute).Unix(),
		"iat":         time.Now().Unix(),
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
