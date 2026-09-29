package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// buildPinjamanValid adalah masukan sah yang dipakai banyak kasus uji.
func buildPinjamanValid() UpdatePinjamanItemInput {
	return UpdatePinjamanItemInput{
		CounterpartyID:             "  KRT-001 ",
		CreditorGroupCode:          "600",
		BankCode:                   "123456",
		LocationCode:               "1101",
		JenisCode:                  "10",
		RelationshipCode:           "12",
		StartDate:                  "2025-01-02",
		MaturityDate:               "2026-01-02",
		InterestRate:               decimal.NewFromInt(9),
		InterestCalcCode:           "11",
		Plafon:                     decimal.NewFromInt(1_000_000_000),
		CollateralTypeCode:         "02",
		CollateralAmount:           decimal.NewFromInt(1_500_000_000),
		BakiDebet:                  decimal.NewFromInt(800_000_000),
		UnamortizedTransactionCost: decimal.NewFromInt(5_000_000),
		UnamortizedDiscount:        decimal.NewFromInt(2_000_000),
		AsOf:                       "2026-03-31",
	}
}

// BuildPinjamanItem menerima masukan sah, menormalkan teks, mengurai tiga tanggal,
// memberi id, dan mengisi status bawaan AKTIF.
func TestBuildPinjamanItemSah(t *testing.T) {
	item, err := BuildPinjamanItem(buildPinjamanValid())
	if err != nil {
		t.Fatalf("masukan sah: %v", err)
	}
	if item.ID == [16]byte{} {
		t.Fatal("id kosong tidak dibangkitkan")
	}
	if item.CounterpartyID != "KRT-001" {
		t.Fatalf("counterparty_id tidak dinormalkan: %q", item.CounterpartyID)
	}
	if item.Status != PinjamanStatusAktif {
		t.Fatalf("status bawaan = %q, ingin %q", item.Status, PinjamanStatusAktif)
	}
	if !item.StartDate.Equal(time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("start_date = %s", item.StartDate)
	}
	if !item.AsOf.Equal(time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("as_of = %s", item.AsOf)
	}
}

// Masukan tidak sah ditolak dengan ErrPinjamanInputInvalid: sandi asing, sandi bank
// bukan 6 digit, angka negatif, tanggal/status salah, dan id bukan UUID.
func TestBuildPinjamanItemTidakSah(t *testing.T) {
	cases := []struct {
		nama string
		ubah func(*UpdatePinjamanItemInput)
	}{
		{"id pihak lawan kosong", func(in *UpdatePinjamanItemInput) { in.CounterpartyID = "  " }},
		{"gol kreditur kosong", func(in *UpdatePinjamanItemInput) { in.CreditorGroupCode = "" }},
		{"sandi bank bukan 6 digit", func(in *UpdatePinjamanItemInput) { in.BankCode = "12345" }},
		{"sandi bank bukan angka", func(in *UpdatePinjamanItemInput) { in.BankCode = "12345a" }},
		{"lokasi kosong", func(in *UpdatePinjamanItemInput) { in.LocationCode = "" }},
		{"jenis asing", func(in *UpdatePinjamanItemInput) { in.JenisCode = "11" }},
		{"hubungan asing", func(in *UpdatePinjamanItemInput) { in.RelationshipCode = "13" }},
		{"cara perhitungan asing", func(in *UpdatePinjamanItemInput) { in.InterestCalcCode = "13" }},
		{"suku bunga negatif", func(in *UpdatePinjamanItemInput) { in.InterestRate = decimal.NewFromInt(-1) }},
		{"plafon negatif", func(in *UpdatePinjamanItemInput) { in.Plafon = decimal.NewFromInt(-1) }},
		{"jenis agunan kosong", func(in *UpdatePinjamanItemInput) { in.CollateralTypeCode = "" }},
		{"nominal agunan negatif", func(in *UpdatePinjamanItemInput) { in.CollateralAmount = decimal.NewFromInt(-1) }},
		{"baki debet negatif", func(in *UpdatePinjamanItemInput) { in.BakiDebet = decimal.NewFromInt(-1) }},
		{"biaya transaksi negatif", func(in *UpdatePinjamanItemInput) { in.UnamortizedTransactionCost = decimal.NewFromInt(-1) }},
		{"diskonto negatif", func(in *UpdatePinjamanItemInput) { in.UnamortizedDiscount = decimal.NewFromInt(-1) }},
		{"status asing", func(in *UpdatePinjamanItemInput) { in.Status = "HAPUS" }},
		{"start_date kosong", func(in *UpdatePinjamanItemInput) { in.StartDate = "" }},
		{"maturity_date salah format", func(in *UpdatePinjamanItemInput) { in.MaturityDate = "02-01-2026" }},
		{"as_of kosong", func(in *UpdatePinjamanItemInput) { in.AsOf = "" }},
		{"id bukan uuid", func(in *UpdatePinjamanItemInput) { in.ID = "bukan-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			in := buildPinjamanValid()
			tc.ubah(&in)
			if _, err := BuildPinjamanItem(in); !errors.Is(err, ErrPinjamanInputInvalid) {
				t.Fatalf("err = %v, ingin ErrPinjamanInputInvalid", err)
			}
		})
	}
}

