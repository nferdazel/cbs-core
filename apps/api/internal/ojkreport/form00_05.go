package ojkreport

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
)

// PihakTerkaitOJKSource menyediakan baris register pihak terkait lainnya untuk Form 00.05.
// Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 00.05 dinyatakan
// belum tersedia, bukan ditulis kosong.
type PihakTerkaitOJKSource interface {
	ListPihakTerkaitForOJK(ctx context.Context) ([]domain.PihakTerkaitItem, error)
}

// form00_05.go membangun Form 00.05 "DATA PIHAK TERKAIT LAINNYA" dari register
// pihak_terkait_lainnya_register (migrasi 000127).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.05 "DATA PIHAK TERKAIT LAINNYA": PDF #page 96 (hlm. tercetak 44) susunan,
//     sandi PDF #page 97 (hlm. 45), penjelasan PDF #page 98 (hlm. 46). Lima kolom:
//     I Nama Pihak Terkait, II No. Identitas, III Alamat Pihak Terkait,
//     IV Jenis Pihak Terkait, V Hubungan Pihak Terkait. TIDAK ADA baris JUMLAH.
//   - Sandi IV: 01 Perorangan, 02 Perusahaan/Badan, 03 Pemerintah Daerah/Pusat.
//     Sandi V: 01-06 (definisi rinci PDF #98).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Form ini memuat pihak terkait BPR SELAIN pemegang saham, anggota direksi, anggota
//     dewan komisaris, dan pejabat eksekutif BPR (PDF #98). Mereka tidak harus nasabah,
//     karena itu register berdiri sendiri dan tidak menautkan ke customers.
//   - Kolom II No. Identitas (NIK/NPWP) SENGAJA TIDAK DISIMPAN mengikuti keputusan privasi
//     (docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6, pola Form 00.01/06.02/form berhenti).
//     Kolom selalu ditulis "-" beserta alasannya, bukan dikarang.
//   - Sandi IV dan V adalah ISIAN BANK; laporan tidak menurunkan atau menerka sandi.
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice, bukan dari register.
//   - Tanpa baris register, form dinyatakan belum tersedia, bukan ditulis kosong.

// BuildForm00_05 menyusun tabel Form 00.05 dari baris register pihak terkait dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Form ini tidak
// memiliki baris JUMLAH.
func BuildForm00_05(rows []domain.PihakTerkaitItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "00.05",
		Name:     formName("00.05"),
		KeyLabel: "Nama Pihak Terkait",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "No. Identitas"},
			{Sandi: "III", Nama: "Alamat Pihak Terkait"},
			{Sandi: "IV", Nama: "Jenis Pihak Terkait"},
			{Sandi: "V", Nama: "Hubungan Pihak Terkait"},
		},
		Notes: []string{
			"Baris per pihak terkait dari register pihak_terkait_lainnya_register yang bank isi (Form 00.05, PDF #page 96-98); satu baris = satu pihak terkait, TANPA baris JUMLAH.",
			"Form ini memuat pihak terkait BPR SELAIN pemegang saham, anggota direksi, anggota dewan komisaris, dan pejabat eksekutif BPR (PDF #page 98); pihak tersebut tidak harus nasabah.",
			"Batas privasi: kolom II No. Identitas (NIK dalam hal perorangan atau NPWP dalam hal badan usaha) sengaja tidak disimpan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6, sama seperti Form 00.01/06.02/form berhenti); kolom ditulis \"-\" dan tidak pernah dikarang.",
			"Jenis Pihak Terkait (IV) memakai sandi baku PDF #page 97: 01 Perorangan, 02 Perusahaan atau Badan, 03 Pemerintah Daerah atau Pemerintah Pusat. Isian bank.",
			"Hubungan Pihak Terkait (V) memakai sandi baku PDF #page 97/98: 01 pengendali/keluarga, 02 perusahaan bukan bank milik pengurus (>=25% modal disetor), 03 BPR/BPRS lain yang dimiliki (>=10% modal disetor), 04 BPR/BPRS lain dengan rangkap komisaris (>=50%), 05 perusahaan dengan rangkap komisaris (>=50%), 06 peminjam yang dijamin pengurus. Definisi rinci tidak disalin ke kode; isian bank.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form00_05RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "No. Identitas", Value: "-"},
			{Sandi: "III", Nama: "Alamat Pihak Terkait", Value: dashIfEmpty(item.Alamat)},
			{Sandi: "IV", Nama: "Jenis Pihak Terkait", Value: form00_05JenisLabel(item.JenisCode)},
			{Sandi: "V", Nama: "Hubungan Pihak Terkait", Value: form00_05HubunganLabel(item.HubunganCode)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form00_05RowKey mengidentifikasi satu baris Form 00.05 secara terbaca; bila nama kosong
// (tidak mungkin karena wajib diisi), dipakai id baris agar tetap unik.
func form00_05RowKey(item domain.PihakTerkaitItem) string {
	if item.Nama != "" {
		return item.Nama
	}
	return "pihak-terkait:" + item.ID.String()
}

// form00_05JenisLabel menulis sandi IV sebagai label baku (PDF #97).
func form00_05JenisLabel(code string) string {
	switch code {
	case domain.PihakTerkaitJenisPerorangan:
		return "01 - Perorangan"
	case domain.PihakTerkaitJenisBadan:
		return "02 - Perusahaan atau Badan"
	case domain.PihakTerkaitJenisPemerintah:
		return "03 - Pemerintah Daerah atau Pemerintah Pusat"
	default:
		return dashIfEmpty(code)
	}
}

// form00_05HubunganLabel menulis sandi V sebagai label ringkas (definisi rinci di PDF #98,
// tidak disalin ke kode).
func form00_05HubunganLabel(code string) string {
	switch code {
	case domain.PihakTerkaitHubunganPengendaliKeluarga:
		return "01 - Pengendali/Keluarga"
	case domain.PihakTerkaitHubunganPerusahaanBukanBank:
		return "02 - Perusahaan Bukan Bank Milik Pengurus"
	case domain.PihakTerkaitHubunganBPRLainDimiliki:
		return "03 - BPR/BPRS Lain yang Dimiliki"
	case domain.PihakTerkaitHubunganBPRRangkapKomisaris:
		return "04 - BPR/BPRS Lain dengan Rangkap Komisaris"
	case domain.PihakTerkaitHubunganPerusahaanRangkap:
		return "05 - Perusahaan dengan Rangkap Komisaris"
	case domain.PihakTerkaitHubunganPeminjamDijamin:
		return "06 - Peminjam yang Dijamin Pengurus"
	default:
		return dashIfEmpty(code)
	}
}
