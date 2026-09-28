package ojkreport

import "github.com/shopspring/decimal"

// form10.go membangun Form 10.00 "RINCIAN LIABILITAS SEGERA" dari saldo COA yang
// dipetakan ke pos Liabilitas Segera (COA bersandi 2101000000 pada Form 01.00).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 10.00 "RINCIAN LIABILITAS SEGERA": PDF #page 191 (hlm. tercetak 139).
//     Kolom I Sandi Kantor, II Nama Rekening, III Sandi, IV Jumlah. Baris tetap:
//       2101010000 Liabilitas kepada Pemerintah yang Harus Dibayar
//       2101020000 Sanksi Liabilitas Membayar kepada Otoritas yang Belum Dibayarkan
//       2101030000 Titipan Nasabah
//       2101040000 Kredit yang Diberikan Bersaldo Kredit
//       2101050000 Dividen yang Belum Dibayarkan
//       2101060000 Selisih Lebih Hasil Penjualan Agunan Milik Nasabah
//       2101070000 Imbalan Kerja
//       2101990000 Lainnya
//       JUMLAH
//     Seluruh pos diisi dalam rupiah penuh. Tidak ada aturan redirect ke form lain
//     (berbeda dari Form 09.00/14.00) dan tidak ada aturan "Desember".
//   - Penjelasan pos: PDF #page 192 (hlm. tercetak 140); posisi keuangan: PDF #page 106
//     (cetak 54) yang menyatakan pos Liabilitas Segera dirinci pada Form 10.00.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Seluruh angka berasal dari saldo COA pada akhir periode (periodEnd) yang sama
//     dengan Form 01.00 (sumber jurnal ReportingRepository.BalanceSheet lewat
//     Builder.collect). Pemetaannya ada di COAMapping10Draft.
//   - Pos 2101010000 (pajak) SENGAJA ditulis "-": definisi PDF #page 192 adalah pajak
//     "untuk periode sebelum bulan laporan yang dibayarkan pada bulan laporan", yaitu
//     ARUS pembayaran pajak bulan berjalan, sedangkan yang tersedia hanyalah SALDO akun
//     20500 Utang Pajak pada periodEnd. Saldo itu bukan makna pembayaran bulan berjalan
//     sehingga tidak dipaksa ke pos ini; akun 20500 sudah dilaporkan sebagai Utang Pajak
//     (2299020000) Form 14.00.
//   - Pos yang belum punya akun COA ditulis sebagai baris tidak tersedia beserta
//     alasannya, bukan nol. Karena itu baris JUMLAH pun hanya diisi bila seluruh pos
//     sudah punya akun; bila belum, JUMLAH ditulis "-" agar angka sebagian tidak
//     disalahartikan sebagai total.
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal pada bank_offices
//     (migrasi 000112); saldo COA-nya sendiri bank-wide. Aturan pemilihan ada di
//     selectReportingOffice (repo_source.go).

// form10TotalKey adalah kunci baris non-sandi untuk baris JUMLAH; bukan sandi pos OJK.
const form10TotalKey = "JUMLAH"

// form10Lines adalah susunan resmi Form 10.00. Sandi dan nama pos diambil apa adanya
// dari PDF #page 191; Reason diisi hanya untuk pos yang belum punya akun COA atau yang
// definisinya tidak dapat dipenuhi sumber saldo periodEnd.
var form10Lines = []formLine{
	{Sandi: "2101010000", Name: "Liabilitas kepada Pemerintah yang Harus Dibayar",
		Reason: "definisi PDF #page 192 adalah pajak \"untuk periode sebelum bulan laporan yang dibayarkan pada bulan laporan\" (arus pembayaran bulan berjalan), sedangkan yang tersedia hanya saldo COA 20500 Utang Pajak pada periodEnd; saldo itu bukan makna pembayaran bulan berjalan dan akun 20500 sudah dilaporkan sebagai Utang Pajak (2299020000) Form 14.00"},
	{Sandi: "2101020000", Name: "Sanksi Liabilitas Membayar kepada Otoritas yang Belum Dibayarkan",
		Reason: "belum ada akun COA untuk sanksi administratif/denda otoritas yang sudah disampaikan via surat pemberitahuan namun belum dibayar; bagan akun belum memisahkan sanksi dari utang lain"},
	{Sandi: "2101030000", Name: "Titipan Nasabah",
		Reason: "belum ada akun COA titipan nasabah (pengurusan asuransi, biaya notaris, kiriman uang, setoran tak teridentifikasi); bagan akun belum memisahkannya dari simpanan"},
	{Sandi: "2101040000", Name: "Kredit yang Diberikan Bersaldo Kredit",
		Reason: "belum ada akun COA kredit bersaldo kredit akibat kelebihan pembayaran pelunasan; kredit tercatat pada akun aset 10300/10301 dan baris kredit tidak menyimpan saldo kredit"},
	{Sandi: "2101050000", Name: "Dividen yang Belum Dibayarkan",
		Reason: "belum ada akun COA dividen yang belum dibayarkan; bagan akun belum menyimpannya"},
	{Sandi: "2101060000", Name: "Selisih Lebih Hasil Penjualan Agunan Milik Nasabah",
		Reason: "belum ada akun COA selisih lebih hasil penjualan agunan milik nasabah; akun agunan yang diambil alih (10500) tidak menyimpan hasil penjualannya"},
	{Sandi: "2101070000", Name: "Imbalan Kerja",
		Reason: "belum ada akun COA imbalan kerja; bagan akun belum menyimpannya"},
	{Sandi: "2101990000", Name: "Lainnya"},
}

