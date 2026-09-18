package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrDepositNotFound       = errors.New("deposito tidak ditemukan")
	ErrDepositNotActive      = errors.New("deposito tidak dalam status aktif")
	ErrDepositNotMatured     = errors.New("deposito belum jatuh tempo")
	ErrDepositAlreadyClosed  = errors.New("deposito sudah ditutup")
	ErrInvalidDepositAmount  = errors.New("nominal deposito harus positif")
	ErrInvalidDepositTerm    = errors.New("jangka waktu deposito tidak valid")
	ErrDepositProductInvalid = errors.New("produk bukan deposito berjangka yang aktif")

	// ErrDepositPenaltyExceedsProceeds: denda lebih besar dari pokok + imbal hasil
	// bersih, sehingga pencairan akan bernilai negatif.
	ErrDepositPenaltyExceedsProceeds = errors.New("denda pencairan melebihi pokok dan imbal hasil deposito")
	// ErrDepositPenaltyCOAUnavailable: akun pendapatan denda belum tersedia untuk
	// buku produk (mis. syariah tanpa akun denda baku di bagan akun).
	ErrDepositPenaltyCOAUnavailable = errors.New("akun pendapatan denda deposito belum tersedia")
)

type DepositStatus string

const (
	DepositStatusPlaced  DepositStatus = "PLACED"
	DepositStatusMatured DepositStatus = "MATURED"
	DepositStatusClosed  DepositStatus = "CLOSED"
	DepositStatusBroken  DepositStatus = "BROKEN"
)

// AROInstruction menentukan komposisi pokok saat Automatic Roll Over (ARO).
// PRINCIPAL: hanya pokok yang diperpanjang, imbal hasil tetap terakru.
// PRINCIPAL_AND_PROFIT: pokok ditambah imbal hasil bersih dikapitalisasi.
type AROInstruction string

const (
	AROInstructionNone               AROInstruction = "NONE"
	AROInstructionPrincipal          AROInstruction = "PRINCIPAL"
	AROInstructionPrincipalAndProfit AROInstruction = "PRINCIPAL_AND_PROFIT"
)

// Deposit adalah kontrak deposito berjangka. Nilai profit memakai ProfitType yang
// sama dengan jadwal kredit (INTEREST/MARGIN/BAGI_HASIL) agar laporan konsisten.
type Deposit struct {
	ID              uuid.UUID       `json:"id"`
	AccountNumber   string          `json:"account_number"`
	CustomerID      uuid.UUID       `json:"customer_id"`
	ProductID       uuid.UUID       `json:"product_id"`
	BranchID        *uuid.UUID      `json:"branch_id,omitempty"`
	BranchCode      string          `json:"branch_code,omitempty"` // kode cabang deposito; kosong = data lama
	PlacementAmount decimal.Decimal `json:"placement_amount"`
	Currency        string          `json:"currency"`
	TermMonths      int             `json:"term_months"`
	StartDate       time.Time       `json:"start_date"`
	MaturityDate    time.Time       `json:"maturity_date"`
	ProfitRate      decimal.Decimal `json:"profit_rate"` // bunga tahunan (%) atau nisbah (0-1)
	YieldRate       decimal.Decimal `json:"yield_rate"`  // proyeksi imbal hasil tahunan (%) bagi hasil syariah
	ProfitType      ProfitType      `json:"profit_type"`
	TaxRate         decimal.Decimal `json:"tax_rate"` // PPh final (%) ; 0 = bebas pajak
	ARO             bool            `json:"aro"`
	AROInstruction  AROInstruction  `json:"aro_instruction"`
	Status          DepositStatus   `json:"status"`
	AccruedProfit   decimal.Decimal `json:"accrued_profit"`
	AccruedTax      decimal.Decimal `json:"accrued_tax"`
	PaidProfit      decimal.Decimal `json:"paid_profit"`
	PaidTax         decimal.Decimal `json:"paid_tax"`
	// EarlyWithdrawalPenalty mencatat denda pencairan lebih awal yang dipotong
	// dari hasil pencairan. 0 untuk pencairan saat/setelah jatuh tempo.
	EarlyWithdrawalPenalty decimal.Decimal `json:"early_withdrawal_penalty"`
	MaturityProceeds       decimal.Decimal `json:"maturity_proceeds"`
	LastAccrualDate        *time.Time      `json:"last_accrual_date,omitempty"`
	ClosedAt               *time.Time      `json:"closed_at,omitempty"`
	CreatedAt              time.Time       `json:"created_at"`
	UpdatedAt              time.Time       `json:"updated_at"`
}

type PlaceDepositInput struct {
	CustomerID      uuid.UUID       `json:"customer_id"`
	ProductID       uuid.UUID       `json:"product_id"`
	PlacementAmount decimal.Decimal `json:"placement_amount"`
	TermMonths      int             `json:"term_months"`
	Currency        string          `json:"currency"`
	BranchCode      string          `json:"branch_code"`
	// ProfitRate menimpa bunga/nisbah produk bila diisi (> 0).
	ProfitRate decimal.Decimal `json:"profit_rate"`
	// YieldRate menimpa proyeksi imbal hasil syariah bila diisi (> 0).
	YieldRate      decimal.Decimal `json:"yield_rate"`
	ARO            bool            `json:"aro"`
	AROInstruction AROInstruction  `json:"aro_instruction"`
	StartDate      *time.Time      `json:"start_date,omitempty"`
	IdempotencyKey string          `json:"idempotency_key,omitempty"`
}

