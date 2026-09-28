package ojkreport

import "github.com/shopspring/decimal"

// form14.go membangun Form 14.00 "RINCIAN LIABILITAS LAINNYA" dari saldo COA yang
// dipetakan ke pos Liabilitas Lainnya (COA bersandi 2299000000 pada Form 01.00).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 14.00 "RINCIAN LIABILITAS LAINNYA": PDF #page 213 (hlm. tercetak 161).
//     Kolom I Sandi Kantor, II Nama Rekening, III Sandi, IV Jumlah.
//     Posisi resmi (baris tetap):
//       2299010100 Tabungan Berjangka
//       2299010201 Deposito a. Sudah Jatuh Tempo
//       2299010202 Deposito b. Belum Jatuh Tempo
//       2299010301 Simpanan dari Bank lain a. Sudah Jatuh Tempo
//       2299010302 Simpanan dari Bank lain b. Belum Jatuh Tempo
//       2299010401 Pinjaman yang Diterima dari Bank a. Sudah Jatuh Tempo
//       2299010402 Pinjaman yang Diterima dari Bank b. Belum Jatuh Tempo
//       2299010501 Pinjaman yang Diterima dari Pihak Ketiga Bukan Bank a. Sudah Jatuh Tempo
//       2299010502 Pinjaman yang Diterima dari Pihak Ketiga Bukan Bank b. Belum Jatuh Tempo
//       2299019900 Utang Bunga Lainnya
//       2299020000 Utang Pajak
//       2299030000 Liabilitas Imbalan Kerja
//       2299040000 Liabilitas Sewa Pembiayaan
//       2299050000 Taksiran Pajak Penghasilan
//       2299060000 Pendapatan yang Ditangguhkan
//       2299070000 Liabilitas Pajak Tangguhan
//       2299990000 Lainnya
//     "Utang Bunga" pada PDF #page 213 hanyalah header grup tanpa baris sandi sendiri;
//     yang dilaporkan adalah komponen 22990101xx-22990199xx.
//   - Penjelasan pos: PDF #page 214-215 (hlm. tercetak 162-163).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Seluruh angka berasal dari saldo COA pada akhir periode (periodEnd) yang sama
//     dengan Form 01.00 (sumber jurnal ReportingRepository.BalanceSheet lewat
//     Builder.collect). Pemetaannya ada di COAMapping14Draft.
//   - Pos yang belum punya akun COA ditulis sebagai baris tidak tersedia beserta
//     alasannya, bukan nol. Karena itu baris JUMLAH pun hanya diisi bila seluruh pos
//     sudah punya akun; bila belum, JUMLAH ditulis "-" agar angka sebagian tidak
//     disalahartikan sebagai total.
//   - Aturan redirect PDF #page 215: bila pos Lainnya (2299990000) melebihi 25% dari
//     jumlah liabilitas lainnya, pos itu wajib dirinci pada Form 14.01. Form 14.01
//     belum dibangun; aturannya dicatat pada Notes Form 14.00.
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal pada bank_offices
//     (migrasi 000112); saldo COA-nya sendiri bank-wide. Aturan pemilihan ada di
//     selectReportingOffice (repo_source.go).

// form14HeaderKey dan form14TotalKey adalah kunci baris non-sandi untuk header grup
// "Utang Bunga" (PDF #page 213) dan baris JUMLAH; keduanya bukan sandi pos OJK.
const (
	form14HeaderKey = "UTANG_BUNGA"
	form14TotalKey  = "JUMLAH"
)

