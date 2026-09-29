// Package ojkreport menyediakan fondasi ekspor laporan OJK (APOLO) untuk BPR.
//
// Modul ini SENGAJA tidak menghitung ulang rumus akuntansi. Angka diambil dari
// laporan journal-based yang sudah ada (domain.ReportService.GetBalanceSheet dan
// GetIncomeStatement), lalu hanya dikelompokkan ke pos OJK memakai pemetaan COA di
// coa_mapping.go.
//
// Rujukan aturan: POJK No. 23 Tahun 2024 dan SEOJK No. 16/SEOJK.03/2024
// (Lampiran II). Sandi dan nama pos pada forms.go diambil dari Lampiran II;
// pemetaan baris COA internal ke pos OJK masih DRAF (lihat coa_mapping.go).
package ojkreport

import "time"

// OJKPeriodicity membedakan laporan berkala bulanan dan triwulanan.
type OJKPeriodicity string

const (
	OJKBulanan      OJKPeriodicity = "BULANAN"
	OJKTribulanan   OJKPeriodicity = "TRIWULANAN"
	OJKChannelAPOLO                = "APOLO"
)

// OJKReportDefinition adalah satu jenis laporan berkala beserta periodisitas dan
// tenggatnya. Seluruh definisi disimpan sebagai data agar dapat dibaca program
// (mis. untuk pengingat tenggat), bukan tersebar sebagai literal di kode alur.
type OJKReportDefinition struct {
	Code        string         `json:"code"`
	Name        string         `json:"name"`
	Periodicity OJKPeriodicity `json:"periodicity"`
	// DueDay adalah tanggal penyampaian (tanggal di bulan berikutnya setelah akhir
	// periode). SEOJK 16/2024: laporan bulanan tanggal 10, Laku Pandai tanggal 15.
	DueDay int `json:"due_day"`
	// CorrectionDay adalah batas akhir koreksi. Untuk laporan bulanan koreksi
	// disampaikan paling lambat tanggal 15 bulan berikutnya. 0 berarti mengikuti DueDay.
	CorrectionDay int    `json:"correction_day"`
	Channel       string `json:"channel"`
	// Buildable menandakan laporan sudah dapat dibangun dari data yang ada.
	Buildable bool `json:"buildable"`
	// UnavailableReason wajib terisi bila Buildable=false, menjelaskan alasannya.
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// Deadline menghitung tenggat penyampaian untuk periode pelaporan tertentu.
// Periodisasi apa pun (bulanan/triwulanan) memakai aturan yang sama: tanggal DueDay
// pada bulan berikutnya setelah periode berakhir.
func (d OJKReportDefinition) Deadline(period time.Time) time.Time {
	first := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
	return time.Date(first.Year(), first.Month()+1, d.DueDay, 0, 0, 0, 0, time.UTC)
}

// CorrectionDeadline menghitung batas akhir koreksi. Bila CorrectionDay tidak
// diisi, koreksi dianggap mengikuti tenggat penyampaian.
func (d OJKReportDefinition) CorrectionDeadline(period time.Time) time.Time {
	if d.CorrectionDay == 0 {
		return d.Deadline(period)
	}
	first := time.Date(period.Year(), period.Month(), 1, 0, 0, 0, 0, time.UTC)
	return time.Date(first.Year(), first.Month()+1, d.CorrectionDay, 0, 0, 0, 0, time.UTC)
}

// OJKReportDefinitions adalah daftar lengkap laporan berkala pada SEOJK 16/2024.
// Laporan yang belum dapat dibangun tetap didaftarkan dengan alasannya, supaya
// kekurangan data terlihat, bukan disembunyikan.
var OJKReportDefinitions = []OJKReportDefinition{
	{
		Code: "LAPORAN_BULANAN_BPR", Name: "Laporan Bulanan BPR",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		Buildable: true,
	},
	{
		Code: "LAPORAN_KELEMBAGAAN", Name: "Laporan Kelembagaan BPR",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		// Data jaringan kantor (bank_offices) dan direksi/komisaris/pejabat eksekutif
		// (bank_management) tersedia lewat migrasi 000112, perakit kelembagaan.go
		// tersedia, dan jalur tulis berizin+teraudit sudah ada. Kolom form yang belum
		// punya sumber ditandai belum tersedia oleh perakit, bukan dikarang.
		Buildable: true,
	},
	{
		Code: "LAPORAN_BMPK", Name: "Laporan Batas Maksimum Pemberian Kredit (BMPK) BPR",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		// Fondasi (migrasi 000103), perakit (bmpk.go), dan endpoint ekspor
		// GET /api/v1/reports/ojk/bmpk sudah ada. Pengisian pihak terkait/batas
		// tetap dilakukan bank lewat SQL/seed; laporan hanya membaca.
		Buildable: true,
	},
	{
		Code: "LAPORAN_KEUANGAN_PUBLIKASI", Name: "Laporan Keuangan Publikasi BPR (bukti pengumuman)",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		Buildable:         false,
		UnavailableReason: "bukti pengumuman laporan keuangan publikasi adalah dokumen/berkas dari luar sistem, bukan angka yang dapat dihitung",
	},
	{
		Code: "LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF", Name: "Laporan Perbedaan Kualitas Aset Produktif",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		// Sisi "Pada BPR Bersangkutan" (kolom I-X) dapat dibangun dari baris kredit;
		// perakit (perbedaan_kualitas.go) dan endpoint GET /api/v1/reports/ojk/
		// perbedaan-kualitas sudah ada. Sisi pembanding ("Pada BPR Lain") dan
		// perbandingan kualitas komersial vs PPKA dinyatakan belum tersedia dengan
		// alasan eksplisit karena tidak ada sumber tersimpan.
		Buildable: true,
	},
	{
		Code: "LAPORAN_TPPU_TPPT_PPSPM", Name: "Laporan Dokumen Penilaian Risiko TPPU/TPPT/PPSPM",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		Buildable:         false,
		UnavailableReason: "dokumen penilaian risiko disusun manual di luar sistem; tidak ada data penilaian risiko tersimpan",
	},
	{
		Code: "LAPORAN_BUKTI_PENGUMUMAN_TAHUNAN", Name: "Laporan Bukti Pengumuman Laporan Tahunan",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		Buildable:         false,
		UnavailableReason: "bukti pengumuman laporan tahunan adalah dokumen dari luar sistem",
	},
	{
		Code: "LAPORAN_KEUANGAN_PUBLIKASI_TRIWULANAN", Name: "Laporan Keuangan Publikasi BPR (triwulanan)",
		Periodicity: OJKTribulanan, DueDay: 10, Channel: OJKChannelAPOLO,
		Buildable:         false,
		UnavailableReason: "rasio keuangan triwulanan (Form 00.08) sudah dihitung, tetapi laporan keuangan publikasi, informasi kinerja keuangan lain, dan bukti pengumuman belum tersedia",
	},
	{
		Code: "LAPORAN_LAKU_PANDAI", Name: "Laporan Perkembangan Penyelenggaraan Laku Pandai",
		Periodicity: OJKTribulanan, DueDay: 15, Channel: OJKChannelAPOLO,
		Buildable:         false,
		UnavailableReason: "data agen Laku Pandai dan transaksinya belum tersedia",
	},
}

// OJKFormDefinition mendeskripsikan satu form di dalam Laporan Bulanan BPR.
type OJKFormDefinition struct {
	Form      string `json:"form"`
	Name      string `json:"name"`
	Buildable bool   `json:"buildable"`
	// UnavailableReason wajib terisi bila Buildable=false.
	UnavailableReason string `json:"unavailable_reason,omitempty"`
}

// OJKBulananForms adalah daftar form Laporan Bulanan BPR. Form yang ditandai
// buildable dapat dibangun bila sumber datanya tersedia; bila tidak, builder
// mencatat form itu pada SkippedForms beserta alasannya (lihat builder.go).
var OJKBulananForms = []OJKFormDefinition{
	// Form 00.00 dibangun dari konfigurasi bank (tabel bank_profile dan kunci ojk.*);
	// bila nama bank belum diisi, form dinyatakan belum tersedia saat ekspor.
	{Form: "00.00", Name: "Informasi Pokok BPR", Buildable: true},
	// Form 00.08 selalu disertakan; barisnya terisi hanya untuk posisi Maret,
	// Juni, September, dan Desember, dan rasio yang komponennya belum tersedia
	// ditandai tidak tersedia (lihat ratios.go).
	{Form: "00.08", Name: "Rasio Keuangan Triwulanan", Buildable: true},
	{Form: "01.00", Name: "Laporan Posisi Keuangan", Buildable: true},
	// Form 01.01 dibangun dari register pos komitmen/kontinjensi off-balance
	// (off_balance_items, migrasi 000113) yang bank isi lewat API; bukan dari bagan
	// akun. Bila belum ada baris aktif, builder mencatatnya pada SkippedForms.
	{Form: "01.01", Name: "Rekening Administratif", Buildable: true},
	{Form: "02.00", Name: "Laporan Laba Rugi dan Penghasilan Komprehensif Lain", Buildable: true},
	// Form 05.00 dibangun dari penanda lps_placements (migrasi 000045) yang bukan
	// register lengkap penempatan pada bank lain.
	{Form: "05.00", Name: "Daftar Penempatan pada Bank Lain", Buildable: true},
	// Form 06.00 dibangun dari baris kredit per debitur.
	{Form: "06.00", Name: "Daftar Kredit yang Diberikan", Buildable: true},
	// Form 09.00 "Rincian Aset Lainnya" (Form 09.00 – 1/–2, PDF #page 187-188):
	// posisinya dipecah dari saldo COA yang dipetakan ke pos Aset Lainnya Form 01.00
	// (sandi 1299000000). Pemetaannya masih DRAF dan dipisah di COAMapping09Draft;
	// pos tanpa akun COA ditulis tidak tersedia di dalam form, bukan nol.
	{Form: "09.00", Name: "Rincian Aset Lainnya", Buildable: true},
	// Form 09.01 "Rincian Aset Lainnya - Lain-lain" (PDF #page 189-190) adalah form
	// KONDISIONAL: hanya terbit bila pos Lainnya (1299990000) Form 09.00 melebihi 25%
	// dari jumlah aset lainnya (pos 1299000000 Form 01.00). Bila tidak terlampaui,
	// form memang tidak berlaku untuk periode itu sehingga tidak diikutkan bundel
	// (bukan alasan "belum dibangun"). Barisnya per akun COA pemetaan Form 09.00,
	// dirangkai di form09_01.go.
	{Form: "09.01", Name: "Rincian Aset Lainnya - Lain-lain", Buildable: true},
	{Form: "13.00", Name: "Daftar Simpanan dari Bank Lain", Buildable: true},
	{Form: "00.13", Name: "Dokumen Pendukung", Buildable: false,
		UnavailableReason: "merupakan berkas PDF pendukung, bukan angka"},
	// "00.14" sengaja tidak didaftarkan: nomor itu tidak ada di SEOJK 16/2024
	// (hanya di SEOJK 12/2022 yang dicabut). Agregasi jenis nasabah per produk
	// tetap ada sebagai laporan internal (form00_14.go), tetapi tidak diakui
	// sebagai form OJK; lihat docs/CELAH-FORM-OJK.md §6.
	{Form: "00.15", Name: "Rincian Transaksi Terkait Penilaian Risiko TPPU dan TPPT", Buildable: false,
		UnavailableReason: "data transaksi terkait penilaian risiko TPPU/TPPT belum tersedia"},

	// ── Laporan Gabungan (daftar resmi cetak -7-, PDF #59) ──
	// Form 00.01 dibangun dari register pemegang saham per baris (kepemilikan_bpr_register,
	// migrasi 000116) yang bank isi lewat API, bukan dari tiga kunci teks Form 00.00.
	// Bila belum ada baris AKTIF, builder mencatatnya pada SkippedForms dengan alasan
	// spesifik. Kolom IV No. Identitas sengaja tidak disimpan (keputusan privasi) dan
	// laporan menulisnya "-" beralasan; form tidak memakai kolom Sandi Kantor.
	{Form: "00.01", Name: "Data Kepemilikan BPR", Buildable: true},
	// Form 00.02/00.03/00.04 (Laporan Gabungan) ikut bundel bulanan. Sumbernya data
	// kelembagaan yang bank isi lewat API reports/ojk/kelembagaan/* — bank_management
	// (migrasi 000112:89) untuk 00.02/00.03 dan bank_offices (migrasi 000112:41) untuk
	// 00.04 — lewat perakit yang sama dengan LAPORAN_KELEMBAGAAN
	// (BuildKelembagaanTables, kelembagaan.go). Bila data belum diisi, form yang kosong
	// dicatat pada SkippedForms dengan alasan spesifik, bukan tampil kosong. Kolom yang
	// belum punya sumber (NIK, alamat, sertifikat+pendidikan, komite, blok XIII-XVII,
	// dan 10 dari 15 blok kolom 00.04) tetap ditandai belum tersedia di dalam form.
	{Form: "00.02", Name: "Data Anggota Direksi dan Anggota Dewan Komisaris BPR", Buildable: true},
	{Form: "00.03", Name: "Data Pejabat Eksekutif BPR", Buildable: true},
	{Form: "00.04", Name: "Data Kantor BPR", Buildable: true},
	{Form: "00.05", Name: "Data Pihak Terkait Lainnya", Buildable: false,
		UnavailableReason: "sumber bmpk_related_parties + customers (migrasi 000103:27, 000001:24) sudah ada, tetapi sandi Jenis dan Hubungan masih teks bebas serta kolom NPWP dan pihak terkait bukan nasabah belum ada (PDF #96-98)"},
	{Form: "00.06", Name: "Daftar Modal Disetor, Modal Sumbangan, dan Dana Setoran Modal - Ekuitas", Buildable: false,
		UnavailableReason: "hanya saldo COA 30100/13100 yang dipetakan (coa_mapping.go:166,174); akun Modal Sumbangan dan Dana Setoran Modal serta kolom Jenis dan Tanggal Persetujuan OJK belum ada (PDF #245-247)"},
	// Form 00.07 dibangun dari register pinjaman per kreditur (pinjaman_diterima_register,
	// migrasi 000117) yang bank isi lewat API, bukan dari akun liabilitas 20100-20800
	// yang tidak menyimpan identitas kreditur. Bila belum ada baris AKTIF, builder
	// mencatatnya pada SkippedForms dengan alasan spesifik. Kolom XV Baki Debet Neto
	// dihitung laporan, tidak disimpan. Form tidak memakai kolom Sandi Kantor.
	{Form: "00.07", Name: "Daftar Pinjaman yang Diterima", Buildable: true},
	// Form 00.09/00.10/00.12 kini buildable sebagai form KONDISIONAL berbasis
	// PERISTIWA: sumber bank_management.ended_at (00.09/00.10, migrasi 000112:89) dan
	// bank_offices.closed_at (00.12, migrasi 000112:41), dirakit form_berhenti.go.
	// Baris muncul hanya bila peristiwanya jatuh pada bulan periode; bila tidak ada,
	// form dicatat pada SkippedForms dengan alasan spesifik. Kolom tanpa sumber (NIK,
	// komite, penyebab berhenti sandi 1/2/3, sandi Jenis OJK, sandi induk, koordinat)
	// ditulis "-" beserta alasan; note teks bebas tidak pernah menjadi sandi.
	{Form: "00.09", Name: "Data Anggota Direksi dan Anggota Dewan Komisaris BPR yang Berhenti Menjabat", Buildable: true},
	{Form: "00.10", Name: "Data Pejabat Eksekutif BPR yang Berhenti Menjabat", Buildable: true},
	{Form: "00.11", Name: "Data Kantor selain Kantor Pusat dan Kantor Cabang dan Terminal Perbankan Elektronik", Buildable: false,
		UnavailableReason: "sumber bank_offices (office_type teks bebas) + branches.parent_id ada, tetapi belum ada perakit Form 00.11; sandi induk/pendahulu, koordinat, pimpinan, dan telepon belum ada (PDF #270-275)"},
	{Form: "00.12", Name: "Data Penutupan Kantor dan Terminal Perbankan Elektronik", Buildable: true},
	{Form: "00.16", Name: "Daftar Pihak Lawan", Buildable: false,
		UnavailableReason: "sumber customers + customers.ojk_pihak_lawan_code (migrasi 000104:26) + counterparty_cif (000106:37) ada, tetapi kolom jenis identitas, jenis kelamin, NPWP, kewarganegaraan, tanggal lahir, grup, dan pemeringkat belum ada (PDF #286-292)"},
	// Form 00.17 "Laporan Perubahan Ekuitas" (PDF #page 293-294) disusun sebagai form
	// parsial: 17 baris tetap x 12 kolom, hanya kolom yang punya akun COA ekuitas yang
	// diisi (III 30100/13100, X 30400, XI 30200/30300/13200), baris "Saldo per 31 Des"
	// dibaca dari saldo akhir tahun T-2/T-1/T. Kolom dan baris mutasi yang belum punya
	// akun ditandai tidak tersedia di dalam form, bukan nol. Hanya posisi Desember
	// (PDF #page 293); posisi lain dicatat pada SkippedForms lewat builder.go (form17.go).
	{Form: "00.17", Name: "Laporan Perubahan Ekuitas", Buildable: true},
	{Form: "00.18", Name: "Laporan Arus Kas", Buildable: true},
	// Form 00.18 dibangun dari domain.CashFlow (jurnal akun kas/bank) lewat perakit
	// form18.go dan pemetaan tersendiri COAMapping18Draft; hanya disampaikan untuk
	// laporan posisi bulan Desember (Form 00.18 – 2, PDF #page 296-297). Baris yang
	// belum punya akun COA sumber ditulis tidak tersedia di dalam form, bukan nol.
	{Form: "00.19", Name: "Struktur Organisasi", Buildable: false,
		UnavailableReason: "merupakan berkas PDF; bahan sudah ada (bank_offices, bank_management, branches) sehingga bisa dirakit otomatis, tetapi perakit dan bentuk berkasnya belum diputuskan (PDF #298)"},
	{Form: "00.20", Name: "Struktur Kelompok Usaha", Buildable: false,
		UnavailableReason: "merupakan berkas PDF; bahan hanya satu string ojk.report.ultimate_shareholders (migrasi 000096:40), tanpa relasi kelompok usaha (PDF #299)"},
	{Form: "00.21", Name: "Laporan Dokumen Penilaian Risiko TPPU, TPPT, dan/atau PPSPM", Buildable: false,
		UnavailableReason: "sudah terdaftar sebagai LAPORAN_TPPU_TPPT_PPSPM berstatus manual (definitions.go:105); dokumen PDF disusun di luar sistem dan tidak ikut bundel bulanan (PDF #300, #60)"},

	// ── Laporan per Kantor (daftar resmi cetak -7-/-8-, PDF #59-60) ──
	{Form: "03.00", Name: "Daftar Kas dalam Valuta Asing", Buildable: false,
		UnavailableReason: "hanya tersedia valas sebagai pedagang valuta asing; belum ada tabel kas valas/tabel kurs dan pos 1101020000 sengaja tanpa sumber COA (mapping_review_test.go:54); status PVA sudah dicatat di ojk.report.pva_status (migrasi 000096:28) (PDF #126-128, #102)"},
	{Form: "04.00", Name: "Daftar Surat Berharga", Buildable: false,
		UnavailableReason: "belum ada register surat berharga (25 kolom) dan akun COA 'Surat Berharga'; pos 1102000000 baru berupa sandi (forms.go:122) (PDF #129-134)"},
	{Form: "06.01", Name: "Daftar Agunan", Buildable: false,
		UnavailableReason: "sumber loan_collaterals (migrasi 000036:31) + ojk_agunan_ppka_amount (000109:31) ada, tetapi kolom alamat agunan, nilai yang diagunkan, sandi jenis agunan Lampiran 01, dan PPKA per agunan belum ada (PDF #169-172, #301)"},
	{Form: "06.02", Name: "Daftar Kredit Sindikasi", Buildable: false,
		UnavailableReason: "belum ada tabel/kolom sindikasi; builder sendiri menyatakan kanal penyaluran belum dimodelkan (form06.go:99) (PDF #173-177)"},
	// Form 07.00 dibangun dari register AYDA per kasus (ayda_register, migrasi 000115)
	// yang bank isi lewat API; bukan dari saldo agregat COA 10500. Bila belum ada baris
	// AKTIF, builder mencatatnya pada SkippedForms dengan alasan spesifik.
	{Form: "07.00", Name: "Daftar Agunan yang Diambil Alih", Buildable: true},
	{Form: "08.00", Name: "Daftar Aset Tetap, Inventaris, dan Aset Tidak Berwujud", Buildable: false,
		UnavailableReason: "hanya agregat COA 10600/10700 (forms.go:136-137); belum ada register aset per jenis, COA per jenis, sumber perolehan, dan metode pengukuran (PDF #182-184, #60)"},
	// Form 10.00 "Rincian Liabilitas Segera" (PDF #page 191-192): posisinya dipecah
	// dari saldo COA yang dipetakan ke pos Liabilitas Segera Form 01.00 (sandi
	// 2101000000). Pemetaannya masih DRAF dan dipisah di COAMapping10Draft; pos tanpa
	// akun COA ditulis tidak tersedia di dalam form, bukan nol. Pos 2101010000 (pajak)
	// sengaja tidak diisi karena definisinya adalah arus pembayaran bulan berjalan,
	// bukan saldo periodEnd.
	{Form: "10.00", Name: "Rincian Liabilitas Segera", Buildable: true},
	// Form 11.00 dibangun dari baris per rekening tabungan (produk keluarga SAVINGS)
	// pada posisi akhir periode (form11.go). Kolom tanpa sumber (Nomor Identitas,
	// Jenis, Jangka Waktu, diblokir/alasan, biaya transaksi belum diamortisasi, PEP,
	// Risiko Nasabah, Status Data) ditulis tidak tersedia di dalam form, bukan nol.
	{Form: "11.00", Name: "Daftar Tabungan", Buildable: true},
	// Form 12.00 dibangun dari baris per kontrak deposito berjangka (time_deposits)
	// pada posisi akhir periode (form12.go). Kolom tanpa sumber ditulis tidak tersedia
	// di dalam form, bukan nol.
	{Form: "12.00", Name: "Daftar Deposito", Buildable: true},
	// Form 14.00 "Rincian Liabilitas Lainnya" (Form 14.00 – 1/–2, PDF #page 213-215):
	// posisinya dipecah dari saldo COA yang dipetakan ke pos Liabilitas Lainnya Form
	// 01.00 (sandi 2299000000). Pemetaannya masih DRAF dan dipisah di COAMapping14Draft;
	// pos tanpa akun COA ditulis tidak tersedia di dalam form, bukan nol.
	{Form: "14.00", Name: "Rincian Liabilitas Lainnya", Buildable: true},
	// Form 14.01 "Rincian Liabilitas Lainnya - Lain-lain" (PDF #page 216-217) adalah
	// form KONDISIONAL: hanya terbit bila pos Lainnya (2299990000) Form 14.00 melebihi
	// 25% dari jumlah liabilitas lainnya (pos 2299000000 Form 01.00). Penyebutnya pos
	// Form 01.00, bukan jumlah baris Form 14.00 (jumlah baris itu hanya 20500+20700 dan
	// akan membuat ambang terlampaui palsu). Bila tidak terlampaui, form memang tidak
	// berlaku untuk periode itu sehingga tidak diikutkan bundel (bukan "belum dibangun").
	// Barisnya per akun COA pemetaan Form 14.00, dirangkai di form14_01.go.
	{Form: "14.01", Name: "Rincian Liabilitas Lainnya - Lain-lain", Buildable: true},
	{Form: "15.00", Name: "Daftar Aset Produktif yang Dihapus Buku", Buildable: false,
		UnavailableReason: "status WRITTEN_OFF + written_off_amount ada (migrasi 000085:29), tetapi tanggal hapus buku tidak ada, nominal gabungan (pokok+bunga+denda) tidak bisa dipisah per kolom, dan penempatan belum punya status hapus buku (PDF #218-221)"},
	{Form: "16.00", Name: "Daftar Penyertaan Modal", Buildable: false,
		UnavailableReason: "belum ada tabel register penyertaan dan akun COA 'Penyertaan Modal' (PDF #223-227)"},
	// Form 17.00 dibangun dari register properti terbengkalai per properti
	// (properti_terbengkalai_register, migrasi 000118) yang bank isi lewat API. Bila
	// belum ada baris AKTIF, builder mencatatnya pada SkippedForms dengan alasan
	// spesifik. Kolom IX Jumlah dihitung laporan dari VII - VIII dan form ini tidak
	// punya baris JUMLAH. Kolom I Sandi Kantor diambil dari kantor pelapor tunggal.
	{Form: "17.00", Name: "Daftar Properti Terbengkalai", Buildable: true},
	{Form: "18.00", Name: "Daftar Aset Keuangan Lainnya", Buildable: false,
		UnavailableReason: "belum ada register per rekening, akun COA, dan jejak 'fraud' belum ada di kode (PDF #233-237)"},
	{Form: "19.00", Name: "Daftar Perbedaan Kualitas Aset Produktif", Buildable: false,
		UnavailableReason: "terbit sebagai LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF (kolom I-X, cakupan kredit saja), tetapi kolom XI-XIX 'Pada BPR Lain' belum ada sumber di perbedaan_kualitas.go:95 dan form tidak ikut bundel bulanan (PDF #239-243)"},

	// ── Laporan per Kantor + Gabungan: 01.00, 01.01, 02.00 sudah terdaftar di atas ──
}

// BuildableForms mengembalikan form Laporan Bulanan BPR yang dapat dibangun.
func BuildableForms() []OJKFormDefinition {
	out := make([]OJKFormDefinition, 0, len(OJKBulananForms))
	for _, f := range OJKBulananForms {
		if f.Buildable {
			out = append(out, f)
		}
	}
	return out
}
