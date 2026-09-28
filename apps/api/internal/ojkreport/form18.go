package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form18.go membangun Form 00.18 "LAPORAN ARUS KAS" dari pergerakan jurnal pada akun
// kas/bank yang sudah dihitung layanan laporan (domain.CashFlow), lalu memetakannya ke
// sandi OJK. Modul ini TIDAK menghitung ulang rumus akuntansi; ia hanya mengelompokkan
// lawan jurnal kas ke pos OJK memakai pemetaan COAMapping18Draft (coa_mapping.go).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.18 – 1 "LAPORAN ARUS KAS": PDF #page 295-296 (hlm. tercetak 243-244).
//     Kolom I Nama Rekening, II Sandi, III 31 Des Tahun T, IV 31 Des Tahun T-1.
//     Susunan baris sandi (14010000-60000000) ditranskrip apa adanya dari halaman itu.
//   - Form 00.18 – 2 "PENJELASAN LAPORAN ARUS KAS": PDF #page 297 (hlm. 245).
//   - Kalimat "Form ini hanya disampaikan untuk laporan posisi bulan Desember" ada di
//     PDF #page 296 (hlm. 244) dan diulang pada PDF #page 297 (hlm. 245). Karena itu
//     form hanya dibangun untuk posisi Desember; posisi lain dicatat pada SkippedForms
//     sebagai TIDAK DIBANGUN, bukan ditulis nol.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Nilai kolom III/IV diambil dari domain.CashFlow.Rows hasil jurnal akun kas/bank
//     (report_service.go:215, reporting_repo.go:260). Nominal sudah bertanda: penerimaan
//     positif, pembayaran negatif, sehingga baris "Arus Kas neto" cukup menjumlahkan
//     baris rinciannya. Pembagian per aktivitas mengikuti sandi OJK, bukan tipe COA.
//   - Saldo kas awal/akhir periode dibaca dari neraca journal-based yang sama
//     (reporting_repo.go BalanceSheet) untuk kode COA kas dan setara kas; sehingga
//     "Kas awal + Peningkatan (Penurunan) Arus Kas = Kas akhir" dapat ditelusuri.
//   - Kolom IV (31 Des Tahun T-1) dibangun dari periode setahun sebelumnya dengan jalur
//     sumber yang sama (GetCashFlow dan GetBalanceSheet), bukan angka karangan.
//   - Baris sandi yang tidak punya akun COA sumber (mis. 14030000 Penerimaan beban klaim
//     asuransi, 14220000 Simpanan dari bank lain, 21030000 Surat Berharga) ditulis "-"
//     beserta alasan, bukan nol.
//   - Kode kas dan setara kas (COA 10100/10101/10200/11100/11200) dikeluarkan sumber
//     dari lawan jurnal (reporting_repo.go:24-28); baris 14140000 karena itu tidak
//     punya sumber dan ditulis "-", bukan nol.

// Kolom Form 00.18 – 1 (PDF #page 295).
const (
	form18SandiNama = "I"
	form18SandiKode = "II"
	form18SandiT    = "III"
	form18SandiT1   = "IV"
)

// form18CashCOACodes adalah kode COA yang dihitung sebagai kas dan setara kas oleh
// sumber arus kas. Daftarnya harus sama dengan cashAndBankCOACodes pada
// repository/postgres/reporting_repo.go:28; perbedaannya membuat saldo awal/akhir Form
// 00.18 tidak lagi tie-out dengan Peningkatan (Penurunan) Arus Kas.
var form18CashCOACodes = []string{"10100", "10101", "10200", "11100", "11200"}

// form18OperatingSandi adalah baris rincian aktivitas operasi (PDF #page 295), tanpa
// baris neto 10000000.
var form18OperatingSandi = []string{
	"14010000", "14020000", "14030000", "14040000", "14050000", "14060000",
	"14070000", "14080000", "14090000", "14100000", "14110000", "14120000",
	"14130000", "14140000", "14150000", "14160000", "14170000", "14180000",
	"14190000", "14200000", "14210000", "14220000", "14230000", "14240000",
	"14250000", "14260000",
}

