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
	// branches memeriksa keberadaan kode cabang saat staf ditetapkan. Boleh nil
	// (uji unit tanpa database): saat nil, validasi dilewati agar perilaku lama
	// tetap berjalan.
	branches domain.BranchExistenceChecker
}

// errStaffRoleNotManageable menolak perubahan atas akun yang perannya setingkat atau
// lebih tinggi dari pelaku.
var errStaffRoleNotManageable = domain.ErrStaffRoleNotManageable

func NewStaffService(staffRepo domain.StaffRepository, branches domain.BranchExistenceChecker, auditSinks ...domain.AuditRepository) domain.StaffService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &staffService{staffRepo: staffRepo, branches: branches, auditRepo: auditRepo}
}

// validateStaffBranch memastikan kode cabang staf terdaftar sebagai unit organisasi.
// Peran bercabang biasa tanpa cabang sah akan mendapat cakupan kosong (tidak melihat
// data apa pun) secara senyap; lebih baik ditolak saat penetapan agar operator
// memperbaiki kodenya. Peran lintas cabang dikecualikan karena kode 'HO' milik kantor
// pusat bukan baris branches.
func (s *staffService) validateStaffBranch(ctx context.Context, role domain.StaffRole, branchCode string) error {
	if !(domain.Actor{Role: role}).RequiresRegisteredBranch() {
		return nil
	}
	code := strings.ToUpper(strings.TrimSpace(branchCode))
	if code == "" {
		return domain.ErrStaffBranchRequired
	}
	if s.branches == nil {
		return nil
	}
	if _, err := s.branches.GetByCode(ctx, code); err != nil {
		if errors.Is(err, domain.ErrBranchNotFound) {
			return fmt.Errorf("%w: %s", domain.ErrStaffBranchUnknown, code)
		}
		return err
	}
	return nil
}

// BranchCoverageWarnings mengembalikan peringatan untuk staf aktif bercabang biasa
// yang branch_code-nya tidak cocok dengan unit organisasi mana pun: cakupannya kosong
// sehingga ia tidak melihat data apa pun. Peringatan dicatat saat start agar operator
// memperbaiki data lama, tanpa memperluas cakupan secara diam-diam (yang akan membuka
// kebocoran). Kegagalan membaca data menjadi peringatan tersendiri, bukan diam.
func BranchCoverageWarnings(ctx context.Context, reader domain.BranchScopeMismatchReader) []string {
	if reader == nil {
		return nil
	}
	mismatches, err := reader.ListBranchScopeMismatches(ctx)
	if err != nil {
		return []string{fmt.Sprintf("gagal memeriksa kecocokan cabang staf: %v", err)}
	}
	var warnings []string
	for _, m := range mismatches {
		if !(domain.Actor{Role: m.Role}).RequiresRegisteredBranch() {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"staf %q bercabang %q yang tidak terdaftar; cakupannya kosong sehingga tidak melihat data apa pun — perbaiki branch_code-nya",
			m.Username, m.BranchCode))
	}
	return warnings
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return domain.ErrStaffPasswordTooShort
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
		return domain.ErrStaffPasswordWeak
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

