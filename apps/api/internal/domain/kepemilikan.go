package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// kepemilikan.go memuat fondasi data Form 00.01 "DATA KEPEMILIKAN BPR",
// Lampiran II SEOJK No. 16/SEOJK.03/2024: register pemegang saham per baris yang
// bank catat.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 00.01 "DATA KEPEMILIKAN BPR": PDF #page 72 (hlm. tercetak 20). Delapan
//     kolom: I Nama, II Alamat, III Jenis (01/02/03/04), IV No. Identitas,
//     V Status Pemegang Saham (01/02), VI Jumlah Nominal, VII Persentase Kepemilikan,
//     VIII Status Perubahan (1/2/3/9). TIDAK ADA baris JUMLAH; satu baris = satu
//     pemegang saham.
//   - Sandi: PDF #page 73 (hlm. 21). III 01 Perorangan, 02 Badan Hukum,
//     03 Pemerintah Daerah, 04 Publik; V 01 PSP, 02 Non PSP.
//   - Penjelasan: PDF #page 74 (hlm. 22). VIII 1 pemegang saham baru, 2 perubahan
//     yang mengakibatkan perubahan PSP, 3 perubahan yang tidak mengakibatkan
//     perubahan PSP, 9 tidak ada perubahan. Alamat (II) dan No. Identitas (IV) dapat
//     dikosongkan untuk kepemilikan kurang dari 2%.
//
// Prinsip yang dipegang berkas ini:
//   - Kolom IV "No. Identitas" TIDAK disimpan: keputusan privasi yang berlaku
//     (NIK/NPWP tidak dibuka ke keluaran laporan, docs/KEPUTUSAN-OJK.md §4 dan §7
//     butir 6). Laporan menulis kolom IV "-" beserta alasannya.
//   - Kolom I "Nama" dan III/IV/V/VI/VII/VIII adalah isian bank. Sistem tidak
//     menurunkan Jumlah Nominal dari persentase kali modal, tidak menebak sandi,
//     dan TIDAK memvalidasi bahwa Σ persentase = 100% (komposisi kepemilikan adalah
//     urusan bank, bukan sistem).
//   - Kolom II "Alamat" boleh kosong karena form mengizinkannya untuk kepemilikan
//     kurang dari 2% (PDF #page 74). Bank mengisi apa adanya.
//
// Sandi Kantor (kolom I form bank-wide lain) TIDAK ada di form ini: form 00.01
// dimulai dari Nama. Karena itu register ini bank-wide tanpa kantor pelapor.

// Sandi jenis pemegang saham baku Form 00.01 (PDF #page 73). Nilainya baku karena
// maknanya baku pada form, bukan sandi pos regulasi yang dapat berubah.
const (
	KepemilikanTypePerorangan       = "01"
	KepemilikanTypeBadanHukum       = "02"
	KepemilikanTypePemerintahDaerah = "03"
	KepemilikanTypePublik           = "04"
)

// Sandi status pemegang saham baku Form 00.01 (PDF #page 73).
const (
	KepemilikanStatusPSP    = "01"
	KepemilikanStatusNonPSP = "02"
)

// Sandi status perubahan baku Form 00.01 (PDF #page 74).
const (
	KepemilikanChangeBaru         = "1"
	KepemilikanChangeUbahPSP      = "2"
	KepemilikanChangeUbahTanpaPSP = "3"
	KepemilikanChangeTidakAda     = "9"
)

// Status baku register kepemilikan BPR.
const (
	KepemilikanStatusAktif    = "AKTIF"
	KepemilikanStatusNonaktif = "NONAKTIF"
)

var (
	// ErrKepemilikanBankWide menandai permintaan register kepemilikan BPR oleh aktor
	// yang tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrKepemilikanBankWide = NewLocalizedError("kepemilikan_bank_wide",
		"register kepemilikan BPR bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrKepemilikanInputInvalid menandai data register kepemilikan BPR yang tidak sah.
	ErrKepemilikanInputInvalid = NewLocalizedError("kepemilikan_input_invalid",
		"data register kepemilikan BPR tidak valid")
	// ErrKepemilikanNotFound menandai baris yang hendak dihapus tidak ada.
	ErrKepemilikanNotFound = NewLocalizedError("kepemilikan_not_found",
		"data register kepemilikan BPR tidak ditemukan")
)

