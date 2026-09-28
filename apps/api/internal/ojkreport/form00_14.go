package ojkreport

import (
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

// form00_14.go membangun laporan internal jenis nasabah per produk (dulu diberi nomor
// Form 00.14 — nomor itu tidak ada di SEOJK 16/2024, lihat docs/CELAH-FORM-OJK.md §6)
// sebagai agregasi jenis nasabah per produk simpanan.
//
// Laporan ini bukan form OJK: ia tidak terdaftar pada OJKBulananForms dan tidak
// diikutkan pada bundel ekspor bulanan (lihat builder.go). Namanya tetap dijaga di
// sini supaya agregasinya dapat dipakai sebagai data internal.
//
// Golongan nasabah memakai sandi Lampiran 02 – Daftar Sandi Pihak Lawan, sumber resmi
// "Golongan Nasabah" pada Form 11.00 (Daftar Tabungan) dan Form 12.00 (Daftar Deposito),
// yaitu customers.ojk_pihak_lawan_code. Sandi yang belum diisi ditulis "-".
//
// Form ini TIDAK menghitung ulang saldo dari jurnal; angka diambil dari sumber data
// pada posisi AKHIR PERIODE laporan (saldo rekening direkonstruksi dari jurnal,
// kontrak deposito dipilih menurut tanggal mulai/penutupan), bukan keadaan saat ekspor
// dijalankan.

const (
	form0014SandiJenisProduk  = "JENIS_PRODUK"
	form0014SandiKodeProduk   = "KODE_PRODUK"
	form0014SandiNamaProduk   = "NAMA_PRODUK"
	form0014SandiGolongan     = "GOLONGAN_NASABAH"
	form0014SandiJumlahRek    = "JUMLAH_REKENING"
	form0014SandiTotalNominal = "TOTAL_NOMINAL"
)

// buildForm00_14 menyusun tabel laporan internal jenis nasabah per produk (dulu
// "Form 00.14") dari agregasi yang sudah dihitung sumber data. Fungsi ini murni
// sehingga dapat diuji tanpa basis data; hasilnya tidak diikutkan pada ekspor OJK.
func buildForm00_14(rows []SavingsCustomerTypeRow) TableSection {
	sec := TableSection{
		Form:     "JENIS_NASABAH_PRODUK",
		Name:     "Laporan Internal Jenis Nasabah per Produk",
		KeyLabel: "Kode Produk / Golongan Nasabah",
		Columns: []TableColumn{
			{Sandi: form0014SandiJenisProduk, Nama: "Jenis Produk"},
			{Sandi: form0014SandiKodeProduk, Nama: "Kode Produk"},
			{Sandi: form0014SandiNamaProduk, Nama: "Nama Produk"},
			{Sandi: form0014SandiGolongan, Nama: "Golongan Nasabah (Lampiran 02)"},
			{Sandi: form0014SandiJumlahRek, Nama: "Jumlah Rekening"},
			{Sandi: form0014SandiTotalNominal, Nama: "Total Nominal"},
		},
		Notes: []string{
			"Golongan nasabah memakai sandi Lampiran 02 – Daftar Sandi Pihak Lawan (customers.ojk_pihak_lawan_code), sumber resmi yang sama dengan kolom Golongan Nasabah Form 11.00/12.00 SEOJK 16/2024; sandi yang belum diisi ditulis '-'.",
			"Sumber angka: saldo rekening tabungan/giro dan nominal penempatan deposito berjangka pada posisi akhir periode. Saldo rekening direkonstruksi dari jurnal (entry_date <= akhir periode); kontrak deposito dipilih menurut tanggal mulai dan penutupannya.",
			"Posisi yang dibaca adalah akhir periode laporan, bukan keadaan saat ekspor dijalankan, sehingga ekspor yang ditunda tetap melaporkan bulan yang diminta.",
		},
	}
	for _, r := range rows {
		produk := dashIfEmpty(r.ProductCode)
		golongan := dashIfEmpty(r.CustomerTypeCode)
		row := TableRow{Key: produk + " / " + golongan}
		row.Cells = append(row.Cells,
			TableCell{Sandi: form0014SandiJenisProduk, Nama: "Jenis Produk", Value: labelProductFamily(r.ProductFamily)},
			TableCell{Sandi: form0014SandiKodeProduk, Nama: "Kode Produk", Value: produk},
			TableCell{Sandi: form0014SandiNamaProduk, Nama: "Nama Produk", Value: dashIfEmpty(r.ProductName)},
			TableCell{Sandi: form0014SandiGolongan, Nama: "Golongan Nasabah (Lampiran 02)", Value: golongan},
			TableCell{Sandi: form0014SandiJumlahRek, Nama: "Jumlah Rekening", Value: fmt.Sprintf("%d", r.AccountCount)},
			TableCell{Sandi: form0014SandiTotalNominal, Nama: "Total Nominal", Value: FormatRupiah(r.TotalAmount)},
		)
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// labelProductFamily menulis nama keluarga produk simpanan dalam bahasa Indonesia.
// Keluarga tak dikenal (mis. baris lama tanpa produk) ditulis "-", bukan dikarang.
func labelProductFamily(family string) string {
	switch domain.ProductFamily(strings.TrimSpace(family)) {
	case domain.FamilySavings:
		return "Tabungan"
	case domain.FamilyTimeDeposit:
		return "Deposito Berjangka"
	case domain.FamilyCurrentAccount:
		return "Giro"
	default:
		return "-"
	}
}
