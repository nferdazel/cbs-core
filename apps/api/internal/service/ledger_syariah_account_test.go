package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Rekening tanpa produk (data lama) tetap punya buku COA: sifat syariahnya harus
// ditentukan dari buku itu, bukan hilang menjadi konvensional.
func TestIsSyariahAccountTanpaProdukDariBukuCOA(t *testing.T) {
	svc := &ledgerService{productRepo: &stubProductRepo{}}

	if !svc.isSyariahAccount(context.Background(), &domain.Account{COABook: domain.BookSyariah}) {
		t.Fatal("rekening syariah tanpa produk harus terdeteksi syariah dari buku COA")
	}
	if svc.isSyariahAccount(context.Background(), &domain.Account{COABook: domain.BookConventional}) {
		t.Fatal("rekening konvensional tanpa produk tidak boleh dianggap syariah")
	}
}

// Bila produk ada, bukunya tetap sumber utama meski buku COA belum terisi.
func TestIsSyariahAccountDenganProduk(t *testing.T) {
	productID := uuid.New()
	svc := &ledgerService{productRepo: &stubProductRepo{
		product: &domain.BankingProduct{Book: domain.BookSyariah},
	}}

	if !svc.isSyariahAccount(context.Background(), &domain.Account{ProductID: &productID}) {
		t.Fatal("produk syariah harus terbaca meski buku COA rekening kosong")
	}
}

// Produk yang tidak lagi terbaca (mis. baris produk terhapus) tidak boleh membuat
// rekening syariah diam-diam menjadi konvensional; buku COA dipakai sebagai cadangan.
func TestIsSyariahAccountProdukGagalDibacaMemakaiBukuCOA(t *testing.T) {
	productID := uuid.New()
	svc := &ledgerService{productRepo: &stubProductRepo{}} // GetByID selalu gagal

	acc := &domain.Account{ProductID: &productID, COABook: domain.BookSyariah}
	if !svc.isSyariahAccount(context.Background(), acc) {
		t.Fatal("bila produk tidak terbaca, buku COA harus menjadi sumber cadangan")
	}
}

// Kas lawan untuk rekening syariah tanpa produk harus kas syariah, bukan kas
// konvensional default.
func TestResolveCashAccountRekeningSyariahTanpaProduk(t *testing.T) {
	svc := &ledgerService{productRepo: &stubProductRepo{}, resolver: stubLedgerResolver{}}

	code, err := svc.resolveCashAccount(context.Background(), &domain.Account{COABook: domain.BookSyariah})
	if err != nil {
		t.Fatalf("resolveCashAccount: %v", err)
	}
	if want := "GL-" + defaultCashCOASYariah; code != want {
		t.Fatalf("akun kas = %q, ingin %q (kas syariah)", code, want)
	}
}
