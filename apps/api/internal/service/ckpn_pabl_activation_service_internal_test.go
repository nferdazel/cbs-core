package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// newCKPNPABLSvcForTest merakit layanan pengaturan CKPN PABL tanpa database. Stub store
// dan runner in-memory dipinjam dari uji jalur aktivasi utama (ckpnActivationStoreStub,
// memoryTxRunner): antarmuka penyimpanannya identik.
func newCKPNPABLSvcForTest(values map[string]string, cfg domain.SystemConfigService) (*ckpnPABLActivationService, *ckpnActivationStoreStub) {
	store := &ckpnActivationStoreStub{values: copyValues(values)}
	svc := &ckpnPABLActivationService{repo: store, config: cfg, runner: memoryTxRunner{}}
	return svc, store
}

// Fraksi nol ditolak di lapisan service dan TIDAK menulis apa pun.
func TestCKPNPABLServiceTolakFraksiNol(t *testing.T) {
	svc, store := newCKPNPABLSvcForTest(nil, nil)
	nol := "0"
	_, err := svc.Update(context.Background(),
		domain.UpdateCKPNPABLActivationInput{PDFracGol1: &nol}, domain.Actor{Username: "uji"})
	if !errors.Is(err, domain.ErrPABLCKPNParameterInvalid) {
		t.Fatalf("err = %v, ingin ErrPABLCKPNParameterInvalid", err)
	}
	if store.saved != nil {
		t.Fatalf("tidak boleh menulis saat validasi gagal: %v", store.saved)
	}
}

// enabled=true tanpa fraksi PABL lengkap ditolak dan tidak menulis.
func TestCKPNPABLServiceTolakNyalaPrematur(t *testing.T) {
	svc, store := newCKPNPABLSvcForTest(nil, nil)
	ya := true
	_, err := svc.Update(context.Background(),
		domain.UpdateCKPNPABLActivationInput{Enabled: &ya}, domain.Actor{Username: "uji"})
	if !errors.Is(err, domain.ErrCKPNPABLActivationNotReady) {
		t.Fatalf("err = %v, ingin ErrCKPNPABLActivationNotReady", err)
	}
	if store.saved != nil {
		t.Fatalf("tidak boleh menulis saat penyalakan ditolak: %v", store.saved)
	}
}

// Menonaktifkan selalu boleh dan tersimpan.
func TestCKPNPABLServiceMatikanBebas(t *testing.T) {
	svc, store := newCKPNPABLSvcForTest(map[string]string{domain.CKPNPABLEnabledKey: "true"}, nil)
	tidak := false
	res, err := svc.Update(context.Background(),
		domain.UpdateCKPNPABLActivationInput{Enabled: &tidak}, domain.Actor{Username: "uji"})
	if err != nil {
		t.Fatalf("menonaktifkan: %v", err)
	}
	if store.values[domain.CKPNPABLEnabledKey] != "false" {
		t.Fatalf("nilai tersimpan = %q, ingin false", store.values[domain.CKPNPABLEnabledKey])
	}
	if res.Values[domain.CKPNPABLEnabledKey] != "false" {
		t.Fatalf("hasil = %+v", res)
	}
}

// Penyalakan diterima bila fraksi PABL lengkap dan CKPN utama siap, lalu tersimpan.
func TestCKPNPABLServiceNyalaSetelahLengkap(t *testing.T) {
	cfg := &lpsConfigStub{values: pablMainReadyValues()}
	svc, store := newCKPNPABLSvcForTest(map[string]string{
		domain.CKPNPABLPDFracGol1Key: "0.01",
		domain.CKPNPABLPDFracGol3Key: "0.10",
		domain.CKPNPABLPDFracGol5Key: "0.50",
		domain.CKPNPABLLGDFracKey:    "0.45",
	}, cfg)
	ya := true
	res, err := svc.Update(context.Background(),
		domain.UpdateCKPNPABLActivationInput{Enabled: &ya}, domain.Actor{Username: "uji"})
	if err != nil {
		t.Fatalf("penyalakan: %v", err)
	}
	if store.values[domain.CKPNPABLEnabledKey] != "true" || !res.EnablementReady {
		t.Fatalf("hasil penyalakan = %+v", res)
	}
}

// PUT tanpa bidang ditolak dan tidak menyentuh store.
func TestCKPNPABLServiceTolakKosong(t *testing.T) {
	svc, store := newCKPNPABLSvcForTest(nil, nil)
	if _, err := svc.Update(context.Background(), domain.UpdateCKPNPABLActivationInput{}, domain.Actor{}); !errors.Is(err, domain.ErrCKPNPABLActivationEmpty) {
		t.Fatalf("err = %v, ingin ErrCKPNPABLActivationEmpty", err)
	}
	if store.saved != nil {
		t.Fatalf("tidak boleh menyimpan saat input kosong: %v", store.saved)
	}
}

// pablMainReadyValues mengembalikan nilai CKPN utama yang siap dinyalakan.
func pablMainReadyValues() map[string]string {
	return map[string]string{
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
	}
}
