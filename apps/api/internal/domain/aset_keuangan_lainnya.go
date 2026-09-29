package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// aset_keuangan_lainnya.go memuat fondasi data Form 18.00 "DAFTAR ASET KEUANGAN
// LAINNYA", Lampiran II SEOJK No. 16/SEOJK.03/2024: register aset keuangan lainnya
// per rekening unik yang bank catat.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 18.00 "DAFTAR ASET KEUANGAN LAINNYA": PDF #page 233 (hlm. tercetak 181) dan
//     lanjutan PDF #page 234 (hlm. 182). Lima belas kolom: I Sandi Kantor,
//     II No. Rekening, III ID Pihak Lawan, IV Jenis, V Tanggal Mulai,
//     VI Tanggal Jatuh Tempo, VII Suku Bunga, VIII Nominal,
//     IX Nilai Agunan yang Dapat Diperhitungkan, X Cadangan Kerugian Penurunan Nilai,
//     XI CKPN Aset Baik, XII CKPN Aset Kurang Baik, XIII CKPN Aset Tidak Baik,
//     XIV Klasifikasi Aset Keuangan, XV Jenis CKPN. Satu baris = satu rekening;
//     TIDAK ADA baris JUMLAH (PDF #page 233-234).
//   - Sandi: PDF #page 235 (hlm. 183). IV 10 Tagihan fraud / 99 Tagihan lainnya;
//     XIV 1 nilai wajar melalui laba rugi / 2 nilai wajar melalui penghasilan
//     komprehensif lain / 3 biaya perolehan diamortisasi; XV 1 Individual / 2 Kolektif.
//   - Penjelasan: PDF #page 237-238 (hlm. 185-186). II nomor rekening unik, "tidak boleh
//     sama". IX nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA.
//     Format tanggal form ini TT-MM-TTTT (PDF #page 235).
//
// Prinsip yang dipegang berkas ini:
//   - Tidak ada kolom turunan: form tidak memberikan rumus yang mengikat untuk kolom X
//     maupun blok CKPN, sehingga seluruh nilai adalah isian bank dan laporan menuliskan
//     apa adanya.
//   - Sandi IV/XIV/XV adalah isian bank dan ditegakkan di sini; tidak diturunkan dari
//     kolom lain dan tidak menerapkan aturan historis "Desember 2024 = Kolektif".
//   - Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti Form 16.00/17.00.
//   - No. Rekening (II) unik dan "tidak boleh sama" (PDF #page 237). Karena itu
//     penghapusan register adalah soft-delete (status -> NONAKTIF); baris tetap ada
//     sehingga nomornya tidak pernah kembali bebas. Pelanggaran keunikan dipetakan ke
//     ErrAsetKeuanganNoRekeningUsed.
//   - Kolom III "ID Pihak Lawan" bukan sandi Lampiran 02 (form 18.00 tidak memuat sandi
//     untuk kolom ini): disimpan sebagai teks apa adanya, tanpa daftar sandi karangan.

// Sandi jenis aset keuangan lainnya baku Form 18.00 (PDF #page 235).
const (
	AsetKeuanganJenisTagihanFraud = "10"
	AsetKeuanganJenisTagihanLain  = "99"
)

// Sandi klasifikasi aset keuangan baku Form 18.00 (PDF #page 235).
const (
	AsetKeuanganKlasifikasiNilaiWajarLabaRugi       = "1"
	AsetKeuanganKlasifikasiNilaiWajarPKL            = "2"
	AsetKeuanganKlasifikasiBiayaPerolehanDiamortasi = "3"
)

// Sandi jenis CKPN baku Form 18.00 (PDF #page 235).
const (
	AsetKeuanganJenisCKPNIndividual = "1"
	AsetKeuanganJenisCKPNKolektif   = "2"
)

// Status baku register aset keuangan lainnya.
const (
	AsetKeuanganStatusAktif    = "AKTIF"
	AsetKeuanganStatusNonaktif = "NONAKTIF"
)

