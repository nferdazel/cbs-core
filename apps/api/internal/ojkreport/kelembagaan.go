package ojkreport

import (
	"context"
	"errors"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// kelembagaan.go merakit LAPORAN_KELEMBAGAAN BPR dari data bank (tabel bank_offices
// dan bank_management, migrasi 000112).
//
// Struktur bersumber dari Lampiran II SEOJK No. 16/SEOJK.03/2024 (PDF resmi 528 hlm.,
// salinan di luar repo):
//   - Form 00.04 "Data Kantor BPR"          : PDF #page 88-95, hlm. 36-43.
//   - Form 00.02 "Data Direksi/Komisaris"   : PDF #page 75-82, hlm. 23-30.
//   - Form 00.03 "Data Pejabat Eksekutif"   : PDF #page 83-87, hlm. 31-35.
//   - Ruang lingkup Laporan Kelembagaan     : PDF #page 2.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Yang TERSIMPAN baru sebagian kolom form (identitas kantor dan identitas orang).
//     Kolom form yang belum punya sumber (NIK, alamat orang, sertifikat kompetensi,
//     pendidikan, keanggotaan komite, koordinat, jumlah pegawai per jenjang, EDC/ATM,
//     jumlah kantor kas/kas keliling/SKK) didaftarkan pada Unavailable dengan alasan
//     eksplisit, bukan dikosongkan diam-diam atau diisi nol.
//   - NIK tidak dimodelkan sama sekali: keputusan panel menetapkan NIK tidak dipaparkan
//     sebagai keluaran laporan (docs/KEPUTUSAN-OJK.md §4).
//   - Sandi jabatan OJK (110/120/210/220 dan 00/01/02) adalah nilai yang BANK isi, bukan
//     diturunkan dari teks jabatan; kosong ditulis "-".

// KelembagaanSource menyediakan laporan kelembagaan terhitung. Kontraknya opsional pada
// perakitan ekspor: tanpa sumber ini laporan kelembagaan belum dapat dibangun.
type KelembagaanSource interface {
	KelembagaanReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.KelembagaanReport, error)
}

// ErrKelembagaanSourceUnavailable menandai sumber kelembagaan yang belum dirangkai pada
// perakitan ini.
var ErrKelembagaanSourceUnavailable = errors.New("modul kelembagaan belum dikonfigurasi pada ekspor ini")

// GenerateKelembagaan menyusun Bundle berisi tabel Laporan Kelembagaan untuk posisi
// asOf. Sumber wajib tersedia; tanpa itu ekspor ditolak, bukan menghasilkan berkas
// kosong yang tampak sah.
func GenerateKelembagaan(ctx context.Context, source KelembagaanSource, asOf time.Time, actor domain.Actor) (*Bundle, error) {
	if source == nil {
		return nil, errors.New("sumber laporan kelembagaan belum dikonfigurasi")
	}
	report, err := source.KelembagaanReport(ctx, asOf, actor)
	if err != nil {
		return nil, err
	}
	def := reportByCode("LAPORAN_KELEMBAGAAN")
	periodStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	return &Bundle{
		Period:             periodStart,
		PeriodEnd:          MonthEnd(asOf),
		GeneratedAt:        time.Now().UTC(),
		Deadline:           def.Deadline(periodStart),
		CorrectionDeadline: def.CorrectionDeadline(periodStart),
		MappingStatus:      MappingStatus,
		Tables:             BuildKelembagaanTables(report),
	}, nil
}

// BuildKelembagaanTables menyusun tiga bagian laporan (kantor, direksi/komisaris,
// pejabat eksekutif) dari keluaran layanan kelembagaan. Fungsi ini murni sehingga dapat
// diuji tanpa basis data.
func BuildKelembagaanTables(report domain.KelembagaanReport) []TableSection {
	return []TableSection{
		buildKelembagaanKantorTable(report.Offices),
		buildKelembagaanPengurusTable(
			"00.02", "DIREKSI/KOMISARIS",
			"Data Anggota Direksi dan Anggota Dewan Komisaris BPR (PDF #page 75-82, hlm. 23-30)",
			"kolom sandi jabatan Form 00.02 (110/120 Direksi, 210/220 Komisaris)",
			report.Management, func(category string) bool {
				return category == domain.KelembagaanKategoriDireksi || category == domain.KelembagaanKategoriKomisaris
			}),
		buildKelembagaanPengurusTable(
			"00.03", "PEJABAT_EKSEKUTIF",
			"Data Pejabat Eksekutif BPR (PDF #page 83-87, hlm. 31-35)",
			"kolom sandi jabatan Form 00.03 (00/01/02)",
			report.Management, func(category string) bool {
				return category == domain.KelembagaanKategoriPejabatEksekutif
			}),
	}
}

