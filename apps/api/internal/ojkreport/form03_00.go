package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form03_00.go membangun Form 03.00 "DAFTAR KAS DALAM VALUTA ASING" dari register
// kas_valas_register (migrasi 000123).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 03.00 "DAFTAR KAS DALAM VALUTA ASING": PDF #page 126 (hlm. tercetak 74),
//     sandi PDF #page 127 (hlm. 75), penjelasan PDF #page 128 (hlm. 76); cross-ref
//     posisi keuangan PDF #page 102 (hlm. 50). Lima kolom: I Sandi Kantor,
//     II Jenis Valuta Asing, III Nominal, IV Kurs Tengah (Rp), V Nilai Rupiah.
//     Satu baris = satu jenis valas yang diperdagangkan; ADA baris JUMLAH (PDF #126).
//   - Definisi dan aturan (PDF #page 128): kas valas = uang kertas/uang logam asing dan
//     cek pelawat yang masih berlaku yang dimiliki BPR sebagai pedagang valuta asing.
//     III adalah nilai valas (original currency) sebelum dirupiahkan pada tanggal
//     laporan. IV adalah kurs tengah Bank Indonesia pada tanggal laporan, atau rata-rata
//     (kurs beli + kurs jual) / 2 bila kurs tengah tidak tersedia. V = III x IV.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Kolom III dan IV adalah isian bank dari register.
//   - Kolom V "Nilai Rupiah" adalah TURUNAN III x IV. Bila III atau IV kosong/tak terisi,
//     kolom V ditulis "-" beserta alasannya, bukan nol; baris JUMLAH mengikuti aturan
//     yang sama agar total sebagian tidak menyesatkan.
//   - Kolom II memakai sandi Lampiran 04; daftar lengkap lampiran tidak disalin ke kode,
//     sehingga laporan menuliskan sandi bank apa adanya. Kolom I Sandi Kantor diambil dari
//     kantor pelapor tunggal bank_offices (migrasi 000112) lewat ReportingOffice.
//   - Form ini hanya relevan bila BPR berstatus pedagang valuta asing; tanpa baris AKTIF
//     pada bulan periode, form dinyatakan belum tersedia, bukan ditulis kosong.

// KasValasRegisterSource menyediakan baris register kas valas bank-wide untuk Form 03.00.
// Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 03.00 dinyatakan
// belum tersedia, bukan ditulis kosong.
type KasValasRegisterSource interface {
	ListKasValasForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.KasValasItem, error)
}

// form03_00TotalKey adalah kunci baris kaki tabel JUMLAH; bukan sandi pos OJK.
const form03_00TotalKey = "JUMLAH"

// BuildForm03_00 menyusun tabel Form 03.00 dari baris register kas valas dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada
// baris, Rows hanya berisi baris JUMLAH berisi "-".
func BuildForm03_00(rows []domain.KasValasItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "03.00",
		Name:     formName("03.00"),
		KeyLabel: "Jenis Valuta Asing",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Jenis Valuta Asing"},
			{Sandi: "III", Nama: "Nominal"},
			{Sandi: "IV", Nama: "Kurs Tengah (Rp)"},
			{Sandi: "V", Nama: "Nilai Rupiah"},
		},
		Notes: []string{
			"Baris per jenis valuta asing dari register kas_valas_register yang bank isi (Form 03.00, PDF #page 126-128); satu baris = satu jenis valas yang diperdagangkan, dengan baris JUMLAH.",
			"Form ini hanya relevan bila BPR berstatus pedagang valuta asing (PVA, PDF #page 128): kas valas adalah uang kertas/uang logam asing dan cek pelawat yang masih berlaku yang dimiliki BPR sebagai PVA.",
			"Nominal (III) adalah nilai valas (original currency) sebelum dirupiahkan, 2 desimal; isian bank. Kurs Tengah (IV) adalah kurs tengah Bank Indonesia pada tanggal laporan, atau rata-rata (kurs beli + kurs jual) / 2 bila kurs tengah tidak tersedia (PDF #page 128); isian bank.",
			"Nilai Rupiah (V) adalah turunan: hasil perkalian Nominal dengan Kurs Tengah (PDF #page 128). Bila Nominal atau Kurs Tengah belum diisi, kolom V ditulis \"-\" beserta alasannya, bukan nol.",
			"Jenis Valuta Asing (II) memakai sandi Lampiran 04; laporan menuliskan sandi bank apa adanya dan tidak menyalin daftar lampiran ke kode.",
			"Baris JUMLAH menjumlahkan Nominal dan Nilai Rupiah; kolom Kurs Tengah pada baris JUMLAH ditulis \"-\" karena penjumlahan kurs tidak bermakna.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	totalNominal := decimal.Zero
	totalNilai := decimal.Zero
	lengkap := true

	for _, item := range rows {
		nilai, ok := item.NilaiRupiah()
		row := TableRow{Key: form03_00RowKey(item)}
		if !ok {
			lengkap = false
			row.Reason = "Nilai Rupiah (V) tidak dapat dihitung karena Nominal (III) atau Kurs Tengah (IV) belum diisi/nol; kolom ditulis \"-\" dan tidak dikarang"
		}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "Jenis Valuta Asing", Value: dashIfEmpty(item.JenisValasCode)},
			{Sandi: "III", Nama: "Nominal", Value: form03_00DuaDesimal(item.Nominal)},
			{Sandi: "IV", Nama: "Kurs Tengah (Rp)", Value: form03_00DuaDesimal(item.KursTengah)},
			{Sandi: "V", Nama: "Nilai Rupiah", Value: form03_00NilaiCell(nilai, ok)},
		}
		totalNominal = totalNominal.Add(item.Nominal)
		totalNilai = totalNilai.Add(nilai)
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form03_00TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Jenis Valuta Asing", Value: "JUMLAH"},
			{Sandi: "III", Nama: "Nominal", Value: "-"},
			{Sandi: "IV", Nama: "Kurs Tengah (Rp)", Value: "-"},
			{Sandi: "V", Nama: "Nilai Rupiah", Value: "-"},
		},
	}
	if len(rows) > 0 && lengkap {
		totalRow.Cells[1].Value = form03_00DuaDesimal(totalNominal)
		totalRow.Cells[3].Value = FormatRupiah(totalNilai)
	} else if len(rows) > 0 {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena ada baris yang Nominal/Kurs Tengah-nya belum diisi; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form03_00RowKey mengidentifikasi satu baris Form 03.00 secara terbaca: sandi jenis valas.
func form03_00RowKey(item domain.KasValasItem) string {
	return item.JenisValasCode
}

// form03_00DuaDesimal menulis Nominal/Kurs Tengah dengan 2 desimal sesuai transkrip form
// (III dan IV adalah "angka 2 desimal"). Ditulis lokal karena FormatPersen bermakna rasio.
func form03_00DuaDesimal(d decimal.Decimal) string {
	return d.Round(2).StringFixed(2)
}

// form03_00NilaiCell menulis kolom V turunan: angka bila dapat dihitung, "-" bila tidak.
func form03_00NilaiCell(nilai decimal.Decimal, ok bool) string {
	if !ok {
		return "-"
	}
	return FormatRupiah(nilai)
}
