package domain

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrProductNotFound = errors.New("produk tidak ditemukan")
	// ErrBagiHasilNisbahMissing menolak pembentukan jadwal bagi hasil tanpa nisbah.
	// Bagi hasil tidak bisa dihitung tanpa porsi yang disepakati, dan menebak nisbah
	// berarti menagih angka yang tidak pernah disepakati nasabah.
	ErrBagiHasilNisbahMissing = errors.New("nisbah bagi hasil produk (profit_sharing_ratio) belum diisi; bank harus mengisi nisbah bagi hasil produk terlebih dahulu")
	// ErrBagiHasilProjectionMissing menolak pembentukan jadwal bagi hasil tanpa
	// proyeksi pendapatan usaha. Nilai nol bukan "bagi hasil nol yang sah", melainkan
	// parameter yang belum diisi; membiarkannya menghasilkan jadwal ber-profit nol
	// yang menyesatkan.
	ErrBagiHasilProjectionMissing = errors.New("proyeksi pendapatan usaha produk (projected_revenue_rate_annual) belum diisi; bank harus mengisi proyeksi pendapatan tahunan pembiayaan bagi hasil terlebih dahulu")
	// ErrBagiHasilNisbahOutOfRange menolak nisbah yang lolos dari rentang (0,1].
	// Nilai seperti 40 hampir pasti salah satuan (maksudnya 40%, yaitu 0,4) dan
	// akan melipatgandakan proyeksi imbal hasil seratus kali.
	ErrBagiHasilNisbahOutOfRange = errors.New("nisbah bagi hasil produk (profit_sharing_ratio) harus lebih dari 0 dan maksimal 1; nisbah dinyatakan sebagai pecahan, mis. 0,4 untuk 40%")
	// ErrBagiHasilProjectionOutOfRange menolak proyeksi pendapatan tahunan di atas
	// batas wajar. Nilai seperti 1200 (maksudnya 12%) adalah salah satuan dan
	// menghasilkan proyeksi imbal hasil yang tidak masuk akal.
	ErrBagiHasilProjectionOutOfRange = errors.New("proyeksi pendapatan usaha produk (projected_revenue_rate_annual) harus lebih dari 0 dan maksimal 100; nilai dinyatakan dalam persen per tahun, mis. 12 untuk 12%")

	// ErrProductParamsEmpty menolak permintaan ubah parameter produk yang tidak
	// menyertakan satu bidang parameter pun. Payload kosong bukan perubahan yang sah.
	ErrProductParamsEmpty = errors.New("tidak ada parameter produk yang diubah; sertakan minimal satu parameter")
	// ErrProductRateNegative menolak suku bunga/margin negatif. Tarif adalah persen
	// per tahun; nilai negatif tidak punya makna pada kontrak kredit maupun simpanan.
	ErrProductRateNegative = errors.New("suku bunga/margin tahunan (rate_annual) tidak boleh negatif; nilai dinyatakan dalam persen per tahun")
	// ErrProductAdminFeeNegative menolak biaya administrasi negatif (rupiah).
	ErrProductAdminFeeNegative = errors.New("biaya administrasi (admin_fee) tidak boleh negatif; nilai dinyatakan dalam rupiah")
	// ErrProductTaxRateNegative menolak tarif pajak negatif (persen).
	ErrProductTaxRateNegative = errors.New("tarif pajak (tax_rate) tidak boleh negatif; nilai dinyatakan dalam persen")
	// ErrProductPenaltyRateNegative menolak penalti penarikan dini negatif (persen).
	ErrProductPenaltyRateNegative = errors.New("tarif penalti penarikan dini (early_withdrawal_penalty_rate) tidak boleh negatif; nilai dinyatakan dalam persen per tahun")
	// ErrProductMinAmountNegative dan ErrProductMaxAmountNegative menolak batas
	// plafon negatif (rupiah).
	ErrProductMinAmountNegative = errors.New("batas plafon minimum (min_amount) tidak boleh negatif; nilai dinyatakan dalam rupiah")
	ErrProductMaxAmountNegative = errors.New("batas plafon maksimum (max_amount) tidak boleh negatif; nilai dinyatakan dalam rupiah")
	// ErrProductAmountRange menjaga hubungan min/max: 0 pada max_amount berarti tanpa
	// batas; selain itu maksimum harus >= minimum. Ini cermin constraint database
	// chk_min_max_amount, ditegakkan lebih dulu agar pesannya jelas dan bukan galat SQL.
	ErrProductAmountRange = errors.New("batas plafon maksimum (max_amount) harus 0 (tanpa batas) atau lebih besar/sama dengan minimum (min_amount); nilai dalam rupiah")
	// ErrProductTermNegative menolak tenor negatif (bulan).
	ErrProductTermNegative = errors.New("tenor minimum/maksimum (min_term_months/max_term_months) tidak boleh negatif; nilai dinyatakan dalam bulan")
	// ErrProductTermRange menolak tenor minimum yang melebihi maksimumnya.
	ErrProductTermRange = errors.New("tenor minimum (min_term_months) tidak boleh melebihi tenor maksimum (max_term_months); nilai dinyatakan dalam bulan")

	// ErrProductRateOutOfRange/ErrProductTaxRateOutOfRange/ErrProductPenaltyRateOutOfRange
	// menolak tarif berbasis persen di atas batas wajar. Nilai seperti 1e9 (maksudnya
	// 1%) adalah salah satuan dan langsung masuk perhitungan bunga/pajak/penalti.
	ErrProductRateOutOfRange    = errors.New("suku bunga/margin tahunan (rate_annual) maksimal 100; nilai dinyatakan dalam persen per tahun, mis. 12 untuk 12%")
	ErrProductTaxRateOutOfRange = errors.New("tarif pajak (tax_rate) maksimal 100; nilai dinyatakan dalam persen, mis. 20 untuk 20%")
	// ErrProductPenaltyRateOutOfRange: penalti penarikan dini dinyatakan persen per tahun.
	ErrProductPenaltyRateOutOfRange = errors.New("tarif penalti penarikan dini (early_withdrawal_penalty_rate) maksimal 100; nilai dinyatakan dalam persen per tahun, mis. 1,5 untuk 1,5%")
	// ErrProductAdminFeeOutOfRange menolak biaya administrasi yang jelas salah satuan.
	// Batasnya sengaja longgar (rupiah); ini jaring pengaman, bukan penentu harga.
	ErrProductAdminFeeOutOfRange = errors.New("biaya administrasi (admin_fee) melebihi batas wajar; nilai dinyatakan dalam rupiah")
)

