package ojkreport

import (
	"strings"

	"github.com/shopspring/decimal"
)

// form11.go membangun Form 11.00 "Daftar Tabungan" dari baris per rekening tabungan.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 11.00 - 1/-2 kolom dan sandi: PDF #page 193-196 (hlm. tercetak 141-144).
//     Kolom I Sandi Kantor, II ID Pihak Lawan, III No. Rekening, IV Jenis, V Hubungan
//     dengan Bank, VI Golongan Nasabah, VII Lokasi Nasabah, VIII Jangka Waktu (Tanggal
//     Mulai/Tanggal Jatuh Tempo), IX Suku Bunga, X Nominal, XI Nominal yang
//     Diblokir/Dijaminkan, XII Alasan Diblokir, XIII Biaya Transaksi Belum
//     Diamortisasi, XIV Jumlah, XV Nomor Identitas, XVI Jenis PEP, XVII Risiko
//     Nasabah, XVIII Status Data.
//   - Form 11.00 - 3 penjelasan: PDF #page 197-199 (hlm. 145-147). Kolom X Nominal
//     adalah "jumlah saldo tabungan pada akhir bulan laporan"; kolom XIV Jumlah adalah
//     "nominal dikurangi biaya transaksi yang belum diamortisasi".
//
// BATAS SUMBER - jangan diisi tebakan:
//   - Baris = satu rekening tabungan unik (PDF #page 197). Posisi yang dibaca adalah
//     AKHIR PERIODE laporan; saldo direkonstruksi dari jurnal.
//   - Kolom I Sandi Kantor memakai kantor pelapor tunggal (bank_offices) yang sama
//     dengan Form 09.00/01.01 lewat pasangKolomSandiKantor.
//   - Kolom XV Nomor Identitas TIDAK dipaparkan: NIK/NPWP nasabah tersimpan
//     terenkripsi dan sengaja tidak dibuka sebagai keluaran laporan (keputusan privasi;
//     lihat form06.go kolom III dan docs/KEPUTUSAN-OJK.md butir 4).
//   - Kolom IV Jenis, VIII Jangka Waktu, XI Nominal Diblokir/Dijaminkan, XII Alasan
//     Diblokir, XIII Biaya Transaksi Belum Diamortisasi, XVI Jenis PEP, XVII Risiko
//     Nasabah, dan XVIII Status Data belum punya sumber tersimpan; ditulis "-" beserta
//     alasan, bukan nol.

// form11Column adalah satu kolom Form 11.00; Reason != "" berarti belum tersedia.
type form11Column struct {
	Sandi  string
	Nama   string
	Reason string
	Value  func(SavingsAccountRow) string
}

const (
	form11SandiKantor     = "I"
	form11SandiIDPihak    = "II"
	form11SandiNoRek      = "III"
	form11SandiJenis      = "IV"
	form11SandiHubungan   = "V"
	form11SandiGolongan   = "VI"
	form11SandiLokasi     = "VII"
	form11SandiJangka     = "VIII"
	form11SandiSukuBunga  = "IX"
	form11SandiNominal    = "X"
	form11SandiDiblokir   = "XI"
	form11SandiAlasan     = "XII"
	form11SandiBiaya      = "XIII"
	form11SandiJumlah     = "XIV"
	form11SandiNomorID    = "XV"
	form11SandiPEP        = "XVI"
	form11SandiRisiko     = "XVII"
	form11SandiStatusData = "XVIII"
)

