package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// ojkPlacementCodesStoreStub meniru domain.OJKPlacementCodesStore tanpa database.
// Perubahan langsung diterapkan ke placement agar GetPlacementByID kedua mengembalikan
// keadaan terbaru.
type ojkPlacementCodesStoreStub struct {
	placement *domain.LPSPlacement
	called    bool
	lastIn    domain.UpdateOJKPlacementCodesInput
	getErr    error
	saveErr   error
}

func (s *ojkPlacementCodesStoreStub) GetPlacementByID(context.Context, uuid.UUID) (*domain.LPSPlacement, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.placement, nil
}

func (s *ojkPlacementCodesStoreStub) UpdateOJKPlacementCodesTx(_ context.Context, _ any, _ uuid.UUID, input domain.UpdateOJKPlacementCodesInput) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.called = true
	s.lastIn = input
	if input.BlockedAmount != nil {
		nd, err := domain.ParseOJKAmount(domain.OJKBlockedAmountField, *input.BlockedAmount)
		if err != nil {
			return err
		}
		if nd.Valid {
			v := nd.Decimal
			s.placement.BlockedAmount = &v
		} else {
			s.placement.BlockedAmount = nil
		}
	}
	return nil
}

func newOJKPlacementCodesSvcForTest(p *domain.LPSPlacement) (*ojkPlacementCodesService, *ojkPlacementCodesStoreStub) {
	store := &ojkPlacementCodesStoreStub{placement: p}
	svc := &ojkPlacementCodesService{store: store, runner: memoryTxRunner{}}
	return svc, store
}

func TestOJKPlacementCodesTolakKosong(t *testing.T) {
	svc, store := newOJKPlacementCodesSvcForTest(&domain.LPSPlacement{ID: uuid.New(), BranchCode: "BR1"})
	_, err := svc.Update(context.Background(), store.placement.ID, domain.UpdateOJKPlacementCodesInput{}, domain.Actor{BranchCode: "BR1"})
	if err == nil || err.Error() != domain.ErrOJKPlacementCodesEmpty.Error() {
		t.Fatalf("err = %v, ingin ErrOJKPlacementCodesEmpty", err)
	}
	if store.called {
		t.Fatal("tidak boleh menulis saat masukan kosong")
	}
}

func TestOJKPlacementCodesTolakInlineDiLuarHimpunan(t *testing.T) {
	invalid := "11" // 11 hanya berlaku untuk nasabah, bukan penempatan.
	svc, store := newOJKPlacementCodesSvcForTest(&domain.LPSPlacement{ID: uuid.New(), BranchCode: "BR1"})
	_, err := svc.Update(context.Background(), store.placement.ID,
		domain.UpdateOJKPlacementCodesInput{OJKHubunganBankCode: &invalid},
		domain.Actor{BranchCode: "BR1"})
	code, _, ok := domain.LocalizedMessage(err)
	if !ok || code != "ojk_inline_code_invalid" {
		t.Fatalf("err = %v, ingin kode ojk_inline_code_invalid", err)
	}
	if store.called {
		t.Fatal("tidak boleh menulis saat validasi gagal")
	}
}

func TestOJKPlacementCodesSimpanNominal(t *testing.T) {
	amount := "1500.50"
	svc, store := newOJKPlacementCodesSvcForTest(&domain.LPSPlacement{ID: uuid.New(), BranchCode: "BR1"})
	placement, err := svc.Update(context.Background(), store.placement.ID,
		domain.UpdateOJKPlacementCodesInput{BlockedAmount: &amount},
		domain.Actor{BranchCode: "BR1"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !store.called {
		t.Fatal("perubahan valid harus tersimpan")
	}
	if placement.BlockedAmount == nil || placement.BlockedAmount.String() != "1500.5" {
		t.Fatalf("blocked_amount = %v, ingin 1500.5", placement.BlockedAmount)
	}
}

func TestOJKPlacementCodesTolakLintasCabang(t *testing.T) {
	amount := "10"
	svc, store := newOJKPlacementCodesSvcForTest(&domain.LPSPlacement{ID: uuid.New(), BranchCode: "BR2"})
	_, err := svc.Update(context.Background(), store.placement.ID,
		domain.UpdateOJKPlacementCodesInput{BlockedAmount: &amount},
		domain.Actor{BranchCode: "BR1"})
	if err == nil || err.Error() != domain.ErrCrossBranchAccess.Error() {
		t.Fatalf("err = %v, ingin ErrCrossBranchAccess", err)
	}
	if store.called {
		t.Fatal("tidak boleh menulis penempatan cabang lain")
	}
}

func TestOJKPlacementCodesGetKosongJadiNull(t *testing.T) {
	svc, store := newOJKPlacementCodesSvcForTest(&domain.LPSPlacement{
		ID:               uuid.New(),
		BranchCode:       "BR1",
		CounterpartyBank: "Bank Uji",
	})
	view, err := svc.Get(context.Background(), store.placement.ID, domain.Actor{BranchCode: "BR1"})
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if view.OJKHubunganBankCode != nil {
		t.Fatal("sandi kosong harus null, bukan string kosong")
	}
	if view.OJKKabupatenCode != nil || view.BlockedAmount != nil {
		t.Fatalf("nilai belum diisi harus null: %+v", view)
	}
	if view.Label != "Bank Uji" {
		t.Fatalf("label = %q", view.Label)
	}
}
