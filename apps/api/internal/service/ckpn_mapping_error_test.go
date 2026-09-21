package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Galat pembacaan pemetaan BUKAN "produk belum dipetakan": kredit harus gagal dan
// TIDAK boleh tanpa jejak terjurnal ke COA bawaan. Kasus "tanpa pemetaan tetap
// memakai bawaan" dibuktikan TestCKPN_RunMempostingSelisihDanMenyimpanTarget.
func TestCKPN_GalatPembacaanPemetaanMenggagalkanKredit(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	productID := uuid.New()
	snap := ckpnLoan()
	snap.ProductID = &productID
	snap.BranchCode = "001"
	snap.RequiredCKPN = decimal.NewFromInt(100_000)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, _ := newTestCKPNService(repo, cfg)
	// Produk terbaca, tetapi pemetaan jurnalnya gagal dibaca (galat sesaat).
	svc.productRepo = &stubProductRepo{
		product:    &domain.BankingProduct{ID: productID, Code: "KRD-01", Book: domain.BookConventional},
		mappingErr: errors.New("koneksi database terputus"),
	}

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester", BranchCode: "001"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("galat pemetaan harus menggagalkan kredit, dapat failed=%d processed=%d", summary.Failed, summary.Processed)
	}
	if len(summary.Failures) != 1 || !strings.Contains(summary.Failures[0].Error, "membaca pemetaan jurnal CKPN") {
		t.Fatalf("pesan gagal tidak menyebut galat pembacaan pemetaan: %+v", summary.Failures)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("tidak boleh ada jurnal bawaan saat pemetaan gagal dibaca, dapat %d", len(posting.requests))
	}
}
