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
}

func NewStaffService(staffRepo domain.StaffRepository) domain.StaffService {
	return &staffService{staffRepo: staffRepo}
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

func (s *staffService) CreateStaff(ctx context.Context, input domain.CreateStaffInput, createdBy uuid.UUID) (*domain.StaffUser, error) {
	if err := validatePassword(input.Password); err != nil {
		return nil, err
	}

	// Prevent creating another SUPERADMIN via this path
	if input.Role == domain.RoleSuperAdmin {
		return nil, errors.New("cannot create SUPERADMIN through this endpoint")
	}

	hash, err := hashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	branch := strings.ToUpper(input.BranchCode)
	if branch == "" {
		branch = "HO"
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
		IsActive:          true,
		PasswordChangedAt: now,
		CreatedBy:         &createdBy,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.staffRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create staff user: %w", err)
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

func (s *staffService) UpdateStaff(ctx context.Context, id uuid.UUID, input domain.UpdateStaffInput) (*domain.StaffUser, error) {
	user, err := s.staffRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.FullName != nil {
		user.FullName = *input.FullName
	}
	if input.Email != nil {
		user.Email = strings.ToLower(*input.Email)
	}
	if input.Role != nil {
		if *input.Role == domain.RoleSuperAdmin {
			return nil, errors.New("cannot assign SUPERADMIN role via update")
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
	return user, nil
}

func (s *staffService) ChangePassword(ctx context.Context, id uuid.UUID, input domain.ChangePasswordInput) error {
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
	return s.staffRepo.UpdatePassword(ctx, id, hash)
}

func (s *staffService) ResetPassword(ctx context.Context, id uuid.UUID, newPassword string, _ uuid.UUID) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.staffRepo.UpdatePassword(ctx, id, hash)
}
