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
// Kegagalan membaca sequence dikembalikan sebagai error, bukan nomor cadangan:
// CIF ganda membuat dua nasabah tampak sebagai satu orang.
type CIFGenerator interface {
	NextCIF() (string, error)
}
