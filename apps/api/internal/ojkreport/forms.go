package ojkreport

// forms.go memuat susunan baris (sandi + nama pos) form Laporan Bulanan BPR
// sesuai Lampiran II SEOJK No. 16/SEOJK.03/2024. Sandi dan nama pos diambil apa
// adanya dari lampiran tersebut.
//
// Baris total dihitung dengan menjumlahkan pos anak yang sudah dipetakan
// (TotalFrom). Ini hanya penjumlahan pos, bukan perhitungan ulang rumus akuntansi.

// totalPart adalah komponen sebuah baris total: sandi pos anak dan bobotnya.
type totalPart struct {
	Sandi  string
	Weight int
}

// formLine adalah satu baris form OJK. TotalFrom diisi untuk baris total; baris
// biasa cukup mengandalkan hasil pemetaan COA.
type formLine struct {
	Sandi     string
	Name      string
	Level     int
	TotalFrom []totalPart
}

// plus membuat komponen total dengan bobot +1.
func plus(sandis ...string) []totalPart {
	parts := make([]totalPart, len(sandis))
	for i, s := range sandis {
		parts[i] = totalPart{Sandi: s, Weight: 1}
	}
	return parts
}

// minus membuat komponen total dengan bobot -1 (pos pengurang).
func minus(sandis ...string) []totalPart {
	parts := make([]totalPart, len(sandis))
	for i, s := range sandis {
		parts[i] = totalPart{Sandi: s, Weight: -1}
	}
	return parts
}

