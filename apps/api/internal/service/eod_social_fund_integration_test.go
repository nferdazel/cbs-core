package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/shopspring/decimal"
)

// TestIntegrasiSaldoDanaKebajikanCocokDenganAkun membuktikan angka SocialFundBalance
// pada aktivitas harian EOD benar-benar saldo akun 12500 "Dana Kebajikan" sampai
// tanggal bisnis tersebut, bukan konstanta. Uji menyisipkan satu jurnal KREDIT
// bernominal diketahui pada tanggal unik, lalu membandingkan hasil repository dengan
// jumlah sisi kredit-dikurangi-debit yang dihitung langsung dari database.
func TestIntegrasiSaldoDanaKebajikanCocokDenganAkun(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi saldo dana kebajikan dilewati")
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

	// Akun GL untuk COA 12500 (bagan akun migrasi 000024/000068).
	var glAccountID string
	if err := db.QueryRowContext(ctx, `
		SELECT a.id::text
		FROM accounts a
		JOIN chart_of_accounts c ON c.id = a.coa_id
		WHERE c.code = '12500' AND a.account_type = 'INTERNAL_GL'
		ORDER BY a.account_number
		LIMIT 1`).Scan(&glAccountID); err != nil {
		t.Fatalf("membaca akun GL 12500: %v", err)
	}

	// Tanggal jauh di masa depan: agregat harian hanya memuat baris uji ini, sedangkan
	// saldo kumulatifnya = saldo sebelum tanggal + nominal yang disisipkan uji.
	date := time.Date(2099, 11, 30, 0, 0, 0, 0, time.UTC)
	ref := fmt.Sprintf("UJI-DANA-KEBAJIKAN-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM journal_entries WHERE entry_date = $1::date`, date); err != nil {
			t.Logf("membersihkan jurnal uji: %v", err)
		}
	})

	// Saldo sebelum tanggal uji, dihitung langsung dari database (query independen).
	var before decimal.Decimal
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE -jl.amount END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE jl.account_id = $1 AND je.entry_date < $2::date`,
		glAccountID, date).Scan(&before); err != nil {
		t.Fatalf("membaca saldo sebelum tanggal uji: %v", err)
	}

	nominal := decimal.NewFromInt(123_456)
	var entryID string
	if err := db.QueryRowContext(ctx, `
		INSERT INTO journal_entries (reference_number, transaction_type, description, entry_date, status)
		VALUES ($1, 'ADJUSTMENT', 'uji saldo dana kebajikan', $2::date, 'POSTED')
		RETURNING id::text`, ref, date).Scan(&entryID); err != nil {
		t.Fatalf("menyisipkan jurnal uji: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
		INSERT INTO journal_lines (journal_entry_id, account_id, direction, amount, balance_after, sequence)
		VALUES ($1, $2, 'CREDIT', $3, $3, 1)`, entryID, glAccountID, nominal); err != nil {
		t.Fatalf("menyisipkan baris jurnal uji: %v", err)
	}

	// Pembacaan saldo tidak boleh menulis jurnal apa pun.
	var jurnalSebelum int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM journal_entries`).Scan(&jurnalSebelum); err != nil {
		t.Fatalf("menghitung jurnal sebelum baca saldo: %v", err)
	}

	repo := postgres.NewBatchActivityRepository(db)
	got, err := repo.DailyActivity(ctx, date)
	if err != nil {
		t.Fatalf("DailyActivity: %v", err)
	}

	var jurnalSesudah int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM journal_entries`).Scan(&jurnalSesudah); err != nil {
		t.Fatalf("menghitung jurnal sesudah baca saldo: %v", err)
	}
	if jurnalSesudah != jurnalSebelum {
		t.Fatalf("pembacaan saldo dana kebajikan menulis jurnal: %d -> %d", jurnalSebelum, jurnalSesudah)
	}

	// Saldo langsung dari database sampai tanggal uji.
	var want decimal.Decimal
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE -jl.amount END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE jl.account_id = $1 AND je.entry_date <= $2::date`,
		glAccountID, date).Scan(&want); err != nil {
		t.Fatalf("membaca saldo sampai tanggal uji: %v", err)
	}

	// Saldo sebelum + nominal uji harus sama dengan query kumulatif; bila tidak,
	// asumsi tanggal unik/isolasi uji ini yang salah, bukan nilai repository.
	if expected := before.Add(nominal); !want.Equal(expected) {
		t.Fatalf("saldo database tidak konsisten: kumulatif %s, sebelum+nominal %s", want, expected)
	}
	if !got.SocialFundBalance.Equal(want) {
		t.Fatalf("SocialFundBalance = %s, saldo akun 12500 di database = %s", got.SocialFundBalance, want)
	}
	t.Logf("SocialFundBalance per %s = %s (saldo akun 12500 di database)", date.Format("2006-01-02"), got.SocialFundBalance)

	// Angka nyata per hari ini, juga dibandingkan langsung dengan database, agar
	// nilai yang dilaporkan EOD dapat ditunjukkan tanpa data uji 2099.
	today := time.Now().UTC()
	gotToday, err := repo.DailyActivity(ctx, today)
	if err != nil {
		t.Fatalf("DailyActivity hari ini: %v", err)
	}
	var wantToday decimal.Decimal
	if err := db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(CASE WHEN jl.direction = 'CREDIT' THEN jl.amount ELSE -jl.amount END), 0)
		FROM journal_lines jl
		JOIN journal_entries je ON je.id = jl.journal_entry_id
		WHERE jl.account_id = $1 AND je.entry_date <= $2::date`,
		glAccountID, today).Scan(&wantToday); err != nil {
		t.Fatalf("membaca saldo hari ini: %v", err)
	}
	if !gotToday.SocialFundBalance.Equal(wantToday) {
		t.Fatalf("SocialFundBalance hari ini = %s, saldo akun 12500 di database = %s", gotToday.SocialFundBalance, wantToday)
	}
	t.Logf("saldo dana kebajikan per %s = %s", today.Format("2006-01-02"), gotToday.SocialFundBalance)
}
