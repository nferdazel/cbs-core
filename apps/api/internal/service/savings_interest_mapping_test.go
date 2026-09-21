package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

func savingsProductForMapping() *domain.BankingProduct {
	return &domain.BankingProduct{Code: "TAB-01", Book: domain.BookConventional}
}

// Tanpa pemetaan (product_journal_mapping kosong) akrual bunga tabungan tetap
// berjalan memakai COA bawaan per buku.
func TestResolveInterestCOAsTanpaPemetaanMemakaiBawaan(t *testing.T) {
	product := savingsProductForMapping()
	svc := &savingsInterestService{productRepo: &stubDepositProductRepo{product: product}}

	expense, payable, mapped, err := svc.resolveInterestCOAs(context.Background(), product, domain.BookConventional)
	if err != nil {
		t.Fatalf("tanpa pemetaan tidak boleh error: %v", err)
	}
	if mapped {
		t.Fatal("mapped harus false saat pemetaan produk kosong")
	}
	if expense == "" || payable == "" {
		t.Fatalf("COA bawaan harus terisi, dapat expense=%q payable=%q", expense, payable)
	}
}

// Galat pembacaan pemetaan harus menggagalkan, bukan diam-diam jatuh ke COA bawaan.
func TestResolveInterestCOAsGalatPembacaanMenggagalkan(t *testing.T) {
	product := savingsProductForMapping()
	svc := &savingsInterestService{productRepo: &stubDepositProductRepo{
		product:    product,
		mappingErr: errors.New("koneksi database terputus"),
	}}

	if _, _, _, err := svc.resolveInterestCOAs(context.Background(), product, domain.BookConventional); err == nil {
		t.Fatal("galat pembacaan pemetaan harus menggagalkan, bukan memakai COA bawaan")
	}
}

func TestAdminFeeRevenueCOATanpaPemetaanMemakaiBawaan(t *testing.T) {
	product := savingsProductForMapping()
	svc := &savingsInterestService{productRepo: &stubDepositProductRepo{product: product}}

	coa, err := svc.adminFeeRevenueCOA(context.Background(), product, domain.BookConventional)
	if err != nil {
		t.Fatalf("tanpa pemetaan tidak boleh error: %v", err)
	}
	if coa == "" {
		t.Fatal("COA bawaan pendapatan administrasi harus terisi")
	}
}

func TestAdminFeeRevenueCOAGalatPembacaanMenggagalkan(t *testing.T) {
	product := savingsProductForMapping()
	svc := &savingsInterestService{productRepo: &stubDepositProductRepo{
		product:    product,
		mappingErr: errors.New("koneksi database terputus"),
	}}

	if _, err := svc.adminFeeRevenueCOA(context.Background(), product, domain.BookConventional); err == nil {
		t.Fatal("galat pembacaan pemetaan harus menggagalkan, bukan memakai COA bawaan")
	}
}
