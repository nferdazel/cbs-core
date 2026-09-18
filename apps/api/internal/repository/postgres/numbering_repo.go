package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
)

// NumberingRepository menerbitkan nomor rekening. Serial diambil di dalam transaksi
// pemanggil dengan upsert atomik, sehingga dua pembukaan rekening bersamaan tidak
// mungkin mendapat nomor yang sama.
type NumberingRepository struct {
	db *sql.DB
}

func NewNumberingRepository(db *sql.DB) *NumberingRepository {
	return &NumberingRepository{db: db}
}

// NextAccountNumber mengambil seri berikutnya lalu menyusun nomor rekening lengkap
// dengan cek digit.
func (r *NumberingRepository) NextAccountNumber(ctx context.Context, tx *sql.Tx, product *domain.BankingProduct, branchCode string) (string, error) {
	if tx == nil {
		return "", errors.New("nomor rekening harus diterbitkan di dalam transaksi")
	}

	productCode, err := domain.ProductSequenceForProduct(product)
	if err != nil {
		return "", err
	}

	var serial int64
	err = tx.QueryRowContext(ctx, `
		INSERT INTO account_number_sequences (product_id, branch_code, last_serial, updated_at)
		VALUES ($1, $2, 1, NOW())
		ON CONFLICT (product_id, branch_code)
		DO UPDATE SET last_serial = account_number_sequences.last_serial + 1, updated_at = NOW()
		RETURNING last_serial
	`, product.ID, branchCode).Scan(&serial)
	if err != nil {
		if isUniqueViolation(err) {
			// Cabang belum terdaftar di branches(code).
			return "", fmt.Errorf("kode cabang %s tidak terdaftar", branchCode)
		}
		return "", fmt.Errorf("gagal mengambil nomor urut rekening: %w", err)
	}

	return domain.BuildAccountNumber(branchCode, productCode, serial)
}
