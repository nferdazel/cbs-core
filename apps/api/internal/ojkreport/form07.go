package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form07.go membangun Form 07.00 "DAFTAR AGUNAN YANG DIAMBIL ALIH" (AYDA) dari register
// ayda_register (migrasi 000115).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 07.00 - 1 "DAFTAR AGUNAN YANG DIAMBIL ALIH": PDF #page 179 (hlm. tercetak
//     127). Kolom I Sandi Kantor, II Jenis Agunan, III Alamat Agunan,
//     IV Tanggal Pengambilalihan, V Nilai Pengakuan Awal, VI Akumulasi Kerugian
//     Penurunan Nilai, VII Jumlah, VIII Nilai Bersih Yang Dapat Direalisasikan.
//     Ada baris JUMLAH di kaki tabel.
//   - Form 07.00 - 2 "SANDI DAFTAR AGUNAN YANG DIAMBIL ALIH": PDF #page 180 (hlm. 128).
//     Jenis agunan 01/02/03/04/05/99; tanggal ditulis TT-BB-TTTT.
//   - Form 07.00 - 3 "PENJELASAN": PDF #page 181 (hlm. 129).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH angka berasal dari register yang bank isi. Kolom V (nilai pengakuan awal)
//     adalah angka bank yang mengikuti nilai wajar dikurangi estimasi biaya penjualan,
//     paling tinggi baki debet (PDF #page 181); sistem TIDAK menghitungnya karena nilai
//     wajar, estimasi biaya penjualan, dan baki debet bukan kolom register.
//   - Kolom VII "Jumlah" adalah turunan: nilai yang lebih rendah dari NRV (VIII) atau
//     nilai tercatat (V - VI), mengikuti domain.AYDAItem.Jumlah. Bila nilai tercatat
//     negatif (akumulasi kerugian melebihi nilai pengakuan awal), kolom VII ditulis "-"
//     beserta alasannya, bukan angka negatif.
//   - Baris JUMLAH kaki tabel hanya diisi bila SELURUH baris punya nilai kolom VII;
//     bila ada yang tidak, JUMLAH ditulis "-" dengan alasan karena total sebagian akan
//     menyesatkan (pola Form 14.00).
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice. Register sendiri bank-wide, tidak per kantor.
//   - Register adalah RINCIAN per kasus; saldo buku besar AYDA tetap akun COA 10500,
//     sehingga total register TIDAK dipaksa sama dengan saldo COA. Keduanya berbeda bila
//     bank belum mengisi semua kasus.

// AYDARegisterSource menyediakan baris register AYDA bank-wide untuk Form 07.00.
// Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 07.00 dinyatakan
// belum tersedia, bukan ditulis kosong.
type AYDARegisterSource interface {
	ListAYDAForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.AYDAItem, error)
}

// form07TotalKey adalah kunci baris kaki tabel JUMLAH; bukan sandi pos OJK.
const form07TotalKey = "JUMLAH"

