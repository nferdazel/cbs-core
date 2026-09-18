package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ProductEventPoster adalah bagian ProductPoster yang dibutuhkan service deposito.
// Dibuat sebagai interface agar service dapat diuji tanpa mesin posting/database.
type ProductEventPoster interface {
	PostEventTx(ctx context.Context, tx any, product *domain.BankingProduct, event domain.PostingEvent, amounts Amounts, meta PostingMeta) (*domain.JournalEntry, error)
}

var defaultDepositTaxRate = decimal.NewFromInt(20)

const (
	depositTaxRateConfigKey  = "tax.deposit.rate"
	mudharabahYieldConfigKey = "deposit.mudharabah.yield_annual"

	// depositPenaltyCOAConfigKey menimpa akun pendapatan denda pencairan.
	// Migrasi 000005 menyediakan 40500 "Pendapatan Denda" hanya untuk buku
	// konvensional (buku syariah tidak punya akun denda baku), jadi fallback
	// konvensional 40500 dan syariah wajib diisi lewat konfigurasi.
	depositPenaltyCOAConfigKey   = "deposit.penalty.income.coa"
	defaultDepositPenaltyCOAConv = "40500"
)

type depositService struct {
	db           *sql.DB
	depositRepo  domain.DepositRepository
	productRepo  domain.ProductRepository
	accountRepo  domain.AccountRepository
	ledgerRepo   domain.LedgerRepository
	customerRepo domain.CustomerRepository
	branchRepo   domain.BranchRepository
	numbering    domain.AccountNumberGenerator
	poster       ProductEventPoster
	posting      domain.PostingService
	resolver     domain.AccountResolver
	configSvc    domain.SystemConfigService
	auditRepo    domain.AuditRepository
}

func NewDepositService(
	db *sql.DB,
	depositRepo domain.DepositRepository,
	productRepo domain.ProductRepository,
	accountRepo domain.AccountRepository,
	ledgerRepo domain.LedgerRepository,
	customerRepo domain.CustomerRepository,
	branchRepo domain.BranchRepository,
	numbering domain.AccountNumberGenerator,
	poster ProductEventPoster,
	posting domain.PostingService,
	resolver domain.AccountResolver,
	configSvc domain.SystemConfigService,
	auditSinks ...domain.AuditRepository,
) domain.DepositService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &depositService{
		db:           db,
		depositRepo:  depositRepo,
		productRepo:  productRepo,
		accountRepo:  accountRepo,
		ledgerRepo:   ledgerRepo,
		customerRepo: customerRepo,
		branchRepo:   branchRepo,
		numbering:    numbering,
		poster:       poster,
		posting:      posting,
		resolver:     resolver,
		configSvc:    configSvc,
		auditRepo:    auditRepo,
	}
}

