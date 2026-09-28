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

// Kelas modal pada pemetaan bagan akun (POJK No. 5/POJK.03/2015 Pasal 3, 5, dan 10;
// SEOJK No. 2/SEOJK.03/2025 Bagian II.3 dan IV.2). Nilai kosong berarti kelasnya
// BELUM PASTI dan sengaja tidak diisi: kode berkelas kosong tidak boleh
// diperhitungkan sebagai modal.
const (
	// ModalClassIntiUtama: modal disetor, agio, dana setoran modal (Pasal 6), modal
	// sumbangan (Pasal 7), cadangan umum/tujuan ber-RUPS, laba tahun lalu & berjalan.
	ModalClassIntiUtama = "INTI_UTAMA"
	// ModalClassIntiTambahan: instrumen yang menuntut persetujuan OJK (Pasal 5 ayat
	// (2)); daftarnya MANUAL per bank, tidak boleh ditebak dari saldo.
	ModalClassIntiTambahan = "INTI_TAMBAHAN"
	// ModalClassPelengkap: instrumen ber-persetujuan OJK, surplus revaluasi aset tetap,
	// dan PPKA umum (Pasal 10).
	ModalClassPelengkap = "PELENGKAP"
	// ModalClassPengurang: pajak tangguhan, goodwill, disagio, AYDA/properti
	// terbengkalai >1 tahun, rugi tahun lalu/berjalan, selisih PPKA-CKPN (Pasal 5
	// ayat (4); SEOJK 2/2025 Bagian IV.2).
	ModalClassPengurang = "PENGURANG"
)

// ModalClassValid melaporkan apakah nilai kelas modal termasuk himpunan yang dikenal.
func ModalClassValid(class string) bool {
	switch class {
	case "", ModalClassIntiUtama, ModalClassIntiTambahan, ModalClassPelengkap, ModalClassPengurang:
		return true
	default:
		return false
	}
}

