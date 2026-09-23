package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// stubStaffRepo melayani akun staf dari peta id dan merekam kata sandi baru, supaya
// test dapat memastikan penggantian kata sandi benar-benar terjadi atau ditolak.
type stubStaffRepo struct {
	domain.StaffRepository
	users     map[uuid.UUID]*domain.StaffUser
	passwords map[uuid.UUID]string
}

func (s *stubStaffRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.StaffUser, error) {
	user, ok := s.users[id]
	if !ok {
		return nil, errors.New("staff user not found")
	}
	return user, nil
}

func (s *stubStaffRepo) Update(_ context.Context, user *domain.StaffUser) error {
	s.users[user.ID] = user
	return nil
}

func (s *stubStaffRepo) UpdatePassword(_ context.Context, id uuid.UUID, hash string) error {
	s.passwords[id] = hash
	return nil
}

const testStaffPassword = "Rahasia#2026"

// staffFixture menyiapkan dua akun: ADMIN cabang 001 dan SUPERADMIN.
func staffFixture(t *testing.T) (domain.StaffService, *stubStaffRepo, *stubAuditRepo, uuid.UUID, uuid.UUID) {
	t.Helper()

	hash, err := bcrypt.GenerateFromPassword([]byte(testStaffPassword), 4)
	if err != nil {
		t.Fatalf("hash kata sandi: %v", err)
	}

	adminID := uuid.New()
	superID := uuid.New()
	repo := &stubStaffRepo{
		users: map[uuid.UUID]*domain.StaffUser{
			adminID: {
				ID: adminID, Username: "admin.uji", EmployeeID: "EMP-2026-00001",
				PasswordHash: string(hash), Role: domain.RoleAdmin, BranchCode: "001", IsActive: true,
			},
			superID: {
				ID: superID, Username: "super.uji", EmployeeID: "EMP-2026-00002",
				PasswordHash: string(hash), Role: domain.RoleSuperAdmin, BranchCode: "001", IsActive: true,
			},
		},
		passwords: map[uuid.UUID]string{},
	}
	audit := &stubAuditRepo{}
	return NewStaffService(repo, nil, audit), repo, audit, adminID, superID
}

func adminActor(id uuid.UUID) domain.Actor {
	return domain.Actor{UserID: id, Username: "admin.uji", Role: domain.RoleAdmin, BranchCode: "001"}
}

// ADMIN tidak boleh mereset kata sandi SUPERADMIN: tanpa batas ini pengelola akun
// tingkat bawah dapat mengambil alih kendali penuh sistem.
func TestResetPassword_DeniesHigherRole(t *testing.T) {
	svc, repo, audit, adminID, superID := staffFixture(t)

	err := svc.ResetPassword(context.Background(), superID, "Ganti#2026Ku", adminActor(adminID))
	if !errors.Is(err, domain.ErrStaffRoleNotManageable) {
		t.Fatalf("reset kata sandi SUPERADMIN harus ditolak, dapat: %v", err)
	}
	if _, ok := repo.passwords[superID]; ok {
		t.Fatal("kata sandi SUPERADMIN berubah meski ditolak")
	}
	if len(audit.events) != 0 {
		t.Fatalf("penolakan tidak boleh menghasilkan audit sukses: %+v", audit.events)
	}
}