// form18InvestingSandi adalah baris rincian aktivitas investasi (PDF #page 296).
var form18InvestingSandi = []string{"21010000", "21020000", "21030000", "21040000", "21990000"}

// form18FinancingSandi adalah baris rincian aktivitas pendanaan (PDF #page 296).
var form18FinancingSandi = []string{"31010000", "31020000", "31030000", "31990000"}

// form18Lines adalah susunan resmi Form 00.18 – 1 (PDF #page 295-296). Sandi dan nama
// pos diambil apa adanya. Reason diisi HANYA untuk pos yang belum punya akun COA sumber;
// nilainya ditulis "-", bukan nol. Baris neto dihitung dengan menjumlahkan baris
// rinciannya (TotalFrom), bukan dihitung ulang dari jurnal.
var form18Lines = []formLine{
	{Sandi: "14010000", Name: "Penerimaan pendapatan bunga", Level: 1},
	{Sandi: "14020000", Name: "Penerimaan pendapatan provisi dan jasa transaksi", Level: 1},
	{Sandi: "14030000", Name: "Penerimaan beban klaim asuransi", Level: 1,
		Reason: "belum ada akun COA untuk penerimaan klaim asuransi; bagan akun baku tidak memuat pos ini (Form 00.18 – 1, PDF #page 295)"},
	{Sandi: "14040000", Name: "Penerimaan atas aset keuangan yang telah dihapusbukukan", Level: 1,
		Reason: "belum ada akun COA untuk penerimaan aset keuangan yang telah dihapusbukukan; Form 02.00 pos 4102040000 juga belum punya akun COA (coa_mapping.go)"},
	{Sandi: "14050000", Name: "Pendapatan operasional lainnya", Level: 1},
	{Sandi: "14060000", Name: "Pembayaran beban bunga", Level: 1},
	{Sandi: "14070000", Name: "Beban gaji dan tunjangan", Level: 1},
	{Sandi: "14080000", Name: "Beban umum dan administrasi", Level: 1},
	{Sandi: "14090000", Name: "Beban operasional lainnya", Level: 1},
	{Sandi: "14100000", Name: "Pendapatan non operasional lainnya", Level: 1,
		Reason: "belum ada akun COA pendapatan non operasional pada bagan akun baku (seed coa_mapping_test.go tidak memuat akun 42xxx); baris ditulis tidak tersedia, bukan nol"},
	{Sandi: "14110000", Name: "Beban non operasional lainnya", Level: 1,
		Reason: "belum ada akun COA beban non operasional pada bagan akun baku (seed coa_mapping_test.go tidak memuat akun 52xxx); baris ditulis tidak tersedia, bukan nol"},
	{Sandi: "14120000", Name: "Pembayaran pajak penghasilan", Level: 1},
	{Sandi: "14130000", Name: "Penyesuaian lainnya atas pendapatan dan beban", Level: 1,
		Reason: "tidak ada akun COA yang dipetakan ke baris penyesuaian; baris ini hanya diisi bila bank mencatat penyesuaian tersendiri yang belum punya sandi baku (PDF #page 295)"},
	{Sandi: "14140000", Name: "Penempatan pada bank lain", Level: 1,
		Reason: "akun penempatan pada bank lain (COA 10200/11200) dihitung seluruhnya sebagai kas dan setara kas oleh sumber (reporting_repo.go:24-28), sehingga perubahannya tidak muncul sebagai lawan jurnal kas dan tidak punya sumber"},
	{Sandi: "14150000", Name: "Kredit yang diberikan", Level: 1},
	{Sandi: "14160000", Name: "Agunan yang diambil alih", Level: 1},
	{Sandi: "14170000", Name: "Aset lain-lain", Level: 1},
	{Sandi: "14180000", Name: "Penyesuaian lainnya atas aset operasional", Level: 1,
		Reason: "tidak ada akun COA yang dipetakan ke baris penyesuaian aset operasional (PDF #page 295)"},
	{Sandi: "14190000", Name: "Liabilitas segera", Level: 1},
	{Sandi: "14200000", Name: "Tabungan", Level: 1},
	{Sandi: "14210000", Name: "Deposito", Level: 1},
	{Sandi: "14220000", Name: "Simpanan dari bank lain", Level: 1,
		Reason: "belum ada akun COA untuk simpanan dari bank lain; bagan akun baku belum memisahkan simpanan bank lawan dari simpanan nasabah (Form 01.00 pos 2103010000 tanpa akun COA)"},
	{Sandi: "14230000", Name: "Pinjaman yang diterima", Level: 1,
		Reason: "belum ada akun COA untuk pinjaman yang diterima dari kreditur; Form 01.00 pos 2201010000 belum punya akun COA (coa_mapping.go)"},
	{Sandi: "14240000", Name: "Liabilitas imbalan kerja", Level: 1,
		Reason: "belum ada akun COA untuk liabilitas imbalan kerja pada bagan akun baku (PDF #page 295)"},
	{Sandi: "14250000", Name: "Liabilitas lain-lain", Level: 1},
	{Sandi: "14260000", Name: "Penyesuaian lainnya atas liabilitas operasional", Level: 1,
		Reason: "tidak ada akun COA yang dipetakan ke baris penyesuaian liabilitas operasional (PDF #page 295)"},
	{Sandi: "10000000", Name: "Arus Kas neto dari aktivitas operasi", Level: 1,
		TotalFrom: plus(form18OperatingSandi...)},

	{Sandi: "21010000", Name: "Pembelian/penjualan aset tetap dan inventaris", Level: 2},
	{Sandi: "21020000", Name: "Pembelian/penjualan aset tidak berwujud", Level: 2,
		Reason: "belum ada akun COA aset tidak berwujud pada bagan akun baku (PDF #page 296); Form 01.00 pos 1203010000 tanpa akun COA"},
	{Sandi: "21030000", Name: "Pembelian/penjualan Surat Berharga", Level: 2,
		Reason: "belum ada akun COA Surat Berharga pada bagan akun baku (Form 01.00 pos 1102000000 hanya sandi, coa_mapping.go)"},
	{Sandi: "21040000", Name: "Pembelian/penjualan Penyertaan Modal", Level: 2,
		Reason: "belum ada akun COA Penyertaan Modal pada bagan akun baku (Form 01.00 pos 1105000000 hanya sandi, coa_mapping.go)"},
	{Sandi: "21990000", Name: "Penyesuaian lainnya", Level: 2,
		Reason: "tidak ada akun COA yang dipetakan ke baris penyesuaian aktivitas investasi (PDF #page 296)"},
	{Sandi: "20000000", Name: "Arus Kas neto dari aktivitas Investasi", Level: 2,
		TotalFrom: plus(form18InvestingSandi...)},

	{Sandi: "31010000", Name: "Penerimaan/pembayaran pinjaman yang diterima sebagai modal pelengkap", Level: 2,
		Reason: "belum ada akun COA pinjaman modal pelengkap; Form 01.00 pos 2201010000 belum punya akun COA (coa_mapping.go)"},
	{Sandi: "31020000", Name: "Penerimaan/pembayaran pinjaman yang diterima sebagai modal inti tambahan", Level: 2,
		Reason: "belum ada akun COA pinjaman modal inti tambahan; Form 01.00 pos 2201010000 belum punya akun COA (coa_mapping.go)"},
	{Sandi: "31030000", Name: "Pembayaran dividen", Level: 2,
		Reason: "belum ada akun COA dividen pada bagan akun baku (PDF #page 296)"},
	{Sandi: "31990000", Name: "Penyesuaian lainnya", Level: 2},
	{Sandi: "30000000", Name: "Arus Kas neto dari aktivitas Pendanaan", Level: 2,
		TotalFrom: plus(form18FinancingSandi...)},

	{Sandi: "40000000", Name: "Peningkatan (Penurunan) Arus Kas", Level: 1,
		TotalFrom: plus("10000000", "20000000", "30000000")},
	{Sandi: "50000000", Name: "Kas dan setara Kas awal periode", Level: 1},
	{Sandi: "60000000", Name: "Kas dan setara Kas akhir periode", Level: 1},
}