// formKelembagaanBulanan adalah form Laporan Gabungan yang sumbernya data kelembagaan
// dan ikut bundel bulanan. Dua di antaranya berbasis peristiwa (00.09/00.10/00.12):
// saat peristiwanya tidak ada pada bulan periode, form tetap dicatat belum dibangun
// dengan alasan spesifik supaya ketidakhadirannya terbaca.
var formKelembagaanBulanan = []string{"00.02", "00.03", "00.04", "00.09", "00.10", "00.12"}

// buildKelembagaanMonthlyTables memilih bagian kelembagaan yang benar-benar berisi
// baris untuk ikut bundel bulanan. Bagian tanpa baris TIDAK di-append, tetapi dicatat
// sebagai SkippedForms dengan alasan spesifik supaya penulis berkas menuliskan
// "# FORM x TIDAK DIBANGUN: ..." alih-alih menampilkan form kosong yang tampak lengkap.
// LAPORAN_KELEMBAGAAN tetap memakai BuildKelembagaanTables apa adanya (selalu tiga
// bagian, termasuk yang kosong).
func buildKelembagaanMonthlyTables(report domain.KelembagaanReport) ([]TableSection, []OJKFormDefinition) {
	var tables []TableSection
	var skipped []OJKFormDefinition
	for _, sec := range BuildKelembagaanTables(report) {
		if len(sec.Rows) > 0 {
			tables = append(tables, sec)
			continue
		}
		skipped = append(skipped, OJKFormDefinition{
			Form: sec.Form, Name: formName(sec.Form),
			UnavailableReason: kelembagaanKosongReason(sec.Form),
		})
	}
	return tables, skipped
}

// kelembagaanSkippedTanpaSumber mencatat form bulanan kelembagaan belum dibangun karena
// sumbernya belum dirangkai pada ekspor ini.
func kelembagaanSkippedTanpaSumber(reason string) []OJKFormDefinition {
	out := make([]OJKFormDefinition, 0, len(formKelembagaanBulanan))
	for _, form := range formKelembagaanBulanan {
		out = append(out, OJKFormDefinition{Form: form, Name: formName(form), UnavailableReason: reason})
	}
	return out
}

// kelembagaanKosongReason merinci mengapa satu bagian kelembagaan belum dibangun saat
// sumbernya ada tetapi bank belum mengisi datanya.
func kelembagaanKosongReason(form string) string {
	switch form {
	case "00.02":
		return "belum ada data anggota direksi/dewan komisaris pada bank_management (migrasi 000112); form tidak ditampilkan kosong agar tidak tampak lengkap"
	case "00.03":
		return "belum ada data pejabat eksekutif pada bank_management (migrasi 000112); form tidak ditampilkan kosong agar tidak tampak lengkap"
	case "00.04":
		return "belum ada data kantor pada bank_offices (migrasi 000112); form tidak ditampilkan kosong agar tidak tampak lengkap"
	default:
		return "data kelembagaan belum diisi bank; form tidak ditampilkan kosong agar tidak tampak lengkap"
	}
}

