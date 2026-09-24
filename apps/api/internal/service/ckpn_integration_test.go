package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi CKPN terhadap PostgreSQL sungguhan. Mengikuti pola
// moneyflow_integration_test.go: di-skip kecuali CBS_TEST_DB_DSN diisi, sehingga
// `go test ./...` tetap hijau tanpa database.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55432/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiCKPN -v

// TestIntegrasiCKPNPerbandinganDanPenyimpanan memakai cabang uji tersendiri dan
// aktor yang terbatas pada cabang itu. Isolasi ini membuat ModalIntiDeduction dapat
// diuji EKSAK (bukan GreaterThanOrEqual yang bisa lolos palsu) sekaligus membuktikan
// filter cabang pada pembacaan kredit benar-benar berlaku.
func TestIntegrasiCKPNPerbandinganDanPenyimpanan(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanConfigCKPN(t, e)

	setConfig := func(key, value, desc string) {
		t.Helper()
		if _, err := e.db.ExecContext(e.ctx, `
			INSERT INTO system_config (key, value, description)
			VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value, desc); err != nil {
			t.Fatalf("menyetel konfigurasi %s: %v", key, err)
		}
		// Cache konfigurasi berlaku 60 detik; buang agar nilai baru langsung terbaca.
		e.configSvc.Invalidate(key)
	}

	// Parameter kebijakan dibaca dari konfigurasi, bukan dari angka di kode.
	setConfig("ckpn.enabled", "true", "uji integrasi CKPN")
	setConfig("ckpn.pd_frac.gol_3", "0.10", "uji integrasi CKPN")
	setConfig("ckpn.lgd_frac", "0.50", "uji integrasi CKPN")

	// Cabang uji sendiri supaya kredit dari uji lain (cabang 001) tidak masuk ringkasan.
	branchCode := ckpnTestBranchCode("C")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujickpn", Role: domain.RoleAdmin, BranchCode: branchCode}

	// Kredit nyata lewat jalur produksi (pengajuan, persetujuan, pencairan).
	cust := e.newCustomer(t, "Nasabah CKPN", fmt.Sprintf("ckpn-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)
	if loan.BranchCode != branchCode {
		t.Fatalf("cabang kredit %q, mau %q", loan.BranchCode, branchCode)
	}

	// PPKA tidak dihitung ulang di uji ini: state kredit dipaksa seperti hasil jalur
	// PPAP. Fokus uji adalah perbandingan CKPN terhadap PPKA yang sudah tersimpan.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility = '3_KURANG_LANCAR', dpd = 100, required_ppap = $1
		WHERE id = $2`, decimal.NewFromInt(1_000_000), loan.ID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}

	ckpnSvc := newCKPNSvcForTest(e)
	// Titik waktu uji ditetapkan eksplisit di tengah hari UTC, bukan time.Now() mentah:
	// gerbang PPAP segar membandingkan TANGGAL, sehingga pada dini hari asOf.Add(-time.Hour)
	// melintasi tengah malam dan ditolak ErrCKPNStalePPAP. Lihat ckpnTestAsOf.
	asOf := ckpnTestAsOf()
	// Gerbang tanggal bisnis CKPN menuntut penanda run PPAP pada tanggal yang sama.
	// Uji ini menyuntik required_ppap langsung, jadi penandanya ditulis langsung.
	e.recordPPAPRun(t, asOf)

	// Compare lebih dulu: sebagai baca-saja ia harus menghormati cabang aktor, jadi
	// kredit cabang 001 dari uji lain tidak boleh muncul.
	comparison, err := ckpnSvc.Compare(e.ctx, asOf.Add(-time.Hour), actor)
	if err != nil {
		t.Fatalf("Compare CKPN: %v", err)
	}
	if comparison.Total != 1 || len(comparison.Items) != 1 {
		t.Fatalf("filter cabang gagal: total=%d items=%d (harus hanya kredit cabang uji)", comparison.Total, len(comparison.Items))
	}

	summary, err := ckpnSvc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}
	item := ckpnItem(t, summary, loan.ID)
	// 10.000.000 x 10% x 50% = 500.000.
	if !item.CKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("CKPN %s, mau 500000", item.CKPN)
	}
	if !item.PPKA.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("PPKA %s, mau 1000000", item.PPKA)
	}
	if item.Larger != domain.CKPNLargerPPKA || !item.Difference.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("perbandingan salah: larger=%s difference=%s", item.Larger, item.Difference)
	}
	// Eksak: hanya satu kredit di cabang uji, jadi pengurang modal inti harus persis
	// selisih kredit itu. GreaterThanOrEqual akan meloloskan perhitungan ganda.
	if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("pengurang modal inti %s, mau tepat 500000", summary.ModalIntiDeduction)
	}

	// Target CKPN tersimpan dan terbaca kembali sama.
	var stored decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `SELECT required_ckpn FROM loans WHERE id=$1`, loan.ID).Scan(&stored); err != nil {
		t.Fatalf("membaca required_ckpn: %v", err)
	}
	if !stored.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("required_ckpn tersimpan %s, mau 500000", stored)
	}

	// Jurnal pembentukan memakai COA CKPN (50301/10950), bukan COA PPAP, dan
	// diatribusikan ke cabang KREDIT (kebetulan sama dengan cabang aktor di sini).
	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(500_000))
	assertCKPNJournal(t, e, key, "50301", "DEBIT", "10950", "CREDIT", decimal.NewFromInt(500_000))
	if bc := e.ckpnJournalBranch(t, key); bc != branchCode {
		t.Fatalf("cabang jurnal pembentukan %q, mau cabang kredit %q", bc, branchCode)
	}

	// Run ulang tanggal yang sama: selisih nol, tidak ada jurnal kedua.
	if _, err := ckpnSvc.Run(e.ctx, asOf, actor); err != nil {
		t.Fatalf("Run CKPN kedua: %v", err)
	}
	if n := countJournals(t, e, key); n != 1 {
		t.Fatalf("jurnal idempoten harus tetap 1, dapat %d", n)
	}

	// CKPN turun: PD diubah ke 5% sehingga target 250.000 < 500.000 yang sudah diakui.
	// Selisihnya harus diposting sebagai pemulihan (debit CKPN, kredit beban).
	setConfig("ckpn.pd_frac.gol_3", "0.05", "uji integrasi CKPN")
	asOf2 := asOf.Add(24 * time.Hour)
	e.recordPPAPRun(t, asOf2)
	if _, err := ckpnSvc.Run(e.ctx, asOf2, actor); err != nil {
		t.Fatalf("Run CKPN pemulihan: %v", err)
	}
	if err := e.db.QueryRowContext(e.ctx, `SELECT required_ckpn FROM loans WHERE id=$1`, loan.ID).Scan(&stored); err != nil {
		t.Fatalf("membaca required_ckpn setelah pemulihan: %v", err)
	}
	if !stored.Equal(decimal.NewFromInt(250_000)) {
		t.Fatalf("required_ckpn setelah pemulihan %s, mau 250000", stored)
	}
	keyReversal := ckpnIdempotencyKey(loan.LoanNumber, asOf2, decimal.NewFromInt(-250_000))
	assertCKPNJournal(t, e, keyReversal, "10950", "DEBIT", "50301", "CREDIT", decimal.NewFromInt(250_000))
}

