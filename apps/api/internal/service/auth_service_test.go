package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// ── In-memory stubs ──

type stubStaffRepo struct {
	user *domain.StaffUser
}

func (s *stubStaffRepo) Create(ctx context.Context, u *domain.StaffUser) error { return nil }
func (s *stubStaffRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.StaffUser, error) {
	if s.user != nil && s.user.ID == id {
		return s.user, nil
	}
	return nil, errors.New("not found")
}
func (s *stubStaffRepo) GetByUsername(ctx context.Context, username string) (*domain.StaffUser, error) {
	if s.user != nil && s.user.Username == username {
		return s.user, nil
	}
	return nil, errors.New("not found")
}
func (s *stubStaffRepo) List(ctx context.Context, limit, offset int) ([]domain.StaffUser, int, error) {
	return nil, 0, nil
}
func (s *stubStaffRepo) Update(ctx context.Context, u *domain.StaffUser) error           { return nil }
func (s *stubStaffRepo) IncrementFailedLogin(ctx context.Context, id uuid.UUID) error    { return nil }
func (s *stubStaffRepo) LockAccount(ctx context.Context, id uuid.UUID, until time.Time) error {
	return nil
}
func (s *stubStaffRepo) ResetFailedLogin(ctx context.Context, id uuid.UUID) error       { return nil }
func (s *stubStaffRepo) UpdateLastLogin(ctx context.Context, id uuid.UUID) error        { return nil }
func (s *stubStaffRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	return nil
}

type stubSessionRepo struct {
	session *domain.StaffSession
	// user adalah pemilik sesi. Identitas sesi disusun darinya, meniru join ke
	// staff_users pada implementasi sebenarnya.
	user     *domain.StaffUser
	identity *domain.SessionIdentity
	err      error
}

func (s *stubSessionRepo) Create(ctx context.Context, sess *domain.StaffSession) error {
	s.session = sess
	return nil
}

func (s *stubSessionRepo) GetIdentity(ctx context.Context, sessionID uuid.UUID) (*domain.SessionIdentity, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.identity != nil {
		return s.identity, nil
	}
	if s.session == nil || s.session.ID != sessionID {
		return nil, domain.ErrSessionExpired
	}
	id := &domain.SessionIdentity{SessionID: s.session.ID, UserID: s.session.UserID, IsActive: true}
	if s.user != nil {
		id.Username, id.Role, id.BranchCode = s.user.Username, s.user.Role, s.user.BranchCode
	}
	return id, nil
}
func (s *stubSessionRepo) GetByTokenHash(ctx context.Context, hash string) (*domain.StaffSession, error) {
	if s.session != nil && s.session.RefreshTokenHash == hash {
		return s.session, nil
	}
	return nil, errors.New("not found")
}
func (s *stubSessionRepo) RevokeByID(ctx context.Context, id uuid.UUID) error         { return nil }
func (s *stubSessionRepo) RevokeAllForUser(ctx context.Context, id uuid.UUID) error   { return nil }
func (s *stubSessionRepo) DeleteExpired(ctx context.Context) error                     { return nil }

type stubConfigRepo struct{}