// form18HeaderBefore menyisipkan baris judul kelompok (tanpa sandi) tepat sebelum sandi
// pertama kelompoknya, mengikuti Form 00.18 – 1 (PDF #page 295-296).
var form18HeaderBefore = map[string]string{
	"14010000": "Arus Kas dari Aktivitas Operasi Metode Langsung",
	"14140000": "Penurunan/Peningkatan atas aset operasional",
	"14190000": "Kenaikan/Peningkatan atas liabilitas operasional",
	"21010000": "Arus Kas dari aktivitas Investasi",
	"31010000": "Arus Kas dari aktivitas Pendanaan",
}

// collectForm18 memetakan baris arus kas (lawan jurnal kas) ke sandi Form 00.18.
// Kode yang belum dipetakan dikembalikan sebagai celah supaya ekspor Desember ditolak
// dengan jelas, bukan menghasilkan neto yang tidak tie-out.
func collectForm18(rows []domain.ReportRow) (map[string]decimal.Decimal, []string, error) {
	index := buildMappingIndex(COAMapping18Draft)
	return collect(rows, "00.18", index)
}

// kasDanSetara menghitung saldo kas dan setara kas dari baris neraca journal-based
// untuk kode COA pada form18CashCOACodes. Neraca nil menghasilkan nol.
func kasDanSetara(bs *domain.BalanceSheet) decimal.Decimal {
	if bs == nil {
		return decimal.Zero
	}
	cash := make(map[string]bool, len(form18CashCOACodes))
	for _, code := range form18CashCOACodes {
		cash[code] = true
	}
	total := decimal.Zero
	for _, row := range bs.Rows {
		if cash[row.AccountCode] {
			total = total.Add(row.Amount)
		}
	}
	return total
}

