package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type CustomerStatus string

const (
	CustomerStatusPendingKYC CustomerStatus = "PENDING_KYC"
	CustomerStatusActive     CustomerStatus = "ACTIVE"
	CustomerStatusBlocked    CustomerStatus = "BLOCKED"
	CustomerStatusClosed     CustomerStatus = "CLOSED"
)

type Customer struct {
	ID           uuid.UUID      `json:"id"`
	CIFNumber    string         `json:"cif_number"`
	FullName     string         `json:"full_name"`
	IDCardNumber string         `json:"id_card_number"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	Address      string         `json:"address"`
	Status       CustomerStatus `json:"status"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type CreateCustomerInput struct {
	FullName     string         `json:"full_name"`
	IDCardNumber string         `json:"id_card_number"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	Address      string         `json:"address"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type CustomerRepository interface {
	Create(ctx context.Context, customer *Customer) error
	GetByID(ctx context.Context, id uuid.UUID) (*Customer, error)
	GetByCIF(ctx context.Context, cif string) (*Customer, error)
	List(ctx context.Context, limit, offset int) ([]Customer, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status CustomerStatus) error
}

type CustomerService interface {
	RegisterCustomer(ctx context.Context, input CreateCustomerInput) (*Customer, error)
	GetCustomer(ctx context.Context, id uuid.UUID) (*Customer, error)
	ListCustomers(ctx context.Context, page, pageSize int) ([]Customer, int, error)
}
