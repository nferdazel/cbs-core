package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// aset_tetap.go memuat fondasi data Form 08.00 "DAFTAR ASET TETAP, INVENTARIS, DAN
// ASET TIDAK BERWUJUD", Lampiran II SEOJK No. 16/SEOJK.03/2024: register aset yang
// bank catat, per aset.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 08.00 "DAFTAR ASSET TETAP, INVENTARIS, DAN ASET TIDAK BERWUJUD": PDF #page
//     182 (hlm. tercetak 130). Sembilan kolom: I Sandi Kantor, II Jenis Aset,
//     III Sumber Perolehan, IV Status Aset, V Biaya Perolehan,
//     VI Akumulasi Penyusutan/Amortisasi, VII Akumulasi Kerugian Penurunan Nilai,
//     VIII Nilai Tercatat, IX Metode Pengukuran. Ada baris JUMLAH.
//   - Sandi: PDF #page 183 (hlm. 131). II 101/102/103/104/199 (aset tetap dan
//     inventaris) dan 201/202/299 (aset tidak berwujud); III 01/02/03/99;
//     IV 1/2; IX 1/2.
//   - Penjelasan: PDF #page 184-186 (hlm. 132-134). Baris per kombinasi jenis aset /
//     sumber perolehan / status / metode pengukuran; bank dapat menggabungkan aset yang
//     memiliki kesamaan sandi rincian dan angka pada kolom I s.d. VI. Status Aset
//     dikosongkan untuk aset tidak berwujud. Biaya Perolehan memakai nilai setelah
//     revaluasi bila ada. Nilai Tercatat = biaya perolehan - akumulasi penyusutan atau
//     amortisasi - kerugian penurunan nilai.
//
// Prinsip yang dipegang berkas ini:
//   - Register disimpan PER ASET; pengelompokan per kombinasi dilakukan builder laporan
//     (form08_00.go), bukan di sini.
//   - Kolom VIII "Nilai Tercatat" TIDAK disimpan: ia turunan yang dihitung laporan dari
//     V - VI - VII. Lihat NilaiTercatat.
//   - Status Aset (IV) NULLABLE: form mengosongkannya untuk aset tidak berwujud. Untuk
//     aset berwujud yang belum diisi, laporan menuliskan "-" beserta alasannya, bukan
//     menebak. Builder mengosongkan status aset tidak berwujud saat menyusun form.
//   - Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
//   - Seluruh angka dan sandi adalah isian bank. Sistem tidak menghitung biaya perolehan
//     maupun akumulasi.

// Sandi jenis aset baku Form 08.00 (PDF #page 183). Awalan "1" aset tetap dan
// inventaris, awalan "2" aset tidak berwujud.
const (
	AsetJenisTanah                 = "101"
	AsetJenisBangunan              = "102"
	AsetJenisPeralatanPerlengkapan = "103"
	AsetJenisKendaraan             = "104"
	AsetJenisAsetTetapLainnya      = "199"
	AsetJenisSoftware              = "201"
	AsetJenisGoodwill              = "202"
	AsetJenisTidakBerwujudLainnya  = "299"
)

// Sandi sumber perolehan baku Form 08.00 (PDF #page 183).
const (
	AsetSumberSewaPembiayaan = "01"
	AsetSumberModalDisetor   = "02"
	AsetSumberModalSumbangan = "03"
	AsetSumberLainnya        = "99"
)

// Sandi status aset baku Form 08.00 (PDF #page 183).
const (
	AsetStatusDijaminkan      = "1"
	AsetStatusTidakDijaminkan = "2"
)

// Sandi metode pengukuran baku Form 08.00 (PDF #page 183).
const (
	AsetMetodeBiaya     = "1"
	AsetMetodeRevaluasi = "2"
)

// Status baku register aset.
const (
	AsetStatusRegisterAktif    = "AKTIF"
	AsetStatusRegisterNonaktif = "NONAKTIF"
)

