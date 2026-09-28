package ojkreport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// sources.go memuat kontrak data tambahan yang dipakai Form 00.00, 05.00, dan 06.00
// beserta komponen rasio NPL/ROA. Kontrak ini SENGAJA terpisah dari Source (laporan
// journal-based): bila sumber data tidak tersedia, form terkait ditandai belum
// tersedia beserta alasannya, bukan diisi angka kosong.
//
// Seluruh kontrak data kredit/penempatan bersifat BANK-WIDE. Handler sudah menolak
// aktor non-lintas cabang sebelum builder dipanggil; implementasi RepoSource
// menegakkannya sekali lagi agar kebijakan tidak bergantung pada satu tempat.

// ErrOJKBankWide menandai permintaan data bank-wide oleh aktor yang tidak berwenang
// atas seluruh bank.
var ErrOJKBankWide = errors.New("laporan OJK bersifat bank-wide dan hanya dapat dibangun oleh peran lintas cabang")

// Kunci konfigurasi identitas bank Form 00.00 yang tidak muat di tabel bank_profile.
// Nilai kanoniknya dimiliki paket domain supaya penyimpanan (repository), layanan,
// dan pengekspor OJK tidak mendefinisikan kunci yang sama dua kali. Kunci 8 butir
// pertama di-seed migrasi 000046; 12 butir sisanya di-seed migrasi 000096.
const (
	OJKBankEmailKey     = domain.OJKBankEmailKey
	OJKBankWebsiteKey   = domain.OJKBankWebsiteKey
	OJKBankCityCodeKey  = domain.OJKBankCityCodeKey
	OJKBankOJKRegionKey = domain.OJKBankOJKRegionKey
	OJKPICNameKey       = domain.OJKPICNameKey
	OJKPICDivisionKey   = domain.OJKPICDivisionKey
	OJKPICPhoneKey      = domain.OJKPICPhoneKey
	OJKPICEmailKey      = domain.OJKPICEmailKey
)

