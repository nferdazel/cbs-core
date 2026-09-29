package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form18_00.go membangun Form 18.00 "DAFTAR ASET KEUANGAN LAINNYA" dari register
// aset_keuangan_lainnya_register (migrasi 000121).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 18.00 "DAFTAR ASET KEUANGAN LAINNYA": PDF #page 233 (hlm. tercetak 181),
//     lanjutan PDF #page 234 (hlm. 182). Lima belas kolom: I Sandi Kantor,
//     II No. Rekening, III ID Pihak Lawan, IV Jenis, V Tanggal Mulai,
//     VI Tanggal Jatuh Tempo, VII Suku Bunga, VIII Nominal,
//     IX Nilai Agunan yang Dapat Diperhitungkan, X Cadangan Kerugian Penurunan Nilai,
//     XI CKPN Aset Baik, XII CKPN Aset Kurang Baik, XIII CKPN Aset Tidak Baik,
//     XIV Klasifikasi Aset Keuangan, XV Jenis CKPN. Satu baris = satu rekening;
//     TIDAK ADA baris JUMLAH (PDF #page 233-234).
//   - Sandi: PDF #page 235 (hlm. 183). IV 10 Tagihan fraud / 99 Tagihan lainnya;
//     XIV 1 VW laba rugi / 2 VW PKL / 3 biaya perolehan diamortisasi; XV 1 Individual /
//     2 Kolektif.
//   - Penjelasan: PDF #page 237-238 (hlm. 185-186). II nomor rekening unik, "tidak boleh
//     sama". IX nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA.
//     Format tanggal form ini TT-MM-TTTT (PDF #page 235), bukan TT-BB-TTTT.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH angka VII-XIII dan sandi adalah isian bank dari register. Tidak ada kolom
//     turunan: form tidak memberikan rumus yang mengikat X = XI+XII+XIII, sehingga
//     laporan menuliskan angka bank apa adanya.
//   - Form ini TIDAK punya baris JUMLAH (PDF #page 233-234): tabel berakhir setelah baris
//     rekening terakhir.
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice; register sendiri bank-wide, tidak per kantor.
//   - Nomor rekening unik dan tidak boleh sama; baris NONAKTIF tidak dibaca
//     (ListAsetKeuanganForOJK menyaring status AKTIF), sehingga tidak muncul di form.

// AsetKeuanganRegisterSource menyediakan baris register aset keuangan lainnya bank-wide
// untuk Form 18.00. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini
// Form 18.00 dinyatakan belum tersedia, bukan ditulis kosong.
type AsetKeuanganRegisterSource interface {
	ListAsetKeuanganForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.AsetKeuanganItem, error)
}

