package domain

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// pinjaman.go memuat fondasi data Form 00.07 "DAFTAR PINJAMAN YANG DITERIMA",
// Lampiran II SEOJK No. 16/SEOJK.03/2024: register pinjaman yang bank terima dari
// bank, Bank Indonesia, dan/atau pihak ketiga bukan bank, per pinjaman/kreditur.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 00.07 - 1 "DAFTAR PINJAMAN YANG DITERIMA": PDF #page 248-249 (hlm. 196-197).
//     Lima belas kolom: I ID Pihak Lawan, II Gol. Kreditur, III Sandi Bank,
//     IV Lokasi Kreditur, V Jenis, VI Hubungan dengan Bank, VII Jangka Waktu,
//     VIII Suku Bunga, IX Plafon, X Jenis Agunan yang Dijaminkan,
//     XI Nominal Agunan yang Dijaminkan, XII Baki Debet,
//     XIII Biaya Transaksi Belum Diamortisasi, XIV Diskonto Belum Diamortisasi,
//     XV Baki Debet Neto. Satu baris = satu pinjaman; ada baris JUMLAH.
//   - Sandi: PDF #page 250 (hlm. 198). V 10/20/31/32/41/42/99; VI 12/20;
//     VIII 11/12/21/22.
//   - Penjelasan: PDF #page 252-254 (hlm. 200-202). X tanpa agunan = 299;
//     XI tanpa agunan = 0; XV = baki debet - (diskonto + biaya transaksi belum
//     diamortisasi).
//
// Prinsip yang dipegang berkas ini:
//   - Kolom XV "Baki Debet Neto" TIDAK disimpan: ia turunan yang dihitung laporan
//     (XII - XIII - XIV). Lihat BakiDebetNeto.
//   - Seluruh angka dan sandi lain adalah isian bank. Sistem tidak menghitung plafon
//     maupun baki debet.
//   - Sandi rujukan Lampiran disimpan sebagai sandi dan diverifikasi basis data lewat
//     FK (II ke ojk_pihak_lawan, IV ke ojk_kabupaten, migrasi 000102). Sandi X
//     (Lampiran 01) belum punya tabel referensi sehingga disimpan sebagai teks apa
//     adanya; daftarnya TIDAK dikarang.
//   - Form ini TIDAK punya kolom Sandi Kantor: kolom I-nya ID Pihak Lawan. Kolom III
//     "Sandi Bank" adalah sandi bank kreditur, bukan kantor pelapor.

// Sandi jenis pinjaman baku Form 00.07 (PDF #page 250).
const (
	PinjamanJenisBilateral              = "10"
	PinjamanJenisSindikasi              = "20"
	PinjamanJenisKhususLembagaPengayom  = "31"
	PinjamanJenisKhususLinkage          = "32"
	PinjamanJenisTertentuModalInti      = "41"
	PinjamanJenisTertentuModalPelengkap = "42"
	PinjamanJenisLainnya                = "99"
)

// Sandi hubungan dengan bank baku Form 00.07 (PDF #page 250).
const (
	PinjamanHubunganTerkait      = "12"
	PinjamanHubunganTidakTerkait = "20"
)

// Sandi cara perhitungan suku bunga baku Form 00.07 (PDF #page 250).
const (
	PinjamanBungaFlatTetap           = "11"
	PinjamanBungaFlatMengambang      = "12"
	PinjamanBungaTidakFlatTetap      = "21"
	PinjamanBungaTidakFlatMengambang = "22"
)

// PinjamanTanpaAgunanCode adalah sandi jenis agunan untuk pinjaman tanpa agunan
// (PDF #page 253). Nominal agunan pinjaman tanpa agunan wajib 0.
const PinjamanTanpaAgunanCode = "299"

// Status baku register pinjaman yang diterima.
const (
	PinjamanStatusAktif    = "AKTIF"
	PinjamanStatusNonaktif = "NONAKTIF"
)

// pinjamanBankCodePattern adalah sandi bank kreditur 6 digit SPOJK (kolom III).
var pinjamanBankCodePattern = regexp.MustCompile(`^[0-9]{6}$`)