// combine menggabungkan beberapa daftar komponen menjadi satu.
func combine(parts ...[]totalPart) []totalPart {
	var out []totalPart
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// assetSandi adalah pos aset Form 01.00 (untuk baris TOTAL ASET).
var assetSandi = []string{
	"1101010000", "1101020000", "1102000000", "1102010000",
	"1103010000", "1103020000",
	"1104010100", "1104010200", "1104010300", "1104010400", "1104010500", "1104020000",
	"1105000000", "1105010000",
	"1201000000", "1205000000",
	"1202010000", "1202020000", "1203010000", "1203020000",
	"1204000000", "1206000000", "1206010000", "1299000000",
}

// liabilitySandi adalah pos liabilitas Form 01.00.
var liabilitySandi = []string{
	"2101000000",
	"2102010100", "2102010200", "2102020100", "2102020200",
	"2103010000", "2103020000",
	"2201010000", "2201020000", "2201030000",
	"2202000000", "2203000000", "2299000000",
}

// equitySandi adalah pos ekuitas Form 01.00.
var equitySandi = []string{
	"3101010000", "3101020000",
	"3102010000", "3102020000", "3102030000", "3102990000",
	"3103010000", "3103020000", "3103990000", "3103980000",
	"3104010000", "3104020000",
	"3105010000", "3105020000",
}

// operationalRevenueSandi adalah pos pendapatan operasional Form 02.00.
var operationalRevenueSandi = []string{
	"4101010100", "4101010201", "4101010202", "4101010203", "4101010204",
	"4101010301", "4101010302", "4101020100", "4101020200",
	"4101030100", "4101030201", "4101030202", "4101040000",
	"4102010000", "4102020000", "4102030000", "4102040000", "4102050000",
	"4102060000", "4102070000", "4201020000", "4203000000", "4202020000", "4102990000",
}

// operationalExpenseSandi adalah pos beban operasional Form 02.00 (butir 1-7,
// tidak termasuk Taksiran Pajak Penghasilan yang posnya terpisah).
var operationalExpenseSandi = []string{
	"5101010100", "5101010200", "5101010300",
	"5101010401", "5101010402", "5101010403", "5101010404", "5101019900",
	"5101020100", "5101020200",
	"5102000000",
	"5103010000", "5103020000", "5103030100", "5103030200", "5103040000", "5103050000",
	"5104000000", "5105000000",
	"5106010100", "5106010200", "5106019900", "5106020000",
	"5106030100", "5106039900", "5106040000", "5106050000", "5106060000",
	"5106070000", "5106080000", "5106100000", "5106110000", "5106111000", "5106112000",
	"5106090000",
	"5199010000", "5199020000", "5199030000", "5201020000", "5202020000", "5199990000",
}

// nonOperationalRevenueSandi adalah pos pendapatan nonoperasional Form 02.00.
var nonOperationalRevenueSandi = []string{
	"4201010000", "4202010000", "4202030000", "4204000000", "4205000000", "4299000000",
}

// nonOperationalExpenseSandi adalah pos beban nonoperasional Form 02.00.
var nonOperationalExpenseSandi = []string{
	"5201010000", "5202010000", "5202030000", "5203000000", "5204000000", "5299000000",
}

// form01Lines adalah susunan Form 01.00 LAPORAN POSISI KEUANGAN.
var form01Lines = []formLine{
	{Sandi: "1101010000", Name: "Kas dalam Rupiah"},
	{Sandi: "1101020000", Name: "Kas dalam Valuta Asing"},
	{Sandi: "1102000000", Name: "Surat Berharga"},
	{Sandi: "1102010000", Name: "-/- Cadangan Kerugian Penurunan Nilai"},
	{Sandi: "1103010000", Name: "Penempatan pada Bank Lain"},
	{Sandi: "1103020000", Name: "-/- Cadangan Kerugian Penurunan Nilai"},
	{Sandi: "1104010100", Name: "Kredit yang Diberikan (Baki Debet)"},
	{Sandi: "1104010200", Name: "-/- Provisi Belum Diamortisasi"},
	{Sandi: "1104010300", Name: "Biaya Transaksi Belum Diamortisasi"},
	{Sandi: "1104010400", Name: "-/- Pendapatan Bunga yang Ditangguhkan Dalam Rangka Restrukturisasi"},
	{Sandi: "1104010500", Name: "-/- Cadangan Kerugian Restrukturisasi"},
	{Sandi: "1104020000", Name: "-/- Cadangan Kerugian Penurunan Nilai"},
	{Sandi: "1105000000", Name: "Penyertaan Modal"},
	{Sandi: "1105010000", Name: "-/- Cadangan Kerugian Penurunan Nilai"},
	{Sandi: "1201000000", Name: "Agunan yang Diambil Alih"},
	{Sandi: "1205000000", Name: "Properti Terbengkalai"},
	{Sandi: "1202010000", Name: "Aset Tetap dan Inventaris"},
	{Sandi: "1202020000", Name: "-/- Akumulasi Penyusutan dan Penurunan Nilai"},
	{Sandi: "1203010000", Name: "Aset Tidak Berwujud"},
	{Sandi: "1203020000", Name: "-/- Akumulasi Amortisasi dan Penurunan Nilai"},
	{Sandi: "1204000000", Name: "Aset Antarkantor"},
	{Sandi: "1206000000", Name: "Aset Keuangan Lainnya"},
	{Sandi: "1206010000", Name: "-/- Cadangan Kerugian Penurunan Nilai"},
	{Sandi: "1299000000", Name: "Aset Lainnya"},
	{Sandi: "1000000000", Name: "TOTAL ASET", Level: 1, TotalFrom: plus(assetSandi...)},

	{Sandi: "2101000000", Name: "Liabilitas Segera"},
	{Sandi: "2102010100", Name: "Simpanan a. Tabungan", Level: 1},
	{Sandi: "2102010200", Name: "-/- Biaya Transaksi Belum Diamortisasi", Level: 2},
	{Sandi: "2102020100", Name: "b. Deposito", Level: 1},
	{Sandi: "2102020200", Name: "-/- Biaya Transaksi Belum Diamortisasi", Level: 2},
	{Sandi: "2103010000", Name: "Simpanan dari Bank Lain"},
	{Sandi: "2103020000", Name: "-/- Biaya Transaksi Belum Diamortisasi"},
	{Sandi: "2201010000", Name: "Pinjaman yang Diterima"},
	{Sandi: "2201020000", Name: "-/- Biaya Transaksi Belum Diamortisasi"},
	{Sandi: "2201030000", Name: "-/- Diskonto Belum Diamortisasi"},
	{Sandi: "2202000000", Name: "Dana Setoran Modal - Kewajiban"},
	{Sandi: "2203000000", Name: "Liabilitas Antarkantor"},
	{Sandi: "2299000000", Name: "Liabilitas Lainnya"},
	{Sandi: "2000000000", Name: "Total Liabilitas", Level: 1, TotalFrom: plus(liabilitySandi...)},

	{Sandi: "3101010000", Name: "Modal Disetor a. Modal Dasar", Level: 1},
	{Sandi: "3101020000", Name: "b. Modal yang Belum Disetor -/-", Level: 2},
	{Sandi: "3102010000", Name: "Tambahan Modal Disetor a. Agio", Level: 1},
	{Sandi: "3102020000", Name: "b. Modal Sumbangan", Level: 2},
	{Sandi: "3102030000", Name: "c. Dana Setoran Modal - Ekuitas", Level: 2},
	{Sandi: "3102990000", Name: "d. Tambahan Modal Disetor Lainnya", Level: 2},
	{Sandi: "3103010000", Name: "Ekuitas lain a. Keuntungan (Kerugian) dari Perubahan Nilai Aset Keuangan Tersedia untuk Dijual", Level: 1},
	{Sandi: "3103020000", Name: "b. Keuntungan Revaluasi Aset Tetap", Level: 2},
	{Sandi: "3103990000", Name: "c. Lainnya", Level: 2},
	{Sandi: "3103980000", Name: "d. Pajak Penghasilan terkait dengan Ekuitas Lain", Level: 2},
	{Sandi: "3104010000", Name: "Cadangan a. Umum", Level: 1},
	{Sandi: "3104020000", Name: "b. Tujuan", Level: 2},
	{Sandi: "3105010000", Name: "Laba (rugi) a. Tahun-Tahun Lalu", Level: 1},
	{Sandi: "3105020000", Name: "b. Tahun Berjalan", Level: 2},
	{Sandi: "3000000000", Name: "Total Ekuitas", Level: 1, TotalFrom: plus(equitySandi...)},
	{Sandi: "", Name: "TOTAL LIABILITAS DAN EKUITAS", Level: 1,
		TotalFrom: combine(plus("2000000000"), plus("3000000000"))},
}

// form02Lines adalah susunan Form 02.00 LAPORAN LABA RUGI DAN PENGHASILAN
// KOMPREHENSIF LAIN (sampai Jumlah Laba (Rugi) Tahun Berjalan).
var form02Lines = []formLine{
	{Sandi: "4100000000", Name: "Pendapatan Operasional", Level: 1, TotalFrom: plus(operationalRevenueSandi...)},

	{Sandi: "4101010100", Name: "1. Pendapatan Bunga a. Bunga Kontraktual i. Surat Berharga", Level: 2},
	{Sandi: "4101010201", Name: "ii. Penempatan pada Bank Lain - Giro", Level: 3},
	{Sandi: "4101010202", Name: "Tabungan", Level: 3},
	{Sandi: "4101010203", Name: "Deposito", Level: 3},
	{Sandi: "4101010204", Name: "Sertifikat Deposito", Level: 3},
	{Sandi: "4101010301", Name: "iii. Kredit yang Diberikan - Kepada Bank Lain", Level: 3},
	{Sandi: "4101010302", Name: "Kepada Pihak Ketiga Bukan Bank", Level: 3},
	{Sandi: "4101020100", Name: "b. Provisi Kredit i. Kepada Bank Lain", Level: 2},
	{Sandi: "4101020200", Name: "ii. Kepada Pihak Ketiga Bukan Bank", Level: 2},
	{Sandi: "4101030100", Name: "c. Biaya Transaksi -/- i. Surat Berharga", Level: 2},
	{Sandi: "4101030201", Name: "ii. Kredit yang Diberikan - Kepada Bank Lain", Level: 3},
	{Sandi: "4101030202", Name: "Kepada Pihak Ketiga Bukan Bank", Level: 3},
	{Sandi: "4101040000", Name: "d. Koreksi atas Pendapatan Bunga -/-", Level: 2},
	{Sandi: "4102010000", Name: "2. Pendapatan Lainnya a. Pendapatan Jasa Transaksi", Level: 2},
	{Sandi: "4102020000", Name: "b. Keuntungan Penjualan Valuta Asing", Level: 3},
	{Sandi: "4102030000", Name: "c. Keuntungan Penjualan Surat Berharga", Level: 3},
	{Sandi: "4102040000", Name: "d. Penerimaan Aset Produktif yang Dihapus Buku", Level: 3},
	{Sandi: "4102050000", Name: "e. Pemulihan Cadangan Kerugian Penurunan Nilai", Level: 3},
	{Sandi: "4102060000", Name: "f. Dividen", Level: 3},
	{Sandi: "4102070000", Name: "g. Keuntungan dari penyertaan equity method", Level: 3},
	{Sandi: "4201020000", Name: "h. Keuntungan penjualan AYDA", Level: 3},
	{Sandi: "4203000000", Name: "i. Pendapatan ganti rugi asuransi", Level: 3},
	{Sandi: "4202020000", Name: "j. Pemulihan penurunan AYDA", Level: 3},
	{Sandi: "4102990000", Name: "k. Lainnya", Level: 3},

	{Sandi: "5100000000", Name: "Beban Operasional", Level: 1, TotalFrom: plus(operationalExpenseSandi...)},
	{Sandi: "5101010100", Name: "1. Beban Bunga a. Beban Bunga Kontraktual i. Tabungan", Level: 2},
	{Sandi: "5101010200", Name: "ii. Deposito", Level: 3},
	{Sandi: "5101010300", Name: "iii. Simpanan dari Bank Lain", Level: 3},
	{Sandi: "5101010401", Name: "iv. Pinjaman yang Diterima 1) Dari Bank Indonesia", Level: 3},
	{Sandi: "5101010402", Name: "2) Dari Bank Lain", Level: 4},
	{Sandi: "5101010403", Name: "3) Dari Pihak Ketiga Bukan Bank", Level: 4},
	{Sandi: "5101010404", Name: "4) Berupa Pinjaman Subordinasi", Level: 4},
	{Sandi: "5101019900", Name: "v. Lainnya", Level: 3},
	{Sandi: "5101020100", Name: "b. Biaya Transaksi i. Kepada Bank Lain", Level: 2},
	{Sandi: "5101020200", Name: "ii. Kepada Pihak Ketiga Bukan Bank", Level: 2},
	{Sandi: "5102000000", Name: "2. Beban Kerugian Restrukturisasi Kredit", Level: 1},
	{Sandi: "5103010000", Name: "3. Beban Kerugian Penurunan Nilai a. Surat Berharga", Level: 2},
	{Sandi: "5103020000", Name: "b. Penempatan pada Bank Lain", Level: 3},
	{Sandi: "5103030100", Name: "c. Kredit yang Diberikan i. Kepada Bank Lain", Level: 3},
	{Sandi: "5103030200", Name: "ii. Kepada Pihak Ketiga Bukan Bank", Level: 4},
	{Sandi: "5103040000", Name: "d. Penyertaan Modal", Level: 3},
	{Sandi: "5103050000", Name: "e. Aset Keuangan Lainnya", Level: 3},
	{Sandi: "5104000000", Name: "4. Beban Pemasaran", Level: 1},
	{Sandi: "5105000000", Name: "5. Beban Penelitian dan Pengembangan", Level: 1},
	{Sandi: "5106010100", Name: "6. Beban Administrasi dan Umum a. Beban Tenaga Kerja i. Gaji dan Upah", Level: 2},
	{Sandi: "5106010200", Name: "ii. Honorarium", Level: 3},
	{Sandi: "5106019900", Name: "iii. Lainnya", Level: 3},
	{Sandi: "5106020000", Name: "b. Beban Pendidikan dan Pelatihan", Level: 2},
	{Sandi: "5106030100", Name: "c. Beban Sewa i. Gedung Kantor", Level: 3},
	{Sandi: "5106039900", Name: "ii. Lainnya", Level: 4},
	{Sandi: "5106040000", Name: "d. Beban Penyusutan/Penghapusan atas Aset Tetap dan Inventaris", Level: 2},
	{Sandi: "5106050000", Name: "e. Beban Amortisasi Aset Tidak Berwujud", Level: 2},
	{Sandi: "5106060000", Name: "f. Beban Premi Asuransi", Level: 2},
	{Sandi: "5106070000", Name: "g. Beban Pemeliharaan dan Perbaikan", Level: 2},
	{Sandi: "5106080000", Name: "h. Beban Barang dan Jasa", Level: 2},
	{Sandi: "5106100000", Name: "i. Beban Penyelenggaraan Teknologi Informasi", Level: 2},
	{Sandi: "5106110000", Name: "j. Kerugian terkait Risiko Operasional", Level: 2},
	{Sandi: "5106111000", Name: "i. Kecurangan internal", Level: 3},
	{Sandi: "5106112000", Name: "ii. Kejahatan eksternal", Level: 3},
	{Sandi: "5106090000", Name: "k. Pajak-Pajak", Level: 2},
	{Sandi: "5199010000", Name: "7. Beban Lainnya a. Kerugian Penjualan Valuta Asing", Level: 2},
	{Sandi: "5199020000", Name: "b. Kerugian Penjualan Surat Berharga", Level: 3},
	{Sandi: "5199030000", Name: "c. Kerugian dari penyertaan dengan Equity Method", Level: 3},
	{Sandi: "5201020000", Name: "d. Kerugian penjualan AYDA", Level: 3},
	{Sandi: "5202020000", Name: "e. Kerugian penurunan nilai AYDA", Level: 3},
	{Sandi: "5199990000", Name: "f. Lainnya", Level: 3},

	{Sandi: "3104040100", Name: "Laba (Rugi) Operasional", Level: 1,
		TotalFrom: combine(plus("4100000000"), minus("5100000000"))},

	{Sandi: "4200000000", Name: "Pendapatan Nonoperasional", Level: 1, TotalFrom: plus(nonOperationalRevenueSandi...)},
	{Sandi: "4201010000", Name: "1. Keuntungan Penjualan Aset Tetap dan Inventaris", Level: 2},
	{Sandi: "4202010000", Name: "2. Pemulihan Penurunan Nilai a. Aset Tetap dan Inventaris", Level: 2},
	{Sandi: "4202030000", Name: "b. Lainnya", Level: 3},
	{Sandi: "4204000000", Name: "4. Bunga Antarkantor", Level: 2},
	{Sandi: "4205000000", Name: "5. Selisih Kurs", Level: 2},
	{Sandi: "4299000000", Name: "6. Lainnya", Level: 2},

	{Sandi: "5200000000", Name: "Beban Nonoperasional", Level: 1, TotalFrom: plus(nonOperationalExpenseSandi...)},
	{Sandi: "5201010000", Name: "1. Kerugian Penjualan/Kehilangan Aset Tetap dan Inventaris", Level: 2},
	{Sandi: "5202010000", Name: "2. Kerugian Penurunan Nilai a. Aset Tetap dan Inventaris", Level: 2},
	{Sandi: "5202030000", Name: "b. Lainnya", Level: 3},
	{Sandi: "5203000000", Name: "3. Bunga Antarkantor", Level: 2},
	{Sandi: "5204000000", Name: "4. Selisih Kurs", Level: 2},
	{Sandi: "5299000000", Name: "5. Lainnya", Level: 2},

	{Sandi: "3104040200", Name: "Laba (Rugi) Nonoperasional", Level: 1,
		TotalFrom: combine(plus("4200000000"), minus("5200000000"))},
	{Sandi: "3104040300", Name: "Laba (Rugi) Tahun Berjalan Sebelum Pajak", Level: 1,
		TotalFrom: combine(plus("3104040100"), plus("3104040200"))},
	{Sandi: "5300000000", Name: "Taksiran Pajak Penghasilan"},
	{Sandi: "4400000000", Name: "Pendapatan Pajak Tangguhan"},
	{Sandi: "5400000000", Name: "Beban Pajak Tangguhan"},
	{Sandi: "3104040400", Name: "Jumlah Laba (Rugi) Tahun Berjalan", Level: 1,
		TotalFrom: combine(plus("3104040300"), minus("5300000000"), plus("4400000000"), minus("5400000000"))},
}