// TestIntegrasiCKPNPelepasanCadanganKreditLunas membuktikan K2: kredit yang sudah
// tidak aktif (PAID_OFF) tetapi masih menyimpan required_ckpn ikut dibaca dan
// cadangannya dilepas lewat pemulihan. Sekaligus membuktikan jurnal memakai cabang
// KREDIT, bukan cabang aktor: aktor lintas cabang di kantor pusat (001) menjalankan
// kredit di cabang uji.
func TestIntegrasiCKPNPelepasanCadanganKreditLunas(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanConfigCKPN(t, e)
	// Uji ini menuntut jalur TULIS (menjurnal pelepasan). Pada database segar
	// ckpn.enabled=false dan mode bayangan tidak pernah menjurnal, jadi parameternya
	// disiapkan sendiri — bukan mengandalkan sisa uji lain — dan dipulihkan setelahnya.
	siapkanParameterCKPNTulis(t, e)

	branchCode := ckpnTestBranchCode("D")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN Lunas")
	cust := e.newCustomer(t, "Nasabah CKPN Lunas", fmt.Sprintf("ckpn-lunas-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	// Aktor lintas cabang (BranchCode berbeda dari cabang kredit) sengaja dipakai agar
	// atribusi cabang jurnal dapat diuji: harus cabang kredit, bukan cabang aktor.
	loan := e.disburseAs(t, e.actor, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)
	if loan.BranchCode != branchCode {
		t.Fatalf("cabang kredit %q, mau %q", loan.BranchCode, branchCode)
	}

	// Kredit sudah lunas: tidak punya eksposur, tetapi masih menyimpan cadangan.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status = 'PAID_OFF', outstanding_principal = 0, dpd = 0,
			collectibility = '1_LANCAR', required_ckpn = 500000
		WHERE id = $1`, loan.ID); err != nil {
		t.Fatalf("menyetel kredit lunas: %v", err)
	}

	ckpnSvc := newCKPNSvcForTest(e)
	asOf := time.Now().UTC()
	e.recordPPAPRun(t, asOf)
	summary, err := ckpnSvc.Run(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}
	// Kredit lunas harus ikut terdaftar (K2) dan targetnya nol.
	item := ckpnItem(t, summary, loan.ID)
	if !item.CKPN.IsZero() {
		t.Fatalf("CKPN kredit lunas %s, mau 0", item.CKPN)
	}
	// Pemulihan 500.000: debit CKPN (10950), kredit beban (50301).
	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(-500_000))
	assertCKPNJournal(t, e, key, "10950", "DEBIT", "50301", "CREDIT", decimal.NewFromInt(500_000))

	if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.IsZero() {
		t.Fatalf("required_ckpn kredit lunas %s, mau nol setelah pelepasan", got)
	}
	// C3: jurnal memakai cabang kredit, bukan cabang aktor (e.actor.BranchCode = 001).
	if bc := e.ckpnJournalBranch(t, key); bc != branchCode {
		t.Fatalf("cabang jurnal pelepasan %q, mau cabang kredit %q (bukan cabang aktor)", bc, branchCode)
	}
}

// TestIntegrasiCKPNSnapshotBasiTidakDipakai membuktikan C1: angka dari snapshot di
// luar transaksi tidak boleh dipakai. Repo dipaksa mengembalikan snapshot lama
// (kredit masih DISBURSED dengan pokok penuh), padahal baris kredit di database sudah
// dibatalkan sebelum kunci diambil. Tanpa perbaikan, kredit itu tetap dijurnal dan
// required_ckpn-nya ditulis dari angka basi.
func TestIntegrasiCKPNSnapshotBasiTidakDipakai(t *testing.T) {
	e := newMoneyEnv(t)
	simpanPulihkanConfigCKPN(t, e)
	// Sama seperti uji pelepasan: jalur tulis disiapkan sendiri agar tidak bergantung
	// pada ckpn.enabled yang mungkin ditinggalkan uji lain.
	siapkanParameterCKPNTulis(t, e)

	branchCode := ckpnTestBranchCode("E")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN Basi")
	cust := e.newCustomer(t, "Nasabah CKPN Basi", fmt.Sprintf("ckpn-basi-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, e.actor, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)

	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility = '3_KURANG_LANCAR', dpd = 100, required_ppap = 1000000
		WHERE id = $1`, loan.ID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}

	// Snapshot basi diambil dari repo nyata SEBELUM kredit dibatalkan.
	realRepo := postgres.NewCKPNRepository(e.db)
	all, err := realRepo.ListActiveLoans(e.ctx, e.actor)
	if err != nil {
		t.Fatalf("membaca snapshot basi: %v", err)
	}
	var stale []domain.CKPNLoanSnapshot
	for _, s := range all {
		if s.LoanID == loan.ID {
			stale = append(stale, s)
		}
	}
	if len(stale) != 1 {
		t.Fatalf("snapshot kredit uji %d, mau 1", len(stale))
	}

	// Kredit dibatalkan di sela baca dan tulis: keluar dari daftar aktif, tanpa cadangan.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET status = 'CANCELLED', outstanding_principal = 0 WHERE id = $1`, loan.ID); err != nil {
		t.Fatalf("membatalkan kredit: %v", err)
	}

	svc := newCKPNSvcWithRepo(e, &staleCKPNRepo{CKPNRepository: realRepo, stale: stale})
	asOf := time.Now().UTC()
	e.recordPPAPRun(t, asOf)
	if _, err := svc.Run(e.ctx, asOf, e.actor); err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}

	// Tidak boleh ada jurnal pembentukan dari angka basi.
	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(500_000))
	if n := countJournals(t, e, key); n != 0 {
		t.Fatalf("kredit yang sudah dibatalkan tidak boleh dijurnal CKPN, dapat %d jurnal", n)
	}
	if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.IsZero() {
		t.Fatalf("required_ckpn kredit batal %s, mau nol (tidak boleh ditulis dari snapshot basi)", got)
	}
}

