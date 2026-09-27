package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// ojkLoanCodesStoreStub meniru domain.OJKLoanCodesStore tanpa database. Perubahan
// langsung diterapkan ke loan agar GetByID kedua mengembalikan keadaan terbaru.
type ojkLoanCodesStoreStub struct {
	loan    *domain.Loan
	called  bool
	lastIn  domain.UpdateOJKLoanCodesInput
	getErr  error
	saveErr error
}

func (s *ojkLoanCodesStoreStub) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.loan, nil
}

func (s *ojkLoanCodesStoreStub) UpdateOJKLoanCodesTx(_ context.Context, _ any, _ uuid.UUID, input domain.UpdateOJKLoanCodesInput) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.called = true
	s.lastIn = input
	if input.OJKJenisPenggunaanCode != nil {
		s.loan.OJKJenisPenggunaanCode = *input.OJKJenisPenggunaanCode
	}
	if input.OJKPeriodePembayaranCode != nil {
		s.loan.OJKPeriodePembayaranCode = *input.OJKPeriodePembayaranCode
	}
	if input.OJKKabupatenCode != nil {
		s.loan.OJKKabupatenCode = *input.OJKKabupatenCode
	}
	return nil
}

func newOJKLoanCodesSvcForTest(loan *domain.Loan) (*ojkLoanCodesService, *ojkLoanCodesStoreStub) {
	store := &ojkLoanCodesStoreStub{loan: loan}
	svc := &ojkLoanCodesService{store: store, runner: memoryTxRunner{}}
	return svc, store
}

func TestOJKLoanCodesTolakKosong(t *testing.T) {
	svc, store := newOJKLoanCodesSvcForTest(&domain.Loan{ID: uuid.New(), BranchCode: "BR1"})
	_, err := svc.Update(context.Background(), store.loan.ID, domain.UpdateOJKLoanCodesInput{}, domain.Actor{BranchCode: "BR1"})
	if err == nil || err.Error() != domain.ErrOJKLoanCodesEmpty.Error() {
		t.Fatalf("err = %v, ingin ErrOJKLoanCodesEmpty", err)
	}
	if store.called {
		t.Fatal("tidak boleh menulis saat masukan kosong")
	}
}

func TestOJKLoanCodesTolakInlineDiLuarHimpunan(t *testing.T) {
	invalid := "99"
	svc, store := newOJKLoanCodesSvcForTest(&domain.Loan{ID: uuid.New(), BranchCode: "BR1"})
	_, err := svc.Update(context.Background(), store.loan.ID,
		domain.UpdateOJKLoanCodesInput{OJKJenisPenggunaanCode: &invalid},
		domain.Actor{BranchCode: "BR1"})
	code, _, ok := domain.LocalizedMessage(err)
	if !ok || code != "ojk_inline_code_invalid" {
		t.Fatalf("err = %v, ingin kode ojk_inline_code_invalid", err)
	}
	if store.called {
		t.Fatal("tidak boleh menulis saat validasi gagal")
	}
}

func TestOJKLoanCodesSimpanYangValid(t *testing.T) {
	jenis := "10"
	periode := "8"
	svc, store := newOJKLoanCodesSvcForTest(&domain.Loan{ID: uuid.New(), BranchCode: "BR1"})
	loan, err := svc.Update(context.Background(), store.loan.ID,
		domain.UpdateOJKLoanCodesInput{OJKJenisPenggunaanCode: &jenis, OJKPeriodePembayaranCode: &periode},
		domain.Actor{BranchCode: "BR1"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !store.called {
		t.Fatal("perubahan valid harus tersimpan")
	}
	if loan.OJKJenisPenggunaanCode != "10" || loan.OJKPeriodePembayaranCode != "8" {
		t.Fatalf("nilai tersimpan = %q/%q", loan.OJKJenisPenggunaanCode, loan.OJKPeriodePembayaranCode)
	}
}

func TestOJKLoanCodesTolakLintasCabang(t *testing.T) {
	jenis := "10"
	svc, store := newOJKLoanCodesSvcForTest(&domain.Loan{ID: uuid.New(), BranchCode: "BR2"})
	_, err := svc.Update(context.Background(), store.loan.ID,
		domain.UpdateOJKLoanCodesInput{OJKJenisPenggunaanCode: &jenis},
		domain.Actor{BranchCode: "BR1"})
	if err == nil || err.Error() != domain.ErrCrossBranchAccess.Error() {
		t.Fatalf("err = %v, ingin ErrCrossBranchAccess", err)
	}
	if store.called {
		t.Fatal("tidak boleh menulis kredit cabang lain")
	}
}