// resolveStaffBook menentukan buku akun staf dari permintaan yang eksplisit. requested
// nil berarti memakai pengambilan bawaan (buku pembuat, buku tunggal instalasi, atau
// konvensional). Permintaan eksplisit wajib: (a) nilainya dikenal, (b) sesuai cakupan
// buku instalasi, dan (c) pada instalasi DUAL, tidak keluar dari buku pengelola kecuali
// pengelola lintas buku. Ini menutup celah penugasan buku lintas lini usaha oleh
// pengelola yang sendiri terikat satu buku.
func (s *staffService) resolveStaffBook(actor domain.Actor, requested *domain.COABook) (domain.COABook, error) {
	if requested == nil {
		book := actor.Book
		if single := actor.BookScope.SingleBook(); single != "" {
			book = single
		} else if book == "" || actor.IsCrossBook() {
			book = domain.BookConventional
		}
		return book, nil
	}
	if !domain.ValidCOABook(*requested) {
		return "", domain.ErrStaffBookInvalid
	}
	if single := actor.BookScope.SingleBook(); single != "" {
		if *requested != single {
			return "", domain.ErrStaffBookInvalid
		}
		return *requested, nil
	}
	if !actor.IsCrossBook() && actor.Book != "" && *requested != actor.Book {
		return "", domain.ErrStaffBookInvalid
	}
	return *requested, nil
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
		return nil, domain.ErrStaffPrivilegedCreate
	}

	// Peran lintas cabang tidak dibuat lewat endpoint ini, tetapi bila kelak
	// diizinkan ia memakai 'HO' bawaan kantor pusat. Peran operasional wajib memakai
	// unit organisasi yang benar-benar terdaftar.
	branch := strings.ToUpper(strings.TrimSpace(input.BranchCode))
	if branch == "" && (domain.Actor{Role: input.Role}).IsCrossBranch() {
		branch = "HO"
	}
	if err := s.validateStaffBranch(ctx, input.Role, branch); err != nil {
		return nil, err
	}

	hash, err := hashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// EmployeeID sengaja dibiarkan kosong: repository mengisinya dari sequence
	// database (bukan potongan waktu) di dalam Create, sehingga nomor pegawai dijamin
	// unik. Bila bentrok tetap terjadi, galat unik dipetakan ke ErrStaffAlreadyExists.
	//
	// Akun baru mengikuti buku pembuatnya bila pembuat terikat satu buku; bila pembuat
	// lintas buku/belum ditentukan, dipakai konvensional sebagai aman bawaan (bank
	// mayoritas konvensional). Instalasi satu buku memaksa buku aktif instalasi agar
	// staf baru tidak lahir di lini usaha yang tidak dilayani. Tanpa ini kolom book
	// akan NULL dan akun baru justru melihat kedua buku — kebocoran yang justru ingin
	// ditutup.
	book, err := s.resolveStaffBook(actor, input.Book)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	user := &domain.StaffUser{
		ID:                uuid.New(),
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
		// Bentrok kolom unik (username/email/nomor pegawai) adalah aturan bisnis,
		// bukan kegagalan server: kembalikan sentinel yang jelas (handler -> 409),
		// bukan galat database yang bocor sebagai 500.
		if isUniqueViolation(err) {
			return nil, domain.ErrStaffAlreadyExists
		}
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
		return nil, domain.ErrStaffSelfDeactivate
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
			return nil, domain.ErrStaffPrivilegedRole
		}
		user.Role = *input.Role
	}
	if input.BranchCode != nil {
		nextBranch := strings.ToUpper(strings.TrimSpace(*input.BranchCode))
		// Divalidasi SEBELUM diubah agar penolakan tidak meninggalkan perubahan
		// setengah jalan pada objek yang dibaca.
		if err := s.validateStaffBranch(ctx, user.Role, nextBranch); err != nil {
			return nil, err
		}
		user.BranchCode = nextBranch
	}
	if input.IsActive != nil {
		user.IsActive = *input.IsActive
	}
	bookBefore := user.Book
	if input.Book != nil {
		next, err := s.resolveStaffBook(actor, input.Book)
		if err != nil {
			return nil, err
		}
		user.Book = next
	}

	if err := s.staffRepo.Update(ctx, user); err != nil {
		return nil, err
	}
	auditChanges := map[string]any{
		"username":    user.Username,
		"role":        string(user.Role),
		"is_active":   user.IsActive,
		"branch_code": user.BranchCode,
		"book":        string(user.Book),
	}
	if user.Book != bookBefore {
		auditChanges["book_before"] = string(bookBefore)
	}
	if err := writeAudit(ctx, s.auditRepo, nil, actor, "UPDATE_STAFF", "staff", user.ID.String(), auditChanges); err != nil {
		return nil, fmt.Errorf("audit perubahan staf: %w", err)
	}
	return user, nil
}

func (s *staffService) ChangePassword(ctx context.Context, id uuid.UUID, input domain.ChangePasswordInput, actor domain.Actor) error {
	// Jalur ini adalah ubah kata sandi milik sendiri: penggantian kata sandi orang lain
	// harus lewat reset yang diaudit dan diperiksa hierarkinya.
	if id != actor.UserID {
		return domain.ErrStaffPasswordOnlySelf
	}

	user, err := s.staffRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.CurrentPassword)); err != nil {
		return domain.ErrStaffCurrentPassword
	}

	if err := validatePassword(input.NewPassword); err != nil {
		return err
	}

	// Prevent reusing the same password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.NewPassword)); err == nil {
		return domain.ErrStaffPasswordUnchanged
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
		return domain.ErrStaffUseOwnPasswordFlow
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