// ADMIN boleh mereset kata sandi peran di bawahnya, dan aksinya meninggalkan jejak.
func TestResetPassword_AllowsLowerRoleAndAudits(t *testing.T) {
	svc, repo, audit, adminID, _ := staffFixture(t)
	tellerID := uuid.New()
	repo.users[tellerID] = &domain.StaffUser{
		ID: tellerID, Username: "teller.uji", Role: domain.RoleTeller, BranchCode: "001", IsActive: true,
	}

	if err := svc.ResetPassword(context.Background(), tellerID, "Ganti#2026Ku", adminActor(adminID)); err != nil {
		t.Fatalf("reset kata sandi teller: %v", err)
	}
	if _, ok := repo.passwords[tellerID]; !ok {
		t.Fatal("kata sandi teller tidak tersimpan")
	}
	if len(audit.events) != 1 {
		t.Fatalf("event audit %d, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "RESET_STAFF_PASSWORD" || event.ResourceType != "staff" || event.ResourceID != tellerID.String() {
		t.Fatalf("event audit tidak sesuai: %+v", event)
	}
	if _, ada := event.Changes["password"]; ada {
		t.Fatal("kata sandi tidak boleh tercatat di audit log")
	}
}

// Menaikkan rekan ke peran setingkat pelaku harus ditolak, begitu pula menurunkan
// akun yang perannya lebih tinggi.
func TestUpdateStaff_DeniesRoleEscalationAndHigherAccount(t *testing.T) {
	svc, repo, _, adminID, superID := staffFixture(t)
	tellerID := uuid.New()
	repo.users[tellerID] = &domain.StaffUser{
		ID: tellerID, Username: "teller.uji", Role: domain.RoleTeller, BranchCode: "001", IsActive: true,
	}

	promote := domain.RoleAdmin
	if _, err := svc.UpdateStaff(context.Background(), tellerID, domain.UpdateStaffInput{Role: &promote}, adminActor(adminID)); !errors.Is(err, domain.ErrStaffRoleNotManageable) {
		t.Fatalf("promosi ke peran setingkat harus ditolak, dapat: %v", err)
	}
	if repo.users[tellerID].Role != domain.RoleTeller {
		t.Fatal("peran teller berubah meski promosi ditolak")
	}

	deactivate := false
	if _, err := svc.UpdateStaff(context.Background(), superID, domain.UpdateStaffInput{IsActive: &deactivate}, adminActor(adminID)); !errors.Is(err, domain.ErrStaffRoleNotManageable) {
		t.Fatalf("mengubah akun SUPERADMIN harus ditolak, dapat: %v", err)
	}
	if !repo.users[superID].IsActive {
		t.Fatal("akun SUPERADMIN ikut nonaktif meski ditolak")
	}
}

// Akun sendiri tidak dapat dinonaktifkan: kesalahan itu mencabut akses satu-satunya
// pengelola sistem yang tersisa.
func TestUpdateStaff_DeniesSelfDeactivate(t *testing.T) {
	svc, repo, _, adminID, _ := staffFixture(t)

	deactivate := false
	if _, err := svc.UpdateStaff(context.Background(), adminID, domain.UpdateStaffInput{IsActive: &deactivate}, adminActor(adminID)); err == nil {
		t.Fatal("menonaktifkan akun sendiri harus ditolak")
	}
	if !repo.users[adminID].IsActive {
		t.Fatal("akun sendiri ikut nonaktif")
	}
}

// Ubah kata sandi hanya berlaku untuk akun pemanggil; penggantian kata sandi orang
// lain harus lewat reset yang diaudit.
func TestChangePassword_DeniesOtherUser(t *testing.T) {
	svc, _, _, adminID, superID := staffFixture(t)

	err := svc.ChangePassword(context.Background(), superID, domain.ChangePasswordInput{
		CurrentPassword: testStaffPassword,
		NewPassword:     "Ganti#2026Ku",
	}, adminActor(adminID))
	if err == nil {
		t.Fatal("mengubah kata sandi akun lain harus ditolak")
	}
}

// Pemilik akun tetap dapat mengubah kata sandinya sendiri dan aksinya tercatat.
func TestChangePassword_AllowsOwnerAndAudits(t *testing.T) {
	svc, _, audit, adminID, _ := staffFixture(t)

	if err := svc.ChangePassword(context.Background(), adminID, domain.ChangePasswordInput{
		CurrentPassword: testStaffPassword,
		NewPassword:     "Ganti#2026Ku",
	}, adminActor(adminID)); err != nil {
		t.Fatalf("ubah kata sandi sendiri: %v", err)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "CHANGE_PASSWORD" {
		t.Fatalf("audit ubah kata sandi tidak tercatat: %+v", audit.events)
	}
}