// BuildForm18_00 menyusun tabel Form 18.00 dari baris register aset keuangan lainnya dan
// kantor pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila
// tidak ada baris, Rows kosong tetapi daftar kolom tetap dibawa. TIDAK ada baris JUMLAH.
func BuildForm18_00(rows []domain.AsetKeuanganItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "18.00",
		Name:     formName("18.00"),
		KeyLabel: "No. Rekening",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "No. Rekening"},
			{Sandi: "III", Nama: "ID Pihak Lawan"},
			{Sandi: "IV", Nama: "Jenis"},
			{Sandi: "V", Nama: "Tanggal Mulai"},
			{Sandi: "VI", Nama: "Tanggal Jatuh Tempo"},
			{Sandi: "VII", Nama: "Suku Bunga"},
			{Sandi: "VIII", Nama: "Nominal"},
			{Sandi: "IX", Nama: "Nilai Agunan yang Dapat Diperhitungkan"},
			{Sandi: "X", Nama: "Cadangan Kerugian Penurunan Nilai"},
			{Sandi: "XI", Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik"},
			{Sandi: "XII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik"},
			{Sandi: "XIII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik"},
			{Sandi: "XIV", Nama: "Klasifikasi Aset Keuangan"},
			{Sandi: "XV", Nama: "Jenis CKPN"},
		},
		Notes: []string{
			"Baris per rekening aset keuangan lainnya dari register aset_keuangan_lainnya_register yang bank isi (Form 18.00, PDF #page 233-234); satu baris = satu rekening unik dan TIDAK ada baris JUMLAH.",
			"Jenis (IV) mengikuti sandi PDF #page 235: 10 Tagihan fraud, 99 Tagihan lainnya. Klasifikasi Aset Keuangan (XIV): 1 Nilai wajar melalui laba rugi, 2 Nilai wajar melalui penghasilan komprehensif lain, 3 Biaya perolehan diamortisasi. Jenis CKPN (XV): 1 Individual, 2 Kolektif.",
			"Seluruh nilai (VII-XIII) adalah isian bank. Form tidak memberikan rumus yang mengikat X = XI+XII+XIII, sehingga laporan tidak menghitung atau menurunkan angka apa pun.",
			"Nilai Agunan yang Dapat Diperhitungkan (IX) menurut PDF #page 237 adalah nilai agunan yang dapat diperhitungkan sebagai pengurang PPKA; diisi bank.",
			"Tanggal (V, VI) ditulis form ini dengan format TT-MM-TTTT mengikuti PDF #page 235, bukan TT-BB-TTTT seperti mayoritas form.",
			"ID Pihak Lawan (III) BUKAN sandi Lampiran 02: tabel sandi Form 18.00 tidak memuat sandi untuk kolom III, sehingga disimpan dan ditulis sebagai teks apa adanya tanpa daftar karangan.",
			"No. Rekening (II) unik dan tidak boleh sama (PDF #page 237). Penghapusan register adalah soft-delete NONAKTIF; baris NONAKTIF tidak ikut form ini karena sumbernya menyaring status AKTIF.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form18_00RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "No. Rekening", Value: dashIfEmpty(item.NoRekening)},
			{Sandi: "III", Nama: "ID Pihak Lawan", Value: dashIfEmpty(item.CounterpartyID)},
			{Sandi: "IV", Nama: "Jenis", Value: dashIfEmpty(item.JenisCode)},
			{Sandi: "V", Nama: "Tanggal Mulai", Value: form18_00Tanggal(item.TanggalMulai)},
			{Sandi: "VI", Nama: "Tanggal Jatuh Tempo", Value: form18_00Tanggal(item.TanggalJatuhTempo)},
			{Sandi: "VII", Nama: "Suku Bunga", Value: FormatPersen(item.SukuBunga)},
			{Sandi: "VIII", Nama: "Nominal", Value: FormatRupiah(item.Nominal)},
			{Sandi: "IX", Nama: "Nilai Agunan yang Dapat Diperhitungkan", Value: FormatRupiah(item.NilaiAgunanDiperhitungkan)},
			{Sandi: "X", Nama: "Cadangan Kerugian Penurunan Nilai", Value: FormatRupiah(item.CKPN)},
			{Sandi: "XI", Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik", Value: FormatRupiah(item.CKPNAsetBaik)},
			{Sandi: "XII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik", Value: FormatRupiah(item.CKPNAsetKurangBaik)},
			{Sandi: "XIII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik", Value: FormatRupiah(item.CKPNAsetTidakBaik)},
			{Sandi: "XIV", Nama: "Klasifikasi Aset Keuangan", Value: dashIfEmpty(item.KlasifikasiAsetKeuanganCode)},
			{Sandi: "XV", Nama: "Jenis CKPN", Value: dashIfEmpty(item.JenisCKPNCode)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form18_00RowKey mengidentifikasi satu baris Form 18.00 secara terbaca: No. Rekening.
func form18_00RowKey(item domain.AsetKeuanganItem) string {
	return item.NoRekening
}

// form18_00Tanggal menulis tanggal Form 18.00 dalam format TT-MM-TTTT mengikuti
// PDF #page 235 (bukan TT-BB-TTTT seperti mayoritas form).
func form18_00Tanggal(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006")
}
