package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// KelembagaanRepository menyimpan data LAPORAN_KELEMBAGAAN (migrasi 000112):
// jaringan kantor (bank_offices) dan direksi/komisaris/pejabat eksekutif
// (bank_management). SQL tetap di repository, bukan di service.
type KelembagaanRepository struct {
	db *sql.DB
}

func NewKelembagaanRepository(db *sql.DB) *KelembagaanRepository {
	return &KelembagaanRepository{db: db}
}

const listBankOfficesQuery = `
	SELECT id, office_type, code, name, address, city,
	       COALESCE(ojk_kabupaten_code, ''), opened_at, closed_at,
	       status, note, created_at, updated_at
	FROM bank_offices
	ORDER BY office_type, code, name`

// ListOffices membaca seluruh kantor bank-wide.
func (r *KelembagaanRepository) ListOffices(ctx context.Context) ([]domain.BankOffice, error) {
	rows, err := r.db.QueryContext(ctx, listBankOfficesQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.BankOffice
	for rows.Next() {
		var (
			o                  domain.BankOffice
			openedAt, closedAt sql.NullTime
		)
		if err := rows.Scan(
			&o.ID, &o.OfficeType, &o.Code, &o.Name, &o.Address, &o.City,
			&o.OJKKabupatenCode, &openedAt, &closedAt,
			&o.Status, &o.Note, &o.CreatedAt, &o.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if openedAt.Valid {
			t := openedAt.Time
			o.OpenedAt = &t
		}
		if closedAt.Valid {
			t := closedAt.Time
			o.ClosedAt = &t
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

const listBankManagementQuery = `
	SELECT id, category, name, position, ojk_position_code, license_number,
	       license_date, started_at, ended_at, status, note, created_at, updated_at
	FROM bank_management
	ORDER BY category, name`

// ListManagement membaca seluruh direksi/komisaris/pejabat eksekutif bank-wide.
func (r *KelembagaanRepository) ListManagement(ctx context.Context) ([]domain.BankManagement, error) {
	rows, err := r.db.QueryContext(ctx, listBankManagementQuery)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []domain.BankManagement
	for rows.Next() {
		var (
			m                           domain.BankManagement
			licenseDate, started, ended sql.NullTime
		)
		if err := rows.Scan(
			&m.ID, &m.Category, &m.Name, &m.Position, &m.OJKPositionCode,
			&m.LicenseNumber, &licenseDate, &started, &ended,
			&m.Status, &m.Note, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if licenseDate.Valid {
			t := licenseDate.Time
			m.LicenseDate = &t
		}
		if started.Valid {
			t := started.Time
			m.StartedAt = &t
		}
		if ended.Valid {
			t := ended.Time
			m.EndedAt = &t
		}
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// UpsertOfficeTx menyimpan kantor (idempotensi lewat PRIMARY KEY id). Kolom NULL
// ditulis untuk tanggal yang belum diisi; nilai kosong tetap ditulis agar perubahan
// "diisi -> dikosongkan" tercatat.
func (r *KelembagaanRepository) UpsertOfficeTx(ctx context.Context, tx any, o domain.BankOffice, actorID uuid.UUID) error {
	sqlTx, err := requireKelembagaanTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO bank_offices (
			id, office_type, code, name, address, city, ojk_kabupaten_code,
			opened_at, closed_at, status, note, created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW(), $12, $12
		)
		ON CONFLICT (id) DO UPDATE SET
			office_type = EXCLUDED.office_type,
			code = EXCLUDED.code,
			name = EXCLUDED.name,
			address = EXCLUDED.address,
			city = EXCLUDED.city,
			ojk_kabupaten_code = EXCLUDED.ojk_kabupaten_code,
			opened_at = EXCLUDED.opened_at,
			closed_at = EXCLUDED.closed_at,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		o.ID, o.OfficeType, o.Code, o.Name, o.Address, o.City,
		nullableKelembagaanString(o.OJKKabupatenCode),
		nullableKelembagaanTime(o.OpenedAt), nullableKelembagaanTime(o.ClosedAt),
		o.Status, o.Note, nullableKelembagaanUUID(actorID))
	return err
}

// DeleteOfficeTx menghapus kantor; found=false bila barisnya tidak ada.
func (r *KelembagaanRepository) DeleteOfficeTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireKelembagaanTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM bank_offices WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UpsertManagementTx menyimpan satu orang direksi/komisaris/pejabat eksekutif.
func (r *KelembagaanRepository) UpsertManagementTx(ctx context.Context, tx any, m domain.BankManagement, actorID uuid.UUID) error {
	sqlTx, err := requireKelembagaanTx(tx)
	if err != nil {
		return err
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO bank_management (
			id, category, name, position, ojk_position_code, license_number,
			license_date, started_at, ended_at, status, note,
			created_at, updated_at, created_by, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW(), $12, $12
		)
		ON CONFLICT (id) DO UPDATE SET
			category = EXCLUDED.category,
			name = EXCLUDED.name,
			position = EXCLUDED.position,
			ojk_position_code = EXCLUDED.ojk_position_code,
			license_number = EXCLUDED.license_number,
			license_date = EXCLUDED.license_date,
			started_at = EXCLUDED.started_at,
			ended_at = EXCLUDED.ended_at,
			status = EXCLUDED.status,
			note = EXCLUDED.note,
			updated_at = NOW(),
			updated_by = EXCLUDED.updated_by`,
		m.ID, m.Category, m.Name, m.Position, m.OJKPositionCode, m.LicenseNumber,
		nullableKelembagaanTime(m.LicenseDate), nullableKelembagaanTime(m.StartedAt),
		nullableKelembagaanTime(m.EndedAt), m.Status, m.Note, nullableKelembagaanUUID(actorID))
	return err
}

// DeleteManagementTx menghapus satu orang; found=false bila barisnya tidak ada.
func (r *KelembagaanRepository) DeleteManagementTx(ctx context.Context, tx any, id uuid.UUID) (bool, error) {
	sqlTx, err := requireKelembagaanTx(tx)
	if err != nil {
		return false, err
	}
	res, err := sqlTx.ExecContext(ctx, `DELETE FROM bank_management WHERE id = $1`, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// requireKelembagaanTx memastikan transaksi tulis sah, konsisten dengan pola
// repository CKPN/OJK profil.
func requireKelembagaanTx(tx any) (*sql.Tx, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("kelembagaan: transaksi tulis tidak sah")
	}
	return sqlTx, nil
}

// nullableKelembagaanString menulis NULL untuk string kosong (belum diisi).
func nullableKelembagaanString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullableKelembagaanTime menulis NULL untuk tanggal yang belum diisi.
func nullableKelembagaanTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}

// nullableKelembagaanUUID menulis NULL untuk aktor tanpa UUID (mis. sistem): kolomnya
// FK ke staff_users, sehingga UUID nol akan ditolak sebagai staf yang tidak ada.
func nullableKelembagaanUUID(id uuid.UUID) any {
	if id == uuid.Nil {
		return nil
	}
	return id
}

var _ domain.KelembagaanRepository = (*KelembagaanRepository)(nil)
