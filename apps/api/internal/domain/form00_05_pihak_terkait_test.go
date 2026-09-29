package domain

import (
	"errors"
	"strings"
	"testing"
)

// TestBuildPihakTerkaitItemValid memastikan masukan sah lolos dan id kosong membuat id baru.
func TestBuildPihakTerkaitItemValid(t *testing.T) {
	item, err := BuildPihakTerkaitItem(UpdatePihakTerkaitItemInput{
		Nama:         "  Budi  ",
		Alamat:       " Jl. Contoh ",
		JenisCode:    PihakTerkaitJenisPerorangan,
		HubunganCode: PihakTerkaitHubunganPengendaliKeluarga,
	})
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	if item.Nama != "Budi" || item.Alamat != "Jl. Contoh" {
		t.Fatalf("spasi tidak dipangkas: %+v", item)
	}
	if item.ID.String() == "" {
		t.Fatal("id kosong seharusnya menghasilkan id baru")
	}
}

// TestBuildPihakTerkaitItemTolakSandiTakBaku memastikan sandi IV/V di luar baku ditolak,
// bukan diterka.
func TestBuildPihakTerkaitItemTolakSandiTakBaku(t *testing.T) {
	cases := []UpdatePihakTerkaitItemInput{
		{Nama: "Budi", JenisCode: "99", HubunganCode: PihakTerkaitHubunganPengendaliKeluarga},
		{Nama: "Budi", JenisCode: PihakTerkaitJenisPerorangan, HubunganCode: "99"},
		{Nama: "", JenisCode: PihakTerkaitJenisPerorangan, HubunganCode: PihakTerkaitHubunganPengendaliKeluarga},
		{Nama: strings.Repeat("x", 256), JenisCode: PihakTerkaitJenisPerorangan, HubunganCode: PihakTerkaitHubunganPengendaliKeluarga},
	}
	for i, c := range cases {
		if _, err := BuildPihakTerkaitItem(c); !errors.Is(err, ErrPihakTerkaitInputInvalid) {
			t.Fatalf("kasus %d: ingin ErrPihakTerkaitInputInvalid, dapat %v", i, err)
		}
	}
}
