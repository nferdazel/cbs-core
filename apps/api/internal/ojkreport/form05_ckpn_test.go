package ojkreport

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// Tanpa satu pun penempatan yang diasesmen, kolom XII (CKPN) dan XXI (Jenis CKPN)
// dinyatakan belum tersedia dengan alasan yang menyebut saklar dan asesmen.
func TestBuildForm05TanpaCKPNKolomXIIXXIBelumTersedia(t *testing.T) {
	rows := []PlacementRow{
		{CounterpartyBank: "Bank Uji A", PlacementType: "DEPOSITO", Collectibility: "LANCAR", Outstanding: decimal.NewFromInt(10_000_000)},
	}
	sec := buildForm05(rows)

	for _, sandi := range []string{form05SandiCKPN, form05SandiJenisCKPN} {
		u := unavailableColumn(t, sec, sandi)
		if !strings.Contains(u.Reason, "ckpn.pabl.enabled") {
			t.Errorf("alasan kolom %s = %q, harus menyebut saklar ckpn.pabl.enabled", sandi, u.Reason)
		}
	}
}

// Bila minimal satu penempatan sudah diasesmen, kolom XII dan XXI tersedia dan baris
// yang belum diasesmen tetap ditulis "-", bukan nol atau angka karangan.
func TestBuildForm05DenganCKPNKolomXIIXXITersedia(t *testing.T) {
	rows := []PlacementRow{
		{
			CounterpartyBank: "Bank Uji Terasesmen",
			PlacementType:    "DEPOSITO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(10_000_000),
			CKPN: &PlacementCKPNRow{
				Method:       "INDIVIDUAL_DCF",
				RequiredCKPN: decimal.NewFromInt(250_000),
			},
		},
		{CounterpartyBank: "Bank Uji Belum", PlacementType: "GIRO", Collectibility: "LANCAR", Outstanding: decimal.NewFromInt(1_000_000)},
	}
	sec := buildForm05(rows)

	// Kedua kolom tersedia: tidak boleh muncul di daftar kolom tidak tersedia.
	for _, sandi := range []string{form05SandiCKPN, form05SandiJenisCKPN} {
		for _, u := range sec.Unavailable {
			if u.Sandi == sandi {
				t.Errorf("kolom %s seharusnya tersedia saat ada asesmen", sandi)
			}
		}
	}

	if got := findCell(t, sec, "Bank Uji Terasesmen", form05SandiCKPN).Value; got != "250000" {
		t.Errorf("CKPN terasesmen = %s, ingin 250000", got)
	}
	if got := findCell(t, sec, "Bank Uji Terasesmen", form05SandiJenisCKPN).Value; got != "1" {
		t.Errorf("jenis CKPN individual = %s, ingin 1", got)
	}
	// Baris yang belum diasesmen ditulis "-" pada kedua kolom.
	if got := findCell(t, sec, "Bank Uji Belum", form05SandiCKPN).Value; got != "-" {
		t.Errorf("CKPN belum diasesmen = %s, ingin -", got)
	}
	if got := findCell(t, sec, "Bank Uji Belum", form05SandiJenisCKPN).Value; got != "-" {
		t.Errorf("jenis CKPN belum diasesmen = %s, ingin -", got)
	}
}

// Metode kolektif memakai sandi 2, cermin sandiJenisCKPN Form 06.00.
func TestBuildForm05JenisCKPNKolektifSandiDua(t *testing.T) {
	rows := []PlacementRow{
		{
			CounterpartyBank: "Bank Uji Kolektif",
			PlacementType:    "GIRO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(1_000_000),
			CKPN:             &PlacementCKPNRow{Method: "COLLECTIVE", RequiredCKPN: decimal.NewFromInt(5_000)},
		},
	}
	sec := buildForm05(rows)
	if got := findCell(t, sec, "Bank Uji Kolektif", form05SandiJenisCKPN).Value; got != "2" {
		t.Fatalf("jenis CKPN kolektif = %s, ingin 2", got)
	}
}
