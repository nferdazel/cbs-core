// Perintah reindex-customer-indexes menghitung ulang blind index nasabah memakai
// kunci indeks yang sedang aktif (ENCRYPTION_INDEX_KEY). Perintah ini adalah
// mekanisme rotasi BERTAHAP: ia TIDAK dijalankan otomatis oleh server, hanya saat
// operator memanggilnya, dan aman diulang.
//
// Latar belakang: pemisahan kunci indeks dari kunci enkripsi (migrasi 000048)
// membuat baris baru memakai versi kunci indeks di kolom index_key_version. Baris
// lama tetap terbaca/dicari karena pencarian menghitung kandidat indeks lintas
// versi, tetapi untuk menyelesaikan rotasi baris lama perlu diindeks ulang dengan
// kunci baru. Perintah ini melakukannya per batch.
//
// Sifat:
//   - Idempoten: hanya menyentuh baris yang index_key_version-nya berbeda dari versi
//     aktif. Baris yang sudah benar dilewati, sehingga aman dijalankan berulang.
//   - Tidak menghapus kunci lama: konfigurasi ENCRYPTION_PREVIOUS_KEYS tetap
//     diperlukan untuk mendekripsi baris lama sampai reindex selesai.
//   - Tidak menuliskan plaintext, nilai kunci, atau turunannya ke log; hanya id
//     nasabah pada baris galat.
//
// Pemakaian:
//
//	cd apps/api && go run ./cmd/reindex-customer-indexes -batch 200
//
// Pastikan ENCRYPTION_INDEX_KEY / ENCRYPTION_INDEX_KEY_ID sudah diset sebelum
// menjalankan. Hentikan bila laporan "sisa versi lama" masih > 0, periksa baris yang
// dilewati, lalu ulangi.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	"cbs-core/apps/core-api/internal/config"
	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
)

func main() {
	batch := flag.Int("batch", 200, "jumlah nasabah yang diproses per batch")
	flag.Parse()

	cfg := config.Load()
	if cfg.EncryptionMasterKey == "" {
		log.Fatal("ENCRYPTION_MASTER_KEY wajib diisi untuk menghitung ulang indeks")
	}
	cipher, err := crypto.NewCipherWithIndexKey(
		cfg.EncryptionKeyID, cfg.EncryptionMasterKey, cfg.EncryptionPreviousKey,
		crypto.IndexKeyConfig{
			ActiveKeyID:  cfg.EncryptionIndexKeyID,
			ActiveKey:    cfg.EncryptionIndexKey,
			PreviousKeys: cfg.EncryptionPreviousIndexKeys,
		},
	)
	if err != nil {
		log.Fatalf("konfigurasi enkripsi tidak valid: %v", err)
	}

	db, err := postgres.NewDB(postgres.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	if err != nil {
		log.Fatalf("koneksi database gagal: %v", err)
	}
	defer func() { _ = db.Close() }()

	updated, skipped, err := run(context.Background(), db, cipher, *batch)
	if err != nil {
		log.Fatalf("reindex indeks nasabah gagal: %v", err)
	}
	remaining, err := countStale(context.Background(), db, cipher.IndexKeyVersion())
	if err != nil {
		log.Fatalf("menghitung sisa baris lama gagal: %v", err)
	}
	fmt.Printf("reindex selesai: versi aktif %s, %d diperbarui, %d dilewati, sisa versi lama %d\n",
		cipher.IndexKeyVersion(), updated, skipped, remaining)
	if remaining > 0 {
		fmt.Println("PERHATIAN: masih ada baris ber-versi lama; periksa baris yang dilewati lalu jalankan ulang.")
	}
}

// run memproses baris per batch dengan kursor id menaik. Kursor ini membuat baris
// yang gagal diproses tidak membuat loop tak berujung: baris itu dilewati pada
// eksekusi ini dan dicoba lagi pada eksekusi berikutnya.
func run(ctx context.Context, db *sql.DB, c *crypto.Cipher, batch int) (updated, skipped int, err error) {
	if batch <= 0 {
		batch = 200
	}
	target := c.IndexKeyVersion()
	lastID := uuid.Nil

	for {
		rows, err := db.QueryContext(ctx, `
			SELECT id, full_name_enc, id_card_number_enc, email_enc
			FROM customers
			WHERE index_key_version IS DISTINCT FROM $1 AND id > $2
			ORDER BY id
			LIMIT $3`, target, lastID, batch)
		if err != nil {
			return updated, skipped, err
		}

		type row struct {
			id          uuid.UUID
			fullNameEnc string
			idCardEnc   string
			emailEnc    string
		}
		var records []row
		for rows.Next() {
			var rec row
			if err := rows.Scan(&rec.id, &rec.fullNameEnc, &rec.idCardEnc, &rec.emailEnc); err != nil {
				_ = rows.Close()
				return updated, skipped, err
			}
			records = append(records, rec)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return updated, skipped, err
		}
		_ = rows.Close()
		if len(records) == 0 {
			return updated, skipped, nil
		}
		lastID = records[len(records)-1].id

		for _, rec := range records {
			if err := reindexOne(ctx, db, c, rec.id, rec.fullNameEnc, rec.idCardEnc, rec.emailEnc); err != nil {
				skipped++
				// Hanya id dan galat teknis (format blob), bukan plaintext.
				log.Printf("lewati nasabah %s: %v", rec.id, err)
				continue
			}
			updated++
		}
	}
}

func reindexOne(ctx context.Context, db *sql.DB, c *crypto.Cipher, id uuid.UUID, fullNameEnc, idCardEnc, emailEnc string) error {
	name, err := c.Decrypt(fullNameEnc)
	if err != nil {
		return fmt.Errorf("dekripsi nama: %w", err)
	}
	idCard, err := decryptOrEmpty(c, idCardEnc)
	if err != nil {
		return fmt.Errorf("dekripsi NIK: %w", err)
	}
	email, err := decryptOrEmpty(c, emailEnc)
	if err != nil {
		return fmt.Errorf("dekripsi email: %w", err)
	}

	tokens := domain.NormalizeNameTokens(name)
	tokenIndexes := make([]string, 0, len(tokens))
	for _, token := range tokens {
		tokenIndexes = append(tokenIndexes, c.NameTokenIndex(token))
	}

	var idCardIndex, emailIndex any
	if strings.TrimSpace(idCard) != "" {
		idCardIndex = c.BlindIndex(idCard)
	}
	if strings.TrimSpace(email) != "" {
		emailIndex = c.BlindIndex(email)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
		UPDATE customers
		SET id_card_index = $1, email_index = $2, index_key_version = $3, updated_at = NOW()
		WHERE id = $4`, idCardIndex, emailIndex, c.IndexKeyVersion(), id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM customer_name_tokens WHERE customer_id = $1`, id); err != nil {
		return err
	}
	for _, index := range tokenIndexes {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO customer_name_tokens (customer_id, token_index, index_key_version) VALUES ($1, $2, $3)
			 ON CONFLICT (customer_id, token_index) DO NOTHING`,
			id, index, c.IndexKeyVersion()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func decryptOrEmpty(c *crypto.Cipher, blob string) (string, error) {
	if strings.TrimSpace(blob) == "" {
		return "", nil
	}
	return c.Decrypt(blob)
}

func countStale(ctx context.Context, db *sql.DB, target string) (int, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM customers WHERE index_key_version IS DISTINCT FROM $1`, target).Scan(&n)
	return n, err
}
