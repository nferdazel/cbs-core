package postgres

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
)

// OJKReferenceRepository menyajikan daftar sandi referensi OJK (Lampiran 02/03 SEOJK
// No. 16/SEOJK.03/2024, migrasi 000102) sebagai bacaan untuk pemilih UI. Tabel diisi
// seed migrasi; repositori ini tidak menulis apa pun.
type OJKReferenceRepository struct {
	db *sql.DB
}

func NewOJKReferenceRepository(db *sql.DB) *OJKReferenceRepository {
	return &OJKReferenceRepository{db: db}
}

// listOJKReferenceQueryPihakLawan membaca Lampiran 02 (Daftar Sandi Pihak Lawan),
// urutan deterministik menurut sandi.
const listOJKReferenceQueryPihakLawan = `
	SELECT code, label
	FROM ojk_pihak_lawan
	ORDER BY code`

// ListOJKCreditorGroups mengembalikan Lampiran 02 (Bank Indonesia, BPR, BPRS, bank
// umum, pemerintah, dll.) untuk pemilih sandi "Gol. Kreditur"/"Pihak Lawan".
func (r *OJKReferenceRepository) ListOJKCreditorGroups(ctx context.Context) ([]domain.OJKReferenceOption, error) {
	rows, err := r.db.QueryContext(ctx, listOJKReferenceQueryPihakLawan)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.OJKReferenceOption
	for rows.Next() {
		var item domain.OJKReferenceOption
		if err := rows.Scan(&item.Code, &item.Name); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// listOJKReferenceQueryKabupaten membaca Lampiran 03 (Daftar Sandi Kabupaten/Kota)
// beserta provinsi pengelompoknya, urutan deterministik menurut sandi.
const listOJKReferenceQueryKabupaten = `
	SELECT code, label, COALESCE(provinsi, '')
	FROM ojk_kabupaten
	ORDER BY code`

// ListOJKRegencies mengembalikan Lampiran 03 (kabupaten/kota + provinsi) untuk pemilih
// sandi "Lokasi".
func (r *OJKReferenceRepository) ListOJKRegencies(ctx context.Context) ([]domain.OJKReferenceOption, error) {
	rows, err := r.db.QueryContext(ctx, listOJKReferenceQueryKabupaten)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.OJKReferenceOption
	for rows.Next() {
		var item domain.OJKReferenceOption
		if err := rows.Scan(&item.Code, &item.Name, &item.Province); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
