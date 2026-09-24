package service_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi jalur uang baru terhadap PostgreSQL sungguhan: pembatalan pencairan
// kredit, koreksi nominal pencairan, run PPAP dengan agunan, dan pencarian nama lewat
// indeks token. Mengikuti pola reversal_integration_test.go: di-skip kecuali
// CBS_TEST_DB_DSN diisi, sehingga `go test ./...` tetap hijau tanpa database.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55432/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiJalurUang -v
//
// Nama uji diawali "IntegrasiJalurUang" agar dapat dijalankan terpisah, tetapi tetap
// ikut terpilih oleh pola "-run Integrasi" yang sudah ada.

var moneyNikCounter int64

// moneyEnv membungkus seluruh grafik service produksi di atas satu koneksi database.
type moneyEnv struct {
	ctx            context.Context
	db             *sql.DB
	actor          domain.Actor
	customerSvc    domain.CustomerService
	loanSvc        domain.LoanService
	ppapSvc        domain.PPAPService
	collateralRepo *postgres.CollateralRepository
	productRepo    *postgres.ProductRepository
	loanRepo       *postgres.LoanRepository
	configSvc      domain.SystemConfigService
	// snapshotKunci mencatat kunci konfigurasi yang sudah di-snapshot uji ini agar
	// setter yang dipanggil berulang tidak menimpa nilai lama dengan nilai antara.
	snapshotKunci map[string]configSnapshot
}

// configSnapshot menyimpan nilai satu kunci system_config sebelum diubah harness,
// termasuk ketiadaan barisnya, agar t.Cleanup dapat memulihkannya persis. Tanpa ini
// kunci yang disetel uji bocor ke uji lain di database yang sama dan hasil suite
// bergantung urutan.
type configSnapshot struct {
	key    string
	value  string
	exists bool
}

// snapshotConfig membaca nilai awal kunci-kunci konfigurasi. Ketiadaan baris dicatat
// sebagai exists=false agar pemulihannya menghapus baris itu, bukan menulis nilai
// kosong yang berbeda.
func snapshotConfig(t *testing.T, db *sql.DB, ctx context.Context, keys ...string) []configSnapshot {
	t.Helper()
	snaps := make([]configSnapshot, 0, len(keys))
	for _, key := range keys {
		var value string
		err := db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value)
		switch {
		case err == sql.ErrNoRows:
			snaps = append(snaps, configSnapshot{key: key})
		case err != nil:
			t.Fatalf("membaca konfigurasi awal %s: %v", key, err)
		default:
			snaps = append(snaps, configSnapshot{key: key, value: value, exists: true})
		}
	}
	return snaps
}

// restoreConfig mengembalikan kunci-kunci yang diubah harness ke nilai semula.
func restoreConfig(t *testing.T, db *sql.DB, ctx context.Context, snaps []configSnapshot) {
	t.Helper()
	for _, s := range snaps {
		if !s.exists {
			if _, err := db.ExecContext(ctx, `DELETE FROM system_config WHERE key = $1`, s.key); err != nil {
				t.Errorf("menghapus konfigurasi uji %s: %v", s.key, err)
			}
			continue
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO system_config (key, value, description)
			VALUES ($1, $2, 'dipulihkan setelah uji integrasi jalur uang')
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, s.key, s.value); err != nil {
			t.Errorf("memulihkan konfigurasi %s: %v", s.key, err)
		}
	}
}

// simpanPulihkanConfigKunci mencatat nilai LAMA satu kunci konfigurasi (termasuk
// ketiadaan barisnya) dan mendaftarkan pemulihannya lewat t.Cleanup, memakai
// snapshotConfig/restoreConfig yang sama dengan harness jalur uang. Idempoten per kunci
// per uji: panggilan berulang untuk kunci yang sama tidak menimpa snapshot pertama,
// sehingga setter yang dipanggil berkali-kali tetap memulihkan nilai sebelum uji,
// bukan nilai antara yang disetel uji sendiri.
func (e *moneyEnv) simpanPulihkanConfigKunci(t *testing.T, key string) {
	t.Helper()
	if e.snapshotKunci == nil {
		e.snapshotKunci = make(map[string]configSnapshot)
	}
	if _, sudah := e.snapshotKunci[key]; sudah {
		return
	}
	snap := snapshotConfig(t, e.db, e.ctx, key)[0]
	e.snapshotKunci[key] = snap
	t.Cleanup(func() { restoreConfig(t, e.db, e.ctx, []configSnapshot{snap}) })
}

// simpanPulihkanConfig mencatat nilai lama sejumlah kunci lalu mendaftarkan
// pemulihannya. Dipakai uji yang mengubah konfigurasi langsung, bukan lewat setter.
func simpanPulihkanConfig(t *testing.T, e *moneyEnv, keys ...string) {
	t.Helper()
	for _, key := range keys {
		e.simpanPulihkanConfigKunci(t, key)
	}
}

