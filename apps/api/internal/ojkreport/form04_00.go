package ojkreport

import (
	"context"
	"strconv"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form04_00.go membangun Form 04.00 "DAFTAR SURAT BERHARGA" dari register
// surat_berharga_register (migrasi 000122).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 04.00 "DAFTAR SURAT BERHARGA": PDF #page 129 (hlm. tercetak 77) kolom I-VI,
//     PDF #page 130 (hlm. 78) kolom VII-XV, PDF #page 131 (hlm. 79) kolom XVI-XXV. Dua
//     puluh lima kolom: I Sandi Kantor, II Klasifikasi, III Suku Bunga,
//     IV Jangka Waktu, V Nominal, VI Nominal yang Dijaminkan, VII Biaya Perolehan,
//     VIII Diskonto/Premium Belum Diamortisasi, IX Biaya Transaksi Belum Diamortisasi,
//     X Laba/Rugi Belum Direalisasi, XI Biaya Perolehan Diamortisasi/Nilai Wajar,
//     XII Nomor Surat Berharga, XIII ID Pihak Lawan, XIV Jenis, XV Kualitas,
//     XVI CKPN, XVII Lembaga Pemeringkat, XVIII Peringkat Surat Berharga,
//     XIX Tanggal Pemeringkatan, XX Tanggal Penerbitan, XXI-XXIII CKPN baik/kurang
//     baik/tidak baik, XXIV Klasifikasi Aset Keuangan, XXV Jenis CKPN.
//     Satu baris = satu surat berharga; ADA baris JUMLAH (PDF #page 129).
//   - Sandi: PDF #page 132-133. II 1 Tersedia untuk dijual / 2 Dimiliki hingga jatuh
//     tempo; XIV 1 BI / 2 Pemerintah / 3 Pemerintah Daerah; XV 1 Lancar / 3 Kurang
//     Lancar / 5 Macet; XXIV 1-3; XXV 1 Individual / 2 Kolektif.
//   - Penjelasan: PDF #page 134-136. XI: nilai nominal - diskonto belum diamortisasi +
//     premium belum diamortisasi + biaya transaksi belum diamortisasi, ATAU nilai wajar.
//     XXIV hanya diisi bila BPR melakukan penawaran umum efek di pasar modal. XVII sandi 9
//     dan XVIII sandi 99 untuk keadaan tanpa peringkat (PDF #page 135).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH nilai dan sandi adalah isian bank dari register. Tidak ada kolom turunan:
//     form memberi DUA kemungkinan untuk kolom XI (amortized cost atau nilai wajar) tanpa
//     rumus pengikat tunggal, sehingga laporan menuliskan angka bank apa adanya.
//   - Baris JUMLAH menjumlahkan kolom angka (III, V-XI, XVI, XXI-XXIII) bila ada baris;
//     kolom sandi/tanggal/teks pada baris JUMLAH ditulis "-".
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice; register sendiri bank-wide, tidak per kantor.
//   - Form 04.00 tidak menetapkan nomor register unik; baris NONAKTIF tetap disaring
//     (ListSuratBerhargaForOJK menyaring status AKTIF), sehingga tidak muncul di form.

// SuratBerhargaRegisterSource menyediakan baris register surat berharga bank-wide untuk
// Form 04.00. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 04.00
// dinyatakan belum tersedia, bukan ditulis kosong.
type SuratBerhargaRegisterSource interface {
	ListSuratBerhargaForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.SuratBerhargaItem, error)
}

// form04_00TotalKey adalah kunci baris kaki tabel JUMLAH; bukan sandi pos OJK.
const form04_00TotalKey = "JUMLAH"

