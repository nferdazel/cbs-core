package domain

import (
	"context"
	"database/sql"
)

// AccountNumberGenerator menerbitkan nomor rekening unik per produk per cabang.
// Implementasi harus atomik: dua pembukaan rekening bersamaan tidak boleh mendapat
// nomor yang sama.
type AccountNumberGenerator interface {
	NextAccountNumber(ctx context.Context, tx *sql.Tx, product *BankingProduct, branchCode string) (string, error)
}

// CIFGenerator menerbitkan nomor CIF berurutan. Dipakai service nasabah.
type CIFGenerator interface {
	NextCIF() string
}
