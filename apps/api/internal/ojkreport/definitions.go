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
		Buildable:         false,
		UnavailableReason: "data jaringan kantor, direksi, dewan komisaris, dan pejabat eksekutif beserta dokumen pendukung belum tersedia di basis data",
	},
	{
		Code: "LAPORAN_BMPK", Name: "Laporan Batas Maksimum Pemberian Kredit (BMPK) BPR",
		Periodicity: OJKBulanan, DueDay: 10, CorrectionDay: 15, Channel: OJKChannelAPOLO,
		Buildable:         false,
		UnavailableReason: "agregasi eksposur per pihak terkait/pihak lawan untuk uji pelanggaran dan pelampauan BMPK belum ada",
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
		Buildable:         false,
		UnavailableReason: "daftar debitur beserta rincian perbedaan kualitas aset produktif belum tersedia sebagai keluaran laporan",
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

// OJKBulananForms adalah daftar form Laporan Bulanan BPR. Form buildable pada
// gelombang ini hanya 01.00 dan 02.00 karena keduanya bersumber dari COA.
var OJKBulananForms = []OJKFormDefinition{
	{Form: "00.00", Name: "Informasi Pokok BPR", Buildable: false,
		UnavailableReason: "butuh data yang belum tersimpan lengkap (organ pelaksana, informasi audit KAP/AP, PVA, PTI, ultimate shareholder); profil bank hanya memuat nama, alamat, telepon, dan NPWP"},
	// Form 00.08 selalu disertakan; barisnya terisi hanya untuk posisi Maret,
	// Juni, September, dan Desember, dan rasio yang komponennya belum tersedia
	// ditandai tidak tersedia (lihat ratios.go).
	{Form: "00.08", Name: "Rasio Keuangan Triwulanan", Buildable: true},
	{Form: "01.00", Name: "Laporan Posisi Keuangan", Buildable: true},
	{Form: "01.01", Name: "Rekening Administratif", Buildable: false,
		UnavailableReason: "pos komitmen/kontinjensi (off-balance) belum dicatat pada bagan akun"},
	{Form: "02.00", Name: "Laporan Laba Rugi dan Penghasilan Komprehensif Lain", Buildable: true},
	{Form: "05.00", Name: "Daftar Penempatan pada Bank Lain", Buildable: false,
		UnavailableReason: "rincian per bank lawan belum tersedia sebagai keluaran laporan"},
	{Form: "06.00", Name: "Daftar Kredit yang Diberikan", Buildable: false,
		UnavailableReason: "rincian per debitur beserta sandi pihak lawan/sektor/agunan belum tersedia sebagai keluaran laporan"},
	{Form: "09.00", Name: "Rincian Aset Lainnya", Buildable: false,
		UnavailableReason: "rincian pos aset lainnya belum tersedia"},
	{Form: "13.00", Name: "Daftar Simpanan dari Bank Lain", Buildable: false,
		UnavailableReason: "rincian per bank lawan belum tersedia"},
	{Form: "00.13", Name: "Dokumen Pendukung", Buildable: false,
		UnavailableReason: "merupakan berkas PDF pendukung, bukan angka"},
	{Form: "00.14", Name: "Daftar Data Jenis Nasabah dan Produk Simpanan di BPR", Buildable: false,
		UnavailableReason: "agregasi jenis nasabah per produk simpanan belum tersedia"},
	{Form: "00.15", Name: "Rincian Transaksi Terkait Penilaian Risiko TPPU dan TPPT", Buildable: false,
		UnavailableReason: "data transaksi terkait penilaian risiko TPPU/TPPT belum tersedia"},
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