// staleCKPNRepo mengembalikan snapshot lama apa adanya, meniru keadaan yang berubah
// setelah daftar dibaca. Penulisan state tetap memakai repo asli.
type staleCKPNRepo struct {
	*postgres.CKPNRepository
	stale []domain.CKPNLoanSnapshot
}

func (r *staleCKPNRepo) ListActiveLoans(context.Context, domain.Actor) ([]domain.CKPNLoanSnapshot, error) {
	return r.stale, nil
}

// ckpnTestAsOf mengembalikan satu titik waktu eksplisit di TENGAH HARI UTC pada tanggal
// kalender hari ini. Uji CKPN memakainya alih-alih time.Now() mentah karena gerbang
// requireFreshPPAP membandingkan TANGGAL bisnis: pada dini hari UTC, asOf.Add(-time.Hour)
// (dipakai uji perbandingan) melintasi tengah malam sehingga tanggalnya berbeda dari
// penanda run PPAP dan Compare gagal dengan ErrCKPNStalePPAP. Tengah hari selalu berada
// satu hari bisnis yang sama dengan pergeseran ±1 jam, jadi hasil uji tidak bergantung
// pada jam dinding.
func ckpnTestAsOf() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
}

// recordPPAPRun menulis penanda tanggal bisnis run PPAP agar gerbang tanggal CKPN
// lolos. Uji CKPN menyuntik required_ppap langsung, jadi penandanya pun ditulis
// langsung untuk tanggal bisnis yang sedang diuji.
func (e *moneyEnv) recordPPAPRun(t *testing.T, asOf time.Time) {
	t.Helper()
	e.simpanPulihkanConfigKunci(t, "ppap.last_run_business_date")
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('ppap.last_run_business_date', $1, 'penanda run PPAP uji CKPN')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`,
		asOf.UTC().Format("2006-01-02")); err != nil {
		t.Fatalf("menulis penanda run PPAP: %v", err)
	}
}

// simpanPulihkanConfigCKPN mengunci parameter CKPN yang lazim diubah uji integrasi ini
// dan memulihkannya setelah selesai. Tanpa ini, uji yang menyalakan ckpn.enabled atau
// mengubah PD/LGD bocor ke uji lain di database yang sama sehingga hasil suite bergantung
// urutan (mis. uji pelepasan hanya lulus bila berjalan setelah uji perbandingan).
func simpanPulihkanConfigCKPN(t *testing.T, e *moneyEnv) {
	t.Helper()
	simpanPulihkanConfig(t, e,
		"ckpn.enabled",
		"ckpn.shadow_mode.enabled",
		"ckpn.pd_frac.gol_3",
		"ckpn.lgd_frac",
		"ppap.last_run_business_date",
	)
}

// siapkanParameterCKPNTulis menyalakan jalur tulis CKPN (ckpn.enabled) beserta PD/LGD
// yang valid untuk uji integrasi yang menjurnal. Nilainya dipulihkan oleh
// simpanPulihkanConfigCKPN, sehingga tidak mengubah konfigurasi setelah uji selesai.
func siapkanParameterCKPNTulis(t *testing.T, e *moneyEnv) {
	t.Helper()
	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd_frac.gol_3", "0.10")
	setCKPNConfig(t, e, "ckpn.lgd_frac", "0.50")
}

// setCKPNConfig menyetel satu kunci CKPN sambil membuang cache konfigurasinya. Nilai
// lama (termasuk kunci yang belum ada) dipulihkan lewat t.Cleanup saat pertama kali
// kunci itu disentuh, sehingga uji tidak lagi menebak nilai semula.
func setCKPNConfig(t *testing.T, e *moneyEnv, key, value string) {
	t.Helper()
	e.simpanPulihkanConfigKunci(t, key)
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi CKPN')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

// ckpnTestBranchCode menghasilkan kode cabang unik (maks 8 karakter) agar uji yang
// diulang pada database yang sama tidak menumpuk kredit di cabang yang sama.
func ckpnTestBranchCode(prefix string) string {
	return (prefix + uuid.New().String())[:8]
}

// ensureBranch memastikan cabang dengan kode tertentu ada, lalu mengembalikan id-nya.
func (e *moneyEnv) ensureBranch(t *testing.T, code, name string) uuid.UUID {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO branches (id, code, name, is_head_office)
		VALUES ($1, $2, $3, false)
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name`,
		uuid.New(), code, name); err != nil {
		t.Fatalf("menyiapkan cabang %s: %v", code, err)
	}
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `SELECT id FROM branches WHERE code = $1`, code).Scan(&id); err != nil {
		t.Fatalf("membaca cabang %s: %v", code, err)
	}
	return id
}

