package ojkreport

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// tanggalUji membangun *time.Time untuk uji tanggal; nil tetap nil.
func tanggalUji(t time.Time) *time.Time { return &t }

// Tanpa data register, kolom VI (Jangka Waktu) dan VIII (Suku Bunga) dinyatakan belum
// tersedia dengan alasan yang menyebut lps_placements.
func TestBuildForm05RegisterVIVIIIBelumTersediaTanpaData(t *testing.T) {
	rows := []PlacementRow{
		{CounterpartyBank: "Bank Uji", PlacementType: "DEPOSITO", Collectibility: "LANCAR", Outstanding: decimal.NewFromInt(1_000_000)},
	}
	sec := buildForm05(rows)

	for _, sandi := range []string{form05SandiJangkaWaktu, form05SandiSukuBunga} {
		u := unavailableColumn(t, sec, sandi)
		if !strings.Contains(u.Reason, "lps_placements") {
			t.Errorf("alasan kolom %s = %q, harus menyebut lps_placements", sandi, u.Reason)
		}
	}
}

// Saat minimal satu baris punya data, kolom VI/VIII tersedia: tanggal diformat
// TT-BB-TTTT, suku bunga 2 digit desimal, dan baris tanpa data ditulis "-".
func TestBuildForm05RegisterVIVIIITersedia(t *testing.T) {
	mulai := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	tempo := time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	rows := []PlacementRow{
		{
			CounterpartyBank:   "Bank Uji Lengkap",
			PlacementType:      "DEPOSITO",
			Collectibility:     "LANCAR",
			Outstanding:        decimal.NewFromInt(1_000_000),
			StartDate:          tanggalUji(mulai),
			MaturityDate:       tanggalUji(tempo),
			InterestRateAnnual: decimal.NewFromFloat(5.5),
		},
		{
			CounterpartyBank: "Bank Uji Sebagian",
			PlacementType:    "GIRO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(500_000),
		},
	}
	sec := buildForm05(rows)

	for _, sandi := range []string{form05SandiJangkaWaktu, form05SandiSukuBunga} {
		for _, u := range sec.Unavailable {
			if u.Sandi == sandi {
				t.Errorf("kolom %s seharusnya tersedia saat ada data", sandi)
			}
		}
	}

	if got := findCell(t, sec, "Bank Uji Lengkap", form05SandiJangkaWaktu).Value; got != "02-01-2026 s.d. 02-07-2026" {
		t.Errorf("jangka waktu = %q, ingin 02-01-2026 s.d. 02-07-2026", got)
	}
	if got := findCell(t, sec, "Bank Uji Lengkap", form05SandiSukuBunga).Value; got != "5.50" {
		t.Errorf("suku bunga = %q, ingin 5.50", got)
	}
	if got := findCell(t, sec, "Bank Uji Sebagian", form05SandiJangkaWaktu).Value; got != "-" {
		t.Errorf("jangka waktu baris sebagian = %q, ingin -", got)
	}
	if got := findCell(t, sec, "Bank Uji Sebagian", form05SandiSukuBunga).Value; got != "-" {
		t.Errorf("suku bunga baris sebagian = %q, ingin -", got)
	}
}

// Tanpa tanggal jatuh tempo, kolom VI hanya memuat tanggal mulai (tidak diisi "-").
func TestBuildForm05RegisterTanpaJatuhTempoHanyaMulai(t *testing.T) {
	mulai := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	rows := []PlacementRow{
		{
			CounterpartyBank: "Bank Uji Giro",
			PlacementType:    "GIRO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(1_000_000),
			StartDate:        tanggalUji(mulai),
		},
	}
	sec := buildForm05(rows)
	if got := findCell(t, sec, "Bank Uji Giro", form05SandiJangkaWaktu).Value; got != "31-03-2026" {
		t.Fatalf("jangka waktu = %q, ingin 31-03-2026", got)
	}
}

// Kolom III (Lokasi Bank) kini terisi sandi Kabupaten/Kota Lampiran 03 bila bank
// mengisinya, dan "-" bila kosong. Kolom ini tidak lagi terdaftar sebagai belum
// tersedia; sandi tidak pernah ditebak dari nama bank lawan.
func TestBuildForm05RegisterLokasiBank(t *testing.T) {
	rows := []PlacementRow{
		{
			CounterpartyBank: "Bank Uji Berisi",
			PlacementType:    "DEPOSITO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(1_000_000),
			OJKKabupatenCode: "0197",
		},
		{
			CounterpartyBank: "Bank Uji Kosong",
			PlacementType:    "GIRO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(500_000),
		},
	}
	sec := buildForm05(rows)

	if got := findCell(t, sec, "Bank Uji Berisi", form05SandiLokasi).Value; got != "0197" {
		t.Errorf("lokasi berisi = %q, ingin 0197", got)
	}
	if got := findCell(t, sec, "Bank Uji Kosong", form05SandiLokasi).Value; got != "-" {
		t.Errorf("lokasi kosong = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form05SandiLokasi {
			t.Errorf("kolom III masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}
