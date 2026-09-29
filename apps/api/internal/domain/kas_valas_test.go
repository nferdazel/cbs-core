package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func kasValasInputSah() UpdateKasValasItemInput {
	return UpdateKasValasItemInput{
		JenisValasCode: "USD",
		Nominal:        decimal.NewFromInt(1_000),
		KursTengah:     decimal.NewFromFloat(16_250.50),
		AsOf:           "2026-03-31",
	}
}

// Masukan sah membentuk baris tanpa mengubah nilai: status default AKTIF, dan Nominal/
// Kurs Tengah diteruskan apa adanya.
func TestBuildKasValasItemSah(t *testing.T) {
	out, err := BuildKasValasItem(kasValasInputSah())
	if err != nil {
		t.Fatalf("build sah: %v", err)
	}
	if out.Status != KasValasStatusAktif {
		t.Fatalf("status default = %q, ingin AKTIF", out.Status)
	}
	if out.JenisValasCode != "USD" {
		t.Fatalf("jenis valas = %q", out.JenisValasCode)
	}
	if out.AsOf.IsZero() {
		t.Fatal("as_of wajib terurai")
	}
}

// Setiap penyimpangan dari aturan form ditolak sebelum menyentuh basis data.
func TestBuildKasValasItemInvalid(t *testing.T) {
	cases := map[string]func(*UpdateKasValasItemInput){
		"jenis kosong":          func(in *UpdateKasValasItemInput) { in.JenisValasCode = "  " },
		"jenis terlalu panjang": func(in *UpdateKasValasItemInput) { in.JenisValasCode = "12345678901234567" },
		"nominal negatif":       func(in *UpdateKasValasItemInput) { in.Nominal = decimal.NewFromInt(-1) },
		"kurs negatif":          func(in *UpdateKasValasItemInput) { in.KursTengah = decimal.NewFromInt(-1) },
		"as_of salah":           func(in *UpdateKasValasItemInput) { in.AsOf = "31-03-2026" },
		"status asing":          func(in *UpdateKasValasItemInput) { in.Status = "DRAFT" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := kasValasInputSah()
			mutate(&in)
			if _, err := BuildKasValasItem(in); !errors.Is(err, ErrKasValasInputInvalid) {
				t.Fatalf("err = %v, ingin ErrKasValasInputInvalid", err)
			}
		})
	}
}

// Kolom V Nilai Rupiah adalah turunan III x IV, dibulatkan ke rupiah penuh.
func TestKasValasNilaiRupiahTurunan(t *testing.T) {
	item := KasValasItem{
		Nominal:    decimal.NewFromInt(1_000),
		KursTengah: decimal.NewFromFloat(16_250.50),
	}
	nilai, ok := item.NilaiRupiah()
	if !ok {
		t.Fatal("NilaiRupiah harus dapat dihitung saat Nominal dan Kurs terisi")
	}
	if got := nilai.String(); got != "16250500" {
		t.Fatalf("Nilai Rupiah = %s, ingin 16250500", got)
	}
}

// Bila Nominal atau Kurs Tengah belum diisi, Nilai Rupiah TIDAK dihitung: laporan harus
// menulis "-" beralasan, bukan nol.
func TestKasValasNilaiRupiahKosongTidakDihitung(t *testing.T) {
	for name, item := range map[string]KasValasItem{
		"nominal nol": KasValasItem{Nominal: decimal.Zero, KursTengah: decimal.NewFromInt(16_000)},
		"kurs nol":    KasValasItem{Nominal: decimal.NewFromInt(100), KursTengah: decimal.Zero},
	} {
		if _, ok := item.NilaiRupiah(); ok {
			t.Fatalf("%s: NilaiRupiah seharusnya tidak dapat dihitung", name)
		}
	}
}