// BuildForm07 menyusun tabel Form 07.00 dari baris register AYDA dan kantor pelapor
// kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada baris,
// Rows kosong tetapi daftar kolom yang belum tersedia tetap dibawa.
func BuildForm07(rows []domain.AYDAItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "07.00",
		Name:     formName("07.00"),
		KeyLabel: "Tanggal|Alamat Agunan",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Jenis Agunan"},
			{Sandi: "III", Nama: "Alamat Agunan"},
			{Sandi: "IV", Nama: "Tanggal Pengambilalihan"},
			{Sandi: "V", Nama: "Nilai Pengakuan Awal"},
			{Sandi: "VI", Nama: "Akumulasi Kerugian Penurunan Nilai"},
			{Sandi: "VII", Nama: "Jumlah"},
			{Sandi: "VIII", Nama: "Nilai Bersih Yang Dapat Direalisasikan"},
		},
		Notes: []string{
			"Baris per AYDA (agunan yang diambil alih) dari register ayda_register yang bank isi; tanggal pengambilalihan ditulis TT-BB-TTTT sesuai Form 07.00 - 2.",
			"Nilai Pengakuan Awal (V) adalah angka bank yang mengikuti nilai wajar dikurangi estimasi biaya penjualan, paling tinggi baki debet (Form 07.00 - 3, PDF #page 181). Sistem tidak menghitungnya karena nilai wajar, estimasi biaya penjualan, dan baki debet bukan kolom register.",
			"Jumlah (VII) dihitung sebagai nilai yang lebih rendah dari NRV (VIII) atau nilai tercatat (nilai pengakuan awal V dikurangi akumulasi kerugian VI). Bila nilai tercatat negatif, kolom VII ditulis \"-\" beserta alasannya, bukan angka negatif.",
			"Baris JUMLAH hanya diisi bila seluruh baris punya nilai kolom VII; bila tidak, JUMLAH ditulis \"-\" karena total sebagian akan menyesatkan.",
			"Register AYDA adalah rincian per kasus, sedangkan saldo AYDA pada laporan posisi keuangan tetap pos 1201000000 dari akun COA 10500. Keduanya dapat berbeda bila bank belum mengisi semua kasus; laporan ini TIDAK memaksa total register sama dengan saldo COA (tidak ada tie otomatis).",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	totalV := decimal.Zero
	totalVI := decimal.Zero
	totalVII := decimal.Zero
	totalVIII := decimal.Zero
	lengkap := true
	for _, item := range rows {
		row := TableRow{Key: form07RowKey(item)}
		row.Cells = append(row.Cells,
			TableCell{Sandi: "II", Nama: "Jenis Agunan", Value: dashIfEmpty(item.CollateralTypeCode)},
			TableCell{Sandi: "III", Nama: "Alamat Agunan", Value: dashIfEmpty(item.CollateralAddress)},
			TableCell{Sandi: "IV", Nama: "Tanggal Pengambilalihan", Value: item.AcquisitionDate.Format("02-01-2006")},
			TableCell{Sandi: "V", Nama: "Nilai Pengakuan Awal", Value: FormatRupiah(item.InitialRecognitionValue)},
			TableCell{Sandi: "VI", Nama: "Akumulasi Kerugian Penurunan Nilai", Value: FormatRupiah(item.AccumulatedImpairment)},
		)
		jumlah, ok := item.Jumlah()
		if !ok {
			lengkap = false
			row.Reason = "akumulasi kerugian penurunan nilai melebihi nilai pengakuan awal; kolom VII Jumlah tidak dapat dihitung dan tidak diisi angka negatif"
			row.Cells = append(row.Cells,
				TableCell{Sandi: "VII", Nama: "Jumlah", Value: "-"},
				TableCell{Sandi: "VIII", Nama: "Nilai Bersih Yang Dapat Direalisasikan", Value: FormatRupiah(item.NetRealizableValue)},
			)
			sec.Rows = append(sec.Rows, row)
			continue
		}
		totalV = totalV.Add(item.InitialRecognitionValue)
		totalVI = totalVI.Add(item.AccumulatedImpairment)
		totalVII = totalVII.Add(jumlah)
		totalVIII = totalVIII.Add(item.NetRealizableValue)
		row.Cells = append(row.Cells,
			TableCell{Sandi: "VII", Nama: "Jumlah", Value: FormatRupiah(jumlah)},
			TableCell{Sandi: "VIII", Nama: "Nilai Bersih Yang Dapat Direalisasikan", Value: FormatRupiah(item.NetRealizableValue)},
		)
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form07TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Jenis Agunan", Value: "JUMLAH"},
			{Sandi: "III", Nama: "Alamat Agunan", Value: "-"},
			{Sandi: "IV", Nama: "Tanggal Pengambilalihan", Value: "-"},
			{Sandi: "V", Nama: "Nilai Pengakuan Awal", Value: "-"},
			{Sandi: "VI", Nama: "Akumulasi Kerugian Penurunan Nilai", Value: "-"},
			{Sandi: "VII", Nama: "Jumlah", Value: "-"},
			{Sandi: "VIII", Nama: "Nilai Bersih Yang Dapat Direalisasikan", Value: "-"},
		},
	}
	if lengkap && len(rows) > 0 {
		totalRow.Cells[3].Value = FormatRupiah(totalV)
		totalRow.Cells[4].Value = FormatRupiah(totalVI)
		totalRow.Cells[5].Value = FormatRupiah(totalVII)
		totalRow.Cells[6].Value = FormatRupiah(totalVIII)
	} else if len(rows) > 0 {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena ada baris yang kolom VII Jumlah-nya tidak tersedia; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form07RowKey mengidentifikasi satu baris Form 07.00 secara terbaca: tanggal
// pengambilalihan dan alamat agunan.
func form07RowKey(item domain.AYDAItem) string {
	return item.AcquisitionDate.Format("2006-01-02") + "|" + item.CollateralAddress
}
