package service_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// Uji integrasi keputusan panel matriks kewenangan & kelengkapan laporan terhadap
// PostgreSQL sungguhan:
//
//   - loans:recover pindah ke pejabat (TELLER/AO ditolak, SUPERVISOR boleh);
//   - rute pembekuan rekening + pemisahan tugas freeze/unfreeze;
//   - pengaju hapus buku tidak boleh menjadi penyetuju;
//   - PPKA umum terhitung dari kredit lancar + penempatan;
//   - penanda kelengkapan laporan KPMM muncul saat komponen belum tersedia.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiPanel -v

// TestIntegrasiPanelRecoverHanyaPejabat membaca izin efektif dari database (bukan
// peta kode) setelah migrasi 000091: TELLER/AO/CS ditolak, pejabat boleh.
func TestIntegrasiPanelRecoverHanyaPejabat(t *testing.T) {
	db, ctx := newPermissionDB(t)
	cases := []struct {
		role domain.StaffRole
		want bool
	}{
		{domain.RoleTeller, false},
		{domain.RoleAO, false},
		{domain.RoleCS, false},
		{domain.RoleSupervisor, true},
		{domain.RoleAdmin, true},
		{domain.RoleSuperAdmin, true},
	}
	for _, c := range cases {
		t.Run(string(c.role), func(t *testing.T) {
			_, sid := seedTempUser(t, db, ctx, c.role)
			perms, _ := identityPermissions(t, db, ctx, sid)
			has := false
			for _, p := range perms {
				if p == domain.PermLoansRecover {
					has = true
				}
			}
			if has != c.want {
				t.Fatalf("%s punya loans:recover = %v, ingin %v (izin=%v)", c.role, has, c.want, perms)
			}
		})
	}
}

// TestIntegrasiPanelPembekuanPemisahanTugas membuktikan rute pembekuan berjalan
// (status ACTIVE -> FROZEN, teraudit), hanya ACTIVE yang boleh dibekukan, dan
// pembekuan tidak dapat dibatalkan oleh pelaksana yang sama.
func TestIntegrasiPanelPembekuanPemisahanTugas(t *testing.T) {
	e := newActorBranchEnv(t)
	freezer := findingsSuperadmin(e)
	cust := seedFindingsCustomer(t, e, "Nasabah Pembekuan Panel")

	product, err := e.money.productRepo.GetByCode(e.money.ctx, "TAB-CONV")
	if err != nil {
		t.Fatalf("membaca produk tabungan: %v", err)
	}
	acc, err := e.accountSvc.OpenAccount(e.money.ctx, domain.OpenAccountInput{
		CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR",
	}, freezer)
	if err != nil {
		t.Fatalf("membuka rekening: %v", err)
	}

	// Tanpa rute ini, pembekuan hanya bisa lewat SQL dan pelaksananya tidak tercatat.
	frozen, err := e.accountSvc.FreezeAccount(e.money.ctx, acc.AccountNumber, "pembekuan uji panel", freezer)
	if err != nil {
		t.Fatalf("membekukan rekening aktif: %v", err)
	}
	if frozen.Status != domain.AccountStatusFrozen {
		t.Fatalf("status = %q, ingin FROZEN", frozen.Status)
	}
	if frozen.FrozenBy == nil || *frozen.FrozenBy != freezer.UserID {
		t.Fatalf("frozen_by = %v, ingin pelaksana %s", frozen.FrozenBy, freezer.UserID)
	}
	var action string
	if err := e.money.db.QueryRowContext(e.money.ctx, `
		SELECT action FROM audit_logs
		WHERE resource_type='account' AND resource_id=$1 AND action='FREEZE_ACCOUNT'
		ORDER BY created_at DESC LIMIT 1`, acc.ID).Scan(&action); err != nil {
		t.Fatalf("audit FREEZE_ACCOUNT tidak ditemukan: %v", err)
	}

	// Rekening FROZEN tidak dapat dibekukan lagi.
	if _, err := e.accountSvc.FreezeAccount(e.money.ctx, acc.AccountNumber, "", freezer); !errors.Is(err, domain.ErrAccountNotFreezable) {
		t.Fatalf("membekukan rekening FROZEN: err = %v, ingin ErrAccountNotFreezable", err)
	}

	// Pelaksana pembekuan tidak boleh membatalkannya sendiri.
	if _, err := e.accountSvc.UnfreezeAccount(e.money.ctx, acc.AccountNumber, "", freezer); !errors.Is(err, domain.ErrAccountUnfreezeSameActor) {
		t.Fatalf("unfreeze oleh pelaksana sama: err = %v, ingin ErrAccountUnfreezeSameActor", err)
	}

	// Orang kedua (pejabat lain) boleh membatalkan. Kode cabang dibaca dari database
	// karena objek Account hasil buka rekening boleh belum membawa kode cabang.
	var accBranchCode string
	if err := e.money.db.QueryRowContext(e.money.ctx, `
		SELECT COALESCE((SELECT b.code FROM branches b WHERE b.id = a.branch_id), '')
		FROM accounts a WHERE a.id = $1`, acc.ID).Scan(&accBranchCode); err != nil {
		t.Fatalf("membaca cabang rekening: %v", err)
	}
	checkerID, _ := seedTempUser(t, e.money.db, e.money.ctx, domain.RoleSupervisor)
	second := domain.Actor{
		UserID: checkerID, Username: "pejabat.panel", Role: domain.RoleSupervisor,
		BranchCode: accBranchCode, Book: acc.COABook,
	}
	unfrozen, err := e.accountSvc.UnfreezeAccount(e.money.ctx, acc.AccountNumber, "dibatalkan pejabat lain", second)
	if err != nil {
		t.Fatalf("unfreeze oleh orang kedua harus boleh: %v", err)
	}
	if unfrozen.Status != domain.AccountStatusActive {
		t.Fatalf("status setelah unfreeze = %q, ingin ACTIVE", unfrozen.Status)
	}
}