// buildForm18 menyusun Form 00.18 untuk posisi Desember. Kolom T dibaca dari arus kas
// 1 Januari s/d akhir periode; kolom T-1 dari periode setahun sebelumnya. Saldo kas
// awal/akhir periode dibaca dari neraca pada 31 Desember T, T-1, dan T-2, sehingga
// "Kas awal + Peningkatan (Penurunan) Arus Kas = Kas akhir" dapat diperiksa.
func (b *Builder) buildForm18(ctx context.Context, yearStart, periodEnd time.Time, bs *domain.BalanceSheet, book string) (TableSection, error) {
	cfT, err := b.source.GetCashFlow(ctx, yearStart, periodEnd, book)
	if err != nil {
		return TableSection{}, err
	}
	priorStart := yearStart.AddDate(-1, 0, 0)
	priorEnd := yearStart.AddDate(0, 0, -1) // 31 Desember T-1
	cfT1, err := b.source.GetCashFlow(ctx, priorStart, priorEnd, book)
	if err != nil {
		return TableSection{}, err
	}

	amountsT, gapsT, err := collectForm18(cfT.Rows)
	if err != nil {
		return TableSection{}, err
	}
	amountsT1, gapsT1, err := collectForm18(cfT1.Rows)
	if err != nil {
		return TableSection{}, err
	}
	if gaps := append(append([]string(nil), gapsT...), gapsT1...); len(gaps) > 0 {
		return TableSection{}, &IncompleteMappingError{Missing: map[string][]string{"00.18": gaps}}
	}

	bsPriorEnd, err := b.source.GetBalanceSheet(ctx, priorEnd, book)
	if err != nil {
		return TableSection{}, err
	}
	bsPrior2End, err := b.source.GetBalanceSheet(ctx, priorEnd.AddDate(-1, 0, 0), book)
	if err != nil {
		return TableSection{}, err
	}

	openingT := kasDanSetara(bsPriorEnd)
	amountsT["50000000"] = openingT
	amountsT["60000000"] = kasDanSetara(bs)
	amountsT1["50000000"] = kasDanSetara(bsPrior2End)
	amountsT1["60000000"] = openingT

	deriveTotals(form18Lines, amountsT)
	deriveTotals(form18Lines, amountsT1)
	return renderForm18(amountsT, amountsT1), nil
}

