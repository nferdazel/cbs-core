package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubActorBranchRepo mengembalikan cabang dari peta kode dan kantor pusat dari
// daftar terpisah, meniru repo produksi yang mengurutkan List berdasarkan kode.
type stubActorBranchRepo struct {
	domain.BranchRepository
	byCode map[string]*domain.Branch
	list   []domain.Branch
}

func (s *stubActorBranchRepo) GetByCode(_ context.Context, code string) (*domain.Branch, error) {
	if b, ok := s.byCode[code]; ok {
		return b, nil
	}
	return nil, domain.ErrBranchNotFound
}

func (s *stubActorBranchRepo) List(context.Context) ([]domain.Branch, error) {
	return s.list, nil
}

func actorBranchFixture() *stubActorBranchRepo {
	return &stubActorBranchRepo{
		byCode: map[string]*domain.Branch{
			"001": {ID: uuid.New(), Code: "001", IsActive: true},
		},
		list: []domain.Branch{
			{ID: uuid.New(), Code: "001", Name: "Kantor Pusat", IsHeadOffice: true, IsActive: true},
		},
	}
}

func TestResolveActorBranch_LintasCabangKodeTakTerdaftarJatuhKeKantorPusat(t *testing.T) {
	repo := actorBranchFixture()
	branch, err := resolveActorBranch(context.Background(), repo, domain.Actor{
		Role:       domain.RoleSuperAdmin,
		BranchCode: "HO",
	})
	if err != nil {
		t.Fatalf("superadmin lintas cabang tidak boleh gagal: %v", err)
	}
	if branch == nil || !branch.IsHeadOffice {
		t.Fatalf("cabang superadmin harus kantor pusat, dapat %+v", branch)
	}
}

func TestResolveActorBranch_LintasCabangKodeKosongJatuhKeKantorPusat(t *testing.T) {
	repo := actorBranchFixture()
	branch, err := resolveActorBranch(context.Background(), repo, domain.Actor{Role: domain.RoleAuditor})
	if err != nil {
		t.Fatalf("auditor tanpa kode cabang tidak boleh gagal: %v", err)
	}
	if branch == nil || !branch.IsHeadOffice {
		t.Fatalf("cabang auditor harus kantor pusat, dapat %+v", branch)
	}
}

func TestResolveActorBranch_PeranCabangTakTerdaftarDitolak(t *testing.T) {
	repo := actorBranchFixture()
	_, err := resolveActorBranch(context.Background(), repo, domain.Actor{
		Role:       domain.RoleTeller,
		BranchCode: "HO",
	})
	if !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("teller dengan cabang tak terdaftar harus ErrBranchNotFound, dapat %v", err)
	}
}

func TestResolveActorBranch_PeranCabangKosongDitolak(t *testing.T) {
	repo := actorBranchFixture()
	if _, err := resolveActorBranch(context.Background(), repo, domain.Actor{Role: domain.RoleTeller}); err == nil {
		t.Fatal("teller tanpa kode cabang harus ditolak")
	}
}

func TestResolveActorBranch_CabangTerdaftarDipakaiApaAdanya(t *testing.T) {
	repo := actorBranchFixture()
	branch, err := resolveActorBranch(context.Background(), repo, domain.Actor{
		Role:       domain.RoleTeller,
		BranchCode: " 001 ",
	})
	if err != nil {
		t.Fatalf("cabang terdaftar: %v", err)
	}
	if branch == nil || branch.Code != "001" {
		t.Fatalf("cabang teller = %+v, ingin 001", branch)
	}
}

// Superadmin dengan kode kantor pusat yang tidak terdaftar tetap dapat menempatkan
// deposito; cabang kontrak diatribusikan ke kantor pusat, bukan NULL.
func TestPreparePlacementSuperadminTanpaCabangTerdaftar(t *testing.T) {
	branchID := uuid.New()
	svc := &depositService{
		productRepo: &stubDepositProductRepo{product: &domain.BankingProduct{
			ID:         uuid.New(),
			Code:       "DEP-CONV",
			Family:     domain.FamilyTimeDeposit,
			Book:       domain.BookConventional,
			IsActive:   true,
			MinAmount:  decimal.NewFromInt(1_000_000),
			RateAnnual: decimal.NewFromInt(4),
		}},
		customerRepo: &stubPlaceCustomerRepo{customer: &domain.CustomerRecord{
			ID:     uuid.New(),
			Status: domain.CustomerStatusActive,
		}},
		branchRepo: &stubActorBranchRepo{
			byCode: map[string]*domain.Branch{},
			list: []domain.Branch{
				{ID: branchID, Code: "001", Name: "Kantor Pusat", IsHeadOffice: true, IsActive: true},
			},
		},
		ledgerRepo: &stubDepositLedgerRepo{coa: &domain.ChartOfAccount{ID: uuid.New()}},
		configSvc:  &stubDepositConfig{values: map[string]decimal.Decimal{}},
	}

	prep, err := svc.preparePlacement(context.Background(), domain.PlaceDepositInput{
		CustomerID:      svc.customerRepo.(*stubPlaceCustomerRepo).customer.ID,
		ProductID:       svc.productRepo.(*stubDepositProductRepo).product.ID,
		PlacementAmount: decimal.NewFromInt(10_000_000),
		TermMonths:      3,
		Currency:        "IDR",
	}, domain.Actor{Role: domain.RoleSuperAdmin, BranchCode: "HO"})
	if err != nil {
		t.Fatalf("superadmin lintas cabang harus dapat menempatkan deposito: %v", err)
	}
	if prep.branch == nil || prep.branch.ID != branchID {
		t.Fatalf("deposito harus diatribusikan ke kantor pusat, dapat %+v", prep.branch)
	}
}
