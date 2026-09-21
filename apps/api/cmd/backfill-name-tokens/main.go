// Perintah backfill-name-tokens mengisi tabel customer_name_tokens untuk nasabah
// yang sudah ada sebelum migrasi 000039.
//
// Token nama adalah HMAC dengan master key enkripsi, sehingga hanya aplikasi yang
// bisa menghitungnya; migrasi SQL tidak mungkin mengisinya. Jalankan sekali setelah
// migrasi 000039 diterapkan:
//
//	cd apps/api && go run ./cmd/backfill-name-tokens
//
// Perintah aman diulang: token yang sudah ada tidak ditimpa (ON CONFLICT DO
// NOTHING). Nasabah yang namanya gagal didekripsi dilewati dan dilaporkan, bukan
// menghentikan seluruh proses.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"cbs-core/apps/core-api/internal/config"
	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
)

func main() {
	cfg := config.Load()

	if cfg.EncryptionMasterKey == "" {
		log.Fatal("ENCRYPTION_MASTER_KEY wajib diisi untuk menghitung token nama")
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
	defer db.Close()

	if err := run(context.Background(), db, cipher); err != nil {
		log.Fatalf("backfill token nama gagal: %v", err)
	}
}

func run(ctx context.Context, db *sql.DB, cipher *crypto.Cipher) error {
	rows, err := db.QueryContext(ctx,
		`SELECT id, full_name_enc FROM customers WHERE full_name_enc IS NOT NULL AND full_name_enc <> ''`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type record struct {
		id          uuid.UUID
		fullNameEnc string
	}
	// Seluruh baris dibaca lebih dulu agar koneksi query tidak terbuka saat menulis
	// token; menghindari deadlock pada driver dengan pool koneksi terbatas.
	var records []record
	for rows.Next() {
		var rec record
		if err := rows.Scan(&rec.id, &rec.fullNameEnc); err != nil {
			return err
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	var processed, inserted, skipped int
	for _, rec := range records {
		name, err := cipher.Decrypt(rec.fullNameEnc)
		if err != nil {
			skipped++
			log.Printf("lewati nasabah %s: gagal mendekripsi nama: %v", rec.id, err)
			continue
		}
		for _, token := range domain.NormalizeNameTokens(name) {
			result, err := db.ExecContext(ctx,
				`INSERT INTO customer_name_tokens (customer_id, token_index, index_key_version) VALUES ($1, $2, $3)
				 ON CONFLICT (customer_id, token_index) DO NOTHING`,
				rec.id, cipher.NameTokenIndex(token), cipher.IndexKeyVersion(),
			)
			if err != nil {
				return fmt.Errorf("nasabah %s: %w", rec.id, err)
			}
			if affected, err := result.RowsAffected(); err == nil && affected > 0 {
				inserted++
			}
		}
		processed++
	}

	fmt.Printf("backfill token nama selesai: %d nasabah diproses, %d token baru, %d dilewati\n",
		processed, inserted, skipped)
	return nil
}