// LoanRow adalah satu fasilitas kredit yang dibutuhkan Form 06.00 dan rasio NPL.
// Hanya field yang benar-benar ada di basis data yang dimuat; kolom Form 06.00 yang
// tidak punya sumber ditandai belum tersedia di form06.go, bukan diisi nol.
type LoanRow struct {
	// Status adalah status kredit internal (DISBURSED, DEFAULTED, dst.).
	Status         string
	BranchCode     string
	LoanNumber     string
	CustomerID     string
	Collectibility string
	DPD            int
	// Outstanding adalah baki debet pokok.
	Outstanding decimal.Decimal
	// RequiredCKPN adalah target CKPN yang terakhir diakui untuk kredit ini.
	RequiredCKPN decimal.Decimal
	// RequiredPPAP adalah target PPKA terakhir yang diakui untuk kredit ini
	// (loans.required_ppap), keluaran modul PPAP. Dipakai Laporan Perbedaan Kualitas
	// sebagai jejak PPKA yang tersimpan; ini NOMINAL cadangan, bukan golongan kualitas.
	RequiredPPAP decimal.Decimal
	// CKPNMethod adalah segel metode per kredit (loans.ckpn_method): sumber kolom
	// "Jenis CKPN" Form 06.00 (sandi XXI). Kosong berarti kredit lama sebelum T3;
	// dilaporkan kolektif (bawaan kebijakan) agar tidak tersangkut data lama.
	CKPNMethod         string
	IsRestructured     bool
	RestructuredCount  int
	InterestRateAnnual decimal.Decimal
	PrincipalAmount    decimal.Decimal
	AkadDate           *time.Time
	FinalDueDate       *time.Time
	// FirstInstallmentDate adalah jatuh tempo angsuran pertama (MIN due_date)
	// menurut jadwal kredit; sumber kolom XIII "Angsuran Pokok Pertama" Form 06.00.
	// Nil berarti kredit tidak punya jadwal angsuran.
	FirstInstallmentDate *time.Time
	// OverdueUnpaid adalah nominal tunggakan pokok dan bunga: jumlah sisa
	// (principal - paid_principal) + (profit - paid_profit) untuk angsuran yang
	// jatuh tempo sebelum asOf dan belum lunas. Sumber kolom XVII "Nominal
	// Tunggakan Pokok dan Bunga" Form 06.00. Nol berarti tidak menunggak.
	OverdueUnpaid decimal.Decimal
	// AccruedProfit adalah piutang bunga yang masih tercatat: jumlah sisa akruan
	// (loan_schedules.profit_accrued_amount) seluruh angsuran. Sumber kolom XXXV
	// "Pendapatan Bunga yang Akan Diterima" Form 06.00. Nol berarti tidak ada
	// bunga yang diakru dan belum diselesaikan pembayarannya.
	AccruedProfit decimal.Decimal
	// OJKPihakLawanCode adalah sandi pihak lawan nasabah (Lampiran 02 SEOJK
	// 16/2024), sumber kolom XVIII "Jenis Debitur" Form 06.00. Kosong berarti
	// bank belum mengisi sandinya; laporan menulis "-".
	OJKPihakLawanCode string
	// OJKSektorEkonomiCode adalah sandi sektor ekonomi nasabah (Lampiran 05 SEOJK
	// 16/2024), sumber kolom XX "Sektor Ekonomi" Form 06.00. Kosong berarti bank
	// belum mengisi sandinya; laporan menulis "-".
	OJKSektorEkonomiCode string
	// OJKHubunganBankCode adalah sandi inline Hubungan dengan Bank (11/12/20,
	// Lampiran II Form 06.00-2), sumber kolom IX Form 06.00. Kosong berarti bank
	// belum mengisi sandinya; laporan menulis "-".
	OJKHubunganBankCode string
	// OJKJenisPenggunaanCode adalah sandi inline Jenis Penggunaan (10/20/31/32/35/39),
	// sumber kolom VIII Form 06.00. Kosong berarti bank belum mengisi sandinya.
	OJKJenisPenggunaanCode string
	// OJKPeriodePembayaranCode adalah sandi inline Periode Pembayaran Pokok dan Bunga
	// (1-8), sumber kolom XI Form 06.00. Kosong berarti bank belum mengisi sandinya.
	OJKPeriodePembayaranCode string
	// OJKKabupatenCode adalah sandi Kabupaten/Kota lokasi penggunaan kredit (Lampiran 03
	// SEOJK 16/2024), sumber kolom XXII "Lokasi Penggunaan" Form 06.00. Kosong berarti
	// bank belum mengisi sandinya; laporan menulis "-".
	OJKKabupatenCode string
	// IDPihakLawan adalah nomor CIF internal nasabah (customers.cif_number), sumber
	// kolom II "ID Pihak Lawan" Form 06.00. Bukan sandi OJK: harus sama dengan CIF yang
	// dilaporkan ke SLIK (BAB II Lampiran II, PDF #page 66). Kosong berarti nasabah tak
	// terbaca atau belum ber-CIF; laporan menulis "-".
	IDPihakLawan string
	// Kolom K1 Form 06.00 lain (migrasi 000107) yang diisi bank lewat SQL/seed. Semua
	// kosong/nil berarti bank belum mengisi; laporan menulis "-", bukan menebak.
	//
	// OJKKelompokKreditCode adalah Kode Kelompok Kredit (kolom IV): kode unik
	// angka/huruf buatan BPR, bukan daftar sandi OJK (PDF #page 158).
	OJKKelompokKreditCode string
	// OJKSumberDanaCode adalah sandi inline Sumber Dana Pelunasan (kolom X).
	OJKSumberDanaCode string
	// OJKKategoriUsahaCode adalah sandi inline Kategori Usaha (kolom XXI).
	OJKKategoriUsahaCode string
	// OJKSifatKreditCode adalah sandi inline Sifat Kredit (kolom XXXVIII).
	OJKSifatKreditCode string
	// OJKPenjaminCode adalah sandi golongan Penjamin (kolom XXIV butir 1, Lampiran 02);
	// OJKPenjaminBagianPct adalah bagian yang dijamin dalam persen (butir 2).
	OJKPenjaminCode      string
	OJKPenjaminBagianPct *decimal.Decimal
	// OJKTanggalMulaiMacet adalah tanggal kredit mulai berkualitas macet (kolom XV).
	OJKTanggalMulaiMacet *time.Time
	// OJKKlasifikasiAsetCode adalah sandi klasifikasi aset keuangan SAK EP per kredit
	// (kolom XLVII, migrasi 000108), diisi bank lewat SQL/seed. Kosong berarti bank
	// belum mengisi dan laporan menulis "-", BUKAN diturunkan dari kolektibilitas.
	OJKKlasifikasiAsetCode string
	// OJKAgunanPPKAAmount adalah nilai agunan yang diperhitungkan untuk PPKA (kolom XXV
	// Form 06.00; loans.ojk_agunan_ppka_amount, migrasi 000109). Nil berarti belum
	// diisi/belum dihitung (modul agunan belum diaktifkan atau bank belum mengisi);
	// laporan menulis "-", BUKAN nol. Nilainya disimpan modul PPAP, tidak dihitung ulang
	// di perakit laporan.
	OJKAgunanPPKAAmount *decimal.Decimal
	// OJKKelonggaranTarikAmount adalah kelonggaran tarik (kolom XXVI Form 06.00;
	// loans.ojk_kelonggaran_tarik_amount, migrasi 000109): bagian plafon komitmen yang
	// belum ditarik. Nil berarti bank belum mengisi; laporan menulis "-".
	OJKKelonggaranTarikAmount *decimal.Decimal
	// Kolom XXIX–XXXII Form 06.00 (migrasi 000111): komponen amortisasi provisi/biaya
	// dan restrukturisasi yang belum dihitung mesin, disimpan nullable dan diisi bank.
	// Nil berarti belum diisi; laporan menulis "-", BUKAN nol. Definisi resmi Lampiran II
	// SEOJK 16/2024 Form 06.00-3 (PDF #page 165).
	//
	// OJKProvisiBelumDiamortisasiAmount adalah provisi yang belum menjadi pendapatan
	// bunga periode berjalan (kolom XXIX).
	OJKProvisiBelumDiamortisasiAmount *decimal.Decimal
	// OJKBiayaTransaksiBelumDiamortisasiAmount adalah biaya transaksi yang belum
	// diamortisasi dan belum menjadi pengurang pendapatan bunga (kolom XXX).
	OJKBiayaTransaksiBelumDiamortisasiAmount *decimal.Decimal
	// OJKPendapatanBungaDitangguhkanAmount adalah pendapatan bunga ditangguhkan dalam
	// rangka restrukturisasi akibat kapitalisasi tunggakan bunga ke pokok (kolom XXXI).
	OJKPendapatanBungaDitangguhkanAmount *decimal.Decimal
	// OJKCadanganKerugianRestrukturisasiAmount adalah cadangan kerugian restrukturisasi
	// (kolom XXXII).
	OJKCadanganKerugianRestrukturisasiAmount *decimal.Decimal
	// RestructuredAt adalah tanggal restrukturisasi terakhir, dipakai menurunkan kolom
	// XLII "Tanggal Akad Akhir" (akad terbaru) bila kredit pernah direstrukturisasi.
	RestructuredAt *time.Time
	// BMPKStatus adalah status BMPK pihak terkait pemilik kredit ini, diisi perakit dari
	// SATU pemuatan laporan BMPK (bukan query per kredit). Kosong berarti nasabah belum
	// ditandai pihak terkait atau modul BMPK tidak tersedia; kolom XXXVII ditulis "-".
	BMPKStatus string
}