// MaxProjectedRevenueRateAnnual adalah batas atas proyeksi pendapatan tahunan usaha
// yang dibiayai, dalam persen. Di atas 100% per tahun bukan proyeksi yang wajar.
var MaxProjectedRevenueRateAnnual = decimal.NewFromInt(100)

// MaxProductPercentRate adalah batas atas tarif produk berbasis persen
// (rate_annual, tax_rate, early_withdrawal_penalty_rate), sejalan dengan batas
// proyeksi pendapatan usaha. Di atas 100% bukan tarif yang wajar dan hampir pasti
// salah satuan.
var MaxProductPercentRate = decimal.NewFromInt(100)

// MaxProductAdminFee adalah batas atas biaya administrasi (rupiah). Sengaja longgar:
// hanya jaring pengaman terhadap nilai yang jelas salah satuan, bukan penentu harga
// jasa, agar parameter produk yang sah tidak ikut tertolak.
var MaxProductAdminFee = decimal.NewFromInt(1_000_000_000_000)

// ValidateProductRateAnnual memeriksa suku bunga/margin tahunan berada pada 0..100
// (persen per tahun). Nilai negatif tetap memakai sentinel lama agar pesannya tidak
// berubah; batas atas baru mencegah nilai tak masuk akal seperti 1e9.
func ValidateProductRateAnnual(v decimal.Decimal) error {
	if v.IsNegative() {
		return ErrProductRateNegative
	}
	if v.GreaterThan(MaxProductPercentRate) {
		return ErrProductRateOutOfRange
	}
	return nil
}

// ValidateProductTaxRate memeriksa tarif pajak berada pada 0..100 (persen).
func ValidateProductTaxRate(v decimal.Decimal) error {
	if v.IsNegative() {
		return ErrProductTaxRateNegative
	}
	if v.GreaterThan(MaxProductPercentRate) {
		return ErrProductTaxRateOutOfRange
	}
	return nil
}

