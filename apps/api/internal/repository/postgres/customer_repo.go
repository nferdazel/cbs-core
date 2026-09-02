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

type CustomerRepository struct {
	db *sql.DB
}

func NewCustomerRepository(db *sql.DB) *CustomerRepository {
	return &CustomerRepository{db: db}
}

func (r *CustomerRepository) Create(ctx context.Context, c *domain.Customer) error {
	metaJSON, err := json.Marshal(c.Metadata)
	if err != nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO customers (id, cif_number, full_name, id_card_number, email, phone_number, address, status, metadata, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`
	_, err = r.db.ExecContext(ctx, query,
		c.ID, c.CIFNumber, c.FullName, c.IDCardNumber, c.Email, c.PhoneNumber, c.Address, c.Status, metaJSON, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert customer: %w", err)
	}
	return nil
}

func (r *CustomerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
	query := `
		SELECT id, cif_number, full_name, id_card_number, email, phone_number, address, status, metadata, created_at, updated_at
		FROM customers WHERE id = $1
	`
	var c domain.Customer
	var metaBytes []byte
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.CIFNumber, &c.FullName, &c.IDCardNumber, &c.Email, &c.PhoneNumber, &c.Address, &c.Status, &metaBytes, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("customer not found")
		}
		return nil, err
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &c.Metadata)
	}
	return &c, nil
}

func (r *CustomerRepository) GetByCIF(ctx context.Context, cif string) (*domain.Customer, error) {
	query := `
		SELECT id, cif_number, full_name, id_card_number, email, phone_number, address, status, metadata, created_at, updated_at
		FROM customers WHERE cif_number = $1
	`
	var c domain.Customer
	var metaBytes []byte
	err := r.db.QueryRowContext(ctx, query, cif).Scan(
		&c.ID, &c.CIFNumber, &c.FullName, &c.IDCardNumber, &c.Email, &c.PhoneNumber, &c.Address, &c.Status, &metaBytes, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("customer not found")
		}
		return nil, err
	}
	if len(metaBytes) > 0 {
		_ = json.Unmarshal(metaBytes, &c.Metadata)
	}
	return &c, nil
}

func (r *CustomerRepository) List(ctx context.Context, limit, offset int) ([]domain.Customer, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM customers").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, cif_number, full_name, id_card_number, email, phone_number, address, status, metadata, created_at, updated_at
		FROM customers
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`
	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var customers []domain.Customer
	for rows.Next() {
		var c domain.Customer
		var metaBytes []byte
		if err := rows.Scan(
			&c.ID, &c.CIFNumber, &c.FullName, &c.IDCardNumber, &c.Email, &c.PhoneNumber, &c.Address, &c.Status, &metaBytes, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		if len(metaBytes) > 0 {
			_ = json.Unmarshal(metaBytes, &c.Metadata)
		}
		customers = append(customers, c)
	}
	return customers, total, nil
}

func (r *CustomerRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.CustomerStatus) error {
	_, err := r.db.ExecContext(ctx, "UPDATE customers SET status = $1, updated_at = NOW() WHERE id = $2", status, id)
	return err
}