var (
	// ErrAsetKeuanganBankWide menandai permintaan register aset keuangan lainnya oleh
	// aktor yang tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrAsetKeuanganBankWide = NewLocalizedError("aset_keuangan_bank_wide",
		"register aset keuangan lainnya bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrAsetKeuanganInputInvalid menandai data register aset keuangan lainnya tidak sah.
	ErrAsetKeuanganInputInvalid = NewLocalizedError("aset_keuangan_input_invalid",
		"data register aset keuangan lainnya tidak valid")
	// ErrAsetKeuanganNotFound menandai baris yang hendak dinonaktifkan tidak ada.
	ErrAsetKeuanganNotFound = NewLocalizedError("aset_keuangan_not_found",
		"data register aset keuangan lainnya tidak ditemukan")
	// ErrAsetKeuanganNoRekeningUsed menandai No. Rekening yang sudah pernah dipakai,
	// termasuk oleh baris NONAKTIF. Aturan "nomor rekening tidak boleh sama" Form 18.00
	// (PDF #page 237).
	ErrAsetKeuanganNoRekeningUsed = NewLocalizedError("aset_keuangan_no_rekening_used",
		"nomor rekening aset keuangan lainnya sudah pernah dipakai dan tidak boleh dipakai ulang")
)

// AsetKeuanganItem adalah satu rekening aset keuangan lainnya (satu baris Form 18.00)
// yang bank catat. Tidak ada kolom turunan di sini: seluruh nilai adalah isian bank.
type AsetKeuanganItem struct {
	ID uuid.UUID `json:"id"`
	// NoRekening kolom II: nomor rekening unik per rekening, "tidak boleh sama".
	NoRekening string `json:"no_rekening"`
	// CounterpartyID kolom III: pengenal pihak lawan yang bank catat.
	CounterpartyID string `json:"counterparty_id"`
	// JenisCode kolom IV: 10 Tagihan fraud, 99 Tagihan lainnya.
	JenisCode string `json:"jenis_code"`
	// TanggalMulai kolom V.
	TanggalMulai time.Time `json:"tanggal_mulai"`
	// TanggalJatuhTempo kolom VI.
	TanggalJatuhTempo time.Time `json:"tanggal_jatuh_tempo"`
	// SukuBunga kolom VII, persen; isian bank.
	SukuBunga decimal.Decimal `json:"suku_bunga"`
	// Nominal kolom VIII, rupiah penuh; isian bank.
	Nominal decimal.Decimal `json:"nominal"`
	// NilaiAgunanDiperhitungkan kolom IX, rupiah penuh; isian bank.
	NilaiAgunanDiperhitungkan decimal.Decimal `json:"nilai_agunan_diperhitungkan"`
	// CKPN kolom X, rupiah penuh; isian bank.
	CKPN decimal.Decimal `json:"cadangan_kerugian_penurunan_nilai"`
	// CKPNAsetBaik kolom XI, rupiah penuh; isian bank.
	CKPNAsetBaik decimal.Decimal `json:"ckpn_aset_baik"`
	// CKPNAsetKurangBaik kolom XII, rupiah penuh; isian bank.
	CKPNAsetKurangBaik decimal.Decimal `json:"ckpn_aset_kurang_baik"`
	// CKPNAsetTidakBaik kolom XIII, rupiah penuh; isian bank.
	CKPNAsetTidakBaik decimal.Decimal `json:"ckpn_aset_tidak_baik"`
	// KlasifikasiAsetKeuanganCode kolom XIV: 1 VW laba rugi, 2 VW PKL,
	// 3 biaya perolehan diamortisasi.
	KlasifikasiAsetKeuanganCode string `json:"klasifikasi_aset_keuangan_code"`
	// JenisCKPNCode kolom XV: 1 Individual, 2 Kolektif.
	JenisCKPNCode string    `json:"jenis_ckpn_code"`
	AsOf          time.Time `json:"as_of"`
	Status        string    `json:"status"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpdateAsetKeuanganItemInput adalah isi upsert satu aset keuangan lainnya. ID kosong =
// buat baru (id dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON
// number maupun string desimal. Tanggal memakai format YYYY-MM-DD.
type UpdateAsetKeuanganItemInput struct {
	ID                          string          `json:"id"`
	NoRekening                  string          `json:"no_rekening"`
	CounterpartyID              string          `json:"counterparty_id"`
	JenisCode                   string          `json:"jenis_code"`
	TanggalMulai                string          `json:"tanggal_mulai"`
	TanggalJatuhTempo           string          `json:"tanggal_jatuh_tempo"`
	SukuBunga                   decimal.Decimal `json:"suku_bunga"`
	Nominal                     decimal.Decimal `json:"nominal"`
	NilaiAgunanDiperhitungkan   decimal.Decimal `json:"nilai_agunan_diperhitungkan"`
	CKPN                        decimal.Decimal `json:"cadangan_kerugian_penurunan_nilai"`
	CKPNAsetBaik                decimal.Decimal `json:"ckpn_aset_baik"`
	CKPNAsetKurangBaik          decimal.Decimal `json:"ckpn_aset_kurang_baik"`
	CKPNAsetTidakBaik           decimal.Decimal `json:"ckpn_aset_tidak_baik"`
	KlasifikasiAsetKeuanganCode string          `json:"klasifikasi_aset_keuangan_code"`
	JenisCKPNCode               string          `json:"jenis_ckpn_code"`
	AsOf                        string          `json:"as_of"`
	Status                      string          `json:"status"`
	Note                        string          `json:"note"`
}

// BuildAsetKeuanganItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID
// kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak
// disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildAsetKeuanganItem(in UpdateAsetKeuanganItemInput) (AsetKeuanganItem, error) {
	out := AsetKeuanganItem{
		NoRekening:                  strings.TrimSpace(in.NoRekening),
		CounterpartyID:              strings.TrimSpace(in.CounterpartyID),
		JenisCode:                   strings.TrimSpace(in.JenisCode),
		SukuBunga:                   in.SukuBunga,
		Nominal:                     in.Nominal,
		NilaiAgunanDiperhitungkan:   in.NilaiAgunanDiperhitungkan,
		CKPN:                        in.CKPN,
		CKPNAsetBaik:                in.CKPNAsetBaik,
		CKPNAsetKurangBaik:          in.CKPNAsetKurangBaik,
		CKPNAsetTidakBaik:           in.CKPNAsetTidakBaik,
		KlasifikasiAsetKeuanganCode: strings.TrimSpace(in.KlasifikasiAsetKeuanganCode),
		JenisCKPNCode:               strings.TrimSpace(in.JenisCKPNCode),
		Status:                      strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                        strings.TrimSpace(in.Note),
	}
	if out.NoRekening == "" {
		return out, fmt.Errorf("%w: No. Rekening wajib diisi", ErrAsetKeuanganInputInvalid)
	}
	if len(out.NoRekening) > 64 {
		return out, fmt.Errorf("%w: No. Rekening maksimal 64 karakter", ErrAsetKeuanganInputInvalid)
	}
	if out.CounterpartyID == "" {
		return out, fmt.Errorf("%w: ID Pihak Lawan wajib diisi", ErrAsetKeuanganInputInvalid)
	}
	if len(out.CounterpartyID) > 64 {
		return out, fmt.Errorf("%w: ID Pihak Lawan maksimal 64 karakter", ErrAsetKeuanganInputInvalid)
	}
	switch out.JenisCode {
	case AsetKeuanganJenisTagihanFraud, AsetKeuanganJenisTagihanLain:
	default:
		return out, fmt.Errorf("%w: jenis hanya 10 atau 99", ErrAsetKeuanganInputInvalid)
	}
	switch out.KlasifikasiAsetKeuanganCode {
	case AsetKeuanganKlasifikasiNilaiWajarLabaRugi, AsetKeuanganKlasifikasiNilaiWajarPKL,
		AsetKeuanganKlasifikasiBiayaPerolehanDiamortasi:
	default:
		return out, fmt.Errorf("%w: klasifikasi aset keuangan hanya 1, 2, atau 3", ErrAsetKeuanganInputInvalid)
	}
	switch out.JenisCKPNCode {
	case AsetKeuanganJenisCKPNIndividual, AsetKeuanganJenisCKPNKolektif:
	default:
		return out, fmt.Errorf("%w: jenis CKPN hanya 1 atau 2", ErrAsetKeuanganInputInvalid)
	}
	if out.SukuBunga.IsNegative() {
		return out, fmt.Errorf("%w: suku bunga tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.Nominal.IsNegative() {
		return out, fmt.Errorf("%w: nominal tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.NilaiAgunanDiperhitungkan.IsNegative() {
		return out, fmt.Errorf("%w: nilai agunan yang dapat diperhitungkan tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.CKPN.IsNegative() {
		return out, fmt.Errorf("%w: CKPN tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.CKPNAsetBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset baik tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.CKPNAsetKurangBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset kurang baik tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.CKPNAsetTidakBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset tidak baik tidak boleh negatif", ErrAsetKeuanganInputInvalid)
	}
	if out.Status == "" {
		out.Status = AsetKeuanganStatusAktif
	}
	if out.Status != AsetKeuanganStatusAktif && out.Status != AsetKeuanganStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrAsetKeuanganInputInvalid)
	}
	var err error
	if out.ID, err = parseAsetKeuanganID(in.ID); err != nil {
		return out, err
	}
	if out.TanggalMulai, err = parseAsetKeuanganDate(in.TanggalMulai, "tanggal_mulai"); err != nil {
		return out, err
	}
	if out.TanggalJatuhTempo, err = parseAsetKeuanganDate(in.TanggalJatuhTempo, "tanggal_jatuh_tempo"); err != nil {
		return out, err
	}
	if out.AsOf, err = parseAsetKeuanganDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseAsetKeuanganID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseAsetKeuanganID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrAsetKeuanganInputInvalid)
	}
	return id, nil
}

// parseAsetKeuanganDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama
// kolomnya.
func parseAsetKeuanganDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrAsetKeuanganInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrAsetKeuanganInputInvalid, field)
	}
	return t, nil
}

// AsetKeuanganRegisterRepository menyimpan dan membaca register aset keuangan lainnya.
// Seluruh pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit
// berada pada transaksi yang sama.
type AsetKeuanganRegisterRepository interface {
	ListItems(ctx context.Context) ([]AsetKeuanganItem, error)
	// ListAsetKeuanganForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 18.00.
	ListAsetKeuanganForOJK(ctx context.Context, asOf time.Time) ([]AsetKeuanganItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu baris.
	// No. Rekening yang sudah dipakai baris lain (termasuk NONAKTIF) ditolak
	// ErrAsetKeuanganNoRekeningUsed.
	UpsertItemTx(ctx context.Context, tx any, item AsetKeuanganItem, actorID uuid.UUID) error
	// SoftDeleteItemTx menonaktifkan satu baris (status -> NONAKTIF); baris tetap ada
	// sehingga nomor rekeningnya tidak dapat dipakai ulang. found=false bila baris tidak ada.
	SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error)
}

// AsetKeuanganReport adalah keluaran baca-saja register aset keuangan lainnya untuk satu
// posisi.
type AsetKeuanganReport struct {
	AsOf  time.Time          `json:"as_of"`
	Items []AsetKeuanganItem `json:"items"`
}

// AsetKeuanganRegisterService merakit laporan register aset keuangan lainnya dan melayani
// pengisian berizin.
type AsetKeuanganRegisterService interface {
	// AsetKeuanganReport menyusun laporan untuk satu posisi. Bank-wide.
	AsetKeuanganReport(ctx context.Context, asOf time.Time, actor Actor) (AsetKeuanganReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]AsetKeuanganItem, error)
	// UpsertItem menyimpan satu baris (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdateAsetKeuanganItemInput, actor Actor) (*AsetKeuanganItem, error)
	// DeleteItem menonaktifkan satu baris (soft-delete), teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
