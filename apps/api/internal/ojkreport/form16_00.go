package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form16_00.go membangun Form 16.00 "DAFTAR PENYERTAAN MODAL" dari register
// penyertaan_modal_register (migrasi 000120).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 16.00 "DAFTAR PENYERTAAN MODAL": PDF #page 223 (hlm. tercetak 171), lanjutan
//     PDF #page 224 (hlm. 172). Lima belas kolom: I Sandi Kantor, II No. Register,
//     III ID Pihak Lawan, IV Metode Penyertaan, V Kualitas, VI Tujuan Penyertaan,
//     VII Tanggal Mulai, VIII Persentase Penyertaan, IX Nominal, X Jumlah Bulan Laporan,
//     XI Cadangan Kerugian Penurunan Nilai, XII CKPN Aset Baik,
//     XIII CKPN Aset Kurang Baik, XIV CKPN Aset Tidak Baik, XV Jenis CKPN.
//     Satu baris = satu penyertaan modal; TIDAK ADA baris JUMLAH.
//   - Sandi: PDF #page 225-226 (hlm. 173-174). IV 1 Biaya Perolehan / 2 Metode Ekuitas;
//     V 1 Lancar / 3 Kurang Lancar / 4 Diragukan / 5 Macet; VI 1 Lembaga Penunjang /
//     9 Lainnya; XV 1 Individual / 2 Kolektif.
//   - Penjelasan: PDF #page 227-228 (hlm. 175-176). No. Register unik, no reuse/no
//     recycle. Kolom X adalah "nilai tercatat penyertaan modal pada bulan laporan"
//     (PDF #page 228), yaitu NILAI rupiah yang diisi bank, bukan jumlah bulan.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH nilai, sandi, dan tanggal adalah isian bank dari register. Tidak ada kolom
//     turunan: form tidak memberikan rumus yang mengikat untuk kolom X maupun blok CKPN,
//     sehingga laporan menuliskan angka bank apa adanya.
//   - Form ini TIDAK punya baris JUMLAH (PDF #page 223-224): tabel berakhir setelah baris
//     penyertaan terakhir.
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice; register sendiri bank-wide, tidak per kantor.
//   - Nomor register unik dan tidak boleh dipakai ulang; baris NONAKTIF tidak dibaca
//     (ListPenyertaanForOJK menyaring status AKTIF), sehingga tidak muncul di form.

// PenyertaanRegisterSource menyediakan baris register penyertaan modal bank-wide untuk
// Form 16.00. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 16.00
// dinyatakan belum tersedia, bukan ditulis kosong.
type PenyertaanRegisterSource interface {
	ListPenyertaanForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.PenyertaanItem, error)
}

