package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Uji integrasi pengaman parameter CKPN SEMENTARA (keputusan panel butir 1) terhadap
// PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// ckpn_config_panel_integration_test.go.

const ckpnGuardMigration = "000093_ckpn_parameters_provisional_guard.up.sql"

// finalTanpaLantai mengembalikan konfigurasi DB ke FINAL + lantai mati supaya uji
// lama yang membandingkan model apa adanya tidak terpengaruh. Uji ini sendiri selalu
// menyetel keadaan yang diuji lebih dulu.
func finalTanpaLantai(t *testing.T, e *moneyEnv) {
	t.Helper()
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusFinal)
	setCKPNConfig(t, e, domain.ConfigKeyCKPNFloorPPKA, "false")
}

// Migrasi 000093: bawaan SEMENTARA, batas 12 bulan, lantai true; idempotent; dan
// TIDAK menimpa nilai yang sudah disesuaikan bank.
func TestIntegrasiMigrasiGuardParameterCKPN(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() { finalTanpaLantai(t, e) })

	// Nilai bank TIDAK boleh ditimpa: status FINAL, tanggal khusus, lantai mati.
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, "FINAL")
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersSince, "2020-01-02")
	setCKPNConfig(t, e, domain.ConfigKeyCKPNFloorPPKA, "false")
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnGuardMigration)

	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNParametersStatus); got != "FINAL" {
		t.Fatalf("%s ditimpa menjadi %q, mau tetap FINAL", domain.ConfigKeyCKPNParametersStatus, got)
	}
	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNParametersSince); got != "2020-01-02" {
		t.Fatalf("%s ditimpa menjadi %q, mau tetap 2020-01-02", domain.ConfigKeyCKPNParametersSince, got)
	}
	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNFloorPPKA); got != "false" {
		t.Fatalf("%s ditimpa menjadi %q, mau tetap false", domain.ConfigKeyCKPNFloorPPKA, got)
	}

	// Kunci kosong diisi bawaan: status SEMENTARA, lantai true, bulan 12, tanggal terisi.
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, "")
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersSince, "")
	setCKPNConfig(t, e, domain.ConfigKeyCKPNFloorPPKA, "")
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnGuardMigration)
	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNParametersStatus); got != "SEMENTARA" {
		t.Fatalf("status kosong diisi %q, mau SEMENTARA", got)
	}
	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNFloorPPKA); got != "true" {
		t.Fatalf("lantai kosong diisi %q, mau true", got)
	}
	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNParametersMonths); got != "12" {
		t.Fatalf("bulan ratifikasi %q, mau 12", got)
	}
	since := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNParametersSince)
	if _, err := time.Parse("2006-01-02", since); err != nil {
		t.Fatalf("%s tidak terisi tanggal YYYY-MM-DD: %q", domain.ConfigKeyCKPNParametersSince, since)
	}

	// Idempotent: jalan kedua tidak mengubah tanggal yang sudah terisi.
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnGuardMigration)
	if got := bacaConfigCKPNCOA(t, e, domain.ConfigKeyCKPNParametersSince); got != since {
		t.Fatalf("jalan kedua mengubah tanggal %q menjadi %q", since, got)
	}
}