// Himpunan sandi inline Form 06.00 yang tersurat di Lampiran II SEOJK 16/2024.
// Konstanta ini adalah daftar resmi yang boleh diisi bank lewat SQL/seed pada kolom
// penyimpanan terkait (migrasi 000107); laporan membacanya apa adanya tanpa pemetaan,
// karena sumbernya sudah sandi.
const (
	// Sumber Dana Pelunasan, kolom X (PDF #page 153 daftar; #page 160 penjelasan).
	OJKSumberDanaGajiHonor       = "10"
	OJKSumberDanaUsahaSubsidi    = "21"
	OJKSumberDanaUsahaNonsubsidi = "22"
	OJKSumberDanaLainSubsidi     = "31"
	OJKSumberDanaLainNonsubsidi  = "32"

	// Kategori Usaha, kolom XXI (PDF #page 154 daftar; #page 162-163 penjelasan).
	OJKKategoriUsahaMikro    = "1"
	OJKKategoriUsahaKecil    = "2"
	OJKKategoriUsahaMenengah = "3"
	OJKKategoriUsahaSelain   = "4"

	// Sifat Kredit, kolom XXXVIII (PDF #page 155 daftar; #page 166 penjelasan).
	OJKSifatKreditPengalihanPiutang = "2"
	OJKSifatKreditLainnya           = "9"
)

// LoanDataSource menyediakan kredit bank-wide. asOf dipakai implementasi untuk
// memilih posisi bila riwayat tersedia.
type LoanDataSource interface {
	ListLoansForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]LoanRow, error)
}

