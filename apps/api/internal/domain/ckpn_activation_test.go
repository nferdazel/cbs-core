package domain

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// ujiNow adalah waktu tetap uji supaya penolakan "tanggal di masa depan" tidak
// bergantung jam sebenarnya.
var ujiNow = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

// lengkapCKPNActivation membangun kandidat aktivasi yang seluruh penahannya kosong:
// parameter FINAL, bukti ratifikasi lengkap, PD/LGD terisi, akun syariah terpetakan.
func lengkapCKPNActivation() map[string]string {
	m := map[string]string{
		ConfigKeyCKPNParametersStatus:          CKPNParameterStatusFinal,
		ConfigKeyCKPNRatificationBANumber:      "BA-01/IX/2026",
		ConfigKeyCKPNRatificationBADate:        "2026-01-05",
		ConfigKeyCKPNRatificationApprovedBy:    "Direktur Utama",
		ConfigKeyCKPNRatificationPDLGDBasis:    "PA BPR 12.6/12.7 data historis 2024",
		ConfigKeyCKPNRatificationPDLGDFromBank: "true",
		CKPNLGDFracKey:                         "0.45",
		CKPNCOAExpenseSyariahKey:               "15901",
		CKPNCOAReserveSyariahKey:               "11950",
	}
	for i := 1; i <= 5; i++ {
		m[CKPNPDFracKey(i)] = "0.005"
	}
	return m
}

// Fraksi di luar 0..1 atau bukan angka ditolak, dan pesannya menyebut kuncinya.
func TestValidateCKPNActivationMenolakFraksiTidakSah(t *testing.T) {
	for _, nilai := range []string{"1.5", "-0.1", "lima persen"} {
		c := lengkapCKPNActivation()
		c[CKPNLGDFracKey] = nilai
		err := ValidateCKPNActivation(c, ujiNow)
		if err == nil {
			t.Fatalf("LGD %q seharusnya ditolak", nilai)
		}
		if !strings.Contains(err.Error(), CKPNLGDFracKey) {
			t.Errorf("pesan galat %q harus menyebut kunci", err.Error())
		}
	}
}

// Persen yang tertulis sebagai 45 (bukan 0,45) ditolak: satuan wajib fraksi.
func TestValidateCKPNActivationMenolakPersenSebagaiFraksi(t *testing.T) {
	c := lengkapCKPNActivation()
	c[CKPNPDFracKey(1)] = "45"
	if err := ValidateCKPNActivation(c, ujiNow); err == nil {
		t.Fatal("PD 45 (persen) seharusnya ditolak")
	}
}

// FINAL tidak boleh disetel bila bukti ratifikasi belum lengkap.
func TestValidateCKPNActivationMenolakFinalTanpaBukti(t *testing.T) {
	c := lengkapCKPNActivation()
	delete(c, ConfigKeyCKPNRatificationBANumber)
	err := ValidateCKPNActivation(c, ujiNow)
	if err == nil || !strings.Contains(err.Error(), ConfigKeyCKPNRatificationBANumber) {
		t.Fatalf("FINAL tanpa BA harus ditolak menyebut kuncinya, dapat %v", err)
	}
}

// FINAL dengan bukti lengkap diterima.
func TestValidateCKPNActivationMenerimaFinalDenganBukti(t *testing.T) {
	if err := ValidateCKPNActivation(lengkapCKPNActivation(), ujiNow); err != nil {
		t.Fatalf("kandidat lengkap harus diterima: %v", err)
	}
}

// Menyalakan CKPN saat parameter masih SEMENTARA ditolak.
func TestValidateCKPNActivationMenolakNyalaPrematur(t *testing.T) {
	c := lengkapCKPNActivation()
	c[ConfigKeyCKPNParametersStatus] = CKPNParameterStatusSementara
	c[ConfigKeyCKPNEnabled] = "true"
	if err := ValidateCKPNActivation(c, ujiNow); err == nil {
		t.Fatal("nyala dengan parameter SEMENTARA harus ditolak")
	}
}

// Menyalakan CKPN saat PD belum diisi juga ditolak, walau status FINAL.
func TestValidateCKPNActivationMenolakNyalaTanpaPD(t *testing.T) {
	c := lengkapCKPNActivation()
	delete(c, CKPNPDFracKey(3))
	c[ConfigKeyCKPNEnabled] = "true"
	if err := ValidateCKPNActivation(c, ujiNow); err == nil {
		t.Fatal("nyala tanpa PD golongan 3 harus ditolak")
	}
}

