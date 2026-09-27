package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// off_balance.go memuat fondasi data Form 01.01 "REKENING ADMINISTRATIF"
// (SEOJK No. 16/SEOJK.03/2024): register pos komitmen/kontinjensi off-balance.
//
// Ini BUKAN perhitungan akuntansi: rekening administratif tidak pernah menjadi
// saldo bagan akun. Yang disimpan adalah pos yang BANK catat (sandi pos OJK bila
// diketahui, uraian, nominal, lawan, referensi, tanggal). Laporan bulanan hanya
// mengagregasi baris aktif per pos/kategori.
//
// Prinsip yang dipegang berkas ini:
//   - TIDAK mengarang sandi/angka. Kategori (KOMITMEN/KONTINJENSI/LAINNYA) dan status
//     baku karena maknanya baku (mengikuti pembagian resmi Form 01.01), bukan sandi
//     regulasi. `position_code` dan seluruh nilai lain adalah isian bank.
//   - Sistem TIDAK menurunkan pos dari jurnal: memakai basis kas untuk kredit NPL dan
//     tidak mereklasifikasi bunga terakru (docs/KEPUTUSAN-OJK.md §4), sehingga pos
//     seperti "Pendapatan Bunga Dalam Penyelesaian" hanya terisi bila bank mencatatnya.
//
// Sumber struktur pos (PDF resmi 528 hlm., di luar repo):
//   - Form 01.01 - 1 "REKENING ADMINISTRATIF": PDF #page 110, hlm. tercetak 58
//     (kolom I Sandi Kantor, II Nama Rekening, III Sandi, IV Jumlah).
//   - Penjelasan "D. LAPORAN KOMITMEN DAN KONTINJENSI": PDF #page 488.

// Kategori pos pada off_balance_items. Nilainya baku karena menentukan pengelompokan
// Form 01.01, bukan sandi regulasi yang dikarang.
const (
	OffBalanceCategoryKomitmen    = "KOMITMEN"
	OffBalanceCategoryKontinjensi = "KONTINJENSI"
	OffBalanceCategoryLainnya     = "LAINNYA"
)

// Status baku register rekening administratif.
const (
	OffBalanceStatusAktif    = "AKTIF"
	OffBalanceStatusNonaktif = "NONAKTIF"
)

var (
	// ErrOffBalanceBankWide menandai permintaan register rekening administratif oleh
	// aktor yang tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrOffBalanceBankWide = NewLocalizedError("off_balance_bank_wide",
		"register rekening administratif bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrOffBalanceInputInvalid menandai data rekening administratif yang tidak sah.
	ErrOffBalanceInputInvalid = NewLocalizedError("off_balance_input_invalid",
		"data rekening administratif tidak valid")
	// ErrOffBalanceNotFound menandai baris yang hendak dihapus tidak ada.
	ErrOffBalanceNotFound = NewLocalizedError("off_balance_not_found",
		"data rekening administratif tidak ditemukan")
)

