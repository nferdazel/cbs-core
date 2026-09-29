package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func penyertaanInputSah() UpdatePenyertaanItemInput {
	return UpdatePenyertaanItemInput{
		NoRegister:           "PM-001",
		CounterpartyID:       "PL-001",
		MetodePenyertaanCode: PenyertaanMetodeBiayaPerolehan,
		KualitasCode:         PenyertaanKualitasLancar,
		TujuanPenyertaanCode: PenyertaanTujuanLembagaPenunjang,
		TanggalMulai:         "2025-06-01",
		PersentasePenyertaan: decimal.NewFromInt(25),
		Nominal:              decimal.NewFromInt(1_000_000_000),
		JumlahBulanLaporan:   decimal.NewFromInt(900_000_000),
		CKPN:                 decimal.NewFromInt(10_000_000),
		CKPNAsetBaik:         decimal.NewFromInt(5_000_000),
		CKPNAsetKurangBaik:   decimal.NewFromInt(3_000_000),
		CKPNAsetTidakBaik:    decimal.NewFromInt(2_000_000),
		JenisCKPNCode:        PenyertaanJenisCKPNKolektif,
		AsOf:                 "2026-03-31",
	}
}

// Masukan sah membentuk baris tanpa mengubah nilai: sandi apa adanya, status default
// AKTIF, dan kolom X (Jumlah Bulan Laporan) diteruskan sebagai nilai bank (bukan turunan).
func TestBuildPenyertaanItemSah(t *testing.T) {
	out, err := BuildPenyertaanItem(penyertaanInputSah())
	if err != nil {
		t.Fatalf("build sah: %v", err)
	}
	if out.Status != PenyertaanStatusAktif {
		t.Fatalf("status default = %q, ingin AKTIF", out.Status)
	}
	if out.NoRegister != "PM-001" || out.CounterpartyID != "PL-001" {
		t.Fatalf("identitas baris = %+v", out)
	}
	if !out.JumlahBulanLaporan.Equal(decimal.NewFromInt(900_000_000)) {
		t.Fatalf("kolom X diteruskan apa adanya = %s", out.JumlahBulanLaporan)
	}
	if out.TanggalMulai.IsZero() || out.AsOf.IsZero() {
		t.Fatal("tanggal wajib terurai")
	}
}

// Setiap penyimpangan dari sandi/aturan form ditolak sebelum menyentuh basis data.
func TestBuildPenyertaanItemInvalid(t *testing.T) {
	cases := map[string]func(*UpdatePenyertaanItemInput){
		"no register kosong":     func(in *UpdatePenyertaanItemInput) { in.NoRegister = "  " },
		"id pihak lawan kosong":  func(in *UpdatePenyertaanItemInput) { in.CounterpartyID = "" },
		"metode asing":           func(in *UpdatePenyertaanItemInput) { in.MetodePenyertaanCode = "9" },
		"kualitas 2 tidak ada":   func(in *UpdatePenyertaanItemInput) { in.KualitasCode = "2" },
		"tujuan asing":           func(in *UpdatePenyertaanItemInput) { in.TujuanPenyertaanCode = "2" },
		"jenis ckpn asing":       func(in *UpdatePenyertaanItemInput) { in.JenisCKPNCode = "3" },
		"persentase lebih 100":   func(in *UpdatePenyertaanItemInput) { in.PersentasePenyertaan = decimal.NewFromInt(101) },
		"persentase negatif":     func(in *UpdatePenyertaanItemInput) { in.PersentasePenyertaan = decimal.NewFromInt(-1) },
		"nominal negatif":        func(in *UpdatePenyertaanItemInput) { in.Nominal = decimal.NewFromInt(-1) },
		"ckpn negatif":           func(in *UpdatePenyertaanItemInput) { in.CKPN = decimal.NewFromInt(-1) },
		"ckpn aset baik negatif": func(in *UpdatePenyertaanItemInput) { in.CKPNAsetBaik = decimal.NewFromInt(-1) },
		"tanggal mulai salah":    func(in *UpdatePenyertaanItemInput) { in.TanggalMulai = "01-06-2025" },
		"status asing":           func(in *UpdatePenyertaanItemInput) { in.Status = "DRAFT" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := penyertaanInputSah()
			mutate(&in)
			if _, err := BuildPenyertaanItem(in); !errors.Is(err, ErrPenyertaanInputInvalid) {
				t.Fatalf("err = %v, ingin ErrPenyertaanInputInvalid", err)
			}
		})
	}
}
