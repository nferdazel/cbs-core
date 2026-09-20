package service_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi ini menjalankan jalur posting dan pembatalan pada database PostgreSQL
// sungguhan. Yang tidak dapat dibuktikan stub adalah hal-hal yang hanya ada di database:
// enum REVERSAL pada transaction_type, kolom source, pengembalian saldo akun oleh posting
// engine, dan penandaan status jurnal asal.
//
// Di-skip kecuali CBS_TEST_DB_DSN diisi, supaya `go test ./...` tetap hijau tanpa database:
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55432/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run Integrasi -v
func TestIntegrasiPembatalanTransaksi(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi dilewati")
	}

	ctx := context.Background()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("membuka database: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database tidak dapat dihubungi: %v", err)
	}

	// Tanggal bisnis disamakan dengan hari ini agar jalur "same-day" yang diuji pasti
	// jalur langsung, bukan jalur persetujuan.
	hariIni := time.Now().UTC().Format("2006-01-02")
	// Baris tanggal bisnis dibuat saat pertama kali dipakai (EOD), jadi pada database yang
	// baru dimigrasi kuncinya belum ada dan harus disisipkan — bukan di-update. Bila baris
	// ini tidak ada, seluruh pembatalan dianggap lintas hari dan wajib persetujuan: arah
	// degradasinya aman, tetapi perilaku same-day tidak akan pernah terpakai.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('system.business_date', $1, 'tanggal bisnis untuk uji integrasi')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, hariIni); err != nil {
		t.Fatalf("menyetel tanggal bisnis: %v", err)
	}

	ledgerRepo := postgres.NewLedgerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	productRepo := postgres.NewProductRepository(db)
	configRepo := postgres.NewSystemConfigRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	configSvc := service.NewSystemConfigService(configRepo)
	referenceGen := postgres.NewReferenceGenerator(db)
	postingSvc := service.NewPostingService(db, ledgerRepo, accountRepo, ledgerRepo, referenceGen)
	limitSvc := service.NewTransactionLimitService(configSvc, ledgerRepo)
	executors := service.NewExecutorRegistry()
	mcSvc := service.NewMakerCheckerService(db, postgres.NewMakerCheckerRepository(db), auditRepo, configSvc, executors)
	ledgerSvc := service.NewLedgerService(db, ledgerRepo, accountRepo, productRepo, ledgerRepo, postingSvc, configSvc, limitSvc, mcSvc,
		postgres.NewBusinessDateRepository(db), auditRepo)

	// Rekening tabungan uji pada COA 20100 (simpanan pihak ketiga), akun kewajiban yang
	// sudah pasti ada karena dipakai jurnal setoran.
	accountNumber := fmt.Sprintf("E2E-%d", time.Now().UnixNano())
	var accountID uuid.UUID
	if err := db.QueryRowContext(ctx, `
		INSERT INTO accounts (account_number, coa_id, account_type, status, balance, available_balance, currency)
		VALUES ($1, (SELECT id FROM chart_of_accounts WHERE code = '20100'),
		        'SAVINGS', 'ACTIVE', 0, 0, 'IDR')
		RETURNING id`, accountNumber).Scan(&accountID); err != nil {
		t.Fatalf("menyiapkan rekening uji: %v", err)
	}

	// audit_logs memuat FK ke staff_users, jadi aktor uji harus menunjuk baris staff yang
	// benar-benar ada — inilah salah satu hal yang tidak dapat ditiru stub. Dua nama
	// berbeda dipakai agar larangan "pencatat transaksi tidak boleh membatalkan transaksinya
	// sendiri" tidak ikut terpicu.
	var staffID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM staff_users ORDER BY created_at LIMIT 1`).Scan(&staffID); err != nil {
		t.Fatalf("membaca staf yang di-seed: %v", err)
	}
	teller := domain.Actor{UserID: staffID, Username: "teller-inti", Role: domain.RoleTeller}
	supervisor := domain.Actor{UserID: staffID, Username: "supervisor-inti", Role: domain.RoleSupervisor}

	saldoAwal := saldoRekening(ctx, t, db, accountID)
	saldoKasAwal := saldoKas(ctx, t, db)

	deposit, err := ledgerSvc.Deposit(ctx, domain.DepositRequest{
		AccountNumber: accountNumber,
		Amount:        decimal.NewFromInt(250000),
		Currency:      "IDR",
		Description:   "setoran uji integrasi",
		Actor:         teller,
	})
	if err != nil {
		t.Fatalf("setoran uji gagal: %v", err)
	}
	if got := saldoRekening(ctx, t, db, accountID); !got.Equal(saldoAwal.Add(decimal.NewFromInt(250000))) {
		t.Fatalf("saldo setelah setoran %s, mau %s", got, saldoAwal.Add(decimal.NewFromInt(250000)))
	}

	// Pembatalan same-day: harus langsung, tanpa persetujuan.
	reversal, err := ledgerSvc.Reverse(ctx, domain.ReversalRequest{
		Reference: deposit.ReferenceNumber,
		Reason:    "salah rekening tujuan",
		Actor:     supervisor,
	})
	if err != nil {
		t.Fatalf("pembatalan same-day gagal: %v", err)
	}
	if reversal.TransactionType != domain.TxTypeReversal {
		t.Fatalf("jenis jurnal kontra %q, mau REVERSAL", reversal.TransactionType)
	}
	if reversal.Source != domain.SourceTeller {
		t.Fatalf("alur jurnal kontra %q, mau TELLER", reversal.Source)
	}

	// Jurnal asal harus ditandai REVERSED, dan saldo akun kembali seperti semula.
	statusAsal := ""
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM journal_entries WHERE reference_number = $1`,
		deposit.ReferenceNumber).Scan(&statusAsal); err != nil {
		t.Fatalf("membaca status jurnal asal: %v", err)
	}
	if statusAsal != string(domain.JournalStatusReversed) {
		t.Fatalf("status jurnal asal %q, mau REVERSED", statusAsal)
	}
	if got := saldoRekening(ctx, t, db, accountID); !got.Equal(saldoAwal) {
		t.Fatalf("saldo setelah pembatalan %s, mau kembali ke %s", got, saldoAwal)
	}
	if got := saldoKas(ctx, t, db); !got.Equal(saldoKasAwal) {
		t.Fatalf("saldo kas setelah pembatalan %s, mau kembali ke %s", got, saldoKasAwal)
	}

	// Jurnal kontra harus seimbang di database, bukan hanya menurut perhitungan aplikasi.
	var selisih decimal.Decimal
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE -amount END), 0)
		FROM journal_lines WHERE journal_entry_id = $1`, reversal.ID).Scan(&selisih); err != nil {
		t.Fatalf("memeriksa keseimbangan jurnal kontra: %v", err)
	}
	if !selisih.IsZero() {
		t.Fatalf("jurnal kontra tidak seimbang, selisih %s", selisih)
	}
	// Cabang jurnal kontra harus mengikuti jurnal asal, bukan NULL (bank-wide).
	var kontraBranch sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT branch_id::text FROM journal_entries WHERE id = $1`, reversal.ID).Scan(&kontraBranch); err != nil {
		t.Fatalf("membaca cabang jurnal kontra: %v", err)
	}
	var asalBranch sql.NullString
	if err := db.QueryRowContext(ctx,
		`SELECT branch_id::text FROM journal_entries WHERE reference_number = $1`,
		deposit.ReferenceNumber).Scan(&asalBranch); err != nil {
		t.Fatalf("membaca cabang jurnal asal: %v", err)
	}
	if kontraBranch.String != asalBranch.String {
		t.Fatalf("cabang jurnal kontra %v, mau mengikuti jurnal asal %v", kontraBranch, asalBranch)
	}

	// Pembatalan kedua atas jurnal yang sama harus ditolak.
	if _, err := ledgerSvc.Reverse(ctx, domain.ReversalRequest{
		Reference: deposit.ReferenceNumber,
		Reason:    "coba kedua",
		Actor:     supervisor,
	}); !errors.Is(err, domain.ErrJournalAlreadyReversed) {
		t.Fatalf("pembatalan kedua: %v, mau ErrJournalAlreadyReversed", err)
	}

	// Jalur lintas hari: tanggal bisnis dimajukan, jurnal hari ini menjadi jurnal kemarin.
	if _, err := db.ExecContext(ctx,
		`UPDATE system_config SET value = $1 WHERE key = 'system.business_date'`,
		time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")); err != nil {
		t.Fatalf("memajukan tanggal bisnis: %v", err)
	}

	deposit2, err := ledgerSvc.Deposit(ctx, domain.DepositRequest{
		AccountNumber: accountNumber,
		Amount:        decimal.NewFromInt(150000),
		Currency:      "IDR",
		Description:   "setoran uji lintas hari",
		Actor:         teller,
	})
	if err != nil {
		t.Fatalf("setoran uji kedua gagal: %v", err)
	}

	var pending *domain.PendingApprovalError
	if _, err := ledgerSvc.Reverse(ctx, domain.ReversalRequest{
		Reference: deposit2.ReferenceNumber,
		Reason:    "ketahuan besok pagi",
		Actor:     supervisor,
	}); !errors.As(err, &pending) {
		t.Fatalf("pembatalan lintas hari: %v, mau menunggu persetujuan", err)
	}
	if pending.ActionType != service.ActionReverse {
		t.Fatalf("jenis aksi persetujuan %q, mau %q", pending.ActionType, service.ActionReverse)
	}
	// Belum boleh ada jurnal kontra maupun perubahan status sebelum disetujui.
	statusDeposit2 := ""
	if err := db.QueryRowContext(ctx,
		`SELECT status FROM journal_entries WHERE reference_number = $1`,
		deposit2.ReferenceNumber).Scan(&statusDeposit2); err != nil {
		t.Fatalf("membaca status jurnal kedua: %v", err)
	}
	if statusDeposit2 != string(domain.JournalStatusPosted) {
		t.Fatalf("status jurnal kedua %q, mau masih POSTED sebelum disetujui", statusDeposit2)
	}
}

func saldoRekening(ctx context.Context, t *testing.T, db *sql.DB, accountID uuid.UUID) decimal.Decimal {
	t.Helper()
	var saldo decimal.Decimal
	if err := db.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE id = $1`, accountID).Scan(&saldo); err != nil {
		t.Fatalf("membaca saldo rekening: %v", err)
	}
	return saldo
}

// saldoKas membaca saldo akun kas teller, yaitu akun internal untuk COA 10101.
func saldoKas(ctx context.Context, t *testing.T, db *sql.DB) decimal.Decimal {
	t.Helper()
	var saldo decimal.Decimal
	if err := db.QueryRowContext(ctx, `
		SELECT a.balance FROM accounts a
		JOIN chart_of_accounts c ON c.id = a.coa_id
		WHERE c.code = '10101' AND a.account_type = 'INTERNAL_GL'`).Scan(&saldo); err != nil {
		t.Fatalf("membaca saldo kas: %v", err)
	}
	return saldo
}