// OffBalanceItem adalah satu pos rekening administratif (Form 01.01) yang bank catat.
type OffBalanceItem struct {
	ID uuid.UUID `json:"id"`
	// PositionCode adalah sandi pos OJK Form 01.01 yang bank ketahui; kosong = belum
	// diisi dan laporan menulis "-".
	PositionCode string `json:"position_code,omitempty"`
	// Category salah satu dari KOMITMEN/KONTINJENSI/LAINNYA.
	Category string `json:"category"`
	// Description adalah uraian/nama rekening yang bank isi (wajib).
	Description string `json:"description"`
	// Amount nominal rupiah penuh; selalu >= 0.
	Amount                 decimal.Decimal `json:"amount"`
	CounterpartyCustomerID *uuid.UUID      `json:"counterparty_customer_id,omitempty"`
	Reference              string          `json:"reference,omitempty"`
	AsOf                   time.Time       `json:"as_of"`
	Status                 string          `json:"status"`
	Note                   string          `json:"note,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

// OffBalanceAggregate adalah satu baris agregat Form 01.01: total nominal per
// kategori/pos/uraian. Dipakai membangun tabel laporan tanpa satu query per baris.
type OffBalanceAggregate struct {
	Category     string          `json:"category"`
	PositionCode string          `json:"position_code,omitempty"`
	Description  string          `json:"description,omitempty"`
	TotalAmount  decimal.Decimal `json:"total_amount"`
	ItemCount    int             `json:"item_count"`
}

// OffBalanceReport adalah keluaran baca-saja register rekening administratif untuk
// satu posisi: seluruh baris beserta agregatnya.
type OffBalanceReport struct {
	AsOf       time.Time             `json:"as_of"`
	Items      []OffBalanceItem      `json:"items"`
	Aggregates []OffBalanceAggregate `json:"aggregates"`
}

// UpdateOffBalanceItemInput adalah isi upsert satu pos. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu. Amount menerima angka JSON
// maupun string desimal. AsOf memakai format YYYY-MM-DD.
type UpdateOffBalanceItemInput struct {
	ID                     string          `json:"id"`
	PositionCode           string          `json:"position_code"`
	Category               string          `json:"category"`
	Description            string          `json:"description"`
	Amount                 decimal.Decimal `json:"amount"`
	CounterpartyCustomerID string          `json:"counterparty_customer_id"`
	Reference              string          `json:"reference"`
	AsOf                   string          `json:"as_of"`
	Status                 string          `json:"status"`
	Note                   string          `json:"note"`
}

// BuildOffBalanceItem memvalidasi masukan dan membentuk baris yang siap disimpan.
// ID kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu
// tidak disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildOffBalanceItem(in UpdateOffBalanceItemInput) (OffBalanceItem, error) {
	out := OffBalanceItem{
		PositionCode: strings.TrimSpace(in.PositionCode),
		Category:     strings.ToUpper(strings.TrimSpace(in.Category)),
		Description:  strings.TrimSpace(in.Description),
		Amount:       in.Amount,
		Reference:    strings.TrimSpace(in.Reference),
		Status:       strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:         strings.TrimSpace(in.Note),
	}
	switch out.Category {
	case OffBalanceCategoryKomitmen, OffBalanceCategoryKontinjensi, OffBalanceCategoryLainnya:
	default:
		return out, fmt.Errorf("%w: category hanya KOMITMEN, KONTINJENSI, atau LAINNYA",
			ErrOffBalanceInputInvalid)
	}
	if out.Description == "" {
		return out, fmt.Errorf("%w: uraian wajib diisi", ErrOffBalanceInputInvalid)
	}
	if len(out.Description) > 255 {
		return out, fmt.Errorf("%w: uraian maksimal 255 karakter", ErrOffBalanceInputInvalid)
	}
	if len(out.PositionCode) > 32 {
		return out, fmt.Errorf("%w: sandi pos maksimal 32 karakter", ErrOffBalanceInputInvalid)
	}
	if out.Amount.IsNegative() {
		return out, fmt.Errorf("%w: nominal tidak boleh negatif", ErrOffBalanceInputInvalid)
	}
	if len(out.Reference) > 128 {
		return out, fmt.Errorf("%w: referensi maksimal 128 karakter", ErrOffBalanceInputInvalid)
	}
	if out.Status == "" {
		out.Status = OffBalanceStatusAktif
	}
	if out.Status != OffBalanceStatusAktif && out.Status != OffBalanceStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrOffBalanceInputInvalid)
	}
	id, err := parseOffBalanceID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	if out.AsOf, err = parseOffBalanceDate(in.AsOf); err != nil {
		return out, err
	}
	if out.CounterpartyCustomerID, err = parseOffBalanceCustomer(in.CounterpartyCustomerID); err != nil {
		return out, err
	}
	return out, nil
}

// parseOffBalanceID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseOffBalanceID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrOffBalanceInputInvalid)
	}
	return id, nil
}

// parseOffBalanceDate mewajibkan tanggal posisi berformat YYYY-MM-DD.
func parseOffBalanceDate(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: as_of wajib diisi (YYYY-MM-DD)", ErrOffBalanceInputInvalid)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: as_of harus format YYYY-MM-DD", ErrOffBalanceInputInvalid)
	}
	return t, nil
}

// parseOffBalanceCustomer mengurai nasabah lawan; kosong berarti tidak terkait.
func parseOffBalanceCustomer(raw string) (*uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("%w: counterparty_customer_id bukan UUID yang sah", ErrOffBalanceInputInvalid)
	}
	return &id, nil
}

// OffBalanceRepository menyimpan dan membaca register rekening administratif.
// Seluruh pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit
// berada pada transaksi yang sama.
type OffBalanceRepository interface {
	ListItems(ctx context.Context) ([]OffBalanceItem, error)
	// ListAggregates menjumlahkan baris AKTIF per kategori/pos/uraian yang as_of-nya
	// berada pada bulan asOf.
	ListAggregates(ctx context.Context, asOf time.Time) ([]OffBalanceAggregate, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu pos.
	UpsertItemTx(ctx context.Context, tx any, item OffBalanceItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu pos; found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// OffBalanceService merakit laporan register rekening administratif dan melayani
// pengisian berizin.
type OffBalanceService interface {
	// OffBalanceReport menyusun laporan untuk satu posisi. Bank-wide.
	OffBalanceReport(ctx context.Context, asOf time.Time, actor Actor) (OffBalanceReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]OffBalanceItem, error)
	// UpsertItem menyimpan satu pos (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdateOffBalanceItemInput, actor Actor) (*OffBalanceItem, error)
	// DeleteItem menghapus satu pos, teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