// renderForm18 menulis tabel Form 00.18 dari jumlah per sandi yang sudah dihitung.
// Fungsi ini murni sehingga dapat diuji tanpa basis data.
func renderForm18(amountsT, amountsT1 map[string]decimal.Decimal) TableSection {
	sec := TableSection{
		Form:     "00.18",
		Name:     formName("00.18"),
		KeyLabel: "Sandi/Nama Pos",
		Columns: []TableColumn{
			{Sandi: form18SandiNama, Nama: "Nama Rekening"},
			{Sandi: form18SandiKode, Nama: "Sandi"},
			{Sandi: form18SandiT, Nama: "31 Des Tahun T"},
			{Sandi: form18SandiT1, Nama: "31 Des Tahun T-1"},
		},
		Notes: []string{
			"Susunan baris dan kolom mengikuti Form 00.18 – 1 (PDF #page 295-296, hlm. 243-244); penjelasan pada Form 00.18 – 2 (PDF #page 297, hlm. 245).",
			"Form hanya disampaikan untuk laporan posisi bulan Desember (PDF #page 296 dan #page 297); posisi bulan lain dicatat TIDAK DIBANGUN, bukan ditulis nol.",
			"Nilai diambil dari domain.CashFlow.Rows (jurnal akun kas/bank, report_service.go:215 dan reporting_repo.go:260). Nominal bertanda: penerimaan positif, pembayaran negatif, sehingga Arus Kas neto adalah penjumlahan baris rinciannya. Pembagian aktivitas mengikuti sandi OJK (COAMapping18Draft), bukan tipe COA.",
			"Kolom IV (31 Des Tahun T-1) dibaca dari periode setahun sebelumnya lewat jalur sumber yang sama, bukan angka karangan.",
			"Saldo Kas dan setara Kas awal/akhir periode diambil dari neraca journal-based untuk kode COA kas dan setara kas (reporting_repo.go:24-28), sehingga Kas awal + Peningkatan (Penurunan) Arus Kas = Kas akhir.",
			"Baris yang belum punya akun COA sumber (mis. 14030000, 14220000, 21030000) ditulis \"-\" beserta alasan, bukan nol; kode COA yang belum dipetakan membuat ekspor Desember ditolak dengan ErrIncompleteMapping.",
		},
	}
	for _, l := range form18Lines {
		if header, ok := form18HeaderBefore[l.Sandi]; ok {
			sec.Rows = append(sec.Rows, TableRow{
				Key:   header,
				Cells: []TableCell{{Sandi: form18SandiNama, Nama: "Nama Rekening", Value: header}},
			})
		}
		row := TableRow{Key: l.Sandi}
		row.Cells = append(row.Cells,
			TableCell{Sandi: form18SandiNama, Nama: "Nama Rekening", Value: l.Name},
			TableCell{Sandi: form18SandiKode, Nama: "Sandi", Value: l.Sandi},
		)
		if l.Reason != "" {
			row.Reason = l.Name + ": " + l.Reason
			row.Cells = append(row.Cells,
				TableCell{Sandi: form18SandiT, Nama: "31 Des Tahun T", Value: "-"},
				TableCell{Sandi: form18SandiT1, Nama: "31 Des Tahun T-1", Value: "-"},
			)
		} else {
			row.Cells = append(row.Cells,
				TableCell{Sandi: form18SandiT, Nama: "31 Des Tahun T", Value: FormatRupiah(amountsT[l.Sandi])},
				TableCell{Sandi: form18SandiT1, Nama: "31 Des Tahun T-1", Value: FormatRupiah(amountsT1[l.Sandi])},
			)
		}
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}
