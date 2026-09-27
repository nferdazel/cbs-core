package ojkreport

import "strings"

// form13.go membangun Form 13.00 "Daftar Simpanan dari Bank Lain" dari simpanan
// (tabungan/giro) dan deposito berjangka milik nasabah bergolongan bank.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 13.00 – 1 (kolom, PDF #page
// 207-208), Form 13.00 – 2 SANDI DAFTAR SIMPANAN DARI BANK LAIN (PDF #page 209-210),
// dan Form 13.00 – 3 PENJELASAN (PDF #page 211-212). Menurut penjelasannya (PDF
// #page 211), "Simpanan dari bank lain" adalah SEMUA liabilitas BPR berupa tabungan
// dan deposito kepada bank lain di Indonesia.
//
// Bank lawan dikenali lewat sandi Lampiran 02 Daftar Sandi Pihak Lawan
// (customers.ojk_pihak_lawan_code): 001 Bank Indonesia, 600 BPR, 601 BPRS, 700 Bank
// Umum, 701 Bank Umum Syariah, dan 901 Unit Usaha Syariah. Nasabah tanpa sandi bank
// tidak dapat dikenali dan tidak masuk form ini.
//
// Baris diagregasi per bank lawan (beserta kantor dan jenis simpanan) dalam SATU
// query sumber, bukan satu query per bank. Kolom yang tidak punya sumber per bank
// lawan dinyatakan belum tersedia dengan alasan, bukan diisi nol.

// form13Column adalah satu kolom Form 13.00; Reason != "" berarti belum tersedia.
type form13Column struct {
	Sandi  string
	Nama   string
	Reason string
	Value  func(BankDepositRow) string
}

const (
	form13SandiKantor    = "I"
	form13SandiIDPihak   = "II"
	form13SandiNoRek     = "III"
	form13SandiJenisBank = "IV"
	form13SandiSandiBank = "V"
	form13SandiLokasi    = "VI"
	form13SandiJenis     = "VII"
	form13SandiHubungan  = "VIII"
	form13SandiJangka    = "IX"
	form13SandiSukuBunga = "X"
	form13SandiNominal   = "XI"
	form13SandiDiblokir  = "XII"
	form13SandiAlasan    = "XIII"
	form13SandiBiaya     = "XIV"
	form13SandiJumlah    = "XV"
)

// form13Columns adalah susunan kolom Form 13.00 sesuai Form 13.00 – 1 (PDF #page
// 207-208). Kolom yang tidak punya sumber per bank lawan diisi Reason.
var form13Columns = []form13Column{
	{Sandi: form13SandiKantor, Nama: "Sandi Kantor", Value: func(r BankDepositRow) string {
		return dashIfEmpty(r.BranchCode)
	}},
	{Sandi: form13SandiIDPihak, Nama: "ID Pihak Lawan", Value: func(r BankDepositRow) string {
		// ID Pihak Lawan = CIF internal bank lawan (BAB II Lampiran II), bukan sandi OJK.
		return dashIfEmpty(r.CounterpartyCIF)
	}},
	{Sandi: form13SandiNoRek, Nama: "No. Rekening", Reason: "form ini diagregasi per bank lawan, sehingga nomor rekening unik per rekening tidak disajikan pada ekspor ini"},
	{Sandi: form13SandiJenisBank, Nama: "Jenis Bank", Value: func(r BankDepositRow) string {
		// Sandi kategori bank lawan, Lampiran 02 (PDF #page 303).
		return dashIfEmpty(r.JenisBankCode)
	}},
	{Sandi: form13SandiSandiBank, Nama: "Sandi Bank", Reason: "sandi bank 6 digit menurut Sistem Pelaporan OJK (APOLO/SPOJK) tidak diterbitkan di SEOJK 16/2024 dan belum ada sumbernya di repo"},
	{Sandi: form13SandiLokasi, Nama: "Lokasi Bank", Value: func(r BankDepositRow) string {
		// Sandi Kabupaten/Kota bank lawan, Lampiran 03 (PDF #page 209/211).
		return dashIfEmpty(r.LocationCode)
	}},
	{Sandi: form13SandiJenis, Nama: "Jenis", Value: func(r BankDepositRow) string {
		return sandiJenisSimpananBank(r.Jenis)
	}},
	{Sandi: form13SandiHubungan, Nama: "Hubungan dengan Bank", Value: func(r BankDepositRow) string {
		// Sandi inline Lampiran II Form 13.00-2 butir VIII (PDF #page 209): 12 terkait,
		// 20 tidak terkait. Diisi bank lewat SQL/seed.
		return dashIfEmpty(r.HubunganBankCode)
	}},
	{Sandi: form13SandiJangka, Nama: "Jangka Waktu", Reason: "tanggal mulai dan jatuh tempo berbeda per rekening; baris diagregasi per bank lawan"},
	{Sandi: form13SandiSukuBunga, Nama: "Suku Bunga", Reason: "suku bunga berbeda per rekening; baris diagregasi per bank lawan"},
	{Sandi: form13SandiNominal, Nama: "Nominal", Value: func(r BankDepositRow) string {
		return FormatRupiah(r.TotalNominal)
	}},
	{Sandi: form13SandiDiblokir, Nama: "Nominal yang Diblokir/Dijaminkan", Value: func(r BankDepositRow) string {
		return FormatRupiah(r.TotalBlocked)
	}},
	{Sandi: form13SandiAlasan, Nama: "Alasan Diblokir", Reason: "alasan pemblokiran belum disimpan per rekening simpanan bank lawan"},
	{Sandi: form13SandiBiaya, Nama: "Biaya Transaksi Belum Diamortisasi", Reason: "saldo biaya transaksi belum diamortisasi per bank lawan belum disimpan"},
	{Sandi: form13SandiJumlah, Nama: "Jumlah", Value: func(r BankDepositRow) string {
		// Form 13.00 – 3 butir XV: jumlah = nominal dikurangi biaya transaksi belum
		// diamortisasi. Karena belum ada saldo biaya transaksi tersimpan per bank lawan
		// (kolom XIV belum tersedia), nilai pengurangnya nol.
		return FormatRupiah(r.TotalNominal)
	}},
}

