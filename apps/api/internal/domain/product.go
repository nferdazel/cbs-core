package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	ScheduleFlat       ScheduleMethod = "FLAT"
	ScheduleAnnuity    ScheduleMethod = "ANNUITY"
	ScheduleSliding    ScheduleMethod = "SLIDING"
	ScheduleBagiHasil  ScheduleMethod = "BAGI_HASIL"
	ScheduleNone       ScheduleMethod = "NONE"
)

// PostingEvent adalah peristiwa bisnis yang menghasilkan jurnal. Produk memetakan
// tiap peristiwa ke akun COA, sehingga posting engine tidak perlu tahu detail produk.
type PostingEvent string

const (
	EventAccountOpen       PostingEvent = "ACCOUNT_OPEN"
	EventDeposit           PostingEvent = "DEPOSIT"
	EventWithdrawal        PostingEvent = "WITHDRAWAL"
	EventAccountClose      PostingEvent = "ACCOUNT_CLOSE"
	EventAccountDormant    PostingEvent = "ACCOUNT_DORMANT"
	EventInterestAccrual   PostingEvent = "INTEREST_ACCRUAL"
	EventInterestPayment   PostingEvent = "INTEREST_PAYMENT"
	EventTaxWithholding    PostingEvent = "TAX_WITHHOLDING"
	EventLoanDisbursement  PostingEvent = "LOAN_DISBURSEMENT"
	EventLoanPrincipalPay  PostingEvent = "LOAN_PRINCIPAL_PAYMENT"
	EventLoanProfitPay     PostingEvent = "LOAN_PROFIT_PAYMENT"
	EventLoanPenalty       PostingEvent = "LOAN_PENALTY"
	EventLoanWriteOff      PostingEvent = "LOAN_WRITE_OFF"
	EventLoanRecovery      PostingEvent = "LOAN_RECOVERY"
	EventPPAPProvision     PostingEvent = "PPAP_PROVISION"
	EventPPAPReversal      PostingEvent = "PPAP_REVERSAL"
	EventFeeIncome         PostingEvent = "FEE_INCOME"
	EventCashIn            PostingEvent = "CASH_IN"
	EventCashOut           PostingEvent = "CASH_OUT"
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
	ID      uuid.UUID `json:"id"`
	Code    string    `json:"code"`
	Name    string    `json:"name"`
	Family  ProductFamily `json:"family"`
	Book    COABook        `json:"book"`

	ProfitScheme   ProfitScheme   `json:"profit_scheme"`
	ScheduleMethod ScheduleMethod `json:"schedule_method"`

	// RateAnnual dalam persen per tahun untuk produk konvensional.
	RateAnnual decimal.Decimal `json:"rate_annual"`
	// ProfitSharingRatio nisbah bagi hasil pemilik dana (0-1) untuk produk syariah.
	ProfitSharingRatio decimal.Decimal `json:"profit_sharing_ratio"`

	MinAmount             decimal.Decimal `json:"min_amount"`
	MaxAmount             decimal.Decimal `json:"max_amount"`
	MinTermMonths         int             `json:"min_term_months"`
	MaxTermMonths         int             `json:"max_term_months"`
	AllowPartialPayment   bool            `json:"allow_partial_payment"`
	EarlyWithdrawalPenaltyRate decimal.Decimal `json:"early_withdrawal_penalty_rate"`

	AdminFee decimal.Decimal `json:"admin_fee"`
	TaxRate  decimal.Decimal `json:"tax_rate"`

	IsActive bool `json:"is_active"`
}

// JournalMappingRule adalah satu sisi jurnal untuk sebuah peristiwa produk.
type JournalMappingRule struct {
	Event        PostingEvent   `json:"event"`
	Direction    EntryDirection `json:"direction"`
	COACode      string         `json:"coa_code"`
	AmountSource AmountSource   `json:"amount_source"`
}

type ProductJournalMapping struct {
	ProductID uuid.UUID            `json:"product_id"`
	Rules     map[PostingEvent][]JournalMappingRule `json:"rules"`
}

type ProductRepository interface {
	List(ctx context.Context) ([]BankingProduct, error)
	GetByID(ctx context.Context, id uuid.UUID) (*BankingProduct, error)
	GetByCode(ctx context.Context, code string) (*BankingProduct, error)
	GetMapping(ctx context.Context, productID uuid.UUID, event PostingEvent) ([]JournalMappingRule, error)
}

type ProductService interface {
	ListProducts(ctx context.Context) ([]BankingProduct, error)
	GetProduct(ctx context.Context, id uuid.UUID) (*BankingProduct, error)
	GetMapping(ctx context.Context, productID uuid.UUID, event PostingEvent) ([]JournalMappingRule, error)
}
