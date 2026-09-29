package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func suratBerhargaInputSah() UpdateSuratBerhargaItemInput {
	return UpdateSuratBerhargaItemInput{
		KlasifikasiCode:                  SuratBerhargaKlasifikasiTersediaUntukDijual,
		SukuBunga:                        decimal.NewFromInt(7),
		TanggalMulai:                     "2025-06-01",
		TanggalJatuhTempo:                "2026-06-01",
		Nominal:                          decimal.NewFromInt(1_000_000_000),
		NominalDijaminkan:                decimal.NewFromInt(500_000_000),
		BiayaPerolehan:                   decimal.NewFromInt(990_000_000),
		DiskontoPremiumBelumDiamortisasi: decimal.NewFromInt(-10_000_000),
		BiayaTransaksiBelumDiamortisasi:  decimal.NewFromInt(1_000_000),
		LabaRugiBelumDirealisasi:         decimal.NewFromInt(5_000_000),
		BiayaPerolehanDiamortisasi:       decimal.NewFromInt(981_000_000),
		NomorSuratBerharga:               "ID0000000001",
		CounterpartyID:                   "PL-001",
		JenisCode:                        SuratBerhargaJenisPemerintah,
		KualitasCode:                     SuratBerhargaKualitasLancar,
		CKPN:                             decimal.NewFromInt(1_000_000),
		LembagaPemeringkatCode:           SuratBerhargaLembagaPemeringkatTanpaPeringkat,
		PeringkatSuratBerhargaCode:       SuratBerhargaPeringkatTanpaPeringkat,
		TanggalPenerbitan:                "2025-05-01",
		CKPNAsetBaik:                     decimal.NewFromInt(1_000_000),
		KlasifikasiAsetKeuanganCode:      SuratBerhargaKlasifikasiAsetBiayaPerolehan,
		JenisCKPNCode:                    SuratBerhargaJenisCKPNKolektif,
		AsOf:                             "2026-03-31",
	}
}

// Masukan sah membentuk baris tanpa mengubah nilai: sandi apa adanya, status default
// AKTIF, tanggal pemeringkatan boleh kosong, dan kolom XI diteruskan apa adanya.
func TestBuildSuratBerhargaItemSah(t *testing.T) {
	out, err := BuildSuratBerhargaItem(suratBerhargaInputSah())
	if err != nil {
		t.Fatalf("build sah: %v", err)
	}
	if out.Status != SuratBerhargaStatusAktif {
		t.Fatalf("status default = %q, ingin AKTIF", out.Status)
	}
	if out.TanggalPemeringkatan != nil {
		t.Fatalf("tanggal pemeringkatan kosong harus tetap nil, dapat %v", out.TanggalPemeringkatan)
	}
	if !out.BiayaPerolehanDiamortisasi.Equal(decimal.NewFromInt(981_000_000)) {
		t.Fatalf("kolom XI diteruskan apa adanya = %s", out.BiayaPerolehanDiamortisasi)
	}
	if !out.DiskontoPremiumBelumDiamortisasi.Equal(decimal.NewFromInt(-10_000_000)) {
		t.Fatalf("diskonto/premium boleh negatif = %s", out.DiskontoPremiumBelumDiamortisasi)
	}
}

// Kolom XXIV (klasifikasi aset keuangan) boleh kosong: hanya diisi bila penawaran umum.
func TestBuildSuratBerhargaItemKlasifikasiAsetBolehKosong(t *testing.T) {
	in := suratBerhargaInputSah()
	in.KlasifikasiAsetKeuanganCode = ""
	if _, err := BuildSuratBerhargaItem(in); err != nil {
		t.Fatalf("klasifikasi aset kosong harus diterima: %v", err)
	}
}

// Setiap penyimpangan dari sandi/aturan form ditolak sebelum menyentuh basis data.
func TestBuildSuratBerhargaItemInvalid(t *testing.T) {
	cases := map[string]func(*UpdateSuratBerhargaItemInput){
		"klasifikasi asing":       func(in *UpdateSuratBerhargaItemInput) { in.KlasifikasiCode = "3" },
		"jenis asing":             func(in *UpdateSuratBerhargaItemInput) { in.JenisCode = "4" },
		"kualitas 2 tidak ada":    func(in *UpdateSuratBerhargaItemInput) { in.KualitasCode = "2" },
		"jenis ckpn asing":        func(in *UpdateSuratBerhargaItemInput) { in.JenisCKPNCode = "3" },
		"klasifikasi aset asing":  func(in *UpdateSuratBerhargaItemInput) { in.KlasifikasiAsetKeuanganCode = "9" },
		"suku bunga negatif":      func(in *UpdateSuratBerhargaItemInput) { in.SukuBunga = decimal.NewFromInt(-1) },
		"nominal negatif":         func(in *UpdateSuratBerhargaItemInput) { in.Nominal = decimal.NewFromInt(-1) },
		"nominal dijaminkan nega": func(in *UpdateSuratBerhargaItemInput) { in.NominalDijaminkan = decimal.NewFromInt(-1) },
		"biaya perolehan negatif": func(in *UpdateSuratBerhargaItemInput) { in.BiayaPerolehan = decimal.NewFromInt(-1) },
		"ckpn negatif":            func(in *UpdateSuratBerhargaItemInput) { in.CKPN = decimal.NewFromInt(-1) },
		"tanggal mulai salah":     func(in *UpdateSuratBerhargaItemInput) { in.TanggalMulai = "01-06-2025" },
		"pemeringkatan salah":     func(in *UpdateSuratBerhargaItemInput) { in.TanggalPemeringkatan = "2026/01/01" },
		"status asing":            func(in *UpdateSuratBerhargaItemInput) { in.Status = "DRAFT" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := suratBerhargaInputSah()
			mutate(&in)
			if _, err := BuildSuratBerhargaItem(in); !errors.Is(err, ErrSuratBerhargaInputInvalid) {
				t.Fatalf("err = %v, ingin ErrSuratBerhargaInputInvalid", err)
			}
		})
	}
}

// Diskonto/premium dan laba/rugi yang negatif DITERIMA (bukan pelanggaran): nilai ini
// memang boleh negatif menurut sifatnya.
func TestBuildSuratBerhargaItemNilaiBolehNegatif(t *testing.T) {
	in := suratBerhargaInputSah()
	in.DiskontoPremiumBelumDiamortisasi = decimal.NewFromInt(-5)
	in.LabaRugiBelumDirealisasi = decimal.NewFromInt(-7)
	if _, err := BuildSuratBerhargaItem(in); err != nil {
		t.Fatalf("nilai negatif yang sah harus diterima: %v", err)
	}
}
