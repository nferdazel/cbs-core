package ojkreport

import (
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// form00_11.go membangun Form 00.11 "DATA KANTOR SELAIN KANTOR PUSAT DAN KANTOR CABANG
// DAN TERMINAL PERBANKAN ELEKTRONIK (TPE)" dari data bank_offices (migrasi 000112,
// kolom Form 00.11 ditambahkan migrasi 000126).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.11 "Data Kantor selain Kantor Pusat dan Kantor Cabang dan TPE":
//     PDF #page 270 (hlm. tercetak 218) susunan kolom, sandi PDF #page 271-272
//     (hlm. 219-220), penjelasan PDF #page 273-275 (hlm. 221-223). Empat belas kolom:
//     I Jenis, II Kode Kantor, III Sandi Kantor Induk, IV Sandi Kantor Sebelumnya,
//     V Nama Kantor, VI Koordinat, VII Alamat, VIII Nama Pimpinan, IX No. Telepon,
//     X Keterangan Data Kantor dan TPE, XI Tanggal Pelaksanaan, XII Sandi Kantor Kendali,
//     XIII Tanggal Persetujuan, XIV Lokasi. TIDAK ada baris JUMLAH.
//   - Sandi I Jenis (PDF #271): 02 Kantor Kas, 03 Kas Keliling, 04 Titik Pembayaran,
//     05 ATM, 06 EDC, 07 Kantor Wilayah, 08 Sentra Keuangan Khusus, 99 Lainnya.
//   - Sandi X Keterangan (PDF #272): 1-7.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - Baris 00.11 dipilih dari kantor yang bank TANDAI dengan sandi Jenis (kolom I,
//     `ojk_office_kind_code`). Kantor pusat/cabang TIDAK punya sandi ini dan karena itu
//     tidak masuk; form ini memang "selain kantor pusat dan kantor cabang". Baris tidak
//     dipilih dari teks bebas `office_type` — menebak dari teks bebas akan mengarang
//     penggolongan.
//   - Seluruh kolom adalah isian bank; laporan tidak menurunkan nilai apa pun.
//   - Kolom II Kode Kantor dikosongkan untuk TPE (sandi I 03-06) menurut PDF #273; laporan
//     menulis "-" apa adanya bila bank belum mengisinya, tanpa mengosongkan paksa.
//   - Kolom XII Sandi Kantor Kendali hanya bermakna untuk Kantor Wilayah (07) dan Sentra
//     Keuangan Khusus (08); untuk jenis lain nilainya ditulis "-" beserta alasan.

// form00_11SandiJenis adalah sandi baku kolom I Form 00.11 (PDF #271).
const (
	form00_11JenisKantorKas       = "02"
	form00_11JenisKasKeliling     = "03"
	form00_11JenisTitikPembayaran = "04"
	form00_11JenisATM             = "05"
	form00_11JenisEDC             = "06"
	form00_11JenisKantorWilayah   = "07"
	form00_11JenisSentraKeuangan  = "08"
	form00_11JenisLainnya         = "99"
)

// BuildForm00_11 menyusun tabel Form 00.11 dari kantor yang bank tandai dengan sandi Jenis.
// Fungsi ini murni. Bila tidak ada kantor tersaring, Rows kosong (builder mencatatnya pada
// SkippedForms). Form ini tidak punya baris JUMLAH.
func BuildForm00_11(offices []domain.BankOffice) TableSection {
	sec := TableSection{
		Form:     "00.11",
		Name:     formName("00.11"),
		KeyLabel: "Kode Kantor",
		Columns: []TableColumn{
			{Sandi: "I", Nama: "Jenis"},
			{Sandi: "II", Nama: "Kode Kantor"},
			{Sandi: "III", Nama: "Sandi Kantor Induk"},
			{Sandi: "IV", Nama: "Sandi Kantor Sebelumnya"},
			{Sandi: "V", Nama: "Nama Kantor"},
			{Sandi: "VI", Nama: "Koordinat"},
			{Sandi: "VII", Nama: "Alamat"},
			{Sandi: "VIII", Nama: "Nama Pimpinan"},
			{Sandi: "IX", Nama: "No. Telepon"},
			{Sandi: "X", Nama: "Keterangan Data Kantor dan TPE"},
			{Sandi: "XI", Nama: "Tanggal Pelaksanaan"},
			{Sandi: "XII", Nama: "Sandi Kantor Kendali"},
			{Sandi: "XIII", Nama: "Tanggal Persetujuan"},
			{Sandi: "XIV", Nama: "Lokasi"},
		},
		Notes: []string{
			"Baris per jaringan kantor SELAIN kantor pusat/cabang dan TPE dari bank_offices yang bank tandai sandi Jenis (Form 00.11, PDF #page 270-275); TIDAK ada baris JUMLAH.",
			"Hanya kantor yang bank beri sandi Jenis (kolom I) yang muncul; kantor pusat/cabang tidak diberi sandi ini. Penggolongan tidak ditebak dari teks bebas office_type.",
			"Jenis (I) memakai sandi baku PDF #271: 02 Kantor Kas, 03 Kas Keliling, 04 Titik Pembayaran, 05 ATM, 06 EDC, 07 Kantor Wilayah, 08 Sentra Keuangan Khusus, 99 Lainnya.",
			"Kode Kantor (II) hanya untuk kantor kas/kantor wilayah/SKK; untuk TPE (sandi 03-06) kolom ini dikosongkan menurut PDF #273. Laporan menulis apa adanya yang bank isi.",
			"Keterangan Data Kantor dan TPE (X) memakai sandi baku PDF #272: 1 pembukaan, 2 pemindahan (induk), 3 pemindahan, 4 tidak berubah, 5-7 perubahan status.",
			"Sandi Kantor Kendali (XII) hanya bermakna untuk Kantor Wilayah (07) dan Sentra Keuangan Khusus (08); untuk jenis lain ditulis \"-\" beserta alasannya karena form menyatakan kolom ini hanya diisi untuk kedua jenis itu.",
			"Lokasi (XIV) memakai sandi Kabupaten/Kota Lampiran 03 (tabel ojk_kabupaten, migrasi 000102); laporan menulis sandi apa adanya.",
			"Seluruh nilai adalah isian bank; laporan tidak menurunkan angka apa pun.",
		},
	}

	for _, o := range offices {
		sandiJenis := strings.TrimSpace(o.OJKOfficeKindCode)
		if sandiJenis == "" {
			// Belum ditandai bank; bukan bagian Form 00.11.
			continue
		}
		row := TableRow{Key: form00_11RowKey(o)}
		row.Cells = []TableCell{
			{Sandi: "I", Nama: "Jenis", Value: form00_11JenisLabel(sandiJenis)},
			{Sandi: "II", Nama: "Kode Kantor", Value: dashIfEmpty(o.Code)},
			{Sandi: "III", Nama: "Sandi Kantor Induk", Value: dashIfEmpty(o.ParentOfficeCode)},
			{Sandi: "IV", Nama: "Sandi Kantor Sebelumnya", Value: dashIfEmpty(o.PreviousOfficeCode)},
			{Sandi: "V", Nama: "Nama Kantor", Value: dashIfEmpty(o.Name)},
			{Sandi: "VI", Nama: "Koordinat", Value: dashIfEmpty(o.Coordinates)},
			{Sandi: "VII", Nama: "Alamat", Value: dashIfEmpty(o.Address)},
			{Sandi: "VIII", Nama: "Nama Pimpinan", Value: dashIfEmpty(o.HeadName)},
			{Sandi: "IX", Nama: "No. Telepon", Value: dashIfEmpty(o.PhoneNumber)},
			{Sandi: "X", Nama: "Keterangan Data Kantor dan TPE", Value: form00_11KeteranganLabel(o.OJKChangeCode)},
			{Sandi: "XI", Nama: "Tanggal Pelaksanaan", Value: form00_11Tanggal(o.ImplementationDate)},
			{Sandi: "XII", Nama: "Sandi Kantor Kendali", Value: form00_11KendaliCell(o, sandiJenis)},
			{Sandi: "XIII", Nama: "Tanggal Persetujuan", Value: form00_11Tanggal(o.OJKApprovalDate)},
			{Sandi: "XIV", Nama: "Lokasi", Value: dashIfEmpty(o.OJKKabupatenCode)},
		}
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// form00_11RowKey mengidentifikasi satu baris: kode kantor; bila kosong (TPE), dipakai
// nama kantor lalu id agar tidak bertabrakan.
func form00_11RowKey(o domain.BankOffice) string {
	if strings.TrimSpace(o.Code) != "" {
		return o.Code
	}
	if strings.TrimSpace(o.Name) != "" {
		return o.Name
	}
	return "kantor:" + o.ID.String()
}

// form00_11JenisLabel menulis sandi I sebagai label baku (PDF #271).
func form00_11JenisLabel(code string) string {
	switch code {
	case form00_11JenisKantorKas:
		return "02 - Kantor Kas"
	case form00_11JenisKasKeliling:
		return "03 - Kas Keliling"
	case form00_11JenisTitikPembayaran:
		return "04 - Titik Pembayaran"
	case form00_11JenisATM:
		return "05 - ATM"
	case form00_11JenisEDC:
		return "06 - EDC"
	case form00_11JenisKantorWilayah:
		return "07 - Kantor Wilayah"
	case form00_11JenisSentraKeuangan:
		return "08 - Sentra Keuangan Khusus"
	case form00_11JenisLainnya:
		return "99 - Lainnya"
	default:
		return dashIfEmpty(code)
	}
}

// form00_11KeteranganLabel menulis sandi X sebagai label baku (PDF #272).
func form00_11KeteranganLabel(code string) string {
	switch code {
	case "1":
		return "1 - Pembukaan/penggunaan/penambahan"
	case "2":
		return "2 - Pemindahan alamat (induk)"
	case "3":
		return "3 - Pemindahan alamat"
	case "4":
		return "4 - Tidak berubah"
	case "5":
		return "5 - Kantor kas dari penurunan status SKK"
	case "6":
		return "6 - SKK dari peningkatan kantor kas"
	case "7":
		return "7 - SKK dari penurunan kantor cabang"
	default:
		return dashIfEmpty(code)
	}
}

// form00_11KendaliCell menulis kolom XII. PDF #274 menyatakan kolom ini HANYA diisi untuk
// Kantor Wilayah (07) dan Sentra Keuangan Khusus (08); jenis lain ditulis "-" apa adanya.
func form00_11KendaliCell(o domain.BankOffice, sandiJenis string) string {
	if sandiJenis != form00_11JenisKantorWilayah && sandiJenis != form00_11JenisSentraKeuangan {
		return "-"
	}
	return dashIfEmpty(o.ControlOfficeCode)
}

// form00_11Tanggal menulis tanggal Form 00.11 (XI, XIII); nil berarti belum diisi.
func form00_11Tanggal(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02")
}
