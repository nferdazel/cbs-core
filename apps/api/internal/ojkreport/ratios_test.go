package ojkreport

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Dua digit terakhir: uji setiap rumus dengan angka bulat yang mudah diperiksa
// tangan. Dasar: Lampiran II SEOJK 16/SEOJK.03/2024 hlm. 204-205.
func TestRumusRasioForm0008(t *testing.T) {
	d := decimal.RequireFromString
	cases := []struct {
		nama   string
		hitung func() (decimal.Decimal, bool)
		want   string
	}{
		{"KPMM modal/ATMR", func() (decimal.Decimal, bool) { return RumusKPMM(d("120"), d("1000")) }, "12"},
		{"Rasio Cadangan terhadap PPKA", func() (decimal.Decimal, bool) { return RumusCadanganPPKA(d("80"), d("100")) }, "80"},
		{"NPL Gross", func() (decimal.Decimal, bool) { return RumusNPLGross(d("10"), d("20"), d("30"), d("1000")) }, "6"},
		{"NPL Neto", func() (decimal.Decimal, bool) { return RumusNPLNeto(d("10"), d("20"), d("30"), d("20"), d("1000")) }, "4"},
		{"ROA", func() (decimal.Decimal, bool) { return RumusROA(d("50"), d("2500")) }, "2"},
		{"BOPO", func() (decimal.Decimal, bool) { return RumusBOPO(d("600"), d("1000")) }, "60"},
		{"NIM", func() (decimal.Decimal, bool) { return RumusNIM(d("40"), d("800")) }, "5"},
		{"LDR", func() (decimal.Decimal, bool) { return RumusLDR(d("800"), d("1000")) }, "80"},
		{"Cash Ratio", func() (decimal.Decimal, bool) { return RumusCashRatio(d("150"), d("500")) }, "30"},
	}
	for _, c := range cases {
		got, ok := c.hitung()
		if !ok {
			t.Errorf("%s: dinyatakan tidak tersedia, ingin %s", c.nama, c.want)
			continue
		}
		if !got.Equal(d(c.want)) {
			t.Errorf("%s = %s, ingin %s", c.nama, got, c.want)
		}
	}
}

// Pembagian nol tidak boleh menghasilkan NaN/Inf maupun panic; hasilnya harus
// ditandai tidak tersedia.
func TestRumusRasioPenyebutNolTidakNaN(t *testing.T) {
	d := decimal.RequireFromString
	cases := []struct {
		nama   string
		hitung func() (decimal.Decimal, bool)
	}{
		{"KPMM", func() (decimal.Decimal, bool) { return RumusKPMM(d("120"), decimal.Zero) }},
		{"Cadangan PPKA", func() (decimal.Decimal, bool) { return RumusCadanganPPKA(d("80"), decimal.Zero) }},
		{"NPL Gross", func() (decimal.Decimal, bool) { return RumusNPLGross(d("10"), d("20"), d("30"), decimal.Zero) }},
		{"NPL Neto", func() (decimal.Decimal, bool) { return RumusNPLNeto(d("10"), d("20"), d("30"), d("20"), decimal.Zero) }},
		{"ROA", func() (decimal.Decimal, bool) { return RumusROA(d("50"), decimal.Zero) }},
		{"BOPO", func() (decimal.Decimal, bool) { return RumusBOPO(d("600"), decimal.Zero) }},
		{"NIM", func() (decimal.Decimal, bool) { return RumusNIM(d("40"), decimal.Zero) }},
		{"LDR", func() (decimal.Decimal, bool) { return RumusLDR(d("800"), decimal.Zero) }},
		{"Cash Ratio", func() (decimal.Decimal, bool) { return RumusCashRatio(d("150"), decimal.Zero) }},
	}
	for _, c := range cases {
		got, ok := c.hitung()
		if ok {
			t.Errorf("%s: penyebut nol harus tidak tersedia", c.nama)
		}
		if !got.IsZero() {
			t.Errorf("%s: hasil %s, ingin nol", c.nama, got)
		}
	}
}

// Komponen yang kosong harus dinyatakan tidak tersedia beserta alasannya, bukan nol.
func TestRasioKeuanganKomponenKosongTidakTersedia(t *testing.T) {
	hasil := RasioKeuangan(
		time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		map[string]decimal.Decimal{}, map[string]decimal.Decimal{},
	)
	if len(hasil) != 9 {
		t.Fatalf("ingin 9 baris rasio, dapat %d", len(hasil))
	}
	for _, h := range hasil {
		if h.Tersedia {
			t.Errorf("sandi %s tersedia padahal komponen kosong", h.Sandi)
		}
		if h.Alasan == "" {
			t.Errorf("sandi %s tidak tersedia tetapi tanpa alasan", h.Sandi)
		}
		if !h.NilaiPersen.IsZero() {
			t.Errorf("sandi %s nilai %s, ingin nol", h.Sandi, h.NilaiPersen)
		}
	}
}