// Lantai wajib dengan data kredit NYATA: model 100jt x 10% x 45% = 4,5jt, PPKA 10jt.
// Selama SEMENTARA lantai menang (10jt) meski kunci floor=false; setelah FINAL dan
// lantai mati, target kembali model apa adanya (4,5jt).
func TestIntegrasiLantaiPPKAHormatiStatusParameter(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() { finalTanpaLantai(t, e) })

	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, "ckpn.shadow_mode.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd_frac.gol_3", "0.10")
	setCKPNConfig(t, e, "ckpn.lgd_frac", "0.45")
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusSementara)
	setCKPNConfig(t, e, domain.ConfigKeyCKPNFloorPPKA, "false") // bank mencoba mematikan

	branchCode := ckpnTestBranchCode("G")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Lantai CKPN")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujilantai", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah Lantai", "lantai-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(100_000_000), 12)
	e.setCKPNState(t, loan.ID, "3_KURANG_LANCAR", 100, idr(10_000_000))

	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)
	summary, err := newCKPNSvcForTest(e).Compare(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Compare SEMENTARA: %v", err)
	}
	item := ckpnItem(t, summary, loan.ID)
	if item.CKPN.LessThan(item.PPKA) {
		t.Fatalf("CKPN %s < PPKA %s: lantai wajib tidak ditegakkan", item.CKPN, item.PPKA)
	}
	if !item.CKPN.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("target lantai %s, mau 10000000 (PPKA)", item.CKPN)
	}
	if !summary.ParameterSementara || summary.Enabled {
		t.Fatalf("ringkasan harus menandai SEMENTARA dan CKPN resmi mati: %+v", summary)
	}
	if summary.OJKExportBlocked {
		t.Fatalf("CKPN resmi masih mati: ekspor OJK belum boleh diblokir: %+v", summary)
	}

	// CKPN resmi dinyalakan tanpa ratifikasi: angka sementara mengalir ke laporan,
	// sehingga ekspor OJK wajib ditutup.
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	if nyala, err := newCKPNSvcForTest(e).Compare(e.ctx, asOf, actor); err != nil {
		t.Fatalf("Compare CKPN nyala + SEMENTARA: %v", err)
	} else if !nyala.OJKExportBlocked {
		t.Fatalf("CKPN nyala + SEMENTARA harus memblokir ekspor OJK: %+v", nyala)
	}
	setCKPNConfig(t, e, "ckpn.enabled", "false")

	// FINAL + lantai dimatikan bank: model apa adanya.
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusFinal)
	if summary2, err := newCKPNSvcForTest(e).Compare(e.ctx, asOf, actor); err != nil {
		t.Fatalf("Compare FINAL: %v", err)
	} else if got := ckpnItem(t, summary2, loan.ID).CKPN; !got.Equal(decimal.NewFromInt(4_500_000)) {
		t.Fatalf("target FINAL lantai mati %s, mau 4500000 (model)", got)
	}
}

// Status nyata dari konfigurasi DB: SEMENTARA memblokir ekspor OJK HANYA saat CKPN
// resmi menyala; selama CKPN mati laporan OJK memuat PPKA sehingga tidak diblokir.
// FINAL tidak pernah memblokir.
func TestIntegrasiStatusParameterCKPNMemblokirOJK(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() { finalTanpaLantai(t, e) })

	// CKPN mati + SEMENTARA: diperingatkan, tetapi ekspor belum ditutup.
	setCKPNConfig(t, e, "ckpn.enabled", "false")
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusSementara)
	st := domain.CKPNParametersStatusFromConfig(context.Background(), e.configSvc, time.Now())
	if !st.Sementara || st.OJKExportBlocked {
		t.Fatalf("CKPN mati + SEMENTARA tidak boleh memblokir OJK: %+v", st)
	}
	if len(st.Warnings) == 0 {
		t.Fatal("peringatan SEMENTARA wajib tetap ada saat CKPN mati")
	}

	// CKPN menyala + SEMENTARA: ekspor wajib ditutup dengan alasan.
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	st = domain.CKPNParametersStatusFromConfig(context.Background(), e.configSvc, time.Now())
	if !st.Sementara || !st.OJKExportBlocked || st.OJKExportBlockReason == "" {
		t.Fatalf("CKPN menyala + SEMENTARA harus memblokir OJK beralasan: %+v", st)
	}

	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusFinal)
	st = domain.CKPNParametersStatusFromConfig(context.Background(), e.configSvc, time.Now())
	if st.Sementara || st.OJKExportBlocked {
		t.Fatalf("FINAL tidak boleh memblokir OJK: %+v", st)
	}

	// Lewat batas 12 bulan: peringatan tingkat tinggi dengan tanggal nyata.
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusSementara)
	setCKPNConfig(t, e, domain.ConfigKeyCKPNParametersSince, time.Now().UTC().AddDate(0, -13, 0).Format("2006-01-02"))
	st = domain.CKPNParametersStatusFromConfig(context.Background(), e.configSvc, time.Now())
	if !st.DeadlinePassed {
		t.Fatalf("13 bulan harus melewati batas: %+v", st)
	}
}