func newMoneyEnv(t *testing.T) *moneyEnv {
	t.Helper()
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi jalur uang dilewati")
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("membuka database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database tidak dapat dihubungi: %v", err)
	}

	// Nilai awal kunci yang diubah harness dipulihkan lewat t.Cleanup, sehingga
	// tanggal bisnis dan parameter CKPN uji tidak tertinggal di database bersama.
	// Cleanup didaftarkan setelah Close sehingga berjalan lebih dulu (LIFO).
	awal := snapshotConfig(t, db, ctx,
		"system.business_date",
		"system.business_date_status",
		"ppap.last_run_business_date",
		domain.ConfigKeyCKPNParametersStatus,
		domain.ConfigKeyCKPNFloorPPKA,
		// Bukti ratifikasi (migrasi 000095): trigger menolak status FINAL tanpa bukti,
		// jadi harness harus mengisi bukti uji lebih dulu dan memulihkannya setelahnya.
		domain.ConfigKeyCKPNRatificationBANumber,
		domain.ConfigKeyCKPNRatificationBADate,
		domain.ConfigKeyCKPNRatificationApprovedBy,
		domain.ConfigKeyCKPNRatificationPDLGDBasis,
		domain.ConfigKeyCKPNRatificationPDLGDFromBank,
	)
	t.Cleanup(func() { restoreConfig(t, db, ctx, awal) })

	// Tanggal bisnis = hari ini: seluruh jalur same-day berjalan langsung, bukan
	// jalur persetujuan. Jurnal akan diuji bertanggal bisnis ini.
	hariIni := time.Now().UTC().Format("2006-01-02")
	if _, err := db.ExecContext(ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('system.business_date', $1, 'tanggal bisnis untuk uji integrasi jalur uang')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, hariIni); err != nil {
		t.Fatalf("menyetel tanggal bisnis: %v", err)
	}

	// Bukti ratifikasi uji HARUS diisi sebelum status FINAL: trigger 000095 menolak
	// transisi ke FINAL tanpa bukti. Ini data uji, bukan klaim ratifikasi bank —
	// uji yang menguji penjaga itu mengosongkannya sendiri.
	for _, kv := range [][2]string{
		{domain.ConfigKeyCKPNRatificationBANumber, "BA-UJI/001"},
		{domain.ConfigKeyCKPNRatificationBADate, hariIni},
		{domain.ConfigKeyCKPNRatificationApprovedBy, "Direksi Uji + Akuntan Uji"},
		{domain.ConfigKeyCKPNRatificationPDLGDBasis, "data historis uji (12.6/12.7)"},
		{domain.ConfigKeyCKPNRatificationPDLGDFromBank, "true"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO system_config (key, value, description)
			VALUES ($1, $2, 'bukti ratifikasi uji integrasi')
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, kv[0], kv[1]); err != nil {
			t.Fatalf("menyetel bukti ratifikasi uji %s: %v", kv[0], err)
		}
	}

	// Pengaman parameter CKPN: uji integrasi lama menilai model apa adanya, jadi
	// harness menetapkan parameter FINAL dan lantai PPKA mati. Uji yang menguji
	// pengaman itu sendiri menyetel SEMENTARA/lantainya sendiri (migrasi 000093
	// meng-seed SEMENTARA; lihat ckpn_parameters_guard_integration_test.go).
	for _, kv := range [][2]string{
		{domain.ConfigKeyCKPNParametersStatus, domain.CKPNParameterStatusFinal},
		{domain.ConfigKeyCKPNFloorPPKA, "false"},
	} {
		if _, err := db.ExecContext(ctx, `
			INSERT INTO system_config (key, value, description)
			VALUES ($1, $2, 'harness uji integrasi')
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, kv[0], kv[1]); err != nil {
			t.Fatalf("menyetel konfigurasi %s: %v", kv[0], err)
		}
	}

	// audit_logs memuat FK ke staff_users, jadi aktor uji harus menunjuk baris staf
	// yang benar-benar ada.
	var staffID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM staff_users ORDER BY created_at LIMIT 1`).Scan(&staffID); err != nil {
		t.Fatalf("membaca staf yang di-seed: %v", err)
	}
	actor := domain.Actor{UserID: staffID, Username: "superadmin.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	customerRepo := postgres.NewCustomerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	productRepo := postgres.NewProductRepository(db)
	loanRepo := postgres.NewLoanRepository(db)
	configRepo := postgres.NewSystemConfigRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	dateRepo := postgres.NewBusinessDateRepository(db)
	referenceGen := postgres.NewReferenceGenerator(db)
	collateralRepo := postgres.NewCollateralRepository(db)

	configSvc := service.NewSystemConfigService(configRepo)
	postingSvc := service.NewPostingService(db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(productRepo, ledgerRepo, postingSvc)
	executors := service.NewExecutorRegistry()
	mcSvc := service.NewMakerCheckerService(db, postgres.NewMakerCheckerRepository(db), auditRepo, configSvc, executors, dateRepo, postgres.NewBranchRepository(db))
	loanSvc := service.NewLoanService(db, loanRepo, productRepo, accountRepo, ledgerRepo, poster, postingSvc, referenceGen, configSvc, mcSvc, dateRepo, auditRepo)
	// Koreksi nominal dieksekusi setelah disetujui; daftarkan agar jalur persetujuan
	// punya eksekutor yang sama seperti produksi.
	executors.Register(service.ActionLoanCorrection, loanSvc)
	ppapSvc := service.NewPPAPService(db, postgres.NewPPAPRepository(db), productRepo, ledgerRepo, poster, postingSvc, configSvc, service.NewPPAPRunMarker(configRepo), collateralRepo)

	cipher, err := crypto.NewCipher("e2e", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), nil)
	if err != nil {
		t.Fatalf("membangun cipher: %v", err)
	}
	customerSvc := service.NewCustomerService(db, customerRepo, cipher, referenceGen, auditRepo)

	return &moneyEnv{
		ctx:            ctx,
		db:             db,
		actor:          actor,
		customerSvc:    customerSvc,
		loanSvc:        loanSvc,
		ppapSvc:        ppapSvc,
		collateralRepo: collateralRepo,
		productRepo:    productRepo,
		loanRepo:       loanRepo,
		configSvc:      configSvc,
	}
}

func idr(n int64) decimal.Decimal { return decimal.NewFromInt(n) }

// newCustomer mendaftarkan nasabah lewat layanan produksi (nama terenkripsi + token).
// NIK dibangkitkan unik agar pemeriksaan duplikat tidak menolak uji yang berulang.
func (e *moneyEnv) newCustomer(t *testing.T, fullName, email string) *domain.Customer {
	t.Helper()
	nik := fmt.Sprintf("%016d", (time.Now().UnixNano()+atomic.AddInt64(&moneyNikCounter, 1))%10_000_000_000_000_000)
	cust, err := e.customerSvc.RegisterCustomer(e.ctx, domain.CreateCustomerInput{
		FullName:     fullName,
		IDCardNumber: nik,
		Email:        email,
	}, e.actor)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah %q: %v", fullName, err)
	}
	return cust
}

// newAccount menyisipkan rekening tabungan nyata pada COA 20100 milik nasabah.
func (e *moneyEnv) newAccount(t *testing.T, customerID uuid.UUID) uuid.UUID {
	t.Helper()
	accountNumber := fmt.Sprintf("E2E-%d", time.Now().UnixNano())
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `
		INSERT INTO accounts (account_number, coa_id, account_type, status, balance, available_balance, currency, customer_id, branch_id)
		VALUES ($1, (SELECT id FROM chart_of_accounts WHERE code = '20100'),
		        'SAVINGS', 'ACTIVE', 0, 0, 'IDR', $2, (SELECT id FROM branches WHERE code = '001'))
		RETURNING id`, accountNumber, customerID).Scan(&id); err != nil {
		t.Fatalf("menyiapkan rekening uji: %v", err)
	}
	return id
}

// disburse menjalankan siklus produksi penuh: pengajuan, persetujuan, lalu pencairan.
// Jurnal pencairan dan jadwal angsuran ditulis oleh jalur yang sama dengan produksi.
func (e *moneyEnv) disburse(t *testing.T, customerID, accountID uuid.UUID, amount decimal.Decimal, term int) *domain.Loan {
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
		Purpose:               "uji integrasi jalur uang",
	}, e.actor)
	if err != nil {
		t.Fatalf("pengajuan kredit: %v", err)
	}
	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, e.actor); err != nil {
		t.Fatalf("persetujuan kredit: %v", err)
	}
	disbursed, err := e.loanSvc.DisburseLoan(e.ctx, loan.ID, e.actor)
	if err != nil {
		t.Fatalf("pencairan kredit: %v", err)
	}
	return disbursed
}

// ---------- pembaca database langsung ----------

func (e *moneyEnv) loanStatus(t *testing.T, loanID uuid.UUID) string {
	t.Helper()
	var status string
	if err := e.db.QueryRowContext(e.ctx, `SELECT status FROM loans WHERE id = $1`, loanID).Scan(&status); err != nil {
		t.Fatalf("membaca status kredit: %v", err)
	}
	return status
}

func (e *moneyEnv) loanDecimal(t *testing.T, loanID uuid.UUID, column string) decimal.Decimal {
	t.Helper()
	var v decimal.Decimal
	// column berasal dari kode uji, bukan masukan pengguna.
	if err := e.db.QueryRowContext(e.ctx, `SELECT `+column+` FROM loans WHERE id = $1`, loanID).Scan(&v); err != nil {
		t.Fatalf("membaca %s kredit: %v", column, err)
	}
	return v
}

func (e *moneyEnv) scheduleCount(t *testing.T, loanID uuid.UUID) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx, `SELECT COUNT(*) FROM loan_schedules WHERE loan_id = $1`, loanID).Scan(&n); err != nil {
		t.Fatalf("menghitung jadwal: %v", err)
	}
	return n
}

