package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// CustomerRepository menyimpan data nasabah apa adanya. Nilai pribadi sudah
// terenkripsi saat mencapai lapisan ini; repository tidak memegang kunci maupun
// mengetahui plaintext. Pencarian NIK/email memakai blind index.
type CustomerRepository struct {
	db *sql.DB
}

func NewCustomerRepository(db *sql.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

const customerColumns = `id, cif_number, full_name_enc, id_card_number_enc, email_enc,
	phone_number_enc, address_enc, id_card_index, email_index, status, branch_id,
	metadata, created_at, updated_at`

func (r *CustomerRepository) Create(ctx context.Context, c *domain.CustomerRecord) error {
	return r.executeCreate(ctx, r.db, c)
}

// CreateTx menyimpan nasabah di dalam transaksi pemanggil, sehingga pendaftaran
// nasabah dan audit log-nya commit bersama.
func (r *CustomerRepository) CreateTx(ctx context.Context, tx *sql.Tx, c *domain.CustomerRecord) error {
	return r.executeCreate(ctx, tx, c)
}

func (r *CustomerRepository) executeCreate(ctx context.Context, exec interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, c *domain.CustomerRecord) error {
	metaJSON, err := json.Marshal(c.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO customers (
			id, cif_number, full_name_enc, id_card_number_enc, email_enc,
			phone_number_enc, address_enc, id_card_index, email_index,
			status, branch_id, metadata, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
	`
	_, err = exec.ExecContext(ctx, query,
		c.ID, c.CIFNumber, c.FullNameEnc, c.IDCardNumberEnc, c.EmailEnc,
		c.PhoneNumberEnc, c.AddressEnc, c.IDCardIndex, c.EmailIndex,
		c.Status, c.BranchID, metaJSON, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert customer: %w", err)
	}
	return nil
}

func (r *CustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.CustomerRecord, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE id = $1`
	return r.scanOne(ctx, query, id)
}

func (r *CustomerRepository) GetByCIF(ctx context.Context, cif string) (*domain.CustomerRecord, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE cif_number = $1`
	return r.scanOne(ctx, query, cif)
}

func (r *CustomerRepository) FindByIDCard(ctx context.Context, idCardIndex string) (*domain.CustomerRecord, error) {
	query := `SELECT ` + customerColumns + ` FROM customers WHERE id_card_index = $1`
	return r.scanOne(ctx, query, idCardIndex)
}

func (r *CustomerRepository) List(ctx context.Context, limit, offset int, q domain.CustomerQuery, actor domain.Actor) ([]domain.CustomerRecord, int, error) {
	where, whereArgs := branchReadClause("branch_id", actor)

	// Pencarian digabung dengan filter cabang, dan COUNT memakai klausa yang sama
	// sehingga total pagination tetap benar saat pencarian aktif.
	if pattern := likePrefixPattern(q.CIF); pattern != "" {
		whereArgs = append(whereArgs, pattern)
		where = andCondition(where, fmt.Sprintf("cif_number ILIKE $%d ESCAPE '\\'", len(whereArgs)))
	}
	if q.IDCardIndex != "" {
		whereArgs = append(whereArgs, q.IDCardIndex)
		where = andCondition(where, fmt.Sprintf("id_card_index = $%d", len(whereArgs)))
	}

	countQuery := "SELECT COUNT(*) FROM customers"
	if where != "" {
		countQuery += " WHERE " + where
	}
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, whereArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + customerColumns + ` FROM customers`
	if where != "" {
		query += " WHERE " + where
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	args := append(append([]any{}, whereArgs...), limit, offset)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var customers []domain.CustomerRecord
	for rows.Next() {
		rec, err := scanCustomer(rows)
		if err != nil {
			return nil, 0, err
		}
		customers = append(customers, *rec)
	}
	return customers, total, rows.Err()
}

func (r *CustomerRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.CustomerStatus) error {
	_, err := r.db.ExecContext(ctx, "UPDATE customers SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (r *CustomerRepository) scanOne(ctx context.Context, query string, arg any) (*domain.CustomerRecord, error) {
	rec, err := scanCustomer(r.db.QueryRowContext(ctx, query, arg))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrCustomerNotFound
		}
		return nil, err
	}
	return rec, nil
}

func scanCustomer(row rowScanner) (*domain.CustomerRecord, error) {
	var c domain.CustomerRecord
	var metaBytes []byte
	var fullName, idCard, email, phone, address, idCardIdx, emailIdx sql.NullString

	if err := row.Scan(
		&c.ID, &c.CIFNumber, &fullName, &idCard, &email, &phone, &address,
		&idCardIdx, &emailIdx, &c.Status, &c.BranchID, &metaBytes, &c.CreatedAt, &c.UpdatedAt,
	); err != nil {
		return nil, err
	}

	c.FullNameEnc = fullName.String
	c.IDCardNumberEnc = idCard.String
	c.EmailEnc = email.String
	c.PhoneNumberEnc = phone.String
	c.AddressEnc = address.String
	c.IDCardIndex = idCardIdx.String
	c.EmailIndex = emailIdx.String

	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &c.Metadata)
	}
	return &c, nil
}
