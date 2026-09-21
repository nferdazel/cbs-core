package main

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
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
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("membuka database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database tidak dapat dihubungi: %v", err)
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
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM customer_name_tokens WHERE customer_id = $1`, id)
		_, _ = db.ExecContext(ctx, `DELETE FROM customers WHERE id = $1`, id)
	})

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
