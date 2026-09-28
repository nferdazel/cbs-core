package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ayda.go memuat fondasi data Form 07.00 "DAFTAR AGUNAN YANG DIAMBIL ALIH" (AYDA),
// Lampiran II SEOJK No. 16/SEOJK.03/2024: register AYDA per kasus yang bank catat.
//
// Ini BUKAN perhitungan akuntansi baru: saldo buku besar AYDA tetap akun COA 10500
// (coa_mapping.go:110). Register adalah RINCIAN per kasus; keduanya dapat berbeda bila
// bank belum mengisi semua kasus, sehingga laporan tidak memaksa total register sama
// dengan saldo COA.
//
// Prinsip yang dipegang berkas ini:
//   - TIDAK mengarang angka. Kolom V (nilai pengakuan awal), VI (akumulasi kerugian
//     penurunan nilai), dan VIII (NRV) adalah isian bank. Sistem tidak menghitung V
//     dari nilai wajar, estimasi biaya penjualan, dan baki debet karena ketiganya bukan
//     kolom register (Form 07.00 - 3, PDF #page 181).
//   - Hanya kolom VII "Jumlah" yang merupakan turunan: laporan menghitungnya sebagai
//     nilai yang lebih rendah dari NRV (VIII) atau nilai tercatat (V - VI). Karena itu
//     kolom VII tidak disimpan (lihat migrasi 000115).
//   - Sandi Kantor (kolom I) tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 07.00 - 1: PDF #page 179 (hlm. tercetak 127), delapan kolom + baris JUMLAH.
//   - Form 07.00 - 2: PDF #page 180 (hlm. 128), sandi II Jenis Agunan.
//   - Form 07.00 - 3: PDF #page 181 (hlm. 129), penjelasan kolom.

// Sandi jenis agunan baku Form 07.00 - 2 (PDF #page 180). Nilainya baku karena
// maknanya baku pada form, bukan sandi pos regulasi yang dapat berubah.
const (
	AYDACollateralTypeEmas          = "01"
	AYDACollateralTypeTanahBangunan = "02"
	AYDACollateralTypeResiGudang    = "03"
	AYDACollateralTypeTempatUsaha   = "04"
	AYDACollateralTypeKendaraan     = "05"
	AYDACollateralTypeLainnya       = "99"
)

// Status baku register AYDA.
const (
	AYDAStatusAktif    = "AKTIF"
	AYDAStatusNonaktif = "NONAKTIF"
)

