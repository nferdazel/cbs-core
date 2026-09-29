package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// surat_berharga.go memuat fondasi data Form 04.00 "DAFTAR SURAT BERHARGA", Lampiran II
// SEOJK No. 16/SEOJK.03/2024: register surat berharga per surat berharga yang bank miliki.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 04.00 "DAFTAR SURAT BERHARGA": PDF #page 129-131 (hlm. tercetak 77-79). Dua
//     puluh lima kolom: I Sandi Kantor, II Klasifikasi, III Suku Bunga,
//     IV Jangka Waktu (Tanggal Mulai + Tanggal Jatuh Tempo), V Nominal,
//     VI Nominal yang Dijaminkan, VII Biaya Perolehan,
//     VIII Diskonto/Premium Belum Diamortisasi, IX Biaya Transaksi Belum Diamortisasi,
//     X Laba/Rugi Belum Direalisasi, XI Biaya Perolehan Diamortisasi/Nilai Wajar,
//     XII Nomor Surat Berharga, XIII ID Pihak Lawan, XIV Jenis, XV Kualitas,
//     XVI Cadangan Kerugian Penurunan Nilai, XVII Lembaga Pemeringkat,
//     XVIII Peringkat Surat Berharga, XIX Tanggal Pemeringkatan, XX Tanggal Penerbitan,
//     XXI CKPN Aset Baik, XXII CKPN Aset Kurang Baik, XXIII CKPN Aset Tidak Baik,
//     XXIV Klasifikasi Aset Keuangan, XXV Jenis CKPN. Satu baris = satu surat berharga;
//     ADA baris JUMLAH (PDF #page 129).
//   - Sandi: PDF #page 132-133 (hlm. 80-81). II 1 Tersedia untuk dijual / 2 Dimiliki
//     hingga jatuh tempo; XIV 1 Bank Indonesia / 2 Pemerintah / 3 Pemerintah Daerah;
//     XV 1 Lancar / 3 Kurang Lancar / 5 Macet; XXIV 1-3; XXV 1 Individual / 2 Kolektif.
//   - Penjelasan: PDF #page 134-136 (hlm. 82-84). XI: nilai nominal - diskonto belum
//     diamortisasi + premium belum diamortisasi + biaya transaksi belum diamortisasi,
//     ATAU nilai wajar. XXIV hanya diisi bila BPR melakukan penawaran umum efek di pasar
//     modal. XVII sandi 9 dan XVIII sandi 99 untuk keadaan tanpa peringkat.
//
// Prinsip yang dipegang berkas ini:
//   - Kolom XI adalah isian bank: form memberi DUA kemungkinan (amortized cost atau nilai
//     wajar) tanpa aturan pengikat tunggal, sehingga laporan tidak menghitungnya.
//   - Sandi II/XIV/XV/XXIV/XXV adalah isian bank dan ditegakkan di sini; tidak diturunkan
//     dari kolom lain dan tidak menerapkan aturan historis "Desember 2024 = Kolektif".
//   - XVII/XVIII adalah sandi Lampiran 08/09 dengan sentinel resmi 9 dan 99; daftar
//     lengkap lampiran tidak disalin ke kode (bukan bagian form).
//   - Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
//   - XII "Nomor Surat Berharga" adalah ISIN dalam teks bebas; tidak ada tabel sandi.
//   - Form 04.00 tidak menetapkan nomor register unik, sehingga register ini tidak
//     memiliki aturan no reuse/no recycle dan penghapusan boleh DELETE fisik.

// Sandi klasifikasi surat berharga baku Form 04.00 (PDF #page 132).
const (
	SuratBerhargaKlasifikasiTersediaUntukDijual      = "1"
	SuratBerhargaKlasifikasiDimilikiHinggaJatuhTempo = "2"
)

// Sandi jenis penerbit surat berharga baku Form 04.00 (PDF #page 132).
const (
	SuratBerhargaJenisBankIndonesia    = "1"
	SuratBerhargaJenisPemerintah       = "2"
	SuratBerhargaJenisPemerintahDaerah = "3"
)

// Sandi kualitas surat berharga baku Form 04.00 (PDF #page 132): hanya tiga ini tercetak.
const (
	SuratBerhargaKualitasLancar       = "1"
	SuratBerhargaKualitasKurangLancar = "3"
	SuratBerhargaKualitasMacet        = "5"
)

// Sandi klasifikasi aset keuangan baku Form 04.00 (PDF #page 133).
const (
	SuratBerhargaKlasifikasiAsetNilaiWajarLabaRugi = "1"
	SuratBerhargaKlasifikasiAsetNilaiWajarPKL      = "2"
	SuratBerhargaKlasifikasiAsetBiayaPerolehan     = "3"
)

