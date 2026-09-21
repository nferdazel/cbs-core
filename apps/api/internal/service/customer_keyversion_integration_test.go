package service_test

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
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// Uji integrasi pemisahan kunci indeks terhadap PostgreSQL sungguhan (migrasi 000048).
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola uji integrasi lain.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55442/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiVersiKunci -v
//
// Yang dibuktikan: baris yang ditulis SEBELUM pemisahan kunci (tanpa mengisi
// index_key_version, indeks dari master key enkripsi) tetap terbaca dan tetap
// ditemukan lewat NIK maupun nama meskipun aplikasi sudah memakai kunci indeks
// terpisah; dan baris baru (kunci terpisah) menyatu dalam satu hasil pencarian.
func TestIntegrasiVersiKunciLintasVersi(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi versi kunci dilewati")
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

	// Kunci enkripsi sama untuk kedua cipher; yang berbeda hanya kunci indeks.
	masterKey := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	indexKey := base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	legacyCipher, err := crypto.NewCipher("k1", masterKey, nil)
	if err != nil {
		t.Fatalf("cipher lama: %v", err)
	}
	splitCipher, err := crypto.NewCipherWithIndexKey("k1", masterKey, nil, crypto.IndexKeyConfig{
		ActiveKeyID: "ik1",
		ActiveKey:   indexKey,
	})
	if err != nil {
		t.Fatalf("cipher kunci terpisah: %v", err)
	}

	suffix := time.Now().UnixNano() % 1_000_000_000_000
	nikLegacy := fmt.Sprintf("9999%012d", suffix)
	nikNew := fmt.Sprintf("9998%012d", suffix)
	emailLegacy := fmt.Sprintf("legacy-%d@uji.local", suffix)
	legacyID, newID := uuid.New(), uuid.New()

	// Baris lama disisipkan langsung TANPA index_key_version dan memakai indeks dari
	// master key, meniru data yang sudah ada sebelum migrasi 000048.
	legacyName := "Legacy Kunci Lama"
	if _, err := db.ExecContext(ctx, `
		INSERT INTO customers (
			id, cif_number, full_name_enc, id_card_number_enc, email_enc,
			phone_number_enc, address_enc, id_card_index, email_index, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,'ACTIVE',NOW(),NOW())`,
		legacyID, "CIF-KV-L-"+fmt.Sprint(suffix),
		encryptOrFail(t, legacyCipher, legacyName),
		encryptOrFail(t, legacyCipher, nikLegacy),
		encryptOrFail(t, legacyCipher, emailLegacy),
		encryptOrFail(t, legacyCipher, "081200000001"),
		encryptOrFail(t, legacyCipher, "Jl. Lama No. 1"),
		legacyCipher.BlindIndex(nikLegacy), legacyCipher.BlindIndex(emailLegacy),
	); err != nil {
		t.Fatalf("menyisipkan baris lama: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO customer_name_tokens (customer_id, token_index) VALUES ($1, $2)`,
		legacyID, legacyCipher.NameTokenIndex("legacy"),
	); err != nil {
		t.Fatalf("menyisipkan token lama: %v", err)
	}

	// Baris baru memakai kunci indeks terpisah dan mencatat versinya.
	newName := "Baru Kunci Pisah"
	if _, err := db.ExecContext(ctx, `
		INSERT INTO customers (
			id, cif_number, full_name_enc, id_card_number_enc, email_enc,
			phone_number_enc, address_enc, id_card_index, email_index, index_key_version,
			status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'ACTIVE',NOW(),NOW())`,
		newID, "CIF-KV-N-"+fmt.Sprint(suffix),
		encryptOrFail(t, splitCipher, newName),
		encryptOrFail(t, splitCipher, nikNew),
		encryptOrFail(t, splitCipher, fmt.Sprintf("baru-%d@uji.local", suffix)),
		encryptOrFail(t, splitCipher, "081200000002"),
		encryptOrFail(t, splitCipher, "Jl. Baru No. 2"),
		splitCipher.BlindIndex(nikNew), splitCipher.BlindIndex(fmt.Sprintf("baru-%d@uji.local", suffix)),
		splitCipher.IndexKeyVersion(),
	); err != nil {
		t.Fatalf("menyisipkan baris baru: %v", err)
	}
	if _, err := db.ExecContext(ctx,
		`INSERT INTO customer_name_tokens (customer_id, token_index, index_key_version) VALUES ($1, $2, $3)`,
		newID, splitCipher.NameTokenIndex("baru"), splitCipher.IndexKeyVersion(),
	); err != nil {
		t.Fatalf("menyisipkan token baru: %v", err)
	}

	ids := []uuid.UUID{legacyID, newID}
	t.Cleanup(func() {
		// Token punya ON DELETE CASCADE, tetapi dihapus eksplisit agar uji tidak
		// bergantung pada urutan/constraint tabel.
		_, _ = db.ExecContext(ctx, `DELETE FROM customer_name_tokens WHERE customer_id = ANY($1)`, ids)
		_, _ = db.ExecContext(ctx, `DELETE FROM customers WHERE id = ANY($1)`, ids)
	})

	repo := postgres.NewCustomerRepository(db)
	svc := service.NewCustomerService(db, repo, splitCipher, postgres.NewReferenceGenerator(db), postgres.NewAuditRepository(db))
	actor := domain.Actor{UserID: uuid.New(), Role: domain.RoleSuperAdmin}

	// 1. Baris lama tetap terbaca (dekripsi) dan versinya tercatat sebagai k1.
	legacyRec, err := repo.GetByID(ctx, legacyID)
	if err != nil {
		t.Fatalf("membaca baris lama: %v", err)
	}
	if legacyRec.IndexKeyVersion != "k1" {
		t.Fatalf("versi baris lama = %q, ingin k1 (DEFAULT migrasi)", legacyRec.IndexKeyVersion)
	}
	name, err := splitCipher.Decrypt(legacyRec.FullNameEnc)
	if err != nil || name != legacyName {
		t.Fatalf("dekripsi baris lama = %q, %v; ingin %q", name, err, legacyName)
	}

	// 2. Pencarian NIK baris lama tetap menemukan baris itu meski cipher memakai
	// kunci indeks terpisah (kandidat lintas versi).
	assertSearchFinds(t, svc, actor, nikLegacy, legacyID, "NIK lama")
	assertSearchFinds(t, svc, actor, nikNew, newID, "NIK baru")

	// 3. Pencarian nama menyatukan kedua versi: kata "legacy" (kunci lama) dan
	// "baru" (kunci baru) sama-sama menemukan barisnya masing-masing.
	assertSearchFinds(t, svc, actor, "legacy", legacyID, "nama versi lama")
	assertSearchFinds(t, svc, actor, "baru", newID, "nama versi baru")
}

func encryptOrFail(t *testing.T, c *crypto.Cipher, plaintext string) string {
	t.Helper()
	blob, err := c.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	return blob
}

func assertSearchFinds(t *testing.T, svc domain.CustomerService, actor domain.Actor, term string, want uuid.UUID, label string) {
	t.Helper()
	customers, total, err := svc.ListCustomers(context.Background(), 1, 50, term, actor)
	if err != nil {
		t.Fatalf("pencarian %s (%q): %v", label, term, err)
	}
	if total != 1 || len(customers) != 1 {
		t.Fatalf("pencarian %s (%q): total=%d len=%d, ingin tepat 1", label, term, total, len(customers))
	}
	if customers[0].ID != want {
		t.Fatalf("pencarian %s (%q) menemukan %s, ingin %s", label, term, customers[0].ID, want)
	}
}
