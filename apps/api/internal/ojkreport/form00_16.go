package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// PihakLawanOJKSource menyediakan baris register pihak lawan untuk Form 00.16. Kontraknya
// opsional pada perakitan RepoSource: tanpa sumber ini Form 00.16 dinyatakan belum
// tersedia, bukan ditulis kosong.
type PihakLawanOJKSource interface {
	ListPihakLawanForOJK(ctx context.Context) ([]domain.PihakLawanItem, error)
}

// form00_16.go membangun Form 00.16 "DAFTAR PIHAK LAWAN" dari register
// pihak_lawan_register (migrasi 000130).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Susunan PDF #page 286 (hlm. tercetak 234) kolom I-IX, PDF #page 287 (hlm. 235)
//     kolom X-XX, sandi PDF #page 288-289 (hlm. 236-237), penjelasan PDF #page 290-291
//     (hlm. 238-239). Dua puluh kolom, TANPA baris JUMLAH.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Kolom III Nomor Identitas (NIK/NPWP) dan VI NPWP SENGAJA TIDAK DISIMPAN mengikuti
//     keputusan privasi (docs/KEPUTUSAN-OJK.md §4 butir III dan §7 butir 6, pola Form
//     00.01/06.02/form berhenti). Keduanya selalu ditulis "-" beserta alasan.
//   - Sandi kondisional (II hanya perorangan, IX hanya LJK) ditulis apa adanya; bank yang
//     menentukan golongannya.
//   - Kolom XII/XIII tanpa peringkat memakai sentinel baku 9/99 (PDF #291); kolom ditulis
//     apa adanya, laporan tidak menebak.
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice, bukan dari register.
//   - Tanpa baris register, form dinyatakan belum tersedia, bukan ditulis kosong.

