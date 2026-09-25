package service_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// TestIntegrasiCKPNActivationJalurLengkap membuktikan jalur aktivasi CKPN terhadap
// PostgreSQL sungguhan: validasi menolak penyalakan prematur dan FINAL tanpa bukti,
// lalu penyetelan lengkap (bukti + FINAL + PD/LGD + akun syariah) DITERIMA walau ditulis
// dalam SATU permintaan — menguji urutan tulis terhadap trigger 000095 yang membaca
// bukti saat baris status FINAL ditulis.
func TestIntegrasiCKPNActivationJalurLengkap(t *testing.T) {
	e := newMoneyEnv(t)
	for _, key := range domain.CKPNActivationKeys() {
		e.simpanPulihkanConfigKunci(t, key)
	}
	// Harness jalur uang sudah mengisi bukti ratifikasi uji dan menyetel status FINAL agar
	// uji lain dapat berjalan. Uji penjaga ini justru harus mulai dari KOSONG: hapus baris
	// kunci yang dikelola supaya validasi diuji dari keadaan belum diisi.
	if _, err := e.db.ExecContext(e.ctx,
		`DELETE FROM system_config WHERE key = ANY($1)`, domain.CKPNActivationKeys()); err != nil {
		t.Fatalf("mengosongkan kunci aktivasi: %v", err)
	}

	svc := service.NewCKPNActivationService(e.db, postgres.NewCKPNActivationRepository(e.db),
		e.configSvc, postgres.NewAuditRepository(e.db))

	// 1) Menyalakan CKPN lebih dulu HARUS ditolak: belum ada parameter sama sekali.
	ya := true
	if _, err := svc.Update(e.ctx, domain.UpdateCKPNActivationInput{CKPNEnabled: &ya}, e.actor); !errors.Is(err, domain.ErrCKPNActivationNotReady) {
		t.Fatalf("penyalakan prematur = %v, ingin ErrCKPNActivationNotReady", err)
	}

	// 2) FINAL tanpa bukti ratifikasi DITOLAK di lapisan service.
	final := domain.CKPNParameterStatusFinal
	if _, err := svc.Update(e.ctx, domain.UpdateCKPNActivationInput{ParametersStatus: &final}, e.actor); !errors.Is(err, domain.ErrCKPNActivationRatificationIncomplete) {
		t.Fatalf("FINAL tanpa bukti = %v, ingin ErrCKPNActivationRatificationIncomplete", err)
	}

	// 3) Penyetelan lengkap DALAM SATU permintaan: bukti + PD/LGD + akun syariah + FINAL.
	pd := "0.0100"
	lgd := "0.4500"
	baNumber := "BA-07/IX/2026"
	baDate := "2026-08-01"
	approvedBy := "Direktur Utama"
	basis := "PA BPR 12.6/12.7 data historis 2024"
	fromBank := true
	coaExpense := "15901"
	coaReserve := "11950"
	input := domain.UpdateCKPNActivationInput{
		PDFracGol1: &pd, PDFracGol2: &pd, PDFracGol3: &pd, PDFracGol4: &pd, PDFracGol5: &pd,
		LGDFrac:           &lgd,
		COAExpenseSyariah: &coaExpense, COAReserveSyariah: &coaReserve,
		ParametersStatus:          &final,
		RatificationBANumber:      &baNumber,
		RatificationBADate:        &baDate,
		RatificationApprovedBy:    &approvedBy,
		RatificationPDLGDBasis:    &basis,
		RatificationPDLGDFromBank: &fromBank,
	}
	got, err := svc.Update(e.ctx, input, e.actor)
	if err != nil {
		t.Fatalf("penyetelan lengkap ditolak: %v", err)
	}
	if got.Status.Status != domain.CKPNParameterStatusFinal || got.Status.Sementara {
		t.Fatalf("status setelah penyetelan = %+v, ingin FINAL", got.Status)
	}

	// 4) Setelah lengkap, penyalakan DITERIMA.
	if _, err := svc.Update(e.ctx, domain.UpdateCKPNActivationInput{CKPNEnabled: &ya}, e.actor); err != nil {
		t.Fatalf("penyalakan setelah lengkap ditolak: %v", err)
	}
	after, err := svc.Get(e.ctx)
	if err != nil {
		t.Fatalf("Get setelah penyalakan: %v", err)
	}
	if after.Values[domain.ConfigKeyCKPNEnabled] != "true" {
		t.Fatalf("ckpn.enabled = %q, ingin true", after.Values[domain.ConfigKeyCKPNEnabled])
	}
	if after.Values[domain.CKPNLGDFracKey] != lgd || after.Values[domain.CKPNPDFracKey(3)] != pd {
		t.Fatalf("nilai PD/LGD tidak tersimpan: %+v", after.Values)
	}

	// 5) Audit mencatat perubahan.
	events, err := postgres.NewAuditRepository(e.db).List(e.ctx, "ckpn_activation", "1", 20)
	if err != nil {
		t.Fatalf("membaca audit: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("audit UPDATE_CKPN_ACTIVATION tidak ditemukan")
	}
}