// WithdrawDepositInput adalah body permintaan pencairan. DepositID opsional bila
// identitas deposito diambil dari path.
type WithdrawDepositInput struct {
	DepositID      uuid.UUID `json:"deposit_id"`
	Reason         string    `json:"reason,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
}

type AccrueDepositInput struct {
	DepositID uuid.UUID `json:"deposit_id"`
	AsOf      time.Time `json:"as_of"`
}

type DepositRepository interface {
	Create(ctx context.Context, tx any, d *Deposit) error
	GetByID(ctx context.Context, id uuid.UUID) (*Deposit, error)
	GetByIDForUpdate(ctx context.Context, tx any, id uuid.UUID) (*Deposit, error)
	List(ctx context.Context, limit, offset int) ([]Deposit, int, error)
	ListMaturedARO(ctx context.Context, asOf time.Time) ([]Deposit, error)
	AddAccrual(ctx context.Context, tx any, id uuid.UUID, profit, tax decimal.Decimal, asOf time.Time) error
	UpdateStatus(ctx context.Context, tx any, id uuid.UUID, status DepositStatus, proceeds, paidProfit, paidTax, penalty decimal.Decimal) error
	Rollover(ctx context.Context, tx any, id uuid.UUID, newPrincipal, paidProfit, paidTax decimal.Decimal, newStart, newMaturity time.Time, resetAccrual bool) error
}

type DepositService interface {
	Place(ctx context.Context, input PlaceDepositInput, actor Actor) (*Deposit, error)
	Accrue(ctx context.Context, depositID uuid.UUID, asOf time.Time, actor Actor) (*Deposit, error)
	MatureOrWithdraw(ctx context.Context, depositID uuid.UUID, actor Actor) (*Deposit, error)
	RunARO(ctx context.Context, asOf time.Time, actor Actor) (int, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Deposit, error)
	List(ctx context.Context, page, pageSize int) ([]Deposit, int, error)
}

// depositDayBasis adalah basis hari per tahun untuk akrual harian proporsional.
const depositDayBasis = 365

// DepositDailyProfit menghitung bunga/bagi hasil satu hari secara proporsional
// terhadap pokok, lalu dibulatkan ke rupiah penuh.
//
// Konvensional (bagiHasil=false): pokok * (bunga% / 100) / 365.
// Syariah (bagiHasil=true): profitRate adalah nisbah (0-1), yieldRate adalah
// proyeksi imbal hasil tahunan (%); porsi pemilik dana = pokok * yield% * nisbah.
func DepositDailyProfit(principal, profitRate, yieldRate decimal.Decimal, bagiHasil bool) decimal.Decimal {
	if !principal.IsPositive() || !profitRate.IsPositive() {
		return decimal.Zero
	}
	hundred := decimal.NewFromInt(100)
	days := decimal.NewFromInt(depositDayBasis)

	if bagiHasil {
		if !yieldRate.IsPositive() {
			return decimal.Zero
		}
		annualRevenue := principal.Mul(yieldRate).Div(hundred)
		ownerShare := annualRevenue.Mul(profitRate)
		return RoundToRupiah(ownerShare.Div(days))
	}

	annualInterest := principal.Mul(profitRate).Div(hundred)
	return RoundToRupiah(annualInterest.Div(days))
}

// DepositDailyTax menghitung PPh final atas laba satu hari, dibulatkan ke rupiah.
func DepositDailyTax(profit, taxRate decimal.Decimal) decimal.Decimal {
	if !profit.IsPositive() || !taxRate.IsPositive() {
		return decimal.Zero
	}
	return RoundToRupiah(profit.Mul(taxRate).Div(decimal.NewFromInt(100)))
}

// DepositMaturityProceeds menghitung hak nasabah saat jatuh tempo:
// pokok + imbal hasil terakru - pajak terakru.
func DepositMaturityProceeds(principal, accruedProfit, accruedTax decimal.Decimal) decimal.Decimal {
	netProfit := accruedProfit.Sub(accruedTax)
	if netProfit.IsNegative() {
		netProfit = decimal.Zero
	}
	return RoundToRupiah(principal.Add(netProfit))
}

// DepositEarlyWithdrawalPenalty menghitung denda pencairan sebelum jatuh tempo.
//
// ASUMSI SATUAN: product.early_withdrawal_penalty_rate dan migration 000005 tidak
// memberi komentar satuan apa pun (tidak seperti rate_annual/tax_rate yang jelas
// persen). Karena itu tarif diperlakukan sebagai PERSEN DARI POKOK, bukan persen
// per tahun — interpretasi yang lazim untuk denda pencairan deposito BPR. Bila
// kelak satuan disepakati berbeda, ubah hanya di fungsi ini.
func DepositEarlyWithdrawalPenalty(principal, rate decimal.Decimal) decimal.Decimal {
	if !principal.IsPositive() || !rate.IsPositive() {
		return decimal.Zero
	}
	return RoundToRupiah(principal.Mul(rate).Div(decimal.NewFromInt(100)))
}

// DepositMaturityDate menghitung tanggal jatuh tempo dari awal kontrak.
func DepositMaturityDate(start time.Time, termMonths int) time.Time {
	if termMonths <= 0 {
		termMonths = 1
	}
	return start.AddDate(0, termMonths, 0)
}

// IsBagiHasilProfit melaporkan apakah imbal hasil memakai skema nisbah.
func IsBagiHasilProfit(pt ProfitType) bool {
	return pt == ProfitTypeBagiHasil
}
