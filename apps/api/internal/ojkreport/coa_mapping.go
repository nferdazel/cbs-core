package ojkreport

import "sort"

// ─────────────────────────────────────────────────────────────────────────────
// PEMETAAN COA → POS LAPORAN OJK — STATUS: DRAF, BELUM TERVERIFIKASI
//
// Berkas ini SENGAJA dipisah dari alur kode agar mudah ditinjau orang (auditor /
// pemilik pedoman konversi). Jangan menyembunyikan pemetaan di dalam builder.
//
// Apa yang sudah terverifikasi:
//   - Sandi dan nama pos pada forms.go diambil langsung dari Lampiran II
//     SEOJK No. 16/SEOJK.03/2024 (dokumen resmi ojk.go.id).
//
// Apa yang BELUM terverifikasi (butuh pedoman konversi bank):
//   - Penetapan baris COA internal (`chart_of_accounts`) ke pos OJK tertentu.
//     SEOJK mewajibkan BPR memiliki pedoman konversi pos laporan keuangan ke
//     aplikasi inti; pemetaan di bawah adalah usulan awal, bukan hasil konversi
//     resmi.
//
// Setiap entri Verified=false sampai diperiksa manusia. Entri dengan tanda
// Catatan perlu perhatian khusus karena pos OJK-nya ambigu.
//
// Konvensi Sign: +1 nilai COA dipakai apa adanya; -1 untuk pos lawan/contra
// (mis. akumulasi penyusutan dan CKPN yang mengurangi kelompoknya).
// ─────────────────────────────────────────────────────────────────────────────

// MappingStatus menandai tingkat verifikasi seluruh berkas pemetaan.
const MappingStatus = "DRAF-BELUM-TERVERIFIKASI"

// MappingEntry menghubungkan satu kode COA internal ke satu pos (sandi) OJK.
type MappingEntry struct {
	COACode string
	Form    string // "01.00" atau "02.00"
	Sandi   string // sandi pos OJK (Lampiran II)
	// Sign +1 bila nilai COA menambah pos, -1 bila mengurangi (pos lawan).
	Sign int
	// Verified true hanya setelah pemetaan diperiksa terhadap pedoman konversi bank.
	Verified bool
	// Note menjelaskan keraguan/penalaran untuk entri yang belum pasti.
	Note string
}

