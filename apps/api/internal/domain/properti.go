package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// properti.go memuat fondasi data Form 17.00 "DAFTAR PROPERTI TERBENGKALAI",
// Lampiran II SEOJK No. 16/SEOJK.03/2024: register properti terbengkalai per properti
// yang bank catat.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 17.00 "DAFTAR PROPERTI TERBENGKALAI": PDF #page 229 (hlm. tercetak 177).
//     Sepuluh kolom: I Sandi Kantor, II No. Register, III Jenis Properti Terbengkalai,
//     IV Alamat Properti Terbengkalai, V Koordinat, VI Tanggal Penetapan,
//     VII Biaya Perolehan atau Nilai Wajar, VIII Akumulasi Penyusutan atau Amortisasi,
//     IX Jumlah, X Metode Pengukuran. Satu baris = satu properti; TIDAK ADA baris JUMLAH.
//   - Sandi: PDF #page 230 (hlm. 178). III 1 Tanah / 2 Bangunan / 3 Tanah dan Bangunan;
//     X 1 Model Biaya / 2 Model Revaluasi (Nilai Wajar).
//   - Penjelasan: PDF #page 231-232 (hlm. 179-180). II No. Register unik, satu register
//     untuk satu properti, "no reuse atau no recycle", mandatory. IX Jumlah = nilai
//     pengakuan awal dikurangi akumulasi penyusutan/amortisasi termasuk akumulasi
//     kerugian penurunan nilai.
//
// Prinsip yang dipegang berkas ini:
//   - Kolom IX "Jumlah" TIDAK disimpan: ia turunan yang dihitung laporan (VII - VIII).
//     Lihat Jumlah.
//   - Seluruh angka, sandi, dan teks lain adalah isian bank. Sistem tidak menghitung
//     biaya perolehan maupun akumulasi.
//   - Nomor Register (II) unik dan TIDAK BOLEH dipakai ulang (PDF #page 231). Karena itu
//     penghapusan register adalah soft-delete (status -> NONAKTIF); baris tetap ada
//     sehingga nomornya tidak pernah kembali bebas. Pelanggaran keunikan dipetakan ke
//     ErrPropertiNoRegisterUsed.
//   - Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti Form 07.00.
//
// Sandi Kantor (kolom I) TIDAK ada di register ini: sama seperti form bank-wide lain,
// ia diambil dari kantor pelapor tunggal. Tidak ada kolom identitas pribadi di form ini.

// Sandi jenis properti terbengkalai baku Form 17.00 (PDF #page 230).
const (
	PropertiJenisTanah         = "1"
	PropertiJenisBangunan      = "2"
	PropertiJenisTanahBangunan = "3"
	// Sandi metode pengukuran baku Form 17.00 (PDF #page 230).
	PropertiMetodeBiaya     = "1"
	PropertiMetodeRevaluasi = "2"
	// Status baku register properti terbengkalai.
	PropertiStatusAktif    = "AKTIF"
	PropertiStatusNonaktif = "NONAKTIF"
)

var (
	// ErrPropertiBankWide menandai permintaan register properti terbengkalai oleh aktor
	// yang tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrPropertiBankWide = NewLocalizedError("properti_bank_wide",
		"register properti terbengkalai bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrPropertiInputInvalid menandai data register properti terbengkalai tidak sah.
	ErrPropertiInputInvalid = NewLocalizedError("properti_input_invalid",
		"data register properti terbengkalai tidak valid")
	// ErrPropertiNotFound menandai baris yang hendak dihapus tidak ada.
	ErrPropertiNotFound = NewLocalizedError("properti_not_found",
		"data register properti terbengkalai tidak ditemukan")
	// ErrPropertiNoRegisterUsed menandai No. Register yang sudah pernah dipakai, termasuk
	// oleh baris NONAKTIF. Aturan no reuse/no recycle Form 17.00 (PDF #page 231).
	ErrPropertiNoRegisterUsed = NewLocalizedError("properti_no_register_used",
		"nomor register properti terbengkalai sudah pernah dipakai dan tidak boleh dipakai ulang")
)