func (s *depositService) Place(ctx context.Context, input domain.PlaceDepositInput, actor domain.Actor) (*domain.Deposit, error) {
	if input.PlacementAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidDepositAmount
	}
	if input.TermMonths <= 0 {
		return nil, domain.ErrInvalidDepositTerm
	}
	if input.Currency == "" {
		input.Currency = "IDR"
	}

	product, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Family != domain.FamilyTimeDeposit || !product.IsActive {
		return nil, domain.ErrDepositProductInvalid
	}
	if input.PlacementAmount.LessThan(product.MinAmount) {
		return nil, fmt.Errorf("nominal di bawah minimum produk %s (%s)", product.Code, product.MinAmount.String())
	}
	if product.MaxAmount.IsPositive() && input.PlacementAmount.GreaterThan(product.MaxAmount) {
		return nil, fmt.Errorf("nominal di atas maksimum produk %s (%s)", product.Code, product.MaxAmount.String())
	}
	if input.TermMonths < product.MinTermMonths || (product.MaxTermMonths > 0 && input.TermMonths > product.MaxTermMonths) {
		return nil, fmt.Errorf("jangka waktu di luar rentang produk %s (%d-%d bulan)", product.Code, product.MinTermMonths, product.MaxTermMonths)
	}

	customer, err := s.customerRepo.GetByID(ctx, input.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("nasabah tidak valid: %w", err)
	}
	if customer.Status != domain.CustomerStatusActive {
		return nil, domain.ErrAccountInactive
	}

	branchCode := actor.BranchCode
	if branchCode == "" {
		return nil, errors.New("kode cabang aktor wajib diisi")
	}
	actorBranch, err := s.branchRepo.GetByCode(ctx, branchCode)
	if err != nil {
		return nil, fmt.Errorf("cabang tidak valid: %w", err)
	}
	// Penegakan kepemilikan cabang: nasabah di cabang lain ditolak. Nasabah tanpa
	// cabang (data pra-migrasi) dibiarkan agar operasional tidak terblokir.
	if !actor.IsCrossBranch() && customer.BranchID != nil && *customer.BranchID != actorBranch.ID {
		return nil, domain.ErrCrossBranchAccess
	}
	// Deposito mengikuti cabang nasabah. Aktor lintas cabang yang melayani nasabah
	// cabang lain memakai cabang nasabah itu, bukan cabang aktornya.
	branch := actorBranch
	if customer.BranchID != nil && *customer.BranchID != actorBranch.ID {
		customerBranch, err := s.branchRepo.GetByID(ctx, *customer.BranchID)
		if err != nil {
			return nil, fmt.Errorf("cabang nasabah tidak valid: %w", err)
		}
		branch = customerBranch
	}
	if !branch.IsActive {
		return nil, fmt.Errorf("cabang %s sedang tidak aktif", branch.Code)
	}

	coaCode, err := liabilityCOAForProduct(product)
	if err != nil {
		return nil, err
	}
	coa, err := s.ledgerRepo.GetCOAByCode(ctx, coaCode)
	if err != nil {
		return nil, fmt.Errorf("akun kewajiban deposito %s tidak ditemukan: %w", coaCode, err)
	}

	start := time.Now().UTC()
	if input.StartDate != nil && !input.StartDate.IsZero() {
		start = input.StartDate.UTC()
	}
	start = depositDateOnly(start)
	maturity := domain.DepositMaturityDate(start, input.TermMonths)

	profitType, profitRate, yieldRate, taxRate := depositTerms(ctx, s.configSvc, product, input)

	instruction := input.AROInstruction
	if !input.ARO {
		instruction = domain.AROInstructionNone
	} else if instruction == "" || instruction == domain.AROInstructionNone {
		instruction = domain.AROInstructionPrincipal
	}
	if instruction != domain.AROInstructionPrincipal && instruction != domain.AROInstructionPrincipalAndProfit && instruction != domain.AROInstructionNone {
		return nil, errors.New("instruksi ARO tidak dikenal")
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	accountNumber, err := s.numbering.NextAccountNumber(ctx, tx, product, branch.Code)
	if err != nil {
		return nil, err
	}

	// Rekening deposito nasabah. Saldo jurnal kewajiban tetap di akun kontrol GL
	// karena posting memakai pemetaan produk; baris akun ini menautkan bilyet ke
	// nasabah dan menjadi nomor rekening yang sah.
	account := &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    accountNumber,
		CustomerID:       &customer.ID,
		ProductID:        &product.ID,
		BranchID:         &branch.ID,
		COAID:            coa.ID,
		AccountType:      accountTypeForFamily(product.Family),
		Currency:         input.Currency,
		Balance:          decimal.Zero,
		AvailableBalance: decimal.Zero,
		HoldBalance:      decimal.Zero,
		Status:           domain.AccountStatusActive,
		Version:          1,
		OpenedAt:         &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := s.accountRepo.CreateTx(ctx, tx, account); err != nil {
		return nil, fmt.Errorf("membuat rekening deposito: %w", err)
	}

	deposit := &domain.Deposit{
		ID:              uuid.New(),
		AccountNumber:   accountNumber,
		CustomerID:      customer.ID,
		ProductID:       product.ID,
		BranchID:        &branch.ID,
		PlacementAmount: input.PlacementAmount,
		Currency:        input.Currency,
		TermMonths:      input.TermMonths,
		StartDate:       start,
		MaturityDate:    maturity,
		ProfitRate:      profitRate,
		YieldRate:       yieldRate,
		ProfitType:      profitType,
		TaxRate:         taxRate,
		ARO:             input.ARO,
		AROInstruction:  instruction,
		Status:          domain.DepositStatusPlaced,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.depositRepo.Create(ctx, tx, deposit); err != nil {
		return nil, err
	}

	idemKey := input.IdempotencyKey
	if idemKey == "" {
		idemKey = "DEP-PLACE-" + deposit.ID.String()
	}
	_, err = s.poster.PostEventTx(ctx, tx, product, domain.EventDeposit, Amounts{
		Principal: input.PlacementAmount,
		Total:     input.PlacementAmount,
	}, PostingMeta{
		TransactionType: domain.TxTypeDeposit,
		Description:     fmt.Sprintf("Penempatan deposito %s %s", product.Name, accountNumber),
		IdempotencyKey:  idemKey,
		CreatedBy:       actor.DisplayName(),
		BranchCode:      actor.BranchCode,
	})
	if err != nil {
		return nil, fmt.Errorf("jurnal penempatan deposito: %w", err)
	}

	if err := writeAudit(ctx, s.auditRepo, tx, actor, "PLACE_DEPOSIT", "deposit", deposit.ID.String(), map[string]any{
		"account_number": accountNumber,
		"product":        product.Code,
		"amount":         input.PlacementAmount.String(),
		"term_months":    input.TermMonths,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return deposit, nil
}

// depositTerms menentukan jenis imbal hasil, tarif/nisbah, proyeksi yield syariah,
// dan tarif pajak. Pajak hanya berlaku untuk buku konvensional dan diambil dari
// produk bila diisi, jika tidak dari konfigurasi database (default 20%).
func depositTerms(ctx context.Context, cfg domain.SystemConfigService, product *domain.BankingProduct, input domain.PlaceDepositInput) (domain.ProfitType, decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	profitType := domain.ProfitTypeInterest
	switch product.ProfitScheme {
	case domain.SchemeMurabahah:
		profitType = domain.ProfitTypeMargin
	case domain.SchemeMudharabah, domain.SchemeMusyarakah:
		profitType = domain.ProfitTypeBagiHasil
	}

	profitRate := product.RateAnnual
	yieldRate := decimal.Zero
	if profitType == domain.ProfitTypeBagiHasil {
		// Nisbah adalah porsi pemilik dana; imbal hasil dihitung dari yield proyeksi.
		profitRate = product.ProfitSharingRatio
		if cfg != nil {
			yieldRate = cfg.GetDecimal(ctx, mudharabahYieldConfigKey, decimal.Zero)
		}
	}
	if input.ProfitRate.IsPositive() {
		profitRate = input.ProfitRate
	}
	if input.YieldRate.IsPositive() {
		yieldRate = input.YieldRate
	}

	taxRate := decimal.Zero
	if product.Book != domain.BookSyariah {
		taxRate = product.TaxRate
		if !taxRate.IsPositive() {
			taxRate = defaultDepositTaxRate
			if cfg != nil {
				taxRate = cfg.GetDecimal(ctx, depositTaxRateConfigKey, defaultDepositTaxRate)
			}
		}
	}
	return profitType, profitRate, yieldRate, taxRate
}

// depositDailyAccrual adalah perhitungan murni akrual satu hari. Dipisah agar
// dapat diuji tanpa database dan dipakai konsisten oleh Accrue.
func depositDailyAccrual(d *domain.Deposit) (profit, tax decimal.Decimal) {
	profit = domain.DepositDailyProfit(d.PlacementAmount, d.ProfitRate, d.YieldRate, domain.IsBagiHasilProfit(d.ProfitType))
	tax = domain.DepositDailyTax(profit, d.TaxRate)
	return profit, tax
}

// depositPayoutAmounts menghitung hak bersih nasabah saat pencairan.
func depositPayoutAmounts(d *domain.Deposit) (netProfit, tax, proceeds decimal.Decimal) {
	tax = d.AccruedTax
	netProfit = d.AccruedProfit.Sub(d.AccruedTax)
	if netProfit.IsNegative() {
		netProfit = decimal.Zero
	}
	proceeds = domain.DepositMaturityProceeds(d.PlacementAmount, d.AccruedProfit, d.AccruedTax)
	return netProfit, tax, proceeds
}

// depositIsEarly melaporkan apakah asOf masih sebelum tanggal jatuh tempo.
func depositIsEarly(asOf, maturity time.Time) bool {
	return depositDateOnly(asOf).Before(depositDateOnly(maturity))
}

// depositWithdrawalAmounts menerapkan denda pada hak pencairan. Proceeds tidak
// pernah negatif: bila denda melebihi pokok + imbal hasil bersih, kembalikan
// ErrDepositPenaltyExceedsProceeds. appliedPenalty di-nol-kan saat tidak valid.
func depositWithdrawalAmounts(d *domain.Deposit, penalty decimal.Decimal) (netProfit, tax, appliedPenalty, proceeds decimal.Decimal, err error) {
	netProfit, tax, proceeds = depositPayoutAmounts(d)
	appliedPenalty = penalty
	if appliedPenalty.IsNegative() {
		appliedPenalty = decimal.Zero
	}
	if appliedPenalty.GreaterThan(proceeds) {
		return netProfit, tax, decimal.Zero, decimal.Zero, domain.ErrDepositPenaltyExceedsProceeds
	}
	proceeds = proceeds.Sub(appliedPenalty)
	return netProfit, tax, appliedPenalty, proceeds, nil
}

// depositPenaltySplit membagi denda: sebesar mungkin diserap porsi imbal hasil
// bersih lebih dulu, sisanya mengurangi pokok. Ini menjaga setiap jurnal tetap
// seimbang tanpa nilai kas negatif.
func depositPenaltySplit(netProfit, penalty decimal.Decimal) (fromProfit, fromPrincipal decimal.Decimal) {
	if penalty.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero
	}
	if penalty.LessThanOrEqual(netProfit) {
		return penalty, decimal.Zero
	}
	return netProfit, penalty.Sub(netProfit)
}

// depositRolloverTerms menghitung syarat kontrak ARO berikutnya tanpa menyentuh
// database: pokok, awal baru (tanggal jatuh tempo lama), jatuh tempo berikutnya,
// dan apakah imbal hasil dikapitalisasi ke pokok.
func depositRolloverTerms(d *domain.Deposit, netProfit decimal.Decimal) (newPrincipal decimal.Decimal, newStart, newMaturity time.Time, capitalise bool) {
	newStart = depositDateOnly(d.MaturityDate)
	newMaturity = domain.DepositMaturityDate(newStart, d.TermMonths)
	newPrincipal = d.PlacementAmount
	if d.AROInstruction == domain.AROInstructionPrincipalAndProfit && netProfit.IsPositive() {
		newPrincipal = d.PlacementAmount.Add(netProfit)
		capitalise = true
	}
	return newPrincipal, newStart, newMaturity, capitalise
}

func (s *depositService) Accrue(ctx context.Context, depositID uuid.UUID, asOf time.Time, actor domain.Actor) (*domain.Deposit, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	dep, err := s.depositRepo.GetByIDForUpdate(ctx, tx, depositID)
	if err != nil {
		return nil, err
	}
	if dep.Status != domain.DepositStatusPlaced && dep.Status != domain.DepositStatusMatured {
		return nil, domain.ErrDepositNotActive
	}
	// Penegakan kepemilikan cabang. Deposito tanpa cabang (data pra-migrasi)
	// dibiarkan lewat; batch memakai RoleSystem yang lintas cabang.
	if !actor.CanAccessBranch(dep.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}

	accrualDate := depositDateOnly(asOf)
	// Idempotent per hari: hari yang sudah diakrual tidak diposting ulang.
	if dep.LastAccrualDate != nil && !accrualDate.After(depositDateOnly(*dep.LastAccrualDate)) {
		return dep, nil
	}
	if accrualDate.Before(depositDateOnly(dep.StartDate)) {
		return dep, nil
	}

	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil {
		return nil, err
	}

	profit, tax := depositDailyAccrual(dep)
	if profit.IsPositive() {
		_, err = s.poster.PostEventTx(ctx, tx, product, domain.EventInterestAccrual, Amounts{
			Profit: profit,
			Total:  profit,
		}, PostingMeta{
			TransactionType: domain.TxTypeInterestAccrual,
			Description:     fmt.Sprintf("Akrual imbal hasil deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-ACCR-" + dep.ID.String() + "-" + accrualDate.Format("20060102"),
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		})
		if err != nil {
			return nil, fmt.Errorf("jurnal akrual deposito: %w", err)
		}
	}

	if err := s.depositRepo.AddAccrual(ctx, tx, dep.ID, profit, tax, accrualDate); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	dep.AccruedProfit = dep.AccruedProfit.Add(profit)
	dep.AccruedTax = dep.AccruedTax.Add(tax)
	dep.LastAccrualDate = &accrualDate
	return dep, nil
}

func (s *depositService) MatureOrWithdraw(ctx context.Context, depositID uuid.UUID, actor domain.Actor) (*domain.Deposit, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	dep, err := s.depositRepo.GetByIDForUpdate(ctx, tx, depositID)
	if err != nil {
		return nil, err
	}
	if dep.Status == domain.DepositStatusClosed || dep.Status == domain.DepositStatusBroken {
		return nil, domain.ErrDepositAlreadyClosed
	}
	// Penegakan kepemilikan cabang. Deposito tanpa cabang (data pra-migrasi)
	// dibiarkan lewat agar pencairan data lama tidak terblokir.
	if !actor.CanAccessBranch(dep.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}

	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	// Sebelum jatuh tempo kontrak dicairkan lebih awal (BROKEN) dan dikenai denda
	// sesuai tarif produk. Denda hanya memotong hasil pencairan, tidak mengubah
	// akrual imbal hasil yang sudah diakui.
	early := depositIsEarly(now, dep.MaturityDate)
	penalty := decimal.Zero
	if early {
		penalty = domain.DepositEarlyWithdrawalPenalty(dep.PlacementAmount, product.EarlyWithdrawalPenaltyRate)
	}
	netProfit, tax, penalty, proceeds, err := depositWithdrawalAmounts(dep, penalty)
	if err != nil {
		return nil, err
	}
	fromProfit, fromPrincipal := depositPenaltySplit(netProfit, penalty)

	status := domain.DepositStatusClosed
	if early {
		status = domain.DepositStatusBroken
	}

	// 1. Potong PPh final: kurangi utang bunga, akui utang pajak (20500).
	if tax.IsPositive() {
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventTaxWithholding, Amounts{
			Tax: tax, Total: tax,
		}, PostingMeta{
			TransactionType: domain.TxTypeWithdrawal,
			Description:     fmt.Sprintf("Potongan PPh final deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-TAX",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}); err != nil {
			return nil, fmt.Errorf("jurnal potongan pajak deposito: %w", err)
		}
	}

	// 2. Akui pendapatan denda: kurangi kewajiban imbal hasil/pokok sebesar denda
	// yang tidak dibayarkan ke nasabah.
	if penalty.IsPositive() {
		if err := s.postDepositPenalty(ctx, tx, product, dep, actor, fromProfit, fromPrincipal, penalty); err != nil {
			return nil, err
		}
	}

	// 3. Bayar imbal hasil bersih setelah denda porsi bunga.
	if interestPaid := netProfit.Sub(fromProfit); interestPaid.IsPositive() {
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventInterestPayment, Amounts{
			Profit: interestPaid, Total: interestPaid,
		}, PostingMeta{
			TransactionType: domain.TxTypeWithdrawal,
			Description:     fmt.Sprintf("Pembayaran imbal hasil deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-INT",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}); err != nil {
			return nil, fmt.Errorf("jurnal pembayaran imbal hasil deposito: %w", err)
		}
	}

	// 4. Bayar pokok setelah denda porsi pokok (bila denda melebihi bunga bersih).
	if principalPaid := dep.PlacementAmount.Sub(fromPrincipal); principalPaid.IsPositive() {
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventWithdrawal, Amounts{
			Principal: principalPaid, Total: principalPaid,
		}, PostingMeta{
			TransactionType: domain.TxTypeWithdrawal,
			Description:     fmt.Sprintf("Pencairan pokok deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-PRIN",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}); err != nil {
			return nil, fmt.Errorf("jurnal pencairan pokok deposito: %w", err)
		}
	}

	if err := s.depositRepo.UpdateStatus(ctx, tx, dep.ID, status, proceeds, netProfit, tax, penalty); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "WITHDRAW_DEPOSIT", "deposit", dep.ID.String(), map[string]any{
		"status":   status,
		"proceeds": proceeds.String(),
		"tax":      tax.String(),
		"penalty":  penalty.String(),
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	dep.Status = status
	dep.MaturityProceeds = proceeds
	dep.PaidProfit = netProfit
	dep.PaidTax = tax
	dep.EarlyWithdrawalPenalty = penalty
	dep.ClosedAt = &now
	return dep, nil
}

func (s *depositService) RunARO(ctx context.Context, asOf time.Time, actor domain.Actor) (int, error) {
	matured, err := s.depositRepo.ListMaturedARO(ctx, asOf)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range matured {
		if err := s.rolloverOne(ctx, matured[i].ID, actor); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// rolloverOne memperpanjang satu kontrak deposito ber-ARO. Tidak ada dana keluar:
// kewajiban pokok tetap di GL. Bila imbal hasil dikapitalisasi, saldo kewajiban
// imbal hasil direklas ke kewajiban pokok lewat posting engine.
func (s *depositService) rolloverOne(ctx context.Context, depositID uuid.UUID, actor domain.Actor) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	dep, err := s.depositRepo.GetByIDForUpdate(ctx, tx, depositID)
	if err != nil {
		return err
	}
	if !dep.ARO || dep.Status == domain.DepositStatusClosed || dep.Status == domain.DepositStatusBroken {
		return nil
	}

	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil {
		return err
	}

	netProfit, tax, _ := depositPayoutAmounts(dep)
	newPrincipal, start, maturity, capitalise := depositRolloverTerms(dep, netProfit)
	var paidProfit, paidTax decimal.Decimal

	if capitalise {
		if tax.IsPositive() {
			if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventTaxWithholding, Amounts{
				Tax: tax, Total: tax,
			}, PostingMeta{
				TransactionType: domain.TxTypeAdjustment,
				Description:     fmt.Sprintf("Potongan PPh final ARO deposito %s", dep.AccountNumber),
				IdempotencyKey:  "DEP-ARO-" + dep.ID.String() + "-" + dep.MaturityDate.Format("20060102") + "-TAX",
				CreatedBy:       actor.DisplayName(),
				BranchCode:      actor.BranchCode,
			}); err != nil {
				return fmt.Errorf("jurnal potongan pajak ARO deposito: %w", err)
			}
		}

		payableAccount, err := s.mappingAccount(ctx, tx, product.ID, domain.EventInterestAccrual, domain.DirectionCredit)
		if err != nil {
			return err
		}
		liabilityAccount, err := s.mappingAccount(ctx, tx, product.ID, domain.EventDeposit, domain.DirectionCredit)
		if err != nil {
			return err
		}
		if _, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
			TransactionType: domain.TxTypeAdjustment,
			Description:     fmt.Sprintf("Kapitalisasi imbal hasil ARO deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-ARO-" + dep.ID.String() + "-" + dep.MaturityDate.Format("20060102") + "-CAP",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
			Lines: []domain.PostingLine{
				{AccountNumber: payableAccount, Direction: domain.DirectionDebit, Amount: netProfit, Description: "Reklas imbal hasil ke pokok"},
				{AccountNumber: liabilityAccount, Direction: domain.DirectionCredit, Amount: netProfit, Description: "Perpanjangan pokok ARO"},
			},
		}); err != nil {
			return fmt.Errorf("jurnal kapitalisasi ARO deposito: %w", err)
		}

		paidProfit = netProfit
		paidTax = tax
	}

	if err := s.depositRepo.Rollover(ctx, tx, dep.ID, newPrincipal, paidProfit, paidTax, start, maturity, capitalise); err != nil {
		return err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "ROLLOVER_DEPOSIT", "deposit", dep.ID.String(), map[string]any{
		"new_principal": newPrincipal.String(),
		"new_maturity":  maturity.Format("2006-01-02"),
		"capitalised":   capitalise,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// mappingAccount mengambil nomor akun GL untuk sebuah peristiwa produk pada arah
// tertentu. Dipakai kapitalisasi ARO agar tidak menebak kode COA di service.
func (s *depositService) mappingAccount(ctx context.Context, tx any, productID uuid.UUID, event domain.PostingEvent, dir domain.EntryDirection) (string, error) {
	rules, err := s.productRepo.GetMapping(ctx, productID, event)
	if err != nil {
		return "", err
	}
	for _, rule := range rules {
		if rule.Direction == dir {
			return s.resolver.ResolveGLAccount(ctx, tx, rule.COACode)
		}
	}
	return "", fmt.Errorf("produk tidak punya pemetaan %s arah %s", event, dir)
}

// depositPenaltyCOA menentukan kode COA pendapatan denda. Urutan: override
// system_config deposit.penalty.income.coa, lalu fallback 40500 "Pendapatan
// Denda" (buku konvensional, migrasi 000005). Buku syariah tidak punya akun
// denda baku, sehingga mengembalikan string kosong bila tidak dikonfigurasi.
func (s *depositService) depositPenaltyCOA(ctx context.Context, product *domain.BankingProduct) string {
	fallback := ""
	if product.Book == domain.BookConventional {
		fallback = defaultDepositPenaltyCOAConv
	}
	if s.configSvc == nil {
		return fallback
	}
	code := strings.TrimSpace(s.configSvc.GetString(ctx, depositPenaltyCOAConfigKey, fallback))
	if code == "" {
		code = fallback
	}
	return code
}

// postDepositPenalty menulis jurnal pendapatan denda. Debit mengikuti dari sisi
// mana denda diserap (kewajiban imbal hasil dan/atau pokok), kredit ke akun
// pendapatan denda. Idempotency key tetap per kontrak agar pencairan ganda tidak
// memposting denda dua kali.
func (s *depositService) postDepositPenalty(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	dep *domain.Deposit,
	actor domain.Actor,
	fromProfit, fromPrincipal, penalty decimal.Decimal,
) error {
	coa := s.depositPenaltyCOA(ctx, product)
	if coa == "" {
		return fmt.Errorf("%w: atur %s untuk buku %s", domain.ErrDepositPenaltyCOAUnavailable, depositPenaltyCOAConfigKey, product.Book)
	}
	penaltyAccount, err := s.resolver.ResolveGLAccount(ctx, tx, coa)
	if err != nil {
		return fmt.Errorf("akun pendapatan denda %s: %w", coa, err)
	}

	lines := make([]domain.PostingLine, 0, 3)
	if fromProfit.IsPositive() {
		account, err := s.mappingAccount(ctx, tx, product.ID, domain.EventInterestPayment, domain.DirectionDebit)
		if err != nil {
			return err
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: account, Direction: domain.DirectionDebit, Amount: fromProfit,
			Description: "Denda pencairan deposito - imbal hasil",
		})
	}
	if fromPrincipal.IsPositive() {
		account, err := s.mappingAccount(ctx, tx, product.ID, domain.EventWithdrawal, domain.DirectionDebit)
		if err != nil {
			return err
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: account, Direction: domain.DirectionDebit, Amount: fromPrincipal,
			Description: "Denda pencairan deposito - pokok",
		})
	}
	lines = append(lines, domain.PostingLine{
		AccountNumber: penaltyAccount, Direction: domain.DirectionCredit, Amount: penalty,
		Description: "Pendapatan denda pencairan deposito",
	})

	if _, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     fmt.Sprintf("Denda pencairan lebih awal deposito %s", dep.AccountNumber),
		IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-PEN",
		CreatedBy:       actor.DisplayName(),
		BranchCode:      actor.BranchCode,
		Lines:           lines,
	}); err != nil {
		return fmt.Errorf("jurnal denda pencairan deposito: %w", err)
	}
	return nil
}

func (s *depositService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Deposit, error) {
	return s.depositRepo.GetByID(ctx, id)
}

func (s *depositService) List(ctx context.Context, page, pageSize int) ([]domain.Deposit, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.depositRepo.List(ctx, pageSize, (page-1)*pageSize)
}

// dateOnly menormalkan waktu ke tanggal UTC tanpa jam, agar akrual dan jatuh
// tempo dibandingkan per hari.
func depositDateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

var _ domain.DepositService = (*depositService)(nil)
