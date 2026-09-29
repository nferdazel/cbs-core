package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// penyertaan.go memuat fondasi data Form 16.00 "DAFTAR PENYERTAAN MODAL",
// Lampiran II SEOJK No. 16/SEOJK.03/2024: register penyertaan modal per pihak lawan
// yang bank catat.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 16.00 "DAFTAR PENYERTAAN MODAL": PDF #page 223 (hlm. tercetak 171) dan
//     lanjutan PDF #page 224 (hlm. 172). Lima belas kolom: I Sandi Kantor,
//     II No. Register, III ID Pihak Lawan, IV Metode Penyertaan, V Kualitas,
//     VI Tujuan Penyertaan, VII Tanggal Mulai, VIII Persentase Penyertaan, IX Nominal,
//     X Jumlah Bulan Laporan, XI Cadangan Kerugian Penurunan Nilai,
//     XII Cadangan Kerugian Penurunan Nilai Aset Baik,
//     XIII Cadangan Kerugian Penurunan Nilai Aset Kurang Baik,
//     XIV Cadangan Kerugian Penurunan Nilai Aset Tidak Baik, XV Jenis CKPN.
//     Satu baris = satu penyertaan modal; TIDAK ADA baris JUMLAH.
//   - Sandi: PDF #page 225-226 (hlm. 173-174). IV 1 Biaya Perolehan / 2 Metode Ekuitas;
//     V 1 Lancar / 3 Kurang Lancar / 4 Diragukan / 5 Macet;
//     VI 1 Investasi pada Lembaga Penunjang / 9 Lainnya; XV 1 Individual / 2 Kolektif.
//   - Penjelasan: PDF #page 227-228 (hlm. 175-176). No. Register unik, satu register
//     untuk satu penyertaan, "no reuse atau no recycle". Kolom X didefinisikan sebagai
//     "nilai tercatat penyertaan modal pada bulan laporan" (PDF #page 228): sebuah NILAI
//     yang diisi bank, bukan jumlah bulan yang dihitung sistem.
//
// Prinsip yang dipegang berkas ini:
//   - Tidak ada kolom turunan: form tidak memberikan rumus yang mengikat untuk kolom X
//     maupun blok CKPN, sehingga seluruh nilai adalah isian bank dan laporan menuliskan
//     apa adanya.
//   - Sandi IV/V/VI/XV adalah isian bank dan ditegakkan di sini; tidak diturunkan dari
//     kolom lain dan tidak menerapkan aturan historis "Desember 2024 = Kolektif".
//   - Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti Form 17.00.
//   - Nomor Register (II) unik dan TIDAK BOLEH dipakai ulang (PDF #page 227). Karena itu
//     penghapusan register adalah soft-delete (status -> NONAKTIF); baris tetap ada
//     sehingga nomornya tidak pernah kembali bebas. Pelanggaran keunikan dipetakan ke
//     ErrPenyertaanNoRegisterUsed.
//   - Kolom III "ID Pihak Lawan" bukan sandi Lampiran 02 (form 16.00 tidak memuat sandi
//     untuk kolom ini): disimpan sebagai teks apa adanya, tanpa daftar sandi karangan.

// Sandi metode penyertaan baku Form 16.00 (PDF #page 225).
const (
	PenyertaanMetodeBiayaPerolehan = "1"
	PenyertaanMetodeEkuitas        = "2"
)

// Sandi kualitas baku Form 16.00 (PDF #page 225).
const (
	PenyertaanKualitasLancar       = "1"
	PenyertaanKualitasKurangLancar = "3"
	PenyertaanKualitasDiragukan    = "4"
	PenyertaanKualitasMacet        = "5"
)

// Sandi tujuan penyertaan baku Form 16.00 (PDF #page 225).
const (
	PenyertaanTujuanLembagaPenunjang = "1"
	PenyertaanTujuanLainnya          = "9"
)

// Sandi jenis CKPN baku Form 16.00 (PDF #page 225).
const (
	PenyertaanJenisCKPNIndividual = "1"
	PenyertaanJenisCKPNKolektif   = "2"
)

// Status baku register penyertaan modal.
const (
	PenyertaanStatusAktif    = "AKTIF"
	PenyertaanStatusNonaktif = "NONAKTIF"
)