func (e *moneyEnv) schedules(t *testing.T, loanID uuid.UUID) []domain.LoanSchedule {
	t.Helper()
	rows, err := e.db.QueryContext(e.ctx, `
		SELECT id, loan_id, installment_no, due_date, principal_amount, profit_amount, total_installment,
		       paid_principal, paid_profit, profit_type, outstanding_principal, status, paid_at, created_at,
		       profit_accrued_at, profit_accrued_amount
		FROM loan_schedules WHERE loan_id = $1 ORDER BY installment_no`, loanID)
	if err != nil {
		t.Fatalf("membaca jadwal: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []domain.LoanSchedule
	for rows.Next() {
		var s domain.LoanSchedule
		if err := rows.Scan(&s.ID, &s.LoanID, &s.InstallmentNo, &s.DueDate, &s.PrincipalAmount, &s.ProfitAmount,
			&s.TotalInstallment, &s.PaidPrincipal, &s.PaidProfit, &s.ProfitType, &s.OutstandingPrincipal,
			&s.Status, &s.PaidAt, &s.CreatedAt, &s.ProfitAccruedAt, &s.ProfitAccruedAmount); err != nil {
			t.Fatalf("memindai jadwal: %v", err)
		}
		out = append(out, s)
	}
	return out
}

func (e *moneyEnv) schedule(t *testing.T, loanID uuid.UUID, installmentNo int) domain.LoanSchedule {
	t.Helper()
	for _, s := range e.schedules(t, loanID) {
		if s.InstallmentNo == installmentNo {
			return s
		}
	}
	t.Fatalf("angsuran ke-%d kredit %s tidak ditemukan", installmentNo, loanID)
	return domain.LoanSchedule{}
}

// journalByKey mengembalikan id dan nomor referensi jurnal dengan kunci idempotensi.
func (e *moneyEnv) journalByKey(t *testing.T, key string) (uuid.UUID, string) {
	t.Helper()
	var id uuid.UUID
	var ref string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT id, reference_number FROM journal_entries WHERE idempotency_key = $1`, key).Scan(&id, &ref); err != nil {
		t.Fatalf("membaca jurnal dengan kunci %q: %v", key, err)
	}
	return id, ref
}

func (e *moneyEnv) journalIDByRef(t *testing.T, ref string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `SELECT id FROM journal_entries WHERE reference_number = $1`, ref).Scan(&id); err != nil {
		t.Fatalf("membaca jurnal %q: %v", ref, err)
	}
	return id
}

func (e *moneyEnv) countJournalsByKey(t *testing.T, key string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM journal_entries WHERE idempotency_key = $1`, key).Scan(&n); err != nil {
		t.Fatalf("menghitung jurnal kunci %q: %v", key, err)
	}
	return n
}

// journalImbalance menjumlahkan DEBIT - CREDIT langsung dari baris jurnal di database.
func (e *moneyEnv) journalImbalance(t *testing.T, journalID uuid.UUID) decimal.Decimal {
	t.Helper()
	var selisih decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT COALESCE(SUM(CASE WHEN direction = 'DEBIT' THEN amount ELSE -amount END), 0)
		FROM journal_lines WHERE journal_entry_id = $1`, journalID).Scan(&selisih); err != nil {
		t.Fatalf("memeriksa keseimbangan jurnal: %v", err)
	}
	return selisih
}

// journalLineRow adalah satu baris jurnal sebagaimana tersimpan di database.
type journalLineRow struct {
	AccountNumber string
	Direction     string
	Amount        decimal.Decimal
}

// journalLines membaca baris satu jurnal langsung dari database dengan urutan stabil.
func (e *moneyEnv) journalLines(t *testing.T, journalID uuid.UUID) []journalLineRow {
	t.Helper()
	rows, err := e.db.QueryContext(e.ctx, `
		SELECT a.account_number, l.direction::text, l.amount
		FROM journal_lines l
		JOIN accounts a ON a.id = l.account_id
		WHERE l.journal_entry_id = $1
		ORDER BY a.account_number, l.direction, l.amount`, journalID)
	if err != nil {
		t.Fatalf("membaca baris jurnal: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []journalLineRow
	for rows.Next() {
		var l journalLineRow
		if err := rows.Scan(&l.AccountNumber, &l.Direction, &l.Amount); err != nil {
			t.Fatalf("memindai baris jurnal: %v", err)
		}
		out = append(out, l)
	}
	return out
}

// oppositeDirection membalik arah baris untuk jurnal kontra.
func oppositeDirection(direction string) string {
	if direction == string(domain.DirectionDebit) {
		return string(domain.DirectionCredit)
	}
	return string(domain.DirectionDebit)
}

// assertLinesMirror menuntut setiap baris jurnal pertama punya tepat satu baris lawan
// dengan akun sama, arah berlawanan, dan nominal sama. Keseimbangan saja tidak cukup:
// kontra yang tergandakan dua kali tetap seimbang, tetapi tidak mencerminkan jurnal asal.
func assertLinesMirror(t *testing.T, label string, original, counterpart []journalLineRow) {
	t.Helper()
	if len(original) != len(counterpart) {
		t.Fatalf("%s: %d baris, mau %d (mencerminkan %d baris asal); asal=%+v kontra=%+v",
			label, len(counterpart), len(original), len(original), original, counterpart)
	}
	want := map[string]int{}
	for _, l := range original {
		key := l.AccountNumber + "|" + oppositeDirection(l.Direction) + "|" + l.Amount.String()
		want[key]++
	}
	for _, l := range counterpart {
		key := l.AccountNumber + "|" + l.Direction + "|" + l.Amount.String()
		if want[key] == 0 {
			t.Fatalf("%s: baris %s tidak mencerminkan jurnal asal (akun/arah/nominal salah); asal=%+v kontra=%+v",
				label, key, original, counterpart)
		}
		want[key]--
	}
}

// assertCorrectionLines memeriksa akun, arah, dan nominal jurnal koreksi terhadap
// jurnal pencairan asal. Koreksi naik mengikuti arah asal; koreksi turun membalik arah.
func assertCorrectionLines(t *testing.T, label string, original, correction []journalLineRow, amount decimal.Decimal, increase bool) {
	t.Helper()
	if len(correction) != len(original) {
		t.Fatalf("%s: %d baris, mau %d; asal=%+v koreksi=%+v", label, len(correction), len(original), original, correction)
	}
	want := map[string]int{}
	for _, l := range original {
		dir := l.Direction
		if !increase {
			dir = oppositeDirection(dir)
		}
		key := l.AccountNumber + "|" + dir + "|" + amount.String()
		want[key]++
	}
	for _, l := range correction {
		key := l.AccountNumber + "|" + l.Direction + "|" + l.Amount.String()
		if want[key] == 0 {
			t.Fatalf("%s: baris %s tidak sesuai arah/nominal yang diharapkan; asal=%+v koreksi=%+v",
				label, key, original, correction)
		}
		want[key]--
	}
}

func (e *moneyEnv) loanString(t *testing.T, loanID uuid.UUID, column string) string {
	t.Helper()
	var v string
	// column berasal dari kode uji, bukan masukan pengguna.
	if err := e.db.QueryRowContext(e.ctx, `SELECT `+column+` FROM loans WHERE id = $1`, loanID).Scan(&v); err != nil {
		t.Fatalf("membaca %s kredit: %v", column, err)
	}
	return v
}

func (e *moneyEnv) loanInt(t *testing.T, loanID uuid.UUID, column string) int {
	t.Helper()
	var v int
	if err := e.db.QueryRowContext(e.ctx, `SELECT `+column+` FROM loans WHERE id = $1`, loanID).Scan(&v); err != nil {
		t.Fatalf("membaca %s kredit: %v", column, err)
	}
	return v
}

func (e *moneyEnv) loanNullableTime(t *testing.T, loanID uuid.UUID, column string) sql.NullTime {
	t.Helper()
	var v sql.NullTime
	if err := e.db.QueryRowContext(e.ctx, `SELECT `+column+` FROM loans WHERE id = $1`, loanID).Scan(&v); err != nil {
		t.Fatalf("membaca %s kredit: %v", column, err)
	}
	return v
}

// setPenaltyRatePerMille menyetel tarif denda harian agar jalur akrual denda berjalan.
func (e *moneyEnv) setPenaltyRatePerMille(t *testing.T, rate int) {
	t.Helper()
	const key = "loan.penalty.rate.daily.per_mille"
	e.simpanPulihkanConfigKunci(t, key)
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'tarif denda untuk uji integrasi')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, strconv.Itoa(rate)); err != nil {
		t.Fatalf("menyetel tarif denda: %v", err)
	}
	e.configSvc.Invalidate(key)
}

func (e *moneyEnv) journalBranch(t *testing.T, ref string) sql.NullString {
	t.Helper()
	var branch sql.NullString
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT branch_id::text FROM journal_entries WHERE reference_number = $1`, ref).Scan(&branch); err != nil {
		t.Fatalf("membaca cabang jurnal %q: %v", ref, err)
	}
	return branch
}