// newAccountInBranch menyisipkan rekening tabungan nyata pada COA 20100 milik nasabah
// di cabang tertentu, sehingga kredit yang dicairkan mewarisi cabang tersebut.
func (e *moneyEnv) newAccountInBranch(t *testing.T, customerID, branchID uuid.UUID) uuid.UUID {
	t.Helper()
	accountNumber := fmt.Sprintf("E2E-CKPN-%d", time.Now().UnixNano())
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `
		INSERT INTO accounts (account_number, coa_id, account_type, status, balance, available_balance, currency, customer_id, branch_id)
		VALUES ($1, (SELECT id FROM chart_of_accounts WHERE code = '20100'),
		        'SAVINGS', 'ACTIVE', 0, 0, 'IDR', $2, $3)
		RETURNING id`, accountNumber, customerID, branchID).Scan(&id); err != nil {
		t.Fatalf("menyiapkan rekening uji: %v", err)
	}
	return id
}

// disburseAs menjalankan siklus produksi penuh dengan aktor tertentu.
func (e *moneyEnv) disburseAs(t *testing.T, actor domain.Actor, customerID, accountID uuid.UUID, amount decimal.Decimal, term int) *domain.Loan {
	t.Helper()
	product, err := e.productRepo.GetByCode(e.ctx, "KRD-FLAT")
	if err != nil {
		t.Fatalf("membaca produk kredit: %v", err)
	}
	loan, err := e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
		CustomerID:            customerID,
		ProductID:             product.ID,
		DisbursementAccountID: accountID,
		PrincipalAmount:       amount,
		TermMonths:            term,
		Purpose:               "uji integrasi CKPN",
	}, actor)
	if err != nil {
		t.Fatalf("pengajuan kredit: %v", err)
	}
	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, actor); err != nil {
		t.Fatalf("persetujuan kredit: %v", err)
	}
	disbursed, err := e.loanSvc.DisburseLoan(e.ctx, loan.ID, actor)
	if err != nil {
		t.Fatalf("pencairan kredit: %v", err)
	}
	return disbursed
}

