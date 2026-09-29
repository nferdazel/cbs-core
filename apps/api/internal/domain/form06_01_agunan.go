package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// form06_01_agunan.go memuat fondasi data Form 06.01 "DAFTAR AGUNAN", Lampiran II
// SEOJK No. 16/SEOJK.03/2024.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 06.01 "DAFTAR AGUNAN": PDF #page 169 (hlm. tercetak 117) susunan kolom,
//     sandi PDF #page 170 (hlm. 118), penjelasan PDF #page 171-172 (hlm. 119-120).
//     Delapan kolom: I Sandi Kantor, II Kode Register/Nomor Agunan, III No. Rekening,
//     IV Jenis Agunan, V Alamat Agunan, VI Nilai yang Diagunkan,
//     VII Nilai Agunan (Nominal + Penilai + Tanggal Penilaian Terakhir),
//     VIII Nilai yang Diperhitungkan untuk PPKA. ADA baris JUMLAH.
//   - Kolom VII.b Penilai: 1 Penilai Independen, 2 Internal BPR (PDF #170).
//   - Kolom IV mengacu Lampiran 01 "Daftar Sandi Jenis Agunan" (PDF #page 301,
//     hlm. cetak 249): sandi 3 digit; kategori induk LIKUID (101-103) dan NON LIKUID (201+).
//
// KEPUTUSAN SME 29 Sep 2026 atas inkonsistensi internal PDF (CELAH-FORM-OJK §3 poin 6):
// sel "Likuid | Non Likuid" pada halaman susunan BUKAN kolom kesembilan — daftar sandi
// resmi berhenti di kolom VIII, dan penjelasan meletakkan Likuid/Non Likuid sebagai
// rincian DI DALAM kolom IV. Keduanya kategori induk dari sandi 3 digit Lampiran 01.
// Konfirmasi OJK tidak lagi diperlukan untuk butir ini.
//
// Prinsip yang dipegang berkas ini:
//   - Seluruh kolom angka adalah ISIAN BANK; form tidak memberi rumus pengikat, sehingga
//     laporan tidak menurunkan apa pun kecuali yang dinyatakan di sini.
//   - Kolom II Kode Register/Nomor Agunan wajib, unik, dan tidak boleh dipakai ulang
//     (no reuse/no recycle); nilai yang sudah dilaporkan tidak boleh berubah (PDF #171).
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal (bank_offices, migrasi
//     000112), bukan dari register.

// Sandi baku Form 06.01 kolom VII.b Penilai (PDF #170).
const (
	AgunanPenilaiIndependen = "1"
	AgunanPenilaiInternal   = "2"
)

// AgunanRegisterMax adalah batas panjang kode register/nomor agunan yang disimpan.
const AgunanRegisterMax = 64

var (
	// ErrAgunanInputInvalid menandai data agunan Form 06.01 tidak sah.
	ErrAgunanInputInvalid = NewLocalizedError("agunan_input_invalid",
		"data agunan Form 06.01 tidak valid")
	// ErrAgunanNotFound menandai agunan yang hendak diubah tidak ada.
	ErrAgunanNotFound = NewLocalizedError("agunan_not_found",
		"data agunan tidak ditemukan")
	// ErrAgunanRegisterUsed menandai Kode Register/Nomor Agunan yang sudah dipakai agunan
	// lain; PDF #171 melarang pemakaian ulang (no reuse/no recycle).
	ErrAgunanRegisterUsed = NewLocalizedError("agunan_register_used",
		"kode register/nomor agunan sudah dipakai agunan lain dan tidak boleh dipakai ulang")
	// ErrAgunanBankWide menandai permintaan daftar agunan oleh peran yang tidak berwenang
	// atas seluruh bank. Form 06.01 bank-wide.
	ErrAgunanBankWide = NewLocalizedError("agunan_bank_wide",
		"daftar agunan bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
)

// AgunanRow adalah satu agunan untuk Form 06.01 (satu baris tabel). Ini proyeksi baca dari
// loan_collaterals beserta loans.loan_number; bukan tabel baru.
type AgunanRow struct {
	ID uuid.UUID `json:"id"`
	// LoanNumber kolom III: nomor rekening fasilitas kredit (dari loans.loan_number).
	LoanNumber string `json:"loan_number"`
	// KodeRegister kolom II: unik, no-reuse.
	KodeRegister string `json:"kode_register"`
	// JenisAgunanCode kolom IV: sandi Lampiran 01 (3 digit), teks apa adanya.
	JenisAgunanCode string `json:"jenis_agunan_code"`
	// AlamatAgunan kolom V.
	AlamatAgunan string `json:"alamat_agunan"`
	// NilaiDiagunkan kolom VI (rupiah penuh): nilai yang diikat sebagai jaminan.
	NilaiDiagunkan decimal.Decimal `json:"nilai_diagunkan"`
	// NilaiAgunan kolom VII.a Nominal (rupiah penuh): taksasi.
	NilaiAgunan decimal.Decimal `json:"nilai_agunan"`
	// PenilaiCode kolom VII.b: 1 Penilai Independen, 2 Internal BPR.
	PenilaiCode string `json:"penilai_code"`
	// TanggalPenilaian kolom VII.c: tanggal penilaian terakhir.
	TanggalPenilaian time.Time `json:"tanggal_penilaian"`
	// PPKAAmount kolom VIII (rupiah penuh): isian bank, tidak diturunkan.
	PPKAAmount decimal.Decimal `json:"ppka_amount"`
	// Status baris agunan (ACTIVE/RELEASED dari loan_collaterals.status). Hanya ACTIVE
	// yang dilaporkan.
	Status string `json:"status"`
}

// UpdateAgunanInput adalah isi yang dapat diubah bank untuk satu agunan Form 06.01. Hanya
// bidang Form 06.01; identitas operasional agunan lain tidak disentuh dari sini.
type UpdateAgunanInput struct {
	KodeRegister     string          `json:"kode_register"`
	JenisAgunanCode  string          `json:"jenis_agunan_code"`
	AlamatAgunan     string          `json:"alamat_agunan"`
	NilaiDiagunkan   decimal.Decimal `json:"nilai_diagunkan"`
	NilaiAgunan      decimal.Decimal `json:"nilai_agunan"`
	PenilaiCode      string          `json:"penilai_code"`
	TanggalPenilaian string          `json:"tanggal_penilaian"`
	PPKAAmount       decimal.Decimal `json:"ppka_amount"`
}

// BuildAgunanUpdate memvalidasi masukan Form 06.01 dan membentuk nilai yang siap disimpan.
// Kolom II wajib (PDF #171 "Kolom ini bersifat mandatory") dan maksimal AgunanRegisterMax.
func BuildAgunanUpdate(in UpdateAgunanInput) (UpdateAgunanInput, error) {
	out := UpdateAgunanInput{
		KodeRegister:     strings.TrimSpace(in.KodeRegister),
		JenisAgunanCode:  strings.TrimSpace(in.JenisAgunanCode),
		AlamatAgunan:     strings.TrimSpace(in.AlamatAgunan),
		NilaiDiagunkan:   in.NilaiDiagunkan,
		NilaiAgunan:      in.NilaiAgunan,
		PenilaiCode:      strings.TrimSpace(in.PenilaiCode),
		TanggalPenilaian: strings.TrimSpace(in.TanggalPenilaian),
		PPKAAmount:       in.PPKAAmount,
	}
	if out.KodeRegister == "" {
		return out, fmt.Errorf("%w: Kode Register/Nomor Agunan wajib diisi", ErrAgunanInputInvalid)
	}
	if len(out.KodeRegister) > AgunanRegisterMax {
		return out, fmt.Errorf("%w: Kode Register maksimal %d karakter",
			ErrAgunanInputInvalid, AgunanRegisterMax)
	}
	if out.JenisAgunanCode == "" {
		return out, fmt.Errorf("%w: Jenis Agunan (sandi Lampiran 01) wajib diisi", ErrAgunanInputInvalid)
	}
	if out.AlamatAgunan == "" {
		return out, fmt.Errorf("%w: Alamat Agunan wajib diisi", ErrAgunanInputInvalid)
	}
	switch out.PenilaiCode {
	case AgunanPenilaiIndependen, AgunanPenilaiInternal:
	default:
		return out, fmt.Errorf("%w: Penilai hanya 1 (independen) atau 2 (internal)", ErrAgunanInputInvalid)
	}
	if out.NilaiDiagunkan.IsNegative() {
		return out, fmt.Errorf("%w: Nilai yang Diagunkan tidak boleh negatif", ErrAgunanInputInvalid)
	}
	if !out.NilaiAgunan.IsPositive() {
		return out, fmt.Errorf("%w: Nilai Agunan harus lebih dari nol", ErrAgunanInputInvalid)
	}
	if out.PPKAAmount.IsNegative() {
		return out, fmt.Errorf("%w: Nilai untuk PPKA tidak boleh negatif", ErrAgunanInputInvalid)
	}
	if _, err := time.Parse("2006-01-02", out.TanggalPenilaian); err != nil {
		return out, fmt.Errorf("%w: Tanggal Penilaian Terakhir harus format YYYY-MM-DD", ErrAgunanInputInvalid)
	}
	return out, nil
}

// AgunanRepository membaca agunan untuk Form 06.01 dan menyimpan kolom Form 06.01.
type AgunanRepository interface {
	// ListAgunanForOJK membaca agunan AKTIF beserta nomor rekening fasilitas, urutan
	// deterministik.
	ListAgunanForOJK(ctx context.Context) ([]AgunanRow, error)
	// UpdateOjkColumnsTx menyimpan kolom Form 06.01 satu agunan. found=false bila agunan
	// tidak ada atau tidak AKTIF.
	UpdateOjkColumnsTx(ctx context.Context, tx any, id uuid.UUID, in UpdateAgunanInput) (bool, error)
}

// AgunanService melayani baca dan pengisian Form 06.01.
type AgunanService interface {
	ListAgunan(ctx context.Context, actor Actor) ([]AgunanRow, error)
	UpdateAgunan(ctx context.Context, id uuid.UUID, in UpdateAgunanInput, actor Actor) error
}