func (e *moneyEnv) accountBalance(t *testing.T, accountID uuid.UUID) decimal.Decimal {
	t.Helper()
	var saldo decimal.Decimal
	if err := e.db.QueryRowContext(e.ctx, `SELECT balance FROM accounts WHERE id = $1`, accountID).Scan(&saldo); err != nil {
		t.Fatalf("membaca saldo rekening: %v", err)
	}
	return saldo
}

// ---------- 1. pembatalan pencairan kredit ----------

func TestIntegrasiJalurUangPembatalanPencairanKredit(t *testing.T) {
	e := newMoneyEnv(t)
	cust := e.newCustomer(t, "Budi Santoso", "")
	accID := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, accID, idr(10_000_000), 4)

	_, disbursementRef := e.journalByKey(t, "DISB-"+loan.LoanNumber)

	cancelled, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{
		LoanID: loan.ID, Reason: "salah nasabah",
	}, e.actor)
	if err != nil {
		t.Fatalf("pembatalan pencairan: %v", err)
	}
	if cancelled.Status != domain.LoanStatusCancelled {
		t.Fatalf("status hasil %q, mau CANCELLED", cancelled.Status)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusCancelled) {
		t.Fatalf("status tersimpan %q, mau CANCELLED", got)
	}
	// Jadwal angsuran terhapus: kredit yang batal tidak pernah ada sebagai tagihan.
	if got := e.scheduleCount(t, loan.ID); got != 0 {
		t.Fatalf("jadwal angsuran tersisa %d, mau 0", got)
	}
	// Sisa pokok nol.
	if got := e.loanDecimal(t, loan.ID, "outstanding_principal"); !got.IsZero() {
		t.Fatalf("sisa pokok %s, mau 0", got)
	}
	// Jurnal asal ditandai REVERSED.
	var statusAsal string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT status FROM journal_entries WHERE reference_number = $1`, disbursementRef).Scan(&statusAsal); err != nil {
		t.Fatalf("membaca status jurnal asal: %v", err)
	}
	if statusAsal != string(domain.JournalStatusReversed) {
		t.Fatalf("status jurnal asal %q, mau REVERSED", statusAsal)
	}
	// Jurnal kontra seimbang di database, bukan hanya menurut aplikasi.
	revID, revRef := e.journalByKey(t, "REV-"+disbursementRef)
	if imb := e.journalImbalance(t, revID); !imb.IsZero() {
		t.Fatalf("jurnal kontra tidak seimbang, selisih %s", imb)
	}
	// Setiap baris kontra harus mencerminkan nominal baris jurnal asal dengan arah
	// berlawanan. Kontra yang tergandakan dua kali tetap seimbang, tetapi tidak
	// mencerminkan jurnal asal; asersi ini menutup celah itu.
	assertLinesMirror(t, "jurnal kontra pembatalan",
		e.journalLines(t, e.journalIDByRef(t, disbursementRef)),
		e.journalLines(t, revID))
	// Cabang kontra mewarisi jurnal asal.
	if kontra, asal := e.journalBranch(t, revRef), e.journalBranch(t, disbursementRef); kontra.String != asal.String {
		t.Fatalf("cabang jurnal kontra %v, mau mengikuti jurnal asal %v", kontra, asal)
	}
	// Pembatalan kedua ditolak: status sudah bukan DISBURSED.
	if _, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{
		LoanID: loan.ID, Reason: "coba kedua",
	}, e.actor); !errors.Is(err, domain.ErrLoanNotCancellable) {
		t.Fatalf("pembatalan kedua: %v, mau ErrLoanNotCancellable", err)
	}

	// Pembatalan ditolak bila sudah ada angsuran dibayar.
	cust2 := e.newCustomer(t, "Dewi Lestari", "")
	acc2 := e.newAccount(t, cust2.ID)
	loan2 := e.disburse(t, cust2.ID, acc2, idr(8_000_000), 4)
	if _, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{
		LoanID: loan2.ID, InstallmentNo: 1,
	}, e.actor); err != nil {
		t.Fatalf("membayar angsuran uji: %v", err)
	}
	if _, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{
		LoanID: loan2.ID, Reason: "sudah ada angsuran",
	}, e.actor); !errors.Is(err, domain.ErrLoanHasInstallmentPayments) {
		t.Fatalf("pembatalan setelah angsuran dibayar: %v, mau ErrLoanHasInstallmentPayments", err)
	}
	if got := e.loanStatus(t, loan2.ID); got != string(domain.LoanStatusDisbursed) {
		t.Fatalf("status kredit berubah meski pembatalan ditolak: %q", got)
	}
}

// ---------- 2. koreksi nominal pencairan ----------

func TestIntegrasiJalurUangKoreksiNominalPencairan(t *testing.T) {
	e := newMoneyEnv(t)
	cust := e.newCustomer(t, "Citra Dewi", "")
	accID := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, accID, idr(10_000_000), 4)
	disbID, _ := e.journalByKey(t, "DISB-"+loan.LoanNumber)
	disbLines := e.journalLines(t, disbID)
	if got := e.accountBalance(t, accID); !got.Equal(idr(10_000_000)) {
		t.Fatalf("saldo rekening setelah pencairan %s, mau 10000000", got)
	}

	if _, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{
		LoanID: loan.ID, InstallmentNo: 1,
	}, e.actor); err != nil {
		t.Fatalf("membayar angsuran uji: %v", err)
	}
	paidBefore := e.schedule(t, loan.ID, 1)

	// Koreksi naik.
	if _, err := e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
		LoanID: loan.ID, NewAmount: idr(12_000_000), Reason: "salah input nominal",
	}, e.actor); err != nil {
		t.Fatalf("koreksi naik: %v", err)
	}
	afterUp, err := e.loanRepo.GetByID(e.ctx, loan.ID)
	if err != nil {
		t.Fatalf("membaca kredit setelah koreksi: %v", err)
	}
	if !afterUp.PrincipalAmount.Equal(idr(12_000_000)) {
		t.Fatalf("nominal kredit %s, mau 12000000", afterUp.PrincipalAmount)
	}
	// Jadwal dihitung ulang: total pokok seluruh angsuran = nominal baru.
	totalPrincipal := decimal.Zero
	for _, s := range e.schedules(t, loan.ID) {
		totalPrincipal = totalPrincipal.Add(s.PrincipalAmount)
	}
	if !totalPrincipal.Equal(idr(12_000_000)) {
		t.Fatalf("total pokok jadwal %s, mau 12000000", totalPrincipal)
	}
	// Sisa pokok = nominal baru - pokok yang sudah dibayar.
	wantOutstanding := idr(12_000_000).Sub(paidBefore.PaidPrincipal)
	if !afterUp.OutstandingPrincipal.Equal(wantOutstanding) {
		t.Fatalf("sisa pokok %s, mau %s", afterUp.OutstandingPrincipal, wantOutstanding)
	}
	// Jurnal selisih seimbang, diperiksa langsung di database.
	// Kunci memuat urutan koreksi per kredit; koreksi naik ini yang pertama.
	corrKey := "CORR-" + loan.LoanNumber + "-12000000-1"
	corrID, _ := e.journalByKey(t, corrKey)
	if imb := e.journalImbalance(t, corrID); !imb.IsZero() {
		t.Fatalf("jurnal koreksi naik tidak seimbang, selisih %s", imb)
	}
	// Akun, arah, dan nominal tiap baris jurnal koreksi harus persis mengikuti jurnal
	// pencairan asal dengan nominal sebesar selisih. Saldo rekening nasabah ikut bergerak
	// sebesar selisih yang dikredit ke rekeningnya.
	assertCorrectionLines(t, "jurnal koreksi naik", disbLines, e.journalLines(t, corrID), idr(2_000_000), true)
	wantUpBalance := idr(12_000_000).Sub(paidBefore.PaidPrincipal).Sub(paidBefore.PaidProfit)
	if got := e.accountBalance(t, accID); !got.Equal(wantUpBalance) {
		t.Fatalf("saldo rekening setelah koreksi naik %s, mau %s", got, wantUpBalance)
	}
	// Angsuran yang sudah dibayar tidak berubah.
	paidAfter := e.schedule(t, loan.ID, 1)
	if paidAfter.ID != paidBefore.ID ||
		!paidAfter.PrincipalAmount.Equal(paidBefore.PrincipalAmount) ||
		!paidAfter.ProfitAmount.Equal(paidBefore.ProfitAmount) ||
		!paidAfter.PaidPrincipal.Equal(paidBefore.PaidPrincipal) ||
		!paidAfter.PaidProfit.Equal(paidBefore.PaidProfit) ||
		paidAfter.Status != paidBefore.Status {
		t.Fatalf("angsuran lunas berubah:\nsebelum %+v\nsesudah %+v", paidBefore, paidAfter)
	}
	// Pengulangan nominal yang sama ditolak dan tidak menggandakan jurnal.
	if _, err := e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
		LoanID: loan.ID, NewAmount: idr(12_000_000), Reason: "ulang",
	}, e.actor); !errors.Is(err, domain.ErrLoanAmountUnchanged) {
		t.Fatalf("pengulangan koreksi: %v, mau ErrLoanAmountUnchanged", err)
	}
	if n := e.countJournalsByKey(t, corrKey); n != 1 {
		t.Fatalf("jumlah jurnal kunci %q = %d, mau 1 (idempotensi)", corrKey, n)
	}

	// Koreksi turun: arah baris jurnal dibalik.
	if _, err := e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
		LoanID: loan.ID, NewAmount: idr(8_000_000), Reason: "kelebihan pencairan",
	}, e.actor); err != nil {
		t.Fatalf("koreksi turun: %v", err)
	}
	downKey := "CORR-" + loan.LoanNumber + "-8000000-2"
	downID, _ := e.journalByKey(t, downKey)
	if imb := e.journalImbalance(t, downID); !imb.IsZero() {
		t.Fatalf("jurnal koreksi turun tidak seimbang, selisih %s", imb)
	}
	// Koreksi turun 12jt -> 8jt: arah baris dibalik dari pencairan asal, nominal 4jt.
	assertCorrectionLines(t, "jurnal koreksi turun", disbLines, e.journalLines(t, downID), idr(4_000_000), false)
	wantDownBalance := idr(8_000_000).Sub(paidBefore.PaidPrincipal).Sub(paidBefore.PaidProfit)
	if got := e.accountBalance(t, accID); !got.Equal(wantDownBalance) {
		t.Fatalf("saldo rekening setelah koreksi turun %s, mau %s", got, wantDownBalance)
	}
	afterDown, err := e.loanRepo.GetByID(e.ctx, loan.ID)
	if err != nil {
		t.Fatalf("membaca kredit setelah koreksi turun: %v", err)
	}
	if !afterDown.OutstandingPrincipal.Equal(idr(8_000_000).Sub(paidBefore.PaidPrincipal)) {
		t.Fatalf("sisa pokok setelah turun %s, mau %s",
			afterDown.OutstandingPrincipal, idr(8_000_000).Sub(paidBefore.PaidPrincipal))
	}
}

// Kunci idempotensi koreksi memuat nominal baru. Bila kredit dikoreksi kembali ke
// nominal yang pernah dipakai, kunci lama bertabrakan. Yang diuji: berapa pun jalurnya,
// saldo rekening nasabah (kaki jurnal pencairan) harus selalu sama dengan pokok kredit.
func TestIntegrasiJalurUangKoreksiKembaliKeNominalSebelumnya(t *testing.T) {
	e := newMoneyEnv(t)
	cust := e.newCustomer(t, "Eka Putra", "")
	accID := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, accID, idr(10_000_000), 4)

	// 10 juta -> 12 juta -> 10 juta -> 12 juta lagi.
	for _, step := range []struct {
		amount int64
		reason string
	}{
		{12_000_000, "naik pertama"},
		{10_000_000, "kembali ke nominal awal"},
		{12_000_000, "naik kedua ke nominal yang sama"},
	} {
		if _, err := e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
			LoanID: loan.ID, NewAmount: idr(step.amount), Reason: step.reason,
		}, e.actor); err != nil {
			t.Fatalf("koreksi ke %d (%s): %v", step.amount, step.reason, err)
		}
	}

	after, err := e.loanRepo.GetByID(e.ctx, loan.ID)
	if err != nil {
		t.Fatalf("membaca kredit: %v", err)
	}
	if !after.PrincipalAmount.Equal(idr(12_000_000)) {
		t.Fatalf("nominal kredit %s, mau 12000000", after.PrincipalAmount)
	}
	if got := e.accountBalance(t, accID); !got.Equal(after.PrincipalAmount) {
		t.Fatalf("saldo rekening nasabah %s tidak sama dengan pokok kredit %s: "+
			"koreksi berulang memakai kunci idempotensi yang sama sehingga jurnal selisih tidak ditulis, "+
			"padahal state kredit berubah", got, after.PrincipalAmount)
	}
}

// ---------- 3. run PPAP dengan agunan ----------

func (e *moneyEnv) setCollateralEnabled(t *testing.T, enabled bool) {
	t.Helper()
	value := "false"
	if enabled {
		value = "true"
	}
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE system_config SET value = $1 WHERE key = $2`, value, domain.PPAPCollateralEnabledKey); err != nil {
		t.Fatalf("menyetel saklar agunan: %v", err)
	}
	e.configSvc.Invalidate(domain.PPAPCollateralEnabledKey)
}

