package ojkreport

import (
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

// form06.go membangun Form 06.00 DAFTAR KREDIT YANG DIBERIKAN dari baris kredit.
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024, Form 06.00 – 1 (hlm. 94-96) dan
// Form 06.00 – 2 SANDI DAFTAR KREDIT YANG DIBERIKAN (hlm. 100-105). Hanya kolom yang
// sumbernya benar-benar ada di basis data yang diisi; kolom lain didaftarkan pada
// Unavailable beserta alasannya.

// form06Column adalah satu kolom Form 06.00. Reason != "" berarti kolom belum
// tersedia dan Value tidak dipakai.
type form06Column struct {
	Sandi  string
	Nama   string
	Reason string
	Value  func(LoanRow) string
}

const (
	form06SandiKantor        = "I"
	form06SandiIDPihakLawan  = "II"
	form06SandiNoIdentitas   = "III"
	form06SandiKelompok      = "IV"
	form06SandiNoRekening    = "V"
	form06SandiJenis         = "VI"
	form06SandiRestruktur    = "VII"
	form06SandiPenggunaan    = "VIII"
	form06SandiHubungan      = "IX"
	form06SandiSumberDana    = "X"
	form06SandiPeriodeBayar  = "XI"
	form06SandiJangkaWaktu   = "XII"
	form06SandiAngsuranRetni = "XIII"
	form06SandiKualitas      = "XIV"
	form06SandiMulaiMacet    = "XV"
	form06SandiHariTunggakan = "XVI"
	form06SandiNominalTungg  = "XVII"
	form06SandiJenisDebitur  = "XVIII"
	form06SandiSandiBank     = "XIX"
	form06SandiSektor        = "XX"
	form06SandiKategori      = "XXI"
	form06SandiLokasi        = "XXII"
	form06SandiSukuBunga     = "XXIII"
	form06SandiPenjamin      = "XXIV"
	form06SandiAgunanPPKA    = "XXV"
	form06SandiKelonggaran   = "XXVI"
	form06SandiPlafon        = "XXVII"
	form06SandiBakiDebet     = "XXVIII"
	form06SandiProvisi       = "XXIX"
	form06SandiBiayaTrans    = "XXX"
	form06SandiBungaTangguh  = "XXXI"
	form06SandiCadRestru     = "XXXII"
	form06SandiBakiNeto      = "XXXIII"
	form06SandiCKPN          = "XXXIV"
	form06SandiBungaAkanTer  = "XXXV"
	form06SandiBungaProses   = "XXXVI"
	form06SandiBMPK          = "XXXVII"
	form06SandiSifatKredit   = "XXXVIII"
	form06SandiProgram       = "XXXIX"
	form06SandiSektorKUR     = "XL"
	form06SandiAkadAwal      = "XLI"
	form06SandiAkadAkhir     = "XLII"
	form06SandiLPBBTI        = "XLIII"
	form06SandiCKPNBaik      = "XLIV"
	form06SandiCKPNKurang    = "XLV"
	form06SandiCKPNTidak     = "XLVI"
	form06SandiKlasifikasi   = "XLVII"
	form06SandiJenisCKPN     = "XLVIII"
)

// form06Columns adalah susunan kolom Form 06.00. Kolom dengan Reason terisi tidak
// dihitung untuk baris mana pun.
var form06Columns = []form06Column{
	{Sandi: form06SandiKantor, Nama: "Sandi Kantor", Value: func(r LoanRow) string {
		return dashIfEmpty(r.BranchCode)
	}},
	{Sandi: form06SandiIDPihakLawan, Nama: "ID Pihak Lawan", Reason: "sandi pihak lawan mengacu Lampiran 02 yang belum dipetakan; sistem hanya menyimpan UUID nasabah internal"},
	{Sandi: form06SandiNoIdentitas, Nama: "No. Identitas", Reason: "NIK/NPWP nasabah tersimpan terenkripsi untuk dokumen dan tidak dibuka sebagai keluaran laporan"},
	{Sandi: form06SandiKelompok, Nama: "Kode Kelompok Kredit", Reason: "kelompok peminjam pihak tidak terkait butuh daftar sandi OJK yang belum ada rujukannya di repo"},
	{Sandi: form06SandiNoRekening, Nama: "No. Rekening", Value: func(r LoanRow) string {
		return dashIfEmpty(r.LoanNumber)
	}},
	{Sandi: form06SandiJenis, Nama: "Jenis", Reason: "kanal penyaluran kredit (sindikasi/kerja sama/LPBBTI) belum dimodelkan"},
	{Sandi: form06SandiRestruktur, Nama: "Status Restrukturisasi", Value: func(r LoanRow) string {
		return sandiStatusRestrukturisasi(r)
	}},
	{Sandi: form06SandiPenggunaan, Nama: "Jenis Penggunaan", Value: func(r LoanRow) string {
		// Sandi inline Lampiran II Form 06.00-2 (docs/LAMPIRAN-OJK.md): 10/20/31/32/35/39.
		// Nilai diisi bank lewat SQL/seed (loans.ojk_jenis_penggunaan_code); loans.purpose
		// tetap teks bebas dan tidak dipetakan agar tidak menebak sandi.
		return dashIfEmpty(r.OJKJenisPenggunaanCode)
	}},
	{Sandi: form06SandiHubungan, Nama: "Hubungan dengan Bank", Value: func(r LoanRow) string {
		// Sandi inline Lampiran II Form 06.00-2 (docs/LAMPIRAN-OJK.md): 11/12/20. Diisi
		// bank per nasabah lewat SQL/seed (customers.ojk_hubungan_bank_code).
		return dashIfEmpty(r.OJKHubunganBankCode)
	}},
	{Sandi: form06SandiSumberDana, Nama: "Sumber Dana Pelunasan", Reason: "sandi sumber dana pelunasan butuh daftar sandi OJK yang belum ada rujukannya di repo"},
	{Sandi: form06SandiPeriodeBayar, Nama: "Periode Pembayaran Pokok dan Bunga", Value: func(r LoanRow) string {
		// Sandi inline Lampiran II Form 06.00-2 (docs/LAMPIRAN-OJK.md): 1 s.d. 8. Nilai
		// diisi bank lewat SQL/seed (loans.ojk_periode_pembayaran_code); jadwal angsuran
		// internal tidak dipetakan karena sandi periodenya tidak tersurat di repo.
		return dashIfEmpty(r.OJKPeriodePembayaranCode)
	}},
	{Sandi: form06SandiJangkaWaktu, Nama: "Jangka Waktu", Value: func(r LoanRow) string {
		// Hanya jatuh tempo akhir yang tersimpan; tanggal mulai kredit tidak dimuat.
		if r.FinalDueDate == nil {
			return "-"
		}
		return r.FinalDueDate.Format("2006-01-02")
	}},
	{Sandi: form06SandiAngsuranRetni, Nama: "Angsuran Pokok Pertama", Value: func(r LoanRow) string {
		// Sumber: MIN(due_date) jadwal kredit. Tanpa jadwal ditulis "-".
		if r.FirstInstallmentDate == nil {
			return "-"
		}
		return r.FirstInstallmentDate.Format("2006-01-02")
	}},
	{Sandi: form06SandiKualitas, Nama: "Kualitas", Value: func(r LoanRow) string {
		return sandiKualitasKredit(r.Collectibility)
	}},
	{Sandi: form06SandiMulaiMacet, Nama: "Tanggal Mulai Macet", Reason: "hanya jumlah hari tunggakan (dpd) yang tersimpan, bukan tanggal mulai macet"},
	{Sandi: form06SandiHariTunggakan, Nama: "Jumlah Hari Tunggakan Pokok dan/atau Bunga", Value: func(r LoanRow) string {
		return fmt.Sprintf("%d", r.DPD)
	}},
	{Sandi: form06SandiNominalTungg, Nama: "Nominal Tunggakan Pokok dan Bunga", Value: func(r LoanRow) string {
		// Nol tetap ditulis "0" (FormatRupiah), bukan dikosongkan.
		return FormatRupiah(r.OverdueUnpaid)
	}},
	{Sandi: form06SandiJenisDebitur, Nama: "Jenis Debitur", Value: func(r LoanRow) string {
		return dashIfEmpty(r.OJKPihakLawanCode)
	}},
	{Sandi: form06SandiSandiBank, Nama: "Sandi Bank", Reason: "sandi bank lawan tidak dimodelkan pada baris kredit"},
	{Sandi: form06SandiSektor, Nama: "Sektor Ekonomi", Value: func(r LoanRow) string {
		return dashIfEmpty(r.OJKSektorEkonomiCode)
	}},
	{Sandi: form06SandiKategori, Nama: "Kategori Usaha", Reason: "kategori usaha mikro/kecil/menengah butuh daftar sandi OJK yang belum ada rujukannya di repo"},
	{Sandi: form06SandiLokasi, Nama: "Lokasi Penggunaan", Value: func(r LoanRow) string {
		// Lampiran 03 SEOJK 16/2024 (sandi 4 digit, FK ke ojk_kabupaten). Nilai diisi
		// bank lewat SQL/seed (loans.ojk_kabupaten_code); baris tanpa sandi ditulis "-".
		return dashIfEmpty(r.OJKKabupatenCode)
	}},
	{Sandi: form06SandiSukuBunga, Nama: "Suku Bunga", Value: func(r LoanRow) string {
		// Lampiran II Form 06.00 – 2 butir XXIII: persentase suku bunga tahunan
		// sampai 2 digit desimal. Kolom basis data sudah dalam persen.
		return r.InterestRateAnnual.Round(2).StringFixed(2)
	}},
	{Sandi: form06SandiPenjamin, Nama: "Penjamin", Reason: "penjamin dan bagian yang dijamin belum tersedia pada baris kredit"},
	{Sandi: form06SandiAgunanPPKA, Nama: "Nilai Agunan yang Diperhitungkan untuk PPKA", Reason: "nilai agunan tersimpan pada modul agunan, tetapi nilai yang diperhitungkan untuk PPKA dihitung modul PPAP, bukan kolom kredit"},
	{Sandi: form06SandiKelonggaran, Nama: "Kelonggaran Tarik", Reason: "kelonggaran tarik (komitmen) belum dimodelkan sebagai fasilitas"},
	{Sandi: form06SandiPlafon, Nama: "Plafon", Value: func(r LoanRow) string {
		return FormatRupiah(r.PrincipalAmount)
	}},
	{Sandi: form06SandiBakiDebet, Nama: "Baki Debet", Value: func(r LoanRow) string {
		return FormatRupiah(r.Outstanding)
	}},
	{Sandi: form06SandiProvisi, Nama: "Provisi Belum Diamortisasi", Reason: "provisi belum diamortisasi tidak disimpan per kredit"},
	{Sandi: form06SandiBiayaTrans, Nama: "Biaya Transaksi Belum Diamortisasi", Reason: "biaya transaksi belum diamortisasi tidak disimpan per kredit"},
	{Sandi: form06SandiBungaTangguh, Nama: "Pendapatan Bunga Ditangguhkan Dalam Rangka Restrukturisasi", Reason: "akun pendapatan bunga ditangguhkan restrukturisasi belum dicatat per kredit"},
	{Sandi: form06SandiCadRestru, Nama: "Cadangan Kerugian Restrukturisasi", Reason: "cadangan kerugian restrukturisasi belum dicatat per kredit"},
	{Sandi: form06SandiBakiNeto, Nama: "Baki Debet Neto", Reason: "butuh provisi, biaya transaksi, pendapatan ditangguhkan, dan cadangan restrukturisasi yang belum tersedia; tidak dihitung sebagian"},
	{Sandi: form06SandiCKPN, Nama: "CKPN Yang Telah Dibentuk", Value: func(r LoanRow) string {
		return FormatRupiah(r.RequiredCKPN)
	}},
	{Sandi: form06SandiBungaAkanTer, Nama: "Pendapatan Bunga yang Akan Diterima", Value: func(r LoanRow) string {
		// Piutang bunga yang masih tercatat: jumlah sisa akruan (10400) yang belum
		// diselesaikan pembayaran. Nol tetap ditulis "0" (FormatRupiah).
		return FormatRupiah(r.AccruedProfit)
	}},
	{Sandi: form06SandiBungaProses, Nama: "Pendapatan Bunga Dalam Penyelesaian", Reason: "pendapatan bunga dalam penyelesaian belum dimodelkan"},
	{Sandi: form06SandiBMPK, Nama: "Status BMPK", Reason: "uji BMPK per pihak terkait belum dihitung"},
	{Sandi: form06SandiSifatKredit, Nama: "Sifat Kredit", Reason: "sifat kredit (pengalihan piutang/lainnya) butuh daftar sandi OJK yang belum ada rujukannya di repo"},
	{Sandi: form06SandiProgram, Nama: "Kredit Program Pemerintah", Reason: "program pemerintah (KUR dan lainnya) belum dimodelkan"},
	{Sandi: form06SandiSektorKUR, Nama: "Sektor Kredit Usaha Rakyat", Reason: "sektor KUR belum dimodelkan"},
	{Sandi: form06SandiAkadAwal, Nama: "Tanggal Akad Awal", Value: func(r LoanRow) string {
		if r.AkadDate == nil {
			return "-"
		}
		return r.AkadDate.Format("2006-01-02")
	}},
	{Sandi: form06SandiAkadAkhir, Nama: "Tanggal Akad Akhir", Reason: "tanggal akad terbaru tidak disimpan terpisah"},
	{Sandi: form06SandiLPBBTI, Nama: "Sandi LPBBTI", Reason: "kerja sama LPBBTI belum dimodelkan"},
	{Sandi: form06SandiCKPNBaik, Nama: "CKPN Aset Baik", Reason: "pemisahan CKPN per golongan kualitas tidak disimpan; required_ckpn hanya total per kredit"},
	{Sandi: form06SandiCKPNKurang, Nama: "CKPN Aset Kurang Baik", Reason: "pemisahan CKPN per golongan kualitas tidak disimpan; required_ckpn hanya total per kredit"},
	{Sandi: form06SandiCKPNTidak, Nama: "CKPN Aset Tidak Baik", Reason: "pemisahan CKPN per golongan kualitas tidak disimpan; required_ckpn hanya total per kredit"},
	{Sandi: form06SandiKlasifikasi, Nama: "Klasifikasi Aset Keuangan", Reason: "klasifikasi SAK EP belum dipetakan per kredit"},
	{Sandi: form06SandiJenisCKPN, Nama: "Jenis CKPN", Value: func(r LoanRow) string {
		return sandiJenisCKPN(r.CKPNMethod)
	}},
}

// sandiJenisCKPN memetakan segel metode per kredit ke sandi kolom "Jenis CKPN"
// (sandi XXI Form 06.00; penjelasan umum kolom Q SEOJK 16/2024): sandi 1 = CKPN
// individual, sandi 2 = CKPN kolektif. Kredit lama tanpa segel dilaporkan kolektif
// (bawaan kebijakan mesin), bukan dikosongkan, agar baris tetap dilaporkan.
func sandiJenisCKPN(method string) string {
	switch domain.CKPNIndividualMethod(method) {
	// Bentuk T3 (DCF/COLLATERAL/MAX) dan bentuk nama penuh T0 (INDIVIDUAL_*)
	// keduanya diterima: kolom ckpn_method pernah membawa kedua konvensi di
	// rancangan, dan laporan tidak boleh salah golongan karena penamaan lama.
	case domain.CKPNIndividualMethodDCF, domain.CKPNIndividualMethodCollateral, domain.CKPNIndividualMethodMax,
		"INDIVIDUAL_DCF", "INDIVIDUAL_COLLATERAL", "INDIVIDUAL_MAX":
		return "1"
	default:
		return "2"
	}
}

// buildForm06 menyusun Form 06.00 dari baris kredit. Kredit di luar status berjalan
// (DISBURSED/DEFAULTED) dilewati karena tidak punya baki debet yang dilaporkan.
func buildForm06(rows []LoanRow) TableSection {
	sec := TableSection{
		Form:     "06.00",
		Name:     formName("06.00"),
		KeyLabel: "No. Rekening",
		Notes: []string{
			"Baris dibangun dari keadaan kredit saat ekspor dijalankan; sistem belum menyimpan riwayat posisi kredit per akhir bulan, sehingga posisi periode lampau tidak dapat direkonstruksi.",
		},
	}
	for _, c := range form06Columns {
		if c.Reason != "" {
			sec.Unavailable = append(sec.Unavailable, ColumnUnavailable{Sandi: c.Sandi, Nama: c.Nama, Reason: c.Reason})
			continue
		}
		sec.Columns = append(sec.Columns, TableColumn{Sandi: c.Sandi, Nama: c.Nama})
	}
	for _, r := range rows {
		if !aktifUntukOJK(r.Status) {
			continue
		}
		row := TableRow{Key: dashIfEmpty(r.LoanNumber)}
		for _, c := range form06Columns {
			if c.Reason != "" || c.Value == nil {
				continue
			}
			row.Cells = append(row.Cells, TableCell{Sandi: c.Sandi, Nama: c.Nama, Value: c.Value(r)})
		}
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// sandiStatusRestrukturisasi memetakan status restrukturisasi ke sandi Lampiran II
// Form 06.00 – 2 butir VII: 10 tidak direstrukturisasi, 20/21/22 restrukturisasi 1-3.
func sandiStatusRestrukturisasi(r LoanRow) string {
	if !r.IsRestructured {
		return "10"
	}
	switch {
	case r.RestructuredCount <= 1:
		return "20"
	case r.RestructuredCount == 2:
		return "21"
	default:
		return "22"
	}
}

// sandiKualitasKredit memetakan kolektibilitas internal ke sandi Form 06.00 – 2
// butir XIV: 1 Lancar, 2 Dalam Perhatian Khusus, 3 Kurang Lancar, 4 Diragukan, 5 Macet.
func sandiKualitasKredit(collectibility string) string {
	switch normalizeCollectibility(collectibility) {
	case "1_LANCAR":
		return "1"
	case "2_DPK":
		return "2"
	case "3_KURANG_LANCAR":
		return "3"
	case "4_DIRAGUKAN":
		return "4"
	case "5_MACET":
		return "5"
	default:
		return "-"
	}
}

// normalizeCollectibility menyeragamkan penulisan kolektibilitas (mis. DPK vs 2_DPK).
func normalizeCollectibility(v string) string {
	s := strings.ToUpper(strings.TrimSpace(v))
	switch s {
	case "LANCAR", "1", "1_LANCAR":
		return "1_LANCAR"
	case "DPK", "2", "2_DPK", "DALAM_PERHATIAN_KHUSUS":
		return "2_DPK"
	case "KURANG_LANCAR", "3", "3_KURANG_LANCAR":
		return "3_KURANG_LANCAR"
	case "DIRAGUKAN", "4", "4_DIRAGUKAN":
		return "4_DIRAGUKAN"
	case "MACET", "5", "5_MACET":
		return "5_MACET"
	default:
		return s
	}
}

// dashIfEmpty mengembalikan "-" untuk nilai kosong, agar kolom teks kosong tidak
// terlihat seperti nilai nol.
func dashIfEmpty(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}