func (s *stubConfigRepo) Get(ctx context.Context, key string) (string, error) {
	defaults := map[string]string{
		"auth.access_token_ttl_minutes": "15",
		"auth.refresh_token_ttl_hours":  "8",
		"auth.max_failed_logins":        "5",
		"auth.lockout_minutes":          "15",
	}
	if v, ok := defaults[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}
func (s *stubConfigRepo) Set(ctx context.Context, key, value string, updatedBy uuid.UUID) error {
	return nil
}
func (s *stubConfigRepo) GetAll(ctx context.Context) (map[string]string, error) { return nil, nil }

// ── Helpers ──

func makeTestUser(password string) *domain.StaffUser {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
	return &domain.StaffUser{
		ID:                uuid.New(),
		EmployeeID:        "EMP-2026-001",
		Username:          "teller01",
		FullName:          "Budi Teller",
		Email:             "teller@cbs.local",
		PasswordHash:      string(hash),
		Role:              domain.RoleTeller,
		BranchCode:        "HO",
		IsActive:          true,
		FailedLoginCount:  0,
		PasswordChangedAt: time.Now(),
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

const testJWTSecret = "test-jwt-secret-at-least-32-bytes-long!!"

// ── Tests ──

func TestAuthService_Login_Success(t *testing.T) {
	password := "Teller@Pass1!"
	user := makeTestUser(password)
	staffRepo := &stubStaffRepo{user: user}
	sessionRepo := &stubSessionRepo{}
	configRepo := &stubConfigRepo{}

	svc := service.NewAuthService(staffRepo, sessionRepo, configRepo, testJWTSecret)

	resp, err := svc.Login(context.Background(), domain.LoginInput{
		Username:  user.Username,
		Password:  password,
		IPAddress: "127.0.0.1",
	})

	if err != nil {
		t.Fatalf("expected successful login, got error: %v", err)
	}
	if resp.AccessToken == "" {
		t.Fatal("expected non-empty access token")
	}
	if resp.RefreshToken == "" {
		t.Fatal("expected non-empty refresh token")
	}
	if resp.User.Username != user.Username {
		t.Fatalf("expected username %q, got %q", user.Username, resp.User.Username)
	}
}

func TestAuthService_Login_WrongPassword(t *testing.T) {
	user := makeTestUser("Correct@Pass1!")
	staffRepo := &stubStaffRepo{user: user}
	svc := service.NewAuthService(staffRepo, &stubSessionRepo{}, &stubConfigRepo{}, testJWTSecret)

	_, err := svc.Login(context.Background(), domain.LoginInput{
		Username: user.Username,
		Password: "WrongPassword!",
	})

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthService_Login_InactiveUser(t *testing.T) {
	password := "Active@Pass1!"
	user := makeTestUser(password)
	user.IsActive = false

	svc := service.NewAuthService(&stubStaffRepo{user: user}, &stubSessionRepo{}, &stubConfigRepo{}, testJWTSecret)

	_, err := svc.Login(context.Background(), domain.LoginInput{
		Username: user.Username,
		Password: password,
	})

	if !errors.Is(err, domain.ErrAccountInactiveUser) {
		t.Fatalf("expected ErrAccountInactiveUser, got: %v", err)
	}
}

func TestAuthService_Login_LockedAccount(t *testing.T) {
	password := "Locked@Pass1!"
	user := makeTestUser(password)
	lockUntil := time.Now().Add(10 * time.Minute)
	user.LockedUntil = &lockUntil

	svc := service.NewAuthService(&stubStaffRepo{user: user}, &stubSessionRepo{}, &stubConfigRepo{}, testJWTSecret)

	_, err := svc.Login(context.Background(), domain.LoginInput{
		Username: user.Username,
		Password: password,
	})

	if !errors.Is(err, domain.ErrAccountLocked) {
		t.Fatalf("expected ErrAccountLocked, got: %v", err)
	}
}

func TestAuthService_ValidateAccessToken(t *testing.T) {
	password := "Valid@Token1!"
	user := makeTestUser(password)
	sessions := &stubSessionRepo{user: user}
	svc := service.NewAuthService(&stubStaffRepo{user: user}, sessions, &stubConfigRepo{}, testJWTSecret)

	resp, err := svc.Login(context.Background(), domain.LoginInput{
		Username: user.Username,
		Password: password,
	})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	claims, err := svc.ValidateAccessToken(context.Background(), resp.AccessToken)
	if err != nil {
		t.Fatalf("expected valid token, got: %v", err)
	}
	if claims.Username != user.Username {
		t.Fatalf("expected username %q in claims, got %q", user.Username, claims.Username)
	}
	if claims.Role != domain.RoleTeller {
		t.Fatalf("expected role Teller, got: %v", claims.Role)
	}
}

func TestStaffRole_HasPermission(t *testing.T) {
	cases := []struct {
		role    domain.StaffRole
		perm    domain.Permission
		allowed bool
	}{
		{domain.RoleTeller, domain.PermTransactionsDeposit, true},
		{domain.RoleTeller, domain.PermTransactionsReverse, false},
		{domain.RoleTeller, domain.PermUsersCreate, false},
		{domain.RoleSupervisor, domain.PermMakerCheckerApprove, true},
		{domain.RoleSupervisor, domain.PermTransactionsDeposit, false},
		{domain.RoleAO, domain.PermLoansApply, true},
		{domain.RoleAO, domain.PermCollectionsInput, true},
		{domain.RoleAO, domain.PermTransactionsDeposit, true},
		{domain.RoleAO, domain.PermTransactionsWithdraw, true},
		{domain.RoleAO, domain.PermTransactionsTransfer, true},
		{domain.RoleAO, domain.PermLedgerRead, true},
		{domain.RoleAO, domain.PermMakerCheckerApprove, false},
		{domain.RoleAO, domain.PermSystemConfig, false},
		{domain.RoleAO, domain.PermSystemConfigRead, false},
		{domain.RoleAuditor, domain.PermLedgerRead, true},
		{domain.RoleAuditor, domain.PermTransactionsDeposit, false},
		{domain.RoleSuperAdmin, domain.PermSystemConfig, true},
		{domain.RoleCS, domain.PermCustomersCreate, true},
		{domain.RoleCS, domain.PermTransactionsDeposit, false},
	}

	for _, tc := range cases {
		got := tc.role.HasPermission(tc.perm)
		if got != tc.allowed {
			t.Errorf("role %s, perm %s: expected allowed=%v, got %v", tc.role, tc.perm, tc.allowed, got)
		}
	}
}

// Token yang tanda tangannya sah tetap harus ditolak bila sesinya sudah tidak berlaku,
// akunnya tidak aktif lagi, atau akunnya sedang dikunci. Tanpa ini, keluar dari aplikasi
// hanya menghapus token di sisi klien: token lama masih dapat dipakai sampai kedaluwarsa.
func TestAuthService_ValidateAccessToken_MenolakSesiYangTidakBerlaku(t *testing.T) {
	password := "Valid@Token1!"
	user := makeTestUser(password)
	sessions := &stubSessionRepo{user: user}
	svc := service.NewAuthService(&stubStaffRepo{user: user}, sessions, &stubConfigRepo{}, testJWTSecret)

	resp, err := svc.Login(context.Background(), domain.LoginInput{Username: user.Username, Password: password})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}
	// Disimpan lebih dulu: subtest berikutnya mengubah keadaan stub tanpa memulihkannya.
	sessionID := sessions.session.ID

	cases := []struct {
		name string
		// atur mengubah keadaan sesi/pengguna di basis data setelah token diterbitkan.
		atur func()
		want error
	}{
		{
			name: "sesi dicabut (keluar aplikasi)",
			atur: func() { sessions.session = nil },
			want: domain.ErrSessionExpired,
		},
		{
			name: "penyimpanan sesi melaporkan kedaluwarsa",
			atur: func() { sessions.session = nil; sessions.err = domain.ErrSessionExpired },
			want: domain.ErrSessionExpired,
		},
		{
			name: "akun dinonaktifkan",
			atur: func() {
				sessions.err = nil
				sessions.identity = &domain.SessionIdentity{
					SessionID: sessionID, UserID: user.ID,
					Username: user.Username, Role: user.Role, BranchCode: user.BranchCode,
				}
			},
			want: domain.ErrAccountInactiveUser,
		},
		{
			name: "akun dikunci karena percobaan masuk gagal",
			atur: func() {
				terkunci := time.Now().Add(time.Hour)
				sessions.identity = &domain.SessionIdentity{
					SessionID: sessionID, UserID: user.ID,
					Username: user.Username, Role: user.Role, BranchCode: user.BranchCode,
					IsActive: true, LockedUntil: &terkunci,
				}
			},
			want: domain.ErrAccountLocked,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.atur()
			_, err := svc.ValidateAccessToken(context.Background(), resp.AccessToken)
			if !errors.Is(err, tc.want) {
				t.Fatalf("kesalahan %v, ingin %v", err, tc.want)
			}
		})
	}
}

// Wewenang dibaca dari basis data, bukan dari klaim token: sesi yang masih berlaku
// dengan peran yang sudah diubah tidak boleh terus memakai wewenang lama.
func TestAuthService_ValidateAccessToken_MemakaiPeranTerbaru(t *testing.T) {
	password := "Valid@Token1!"
	user := makeTestUser(password)
	sessions := &stubSessionRepo{user: user}
	svc := service.NewAuthService(&stubStaffRepo{user: user}, sessions, &stubConfigRepo{}, testJWTSecret)

	resp, err := svc.Login(context.Background(), domain.LoginInput{Username: user.Username, Password: password})
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	sessions.identity = &domain.SessionIdentity{
		SessionID: sessions.session.ID, UserID: user.ID,
		Username: user.Username, Role: domain.RoleSupervisor, BranchCode: "KC002", IsActive: true,
	}

	claims, err := svc.ValidateAccessToken(context.Background(), resp.AccessToken)
	if err != nil {
		t.Fatalf("expected valid token, got: %v", err)
	}
	if claims.Role != domain.RoleSupervisor {
		t.Fatalf("peran %v, ingin peran terbaru Supervisor", claims.Role)
	}
	if claims.BranchCode != "KC002" {
		t.Fatalf("cabang %q, ingin cabang terbaru KC002", claims.BranchCode)
	}
}