func (e *moneyEnv) addLandCollateral(t *testing.T, loanID uuid.UUID, value int64) {
	t.Helper()
	taksasi := time.Now().UTC().AddDate(0, 0, -10)
	c := &domain.LoanCollateral{
		LoanID:         loanID,
		CollateralType: domain.CollateralTanahBangunan,
		Description:    "tanah dan bangunan uji",
		DocumentNumber: fmt.Sprintf("SHM-%d", time.Now().UnixNano()),
		OwnerName:      "Pemilik Uji",
		AppraisalValue: idr(value),
		AppraisalDate:  taksasi,
		HaircutPercent: decimal.Zero,
		Status:         domain.CollateralActive,
		Certified:      true,
		Mortgaged:      true,
		ExistsKnown:    true,
		Executable:     true,
	}
	if err := e.collateralRepo.Create(e.ctx, c, "001"); err != nil {
		t.Fatalf("mencatat agunan tanah: %v", err)
	}
}

func (e *moneyEnv) addCashCollateral(t *testing.T, loanID, cashAccountID uuid.UUID, value int64) {
	t.Helper()
	taksasi := time.Now().UTC().AddDate(0, 0, -10)
	c := &domain.LoanCollateral{
		LoanID:         loanID,
		CollateralType: domain.CollateralDeposit,
		Description:    "deposito uji",
		DocumentNumber: fmt.Sprintf("DEP-%d", time.Now().UnixNano()),
		OwnerName:      "Pemilik Uji",
		AppraisalValue: idr(value),
		AppraisalDate:  taksasi,
		HaircutPercent: decimal.Zero,
		Status:         domain.CollateralActive,
		IsCash:         true,
		CashAccountID:  &cashAccountID,
		ExistsKnown:    true,
		Executable:     true,
	}
	if err := e.collateralRepo.Create(e.ctx, c, "001"); err != nil {
		t.Fatalf("mencatat agunan tunai: %v", err)
	}
}

