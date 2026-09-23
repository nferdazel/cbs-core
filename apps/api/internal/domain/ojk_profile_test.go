package domain

import (
	"errors"
	"strings"
	"testing"
)

func ojkStr(v string) *string { return &v }

// Profil OJK kosong SELALU sah: kosong berarti bank belum menyediakan field itu,
// bukan galat. Validasi hanya menolak bentuk yang jelas salah.
func TestValidateOJKProfileTerimaKosong(t *testing.T) {
	if err := ValidateOJKProfile(&OJKProfile{}); err != nil {
		t.Fatalf("profil OJK kosong ditolak: %v", err)
	}
	// Spasi di tepi dirapikan; bidang tetap boleh kosong.
	p := &OJKProfile{BankEmail: "  ", PICPhone: " ", LakuPandaiAgentCount: ""}
	if err := ValidateOJKProfile(p); err != nil {
		t.Fatalf("bidang kosong berisi spasi ditolak: %v", err)
	}
	if p.BankEmail != "" {
		t.Fatalf("spasi tepi tidak dirapikan: %q", p.BankEmail)
	}
}

// Bentuk surel/situs/telepon/jumlah agen yang jelas salah ditolak dengan sentinel
// masing-masing; nilai tidak dinormalkan diam-diam.
func TestValidateOJKProfileTolakBentukSalah(t *testing.T) {
	cases := []struct {
		nama string
		p    OJKProfile
		want error
	}{
		{"surel tanpa @", OJKProfile{BankEmail: "bpr.example.com"}, ErrOJKProfileEmailInvalid},
		{"surel PIC tanpa @", OJKProfile{PICEmail: "pic.example.com"}, ErrOJKProfileEmailInvalid},
		{"situs tanpa skema", OJKProfile{BankWebsite: "bpr.example.com"}, ErrOJKProfileWebsiteInvalid},
		{"telepon karakter aneh", OJKProfile{PICPhone: "0812#999"}, ErrOJKProfilePhoneInvalid},
		{"jumlah agen bukan angka", OJKProfile{LakuPandaiAgentCount: "dua belas"}, ErrOJKProfileAgentCountInvalid},
	}
	for _, c := range cases {
		err := ValidateOJKProfile(&c.p)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, ingin %v", c.nama, err, c.want)
		}
	}
}

// Bentuk yang wajar diterima, termasuk situs https dan telepon dengan tanda baca.
func TestValidateOJKProfileTerimaBentukWajar(t *testing.T) {
	p := &OJKProfile{
		BankEmail:            "bpr@contoh.co.id",
		BankWebsite:          "https://bpr.contoh.co.id",
		PICEmail:             "pic@contoh.co.id",
		PICPhone:             "+62 (21) 555-1234",
		LakuPandaiAgentCount: "12",
	}
	if err := ValidateOJKProfile(p); err != nil {
		t.Fatalf("profil OJK wajar ditolak: %v", err)
	}
}

// Nilai yang melampaui batas ditolak agar tempelan berkas tidak tersimpan sebagai identitas.
func TestValidateOJKProfileTolakTerlaluPanjang(t *testing.T) {
	p := &OJKProfile{AuditInfo: strings.Repeat("a", OJKProfileFieldMaxLen+1)}
	if err := ValidateOJKProfile(p); !errors.Is(err, ErrOJKProfileFieldTooLong) {
		t.Fatalf("err = %v, ingin ErrOJKProfileFieldTooLong", err)
	}
}

// Pemetaan kunci <-> bidang konsisten dua arah, sehingga baca/tulis/audit memakai
// sumber yang sama.
func TestOJKProfileKeyValuesBolakBalik(t *testing.T) {
	orig := &OJKProfile{
		BankEmail:            "bpr@contoh.co.id",
		AuditInfo:            "KAP Contoh, opini WTP",
		LakuPandaiAgentCount: "7",
	}
	values := orig.KeyValues()
	if values[OJKBankEmailKey] != "bpr@contoh.co.id" || values[OJKAuditInfoKey] != "KAP Contoh, opini WTP" {
		t.Fatalf("KeyValues salah: %#v", values)
	}
	got := OJKProfileFromValues(values)
	if *got != *orig {
		t.Fatalf("bolak-balik tidak identik:\n got=%+v\norig=%+v", got, orig)
	}
	// Kunci yang tidak ada menjadi kosong, bukan galat/nilai bawaan.
	empty := OJKProfileFromValues(map[string]string{})
	if *empty != (OJKProfile{}) {
		t.Fatalf("peta kosong menghasilkan %+v, ingin nol", empty)
	}
}

// UpdateOJKProfileInput.IsEmpty membedakan payload kosong dari payload berisi.
func TestUpdateOJKProfileInputIsEmpty(t *testing.T) {
	if !(UpdateOJKProfileInput{}).IsEmpty() {
		t.Fatal("payload kosong harus IsEmpty")
	}
	if (UpdateOJKProfileInput{BankEmail: ojkStr("")}).IsEmpty() {
		t.Fatal("payload dengan bidang (walau kosong) bukan IsEmpty")
	}
}

// Perubahan audit hanya memuat kunci yang benar-benar berubah.
func TestOJKProfileChangesHanyaYangBerubah(t *testing.T) {
	before := &OJKProfile{BankEmail: "lama@contoh.id", PICName: "Budi"}
	after := &OJKProfile{BankEmail: "baru@contoh.id", PICName: "Budi"}
	changes := OJKProfileChanges(before, after)
	if len(changes) != 1 {
		t.Fatalf("changes = %#v, ingin 1 kunci", changes)
	}
	c, ok := changes[OJKBankEmailKey].(map[string]any)
	if !ok || c["before"] != "lama@contoh.id" || c["after"] != "baru@contoh.id" {
		t.Fatalf("changes[surel] = %#v", changes[OJKBankEmailKey])
	}
}

// NPWP kini divalidasi 15/16 digit angka; format berpemisah tetap diterima dan nilai
// tersimpan tidak diubah.
func TestValidateBankProfileNPWPSetengahDigit(t *testing.T) {
	valid := []string{"123456789012345", "1234567890123456", "01.234.567.8-901.000", "12 345 678 901 2345"}
	for _, npwp := range valid {
		if err := ValidateBankProfile(&BankProfile{Name: "BPR Contoh", NPWP: npwp}); err != nil {
			t.Errorf("NPWP %q ditolak: %v", npwp, err)
		}
	}
	invalid := []string{"12345678901234", "12345678901234567", "abc", "12345678901234a"}
	for _, npwp := range invalid {
		if err := ValidateBankProfile(&BankProfile{Name: "BPR Contoh", NPWP: npwp}); !errors.Is(err, ErrBankProfileNPWPInvalid) {
			t.Errorf("NPWP %q: err = %v, ingin ErrBankProfileNPWPInvalid", npwp, err)
		}
	}
	// Kosong tetap sah (belum diisi).
	if err := ValidateBankProfile(&BankProfile{Name: "BPR Contoh"}); err != nil {
		t.Errorf("NPWP kosong ditolak: %v", err)
	}
}