// form11Columns adalah susunan kolom Form 11.00. Kolom I disisipkan
// pasangKolomSandiKantor; daftar ini mulai dari kolom II.
var form11Columns = []form11Column{
	{Sandi: form11SandiIDPihak, Nama: "ID Pihak Lawan", Value: func(r SavingsAccountRow) string {
		// ID Pihak Lawan = nomor CIF internal nasabah (BAB II Lampiran II), bukan sandi OJK.
		return dashIfEmpty(r.CounterpartyCIF)
	}},
	{Sandi: form11SandiNoRek, Nama: "No. Rekening", Value: func(r SavingsAccountRow) string {
		return dashIfEmpty(r.AccountNumber)
	}},
	{Sandi: form11SandiJenis, Nama: "Jenis", Reason: "banking_products tidak menyimpan penanda eksplisit tabungan dapat ditarik sewaktu-waktu vs tabungan berjangka; jenis tidak diturunkan dari tenor produk agar tidak menebak"},
	{Sandi: form11SandiHubungan, Nama: "Hubungan dengan Bank", Value: func(r SavingsAccountRow) string {
		// Sandi inline Lampiran II Form 11.00 (12 terkait, 20 tidak terkait) per nasabah.
		return dashIfEmpty(r.HubunganBankCode)
	}},
	{Sandi: form11SandiGolongan, Nama: "Golongan Nasabah", Value: func(r SavingsAccountRow) string {
		// Sandi Lampiran 02 - Daftar Sandi Pihak Lawan (customers.ojk_pihak_lawan_code).
		return dashIfEmpty(r.CustomerTypeCode)
	}},
	{Sandi: form11SandiLokasi, Nama: "Lokasi Nasabah", Value: func(r SavingsAccountRow) string {
		// Sandi Lampiran 03 - Daftar Sandi Kabupaten/Kota (customers.ojk_kabupaten_code).
		return dashIfEmpty(r.LocationCode)
	}},
	{Sandi: form11SandiJangka, Nama: "Jangka Waktu", Reason: "tabungan tidak memiliki jatuh tempo dan tanggal mulai perjanjian tidak disimpan terpisah dari waktu rekening dibuat; kedua sub-tanggal dikosongkan"},
	{Sandi: form11SandiSukuBunga, Nama: "Suku Bunga", Value: func(r SavingsAccountRow) string {
		// Hanya produk berbunga (INTEREST) yang dinyatakan sebagai persen; nisbah
		// bagi hasil syariah bukan suku bunga sehingga ditulis "-".
		return sukuBungaPersen(r.ProfitScheme, r.InterestRateAnnual)
	}},
	{Sandi: form11SandiNominal, Nama: "Nominal", Value: func(r SavingsAccountRow) string {
		// Saldo tabungan pada akhir bulan laporan (PDF #page 197).
		return FormatRupiah(r.Balance)
	}},
	{Sandi: form11SandiDiblokir, Nama: "Nominal yang Diblokir/Dijaminkan", Reason: "accounts.hold_balance hanya menyimpan keadaan saat ini; riwayat dana diblokir per akhir periode tidak tersimpan sehingga tidak direkonstruksi (bukan ditulis nol)"},
	{Sandi: form11SandiAlasan, Nama: "Alasan Diblokir", Reason: "alasan pemblokiran dana tidak disimpan per rekening tabungan"},
	{Sandi: form11SandiBiaya, Nama: "Biaya Transaksi Belum Diamortisasi", Reason: "saldo biaya transaksi belum diamortisasi per rekening tabungan belum disimpan; jadwal amortisasinya belum dimodelkan"},
	{Sandi: form11SandiJumlah, Nama: "Jumlah", Value: func(r SavingsAccountRow) string {
		// Form 11.00 - 3 (PDF #page 198): Jumlah = Nominal - Biaya Transaksi Belum
		// Diamortisasi. Karena biaya transaksi (kolom XIII) belum tersimpan, nilai
		// pengurangnya nol; keterbatasan dicatat pada Notes.
		return FormatRupiah(r.Balance)
	}},
	{Sandi: form11SandiNomorID, Nama: "Nomor Identitas", Reason: "NIK/NPWP nasabah tersimpan terenkripsi untuk dokumen dan sengaja tidak dibuka sebagai keluaran laporan (keputusan privasi)"},
	{Sandi: form11SandiPEP, Nama: "Jenis PEP", Reason: "status PEP per nasabah belum disimpan"},
	{Sandi: form11SandiRisiko, Nama: "Risiko Nasabah", Reason: "profil risiko nasabah belum disimpan"},
	{Sandi: form11SandiStatusData, Nama: "Status Data", Reason: "penanda pengkinian data bulan berjalan belum disimpan"},
}

// buildForm11 menyusun Form 11.00 dari baris per rekening tabungan dan kantor pelapor.
// Fungsi ini murni sehingga dapat diuji tanpa basis data. Kantor pelapor dipasang
// sebagai kolom I; bila tidak dapat ditentukan, kolom I dinyatakan tidak tersedia.
func buildForm11(rows []SavingsAccountRow, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "11.00",
		Name:     formName("11.00"),
		KeyLabel: "No. Rekening",
		Notes: []string{
			"Baris dibangun per rekening tabungan unik (Form 11.00 - 3, PDF #page 197); posisi yang dibaca adalah AKHIR PERIODE laporan, bukan keadaan saat ekspor dijalankan. Saldo direkonstruksi dari jurnal (entry_date <= akhir periode).",
			"Kolom II ID Pihak Lawan memakai nomor CIF internal nasabah (BAB II Lampiran II), bukan sandi OJK.",
			"Kolom VI Golongan Nasabah memakai sandi Lampiran 02 dan kolom VII Lokasi Nasabah memakai sandi Lampiran 03 (customers.ojk_pihak_lawan_code / ojk_kabupaten_code); sandi yang belum diisi ditulis '-'.",
			"Kolom IX Suku Bunga diisi hanya untuk produk berbunga (skema INTEREST); nisbah bagi hasil syariah bukan suku bunga sehingga ditulis '-'.",
			"Kolom X Nominal adalah saldo tabungan akhir bulan laporan. Kolom XIV Jumlah ditulis sama dengan Nominal karena biaya transaksi belum diamortisasi (kolom XIII) belum tersimpan (Form 11.00 - 3, PDF #page 198).",
			"Kolom IV, VIII, XI, XII, XIII, XV, XVI, XVII, dan XVIII belum punya sumber tersimpan; masing-masing ditulis '-' beserta alasan, bukan nol. Nomor Identitas tidak dipaparkan karena NIK/NPWP tersimpan terenkripsi (keputusan privasi).",
		},
	}
	for _, c := range form11Columns {
		if c.Reason != "" {
			sec.Unavailable = append(sec.Unavailable, ColumnUnavailable{Sandi: c.Sandi, Nama: c.Nama, Reason: c.Reason})
			continue
		}
		sec.Columns = append(sec.Columns, TableColumn{Sandi: c.Sandi, Nama: c.Nama})
	}
	for _, r := range rows {
		row := TableRow{Key: dashIfEmpty(r.AccountNumber)}
		for _, c := range form11Columns {
			if c.Reason != "" || c.Value == nil {
				continue
			}
			row.Cells = append(row.Cells, TableCell{Sandi: c.Sandi, Nama: c.Nama, Value: c.Value(r)})
		}
		sec.Rows = append(sec.Rows, row)
	}
	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// sukuBungaPersen menulis tarif tahunan sebagai persen dua desimal hanya bila jenis
// imbal hasilnya INTEREST. Skema lain (nisbah bagi hasil, wadiah, margin) bukan suku
// bunga dan ditulis "-"; tarif nol/belum diisi juga "-", bukan "0.00".
func sukuBungaPersen(jenis string, rate decimal.Decimal) string {
	if !strings.EqualFold(strings.TrimSpace(jenis), "INTEREST") {
		return "-"
	}
	if !rate.IsPositive() {
		return "-"
	}
	return rate.StringFixed(2)
}