func ppapItem(t *testing.T, summary domain.PPAPRunSummary, loanID uuid.UUID) domain.PPAPRunItem {
	t.Helper()
	for _, f := range summary.Failures {
		if f.LoanID == loanID {
			t.Fatalf("PPAP gagal untuk kredit %s: %s", loanID, f.Error)
		}
	}
	for _, it := range summary.Items {
		if it.LoanID == loanID {
			return it
		}
	}
	t.Fatalf("kredit %s tidak ada pada ringkasan PPAP", loanID)
	return domain.PPAPRunItem{}
}

// setOldestDueDate memundurkan angsuran pertama sehingga DPD terkendali.
func (e *moneyEnv) setOldestDueDate(t *testing.T, loanID uuid.UUID, due time.Time) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loan_schedules SET due_date = $1 WHERE loan_id = $2 AND installment_no = 1`,
		due, loanID); err != nil {
		t.Fatalf("mengubah jatuh tempo angsuran: %v", err)
	}
}

// Saklar pengurangan agunan diuji lewat Preview karena jalur RunDaily tidak dapat
// dieksekusi pada database sungguhan (lihat TestIntegrasiJalurUangPPAPRunDaily):
// di-skip-nya tidak mungkin, jadi kelakuan saklar diperiksa pada jalur hitung yang
// memang berjalan.
func TestIntegrasiJalurUangPPAPDenganAgunan(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()

	// Kredit kurang lancar (DPD ~100 hari): pengurang Pasal 20 berlaku.
	cust := e.newCustomer(t, "Fajar Nugroho", "")
	accID := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, accID, idr(10_000_000), 4)
	e.setOldestDueDate(t, loan.ID, asOf.AddDate(0, 0, -100))

	e.setCollateralEnabled(t, false)
	off1, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("preview PPAP saklar mati (sebelum agunan): %v", err)
	}
	itemOff1 := ppapItem(t, off1, loan.ID)
	if !itemOff1.CollateralValue.IsZero() || !itemOff1.Exposure.Equal(idr(10_000_000)) {
		t.Fatalf("saklar mati: pengurang %s eksposur %s, mau 0/10000000",
			itemOff1.CollateralValue, itemOff1.Exposure)
	}

	// Agunan aktif ditambahkan; selama saklar mati hasilnya harus identik.
	e.addLandCollateral(t, loan.ID, 4_000_000)
	e.setCollateralEnabled(t, false)
	off2, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("preview PPAP saklar mati (setelah agunan): %v", err)
	}
	itemOff2 := ppapItem(t, off2, loan.ID)
	if !itemOff2.CollateralValue.IsZero() || !itemOff2.Exposure.Equal(idr(10_000_000)) {
		t.Fatalf("saklar mati membaca agunan: pengurang %s eksposur %s, mau 0/10000000",
			itemOff2.CollateralValue, itemOff2.Exposure)
	}
	if !itemOff2.Target.Equal(itemOff1.Target) {
		t.Fatalf("target berubah saat saklar mati: %s vs %s", itemOff1.Target, itemOff2.Target)
	}

	// Saklar hidup: pengurang Pasal 20 dipakai.
	e.setCollateralEnabled(t, true)
	on, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("preview PPAP saklar hidup: %v", err)
	}
	itemOn := ppapItem(t, on, loan.ID)
	// Pasal 20 ayat (1): tanah dan bangunan bersertifikat dengan hak tanggungan
	// diperhitungkan paling tinggi 80% dari nilai agunan. Taksasi 4 juta -> 3,2 juta,
	// sehingga eksposur = 10 juta - 3,2 juta = 6,8 juta.
	wantCollateral := idr(3_200_000)
	if !itemOn.CollateralValue.Equal(wantCollateral) {
		t.Fatalf("saklar hidup: pengurang %s, mau 80%% dari 4000000 = %s", itemOn.CollateralValue, wantCollateral)
	}
	wantExposure := idr(10_000_000).Sub(wantCollateral)
	if !itemOn.Exposure.Equal(wantExposure) {
		t.Fatalf("saklar hidup: eksposur %s, mau %s", itemOn.Exposure, wantExposure)
	}
	if !itemOn.Target.LessThan(itemOff2.Target) {
		t.Fatalf("target saklar hidup %s tidak lebih kecil dari saklar mati %s", itemOn.Target, itemOff2.Target)
	}
}

// RunDaily harus benar-benar menyimpan state ke database: kolektibilitas, DPD, dan
// required_ppap pada baris kredit, serta macet_at yang terisi sekali saat kredit
// pertama kali digolongkan Macet. Bug cast SQL dulu menolak seluruh UPDATE dengan
// SQLSTATE 42P08 tanpa jejak di ringkasan, sehingga asersi state tersimpan inilah yang
// mengunci regresinya.
func TestIntegrasiJalurUangPPAPRunDaily(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()

	cust := e.newCustomer(t, "Kartika Sari", "")
	accID := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, accID, idr(10_000_000), 4)
	e.setOldestDueDate(t, loan.ID, asOf.AddDate(0, 0, -100))
	e.setCollateralEnabled(t, false)

	off, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("run PPAP saklar mati: %v", err)
	}
	itemOff := ppapItem(t, off, loan.ID)
	if !itemOff.Exposure.Equal(idr(10_000_000)) {
		t.Fatalf("saklar mati: eksposur %s, mau 10000000", itemOff.Exposure)
	}
	if itemOff.Collectibility != domain.KolKurangLancar {
		t.Fatalf("saklar mati: kolektibilitas %s, mau Kurang Lancar", itemOff.Collectibility.Label())
	}
	// State benar-benar tersimpan di database, bukan hanya dilaporkan di ringkasan.
	if got := e.loanString(t, loan.ID, "collectibility"); got != string(domain.KolKurangLancar.OJKCode()) {
		t.Fatalf("kolektibilitas tersimpan %q, mau %q", got, domain.KolKurangLancar.OJKCode())
	}
	if got := e.loanInt(t, loan.ID, "dpd"); got != itemOff.DPD {
		t.Fatalf("dpd tersimpan %d, mau %d", got, itemOff.DPD)
	}
	if got := e.loanDecimal(t, loan.ID, "required_ppap"); !got.Equal(itemOff.Target) {
		t.Fatalf("required_ppap tersimpan %s, mau %s", got, itemOff.Target)
	}

	e.addLandCollateral(t, loan.ID, 4_000_000)
	e.setCollateralEnabled(t, true)
	on, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("run PPAP saklar hidup: %v", err)
	}
	itemOn := ppapItem(t, on, loan.ID)
	if !itemOn.CollateralValue.Equal(idr(3_200_000)) {
		t.Fatalf("saklar hidup: pengurang %s, mau 3200000 (80%% Pasal 20)", itemOn.CollateralValue)
	}
	if got := e.loanDecimal(t, loan.ID, "required_ppap"); !got.Equal(itemOn.Target) {
		t.Fatalf("required_ppap tersimpan setelah agunan %s, mau %s", got, itemOn.Target)
	}

	// Kredit Macet: macet_at terisi saat pertama digolongkan Macet dan tidak ditimpa
	// pada run berikutnya.
	custMacet := e.newCustomer(t, "Lukman Hakim", "")
	accMacet := e.newAccount(t, custMacet.ID)
	loanMacet := e.disburse(t, custMacet.ID, accMacet, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanMacet.ID, asOf.AddDate(0, 0, -400))

	macet1, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("run PPAP kredit macet: %v", err)
	}
	itemMacet := ppapItem(t, macet1, loanMacet.ID)
	if itemMacet.Collectibility != domain.KolMacet {
		t.Fatalf("kolektibilitas kredit macet %s, mau Macet", itemMacet.Collectibility.Label())
	}
	if got := e.loanString(t, loanMacet.ID, "collectibility"); got != string(domain.KolMacet.OJKCode()) {
		t.Fatalf("kolektibilitas macet tersimpan %q, mau %q", got, domain.KolMacet.OJKCode())
	}
	macetAt1 := e.loanNullableTime(t, loanMacet.ID, "macet_at")
	if !macetAt1.Valid {
		t.Fatal("macet_at tidak terisi saat kredit digolongkan Macet")
	}

	if _, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.actor); err != nil {
		t.Fatalf("run PPAP ulang: %v", err)
	}
	macetAt2 := e.loanNullableTime(t, loanMacet.ID, "macet_at")
	if !macetAt2.Valid || !macetAt2.Time.Equal(macetAt1.Time) {
		t.Fatalf("macet_at berubah pada run berikutnya: %v vs %v", macetAt1.Time, macetAt2.Time)
	}
}

// Agunan tunai: mengecualikan bagian yang dijamin dari PPKA umum (kualitas Lancar),
// tetapi TIDAK mengurangi PPKA khusus (bukan daftar Pasal 20 ayat (1)).
func TestIntegrasiJalurUangPPAPAgunanTunai(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setCollateralEnabled(t, true)

	// PPKA umum: kredit Lancar tanpa tunggakan, agunan tunai 4 juta.
	custUmum := e.newCustomer(t, "Gita Puspita", "")
	accUmum := e.newAccount(t, custUmum.ID)
	loanUmum := e.disburse(t, custUmum.ID, accUmum, idr(10_000_000), 4)
	e.addCashCollateral(t, loanUmum.ID, accUmum, 4_000_000)

	// PPKA khusus: kredit Kurang Lancar, agunan tunai 4 juta.
	custKhusus := e.newCustomer(t, "Hadi Susanto", "")
	accKhusus := e.newAccount(t, custKhusus.ID)
	loanKhusus := e.disburse(t, custKhusus.ID, accKhusus, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanKhusus.ID, asOf.AddDate(0, 0, -100))
	e.addCashCollateral(t, loanKhusus.ID, accKhusus, 4_000_000)

	summary, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("preview PPAP: %v", err)
	}

	umum := ppapItem(t, summary, loanUmum.ID)
	if umum.Collectibility != domain.KolLancar {
		t.Fatalf("kredit tunai umum kolektibilitas %s, mau Lancar", umum.Collectibility.Label())
	}
	if !umum.CollateralValue.Equal(idr(4_000_000)) || !umum.Exposure.Equal(idr(6_000_000)) {
		t.Fatalf("PPKA umum: pengurang %s eksposur %s, mau 4000000/6000000",
			umum.CollateralValue, umum.Exposure)
	}

	khusus := ppapItem(t, summary, loanKhusus.ID)
	if khusus.Collectibility != domain.KolKurangLancar {
		t.Fatalf("kredit tunai khusus kolektibilitas %s, mau Kurang Lancar", khusus.Collectibility.Label())
	}
	if !khusus.CollateralValue.IsZero() || !khusus.Exposure.Equal(idr(10_000_000)) {
		t.Fatalf("PPKA khusus: pengurang %s eksposur %s, mau 0/10000000 (agunan tunai bukan Pasal 20(1))",
			khusus.CollateralValue, khusus.Exposure)
	}
}

// ---------- 4. pencarian nama lewat indeks token ----------

func TestIntegrasiJalurUangPencarianNamaNasabah(t *testing.T) {
	e := newMoneyEnv(t)
	duaKata := e.newCustomer(t, "Siti Rahayu", "")

	hasilSiti, totalSiti, err := e.customerSvc.ListCustomers(e.ctx, 1, 50, "siti", e.actor)
	if err != nil {
		t.Fatalf("pencarian \"siti\": %v", err)
	}
	if totalSiti == 0 || !containsCustomer(hasilSiti, duaKata.ID) {
		t.Fatalf("pencarian \"siti\" tidak menemukan nasabah dua kata (total %d)", totalSiti)
	}
	hasilRahayu, _, err := e.customerSvc.ListCustomers(e.ctx, 1, 50, "rahayu", e.actor)
	if err != nil {
		t.Fatalf("pencarian \"rahayu\": %v", err)
	}
	if !containsCustomer(hasilRahayu, duaKata.ID) {
		t.Fatal("pencarian \"rahayu\" tidak menemukan nasabah dua kata")
	}

	hasilKosong, totalKosong, err := e.customerSvc.ListCustomers(e.ctx, 1, 50, "zzzznamaentah", e.actor)
	if err != nil {
		t.Fatalf("pencarian tanpa hasil: %v", err)
	}
	if totalKosong != 0 || len(hasilKosong) != 0 {
		t.Fatalf("pencarian tanpa hasil mengembalikan %d baris (total %d), mau kosong", len(hasilKosong), totalKosong)
	}

	// Dua nasabah tanpa email: indeks unik email_index bersifat parsial, sehingga
	// keduanya boleh ada. Bila NULL ditulis sebagai string kosong, yang kedua gagal.
	if e.newCustomer(t, "Indra Wijaya", "") == nil {
		t.Fatal("nasabah tanpa email pertama gagal")
	}
	tanpaEmailKedua := e.newCustomer(t, "Indra Kurniawan", "")
	if tanpaEmailKedua == nil {
		t.Fatal("nasabah tanpa email kedua gagal")
	}
	var n int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM customers WHERE id = $1 AND email_index IS NULL`, tanpaEmailKedua.ID).Scan(&n); err != nil {
		t.Fatalf("memeriksa email_index: %v", err)
	}
	if n != 1 {
		t.Fatalf("email_index nasabah tanpa email bukan NULL (baris %d)", n)
	}
}

