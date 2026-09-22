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
	// ErrOrgUnitLevelInvalid menolak jenjang unit di luar CABANG/AREA/WILAYAH.
	ErrOrgUnitLevelInvalid = errors.New("jenjang unit organisasi tidak dikenal")
	// ErrOrgUnitParentInvalid menolak atasan yang jenjangnya tidak lebih tinggi
	// dari unit anak. Hierarki wajib menurun: wilayah -> area -> cabang.
	ErrOrgUnitParentInvalid = errors.New("atasan harus berjenjang lebih tinggi dari unit anak")
)

// OrgUnitLevel adalah jenjang unit organisasi. CABANG unit operasional yang
// memegang penomoran rekening; AREA dan WILAYAH hanya mengelompokkan unit di
// bawahnya dan OPSIONAL bagi bank. Jenjang dipakai bersama oleh struktur
// (branches.parent_id) dan matriks kewenangan, bukan struktur kedua.
type OrgUnitLevel string

const (
	UnitLevelBranch OrgUnitLevel = "CABANG"
	UnitLevelArea   OrgUnitLevel = "AREA"
	UnitLevelRegion OrgUnitLevel = "WILAYAH"
)

// Valid melaporkan apakah nilai jenjang dikenal.
func (l OrgUnitLevel) Valid() bool {
	switch l {
	case UnitLevelBranch, UnitLevelArea, UnitLevelRegion:
		return true
	default:
		return false
	}
}

// Rank mengurutkan kedalaman jenjang: makin tinggi makin besar. Dipakai untuk
// menolak atasan yang setingkat/lebih rendah tanpa daftar pasangan khusus.
func (l OrgUnitLevel) Rank() int {
	switch l {
	case UnitLevelBranch:
		return 1
	case UnitLevelArea:
		return 2
	case UnitLevelRegion:
		return 3
	default:
		return 0
	}
}

// CanBeParentOf melaporkan apakah unit berjenjang l boleh membawahi anak.
func (l OrgUnitLevel) CanBeParentOf(child OrgUnitLevel) bool {
	return l.Rank() > 0 && l.Rank() > child.Rank()
}

type Branch struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Address      string    `json:"address,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	IsHeadOffice bool      `json:"is_head_office"`
	IsActive     bool      `json:"is_active"`
	// ParentID adalah unit atasan (opsional). NULL berarti unit puncak, yang
	// membuat bank tanpa area/wilayah berperilaku persis seperti sebelumnya.
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	// UnitLevel adalah jenjang unit. Nilai lama adalah CABANG.
	UnitLevel OrgUnitLevel `json:"unit_level"`
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

// CreateOrgUnitInput adalah masukan pengelolaan susunan organisasi (W14).
// ParentCode kosong berarti unit puncak; untuk AREA/WILAYAH atasan tetap boleh
// kosong karena bank dapat memakai hanya salah satu jenjang.
type CreateOrgUnitInput struct {
	Code       string       `json:"code"`
	Name       string       `json:"name"`
	Level      OrgUnitLevel `json:"level"`
	ParentCode string       `json:"parent_code"`
	Address    string       `json:"address"`
	Phone      string       `json:"phone"`
}

// SetOrgUnitParentInput memindahkan sebuah unit ke bawah atasan lain. ParentCode
// kosong berarti melepas unit menjadi puncak (hanya sah untuk CABANG, karena
// AREA/WILAYAH tanpa atasan tetap diperbolehkan pada jenjang mana pun).
type SetOrgUnitParentInput struct {
	ParentCode string `json:"parent_code"`
}

// ValidateOrgUnitCode menegakkan format kode unit non-operasional. Berbeda dari
// cabang (wajib 3 digit untuk awalan nomor rekening), area/wilayah hanya butuh
// kode unik yang tidak kosong sehingga bank bebas memakai penomoran internalnya.
func ValidateOrgUnitCode(code string) error {
	if code == "" {
		return ErrInvalidBranchCode
	}
	return nil
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
	// ResolveScopeCodes mengembalikan kode unit yang boleh diakses aktor ber-kode
	// code: unit itu sendiri ditambah seluruh turunannya. Inilah satu tempat yang
	// menjawab cakupan unit organisasi (cabang/area/wilayah), dipakai middleware
	// untuk mengisi Actor.BranchCodes, sehingga baca dan tulis memakai sumbu yang sama.
	ResolveScopeCodes(ctx context.Context, code string) ([]string, error)
	// SetParentTx memindahkan unit ke bawah atasan (parentID nil = puncak) di
	// dalam transaksi pemanggil agar perubahan dan auditnya commit bersama.
	SetParentTx(ctx context.Context, tx any, id uuid.UUID, parentID *uuid.UUID) error
}

// BranchScopeResolver adalah kontrak yang dipakai middleware untuk memuat
// cakupan unit aktor setiap permintaan. Dipisah dari BranchRepository agar
// middleware tidak bergantung pada seluruh antarmuka repositori.
type BranchScopeResolver interface {
	ResolveScopeCodes(ctx context.Context, code string) ([]string, error)
}

type BranchService interface {
	ListBranches(ctx context.Context) ([]Branch, error)
	CreateBranch(ctx context.Context, input CreateBranchInput, actor Actor) (*Branch, error)
	// ListOrgUnits mengembalikan seluruh unit (semua jenjang) untuk pengelolaan
	// susunan hierarki. Bila tidak ada area/wilayah, isinya sama dengan ListBranches.
	ListOrgUnits(ctx context.Context) ([]Branch, error)
	// CreateOrgUnit membuat cabang/area/wilayah beserta atasannya.
	CreateOrgUnit(ctx context.Context, input CreateOrgUnitInput, actor Actor) (*Branch, error)
	// SetOrgUnitParent memindahkan unit ke bawah atasan lain atau melepasnya ke puncak.
	SetOrgUnitParent(ctx context.Context, code, parentCode string, actor Actor) (*Branch, error)
}