// TestIntegrasiPanelHapusBukuPengajuBukanPenyetuju membuktikan alur maker-checker
// menolak pengaju menyetujui hapus buku yang dia ajukan sendiri.
func TestIntegrasiPanelHapusBukuPengajuBukanPenyetuju(t *testing.T) {
	e := newBookWriteEnv(t)
	loan, actor := woffLoanOf(t, e, "WP", 12_000_000)
	woffSetState(t, e, loan.ID, string(domain.CollectibilityKol5), 12_000_000)
	// Ambang 0 = setiap hapus buku wajib disetujui pejabat kedua (bawaan 000029).
	woffSetThreshold(t, e, "loan_write_off", "0")

	_, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{
		LoanID: loan.ID, Reason: "debitor pailit", CollectionEfforts: "somasi + kunjungan",
	}, actor)
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("hapus buku harus menunggu persetujuan, dapat: %v", err)
	}
	if pending.ActionType != service.ActionLoanWriteOff {
		t.Fatalf("jenis aksi %s, ingin LOAN_WRITE_OFF", pending.ActionType)
	}

	err = e.mcSvc.Approve(e.ctx, pending.RequestID, actor, "coba setujui sendiri")
	if !errors.Is(err, domain.ErrCannotSelfApprove) {
		t.Fatalf("pengaju menyetujui hapus bukunya sendiri: err = %v, ingin ErrCannotSelfApprove", err)
	}
	if n := woffJournalCount(t, e, "WOFF-", loan.LoanNumber); n != 0 {
		t.Fatalf("jurnal hapus buku %d, ingin 0 sebelum disetujui pejabat lain", n)
	}
}

