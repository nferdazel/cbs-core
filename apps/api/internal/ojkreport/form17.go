package ojkreport

import (
	"context"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form17.go membangun Form 00.17 "Laporan Perubahan Ekuitas" dari saldo COA ekuitas
// per akhir tahun. Modul ini TIDAK menghitung ulang rumus akuntansi dan TIDAK
// mengarang mutasi per sebab: yang tersedia hanya saldo akhir tahun.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.17 "LAPORAN PERUBAHAN EKUITAS": PDF #page 293 (hlm. tercetak 241).
//     17 baris tetap (sandi 10000000-30000000) x 12 kolom: I Nama Rekening, II Sandi,
//     III Modal Disetor, IV Tambahan Modal, V Modal Sumbangan, VI DSM Ekuitas,
//     VII Laba/Rugi yang Belum Direalisasi, VIII Surplus Revaluasi Aset Tetap,
//     IX Cadangan Tujuan, X Cadangan Umum, XI Saldo Laba yang Belum Ditentukan,
//     XII Jumlah. Susunan baris/kolom ditranskrip apa adanya dari halaman itu.
//   - Penjelasan Form 00.17: PDF #page 294 (hlm. 242).
//   - Kalimat "Form ini hanya disampaikan untuk laporan posisi bulan Desember" ada di
//     PDF #page 293. Karena itu form hanya dibangun untuk posisi Desember; posisi lain
//     dicatat pada SkippedForms sebagai TIDAK DIBANGUN, bukan ditulis nol.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Bagan akun ekuitas hanya memiliki 30100 Modal Disetor, 30200 Laba Ditahan,
//     30300 Laba Tahun Berjalan, 30400 Cadangan Umum (migrasi 000005:211-215) dan
//     varian syariah 13100 Modal Disetor Syariah, 13200 Laba Ditahan Syariah
//     (migrasi 000005:272-274). Karena itu hanya kolom III, X, dan XI yang punya akun
//     sumber.
//   - Kolom IV Tambahan Modal, V Modal Sumbangan, VI DSM Ekuitas, VII Laba/Rugi yang
//     Belum Direalisasi, VIII Surplus Revaluasi Aset Tetap, dan IX Cadangan Tujuan
//     belum punya akun COA; didaftarkan sebagai kolom tidak tersedia beserta alasan,
//     nilainya ditulis "-", bukan nol.
//   - Baris mutasi (Dividen, Pembentukan Cadangan, Setoran Modal, Laba/Rugi Periode
//     Berjalan, Revaluasi Aset Tetap) TIDAK diisi walau selisih saldo antar tahun bisa
//     dihitung: sebab mutasinya tidak tersimpan sebagai akun tersendiri. Baris
//     "Pos Penambah/Pengurang Lainnya" juga sengaja tidak dipakai sebagai residu.
//   - Kolom XII Jumlah hanya diisi bila seluruh kolom angka baris itu bernilai (pola
//     Form 14.00); karena sebagian kolom belum punya akun, Jumlah ditulis "-".

// Sandi kolom Form 00.17 (PDF #page 293).
const (
	form17SandiNamaRekening = "I"
	form17SandiSandi        = "II"
	form17SandiModalDisetor = "III"
	form17SandiTambahan     = "IV"
	form17SandiSumbangan    = "V"
	form17SandiDSM          = "VI"
	form17SandiLabaRugi     = "VII"
	form17SandiRevaluasi    = "VIII"
	form17SandiCadTujuan    = "IX"
	form17SandiCadUmum      = "X"
	form17SandiSaldoLaba    = "XI"
	form17SandiJumlah       = "XII"
)

// form17Tahun menandai baris "Saldo per 31 Des" menurut tahun saldonya.
const (
	form17TahunT2 = "T-2"
	form17TahunT1 = "T-1"
	form17TahunT  = "T"
)

// form17Column adalah satu kolom angka Form 00.17. COA berisi kode akun yang
// menjadi sumber kolom; kosong berarti belum ada akun sehingga Reason wajib terisi.
type form17Column struct {
	Sandi  string
	Nama   string
	COA    []string
	Reason string
}

// form17Columns adalah susunan kolom angka resmi Form 00.17 (PDF #page 293), urut
// III-XII. Nama kolom diambil apa adanya dari transkrip (termasuk "DSM Ekuitas").
var form17Columns = []form17Column{
	{Sandi: form17SandiModalDisetor, Nama: "Modal Disetor", COA: []string{"30100", "13100"}},
	{Sandi: form17SandiTambahan, Nama: "Tambahan Modal",
		Reason: "belum ada akun COA Tambahan Modal; bagan akun ekuitas hanya memiliki Modal Disetor (30100/13100), Laba Ditahan (30200/13200), Laba Tahun Berjalan (30300), dan Cadangan Umum (30400) (migrasi 000005:211-215, 272-274)"},
	{Sandi: form17SandiSumbangan, Nama: "Modal Sumbangan",
		Reason: "belum ada akun COA Modal Sumbangan; bagan akun ekuitas belum memisahkan setoran modal sumbangan"},
	{Sandi: form17SandiDSM, Nama: "DSM Ekuitas",
		Reason: "belum ada akun COA Dana Setoran Modal (DSM); bagan akun ekuitas belum menyimpannya"},
	{Sandi: form17SandiLabaRugi, Nama: "Laba/Rugi yang Belum Direalisasi",
		Reason: "belum ada akun COA laba/rugi yang belum direalisasi; bagan akun ekuitas belum menyimpannya"},
	{Sandi: form17SandiRevaluasi, Nama: "Surplus Revaluasi Aset Tetap",
		Reason: "belum ada akun COA surplus revaluasi aset tetap; akun aset tetap 10600/10700 hanya mencatat nilai perolehan dan akumulasi penyusutan"},
	{Sandi: form17SandiCadTujuan, Nama: "Cadangan Tujuan",
		Reason: "belum ada akun COA Cadangan Tujuan; bagan akun hanya punya Cadangan Umum (30400) tanpa pemisahan cadangan tujuan"},
	{Sandi: form17SandiCadUmum, Nama: "Cadangan Umum", COA: []string{"30400"}},
	{Sandi: form17SandiSaldoLaba, Nama: "Saldo Laba yang Belum Ditentukan", COA: []string{"30200", "30300", "13200"}},
	{Sandi: form17SandiJumlah, Nama: "Jumlah"},
}

// Alasan baris mutasi Form 00.17.
const (
	form17AlasanMutasi  = "mutasi ekuitas per sebab (dividen, setoran modal, pembentukan cadangan) tidak disimpan sebagai akun/jejak tersendiri; yang tersedia hanya saldo akhir tahun, sehingga baris mutasi tidak diisi, bukan nol"
	form17AlasanLaba    = "laba/rugi periode berjalan tidak diambil dari laporan laba rugi di sini agar tidak mencampur basis saldo ekuitas per akhir tahun; yang tersedia hanya saldo akun ekuitas, bukan mutasi per periode"
	form17AlasanPosLain = "baris Pos Penambah/Pengurang Lainnya sengaja tidak diisi sebagai residu/selisih saldo, karena angka sisa yang tidak punya sumber seperti itu adalah angka karangan"
)

// form17Line adalah baris tetap Form 00.17. Tahun diisi untuk baris "Saldo per 31 Des"
// (saldonya dibaca dari neraca akhir tahun itu); baris mutasi mengisi Reason.
type form17Line struct {
	Sandi  string
	Name   string
	Tahun  string
	Reason string
}

// form17Lines adalah susunan resmi 17 baris Form 00.17 (PDF #page 293). Sandi dan nama
// baris diambil apa adanya.
var form17Lines = []form17Line{
	{Sandi: "10000000", Name: "Saldo per 31 Des Tahun T-2", Tahun: form17TahunT2},
	{Sandi: "10100000", Name: "Dividen", Reason: form17AlasanMutasi},
	{Sandi: "10200000", Name: "Pembentukan Cadangan", Reason: form17AlasanMutasi},
	{Sandi: "10300000", Name: "Setoran Modal", Reason: form17AlasanMutasi},
	{Sandi: "10400000", Name: "Laba/Rugi yang Belum Direalisasi", Reason: form17AlasanMutasi},
	{Sandi: "10500000", Name: "Revaluasi Aset Tetap", Reason: form17AlasanMutasi},
	{Sandi: "10600000", Name: "Laba/Rugi Periode Berjalan", Reason: form17AlasanLaba},
	{Sandi: "19900000", Name: "Pos Penambah/Pengurang Lainnya", Reason: form17AlasanPosLain},
	{Sandi: "20000000", Name: "Saldo per 31 Des Tahun T-1", Tahun: form17TahunT1},
	{Sandi: "20100000", Name: "Dividen", Reason: form17AlasanMutasi},
	{Sandi: "20200000", Name: "Pembentukan Cadangan", Reason: form17AlasanMutasi},
	{Sandi: "20300000", Name: "Setoran Modal", Reason: form17AlasanMutasi},
	{Sandi: "20400000", Name: "Laba/Rugi yang Belum Direalisasi", Reason: form17AlasanMutasi},
	{Sandi: "20500000", Name: "Revaluasi Aset Tetap", Reason: form17AlasanMutasi},
	{Sandi: "20600000", Name: "Laba/Rugi Periode Berjalan", Reason: form17AlasanLaba},
	{Sandi: "29900000", Name: "Pos Penambah/Pengurang Lainnya", Reason: form17AlasanPosLain},
	{Sandi: "30000000", Name: "Saldo per 31 Des Tahun T", Tahun: form17TahunT},
}

// form17SaldoEkuitas menghitung saldo tiap kolom bersumber dari baris neraca. Hanya
// kode COA pada form17Columns yang dijumlahkan; kode lain diabaikan. Neraca nil
// menghasilkan map kosong (nilai nol saat dibaca).
func form17SaldoEkuitas(bs *domain.BalanceSheet) map[string]decimal.Decimal {
	out := make(map[string]decimal.Decimal, 3)
	if bs == nil {
		return out
	}
	for _, col := range form17Columns {
		if len(col.COA) == 0 {
			continue
		}
		for _, row := range bs.Rows {
			for _, code := range col.COA {
				if row.AccountCode == code {
					out[col.Sandi] = out[col.Sandi].Add(row.Amount)
				}
			}
		}
	}
	return out
}

// buildForm17 menyusun Form 00.17 untuk posisi Desember. Saldo dibaca dari neraca
// journal-based pada 31 Des T-2, T-1, dan T (naraca T sudah dipegang pemanggil);
// periode lain tidak memanggil fungsi ini (lihat gerbang Desember di builder.go).
func (b *Builder) buildForm17(ctx context.Context, yearStart time.Time, bs *domain.BalanceSheet, book string) (TableSection, error) {
	t1End := yearStart.AddDate(0, 0, -1) // 31 Desember T-1
	t2End := t1End.AddDate(-1, 0, 0)     // 31 Desember T-2
	bsT1, err := b.source.GetBalanceSheet(ctx, t1End, book)
	if err != nil {
		return TableSection{}, err
	}
	bsT2, err := b.source.GetBalanceSheet(ctx, t2End, book)
	if err != nil {
		return TableSection{}, err
	}
	return renderForm17(form17SaldoEkuitas(bsT2), form17SaldoEkuitas(bsT1), form17SaldoEkuitas(bs)), nil
}

// form17SaldoTahun memilih map saldo sesuai tahun baris. Baris mutasi (tahun kosong)
// tidak punya saldo.
func form17SaldoTahun(tahun string, t2, t1, t map[string]decimal.Decimal) map[string]decimal.Decimal {
	switch tahun {
	case form17TahunT2:
		return t2
	case form17TahunT1:
		return t1
	case form17TahunT:
		return t
	default:
		return nil
	}
}

// form17HitungJumlah menghitung kolom XII dari nilai kolom angka yang sudah tersedia.
// Bila ada kolom yang tidak bernilai ("-"), jumlah tidak dihitung karena angka
// sebagian akan menyesatkan (pola Form 14.00).
func form17HitungJumlah(angka []string) (decimal.Decimal, bool) {
	if len(angka) == 0 {
		return decimal.Zero, false
	}
	total := decimal.Zero
	for _, v := range angka {
		if strings.TrimSpace(v) == "" || v == "-" {
			return decimal.Zero, false
		}
		d, err := decimal.NewFromString(v)
		if err != nil {
			return decimal.Zero, false
		}
		total = total.Add(d)
	}
	return total, true
}

// renderForm17 menulis tabel Form 00.17 dari saldo per kolom yang sudah dihitung.
// Fungsi ini murni sehingga dapat diuji tanpa basis data.
func renderForm17(saldoT2, saldoT1, saldoT map[string]decimal.Decimal) TableSection {
	sec := TableSection{
		Form:     "00.17",
		Name:     formName("00.17"),
		KeyLabel: "Sandi/Nama Pos",
		Notes: []string{
			"Susunan 17 baris dan 12 kolom mengikuti Form 00.17 (PDF #page 293, hlm. 241); penjelasan pada PDF #page 294.",
			"Form hanya disampaikan untuk laporan posisi bulan Desember (PDF #page 293); posisi bulan lain dicatat TIDAK DIBANGUN, bukan ditulis nol.",
			"Angka dibaca dari saldo COA per akhir tahun (sumber jurnal ReportingRepository.BalanceSheet yang sama dengan Form 01.00) pada 31 Des T-2, T-1, dan T. Hanya kolom III Modal Disetor (30100/13100), X Cadangan Umum (30400), dan XI Saldo Laba yang Belum Ditentukan (30200/30300/13200) yang punya akun COA; baris \"Saldo per 31 Des\" diisi dari saldo akhir tahun itu.",
			"Kolom IV Tambahan Modal, V Modal Sumbangan, VI DSM Ekuitas, VII Laba/Rugi yang Belum Direalisasi, VIII Surplus Revaluasi Aset Tetap, dan IX Cadangan Tujuan belum punya akun COA sehingga didaftarkan sebagai kolom tidak tersedia. Seluruh baris mutasi juga ditulis \"-\" beserta alasannya karena sebab mutasinya tidak tersimpan per akun; baris Pos Penambah/Pengurang Lainnya tidak diisi sebagai residu.",
			"Kolom XII Jumlah hanya diisi bila seluruh kolom angka pada baris itu bernilai (pola Form 14.00); karena sebagian kolom belum punya akun, Jumlah ditulis \"-\" (bukan 0).",
			"Kolom XI menjumlahkan Laba Ditahan (30200/13200) dan Laba Tahun Berjalan (30300) karena keduanya adalah saldo laba yang belum ditentukan penggunaannya. Bila laba berjalan belum diposting ke 30300 dan masih berupa baris penyeimbang \"-\" pada neraca (reporting_repo.go:229-237), kolom XI dapat lebih kecil dari total ekuitas; kekurangan itu dicatat, bukan ditutup dengan residu.",
		},
	}
	sec.Columns = append(sec.Columns,
		TableColumn{Sandi: form17SandiNamaRekening, Nama: "Nama Rekening"},
		TableColumn{Sandi: form17SandiSandi, Nama: "Sandi"},
	)
	for _, c := range form17Columns {
		if c.Reason != "" {
			sec.Unavailable = append(sec.Unavailable, ColumnUnavailable{Sandi: c.Sandi, Nama: c.Nama, Reason: c.Reason})
			continue
		}
		sec.Columns = append(sec.Columns, TableColumn{Sandi: c.Sandi, Nama: c.Nama})
	}

	for _, l := range form17Lines {
		row := TableRow{Key: l.Sandi, Reason: l.Reason}
		row.Cells = append(row.Cells,
			TableCell{Sandi: form17SandiNamaRekening, Nama: "Nama Rekening", Value: l.Name},
			TableCell{Sandi: form17SandiSandi, Nama: "Sandi", Value: l.Sandi},
		)
		saldo := form17SaldoTahun(l.Tahun, saldoT2, saldoT1, saldoT)
		var angka []string
		for _, c := range form17Columns {
			if c.Sandi == form17SandiJumlah {
				continue
			}
			val := "-"
			if c.Reason == "" {
				if l.Tahun != "" {
					val = FormatRupiah(saldo[c.Sandi])
				}
				row.Cells = append(row.Cells, TableCell{Sandi: c.Sandi, Nama: c.Nama, Value: val})
			}
			angka = append(angka, val)
		}
		jml, lengkap := form17HitungJumlah(angka)
		nilaiJumlah := "-"
		if lengkap {
			nilaiJumlah = FormatRupiah(jml)
		}
		row.Cells = append(row.Cells, TableCell{Sandi: form17SandiJumlah, Nama: "Jumlah", Value: nilaiJumlah})
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}
