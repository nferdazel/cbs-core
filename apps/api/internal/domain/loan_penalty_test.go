package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// DPD 0 atau basis kosong tidak menghasilkan denda apa pun.
func TestLoanPenaltyAmount_ZeroDPD(t *testing.T) {
	base := decimal.NewFromInt(1_000_000)
	rate := decimal.NewFromInt(1) // 1‰ per hari

	if got := domain.LoanPenaltyAmount(base, 0, rate); !got.IsZero() {
		t.Fatalf("DPD 0 harus nol, got %s", got)
	}
	if got := domain.LoanPenaltyAmount(base, -3, rate); !got.IsZero() {
		t.Fatalf("DPD negatif harus nol, got %s", got)
	}
	if got := domain.LoanPenaltyAmount(decimal.Zero, 10, rate); !got.IsZero() {
		t.Fatalf("basis nol harus nol, got %s", got)
	}
}

// Tarif 0 (belum dikonfigurasi operator) tidak boleh mengarang denda.
func TestLoanPenaltyAmount_ZeroRate(t *testing.T) {
	got := domain.LoanPenaltyAmount(decimal.NewFromInt(5_000_000), 30, decimal.Zero)
	if !got.IsZero() {
		t.Fatalf("tarif 0 harus nol, got %s", got)
	}
}

// Denda = pokok tunggakan x tarif harian (‰/1000) x DPD.
func TestLoanPenaltyAmount_PositiveDPD(t *testing.T) {
	// 1.000.000 x (1/1000) x 10 hari = 10.000
	got := domain.LoanPenaltyAmount(decimal.NewFromInt(1_000_000), 10, decimal.NewFromInt(1))
	if !got.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("got %s, want 10.000", got)
	}

	// Tarif 0,5‰ per hari: 2.000.000 x 0,0005 x 4 = 4.000
	got = domain.LoanPenaltyAmount(decimal.NewFromInt(2_000_000), 4, decimal.NewFromFloat(0.5))
	if !got.Equal(decimal.NewFromInt(4_000)) {
		t.Fatalf("got %s, want 4.000", got)
	}
}

// Pembulatan memakai Banker's Rounding yang sama dengan seluruh perhitungan uang.
func TestLoanPenaltyAmount_RoundsToRupiah(t *testing.T) {
	// 2.500 x (1/1000) x 1 = 2,5 -> 2 (ke genap)
	got := domain.LoanPenaltyAmount(decimal.NewFromInt(2_500), 1, decimal.NewFromInt(1))
	if !got.Equal(decimal.NewFromInt(2)) {
		t.Fatalf("2,5 harus dibulatkan ke 2, got %s", got)
	}

	// 3.500 x (1/1000) x 1 = 3,5 -> 4 (ke genap)
	got = domain.LoanPenaltyAmount(decimal.NewFromInt(3_500), 1, decimal.NewFromInt(1))
	if !got.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("3,5 harus dibulatkan ke 4, got %s", got)
	}

	// 333.333 x 0,001 = 333,333 -> 333
	got = domain.LoanPenaltyAmount(decimal.NewFromInt(333_333), 1, decimal.NewFromInt(1))
	if !got.Equal(decimal.NewFromInt(333)) {
		t.Fatalf("333,333 harus dibulatkan ke 333, got %s", got)
	}
}

// Kredit yang sudah lunas atau hapus buku tidak boleh dikenai denda.
func TestLoanPenaltyEligible(t *testing.T) {
	eligible := []domain.LoanStatus{
		domain.LoanStatusDisbursed,
		domain.LoanStatusDefaulted,
	}
	for _, s := range eligible {
		if !domain.LoanPenaltyEligible(s) {
			t.Errorf("status %s seharusnya boleh dikenai denda", s)
		}
	}

	notEligible := []domain.LoanStatus{
		domain.LoanStatusPaidOff,
		domain.LoanStatusWrittenOff,
		domain.LoanStatusRejected,
		domain.LoanStatusPendingApproval,
		domain.LoanStatusApproved,
	}
	for _, s := range notEligible {
		if domain.LoanPenaltyEligible(s) {
			t.Errorf("status %s tidak boleh dikenai denda", s)
		}
	}
}