// Catatan kelengkapan: bagan akun baku belum memisahkan sejumlah komponen modal,
// sehingga kelasnya sengaja DIBIARKAN KOSONG dan tidak ditebak:
//   - INTI_TAMBAHAN: instrumen dengan persetujuan OJK (daftar manual per bank).
//   - PELENGKAP: agio/disagio tidak dipisah dari modal, surplus revaluasi aset tetap
//     belum berakun tersendiri, dan PPKA umum dihitung dari aset produktif lancar.
//   - PENGURANG: pajak tangguhan, goodwill, disagio, AYDA/properti terbengkalai >1
//     tahun (umur AYDA tidak tersedia pada bagan akun), rugi tahun lalu/berjalan
//     (tanda saldo), dan selisih PPKA-CKPN yang dihitung modul CKPN, bukan saldo COA.

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
	// ModalClass adalah kelas modal kode COA ini (lihat konstanta ModalClass*).
	// Kosong berarti belum pasti dan TIDAK boleh diperhitungkan sebagai modal.
	ModalClass string
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
	{COACode: "10310", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Premi Penjaminan LPS Dibayar di Muka (migrasi 000114); Aset Lainnya Form 09.00"},
	{COACode: "10320", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Uang Muka Pajak (migrasi 000114); Aset Lainnya Form 09.00"},
	{COACode: "10330", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Aset Pajak Tangguhan (migrasi 000114); Aset Lainnya Form 09.00"},
	{COACode: "10340", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Biaya Dibayar di Muka (migrasi 000114); Aset Lainnya Form 09.00"},
	{COACode: "10350", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Tagihan kepada Perusahaan Asuransi (migrasi 000114); Aset Lainnya Form 09.00"},
	{COACode: "10360", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Uang Muka untuk Kegiatan Operasional (migrasi 000114); Aset Lainnya Form 09.00"},
	{COACode: "10400", Form: "01.00", Sandi: "1299000000", Sign: 1, Verified: false,
		Note: "Bunga kredit masih akan diterima; diusulkan ke Aset Lainnya (perlu konfirmasi apakah digabung ke Kredit)"},
	{COACode: "10500", Form: "01.00", Sandi: "1201000000", Sign: 1, Note: "Agunan yang Diambil Alih"},
	{COACode: "10600", Form: "01.00", Sandi: "1202010000", Sign: 1, Note: "Aset Tetap dan Inventaris"},
	{COACode: "10700", Form: "01.00", Sandi: "1202020000", Sign: -1, Note: "Akumulasi Penyusutan mengurangi aset tetap"},
	{COACode: "10800", Form: "01.00", Sandi: "1204000000", Sign: 1, Note: "Aset Antarkantor"},
	{COACode: "10900", Form: "01.00", Sandi: "1104020000", Sign: -1, Note: "CKPN/PPAP kredit mengurangi Kredit yang Diberikan"},
	{COACode: "10950", Form: "01.00", Sandi: "1104020000", Sign: -1, Verified: false,
		Note: "CKPN - Kredit: akun cadangan mesin CKPN (migrasi 000042), kontra-aset atas Kredit yang Diberikan; perlakuan sama dengan 10900 (PPAP). Tanpa pemetaan ini saldonya hilang dari Form 01.00 begitu CKPN menyala."},
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
	{COACode: "11950", Form: "01.00", Sandi: "1104020000", Sign: -1, Verified: false,
		Note: "CKPN - Pembiayaan (buku syariah, migrasi 000042): kontra-aset pembiayaan; perlakuan sama dengan 11900"},

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
	// Modal inti utama (POJK 5/2015 Pasal 5 ayat (1)). Laba/rugi dipetakan ke
	// INTI_UTAMA; saldo negatifnya (rugi tahun lalu/berjalan) adalah pengurang modal
	// inti menurut Pasal 5 ayat (4) huruf e/f, tetapi kelas statis di sini tidak dapat
	// membedakan tanda sehingga tanda itu ditangani pemakainya, bukan diisi ke kelas
	// PENGURANG secara buta.
	{COACode: "30100", Form: "01.00", Sandi: "3101010000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "Modal Disetor -> Modal Dasar (Pasal 5 ayat (1) huruf a)"},
	{COACode: "30200", Form: "01.00", Sandi: "3105010000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "Laba Ditahan -> Laba (rugi) Tahun-Tahun Lalu (Pasal 5 ayat (1) huruf b angka 6); saldo negatif = pengurang modal inti (Pasal 5 ayat (4) huruf e)"},
	{COACode: "30300", Form: "01.00", Sandi: "3105020000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "Laba Tahun Berjalan (Pasal 5 ayat (1) huruf b angka 7; paling tinggi 50% setelah taksiran pajak); saldo negatif = pengurang modal inti (Pasal 5 ayat (4) huruf f)"},
	{COACode: "30400", Form: "01.00", Sandi: "3104010000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "Cadangan Umum -> Cadangan a. Umum (Pasal 5 ayat (1) huruf b angka 4; wajib persetujuan RUPS)"},
	{COACode: "13100", Form: "01.00", Sandi: "3101010000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "Modal Disetor Syariah -> Modal Dasar (Pasal 5 ayat (1) huruf a)"},
	{COACode: "13200", Form: "01.00", Sandi: "3105010000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "Laba Ditahan Syariah -> Laba Tahun-Tahun Lalu (Pasal 5 ayat (1) huruf b angka 6)"},

	// Baris penyeimbang dari reporting repo: kode "-" = Laba/Rugi Berjalan.
	// Nilainya memang laba rugi berjalan yang belum ditutup ke ekuitas.
	{COACode: "-", Form: "01.00", Sandi: "3105020000", Sign: 1, ModalClass: ModalClassIntiUtama,
		Note: "baris penyeimbang laba/rugi berjalan dari domain.BalanceSheet (Pasal 5 ayat (1) huruf b angka 7)"},

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
	// Akun CKPN dan restrukturisasi (migrasi 000042/000043) berdiri sendiri dari
	// PPAP 50200; posnya mengikuti pemakaian 50200/15200 agar satu jenis beban tidak
	// terpecah dua pos.
	{COACode: "50301", Form: "02.00", Sandi: "5103030200", Sign: 1, Verified: false,
		Note: "Beban Kerugian Penurunan Nilai - Kredit (sisi debit pembentukan CKPN, migrasi 000042); pos sama dengan 50200"},
	{COACode: "50401", Form: "02.00", Sandi: "5102000000", Sign: 1, Verified: false,
		Note: "Beban Kerugian Restrukturisasi Kredit (migrasi 000043) -> pos Form 02.00 bernama sama persis"},
	{COACode: "50300", Form: "02.00", Sandi: "5106010100", Sign: 1, Note: "Beban Gaji dan Tunjangan -> Gaji dan Upah"},
	{COACode: "50400", Form: "02.00", Sandi: "5106080000", Sign: 1, Verified: false,
		Note: "Beban Umum dan Administrasi belum dirinci; diusulkan ke Beban Barang dan Jasa"},
	{COACode: "50500", Form: "02.00", Sandi: "5106040000", Sign: 1, Note: "Beban Penyusutan -> Beban Penyusutan/Penghapusan Aset Tetap"},
	{COACode: "50900", Form: "02.00", Sandi: "5199990000", Sign: 1, Note: "Beban Lainnya -> Beban Lainnya - Lainnya"},
	{COACode: "15100", Form: "02.00", Sandi: "5101010200", Sign: 1, Verified: false,
		Note: "Bagi Hasil untuk Pemilik Dana; diusulkan ke beban bunga deposito, form syariah belum dipetakan rinci"},
	{COACode: "15200", Form: "02.00", Sandi: "5103030200", Sign: 1, Note: "Beban Penyisihan Kerugian Pembiayaan -> Beban Kerugian Penurunan Nilai kredit"},
	{COACode: "15901", Form: "02.00", Sandi: "5103030200", Sign: 1, Verified: false,
		Note: "Beban Kerugian Penurunan Nilai - Pembiayaan (migrasi 000042); pos sama dengan 15200"},
	{COACode: "15902", Form: "02.00", Sandi: "5102000000", Sign: 1, Verified: false,
		Note: "Beban Kerugian Restrukturisasi Pembiayaan (migrasi 000043); pos sama dengan 50401"},
	{COACode: "15900", Form: "02.00", Sandi: "5199990000", Sign: 1, Note: "Beban Lainnya Syariah -> Beban Lainnya - Lainnya"},

	// ── Form 02.00: PAJAK (di bawah laba sebelum pajak) ─────────────────────
	{COACode: "60100", Form: "02.00", Sandi: "5300000000", Sign: 1, Note: "Beban Pajak Penghasilan -> Taksiran Pajak Penghasilan"},
}

// COAMapping09Draft adalah pemetaan tersendiri pos Aset Lainnya (COA bersandi
// 1299000000 pada Form 01.00) ke susunan Form 09.00. Slice ini SENGAJA terpisah dari
// COAMappingDraft: DuplicateCOACodes dan defaultMappingIndex (dipakai KPMM/ModalClass)
// mewajibkan satu kode COA hanya punya satu entri, sedangkan satu akun Aset Lainnya
// perlu muncul pada dua form. Setiap entri di sini berpasangan dengan tepat satu entri
// COAMappingDraft bersandi 1299000000; invarian cakupan itu ditegakkan
// TestForm09PemetaanSejajarDenganForm01 agar total Form 09.00 selalu sama dengan pos
// Aset Lainnya Form 01.00.
//
// Pos 1299010100 (a. Penempatan pada Bank Lain), 1299010300 (c. Surat Berharga), dan
// 1299010900 (d. Lainnya) tidak punya entri di sini karena bagan akun belum punya
// akunnya; buildForm09 menuliskannya sebagai baris tidak tersedia beserta alasannya,
// bukan nol atau angka karangan.
var COAMapping09Draft = []MappingEntry{
	{COACode: "10305", Form: "09.00", Sandi: "1299990000", Sign: 1, Verified: false,
		Note: "Piutang Denda -> Lainnya; pos resmi Form 09.00 tidak punya baris piutang denda"},
	{COACode: "10310", Form: "09.00", Sandi: "1299020000", Sign: 1, Verified: false,
		Note: "Premi Penjaminan LPS Dibayar di Muka -> pos resmi bernama sama persis"},
	{COACode: "10320", Form: "09.00", Sandi: "1299030000", Sign: 1, Verified: false,
		Note: "Uang Muka Pajak -> pos resmi bernama sama persis"},
	{COACode: "10330", Form: "09.00", Sandi: "1299040000", Sign: 1, Verified: false,
		Note: "Aset Pajak Tangguhan -> pos resmi bernama sama persis"},
	{COACode: "10340", Form: "09.00", Sandi: "1299050000", Sign: 1, Verified: false,
		Note: "Biaya Dibayar di Muka -> pos resmi bernama sama persis"},
	{COACode: "10350", Form: "09.00", Sandi: "1299060000", Sign: 1, Verified: false,
		Note: "Tagihan kepada Perusahaan Asuransi -> pos resmi bernama sama persis"},
	{COACode: "10360", Form: "09.00", Sandi: "1299070000", Sign: 1, Verified: false,
		Note: "Uang Muka untuk Kegiatan Operasional -> pos resmi bernama sama persis"},
	{COACode: "10400", Form: "09.00", Sandi: "1299010200", Sign: 1, Verified: false,
		Note: "Bunga Kredit yang Masih Akan Diterima -> b. Kredit yang Diberikan"},
	{COACode: "10999", Form: "09.00", Sandi: "1299990000", Sign: 1, Verified: false,
		Note: "Akun Sementara (Suspense) -> Lainnya; pos resmi Form 09.00 tidak punya baris akun sementara"},
	{COACode: "11700", Form: "09.00", Sandi: "1299990000", Sign: 1, Verified: false,
		Note: "Piutang Denda Syariah -> Lainnya; pos resmi Form 09.00 tidak punya baris piutang denda"},
}

// COAMapping14Draft adalah pemetaan tersendiri kode COA ke baris Form 14.00 "RINCIAN
// LIABILITAS LAINNYA" (PDF #page 213-215). Slice ini SENGAJA terpisah dari
// COAMappingDraft: satu kode COA memang muncul pada beberapa form (mis. 20700 ada di
// Form 01.00 dan Form 14.00), sedangkan DuplicateCOACodes/defaultMappingIndex mewajibkan
// satu entri per kode di COAMappingDraft. Setiap entri di sini tidak menggantikan entri
// Form 01.00.
//
// Hanya akun yang SUDAH ADA dan namanya secara tidak ambigu cocok dengan nama pos resmi
// Form 14.00 yang dipetakan:
//   - 20500 "Utang Pajak" -> 2299020000 "Utang Pajak": nama sama persis.
//   - 20700 "Utang Lainnya" -> 2299990000 "Lainnya": akun ini sudah dipetakan ke pos
//     Liabilitas Lainnya Form 01.00 (2299000000), jadi jelas bagian dari rincian ini,
//     dan pos resmi "Lainnya" adalah penampung resminya.
//
// Pos lain tidak dipetakan karena bagan akun belum punya akunnya atau namanya ambigu,
// sehingga buildForm14 menuliskannya sebagai baris tidak tersedia beserta alasannya,
// bukan nol dan bukan tebakan. Utang Bunga (2299010100-2299019900) hanya punya akun
// agregat 20400/20600 yang tidak menyimpan status jatuh tempo; Giro (20300), Dana
// Kebajikan (12500), dan Kewajiban Lainnya Syariah (12900) sengaja TIDAK dipetakan ke
// "Lainnya" karena namanya tidak cocok dengan pos resmi mana pun (berbeda dari Form 09.00
// yang memakai "Lainnya" sebagai penampung resmi akun tanpa baris tersendiri).
//
// Catatan: 20500 saat ini masih dipetakan Form 01.00 ke Liabilitas Segera (2101000000).
// Sampai bank memperbaiki pedoman konversinya, angka Utang Pajak dapat muncul pada dua
// form; hal itu dicatat di Notes Form 14.00, bukan disembunyikan.
var COAMapping14Draft = []MappingEntry{
	{COACode: "20500", Form: "14.00", Sandi: "2299020000", Sign: 1, Verified: false,
		Note: "Utang Pajak -> pos resmi bernama sama persis; Form 01.00 masih menaruhnya di Liabilitas Segera"},
	{COACode: "20700", Form: "14.00", Sandi: "2299990000", Sign: 1, Verified: false,
		Note: "Utang Lainnya -> Lainnya; sudah dipetakan ke Liabilitas Lainnya Form 01.00 (2299000000)"},
}

// COAMapping10Draft adalah pemetaan tersendiri kode COA ke baris Form 10.00 "RINCIAN
// LIABILITAS SEGERA" (PDF #page 191-192). Slice ini SENGAJA terpisah dari
// COAMappingDraft: satu kode COA memang muncul pada beberapa form (mis. 20400/20600 ada
// di Form 00.18), sedangkan DuplicateCOACodes/defaultMappingIndex mewajibkan satu entri
// per kode di COAMappingDraft. Setiap entri di sini tidak menggantikan entri Form 01.00.
//
// Form 10.00 adalah rincian pos Liabilitas Segera (2101000000) Form 01.00, yang saat
// ini diisi empat akun: 20400 Bunga Deposito yang Masih Harus Dibayar, 20500 Utang
// Pajak, 20600 Utang Bunga, dan 12400 Bagi Hasil Masih Harus Dibayar.
//
// Hanya akun yang tidak ambigu atau tidak punya pos resmi lain yang dipetakan:
//   - 20400 "Bunga Deposito yang Masih Harus Dibayar", 20600 "Utang Bunga", dan 12400
//     "Bagi Hasil Masih Harus Dibayar" -> 2101990000 "Lainnya": ketiganya komponen
//     Liabilitas Segera Form 01.00, tetapi bagan akun tidak punya pos Form 10.00 yang
//     bernama lebih spesifik; "Lainnya" adalah penampung resminya (PDF #page 192:
//     "antara lain ...").
//   - 20500 "Utang Pajak" SENGAJA TIDAK dipetakan. Secara nama pos 1 (2101010000
//     "Liabilitas kepada Pemerintah yang Harus Dibayar") adalah rumahnya, tetapi
//     definisi PDF #page 192 menuntut pajak "untuk periode sebelum bulan laporan yang
//     dibayarkan pada bulan laporan" (arus pembayaran bulan berjalan), sedangkan yang
//     tersedia hanyalah SALDO periodEnd. Saldo itu tidak sepadan, jadi tidak dipaksa
//     ke pos 1; memindahkannya ke "Lainnya" juga akan salah golong (bukan penampung
//     tanpa pos) dan menggandakan laporan karena 20500 sudah menjadi pos Utang Pajak
//     (2299020000) Form 14.00.
//
// Pos 2101020000-2101070000 tidak dipetakan karena bagan akun belum punya akunnya;
// buildForm10 menuliskannya sebagai baris tidak tersedia beserta alasannya, bukan nol.
var COAMapping10Draft = []MappingEntry{
	{COACode: "20400", Form: "10.00", Sandi: "2101990000", Sign: 1, Verified: false,
		Note: "Bunga Deposito yang Masih Harus Dibayar -> Lainnya; komponen Liabilitas Segera Form 01.00 tetapi tidak ada pos Form 10.00 yang lebih spesifik"},
	{COACode: "20600", Form: "10.00", Sandi: "2101990000", Sign: 1, Verified: false,
		Note: "Utang Bunga -> Lainnya; tidak ada baris utang bunga pada Form 10.00"},
	{COACode: "12400", Form: "10.00", Sandi: "2101990000", Sign: 1, Verified: false,
		Note: "Bagi Hasil Masih Harus Dibayar -> Lainnya; tidak ada baris bagi hasil pada Form 10.00"},
}

// COAMapping18Draft adalah pemetaan tersendiri kode COA ke baris arus kas Form 00.18
// "LAPORAN ARUS KAS" menurut susunan Form 00.18 – 1 (PDF #page 295-296). Slice ini
// SENGAJA terpisah dari COAMappingDraft: satu kode COA memang muncul pada beberapa form
// (mis. akun pendapatan bunga ada di Form 02.00 dan Form 00.18), sedangkan
// DuplicateCOACodes/defaultMappingIndex mewajibkan satu entri per kode di
// COAMappingDraft. Sama seperti COAMapping09Draft, setiap entri di sini tidak
// menggantikan entri Form 01.00/02.00.
//
// Nominal pada domain.CashFlow.Rows SUDAH bertanda relatif terhadap kas: lawan jurnal
// bersisi KREDIT (penerimaan) positif, bersisi DEBIT (pembayaran) negatif
// (reporting_repo.go:290-296). Karena itu seluruh Sign = +1 dan baris "neto" cukup
// menjumlahkan baris rinciannya.
//
// Kode kas dan setara kas (10100, 10101, 10200, 11100, 11200) SENGAJA tidak dipetakan:
// sumber arus kas mengeluarkan kode itu dari lawan jurnal (reporting_repo.go:24-28),
// sehingga perubahannya tidak pernah muncul pada baris. Baris 14140000 (Penempatan pada
// bank lain) karena itu ditulis tidak tersedia, bukan nol.
var COAMapping18Draft = []MappingEntry{
	// ── Arus kas operasi: penerimaan/pembayaran pendapatan dan beban (PDF #page 295) ──
	{COACode: "40100", Form: "00.18", Sandi: "14010000", Sign: 1, Note: "Pendapatan Bunga Kredit -> Penerimaan pendapatan bunga"},
	{COACode: "40200", Form: "00.18", Sandi: "14010000", Sign: 1, Note: "Pendapatan Bunga Penempatan -> Penerimaan pendapatan bunga"},
	{COACode: "14100", Form: "00.18", Sandi: "14010000", Sign: 1, Note: "Margin Murabahah -> Penerimaan pendapatan bunga"},
	{COACode: "14200", Form: "00.18", Sandi: "14010000", Sign: 1, Note: "Bagi Hasil Mudharabah -> Penerimaan pendapatan bunga"},
	{COACode: "14300", Form: "00.18", Sandi: "14010000", Sign: 1, Note: "Bagi Hasil Musyarakah -> Penerimaan pendapatan bunga"},
	{COACode: "14400", Form: "00.18", Sandi: "14010000", Sign: 1, Note: "Pendapatan Ijarah -> Penerimaan pendapatan bunga"},
	{COACode: "40300", Form: "00.18", Sandi: "14020000", Sign: 1, Note: "Pendapatan Provisi dan Komisi -> Penerimaan pendapatan provisi dan jasa transaksi"},
	{COACode: "40400", Form: "00.18", Sandi: "14020000", Sign: 1, Note: "Pendapatan Administrasi -> Penerimaan pendapatan provisi dan jasa transaksi"},
	{COACode: "14500", Form: "00.18", Sandi: "14020000", Sign: 1, Note: "Pendapatan Administrasi Syariah -> Penerimaan pendapatan provisi dan jasa transaksi"},
	{COACode: "40500", Form: "00.18", Sandi: "14050000", Sign: 1, Note: "Pendapatan Denda -> Pendapatan operasional lainnya"},
	{COACode: "40900", Form: "00.18", Sandi: "14050000", Sign: 1, Note: "Pendapatan Lainnya -> Pendapatan operasional lainnya"},
	{COACode: "14600", Form: "00.18", Sandi: "14050000", Sign: 1, Note: "Pendapatan Denda Syariah -> Pendapatan operasional lainnya"},
	{COACode: "14900", Form: "00.18", Sandi: "14050000", Sign: 1, Note: "Pendapatan Lainnya Syariah -> Pendapatan operasional lainnya"},
	{COACode: "50100", Form: "00.18", Sandi: "14060000", Sign: 1, Note: "Beban Bunga Deposito -> Pembayaran beban bunga"},
	{COACode: "15100", Form: "00.18", Sandi: "14060000", Sign: 1, Note: "Bagi Hasil untuk Pemilik Dana -> Pembayaran beban bunga"},
	{COACode: "50300", Form: "00.18", Sandi: "14070000", Sign: 1, Note: "Beban Gaji dan Tunjangan -> Beban gaji dan tunjangan"},
	{COACode: "50400", Form: "00.18", Sandi: "14080000", Sign: 1, Note: "Beban Umum dan Administrasi -> Beban umum dan administrasi"},
	{COACode: "50200", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Penyisihan Kerugian Kredit -> Beban operasional lainnya"},
	{COACode: "50301", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Kerugian Penurunan Nilai - Kredit -> Beban operasional lainnya"},
	{COACode: "50401", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Kerugian Restrukturisasi Kredit -> Beban operasional lainnya"},
	{COACode: "50500", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Penyusutan -> Beban operasional lainnya"},
	{COACode: "50900", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Lainnya -> Beban operasional lainnya"},
	{COACode: "15200", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Penyisihan Kerugian Pembiayaan -> Beban operasional lainnya"},
	{COACode: "15900", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Lainnya Syariah -> Beban operasional lainnya"},
	{COACode: "15901", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Kerugian Penurunan Nilai - Pembiayaan -> Beban operasional lainnya"},
	{COACode: "15902", Form: "00.18", Sandi: "14090000", Sign: 1, Note: "Beban Kerugian Restrukturisasi Pembiayaan -> Beban operasional lainnya"},
	{COACode: "60100", Form: "00.18", Sandi: "14120000", Sign: 1, Note: "Beban Pajak Penghasilan -> Pembayaran pajak penghasilan"},

	// ── Arus kas operasi: penurunan/peningkatan aset operasional (PDF #page 295) ──
	{COACode: "10300", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Kredit yang Diberikan -> Kredit yang diberikan"},
	{COACode: "10301", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Kredit - Pokok -> Kredit yang diberikan"},
	{COACode: "11300", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Pembiayaan Murabahah -> Kredit yang diberikan"},
	{COACode: "11310", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Murabahah - Piutang -> Kredit yang diberikan"},
	{COACode: "11400", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Pembiayaan Mudharabah -> Kredit yang diberikan"},
	{COACode: "11500", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Pembiayaan Musyarakah -> Kredit yang diberikan"},
	{COACode: "11600", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Pembiayaan Ijarah -> Kredit yang diberikan"},
	{COACode: "10900", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "CKPN/PPAP Kredit (kontra kredit) -> Kredit yang diberikan"},
	{COACode: "10950", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "CKPN - Kredit (kontra kredit) -> Kredit yang diberikan"},
	{COACode: "11900", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "Cadangan Kerugian Pembiayaan (kontra kredit) -> Kredit yang diberikan"},
	{COACode: "11950", Form: "00.18", Sandi: "14150000", Sign: 1, Note: "CKPN - Pembiayaan (kontra kredit) -> Kredit yang diberikan"},
	{COACode: "10500", Form: "00.18", Sandi: "14160000", Sign: 1, Note: "Agunan yang Diambil Alih -> Agunan yang diambil alih"},
	{COACode: "10305", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Piutang Denda -> Aset lain-lain"},
	{COACode: "10310", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Premi Penjaminan LPS Dibayar di Muka -> Aset lain-lain"},
	{COACode: "10320", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Uang Muka Pajak -> Aset lain-lain"},
	{COACode: "10330", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Aset Pajak Tangguhan -> Aset lain-lain"},
	{COACode: "10340", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Biaya Dibayar di Muka -> Aset lain-lain"},
	{COACode: "10350", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Tagihan kepada Perusahaan Asuransi -> Aset lain-lain"},
	{COACode: "10360", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Uang Muka untuk Kegiatan Operasional -> Aset lain-lain"},
	{COACode: "10400", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Bunga Kredit Masih Akan Diterima -> Aset lain-lain"},
	{COACode: "10800", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Aset Antarkantor -> Aset lain-lain"},
	{COACode: "10999", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Akun Sementara (Suspense) -> Aset lain-lain"},
	{COACode: "11700", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Piutang Denda Syariah -> Aset lain-lain"},
	{COACode: "11320", Form: "00.18", Sandi: "14170000", Sign: 1, Note: "Margin Murabahah Ditangguhkan -> Aset lain-lain"},

	// ── Arus kas operasi: kenaikan/peningkatan liabilitas operasional (PDF #page 295) ──
	{COACode: "20400", Form: "00.18", Sandi: "14190000", Sign: 1, Note: "Bunga Deposito Masih Harus Dibayar -> Liabilitas segera"},
	{COACode: "20500", Form: "00.18", Sandi: "14190000", Sign: 1, Note: "Utang Pajak -> Liabilitas segera"},
	{COACode: "20600", Form: "00.18", Sandi: "14190000", Sign: 1, Note: "Utang Bunga -> Liabilitas segera"},
	{COACode: "12400", Form: "00.18", Sandi: "14190000", Sign: 1, Note: "Bagi Hasil Masih Harus Dibayar -> Liabilitas segera"},
	{COACode: "20100", Form: "00.18", Sandi: "14200000", Sign: 1, Note: "Tabungan -> Tabungan"},
	{COACode: "12100", Form: "00.18", Sandi: "14200000", Sign: 1, Note: "Tabungan Wadiah -> Tabungan"},
	{COACode: "12200", Form: "00.18", Sandi: "14200000", Sign: 1, Note: "Tabungan Mudharabah -> Tabungan"},
	{COACode: "20200", Form: "00.18", Sandi: "14210000", Sign: 1, Note: "Deposito Berjangka -> Deposito"},
	{COACode: "12300", Form: "00.18", Sandi: "14210000", Sign: 1, Note: "Deposito Mudharabah -> Deposito"},
	{COACode: "20300", Form: "00.18", Sandi: "14250000", Sign: 1, Note: "Giro -> Liabilitas lain-lain (Form 00.18 tidak punya baris giro tersendiri)"},
	{COACode: "20700", Form: "00.18", Sandi: "14250000", Sign: 1, Note: "Utang Lainnya -> Liabilitas lain-lain"},
	{COACode: "12500", Form: "00.18", Sandi: "14250000", Sign: 1, Note: "Dana Kebajikan -> Liabilitas lain-lain"},
	{COACode: "12900", Form: "00.18", Sandi: "14250000", Sign: 1, Note: "Kewajiban Lainnya Syariah -> Liabilitas lain-lain"},
	{COACode: "20800", Form: "00.18", Sandi: "14250000", Sign: 1, Note: "Rekening Antarkantor -> Liabilitas lain-lain"},

	// ── Arus kas investasi (PDF #page 296) ──
	{COACode: "10600", Form: "00.18", Sandi: "21010000", Sign: 1, Note: "Aset Tetap dan Inventaris -> Pembelian/penjualan aset tetap dan inventaris"},
	{COACode: "10700", Form: "00.18", Sandi: "21010000", Sign: 1, Note: "Akumulasi Penyusutan Aset Tetap -> Pembelian/penjualan aset tetap dan inventaris"},

	// ── Arus kas pendanaan (PDF #page 296) ──
	// Form 00.18 tidak punya baris khusus setoran/penarikan modal disetor; seluruh
	// perubahan ekuitas dipetakan ke baris "Penyesuaian lainnya".
	{COACode: "30100", Form: "00.18", Sandi: "31990000", Sign: 1, Note: "Modal Disetor -> Penyesuaian lainnya (pendanaan)"},
	{COACode: "30200", Form: "00.18", Sandi: "31990000", Sign: 1, Note: "Laba Ditahan -> Penyesuaian lainnya (pendanaan)"},
	{COACode: "30300", Form: "00.18", Sandi: "31990000", Sign: 1, Note: "Laba Tahun Berjalan -> Penyesuaian lainnya (pendanaan)"},
	{COACode: "30400", Form: "00.18", Sandi: "31990000", Sign: 1, Note: "Cadangan Umum -> Penyesuaian lainnya (pendanaan)"},
	{COACode: "13100", Form: "00.18", Sandi: "31990000", Sign: 1, Note: "Modal Disetor Syariah -> Penyesuaian lainnya (pendanaan)"},
	{COACode: "13200", Form: "00.18", Sandi: "31990000", Sign: 1, Note: "Laba Ditahan Syariah -> Penyesuaian lainnya (pendanaan)"},
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