// Kandidat lengkap boleh dinyalakan.
func TestValidateCKPNActivationMenerimaNyalaSaatLengkap(t *testing.T) {
	c := lengkapCKPNActivation()
	c[ConfigKeyCKPNEnabled] = "true"
	if err := ValidateCKPNActivation(c, ujiNow); err != nil {
		t.Fatalf("penyalakan dengan kandidat lengkap harus diterima: %v", err)
	}
}

// Status selain SEMENTARA/FINAL ditolak.
func TestValidateCKPNActivationMenolakStatusAsing(t *testing.T) {
	c := lengkapCKPNActivation()
	c[ConfigKeyCKPNParametersStatus] = "DRAFT"
	if err := ValidateCKPNActivation(c, ujiNow); err == nil {
		t.Fatal("status DRAFT seharusnya ditolak")
	}
}

// Tanggal di masa depan ditolak; format salah juga.
func TestValidateCKPNActivationMenolakTanggalMasaDepan(t *testing.T) {
	c := lengkapCKPNActivation()
	c[ConfigKeyCKPNRatificationBADate] = "2027-01-01"
	if err := ValidateCKPNActivation(c, ujiNow); err == nil {
		t.Fatal("tanggal BA di masa depan harus ditolak")
	}
	c[ConfigKeyCKPNRatificationBADate] = "05-01-2026"
	if err := ValidateCKPNActivation(c, ujiNow); err == nil {
		t.Fatal("tanggal BA bukan YYYY-MM-DD harus ditolak")
	}
}

// Changes hanya mengembalikan bidang yang dikirim DAN berbeda; status dinormalkan besar.
func TestUpdateCKPNActivationChangesHanyaYangBerubah(t *testing.T) {
	sama := "0.45"
	baru := "0.50"
	status := " final "
	in := UpdateCKPNActivationInput{LGDFrac: &sama, PDFracGol1: &baru, ParametersStatus: &status}
	changes := in.Changes(map[string]string{CKPNLGDFracKey: "0.45"})

	if _, ok := changes[CKPNLGDFracKey]; ok {
		t.Error("nilai yang sama tidak boleh masuk perubahan")
	}
	if changes[CKPNPDFracKey(1)] != "0.50" {
		t.Errorf("PD gol 1 = %q, ingin 0.50", changes[CKPNPDFracKey(1)])
	}
	if changes[ConfigKeyCKPNParametersStatus] != "FINAL" {
		t.Errorf("status = %q, ingin FINAL (dinormalkan besar)", changes[ConfigKeyCKPNParametersStatus])
	}
}

// IsEmpty benar hanya bila tidak ada satu bidang pun dikirim.
func TestUpdateCKPNActivationIsEmpty(t *testing.T) {
	if !(UpdateCKPNActivationInput{}).IsEmpty() {
		t.Error("input kosong harus IsEmpty")
	}
	v := "x"
	if (UpdateCKPNActivationInput{LGDFrac: &v}).IsEmpty() {
		t.Error("input berisi satu bidang tidak boleh IsEmpty")
	}
}

// ConfigSnapshot mengikuti parsing config_service: nilai kosong/tak sah jatuh fallback.
func TestConfigSnapshotGagalAman(t *testing.T) {
	snap := NewConfigSnapshot(map[string]string{"a": "", "b": "bukan-angka"})
	if got := snap.GetDecimal(context.Background(), "a", mustDecimal(t, "0.5")); !got.Equal(mustDecimal(t, "0.5")) {
		t.Errorf("nilai kosong harus jatuh fallback, dapat %s", got)
	}
	if got := snap.GetBool(context.Background(), "b", true); !got {
		t.Error("nilai boolean tak sah harus jatuh fallback")
	}
	if got := snap.GetString(context.Background(), "tidak-ada", "bawaan"); got != "bawaan" {
		t.Errorf("kunci tak ada harus jatuh fallback, dapat %q", got)
	}
}

func mustDecimal(t *testing.T, raw string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(raw)
	if err != nil {
		t.Fatalf("desimal uji %q: %v", raw, err)
	}
	return d
}
