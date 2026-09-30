package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// form15_00_hapus_buku.go memuat fondasi data Form 15.00 "DAFTAR ASET PRODUKTIF YANG
// DIHAPUS BUKU", Lampiran II SEOJK No. 16/SEOJK.03/2024.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Susunan PDF #page 218 (hlm. tercetak 166) kolom I-VI, PDF #page 219 (hlm. 167)
//     kolom VII-X + baris JUMLAH, sandi PDF #page 220 (hlm. 168), penjelasan PDF
//     #page 221-222 (hlm. 169-170).
//   - Sandi IV Jenis Aset: 10 Kredit yang Diberikan, 20 Penempatan pada Bank Lain.
//     VI Hubungan dengan Bank: 11 Terkait Dalam Rangka Kesejahteraan, 12 Terkait
//     Lainnya, 20 Tidak Terkait. X Jenis Agunan mengacu Lampiran 01; tanpa agunan = 299
//     dan Nilai = 0.
//
// MENGAPA REGISTER SENDIRI: form ini memuat kredit MAUPUN penempatan pada bank lain yang
// dihapus buku; penempatan bukan kredit sehingga tidak dapat disimpan pada loans. Baris
// loans.written_off_amount hanya menopang agenda pemulihan dan tidak menyimpan dimensi
// pelaporan ini (tanggal hapus buku, jenis aset, pemisahan pokok/bunga, akumulasi
// tertagih, agunan saat hapus buku). Menurunkannya dari loans akan mengarang dimensi yang
// tidak ada.

// Sandi baku Form 15.00 kolom IV (PDF #220).
const (
	HapusBukuJenisAsetKredit     = "10"
	HapusBukuJenisAsetPenempatan = "20"
)

// Sandi baku Form 15.00 kolom VI (PDF #220).
const (
	HapusBukuHubunganKesejahteraan = "11"
	HapusBukuHubunganTerkaitLain   = "12"
	HapusBukuHubunganTidakTerkait  = "20"
)

