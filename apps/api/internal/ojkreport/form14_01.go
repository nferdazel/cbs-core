package ojkreport

import "github.com/shopspring/decimal"

// form14_01.go membangun Form 14.01 "RINCIAN LIABILITAS LAINNYA - LAIN-LAIN" dari pos
// Lainnya Form 14.00, satu baris per akun COA.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 14.01 "Rincian Liabilitas Lainnya - Lain-lain": PDF #page 216 (hlm. tercetak
//     164); penjelasan PDF #page 217 (hlm. 165). Kolom I Sandi Kantor, II Uraian,
//     III Jumlah. Baris per-rincian bebas + baris JUMLAH.
//   - Pemicu (PDF #215 dan #217): pos Lainnya (2299990000) Form 14.00 dirinci pada
//     Form 14.01 bila melebihi 25% dari jumlah liabilitas lainnya.
//   - PDF #217 hanya menyebut kolom III "sebesar liabilitas BPR yang harus
//     diselesaikan" dan TIDAK menegaskan ikatan ke pos Lainnya Form 14.00 (berbeda dari
//     09.01 yang eksplisit, lihat docs/transkrip-form-a4-a5-a6.md butir Anomali 7).
//     Karena itu form ini hanya menjamin total = jumlah barisnya dan tidak mengklaim
//     tie-out wajib ke Form 14.00.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Baris = akun COA yang dipetakan ke pos Lainnya (COAMapping14Draft): saat ini
//     20700. Kolom II memakai nama akun dari baris neraca, kolom III saldo akun itu
//     pada periodEnd.
//   - Pemicu memakai penyebut pos 2299000000 Form 01.00 (jumlah liabilitas lainnya),
//     BUKAN jumlah baris Form 14.00: jumlah baris Form 14.00 hanya 20500+20700 sehingga
//     ambang 25% akan terlampaui palsu bila dipakai sebagai penyebut.
//   - Bila tidak terpicu, form memang tidak berlaku untuk periode itu dan tidak
//     di-append sama sekali (bukan "TIDAK DIBANGUN"), karena ketidaktepatan itu aturan,
//     bukan kegagalan data.
//   - Bila terpicu tetapi ada akun yang belum punya saldo, barisnya ditulis "-"
//     beserta alasan dan baris JUMLAH tidak dihitung (pola form14.go).

// form14_01TotalKey adalah kunci baris non-sandi untuk baris JUMLAH; bukan sandi OJK.
const form14_01TotalKey = "JUMLAH"

// buildForm14_01 menyusun tabel Form 14.01 dari rincian per akun dan kantor pelapor
// kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data.
func buildForm14_01(rows []rincianLainnya, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "14.01",
		Name:     formName("14.01"),
		KeyLabel: "Sandi Akun",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Uraian"},
			{Sandi: "III", Nama: "Jumlah"},
		},
		Notes: []string{
			"Form ini KONDISIONAL: hanya terbit bila pos Lainnya (2299990000) Form 14.00 melebihi 25% dari jumlah liabilitas lainnya (pos 2299000000 Form 01.00, PDF #215, #217). Bila tidak, form memang tidak berlaku untuk periode itu dan tidak dituliskan sebagai \"tidak dibangun\".",
			"Satu baris = satu akun COA yang dipetakan ke pos Lainnya pada Form 14.00 (COAMapping14Draft); kolom II Uraian memakai nama akun dan kolom III memuat saldo akun itu pada akhir periode (periodEnd).",
			"Kolom III Jumlah adalah jumlah baris rincian di atas. PDF #page 217 hanya menyebut nilainya \"sebesar liabilitas BPR yang harus diselesaikan\" dan TIDAK menegaskan ikatan wajib ke pos Lainnya (2299990000) Form 14.00; ikatan itu karena itu tidak diklaim di sini.",
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
		Key: form14_01TotalKey,
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
