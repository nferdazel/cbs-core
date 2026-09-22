package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
)

// BankProfileRepository membaca dan menulis identitas bank dari tabel bank_profile.
// SQL tetap di repository, bukan di service.
type BankProfileRepository struct {
	db *sql.DB
}

func NewBankProfileRepository(db *sql.DB) *BankProfileRepository {
	return &BankProfileRepository{db: db}
}

const bankProfileColumns = `bank_name, address, city, phone, npwp, updated_at`

func scanBankProfile(row interface {
	Scan(dest ...any) error
}) (*domain.BankProfile, error) {
	var p domain.BankProfile
	if err := row.Scan(&p.Name, &p.Address, &p.City, &p.Phone, &p.NPWP, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *BankProfileRepository) Get(ctx context.Context) (*domain.BankProfile, error) {
	p, err := scanBankProfile(r.db.QueryRowContext(ctx,
		`SELECT `+bankProfileColumns+` FROM bank_profile WHERE id = 1`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// GetForUpdate membaca profil sambil mengunci barisnya di dalam transaksi tulis,
// sehingga audit "sebelum" selalu mencerminkan keadaan yang benar-benar ditimpa.
func (r *BankProfileRepository) GetForUpdate(ctx context.Context, tx any) (*domain.BankProfile, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return nil, fmt.Errorf("bank_profile: transaksi tulis tidak sah")
	}
	p, err := scanBankProfile(sqlTx.QueryRowContext(ctx,
		`SELECT `+bankProfileColumns+` FROM bank_profile WHERE id = 1 FOR UPDATE`))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateTx menyimpan profil bank. Upsert dipakai agar instalasi yang barisnya
// belum ada (mis. migrasi belum dijalankan) tetap dapat mengisi identitas tanpa
// gagal senyap pada UPDATE yang tidak mengenai baris.
func (r *BankProfileRepository) UpdateTx(ctx context.Context, tx any, profile *domain.BankProfile) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok || sqlTx == nil {
		return fmt.Errorf("bank_profile: transaksi tulis tidak sah")
	}
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO bank_profile (id, bank_name, address, city, phone, npwp, updated_at)
		VALUES (1, $1, $2, $3, $4, $5, NOW())
		ON CONFLICT (id) DO UPDATE SET
			bank_name = EXCLUDED.bank_name,
			address   = EXCLUDED.address,
			city      = EXCLUDED.city,
			phone     = EXCLUDED.phone,
			npwp      = EXCLUDED.npwp,
			updated_at = NOW()`,
		profile.Name, profile.Address, profile.City, profile.Phone, profile.NPWP)
	return err
}

var _ domain.BankProfileStore = (*BankProfileRepository)(nil)
