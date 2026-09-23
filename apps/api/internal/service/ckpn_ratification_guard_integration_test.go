package service_test

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// Uji integrasi penjaga ratifikasi parameter CKPN (migrasi 000095, keputusan panel
// butir 1) terhadap PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiPenjagaRatifikasiCKPN -v
//
// Yang dibuktikan: perubahan ckpn.parameters.status menuju FINAL DITOLAK trigger selama
// bukti belum lengkap (dengan pesan yang menyebut apa yang kurang), DITERIMA setelah
// bukti lengkap, dan jalur balik ke SEMENTARA tidak dihalangi.
const ckpnRatificationMigration = "000095_ckpn_ratification_guard.up.sql"

const (
	keyStatus     = domain.ConfigKeyCKPNParametersStatus
	keyBANumber   = domain.ConfigKeyCKPNRatificationBANumber
	keyBADate     = domain.ConfigKeyCKPNRatificationBADate
	keyApprovedBy = domain.ConfigKeyCKPNRatificationApprovedBy
	keyBasis      = domain.ConfigKeyCKPNRatificationPDLGDBasis
	keyFromBank   = domain.ConfigKeyCKPNRatificationPDLGDFromBank
)

// kosongkanBuktiRatifikasi menyetel semua bukti ke nilai "belum diisi" dan menurunkan
// status ke SEMENTARA, supaya transisi menuju FINAL benar-benar diuji (FINAL->FINAL
// memang tidak diperiksa ulang).
func kosongkanBuktiRatifikasi(t *testing.T, e *moneyEnv) {
	t.Helper()
	setCKPNConfig(t, e, keyStatus, domain.CKPNParameterStatusSementara)
	setCKPNConfig(t, e, keyBANumber, "")
	setCKPNConfig(t, e, keyBADate, "")
	setCKPNConfig(t, e, keyApprovedBy, "")
	setCKPNConfig(t, e, keyBasis, "")
	setCKPNConfig(t, e, keyFromBank, "false")
}

// simpanPulihkanCKPNEnabled menjaga uji ini tidak mengubah nilai ckpn.enabled yang
// mungkin sudah disetel uji lain pada database bersama: nilai lama dipulihkan setelah
// uji selesai, sehingga isolasi uji (dan perilaku uji lain) tidak berubah.
func simpanPulihkanCKPNEnabled(t *testing.T, e *moneyEnv) {
	t.Helper()
	snaps := snapshotConfig(t, e.db, e.ctx, "ckpn.enabled")
	t.Cleanup(func() { restoreConfig(t, e.db, e.ctx, snaps) })
}

func TestIntegrasiPenjagaRatifikasiCKPN(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanCKPNEnabled(t, e)
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnRatificationMigration)
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	kosongkanBuktiRatifikasi(t, e)
	t.Cleanup(func() {
		// Pulihkan keadaan: status SEMENTARA dan bukti kosong. Balik ke SEMENTARA tidak
		// dihalangi trigger (hanya transisi menuju FINAL yang dijaga).
		setCKPNConfig(t, e, keyStatus, domain.CKPNParameterStatusSementara)
		kosongkanBuktiRatifikasi(t, e)
	})

	// 1. Tanpa bukti: perubahan ke FINAL DITOLAK dan pesannya menyebut yang kurang.
	_, err := e.db.ExecContext(e.ctx, `UPDATE system_config SET value='FINAL' WHERE key=$1`, keyStatus)
	if err == nil {
		t.Fatal("perubahan ke FINAL tanpa bukti harus DITOLAK trigger")
	}
	pesan := err.Error()
	for _, kurang := range []string{keyBANumber, keyApprovedBy, keyBasis} {
		if !strings.Contains(pesan, kurang) {
			t.Fatalf("pesan penolakan wajib menyebut %s, dapat: %s", kurang, pesan)
		}
	}
	if got := bacaConfigCKPNCOA(t, e, keyStatus); got == domain.CKPNParameterStatusFinal {
		t.Fatalf("status tidak boleh berubah menjadi FINAL saat bukti kurang: %q", got)
	}

	// 2. Bukti lengkap: perubahan ke FINAL DITERIMA.
	setCKPNConfig(t, e, keyBANumber, "BA/001/DIR-2026")
	setCKPNConfig(t, e, keyBADate, time.Now().UTC().Format("2006-01-02"))
	setCKPNConfig(t, e, keyApprovedBy, "Direktur A & Akuntan B")
	setCKPNConfig(t, e, keyBasis, "12.6 net flow 3 tahun; 12.7 LGD agunan; lampiran BA/001")
	setCKPNConfig(t, e, keyFromBank, "true")
	if _, err := e.db.ExecContext(e.ctx, `UPDATE system_config SET value='FINAL' WHERE key=$1`, keyStatus); err != nil {
		t.Fatalf("perubahan ke FINAL dengan bukti lengkap harus DITERIMA: %v", err)
	}
	if got := bacaConfigCKPNCOA(t, e, keyStatus); got != domain.CKPNParameterStatusFinal {
		t.Fatalf("status %q, mau FINAL", got)
	}

	// FINAL -> FINAL idempotent tidak diperiksa ulang.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE system_config SET value='FINAL' WHERE key=$1`, keyStatus); err != nil {
		t.Fatalf("FINAL -> FINAL tidak boleh gagal: %v", err)
	}

	// 3. Balik ke SEMENTARA tidak dihalangi, lalu bukti dihapus: kembali ke FINAL ditolak.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE system_config SET value='SEMENTARA' WHERE key=$1`, keyStatus); err != nil {
		t.Fatalf("balik ke SEMENTARA tidak boleh gagal: %v", err)
	}
	kosongkanBuktiRatifikasi(t, e)
	if _, err := e.db.ExecContext(e.ctx, `UPDATE system_config SET value='FINAL' WHERE key=$1`, keyStatus); err == nil {
		t.Fatal("setelah bukti dihapus, perubahan ke FINAL harus DITOLAK lagi")
	}
}

// Kesiapan mesin: status yang dikembalikan konfigurasi DB menyebut bukti yang kurang.
func TestIntegrasiKesiapanRatifikasiCKPNTerlihat(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanCKPNEnabled(t, e)
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnRatificationMigration)
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	kosongkanBuktiRatifikasi(t, e)
	setCKPNConfig(t, e, keyStatus, domain.CKPNParameterStatusSementara)
	t.Cleanup(func() {
		setCKPNConfig(t, e, keyStatus, domain.CKPNParameterStatusSementara)
		kosongkanBuktiRatifikasi(t, e)
	})

	st := domain.CKPNParametersStatusFromConfig(e.ctx, e.configSvc, time.Now())
	if st.RatificationReady {
		t.Fatalf("bukti kosong tidak boleh siap: %+v", st)
	}
	if len(st.RatificationMissing) == 0 {
		t.Fatal("RatificationMissing wajib menyebut kunci yang kurang")
	}
	if len(st.EnablementGaps) == 0 {
		t.Fatal("EnablementGaps wajib menyebut penahan penyalakan CKPN")
	}
}
