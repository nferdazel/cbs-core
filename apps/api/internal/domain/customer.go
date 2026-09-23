package domain

import (
	"context"
	"database/sql"
	"errors"
	"strings"
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
	// ErrInvalidEmail menandai email yang formatnya tidak wajar. Email opsional, jadi
	// error ini hanya muncul bila field memang diisi.
	ErrInvalidEmail = errors.New("format email tidak valid")
)

// ValidateEmail memeriksa format email yang DIISI. Email nasabah bersifat opsional,
// sehingga string kosong (atau hanya spasi) dianggap sah dan tidak diproses lebih lanjut.
//
// Pemeriksaannya sengaja sederhana dan bukan RFC 5322 penuh: tujuannya menangkap salah
// ketik yang jelas (tanpa "@", domain tanpa titik, ada spasi), bukan memutuskan alamat
// eksotis yang jarang dipakai. Menolak terlalu banyak alamat sah lebih merugikan teller.
func ValidateEmail(email string) error {
	trimmed := strings.TrimSpace(email)
	if trimmed == "" {
		return nil
	}
	if strings.ContainsAny(trimmed, " \t\r\n") {
		return ErrInvalidEmail
	}
	at := strings.LastIndex(trimmed, "@")
	if at <= 0 || at == len(trimmed)-1 {
		return ErrInvalidEmail
	}
	local, domainPart := trimmed[:at], trimmed[at+1:]
	if strings.Contains(local, "@") || strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") {
		return ErrInvalidEmail
	}
	if !strings.Contains(domainPart, ".") {
		return ErrInvalidEmail
	}
	return nil
}

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
	// IndexKeyVersion adalah versi kunci indeks yang dipakai membentuk IDCardIndex,
	// EmailIndex, dan NameTokenIndexes. Versi kunci ENKRIPSI tidak perlu kolom
	// terpisah karena sudah tertanam di dalam blob ciphertext. Baris lama bernilai
	// "k1" (kunci enkripsi bawaan) sampai migrasi/reindex mengubahnya.
	IndexKeyVersion string
	// NameTokenIndexes adalah blind index tiap kata nama (lihat NormalizeNameTokens
	// dan Cipher.NameTokenIndex). Kosong berarti nama belum diindeks; repository
	// menuliskannya pada tabel customer_name_tokens.
	NameTokenIndexes []string
	Status           CustomerStatus
	BranchID         *uuid.UUID
	Metadata         map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
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

