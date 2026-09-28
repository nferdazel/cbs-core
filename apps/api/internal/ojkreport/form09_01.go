package ojkreport

import "github.com/shopspring/decimal"

// form09_01.go membangun Form 09.01 "RINCIAN ASET LAINNYA - LAIN-LAIN" dari pos
// Lainnya Form 09.00, satu baris per akun COA.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 09.01 "Rincian Aset Lainnya - Lain-lain": PDF #page 189 (hlm. tercetak 137);
//     penjelasan PDF #page 190 (hlm. 138). Kolom I Sandi Kantor, II Uraian,
//     III Jumlah. Baris per-rincian bebas + baris JUMLAH.
//   - Pemicu (PDF #188 dan #190): pos Lainnya (1299990000) Form 09.00 dirinci pada
//     Form 09.01 bila melebihi 25% dari jumlah aset lainnya.
//   - Syarat numerik (PDF #190): kolom III Jumlah "harus sama dengan jumlah pos
//     Lainnya pada Form 09.00 - Rincian Aset Lainnya".
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Baris = akun COA yang dipetakan ke pos Lainnya (COAMapping09Draft): saat ini
//     10305, 10999, dan 11700. Kolom II memakai nama akun dari baris neraca, kolom III
//     saldo akun itu pada periodEnd. Tidak ada register "uraian" baru; granularitasnya
//     adalah akun COA.
//   - Pemicu memakai penyebut pos 1299000000 Form 01.00 (jumlah aset lainnya), BUKAN
//     banyaknya baris Form 09.00 yang terisi: tiga pos anak 09.00 ber-Reason sehingga
//     jumlah baris terisi tidak sama dengan jumlah aset lainnya.
//   - Bila tidak terpicu, form memang tidak berlaku untuk periode itu dan tidak
//     di-append sama sekali (bukan "TIDAK DIBANGUN"), karena ketidaktepatan itu aturan,
//     bukan kegagalan data.
//   - Bila terpicu tetapi ada akun yang belum punya saldo, barisnya ditulis "-"
//     beserta alasan dan baris JUMLAH tidak dihitung (pola form09.go/form14.go).

// form09_01TotalKey adalah kunci baris non-sandi untuk baris JUMLAH; bukan sandi OJK.
const form09_01TotalKey = "JUMLAH"

// melebihi25Persen melaporkan apakah nilai melebihi 25% pembanding. Perkalian silang
// dipakai agar ambang persis 25% TIDAK dianggap melebihi (harus >, bukan >=) tanpa
// pembulatan desimal.
func melebihi25Persen(nilai, pembanding decimal.Decimal) bool {
	return nilai.Mul(decimal.NewFromInt(100)).GreaterThan(pembanding.Mul(decimal.NewFromInt(25)))
}

// buildForm09_01 menyusun tabel Form 09.01 dari rincian per akun dan kantor pelapor
// kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data.
func buildForm09_01(rows []rincianLainnya, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "09.01",
		Name:     formName("09.01"),
		KeyLabel: "Sandi Akun",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Uraian"},
			{Sandi: "III", Nama: "Jumlah"},
		},
		Notes: []string{
			"Form ini KONDISIONAL: hanya terbit bila pos Lainnya (1299990000) Form 09.00 melebihi 25% dari jumlah aset lainnya (pos 1299000000 Form 01.00, PDF #188, #190). Bila tidak, form memang tidak berlaku untuk periode itu dan tidak dituliskan sebagai \"tidak dibangun\".",
			"Satu baris = satu akun COA yang dipetakan ke pos Lainnya pada Form 09.00 (COAMapping09Draft); kolom II Uraian memakai nama akun dan kolom III memuat saldo akun itu pada akhir periode (periodEnd).",
			"Kolom III Jumlah harus sama dengan pos Lainnya (1299990000) Form 09.00 (PDF #190); keduanya membaca saldo akun yang sama.",
			"Akun yang dipetakan ke Lainnya tetapi belum punya saldo pada periodEnd ditulis \"-\" beserta alasannya, bukan nol; baris JUMLAH hanya diisi bila seluruh baris bernilai.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112); bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	total := decimal.Zero
	lengkap := true
	for _, r := range rows {
		if !r.Tersedia {
			lengkap = false
			sec.Rows = append(sec.Rows, TableRow{
				Key:    r.COACode,
				Reason: "akun " + r.COACode + " dipetakan ke pos Lainnya tetapi belum punya saldo pada akhir periode; nama dan jumlahnya ditulis \"-\", bukan nol",
				Cells: []TableCell{
					{Sandi: "II", Nama: "Uraian", Value: r.COACode},
					{Sandi: "III", Nama: "Jumlah", Value: "-"},
				},
			})
			continue
		}
		total = total.Add(r.Amount)
		sec.Rows = append(sec.Rows, TableRow{
			Key: r.COACode,
			Cells: []TableCell{
				{Sandi: "II", Nama: "Uraian", Value: r.Name},
				{Sandi: "III", Nama: "Jumlah", Value: FormatRupiah(r.Amount)},
			},
		})
	}

	totalRow := TableRow{
		Key: form09_01TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Uraian", Value: "JUMLAH"},
			{Sandi: "III", Nama: "Jumlah", Value: "-"},
		},
	}
	if lengkap {
		totalRow.Cells[1].Value = FormatRupiah(total)
	} else {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena masih ada akun yang belum punya saldo; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}
