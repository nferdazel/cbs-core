package domain

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Panjang maksimum bidang identitas bank. Batas ini menolak masukan yang jelas
// tidak masuk akal (mis. tempelan berkas) tanpa mengubah data yang sudah wajar,
// sehingga bank yang sudah mengisi profilnya tidak kehilangan akses menyimpan.
const (
	BankProfileNameMaxLen    = 150
	BankProfileAddressMaxLen = 500
	BankProfileCityMaxLen    = 100
	BankProfilePhoneMaxLen   = 50
	BankProfileNPWPMaxLen    = 30
)

var (
	// ErrBankProfileNameRequired menolak profil tanpa nama bank. Nama adalah
	// identitas minimum yang dipakai dokumen cetak dan endpoint /app-info, dan
	// inilah inti W16: profil kosong harus ditolak dengan pesan yang jelas.
	ErrBankProfileNameRequired = NewLocalizedError("bank_profile_name_required", "nama bank wajib diisi")
	// ErrBankProfileNameTooShort menolak nama yang terlalu pendek untuk menjadi
	// identitas badan hukum.
	ErrBankProfileNameTooShort = NewLocalizedError("bank_profile_name_too_short", "nama bank terlalu pendek (minimal 3 karakter)")
	// ErrBankProfileEmpty menolak permintaan ubah tanpa satu bidang pun, agar
	// pemanggil tidak menganggap aksi kosong sebagai perubahan tersimpan.
	ErrBankProfileEmpty = NewLocalizedError("bank_profile_empty", "tidak ada bidang profil bank yang dikirim")
	// ErrBankProfileFieldTooLong menolak bidang yang melampaui batas kolomnya.
	// Pesannya menyebut bidang dan batasnya agar operator dapat memperbaiki.
	ErrBankProfileFieldTooLong = NewLocalizedError("bank_profile_field_too_long", "nilai identitas bank terlalu panjang")
	// ErrBankProfileNPWPInvalid dan ErrBankProfilePhoneInvalid menolak karakter
	// yang tidak mungkin ada pada identitas bank. Format nama/alamat bebas karena
	// bank memakai ejaan sendiri.
	ErrBankProfileNPWPInvalid  = NewLocalizedError("bank_profile_npwp_invalid", "NPWP hanya boleh berisi angka, titik, dan tanda hubung")
	ErrBankProfilePhoneInvalid = NewLocalizedError("bank_profile_phone_invalid", "nomor telepon hanya boleh berisi angka, spasi, dan tanda + - ( ) .") //nolint:staticcheck // pesan operator berbahasa Indonesia; tanda baca bagian dari daftar karakter yang sah
)