// ---------- 5. pembatalan menolak akrual yang sudah terposting (T2) ----------

// Pembatalan pencairan hanya menolak kredit yang sudah punya angsuran dibayar. Padahal
// akrual bunga, akrual denda, dan PPAP dapat sudah memposting jurnal tanpa menyentuh
// paid_* maupun status. Ketiganya harus menolak pembatalan dengan sentinel yang
// menjelaskan apa yang harus dibereskan lebih dulu.
func TestIntegrasiJalurUangPembatalanDitolakSaatAdaAkrual(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()

	// (a) Akrual bunga: angsuran jatuh tempo kemarin, masih Lancar sehingga diakru.
	custBunga := e.newCustomer(t, "Maya Puspita", "")
	accBunga := e.newAccount(t, custBunga.ID)
	loanBunga := e.disburse(t, custBunga.ID, accBunga, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanBunga.ID, asOf.AddDate(0, 0, -1))
	bungaSum, err := e.loanSvc.AccrueInterest(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("akrual bunga: %v", err)
	}
	if bungaSum.Accrued == 0 {
		t.Fatalf("akrual bunga tidak terjadi: %+v", bungaSum)
	}
	if _, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{
		LoanID: loanBunga.ID, Reason: "coba batal setelah akrual bunga",
	}, e.actor); !errors.Is(err, domain.ErrLoanHasAccrualPostings) {
		t.Fatalf("pembatalan setelah akrual bunga: %v, mau ErrLoanHasAccrualPostings", err)
	}
	if got := e.loanStatus(t, loanBunga.ID); got != string(domain.LoanStatusDisbursed) {
		t.Fatalf("status kredit bunga berubah meski ditolak: %q", got)
	}

	// (b) Akrual denda: tarif dipasang, lalu tunggakan sehari diakru.
	e.setPenaltyRatePerMille(t, 10)
	custDenda := e.newCustomer(t, "Nina Marlina", "")
	accDenda := e.newAccount(t, custDenda.ID)
	loanDenda := e.disburse(t, custDenda.ID, accDenda, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanDenda.ID, asOf.AddDate(0, 0, -1))
	penSum, err := e.loanSvc.AccruePenalties(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("akrual denda: %v", err)
	}
	if penSum.Accrued == 0 {
		t.Fatalf("akrual denda tidak terjadi: %+v", penSum)
	}
	if _, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{
		LoanID: loanDenda.ID, Reason: "coba batal setelah akrual denda",
	}, e.actor); !errors.Is(err, domain.ErrLoanHasAccrualPostings) {
		t.Fatalf("pembatalan setelah akrual denda: %v, mau ErrLoanHasAccrualPostings", err)
	}

	// (c) PPAP: run harian memposting cadangan tapi tidak mengubah status.
	custPpap := e.newCustomer(t, "Omar Surya", "")
	accPpap := e.newAccount(t, custPpap.ID)
	loanPpap := e.disburse(t, custPpap.ID, accPpap, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanPpap.ID, asOf.AddDate(0, 0, -100))
	e.setCollateralEnabled(t, false)
	if _, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.actor); err != nil {
		t.Fatalf("run PPAP: %v", err)
	}
	if got := e.loanDecimal(t, loanPpap.ID, "required_ppap"); !got.IsPositive() {
		t.Fatalf("required_ppap tidak positif setelah run PPAP: %s", got)
	}
	if _, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{
		LoanID: loanPpap.ID, Reason: "coba batal setelah PPAP",
	}, e.actor); !errors.Is(err, domain.ErrLoanHasAccrualPostings) {
		t.Fatalf("pembatalan setelah PPAP: %v, mau ErrLoanHasAccrualPostings", err)
	}
}

