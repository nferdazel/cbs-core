package ojkreport

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func rasioBySandi(hasil []RasioHasil, sandi string) RasioHasil {
	for _, h := range hasil {
		if h.Sandi == sandi {
			return h
		}
	}
	return RasioHasil{}
}

// NPL gross dan neto menjadi tersedia bila komponen kredit disuplai; nilainya sesuai
// Lampiran II hlm. 204 butir 3 dan 4.
func TestRasioKeuanganNPLTersediaDariKomponen(t *testing.T) {
	amounts01 := map[string]decimal.Decimal{}
	amounts02 := map[string]decimal.Decimal{}
	komponen := KomponenRasio{Kredit: KomponenKreditNPL{
		KurangLancar: decimal.NewFromInt(100),
		Diragukan:    decimal.NewFromInt(150),
		Macet:        decimal.NewFromInt(150),
		CKPNNPL:      decimal.NewFromInt(90),
		TotalKredit:  decimal.NewFromInt(1000),
		Tersedia:     true,
	}}

	hasil := RasioKeuangan(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), amounts01, amounts02, komponen)

	gross := rasioBySandi(hasil, sandiNPLGross)
	if !gross.Tersedia || !gross.NilaiPersen.Equal(decimal.NewFromInt(40)) {
		t.Errorf("NPL gross = %s tersedia=%v, ingin 40", gross.NilaiPersen, gross.Tersedia)
	}
	neto := rasioBySandi(hasil, sandiNPLNeto)
	if !neto.Tersedia || !neto.NilaiPersen.Equal(decimal.NewFromInt(31)) {
		t.Errorf("NPL neto = %s tersedia=%v, ingin 31", neto.NilaiPersen, neto.Tersedia)
	}
}

// Komponen kredit tersedia tetapi total kredit nol tetap tidak tersedia (bukan NaN).
func TestRasioKeuanganNPLTotalNolTidakTersedia(t *testing.T) {
	komponen := KomponenRasio{Kredit: KomponenKreditNPL{Tersedia: true, TotalKredit: decimal.Zero}}
	hasil := RasioKeuangan(time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), nil, nil, komponen)

	gross := rasioBySandi(hasil, sandiNPLGross)
	if gross.Tersedia {
		t.Fatal("total kredit nol harus membuat NPL tidak tersedia")
	}
	if gross.Alasan == "" {
		t.Fatal("NPL tidak tersedia harus punya alasan")
	}
}

// Komponen kredit belum tersedia: NPL ditulis tidak tersedia, bukan nol.
func TestRasioKeuanganKreditBelumTersedia(t *testing.T) {
	hasil := RasioKeuangan(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), nil, nil, KomponenRasio{})
	for _, sandi := range []string{sandiNPLGross, sandiNPLNeto} {
		h := rasioBySandi(hasil, sandi)
		if h.Tersedia || h.Alasan == "" {
			t.Fatalf("sandi %s harus tidak tersedia dengan alasan", sandi)
		}
	}
}

// ROA dihitung dari laba sebelum pajak dibagi rata-rata total aset yang diturunkan
// dari jurnal (Lampiran II hlm. 204 butir 5).
func TestRasioKeuanganROADariRataRataTotalAset(t *testing.T) {
	amounts02 := map[string]decimal.Decimal{
		"3104040300": decimal.NewFromInt(50), // laba sebelum pajak
	}
	komponen := KomponenRasio{RataRataTotalAset: decimal.NewFromInt(2500), RataRataTotalAsetTersedia: true}
	hasil := RasioKeuangan(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), nil, amounts02, komponen)

	roa := rasioBySandi(hasil, sandiROA)
	if !roa.Tersedia || !roa.NilaiPersen.Equal(decimal.NewFromInt(2)) {
		t.Errorf("ROA = %s tersedia=%v, ingin 2", roa.NilaiPersen, roa.Tersedia)
	}
}

// Rata-rata total aset nol membuat ROA tidak tersedia, bukan NaN.
func TestRasioKeuanganROAAsetNolTidakTersedia(t *testing.T) {
	amounts02 := map[string]decimal.Decimal{"3104040300": decimal.NewFromInt(50)}
	komponen := KomponenRasio{RataRataTotalAset: decimal.Zero, RataRataTotalAsetTersedia: true}
	hasil := RasioKeuangan(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), nil, amounts02, komponen)

	if roa := rasioBySandi(hasil, sandiROA); roa.Tersedia || roa.Alasan == "" {
		t.Fatalf("ROA dengan aset nol harus tidak tersedia dan beralasan, dapat tersedia=%v alasan=%q", roa.Tersedia, roa.Alasan)
	}
}
