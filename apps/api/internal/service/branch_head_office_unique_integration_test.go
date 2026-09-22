package service_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/google/uuid"
)

// TestIntegrasiKantorPusatTunggalMengujiIndeksUnik membuktikan penjaga lapisan
// database (migrasi 000070): INSERT kantor pusat kedua ditolak indeks unik parsial
// idx_branches_single_head_office, bukan hanya oleh validasi service. Pemilihan
// kantor pusat memakai is_head_office sehingga HO kedua dapat menggeser atribusi
// cabang. Bila indeks belum ada, uji ini gagal karena insert berhasil.
func TestIntegrasiKantorPusatTunggalMengujiIndeksUnik(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi kantor pusat tunggal dilewati")
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

	var sebelum int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM branches WHERE is_head_office`).Scan(&sebelum); err != nil {
		t.Fatalf("menghitung kantor pusat: %v", err)
	}
	if sebelum != 1 {
		t.Fatalf("kantor pusat = %d, ingin tepat 1", sebelum)
	}

	// Kode maks 8 karakter; unik per run agar tidak bentrok bila uji diulang.
	code := "HO" + uuid.NewString()[:6]
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, `DELETE FROM branches WHERE code = $1`, code); err != nil {
			t.Logf("membersihkan kantor pusat uji: %v", err)
		}
	})

	_, err = db.ExecContext(ctx, `
		INSERT INTO branches (code, name, is_head_office)
		VALUES ($1, $2, TRUE)`, code, fmt.Sprintf("Kantor Pusat Uji %s", code))
	if err == nil {
		t.Fatal("kantor pusat kedua harus ditolak indeks unik idx_branches_single_head_office")
	}

	var sesudah int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM branches WHERE is_head_office`).Scan(&sesudah); err != nil {
		t.Fatalf("menghitung kantor pusat setelah percobaan: %v", err)
	}
	if sesudah != 1 {
		t.Fatalf("kantor pusat setelah percobaan = %d, ingin tetap 1", sesudah)
	}
}