var (
	// ErrAsetTetapBankWide menandai permintaan register aset oleh aktor yang tidak
	// berwenang atas seluruh bank. Register ini bank-wide.
	ErrAsetTetapBankWide = NewLocalizedError("aset_tetap_bank_wide",
		"register aset tetap bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrAsetTetapInputInvalid menandai data register aset tidak sah.
	ErrAsetTetapInputInvalid = NewLocalizedError("aset_tetap_input_invalid",
		"data register aset tetap tidak valid")
	// ErrAsetTetapNotFound menandai baris yang hendak dihapus tidak ada.
	ErrAsetTetapNotFound = NewLocalizedError("aset_tetap_not_found",
		"data register aset tetap tidak ditemukan")
)

// AsetJenisTidakBerwujud melaporkan apakah sandi jenis aset adalah aset tidak
// berwujud (kode 2xx, PDF #page 183). Untuk jenis ini kolom IV Status Aset
// dikosongkan saat menyusun form.
func AsetJenisTidakBerwujud(jenisAsetCode string) bool {
	return strings.HasPrefix(strings.TrimSpace(jenisAsetCode), "2")
}

// AsetTetapItem adalah satu aset (satu baris register) yang bank catat. Kolom VIII
// Nilai Tercatat tidak menjadi bidang di sini: nilainya turunan dan dihitung laporan
// lewat NilaiTercatat.
type AsetTetapItem struct {
	ID uuid.UUID `json:"id"`
	// JenisAsetCode kolom II: 101/102/103/104/199 atau 201/202/299.
	JenisAsetCode string `json:"jenis_aset_code"`
	// SumberPerolehanCode kolom III: 01/02/03/99.
	SumberPerolehanCode string `json:"sumber_perolehan_code"`
	// StatusAsetCode kolom IV: "1" Dijaminkan, "2" Tidak Dijaminkan, atau "" bila
	// belum diisi (dan untuk aset tidak berwujud, yang dikosongkan form).
	StatusAsetCode string `json:"status_aset_code"`
	// BiayaPerolehan kolom V, rupiah penuh; selalu >= 0 (isian bank). Bila aset telah
	// direvaluasi, nilainya adalah nilai setelah revaluasi.
	BiayaPerolehan decimal.Decimal `json:"biaya_perolehan"`
	// AkumulasiPenyusutanAmortisasi kolom VI sebagai BESARAN penyusutan >= 0.
	AkumulasiPenyusutanAmortisasi decimal.Decimal `json:"akumulasi_penyusutan_amortisasi"`
	// AkumulasiKerugianPenurunanNilai kolom VII sebagai BESARAN kerugian >= 0.
	AkumulasiKerugianPenurunanNilai decimal.Decimal `json:"akumulasi_kerugian_penurunan_nilai"`
	// MetodePengukuranCode kolom IX: 1 Model Biaya, 2 Model Revaluasi.
	MetodePengukuranCode string    `json:"metode_pengukuran_code"`
	AsOf                 time.Time `json:"as_of"`
	Status               string    `json:"status"`
	Note                 string    `json:"note,omitempty"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// NilaiTercatat menghitung kolom VIII "Nilai Tercatat": biaya perolehan (V) dikurangi
// akumulasi penyusutan atau amortisasi (VI) dan akumulasi kerugian penurunan nilai
// (VII) (PDF #page 185). ok=false bila hasilnya negatif (akumulasi melebihi biaya);
// kolom VIII ditulis "-" beserta alasannya, bukan angka negatif yang menyesatkan.
func (i AsetTetapItem) NilaiTercatat() (decimal.Decimal, bool) {
	nilai := i.BiayaPerolehan.Sub(i.AkumulasiPenyusutanAmortisasi).
		Sub(i.AkumulasiKerugianPenurunanNilai)
	if nilai.IsNegative() {
		return decimal.Zero, false
	}
	return nilai, true
}

// UpdateAsetTetapItemInput adalah isi upsert satu aset. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON number
// maupun string desimal. Tanggal memakai format YYYY-MM-DD.
type UpdateAsetTetapItemInput struct {
	ID                              string          `json:"id"`
	JenisAsetCode                   string          `json:"jenis_aset_code"`
	SumberPerolehanCode             string          `json:"sumber_perolehan_code"`
	StatusAsetCode                  string          `json:"status_aset_code"`
	BiayaPerolehan                  decimal.Decimal `json:"biaya_perolehan"`
	AkumulasiPenyusutanAmortisasi   decimal.Decimal `json:"akumulasi_penyusutan_amortisasi"`
	AkumulasiKerugianPenurunanNilai decimal.Decimal `json:"akumulasi_kerugian_penurunan_nilai"`
	MetodePengukuranCode            string          `json:"metode_pengukuran_code"`
	AsOf                            string          `json:"as_of"`
	Status                          string          `json:"status"`
	Note                            string          `json:"note"`
}

// BuildAsetTetapItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID
// kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak
// disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildAsetTetapItem(in UpdateAsetTetapItemInput) (AsetTetapItem, error) {
	out := AsetTetapItem{
		JenisAsetCode:                   strings.TrimSpace(in.JenisAsetCode),
		SumberPerolehanCode:             strings.TrimSpace(in.SumberPerolehanCode),
		StatusAsetCode:                  strings.TrimSpace(in.StatusAsetCode),
		BiayaPerolehan:                  in.BiayaPerolehan,
		AkumulasiPenyusutanAmortisasi:   in.AkumulasiPenyusutanAmortisasi,
		AkumulasiKerugianPenurunanNilai: in.AkumulasiKerugianPenurunanNilai,
		MetodePengukuranCode:            strings.TrimSpace(in.MetodePengukuranCode),
		Status:                          strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                            strings.TrimSpace(in.Note),
	}
	switch out.JenisAsetCode {
	case AsetJenisTanah, AsetJenisBangunan, AsetJenisPeralatanPerlengkapan,
		AsetJenisKendaraan, AsetJenisAsetTetapLainnya,
		AsetJenisSoftware, AsetJenisGoodwill, AsetJenisTidakBerwujudLainnya:
	default:
		return out, fmt.Errorf("%w: jenis aset hanya 101, 102, 103, 104, 199, 201, 202, atau 299",
			ErrAsetTetapInputInvalid)
	}
	switch out.SumberPerolehanCode {
	case AsetSumberSewaPembiayaan, AsetSumberModalDisetor,
		AsetSumberModalSumbangan, AsetSumberLainnya:
	default:
		return out, fmt.Errorf("%w: sumber perolehan hanya 01, 02, 03, atau 99",
			ErrAsetTetapInputInvalid)
	}
	// Status aset boleh kosong (aset tidak berwujud, dan aset berwujud yang belum
	// diisi). Bila terisi, hanya 1 atau 2.
	switch out.StatusAsetCode {
	case "", AsetStatusDijaminkan, AsetStatusTidakDijaminkan:
	default:
		return out, fmt.Errorf("%w: status aset hanya 1, 2, atau kosong", ErrAsetTetapInputInvalid)
	}
	if out.BiayaPerolehan.IsNegative() {
		return out, fmt.Errorf("%w: biaya perolehan tidak boleh negatif", ErrAsetTetapInputInvalid)
	}
	if out.AkumulasiPenyusutanAmortisasi.IsNegative() {
		return out, fmt.Errorf("%w: akumulasi penyusutan atau amortisasi tidak boleh negatif",
			ErrAsetTetapInputInvalid)
	}
	if out.AkumulasiKerugianPenurunanNilai.IsNegative() {
		return out, fmt.Errorf("%w: akumulasi kerugian penurunan nilai tidak boleh negatif",
			ErrAsetTetapInputInvalid)
	}
	switch out.MetodePengukuranCode {
	case AsetMetodeBiaya, AsetMetodeRevaluasi:
	default:
		return out, fmt.Errorf("%w: metode pengukuran hanya 1 atau 2", ErrAsetTetapInputInvalid)
	}
	if out.Status == "" {
		out.Status = AsetStatusRegisterAktif
	}
	if out.Status != AsetStatusRegisterAktif && out.Status != AsetStatusRegisterNonaktif {
		return out, fmt.Errorf("%w: status register hanya AKTIF atau NONAKTIF", ErrAsetTetapInputInvalid)
	}
	var err error
	if out.ID, err = parseAsetTetapID(in.ID); err != nil {
		return out, err
	}
	if out.AsOf, err = parseAsetTetapDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseAsetTetapID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseAsetTetapID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrAsetTetapInputInvalid)
	}
	return id, nil
}

// parseAsetTetapDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parseAsetTetapDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrAsetTetapInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrAsetTetapInputInvalid, field)
	}
	return t, nil
}

// AsetTetapRegisterRepository menyimpan dan membaca register aset. Seluruh pembacaan
// bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada transaksi
// yang sama.
type AsetTetapRegisterRepository interface {
	ListItems(ctx context.Context) ([]AsetTetapItem, error)
	// ListAsetTetapForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 08.00.
	ListAsetTetapForOJK(ctx context.Context, asOf time.Time) ([]AsetTetapItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu aset.
	UpsertItemTx(ctx context.Context, tx any, item AsetTetapItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu aset; found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// AsetTetapReport adalah keluaran baca-saja register aset untuk satu posisi.
type AsetTetapReport struct {
	AsOf  time.Time       `json:"as_of"`
	Items []AsetTetapItem `json:"items"`
}

// AsetTetapRegisterService merakit laporan register aset dan melayani pengisian berizin.
type AsetTetapRegisterService interface {
	// AsetTetapReport menyusun laporan untuk satu posisi. Bank-wide.
	AsetTetapReport(ctx context.Context, asOf time.Time, actor Actor) (AsetTetapReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]AsetTetapItem, error)
	// UpsertItem menyimpan satu aset (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdateAsetTetapItemInput, actor Actor) (*AsetTetapItem, error)
	// DeleteItem menghapus satu aset, teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