var (
	// ErrPinjamanBankWide menandai permintaan register pinjaman yang diterima oleh
	// aktor yang tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrPinjamanBankWide = NewLocalizedError("pinjaman_bank_wide",
		"register pinjaman yang diterima bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrPinjamanInputInvalid menandai data register pinjaman yang diterima tidak sah.
	ErrPinjamanInputInvalid = NewLocalizedError("pinjaman_input_invalid",
		"data register pinjaman yang diterima tidak valid")
	// ErrPinjamanNotFound menandai baris yang hendak dihapus tidak ada.
	ErrPinjamanNotFound = NewLocalizedError("pinjaman_not_found",
		"data register pinjaman yang diterima tidak ditemukan")
)

// PinjamanItem adalah satu pinjaman yang diterima (satu baris Form 00.07) yang bank
// catat. Kolom XV Baki Debet Neto tidak menjadi bidang di sini: nilainya turunan dan
// dihitung laporan lewat BakiDebetNeto.
type PinjamanItem struct {
	ID uuid.UUID `json:"id"`
	// CounterpartyID kolom I: sandi unik kreditur (wajib).
	CounterpartyID string `json:"counterparty_id"`
	// CreditorGroupCode kolom II: sandi Lampiran 02 (FK ojk_pihak_lawan).
	CreditorGroupCode string `json:"creditor_group_code"`
	// BankCode kolom III: sandi bank kreditur 6 digit SPOJK.
	BankCode string `json:"bank_code"`
	// LocationCode kolom IV: sandi Lampiran 03 (FK ojk_kabupaten).
	LocationCode string `json:"location_code"`
	// JenisCode kolom V: 10/20/31/32/41/42/99 (PDF #page 250).
	JenisCode string `json:"jenis_code"`
	// RelationshipCode kolom VI: 12 terkait / 20 tidak terkait (PDF #page 250).
	RelationshipCode string `json:"relationship_code"`
	// StartDate kolom VII sub Tanggal Mulai.
	StartDate time.Time `json:"start_date"`
	// MaturityDate kolom VII sub Tanggal Jatuh Tempo.
	MaturityDate time.Time `json:"maturity_date"`
	// InterestRate kolom VIII sub Persentase (%), selalu >= 0 (isian bank).
	InterestRate decimal.Decimal `json:"interest_rate"`
	// InterestCalcCode kolom VIII sub Cara Perhitungan: 11/12/21/22.
	InterestCalcCode string `json:"interest_calc_code"`
	// Plafon kolom IX, rupiah penuh; selalu >= 0 (isian bank).
	Plafon decimal.Decimal `json:"plafon"`
	// CollateralTypeCode kolom X; tanpa agunan = "299" (PDF #page 253).
	CollateralTypeCode string `json:"collateral_type_code"`
	// CollateralAmount kolom XI, rupiah penuh; tanpa agunan = 0.
	CollateralAmount decimal.Decimal `json:"collateral_amount"`
	// BakiDebet kolom XII, rupiah penuh; selalu >= 0 (isian bank).
	BakiDebet decimal.Decimal `json:"baki_debet"`
	// UnamortizedTransactionCost kolom XIII, rupiah penuh; selalu >= 0.
	UnamortizedTransactionCost decimal.Decimal `json:"unamortized_transaction_cost"`
	// UnamortizedDiscount kolom XIV, rupiah penuh; selalu >= 0.
	UnamortizedDiscount decimal.Decimal `json:"unamortized_discount"`
	AsOf                time.Time       `json:"as_of"`
	Status              string          `json:"status"`
	Note                string          `json:"note,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
	UpdatedAt           time.Time       `json:"updated_at"`
}

// BakiDebetNeto menghitung kolom XV "Baki Debet Neto": baki debet dikurangi diskonto
// dan biaya transaksi belum diamortisasi (PDF #page 254). ok=false bila hasilnya
// negatif (biaya/diskonto melebihi baki debet); kolom XV ditulis "-" beserta alasannya,
// bukan angka negatif yang menyesatkan.
func (i PinjamanItem) BakiDebetNeto() (decimal.Decimal, bool) {
	neto := i.BakiDebet.Sub(i.UnamortizedTransactionCost).Sub(i.UnamortizedDiscount)
	if neto.IsNegative() {
		return decimal.Zero, false
	}
	return neto, true
}

// UpdatePinjamanItemInput adalah isi upsert satu pinjaman. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON number
// maupun string desimal. Tanggal memakai format YYYY-MM-DD.
type UpdatePinjamanItemInput struct {
	ID                         string          `json:"id"`
	CounterpartyID             string          `json:"counterparty_id"`
	CreditorGroupCode          string          `json:"creditor_group_code"`
	BankCode                   string          `json:"bank_code"`
	LocationCode               string          `json:"location_code"`
	JenisCode                  string          `json:"jenis_code"`
	RelationshipCode           string          `json:"relationship_code"`
	StartDate                  string          `json:"start_date"`
	MaturityDate               string          `json:"maturity_date"`
	InterestRate               decimal.Decimal `json:"interest_rate"`
	InterestCalcCode           string          `json:"interest_calc_code"`
	Plafon                     decimal.Decimal `json:"plafon"`
	CollateralTypeCode         string          `json:"collateral_type_code"`
	CollateralAmount           decimal.Decimal `json:"collateral_amount"`
	BakiDebet                  decimal.Decimal `json:"baki_debet"`
	UnamortizedTransactionCost decimal.Decimal `json:"unamortized_transaction_cost"`
	UnamortizedDiscount        decimal.Decimal `json:"unamortized_discount"`
	AsOf                       string          `json:"as_of"`
	Status                     string          `json:"status"`
	Note                       string          `json:"note"`
}

// BuildPinjamanItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID
// kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak
// disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildPinjamanItem(in UpdatePinjamanItemInput) (PinjamanItem, error) {
	out := PinjamanItem{
		CounterpartyID:             strings.TrimSpace(in.CounterpartyID),
		CreditorGroupCode:          strings.TrimSpace(in.CreditorGroupCode),
		BankCode:                   strings.TrimSpace(in.BankCode),
		LocationCode:               strings.TrimSpace(in.LocationCode),
		JenisCode:                  strings.TrimSpace(in.JenisCode),
		RelationshipCode:           strings.TrimSpace(in.RelationshipCode),
		InterestRate:               in.InterestRate,
		InterestCalcCode:           strings.TrimSpace(in.InterestCalcCode),
		Plafon:                     in.Plafon,
		CollateralTypeCode:         strings.TrimSpace(in.CollateralTypeCode),
		CollateralAmount:           in.CollateralAmount,
		BakiDebet:                  in.BakiDebet,
		UnamortizedTransactionCost: in.UnamortizedTransactionCost,
		UnamortizedDiscount:        in.UnamortizedDiscount,
		Status:                     strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                       strings.TrimSpace(in.Note),
	}
	if out.CounterpartyID == "" {
		return out, fmt.Errorf("%w: ID Pihak Lawan wajib diisi", ErrPinjamanInputInvalid)
	}
	if len(out.CounterpartyID) > 64 {
		return out, fmt.Errorf("%w: ID Pihak Lawan maksimal 64 karakter", ErrPinjamanInputInvalid)
	}
	if out.CreditorGroupCode == "" {
		return out, fmt.Errorf("%w: Gol. Kreditur wajib diisi (Lampiran 02)", ErrPinjamanInputInvalid)
	}
	if !pinjamanBankCodePattern.MatchString(out.BankCode) {
		return out, fmt.Errorf("%w: Sandi Bank harus 6 digit", ErrPinjamanInputInvalid)
	}
	if out.LocationCode == "" {
		return out, fmt.Errorf("%w: Lokasi Kreditur wajib diisi (Lampiran 03)", ErrPinjamanInputInvalid)
	}
	switch out.JenisCode {
	case PinjamanJenisBilateral, PinjamanJenisSindikasi, PinjamanJenisKhususLembagaPengayom,
		PinjamanJenisKhususLinkage, PinjamanJenisTertentuModalInti,
		PinjamanJenisTertentuModalPelengkap, PinjamanJenisLainnya:
	default:
		return out, fmt.Errorf("%w: jenis hanya 10, 20, 31, 32, 41, 42, atau 99",
			ErrPinjamanInputInvalid)
	}
	switch out.RelationshipCode {
	case PinjamanHubunganTerkait, PinjamanHubunganTidakTerkait:
	default:
		return out, fmt.Errorf("%w: hubungan dengan bank hanya 12 atau 20", ErrPinjamanInputInvalid)
	}
	switch out.InterestCalcCode {
	case PinjamanBungaFlatTetap, PinjamanBungaFlatMengambang,
		PinjamanBungaTidakFlatTetap, PinjamanBungaTidakFlatMengambang:
	default:
		return out, fmt.Errorf("%w: cara perhitungan hanya 11, 12, 21, atau 22",
			ErrPinjamanInputInvalid)
	}
	if out.InterestRate.IsNegative() {
		return out, fmt.Errorf("%w: suku bunga tidak boleh negatif", ErrPinjamanInputInvalid)
	}
	if out.Plafon.IsNegative() {
		return out, fmt.Errorf("%w: plafon tidak boleh negatif", ErrPinjamanInputInvalid)
	}
	if out.CollateralTypeCode == "" {
		return out, fmt.Errorf("%w: jenis agunan wajib diisi (tanpa agunan = 299)",
			ErrPinjamanInputInvalid)
	}
	if out.CollateralAmount.IsNegative() {
		return out, fmt.Errorf("%w: nominal agunan tidak boleh negatif", ErrPinjamanInputInvalid)
	}
	// Aturan tanpa agunan (PDF #page 253): sandi 299 hanya sah bila nominal 0, dan
	// nominal 0 hanya sah bila sandinya 299. Jangan mengisi nominal karangan.
	tanpaAgunan := out.CollateralTypeCode == PinjamanTanpaAgunanCode
	if tanpaAgunan && !out.CollateralAmount.IsZero() {
		return out, fmt.Errorf("%w: pinjaman tanpa agunan (sandi 299) wajib bernominal 0",
			ErrPinjamanInputInvalid)
	}
	if !tanpaAgunan && out.CollateralAmount.IsZero() {
		return out, fmt.Errorf("%w: nominal agunan 0 hanya untuk pinjaman tanpa agunan (sandi 299)",
			ErrPinjamanInputInvalid)
	}
	if out.BakiDebet.IsNegative() {
		return out, fmt.Errorf("%w: baki debet tidak boleh negatif", ErrPinjamanInputInvalid)
	}
	if out.UnamortizedTransactionCost.IsNegative() {
		return out, fmt.Errorf("%w: biaya transaksi belum diamortisasi tidak boleh negatif",
			ErrPinjamanInputInvalid)
	}
	if out.UnamortizedDiscount.IsNegative() {
		return out, fmt.Errorf("%w: diskonto belum diamortisasi tidak boleh negatif",
			ErrPinjamanInputInvalid)
	}
	if out.Status == "" {
		out.Status = PinjamanStatusAktif
	}
	if out.Status != PinjamanStatusAktif && out.Status != PinjamanStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrPinjamanInputInvalid)
	}
	var err error
	if out.ID, err = parsePinjamanID(in.ID); err != nil {
		return out, err
	}
	if out.StartDate, err = parsePinjamanDate(in.StartDate, "start_date"); err != nil {
		return out, err
	}
	if out.MaturityDate, err = parsePinjamanDate(in.MaturityDate, "maturity_date"); err != nil {
		return out, err
	}
	if out.AsOf, err = parsePinjamanDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parsePinjamanID mengurai id yang dikirim klien; kosong berarti buat baru.
func parsePinjamanID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrPinjamanInputInvalid)
	}
	return id, nil
}

// parsePinjamanDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parsePinjamanDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrPinjamanInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrPinjamanInputInvalid, field)
	}
	return t, nil
}

// PinjamanRegisterRepository menyimpan dan membaca register pinjaman yang diterima.
// Seluruh pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit
// berada pada transaksi yang sama.
type PinjamanRegisterRepository interface {
	ListItems(ctx context.Context) ([]PinjamanItem, error)
	// ListPinjamanForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 00.07.
	ListPinjamanForOJK(ctx context.Context, asOf time.Time) ([]PinjamanItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu pinjaman.
	UpsertItemTx(ctx context.Context, tx any, item PinjamanItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu pinjaman; found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// PinjamanReport adalah keluaran baca-saja register pinjaman yang diterima untuk satu
// posisi.
type PinjamanReport struct {
	AsOf  time.Time      `json:"as_of"`
	Items []PinjamanItem `json:"items"`
}

// PinjamanRegisterService merakit laporan register pinjaman yang diterima dan melayani
// pengisian berizin.
type PinjamanRegisterService interface {
	// PinjamanReport menyusun laporan untuk satu posisi. Bank-wide.
	PinjamanReport(ctx context.Context, asOf time.Time, actor Actor) (PinjamanReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]PinjamanItem, error)
	// UpsertItem menyimpan satu pinjaman (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdatePinjamanItemInput, actor Actor) (*PinjamanItem, error)
	// DeleteItem menghapus satu pinjaman, teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
