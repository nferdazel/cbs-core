package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// kas_valas.go memuat fondasi data Form 03.00 "DAFTAR KAS DALAM VALUTA ASING",
// Lampiran II SEOJK No. 16/SEOJK.03/2024: register kas valuta asing per jenis valas
// yang bank perdagangkan.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 03.00 "DAFTAR KAS DALAM VALUTA ASING": PDF #page 126 (hlm. tercetak 74),
//     sandi PDF #page 127 (hlm. 75), penjelasan PDF #page 128 (hlm. 76); cross-ref
//     posisi keuangan PDF #page 102 (hlm. 50). Lima kolom: I Sandi Kantor,
//     II Jenis Valuta Asing, III Nominal, IV Kurs Tengah (Rp), V Nilai Rupiah.
//     Satu baris = satu jenis valas yang diperdagangkan; ADA baris JUMLAH (PDF #126).
//   - Definisi (PDF #page 128): kas valas = uang kertas/uang logam asing dan cek pelawat
//     yang masih berlaku yang dimiliki BPR sebagai pedagang valuta asing. III adalah
//     nilai valas (original currency) sebelum dirupiahkan. IV adalah kurs tengah Bank
//     Indonesia pada tanggal laporan, atau rata-rata (kurs beli + kurs jual) / 2 bila
//     kurs tengah tidak tersedia. V = hasil perkalian III dengan IV.
//
// Prinsip yang dipegang berkas ini:
//   - Kolom V Nilai Rupiah adalah TURUNAN (III x IV) dan dihitung saat perakitan laporan;
//     tidak disimpan di register agar tidak ada dua sumber kebenaran.
//   - Kolom III dan IV adalah isian bank: laporan tidak mengarang nominal maupun kurs.
//   - Kolom II adalah sandi Lampiran 04. Daftar lengkap Lampiran 04 tidak disalin ke
//     kode (bukan bagian form yang ditranskrip), sehingga disimpan sebagai teks sandi
//     apa adanya tanpa daftar karangan.
//   - Kolom I "Sandi Kantor" tidak disimpan di register: diambil dari kantor pelapor
//     tunggal bank_offices (migrasi 000112), sama seperti form bank-wide lain.
//   - Form hanya relevan bila BPR berstatus pedagang valuta asing (ojk.report.pva_status);
//     register kosong tetap berarti form belum tersedia, bukan nol.

// Status baku register kas valuta asing.
const (
	KasValasStatusAktif    = "AKTIF"
	KasValasStatusNonaktif = "NONAKTIF"
)

// KasValasJenisCodeMax adalah batas panjang sandi Lampiran 04 yang disimpan apa adanya.
const KasValasJenisCodeMax = 16