// ValidateProductEarlyWithdrawalPenaltyRate memeriksa penalti penarikan dini berada
// pada 0..100 (persen per tahun).
func ValidateProductEarlyWithdrawalPenaltyRate(v decimal.Decimal) error {
	if v.IsNegative() {
		return ErrProductPenaltyRateNegative
	}
	if v.GreaterThan(MaxProductPercentRate) {
		return ErrProductPenaltyRateOutOfRange
	}
	return nil
}

// ValidateProductAdminFee memeriksa biaya administrasi tidak negatif dan tidak
// melewati batas wajar (rupiah).
func ValidateProductAdminFee(v decimal.Decimal) error {
	if v.IsNegative() {
		return ErrProductAdminFeeNegative
	}
	if v.GreaterThan(MaxProductAdminFee) {
		return ErrProductAdminFeeOutOfRange
	}
	return nil
}

type COABook string

const (
	BookConventional COABook = "CONVENTIONAL"
	BookSyariah      COABook = "SYARIAH"
)

type ProductFamily string

const (
	FamilySavings        ProductFamily = "SAVINGS"
	FamilyTimeDeposit    ProductFamily = "TIME_DEPOSIT"
	FamilyLoan           ProductFamily = "LOAN"
	FamilyCurrentAccount ProductFamily = "CURRENT_ACCOUNT"
)

// ProfitScheme menentukan bagaimana imbal hasil dihitung dan dijurnal. Ini yang
// membedakan produk konvensional (bunga) dari syariah (margin/bagi hasil).
type ProfitScheme string

const (
	SchemeInterest   ProfitScheme = "INTEREST"
	SchemeMurabahah  ProfitScheme = "MURABAHAH"
	SchemeMudharabah ProfitScheme = "MUDHARABAH"
	SchemeMusyarakah ProfitScheme = "MUSYARAKAH"
	SchemeIjarah     ProfitScheme = "IJARAH"
	SchemeWadiah     ProfitScheme = "WADIAH"
)

type ScheduleMethod string

const (
	ScheduleFlat      ScheduleMethod = "FLAT"
	ScheduleAnnuity   ScheduleMethod = "ANNUITY"
	ScheduleSliding   ScheduleMethod = "SLIDING"
	ScheduleBagiHasil ScheduleMethod = "BAGI_HASIL"
	ScheduleNone      ScheduleMethod = "NONE"
)

// PostingEvent adalah peristiwa bisnis yang menghasilkan jurnal. Produk memetakan
// tiap peristiwa ke akun COA, sehingga posting engine tidak perlu tahu detail produk.
type PostingEvent string

const (
	EventAccountOpen      PostingEvent = "ACCOUNT_OPEN"
	EventDeposit          PostingEvent = "DEPOSIT"
	EventWithdrawal       PostingEvent = "WITHDRAWAL"
	EventAccountClose     PostingEvent = "ACCOUNT_CLOSE"
	EventAccountDormant   PostingEvent = "ACCOUNT_DORMANT"
	EventInterestAccrual  PostingEvent = "INTEREST_ACCRUAL"
	EventInterestPayment  PostingEvent = "INTEREST_PAYMENT"
	EventTaxWithholding   PostingEvent = "TAX_WITHHOLDING"
	EventLoanDisbursement PostingEvent = "LOAN_DISBURSEMENT"
	EventLoanPrincipalPay PostingEvent = "LOAN_PRINCIPAL_PAYMENT"
	EventLoanProfitPay    PostingEvent = "LOAN_PROFIT_PAYMENT"
	EventLoanPenalty      PostingEvent = "LOAN_PENALTY"
	EventLoanWriteOff     PostingEvent = "LOAN_WRITE_OFF"
	EventLoanRecovery     PostingEvent = "LOAN_RECOVERY"
	EventPPAPProvision    PostingEvent = "PPAP_PROVISION"
	EventPPAPReversal     PostingEvent = "PPAP_REVERSAL"
	// CKPN adalah konsep akuntansi (SAK EP) yang terpisah dari PPKA (POJK kualitas
	// aset). Peristiwanya sengaja tidak menumpang PPAP_PROVISION/PPAP_REVERSAL agar
	// rekening cadangan dan beban keduanya dapat dibandingkan tanpa tercampur.
	// Dasar jurnalnya: SEOJK No. 21/SEOJK.03/2024 Bab XII butir 12.5 dan 12.10
	// (Db. Beban kerugian penurunan nilai; Kr. CKPN).
	EventCKPNProvision PostingEvent = "CKPN_PROVISION"
	EventCKPNReversal  PostingEvent = "CKPN_REVERSAL"
	// Kerugian restrukturisasi kredit (Pasal 32 POJK No. 1 Tahun 2024 jo. SEOJK
	// No. 21/SEOJK.03/2024 PA BPR Bab 5.2 hlm. 60-61): selisih kurang antara nilai
	// tercatat dan nilai kini arus kas baru yang didiskonto pada suku bunga efektif
	// orisinal, dijurnal Db. Beban kerugian penurunan nilai; Kr. Kredit yang diberikan.
	EventLoanRestructureLoss PostingEvent = "LOAN_RESTRUCTURE_LOSS"
	EventFeeIncome           PostingEvent = "FEE_INCOME"
	EventCashIn              PostingEvent = "CASH_IN"
	EventCashOut             PostingEvent = "CASH_OUT"
)

