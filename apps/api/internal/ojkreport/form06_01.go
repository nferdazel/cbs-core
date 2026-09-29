package ojkreport

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form06_01.go membangun Form 06.01 "DAFTAR AGUNAN" dari loan_collaterals (migrasi 000125
// menambahkan kolom Form 06.01-nya).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 06.01 "DAFTAR AGUNAN": PDF #page 169 (hlm. tercetak 117), sandi PDF #page 170
//     (hlm. 118), penjelasan PDF #page 171-172 (hlm. 119-120). Delapan kolom: I Sandi
//     Kantor, II Kode Register/Nomor Agunan, III No. Rekening, IV Jenis Agunan,
//     V Alamat Agunan, VI Nilai yang Diagunkan, VII Nilai Agunan (Nominal + Penilai +
//     Tanggal Penilaian Terakhir), VIII Nilai yang Diperhitungkan untuk PPKA.
//     ADA baris JUMLAH.
//   - Penilai (VII.b): 1 Penilai Independen, 2 Internal BPR (PDF #170).
//   - Kolom IV mengacu Lampiran 01 (PDF #page 301): sandi 3 digit; kategori induk
//     Likuid (101-103) / Non Likuid (201+).
//
// KEPUTUSAN SME 29 Sep 2026: "Likuid | Non Likuid" pada halaman susunan BUKAN kolom
// kesembilan; daftar sandi resmi berhenti di kolom VIII dan penjelasan meletakkannya
// sebagai rincian di dalam kolom IV. Keduanya kategori induk dari sandi Lampiran 01.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Seluruh kolom angka (VI, VII.a, VIII) adalah ISIAN BANK; form tidak memberi rumus
//     pengikat, jadi laporan TIDAK menurunkan nilai apa pun.
//   - Kolom VIII Nilai untuk PPKA tidak dihitung dari taksasi/bound; kebijakan PPKA per
//     agunan adalah keputusan bank.
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice, bukan dari register.
//   - Tanpa agunan AKTIF, form dinyatakan belum tersedia, bukan ditulis kosong.
//   - Kolom II unik dan wajib; bila belum diisi bank, baris tetap tampil dengan kolom II
//     "-" beralasan (bukan dikarang), karena kode register harus berasal dari SLIK bank.

// AgunanOJKSource menyediakan baris agunan untuk Form 06.01. Kontraknya opsional pada
// perakitan RepoSource: tanpa sumber ini Form 06.01 dinyatakan belum tersedia.
type AgunanOJKSource interface {
	ListAgunanForOJK(ctx context.Context, actor domain.Actor) ([]domain.AgunanRow, error)
}

// form06_01TotalKey adalah kunci baris kaki tabel JUMLAH.
const form06_01TotalKey = "JUMLAH"

// BuildForm06_01 menyusun tabel Form 06.01 dari baris agunan dan kantor pelapor kolom I.
// Fungsi ini murni sehingga dapat diuji tanpa basis data. Form ini punya baris JUMLAH.
func BuildForm06_01(rows []domain.AgunanRow, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "06.01",
		Name:     formName("06.01"),
		KeyLabel: "Kode Register/Nomor Agunan",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Kode Register/Nomor Agunan"},
			{Sandi: "III", Nama: "No. Rekening"},
			{Sandi: "IV", Nama: "Jenis Agunan"},
			{Sandi: "V", Nama: "Alamat Agunan"},
			{Sandi: "VI", Nama: "Nilai yang Diagunkan"},
			{Sandi: "VII", Nama: "Nilai Agunan"},
			{Sandi: "VII", Nama: "Penilai"},
			{Sandi: "VII", Nama: "Tanggal Penilaian Terakhir"},
			{Sandi: "VIII", Nama: "Nilai yang Diperhitungkan untuk PPKA"},
		},
		Notes: []string{
			"Baris per agunan AKTIF dari loan_collaterals yang bank isi (Form 06.01, PDF #page 169-172); satu baris = satu agunan, dengan baris JUMLAH.",
			"Kode Register/Nomor Agunan (II) wajib, unik, tidak boleh dipakai ulang (no reuse/no recycle), dan tidak boleh berubah selama fasilitas tercatat; harus sama dengan SLIK (PDF #171).",
			"Jenis Agunan (IV) memakai sandi Lampiran 01 (3 digit, mis. 101 SBI/Surat Utang Pemerintah, 202 tanah/bangunan). Kategori induk Likuid (1xx)/Non Likuid (2xx) adalah turunan digit pertama sandi, bukan kolom tersendiri; daftar lampiran tidak disalin ke kode.",
			"Nilai yang Diagunkan (VI) adalah nilai yang diikat sebagai jaminan, berbeda dari Nilai Agunan (VII) yang merupakan taksasi (PDF #170-171); keduanya isian bank dan laporan tidak memaksa keduanya sama.",
			"Penilai (VII.b) ditulis sebagai label baku: 1 Penilai Independen, 2 Internal BPR (PDF #170). Tanggal Penilaian Terakhir (VII.c) memakai format TT-BB-TTTT pada form.",
			"Nilai yang Diperhitungkan untuk PPKA (VIII) adalah isian bank, TIDAK diturunkan dari taksasi/nilai yang diagunkan karena form tidak memberi rumus pengikat.",
			"Baris JUMLAH menjumlahkan kolom VI, VII Nilai Agunan, dan VIII; kolom sandi/tanggal pada baris JUMLAH ditulis \"-\" karena penjumlahan sandi tidak bermakna. Bila ada baris yang kolomnya belum diisi, JUMLAH tidak dihitung sebagian agar tidak menyesatkan.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	totalDiagunkan := decimal.Zero
	totalNilai := decimal.Zero
	totalPPKA := decimal.Zero
	lengkap := true

	for _, item := range rows {
		row := TableRow{Key: form06_01RowKey(item)}
		if item.KodeRegister == "" || item.JenisAgunanCode == "" || item.AlamatAgunan == "" || item.PenilaiCode == "" {
			lengkap = false
			row.Reason = "sebagian kolom Form 06.01 belum diisi bank (Kode Register/Jenis/ Alamat/Penilai); sel ditulis \"-\" dan JUMLAH tidak dihitung sebagian"
		}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "Kode Register/Nomor Agunan", Value: dashIfEmpty(item.KodeRegister)},
			{Sandi: "III", Nama: "No. Rekening", Value: dashIfEmpty(item.LoanNumber)},
			{Sandi: "IV", Nama: "Jenis Agunan", Value: dashIfEmpty(item.JenisAgunanCode)},
			{Sandi: "V", Nama: "Alamat Agunan", Value: dashIfEmpty(item.AlamatAgunan)},
			{Sandi: "VI", Nama: "Nilai yang Diagunkan", Value: FormatRupiah(item.NilaiDiagunkan)},
			{Sandi: "VII", Nama: "Nilai Agunan", Value: FormatRupiah(item.NilaiAgunan)},
			{Sandi: "VII", Nama: "Penilai", Value: form06_01PenilaiLabel(item.PenilaiCode)},
			{Sandi: "VII", Nama: "Tanggal Penilaian Terakhir", Value: formatTanggalForm06_01(item)},
			{Sandi: "VIII", Nama: "Nilai yang Diperhitungkan untuk PPKA", Value: FormatRupiah(item.PPKAAmount)},
		}
		totalDiagunkan = totalDiagunkan.Add(item.NilaiDiagunkan)
		totalNilai = totalNilai.Add(item.NilaiAgunan)
		totalPPKA = totalPPKA.Add(item.PPKAAmount)
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form06_01TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Kode Register/Nomor Agunan", Value: "JUMLAH"},
			{Sandi: "III", Nama: "No. Rekening", Value: "-"},
			{Sandi: "IV", Nama: "Jenis Agunan", Value: "-"},
			{Sandi: "V", Nama: "Alamat Agunan", Value: "-"},
			{Sandi: "VI", Nama: "Nilai yang Diagunkan", Value: "-"},
			{Sandi: "VII", Nama: "Nilai Agunan", Value: "-"},
			{Sandi: "VII", Nama: "Penilai", Value: "-"},
			{Sandi: "VII", Nama: "Tanggal Penilaian Terakhir", Value: "-"},
			{Sandi: "VIII", Nama: "Nilai yang Diperhitungkan untuk PPKA", Value: "-"},
		},
	}
	if len(rows) > 0 && lengkap {
		totalRow.Cells[4].Value = FormatRupiah(totalDiagunkan)
		totalRow.Cells[5].Value = FormatRupiah(totalNilai)
		totalRow.Cells[8].Value = FormatRupiah(totalPPKA)
	} else if len(rows) > 0 {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena ada baris yang kolomnya belum diisi; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form06_01RowKey mengidentifikasi satu baris Form 06.01 secara terbaca: kode register;
// bila belum diisi, dipakai id agunan agar baris tidak bertabrakan.
func form06_01RowKey(item domain.AgunanRow) string {
	if item.KodeRegister != "" {
		return item.KodeRegister
	}
	return "agunan:" + item.ID.String()
}

// form06_01PenilaiLabel menulis sandi VII.b sebagai label baku (PDF #170).
func form06_01PenilaiLabel(code string) string {
	switch code {
	case domain.AgunanPenilaiIndependen:
		return "1 - Penilai Independen"
	case domain.AgunanPenilaiInternal:
		return "2 - Internal BPR"
	default:
		return dashIfEmpty(code)
	}
}

// formatTanggalForm06_01 menulis tanggal penilaian terakhir; nol-waktu berarti belum diisi.
func formatTanggalForm06_01(item domain.AgunanRow) string {
	if item.TanggalPenilaian.IsZero() {
		return "-"
	}
	return item.TanggalPenilaian.Format("2006-01-02")
}
