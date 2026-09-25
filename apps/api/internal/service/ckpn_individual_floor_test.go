package service

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"

	"github.com/shopspring/decimal"
)

// Uji murni tanpa DB: pemetaan Loan <-> CKPNLoanSnapshot harus membawa
// ckpn_individual_target apa adanya. Nilai inilah yang menjadi lantai 12.4.g.1.c;
// bila pemetaan ini hilang, lantai diam-diam kembali memakai required_ckpn (angka
// kolektif) dan pengurang modal inti KPMM mengecil.
func TestSnapshotDanLoanFreshMembawaTargetIndividual(t *testing.T) {
	l := &domain.Loan{
		RequiredCKPN:         decimal.NewFromInt(60_000),
		CKPNIndividualTarget: decimal.NewFromInt(30_000),
	}
	s := ckpnSnapshotFromLoan(l)
	if !s.CKPNIndividualTarget.Equal(decimal.NewFromInt(30_000)) {
		t.Fatalf("snapshot CKPNIndividualTarget = %s, mau 30000", s.CKPNIndividualTarget)
	}
	if !s.RequiredCKPN.Equal(decimal.NewFromInt(60_000)) {
		t.Fatalf("snapshot RequiredCKPN = %s, mau 60000", s.RequiredCKPN)
	}

	// loanFresh dipakai jalur tulis terkunci (apply); lantai membaca dari sini.
	f := loanFresh(s)
	if !f.CKPNIndividualTarget.Equal(decimal.NewFromInt(30_000)) {
		t.Fatalf("loanFresh CKPNIndividualTarget = %s, mau 30000", f.CKPNIndividualTarget)
	}
}
