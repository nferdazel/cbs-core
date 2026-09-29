package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// buildPropertiValid adalah masukan sah yang dipakai banyak kasus uji.
func buildPropertiValid() UpdatePropertiItemInput {
	return UpdatePropertiItemInput{
		NoRegister:                    "  PRP-001 ",
		JenisPropertiCode:             "1",
		AlamatProperti:                "Jl. Contoh No. 1",
		Koordinat:                     "-6.200000, 106.816666",
		TanggalPenetapan:              "2025-06-01",
		BiayaPerolehanNilaiWajar:      decimal.NewFromInt(1_000_000_000),
		AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(200_000_000),
		MetodePengukuranCode:          "1",
		AsOf:                          "2026-03-31",
	}
}

// BuildPropertiItem menerima masukan sah, menormalkan teks, mengurai dua tanggal,
// memberi id, dan mengisi status bawaan AKTIF.
func TestBuildPropertiItemSah(t *testing.T) {
	item, err := BuildPropertiItem(buildPropertiValid())
	if err != nil {
		t.Fatalf("masukan sah: %v", err)
	}
	if item.ID == [16]byte{} {
		t.Fatal("id kosong tidak dibangkitkan")
	}
	if item.NoRegister != "PRP-001" {
		t.Fatalf("no_register tidak dinormalkan: %q", item.NoRegister)
	}
	if item.Status != PropertiStatusAktif {
		t.Fatalf("status bawaan = %q, ingin %q", item.Status, PropertiStatusAktif)
	}
	if !item.TanggalPenetapan.Equal(time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("tanggal_penetapan = %s", item.TanggalPenetapan)
	}
	if !item.AsOf.Equal(time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("as_of = %s", item.AsOf)
	}
}

// Masukan tidak sah ditolak dengan ErrPropertiInputInvalid: nomor/alamat kosong, sandi
// asing, angka negatif, tanggal/status salah, dan id bukan UUID.
func TestBuildPropertiItemTidakSah(t *testing.T) {
	cases := []struct {
		nama string
		ubah func(*UpdatePropertiItemInput)
	}{
		{"no register kosong", func(in *UpdatePropertiItemInput) { in.NoRegister = "  " }},
		{"no register terlalu panjang", func(in *UpdatePropertiItemInput) {
			in.NoRegister = string(make([]byte, 65))
		}},
		{"jenis properti asing", func(in *UpdatePropertiItemInput) { in.JenisPropertiCode = "4" }},
		{"jenis properti kosong", func(in *UpdatePropertiItemInput) { in.JenisPropertiCode = "" }},
		{"alamat kosong", func(in *UpdatePropertiItemInput) { in.AlamatProperti = " " }},
		{"alamat terlalu panjang", func(in *UpdatePropertiItemInput) {
			in.AlamatProperti = string(make([]byte, 256))
		}},
		{"biaya negatif", func(in *UpdatePropertiItemInput) {
			in.BiayaPerolehanNilaiWajar = decimal.NewFromInt(-1)
		}},
		{"akumulasi negatif", func(in *UpdatePropertiItemInput) {
			in.AkumulasiPenyusutanAmortisasi = decimal.NewFromInt(-1)
		}},
		{"metode pengukuran asing", func(in *UpdatePropertiItemInput) { in.MetodePengukuranCode = "3" }},
		{"status asing", func(in *UpdatePropertiItemInput) { in.Status = "HAPUS" }},
		{"tanggal penetapan kosong", func(in *UpdatePropertiItemInput) { in.TanggalPenetapan = "" }},
		{"tanggal penetapan salah format", func(in *UpdatePropertiItemInput) { in.TanggalPenetapan = "01-06-2025" }},
		{"as_of kosong", func(in *UpdatePropertiItemInput) { in.AsOf = "" }},
		{"id bukan uuid", func(in *UpdatePropertiItemInput) { in.ID = "bukan-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			in := buildPropertiValid()
			tc.ubah(&in)
			if _, err := BuildPropertiItem(in); !errors.Is(err, ErrPropertiInputInvalid) {
				t.Fatalf("err = %v, ingin ErrPropertiInputInvalid", err)
			}
		})
	}
}

// Jumlah = biaya perolehan atau nilai wajar - akumulasi penyusutan/amortisasi; hasil
// negatif tidak sah untuk dilaporkan dan tidak boleh dikarang (PDF #page 232).
func TestPropertiJumlah(t *testing.T) {
	item := PropertiItem{
		BiayaPerolehanNilaiWajar:      decimal.NewFromInt(1_000),
		AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(250),
	}
	jumlah, ok := item.Jumlah()
	if !ok {
		t.Fatal("jumlah harus dapat dihitung")
	}
	if !jumlah.Equal(decimal.NewFromInt(750)) {
		t.Fatalf("jumlah = %s, ingin 750", jumlah)
	}

	sama := PropertiItem{
		BiayaPerolehanNilaiWajar:      decimal.NewFromInt(100),
		AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(100),
	}
	if j, ok := sama.Jumlah(); !ok || !j.IsZero() {
		t.Fatalf("jumlah nol harus sah: j=%s ok=%v", j, ok)
	}

	negatif := PropertiItem{
		BiayaPerolehanNilaiWajar:      decimal.NewFromInt(100),
		AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(150),
	}
	if _, ok := negatif.Jumlah(); ok {
		t.Fatal("jumlah negatif tidak boleh dianggap dapat dihitung")
	}
}
