package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// staffCreateRepo adalah repository staf untuk uji unit CreateStaff: ia dapat
// dipaksa mengembalikan bentrok unique (galat database) agar pemetaan ke sentinel
// bisnis diuji tanpa database.
type staffCreateRepo struct {
	domain.StaffRepository
	createErr error
	created   []*domain.StaffUser
}

func (r *staffCreateRepo) Create(_ context.Context, u *domain.StaffUser) error {
	if r.createErr != nil {
		return r.createErr
	}
	r.created = append(r.created, u)
	return nil
}

// stubBranchChecker mensimulasikan tabel branches untuk uji penetapan cabang staf.
type stubBranchChecker struct {
	codes map[string]bool
}

func (s stubBranchChecker) GetByCode(_ context.Context, code string) (*domain.Branch, error) {
	if s.codes[code] {
		return &domain.Branch{Code: code}, nil
	}
	return nil, domain.ErrBranchNotFound
}

func createStaffInput() domain.CreateStaffInput {
	return domain.CreateStaffInput{
		Username:   "teller.baru",
		FullName:   "Teller Baru",
		Email:      "teller.baru@cbs.local",
		Password:   testStaffPassword,
		Role:       domain.RoleTeller,
		BranchCode: "001",
	}
}

// N1: bentrok unique dipetakan ke ErrStaffAlreadyExists sehingga handler dapat
// membalas 409 dengan pesan jelas.
func TestCreateStaff_UniqueViolationJadiErrStaffAlreadyExists(t *testing.T) {
	repo := &staffCreateRepo{createErr: errors.New(`pq: duplicate key value violates unique constraint "staff_users_employee_id_key"`)}
	svc := NewStaffService(repo, nil)

	actor := domain.Actor{UserID: uuid.New(), Username: "admin.uji", Role: domain.RoleAdmin, BranchCode: "001"}
	_, err := svc.CreateStaff(context.Background(), createStaffInput(), actor)
	if !errors.Is(err, domain.ErrStaffAlreadyExists) {
		t.Fatalf("bentrok unique harus menjadi ErrStaffAlreadyExists, dapat: %v", err)
	}
	if len(repo.created) != 0 {
		t.Fatalf("staf tercatat %d meski bentrok", len(repo.created))
	}
}

// N4: peran bercabang biasa tanpa cabang terdaftar ditolak dengan pesan jelas,
// bukan dibiarkan mendapat cakupan kosong secara senyap.
func TestCreateStaff_MenolakCabangTidakTerdaftar(t *testing.T) {
	svc := NewStaffService(&staffCreateRepo{}, stubBranchChecker{codes: map[string]bool{"001": true}})
	actor := domain.Actor{UserID: uuid.New(), Username: "admin.uji", Role: domain.RoleAdmin, BranchCode: "001"}

	unknown := createStaffInput()
	unknown.BranchCode = "HO"
	if _, err := svc.CreateStaff(context.Background(), unknown, actor); !errors.Is(err, domain.ErrStaffBranchUnknown) {
		t.Fatalf("cabang tak terdaftar harus ditolak ErrStaffBranchUnknown, dapat: %v", err)
	}

	empty := createStaffInput()
	empty.BranchCode = ""
	if _, err := svc.CreateStaff(context.Background(), empty, actor); !errors.Is(err, domain.ErrStaffBranchRequired) {
		t.Fatalf("cabang kosong harus ditolak ErrStaffBranchRequired, dapat: %v", err)
	}

	valid := createStaffInput()
	if _, err := svc.CreateStaff(context.Background(), valid, actor); err != nil {
		t.Fatalf("cabang terdaftar harus diterima: %v", err)
	}
}

// N4: penetapan cabang lewat update juga divalidasi.
func TestUpdateStaff_MenolakCabangTidakTerdaftar(t *testing.T) {
	id := uuid.New()
	repo := &stubStaffRepo{users: map[uuid.UUID]*domain.StaffUser{
		id: {ID: id, Username: "teller.uji", Role: domain.RoleTeller, BranchCode: "001", IsActive: true},
	}, passwords: map[uuid.UUID]string{}}
	svc := NewStaffService(repo, stubBranchChecker{codes: map[string]bool{"001": true}})
	actor := domain.Actor{UserID: uuid.New(), Username: "admin.uji", Role: domain.RoleAdmin, BranchCode: "001"}

	bad := "HO"
	if _, err := svc.UpdateStaff(context.Background(), id, domain.UpdateStaffInput{BranchCode: &bad}, actor); !errors.Is(err, domain.ErrStaffBranchUnknown) {
		t.Fatalf("penetapan cabang tak terdaftar harus ditolak, dapat: %v", err)
	}
	if repo.users[id].BranchCode != "001" {
		t.Fatalf("cabang staf berubah meski ditolak: %s", repo.users[id].BranchCode)
	}
}

// N4: peringatan start hanya untuk staf bercabang biasa; peran lintas cabang dengan
// kode 'HO' tidak diperingatkan.
func TestBranchCoverageWarnings_MenyaringPeranLintasCabang(t *testing.T) {
	reader := stubMismatchReader{items: []domain.StaffBranchMismatch{
		{Username: "adminho", BranchCode: "HO", Role: domain.RoleAdmin},
		{Username: "superadmin", BranchCode: "HO", Role: domain.RoleSuperAdmin},
	}}
	warnings := BranchCoverageWarnings(context.Background(), reader)
	if len(warnings) != 1 {
		t.Fatalf("peringatan %d, ingin 1: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "adminho") || !strings.Contains(warnings[0], "HO") {
		t.Fatalf("peringatan tidak menyebut staf/cabangnya: %q", warnings[0])
	}
}

// stubMismatchReader memenuhi domain.BranchScopeMismatchReader untuk uji peringatan.
type stubMismatchReader struct {
	items []domain.StaffBranchMismatch
	err   error
}

func (s stubMismatchReader) ListBranchScopeMismatches(context.Context) ([]domain.StaffBranchMismatch, error) {
	return s.items, s.err
}
