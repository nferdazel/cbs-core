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

// TestIntegrasiAktivitasHarianMemisahkanPenempatanDeposito membuktikan query
// DailyActivity memisahkan setoran tunai teller (DEPOSIT) dari penempatan deposito
// berjangka (DEPOSIT_PLACEMENT) pada database sungguhan. Sekaligus dipatok
// keterbatasan data lama: jurnal penempatan yang bertipe DEPOSIT tetap terhitung
// sebagai setoran tunai, bukan direklasifikasi diam-diam.
func TestIntegrasiAktivitasHarianMemisahkanPenempatanDeposito(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi aktivitas harian dilewati")
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

	var accountID string
	if err := db.QueryRowContext(ctx, `SELECT id::text FROM accounts LIMIT 1`).Scan(&accountID); err != nil {
		t.Fatalf("membaca rekening uji: %v", err)
	}

	// Tanggal sengaja jauh dari tanggal bisnis uji lain agar agregat per tanggal
	// hanya memuat baris yang disiapkan uji ini.
	date := time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
	ref := fmt.Sprintf("UJI-AKTIVITAS-%d", time.Now().UnixNano())
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM journal_entries WHERE entry_date = $1::date`, date); err != nil {
			t.Logf("membersihkan jurnal uji: %v", err)
		}
	})

	insertEntry := func(transactionType, reference, description string, amount int64) {
		t.Helper()
		var id string
		if err := db.QueryRowContext(ctx, `
			INSERT INTO journal_entries (reference_number, transaction_type, description, entry_date, status)
			VALUES ($1, $2::transaction_type, $3, $4::date, 'POSTED')
			RETURNING id::text`, reference, transactionType, description, date).Scan(&id); err != nil {
			t.Fatalf("menyisipkan jurnal %s: %v", transactionType, err)
		}
		if _, err := db.ExecContext(ctx, `
			INSERT INTO journal_lines (journal_entry_id, account_id, direction, amount, balance_after, sequence)
			VALUES ($1, $2, 'CREDIT', $3, $3, 1)`, id, accountID, amount); err != nil {
			t.Fatalf("menyisipkan baris jurnal %s: %v", transactionType, err)
		}
	}

	insertEntry("DEPOSIT", ref+"-D", "setoran tunai teller", 1_000_000)
	insertEntry("DEPOSIT_PLACEMENT", ref+"-P1", "penempatan deposito baru", 25_000_000)
	insertEntry("DEPOSIT_PLACEMENT", ref+"-P2", "penempatan deposito baru", 5_000_000)
	// Data lama: penempatan yang dibuat sebelum TxTypeDepositPlacement ada tetap
	// bertipe DEPOSIT dan harus tetap terhitung sebagai setoran tunai.
	insertEntry("DEPOSIT", ref+"-OLD", "penempatan deposito (jurnal lama bertipe DEPOSIT)", 2_000_000)

	repo := postgres.NewBatchActivityRepository(db)
	got, err := repo.DailyActivity(ctx, date)
	if err != nil {
		t.Fatalf("DailyActivity: %v", err)
	}

	// Setoran tunai = 1.000.000 (baru) + 2.000.000 (lama bertipe DEPOSIT).
	if want := decimal.NewFromInt(3_000_000); !got.TotalDepositAmount.Equal(want) {
		t.Fatalf("total setoran tunai %s, mau %s", got.TotalDepositAmount, want)
	}
	if got.DepositPlacementCount != 2 {
		t.Fatalf("jumlah penempatan deposito %d, mau 2", got.DepositPlacementCount)
	}
	if want := decimal.NewFromInt(30_000_000); !got.TotalDepositPlacementAmount.Equal(want) {
		t.Fatalf("nominal penempatan deposito %s, mau %s", got.TotalDepositPlacementAmount, want)
	}
	if got.PostedJournals != 4 {
		t.Fatalf("jurnal terposting %d, mau 4", got.PostedJournals)
	}
}