var (
	// ErrKasValasBankWide menandai permintaan register kas valas oleh aktor yang tidak
	// berwenang atas seluruh bank. Register ini bank-wide.
	ErrKasValasBankWide = NewLocalizedError("kas_valas_bank_wide",
		"register kas valuta asing bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrKasValasInputInvalid menandai data register kas valas tidak sah.
	ErrKasValasInputInvalid = NewLocalizedError("kas_valas_input_invalid",
		"data register kas valuta asing tidak valid")
	// ErrKasValasNotFound menandai baris yang hendak dihapus tidak ada.
	ErrKasValasNotFound = NewLocalizedError("kas_valas_not_found",
		"data register kas valuta asing tidak ditemukan")
)

// KasValasItem adalah satu jenis valas (satu baris Form 03.00) yang bank catat.
// Kolom V Nilai Rupiah tidak disimpan: ia turunan III x IV lewat NilaiRupiah.
type KasValasItem struct {
	ID uuid.UUID `json:"id"`
	// JenisValasCode kolom II: sandi Lampiran 04, teks apa adanya.
	JenisValasCode string `json:"jenis_valas_code"`
	// Nominal kolom III: nilai valas original currency, 2 desimal; isian bank.
	Nominal decimal.Decimal `json:"nominal"`
	// KursTengah kolom IV: kurs tengah (Rp), 2 desimal; isian bank.
	KursTengah decimal.Decimal `json:"kurs_tengah"`
	AsOf       time.Time       `json:"as_of"`
	Status     string          `json:"status"`
	Note       string          `json:"note,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// NilaiRupiah adalah kolom V Nilai Rupiah: hasil perkalian Nominal dengan KursTengah
// (PDF #page 128), dibulatkan ke rupiah penuh seperti kolom rupiah lain. Ok=false bila
// salah satu faktor kosong/tak terisi sehingga laporan menuliskan "-" beserta alasannya,
// bukan nol.
func (i KasValasItem) NilaiRupiah() (decimal.Decimal, bool) {
	if i.Nominal.IsZero() || i.KursTengah.IsZero() {
		return decimal.Zero, false
	}
	return i.Nominal.Mul(i.KursTengah).Round(0), true
}

// UpdateKasValasItemInput adalah isi upsert satu baris kas valas. ID kosong = buat baru;
// ID terisi = perbarui baris itu. Angka menerima JSON number maupun string desimal.
type UpdateKasValasItemInput struct {
	ID             string          `json:"id"`
	JenisValasCode string          `json:"jenis_valas_code"`
	Nominal        decimal.Decimal `json:"nominal"`
	KursTengah     decimal.Decimal `json:"kurs_tengah"`
	AsOf           string          `json:"as_of"`
	Status         string          `json:"status"`
	Note           string          `json:"note"`
}

// BuildKasValasItem memvalidasi masukan dan membentuk baris yang siap disimpan. ID kosong
// dibangkitkan server; as_of diurai dengan format YYYY-MM-DD.
func BuildKasValasItem(in UpdateKasValasItemInput) (KasValasItem, error) {
	out := KasValasItem{
		JenisValasCode: strings.TrimSpace(in.JenisValasCode),
		Nominal:        in.Nominal,
		KursTengah:     in.KursTengah,
		Status:         strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:           strings.TrimSpace(in.Note),
	}
	if out.JenisValasCode == "" {
		return out, fmt.Errorf("%w: Jenis Valuta Asing wajib diisi", ErrKasValasInputInvalid)
	}
	if len(out.JenisValasCode) > KasValasJenisCodeMax {
		return out, fmt.Errorf("%w: Jenis Valuta Asing maksimal %d karakter",
			ErrKasValasInputInvalid, KasValasJenisCodeMax)
	}
	if out.Nominal.IsNegative() {
		return out, fmt.Errorf("%w: nominal tidak boleh negatif", ErrKasValasInputInvalid)
	}
	if out.KursTengah.IsNegative() {
		return out, fmt.Errorf("%w: kurs tengah tidak boleh negatif", ErrKasValasInputInvalid)
	}
	if out.Status == "" {
		out.Status = KasValasStatusAktif
	}
	if out.Status != KasValasStatusAktif && out.Status != KasValasStatusNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrKasValasInputInvalid)
	}
	var err error
	if out.ID, err = parseKasValasID(in.ID); err != nil {
		return out, err
	}
	if out.AsOf, err = parseKasValasDate(in.AsOf, "as_of"); err != nil {
		return out, err
	}
	return out, nil
}

// parseKasValasID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseKasValasID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrKasValasInputInvalid)
	}
	return id, nil
}

// parseKasValasDate mewajibkan tanggal berformat YYYY-MM-DD dan menyebut nama kolomnya.
func parseKasValasDate(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, fmt.Errorf("%w: %s wajib diisi (YYYY-MM-DD)", ErrKasValasInputInvalid, field)
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrKasValasInputInvalid, field)
	}
	return t, nil
}

// KasValasRegisterRepository menyimpan dan membaca register kas valuta asing. Seluruh
// pembacaan bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada
// transaksi yang sama.
type KasValasRegisterRepository interface {
	ListItems(ctx context.Context) ([]KasValasItem, error)
	// ListKasValasForOJK membaca baris AKTIF yang as_of-nya berada pada bulan asOf,
	// urutan deterministik. Sumber Form 03.00.
	ListKasValasForOJK(ctx context.Context, asOf time.Time) ([]KasValasItem, error)
	UpsertItemTx(ctx context.Context, tx any, item KasValasItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu baris. Form 03.00 tidak menetapkan nomor unik, sehingga
	// penghapusan adalah DELETE fisik. found=false bila baris tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// KasValasReport adalah keluaran baca-saja register kas valas untuk satu posisi.
type KasValasReport struct {
	AsOf  time.Time      `json:"as_of"`
	Items []KasValasItem `json:"items"`
}

// KasValasRegisterService merakit laporan register kas valas dan melayani pengisian
// berizin.
type KasValasRegisterService interface {
	KasValasReport(ctx context.Context, asOf time.Time, actor Actor) (KasValasReport, error)
	ListItems(ctx context.Context) ([]KasValasItem, error)
	UpsertItem(ctx context.Context, input UpdateKasValasItemInput, actor Actor) (*KasValasItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
