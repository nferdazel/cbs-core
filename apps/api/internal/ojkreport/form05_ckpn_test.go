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

// Kolom XVII-XIX memisahkan CKPN menurut golongan kualitas: Baik = Lancar, Kurang Baik
// = Kurang Lancar, Tidak Baik = Macet. CKPN satu penempatan hanya muncul pada kolom
// golongannya; kolom lain "-", dan tanpa asesmen ketiga kolom belum tersedia.
func TestBuildForm05CKPNPerGolonganKualitas(t *testing.T) {
	rows := []PlacementRow{
		{
			CounterpartyBank: "Bank Lancar", PlacementType: "GIRO", Collectibility: "LANCAR",
			Outstanding: decimal.NewFromInt(1_000_000),
			CKPN:        &PlacementCKPNRow{Method: "COLLECTIVE", RequiredCKPN: decimal.NewFromInt(11_000)},
		},
		{
			CounterpartyBank: "Bank Kurang Lancar", PlacementType: "DEPOSITO", Collectibility: "KURANG_LANCAR",
			Outstanding: decimal.NewFromInt(1_000_000),
			CKPN:        &PlacementCKPNRow{Method: "COLLECTIVE", RequiredCKPN: decimal.NewFromInt(22_000)},
		},
		{
			CounterpartyBank: "Bank Macet", PlacementType: "DEPOSITO", Collectibility: "MACET",
			Outstanding: decimal.NewFromInt(1_000_000),
			CKPN:        &PlacementCKPNRow{Method: "COLLECTIVE", RequiredCKPN: decimal.NewFromInt(33_000)},
		},
	}
	sec := buildForm05(rows)

	if got := findCell(t, sec, "Bank Lancar", form05SandiCKPNBaik).Value; got != "11000" {
		t.Errorf("CKPN aset baik = %q, ingin 11000", got)
	}
	if got := findCell(t, sec, "Bank Lancar", form05SandiCKPNKurang).Value; got != "-" {
		t.Errorf("bank lancar pada kolom kurang baik = %q, ingin -", got)
	}
	if got := findCell(t, sec, "Bank Kurang Lancar", form05SandiCKPNKurang).Value; got != "22000" {
		t.Errorf("CKPN aset kurang baik = %q, ingin 22000", got)
	}
	if got := findCell(t, sec, "Bank Macet", form05SandiCKPNTidak).Value; got != "33000" {
		t.Errorf("CKPN aset tidak baik = %q, ingin 33000", got)
	}
	if got := findCell(t, sec, "Bank Macet", form05SandiCKPNBaik).Value; got != "-" {
		t.Errorf("bank macet pada kolom baik = %q, ingin -", got)
	}
}

// Tanpa asesmen, kolom XVII-XIX belum tersedia dan beralasan seperti XII/XXI.
func TestBuildForm05CKPNGolonganBelumTersediaTanpaAsesmen(t *testing.T) {
	rows := []PlacementRow{
		{CounterpartyBank: "Bank Uji", PlacementType: "GIRO", Collectibility: "LANCAR", Outstanding: decimal.NewFromInt(1_000_000)},
	}
	sec := buildForm05(rows)
	for _, sandi := range []string{form05SandiCKPNBaik, form05SandiCKPNKurang, form05SandiCKPNTidak} {
		u := unavailableColumn(t, sec, sandi)
		if !strings.Contains(u.Reason, "ckpn.pabl.enabled") {
			t.Errorf("alasan kolom %s = %q, harus menyebut ckpn.pabl.enabled", sandi, u.Reason)
		}
	}
}
