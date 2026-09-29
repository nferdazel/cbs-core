package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form00_01.go membangun Form 00.01 "DATA KEPEMILIKAN BPR" dari register pemegang
// saham kepemilikan_bpr_register (migrasi 000116).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.01 "DATA KEPEMILIKAN BPR": PDF #page 72 (hlm. tercetak 20). Delapan
//     kolom: I Nama, II Alamat, III Jenis, IV No. Identitas, V Status Pemegang Saham,
//     VI Jumlah Nominal, VII Persentase Kepemilikan, VIII Status Perubahan. Satu baris
//     = satu pemegang saham; TIDAK ADA baris JUMLAH.
//   - Sandi: PDF #page 73 (hlm. 21). III 01/02/03/04; V 01 PSP / 02 Non PSP.
//   - Penjelasan: PDF #page 74 (hlm. 22). VIII 1/2/3/9; Alamat dan No. Identitas
//     dapat dikosongkan untuk kepemilikan kurang dari 2%.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Kolom IV "No. Identitas" TIDAK DISIMPAN: keputusan privasi yang berlaku
//     (NIK/NPWP tidak dibuka ke keluaran laporan, docs/KEPUTUSAN-OJK.md §4 dan §7
//     butir 6). Selalu ditulis "-" beserta alasannya, dan tidak pernah dikarang.
//   - Kolom II "Alamat" boleh kosong karena form mengizinkannya untuk kepemilikan
//     kurang dari 2% (PDF #page 74). Laporan mengikuti isi bank apa adanya; kekosongan
//     diberi alasan aturan form, BUKAN alasan "sumber tidak ada".
//   - Kolom VI/VII adalah angka bank. Sistem tidak menurunkan nominal dari persentase
//     kali modal dan TIDAK memvalidasi Σ persentase = 100%; itu urusan bank.
//   - Form ini TIDAK punya kolom I "Sandi Kantor": ia dimulai dari Nama, jadi tidak
//     ada kantor pelapor yang perlu dipilih.
//   - Tidak ada baris JUMLAH: form berakhir per pemegang saham (PDF #page 72).

// KepemilikanRegisterSource menyediakan baris register pemegang saham bank-wide untuk
// Form 00.01. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form
// 00.01 dinyatakan belum tersedia, bukan ditulis kosong.
type KepemilikanRegisterSource interface {
	ListKepemilikanForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.KepemilikanItem, error)
}

// BuildForm00_01 menyusun tabel Form 00.01 dari baris register pemegang saham. Fungsi
// ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada baris, Rows kosong
// tetapi daftar kolom dan alasan kolom IV tetap dibawa.
func BuildForm00_01(rows []domain.KepemilikanItem) TableSection {
	sec := TableSection{
		Form:     "00.01",
		Name:     formName("00.01"),
		KeyLabel: "Nama|Jenis",
		Columns: []TableColumn{
			{Sandi: "I", Nama: "Nama"},
			{Sandi: "II", Nama: "Alamat"},
			{Sandi: "III", Nama: "Jenis"},
			{Sandi: "IV", Nama: "No. Identitas"},
			{Sandi: "V", Nama: "Status Pemegang Saham"},
			{Sandi: "VI", Nama: "Jumlah Nominal"},
			{Sandi: "VII", Nama: "Persentase Kepemilikan"},
			{Sandi: "VIII", Nama: "Status Perubahan"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "IV", Nama: "No. Identitas",
				Reason: "No. Identitas (NIK/NPWP) sengaja tidak disimpan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6); kolom ditulis \"-\" dan tidak pernah dikarang"},
		},
		Notes: []string{
			"Baris per pemegang saham dari register kepemilikan_bpr_register yang bank isi; satu baris = satu pemegang saham dan TIDAK ADA baris JUMLAH (Form 00.01, PDF #page 72).",
			"Jenis (III) mengikuti sandi PDF #page 73: 01 Perorangan, 02 Badan Hukum, 03 Pemerintah Daerah, 04 Publik. Status Pemegang Saham (V): 01 PSP, 02 Non PSP. Status Perubahan (VIII): 1 pemegang saham baru, 2 perubahan yang mengakibatkan perubahan PSP, 3 perubahan yang tidak mengakibatkan perubahan PSP, 9 tidak ada perubahan (PDF #page 74).",
			"Alamat (II) boleh kosong: form mengizinkannya untuk kepemilikan kurang dari 2% (PDF #page 74). Bank mengisi apa adanya; kekosongan tidak diisi alamat karangan.",
			"No. Identitas (IV) sengaja tidak disimpan sebagai keputusan privasi (NIK/NPWP tidak dibuka ke keluaran laporan, docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6), sehingga selalu ditulis \"-\" beserta alasannya.",
			"Jumlah Nominal (VI) dan Persentase Kepemilikan (VII) adalah angka bank. Sistem tidak menghitungnya dan TIDAK memvalidasi bahwa seluruh persentase berjumlah 100%; komposisi kepemilikan adalah urusan bank, bukan sistem.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form00_01RowKey(item)}
		alamat := dashIfEmpty(item.ShareholderAddress)
		if alamat == "-" {
			// Kekosongan mengikuti aturan form (<2% boleh dikosongkan), bukan alasan
			// "sumber tidak ada": bank memang tidak mengisi alamatnya.
			row.Reason = "alamat dikosongkan bank; Form 00.01 mengizinkan kolom Alamat kosong untuk kepemilikan kurang dari 2% (PDF #page 74), bukan karena sumber tidak ada"
		}
		row.Cells = []TableCell{
			{Sandi: "I", Nama: "Nama", Value: dashIfEmpty(item.ShareholderName)},
			{Sandi: "II", Nama: "Alamat", Value: alamat},
			{Sandi: "III", Nama: "Jenis", Value: dashIfEmpty(item.ShareholderTypeCode)},
			// Kolom IV: identitas sengaja tidak disimpan; selalu "-" dan alasannya
			// didaftarkan pada Unavailable. Tidak ada nilai identitas yang mengalir.
			{Sandi: "IV", Nama: "No. Identitas", Value: "-"},
			{Sandi: "V", Nama: "Status Pemegang Saham", Value: dashIfEmpty(item.ShareholderStatusCode)},
			{Sandi: "VI", Nama: "Jumlah Nominal", Value: FormatRupiah(item.NominalAmount)},
			{Sandi: "VII", Nama: "Persentase Kepemilikan", Value: FormatPersen(item.OwnershipPercentage)},
			{Sandi: "VIII", Nama: "Status Perubahan", Value: dashIfEmpty(item.ChangeStatusCode)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	return sec
}

// form00_01RowKey mengidentifikasi satu baris Form 00.01 secara terbaca: nama
// pemegang saham dan sandi jenisnya.
func form00_01RowKey(item domain.KepemilikanItem) string {
	return item.ShareholderName + "|" + item.ShareholderTypeCode
}
