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

// Uji penegakan kebijakan kedaluwarsa kata sandi di jalur login.
//
// Stub di bawah sengaja mandiri (bukan memakai stub di auth_service_test.go) agar
// berkas uji ini tidak ikut rusak bila berkas uji lain diubah pekerjaan lain.

const pwdTestJWTSecret = "pwd-expiry-test-secret-at-least-32-bytes!!"

type pwdStaffRepo struct{ user *domain.StaffUser }

func (r *pwdStaffRepo) Create(context.Context, *domain.StaffUser) error { return nil }
func (r *pwdStaffRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.StaffUser, error) {
	if r.user != nil && r.user.ID == id {
		return r.user, nil
	}
	return nil, errors.New("not found")
}
func (r *pwdStaffRepo) GetByUsername(_ context.Context, username string) (*domain.StaffUser, error) {
	if r.user != nil && r.user.Username == username {
		return r.user, nil
	}
	return nil, errors.New("not found")
}
func (r *pwdStaffRepo) List(context.Context, int, int) ([]domain.StaffUser, int, error) {
	return nil, 0, nil
}
func (r *pwdStaffRepo) Update(context.Context, *domain.StaffUser) error         { return nil }
func (r *pwdStaffRepo) IncrementFailedLogin(context.Context, uuid.UUID) error   { return nil }
func (r *pwdStaffRepo) LockAccount(context.Context, uuid.UUID, time.Time) error { return nil }
func (r *pwdStaffRepo) ResetFailedLogin(context.Context, uuid.UUID) error       { return nil }
func (r *pwdStaffRepo) UpdateLastLogin(context.Context, uuid.UUID) error        { return nil }
func (r *pwdStaffRepo) UpdatePassword(context.Context, uuid.UUID, string) error { return nil }

type pwdSessionRepo struct{ session *domain.StaffSession }

func (r *pwdSessionRepo) Create(_ context.Context, s *domain.StaffSession) error {
	r.session = s
	return nil
}
func (r *pwdSessionRepo) GetByTokenHash(context.Context, string) (*domain.StaffSession, error) {
	return nil, errors.New("not found")
}
func (r *pwdSessionRepo) GetIdentity(_ context.Context, sessionID uuid.UUID) (*domain.SessionIdentity, error) {
	if r.session == nil || r.session.ID != sessionID {
		return nil, domain.ErrSessionExpired
	}
	return &domain.SessionIdentity{SessionID: r.session.ID, UserID: r.session.UserID, IsActive: true}, nil
}
func (r *pwdSessionRepo) RevokeByID(context.Context, uuid.UUID) error       { return nil }
func (r *pwdSessionRepo) RevokeAllForUser(context.Context, uuid.UUID) error { return nil }
func (r *pwdSessionRepo) DeleteExpired(context.Context) error               { return nil }

type pwdConfigRepo struct{ values map[string]string }

func (r *pwdConfigRepo) Get(_ context.Context, key string) (string, error) {
	if v, ok := r.values[key]; ok {
		return v, nil
	}
	return "", errors.New("not found")
}
func (r *pwdConfigRepo) Set(context.Context, string, string, uuid.UUID) error { return nil }
func (r *pwdConfigRepo) GetAll(context.Context) (map[string]string, error)    { return nil, nil }

// pwdConfig membuat konfigurasi dengan kebijakan kedaluwarsa `days`.
func pwdConfig(days string) *pwdConfigRepo {
	return &pwdConfigRepo{values: map[string]string{
		"auth.access_token_ttl_minutes": "15",
		"auth.refresh_token_ttl_hours":  "8",
		"auth.password_expiry_days":     days,
	}}
}

func pwdTestUser(password string, changedAt time.Time) *domain.StaffUser {
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), 12)
	return &domain.StaffUser{
		ID:                uuid.New(),
		Username:          "pwduser",
		PasswordHash:      string(hash),
		Role:              domain.RoleTeller,
		BranchCode:        "HO",
		IsActive:          true,
		PasswordChangedAt: changedAt,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
}

func pwdLogin(t *testing.T, svc domain.AuthService, user *domain.StaffUser, password string) *domain.LoginResponse {
	t.Helper()
	resp, err := svc.Login(context.Background(), domain.LoginInput{Username: user.Username, Password: password})
	if err != nil {
		t.Fatalf("login gagal: %v", err)
	}
	return resp
}