// BankProfileConfig adalah identitas bank untuk Form 00.00. Nilai kosong berarti
// field belum dikonfigurasi; Configured=false berarti identitas inti (nama) belum ada
// sehingga seluruh form dinyatakan belum tersedia.
type BankProfileConfig struct {
	Name          string
	Address       string
	City          string
	CityCode      string
	OJKRegionCode string
	Phone         string
	Email         string
	Website       string
	NPWP          string
	PICName       string
	PICDivision   string
	PICPhone      string
	PICEmail      string
	// Butir 10 s.d. 21 Form 00.00 (migrasi 000096). Kosong berarti belum diisi.
	DividendsPaid        string
	AnnualBonusTantiem   string
	AuditInfo            string
	ShareNominalValue    string
	PublicOfferingStatus string
	PVAStatus            string
	EBankingStatus       string
	ITProvider           string
	LakuPandaiProvider   string
	LakuPandaiAgentCount string
	RUPSOwnershipChange  string
	UltimateShareholders string
	Configured           bool
}

// BankProfileSource membaca identitas bank dari konfigurasi. Implementasi tidak
// mengarang nilai: field yang tidak diisi dibiarkan kosong.
type BankProfileSource interface {
	GetBankProfileConfig(ctx context.Context) (*BankProfileConfig, error)
}

// PlacementRow adalah satu penempatan pada bank lain. Sumbernya adalah penanda
// lps_placements (migrasi 000045); kolom Form 05.00 yang tidak ditandai di sana
// dinyatakan belum tersedia di form05.go. CKPN nil berarti penempatan belum diasesmen.
type PlacementRow struct {
	BranchCode       string
	CounterpartyBank string
	PlacementType    string
	Outstanding      decimal.Decimal
	Collectibility   string
	AsOf             time.Time
	// StartDate dan MaturityDate adalah sumber kolom VI (Jangka Waktu) Form 05.00;
	// StartDate nil berarti belum diisi. InterestRateAnnual adalah suku bunga TAHUNAN
	// dalam persen, sumber kolom VIII (Suku Bunga).
	StartDate          *time.Time
	MaturityDate       *time.Time
	InterestRateAnnual decimal.Decimal
	// OJKKabupatenCode adalah sandi Kabupaten/Kota bank lawan (Lampiran 03 SEOJK
	// 16/2024), sumber kolom III "Lokasi Bank" Form 05.00. Kosong berarti bank
	// belum mengisi sandinya; laporan menulis "-".
	OJKKabupatenCode string
	// Kolom Form 05.00 lain yang diisi bank lewat SQL/seed (migrasi 000106). Semua
	// nullable; kosong/nil berarti bank belum mengisi dan laporan menulis "-".
	//
	// OJKHubunganBankCode adalah sandi inline Hubungan dengan Bank (12/20), sumber
	// kolom V.
	OJKHubunganBankCode string
	// BlockedAmount adalah nominal yang diblokir/dijaminkan, sumber kolom X.
	BlockedAmount *decimal.Decimal
	// OJKAlasanDiblokirCode adalah sandi alasan diblokir, sumber kolom XI.
	OJKAlasanDiblokirCode string
	// AccruedInterestReceivable adalah pendapatan bunga yang akan diterima (akrual
	// bunga penempatan), sumber kolom XIII.
	AccruedInterestReceivable *decimal.Decimal
	// AccruedInterestPending adalah pendapatan bunga dalam penyelesaian, sumber
	// kolom XIV.
	AccruedInterestPending *decimal.Decimal
	// CounterpartyCIF adalah ID Pihak Lawan kolom XVI: nomor CIF internal bank
	// lawan (sama dengan SLIK), bukan sandi OJK.
	CounterpartyCIF string
	// OJKKlasifikasiAsetCode adalah sandi klasifikasi aset keuangan SAK EP per
	// penempatan (kolom XX, migrasi 000108), diisi bank lewat SQL/seed. Kosong berarti
	// bank belum mengisi dan laporan menulis "-", BUKAN diturunkan dari kualitas.
	OJKKlasifikasiAsetCode string
	// CustomerID adalah nasabah pihak terkait yang ditautkan ke penempatan
	// (lps_placements.customer_id, migrasi 000103). Kosong berarti penempatan belum
	// ditautkan; sumber pemetaan kolom XV Form 05.00 (Status BMPK Individu).
	CustomerID string
	// BMPKStatus adalah status BMPK pihak terkait pemilik penempatan ini, diisi perakit
	// dari SATU pemuatan laporan BMPK. Kosong berarti penempatan belum ditautkan ke
	// pihak terkait atau modul BMPK tidak tersedia; kolom XV ditulis "-".
	BMPKStatus string
	CKPN       *PlacementCKPNRow
}

