package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// storeStub meniru CKPNActivationStore tanpa database; txRunner in-memory memanggil
// callback dengan tx nil karena stub mengabaikannya.
type ckpnActivationStoreStub struct {
	values  map[string]string
	saved   map[string]string
	getErr  error
	saveErr error
}

func (s *ckpnActivationStoreStub) Get(context.Context) (map[string]string, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return copyValues(s.values), nil
}

func (s *ckpnActivationStoreStub) GetTx(context.Context, any) (map[string]string, error) {
	return s.Get(context.Background())
}

func (s *ckpnActivationStoreStub) SaveTx(_ context.Context, _ any, changes map[string]string, _ uuid.UUID) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	s.saved = copyValues(changes)
	for k, v := range changes {
		s.values[k] = v
	}
	return nil
}

type memoryTxRunner struct{ err error }

func (r memoryTxRunner) Run(_ context.Context, fn func(tx any) error) error {
	if r.err != nil {
		return r.err
	}
	return fn(nil)
}

func copyValues(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func newCKPNActivationSvcForTest(values map[string]string) (*ckpnActivationService, *ckpnActivationStoreStub) {
	store := &ckpnActivationStoreStub{values: copyValues(values)}
	svc := &ckpnActivationService{repo: store, runner: memoryTxRunner{}}
	return svc, store
}

// Menyetel FINAL tanpa bukti ditolak di lapisan service dan TIDAK menulis apa pun.
func TestCKPNActivationServiceTolakFinalTanpaBukti(t *testing.T) {
	svc, store := newCKPNActivationSvcForTest(map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
	})
	final := domain.CKPNParameterStatusFinal
	_, err := svc.Update(context.Background(), domain.UpdateCKPNActivationInput{ParametersStatus: &final}, domain.Actor{Username: "uji"})
	if !errors.Is(err, domain.ErrCKPNActivationRatificationIncomplete) {
		t.Fatalf("err = %v, ingin ErrCKPNActivationRatificationIncomplete", err)
	}
	if store.saved != nil {
		t.Fatalf("tidak boleh ada yang tersimpan saat validasi gagal: %v", store.saved)
	}
}

// Bukti lengkap + FINAL disimpan, lalu penyalakan diterima.
func TestCKPNActivationServiceAktifkanSetelahLengkap(t *testing.T) {
	svc, store := newCKPNActivationSvcForTest(map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
	})
	pd, lgd := "0.0100", "0.4500"
	ba, tanggal, oleh, dasar := "BA-1", "2026-01-05", "Dirut", "PA BPR 12.6/12.7"
	dariBank := true
	coaE, coaR := "15901", "11950"
	final := domain.CKPNParameterStatusFinal
	if _, err := svc.Update(context.Background(), domain.UpdateCKPNActivationInput{
		PDFracGol1: &pd, PDFracGol2: &pd, PDFracGol3: &pd, PDFracGol4: &pd, PDFracGol5: &pd,
		LGDFrac: &lgd, COAExpenseSyariah: &coaE, COAReserveSyariah: &coaR,
		ParametersStatus: &final, RatificationBANumber: &ba, RatificationBADate: &tanggal,
		RatificationApprovedBy: &oleh, RatificationPDLGDBasis: &dasar, RatificationPDLGDFromBank: &dariBank,
	}, domain.Actor{Username: "uji"}); err != nil {
		t.Fatalf("penyetelan lengkap: %v", err)
	}
	if store.values[domain.ConfigKeyCKPNParametersStatus] != domain.CKPNParameterStatusFinal {
		t.Fatalf("status tersimpan = %q, ingin FINAL", store.values[domain.ConfigKeyCKPNParametersStatus])
	}

	ya := true
	res, err := svc.Update(context.Background(), domain.UpdateCKPNActivationInput{CKPNEnabled: &ya}, domain.Actor{Username: "uji"})
	if err != nil {
		t.Fatalf("penyalakan: %v", err)
	}
	if res.Values[domain.ConfigKeyCKPNEnabled] != "true" || !res.EnablementReady {
		t.Fatalf("hasil penyalakan = %+v", res)
	}
}

// PUT tanpa bidang ditolak dan tidak menyentuh store.
func TestCKPNActivationServiceTolakKosong(t *testing.T) {
	svc, store := newCKPNActivationSvcForTest(nil)
	if _, err := svc.Update(context.Background(), domain.UpdateCKPNActivationInput{}, domain.Actor{}); !errors.Is(err, domain.ErrCKPNActivationEmpty) {
		t.Fatalf("err = %v, ingin ErrCKPNActivationEmpty", err)
	}
	if store.saved != nil {
		t.Fatalf("tidak boleh menyimpan saat input kosong: %v", store.saved)
	}
}