// Sandi jenis CKPN baku Form 04.00 (PDF #page 133).
const (
	SuratBerhargaJenisCKPNIndividual = "1"
	SuratBerhargaJenisCKPNKolektif   = "2"
)

// Sentinel sandi peringkat Form 04.00 (PDF #page 135).
const (
	// SuratBerhargaLembagaPemeringkatTanpaPeringkat adalah sandi XVII bila pihak lawan
	// tanpa peringkat atau peringkat dari lembaga yang tidak diakui OJK.
	SuratBerhargaLembagaPemeringkatTanpaPeringkat = "9"
	// SuratBerhargaPeringkatTanpaPeringkat adalah sandi XVIII pada keadaan yang sama.
	SuratBerhargaPeringkatTanpaPeringkat = "99"
)

// Status baku register surat berharga.
const (
	SuratBerhargaStatusAktif    = "AKTIF"
	SuratBerhargaStatusNonaktif = "NONAKTIF"
)

var (
	ErrSuratBerhargaBankWide = NewLocalizedError("surat_berharga_bank_wide",
		"register surat berharga bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	ErrSuratBerhargaInputInvalid = NewLocalizedError("surat_berharga_input_invalid",
		"data register surat berharga tidak valid")
	ErrSuratBerhargaNotFound = NewLocalizedError("surat_berharga_not_found",
		"data register surat berharga tidak ditemukan")
)

