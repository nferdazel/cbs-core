package domain_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Validasi proyeksi arus kas manual T1: masukan operator yang salah DITOLAK sebelum
// disimpan, bukan dihitung diam-diam.
func TestCKPNCashflowProjectionValidation(t *testing.T) {
	kasus := []struct {
		nama string
		in   []domain.CKPNCashflowProjection
	}{
		{"kosong", nil},
		{"periode nol", []domain.CKPNCashflowProjection{{Period: 0, Amount: decimal.NewFromInt(100)}}},
		{"periode duplikat", []domain.CKPNCashflowProjection{
			{Period: 1, Amount: decimal.NewFromInt(100)},
			{Period: 1, Amount: decimal.NewFromInt(200)},
		}},
		{"nominal negatif", []domain.CKPNCashflowProjection{{Period: 1, Amount: decimal.NewFromInt(-1)}}},
	}
	for _, c := range kasus {
		if err := domain.ValidateCKPNCashflowProjections(c.in); !errors.Is(err, domain.ErrCKPNProjectionInvalid) {
			t.Fatalf("%s: err=%v, mau ErrCKPNProjectionInvalid", c.nama, err)
		}
	}

	sah := []domain.CKPNCashflowProjection{{Period: 1, Amount: decimal.NewFromInt(100)}}
	if err := domain.ValidateCKPNCashflowProjections(sah); err != nil {
		t.Fatalf("proyeksi sah ditolak: %v", err)
	}
}

// DCF T1: PV = Σ CF/(1+eir)^t dan target = max(0, carrying - PV). Contoh mudah:
// 110 pada periode 1 dengan EIR 10%/bulan -> PV 100. EIR tidak ada -> ErrEIRMissing,
// bukan nol (PA BPR 12.4.g.1.a).
func TestCKPNIndividualDCF(t *testing.T) {
	ctx := decimal.NewFromFloat(0.10)
	proyeksi := []domain.CKPNCashflowProjection{{Period: 1, Amount: decimal.NewFromInt(110)}}

	pv, target, err := domain.CKPNIndividualDCF(decimal.NewFromInt(100), ctx, proyeksi)
	if err != nil {
		t.Fatalf("DCF sah gagal: %v", err)
	}
	if !pv.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("PV %s, mau 100", pv)
	}
	if !target.IsZero() {
		t.Fatalf("target %s, mau 0 (carrying = PV)", target)
	}

	// carrying lebih besar -> target = selisih.
	pv, target, err = domain.CKPNIndividualDCF(decimal.NewFromInt(150), ctx, proyeksi)
	if err != nil {
		t.Fatalf("DCF sah gagal: %v", err)
	}
	if !pv.Equal(decimal.NewFromInt(100)) || !target.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("PV/target = %s/%s, mau 100/50", pv, target)
	}

	// PV melebihi carrying -> target 0, tidak negatif.
	if _, target, err = domain.CKPNIndividualDCF(decimal.NewFromInt(50), ctx, proyeksi); err != nil {
		t.Fatalf("DCF sah gagal: %v", err)
	} else if !target.IsZero() {
		t.Fatalf("target %s, mau 0 (tidak boleh negatif)", target)
	}

	// EIR kosong/nol -> ErrEIRMissing.
	if _, _, err := domain.CKPNIndividualDCF(decimal.NewFromInt(100), decimal.Zero, proyeksi); !errors.Is(err, domain.ErrEIRMissing) {
		t.Fatalf("EIR nol: err=%v, mau ErrEIRMissing", err)
	}

	// Proyeksi tidak sah ditolak sebelum menghitung.
	if _, _, err := domain.CKPNIndividualDCF(decimal.NewFromInt(100), ctx, nil); !errors.Is(err, domain.ErrCKPNProjectionInvalid) {
		t.Fatalf("proyeksi kosong: err=%v, mau ErrCKPNProjectionInvalid", err)
	}
}