var (
	// ErrPenyertaanBankWide menandai permintaan register penyertaan modal oleh aktor
	// yang tidak berwenang atas seluruh bank. Register ini bank-wide.
	ErrPenyertaanBankWide = NewLocalizedError("penyertaan_bank_wide",
		"register penyertaan modal bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrPenyertaanInputInvalid menandai data register penyertaan modal tidak sah.
	ErrPenyertaanInputInvalid = NewLocalizedError("penyertaan_input_invalid",
		"data register penyertaan modal tidak valid")
	// ErrPenyertaanNotFound menandai baris yang hendak dinonaktifkan tidak ada.
	ErrPenyertaanNotFound = NewLocalizedError("penyertaan_not_found",
		"data register penyertaan modal tidak ditemukan")
	// ErrPenyertaanNoRegisterUsed menandai No. Register yang sudah pernah dipakai,
	// termasuk oleh baris NONAKTIF. Aturan no reuse/no recycle Form 16.00 (PDF #page 227).
	ErrPenyertaanNoRegisterUsed = NewLocalizedError("penyertaan_no_register_used",
		"nomor register penyertaan modal sudah pernah dipakai dan tidak boleh dipakai ulang")
)

// PenyertaanItem adalah satu penyertaan modal (satu baris Form 16.00) yang bank catat.
// Tidak ada kolom turunan di sini: seluruh nilai adalah isian bank.
type PenyertaanItem struct {
	ID uuid.UUID `json:"id"`
	// NoRegister kolom II: kode unik penyertaan, mandatory, no reuse/no recycle.
	NoRegister string `json:"no_register"`
	// CounterpartyID kolom III: pengenal pihak lawan yang bank catat.
	CounterpartyID string `json:"counterparty_id"`
	// MetodePenyertaanCode kolom IV: 1 Biaya Perolehan, 2 Metode Ekuitas.
	MetodePenyertaanCode string `json:"metode_penyertaan_code"`
	// KualitasCode kolom V: 1 Lancar, 3 Kurang Lancar, 4 Diragukan, 5 Macet.
	KualitasCode string `json:"kualitas_code"`
	// TujuanPenyertaanCode kolom VI: 1 Lembaga Penunjang, 9 Lainnya.
	TujuanPenyertaanCode string `json:"tujuan_penyertaan_code"`
	// TanggalMulai kolom VII.
	TanggalMulai time.Time `json:"tanggal_mulai"`
	// PersentasePenyertaan kolom VIII, persen 0..100; isian bank.
	PersentasePenyertaan decimal.Decimal `json:"persentase_penyertaan"`
	// Nominal kolom IX, rupiah penuh; isian bank.
	Nominal decimal.Decimal `json:"nominal"`
	// JumlahBulanLaporan kolom X. Nama mengikuti kolom resmi, tetapi definisinya (PDF
	// #page 228) adalah nilai tercatat penyertaan pada bulan laporan: nilai rupiah yang
	// diisi bank, bukan jumlah bulan dan bukan turunan.
	JumlahBulanLaporan decimal.Decimal `json:"jumlah_bulan_laporan"`
	// CKPN kolom XI, rupiah penuh; isian bank.
	CKPN decimal.Decimal `json:"cadangan_kerugian_penurunan_nilai"`
	// CKPNAsetBaik kolom XII, rupiah penuh; isian bank.
	CKPNAsetBaik decimal.Decimal `json:"ckpn_aset_baik"`
	// CKPNAsetKurangBaik kolom XIII, rupiah penuh; isian bank.
	CKPNAsetKurangBaik decimal.Decimal `json:"ckpn_aset_kurang_baik"`
	// CKPNAsetTidakBaik kolom XIV, rupiah penuh; isian bank.
	CKPNAsetTidakBaik decimal.Decimal `json:"ckpn_aset_tidak_baik"`
	// JenisCKPNCode kolom XV: 1 Individual, 2 Kolektif.
	JenisCKPNCode string    `json:"jenis_ckpn_code"`
	AsOf          time.Time `json:"as_of"`
	Status        string    `json:"status"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpdatePenyertaanItemInput adalah isi upsert satu penyertaan. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu. Angka menerima JSON number
// maupun string desimal. Tanggal memakai format YYYY-MM-DD.
type UpdatePenyertaanItemInput struct {
	ID                   string          `json:"id"`
	NoRegister           string          `json:"no_register"`
	CounterpartyID       string          `json:"counterparty_id"`
	MetodePenyertaanCode string          `json:"metode_penyertaan_code"`
	KualitasCode         string          `json:"kualitas_code"`
	TujuanPenyertaanCode string          `json:"tujuan_penyertaan_code"`
	TanggalMulai         string          `json:"tanggal_mulai"`
	PersentasePenyertaan decimal.Decimal `json:"persentase_penyertaan"`
	Nominal              decimal.Decimal `json:"nominal"`
	JumlahBulanLaporan   decimal.Decimal `json:"jumlah_bulan_laporan"`
	CKPN                 decimal.Decimal `json:"cadangan_kerugian_penurunan_nilai"`
	CKPNAsetBaik         decimal.Decimal `json:"ckpn_aset_baik"`
	CKPNAsetKurangBaik   decimal.Decimal `json:"ckpn_aset_kurang_baik"`
	CKPNAsetTidakBaik    decimal.Decimal `json:"ckpn_aset_tidak_baik"`
	JenisCKPNCode        string          `json:"jenis_ckpn_code"`
	AsOf                 string          `json:"as_of"`
	Status               string          `json:"status"`
	Note                 string          `json:"note"`
}

// BuildPenyertaanItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID
// kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak
// disetel di sini (diisi repositori/layanan saat menyimpan).
func BuildPenyertaanItem(in UpdatePenyertaanItemInput) (PenyertaanItem, error) {
	out := PenyertaanItem{
		NoRegister:           strings.TrimSpace(in.NoRegister),
		CounterpartyID:       strings.TrimSpace(in.CounterpartyID),
		MetodePenyertaanCode: strings.TrimSpace(in.MetodePenyertaanCode),
		KualitasCode:         strings.TrimSpace(in.KualitasCode),
		TujuanPenyertaanCode: strings.TrimSpace(in.TujuanPenyertaanCode),
		PersentasePenyertaan: in.PersentasePenyertaan,
		Nominal:              in.Nominal,
		JumlahBulanLaporan:   in.JumlahBulanLaporan,
		CKPN:                 in.CKPN,
		CKPNAsetBaik:         in.CKPNAsetBaik,
		CKPNAsetKurangBaik:   in.CKPNAsetKurangBaik,
		CKPNAsetTidakBaik:    in.CKPNAsetTidakBaik,
		JenisCKPNCode:        strings.TrimSpace(in.JenisCKPNCode),
		Status:               strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                 strings.TrimSpace(in.Note),
	}
	if out.NoRegister == "" {
		return out, fmt.Errorf("%w: No. Register wajib diisi", ErrPenyertaanInputInvalid)
	}
	if len(out.NoRegister) > 64 {
		return out, fmt.Errorf("%w: No. Register maksimal 64 karakter", ErrPenyertaanInputInvalid)
	}
	if out.CounterpartyID == "" {
		return out, fmt.Errorf("%w: ID Pihak Lawan wajib diisi", ErrPenyertaanInputInvalid)
	}
	if len(out.CounterpartyID) > 64 {
		return out, fmt.Errorf("%w: ID Pihak Lawan maksimal 64 karakter", ErrPenyertaanInputInvalid)
	}
	switch out.MetodePenyertaanCode {
	case PenyertaanMetodeBiayaPerolehan, PenyertaanMetodeEkuitas:
	default:
		return out, fmt.Errorf("%w: metode penyertaan hanya 1 atau 2", ErrPenyertaanInputInvalid)
	}
	switch out.KualitasCode {
	case PenyertaanKualitasLancar, PenyertaanKualitasKurangLancar,
		PenyertaanKualitasDiragukan, PenyertaanKualitasMacet:
	default:
		return out, fmt.Errorf("%w: kualitas hanya 1, 3, 4, atau 5", ErrPenyertaanInputInvalid)
	}
	switch out.TujuanPenyertaanCode {
	case PenyertaanTujuanLembagaPenunjang, PenyertaanTujuanLainnya:
	default:
		return out, fmt.Errorf("%w: tujuan penyertaan hanya 1 atau 9", ErrPenyertaanInputInvalid)
	}
	if out.PersentasePenyertaan.IsNegative() || out.PersentasePenyertaan.GreaterThan(decimal.NewFromInt(100)) {
		return out, fmt.Errorf("%w: persentase penyertaan harus antara 0 dan 100",
			ErrPenyertaanInputInvalid)
	}
	if out.Nominal.IsNegative() {
		return out, fmt.Errorf("%w: nominal tidak boleh negatif", ErrPenyertaanInputInvalid)
	}
	if out.JumlahBulanLaporan.IsNegative() {
		return out, fmt.Errorf("%w: jumlah bulan laporan tidak boleh negatif", ErrPenyertaanInputInvalid)
	}
	switch out.JenisCKPNCode {
	case PenyertaanJenisCKPNIndividual, PenyertaanJenisCKPNKolektif:
	default:
		return out, fmt.Errorf("%w: jenis CKPN hanya 1 atau 2", ErrPenyertaanInputInvalid)
	}
	if out.CKPN.IsNegative() {
		return out, fmt.Errorf("%w: CKPN tidak boleh negatif", ErrPenyertaanInputInvalid)
	}
	if out.CKPNAsetBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset baik tidak boleh negatif", ErrPenyertaanInputInvalid)
	}
	if out.CKPNAsetKurangBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset kurang baik tidak boleh negatif", ErrPenyertaanInputInvalid)
	}
	if out.CKPNAsetTidakBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset tidak baik tidak boleh negatif", ErrPenyertaanInputInvalid)
	}
	if out.Status == "" {
		out.Status = PenyertaanStatusAktif
	}
	if out.Status != PenyertaanStatusAktif && out.Status != PenyertaanStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrPenyertaanInputInvalid)
	}
	var err error
	if out.ID, err = parsePenyertaanID(in.ID); err != nil {
		return out, err
	}
	if out.TanggalMulai, err = parsePenyertaanDate(in.TanggalMulai, "tanggal_mulai"); err != nil {
		return out, err
	}
	if out.AsOf, err = parsePenyertaanDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parsePenyertaanID mengurai id yang dikirim klien; kosong berarti buat baru.
func parsePenyertaanID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrPenyertaanInputInvalid)
	}
	return id, nil
}

// parsePenyertaanDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parsePenyertaanDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrPenyertaanInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrPenyertaanInputInvalid, field)
	}
	return t, nil
}

// PenyertaanRegisterRepository menyimpan dan membaca register penyertaan modal. Seluruh
// pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada
// transaksi yang sama.
type PenyertaanRegisterRepository interface {
	ListItems(ctx context.Context) ([]PenyertaanItem, error)
	// ListPenyertaanForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 16.00.
	ListPenyertaanForOJK(ctx context.Context, asOf time.Time) ([]PenyertaanItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu penyertaan.
	// No. Register yang sudah dipakai baris lain (termasuk NONAKTIF) ditolak
	// ErrPenyertaanNoRegisterUsed.
	UpsertItemTx(ctx context.Context, tx any, item PenyertaanItem, actorID uuid.UUID) error
	// SoftDeleteItemTx menonaktifkan satu penyertaan (status -> NONAKTIF); baris tetap
	// ada sehingga nomor register-nya tidak dapat dipakai ulang. found=false bila baris
	// tidak ada.
	SoftDeleteItemTx(ctx context.Context, tx any, id uuid.UUID, actorID uuid.UUID) (bool, error)
}

// PenyertaanReport adalah keluaran baca-saja register penyertaan modal untuk satu posisi.
type PenyertaanReport struct {
	AsOf  time.Time        `json:"as_of"`
	Items []PenyertaanItem `json:"items"`
}

// PenyertaanRegisterService merakit laporan register penyertaan modal dan melayani
// pengisian berizin.
type PenyertaanRegisterService interface {
	// PenyertaanReport menyusun laporan untuk satu posisi. Bank-wide.
	PenyertaanReport(ctx context.Context, asOf time.Time, actor Actor) (PenyertaanReport, error)
	// ListItems membaca baris mentah register untuk UI edit, urutan deterministik.
	ListItems(ctx context.Context) ([]PenyertaanItem, error)
	// UpsertItem menyimpan satu penyertaan (id kosong = buat baru), teraudit.
	UpsertItem(ctx context.Context, input UpdatePenyertaanItemInput, actor Actor) (*PenyertaanItem, error)
	// DeleteItem menonaktifkan satu penyertaan (soft-delete), teraudit.
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