// buildKelembagaanKantorTable menyusun bagian Data Kantor BPR (Form 00.04).
func buildKelembagaanKantorTable(offices []domain.BankOffice) TableSection {
	sec := TableSection{
		Form:     "00.04",
		Name:     "Data Kantor BPR (PDF #page 88-95, hlm. 36-43)",
		KeyLabel: "Sandi/Nama Kantor",
		Columns: []TableColumn{
			{Sandi: "I", Nama: "Sandi Kantor"},
			{Sandi: "JENIS", Nama: "Jenis Kantor (teks bank)"},
			{Sandi: "II", Nama: "Nama Kantor"},
			{Sandi: "IV", Nama: "Alamat"},
			{Sandi: "IV.3", Nama: "Kabupaten/Kota"},
			{Sandi: "KAB_OJK", Nama: "Sandi Kabupaten/Kota OJK"},
			{Sandi: "TGL_BUKA", Nama: "Tanggal Buka"},
			{Sandi: "TGL_TUTUP", Nama: "Tanggal Tutup"},
			{Sandi: "STATUS", Nama: "Status"},
			{Sandi: "CATATAN", Nama: "Catatan"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "III", Nama: "Koordinat Kantor (Form 00.04 kolom III)",
				Reason: "koordinat kantor belum dimodelkan pada bank_offices (migrasi 000112); tidak dikarang dari alamat"},
			{Sandi: "V", Nama: "Nama Pimpinan (Form 00.04 kolom V)",
				Reason: "nama pimpinan kantor belum dimodelkan; pimpinan kantor pusat dapat dilihat pada bagian direksi, pimpinan cabang belum tersimpan"},
			{Sandi: "VII", Nama: "Jumlah Pegawai per status dan jenjang pendidikan (Form 00.04 kolom VII)",
				Reason: "jumlah pegawai per jenjang pendidikan belum tersimpan pada bank_offices"},
			{Sandi: "VIII", Nama: "Jumlah Kantor Kas (Form 00.04 kolom VIII)",
				Reason: "kantor kas dapat didaftarkan sebagai baris bank_offices, tetapi hitungan agregat per kantor induk belum dihitung mesin"},
			{Sandi: "IX", Nama: "Status Kepemilikan Gedung (Form 00.04 kolom IX)",
				Reason: "status kepemilikan gedung belum dimodelkan pada bank_offices"},
			{Sandi: "X", Nama: "Jumlah Kas Keliling (Form 00.04 kolom X)",
				Reason: "kas keliling belum dimodelkan pada bank_offices"},
			{Sandi: "XI-XII", Nama: "EDC/ATM (Form 00.04 kolom XI-XII)",
				Reason: "jumlah/EDC/ATM belum dimodelkan pada bank_offices"},
			{Sandi: "XIII", Nama: "Perubahan Selama Bulan Posisi Laporan (Form 00.04 kolom XIII)",
				Reason: "keterangan perubahan kantor dan surat persetujuan OJK per bulan belum dimodelkan; tanggal buka/tutup tersedia sebagai pendekatan"},
			{Sandi: "XIV", Nama: "Jumlah Pegawai per bidang/jenis kelamin/usia (Form 00.04 kolom XIV)",
				Reason: "rincian jumlah pegawai belum tersimpan pada bank_offices"},
			{Sandi: "XV", Nama: "Jumlah SKK (Form 00.04 kolom XV)",
				Reason: "jumlah sentra keuangan khusus per kantor induk belum dihitung mesin"},
		},
		Notes: []string{
			"Hanya kolom yang benar-benar dipetakan dari bank_offices yang diisi; sisanya didaftarkan sebagai belum tersedia, bukan nol.",
			"Jenis kantor, sandi kantor, dan catatan adalah nilai bank (bukan daftar sandi OJK yang dikarang). Sandi Kabupaten/Kota OJK mengacu Lampiran 03 dan ditulis \"-\" bila bank belum mengisi.",
			"Status AKTIF berarti kantor beroperasi; TUTUP berarti kantor sudah ditutup dan tetap ditampilkan agar riwayat jaringan terbaca.",
		},
	}
	for _, o := range offices {
		row := TableRow{Key: kelembagaanOfficeKey(o)}
		row.Cells = append(row.Cells,
			TableCell{Sandi: "I", Nama: "Sandi Kantor", Value: dashIfEmpty(o.Code)},
			TableCell{Sandi: "JENIS", Nama: "Jenis Kantor (teks bank)", Value: dashIfEmpty(o.OfficeType)},
			TableCell{Sandi: "II", Nama: "Nama Kantor", Value: dashIfEmpty(o.Name)},
			TableCell{Sandi: "IV", Nama: "Alamat", Value: dashIfEmpty(o.Address)},
			TableCell{Sandi: "IV.3", Nama: "Kabupaten/Kota", Value: dashIfEmpty(o.City)},
			TableCell{Sandi: "KAB_OJK", Nama: "Sandi Kabupaten/Kota OJK", Value: dashIfEmpty(o.OJKKabupatenCode)},
			TableCell{Sandi: "TGL_BUKA", Nama: "Tanggal Buka", Value: formatTanggalAtauDash(o.OpenedAt)},
			TableCell{Sandi: "TGL_TUTUP", Nama: "Tanggal Tutup", Value: formatTanggalAtauDash(o.ClosedAt)},
			TableCell{Sandi: "STATUS", Nama: "Status", Value: dashIfEmpty(o.Status)},
			TableCell{Sandi: "CATATAN", Nama: "Catatan", Value: dashIfEmpty(o.Note)},
		)
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// buildKelembagaanPengurusTable menyusun satu bagian direksi/komisaris (Form 00.02)
// atau pejabat eksekutif (Form 00.03), memilih baris lewat predikat kategori.
func buildKelembagaanPengurusTable(form, keyLabel, name, sandiNote string, rows []domain.BankManagement, ambil func(string) bool) TableSection {
	sec := TableSection{
		Form:     form,
		Name:     name,
		KeyLabel: "Nama",
		Columns: []TableColumn{
			{Sandi: "KATEGORI", Nama: "Kategori"},
			{Sandi: "I", Nama: "Nama"},
			{Sandi: "V", Nama: "Jabatan (teks bank)"},
			{Sandi: "IV", Nama: "Sandi Jabatan OJK"},
			{Sandi: "VII/VI", Nama: "Nomor Surat Persetujuan/Pengangkatan"},
			{Sandi: "TGL_SURAT", Nama: "Tanggal Surat"},
			{Sandi: "MULAI", Nama: "Tanggal Mulai Menjabat"},
			{Sandi: "SELESAI", Nama: "Tanggal Selesai Menjabat"},
			{Sandi: "STATUS", Nama: "Status"},
			{Sandi: "CATATAN", Nama: "Catatan"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "III", Nama: "NIK",
				Reason: "NIK tidak dimodelkan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4)"},
			{Sandi: "II", Nama: "Alamat",
				Reason: "alamat pribadi pengurus belum dimodelkan pada bank_management"},
			{Sandi: "VIII-XI", Nama: "Sertifikat Kompetensi Kerja dan Pendidikan",
				Reason: "sertifikat kompetensi kerja, pendidikan formal/non formal belum dimodelkan pada bank_management"},
			{Sandi: "XII", Nama: "Keanggotaan Komite",
				Reason: "keanggotaan komite (audit/pemantau risiko/remunerasi/manajemen risiko) belum dimodelkan pada bank_management"},
			{Sandi: "XIII-XVII", Nama: "Fungsi Kepatuhan, Komisaris Independen, Keterangan Kepengurusan, Alasan Perubahan, Keterangan Jabatan",
				Reason: "kolom-kolom Form 00.02 ini belum dimodelkan pada bank_management; sebagian identik dengan status/jabatan yang tersimpan, tetapi pemetaannya adalah keputusan bank"},
		},
		Notes: []string{
			"Sandi jabatan OJK (" + sandiNote + ") adalah nilai yang BANK isi, bukan diturunkan dari teks jabatan; kosong ditulis \"-\".",
			"Hanya baris dengan kategori yang cocok yang tampil pada bagian ini; baris lain masuk bagian yang sesuai.",
			"NIK, alamat, sertifikat, pendidikan, dan keanggotaan komite belum punya sumber dan dinyatakan belum tersedia, bukan dikosongkan diam-diam.",
		},
	}
	for _, m := range rows {
		if !ambil(m.Category) {
			continue
		}
		row := TableRow{Key: dashIfEmpty(m.Name)}
		row.Cells = append(row.Cells,
			TableCell{Sandi: "KATEGORI", Nama: "Kategori", Value: dashIfEmpty(m.Category)},
			TableCell{Sandi: "I", Nama: "Nama", Value: dashIfEmpty(m.Name)},
			TableCell{Sandi: "V", Nama: "Jabatan (teks bank)", Value: dashIfEmpty(m.Position)},
			TableCell{Sandi: "IV", Nama: "Sandi Jabatan OJK", Value: dashIfEmpty(m.OJKPositionCode)},
			TableCell{Sandi: "VII/VI", Nama: "Nomor Surat Persetujuan/Pengangkatan", Value: dashIfEmpty(m.LicenseNumber)},
			TableCell{Sandi: "TGL_SURAT", Nama: "Tanggal Surat", Value: formatTanggalAtauDash(m.LicenseDate)},
			TableCell{Sandi: "MULAI", Nama: "Tanggal Mulai Menjabat", Value: formatTanggalAtauDash(m.StartedAt)},
			TableCell{Sandi: "SELESAI", Nama: "Tanggal Selesai Menjabat", Value: formatTanggalAtauDash(m.EndedAt)},
			TableCell{Sandi: "STATUS", Nama: "Status", Value: dashIfEmpty(m.Status)},
			TableCell{Sandi: "CATATAN", Nama: "Catatan", Value: dashIfEmpty(m.Note)},
		)
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// kelembagaanOfficeKey memakai sandi kantor bila ada, jika tidak namanya. Kunci tidak
// pernah kosong agar baris tetap dapat diacak ke pemiliknya.
func kelembagaanOfficeKey(o domain.BankOffice) string {
	if s := strings.TrimSpace(o.Code); s != "" {
		return s
	}
	return dashIfEmpty(o.Name)
}