// PlacementCKPNRow adalah CKPN tersimpan satu penempatan: metode asesmen dan target yang
// berlaku. Nilainya dibaca dari lps_placements (migrasi 000099).
type PlacementCKPNRow struct {
	Method       string
	RequiredCKPN decimal.Decimal
}

// PlacementDataSource menyediakan penempatan pada bank lain bank-wide.
type PlacementDataSource interface {
	ListPlacementsForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]PlacementRow, error)
}

// SavingsCustomerTypeRow adalah satu baris agregasi laporan internal jenis nasabah per
// produk (dulu diberi nomor Form 00.14 — nomor itu tidak ada di SEOJK 16/2024, lihat
// docs/CELAH-FORM-OJK.md §6): jumlah rekening dan total nominal per produk simpanan
// menurut golongan nasabah.
//
// CustomerTypeCode adalah sandi Lampiran 02 – Daftar Sandi Pihak Lawan
// (customers.ojk_pihak_lawan_code), sumber resmi "Golongan Nasabah" Form 11.00/12.00
// SEOJK 16/2024. Kosong berarti bank belum mengisi; laporan menulis "-", bukan
// menebak dari nama atau jenis identitas.
type SavingsCustomerTypeRow struct {
	ProductFamily    string
	ProductCode      string
	ProductName      string
	CustomerTypeCode string
	AccountCount     int
	TotalAmount      decimal.Decimal
}

// SavingsCustomerTypeSource menyediakan agregasi laporan internal jenis nasabah per
// produk (dulu diberi nomor Form 00.14 — nomor itu tidak ada di SEOJK 16/2024, lihat
// docs/CELAH-FORM-OJK.md §6) bank-wide. Posisi yang dibaca adalah keadaan saat ekspor
// dijalankan; sistem belum menyimpan riwayat saldo simpanan per akhir bulan.
type SavingsCustomerTypeSource interface {
	ListSavingsCustomerTypes(ctx context.Context, actor domain.Actor) ([]SavingsCustomerTypeRow, error)
}

// BankDepositRow adalah satu baris agregasi Form 13.00 "Daftar Simpanan dari Bank
// Lain": simpanan (tabungan/giro) dan deposito berjangka milik nasabah bergolongan
// bank, dikelompokkan per bank lawan, kantor, dan jenis simpanan.
//
// Bank lawan dikenali dari sandi Lampiran 02 (customers.ojk_pihak_lawan_code):
// 001 Bank Indonesia, 600 BPR, 601 BPRS, 700 Bank Umum, 701 Bank Umum Syariah, dan
// 901 Unit Usaha Syariah. Nasabah tanpa sandi bank tidak dapat dikenali.
type BankDepositRow struct {
	// BranchCode adalah sandi kantor BPR (branches.code), sumber kolom I.
	BranchCode string
	// CounterpartyCIF adalah ID Pihak Lawan kolom II: nomor CIF internal bank lawan
	// (sama dengan SLIK), bukan sandi OJK. Kosong berarti nasabah tak terbaca.
	CounterpartyCIF string
	// JenisBankCode adalah sandi Lampiran 02 kategori bank lawan, sumber kolom IV.
	JenisBankCode string
	// HubunganBankCode adalah sandi inline Hubungan dengan Bank (12/20), sumber
	// kolom VIII. Kosong berarti bank belum mengisi; laporan menulis "-".
	HubunganBankCode string
	// LocationCode adalah sandi Kabupaten/Kota bank lawan (Lampiran 03), sumber
	// kolom VI. Kosong berarti bank belum mengisi; laporan menulis "-".
	LocationCode string
	// Jenis adalah sandi Form 13.00 kolom VII: "01" Tabungan, "02" Deposito.
	Jenis string
	// AccountCount adalah jumlah rekening/kontrak satuan pada kelompok ini.
	AccountCount int
	// TotalNominal adalah total saldo tabungan/giro atau nominal penempatan deposito
	// dalam rupiah penuh, sumber kolom XI.
	TotalNominal decimal.Decimal
	// TotalBlocked adalah total saldo yang diblokir/dijaminkan (accounts.hold_balance),
	// sumber kolom XII.
	TotalBlocked decimal.Decimal
}

