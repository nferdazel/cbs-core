package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrBranchNotFound = errors.New("cabang tidak ditemukan")
	// ErrBranchCodeExists menolak pembuatan cabang dengan kode yang sudah dipakai.
	ErrBranchCodeExists = errors.New("kode cabang sudah terpakai")
	// ErrBranchNameRequired menolak cabang tanpa nama.
	ErrBranchNameRequired = errors.New("nama cabang wajib diisi")
	// ErrBranchHeadOfficeNotAllowed menolak pembuatan kantor pusat lewat API.
	// Kantor pusat adalah data fondasi (hanya satu, menentukan atribusi pelaku
	// lintas cabang), sehingga dibuat lewat migrasi/seed, bukan oleh operator.
	// Cabang biasa yang dibuat lewat API cukup dengan is_head_office=false.
	ErrBranchHeadOfficeNotAllowed = errors.New("kantor pusat tidak dapat dibuat lewat API; is_head_office harus false")
)

type Branch struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Address      string    `json:"address,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	IsHeadOffice bool      `json:"is_head_office"`
	IsActive     bool      `json:"is_active"`
}

// CreateBranchInput adalah masukan pembuatan cabang. IsActive tidak diinput:
// cabang baru selalu aktif, penonaktifan menyusul lewat jalur terpisah.
type CreateBranchInput struct {
	Code         string `json:"code"`
	Name         string `json:"name"`
	Address      string `json:"address"`
	Phone        string `json:"phone"`
	IsHeadOffice bool   `json:"is_head_office"`
}

// ValidateBranchCode menegakkan format kode cabang. Kode dipakai sebagai 3 digit
// pertama nomor rekening (lihat BuildAccountNumber), jadi wajib tepat 3 angka.
func ValidateBranchCode(code string) error {
	if !isDigits(code, accountBranchDigits) {
		return ErrInvalidBranchCode
	}
	return nil
}

type BranchRepository interface {
	List(ctx context.Context) ([]Branch, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Branch, error)
	GetByCode(ctx context.Context, code string) (*Branch, error)
	Create(ctx context.Context, branch *Branch) error
	// CreateTx menyimpan cabang di dalam transaksi pemanggil agar penulisan cabang
	// dan jejak auditnya commit bersama (tidak ada cabang tanpa audit bila audit gagal).
	CreateTx(ctx context.Context, tx any, branch *Branch) error
}

type BranchService interface {
	ListBranches(ctx context.Context) ([]Branch, error)
	CreateBranch(ctx context.Context, input CreateBranchInput, actor Actor) (*Branch, error)
}
