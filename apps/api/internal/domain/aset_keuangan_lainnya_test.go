package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func asetKeuanganInputSah() UpdateAsetKeuanganItemInput {
	return UpdateAsetKeuanganItemInput{
		NoRekening:                  "AKL-001",
		CounterpartyID:              "PL-001",
		JenisCode:                   AsetKeuanganJenisTagihanFraud,
		TanggalMulai:                "2025-06-01",
		TanggalJatuhTempo:           "2026-06-01",
		SukuBunga:                   decimal.NewFromInt(7),
		Nominal:                     decimal.NewFromInt(1_000_000_000),
		NilaiAgunanDiperhitungkan:   decimal.NewFromInt(500_000_000),
		CKPN:                        decimal.NewFromInt(10_000_000),
		CKPNAsetBaik:                decimal.NewFromInt(5_000_000),
		CKPNAsetKurangBaik:          decimal.NewFromInt(3_000_000),
		CKPNAsetTidakBaik:           decimal.NewFromInt(2_000_000),
		KlasifikasiAsetKeuanganCode: AsetKeuanganKlasifikasiBiayaPerolehanDiamortasi,
		JenisCKPNCode:               AsetKeuanganJenisCKPNKolektif,
		AsOf:                        "2026-03-31",
	}
}

// Masukan sah membentuk baris tanpa mengubah nilai: sandi apa adanya, status default
// AKTIF, dan seluruh nilai diteruskan sebagai isian bank (tidak ada turunan).
func TestBuildAsetKeuanganItemSah(t *testing.T) {
	out, err := BuildAsetKeuanganItem(asetKeuanganInputSah())
	if err != nil {
		t.Fatalf("build sah: %v", err)
	}
	if out.Status != AsetKeuanganStatusAktif {
		t.Fatalf("status default = %q, ingin AKTIF", out.Status)
	}
	if out.NoRekening != "AKL-001" || out.CounterpartyID != "PL-001" {
		t.Fatalf("identitas baris = %+v", out)
	}
	if !out.NilaiAgunanDiperhitungkan.Equal(decimal.NewFromInt(500_000_000)) {
		t.Fatalf("kolom IX diteruskan apa adanya = %s", out.NilaiAgunanDiperhitungkan)
	}
	if out.TanggalMulai.IsZero() || out.TanggalJatuhTempo.IsZero() || out.AsOf.IsZero() {
		t.Fatal("tanggal wajib terurai")
	}
}

// Setiap penyimpangan dari sandi/aturan form ditolak sebelum menyentuh basis data.
func TestBuildAsetKeuanganItemInvalid(t *testing.T) {
	cases := map[string]func(*UpdateAsetKeuanganItemInput){
		"no rekening kosong":        func(in *UpdateAsetKeuanganItemInput) { in.NoRekening = "  " },
		"id pihak lawan kosong":     func(in *UpdateAsetKeuanganItemInput) { in.CounterpartyID = "" },
		"jenis asing":               func(in *UpdateAsetKeuanganItemInput) { in.JenisCode = "11" },
		"klasifikasi asing":         func(in *UpdateAsetKeuanganItemInput) { in.KlasifikasiAsetKeuanganCode = "4" },
		"jenis ckpn asing":          func(in *UpdateAsetKeuanganItemInput) { in.JenisCKPNCode = "3" },
		"suku bunga negatif":        func(in *UpdateAsetKeuanganItemInput) { in.SukuBunga = decimal.NewFromInt(-1) },
		"nominal negatif":           func(in *UpdateAsetKeuanganItemInput) { in.Nominal = decimal.NewFromInt(-1) },
		"nilai agunan negatif":      func(in *UpdateAsetKeuanganItemInput) { in.NilaiAgunanDiperhitungkan = decimal.NewFromInt(-1) },
		"ckpn negatif":              func(in *UpdateAsetKeuanganItemInput) { in.CKPN = decimal.NewFromInt(-1) },
		"ckpn aset baik negatif":    func(in *UpdateAsetKeuanganItemInput) { in.CKPNAsetBaik = decimal.NewFromInt(-1) },
		"tanggal mulai salah":       func(in *UpdateAsetKeuanganItemInput) { in.TanggalMulai = "01-06-2025" },
		"tanggal jatuh tempo salah": func(in *UpdateAsetKeuanganItemInput) { in.TanggalJatuhTempo = "2026/06/01" },
		"status asing":              func(in *UpdateAsetKeuanganItemInput) { in.Status = "DRAFT" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := asetKeuanganInputSah()
			mutate(&in)
			if _, err := BuildAsetKeuanganItem(in); !errors.Is(err, ErrAsetKeuanganInputInvalid) {
				t.Fatalf("err = %v, ingin ErrAsetKeuanganInputInvalid", err)
			}
		})
	}
}
