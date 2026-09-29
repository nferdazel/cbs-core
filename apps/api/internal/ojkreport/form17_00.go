package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form17_00.go membangun Form 17.00 "DAFTAR PROPERTI TERBENGKALAI" dari register
// properti_terbengkalai_register (migrasi 000118).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 17.00 "DAFTAR PROPERTI TERBENGKALAI": PDF #page 229 (hlm. tercetak 177).
//     Kolom I Sandi Kantor, II No. Register, III Jenis Properti Terbengkalai,
//     IV Alamat Properti Terbengkalai, V Koordinat, VI Tanggal Penetapan,
//     VII Biaya Perolehan atau Nilai Wajar, VIII Akumulasi Penyusutan atau Amortisasi,
//     IX Jumlah, X Metode Pengukuran. Satu baris = satu properti; TIDAK ADA baris JUMLAH.
//   - Sandi: PDF #page 230 (hlm. 178). III 1 Tanah / 2 Bangunan / 3 Tanah dan Bangunan;
//     X 1 Model Biaya / 2 Model Revaluasi (Nilai Wajar).
//   - Penjelasan: PDF #page 231-232 (hlm. 179-180). No. Register unik, no reuse/no
//     recycle; IX Jumlah = VII - VIII termasuk akumulasi penurunan nilai.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH angka VII/VIII, sandi, alamat, dan tanggal adalah isian bank dari
//     register. Sistem tidak menghitung biaya perolehan maupun akumulasi.
//   - Kolom IX "Jumlah" adalah turunan: biaya perolehan atau nilai wajar (VII) dikurangi
//     akumulasi penyusutan atau amortisasi (VIII), mengikuti domain.PropertiItem.Jumlah.
//     Bila hasilnya negatif, kolom IX ditulis "-" beserta alasannya, bukan angka negatif.
//   - Form ini TIDAK punya baris JUMLAH (PDF #page 229): tabel berakhir setelah baris
//     properti terakhir.
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice; register sendiri bank-wide, tidak per kantor.
//   - Nomor register unik dan tidak boleh dipakai ulang; baris NONAKTIF tidak dibaca
//     (ListPropertiForOJK menyaring status AKTIF), sehingga tidak muncul di form.

// PropertiRegisterSource menyediakan baris register properti terbengkalai bank-wide
// untuk Form 17.00. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form
// 17.00 dinyatakan belum tersedia, bukan ditulis kosong.
type PropertiRegisterSource interface {
	ListPropertiForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.PropertiItem, error)
}

// BuildForm17_00 menyusun tabel Form 17.00 dari baris register properti dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak
// ada baris, Rows kosong tetapi daftar kolom tetap dibawa. TIDAK ada baris JUMLAH.
func BuildForm17_00(rows []domain.PropertiItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "17.00",
		Name:     formName("17.00"),
		KeyLabel: "No. Register",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "No. Register"},
			{Sandi: "III", Nama: "Jenis Properti Terbengkalai"},
			{Sandi: "IV", Nama: "Alamat Properti Terbengkalai"},
			{Sandi: "V", Nama: "Koordinat"},
			{Sandi: "VI", Nama: "Tanggal Penetapan"},
			{Sandi: "VII", Nama: "Biaya Perolehan atau Nilai Wajar"},
			{Sandi: "VIII", Nama: "Akumulasi Penyusutan atau Amortisasi"},
			{Sandi: "IX", Nama: "Jumlah"},
			{Sandi: "X", Nama: "Metode Pengukuran"},
		},
		Notes: []string{
			"Baris per properti terbengkalai dari register properti_terbengkalai_register yang bank isi (Form 17.00, PDF #page 229); satu baris = satu properti dan TIDAK ada baris JUMLAH.",
			"Jenis Properti Terbengkalai (III) mengikuti sandi PDF #page 230: 1 Tanah, 2 Bangunan, 3 Tanah dan Bangunan. Metode Pengukuran (X): 1 Model Biaya, 2 Model Revaluasi (Nilai Wajar).",
			"Jumlah (IX) dihitung sebagai Biaya Perolehan atau Nilai Wajar (VII) dikurangi Akumulasi Penyusutan atau Amortisasi (VIII), termasuk akumulasi kerugian penurunan nilai bila ada (PDF #page 232). Bila hasilnya negatif, kolom IX ditulis \"-\" beserta alasannya, bukan angka negatif.",
			"No. Register (II) unik dan tidak boleh dipakai ulang (no reuse/no recycle, PDF #page 231). Penghapusan register adalah soft-delete NONAKTIF; baris NONAKTIF tidak ikut form ini karena sumbernya menyaring status AKTIF.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form17_00RowKey(item)}
		jumlah, ok := item.Jumlah()
		if !ok {
			row.Reason = "akumulasi penyusutan atau amortisasi melebihi biaya perolehan atau nilai wajar; kolom IX Jumlah tidak dapat dihitung dan tidak diisi angka negatif"
		}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "No. Register", Value: dashIfEmpty(item.NoRegister)},
			{Sandi: "III", Nama: "Jenis Properti Terbengkalai", Value: dashIfEmpty(item.JenisPropertiCode)},
			{Sandi: "IV", Nama: "Alamat Properti Terbengkalai", Value: dashIfEmpty(item.AlamatProperti)},
			{Sandi: "V", Nama: "Koordinat", Value: dashIfEmpty(item.Koordinat)},
			{Sandi: "VI", Nama: "Tanggal Penetapan", Value: form17_00Tanggal(item.TanggalPenetapan)},
			{Sandi: "VII", Nama: "Biaya Perolehan atau Nilai Wajar", Value: FormatRupiah(item.BiayaPerolehanNilaiWajar)},
			{Sandi: "VIII", Nama: "Akumulasi Penyusutan atau Amortisasi", Value: FormatRupiah(item.AkumulasiPenyusutanAmortisasi)},
			{Sandi: "IX", Nama: "Jumlah", Value: propertiJumlahCell(jumlah, ok)},
			{Sandi: "X", Nama: "Metode Pengukuran", Value: dashIfEmpty(item.MetodePengukuranCode)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// propertiJumlahCell menulis kolom IX: angka jumlah bila dapat dihitung, "-" bila tidak
// (hasil negatif). Baris yang "-" sudah membawa alasannya lewat TableRow.Reason.
func propertiJumlahCell(jumlah decimal.Decimal, ok bool) string {
	if !ok {
		return "-"
	}
	return FormatRupiah(jumlah)
}

// form17_00RowKey mengidentifikasi satu baris Form 17.00 secara terbaca: No. Register.
func form17_00RowKey(item domain.PropertiItem) string {
	return item.NoRegister
}

// form17_00Tanggal menulis Tanggal Penetapan dalam format TT-BB-TTTT, mengikuti
// mayoritas form (transkrip: "tanggal (TT-BB-TTTT dsb.)"). Form 17.00 tidak
// menyimpang seperti Form 18.00 yang memakai TT-MM-TTTT.
func form17_00Tanggal(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006")
}
