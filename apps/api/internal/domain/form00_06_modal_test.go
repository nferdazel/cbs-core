package domain

import (
	"errors"
	"testing"
)

// TestBuildModalItemValid memastikan masukan sah lolos; id kosong membuat id baru.
func TestBuildModalItemValid(t *testing.T) {
	item, err := BuildModalItem(UpdateModalItemInput{
		JenisCode:          ModalJenisDana,
		TanggalPersetujuan: " 2026-03-15 ",
		JenisModalCode:     ModalJenisModalDisetor,
		Jumlah:             "1000000.50",
	})
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	if item.TanggalPersetujuan == nil || item.TanggalPersetujuan.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("tanggal tidak terurai: %+v", item.TanggalPersetujuan)
	}
	if item.Jumlah.String() != "1000000.5" {
		t.Fatalf("jumlah = %s, ingin 1000000.5", item.Jumlah)
	}
}

// TestBuildModalItemTanggalKosongSah memastikan kolom II boleh kosong (belum disetujui).
func TestBuildModalItemTanggalKosongSah(t *testing.T) {
	item, err := BuildModalItem(UpdateModalItemInput{
		JenisCode:      ModalJenisTanahBangunanInti,
		JenisModalCode: ModalJenisModalSumbangan,
		Jumlah:         "0",
	})
	if err != nil {
		t.Fatalf("tanggal kosong seharusnya sah: %v", err)
	}
	if item.TanggalPersetujuan != nil {
		t.Fatal("tanggal kosong harus nil, bukan tanggal nol")
	}
	if !item.Jumlah.IsZero() {
		t.Fatalf("jumlah nol sah, dapat %s", item.Jumlah)
	}
}

// TestBuildModalItemTolakInputTakSah memastikan sandi, tanggal, dan jumlah tak sah ditolak.
func TestBuildModalItemTolakInputTakSah(t *testing.T) {
	cases := []UpdateModalItemInput{
		{JenisCode: "99", JenisModalCode: ModalJenisModalDisetor, Jumlah: "1"},
		{JenisCode: ModalJenisDana, JenisModalCode: "99", Jumlah: "1"},
		{JenisCode: ModalJenisDana, JenisModalCode: ModalJenisModalDisetor, Jumlah: ""},
		{JenisCode: ModalJenisDana, JenisModalCode: ModalJenisModalDisetor, Jumlah: "abc"},
		{JenisCode: ModalJenisDana, JenisModalCode: ModalJenisModalDisetor, Jumlah: "-1"},
		{JenisCode: ModalJenisDana, JenisModalCode: ModalJenisModalDisetor, Jumlah: "1", TanggalPersetujuan: "15-03-2026"},
	}
	for i, c := range cases {
		if _, err := BuildModalItem(c); !errors.Is(err, ErrModalInputInvalid) {
			t.Fatalf("kasus %d: ingin ErrModalInputInvalid, dapat %v", i, err)
		}
	}
}
