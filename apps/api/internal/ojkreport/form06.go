package ojkreport

import (
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
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
	{Sandi: form06SandiIDPihakLawan, Nama: "ID Pihak Lawan", Value: func(r LoanRow) string {
		// ID Pihak Lawan = nomor CIF internal nasabah (harus sama dengan CIF pada SLIK;
		// BUKAN sandi Lampiran 02). BAB II Lampiran II SEOJK 16/2024, PDF #page 66.
		return dashIfEmpty(r.IDPihakLawan)
	}},
	{Sandi: form06SandiNoIdentitas, Nama: "No. Identitas", Reason: "NIK/NPWP nasabah tersimpan terenkripsi untuk dokumen dan tidak dibuka sebagai keluaran laporan"},
	{Sandi: form06SandiKelompok, Nama: "Kode Kelompok Kredit", Value: func(r LoanRow) string {
		// Kode unik angka/huruf buatan BPR per kelompok peminjam pihak tidak terkait,
		// BUKAN daftar sandi OJK (Lampiran II Form 06.00-3, PDF #page 158). Diisi bank
		// lewat SQL/seed (loans.ojk_kelompok_kredit_code); baris tanpa isian ditulis "-".
		return dashIfEmpty(r.OJKKelompokKreditCode)
	}},
	{Sandi: form06SandiNoRekening, Nama: "No. Rekening", Value: func(r LoanRow) string {
		return dashIfEmpty(r.LoanNumber)
	}},
	{Sandi: form06SandiJenis, Nama: "Jenis", Reason: "kanal penyaluran kredit (sindikasi/kerja sama/LPBBTI) belum dimodelkan; " +
		"kebijakan: bank bukan peserta secara bawaan, " +
		"partisipasi dikonfirmasi saat onboarding bank, dan selama bukan peserta kolom ditulis '-' (bukan cacat data)"},
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
	{Sandi: form06SandiSumberDana, Nama: "Sumber Dana Pelunasan", Value: func(r LoanRow) string {
		// Sandi inline Lampiran II Form 06.00-2 (PDF #page 153 daftar; #page 160
		// penjelasan): 10/21/22/31/32. Diisi bank lewat SQL/seed; kosong ditulis "-".
		return dashIfEmpty(r.OJKSumberDanaCode)
	}},
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
	{Sandi: form06SandiMulaiMacet, Nama: "Tanggal Mulai Macet", Value: func(r LoanRow) string {
		// Tanggal kredit mulai dinyatakan macet (Lampiran II Form 06.00-3, PDF #page
		// 161). Tidak diturunkan dari DPD: DPD menghitung hari sejak jatuh tempo,
		// bukan tanggal peralihan kualitas. Diisi bank lewat SQL/seed; kosong ditulis "-".
		if r.OJKTanggalMulaiMacet == nil {
			return "-"
		}
		return r.OJKTanggalMulaiMacet.Format("2006-01-02")
	}},
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
	{Sandi: form06SandiKategori, Nama: "Kategori Usaha", Value: func(r LoanRow) string {
		// Sandi inline Lampiran II Form 06.00-2 (PDF #page 154 daftar; #page 162-163
		// penjelasan kriteria): 1 Mikro, 2 Kecil, 3 Menengah, 4 Selain Mikro/Kecil/
		// Menengah. Diisi bank lewat SQL/seed; kosong ditulis "-".
		return dashIfEmpty(r.OJKKategoriUsahaCode)
	}},
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
	// Lampiran II Form 06.00-1 (PDF #page 148) memecah kolom XXIV menjadi dua subkolom:
	// Golongan Penjamin dan Bagian yang Dijamin. Keduanya dibawa sebagai dua sel bersandi
	// XXIV yang sama, mengikuti susunan grid.
	{Sandi: form06SandiPenjamin, Nama: "Golongan Penjamin", Value: func(r LoanRow) string {
		// Golongan penjamin mengacu Lampiran 02 Daftar Sandi Pihak Lawan (PDF #page 154;
		// penjelasan #page 163-164). Diisi bank lewat SQL/seed (loans.ojk_penjamin_code).
		return dashIfEmpty(r.OJKPenjaminCode)
	}},
	{Sandi: form06SandiPenjamin, Nama: "Bagian yang Dijamin", Value: func(r LoanRow) string {
		// Persentase bagian yang dijamin, 0-100 sampai 2 desimal (PDF #page 154/164).
		if r.OJKPenjaminBagianPct == nil {
			return "-"
		}
		return r.OJKPenjaminBagianPct.StringFixed(2)
	}},
	{Sandi: form06SandiAgunanPPKA, Nama: "Nilai Agunan yang Diperhitungkan untuk PPKA", Value: func(r LoanRow) string {
		// Nilai agunan yang benar-benar mengurangi dasar PPKA pada run PPAP terakhir
		// (loans.ojk_agunan_ppka_amount, migrasi 000109). Disimpan modul PPAP saat
		// ppap.collateral.enabled aktif. Bila modul agunan belum diaktifkan/belum
		// dijalankan atau bank belum mengisi, laporan menulis "-", BUKAN 0: nilainya
		// tidak dihitung ulang di sini (rumus pengurang ada di modul PPAP/domain).
		if r.OJKAgunanPPKAAmount == nil {
			return "-"
		}
		return FormatRupiah(*r.OJKAgunanPPKAAmount)
	}},
	{Sandi: form06SandiKelonggaran, Nama: "Kelonggaran Tarik", Value: func(r LoanRow) string {
		// Bagian plafon komitmen yang belum ditarik
		// (loans.ojk_kelonggaran_tarik_amount, migrasi 000109). Fasilitas komitmen belum
		// dimodelkan, jadi bank mengisinya; kosong ditulis "-", bukan diturunkan dari
		// plafon dikurangi baki.
		if r.OJKKelonggaranTarikAmount == nil {
			return "-"
		}
		return FormatRupiah(*r.OJKKelonggaranTarikAmount)
	}},
	{Sandi: form06SandiPlafon, Nama: "Plafon", Value: func(r LoanRow) string {
		return FormatRupiah(r.PrincipalAmount)
	}},
	{Sandi: form06SandiBakiDebet, Nama: "Baki Debet", Value: func(r LoanRow) string {
		return FormatRupiah(r.Outstanding)
	}},
	{Sandi: form06SandiProvisi, Nama: "Provisi Belum Diamortisasi", Value: func(r LoanRow) string {
		// Bagian provisi yang belum menjadi pendapatan bunga periode berjalan (kolom
		// XXIX; loans.ojk_provisi_belum_diamortisasi_amount, migrasi 000111). Jadwal
		// amortisasi provisi belum dimodelkan sehingga bank mengisinya lewat SQL/seed;
		// definisi resmi Lampiran II Form 06.00-3 (PDF #page 165). Nil = belum diisi,
		// ditulis "-", BUKAN diturunkan dari rumus amortisasi karangan.
		return formatNominalOpsional(r.OJKProvisiBelumDiamortisasiAmount)
	}},
	{Sandi: form06SandiBiayaTrans, Nama: "Biaya Transaksi Belum Diamortisasi", Value: func(r LoanRow) string {
		// Bagian biaya transaksi yang belum diamortisasi dan belum menjadi pengurang
		// pendapatan bunga (kolom XXX; migrasi 000111). PDF #page 165. Bank mengisi.
		return formatNominalOpsional(r.OJKBiayaTransaksiBelumDiamortisasiAmount)
	}},
	{Sandi: form06SandiBungaTangguh, Nama: "Pendapatan Bunga Ditangguhkan Dalam Rangka Restrukturisasi", Value: func(r LoanRow) string {
		// Pendapatan bunga ditangguhkan dalam rangka restrukturisasi melalui
		// kapitalisasi tunggakan bunga ke pokok (kolom XXXI; migrasi 000111).
		// PDF #page 165. Pencatatan per kredit belum dimodelkan; bank mengisi.
		return formatNominalOpsional(r.OJKPendapatanBungaDitangguhkanAmount)
	}},
	{Sandi: form06SandiCadRestru, Nama: "Cadangan Kerugian Restrukturisasi", Value: func(r LoanRow) string {
		// Selisih nilai kini arus kas masa depan menurut perjanjian restrukturisasi dan
		// baki debet sebelum restrukturisasi (kolom XXXII; migrasi 000111).
		// PDF #page 165. Pemetaan dari saldo kerugian restrukturisasi belum diputuskan;
		// bank mengisi.
		return formatNominalOpsional(r.OJKCadanganKerugianRestrukturisasiAmount)
	}},
	{Sandi: form06SandiBakiNeto, Nama: "Baki Debet Neto", Value: func(r LoanRow) string {
		// Rumus resmi Lampiran II Form 06.00-3 (PDF #page 165): baki debet dikurangi
		// provisi belum diamortisasi, ditambah biaya transaksi belum diamortisasi,
		// dikurangi pendapatan bunga ditangguhkan restrukturisasi, dan dikurangi
		// cadangan kerugian restrukturisasi. Dihitung hanya bila komponennya sudah
		// tersimpan; bila belum, ditulis "-" (BUKAN dianggap nol).
		neto, ok := bakiDebetNeto(r)
		if !ok {
			return "-"
		}
		return FormatRupiah(neto)
	}},
	{Sandi: form06SandiCKPN, Nama: "CKPN Yang Telah Dibentuk", Value: func(r LoanRow) string {
		return FormatRupiah(r.RequiredCKPN)
	}},
	{Sandi: form06SandiBungaAkanTer, Nama: "Pendapatan Bunga yang Akan Diterima", Value: func(r LoanRow) string {
		// Piutang bunga yang masih tercatat: jumlah sisa akruan (10400) yang belum
		// diselesaikan pembayaran. Nol tetap ditulis "0" (FormatRupiah).
		return FormatRupiah(r.AccruedProfit)
	}},
	{Sandi: form06SandiBungaProses, Nama: "Pendapatan Bunga Dalam Penyelesaian", Reason: "pendapatan bunga dalam penyelesaian belum dimodelkan"},
	{Sandi: form06SandiBMPK, Nama: "Status BMPK", Value: func(r LoanRow) string {
		// Status batas pihak terkait (bmpk_related_parties/bmpk_limits) yang dimuat
		// satu kali oleh perakit. Baris yang nasabahnya belum ditandai pihak terkait
		// ditulis "-", bukan dianggap sesuai batas.
		return dashIfEmpty(r.BMPKStatus)
	}},
	{Sandi: form06SandiSifatKredit, Nama: "Sifat Kredit", Value: func(r LoanRow) string {
		// Sandi inline Lampiran II Form 06.00-2 (PDF #page 155 daftar; #page 166
		// penjelasan): 2 Pengalihan piutang, 9 Lainnya. Diisi bank lewat SQL/seed;
		// kosong ditulis "-".
		return dashIfEmpty(r.OJKSifatKreditCode)
	}},
	{Sandi: form06SandiProgram, Nama: "Kredit Program Pemerintah", Reason: "program pemerintah (KUR dan lainnya) belum dimodelkan; " +
		"kebijakan: bank bukan peserta secara bawaan, " +
		"partisipasi dikonfirmasi saat onboarding bank, dan selama bukan peserta kolom ditulis '-' (bukan cacat data)"},
	{Sandi: form06SandiSektorKUR, Nama: "Sektor Kredit Usaha Rakyat", Reason: "sektor KUR belum dimodelkan; " +
		"kebijakan: bank bukan peserta secara bawaan, " +
		"partisipasi dikonfirmasi saat onboarding bank, dan selama bukan peserta kolom ditulis '-' (bukan cacat data)"},
	{Sandi: form06SandiAkadAwal, Nama: "Tanggal Akad Awal", Value: func(r LoanRow) string {
		if r.AkadDate == nil {
			return "-"
		}
		return r.AkadDate.Format("2006-01-02")
	}},
	{Sandi: form06SandiAkadAkhir, Nama: "Tanggal Akad Akhir", Value: func(r LoanRow) string {
		// Tanggal akad terbaru (Lampiran II Form 06.00-3, PDF #page 167): bila ada
		// addendum akibat restrukturisasi, tanggal addendum terakhir; selain itu akad
		// awal. Koreksi nominal yang tidak mengubah perjanjian tidak menambah addendum.
		if r.RestructuredAt != nil {
			return r.RestructuredAt.Format("2006-01-02")
		}
		if r.AkadDate == nil {
			return "-"
		}
		return r.AkadDate.Format("2006-01-02")
	}},
	{Sandi: form06SandiLPBBTI, Nama: "Sandi LPBBTI", Reason: "kerja sama LPBBTI belum dimodelkan; " +
		"kebijakan: bank bukan peserta secara bawaan, " +
		"partisipasi dikonfirmasi saat onboarding bank, dan selama bukan peserta kolom ditulis '-' (bukan cacat data)"},
	{Sandi: form06SandiCKPNBaik, Nama: "CKPN Aset Baik", Reason: "pemisahan CKPN per golongan kualitas tidak disimpan; required_ckpn hanya total per kredit"},
	{Sandi: form06SandiCKPNKurang, Nama: "CKPN Aset Kurang Baik", Reason: "pemisahan CKPN per golongan kualitas tidak disimpan; required_ckpn hanya total per kredit"},
	{Sandi: form06SandiCKPNTidak, Nama: "CKPN Aset Tidak Baik", Reason: "pemisahan CKPN per golongan kualitas tidak disimpan; required_ckpn hanya total per kredit"},
	{Sandi: form06SandiKlasifikasi, Nama: "Klasifikasi Aset Keuangan", Value: func(r LoanRow) string {
		// Keputusan pemilik: sediakan kolom, tanpa auto-klasifikasi. Nilai diisi bank
		// lewat SQL/seed (loans.ojk_klasifikasi_aset_code, migrasi 000108); baris tanpa
		// isian ditulis "-", bukan diturunkan dari kolektibilitas.
		return dashIfEmpty(r.OJKKlasifikasiAsetCode)
	}},
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
			"Status BMPK (kolom XXXVII) diambil dari hasil uji batas modul BMPK per pihak terkait; baris yang nasabahnya belum ditandai pihak terkait ditulis '-' (bukan dianggap sesuai batas).",
			"Nilai Agunan yang Diperhitungkan untuk PPKA (kolom XXV) diambil dari hasil run PPAP terakhir (loans.ojk_agunan_ppka_amount) dan tidak dihitung ulang di sini; bila modul agunan belum aktif/belum dijalankan atau bank belum mengisi, kolom ditulis '-' (bukan nol). Kelonggaran Tarik (kolom XXVI) diisi bank dan ditulis '-' bila belum diisi.",
			"Kolom XXIX Provisi Belum Diamortisasi, XXX Biaya Transaksi Belum Diamortisasi, XXXI Pendapatan Bunga Ditangguhkan Dalam Rangka Restrukturisasi, dan XXXII Cadangan Kerugian Restrukturisasi diisi bank (jadwal amortisasi provisi/biaya dan pencatatan penangguhan restrukturisasi belum dimodelkan); nilainya ditulis '-' bila belum diisi, BUKAN diturunkan dari rumus karangan. Kolom XXXIII Baki Debet Neto dihitung dari komponen di atas menurut definisi Lampiran II Form 06.00-3 (PDF #page 165): baki debet - provisi belum diamortisasi + biaya transaksi belum diamortisasi - pendapatan bunga ditangguhkan restrukturisasi - cadangan kerugian restrukturisasi; bila komponennya belum tersimpan, kolom ditulis '-' (bukan nol).",
			"Kolom kondisional bank (VI, XXXIX, XL, XLIII) dinyatakan belum tersedia dengan alasan KEBIJAKAN, bukan cacat data: bank bukan peserta KUR/LPBBTI secara bawaan, partisipasi dikonfirmasi saat onboarding bank, dan selama bukan peserta kolom terkait ditulis '-'.",
		},
	}

	// Kolom BMPK hanya tersedia bila minimal satu baris punya status pihak terkait.
	// Tanpa itu kolom dinyatakan belum tersedia beserta alasannya, bukan diisi nol.
	adaBMPK := false
	for _, r := range rows {
		if strings.TrimSpace(r.BMPKStatus) != "" {
			adaBMPK = true
			break
		}
	}
	cols := form06Columns
	if !adaBMPK {
		cols = make([]form06Column, len(form06Columns))
		copy(cols, form06Columns)
		for i := range cols {
			if cols[i].Sandi == form06SandiBMPK {
				cols[i].Reason = "modul BMPK tidak tersedia atau belum ada nasabah pada baris ini yang ditandai pihak terkait (bmpk_related_parties) sehingga batasnya belum dapat diuji"
			}
		}
	}

	for _, c := range cols {
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
		for _, c := range cols {
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

// formatNominalOpsional menulis nominal dalam rupiah penuh untuk nilai yang tersimpan
// dan "-" bila nilainya belum diisi (NULL). Nol yang benar-benar tersimpan tetap
// ditulis "0", berbeda dari NULL.
func formatNominalOpsional(d *decimal.Decimal) string {
	if d == nil {
		return "-"
	}
	return FormatRupiah(*d)
}

// bakiDebetNeto menghitung kolom XXXIII Baki Debet Neto menurut definisi resmi
// Lampiran II SEOJK No. 16/SEOJK.03/2024 Form 06.00-3 (PDF #page 165, hlm. 113): "baki
// debet kredit setelah dikurangi dengan provisi yang belum diamortisasi dan ditambah
// dengan biaya transaksi yang belum diamortisasi serta dikurangi dengan pendapatan bunga
// yang ditangguhkan dalam rangka restrukturisasi kredit dan cadangan kerugian
// restrukturisasi." Urutan dan tanda mengikuti kalimat itu apa adanya.
//
// ok=false bila komponen belum dapat ditentukan: provisi dan biaya transaksi wajib
// tersimpan. Untuk kredit yang TIDAK direstrukturisasi, pendapatan ditangguhkan dan
// cadangan kerugian restrukturisasi memang nol menurut definisi kolomnya, jadi tidak
// menghalangi; untuk kredit yang direstrukturisasi keduanya wajib tersimpan agar kolom
// tidak mencampur angka yang ada dengan yang belum diisi. Nilai nil BUKAN nol.
func bakiDebetNeto(r LoanRow) (decimal.Decimal, bool) {
	if r.OJKProvisiBelumDiamortisasiAmount == nil || r.OJKBiayaTransaksiBelumDiamortisasiAmount == nil {
		return decimal.Zero, false
	}
	neto := r.Outstanding.
		Sub(*r.OJKProvisiBelumDiamortisasiAmount).
		Add(*r.OJKBiayaTransaksiBelumDiamortisasiAmount)

	pendapatan := decimal.Zero
	if r.OJKPendapatanBungaDitangguhkanAmount != nil {
		pendapatan = *r.OJKPendapatanBungaDitangguhkanAmount
	} else if r.IsRestructured {
		return decimal.Zero, false
	}
	cadangan := decimal.Zero
	if r.OJKCadanganKerugianRestrukturisasiAmount != nil {
		cadangan = *r.OJKCadanganKerugianRestrukturisasiAmount
	} else if r.IsRestructured {
		return decimal.Zero, false
	}
	return neto.Sub(pendapatan).Sub(cadangan), true
}
