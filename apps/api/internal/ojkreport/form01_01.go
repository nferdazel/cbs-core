package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form01_01.go membangun Form 01.01 "REKENING ADMINISTRATIF" dari register pos
// komitmen/kontinjensi off-balance (tabel off_balance_items, migrasi 000113).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 01.01 - 1 "REKENING ADMINISTRATIF": PDF #page 110 (hlm. tercetak 58).
//     Kolom I Sandi Kantor, II Nama Rekening, III Sandi, IV Jumlah.
//   - Penjelasan pos "D. LAPORAN KOMITMEN DAN KONTINJENSI": PDF #page 488.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH angka berasal dari register yang bank isi; sistem TIDAK menurunkan pos
//     dari jurnal. Memakai basis kas untuk kredit NPL dan tidak mereklasifikasi bunga
//     terakru (docs/KEPUTUSAN-OJK.md §4), sehingga pos seperti "Pendapatan Bunga Dalam
//     Penyelesaian" dan "Aset Produktif yang Dihapus Buku" hanya terisi bila bank
//     mencatatnya, bukan dihitung mesin.
//   - Nama pos resmi hanya dipetakan untuk sandi yang memang tercantum di PDF #page
//     110; sandi di luar itu ditampilkan dengan uraian bank apa adanya, tidak ditutup
//     dengan nama karangan. Baris tanpa sandi ditulis "-" pada kolom Sandi.
//   - Kolom I "Sandi Kantor" tidak punya sumber: register bersifat bank-wide dan tidak
//     menyimpan kantor, jadi didaftarkan Unavailable, bukan diisi "-" tanpa alasan.

// OffBalanceSource menyediakan agregat register rekening administratif bank-wide.
// Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 01.01
// dinyatakan belum tersedia, bukan ditulis kosong.
type OffBalanceSource interface {
	ListOffBalanceForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.OffBalanceAggregate, error)
}

// form0101PositionNames memetakan sandi pos Form 01.01 ke nama resminya menurut
// PDF #page 110 (hlm. 58), diulang pada penjelasan PDF #page 488. Hanya sandi ini yang
// punya nama resmi; sandi lain memakai uraian bank.
var form0101PositionNames = map[string]string{
	// Tagihan Komitmen.
	"6101010000": "Fasilitas Pinjaman yang Diterima yang Belum Ditarik",
	"6101990000": "Tagihan Komitmen Lainnya",
	// Kewajiban Komitmen.
	"6102010000": "Fasilitas Kredit kepada Nasabah yang Belum Ditarik",
	"6102020000": "Penerusan Kredit",
	"6102990000": "Kewajiban Komitmen Lainnya",
	// Tagihan Kontinjensi: Pendapatan Bunga Dalam Penyelesaian.
	"6201010100": "Bunga Kredit yang Diberikan",
	"6201010200": "Bunga Penempatan pada Bank Lain",
	"6201010300": "Surat Berharga",
	"6201010900": "Pendapatan Bunga Dalam Penyelesaian Lainnya",
	// Tagihan Kontinjensi: Aset Produktif yang Dihapus Buku.
	"6201020100": "Kredit yang Diberikan",
	"6201020200": "Penempatan pada Bank Lain",
	"6201020300": "Pendapatan Bunga atas Kredit yang Dihapus Buku",
	"6201020400": "Pendapatan Bunga atas Penempatan Dana pada Bank Lain yang Dihapus Buku",
	// Tagihan Kontinjensi lain dan Kewajiban/Administratif.
	"6201030000": "Agunan dalam Proses Penyelesaian Kredit",
	"6201990000": "Tagihan Kontinjensi Lainnya",
	"6202000000": "Kewajiban Kontinjensi",
	"6900000000": "Rekening Administratif Lainnya",
}

// BuildForm01_01 menyusun tabel Form 01.01 dari agregat register yang sudah dihitung
// sumber data. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada
// baris, Rows kosong tetapi daftar kolom yang belum tersedia tetap dibawa.
func BuildForm01_01(rows []domain.OffBalanceAggregate) TableSection {
	sec := TableSection{
		Form:     "01.01",
		Name:     formName("01.01"),
		KeyLabel: "Sandi/Nama Pos",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Nama Rekening"},
			{Sandi: "III", Nama: "Sandi"},
			{Sandi: "KATEGORI", Nama: "Kategori"},
			{Sandi: "IV", Nama: "Jumlah"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "I", Nama: "Sandi Kantor",
				Reason: "register rekening administratif dicatat bank-wide (tidak menyimpan kantor); kolom Sandi Kantor belum punya sumber dan tidak dikarang"},
		},
		Notes: []string{
			"Nama pos resmi mengikuti Form 01.01 - 1 SEOJK No. 16/SEOJK.03/2024 (PDF #page 110, hlm. 58); sandi di luar daftar resmi ditampilkan memakai uraian bank, bukan nama karangan.",
			"Seluruh angka berasal dari register rekening administratif yang bank isi. Sistem tidak menurunkan pos dari jurnal dan memakai basis kas untuk kredit NPL (docs/KEPUTUSAN-OJK.md §4), sehingga pos seperti Pendapatan Bunga Dalam Penyelesaian dan Aset Produktif yang Dihapus Buku hanya muncul bila bank mencatatnya.",
			"Baris diagregasi per kategori dan sandi pos; pos tanpa sandi dipisah menurut uraian bank. Baris tanpa sandi ditulis \"-\" pada kolom Sandi.",
			"Form 01.01 lengkap (Sandi Kantor + Nama Rekening + Sandi + Jumlah) belum dapat direkonstruksi karena register tidak menyimpan kantor; yang disajikan adalah posisi bank-wide.",
		},
	}
	for _, a := range rows {
		nama := form0101PositionNames[a.PositionCode]
		if nama == "" {
			nama = dashIfEmpty(a.Description)
		}
		key := a.PositionCode
		if key == "" {
			key = a.Category + "/" + dashIfEmpty(a.Description)
		}
		row := TableRow{Key: key}
		row.Cells = append(row.Cells,
			TableCell{Sandi: "II", Nama: "Nama Rekening", Value: nama},
			TableCell{Sandi: "III", Nama: "Sandi", Value: dashIfEmpty(a.PositionCode)},
			TableCell{Sandi: "KATEGORI", Nama: "Kategori", Value: a.Category},
			TableCell{Sandi: "IV", Nama: "Jumlah", Value: FormatRupiah(a.TotalAmount)},
		)
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}