// PropertiItem adalah satu properti terbengkalai (satu baris Form 17.00) yang bank
// catat. Kolom IX Jumlah tidak menjadi bidang di sini: nilainya turunan dan dihitung
// laporan lewat Jumlah.
type PropertiItem struct {
	ID uuid.UUID `json:"id"`
	// NoRegister kolom II: kode unik properti, mandatory, no reuse/no recycle.
	NoRegister string `json:"no_register"`
	// JenisPropertiCode kolom III: 1 Tanah / 2 Bangunan / 3 Tanah dan Bangunan.
	JenisPropertiCode string `json:"jenis_properti_code"`
	// AlamatProperti kolom IV (wajib).
	AlamatProperti string `json:"alamat_properti"`
	// Koordinat kolom V, teks bebas; boleh kosong.
	Koordinat string `json:"koordinat"`
	// TanggalPenetapan kolom VI.
	TanggalPenetapan time.Time `json:"tanggal_penetapan"`
	// BiayaPerolehanNilaiWajar kolom VII, rupiah penuh; selalu >= 0 (isian bank).
	BiayaPerolehanNilaiWajar decimal.Decimal `json:"biaya_perolehan_atau_nilai_wajar"`
	// AkumulasiPenyusutanAmortisasi kolom VIII sebagai BESARAN penyusutan >= 0; termasuk
	// akumulasi penurunan nilai bila ada (PDF #page 231).
	AkumulasiPenyusutanAmortisasi decimal.Decimal `json:"akumulasi_penyusutan_atau_amortisasi"`
	// MetodePengukuranCode kolom X: 1 Model Biaya / 2 Model Revaluasi.
	MetodePengukuranCode string    `json:"metode_pengukuran_code"`
	AsOf                 time.Time `json:"as_of"`
	Status               string    `json:"status"`
	Note                 string    `json:"note,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// Jumlah menghitung kolom IX "Jumlah": biaya perolehan atau nilai wajar (VII) dikurangi
// akumulasi penyusutan atau amortisasi (VIII) termasuk akumulasi kerugian penurunan
// nilai (PDF #page 232). ok=false bila hasilnya negatif (akumulasi melebihi biaya);
// kolom IX ditulis "-" beserta alasannya, bukan angka negatif yang menyesatkan.
func (i PropertiItem) Jumlah() (decimal.Decimal, bool) {
	jumlah := i.BiayaPerolehanNilaiWajar.Sub(i.AkumulasiPenyusutanAmortisasi)
	if jumlah.IsNegative() {
		return decimal.Zero, false
	}
	return jumlah, true
}

// UpdatePropertiItemInput adalah isi upsert satu properti. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON number
// maupun string desimal. Tanggal memakai format YYYY-MM-DD.
type UpdatePropertiItemInput struct {
	ID                            string          `json:"id"`
	NoRegister                    string          `json:"no_register"`
	JenisPropertiCode             string          `json:"jenis_properti_code"`
	AlamatProperti                string          `json:"alamat_properti"`
	Koordinat                     string          `json:"koordinat"`
	TanggalPenetapan              string          `json:"tanggal_penetapan"`
	BiayaPerolehanNilaiWajar      decimal.Decimal `json:"biaya_perolehan_atau_nilai_wajar"`
	AkumulasiPenyusutanAmortisasi decimal.Decimal `json:"akumulasi_penyusutan_atau_amortisasi"`
	MetodePengukuranCode          string          `json:"metode_pengukuran_code"`
	AsOf                          string          `json:"as_of"`
	Status                        string          `json:"status"`
	Note                          string          `json:"note"`
}

// BuildPropertiItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID
// kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak
// disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildPropertiItem(in UpdatePropertiItemInput) (PropertiItem, error) {
	out := PropertiItem{
		NoRegister:                    strings.TrimSpace(in.NoRegister),
		JenisPropertiCode:             strings.TrimSpace(in.JenisPropertiCode),
		AlamatProperti:                strings.TrimSpace(in.AlamatProperti),
		Koordinat:                     strings.TrimSpace(in.Koordinat),
		BiayaPerolehanNilaiWajar:      in.BiayaPerolehanNilaiWajar,
		AkumulasiPenyusutanAmortisasi: in.AkumulasiPenyusutanAmortisasi,
		MetodePengukuranCode:          strings.TrimSpace(in.MetodePengukuranCode),
		Status:                        strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                          strings.TrimSpace(in.Note),
	}
	if out.NoRegister == "" {
		return out, fmt.Errorf("%w: No. Register wajib diisi", ErrPropertiInputInvalid)
	}
	if len(out.NoRegister) > 64 {
		return out, fmt.Errorf("%w: No. Register maksimal 64 karakter", ErrPropertiInputInvalid)
	}
	switch out.JenisPropertiCode {
	case PropertiJenisTanah, PropertiJenisBangunan, PropertiJenisTanahBangunan:
	default:
		return out, fmt.Errorf("%w: jenis properti hanya 1, 2, atau 3", ErrPropertiInputInvalid)
	}
	if out.AlamatProperti == "" {
		return out, fmt.Errorf("%w: alamat properti wajib diisi", ErrPropertiInputInvalid)
	}
	if len(out.AlamatProperti) > 255 {
		return out, fmt.Errorf("%w: alamat properti maksimal 255 karakter", ErrPropertiInputInvalid)
	}
	if len(out.Koordinat) > 64 {
		return out, fmt.Errorf("%w: koordinat maksimal 64 karakter", ErrPropertiInputInvalid)
	}
	if out.BiayaPerolehanNilaiWajar.IsNegative() {
		return out, fmt.Errorf("%w: biaya perolehan atau nilai wajar tidak boleh negatif",
			ErrPropertiInputInvalid)
	}
	if out.AkumulasiPenyusutanAmortisasi.IsNegative() {
		return out, fmt.Errorf("%w: akumulasi penyusutan atau amortisasi tidak boleh negatif",
			ErrPropertiInputInvalid)
	}
	switch out.MetodePengukuranCode {
	case PropertiMetodeBiaya, PropertiMetodeRevaluasi:
	default:
		return out, fmt.Errorf("%w: metode pengukuran hanya 1 atau 2", ErrPropertiInputInvalid)
	}
	if out.Status == "" {
		out.Status = PropertiStatusAktif
	}
	if out.Status != PropertiStatusAktif && out.Status != PropertiStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrPropertiInputInvalid)
	}
	var err error
	if out.ID, err = parsePropertiID(in.ID); err != nil {
		return out, err
	}
	if out.TanggalPenetapan, err = parsePropertiDate(in.TanggalPenetapan, "tanggal_penetapan"); err != nil {
		return out, err
	}
	if out.AsOf, err = parsePropertiDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parsePropertiID mengurai id yang dikirim klien; kosong berarti buat baru.
func parsePropertiID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrPropertiInputInvalid)
	}
	return id, nil
}

// parsePropertiDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parsePropertiDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrPropertiInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrPropertiInputInvalid, field)
	}
	return t, nil
}

// PropertiRegisterRepository menyimpan dan membaca register properti terbengkalai.
// Seluruh pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit
// berada pada transaksi yang sama.
type PropertiRegisterRepository interface {
	ListItems(ctx context.Context) ([]PropertiItem, error)
	// ListPropertiForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 17.00.
	ListPropertiForOJK(ctx context.Context, asOf time.Time) ([]PropertiItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu properti.
	// No. Register yang sudah dipakai baris lain (termasuk NONAKTIF) ditolak
	// ErrPropertiNoRegisterUsed.
	UpsertItemTx(ctx context.Context, tx any, item PropertiItem, actorID uuid.UUID) error
	// SoftDeleteItemTx menonaktifkan satu properti (status -> NONAKTIF); baris tetap
	// ada sehingga nomor register-nya tidak dapat dipakai ulang. found=false bila baris
	// tidak ada.
	SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error)
}

// PropertiReport adalah keluaran baca-saja register properti terbengkalai untuk satu
// posisi.
type PropertiReport struct {
	AsOf  time.Time      `json:"as_of"`
	Items []PropertiItem `json:"items"`
}

// PropertiRegisterService merakit laporan register properti terbengkalai dan melayani
// pengisian berizin.
type PropertiRegisterService interface {
	// PropertiReport menyusun laporan untuk satu posisi. Bank-wide.
	PropertiReport(ctx context.Context, asOf time.Time, actor Actor) (PropertiReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]PropertiItem, error)
	// UpsertItem menyimpan satu properti (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdatePropertiItemInput, actor Actor) (*PropertiItem, error)
	// DeleteItem menonaktifkan satu properti (soft-delete), teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