// COAMappingDraft adalah pemetaan usulan. Terurut menurut kode COA agar mudah dibaca.
var COAMappingDraft = []MappingEntry{
	// ── Form 01.00: ASET ────────────────────────────────────────────────────
	{COACode: "10100", Form: "01.00", Sandi: "1101010000", Sign: 1, Note: "Kas (akun induk) -> Kas dalam Rupiah"},
	{COACode: "10101", Form: "01.00", Sandi: "1101010000", Sign: 1, Note: "Kas Teller -> Kas dalam Rupiah"},
	{COACode: "10200", Form: "01.00", Sandi: "1103010000", Sign: 1, Note: "Penempatan pada Bank Lain"},
	{COACode: "10300", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Kredit yang Diberikan (Baki Debet)"},
	{COACode: "10301", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Kredit - Pokok -> Kredit yang Diberikan (Baki Debet)"},
	{COACode: "10305", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Piutang Denda tidak punya pos tersendiri; diusulkan ke Aset Lainnya"},
	{COACode: "10400", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Bunga kredit masih akan diterima; diusulkan ke Aset Lainnya (perlu konfirmasi apakah digabung ke Kredit)"},
	{COACode: "10500", Form: "01.00", Sandi: "1201000000", Sign: 1, Note: "Agunan yang Diambil Alih"},
	{COACode: "10600", Form: "01.00", Sandi: "1202010000", Sign: 1, Note: "Aset Tetap dan Inventaris"},
	{COACode: "10700", Form: "01.00", Sandi: "1202020000", Sign: -1, Note: "Akumulasi Penyusutan mengurangi aset tetap"},
	{COACode: "10800", Form: "01.00", Sandi: "1204000000", Sign: 1, Note: "Aset Antarkantor"},
	{COACode: "10900", Form: "01.00", Sandi: "1104020000", Sign: -1, Note: "CKPN/PPAP kredit mengurangi Kredit yang Diberikan"},
	{COACode: "10999", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Akun Sementara (Suspense); diusulkan ke Aset Lainnya"},

	// ── Form 01.00: ASET SYARIAH ────────────────────────────────────────────
	{COACode: "11100", Form: "01.00", Sandi: "1101010000", Sign: 1, Note: "Kas Syariah -> Kas dalam Rupiah"},
	{COACode: "11200", Form: "01.00", Sandi: "1103010000", Sign: 1, Note: "Penempatan pada Bank Syariah -> Penempatan pada Bank Lain"},
	{COACode: "11300", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Pembiayaan Murabahah -> Kredit yang Diberikan"},
	{COACode: "11310", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Murabahah - Piutang -> Kredit yang Diberikan"},
	{COACode: "11320", Form: "01.00", Sandi: "1104010400", Sign: -1, Verified: false,
		Note: "Margin Murabahah ditangguhkan; dipetakan ke pos pendapatan ditangguhkan, perlu konfirmasi perlakuannya"},
	{COACode: "11400", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Pembiayaan Mudharabah -> Kredit yang Diberikan"},
	{COACode: "11500", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Pembiayaan Musyarakah -> Kredit yang Diberikan"},
	{COACode: "11600", Form: "01.00", Sandi: "1104010100", Sign: 1, Note: "Pembiayaan Ijarah -> Kredit yang Diberikan"},
	{COACode: "11700", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Piutang Denda Syariah; diusulkan ke Aset Lainnya"},
	{COACode: "11900", Form: "01.00", Sandi: "1104020000", Sign: -1, Note: "Cadangan Kerugian Pembiayaan mengurangi Kredit yang Diberikan"},

	// ── Form 01.00: LIABILITAS ──────────────────────────────────────────────
	{COACode: "20100", Form: "01.00", Sandi: "2102010100", Sign: 1, Note: "Tabungan -> Simpanan a. Tabungan"},
	{COACode: "20200", Form: "01.00", Sandi: "2102020100", Sign: 1, Note: "Deposito Berjangka -> Simpanan b. Deposito"},
	{COACode: "20300", Form: "01.00", Sandi: "2299000000", Sign: 1, Verified: false,
		Note: "Giro tidak punya pos tersendiri pada form BPR; diusulkan ke Liabilitas Lainnya"},
	{COACode: "20400", Form: "01.00", Sandi: "2101000000", Sign: 1, Verified: false,
		Note: "Bunga deposito masih harus dibayar; diusulkan ke Liabilitas Segera"},
	{COACode: "20500", Form: "01.00", Sandi: "2101000000", Sign: 1, Verified: false,
		Note: "Utang Pajak; diusulkan ke Liabilitas Segera"},
	{COACode: "20600", Form: "01.00", Sandi: "2101000000", Sign: 1, Verified: false,
		Note: "Utang Bunga; diusulkan ke Liabilitas Segera"},
	{COACode: "20700", Form: "01.00", Sandi: "2299000000", Sign: 1, Note: "Utang Lainnya -> Liabilitas Lainnya"},
	{COACode: "20800", Form: "01.00", Sandi: "2203000000", Sign: 1, Note: "Rekening Antar Kantor - Kredit -> Liabilitas Antarkantor"},

	// ── Form 01.00: LIABILITAS SYARIAH ──────────────────────────────────────
	{COACode: "12100", Form: "01.00", Sandi: "2102010100", Sign: 1, Note: "Tabungan Wadiah -> Simpanan a. Tabungan"},
	{COACode: "12200", Form: "01.00", Sandi: "2102010100", Sign: 1, Note: "Tabungan Mudharabah -> Simpanan a. Tabungan"},
	{COACode: "12300", Form: "01.00", Sandi: "2102020100", Sign: 1, Note: "Deposito Mudharabah -> Simpanan b. Deposito"},
	{COACode: "12400", Form: "01.00", Sandi: "2101000000", Sign: 1, Verified: false,
		Note: "Bagi hasil masih harus dibayar; diusulkan ke Liabilitas Segera"},
	{COACode: "12500", Form: "01.00", Sandi: "2299000000", Sign: 1, Verified: false,
		Note: "Dana Kebajikan; diusulkan ke Liabilitas Lainnya"},
	{COACode: "12900", Form: "01.00", Sandi: "2299000000", Sign: 1, Note: "Kewajiban Lainnya Syariah -> Liabilitas Lainnya"},

	// ── Form 01.00: EKUITAS ─────────────────────────────────────────────────
	{COACode: "30100", Form: "01.00", Sandi: "3101010000", Sign: 1, Note: "Modal Disetor -> Modal Dasar"},
	{COACode: "30200", Form: "01.00", Sandi: "3105010000", Sign: 1, Note: "Laba Ditahan -> Laba (rugi) Tahun-Tahun Lalu"},
	{COACode: "30300", Form: "01.00", Sandi: "3105020000", Sign: 1, Note: "Laba Tahun Berjalan"},
	{COACode: "30400", Form: "01.00", Sandi: "3104010000", Sign: 1, Note: "Cadangan Umum -> Cadangan a. Umum"},
	{COACode: "13100", Form: "01.00", Sandi: "3101010000", Sign: 1, Note: "Modal Disetor Syariah -> Modal Dasar"},
	{COACode: "13200", Form: "01.00", Sandi: "3105010000", Sign: 1, Note: "Laba Ditahan Syariah -> Laba Tahun-Tahun Lalu"},

	// Baris penyeimbang dari reporting repo: kode "-" = Laba/Rugi Berjalan.
	// Nilainya memang laba rugi berjalan yang belum ditutup ke ekuitas.
	{COACode: "-", Form: "01.00", Sandi: "3105020000", Sign: 1,
		Note: "baris penyeimbang laba/rugi berjalan dari domain.BalanceSheet"},

	// ── Form 02.00: PENDAPATAN OPERASIONAL ──────────────────────────────────
	{COACode: "40100", Form: "02.00", Sandi: "4101010302", Sign: 1, Verified: false,
		Note: "Pendapatan Bunga Kredit; diusulkan ke bunga kredit pihak ketiga bukan bank"},
	{COACode: "40200", Form: "02.00", Sandi: "4101010203", Sign: 1, Verified: false,
		Note: "Pendapatan Bunga Penempatan; diusulkan ke deposito (rincian giro/tabungan/deposito belum dipisah)"},
	{COACode: "40300", Form: "02.00", Sandi: "4101020200", Sign: 1, Verified: false,
		Note: "Pendapatan Provisi dan Komisi; diusulkan ke Provisi Kredit pihak ketiga (perlu konfirmasi jasa transaksi)"},
	{COACode: "40400", Form: "02.00", Sandi: "4102010000", Sign: 1, Note: "Pendapatan Administrasi -> Pendapatan Jasa Transaksi"},
	{COACode: "40500", Form: "02.00", Sandi: "4102990000", Sign: 1, Verified: false,
		Note: "Pendapatan Denda; diusulkan ke Pendapatan Lainnya - Lainnya"},
	{COACode: "40900", Form: "02.00", Sandi: "4102990000", Sign: 1, Note: "Pendapatan Lainnya -> Pendapatan Lainnya - Lainnya"},
	{COACode: "14100", Form: "02.00", Sandi: "4101010302", Sign: 1, Verified: false,
		Note: "Margin Murabahah; form BPR(S) syariah belum dipetakan rinci"},
	{COACode: "14200", Form: "02.00", Sandi: "4101010302", Sign: 1, Verified: false,
		Note: "Bagi Hasil Mudharabah; form BPR(S) syariah belum dipetakan rinci"},
	{COACode: "14300", Form: "02.00", Sandi: "4101010302", Sign: 1, Verified: false,
		Note: "Bagi Hasil Musyarakah; form BPR(S) syariah belum dipetakan rinci"},
	{COACode: "14400", Form: "02.00", Sandi: "4102990000", Sign: 1, Verified: false,
		Note: "Pendapatan Ijarah; form BPR(S) syariah belum dipetakan rinci"},
	{COACode: "14500", Form: "02.00", Sandi: "4102010000", Sign: 1, Note: "Pendapatan Administrasi Syariah -> Pendapatan Jasa Transaksi"},
	{COACode: "14600", Form: "02.00", Sandi: "4102990000", Sign: 1, Verified: false,
		Note: "Pendapatan Denda Syariah; secara syariah dana denda adalah dana sosial, perlu konfirmasi"},
	{COACode: "14900", Form: "02.00", Sandi: "4102990000", Sign: 1, Note: "Pendapatan Lainnya Syariah -> Pendapatan Lainnya - Lainnya"},

	// ── Form 02.00: BEBAN OPERASIONAL ───────────────────────────────────────
	{COACode: "50100", Form: "02.00", Sandi: "5101010200", Sign: 1, Note: "Beban Bunga Deposito -> Beban Bunga Kontraktual Deposito"},
	{COACode: "50200", Form: "02.00", Sandi: "5103030200", Sign: 1, Note: "Beban Penyisihan Kerugian Kredit -> Beban Kerugian Penurunan Nilai kredit pihak ketiga"},
	{COACode: "50300", Form: "02.00", Sandi: "5106010100", Sign: 1, Note: "Beban Gaji dan Tunjangan -> Gaji dan Upah"},
	{COACode: "50400", Form: "02.00", Sandi: "5106080000", Sign: 1, Verified: false,
		Note: "Beban Umum dan Administrasi belum dirinci; diusulkan ke Beban Barang dan Jasa"},
	{COACode: "50500", Form: "02.00", Sandi: "5106040000", Sign: 1, Note: "Beban Penyusutan -> Beban Penyusutan/Penghapusan Aset Tetap"},
	{COACode: "50900", Form: "02.00", Sandi: "5199990000", Sign: 1, Note: "Beban Lainnya -> Beban Lainnya - Lainnya"},
	{COACode: "15100", Form: "02.00", Sandi: "5101010200", Sign: 1, Verified: false,
		Note: "Bagi Hasil untuk Pemilik Dana; diusulkan ke beban bunga deposito, form syariah belum dipetakan rinci"},
	{COACode: "15200", Form: "02.00", Sandi: "5103030200", Sign: 1, Note: "Beban Penyisihan Kerugian Pembiayaan -> Beban Kerugian Penurunan Nilai kredit"},
	{COACode: "15900", Form: "02.00", Sandi: "5199990000", Sign: 1, Note: "Beban Lainnya Syariah -> Beban Lainnya - Lainnya"},

	// ── Form 02.00: PAJAK (di bawah laba sebelum pajak) ─────────────────────
	{COACode: "60100", Form: "02.00", Sandi: "5300000000", Sign: 1, Note: "Beban Pajak Penghasilan -> Taksiran Pajak Penghasilan"},
}

// defaultMappingIndex adalah indeks COACode->entry untuk pemetaan bawaan.
var defaultMappingIndex = buildMappingIndex(COAMappingDraft)

// buildMappingIndex membangun indeks satu entri per kode COA.
func buildMappingIndex(entries []MappingEntry) map[string]MappingEntry {
	index := make(map[string]MappingEntry, len(entries))
	for _, e := range entries {
		index[e.COACode] = e
	}
	return index
}

// COACodesForForm mengembalikan kode COA yang dipetakan ke form tertentu, terurut.
// Dipakai validasi kelengkapan dan pengujian.
func COACodesForForm(entries []MappingEntry, form string) []string {
	codes := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Form == form {
			codes = append(codes, e.COACode)
		}
	}
	sort.Strings(codes)
	return codes
}

// DuplicateCOACodes mengembalikan kode COA yang muncul lebih dari sekali pada
// daftar pemetaan. Satu kode harus dipetakan tepat ke satu pos agar tidak ada
// operan yang saling meniadakan tanpa terlihat.
func DuplicateCOACodes(entries []MappingEntry) []string {
	seen := make(map[string]int, len(entries))
	for _, e := range entries {
		seen[e.COACode]++
	}
	var dup []string
	for code, n := range seen {
		if n > 1 {
			dup = append(dup, code)
		}
	}
	sort.Strings(dup)
	return dup
}
