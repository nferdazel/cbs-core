package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type staffService struct {
	staffRepo domain.StaffRepository
	auditRepo domain.AuditRepository
}

// errStaffRoleNotManageable menolak perubahan atas akun yang perannya setingkat atau
// lebih tinggi dari pelaku.
var errStaffRoleNotManageable = domain.ErrStaffRoleNotManageable

func NewStaffService(staffRepo domain.StaffRepository, auditSinks ...domain.AuditRepository) domain.StaffService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &staffService{staffRepo: staffRepo, auditRepo: auditRepo}
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, c := range password {
		switch {
		case unicode.IsUpper(c):
			hasUpper = true
		case unicode.IsLower(c):
			hasLower = true
		case unicode.IsDigit(c):
			hasDigit = true
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			hasSpecial = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit || !hasSpecial {
		return errors.New("password must contain uppercase, lowercase, number, and special character")
	}
	return nil
}

func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func generateEmployeeID() string {
	// EMP-YYYY-XXXXX (collision handled by DB unique constraint)
	return fmt.Sprintf("EMP-%s-%05d", time.Now().Format("2006"), time.Now().Nanosecond()%100000)
}

func (s *staffService) CreateStaff(ctx context.Context, input domain.CreateStaffInput, actor domain.Actor) (*domain.StaffUser, error) {
	if !actor.CanManageStaff(input.Role) {
		return nil, errStaffRoleNotManageable
	}
	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	// Prevent creating another SUPERADMIN via this path
	if input.Role == domain.RoleSuperAdmin || input.Role == domain.RoleSystem {
		return nil, errors.New("cannot create SUPERADMIN or SYSTEM through this endpoint")
	}

	hash, err := hashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	branch := strings.ToUpper(input.BranchCode)
	if branch == "" {
		branch = "HO"
	}

	// Akun baru mengikuti buku pembuatnya bila pembuat terikat satu buku; bila
	// pembuat lintas buku/belum ditentukan, dipakai konvensional sebagai aman
	// bawaan (bank mayoritas konvensional). Tanpa ini kolom book akan NULL dan
	// akun baru justru melihat kedua buku — kebocoran yang justru ingin ditutup.
	book := actor.Book
	if book == "" || actor.IsCrossBook() {
		book = domain.BookConventional
	}

	now := time.Now().UTC()
	user := &domain.StaffUser{
		ID:                uuid.New(),
		EmployeeID:        generateEmployeeID(),
		Username:          strings.ToLower(input.Username),
		FullName:          input.FullName,
		Email:             strings.ToLower(input.Email),
		PasswordHash:      hash,
		Role:              input.Role,
		BranchCode:        branch,
		Book:              book,
		IsActive:          true,
		PasswordChangedAt: now,
		CreatedBy:         &actor.UserID,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.staffRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create staff user: %w", err)
	}
	// Kata sandi tidak pernah masuk audit log; yang dicatat adalah identitas akun baru.
	if err := writeAudit(ctx, s.auditRepo, nil, actor, "CREATE_STAFF", "staff", user.ID.String(), map[string]any{
		"username":    user.Username,
		"employee_id": user.EmployeeID,
		"role":        string(user.Role),
		"branch_code": user.BranchCode,
		"book":        string(user.Book),
	}); err != nil {
		return nil, fmt.Errorf("audit pembuatan staf: %w", err)
	}
	return user, nil
}

func (s *staffService) GetStaff(ctx context.Context, id uuid.UUID) (*domain.StaffUser, error) {
	return s.staffRepo.GetByID(ctx, id)
}

func (s *staffService) ListStaff(ctx context.Context, page, pageSize int) ([]domain.StaffUser, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.staffRepo.List(ctx, pageSize, (page-1)*pageSize)
}

func (s *staffService) UpdateStaff(ctx context.Context, id uuid.UUID, input domain.UpdateStaffInput, actor domain.Actor) (*domain.StaffUser, error) {
	user, err := s.staffRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanManageStaff(user.Role) {
		return nil, errStaffRoleNotManageable
	}
	if input.IsActive != nil && !*input.IsActive && id == actor.UserID {
		return nil, errors.New("akun sendiri tidak dapat dinonaktifkan")
	}

	if input.FullName != nil {
		user.FullName = *input.FullName
	}
	if input.Email != nil {
		user.Email = strings.ToLower(*input.Email)
	}
	if input.Role != nil {
		// Peran tujuan juga diperiksa: tanpa ini pengelola tingkat bawah dapat
		// menaikkan rekannya ke peran di atas dirinya.
		if !actor.CanManageStaff(*input.Role) {
			return nil, errStaffRoleNotManageable
		}
		if *input.Role == domain.RoleSuperAdmin || *input.Role == domain.RoleSystem {
			return nil, errors.New("cannot assign SUPERADMIN or SYSTEM role via update")
		}
		user.Role = *input.Role
	}
	if input.BranchCode != nil {
		user.BranchCode = strings.ToUpper(*input.BranchCode)
	}
	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}

	if err := s.staffRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, s.auditRepo, nil, actor, "UPDATE_STAFF", "staff", user.ID.String(), map[string]any{
		"username":    user.Username,
		"role":        string(user.Role),
		"is_active":   user.IsActive,
		"branch_code": user.BranchCode,
	}); err != nil {
		return nil, fmt.Errorf("audit perubahan staf: %w", err)
	}
	return user, nil
}

func (s *staffService) ChangePassword(ctx context.Context, id uuid.UUID, input domain.ChangePasswordInput, actor domain.Actor) error {
	// Jalur ini adalah ubah kata sandi milik sendiri: penggantian kata sandi orang lain
	// harus lewat reset yang diaudit dan diperiksa hierarkinya.
	if id != actor.UserID {
		return errors.New("ubah kata sandi hanya berlaku untuk akun sendiri")
	}

	user, err := s.staffRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		return errors.New("current password is incorrect")
	}

	if err := validatePassword(input.NewPassword); err != nil {
		return err
	}

	// Prevent reusing the same password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.NewPassword)); err == nil {
		return errors.New("new password must be different from current password")
	}

	hash, err := hashPassword(input.NewPassword)
	if err != nil {
		return err
	}
	if err := s.staffRepo.UpdatePassword(ctx, id, hash); err != nil {
		return err
	}
	return writeAudit(ctx, s.auditRepo, nil, actor, "CHANGE_PASSWORD", "staff", id.String(), map[string]any{
		"username": user.Username,
	})
}

func (s *staffService) ResetPassword(ctx context.Context, id uuid.UUID, newPassword string, actor domain.Actor) error {
	user, err := s.staffRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if id == actor.UserID {
		return errors.New("gunakan ubah kata sandi untuk akun sendiri")
	}
	// Reset kata sandi adalah jalan masuk ke akun orang lain: hierarki peran wajib
	// diperiksa lebih dulu, dan aksinya meninggalkan jejak audit.
	if !actor.CanManageStaff(user.Role) {
		return errStaffRoleNotManageable
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.staffRepo.UpdatePassword(ctx, id, hash); err != nil {
		return err
	}
	return writeAudit(ctx, s.auditRepo, nil, actor, "RESET_STAFF_PASSWORD", "staff", id.String(), map[string]any{
		"username": user.Username,
		"role":     string(user.Role),
	})
}
