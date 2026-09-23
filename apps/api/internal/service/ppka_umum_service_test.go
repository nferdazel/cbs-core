package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// ppkaPPAPStub hanya menyediakan Preview; metode lain diwarisi nil dari antarmuka.
type ppkaPPAPStub struct {
	domain.PPAPService
	summary domain.PPAPRunSummary
}

func (s ppkaPPAPStub) Preview(context.Context, time.Time, domain.Actor) (domain.PPAPRunSummary, error) {
	return s.summary, nil
}

// ppkaPlacementStub hanya menyediakan Calculate.
type ppkaPlacementStub struct {
	domain.LPSPlacementService
	summary domain.LPSPlacementSummary
}

func (s ppkaPlacementStub) Calculate(context.Context, time.Time, domain.Actor) (domain.LPSPlacementSummary, error) {
	return s.summary, nil
}

// TestPPKAUmumMenjumlahkanKreditDanPenempatan memverifikasi hitungan tangan:
// dasar = kredit lancar 1.000 + penempatan lancar 2.000 = 3.000; tarif 0,5% =>
// kredit 5 + penempatan 10 = 15.
func TestPPKAUmumMenjumlahkanKreditDanPenempatan(t *testing.T) {
	svc := NewPPKAUmumService(
		ppkaPPAPStub{summary: domain.PPAPRunSummary{Items: []domain.PPAPRunItem{
			{Collectibility: domain.KolLancar, Exposure: kpmmD("1000")},
			{Collectibility: domain.KolDPK, Exposure: kpmmD("500")},
		}}},
		ppkaPlacementStub{summary: domain.LPSPlacementSummary{
			Enabled: true,
			Items: []domain.LPSPlacementItem{
				{Collectibility: domain.KolLancar, Base: kpmmD("2000"), GeneralPPKA: kpmmD("10")},
			},
			TotalGeneralPPKA: kpmmD("10"),
		}},
		newKPMMConfig(),
	)
	got, err := svc.Hitung(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}
	if !got.KreditLancarDasar.Equal(kpmmD("1000")) || !got.KreditLancarPPKA.Equal(kpmmD("5")) {
		t.Fatalf("kredit = dasar %s ppka %s, mau 1000 / 5", got.KreditLancarDasar, got.KreditLancarPPKA)
	}
	if !got.PenempatanLancarDasar.Equal(kpmmD("2000")) || !got.PenempatanLancarPPKA.Equal(kpmmD("10")) {
		t.Fatalf("penempatan = dasar %s ppka %s, mau 2000 / 10", got.PenempatanLancarDasar, got.PenempatanLancarPPKA)
	}
	if !got.TotalDasar.Equal(kpmmD("3000")) || !got.TotalPPKA.Equal(kpmmD("15")) {
		t.Fatalf("total = dasar %s ppka %s, mau 3000 / 15", got.TotalDasar, got.TotalPPKA)
	}
	if !got.Lengkap {
		t.Fatalf("harus lengkap, alasan=%v", got.AlasanTidakLengkap)
	}
}

// TestPPKAUmumPenempatanMatiTidakLengkap: saklar ppap.lps.enabled mati membuat
// komponen penempatan belum tersedia; hasilnya ditandai TIDAK lengkap, bukan
// diperlakukan seolah nol adalah angka final.
func TestPPKAUmumPenempatanMatiTidakLengkap(t *testing.T) {
	svc := NewPPKAUmumService(
		ppkaPPAPStub{summary: domain.PPAPRunSummary{Items: []domain.PPAPRunItem{
			{Collectibility: domain.KolLancar, Exposure: kpmmD("1000")},
		}}},
		ppkaPlacementStub{summary: domain.LPSPlacementSummary{Enabled: false}},
		newKPMMConfig(),
	)
	got, err := svc.Hitung(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}
	if got.Lengkap {
		t.Fatal("harus tidak lengkap saat penempatan belum tersedia")
	}
	if !strings.Contains(strings.Join(got.AlasanTidakLengkap, " | "), "ppap.lps.enabled") {
		t.Fatalf("alasan harus menyebut saklar penempatan: %v", got.AlasanTidakLengkap)
	}
}