var (
	// ErrHapusBukuInputInvalid menandai data register hapus buku tidak sah.
	ErrHapusBukuInputInvalid = NewLocalizedError("hapus_buku_input_invalid",
		"data hapus buku Form 15.00 tidak valid")
	// ErrHapusBukuNotFound menandai baris yang hendak dihapus tidak ada.
	ErrHapusBukuNotFound = NewLocalizedError("hapus_buku_not_found",
		"data hapus buku tidak ditemukan")
	// ErrHapusBukuBankWide menandai permintaan register hapus buku oleh peran yang tidak
	// berwenang atas seluruh bank.
	ErrHapusBukuBankWide = NewLocalizedError("hapus_buku_bank_wide",
		"register hapus buku bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
)

// HapusBukuItem adalah satu aset produktif yang dihapus buku (satu baris Form 15.00).
type HapusBukuItem struct {
	ID uuid.UUID `json:"id"`
	// JenisAsetCode kolom IV: 10/20.
	JenisAsetCode string `json:"jenis_aset_code"`
	// PihakLawanID kolom II (ID pihak lawan debitur atau sandi bank).
	PihakLawanID string `json:"pihak_lawan_id"`
	// NomorRekening kolom III.
	NomorRekening string `json:"nomor_rekening"`
	// JenisDebiturCode kolom V (Lampiran 02); boleh kosong untuk penempatan.
	JenisDebiturCode string `json:"jenis_debitur_code"`
	// HubunganBankCode kolom VI: 11/12/20.
	HubunganBankCode string `json:"hubungan_bank_code"`
	// TanggalHapusBuku kolom VII.
	TanggalHapusBuku time.Time `json:"tanggal_hapus_buku"`
	// VIII Saldo Pokok.
	SaldoPokokSaatHapus    decimal.Decimal `json:"saldo_pokok_saat_hapus"`
	SaldoPokokAkumTertagih decimal.Decimal `json:"saldo_pokok_akum_tertagih"`
	SaldoPokokPerPosisi    decimal.Decimal `json:"saldo_pokok_per_posisi"`
	// IX Tunggakan Bunga.
	BungaSaatHapus    decimal.Decimal `json:"bunga_saat_hapus"`
	BungaAkumTertagih decimal.Decimal `json:"bunga_akum_tertagih"`
	BungaAkumTambahan decimal.Decimal `json:"bunga_akum_tambahan"`
	BungaPerPosisi    decimal.Decimal `json:"bunga_per_posisi"`
	// X Agunan.
	AgunanJenisCode string          `json:"agunan_jenis_code"`
	AgunanAlamat    string          `json:"agunan_alamat"`
	AgunanNilai     decimal.Decimal `json:"agunan_nilai"`
	Note            string          `json:"note,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// UpdateHapusBukuItemInput adalah isi upsert satu baris. ID kosong = buat baru.
// Semua nominal dikirim sebagai teks desimal agar presisi tidak hilang.
type UpdateHapusBukuItemInput struct {
	ID                     string `json:"id"`
	JenisAsetCode          string `json:"jenis_aset_code"`
	PihakLawanID           string `json:"pihak_lawan_id"`
	NomorRekening          string `json:"nomor_rekening"`
	JenisDebiturCode       string `json:"jenis_debitur_code"`
	HubunganBankCode       string `json:"hubungan_bank_code"`
	TanggalHapusBuku       string `json:"tanggal_hapus_buku"`
	SaldoPokokSaatHapus    string `json:"saldo_pokok_saat_hapus"`
	SaldoPokokAkumTertagih string `json:"saldo_pokok_akum_tertagih"`
	SaldoPokokPerPosisi    string `json:"saldo_pokok_per_posisi"`
	BungaSaatHapus         string `json:"bunga_saat_hapus"`
	BungaAkumTertagih      string `json:"bunga_akum_tertagih"`
	BungaAkumTambahan      string `json:"bunga_akum_tambahan"`
	BungaPerPosisi         string `json:"bunga_per_posisi"`
	AgunanJenisCode        string `json:"agunan_jenis_code"`
	AgunanAlamat           string `json:"agunan_alamat"`
	AgunanNilai            string `json:"agunan_nilai"`
	Note                   string `json:"note"`
}

// BuildHapusBukuItem memvalidasi masukan dan membentuk baris yang siap disimpan.
func BuildHapusBukuItem(in UpdateHapusBukuItemInput) (HapusBukuItem, error) {
	out := HapusBukuItem{
		JenisAsetCode:    strings.TrimSpace(in.JenisAsetCode),
		PihakLawanID:     strings.TrimSpace(in.PihakLawanID),
		NomorRekening:    strings.TrimSpace(in.NomorRekening),
		JenisDebiturCode: strings.TrimSpace(in.JenisDebiturCode),
		HubunganBankCode: strings.TrimSpace(in.HubunganBankCode),
		AgunanJenisCode:  strings.TrimSpace(in.AgunanJenisCode),
		AgunanAlamat:     strings.TrimSpace(in.AgunanAlamat),
		Note:             strings.TrimSpace(in.Note),
	}
	switch out.JenisAsetCode {
	case HapusBukuJenisAsetKredit, HapusBukuJenisAsetPenempatan:
	default:
		return out, fmt.Errorf("%w: Jenis Aset hanya 10 atau 20", ErrHapusBukuInputInvalid)
	}
	switch out.HubunganBankCode {
	case HapusBukuHubunganKesejahteraan, HapusBukuHubunganTerkaitLain, HapusBukuHubunganTidakTerkait:
	default:
		return out, fmt.Errorf("%w: Hubungan dengan Bank hanya 11, 12, atau 20", ErrHapusBukuInputInvalid)
	}
	tanggal, err := time.Parse("2006-01-02", strings.TrimSpace(in.TanggalHapusBuku))
	if err != nil {
		return out, fmt.Errorf("%w: Tanggal Hapus Buku harus format YYYY-MM-DD", ErrHapusBukuInputInvalid)
	}
	out.TanggalHapusBuku = tanggal

	amounts := []struct {
		raw  string
		dest *decimal.Decimal
		name string
	}{
		{in.SaldoPokokSaatHapus, &out.SaldoPokokSaatHapus, "Saldo Pokok Saat Hapus Buku"},
		{in.SaldoPokokAkumTertagih, &out.SaldoPokokAkumTertagih, "Saldo Pokok Akumulasi Tertagih"},
		{in.SaldoPokokPerPosisi, &out.SaldoPokokPerPosisi, "Saldo Pokok Per Posisi Laporan"},
		{in.BungaSaatHapus, &out.BungaSaatHapus, "Tunggakan Bunga Saat Hapus Buku"},
		{in.BungaAkumTertagih, &out.BungaAkumTertagih, "Tunggakan Bunga Akumulasi Tertagih"},
		{in.BungaAkumTambahan, &out.BungaAkumTambahan, "Tunggakan Bunga Akumulasi Tambahan Bunga Berjalan"},
		{in.BungaPerPosisi, &out.BungaPerPosisi, "Tunggakan Bunga Per Posisi Laporan"},
		{in.AgunanNilai, &out.AgunanNilai, "Nilai Agunan"},
	}
	for _, a := range amounts {
		d, err := parseHapusBukuAmount(a.raw, a.name)
		if err != nil {
			return out, err
		}
		*a.dest = d
	}
	id, err := parseHapusBukuID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	return out, nil
}

// parseHapusBukuAmount mengurai nominal rupiah. Kosong berarti nol (kolom ini boleh nol
// menurut form, mis. akumulasi tertagih yang belum ada); nilai negatif ditolak.
func parseHapusBukuAmount(raw, name string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: %s bukan angka yang sah", ErrHapusBukuInputInvalid, name)
	}
	if d.IsNegative() {
		return decimal.Zero, fmt.Errorf("%w: %s tidak boleh negatif", ErrHapusBukuInputInvalid, name)
	}
	return d, nil
}

// parseHapusBukuID mengurai id; kosong berarti buat baru.
func parseHapusBukuID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrHapusBukuInputInvalid)
	}
	return id, nil
}

// HapusBukuReport adalah keluaran baca-saja register hapus buku.
type HapusBukuReport struct {
	Items []HapusBukuItem `json:"items"`
}

// HapusBukuRepository menyimpan dan membaca register hapus buku.
type HapusBukuRepository interface {
	ListItems(ctx context.Context) ([]HapusBukuItem, error)
	// ListHapusBukuForOJK membaca baris register untuk Form 15.00, urutan deterministik.
	ListHapusBukuForOJK(ctx context.Context) ([]HapusBukuItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu baris.
	UpsertItemTx(ctx context.Context, tx any, item HapusBukuItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu baris secara fisik; found=false bila tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// HapusBukuService melayani baca dan pengisian register hapus buku.
type HapusBukuService interface {
	ListHapusBuku(ctx context.Context, actor Actor) (HapusBukuReport, error)
	ListItems(ctx context.Context) ([]HapusBukuItem, error)
	UpsertItem(ctx context.Context, in UpdateHapusBukuItemInput, actor Actor) (*HapusBukuItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