// BankDepositSource menyediakan agregasi Form 13.00 bank-wide. Posisi yang dibaca
// adalah keadaan saat ekspor dijalankan; sistem belum menyimpan riwayat saldo
// simpanan per akhir bulan.
type BankDepositSource interface {
	ListBankDepositsForOJK(ctx context.Context, actor domain.Actor) ([]BankDepositRow, error)
}

// ReportingOffice adalah kantor pelapor yang dipakai mengisi kolom I "Sandi Kantor"
// pada form bank-wide (09.00, 01.01). Sumbernya bank_offices (migrasi 000112):
// Code = sandi kantor yang bank pakai, Name = nama kantor. Reason terisi berarti
// kantor pelapor tidak dapat ditentukan; Sandi dan Nama dibiarkan kosong dan form
// menulis kolom I sebagai tidak tersedia, bukan dikarang.
type ReportingOffice struct {
	Sandi  string
	Nama   string
	Reason string
}

// ReportingOfficeSource menyediakan satu kantor pelapor untuk form bank-wide.
// Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini, atau bila jumlah
// kantor aktif ber-sandi bukan tepat satu, kolom I dinyatakan tidak tersedia beserta
// alasan spesifik. Aturan pemilihan ada di selectReportingOffice (repo_source.go).
type ReportingOfficeSource interface {
	ReportingOffice(ctx context.Context) (ReportingOffice, error)
}

// aktifUntukOJK melaporkan apakah kredit masih punya eksposur berjalan menurut
// statusnya. Mengikuti domain.LoanStatus.IsCKPNActive: hanya DISBURSED dan DEFAULTED.
func aktifUntukOJK(status string) bool {
	switch domain.LoanStatus(status) {
	case domain.LoanStatusDisbursed, domain.LoanStatusDefaulted:
		return true
	default:
		return false
	}
}

// KomponenKreditNPL merangkum kualitas kredit untuk rasio NPL. Tersedia=false berarti
// data kredit belum ada; pemanggil tidak boleh menafsirkan nilai nol sebagai tersedia.
type KomponenKreditNPL struct {
	KurangLancar decimal.Decimal
	Diragukan    decimal.Decimal
	Macet        decimal.Decimal
	CKPNNPL      decimal.Decimal
	TotalKredit  decimal.Decimal
	Tersedia     bool
}

// NPLDariLoanRows menjumlahkan baki debet menurut kualitas dan CKPN kredit tidak lancar.
// Kredit dengan status di luar DISBURSED/DEFAULTED dilewati karena tidak punya eksposur
// berjalan (lihat domain.LoanStatus.IsCKPNActive).
func NPLDariLoanRows(rows []LoanRow) KomponenKreditNPL {
	var k KomponenKreditNPL
	k.Tersedia = true
	for _, r := range rows {
		if !aktifUntukOJK(r.Status) {
			continue
		}
		k.TotalKredit = k.TotalKredit.Add(r.Outstanding)
		switch domain.OJKCollectibility(r.Collectibility) {
		case domain.CollectibilityKol3:
			k.KurangLancar = k.KurangLancar.Add(r.Outstanding)
		case domain.CollectibilityKol4:
			k.Diragukan = k.Diragukan.Add(r.Outstanding)
		case domain.CollectibilityKol5:
			k.Macet = k.Macet.Add(r.Outstanding)
		default:
			continue
		}
		// CKPN yang dikurangkan pada NPL neto adalah CKPN kredit tidak lancar
		// (Lampiran II hlm. 204 butir 3).
		k.CKPNNPL = k.CKPNNPL.Add(r.RequiredCKPN)
	}
	return k
}

// RataRata menghitung rata-rata aritmetika sederhana. ok=false bila tidak ada nilai,
// sehingga pemanggil menyatakan komponen belum tersedia, bukan rata-rata nol.
func RataRata(values []decimal.Decimal) (decimal.Decimal, bool) {
	if len(values) == 0 {
		return decimal.Zero, false
	}
	total := decimal.Zero
	for _, v := range values {
		total = total.Add(v)
	}
	return total.Div(decimal.NewFromInt(int64(len(values)))), true
}

// pastikanLintasCabang menegakkan kebijakan bank-wide pada lapisan data.
func pastikanLintasCabang(actor domain.Actor) error {
	if !actor.IsCrossBranch() {
		return fmt.Errorf("%w: peran %s tidak berwenang", ErrOJKBankWide, actor.Role)
	}
	return nil
}