// form14Lines adalah susunan resmi Form 14.00. Sandi dan nama pos diambil apa adanya
// dari PDF #page 213; Reason diisi hanya untuk pos yang belum punya akun COA.
var form14Lines = []formLine{
	{Sandi: "2299010100", Name: "Tabungan Berjangka",
		Reason: "belum ada akun COA untuk akrual bunga tabungan berjangka; bagan akun belum memisahkan bunga tabungan dari pokoknya (20100 Tabungan)"},
	{Sandi: "2299010201", Name: "Deposito a. Sudah Jatuh Tempo",
		Reason: "belum ada akun COA yang memisahkan bunga deposito sudah jatuh tempo; akun 20400 hanya mencatat bunga deposito yang masih harus dibayar tanpa status jatuh tempo"},
	{Sandi: "2299010202", Name: "Deposito b. Belum Jatuh Tempo",
		Reason: "belum ada akun COA yang memisahkan akrual bunga deposito belum jatuh tempo; akun 20400 tidak menyimpan status jatuh tempo"},
	{Sandi: "2299010301", Name: "Simpanan dari Bank lain a. Sudah Jatuh Tempo",
		Reason: "belum ada akun COA bunga simpanan dari bank lain; bagan akun hanya punya liabilitas pokoknya (2103010000 Simpanan dari Bank Lain)"},
	{Sandi: "2299010302", Name: "Simpanan dari Bank lain b. Belum Jatuh Tempo",
		Reason: "belum ada akun COA akrual bunga simpanan dari bank lain; bagan akun belum memisahkannya"},
	{Sandi: "2299010401", Name: "Pinjaman yang Diterima dari Bank a. Sudah Jatuh Tempo",
		Reason: "belum ada akun COA bunga pinjaman yang diterima dari bank; bagan akun belum memisahkan bunga pinjaman"},
	{Sandi: "2299010402", Name: "Pinjaman yang Diterima dari Bank b. Belum Jatuh Tempo",
		Reason: "belum ada akun COA akrual bunga pinjaman yang diterima dari bank"},
	{Sandi: "2299010501", Name: "Pinjaman yang Diterima dari Pihak Ketiga Bukan Bank a. Sudah Jatuh Tempo",
		Reason: "belum ada akun COA bunga pinjaman yang diterima dari pihak ketiga bukan bank"},
	{Sandi: "2299010502", Name: "Pinjaman yang Diterima dari Pihak Ketiga Bukan Bank b. Belum Jatuh Tempo",
		Reason: "belum ada akun COA akrual bunga pinjaman yang diterima dari pihak ketiga bukan bank"},
	{Sandi: "2299019900", Name: "Utang Bunga Lainnya",
		Reason: "belum ada akun COA utang bunga di luar bunga nasabah; akun 20600 hanya agregat Utang Bunga dan tidak dapat dirinci tanpa status jatuh tempo"},
	{Sandi: "2299020000", Name: "Utang Pajak"},
	{Sandi: "2299030000", Name: "Liabilitas Imbalan Kerja",
		Reason: "belum ada akun COA liabilitas imbalan kerja; bagan akun belum menyimpannya"},
	{Sandi: "2299040000", Name: "Liabilitas Sewa Pembiayaan",
		Reason: "belum ada akun COA liabilitas sewa pembiayaan"},
	{Sandi: "2299050000", Name: "Taksiran Pajak Penghasilan",
		Reason: "belum ada akun COA taksiran PPh; akun 60100 hanya beban pajak penghasilan, bukan taksiran utang pajaknya"},
	{Sandi: "2299060000", Name: "Pendapatan yang Ditangguhkan",
		Reason: "belum ada akun COA pendapatan yang ditangguhkan; pendapatan bunga ditangguhkan dalam rangka restrukturisasi pun tidak termasuk pos ini (PDF #page 215)"},
	{Sandi: "2299070000", Name: "Liabilitas Pajak Tangguhan",
		Reason: "belum ada akun COA liabilitas pajak tangguhan; akun 10330 hanya aset pajak tangguhan"},
	{Sandi: "2299990000", Name: "Lainnya"},
}

// buildForm14 menyusun tabel Form 14.00 dari saldo per sandi yang sudah dihitung dan
// kantor pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data.
//
// Header grup "Utang Bunga" ditulis sebagai baris tanpa sandi (kolom III "-"), dan baris
// JUMLAH hanya diisi bila seluruh pos punya akun COA; bila ada pos yang belum punya akun,
// JUMLAH ditulis "-" dengan alasan karena total sebagian akan menyesatkan.
func buildForm14(amounts map[string]decimal.Decimal, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "14.00",
		Name:     formName("14.00"),
		KeyLabel: "Sandi Pos",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Nama Rekening"},
			{Sandi: "III", Nama: "Sandi"},
			{Sandi: "IV", Nama: "Jumlah"},
		},
		Notes: []string{
			"Seluruh angka berasal dari saldo COA pada akhir periode (periodEnd) yang sama dengan Form 01.00; pemetaan ada di COAMapping14Draft (DRAF, belum terverifikasi bank).",
			"Pos Utang Bunga hanyalah header grup tanpa sandi sendiri; yang dilaporkan adalah komponen 2299010100 sampai 2299019900 (PDF #page 213).",
			"Pos 2299020000 Utang Pajak diisi akun 20500 yang pada Form 01.00 masih dipetakan ke Liabilitas Segera (2101000000); sampai pedoman konversi bank menetapkan tempatnya, angka ini dapat muncul pada dua form.",
			"Pos yang belum punya akun COA ditulis \"-\" beserta alasannya, bukan nol. Karena itu baris JUMLAH hanya diisi bila seluruh pos sudah punya akun; bila belum, JUMLAH ditulis \"-\" karena angka sebagian akan menyesatkan.",
			"Aturan PDF #page 215: bila jumlah pos Lainnya (2299990000) melebihi 25% dari jumlah liabilitas lainnya, pos itu wajib dirinci pada Form 14.01 - Rincian Liabilitas Lainnya - Lain-lain. Form 14.01 belum dibangun.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112); bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	// Header grup "Utang Bunga": tidak punya sandi sendiri (PDF #page 213).
	sec.Rows = append(sec.Rows, TableRow{
		Key: form14HeaderKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Nama Rekening", Value: "Utang Bunga"},
			{Sandi: "III", Nama: "Sandi", Value: "-"},
			{Sandi: "IV", Nama: "Jumlah", Value: "-"},
		},
	})

	total := decimal.Zero
	lengkap := true
	for _, l := range form14Lines {
		if l.Reason != "" {
			lengkap = false
			// Baris tetap menampilkan sandi dan namanya, kolom Jumlah ditulis "-",
			// disertai alasan konkret ketiadaan akun COA.
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
		Key: form14TotalKey,
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
