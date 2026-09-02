package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type DocumentType string

const (
	DocTypeDepositSlip       DocumentType = "DEPOSIT_SLIP"
	DocTypeWithdrawalSlip    DocumentType = "WITHDRAWAL_SLIP"
	DocTypeLoanAgreement     DocumentType = "LOAN_AGREEMENT"
	DocTypePassbookPrint     DocumentType = "PASSBOOK_PRINT"
	DocTypeThermalCollection DocumentType = "THERMAL_COLLECTION"
)

// Printable Deposit/Withdrawal Receipt DTO
type TransactionReceiptData struct {
	BankName        string          `json:"bank_name"`
	BranchName      string          `json:"branch_name"`
	ReferenceNumber string          `json:"reference_number"`
	TransactionType string          `json:"transaction_type"`
	AccountNumber   string          `json:"account_number"`
	CustomerName    string          `json:"customer_name"`
	Amount          decimal.Decimal `json:"amount"`
	AmountInWords   string          `json:"amount_in_words"`
	Description     string          `json:"description"`
	TellerID        string          `json:"teller_id"`
	TellerName      string          `json:"teller_name"`
	BalanceAfter    decimal.Decimal `json:"balance_after"`
	PrintedAt       time.Time       `json:"printed_at"`
}

// Printable Loan Agreement DTO
type LoanAgreementData struct {
	BankName            string          `json:"bank_name"`
	BranchName          string          `json:"branch_name"`
	LoanNumber          string          `json:"loan_number"`
	CustomerName        string          `json:"customer_name"`
	IDCardNumber        string          `json:"id_card_number"`
	Address             string          `json:"address"`
	LoanType            LoanType        `json:"loan_type"`
	PrincipalAmount     decimal.Decimal `json:"principal_amount"`
	InterestRateAnnual  decimal.Decimal `json:"interest_rate_annual"`
	MarginAmount        decimal.Decimal `json:"margin_amount"`
	TotalPayable        decimal.Decimal `json:"total_payable"`
	TermMonths          int             `json:"term_months"`
	MonthlyInstallment  decimal.Decimal `json:"monthly_installment"`
	DisbursementAccount string          `json:"disbursement_account"`
	ApprovedAt          time.Time       `json:"approved_at"`
	Schedules           []LoanSchedule  `json:"schedules"`
}

// Passbook Line Entry for Dot-Matrix Printer (PLQ-20/30)
type PassbookLine struct {
	LineNumber    int             `json:"line_number"`
	Date          string          `json:"date"`
	TxCode        string          `json:"tx_code"`
	DebitAmount   decimal.Decimal `json:"debit_amount"`
	CreditAmount  decimal.Decimal `json:"credit_amount"`
	Balance       decimal.Decimal `json:"balance"`
	TellerID      string          `json:"teller_id"`
}

type DocumentService interface {
	GenerateDepositSlipHTML(ctx context.Context, refNo string) (string, error)
	GenerateWithdrawalSlipHTML(ctx context.Context, refNo string) (string, error)
	GenerateLoanAgreementHTML(ctx context.Context, loanID uuid.UUID) (string, error)
	GenerateThermalReceiptText(ctx context.Context, receiptNo string) (string, error)
}
