package domain

import (
	"errors"
	"testing"
)

// TestBuildOfficeForm00_11MenerimaMasukanSah memastikan sandi baku diterima.
func TestBuildOfficeForm00_11MenerimaMasukanSah(t *testing.T) {
	in := UpdateOfficeForm00_11Input{
		OJKOfficeKindCode:  KantorKindKantorKas,
		ParentOfficeCode:   "001",
		OJKChangeCode:      "1",
		ImplementationDate: "2026-07-01",
	}
	out, err := BuildOfficeForm00_11(in)
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	if out.OJKOfficeKindCode != KantorKindKantorKas {
		t.Fatalf("sandi jenis = %q", out.OJKOfficeKindCode)
	}
}

// TestBuildOfficeForm00_11WajibSandiJenis memastikan sandi Jenis wajib: baris tanpa sandi
// tidak masuk Form 00.11, sehingga tidak boleh diterima lewat jalur ini.
func TestBuildOfficeForm00_11WajibSandiJenis(t *testing.T) {
	if _, err := BuildOfficeForm00_11(UpdateOfficeForm00_11Input{}); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("tanpa sandi Jenis harus ditolak, dapat %v", err)
	}
}

// TestBuildOfficeForm00_11SandiDiLuarBaku menguji sandi Jenis dan Keterangan di luar baku.
func TestBuildOfficeForm00_11SandiDiLuarBaku(t *testing.T) {
	jenis := UpdateOfficeForm00_11Input{OJKOfficeKindCode: "01"}
	if _, err := BuildOfficeForm00_11(jenis); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("sandi jenis 01 harus ditolak, dapat %v", err)
	}
	keterangan := UpdateOfficeForm00_11Input{OJKOfficeKindCode: KantorKindKantorKas, OJKChangeCode: "8"}
	if _, err := BuildOfficeForm00_11(keterangan); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("sandi keterangan 8 harus ditolak, dapat %v", err)
	}
}

// TestBuildOfficeForm00_11TanggalSalahFormat memastikan tanggal harus YYYY-MM-DD.
func TestBuildOfficeForm00_11TanggalSalahFormat(t *testing.T) {
	in := UpdateOfficeForm00_11Input{OJKOfficeKindCode: KantorKindKantorKas, ImplementationDate: "01-07-2026"}
	if _, err := BuildOfficeForm00_11(in); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("tanggal salah format harus ditolak, dapat %v", err)
	}
}