// Kebijakan 0 berarti nonaktif: umur kata sandi berapa pun tetap menerbitkan token
// normal. Ini adalah keadaan bawaan produksi setelah migrasi 000053.
func TestLoginKebijakanNonaktifMenerbitkanTokenNormal(t *testing.T) {
	password := "Teller@Pass1!"
	user := pwdTestUser(password, time.Now().Add(-400*24*time.Hour))
	svc := service.NewAuthService(&pwdStaffRepo{user: user}, &pwdSessionRepo{}, pwdConfig("0"), pwdTestJWTSecret)

	resp := pwdLogin(t, svc, user, password)
	if resp.PasswordExpired {
		t.Fatal("kebijakan nonaktif seharusnya tidak menandai password kedaluwarsa")
	}

	claims, err := svc.ValidateAccessToken(context.Background(), resp.AccessToken)
	if err != nil {
		t.Fatalf("token seharusnya sah: %v", err)
	}
	if claims.PasswordExpired {
		t.Fatal("token normal seharusnya tidak membawa penanda kedaluwarsa")
	}
}

// Kata sandi yang lebih tua dari kebijakan tetap boleh login, tetapi tokennya
// ditandai terbatas supaya pengguna punya jalur ganti kata sandi.
func TestLoginPasswordKedaluwarsaMenerbitkanTokenTerbatas(t *testing.T) {
	password := "Teller@Pass1!"
	user := pwdTestUser(password, time.Now().Add(-100*24*time.Hour))
	svc := service.NewAuthService(&pwdStaffRepo{user: user}, &pwdSessionRepo{}, pwdConfig("90"), pwdTestJWTSecret)

	resp := pwdLogin(t, svc, user, password)
	if !resp.PasswordExpired {
		t.Fatal("umur 100 hari dengan kebijakan 90 hari seharusnya ditandai kedaluwarsa")
	}

	claims, err := svc.ValidateAccessToken(context.Background(), resp.AccessToken)
	if err != nil {
		t.Fatalf("token seharusnya sah: %v", err)
	}
	if !claims.PasswordExpired {
		t.Fatal("token seharusnya membawa penanda kedaluwarsa agar middleware membatasinya")
	}
}

func TestLoginPasswordBelumKedaluwarsaMenerbitkanTokenNormal(t *testing.T) {
	password := "Teller@Pass1!"
	user := pwdTestUser(password, time.Now().Add(-10*24*time.Hour))
	svc := service.NewAuthService(&pwdStaffRepo{user: user}, &pwdSessionRepo{}, pwdConfig("90"), pwdTestJWTSecret)

	resp := pwdLogin(t, svc, user, password)
	if resp.PasswordExpired {
		t.Fatal("umur 10 hari dengan kebijakan 90 hari seharusnya belum kedaluwarsa")
	}
}

// Setelah kata sandi diganti (password_changed_at diperbarui), login berikutnya
// harus normal. Langkah ganti ditiru dengan mengubah stempel waktu, sama seperti
// staff_repo.UpdatePassword yang mengisi password_changed_at=NOW().
func TestLoginSetelahGantiPasswordMenerbitkanTokenNormal(t *testing.T) {
	password := "Teller@Pass1!"
	user := pwdTestUser(password, time.Now().Add(-100*24*time.Hour))
	svc := service.NewAuthService(&pwdStaffRepo{user: user}, &pwdSessionRepo{}, pwdConfig("90"), pwdTestJWTSecret)

	if resp := pwdLogin(t, svc, user, password); !resp.PasswordExpired {
		t.Fatal("prasyarat: login sebelum ganti seharusnya terbatas")
	}

	user.PasswordChangedAt = time.Now()

	resp := pwdLogin(t, svc, user, password)
	if resp.PasswordExpired {
		t.Fatal("setelah ganti kata sandi dan login ulang, token harus normal")
	}
	claims, err := svc.ValidateAccessToken(context.Background(), resp.AccessToken)
	if err != nil {
		t.Fatalf("token seharusnya sah: %v", err)
	}
	if claims.PasswordExpired {
		t.Fatal("token setelah ganti kata sandi seharusnya tidak lagi membawa penanda")
	}
}
