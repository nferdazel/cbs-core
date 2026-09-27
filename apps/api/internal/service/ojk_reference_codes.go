package service

import (
	"context"
	"database/sql"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

// ojk_reference_codes.go memuat pemeriksaan tabel referensi sandi OJK yang dipakai
// jalur tulis nasabah (customers.ojk_pihak_lawan_code / ojk_sektor_ekonomi_code) dan
// jalur tulis kredit (loans.ojk_kabupaten_code). Foreign key tetap menjadi penjaga
// terakhir; pemeriksaan di sini ada agar pengguna menerima pesan yang menyebut kolom
// bermasalah, bukan galat foreign key mentah.
//
// Query dipilih dari tabel whitelist, bukan dirangkai dari masukan pengguna.

// rowQuerier adalah bagian *sql.DB / *sql.Tx yang dibutuhkan pemeriksaan referensi.
type rowQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ojkReferenceQueries memetakan tabel referensi OJK yang dikenal ke query
// keberadaannya. Kunci bertipe domain.OJKReferenceTable, jadi nilai tak dikenal
// tidak punya query.
var ojkReferenceQueries = map[domain.OJKReferenceTable]string{
	domain.OJKRefPihakLawan:    "SELECT EXISTS(SELECT 1 FROM ojk_pihak_lawan WHERE code = $1)",
	domain.OJKRefSektorEkonomi: "SELECT EXISTS(SELECT 1 FROM ojk_sektor_ekonomi WHERE code = $1)",
	domain.OJKRefKabupaten:     "SELECT EXISTS(SELECT 1 FROM ojk_kabupaten WHERE code = $1)",
}

// requireOJKReference menolak sandi referensi yang tidak ada di tabel ojk_* dengan
// galat domain yang menyebut kolomnya. Sandi kosong berarti "belum diisi" dan lolos.
func requireOJKReference(ctx context.Context, q rowQuerier, table domain.OJKReferenceTable, code, field string) error {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil
	}
	query, ok := ojkReferenceQueries[table]
	if !ok {
		return domain.OJKReferenceCodeInvalid(field)
	}
	var exists bool
	if err := q.QueryRowContext(ctx, query, code).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return domain.OJKReferenceCodeInvalid(field)
	}
	return nil
}
