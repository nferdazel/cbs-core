package domain

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// ojk_reference_codes.go membuka pengisian sandi referensi/inline OJK lewat API
// untuk kolom yang sebelumnya hanya dapat diisi lewat SQL/seed (migrasi 000104 dan
// 000105):
//
//   - customers.ojk_pihak_lawan_code    (Form 06.00 XVIII, FK ojk_pihak_lawan)
//   - customers.ojk_sektor_ekonomi_code (Form 06.00 XX, FK ojk_sektor_ekonomi)
//   - customers.ojk_hubungan_bank_code  (Form 06.00 IX, inline 11/12/20)
//   - loans.ojk_jenis_penggunaan_code   (Form 06.00 VIII, inline 10/20/31/32/35/39)
//   - loans.ojk_periode_pembayaran_code (Form 06.00 XI, inline 1..8)
//   - loans.ojk_kabupaten_code          (Form 06.00 XXII, FK ojk_kabupaten)
//
// Sandi inline divalidasi terhadap himpunan yang diizinkan di bawah ini (konstanta
// Go, bukan literal yang tersebar). Sandi referensi wajib ada di tabel ojk_* dan
// diperiksa service lewat foreign key serta pemeriksaan eksplisit. Tidak ada nilai
// yang dikarang: kosong berarti "belum diisi" dan laporan menulis "-".

// OJKReferenceTable adalah nama tabel referensi sandi OJK (Lampiran 02/03/05 SEOJK
// 16/2024). Nilainya dipakai memilih query dan pesan galat; bukan masukan pengguna.
type OJKReferenceTable string

const (
	OJKRefPihakLawan    OJKReferenceTable = "ojk_pihak_lawan"
	OJKRefSektorEkonomi OJKReferenceTable = "ojk_sektor_ekonomi"
	OJKRefKabupaten     OJKReferenceTable = "ojk_kabupaten"
)

// Nama kolom sandi OJK dipakai pada pesan galat agar pengguna tahu sandi mana yang
// bermasalah. Nilai ini sama dengan nama kolom di database, bukan teks tampilan.
const (
	OJKJenisPenggunaanField   = "ojk_jenis_penggunaan_code"
	OJKPeriodePembayaranField = "ojk_periode_pembayaran_code"
	OJKKabupatenField         = "ojk_kabupaten_code"
	OJKPihakLawanField        = "ojk_pihak_lawan_code"
	OJKSektorEkonomiField     = "ojk_sektor_ekonomi_code"
	OJKHubunganBankField      = "ojk_hubungan_bank_code"
)

// Himpunan sandi inline Form 06.00-2 (Lampiran II SEOJK 16/2024). Peta membuat
// pemeriksaan O(1) dan menghindari literal yang tersebar di validasi.
var (
	ojkJenisPenggunaanAllowed = map[string]struct{}{
		"10": {}, "20": {}, "31": {}, "32": {}, "35": {}, "39": {},
	}
	ojkPeriodePembayaranAllowed = map[string]struct{}{
		"1": {}, "2": {}, "3": {}, "4": {}, "5": {}, "6": {}, "7": {}, "8": {},
	}
	ojkHubunganBankAllowed = map[string]struct{}{
		"11": {}, "12": {}, "20": {},
	}
)

// validateOJKInline menerima nilai kosong (belum diisi) atau sandi yang ada pada
// himpunan yang diizinkan. Spasi tepi dibuang lebih dulu.
func validateOJKInline(raw string, allowed map[string]struct{}, field string) error {
	code := strings.TrimSpace(raw)
	if code == "" {
		return nil
	}
	if _, ok := allowed[code]; !ok {
		return OJKInlineCodeInvalid(field)
	}
	return nil
}

// ValidateOJKHubunganBankCode memeriksa sandi inline Hubungan dengan Bank nasabah
// (Form 06.00 kolom IX: 11/12/20).
func ValidateOJKHubunganBankCode(raw string) error {
	return validateOJKInline(raw, ojkHubunganBankAllowed, OJKHubunganBankField)
}

// UpdateOJKLoanCodesInput adalah isi PUT sandi OJK per kredit. Pointer nil berarti
// "jangan ubah"; string kosong berarti "kosongkan" (belum diisi). Semua bidang
// opsional agar klien web dapat mengirim satu bidang pada satu waktu.
type UpdateOJKLoanCodesInput struct {
	OJKJenisPenggunaanCode   *string `json:"ojk_jenis_penggunaan_code,omitempty"`
	OJKPeriodePembayaranCode *string `json:"ojk_periode_pembayaran_code,omitempty"`
	OJKKabupatenCode         *string `json:"ojk_kabupaten_code,omitempty"`
}

// IsEmpty melaporkan apakah tidak ada satu bidang pun yang dikirim.
func (in UpdateOJKLoanCodesInput) IsEmpty() bool {
	return in.OJKJenisPenggunaanCode == nil &&
		in.OJKPeriodePembayaranCode == nil &&
		in.OJKKabupatenCode == nil
}

// Validate memeriksa sandi inline yang dikirim. Sandi referensi (kabupaten) tidak
// diperiksa di sini karena harus dibandingkan dengan tabel referensi lewat service.
func (in UpdateOJKLoanCodesInput) Validate() error {
	if in.OJKJenisPenggunaanCode != nil {
		if err := validateOJKInline(*in.OJKJenisPenggunaanCode, ojkJenisPenggunaanAllowed, OJKJenisPenggunaanField); err != nil {
			return err
		}
	}
	if in.OJKPeriodePembayaranCode != nil {
		if err := validateOJKInline(*in.OJKPeriodePembayaranCode, ojkPeriodePembayaranAllowed, OJKPeriodePembayaranField); err != nil {
			return err
		}
	}
	return nil
}

// OJKLoanCodesStore membaca kredit dan menulis sandi OJK-nya. Implementasinya adalah
// LoanRepository postgres yang sama; interface sempit ini menjaga agar layanan sandi
// tidak bergantung pada seluruh domain.LoanRepository.
type OJKLoanCodesStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Loan, error)
	// UpdateOJKLoanCodesTx menyimpan hanya sandi yang dikirim (pointer non-nil) di
	// dalam transaksi pemanggil. Pointer nil tidak menyentuh kolomnya.
	UpdateOJKLoanCodesTx(ctx context.Context, tx any, id uuid.UUID, input UpdateOJKLoanCodesInput) error
}

// OJKLoanCodesService mengelola sandi referensi/inline OJK per kredit: memvalidasi,
// menyimpan, dan mengaudit dalam satu transaksi.
type OJKLoanCodesService interface {
	// Update mengubah sandi OJK satu kredit dan mengembalikan kredit yang sudah
	// diperbarui. Kredit di luar cakupan unit aktor ditolak ErrCrossBranchAccess.
	Update(ctx context.Context, loanID uuid.UUID, input UpdateOJKLoanCodesInput, actor Actor) (*Loan, error)
}
