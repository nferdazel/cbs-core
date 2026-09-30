package domain

import (
	"errors"
	"testing"
)

// TestBuildPihakLawanItemValid memastikan masukan sah lolos, sandi kondisional boleh kosong.
func TestBuildPihakLawanItemValid(t *testing.T) {
	item, err := BuildPihakLawanItem(UpdatePihakLawanItemInput{
		PihakLawanID:         "CP-1",
		Nama:                 "  PT Contoh  ",
		JenisIdentitasCode:   PihakLawanIdentitasKTP,
		JenisKelaminCode:     PihakLawanKelaminLakiLaki,
		HubunganBankCode:     PihakLawanHubunganTidakTerkait,
		TanggalLahir:         "1990-01-02",
		TanggalPemeringkatan: "",
	})
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	if item.Nama != "PT Contoh" {
		t.Fatalf("nama tidak dipangkas: %q", item.Nama)
	}
	if item.TanggalLahir == nil || item.TanggalLahir.Format("2006-01-02") != "1990-01-02" {
		t.Fatalf("tanggal lahir salah: %+v", item.TanggalLahir)
	}
	if item.TanggalPemeringkatan != nil {
		t.Fatal("tanggal pemeringkatan kosong harus nil")
	}
}

// TestBuildPihakLawanItemTolakInputTakSah memastikan sandi/nama/tanggal tak sah ditolak.
func TestBuildPihakLawanItemTolakInputTakSah(t *testing.T) {
	ok := func() UpdatePihakLawanItemInput {
		return UpdatePihakLawanItemInput{PihakLawanID: "CP-1", Nama: "PT Contoh"}
	}
	cases := map[string]func(UpdatePihakLawanItemInput) UpdatePihakLawanItemInput{
		"id kosong":       func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput { in.PihakLawanID = ""; return in },
		"nama kosong":     func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput { in.Nama = ""; return in },
		"jenis identitas": func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput { in.JenisIdentitasCode = "9"; return in },
		"jenis kelamin":   func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput { in.JenisKelaminCode = "9"; return in },
		"jenis usaha":     func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput { in.JenisUsahaCode = "9"; return in },
		"hubungan":        func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput { in.HubunganBankCode = "11"; return in },
		"tanggal lahir": func(in UpdatePihakLawanItemInput) UpdatePihakLawanItemInput {
			in.TanggalLahir = "02-01-1990"
			return in
		},
	}
	for name, mutate := range cases {
		if _, err := BuildPihakLawanItem(mutate(ok())); !errors.Is(err, ErrPihakLawanInputInvalid) {
			t.Fatalf("kasus %q: ingin ErrPihakLawanInputInvalid, dapat %v", name, err)
		}
	}
}