// BuildForm00_16 menyusun tabel Form 00.16 dari baris register pihak lawan dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Form ini TIDAK
// memiliki baris JUMLAH.
func BuildForm00_16(rows []domain.PihakLawanItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "00.16",
		Name:     formName("00.16"),
		KeyLabel: "ID Pihak Lawan",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Jenis Identitas"},
			{Sandi: "III", Nama: "Nomor Identitas"},
			{Sandi: "IV", Nama: "Jenis Kelamin"},
			{Sandi: "V", Nama: "Nama Lengkap/Nama Badan Usaha"},
			{Sandi: "VI", Nama: "NPWP"},
			{Sandi: "VII", Nama: "Kewarganegaraan"},
			{Sandi: "VIII", Nama: "Negara"},
			{Sandi: "IX", Nama: "Jenis Kegiatan Usaha"},
			{Sandi: "X", Nama: "Hubungan dengan Bank"},
			{Sandi: "XI", Nama: "Golongan Pihak Lawan"},
			{Sandi: "XII", Nama: "Lembaga Pemeringkat"},
			{Sandi: "XIII", Nama: "Peringkat Pihak Lawan"},
			{Sandi: "XIV", Nama: "Tanggal Pemeringkatan"},
			{Sandi: "XV", Nama: "Tanggal Lahir"},
			{Sandi: "XVI", Nama: "Lokasi"},
			{Sandi: "XVII", Nama: "ID Grup"},
			{Sandi: "XVIII", Nama: "Nama Grup"},
			{Sandi: "XIX", Nama: "No. Telp/No. HP"},
			{Sandi: "XX", Nama: "Alamat"},
		},
		Notes: []string{
			"Baris per pihak lawan dari register pihak_lawan_register yang bank isi (Form 00.16, PDF #page 286-291); satu baris = satu pihak lawan, TANPA baris JUMLAH.",
			"Form ini memuat SELURUH pihak lawan baik bank maupun bukan bank yang melakukan transaksi dengan BPR pelapor (PDF #page 290), termasuk yang bukan nasabah.",
			"Batas privasi: kolom III Nomor Identitas (NIK/NPWP) dan VI NPWP sengaja tidak disimpan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4 butir III dan §7 butir 6, sama seperti Form 00.01/06.02/form berhenti); keduanya ditulis \"-\" dan tidak pernah dikarang.",
			"Jenis Identitas (II) memakai sandi baku PDF #page 288: 1 KTP, 2 Paspor, 3 KITAS/KITAP, 4 Kartu Keluarga; hanya diisi untuk perorangan. Jenis Kelamin (IV): 1 Laki-laki, 2 Perempuan.",
			"Jenis Kegiatan Usaha (IX) memakai sandi baku PDF #page 288: 1 Konvensional, 2 Syariah; hanya diisi untuk Lembaga Jasa Keuangan.",
			"Hubungan dengan Bank (X) memakai sandi baku PDF #page 288: 12 Pihak Terkait, 20 Pihak Tidak Terkait.",
			"Kewarganegaraan (VII) dan Negara (VIII) mengacu Lampiran 10; Golongan Pihak Lawan (XI) mengacu Lampiran 02; Lokasi (XVI) mengacu Lampiran 03. Daftar lampiran tidak disalin ke kode, sehingga ditulis apa adanya.",
			"Lembaga Pemeringkat (XII) dan Peringkat Pihak Lawan (XIII) mengacu Lampiran 08/09; tanpa peringkat/pemeringkat tak diakui OJK diisi sentinel 9 dan 99 (PDF #page 291). Tanggal Pemeringkatan (XIV) adalah tanggal reviu terakhir.",
			"Tanggal Lahir (XV) dikosongkan untuk pihak lawan bukan perorangan; dibuat dari bank, bukan diturunkan.",
			"ID Grup (XVII) dan Nama Grup (XVIII) ditatausahakan bank menurut kriterianya sendiri (PDF #page 291).",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form00_16RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "Jenis Identitas", Value: form00_16JenisIdentitasLabel(item.JenisIdentitasCode)},
			{Sandi: "III", Nama: "Nomor Identitas", Value: "-"},
			{Sandi: "IV", Nama: "Jenis Kelamin", Value: form00_16JenisKelaminLabel(item.JenisKelaminCode)},
			{Sandi: "V", Nama: "Nama Lengkap/Nama Badan Usaha", Value: dashIfEmpty(item.Nama)},
			{Sandi: "VI", Nama: "NPWP", Value: "-"},
			{Sandi: "VII", Nama: "Kewarganegaraan", Value: dashIfEmpty(item.KewarganegaraanCode)},
			{Sandi: "VIII", Nama: "Negara", Value: dashIfEmpty(item.NegaraCode)},
			{Sandi: "IX", Nama: "Jenis Kegiatan Usaha", Value: form00_16JenisUsahaLabel(item.JenisUsahaCode)},
			{Sandi: "X", Nama: "Hubungan dengan Bank", Value: form00_16HubunganLabel(item.HubunganBankCode)},
			{Sandi: "XI", Nama: "Golongan Pihak Lawan", Value: dashIfEmpty(item.GolonganCode)},
			{Sandi: "XII", Nama: "Lembaga Pemeringkat", Value: dashIfEmpty(item.LembagaPemeringkatCode)},
			{Sandi: "XIII", Nama: "Peringkat Pihak Lawan", Value: dashIfEmpty(item.PeringkatCode)},
			{Sandi: "XIV", Nama: "Tanggal Pemeringkatan", Value: formatTanggalOpsional(item.TanggalPemeringkatan)},
			{Sandi: "XV", Nama: "Tanggal Lahir", Value: formatTanggalOpsional(item.TanggalLahir)},
			{Sandi: "XVI", Nama: "Lokasi", Value: dashIfEmpty(item.LokasiCode)},
			{Sandi: "XVII", Nama: "ID Grup", Value: dashIfEmpty(item.GrupID)},
			{Sandi: "XVIII", Nama: "Nama Grup", Value: dashIfEmpty(item.GrupNama)},
			{Sandi: "XIX", Nama: "No. Telp/No. HP", Value: dashIfEmpty(item.Telepon)},
			{Sandi: "XX", Nama: "Alamat", Value: dashIfEmpty(item.Alamat)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form00_16RowKey mengidentifikasi satu baris Form 00.16 secara terbaca.
func form00_16RowKey(item domain.PihakLawanItem) string {
	if item.PihakLawanID != "" {
		return item.PihakLawanID
	}
	return "pihak-lawan:" + item.ID.String()
}

// formatTanggalOpsional menulis tanggal yang boleh kosong.
func formatTanggalOpsional(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02")
}

// form00_16JenisIdentitasLabel menulis sandi II sebagai label baku (PDF #288).
func form00_16JenisIdentitasLabel(code string) string {
	switch code {
	case domain.PihakLawanIdentitasKTP:
		return "1 - Kartu Tanda Penduduk"
	case domain.PihakLawanIdentitasPaspor:
		return "2 - Paspor"
	case domain.PihakLawanIdentitasKITAS:
		return "3 - KITAS/KITAP"
	case domain.PihakLawanIdentitasKK:
		return "4 - Kartu Keluarga"
	default:
		return dashIfEmpty(code)
	}
}

// form00_16JenisKelaminLabel menulis sandi IV sebagai label baku (PDF #288).
func form00_16JenisKelaminLabel(code string) string {
	switch code {
	case domain.PihakLawanKelaminLakiLaki:
		return "1 - Laki-laki"
	case domain.PihakLawanKelaminPerempuan:
		return "2 - Perempuan"
	default:
		return dashIfEmpty(code)
	}
}

// form00_16JenisUsahaLabel menulis sandi IX sebagai label baku (PDF #288).
func form00_16JenisUsahaLabel(code string) string {
	switch code {
	case domain.PihakLawanUsahaKonvensional:
		return "1 - Konvensional"
	case domain.PihakLawanUsahaSyariah:
		return "2 - Syariah"
	default:
		return dashIfEmpty(code)
	}
}

// form00_16HubunganLabel menulis sandi X sebagai label baku (PDF #288).
func form00_16HubunganLabel(code string) string {
	switch code {
	case domain.PihakLawanHubunganTerkait:
		return "12 - Pihak Terkait"
	case domain.PihakLawanHubunganTidakTerkait:
		return "20 - Pihak Tidak Terkait"
	default:
		return dashIfEmpty(code)
	}
}