// Aturan tanpa agunan (PDF #page 253) ditegakkan dua arah: sandi 299 hanya sah bila
// nominal 0, dan nominal 0 hanya sah bila sandinya 299. Kombinasi yang konsisten
// diterima.
func TestBuildPinjamanItemTanpaAgunan(t *testing.T) {
	// 299 + 0 sah.
	kosong := buildPinjamanValid()
	kosong.CollateralTypeCode = PinjamanTanpaAgunanCode
	kosong.CollateralAmount = decimal.Zero
	if _, err := BuildPinjamanItem(kosong); err != nil {
		t.Fatalf("sandi 299 nominal 0 harus sah: %v", err)
	}

	// 299 + nominal > 0 ditolak.
	salah1 := buildPinjamanValid()
	salah1.CollateralTypeCode = PinjamanTanpaAgunanCode
	salah1.CollateralAmount = decimal.NewFromInt(1)
	if _, err := BuildPinjamanItem(salah1); !errors.Is(err, ErrPinjamanInputInvalid) {
		t.Fatalf("sandi 299 nominal > 0: err=%v, ingin ErrPinjamanInputInvalid", err)
	}

	// sandi selain 299 + nominal 0 ditolak.
	salah2 := buildPinjamanValid()
	salah2.CollateralTypeCode = "02"
	salah2.CollateralAmount = decimal.Zero
	if _, err := BuildPinjamanItem(salah2); !errors.Is(err, ErrPinjamanInputInvalid) {
		t.Fatalf("sandi 02 nominal 0: err=%v, ingin ErrPinjamanInputInvalid", err)
	}
}

// BakiDebetNeto = baki debet - biaya transaksi - diskonto; hasil negatif tidak sah
// untuk dilaporkan dan tidak boleh dikarang.
func TestPinjamanBakiDebetNeto(t *testing.T) {
	item := PinjamanItem{
		BakiDebet:                  decimal.NewFromInt(1_000),
		UnamortizedTransactionCost: decimal.NewFromInt(100),
		UnamortizedDiscount:        decimal.NewFromInt(50),
	}
	neto, ok := item.BakiDebetNeto()
	if !ok {
		t.Fatal("neto harus dapat dihitung")
	}
	if !neto.Equal(decimal.NewFromInt(850)) {
		t.Fatalf("neto = %s, ingin 850", neto)
	}

	negatif := PinjamanItem{
		BakiDebet:                  decimal.NewFromInt(100),
		UnamortizedTransactionCost: decimal.NewFromInt(150),
	}
	if _, ok := negatif.BakiDebetNeto(); ok {
		t.Fatal("neto negatif tidak boleh dianggap dapat dihitung")
	}
}
