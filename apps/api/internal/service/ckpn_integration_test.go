package service_test

import (
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
func TestIntegrasiCKPNPerbandinganDanPenyimpanan(t *testing.T) {
	e := newMoneyEnv(t)

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
	setConfig("ckpn.pd.3", "0.10", "uji integrasi CKPN")
	setConfig("ckpn.lgd", "0.50", "uji integrasi CKPN")

	// Kredit nyata lewat jalur produksi (pengajuan, persetujuan, pencairan).
	cust := e.newCustomer(t, "Nasabah CKPN", fmt.Sprintf("ckpn-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)

	// PPKA tidak dihitung ulang di uji ini: state kredit dipaksa seperti hasil jalur
	// PPAP. Fokus uji adalah perbandingan CKPN terhadap PPKA yang sudah tersimpan.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility = '3_KURANG_LANCAR', dpd = 100, required_ppap = $1
		WHERE id = $2`, decimal.NewFromInt(1_000_000), loan.ID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}

	ckpnSvc := newCKPNSvcForTest(e)
	asOf := time.Now().UTC()

	summary, err := ckpnSvc.Run(e.ctx, asOf, e.actor)
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
	if !summary.ModalIntiDeduction.GreaterThanOrEqual(decimal.NewFromInt(500_000)) {
		t.Fatalf("pengurang modal inti %s, minimal 500000", summary.ModalIntiDeduction)
	}

	// Target CKPN tersimpan dan terbaca kembali sama.
	var stored decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `SELECT required_ckpn FROM loans WHERE id=$1`, loan.ID).Scan(&stored); err != nil {
		t.Fatalf("membaca required_ckpn: %v", err)
	}
	if !stored.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("required_ckpn tersimpan %s, mau 500000", stored)
	}

	// Jurnal pembentukan memakai COA CKPN (50301/10950), bukan COA PPAP.
	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(500_000))
	assertCKPNJournal(t, e, key, "50301", "DEBIT", "10950", "CREDIT", decimal.NewFromInt(500_000))

	// Run ulang tanggal yang sama: selisih nol, tidak ada jurnal kedua.
	if _, err := ckpnSvc.Run(e.ctx, asOf, e.actor); err != nil {
		t.Fatalf("Run CKPN kedua: %v", err)
	}
	if n := countJournals(t, e, key); n != 1 {
		t.Fatalf("jurnal idempoten harus tetap 1, dapat %d", n)
	}

	// CKPN turun: PD diubah ke 5% sehingga target 250.000 < 500.000 yang sudah diakui.
	// Selisihnya harus diposting sebagai pemulihan (debit CKPN, kredit beban).
	setConfig("ckpn.pd.3", "0.05", "uji integrasi CKPN")
	asOf2 := asOf.Add(24 * time.Hour)
	if _, err := ckpnSvc.Run(e.ctx, asOf2, e.actor); err != nil {
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

func newCKPNSvcForTest(e *moneyEnv) domain.CKPNService {
	ledgerRepo := postgres.NewLedgerRepository(e.db)
	accountRepo := postgres.NewAccountRepository(e.db)
	dateRepo := postgres.NewBusinessDateRepository(e.db)
	referenceGen := postgres.NewReferenceGenerator(e.db)
	postingSvc := service.NewPostingService(e.db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(e.productRepo, ledgerRepo, postingSvc)
	return service.NewCKPNService(e.db, postgres.NewCKPNRepository(e.db), e.productRepo, ledgerRepo, poster, postingSvc, e.configSvc, e.loanRepo)
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

// assertCKPNJournal memastikan ada jurnal dengan dua baris: akun debit dan akun kredit
// yang diminta, masing-masing senilai amount.
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
	defer rows.Close()

	got := map[string]decimal.Decimal{}
	for rows.Next() {
		var code, dir string
		var amt decimal.Decimal
		if err := rows.Scan(&code, &dir, &amt); err != nil {
			t.Fatalf("memindai baris jurnal: %v", err)
		}
		got[code+"|"+dir] = amt
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("baris jurnal: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("jurnal %s punya %d baris, mau 2: %v", idempotencyKey, len(got), got)
	}
	if amt, ok := got[debitCOA+"|"+debitDir]; !ok || !amt.Equal(amount) {
		t.Fatalf("baris debit %s|%s = %v, mau %s", debitCOA, debitDir, amt, amount)
	}
	if amt, ok := got[creditCOA+"|"+creditDir]; !ok || !amt.Equal(amount) {
		t.Fatalf("baris kredit %s|%s = %v, mau %s", creditCOA, creditDir, amt, amount)
	}
}