// BankProfile adalah identitas bank yang dipakai dokumen cetak dan endpoint
// /app-info. Data ini bersifat konfigurasi: operator mengisinya lewat API/web,
// bukan literal di kode. Nilai nama kosong berarti profil belum dikonfigurasi dan
// pemanggil wajib menandainya, bukan mengarang nama.
type BankProfile struct {
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	City      string    `json:"city"`
	Phone     string    `json:"phone"`
	NPWP      string    `json:"npwp"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Configured melaporkan apakah identitas bank minimum (nama) sudah diisi.
func (p *BankProfile) Configured() bool {
	return p != nil && strings.TrimSpace(p.Name) != ""
}

// Normalize merapikan spasi di tepi tiap bidang. Dipanggil sebelum validasi dan
// penyimpanan supaya spasi tak sengaja tidak tersimpan sebagai bagian identitas.
func (p *BankProfile) Normalize() {
	if p == nil {
		return
	}
	p.Name = strings.TrimSpace(p.Name)
	p.Address = strings.TrimSpace(p.Address)
	p.City = strings.TrimSpace(p.City)
	p.Phone = strings.TrimSpace(p.Phone)
	p.NPWP = strings.TrimSpace(p.NPWP)
}

// UpdateBankProfileInput adalah masukan ubah profil bank. Bidang bertipe pointer
// agar hanya bidang yang benar-benar dikirim yang diubah; bidang yang tidak
// disebut dipertahankan dari nilai tersimpan, sehingga API tidak menghapus data
// yang tidak sengaja tidak ikut terkirim.
type UpdateBankProfileInput struct {
	Name    *string `json:"name,omitempty"`
	Address *string `json:"address,omitempty"`
	City    *string `json:"city,omitempty"`
	Phone   *string `json:"phone,omitempty"`
	NPWP    *string `json:"npwp,omitempty"`
}

// IsEmpty melaporkan tidak ada bidang pun yang dikirim.
func (in UpdateBankProfileInput) IsEmpty() bool {
	return in.Name == nil && in.Address == nil && in.City == nil && in.Phone == nil && in.NPWP == nil
}

// ValidateBankProfile menolak profil yang kosong atau tidak masuk akal. Dipanggil
// pada gabungan nilai tersimpan + masukan, sehingga hasil akhir yang tersimpan
// selalu punya nama dan panjang bidang yang wajar.
func ValidateBankProfile(p *BankProfile) error {
	if p == nil {
		return ErrBankProfileNameRequired
	}
	p.Normalize()
	if p.Name == "" {
		return ErrBankProfileNameRequired
	}
	if len([]rune(p.Name)) < 3 {
		return ErrBankProfileNameTooShort
	}
	if len([]rune(p.Name)) > BankProfileNameMaxLen {
		return fmt.Errorf("%w: nama bank maksimum %d karakter", ErrBankProfileFieldTooLong, BankProfileNameMaxLen)
	}
	lengths := []struct {
		label string
		value string
		max   int
	}{
		{"alamat", p.Address, BankProfileAddressMaxLen},
		{"kota", p.City, BankProfileCityMaxLen},
		{"nomor telepon", p.Phone, BankProfilePhoneMaxLen},
		{"NPWP", p.NPWP, BankProfileNPWPMaxLen},
	}
	for _, l := range lengths {
		if len([]rune(l.value)) > l.max {
			return fmt.Errorf("%w: %s maksimum %d karakter", ErrBankProfileFieldTooLong, l.label, l.max)
		}
	}
	if p.Phone != "" && !allowedChars(p.Phone, "0123456789 +-().") {
		return ErrBankProfilePhoneInvalid
	}
	if p.NPWP != "" && !allowedChars(p.NPWP, "0123456789.- ") {
		return ErrBankProfileNPWPInvalid
	}
	return nil
}

// allowedChars melaporkan apakah seluruh karakter ada di himpunan yang diizinkan.
func allowedChars(value, allowed string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) {
			continue
		}
		if !strings.ContainsRune(allowed, r) {
			return false
		}
	}
	return true
}

// BankProfileRepository adalah pembacaan profil bank. Dipakai dokumen cetak,
// laporan OJK, dan identitas aplikasi. Antarmuka baca dipisah dari jalur tulis
// agar pemakai baca tidak ikut bergantung pada operasi tulis.
type BankProfileRepository interface {
	// Get mengembalikan profil bank. (nil, nil) berarti baris profil belum ada,
	// sehingga pemanggil dapat membedakan "belum dikonfigurasi" dari error database.
	Get(ctx context.Context) (*BankProfile, error)
}

// BankProfileWriteRepository adalah jalur tulis profil bank. Keduanya dibungkus
// dalam satu transaksi dengan penulisan audit oleh service.
type BankProfileWriteRepository interface {
	// GetForUpdate membaca profil sambil mengunci barisnya, sehingga nilai
	// "sebelum" pada audit mencerminkan keadaan yang benar-benar ditimpa.
	GetForUpdate(ctx context.Context, tx any) (*BankProfile, error)
	// UpdateTx menyimpan profil (upsert baris tunggal id = 1).
	UpdateTx(ctx context.Context, tx any, profile *BankProfile) error
}

// BankProfileStore melengkapi baca dan tulis profil bank pada satu repositori.
type BankProfileStore interface {
	BankProfileRepository
	BankProfileWriteRepository
}

// BankProfileService mengelola identitas bank: membaca untuk pengaturan dan
// mengubah dengan validasi serta audit dalam satu transaksi.
type BankProfileService interface {
	Get(ctx context.Context) (*BankProfile, error)
	Update(ctx context.Context, input UpdateBankProfileInput, actor Actor) (*BankProfile, error)
}