// BuildForm04_00 menyusun tabel Form 04.00 dari baris register surat berharga dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Bila tidak ada
// baris, Rows hanya berisi baris JUMLAH berisi "-".
func BuildForm04_00(rows []domain.SuratBerhargaItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "04.00",
		Name:     formName("04.00"),
		KeyLabel: "Nomor Surat Berharga",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Klasifikasi"},
			{Sandi: "III", Nama: "Suku Bunga"},
			{Sandi: "IV", Nama: "Jangka Waktu"},
			{Sandi: "V", Nama: "Nominal"},
			{Sandi: "VI", Nama: "Nominal yang Dijaminkan"},
			{Sandi: "VII", Nama: "Biaya Perolehan"},
			{Sandi: "VIII", Nama: "Diskonto/Premium Belum Diamortisasi"},
			{Sandi: "IX", Nama: "Biaya Transaksi Belum Diamortisasi"},
			{Sandi: "X", Nama: "Laba/Rugi Belum Direalisasi"},
			{Sandi: "XI", Nama: "Biaya Perolehan Diamortisasi/Nilai Wajar"},
			{Sandi: "XII", Nama: "Nomor Surat Berharga"},
			{Sandi: "XIII", Nama: "ID Pihak Lawan"},
			{Sandi: "XIV", Nama: "Jenis"},
			{Sandi: "XV", Nama: "Kualitas"},
			{Sandi: "XVI", Nama: "Cadangan Kerugian Penurunan Nilai"},
			{Sandi: "XVII", Nama: "Lembaga Pemeringkat"},
			{Sandi: "XVIII", Nama: "Peringkat Surat Berharga"},
			{Sandi: "XIX", Nama: "Tanggal Pemeringkatan"},
			{Sandi: "XX", Nama: "Tanggal Penerbitan"},
			{Sandi: "XXI", Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik"},
			{Sandi: "XXII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik"},
			{Sandi: "XXIII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik"},
			{Sandi: "XXIV", Nama: "Klasifikasi Aset Keuangan"},
			{Sandi: "XXV", Nama: "Jenis CKPN"},
		},
		Notes: []string{
			"Baris per surat berharga dari register surat_berharga_register yang bank isi (Form 04.00, PDF #page 129-131); satu baris = satu surat berharga yang diterbitkan Bank Indonesia, Pemerintah, atau Pemerintah Daerah, dengan baris JUMLAH.",
			"Klasifikasi (II) mengikuti sandi PDF #page 132: 1 Tersedia untuk dijual, 2 Dimiliki hingga jatuh tempo. Jenis (XIV): 1 diterbitkan Bank Indonesia, 2 Pemerintah, 3 Pemerintah Daerah. Kualitas (XV): 1 Lancar, 3 Kurang Lancar, 5 Macet. Klasifikasi Aset Keuangan (XXIV): 1 VW laba rugi, 2 VW PKL, 3 biaya perolehan diamortisasi. Jenis CKPN (XXV): 1 Individual, 2 Kolektif.",
			"Seluruh nilai (III, V-XI, XVI, XXI-XXIII) adalah isian bank. Kolom XI menurut PDF #page 135 adalah nilai nominal - diskonto belum diamortisasi + premium belum diamortisasi + biaya transaksi belum diamortisasi, ATAU nilai wajar; form memberi dua kemungkinan tanpa aturan pengikat tunggal, sehingga laporan tidak menghitung kolom ini.",
			"Klasifikasi Aset Keuangan (XXIV) hanya diisi apabila BPR melakukan penawaran umum efek di pasar modal (PDF #page 135); karenanya boleh kosong.",
			"Lembaga Pemeringkat (XVII) memakai sandi Lampiran 08 dengan sentinel 9 (tanpa peringkat / lembaga tidak diakui OJK); Peringkat Surat Berharga (XVIII) memakai sandi Lampiran 09 dengan sentinel 99 (PDF #page 135). Laporan menuliskan sandi bank apa adanya; daftar lengkap lampiran tidak disalin ke kode.",
			"Nomor Surat Berharga (XII) adalah ISIN dalam teks apa adanya; form tidak menyediakan tabel sandi untuk kolom ini.",
			"Baris JUMLAH menjumlahkan kolom angka (III, V-XI, XVI, XXI-XXIII); kolom sandi, tanggal, dan teks pada baris JUMLAH ditulis \"-\".",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	total := make([]decimal.Decimal, len(sec.Columns))

	for i, item := range rows {
		row := TableRow{Key: form04_00RowKey(item, i)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "Klasifikasi", Value: dashIfEmpty(item.KlasifikasiCode)},
			{Sandi: "III", Nama: "Suku Bunga", Value: FormatPersen(item.SukuBunga)},
			{Sandi: "IV", Nama: "Jangka Waktu", Value: form04_00JangkaWaktu(item)},
			{Sandi: "V", Nama: "Nominal", Value: FormatRupiah(item.Nominal)},
			{Sandi: "VI", Nama: "Nominal yang Dijaminkan", Value: FormatRupiah(item.NominalDijaminkan)},
			{Sandi: "VII", Nama: "Biaya Perolehan", Value: FormatRupiah(item.BiayaPerolehan)},
			{Sandi: "VIII", Nama: "Diskonto/Premium Belum Diamortisasi", Value: FormatRupiah(item.DiskontoPremiumBelumDiamortisasi)},
			{Sandi: "IX", Nama: "Biaya Transaksi Belum Diamortisasi", Value: FormatRupiah(item.BiayaTransaksiBelumDiamortisasi)},
			{Sandi: "X", Nama: "Laba/Rugi Belum Direalisasi", Value: FormatRupiah(item.LabaRugiBelumDirealisasi)},
			{Sandi: "XI", Nama: "Biaya Perolehan Diamortisasi/Nilai Wajar", Value: FormatRupiah(item.BiayaPerolehanDiamortisasi)},
			{Sandi: "XII", Nama: "Nomor Surat Berharga", Value: dashIfEmpty(item.NomorSuratBerharga)},
			{Sandi: "XIII", Nama: "ID Pihak Lawan", Value: dashIfEmpty(item.CounterpartyID)},
			{Sandi: "XIV", Nama: "Jenis", Value: dashIfEmpty(item.JenisCode)},
			{Sandi: "XV", Nama: "Kualitas", Value: dashIfEmpty(item.KualitasCode)},
			{Sandi: "XVI", Nama: "Cadangan Kerugian Penurunan Nilai", Value: FormatRupiah(item.CKPN)},
			{Sandi: "XVII", Nama: "Lembaga Pemeringkat", Value: dashIfEmpty(item.LembagaPemeringkatCode)},
			{Sandi: "XVIII", Nama: "Peringkat Surat Berharga", Value: dashIfEmpty(item.PeringkatSuratBerhargaCode)},
			{Sandi: "XIX", Nama: "Tanggal Pemeringkatan", Value: form04_00OptionalTanggal(item.TanggalPemeringkatan)},
			{Sandi: "XX", Nama: "Tanggal Penerbitan", Value: form04_00Tanggal(item.TanggalPenerbitan)},
			{Sandi: "XXI", Nama: "Cadangan Kerugian Penurunan Nilai Aset Baik", Value: FormatRupiah(item.CKPNAsetBaik)},
			{Sandi: "XXII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Kurang Baik", Value: FormatRupiah(item.CKPNAsetKurangBaik)},
			{Sandi: "XXIII", Nama: "Cadangan Kerugian Penurunan Nilai Aset Tidak Baik", Value: FormatRupiah(item.CKPNAsetTidakBaik)},
			{Sandi: "XXIV", Nama: "Klasifikasi Aset Keuangan", Value: dashIfEmpty(item.KlasifikasiAsetKeuanganCode)},
			{Sandi: "XXV", Nama: "Jenis CKPN", Value: dashIfEmpty(item.JenisCKPNCode)},
		}
		sec.Rows = append(sec.Rows, row)

		// Kolom angka dijumlahkan (indeks pada sec.Columns: II=0 ... XXV=23):
		// III=1, V=3, VI=4, VII=5, VIII=6, IX=7, X=8, XI=9, XVI=14, XXI=19,
		// XXII=20, XXIII=21.
		total[1] = total[1].Add(item.SukuBunga)
		total[3] = total[3].Add(item.Nominal)
		total[4] = total[4].Add(item.NominalDijaminkan)
		total[5] = total[5].Add(item.BiayaPerolehan)
		total[6] = total[6].Add(item.DiskontoPremiumBelumDiamortisasi)
		total[7] = total[7].Add(item.BiayaTransaksiBelumDiamortisasi)
		total[8] = total[8].Add(item.LabaRugiBelumDirealisasi)
		total[9] = total[9].Add(item.BiayaPerolehanDiamortisasi)
		total[14] = total[14].Add(item.CKPN)
		total[19] = total[19].Add(item.CKPNAsetBaik)
		total[20] = total[20].Add(item.CKPNAsetKurangBaik)
		total[21] = total[21].Add(item.CKPNAsetTidakBaik)
	}

	totalRow := TableRow{Key: form04_00TotalKey}
	for i, col := range sec.Columns {
		value := "-"
		if len(rows) > 0 && form04_00KolomAngka(i) {
			if i == 1 {
				// Kolom III Suku Bunga adalah persen, bukan rupiah.
				value = FormatPersen(total[i])
			} else {
				value = FormatRupiah(total[i])
			}
		}
		totalRow.Cells = append(totalRow.Cells, TableCell{Sandi: col.Sandi, Nama: col.Nama, Value: value})
	}
	totalRow.Cells[0].Value = "JUMLAH"
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form04_00KolomAngka menandai indeks kolom dalam sec.Columns yang dijumlahkan pada baris
// JUMLAH (kolom angka: II tidak, III=1, V=3, VI=4, VII=5, VIII=6, IX=7, X=8, XI=9,
// XVI=14, XXI=19, XXII=20, XXIII=21). Kolom sandi/tanggal/teks tidak dijumlahkan.
func form04_00KolomAngka(i int) bool {
	switch i {
	case 1, 3, 4, 5, 6, 7, 8, 9, 14, 19, 20, 21:
		return true
	default:
		return false
	}
}

// form04_00RowKey mengidentifikasi satu baris Form 04.00 secara terbaca: Nomor Surat
// Berharga (ISIN); bila kosong, pakai urutan agar kunci tetap unik.
func form04_00RowKey(item domain.SuratBerhargaItem, index int) string {
	if key := item.NomorSuratBerharga; key != "" {
		return key
	}
	return "SB-" + strconv.Itoa(index+1)
}

// form04_00JangkaWaktu menggabungkan Tanggal Mulai dan Tanggal Jatuh Tempo (kolom IV).
func form04_00JangkaWaktu(item domain.SuratBerhargaItem) string {
	awal := form04_00Tanggal(item.TanggalMulai)
	akhir := form04_00Tanggal(item.TanggalJatuhTempo)
	if awal == "-" && akhir == "-" {
		return "-"
	}
	return awal + " s.d. " + akhir
}

// form04_00Tanggal menulis tanggal dalam format TT-BB-TTTT mengikuti mayoritas form.
func form04_00Tanggal(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006")
}

// form04_00OptionalTanggal menulis tanggal yang boleh kosong (tanggal pemeringkatan).
func form04_00OptionalTanggal(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("02-01-2006")
}
