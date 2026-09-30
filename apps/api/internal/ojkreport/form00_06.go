package ojkreport

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// ModalOJKSource menyediakan baris register modal untuk Form 00.06. Kontraknya opsional
// pada perakitan RepoSource: tanpa sumber ini Form 00.06 dinyatakan belum tersedia, bukan
// ditulis kosong.
type ModalOJKSource interface {
	ListModalForOJK(ctx context.Context) ([]domain.ModalItem, error)
}

// form00_06.go membangun Form 00.06 "DAFTAR MODAL DISETOR, MODAL SUMBANGAN, DAN DANA
// SETORAN MODAL - EKUITAS" dari register modal_register (migrasi 000128).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.06: susunan PDF #page 245 (hlm. tercetak 193), sandi PDF #page 246
//     (hlm. 194), penjelasan PDF #page 247 (hlm. 195). Empat kolom: I Jenis,
//     II Tanggal Persetujuan Otoritas, III Jenis Modal, IV Jumlah. ADA baris JUMLAH.
//   - Sandi I: 01 Dana, 02 Tanah/bangunan (modal inti), 03 Tanah/bangunan (bukan modal
//     inti). Sandi III: 01 Modal Disetor, 02 Modal Sumbangan, 03 Dana Setoran Modal -
//     Ekuitas.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Kolom IV Jumlah adalah ISIAN BANK (nominal yang diakui sebagai modal), bukan hasil
//     hitung dari saldo bagan akun; laporan hanya menjumlahkan baris untuk kaki JUMLAH.
//   - Kolom II Tanggal Persetujuan Otoritas boleh kosong (modal belum disetujui otoritas);
//     laporan menulis "-" tanpa menyimpulkan tanggal.
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice, bukan dari register.
//   - Tanpa baris register, form dinyatakan belum tersedia, bukan ditulis kosong.

// form00_06TotalKey adalah kunci baris kaki tabel JUMLAH.
const form00_06TotalKey = "JUMLAH"

// BuildForm00_06 menyusun tabel Form 00.06 dari baris register modal dan kantor pelapor
// kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Form ini punya baris
// JUMLAH yang menjumlahkan kolom IV.
func BuildForm00_06(rows []domain.ModalItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "00.06",
		Name:     formName("00.06"),
		KeyLabel: "Jenis Modal",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Tanggal Persetujuan Otoritas"},
			{Sandi: "III", Nama: "Jenis Modal"},
			{Sandi: "IV", Nama: "Jumlah"},
		},
		Notes: []string{
			"Baris per setoran/sumbangan modal dari register modal_register yang bank isi (Form 00.06, PDF #page 245-247); dengan baris JUMLAH.",
			"Jenis (I) memakai sandi baku PDF #page 246: 01 Dana, 02 Tanah dan bangunan yang dapat diperhitungkan sebagai modal inti, 03 Tanah dan bangunan yang tidak dapat diperhitungkan sebagai modal inti.",
			"Jenis Modal (III) memakai sandi baku PDF #page 246: 01 Modal Disetor, 02 Modal Sumbangan, 03 Dana Setoran Modal - Ekuitas. Definisi rinci ada di PDF #page 247 dan tidak disalin ke kode.",
			"Tanggal Persetujuan Otoritas (II) ditulis TT-BB-TTTT pada form; kekosongan sah untuk modal yang belum disetujui otoritas dan ditulis \"-\", bukan disimpulkan tanggalnya.",
			"Jumlah (IV) adalah nominal yang diakui sebagai modal (rupiah penuh), ISIAN BANK; laporan TIDAK menurunkannya dari saldo bagan akun karena saldo tidak menyimpan bentuk setoran maupun tanggal persetujuan.",
			"Baris JUMLAH menjumlahkan kolom IV; kolom sandi/tanggal pada baris JUMLAH ditulis \"-\" karena penjumlahan sandi tidak bermakna.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	total := decimal.Zero
	for _, item := range rows {
		row := TableRow{Key: form00_06RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "Tanggal Persetujuan Otoritas", Value: form00_06TanggalCell(item)},
			{Sandi: "III", Nama: "Jenis Modal", Value: form00_06JenisModalLabel(item.JenisModalCode)},
			{Sandi: "IV", Nama: "Jumlah", Value: FormatRupiah(item.Jumlah)},
		}
		// Jenis (I) tidak diserialisasi sebagai sel karena ia kunci baris; tampilkan
		// sebagai label pada key agar terbaca di tabel.
		row.Key = row.Key + " [" + form00_06JenisLabel(item.JenisCode) + "]"
		total = total.Add(item.Jumlah)
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form00_06TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Tanggal Persetujuan Otoritas", Value: "-"},
			{Sandi: "III", Nama: "Jenis Modal", Value: "JUMLAH"},
			{Sandi: "IV", Nama: "Jumlah", Value: "-"},
		},
	}
	if len(rows) > 0 {
		totalRow.Cells[2].Value = FormatRupiah(total)
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form00_06RowKey mengidentifikasi satu baris Form 00.06 secara terbaca.
func form00_06RowKey(item domain.ModalItem) string {
	return item.ID.String()
}

// form00_06TanggalCell menulis kolom II; kosong berarti belum dicatat (bukan tanggal nol).
func form00_06TanggalCell(item domain.ModalItem) string {
	if item.TanggalPersetujuan == nil {
		return "-"
	}
	return item.TanggalPersetujuan.Format("2006-01-02")
}

// form00_06JenisLabel menulis sandi I sebagai label baku (PDF #246).
func form00_06JenisLabel(code string) string {
	switch code {
	case domain.ModalJenisDana:
		return "01 - Dana"
	case domain.ModalJenisTanahBangunanInti:
		return "02 - Tanah/Bangunan (Modal Inti)"
	case domain.ModalJenisTanahBangunanNonInti:
		return "03 - Tanah/Bangunan (Bukan Modal Inti)"
	default:
		return dashIfEmpty(code)
	}
}

// form00_06JenisModalLabel menulis sandi III sebagai label baku (PDF #246).
func form00_06JenisModalLabel(code string) string {
	switch code {
	case domain.ModalJenisModalDisetor:
		return "01 - Modal Disetor"
	case domain.ModalJenisModalSumbangan:
		return "02 - Modal Sumbangan"
	case domain.ModalJenisDanaSetoranEkuitas:
		return "03 - Dana Setoran Modal - Ekuitas"
	default:
		return dashIfEmpty(code)
	}
}