var (
	// ErrAYDABankWide menandai permintaan register AYDA oleh aktor yang tidak
	// berwenang atas seluruh bank. Register ini bank-wide.
	ErrAYDABankWide = NewLocalizedError("ayda_bank_wide",
		"register AYDA bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrAYDAInputInvalid menandai data register AYDA yang tidak sah.
	ErrAYDAInputInvalid = NewLocalizedError("ayda_input_invalid",
		"data register AYDA tidak valid")
	// ErrAYDANotFound menandai baris yang hendak dihapus tidak ada.
	ErrAYDANotFound = NewLocalizedError("ayda_not_found",
		"data register AYDA tidak ditemukan")
)

// AYDAItem adalah satu AYDA (satu baris Form 07.00) yang bank catat.
type AYDAItem struct {
	ID uuid.UUID `json:"id"`
	// CollateralTypeCode salah satu sandi jenis agunan Form 07.00 - 2.
	CollateralTypeCode string `json:"collateral_type_code"`
	// CollateralAddress adalah alamat lengkap agunan (wajib).
	CollateralAddress string `json:"collateral_address"`
	// AcquisitionDate tanggal pengambilalihan AYDA.
	AcquisitionDate time.Time `json:"acquisition_date"`
	// InitialRecognitionValue kolom V; selalu >= 0 (isian bank).
	InitialRecognitionValue decimal.Decimal `json:"initial_recognition_value"`
	// AccumulatedImpairment kolom VI sebagai BESARAN kerugian; selalu >= 0.
	AccumulatedImpairment decimal.Decimal `json:"accumulated_impairment"`
	// NetRealizableValue kolom VIII; selalu >= 0 (isian bank).
	NetRealizableValue decimal.Decimal `json:"net_realizable_value"`
	AsOf               time.Time       `json:"as_of"`
	Status             string          `json:"status"`
	Note               string          `json:"note,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

// Jumlah menghitung kolom VII "Jumlah": nilai yang lebih rendah dari NRV (VIII) atau
// nilai tercatat (nilai pengakuan awal V dikurangi akumulasi kerugian VI), mengikuti
// Form 07.00 - 3 (PDF #page 181). ok=false bila nilai tercatat negatif (akumulasi
// kerugian melebihi nilai pengakuan awal); kolom VII ditulis "-" beserta alasannya,
// bukan angka negatif yang menyesatkan.
func (i AYDAItem) Jumlah() (decimal.Decimal, bool) {
	tercatat := i.InitialRecognitionValue.Sub(i.AccumulatedImpairment)
	if tercatat.IsNegative() {
		return decimal.Zero, false
	}
	if i.NetRealizableValue.LessThan(tercatat) {
		return i.NetRealizableValue, true
	}
	return tercatat, true
}

// UpdateAYDAItemInput adalah isi upsert satu AYDA. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON number
// maupun string desimal. Tanggal memakai format YYYY-MM-DD.
type UpdateAYDAItemInput struct {
	ID                      string          `json:"id"`
	CollateralTypeCode      string          `json:"collateral_type_code"`
	CollateralAddress       string          `json:"collateral_address"`
	AcquisitionDate         string          `json:"acquisition_date"`
	InitialRecognitionValue decimal.Decimal `json:"initial_recognition_value"`
	AccumulatedImpairment   decimal.Decimal `json:"accumulated_impairment"`
	NetRealizableValue      decimal.Decimal `json:"net_realizable_value"`
	AsOf                    string          `json:"as_of"`
	Status                  string          `json:"status"`
	Note                    string          `json:"note"`
}

// BuildAYDAItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID kosong
// dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak disetel di
// sini (diisi repositori/layanan saat menyimpan).
func BuildAYDAItem(in UpdateAYDAItemInput) (AYDAItem, error) {
	out := AYDAItem{
		CollateralTypeCode:      strings.TrimSpace(in.CollateralTypeCode),
		CollateralAddress:       strings.TrimSpace(in.CollateralAddress),
		InitialRecognitionValue: in.InitialRecognitionValue,
		AccumulatedImpairment:   in.AccumulatedImpairment,
		NetRealizableValue:      in.NetRealizableValue,
		Status:                  strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                    strings.TrimSpace(in.Note),
	}
	switch out.CollateralTypeCode {
	case AYDACollateralTypeEmas, AYDACollateralTypeTanahBangunan, AYDACollateralTypeResiGudang,
		AYDACollateralTypeTempatUsaha, AYDACollateralTypeKendaraan, AYDACollateralTypeLainnya:
	default:
		return out, fmt.Errorf("%w: jenis agunan hanya 01, 02, 03, 04, 05, atau 99",
			ErrAYDAInputInvalid)
	}
	if out.CollateralAddress == "" {
		return out, fmt.Errorf("%w: alamat agunan wajib diisi", ErrAYDAInputInvalid)
	}
	if len(out.CollateralAddress) > 255 {
		return out, fmt.Errorf("%w: alamat agunan maksimal 255 karakter", ErrAYDAInputInvalid)
	}
	if out.InitialRecognitionValue.IsNegative() {
		return out, fmt.Errorf("%w: nilai pengakuan awal tidak boleh negatif", ErrAYDAInputInvalid)
	}
	if out.AccumulatedImpairment.IsNegative() {
		return out, fmt.Errorf("%w: akumulasi kerugian penurunan nilai tidak boleh negatif",
			ErrAYDAInputInvalid)
	}
	if out.NetRealizableValue.IsNegative() {
		return out, fmt.Errorf("%w: nilai bersih yang dapat direalisasikan tidak boleh negatif",
			ErrAYDAInputInvalid)
	}
	if out.Status == "" {
		out.Status = AYDAStatusAktif
	}
	if out.Status != AYDAStatusAktif && out.Status != AYDAStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrAYDAInputInvalid)
	}
	var err error
	if out.ID, err = parseAYDAID(in.ID); err != nil {
		return out, err
	}
	if out.AcquisitionDate, err = parseAYDADate(in.AcquisitionDate, "acquisition_date"); err != nil {
		return out, err
	}
	if out.AsOf, err = parseAYDADate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseAYDAID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseAYDAID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrAYDAInputInvalid)
	}
	return id, nil
}

// parseAYDADate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parseAYDADate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrAYDAInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrAYDAInputInvalid, field)
	}
	return t, nil
}

// AYDARegisterRepository menyimpan dan membaca register AYDA. Seluruh pembacaan
// bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada
// transaksi yang sama.
type AYDARegisterRepository interface {
	ListItems(ctx context.Context) ([]AYDAItem, error)
	// ListAYDAForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 07.00.
	ListAYDAForOJK(ctx context.Context, asOf time.Time) ([]AYDAItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu AYDA.
	UpsertItemTx(ctx context.Context, tx any, item AYDAItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu AYDA; found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// AYDAReport adalah keluaran baca-saja register AYDA untuk satu posisi.
type AYDAReport struct {
	AsOf  time.Time  `json:"as_of"`
	Items []AYDAItem `json:"items"`
}

// AYDARegisterService merakit laporan register AYDA dan melayani pengisian berizin.
type AYDARegisterService interface {
	// AYDAReport menyusun laporan untuk satu posisi. Bank-wide.
	AYDAReport(ctx context.Context, asOf time.Time, actor Actor) (AYDAReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]AYDAItem, error)
	// UpsertItem menyimpan satu AYDA (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdateAYDAItemInput, actor Actor) (*AYDAItem, error)
	// DeleteItem menghapus satu AYDA, teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