// Posisi di luar triwulan dikosongkan walaupun komponennya ada (Lampiran II hlm. 204).
func TestRasioKeuanganBukanBulanTriwulanDikosongkan(t *testing.T) {
	amounts01 := map[string]decimal.Decimal{
		"1104010100": decimal.NewFromInt(800),
		"2102010100": decimal.NewFromInt(300),
		"2102020100": decimal.NewFromInt(500),
	}
	amounts02 := map[string]decimal.Decimal{
		"4100000000": decimal.NewFromInt(1000),
		"5100000000": decimal.NewFromInt(600),
	}

	hasil := RasioKeuangan(time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC), amounts01, amounts02)
	for _, h := range hasil {
		if h.Tersedia {
			t.Errorf("sandi %s harus dikosongkan di luar triwulan", h.Sandi)
		}
		if h.Alasan != alasanDikosongkan {
			t.Errorf("sandi %s alasan = %q", h.Sandi, h.Alasan)
		}
	}
}

// BOPO dan LDR dapat dihitung dari laporan yang ada; rasio lain tetap tidak tersedia.
func TestRasioKeuanganTersediaBOPOdanLDR(t *testing.T) {
	amounts01 := map[string]decimal.Decimal{
		"1104010100": decimal.NewFromInt(800),
		"2102010100": decimal.NewFromInt(400),
		"2102020100": decimal.NewFromInt(600),
	}
	amounts02 := map[string]decimal.Decimal{
		"4100000000": decimal.NewFromInt(1000),
		"5100000000": decimal.NewFromInt(600),
	}

	hasil := RasioKeuangan(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), amounts01, amounts02)
	bySandi := make(map[string]RasioHasil, len(hasil))
	for _, h := range hasil {
		bySandi[h.Sandi] = h
	}

	bopo := bySandi[sandiBOPO]
	if !bopo.Tersedia || !bopo.NilaiPersen.Equal(decimal.NewFromInt(60)) {
		t.Errorf("BOPO = %s (tersedia=%v), ingin 60", bopo.NilaiPersen, bopo.Tersedia)
	}
	ldr := bySandi[sandiLDR]
	if !ldr.Tersedia || !ldr.NilaiPersen.Equal(decimal.NewFromInt(80)) {
		t.Errorf("LDR = %s (tersedia=%v), ingin 80", ldr.NilaiPersen, ldr.Tersedia)
	}
	for _, sandi := range []string{sandiKPMM, sandiCadanganPPKA, sandiNPLNeto, sandiNPLGross, sandiROA, sandiNIM, sandiCashRatio} {
		if bySandi[sandi].Tersedia {
			t.Errorf("sandi %s seharusnya belum tersedia", sandi)
		}
	}
}

// Format keluaran: baris rasio memakai persen dua desimal, yang tidak tersedia
// ditulis "-" (bukan 0) beserta alasannya.
func TestWriteTextBarisRasioPersenDanTidakTersedia(t *testing.T) {
	b := &Bundle{
		Period:    time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd: time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
		Sections: []Section{
			{Form: "00.08", Name: "Rasio Keuangan Triwulanan", Lines: []Line{
				{Sandi: sandiBOPO, Name: "BOPO", Percent: true, Amount: decimal.RequireFromString("60")},
				{Sandi: sandiNPLGross, Name: "NPL Gross", Percent: true,
					UnavailableReason: "kualitas kredit belum tersedia"},
			}},
		},
	}

	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "00.08|0402|BOPO|60.00") {
		t.Fatalf("baris rasio harus persen 2 desimal; keluaran=%s", out)
	}
	if !strings.Contains(out, "00.08|0204|NPL Gross|-") {
		t.Fatalf("rasio tidak tersedia harus ditulis '-' bukan 0; keluaran=%s", out)
	}
	if strings.Contains(out, "0204|NPL Gross|0") {
		t.Fatal("rasio tidak tersedia tidak boleh ditulis 0")
	}
	if !strings.Contains(out, "TIDAK TERSEDIA: kualitas kredit belum tersedia") {
		t.Fatal("alasan rasio tidak tersedia harus dicatat")
	}
}

// Integrasi builder: Form 00.08 hadir dan terisi pada posisi triwulan.
func TestGenerateMonthlyMenyertakanForm0008(t *testing.T) {
	b, err := NewBuilder(newStubSource()).
		GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	line := findLine(t, b, "00.08", sandiBOPO)
	if line.UnavailableReason != "" {
		t.Fatalf("BOPO posisi triwulan harus terisi: %s", line.UnavailableReason)
	}
	if !line.Percent {
		t.Fatal("baris rasio harus bertanda persen")
	}
	if line.Amount.IsZero() {
		t.Fatal("BOPO tidak boleh nol pada data stub")
	}
	for _, f := range b.SkippedForms {
		if f.Form == "00.08" {
			t.Fatal("Form 00.08 yang buildable tidak boleh masuk daftar form tidak dibangun")
		}
	}

	bFeb, err := NewBuilder(newStubSource()).
		GenerateMonthly(context.Background(), time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	if got := findLine(t, bFeb, "00.08", sandiBOPO).UnavailableReason; got == "" {
		t.Fatal("posisi non-triwulan harus dikosongkan")
	}
}
