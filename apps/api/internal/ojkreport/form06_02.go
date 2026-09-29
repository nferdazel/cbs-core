package ojkreport

import (
	"context"
	"strconv"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// SindikasiRegisterSource menyediakan baris register kredit sindikasi bank-wide untuk
// Form 06.02. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 06.02
// dinyatakan belum tersedia, bukan ditulis kosong.
type SindikasiRegisterSource interface {
	ListSindikasiForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.SindikasiItem, error)
}

// form06_02.go membangun Form 06.02 "DAFTAR KREDIT SINDIKASI" dari register
// kredit_sindikasi_register (migrasi 000124).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 06.02 "DAFTAR KREDIT SINDIKASI": PDF #page 173 (hlm. tercetak 121) kolom I-VII,
//     PDF #page 174 (hlm. 122) kolom VIII-XIII. Tiga belas kolom: I Sandi Kantor,
//     II ID Pihak Lawan, III No. Identitas, IV No. Rekening,
//     V Jumlah Pendanaan Sindikasi, VI Bagian Pendanaan,
//     VII Sandi Bank Peserta Sindikasi (Plafon + Baki Debet),
//     VIII Status Kepesertaan, IX Nomor Perjanjian Kredit Sindikasi,
//     X Pendanaan di Bank Pelapor, XI Kualitas,
//     XII Nominal Tunggakan (Pokok + Bunga), XIII Hari Tunggakan (Pokok + Bunga).
//   - Sandi: PDF #page 175-176 (hlm. 123-124). VIII 1 Arranger / 2 Anggota sindikasi;
//     X 1 Ya / 2 Tidak; XI 1 Lancar / 2 Dalam Perhatian Khusus / 3 Kurang Lancar /
//     4 Diragukan / 5 Macet.
//   - Penjelasan: PDF #page 177-178 (hlm. 125-126). Satu baris = satu rekening fasilitas
//     kredit sindikasi. TIDAK ADA baris JUMLAH.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH kolom angka adalah ISIAN BANK; form tidak memberi rumus pengikat, sehingga
//     laporan TIDAK menurunkan nilai apa pun dan tidak menjumlahkan apa pun.
//   - Kolom III No. Identitas (NIK/NPWP debitur) SENGAJA TIDAK DISIMPAN mengikuti keputusan
//     privasi (docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6, pola Form 00.01 dan form berhenti).
//     Kolom selalu ditulis "-" beserta alasannya, bukan dikarang.
//   - Kolom IV No. Rekening kosong bila kolom X diisi sandi 2 (tidak); ini sah menurut
//     penjelasan PDF #page 177, jadi ditulis "-" TANPA alasan "tidak tersedia".
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice, bukan dari register.
//   - Tanpa baris AKTIF pada bulan periode, form dinyatakan belum tersedia, bukan ditulis
//     kosong seolah tidak ada kredit sindikasi.

