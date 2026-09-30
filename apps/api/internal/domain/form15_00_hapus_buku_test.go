package domain

import (
	"errors"
	"testing"
)

// TestBuildHapusBukuItemValid memastikan masukan sah lolos dan nominal terurai.
func TestBuildHapusBukuItemValid(t *testing.T) {
	item, err := BuildHapusBukuItem(UpdateHapusBukuItemInput{
		JenisAsetCode:       HapusBukuJenisAsetKredit,
		HubunganBankCode:    HapusBukuHubunganTidakTerkait,
		TanggalHapusBuku:    "2026-02-10",
		SaldoPokokSaatHapus: "1000000",
		SaldoPokokPerPosisi: "800000",
		BungaSaatHapus:      "250000",
		AgunanJenisCode:     "299",
		AgunanNilai:         "0",
	})
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	if item.SaldoPokokSaatHapus.String() != "1000000" {
		t.Fatalf("pokok = %s", item.SaldoPokokSaatHapus)
	}
	if item.TanggalHapusBuku.Format("2006-01-02") != "2026-02-10" {
		t.Fatalf("tanggal = %s", item.TanggalHapusBuku)
	}
}

// TestBuildHapusBukuItemTolakInputTakSah memastikan sandi, tanggal, dan nominal tak sah ditolak.
func TestBuildHapusBukuItemTolakInputTakSah(t *testing.T) {
	ok := func() UpdateHapusBukuItemInput {
		return UpdateHapusBukuItemInput{
			JenisAsetCode:    HapusBukuJenisAsetKredit,
			HubunganBankCode: HapusBukuHubunganTidakTerkait,
			TanggalHapusBuku: "2026-02-10",
			AgunanNilai:      "0",
		}
	}
	cases := map[string]func(UpdateHapusBukuItemInput) UpdateHapusBukuItemInput{
		"jenis aset": func(in UpdateHapusBukuItemInput) UpdateHapusBukuItemInput { in.JenisAsetCode = "99"; return in },
		"hubungan":   func(in UpdateHapusBukuItemInput) UpdateHapusBukuItemInput { in.HubunganBankCode = "99"; return in },
		"tanggal": func(in UpdateHapusBukuItemInput) UpdateHapusBukuItemInput {
			in.TanggalHapusBuku = "10-02-2026"
			return in
		},
		"nominal bukan angka": func(in UpdateHapusBukuItemInput) UpdateHapusBukuItemInput { in.SaldoPokokSaatHapus = "abc"; return in },
		"nominal negatif":     func(in UpdateHapusBukuItemInput) UpdateHapusBukuItemInput { in.BungaSaatHapus = "-1"; return in },
		"agunan bukan angka":  func(in UpdateHapusBukuItemInput) UpdateHapusBukuItemInput { in.AgunanNilai = "abc"; return in },
	}
	for name, mutate := range cases {
		if _, err := BuildHapusBukuItem(mutate(ok())); !errors.Is(err, ErrHapusBukuInputInvalid) {
			t.Fatalf("kasus %q: ingin ErrHapusBukuInputInvalid, dapat %v", name, err)
		}
	}
}

// TestBuildHapusBukuItemJenisDebiturBolehKosong memastikan kolom V boleh kosong.
func TestBuildHapusBukuItemJenisDebiturBolehKosong(t *testing.T) {
	_, err := BuildHapusBukuItem(UpdateHapusBukuItemInput{
		JenisAsetCode:    HapusBukuJenisAsetPenempatan,
		HubunganBankCode: HapusBukuHubunganTidakTerkait,
		TanggalHapusBuku: "2026-02-10",
		AgunanNilai:      "0",
	})
	if err != nil {
		t.Fatalf("jenis debitur kosong seharusnya sah: %v", err)
	}
}
