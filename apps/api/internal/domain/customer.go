package domain

import (
	"context"
	"database/sql"
	"errors"
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

var (
	ErrCustomerNotFound    = errors.New("nasabah tidak ditemukan")
	ErrDuplicateIDCard     = errors.New("NIK sudah terdaftar")
	ErrDuplicateEmail      = errors.New("email sudah terdaftar")
	ErrCipherNotConfigured = errors.New("kunci enkripsi data nasabah belum dikonfigurasi")
)

// Customer menyatukan data pribadi. Nilai pribadi (nama, NIK, email, telepon, alamat)
// disimpan terenkripsi di database; struct ini membawa nilai yang sudah didekripsi
// untuk dipakai lapisan atas. Jangan menuliskan struct ini ke log.
type Customer struct {
	ID           uuid.UUID      `json:"id"`
	CIFNumber    string         `json:"cif_number"`
	FullName     string         `json:"full_name"`
	IDCardNumber string         `json:"id_card_number"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	Address      string         `json:"address"`
	Status       CustomerStatus `json:"status"`
	BranchID     *uuid.UUID     `json:"branch_id,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

// CustomerRecord adalah representasi penyimpanan: nilai pribadi dalam bentuk
// terenkripsi beserta blind index untuk pencarian. Dipakai repository, tidak
// diekspos ke API.
type CustomerRecord struct {
	ID              uuid.UUID
	CIFNumber       string
	FullNameEnc     string
	IDCardNumberEnc string
	EmailEnc        string
	PhoneNumberEnc  string
	AddressEnc      string
	IDCardIndex     string
	EmailIndex      string
	Status          CustomerStatus
	BranchID        *uuid.UUID
	Metadata        map[string]any
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type CreateCustomerInput struct {
	FullName     string         `json:"full_name"`
	IDCardNumber string         `json:"id_card_number"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	Address      string         `json:"address"`
	BranchCode   string         `json:"branch_code"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type CustomerRepository interface {
	Create(ctx context.Context, record *CustomerRecord) error
	// CreateTx menyimpan nasabah di dalam transaksi pemanggil agar pendaftaran dan
	// audit log-nya atomik.
	CreateTx(ctx context.Context, tx *sql.Tx, record *CustomerRecord) error
	GetByID(ctx context.Context, id uuid.UUID) (*CustomerRecord, error)
	GetByCIF(ctx context.Context, cif string) (*CustomerRecord, error)
	// FindByIDCard mencari nasabah lewat blind index NIK tanpa membuka enkripsi.
	FindByIDCard(ctx context.Context, idCardIndex string) (*CustomerRecord, error)
	// GetByIDs mengambil banyak nasabah sekaligus untuk menghindari N+1 pada daftar.
	GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*CustomerRecord, error)
	// List mengembalikan daftar nasabah yang boleh dibaca aktor. Filter cabang
	// diterapkan di query agar pagination dan total tetap benar.
	List(ctx context.Context, limit, offset int, actor Actor) ([]CustomerRecord, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status CustomerStatus) error
}

type CustomerService interface {
	RegisterCustomer(ctx context.Context, input CreateCustomerInput, actor Actor) (*Customer, error)
	GetCustomer(ctx context.Context, id uuid.UUID, actor Actor) (*Customer, error)
	ListCustomers(ctx context.Context, page, pageSize int, actor Actor) ([]Customer, int, error)
	// NamesByIDs mengembalikan nama nasabah yang sudah didekripsi untuk pelengkapan tampilan.
	NamesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}
