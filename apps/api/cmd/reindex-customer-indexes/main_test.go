package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Uji integrasi perintah reindex terhadap PostgreSQL sungguhan. Di-skip kecuali
// CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55442/cbs?sslmode=disable' \
//	  go test ./cmd/reindex-customer-indexes/ -run TestReindex -v
//
// Membuktikan: baris lama dipindahkan ke versi kunci aktif beserta token namanya,
// dan menjalankan perintah dua kali tidak mengubah apa pun lagi (idempoten).
func TestReindexMovesLegacyRowAndIsIdempotent(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi reindex dilewati")
	}
	ctx := context.Background()

	// Isolasi skema: `run` memindai SELURUH tabel customers, sehingga peninggalan uji
	// lain (mis. uji OJK menyisipkan nasabah tanpa full_name_enc) membuat baris dengan
	// full_name_enc NULL ikut ter-scan dan menggagalkan uji ini tergantung urutan paket.
	// Uji ini karena itu bekerja di skema khusus yang hanya berisi datanya sendiri;
	// struktur tabel disalin dari tabel asli (LIKE ... INCLUDING ALL) agar kolom,
	// default, dan constraint tetap sama dengan yang dipakai di produksi.
	schema := fmt.Sprintf("reindex_test_%d", time.Now().UnixNano()%1_000_000_000_000)
	admin, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("membuka database admin: %v", err)
	}
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		t.Fatalf("database tidak dapat dihubungi: %v", err)
	}

	var db *sql.DB
	t.Cleanup(func() {
		if db != nil {
			_ = db.Close()
		}
		_, _ = admin.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %q CASCADE", schema))
		_ = admin.Close()
	})

	if _, err := admin.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA %q", schema)); err != nil {
		t.Fatalf("membuat skema isolasi: %v", err)
	}
	for _, table := range []string{"customers", "customer_name_tokens"} {
		if _, err := admin.ExecContext(ctx, fmt.Sprintf(
			"CREATE TABLE %q.%s (LIKE public.%s INCLUDING ALL)", schema, table, table)); err != nil {
			t.Fatalf("menyalin tabel %s: %v", table, err)
		}
	}

	db, err = sql.Open("pgx", dsnWithSearchPath(dsn, schema))
	if err != nil {
		t.Fatalf("membuka skema isolasi: %v", err)
	}
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("skema isolasi tidak dapat dihubungi: %v", err)
	}

	masterKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	legacyCipher, err := crypto.NewCipher("k1", masterKey, nil)
	if err != nil {
		t.Fatalf("cipher lama: %v", err)
	}
	splitCipher, err := crypto.NewCipherWithIndexKey("k1", masterKey, nil, crypto.IndexKeyConfig{
		ActiveKeyID: "ik1",
		ActiveKey:   base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789")),
	})
	if err != nil {
		t.Fatalf("cipher kunci terpisah: %v", err)
	}

	suffix := time.Now().UnixNano() % 1_000_000_000_000
	nik := fmt.Sprintf("9997%012d", suffix)
	id := uuid.New()
	name := "Nasabah Reindex"
	fullNameBlob, _ := legacyCipher.Encrypt(name)
	idCardBlob, _ := legacyCipher.Encrypt(nik)

	if _, err := db.ExecContext(ctx, `
		INSERT INTO customers (
			id, cif_number, full_name_enc, id_card_number_enc, email_enc, phone_number_enc,
			address_enc, id_card_index, email_index, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,'','','', $5, NULL, 'ACTIVE', NOW(), NOW())`,
		id, "CIF-RIDX-"+fmt.Sprint(suffix), fullNameBlob, idCardBlob, legacyCipher.BlindIndex(nik),
	); err != nil {
		t.Fatalf("menyisipkan baris lama: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO customer_name_tokens (customer_id, token_index) VALUES ($1, $2)`,
		id, legacyCipher.NameTokenIndex("nasabah")); err != nil {
		t.Fatalf("menyisipkan token lama: %v", err)
	}

	updated, skipped, err := run(ctx, db, splitCipher, 50)
	if err != nil {
		t.Fatalf("reindex: %v", err)
	}
	if updated == 0 {
		t.Fatalf("reindex tidak memperbarui baris apa pun (skipped=%d)", skipped)
	}

	var version, idCardIndex string
	if err := db.QueryRowContext(ctx,
		`SELECT index_key_version, id_card_index FROM customers WHERE id = $1`, id).
		Scan(&version, &idCardIndex); err != nil {
		t.Fatalf("membaca baris hasil reindex: %v", err)
	}
	if version != "ik1" {
		t.Fatalf("versi setelah reindex = %q, ingin ik1", version)
	}
	if want := splitCipher.BlindIndex(nik); idCardIndex != want {
		t.Fatalf("blind index setelah reindex = %q, ingin %q", idCardIndex, want)
	}

	var tokenCount, tokenVersioned int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*), COUNT(*) FILTER (WHERE index_key_version = $2)
		 FROM customer_name_tokens WHERE customer_id = $1`, id, "ik1").
		Scan(&tokenCount, &tokenVersioned); err != nil {
		t.Fatalf("membaca token hasil reindex: %v", err)
	}
	wantTokens := len(domain.NormalizeNameTokens(name))
	if tokenCount != wantTokens || tokenVersioned != wantTokens {
		t.Fatalf("token setelah reindex: total=%d ber-versi-ik1=%d, ingin %d", tokenCount, tokenVersioned, wantTokens)
	}

	// Idempoten: baris yang sudah ber-versi aktif tidak disentuh lagi.
	updatedAgain, _, err := run(ctx, db, splitCipher, 50)
	if err != nil {
		t.Fatalf("reindex kedua: %v", err)
	}
	if updatedAgain != 0 {
		t.Fatalf("reindex kedua memperbarui %d baris, ingin 0 (tidak idempoten)", updatedAgain)
	}
}

// dsnWithSearchPath menambahkan search_path ke DSN tanpa merusak parameter yang sudah
// ada, sehingga koneksi uji hanya melihat tabel di skema isolasinya.
func dsnWithSearchPath(dsn, schema string) string {
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "search_path=" + schema
}