// buildForm13 menyusun Form 13.00 dari agregasi yang sudah dihitung sumber data.
// Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada baris,
// Rows kosong tetapi daftar kolom yang belum tersedia tetap dibawa.
func buildForm13(rows []BankDepositRow) TableSection {
	sec := TableSection{
		Form:     "13.00",
		Name:     formName("13.00"),
		KeyLabel: "Bank Lawan / Jenis Simpanan",
		Notes: []string{
			"Simpanan dari bank lain adalah seluruh liabilitas BPR berupa tabungan dan deposito kepada bank lain di Indonesia (Form 13.00 – 3, PDF #page 211).",
			"Bank lawan dikenali dari sandi Lampiran 02 Daftar Sandi Pihak Lawan (customers.ojk_pihak_lawan_code): 001 Bank Indonesia, 600 BPR, 601 BPRS, 700 Bank Umum, 701 Bank Umum Syariah, dan 901 Unit Usaha Syariah. Nasabah tanpa sandi bank tidak dikenali sebagai bank lawan.",
			"Baris diagregasi per bank lawan, kantor, dan jenis simpanan; sistem belum menyimpan riwayat saldo per akhir bulan, sehingga posisi yang dibaca adalah keadaan saat ekspor dijalankan.",
			"Kolom III (No. Rekening), IX (Jangka Waktu), dan X (Suku Bunga) tidak dapat disajikan pada baris agregat; kolom V (Sandi Bank) menunggu sandi APOLO/SPOJK, dan XIII/XIV belum punya sumber tersimpan.",
			"Kolom XV (Jumlah) ditulis sama dengan Nominal karena belum ada saldo biaya transaksi belum diamortisasi (kolom XIV) yang tersimpan per bank lawan.",
		},
	}
	for _, c := range form13Columns {
		if c.Reason != "" {
			sec.Unavailable = append(sec.Unavailable, ColumnUnavailable{Sandi: c.Sandi, Nama: c.Nama, Reason: c.Reason})
			continue
		}
		sec.Columns = append(sec.Columns, TableColumn{Sandi: c.Sandi, Nama: c.Nama})
	}
	for _, r := range rows {
		cif := dashIfEmpty(r.CounterpartyCIF)
		key := cif + " / " + sandiJenisSimpananBank(r.Jenis)
		// Kantor ikut menjadi bagian kunci karena satu bank lawan dapat dilayani
		// beberapa kantor; tanpa itu dua baris berbagi kunci yang sama.
		if strings.TrimSpace(r.BranchCode) != "" {
			key += " / " + r.BranchCode
		}
		row := TableRow{Key: key}
		for _, c := range form13Columns {
			if c.Reason != "" || c.Value == nil {
				continue
			}
			row.Cells = append(row.Cells, TableCell{Sandi: c.Sandi, Nama: c.Nama, Value: c.Value(r)})
		}
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// sandiJenisSimpananBank mengembalikan sandi Form 13.00 – 2 butir VII. Sumber hanya
// membedakan tabungan/giro ("01") dan deposito ("02"); nilai lain ditulis "-".
func sandiJenisSimpananBank(jenis string) string {
	switch strings.TrimSpace(jenis) {
	case "01", "02":
		return jenis
	default:
		return "-"
	}
}