// UpdateCustomerInput adalah isi perubahan data nasabah. CIF, status, dan cabang
// tidak diubah lewat endpoint ini: CIF adalah identitas tetap, sedangkan status dan
// cabang punya jalurnya sendiri. branch_code sengaja tidak ada agar tidak disalahpahami
// sebagai pemindahan cabang (perilaku create pun mengabaikannya).
type UpdateCustomerInput struct {
	FullName     string         `json:"full_name"`
	IDCardNumber string         `json:"id_card_number"`
	Email        string         `json:"email"`
	PhoneNumber  string         `json:"phone_number"`
	Address      string         `json:"address"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// IDCardDigits adalah panjang NIK yang sah.
const IDCardDigits = 16

// ErrInvalidIDCard menandai NIK yang bentuknya tidak sah.
var ErrInvalidIDCard = errors.New("NIK harus 16 digit angka")

// NormalizeIDCardNumber membuang pemisah yang biasa dituliskan petugas (spasi,
// titik, tanda hubung) lalu memastikan NIK tepat 16 digit.
//
// Normalisasi ini wajib sebelum NIK dienkripsi dan diindeks: blind index dihitung
// dari nilai apa adanya, sehingga NIK yang ditulis dengan spasi akan menghasilkan
// indeks berbeda. Akibatnya nasabah yang sama lolos dari pemeriksaan duplikat dan
// tidak ditemukan saat dicari.
func NormalizeIDCardNumber(raw string) (string, error) {
	out := make([]byte, 0, IDCardDigits)
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch {
		case c >= '0' && c <= '9':
			out = append(out, c)
		case c == ' ' || c == '.' || c == '-':
			// pemisah yang wajar ditulis petugas; diabaikan
		default:
			return "", ErrInvalidIDCard
		}
	}
	if len(out) != IDCardDigits {
		return "", ErrInvalidIDCard
	}
	return string(out), nil
}

// CustomerQuery membatasi daftar nasabah. Field kosong berarti tidak menyaring.
//
// NIK tidak dapat dicari sebagai teks karena kolomnya terenkripsi; pencocokannya
// memakai blind index (HMAC deterministik) yang dihitung service dari kata kunci.
type CustomerQuery struct {
	// CIF mencocokkan awalan nomor CIF.
	CIF string
	// IDCardIndexes adalah kandidat blind index NIK lintas versi kunci indeks
	// (lihat Cipher.BlindIndexCandidates). Pencocokan memakai `= ANY(...)`, sehingga
	// baris yang diindeks dengan kunci lama maupun baru tetap ikut terpilih.
	IDCardIndexes []string
	// NameTokenIndexes adalah kandidat blind index tiap kata nama yang dicari.
	// Elemen luar berpasangan dengan kata (semua kata wajib ada, AND antar kata),
	// elemen dalam berisi kandidat indeks lintas versi untuk kata itu. Bentuk ini
	// membuat "siti rahayu" tetap tidak mencocokkan orang yang hanya bernama "Siti".
	NameTokenIndexes [][]string
}

type CustomerRepository interface {
	Create(ctx context.Context, record *CustomerRecord) error
	// CreateTx menyimpan nasabah di dalam transaksi pemanggil agar pendaftaran dan
	// audit log-nya atomik.
	CreateTx(ctx context.Context, tx *sql.Tx, record *CustomerRecord) error
	GetByID(ctx context.Context, id uuid.UUID) (*CustomerRecord, error)
	GetByCIF(ctx context.Context, cif string) (*CustomerRecord, error)
	// FindByIDCard mencari nasabah lewat blind index NIK tanpa membuka enkripsi.
	// Menerima kandidat lintas versi kunci indeks agar deteksi duplikat tetap
	// menemukan NIK yang sudah terdaftar sebelum kunci indeks diganti.
	FindByIDCard(ctx context.Context, idCardIndexes []string) (*CustomerRecord, error)
	// GetByIDs mengambil banyak nasabah sekaligus untuk menghindari N+1 pada daftar.
	GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*CustomerRecord, error)
	// List mengembalikan daftar nasabah yang boleh dibaca aktor. Filter cabang
	// diterapkan di query agar pagination dan total tetap benar.
	List(ctx context.Context, limit, offset int, q CustomerQuery, actor Actor) ([]CustomerRecord, int, error)
	// ReplaceNameTokens mengganti seluruh token nama nasabah. Dipanggil saat nama
	// diubah, memakai transaksi pemanggil agar token lama tidak tertinggal dan
	// perubahan nama serta tokennya commit bersama. indexKeyVersion mencatat versi
	// kunci indeks yang dipakai token baru.
	ReplaceNameTokens(ctx context.Context, tx *sql.Tx, customerID uuid.UUID, tokenIndexes []string, indexKeyVersion string) error
	// UpdateTx memperbarui kolom terenkripsi nasabah beserta token namanya di dalam
	// transaksi pemanggil, sehingga data, blind index, dan token commit bersama.
	UpdateTx(ctx context.Context, tx *sql.Tx, record *CustomerRecord) error
	UpdateStatus(ctx context.Context, id uuid.UUID, status CustomerStatus) error
}

type CustomerService interface {
	RegisterCustomer(ctx context.Context, input CreateCustomerInput, actor Actor) (*Customer, error)
	// UpdateCustomer mengubah data nasabah yang sudah ada. Nasabah di luar cakupan
	// unit aktor ditolak ErrCrossBranchAccess; NIK/email yang sudah dipakai nasabah
	// lain ditolak agar tidak menyatukan dua orang.
	UpdateCustomer(ctx context.Context, id uuid.UUID, input UpdateCustomerInput, actor Actor) (*Customer, error)
	GetCustomer(ctx context.Context, id uuid.UUID, actor Actor) (*Customer, error)
	ListCustomers(ctx context.Context, page, pageSize int, search string, actor Actor) ([]Customer, int, error)
	// NamesByIDs mengembalikan nama nasabah yang sudah didekripsi untuk pelengkapan tampilan.
	NamesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
}