// ---------- 6. dua koreksi bersamaan (T1/T6) ----------

// Dua koreksi bersamaan terhadap kredit yang sama harus terserialkan oleh kunci baris
// kredit: hanya satu jurnal selisih terbit dan saldo akhir konsisten. Tanpa kunci,
// keduanya membaca pokok lama dan menerbitkan jurnal selisih ganda dengan urutan berbeda.
func TestIntegrasiJalurUangDuaKoreksiBersamaan(t *testing.T) {
	e := newMoneyEnv(t)
	cust := e.newCustomer(t, "Putri Anggraini", "")
	accID := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, accID, idr(10_000_000), 4)

	const workers = 2
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
				LoanID: loan.ID, NewAmount: idr(12_000_000), Reason: "koreksi bersamaan",
			}, e.actor)
		}(i)
	}
	wg.Wait()

	sukses := 0
	for i, err := range errs {
		switch {
		case err == nil:
			sukses++
		case errors.Is(err, domain.ErrLoanAmountUnchanged):
			// Kalah balapan: kredit sudah bernilai 12jt saat kunci diperoleh.
		default:
			t.Fatalf("koreksi #%d gagal tak terduga: %v", i, err)
		}
	}
	if sukses == 0 {
		t.Fatal("tidak ada koreksi yang berhasil")
	}

	// Tepat satu jurnal selisih terbit untuk pasangan 10jt -> 12jt.
	if n := e.countJournalsByKey(t, "CORR-"+loan.LoanNumber+"-12000000-1"); n != 1 {
		t.Fatalf("jurnal selisih urutan pertama %d, mau 1", n)
	}
	if n := e.countJournalsByKey(t, "CORR-"+loan.LoanNumber+"-12000000-2"); n != 0 {
		t.Fatalf("jurnal selisih urutan kedua %d, mau 0 (koreksi kedua tidak menerbitkan selisih)", n)
	}
	after, err := e.loanRepo.GetByID(e.ctx, loan.ID)
	if err != nil {
		t.Fatalf("membaca kredit: %v", err)
	}
	if !after.PrincipalAmount.Equal(idr(12_000_000)) {
		t.Fatalf("nominal akhir %s, mau 12000000", after.PrincipalAmount)
	}
	if got := e.accountBalance(t, accID); !got.Equal(idr(12_000_000)) {
		t.Fatalf("saldo rekening akhir %s, mau 12000000 (kaki jurnal pencairan)", got)
	}
}

// Akrual denda menghentikan kredit NPL (kol 3-5) memakai gerbang kolektibilitas
// yang sama seperti akrual bunga: tarif denda diisi > 0, tetapi hanya kredit lancar
// yang menambah penalty_accrued dan menerbitkan jurnal; kredit NPL tidak.
func TestIntegrasiJalurUangAkrualDendaNPLDihentikan(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setPenaltyRatePerMille(t, 10)

	// Kredit lancar: DPD 1 -> kolektibilitas Lancar, denda diakru.
	custLancar := e.newCustomer(t, "Rina Lestari", "")
	accLancar := e.newAccount(t, custLancar.ID)
	loanLancar := e.disburse(t, custLancar.ID, accLancar, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanLancar.ID, asOf.AddDate(0, 0, -1))

	// Kredit NPL: DPD 100 -> Kurang Lancar, denda tidak diakru.
	custNpl := e.newCustomer(t, "Bayu Pratama", "")
	accNpl := e.newAccount(t, custNpl.ID)
	loanNpl := e.disburse(t, custNpl.ID, accNpl, idr(10_000_000), 4)
	e.setOldestDueDate(t, loanNpl.ID, asOf.AddDate(0, 0, -100))

	if _, err := e.loanSvc.AccruePenalties(e.ctx, asOf, e.actor); err != nil {
		t.Fatalf("akrual denda: %v", err)
	}

	if got := e.loanDecimal(t, loanNpl.ID, "penalty_accrued"); !got.IsZero() {
		t.Fatalf("kredit NPL menambah penalty_accrued %s, mau 0", got)
	}
	if got := e.loanDecimal(t, loanLancar.ID, "penalty_accrued"); !got.IsPositive() {
		t.Fatalf("kredit lancar tidak diakru, penalty_accrued=%s", got)
	}
	day := asOf.Format("2006-01-02")
	if n := e.countJournalsByKey(t, "PENALTY-"+loanNpl.LoanNumber+"-"+day); n != 0 {
		t.Fatalf("jurnal denda NPL terbit %d, mau 0", n)
	}
	if n := e.countJournalsByKey(t, "PENALTY-"+loanLancar.LoanNumber+"-"+day); n != 1 {
		t.Fatalf("jurnal denda lancar terbit %d, mau 1", n)
	}
}

func containsCustomer(list []domain.Customer, id uuid.UUID) bool {
	for _, c := range list {
		if c.ID == id {
			return true
		}
	}
	return false
}
