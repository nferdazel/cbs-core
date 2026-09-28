package domain_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// pablMainReady membangun konfigurasi CKPN utama yang siap dinyalakan: parameter FINAL,
// bukti ratifikasi lengkap, PD/LGD kredit terisi sah, dan akun syariah terpetakan.
func pablMainReady() *mapConfig {
	return &mapConfig{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus:          domain.CKPNParameterStatusFinal,
		domain.ConfigKeyCKPNRatificationBANumber:      "BA/001",
		domain.ConfigKeyCKPNRatificationBADate:        "2026-09-01",
		domain.ConfigKeyCKPNRatificationApprovedBy:    "Direksi + Akuntan",
		domain.ConfigKeyCKPNRatificationPDLGDBasis:    "12.6/12.7 data historis",
		domain.ConfigKeyCKPNRatificationPDLGDFromBank: "true",
		"ckpn.pd_frac.gol_1":                          "0.0111",
		"ckpn.pd_frac.gol_2":                          "0.0667",
		"ckpn.pd_frac.gol_3":                          "0.2222",
		"ckpn.pd_frac.gol_4":                          "1",
		"ckpn.pd_frac.gol_5":                          "1",
		"ckpn.lgd_frac":                               "0.45",
		"ckpn.coa.expense.syariah":                    "15901",
		"ckpn.coa.reserve.syariah":                    "11950",
	}}
}

// fraksi PABL nol ditolak (nol = belum diisi dan mesin menolaknya) dan pesannya
// menyebut kunci yang salah; nol bukan satu-satunya nilai di luar rentang.
func TestValidateCKPNPABLActivationFraksiDitolak(t *testing.T) {
	ctx := context.Background()
	main := pablMainReady()

	for _, nilai := range []string{"0", "0.0", "-0.1", "1.5", "dua"} {
		err := domain.ValidateCKPNPABLActivation(ctx, map[string]string{
			domain.CKPNPABLPDFracGol1Key: nilai,
		}, main)
		if !errors.Is(err, domain.ErrPABLCKPNParameterInvalid) {
			t.Fatalf("nilai %q: err = %v, ingin ErrPABLCKPNParameterInvalid", nilai, err)
		}
		if !strings.Contains(err.Error(), domain.CKPNPABLPDFracGol1Key) {
			t.Fatalf("nilai %q: pesan %q harus menyebut nama kunci", nilai, err)
		}
	}
}

// Nilai kosong (belum diisi/dikosongkan) dan batas atas 1 diterima saat saklar mati.
func TestValidateCKPNPABLActivationFraksiSah(t *testing.T) {
	ctx := context.Background()
	for _, nilai := range []string{"", "0.0001", "1"} {
		if err := domain.ValidateCKPNPABLActivation(ctx, map[string]string{
			domain.CKPNPABLPDFracGol1Key: nilai,
		}, pablMainReady()); err != nil {
			t.Fatalf("nilai %q: err = %v, ingin diterima", nilai, err)
		}
	}
}

// enabled=true ditolak selama fraksi PABL belum lengkap, dengan penahan menyebut kunci.
func TestValidateCKPNPABLActivationNyalaTanpaFraksiLengkap(t *testing.T) {
	err := domain.ValidateCKPNPABLActivation(context.Background(), map[string]string{
		domain.CKPNPABLEnabledKey: "true",
	}, pablMainReady())
	if !errors.Is(err, domain.ErrCKPNPABLActivationNotReady) {
		t.Fatalf("err = %v, ingin ErrCKPNPABLActivationNotReady", err)
	}
	if !strings.Contains(err.Error(), domain.CKPNPABLPDFracGol1Key) {
		t.Fatalf("penahan %q harus menyebut kunci fraksi yang kosong", err)
	}
}

// enabled=true ditolak bila fraksi PABL lengkap tetapi CKPN utama belum siap (parameter
// SEMENTARA/tanpa ratifikasi): Form 05.00 tidak boleh memakai parameter sementara.
func TestValidateCKPNPABLActivationNyalaTanpaKesiapanUtama(t *testing.T) {
	err := domain.ValidateCKPNPABLActivation(context.Background(), map[string]string{
		domain.CKPNPABLPDFracGol1Key: "0.01",
		domain.CKPNPABLPDFracGol3Key: "0.10",
		domain.CKPNPABLPDFracGol5Key: "0.50",
		domain.CKPNPABLLGDFracKey:    "0.45",
		domain.CKPNPABLEnabledKey:    "true",
	}, &mapConfig{values: map[string]string{}})
	if !errors.Is(err, domain.ErrCKPNPABLActivationNotReady) {
		t.Fatalf("err = %v, ingin ErrCKPNPABLActivationNotReady", err)
	}
}

// Fraksi PABL lengkap + CKPN utama siap -> penyalakan diterima; menonaktifkan selalu
// boleh walau fraksi belum lengkap.
func TestValidateCKPNPABLActivationNyalaSiapDanMatikanBebas(t *testing.T) {
	lengkap := map[string]string{
		domain.CKPNPABLPDFracGol1Key: "0.01",
		domain.CKPNPABLPDFracGol3Key: "0.10",
		domain.CKPNPABLPDFracGol5Key: "0.50",
		domain.CKPNPABLLGDFracKey:    "0.45",
		domain.CKPNPABLEnabledKey:    "true",
	}
	if err := domain.ValidateCKPNPABLActivation(context.Background(), lengkap, pablMainReady()); err != nil {
		t.Fatalf("penyalakan saat siap: %v", err)
	}
	if err := domain.ValidateCKPNPABLActivation(context.Background(), map[string]string{
		domain.CKPNPABLEnabledKey: "false",
	}, &mapConfig{values: map[string]string{}}); err != nil {
		t.Fatalf("menonaktifkan tidak boleh ditahan: %v", err)
	}
}

// Input mengabaikan bidang nil, menormalkan bool, dan mendeteksi permintaan kosong.
func TestUpdateCKPNPABLActivationInputChanges(t *testing.T) {
	kosong := domain.UpdateCKPNPABLActivationInput{}
	if !kosong.IsEmpty() {
		t.Fatal("input tanpa bidang harus dianggap kosong")
	}

	pd := "0.0100"
	ya := true
	changes := domain.UpdateCKPNPABLActivationInput{PDFracGol1: &pd, Enabled: &ya}.
		Changes(map[string]string{domain.CKPNPABLAsetBaikBentukCKPNKey: "false"})
	if changes[domain.CKPNPABLPDFracGol1Key] != pd || changes[domain.CKPNPABLEnabledKey] != "true" {
		t.Fatalf("changes = %v", changes)
	}
	// Nilai yang sama tidak menghasilkan perubahan semu.
	if got := (domain.UpdateCKPNPABLActivationInput{Enabled: &ya}).
		Changes(map[string]string{domain.CKPNPABLEnabledKey: "true"}); len(got) != 0 {
		t.Fatalf("nilai sama harus dilewati, got %v", got)
	}
}