// AmountSource menamai sisi transaksi yang dipakai sebagai nominal jurnal.
type AmountSource string

const (
	AmountPrincipal AmountSource = "PRINCIPAL"
	AmountProfit    AmountSource = "PROFIT"
	AmountFee       AmountSource = "FEE"
	AmountTax       AmountSource = "TAX"
	AmountPenalty   AmountSource = "PENALTY"
	AmountTotal     AmountSource = "TOTAL"
)

type BankingProduct struct {
	ID     uuid.UUID     `json:"id"`
	Code   string        `json:"code"`
	Name   string        `json:"name"`
	Family ProductFamily `json:"family"`
	Book   COABook       `json:"book"`

	ProfitScheme   ProfitScheme   `json:"profit_scheme"`
	ScheduleMethod ScheduleMethod `json:"schedule_method"`

	// RateAnnual dalam persen per tahun untuk produk konvensional.
	RateAnnual decimal.Decimal `json:"rate_annual"`
	// ProfitSharingRatio nisbah bagi hasil pemilik dana (0-1) untuk produk syariah.
	ProfitSharingRatio decimal.Decimal `json:"profit_sharing_ratio"`
	// ProjectedRevenueRateAnnual adalah PROYEKSI pendapatan usaha yang dibiayai,
	// dalam persen per tahun, yang menjadi dasar tarif ekuivalen jadwal angsuran akad
	// bagi hasil (mudharabah/musyarakah): tarif ekuivalen = nisbah x nilai ini.
	//
	// Ini bukan angka pasti: pendapatan/laba usaha baru diketahui saat realisasi,
	// sehingga jadwal hanya proyeksi dan WAJIB disesuaikan saat bagi hasil aktual
	// dihitung. Nilai 0 berarti parameter belum diisi dan jadwal bagi hasil ditolak.
	ProjectedRevenueRateAnnual decimal.Decimal `json:"projected_revenue_rate_annual"`

	MinAmount                  decimal.Decimal `json:"min_amount"`
	MaxAmount                  decimal.Decimal `json:"max_amount"`
	MinTermMonths              int             `json:"min_term_months"`
	MaxTermMonths              int             `json:"max_term_months"`
	AllowPartialPayment        bool            `json:"allow_partial_payment"`
	EarlyWithdrawalPenaltyRate decimal.Decimal `json:"early_withdrawal_penalty_rate"`

	AdminFee decimal.Decimal `json:"admin_fee"`
	TaxRate  decimal.Decimal `json:"tax_rate"`

	IsActive bool `json:"is_active"`
}