// buildForm10 menyusun tabel Form 10.00 dari saldo per sandi yang sudah dihitung dan
// kantor pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data.
//
// Baris JUMLAH hanya diisi bila seluruh pos punya akun COA; bila ada pos yang belum
// punya akun, JUMLAH ditulis "-" karena total sebagian akan menyesatkan.
func buildForm10(amounts map[string]decimal.Decimal, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "10.00",
		Name:     formName("10.00"),
		KeyLabel: "Sandi Pos",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Nama Rekening"},
			{Sandi: "III", Nama: "Sandi"},
			{Sandi: "IV", Nama: "Jumlah"},
		},
		Notes: []string{
			"Seluruh angka berasal dari saldo COA pada akhir periode (periodEnd) yang sama dengan Form 01.00; pemetaan ada di COAMapping10Draft (DRAF, belum terverifikasi bank).",
			"Pos 2101010000 Liabilitas kepada Pemerintah yang Harus Dibayar ditulis \"-\": definisi PDF #page 192 adalah pajak untuk periode sebelum bulan laporan yang DIBAYARKAN pada bulan laporan (arus), sedangkan sumber yang tersedia hanyalah saldo Utang Pajak (20500) pada periodEnd; saldo itu sudah dilaporkan sebagai Utang Pajak (2299020000) Form 14.00 sehingga tidak diulang di sini.",
			"Pos 2101040000 Kredit yang Diberikan Bersaldo Kredit khusus kredit bersaldo kredit akibat kelebihan pembayaran pelunasan (PDF #page 192); baris kredit pada sumber tidak menyimpan saldo kredit, jadi pos ditulis belum tersedia, bukan nol.",
			"Akun 20400 Bunga Deposito yang Masih Harus Dibayar, 20600 Utang Bunga, dan 12400 Bagi Hasil Masih Harus Dibayar dipetakan ke Lainnya (2101990000) karena bagan akun tidak punya pos resmi yang lebih spesifik; akun 20500 Utang Pajak sengaja TIDAK dipetakan (lihat pos 2101010000).",
			"Pos yang belum punya akun COA: 2101010000 (pajak, alasan di atas), 2101020000, 2101030000, 2101040000, 2101050000, 2101060000, dan 2101070000. Semuanya ditulis \"-\" beserta alasannya, bukan nol. Karena itu baris JUMLAH hanya diisi bila seluruh pos sudah punya akun; bila belum, JUMLAH ditulis \"-\" karena angka sebagian akan menyesatkan.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112); bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	total := decimal.Zero
	lengkap := true
	for _, l := range form10Lines {
		if l.Reason != "" {
			lengkap = false
			// Baris tetap menampilkan sandi dan namanya, kolom Jumlah ditulis "-",
			// disertai alasan konkret ketiadaan akun COA / ketidakcocokan definisi.
			sec.Rows = append(sec.Rows, TableRow{
				Key:    l.Sandi,
				Reason: l.Name + ": " + l.Reason,
				Cells: []TableCell{
					{Sandi: "II", Nama: "Nama Rekening", Value: l.Name},
					{Sandi: "III", Nama: "Sandi", Value: l.Sandi},
					{Sandi: "IV", Nama: "Jumlah", Value: "-"},
				},
			})
			continue
		}
		nilai := amounts[l.Sandi]
		total = total.Add(nilai)
		sec.Rows = append(sec.Rows, TableRow{
			Key: l.Sandi,
			Cells: []TableCell{
				{Sandi: "II", Nama: "Nama Rekening", Value: l.Name},
				{Sandi: "III", Nama: "Sandi", Value: l.Sandi},
				{Sandi: "IV", Nama: "Jumlah", Value: FormatRupiah(nilai)},
			},
		})
	}

	totalRow := TableRow{
		Key: form10TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Nama Rekening", Value: "JUMLAH"},
			{Sandi: "III", Nama: "Sandi", Value: "-"},
			{Sandi: "IV", Nama: "Jumlah", Value: "-"},
		},
	}
	if lengkap {
		totalRow.Cells[2].Value = FormatRupiah(total)
	} else {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena masih ada pos yang belum punya akun COA; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}