// BuildForm06_02 menyusun tabel Form 06.02 dari baris register kredit sindikasi dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Form ini tidak
// memiliki baris JUMLAH; bila tidak ada baris, Rows tetap kosong dan Notes menjelaskan
// keadaan itu.
func BuildForm06_02(rows []domain.SindikasiItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "06.02",
		Name:     formName("06.02"),
		KeyLabel: "No. Rekening",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "ID Pihak Lawan"},
			{Sandi: "III", Nama: "No. Identitas"},
			{Sandi: "IV", Nama: "No. Rekening"},
			{Sandi: "V", Nama: "Jumlah Pendanaan Sindikasi"},
			{Sandi: "VI", Nama: "Bagian Pendanaan"},
			{Sandi: "VII", Nama: "Sandi Bank Peserta Sindikasi"},
			{Sandi: "VII", Nama: "Plafon"},
			{Sandi: "VII", Nama: "Baki Debet"},
			{Sandi: "VIII", Nama: "Status Kepesertaan"},
			{Sandi: "IX", Nama: "Nomor Perjanjian Kredit Sindikasi"},
			{Sandi: "X", Nama: "Pendanaan di Bank Pelapor"},
			{Sandi: "XI", Nama: "Kualitas"},
			{Sandi: "XII", Nama: "Nominal Tunggakan Pokok"},
			{Sandi: "XII", Nama: "Nominal Tunggakan Bunga"},
			{Sandi: "XIII", Nama: "Hari Tunggakan Pokok"},
			{Sandi: "XIII", Nama: "Hari Tunggakan Bunga"},
		},
		Notes: []string{
			"Baris per rekening fasilitas kredit sindikasi dari register kredit_sindikasi_register yang bank isi (Form 06.02, PDF #page 173-178); satu baris = satu rekening fasilitas kredit, TANPA baris JUMLAH.",
			"Kredit sindikasi adalah kredit yang diberikan bersama-sama dua bank atau lebih (atau perusahaan pembiayaan lain) dengan pembagian dana, risiko, dan pendapatan sesuai porsi kepesertaan (PDF #page 177).",
			"Batas privasi: kolom III No. Identitas (NIK/NPWP debitur) sengaja tidak disimpan dan tidak dipaparkan sebagai keluaran laporan (keputusan privasi docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6, sama seperti Form 00.01 dan form berhenti); kolom ditulis \"-\" dan tidak pernah dikarang.",
			"Kolom IV No. Rekening dikosongkan bila kolom X Pendanaan di Bank Pelapor diisi sandi 2 (tidak), sesuai penjelasan PDF #page 177; kekosongan itu sah dan ditulis \"-\". Bila diisi, nomor rekening unik dan \"tidak boleh sama\" serta harus sama dengan nomor rekening SLIK.",
			"Jumlah Pendanaan Sindikasi (V) adalah jumlah pendanaan SELURUH anggota sindikasi menurut perjanjian kredit, sedangkan Bagian Pendanaan (VI) adalah bagian BPR pelapor (PDF #page 177); keduanya isian bank dan laporan tidak menurunkannya.",
			"Kolom VII Sandi Bank Peserta Sindikasi memakai sandi mengacu Sistem Pelaporan OJK (SPOJK); daftar sandi tidak disalin ke kode, sehingga ditulis apa adanya.",
			"Nomor Perjanjian Kredit Sindikasi (IX) adalah nomor perjanjian INDUK awal tanpa spasi (PDF #page 178).",
			"Hari Tunggakan (XIII) adalah jumlah hari debitur belum membayar angsuran pokok dan/atau bunga sejak tanggal kewajiban sampai tanggal laporan, paling singkat 0 (PDF #page 176/178).",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	for _, item := range rows {
		row := TableRow{Key: form06_02RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "ID Pihak Lawan", Value: dashIfEmpty(item.CounterpartyID)},
			{Sandi: "III", Nama: "No. Identitas", Value: "-"},
			{Sandi: "IV", Nama: "No. Rekening", Value: form06_02NoRekeningCell(item)},
			{Sandi: "V", Nama: "Jumlah Pendanaan Sindikasi", Value: FormatRupiah(item.JumlahPendanaanSindikasi)},
			{Sandi: "VI", Nama: "Bagian Pendanaan", Value: FormatRupiah(item.BagianPendanaan)},
			{Sandi: "VII", Nama: "Sandi Bank Peserta Sindikasi", Value: dashIfEmpty(item.SandiBankPeserta)},
			{Sandi: "VII", Nama: "Plafon", Value: FormatRupiah(item.Plafon)},
			{Sandi: "VII", Nama: "Baki Debet", Value: FormatRupiah(item.BakiDebet)},
			{Sandi: "VIII", Nama: "Status Kepesertaan", Value: form06_02KepesertaanLabel(item.StatusKepesertaanCode)},
			{Sandi: "IX", Nama: "Nomor Perjanjian Kredit Sindikasi", Value: dashIfEmpty(item.NomorPerjanjianInduk)},
			{Sandi: "X", Nama: "Pendanaan di Bank Pelapor", Value: form06_02PendanaanLabel(item.PendanaanDiBankPelaporCode)},
			{Sandi: "XI", Nama: "Kualitas", Value: form06_02KualitasLabel(item.KualitasCode)},
			{Sandi: "XII", Nama: "Nominal Tunggakan Pokok", Value: FormatRupiah(item.TunggakanPokok)},
			{Sandi: "XII", Nama: "Nominal Tunggakan Bunga", Value: FormatRupiah(item.TunggakanBunga)},
			{Sandi: "XIII", Nama: "Hari Tunggakan Pokok", Value: strconv.Itoa(item.HariTunggakanPokok)},
			{Sandi: "XIII", Nama: "Hari Tunggakan Bunga", Value: strconv.Itoa(item.HariTunggakanBunga)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form06_02RowKey mengidentifikasi satu baris Form 06.02 secara terbaca. Bila No. Rekening
// kosong (kolom X = sandi 2), baris dikunci oleh nomor perjanjian induk agar tidak ada dua
// baris berkunci sama yang tak terbedakan.
func form06_02RowKey(item domain.SindikasiItem) string {
	if item.NoRekening != "" {
		return item.NoRekening
	}
	return "perjanjian:" + item.NomorPerjanjianInduk
}

// form06_02NoRekeningCell menulis kolom IV. Kosong SAH bila kolom X = sandi 2 (tidak),
// jadi tetap "-" tanpa alasan "tidak tersedia"; bila kolom X = sandi 1 kosong tidak
// seharusnya terjadi karena ditolak validasi domain, tetapi tetap ditulis "-".
func form06_02NoRekeningCell(item domain.SindikasiItem) string {
	return dashIfEmpty(item.NoRekening)
}

// form06_02KepesertaanLabel menulis sandi VIII sebagai label baku (PDF #page 175).
func form06_02KepesertaanLabel(code string) string {
	switch code {
	case domain.SindikasiKepesertaanArranger:
		return "1 - Arranger"
	case domain.SindikasiKepesertaanAnggota:
		return "2 - Anggota sindikasi"
	default:
		return dashIfEmpty(code)
	}
}

// form06_02PendanaanLabel menulis sandi X sebagai label baku (PDF #page 176).
func form06_02PendanaanLabel(code string) string {
	switch code {
	case domain.SindikasiPendanaanYa:
		return "1 - Ya"
	case domain.SindikasiPendanaanTidak:
		return "2 - Tidak"
	default:
		return dashIfEmpty(code)
	}
}

// form06_02KualitasLabel menulis sandi XI sebagai label baku (PDF #page 175).
func form06_02KualitasLabel(code string) string {
	switch code {
	case domain.SindikasiKualitasLancar:
		return "1 - Lancar"
	case domain.SindikasiKualitasDalamPerhatianKhusus:
		return "2 - Dalam Perhatian Khusus"
	case domain.SindikasiKualitasKurangLancar:
		return "3 - Kurang Lancar"
	case domain.SindikasiKualitasDiragukan:
		return "4 - Diragukan"
	case domain.SindikasiKualitasMacet:
		return "5 - Macet"
	default:
		return dashIfEmpty(code)
	}
}