// TestIntegrasiPanelPPKAUmumTerhitung menghitung PPKA umum dari kredit lancar dan
// penempatan pada bank lain, lalu membandingkannya dengan hitungan tangan:
// kredit 10.000.000 x 0,5% = 50.000; penempatan 2.000.000 x 0,5% = 10.000;
// total 60.000. Cabang unik memisahkan kredit/penempatan uji dari data lain.
func TestIntegrasiPanelPPKAUmumTerhitung(t *testing.T) {
	e := newBookWriteEnv(t)

	// Saklar modul penempatan Pasal 23 dinyalakan selama uji, lalu dipulihkan.
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('ppap.lps.enabled', 'true', 'uji integrasi panel')
		ON CONFLICT (key) DO UPDATE SET value = 'true'`); err != nil {
		t.Fatalf("menyalakan saklar penempatan: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `UPDATE system_config SET value='false' WHERE key='ppap.lps.enabled'`)
		e.configSvc.Invalidate("ppap.lps.enabled")
	})
	e.configSvc.Invalidate("ppap.lps.enabled")

	loan, actor := woffLoanOf(t, e, "PQ", 10_000_000)
	_ = loan

	// Penempatan pada bank lain di cabang uji yang sama, dijamin LPS 0.
	branchCode := actor.BranchCode
	var branchID string
	if err := e.db.QueryRowContext(e.ctx, `SELECT id::text FROM branches WHERE code=$1`, branchCode).Scan(&branchID); err != nil {
		t.Fatalf("membaca cabang uji: %v", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO lps_placements (coa_code, counterparty_bank, placement_type, outstanding, lps_guaranteed, collectibility, as_of, branch_id)
		VALUES ('10200', 'Bank Lawan Uji', 'DEPOSITO', 2000000, 0, 'LANCAR', CURRENT_DATE, $1::uuid)`, branchID); err != nil {
		t.Fatalf("menyisipkan penempatan uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx, `DELETE FROM lps_placements WHERE counterparty_bank='Bank Lawan Uji'`)
	})

	lpsSvc := service.NewLPSPlacementService(postgres.NewLPSPlacementRepository(e.db), e.configSvc)
	ppkaSvc := service.NewPPKAUmumService(e.ppapSvc, lpsSvc, e.configSvc)

	asOf := time.Now()
	got, err := ppkaSvc.Hitung(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Hitung PPKA umum: %v", err)
	}
	if !got.Lengkap {
		t.Fatalf("PPKA umum harus lengkap, alasan=%v", got.AlasanTidakLengkap)
	}

	// Hitungan tangan: jumlahkan eksposur kredit Lancar yang benar-benar dibaca,
	// lalu kalikan tarif 0,5%. Penempatan uji 2.000.000 x 0,5% = 10.000.
	preview, err := e.ppapSvc.Preview(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Preview PPAP: %v", err)
	}
	var manualKredit, manualPenempatan decimal.Decimal
	foundLoan := false
	for _, item := range preview.Items {
		if item.Collectibility != domain.KolLancar {
			continue
		}
		manualKredit = manualKredit.Add(item.Exposure)
		if item.LoanNumber == loan.LoanNumber {
			foundLoan = true
		}
	}
	lpsSummary, err := lpsSvc.Calculate(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Calculate penempatan: %v", err)
	}
	for _, item := range lpsSummary.Items {
		if item.Collectibility == domain.KolLancar {
			manualPenempatan = manualPenempatan.Add(item.Base)
		}
	}
	manualKreditPPKA := domain.RoundToRupiah(manualKredit.Mul(decimal.NewFromFloat(0.005)))
	manualPenempatanPPKA := lpsSummary.TotalGeneralPPKA
	manualTotal := manualKreditPPKA.Add(manualPenempatanPPKA)

	if !foundLoan {
		t.Fatalf("kredit uji %s tidak ikut dibaca pada cabang uji %s", loan.LoanNumber, actor.BranchCode)
	}
	if !got.KreditLancarPPKA.Equal(manualKreditPPKA) {
		t.Fatalf("PPKA umum kredit sistem = %s, hitungan tangan %s", got.KreditLancarPPKA, manualKreditPPKA)
	}
	if !got.PenempatanLancarPPKA.Equal(manualPenempatanPPKA) {
		t.Fatalf("PPKA umum penempatan sistem = %s, hitungan tangan %s", got.PenempatanLancarPPKA, manualPenempatanPPKA)
	}
	if !got.TotalPPKA.Equal(manualTotal) {
		t.Fatalf("PPKA umum total sistem = %s, hitungan tangan %s (kredit %s + penempatan %s)",
			got.TotalPPKA, manualTotal, manualKreditPPKA, manualPenempatanPPKA)
	}
	// Penempatan uji tunggal harus menyumbang 10.000 (2.000.000 x 0,5%).
	if !manualPenempatanPPKA.Equal(decimal.NewFromInt(10000)) {
		t.Fatalf("PPKA umum penempatan = %s, mau 10000", manualPenempatanPPKA)
	}
	t.Logf("PPKA umum: sistem=%s (kredit %s + penempatan %s); manual=%s (kredit %s + penempatan %s)",
		got.TotalPPKA, got.KreditLancarPPKA, got.PenempatanLancarPPKA,
		manualTotal, manualKreditPPKA, manualPenempatanPPKA)
}

// TestIntegrasiPanelKPMMPenandaKelengkapan memastikan laporan KPMM dari jalur nyata
// menampilkan lengkap=false + alasan saat komponen modal belum tersedia, tanpa
// mengubah angka. (Bentuk lengkap=true diuji di lapisan fungsi; saat ini komponen
// instrumen/surplus revaluasi memang belum dapat dipisah dari data, sehingga laporan
// belum boleh disebut final.)
func TestIntegrasiPanelKPMMPenandaKelengkapan(t *testing.T) {
	e := newBookWriteEnv(t)
	reportSvc := service.NewReportService(postgres.NewReportRepository(e.db))
	lpsSvc := service.NewLPSPlacementService(postgres.NewLPSPlacementRepository(e.db), e.configSvc)
	ppkaSvc := service.NewPPKAUmumService(e.ppapSvc, lpsSvc, e.configSvc)
	kpmmSvc := service.NewKPMMService(reportSvc, e.ckpnSvc, e.configSvc, ppkaSvc)

	actor := domain.Actor{UserID: e.actor.UserID, Username: "panel.kpmm", Role: domain.RoleSuperAdmin, BranchCode: "001"}
	report, err := kpmmSvc.Hitung(e.ctx, time.Now(), "CONVENTIONAL", actor)
	if err != nil {
		t.Fatalf("Hitung KPMM: %v", err)
	}
	if report.Lengkap {
		t.Fatal("laporan harus ditandai belum lengkap saat komponen modal belum tersedia")
	}
	gabung := strings.Join(report.AlasanTidakLengkap, " | ")
	if !strings.Contains(gabung, "modal pelengkap") {
		t.Fatalf("alasan tidak menyebut modal pelengkap: %q", gabung)
	}
	if len(report.AlasanTidakLengkap) == 0 {
		t.Fatal("alasan tidak boleh kosong saat lengkap=false")
	}
}