// BuildForm16_00 menyusun tabel Form 16.00 dari baris register penyertaan dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada
// baris, Rows kosong tetapi daftar kolom tetap dibawa. TIDAK ada baris JUMLAH.
func BuildForm16_00(rows []domain.PenyertaanItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "16.00",
		Name:     formName("16.00"),
		KeyLabel: "No. Register",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "No. Register"},
			{Sandi: "III", Nama: "ID Pihak Lawan"},
			{Sandi: "IV", Nama: "Metode Penyertaan"},
			{Sandi: "V", Nama: "Kualitas"},
			{Sandi: "VI", Nama: "Tujuan Penyertaan"},
			{Sandi: "VII", Nama: "Tanggal Mulai"},
			{Sandi: "VIII", Nama: "Persentase Penyertaan"},
			{Sandi: "IX", Nama: "Nominal"},
			{Sandi: "X", Nama: "Jumlah Bulan Laporan"},
			{Sandi: "XI", Nama: "Cadangan Kerugian Penurunan Nilai"},
			{Sandi: "XII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik"},
			{Sandi: "XIII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik"},
			{Sandi: "XIV", Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik"},
			{Sandi: "XV", Nama: "Jenis CKPN"},
		},
		Notes: []string{
			"Baris per penyertaan modal dari register penyertaan_modal_register yang bank isi (Form 16.00, PDF #page 223-224); satu baris = satu penyertaan modal kepada satu pihak lawan dan TIDAK ada baris JUMLAH.",
			"Metode Penyertaan (IV) mengikuti sandi PDF #page 225: 1 Biaya Perolehan, 2 Metode Ekuitas. Kualitas (V): 1 Lancar, 3 Kurang Lancar, 4 Diragukan, 5 Macet. Tujuan Penyertaan (VI): 1 Dalam Rangka Investasi - Penyertaan pada Lembaga Penunjang, 9 Lainnya. Jenis CKPN (XV): 1 Individual, 2 Kolektif.",
			"Seluruh nilai (VIII, IX, X, XI-XIV) adalah isian bank. Form tidak memberikan rumus yang mengikat untuk kolom X maupun blok CKPN, sehingga laporan tidak menghitung atau menurunkan angka apa pun.",
			"Kolom X bernama \"Jumlah Bulan Laporan\" tetapi menurut definisi PDF #page 228 adalah nilai tercatat penyertaan modal pada bulan laporan, yaitu NILAI rupiah pada bulan laporan; diisi bank, bukan jumlah bulan.",
			"ID Pihak Lawan (III) BUKAN sandi Lampiran 02: tabel sandi Form 16.00 tidak memuat sandi untuk kolom III, sehingga disimpan dan ditulis sebagai teks apa adanya tanpa daftar karangan.",
			"No. Register (II) unik dan tidak boleh dipakai ulang (no reuse/no recycle, PDF #page 227). Penghapusan register adalah soft-delete NONAKTIF; baris NONAKTIF tidak ikut form ini karena sumbernya menyaring status AKTIF.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form16_00RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "No. Register", Value: dashIfEmpty(item.NoRegister)},
			{Sandi: "III", Nama: "ID Pihak Lawan", Value: dashIfEmpty(item.CounterpartyID)},
			{Sandi: "IV", Nama: "Metode Penyertaan", Value: dashIfEmpty(item.MetodePenyertaanCode)},
			{Sandi: "V", Nama: "Kualitas", Value: dashIfEmpty(item.KualitasCode)},
			{Sandi: "VI", Nama: "Tujuan Penyertaan", Value: dashIfEmpty(item.TujuanPenyertaanCode)},
			{Sandi: "VII", Nama: "Tanggal Mulai", Value: form16_00Tanggal(item.TanggalMulai)},
			{Sandi: "VIII", Nama: "Persentase Penyertaan", Value: FormatPersen(item.PersentasePenyertaan)},
			{Sandi: "IX", Nama: "Nominal", Value: FormatRupiah(item.Nominal)},
			{Sandi: "X", Nama: "Jumlah Bulan Laporan", Value: FormatRupiah(item.JumlahBulanLaporan)},
			{Sandi: "XI", Nama: "Cadangan Kerugian Penurunan Nilai", Value: FormatRupiah(item.CKPN)},
			{Sandi: "XII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik", Value: FormatRupiah(item.CKPNAsetBaik)},
			{Sandi: "XIII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik", Value: FormatRupiah(item.CKPNAsetKurangBaik)},
			{Sandi: "XIV", Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik", Value: FormatRupiah(item.CKPNAsetTidakBaik)},
			{Sandi: "XV", Nama: "Jenis CKPN", Value: dashIfEmpty(item.JenisCKPNCode)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form16_00RowKey mengidentifikasi satu baris Form 16.00 secara terbaca: No. Register.
func form16_00RowKey(item domain.PenyertaanItem) string {
	return item.NoRegister
}

// form16_00Tanggal menulis Tanggal Mulai dalam format TT-BB-TTTT, mengikuti mayoritas
// form. Form 16.00 tidak menyimpang seperti Form 18.00 yang memakai TT-MM-TTTT.
func form16_00Tanggal(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006")
}