// ckpnJournalBranch membaca KODE cabang yang tercatat pada jurnal dengan kunci
// idempotensi itu. journal_entries menyimpan branch_id, jadi kode diambil lewat join.
func (e *moneyEnv) ckpnJournalBranch(t *testing.T, idempotencyKey string) string {
	t.Helper()
	var branch string
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT COALESCE(b.code, '')
		FROM journal_entries je
		LEFT JOIN branches b ON b.id = je.branch_id
		WHERE je.idempotency_key = $1`, idempotencyKey).Scan(&branch); err != nil {
		t.Fatalf("membaca cabang jurnal: %v", err)
	}
	return branch
}

func newCKPNSvcForTest(e *moneyEnv) domain.CKPNService {
	return newCKPNSvcWithRepo(e, postgres.NewCKPNRepository(e.db))
}

func newCKPNSvcWithRepo(e *moneyEnv, repo domain.CKPNRepository) domain.CKPNService {
	ledgerRepo := postgres.NewLedgerRepository(e.db)
	accountRepo := postgres.NewAccountRepository(e.db)
	dateRepo := postgres.NewBusinessDateRepository(e.db)
	referenceGen := postgres.NewReferenceGenerator(e.db)
	postingSvc := service.NewPostingService(e.db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(e.productRepo, ledgerRepo, postingSvc)
	return service.NewCKPNService(e.db, repo, e.productRepo, ledgerRepo, poster, postingSvc, e.configSvc, e.loanRepo,
		service.NewPPAPRunMarker(postgres.NewSystemConfigRepository(e.db)))
}

func ckpnIdempotencyKey(loanNumber string, asOf time.Time, adjustment decimal.Decimal) string {
	return fmt.Sprintf("CKPN-%s-%s-%s", loanNumber, asOf.Format("2006-01-02"), adjustment.String())
}

func ckpnItem(t *testing.T, summary domain.CKPNComparisonSummary, loanID uuid.UUID) domain.CKPNComparisonItem {
	t.Helper()
	for _, it := range summary.Items {
		if it.LoanID == loanID {
			return it
		}
	}
	t.Fatalf("kredit %s tidak ada di hasil CKPN (failures: %+v)", loanID, summary.Failures)
	return domain.CKPNComparisonItem{}
}

func countJournals(t *testing.T, e *moneyEnv, idempotencyKey string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM journal_entries WHERE idempotency_key=$1`, idempotencyKey).Scan(&n); err != nil {
		t.Fatalf("menghitung jurnal: %v", err)
	}
	return n
}

