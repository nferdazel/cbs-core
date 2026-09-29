package domain

import (
	"errors"
	"testing"

	"github.com/shopspring/decimal"
)

func asetTetapInputSah() UpdateAsetTetapItemInput {
	return UpdateAsetTetapItemInput{
		JenisAsetCode:                   AsetJenisTanah,
		SumberPerolehanCode:             AsetSumberSewaPembiayaan,
		StatusAsetCode:                  AsetStatusDijaminkan,
		BiayaPerolehan:                  decimal.NewFromInt(1_000_000_000),
		AkumulasiPenyusutanAmortisasi:   decimal.NewFromInt(200_000_000),
		AkumulasiKerugianPenurunanNilai: decimal.NewFromInt(50_000_000),
		MetodePengukuranCode:            AsetMetodeBiaya,
		AsOf:                            "2026-03-31",
	}
}

// BuildAsetTetapItem menerima baris sah (id dibangkitkan, status register default
// AKTIF) dan menolak sandi/angka tak sah dengan ErrAsetTetapInputInvalid.
func TestBuildAsetTetapItemValidasi(t *testing.T) {
	item, err := BuildAsetTetapItem(asetTetapInputSah())
	if err != nil {
		t.Fatalf("masukan sah: %v", err)
	}
	if item.ID == [16]byte{} {
		t.Fatal("id kosong harus dibangkitkan server")
	}
	if item.Status != AsetStatusRegisterAktif {
		t.Fatalf("status register default = %q, ingin AKTIF", item.Status)
	}

	ubah := func(f func(*UpdateAsetTetapItemInput)) UpdateAsetTetapItemInput {
		in := asetTetapInputSah()
		f(&in)
		return in
	}
	kasus := map[string]UpdateAsetTetapItemInput{
		"jenis tak dikenal":       ubah(func(in *UpdateAsetTetapItemInput) { in.JenisAsetCode = "999" }),
		"sumber tak dikenal":      ubah(func(in *UpdateAsetTetapItemInput) { in.SumberPerolehanCode = "00" }),
		"status aset tak dikenal": ubah(func(in *UpdateAsetTetapItemInput) { in.StatusAsetCode = "9" }),
		"metode tak dikenal":      ubah(func(in *UpdateAsetTetapItemInput) { in.MetodePengukuranCode = "3" }),
		"biaya negatif":           ubah(func(in *UpdateAsetTetapItemInput) { in.BiayaPerolehan = decimal.NewFromInt(-1) }),
		"penyusutan negatif":      ubah(func(in *UpdateAsetTetapItemInput) { in.AkumulasiPenyusutanAmortisasi = decimal.NewFromInt(-1) }),
		"penurunan nilai negatif": ubah(func(in *UpdateAsetTetapItemInput) { in.AkumulasiKerugianPenurunanNilai = decimal.NewFromInt(-1) }),
		"as_of kosong":            ubah(func(in *UpdateAsetTetapItemInput) { in.AsOf = "" }),
		"as_of salah format":      ubah(func(in *UpdateAsetTetapItemInput) { in.AsOf = "31-03-2026" }),
	}
	for nama, in := range kasus {
		if _, err := BuildAsetTetapItem(in); !errors.Is(err, ErrAsetTetapInputInvalid) {
			t.Errorf("%s: err=%v, ingin ErrAsetTetapInputInvalid", nama, err)
		}
	}
}

// Status aset boleh kosong (aset tidak berwujud dan aset berwujud yang belum diisi);
// jenis aset tidak berwujud diterima tanpa memaksa status 1/2.
func TestBuildAsetTetapItemStatusKosong(t *testing.T) {
	in := asetTetapInputSah()
	in.JenisAsetCode = AsetJenisSoftware
	in.StatusAsetCode = ""
	item, err := BuildAsetTetapItem(in)
	if err != nil {
		t.Fatalf("aset tidak berwujud tanpa status: %v", err)
	}
	if item.StatusAsetCode != "" {
		t.Fatalf("status aset = %q, ingin kosong", item.StatusAsetCode)
	}
	if !AsetJenisTidakBerwujud(item.JenisAsetCode) {
		t.Fatalf("201 harus dikenali sebagai aset tidak berwujud")
	}
}

// NilaiTercatat menghitung V - VI - VII; hasil negatif ditandai ok=false agar laporan
// menulis "-", bukan angka negatif.
func TestAsetTetapItemNilaiTercatat(t *testing.T) {
	item := AsetTetapItem{
		BiayaPerolehan:                  decimal.NewFromInt(1_000),
		AkumulasiPenyusutanAmortisasi:   decimal.NewFromInt(200),
		AkumulasiKerugianPenurunanNilai: decimal.NewFromInt(50),
	}
	nilai, ok := item.NilaiTercatat()
	if !ok || !nilai.Equal(decimal.NewFromInt(750)) {
		t.Fatalf("nilai tercatat = %s ok=%v, ingin 750/true", nilai, ok)
	}

	item.AkumulasiPenyusutanAmortisasi = decimal.NewFromInt(900)
	item.AkumulasiKerugianPenurunanNilai = decimal.NewFromInt(200)
	if _, ok := item.NilaiTercatat(); ok {
		t.Fatal("nilai tercatat negatif harus ok=false")
	}
}
