package ojkreport

import (
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form_berhenti.go merakit tiga form BERBASIS PERISTIWA pada bundel bulanan:
//
//   - Form 00.09 "Data Anggota Direksi dan Anggota Dewan Komisaris BPR yang
//     Berhenti Menjabat": PDF #page 258-263 (hlm. tercetak 206-211).
//   - Form 00.10 "Data Pejabat Eksekutif BPR yang Berhenti Menjabat":
//     PDF #page 264-269 (hlm. 212-217).
//   - Form 00.12 "Data Penutupan Kantor dan Terminal Perbankan Elektronik (TPE)":
//     PDF #page 276-280 (hlm. 224-228).
//
// Sumber: bank_management (migrasi 000112:89) untuk 00.09/00.10 dan bank_offices
// (000112:41) untuk 00.12, lewat KelembagaanReport yang sama dengan Form 00.02/00.03/
// 00.04. TIDAK ada kolom/tabel baru; hanya kolom yang sudah ada yang dibaca.
//
// JENDELA PERISTIWA — bulan periode laporan. Sebuah baris muncul bila tanggal
// peristiwanya (ended_at untuk manajemen, closed_at untuk kantor) berada pada
// [awal bulan, akhir bulan] periode, persis pola filter bulan off_balance_repo.go
// (date_trunc('month', as_of) = date_trunc('month', $1)). Peristiwa bulan sebelumnya
// TIDAK muncul lagi, sehingga satu pemberhentian/penutupan tidak dilaporkan dua kali.
//
// Form tanpa peristiwa TIDAK di-append ke Tables, tetapi dicatat pada SkippedForms
// dengan alasan spesifik (pola runtimeSkipped builder.go) supaya penulis berkas
// menuliskan "# FORM x TIDAK DIBANGUN: tidak ada peristiwa ... pada periode ini",
// bukan menampilkan form kosong yang tampak lengkap.
//
// BATAS SUMBER — kolom tanpa sumber ditulis "-" beserta alasan, bukan dikarang:
//   - NIK tidak dimodelkan (keputusan privasi docs/KEPUTUSAN-OJK.md §4).
//   - Keanggotaan komite, fungsi kepatuhan, komisaris independen, penyebab berhenti
//     (sandi 1/2/3), dan Surat Pemberhentian belum tersimpan. `note` adalah teks
//     bebas: JANGAN diterjemahkan menjadi sandi penyebab; kolom Alasan (teks) diisi
//     dari `note` apa adanya.
//   - 00.10 kolom V Surat Pengangkatan memakai license_number/license_date
//     (000112:102-104), yaitu surat PENGANGKATAN. license_* TIDAK dipakai ulang
//     sebagai Surat Pemberhentian (kolom IX) yang memang belum ada.
//   - 00.12 sandi Jenis OJK tidak diturunkan dari office_type (teks bebas), sandi
//     kantor induk dan koordinat belum dimodelkan.
//
// ATURAN KOSONG: kolom yang menurut aturan form memang dikosongkan diisi kosong/"-"
// dengan alasan "tidak berlaku menurut aturan form", bukan "sumber tidak ada"
// (mis. fungsi kepatuhan bagi anggota dewan komisaris, PDF #263; nama/alamat TPE,
// PDF #279).

// dalamBulanPeristiwa melaporkan apakah tanggal peristiwa berada pada bulan laporan.
// Tanggal nil/kosong berarti belum ada peristiwa. Perbandingan memakai bulan
// kalender, sama dengan date_trunc('month') pada off_balance_repo.go.
func dalamBulanPeristiwa(t *time.Time, periodStart time.Time) bool {
	if t == nil || t.IsZero() {
		return false
	}
	return t.Year() == periodStart.Year() && t.Month() == periodStart.Month()
}

// buildKelembagaanPeristiwaTables memilih peristiwa pada bulan periode dan merakit
// Form 00.09/00.10/00.12. Form tanpa peristiwa tidak di-append; dicatat pada skipped
// dengan alasan spesifik agar ketidakhadirannya terbaca, bukan disembunyikan.
func buildKelembagaanPeristiwaTables(report domain.KelembagaanReport, periodStart time.Time) ([]TableSection, []OJKFormDefinition) {
	var tables []TableSection
	var skipped []OJKFormDefinition

	if sec, ada := buildForm00_09(report.Management, periodStart); ada {
		tables = append(tables, sec)
	} else {
		skipped = append(skipped, OJKFormDefinition{
			Form: "00.09", Name: formName("00.09"),
			UnavailableReason: "tidak ada peristiwa direksi/dewan komisaris berhenti pada periode ini",
		})
	}
	if sec, ada := buildForm00_10(report.Management, periodStart); ada {
		tables = append(tables, sec)
	} else {
		skipped = append(skipped, OJKFormDefinition{
			Form: "00.10", Name: formName("00.10"),
			UnavailableReason: "tidak ada peristiwa pejabat eksekutif berhenti pada periode ini",
		})
	}
	if sec, ada := buildForm00_12(report.Offices, periodStart); ada {
		tables = append(tables, sec)
	} else {
		skipped = append(skipped, OJKFormDefinition{
			Form: "00.12", Name: formName("00.12"),
			UnavailableReason: "tidak ada peristiwa penutupan kantor/terminal perbankan elektronik pada periode ini",
		})
	}
	return tables, skipped
}

// buildForm00_09 menyusun Form 00.09 dari manajemen kategori DIREKSI/KOMISARIS yang
// ended_at-nya pada bulan periode. Fungsi murni, dapat diuji tanpa basis data.
func buildForm00_09(rows []domain.BankManagement, periodStart time.Time) (TableSection, bool) {
	sec := TableSection{
		Form:     "00.09",
		Name:     formName("00.09"),
		KeyLabel: "Nama",
		Columns: []TableColumn{
			{Sandi: "I", Nama: "Nama"},
			{Sandi: "III", Nama: "Jabatan"},
			{Sandi: "IV", Nama: "Tanggal Mulai Menjabat"},
			{Sandi: "IX", Nama: "Tanggal Berhenti Menjabat"},
			{Sandi: "X", Nama: "Alasan Mengundurkan Diri/Pemberhentian"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "II", Nama: "NIK",
				Reason: "NIK sengaja tidak disimpan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4)"},
			{Sandi: "V.1", Nama: "Keanggotaan Komite Audit",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "V.2", Nama: "Keanggotaan Komite Pemantau Risiko",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "V.3", Nama: "Keanggotaan Komite Remunerasi dan Nominasi",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "V.4", Nama: "Keanggotaan Komite Manajemen Risiko",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "VI", Nama: "Membawahkan Fungsi Kepatuhan",
				Reason: "fungsi kepatuhan (sandi 1/2) hanya berlaku bagi direksi dan belum dimodelkan pada bank_management; bagi anggota dewan komisaris kolom ini memang dikosongkan menurut aturan form (PDF #263), bukan karena sumber tidak ada"},
			{Sandi: "VII", Nama: "Komisaris Independen",
				Reason: "penanda komisaris independen (sandi 1/2) belum dimodelkan pada bank_management"},
			{Sandi: "VIII", Nama: "Keterangan Penyebab Berhenti Menjabat",
				Reason: "sandi penyebab berhenti (1 pengunduran diri/2 pemberhentian/3 meninggal dunia) belum tersimpan terstruktur; note adalah teks bebas dan TIDAK diterjemahkan menjadi sandi, sedangkan kolom X Alasan memuat note apa adanya"},
		},
		Notes: []string{
			"Form ini KONDISIONAL berbasis peristiwa: hanya terbit bila ada anggota direksi/dewan komisaris dengan ended_at pada bulan periode (PDF #262). Peristiwa bulan lain tidak diulang.",
			"Kolom III Jabatan memakai ojk_position_code, yaitu sandi jabatan resmi OJK yang BANK isi (bank_management.ojk_position_code, migrasi 000112:124); sandi kosong ditulis \"-\", tidak diturunkan dari teks jabatan.",
			"Baris = per orang yang berhenti; tidak ada baris JUMLAH (PDF #258-259).",
			"Kolom tanpa sumber (NIK, komite, fungsi kepatuhan, komisaris independen, penyebab berhenti) didaftarkan belum tersedia; note hanya menjadi kolom X Alasan sebagai teks, bukan sandi.",
		},
	}
	for _, m := range rows {
		if m.Category != domain.KelembagaanKategoriDireksi && m.Category != domain.KelembagaanKategoriKomisaris {
			continue
		}
		if !dalamBulanPeristiwa(m.EndedAt, periodStart) {
			continue
		}
		sec.Rows = append(sec.Rows, TableRow{
			Key: dashIfEmpty(m.Name),
			Cells: []TableCell{
				{Sandi: "I", Nama: "Nama", Value: dashIfEmpty(m.Name)},
				{Sandi: "III", Nama: "Jabatan", Value: dashIfEmpty(m.OJKPositionCode)},
				{Sandi: "IV", Nama: "Tanggal Mulai Menjabat", Value: formatTanggalAtauDash(m.StartedAt)},
				{Sandi: "IX", Nama: "Tanggal Berhenti Menjabat", Value: formatTanggalAtauDash(m.EndedAt)},
				{Sandi: "X", Nama: "Alasan Mengundurkan Diri/Pemberhentian", Value: dashIfEmpty(m.Note)},
			},
		})
	}
	return sec, len(sec.Rows) > 0
}

// buildForm00_10 menyusun Form 00.10 dari pejabat eksekutif yang ended_at-nya pada
// bulan periode. Fungsi murni, dapat diuji tanpa basis data.
func buildForm00_10(rows []domain.BankManagement, periodStart time.Time) (TableSection, bool) {
	sec := TableSection{
		Form:     "00.10",
		Name:     formName("00.10"),
		KeyLabel: "Nama",
		Columns: []TableColumn{
			{Sandi: "I", Nama: "Nama"},
			{Sandi: "IV", Nama: "Tanggal Mulai Menjabat"},
			{Sandi: "V.1", Nama: "Surat Pengangkatan - No."},
			{Sandi: "V.2", Nama: "Surat Pengangkatan - Tanggal"},
			{Sandi: "VIII", Nama: "Alasan Mengundurkan Diri/Pemberhentian"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "II", Nama: "NIK",
				Reason: "NIK sengaja tidak disimpan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4)"},
			{Sandi: "III.1", Nama: "Jabatan - Kepatuhan",
				Reason: "sandi fungsi jabatan pejabat eksekutif (00/01/02) per 5 fungsi belum dimodelkan pada bank_management"},
			{Sandi: "III.2", Nama: "Jabatan - Manajemen Risiko",
				Reason: "sandi fungsi jabatan pejabat eksekutif (00/01/02) per 5 fungsi belum dimodelkan pada bank_management"},
			{Sandi: "III.3", Nama: "Jabatan - Audit Intern",
				Reason: "sandi fungsi jabatan pejabat eksekutif (00/01/02) per 5 fungsi belum dimodelkan pada bank_management"},
			{Sandi: "III.4", Nama: "Jabatan - APU dan PPT",
				Reason: "sandi fungsi jabatan pejabat eksekutif (00/01/02) per 5 fungsi belum dimodelkan pada bank_management"},
			{Sandi: "III.5", Nama: "Jabatan - Lainnya",
				Reason: "sandi fungsi jabatan pejabat eksekutif (00/01/02) per 5 fungsi belum dimodelkan pada bank_management"},
			{Sandi: "VI.1", Nama: "Keanggotaan Komite Audit",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "VI.2", Nama: "Keanggotaan Komite Pemantau Risiko",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "VI.3", Nama: "Keanggotaan Komite Remunerasi dan Nominasi",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "VI.4", Nama: "Keanggotaan Komite Manajemen Risiko",
				Reason: "keanggotaan komite per orang belum dimodelkan pada bank_management"},
			{Sandi: "VII", Nama: "Keterangan Penyebab Berhenti Menjabat",
				Reason: "sandi penyebab berhenti (1 pengunduran diri/2 pemberhentian/3 meninggal dunia) belum tersimpan terstruktur; note adalah teks bebas dan TIDAK diterjemahkan menjadi sandi, sedangkan kolom VIII Alasan memuat note apa adanya"},
			{Sandi: "IX", Nama: "Surat Pemberhentian",
				Reason: "surat pemberhentian belum tersimpan; license_number/license_date adalah surat PENGANGKATAN (kolom V) dan tidak dipakai ulang untuk pemberhentian"},
		},
		Notes: []string{
			"Form ini KONDISIONAL berbasis peristiwa: hanya terbit bila ada pejabat eksekutif dengan ended_at pada bulan periode (PDF #268). Peristiwa bulan lain tidak diulang.",
			"Kolom IV/Tgl dan V/Surat Pengangkatan diisi dari started_at dan license_number/license_date (surat pengangkatan, migrasi 000112:102-104). license_* tidak dipakai sebagai Surat Pemberhentian (kolom IX) yang belum ada.",
			"Baris = per pejabat eksekutif yang berhenti; tidak ada baris JUMLAH (PDF #264-265).",
			"Kolom tanpa sumber (NIK, 5 sub-kolom jabatan, komite, penyebab berhenti, Surat Pemberhentian) didaftarkan belum tersedia; note hanya menjadi kolom VIII Alasan sebagai teks, bukan sandi.",
		},
	}
	for _, m := range rows {
		if m.Category != domain.KelembagaanKategoriPejabatEksekutif {
			continue
		}
		if !dalamBulanPeristiwa(m.EndedAt, periodStart) {
			continue
		}
		sec.Rows = append(sec.Rows, TableRow{
			Key: dashIfEmpty(m.Name),
			Cells: []TableCell{
				{Sandi: "I", Nama: "Nama", Value: dashIfEmpty(m.Name)},
				{Sandi: "IV", Nama: "Tanggal Mulai Menjabat", Value: formatTanggalAtauDash(m.StartedAt)},
				{Sandi: "V.1", Nama: "Surat Pengangkatan - No.", Value: dashIfEmpty(m.LicenseNumber)},
				{Sandi: "V.2", Nama: "Surat Pengangkatan - Tanggal", Value: formatTanggalAtauDash(m.LicenseDate)},
				{Sandi: "VIII", Nama: "Alasan Mengundurkan Diri/Pemberhentian", Value: dashIfEmpty(m.Note)},
			},
		})
	}
	return sec, len(sec.Rows) > 0
}

// buildForm00_12 menyusun Form 00.12 dari kantor/TPE yang closed_at-nya pada bulan
// periode. Fungsi murni, dapat diuji tanpa basis data.
func buildForm00_12(rows []domain.BankOffice, periodStart time.Time) (TableSection, bool) {
	sec := TableSection{
		Form:     "00.12",
		Name:     formName("00.12"),
		KeyLabel: "Sandi/Nama Kantor",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Sandi Kantor/Kode Kantor"},
			{Sandi: "IV", Nama: "Nama Kantor"},
			{Sandi: "VI", Nama: "Alamat"},
			{Sandi: "VII", Nama: "Tanggal Pelaksanaan"},
			{Sandi: "VIII", Nama: "Lokasi"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "I", Nama: "Jenis",
				Reason: "office_type adalah teks bebas bank (000112:44) dan TIDAK dipetakan ke sandi Jenis OJK 01-08/99; sandi jenis penutupan belum diisi sebagai sandi"},
			{Sandi: "III", Nama: "Sandi Kantor Induk",
				Reason: "relasi kantor induk belum dimodelkan pada bank_offices (tidak ada parent_id)"},
			{Sandi: "V", Nama: "Koordinat",
				Reason: "koordinat kantor belum dimodelkan pada bank_offices; tidak dikarang dari alamat"},
		},
		Notes: []string{
			"Form ini KONDISIONAL berbasis peristiwa: hanya terbit bila ada kantor/TPE dengan closed_at pada bulan periode (PDF #279). Peristiwa bulan lain tidak diulang.",
			"Kolom II Sandi Kantor dan IV Nama Kantor diisi dari bank_offices.code/name; kolom VII Tanggal Pelaksanaan dari closed_at; kolom VIII Lokasi dari ojk_kabupaten_code.",
			"Baris = per kantor/TPE yang ditutup; tidak ada baris JUMLAH (PDF #276).",
			"Untuk baris TPE, kolom IV Nama Kantor dan VI Alamat memang dikosongkan menurut aturan form (PDF #279), bukan karena sumber tidak ada; alasannya ditulis per baris.",
			"Kolom I Jenis, III Sandi Kantor Induk, dan V Koordinat belum punya sumber; ditulis \"-\" beserta alasan, bukan dikarang.",
		},
	}
	for _, o := range rows {
		if !dalamBulanPeristiwa(o.ClosedAt, periodStart) {
			continue
		}
		tpe := adalahTPE(o.OfficeType)
		row := TableRow{Key: kelembagaanOfficeKey(o)}
		nama := dashIfEmpty(o.Name)
		alamat := dashIfEmpty(o.Address)
		if tpe {
			// Aturan form mengosongkan nama dan alamat untuk TPE; beda dari kolom
			// tanpa sumber. Alasan ditulis "tidak berlaku menurut aturan form".
			nama, alamat = "-", "-"
			row.Reason = "kolom IV Nama Kantor dan VI Alamat dikosongkan karena baris ini TPE menurut aturan form (PDF #279), bukan karena sumber tidak ada"
		}
		row.Cells = append(row.Cells,
			TableCell{Sandi: "II", Nama: "Sandi Kantor/Kode Kantor", Value: dashIfEmpty(o.Code)},
			TableCell{Sandi: "IV", Nama: "Nama Kantor", Value: nama},
			TableCell{Sandi: "VI", Nama: "Alamat", Value: alamat},
			TableCell{Sandi: "VII", Nama: "Tanggal Pelaksanaan", Value: formatTanggalAtauDash(o.ClosedAt)},
			TableCell{Sandi: "VIII", Nama: "Lokasi", Value: dashIfEmpty(o.OJKKabupatenCode)},
		)
		sec.Rows = append(sec.Rows, row)
	}
	return sec, len(sec.Rows) > 0
}

// adalahTPE melaporkan apakah office_type bank menunjuk terminal perbankan elektronik.
// office_type adalah teks bebas (000112:44, contoh TERMINAL_PERBANKAN_ELEKTRONIK), jadi
// pengenalan ini HANYA untuk menerapkan aturan kosong Form 00.12, BUKAN memetakan teks
// ke sandi Jenis OJK (kolom I tetap tidak tersedia).
func adalahTPE(officeType string) bool {
	u := strings.ToUpper(strings.TrimSpace(officeType))
	return strings.Contains(u, "TPE") ||
		strings.Contains(u, "TERMINAL_PERBANKAN_ELEKTRONIK") ||
		strings.Contains(u, "ATM") ||
		strings.Contains(u, "EDC")
}