// UpdateProductParamsInput memuat parameter produk yang boleh diubah lewat API.
//
// Sengaja TIDAK memuat identitas produk (id, code, name, family, book) maupun jenis
// perhitungannya (profit_scheme, schedule_method) dan lifecycle (is_active): bidang
// itu menentukan "produk apa ini" dan tidak boleh bergeser di bawah kaki transaksi
// yang sudah ada. allow_partial_payment juga tidak diubah di sini karena mengubah
// perilaku pembayaran yang sudah berjalan, bukan parameter tarif.
//
// Semua bidang memakai pointer agar "tidak dikirim" (nil) berbeda dari "diisi nol",
// sehingga payload parsing tidak menimpa nilai yang tidak disebut pemanggil.
type UpdateProductParamsInput struct {
	RateAnnual                 *decimal.Decimal `json:"rate_annual,omitempty"`
	ProfitSharingRatio         *decimal.Decimal `json:"profit_sharing_ratio,omitempty"`
	ProjectedRevenueRateAnnual *decimal.Decimal `json:"projected_revenue_rate_annual,omitempty"`
	MinAmount                  *decimal.Decimal `json:"min_amount,omitempty"`
	MaxAmount                  *decimal.Decimal `json:"max_amount,omitempty"`
	MinTermMonths              *int             `json:"min_term_months,omitempty"`
	MaxTermMonths              *int             `json:"max_term_months,omitempty"`
	AdminFee                   *decimal.Decimal `json:"admin_fee,omitempty"`
	TaxRate                    *decimal.Decimal `json:"tax_rate,omitempty"`
	EarlyWithdrawalPenaltyRate *decimal.Decimal `json:"early_withdrawal_penalty_rate,omitempty"`
}

// IsEmpty melaporkan tidak ada satu parameter pun yang disertakan.
func (in UpdateProductParamsInput) IsEmpty() bool {
	return in.RateAnnual == nil &&
		in.ProfitSharingRatio == nil &&
		in.ProjectedRevenueRateAnnual == nil &&
		in.MinAmount == nil &&
		in.MaxAmount == nil &&
		in.MinTermMonths == nil &&
		in.MaxTermMonths == nil &&
		in.AdminFee == nil &&
		in.TaxRate == nil &&
		in.EarlyWithdrawalPenaltyRate == nil
}

// JournalMappingRule adalah satu sisi jurnal untuk sebuah peristiwa produk.
type JournalMappingRule struct {
	Event        PostingEvent   `json:"event"`
	Direction    EntryDirection `json:"direction"`
	COACode      string         `json:"coa_code"`
	AmountSource AmountSource   `json:"amount_source"`
}

type ProductJournalMapping struct {
	ProductID uuid.UUID                             `json:"product_id"`
	Rules     map[PostingEvent][]JournalMappingRule `json:"rules"`
}

type ProductRepository interface {
	List(ctx context.Context) ([]BankingProduct, error)
	GetByID(ctx context.Context, id uuid.UUID) (*BankingProduct, error)
	GetByCode(ctx context.Context, code string) (*BankingProduct, error)
	GetMapping(ctx context.Context, productID uuid.UUID, event PostingEvent) ([]JournalMappingRule, error)
}

// ProductParamRepository adalah jalur tulis parameter produk. Dipisahkan dari
// ProductRepository yang murni baca agar layanan lain yang hanya membaca produk
// tidak ikut terpaksa menyediakan operasi tulis pada test gandanya.
type ProductParamRepository interface {
	ProductRepository
	// GetByCodeTx membaca produk di dalam transaksi penulisan dan mengunci barisnya
	// (SELECT ... FOR UPDATE), sehingga nilai before yang dibandingkan dan diaudit
	// mencerminkan baris yang benar-benar ditimpa, bukan potret di luar transaksi.
	GetByCodeTx(ctx context.Context, tx any, code string) (*BankingProduct, error)
	// UpdateParamsTx menyimpan parameter produk. Bila tx diberikan, penulisan ikut
	// transaksi bisnis sehingga perubahan dan auditnya sukses/gagal bersama-sama.
	UpdateParamsTx(ctx context.Context, tx any, p *BankingProduct) error
}

type ProductService interface {
	ListProducts(ctx context.Context) ([]BankingProduct, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*BankingProduct, error)
	GetMapping(ctx context.Context, productID uuid.UUID, event PostingEvent) ([]JournalMappingRule, error)
	// UpdateParams mengubah parameter produk berdasarkan kode. Identitas produk tidak
	// dapat diubah dan produk tidak dapat dihapus lewat jalur ini.
	UpdateParams(ctx context.Context, code string, input UpdateProductParamsInput, actor Actor) (*BankingProduct, error)
}
