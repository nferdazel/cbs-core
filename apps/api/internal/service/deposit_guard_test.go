package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubDepositLedgerRepo hanya melayani GetCOAByCode yang dipakai preparePlacement.
type stubDepositLedgerRepo struct {
	domain.LedgerRepository
	coa *domain.ChartOfAccount
}

func (s *stubDepositLedgerRepo) GetCOAByCode(ctx context.Context, code string) (*domain.ChartOfAccount, error) {
	if s.coa == nil {
		return nil, domain.ErrAccountNotFound
	}
	return s.coa, nil
}

// newGuardDepositService merakit depositService minimal untuk menguji penjaga batas
// sebelum penulisan menyentuh database. Penjaga sengaja dijalankan sebelum transaksi
// dibuka, sehingga test dapat memakai db=nil untuk membuktikan tidak ada penulisan.
func newGuardDepositService(limits domain.TransactionLimitService, approvals domain.MakerCheckerService) *depositService {
	branchID := uuid.New()
	return &depositService{
		productRepo: &stubDepositProductRepo{product: &domain.BankingProduct{
			ID:         uuid.New(),
			Code:       "DEP-CONV",
			Family:     domain.FamilyTimeDeposit,
			Book:       domain.BookConventional,
			IsActive:   true,
			MinAmount:  decimal.NewFromInt(1_000_000),
			RateAnnual: decimal.NewFromInt(4),
			TaxRate:    decimal.NewFromInt(20),
		}},
		customerRepo: &stubPlaceCustomerRepo{customer: &domain.CustomerRecord{
			ID:       uuid.New(),
			Status:   domain.CustomerStatusActive,
			BranchID: &branchID,
		}},
		branchRepo: &stubPlaceBranchRepo{branch: &domain.Branch{ID: branchID, Code: "001", IsActive: true}},
		ledgerRepo: &stubDepositLedgerRepo{coa: &domain.ChartOfAccount{ID: uuid.New()}},
		configSvc:  &stubDepositConfig{values: map[string]decimal.Decimal{}},
		limits:     limits,
		approvals:  approvals,
	}
}

// Penempatan deposito di atas ambang persetujuan harus DIALIHKAN ke maker-checker dan
// TIDAK menulis apa pun (db=nil akan panik bila penulisan dicoba).
func TestDepositPlaceDialihkanKePersetujuan(t *testing.T) {
	approvals := &stubApprovals{}
	svc := newGuardDepositService(stubLimits{checkErr: domain.ErrRequiresApproval}, approvals)

	_, err := svc.Place(context.Background(), domain.PlaceDepositInput{
		CustomerID:      uuid.New(),
		ProductID:       uuid.New(),
		PlacementAmount: decimal.NewFromInt(150_000_000),
		TermMonths:      3,
		Currency:        "IDR",
	}, testActor())

	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("harus dialihkan ke persetujuan, dapat %v", err)
	}
	if pending.ActionType != ActionPlaceDeposit {
		t.Fatalf("jenis aksi %q, mau %q", pending.ActionType, ActionPlaceDeposit)
	}
	if len(approvals.created) != 1 {
		t.Fatalf("harus ada satu pengajuan persetujuan, dapat %d", len(approvals.created))
	}
	if approvals.created[0].Payload["maker_username"] != testActor().Username {
		t.Fatalf("payload harus memuat identitas pembuat, dapat %v", approvals.created[0].Payload)
	}
}

// Nominal yang melebihi batas keras ditolak tegas, tidak dialihkan ke persetujuan.
func TestDepositPlaceDitolakBatasKeras(t *testing.T) {
	approvals := &stubApprovals{}
	svc := newGuardDepositService(stubLimits{checkErr: domain.ErrLimitPerTransaction}, approvals)

	_, err := svc.Place(context.Background(), domain.PlaceDepositInput{
		CustomerID:      uuid.New(),
		ProductID:       uuid.New(),
		PlacementAmount: decimal.NewFromInt(60_000_000),
		TermMonths:      3,
	}, testActor())
	if !errors.Is(err, domain.ErrLimitPerTransaction) {
		t.Fatalf("mau ErrLimitPerTransaction, dapat %v", err)
	}
	if len(approvals.created) != 0 {
		t.Fatal("batas keras tidak boleh menjadi pengajuan persetujuan")
	}
}

// Payload persetujuan harus dapat dipulihkan utuh, termasuk identitas teller yang
// mencatat transaksi (bukan pejabat yang menyetujui).
func TestPlaceDepositPayloadRoundTrip(t *testing.T) {
	maker := domain.Actor{UserID: uuid.New(), Username: "teller01", Role: domain.RoleTeller, BranchCode: "001"}
	input := domain.PlaceDepositInput{
		CustomerID:      uuid.New(),
		ProductID:       uuid.New(),
		PlacementAmount: decimal.NewFromInt(150_000_000),
		TermMonths:      3,
		Currency:        "IDR",
		BranchCode:      "001",
	}

	payload, err := placeDepositPayload(input, maker)
	if err != nil {
		t.Fatalf("placeDepositPayload: %v", err)
	}
	got, gotMaker, err := placeDepositInputFromPayload(payload, domain.Actor{})
	if err != nil {
		t.Fatalf("placeDepositInputFromPayload: %v", err)
	}
	if got.CustomerID != input.CustomerID || got.ProductID != input.ProductID ||
		got.TermMonths != input.TermMonths || !got.PlacementAmount.Equal(input.PlacementAmount) {
		t.Fatalf("input tidak utuh: %+v", got)
	}
	if gotMaker.UserID != maker.UserID || gotMaker.Username != maker.Username ||
		gotMaker.Role != maker.Role || gotMaker.BranchCode != maker.BranchCode {
		t.Fatalf("identitas pembuat tidak utuh: %+v", gotMaker)
	}
}
