package domain_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// Hasil gerbang membawa status yang dipakai endpoint baca-saja: keadaan kategori,
// ambang cakupan, dan umur mode bayangan. Uji murni ini mengunci perhitungannya.
func TestCollateralWeightResultMelaporkanStatusBayangan(t *testing.T) {
	asOf := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)

	kasus := []struct {
		nama       string
		start      *time.Time
		wantMonths int
	}{
		{"tiga bulan penuh", cwTimePtr(asOf.AddDate(0, -3, 0)), 3},
		{"belum sebulan", cwTimePtr(asOf.AddDate(0, 0, -10)), 0},
		{"lewat tanggal pada bulan berjalan", cwTimePtr(time.Date(2026, 6, 25, 0, 0, 0, 0, time.UTC)), 2},
		{"mulai di masa depan", cwTimePtr(asOf.AddDate(0, 1, 0)), 0},
		{"belum dimulai", nil, -1},
	}
	for _, k := range kasus {
		req := validWeightRequest()
		req.AsOf = asOf
		req.Category.ShadowStartedAt = k.start
		res := domain.EvaluateCollateralWeightActivation(req)
		if res.ShadowMonthsElapsed != k.wantMonths {
			t.Errorf("%s: bulan berjalan %d, mau %d", k.nama, res.ShadowMonthsElapsed, k.wantMonths)
		}
		if res.ShadowMonthsRequired != 2 {
			t.Errorf("%s: bulan minimum %d, mau 2", k.nama, res.ShadowMonthsRequired)
		}
		if res.CategoryEnabled {
			t.Errorf("%s: kategori dilaporkan menyala padahal belum diaktifkan", k.nama)
		}
		if res.CategoryCode == "" {
			t.Errorf("%s: kode kategori tidak disalin ke hasil", k.nama)
		}
		if !res.CoverageMinFrac.Equal(req.CoverageMinFrac) {
			t.Errorf("%s: ambang cakupan %s, mau %s", k.nama, res.CoverageMinFrac, req.CoverageMinFrac)
		}
	}

	// Mode bayangan yang cukup bulan tetap TIDAK meloloskan gerbang bila syarat lain
	// gagal: status baca tidak boleh menggoda pemanggil menyalakan kategori langsung.
	req := validWeightRequest()
	req.AsOf = asOf
	req.Category.ShadowStartedAt = cwTimePtr(asOf.AddDate(0, -3, 0))
	req.Maker = ""
	res := domain.EvaluateCollateralWeightActivation(req)
	if res.Allowed {
		t.Fatalf("gate lolos tanpa maker; failures=%v", res.Failures)
	}
	if !cwHasFailure(res, domain.CollateralWeightC9) {
		t.Fatalf("C9 tidak disebut gagal; failures=%v", res.Failures)
	}
}
