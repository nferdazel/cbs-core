package ojkreport

import "github.com/shopspring/decimal"

// form09.go membangun Form 09.00 "RINCIAN ASET LAINNYA" dari saldo COA yang dipetakan
// ke pos Aset Lainnya (COA bersandi 1299000000 pada Form 01.00).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 09.00 - 1 "RINCIAN ASET LAINNYA": PDF #page 187 (hlm. tercetak 135).
//     Kolom I Sandi Kantor, II Nama Rekening, III Sandi, IV Jumlah. Posisi resmi:
//       1299010000 Pendapatan Bunga yang Akan Diterima (jumlah dari pos a-d)
//         1299010100 a. Penempatan pada Bank Lain
//         1299010200 b. Kredit yang Diberikan
//         1299010300 c. Surat Berharga
//         1299010900 d. Lainnya
//       1299020000 Premi Penjaminan LPS Dibayar di Muka
//       1299030000 Uang Muka Pajak
//       1299040000 Aset Pajak Tangguhan
//       1299050000 Biaya Dibayar di Muka
//       1299060000 Tagihan kepada Perusahaan Asuransi
//       1299070000 Uang Muka untuk Kegiatan Operasional
//       1299990000 Lainnya
//   - Form 09.00 - 2 "PENJELASAN RINCIAN ASET LAINNYA": PDF #page 188 (hlm. 136).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Seluruh angka berasal dari saldo COA yang sama dengan Form 01.00 (sumber jurnal
//     ReportingRepository.BalanceSheet lewat Builder.collect), sehingga total Form 09.00
//     selalu sama dengan pos 1299000000 Form 01.00. Pemetaannya ada di COAMapping09Draft.
//   - Tiga pos anak 1299010100/1299010300/1299010900 belum punya akun COA; ditulis
//     sebagai baris tidak tersedia dengan alasan, bukan nol. Pos itu TIDAK diisi dari
//     Form 05.00/06.00 karena akan membuat total Form 09.00 berbeda dari Form 01.00.
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal pada bank_offices
//     (migrasi 000112); saldo COA-nya sendiri bank-wide. Aturan pemilihan ada di
//     selectReportingOffice (repo_source.go).

// form09Lines adalah susunan resmi Form 09.00. Sandi dan nama pos diambil apa adanya
// dari PDF #page 187; Reason diisi hanya untuk pos yang belum punya akun COA.
var form09Lines = []formLine{
	{Sandi: "1299010000", Name: "Pendapatan Bunga yang Akan Diterima", Level: 1,
		TotalFrom: plus("1299010100", "1299010200", "1299010300", "1299010900")},
	{Sandi: "1299010100", Name: "a. Penempatan pada Bank Lain", Level: 2,
		Reason: "belum ada akun COA untuk pendapatan bunga yang akan diterima dari penempatan pada bank lain; bagan akun belum memisahkan bunga penempatan dari pokok penempatan"},
	{Sandi: "1299010200", Name: "b. Kredit yang Diberikan", Level: 2},
	{Sandi: "1299010300", Name: "c. Surat Berharga", Level: 2,
		Reason: "belum ada akun COA untuk pendapatan bunga yang akan diterima dari surat berharga; bagan akun belum punya akun bunga surat berharga"},
	{Sandi: "1299010900", Name: "d. Lainnya", Level: 2,
		Reason: "belum ada akun COA untuk pendapatan bunga yang akan diterima dari aset produktif lain; bagan akun belum memisahkannya"},
	{Sandi: "1299020000", Name: "Premi Penjaminan LPS Dibayar di Muka", Level: 1},
	{Sandi: "1299030000", Name: "Uang Muka Pajak", Level: 1},
	{Sandi: "1299040000", Name: "Aset Pajak Tangguhan", Level: 1},
	{Sandi: "1299050000", Name: "Biaya Dibayar di Muka", Level: 1},
	{Sandi: "1299060000", Name: "Tagihan kepada Perusahaan Asuransi", Level: 1},
	{Sandi: "1299070000", Name: "Uang Muka untuk Kegiatan Operasional", Level: 1},
	{Sandi: "1299990000", Name: "Lainnya", Level: 1},
}

// buildForm09 menyusun tabel Form 09.00 dari saldo per sandi yang sudah dihitung
// (setelah deriveTotals mengisi baris 1299010000 dari pos anaknya) dan kantor pelapor
// kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data.
func buildForm09(amounts map[string]decimal.Decimal, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "09.00",
		Name:     formName("09.00"),
		KeyLabel: "Sandi Pos",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Nama Rekening"},
			{Sandi: "III", Nama: "Sandi"},
			{Sandi: "IV", Nama: "Jumlah"},
		},
		Notes: []string{
			"Seluruh angka berasal dari saldo COA yang sama dengan pos Aset Lainnya (1299000000) Form 01.00, sehingga total Form 09.00 selalu sama dengan pos itu; pemetaan ada di COAMapping09Draft.",
			"Pos 1299010100 (a. Penempatan pada Bank Lain), 1299010300 (c. Surat Berharga), dan 1299010900 (d. Lainnya) belum punya akun COA, jadi ditulis tidak tersedia, bukan nol. Pos itu tidak diisi dari Form 05.00/06.00 karena akan membuat total Form 09.00 berbeda dari Form 01.00.",
			"Pos 1299010000 (Pendapatan Bunga yang Akan Diterima) adalah jumlah dari pos anak a sampai d.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (hanya kantor AKTIF ber-sandi); bila jumlahnya bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}
	for _, l := range form09Lines {
		if l.Reason != "" {
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
		sec.Rows = append(sec.Rows, TableRow{
			Key: l.Sandi,
			Cells: []TableCell{
				{Sandi: "II", Nama: "Nama Rekening", Value: l.Name},
				{Sandi: "III", Nama: "Sandi", Value: l.Sandi},
				{Sandi: "IV", Nama: "Jumlah", Value: FormatRupiah(amounts[l.Sandi])},
			},
		})
	}
	pasangKolomSandiKantor(&sec, kantor)
	return sec
}