// assertCKPNJournal memastikan jurnal itu memuat TEPAT DUA baris: satu debit pada
// debitCOA dan satu kredit pada creditCOA, masing-masing senilai amount. Penghitungan
// per baris (bukan peta kode|arah) dipakai agar baris kembar pada akun yang sama tidak
// lolos: peta akan menggabungkannya menjadi satu kunci.
func assertCKPNJournal(t *testing.T, e *moneyEnv, idempotencyKey, debitCOA, debitDir, creditCOA, creditDir string, amount decimal.Decimal) {
	t.Helper()
	rows, err := e.db.QueryContext(e.ctx, `
		SELECT coa.code, jl.direction::text, jl.amount
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		JOIN accounts a ON a.id = jl.account_id
		JOIN chart_of_accounts coa ON coa.id = a.coa_id
		WHERE je.idempotency_key = $1`, idempotencyKey)
	if err != nil {
		t.Fatalf("membaca baris jurnal: %v", err)
	}
	defer func() { _ = rows.Close() }()

	type line struct {
		code   string
		dir    string
		amount decimal.Decimal
	}
	var lines []line
	for rows.Next() {
		var l line
		if err := rows.Scan(&l.code, &l.dir, &l.amount); err != nil {
			t.Fatalf("memindai baris jurnal: %v", err)
		}
		lines = append(lines, l)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("baris jurnal: %v", err)
	}

	if len(lines) != 2 {
		t.Fatalf("jurnal %s punya %d baris, mau tepat 2: %+v", idempotencyKey, len(lines), lines)
	}

	debitKey := debitCOA + "|" + debitDir
	creditKey := creditCOA + "|" + creditDir
	seen := map[string]decimal.Decimal{}
	for _, l := range lines {
		key := l.code + "|" + l.dir
		if key != debitKey && key != creditKey {
			t.Fatalf("baris jurnal tak terduga %s senilai %s pada jurnal %s", key, l.amount, idempotencyKey)
		}
		if prev, dup := seen[key]; dup {
			t.Fatalf("baris jurnal %s muncul lebih dari sekali (nilai %s dan %s)", key, prev, l.amount)
		}
		seen[key] = l.amount
	}
	if amt, ok := seen[debitKey]; !ok || !amt.Equal(amount) {
		t.Fatalf("baris debit %s = %v, mau %s", debitKey, seen[debitKey], amount)
	}
	if amt, ok := seen[creditKey]; !ok || !amt.Equal(amount) {
		t.Fatalf("baris kredit %s = %v, mau %s", creditKey, seen[creditKey], amount)
	}
}