// KepemilikanItem adalah satu pemegang saham BPR (satu baris Form 00.01) yang bank
// catat. Kolom IV No. Identitas sengaja tidak menjadi bidang di sini: nilainya tidak
// disimpan (keputusan privasi).
type KepemilikanItem struct {
	ID uuid.UUID `json:"id"`
	// ShareholderName kolom I (wajib).
	ShareholderName string `json:"shareholder_name"`
	// ShareholderAddress kolom II. Boleh kosong untuk kepemilikan <2% (PDF #page 74).
	ShareholderAddress string `json:"shareholder_address"`
	// ShareholderTypeCode kolom III: 01/02/03/04 (PDF #page 73).
	ShareholderTypeCode string `json:"shareholder_type_code"`
	// ShareholderStatusCode kolom V: 01 PSP / 02 Non PSP (PDF #page 73).
	ShareholderStatusCode string `json:"shareholder_status_code"`
	// NominalAmount kolom VI; selalu >= 0 (isian bank, rupiah penuh).
	NominalAmount decimal.Decimal `json:"nominal_amount"`
	// OwnershipPercentage kolom VII; selalu >= 0 dan <= 100. Sistem tidak
	// memvalidasi jumlah seluruh baris = 100%.
	OwnershipPercentage decimal.Decimal `json:"ownership_percentage"`
	// ChangeStatusCode kolom VIII: 1/2/3/9 (PDF #page 74).
	ChangeStatusCode string    `json:"change_status_code"`
	AsOf             time.Time `json:"as_of"`
	Status           string    `json:"status"`
	Note             string    `json:"note,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// UpdateKepemilikanItemInput adalah isi upsert satu pemegang saham. ID kosong = buat
// baru (id dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON
// number maupun string desimal. Tanggal memakai format YYYY-MM-DD.
//
// Tidak ada bidang identitas (kolom IV) di sini karena identitas sengaja tidak
// disimpan; mengirimkannya akan ditolak decoder yang menolak bidang tak dikenal.
type UpdateKepemilikanItemInput struct {
	ID                    string          `json:"id"`
	ShareholderName       string          `json:"shareholder_name"`
	ShareholderAddress    string          `json:"shareholder_address"`
	ShareholderTypeCode   string          `json:"shareholder_type_code"`
	ShareholderStatusCode string          `json:"shareholder_status_code"`
	NominalAmount         decimal.Decimal `json:"nominal_amount"`
	OwnershipPercentage   decimal.Decimal `json:"ownership_percentage"`
	ChangeStatusCode      string          `json:"change_status_code"`
	AsOf                  string          `json:"as_of"`
	Status                string          `json:"status"`
	Note                  string          `json:"note"`
}

// BuildKepemilikanItem memvalidasi masukan dan membentuk baris yang siap disimpan.
// ID kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu
// tidak disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildKepemilikanItem(in UpdateKepemilikanItemInput) (KepemilikanItem, error) {
	out := KepemilikanItem{
		ShareholderName:       strings.TrimSpace(in.ShareholderName),
		ShareholderAddress:    strings.TrimSpace(in.ShareholderAddress),
		ShareholderTypeCode:   strings.TrimSpace(in.ShareholderTypeCode),
		ShareholderStatusCode: strings.TrimSpace(in.ShareholderStatusCode),
		NominalAmount:         in.NominalAmount,
		OwnershipPercentage:   in.OwnershipPercentage,
		ChangeStatusCode:      strings.TrimSpace(in.ChangeStatusCode),
		Status:                strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                  strings.TrimSpace(in.Note),
	}
	if out.ShareholderName == "" {
		return out, fmt.Errorf("%w: nama pemegang saham wajib diisi", ErrKepemilikanInputInvalid)
	}
	if len(out.ShareholderName) > 255 {
		return out, fmt.Errorf("%w: nama pemegang saham maksimal 255 karakter", ErrKepemilikanInputInvalid)
	}
	if len(out.ShareholderAddress) > 255 {
		return out, fmt.Errorf("%w: alamat pemegang saham maksimal 255 karakter", ErrKepemilikanInputInvalid)
	}
	switch out.ShareholderTypeCode {
	case KepemilikanTypePerorangan, KepemilikanTypeBadanHukum,
		KepemilikanTypePemerintahDaerah, KepemilikanTypePublik:
	default:
		return out, fmt.Errorf("%w: jenis pemegang saham hanya 01, 02, 03, atau 04",
			ErrKepemilikanInputInvalid)
	}
	switch out.ShareholderStatusCode {
	case KepemilikanStatusPSP, KepemilikanStatusNonPSP:
	default:
		return out, fmt.Errorf("%w: status pemegang saham hanya 01 atau 02",
			ErrKepemilikanInputInvalid)
	}
	if out.NominalAmount.IsNegative() {
		return out, fmt.Errorf("%w: jumlah nominal tidak boleh negatif", ErrKepemilikanInputInvalid)
	}
	if out.OwnershipPercentage.IsNegative() {
		return out, fmt.Errorf("%w: persentase kepemilikan tidak boleh negatif",
			ErrKepemilikanInputInvalid)
	}
	if out.OwnershipPercentage.GreaterThan(decimal.NewFromInt(100)) {
		return out, fmt.Errorf("%w: persentase kepemilikan tidak boleh melebihi 100",
			ErrKepemilikanInputInvalid)
	}
	switch out.ChangeStatusCode {
	case KepemilikanChangeBaru, KepemilikanChangeUbahPSP,
		KepemilikanChangeUbahTanpaPSP, KepemilikanChangeTidakAda:
	default:
		return out, fmt.Errorf("%w: status perubahan hanya 1, 2, 3, atau 9",
			ErrKepemilikanInputInvalid)
	}
	if out.Status == "" {
		out.Status = KepemilikanStatusAktif
	}
	if out.Status != KepemilikanStatusAktif && out.Status != KepemilikanStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrKepemilikanInputInvalid)
	}
	var err error
	if out.ID, err = parseKepemilikanID(in.ID); err != nil {
		return out, err
	}
	if out.AsOf, err = parseKepemilikanDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseKepemilikanID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseKepemilikanID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrKepemilikanInputInvalid)
	}
	return id, nil
}

// parseKepemilikanDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama
// kolomnya.
func parseKepemilikanDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrKepemilikanInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrKepemilikanInputInvalid, field)
	}
	return t, nil
}

// KepemilikanRegisterRepository menyimpan dan membaca register pemegang saham BPR.
// Seluruh pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit
// berada pada transaksi yang sama.
type KepemilikanRegisterRepository interface {
	ListItems(ctx context.Context) ([]KepemilikanItem, error)
	// ListKepemilikanForOJK membaca baris AKTIF yang as_of-nya berada pada bulan
	// asOf, urutan deterministik. Sumber Form 00.01.
	ListKepemilikanForOJK(ctx context.Context, asOf time.Time) ([]KepemilikanItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu pemegang saham.
	UpsertItemTx(ctx context.Context, tx any, item KepemilikanItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu pemegang saham; found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// KepemilikanReport adalah keluaran baca-saja register pemegang saham untuk satu posisi.
type KepemilikanReport struct {
	AsOf  time.Time         `json:"as_of"`
	Items []KepemilikanItem `json:"items"`
}

// KepemilikanRegisterService merakit laporan register kepemilikan BPR dan melayani
// pengisian berizin.
type KepemilikanRegisterService interface {
	// KepemilikanReport menyusun laporan untuk satu posisi. Bank-wide.
	KepemilikanReport(ctx context.Context, asOf time.Time, actor Actor) (KepemilikanReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]KepemilikanItem, error)
	// UpsertItem menyimpan satu pemegang saham (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdateKepemilikanItemInput, actor Actor) (*KepemilikanItem, error)
	// DeleteItem menghapus satu pemegang saham, teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