// SuratBerhargaItem adalah satu surat berharga (satu baris Form 04.00) yang bank catat.
// Tidak ada kolom turunan di sini: seluruh nilai adalah isian bank.
type SuratBerhargaItem struct {
	ID uuid.UUID `json:"id"`
	// KlasifikasiCode kolom II: 1 Tersedia untuk dijual, 2 Dimiliki hingga jatuh tempo.
	KlasifikasiCode string `json:"klasifikasi_code"`
	// SukuBunga kolom III, persen; isian bank.
	SukuBunga decimal.Decimal `json:"suku_bunga"`
	// TanggalMulai kolom IV (Jangka Waktu).
	TanggalMulai time.Time `json:"tanggal_mulai"`
	// TanggalJatuhTempo kolom IV (Jangka Waktu).
	TanggalJatuhTempo time.Time `json:"tanggal_jatuh_tempo"`
	// Nominal kolom V, rupiah penuh; isian bank.
	Nominal decimal.Decimal `json:"nominal"`
	// NominalDijaminkan kolom VI, rupiah penuh; isian bank.
	NominalDijaminkan decimal.Decimal `json:"nominal_dijaminkan"`
	// BiayaPerolehan kolom VII, rupiah penuh; isian bank.
	BiayaPerolehan decimal.Decimal `json:"biaya_perolehan"`
	// DiskontoPremiumBelumDiamortisasi kolom VIII, rupiah penuh; isian bank (boleh negatif).
	DiskontoPremiumBelumDiamortisasi decimal.Decimal `json:"diskonto_premium_belum_diamortisasi"`
	// BiayaTransaksiBelumDiamortisasi kolom IX, rupiah penuh; isian bank.
	BiayaTransaksiBelumDiamortisasi decimal.Decimal `json:"biaya_transaksi_belum_diamortisasi"`
	// LabaRugiBelumDirealisasi kolom X, rupiah penuh; isian bank (boleh negatif).
	LabaRugiBelumDirealisasi decimal.Decimal `json:"laba_rugi_belum_direalisasi"`
	// BiayaPerolehanDiamortisasi kolom XI, rupiah penuh; isian bank (form memberi dua
	// kemungkinan: amortized cost atau nilai wajar).
	BiayaPerolehanDiamortisasi decimal.Decimal `json:"biaya_perolehan_diamortisasi"`
	// NomorSuratBerharga kolom XII: ISIN, teks apa adanya.
	NomorSuratBerharga string `json:"nomor_surat_berharga"`
	// CounterpartyID kolom XIII: pengenal pihak lawan, teks apa adanya.
	CounterpartyID string `json:"counterparty_id"`
	// JenisCode kolom XIV: 1 BI, 2 Pemerintah, 3 Pemerintah Daerah.
	JenisCode string `json:"jenis_code"`
	// KualitasCode kolom XV: 1 Lancar, 3 Kurang Lancar, 5 Macet.
	KualitasCode string `json:"kualitas_code"`
	// CKPN kolom XVI, rupiah penuh; isian bank.
	CKPN decimal.Decimal `json:"cadangan_kerugian_penurunan_nilai"`
	// LembagaPemeringkatCode kolom XVII: sandi Lampiran 08; 9 = tanpa peringkat.
	LembagaPemeringkatCode string `json:"lembaga_pemeringkat_code"`
	// PeringkatSuratBerhargaCode kolom XVIII: sandi Lampiran 09; 99 = tanpa peringkat.
	PeringkatSuratBerhargaCode string `json:"peringkat_surat_berharga_code"`
	// TanggalPemeringkatan kolom XIX; nol bila tanpa peringkat.
	TanggalPemeringkatan *time.Time `json:"tanggal_pemeringkatan"`
	// TanggalPenerbitan kolom XX.
	TanggalPenerbitan time.Time `json:"tanggal_penerbitan"`
	// CKPNAsetBaik kolom XXI, rupiah penuh; isian bank.
	CKPNAsetBaik decimal.Decimal `json:"ckpn_aset_baik"`
	// CKPNAsetKurangBaik kolom XXII, rupiah penuh; isian bank.
	CKPNAsetKurangBaik decimal.Decimal `json:"ckpn_aset_kurang_baik"`
	// CKPNAsetTidakBaik kolom XXIII, rupiah penuh; isian bank.
	CKPNAsetTidakBaik decimal.Decimal `json:"ckpn_aset_tidak_baik"`
	// KlasifikasiAsetKeuanganCode kolom XXIV; boleh kosong (hanya bila penawaran umum).
	KlasifikasiAsetKeuanganCode string `json:"klasifikasi_aset_keuangan_code"`
	// JenisCKPNCode kolom XXV: 1 Individual, 2 Kolektif.
	JenisCKPNCode string    `json:"jenis_ckpn_code"`
	AsOf          time.Time `json:"as_of"`
	Status        string    `json:"status"`
	Note          string    `json:"note,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// UpdateSuratBerhargaItemInput adalah isi upsert satu surat berharga. ID kosong = buat
// baru; ID terisi = perbarui baris itu. Tanggal memakai format YYYY-MM-DD.
type UpdateSuratBerhargaItemInput struct {
	ID                               string          `json:"id"`
	KlasifikasiCode                  string          `json:"klasifikasi_code"`
	SukuBunga                        decimal.Decimal `json:"suku_bunga"`
	TanggalMulai                     string          `json:"tanggal_mulai"`
	TanggalJatuhTempo                string          `json:"tanggal_jatuh_tempo"`
	Nominal                          decimal.Decimal `json:"nominal"`
	NominalDijaminkan                decimal.Decimal `json:"nominal_dijaminkan"`
	BiayaPerolehan                   decimal.Decimal `json:"biaya_perolehan"`
	DiskontoPremiumBelumDiamortisasi decimal.Decimal `json:"diskonto_premium_belum_diamortisasi"`
	BiayaTransaksiBelumDiamortisasi  decimal.Decimal `json:"biaya_transaksi_belum_diamortisasi"`
	LabaRugiBelumDirealisasi         decimal.Decimal `json:"laba_rugi_belum_direalisasi"`
	BiayaPerolehanDiamortisasi       decimal.Decimal `json:"biaya_perolehan_diamortisasi"`
	NomorSuratBerharga               string          `json:"nomor_surat_berharga"`
	CounterpartyID                   string          `json:"counterparty_id"`
	JenisCode                        string          `json:"jenis_code"`
	KualitasCode                     string          `json:"kualitas_code"`
	CKPN                             decimal.Decimal `json:"cadangan_kerugian_penurunan_nilai"`
	LembagaPemeringkatCode           string          `json:"lembaga_pemeringkat_code"`
	PeringkatSuratBerhargaCode       string          `json:"peringkat_surat_berharga_code"`
	TanggalPemeringkatan             string          `json:"tanggal_pemeringkatan"`
	TanggalPenerbitan                string          `json:"tanggal_penerbitan"`
	CKPNAsetBaik                     decimal.Decimal `json:"ckpn_aset_baik"`
	CKPNAsetKurangBaik               decimal.Decimal `json:"ckpn_aset_kurang_baik"`
	CKPNAsetTidakBaik                decimal.Decimal `json:"ckpn_aset_tidak_baik"`
	KlasifikasiAsetKeuanganCode      string          `json:"klasifikasi_aset_keuangan_code"`
	JenisCKPNCode                    string          `json:"jenis_ckpn_code"`
	AsOf                             string          `json:"as_of"`
	Status                           string          `json:"status"`
	Note                             string          `json:"note"`
}

// BuildSuratBerhargaItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID
// kosong dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD (kecuali
// tanggal_pemeringkatan yang boleh kosong).
func BuildSuratBerhargaItem(in UpdateSuratBerhargaItemInput) (SuratBerhargaItem, error) {
	out := SuratBerhargaItem{
		KlasifikasiCode:                  strings.TrimSpace(in.KlasifikasiCode),
		SukuBunga:                        in.SukuBunga,
		Nominal:                          in.Nominal,
		NominalDijaminkan:                in.NominalDijaminkan,
		BiayaPerolehan:                   in.BiayaPerolehan,
		DiskontoPremiumBelumDiamortisasi: in.DiskontoPremiumBelumDiamortisasi,
		BiayaTransaksiBelumDiamortisasi:  in.BiayaTransaksiBelumDiamortisasi,
		LabaRugiBelumDirealisasi:         in.LabaRugiBelumDirealisasi,
		BiayaPerolehanDiamortisasi:       in.BiayaPerolehanDiamortisasi,
		NomorSuratBerharga:               strings.TrimSpace(in.NomorSuratBerharga),
		CounterpartyID:                   strings.TrimSpace(in.CounterpartyID),
		JenisCode:                        strings.TrimSpace(in.JenisCode),
		KualitasCode:                     strings.TrimSpace(in.KualitasCode),
		CKPN:                             in.CKPN,
		LembagaPemeringkatCode:           strings.TrimSpace(in.LembagaPemeringkatCode),
		PeringkatSuratBerhargaCode:       strings.TrimSpace(in.PeringkatSuratBerhargaCode),
		CKPNAsetBaik:                     in.CKPNAsetBaik,
		CKPNAsetKurangBaik:               in.CKPNAsetKurangBaik,
		CKPNAsetTidakBaik:                in.CKPNAsetTidakBaik,
		KlasifikasiAsetKeuanganCode:      strings.TrimSpace(in.KlasifikasiAsetKeuanganCode),
		JenisCKPNCode:                    strings.TrimSpace(in.JenisCKPNCode),
		Status:                           strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:                             strings.TrimSpace(in.Note),
	}
	switch out.KlasifikasiCode {
	case SuratBerhargaKlasifikasiTersediaUntukDijual, SuratBerhargaKlasifikasiDimilikiHinggaJatuhTempo:
	default:
		return out, fmt.Errorf("%w: klasifikasi hanya 1 atau 2", ErrSuratBerhargaInputInvalid)
	}
	switch out.JenisCode {
	case SuratBerhargaJenisBankIndonesia, SuratBerhargaJenisPemerintah, SuratBerhargaJenisPemerintahDaerah:
	default:
		return out, fmt.Errorf("%w: jenis hanya 1, 2, atau 3", ErrSuratBerhargaInputInvalid)
	}
	switch out.KualitasCode {
	case SuratBerhargaKualitasLancar, SuratBerhargaKualitasKurangLancar, SuratBerhargaKualitasMacet:
	default:
		return out, fmt.Errorf("%w: kualitas hanya 1, 3, atau 5", ErrSuratBerhargaInputInvalid)
	}
	switch out.JenisCKPNCode {
	case SuratBerhargaJenisCKPNIndividual, SuratBerhargaJenisCKPNKolektif:
	default:
		return out, fmt.Errorf("%w: jenis CKPN hanya 1 atau 2", ErrSuratBerhargaInputInvalid)
	}
	// XXIV boleh kosong (hanya diisi bila penawaran umum efek); bila terisi harus sah.
	switch out.KlasifikasiAsetKeuanganCode {
	case "", SuratBerhargaKlasifikasiAsetNilaiWajarLabaRugi,
		SuratBerhargaKlasifikasiAsetNilaiWajarPKL, SuratBerhargaKlasifikasiAsetBiayaPerolehan:
	default:
		return out, fmt.Errorf("%w: klasifikasi aset keuangan hanya 1, 2, atau 3", ErrSuratBerhargaInputInvalid)
	}
	// Sandi Lampiran 08/09: panjang dibatasi; sentinel resmi diizinkan. Daftar lengkap
	// lampiran tidak disalin ke kode.
	if len(out.LembagaPemeringkatCode) > 9 {
		return out, fmt.Errorf("%w: sandi lembaga pemeringkat maksimal 9 karakter", ErrSuratBerhargaInputInvalid)
	}
	if len(out.PeringkatSuratBerhargaCode) > 9 {
		return out, fmt.Errorf("%w: sandi peringkat maksimal 9 karakter", ErrSuratBerhargaInputInvalid)
	}
	if len(out.NomorSuratBerharga) > 64 {
		return out, fmt.Errorf("%w: nomor surat berharga maksimal 64 karakter", ErrSuratBerhargaInputInvalid)
	}
	if len(out.CounterpartyID) > 64 {
		return out, fmt.Errorf("%w: ID pihak lawan maksimal 64 karakter", ErrSuratBerhargaInputInvalid)
	}
	if out.SukuBunga.IsNegative() {
		return out, fmt.Errorf("%w: suku bunga tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.Nominal.IsNegative() {
		return out, fmt.Errorf("%w: nominal tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.NominalDijaminkan.IsNegative() {
		return out, fmt.Errorf("%w: nominal yang dijaminkan tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.BiayaPerolehan.IsNegative() {
		return out, fmt.Errorf("%w: biaya perolehan tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.BiayaTransaksiBelumDiamortisasi.IsNegative() {
		return out, fmt.Errorf("%w: biaya transaksi belum diamortisasi tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.BiayaPerolehanDiamortisasi.IsNegative() {
		return out, fmt.Errorf("%w: biaya perolehan diamortisasi/nilai wajar tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.CKPN.IsNegative() {
		return out, fmt.Errorf("%w: CKPN tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.CKPNAsetBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset baik tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.CKPNAsetKurangBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset kurang baik tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.CKPNAsetTidakBaik.IsNegative() {
		return out, fmt.Errorf("%w: CKPN aset tidak baik tidak boleh negatif", ErrSuratBerhargaInputInvalid)
	}
	if out.Status == "" {
		out.Status = SuratBerhargaStatusAktif
	}
	if out.Status != SuratBerhargaStatusAktif && out.Status != SuratBerhargaStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrSuratBerhargaInputInvalid)
	}
	var err error
	if out.ID, err = parseSuratBerhargaID(in.ID); err != nil {
		return out, err
	}
	if out.TanggalMulai, err = parseSuratBerhargaDate(in.TanggalMulai, "tanggal_mulai"); err != nil {
		return out, err
	}
	if out.TanggalJatuhTempo, err = parseSuratBerhargaDate(in.TanggalJatuhTempo, "tanggal_jatuh_tempo"); err != nil {
		return out, err
	}
	if out.TanggalPenerbitan, err = parseSuratBerhargaDate(in.TanggalPenerbitan, "tanggal_penerbitan"); err != nil {
		return out, err
	}
	if out.TanggalPemeringkatan, err = parseSuratBerhargaOptionalDate(in.TanggalPemeringkatan, "tanggal_pemeringkatan"); err != nil {
		return out, err
	}
	if out.AsOf, err = parseSuratBerhargaDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseSuratBerhargaID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseSuratBerhargaID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrSuratBerhargaInputInvalid)
	}
	return id, nil
}

// parseSuratBerhargaDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama
// kolomnya.
func parseSuratBerhargaDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrSuratBerhargaInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrSuratBerhargaInputInvalid, field)
	}
	return t, nil
}

// parseSuratBerhargaOptionalDate mengurai tanggal yang boleh kosong (tanggal pemeringkatan
// tidak wajib bila surat berharga tanpa peringkat).
func parseSuratBerhargaOptionalDate(raw, field string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrSuratBerhargaInputInvalid, field)
	}
	return &t, nil
}

// SuratBerhargaRegisterRepository menyimpan dan membaca register surat berharga. Seluruh
// pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada
// transaksi yang sama.
type SuratBerhargaRegisterRepository interface {
	ListItems(ctx context.Context) ([]SuratBerhargaItem, error)
	// ListSuratBerhargaForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 04.00.
	ListSuratBerhargaForOJK(ctx context.Context, asOf time.Time) ([]SuratBerhargaItem, error)
	UpsertItemTx(ctx context.Context, tx any, item SuratBerhargaItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu baris. Form 04.00 tidak menetapkan nomor register unik,
	// sehingga penghapusan adalah DELETE fisik. found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// SuratBerhargaReport adalah keluaran baca-saja register surat berharga untuk satu posisi.
type SuratBerhargaReport struct {
	AsOf  time.Time           `json:"as_of"`
	Items []SuratBerhargaItem `json:"items"`
}

// SuratBerhargaRegisterService merakit laporan register surat berharga dan melayani
// pengisian berizin.
type SuratBerhargaRegisterService interface {
	SuratBerhargaReport(ctx context.Context, asOf time.Time, actor Actor) (SuratBerhargaReport, error)
	ListItems(ctx context.Context) ([]SuratBerhargaItem, error)
	UpsertItem(ctx context.Context, input UpdateSuratBerhargaItemInput, actor Actor) (*SuratBerhargaItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
