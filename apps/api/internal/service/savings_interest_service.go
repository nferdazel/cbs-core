package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi dan COA cadangan. Nilai produksi wajib diisi di system_config;
// fallback di sini hanya agar batch tetap berjalan di lingkungan yang belum
// di-provision. Kode COA mengikuti bagan baku BPR di migrasi 000005.
const (
	configSavingsRateAnnual      = "savings.interest.rate_annual"
	configSavingsExpenseCOAConv  = "savings.interest.expense.coa.conventional"
	configSavingsExpenseCOASyar  = "savings.interest.expense.coa.syariah"
	configSavingsPayableCOAConv  = "savings.interest.payable.coa.conventional"
	configSavingsPayableCOASyar  = "savings.interest.payable.coa.syariah"
	configMudharabahProfitPool   = "savings.mudharabah.distributable_profit"
	configAdminFeeMonthly        = "fee.admin.monthly"
	configAdminFeeRevenueCOAConv = "fee.admin.income.coa.conventional"
	configAdminFeeRevenueCOASyar = "fee.admin.income.coa.syariah"
	// PPh final atas bunga tabungan (PP 131/2000). Tarif dan ambang pembebasan
	// dibandingkan dengan SALDO tabungan, bukan bunga yang dibayarkan.
	configSavingsTaxRate       = "tax.savings.rate"
	configSavingsTaxExempt     = "tax.savings.exempt_amount"
	configSavingsTaxPayableCOA = "tax.savings.payable.coa"

	defaultSavingsExpenseCOAConv  = "50100" // Beban Bunga Deposito (satu-satunya akun beban bunga)
	defaultSavingsExpenseCOASyar  = "15100" // Bagi Hasil untuk Pemilik Dana
	defaultSavingsPayableCOAConv  = "20400" // Bunga Deposito yang Masih Harus Dibayar
	defaultSavingsPayableCOASyar  = "12400" // Bagi Hasil yang Masih Harus Dibayar
	defaultAdminFeeRevenueCOAConv = "40400" // Pendapatan Administrasi
	defaultAdminFeeRevenueCOASyar = "14500" // Pendapatan Administrasi Syariah
	defaultSavingsTaxPayableCOA   = "20500" // Utang Pajak
)

var (
	defaultSavingsTaxRate   = decimal.NewFromInt(20)
	defaultSavingsTaxExempt = decimal.NewFromInt(7_500_000)
)

type savingsInterestService struct {
	db          *sql.DB
	repo        domain.SavingsInterestRepository
	accountRepo domain.AccountRepository
	productRepo domain.ProductRepository
	poster      *ProductPoster
	posting     domain.PostingService
	resolver    domain.AccountResolver
	configSvc   domain.SystemConfigService
}

func NewSavingsInterestService(
	db *sql.DB,
	repo domain.SavingsInterestRepository,
	accountRepo domain.AccountRepository,
	productRepo domain.ProductRepository,
	poster *ProductPoster,
	posting domain.PostingService,
	resolver domain.AccountResolver,
	configSvc domain.SystemConfigService,
) domain.SavingsInterestService {
	return &savingsInterestService{
		db:          db,
		repo:        repo,
		accountRepo: accountRepo,
		productRepo: productRepo,
		poster:      poster,
		posting:     posting,
		resolver:    resolver,
		configSvc:   configSvc,
	}
}

// AccrueAll menghitung dan memposting akrual bunga/bagi hasil untuk seluruh rekening
// tabungan aktif pada satu bulan. book kosong berarti semua buku; setiap rekening
// tetap dihitung menurut bukunya sendiri sehingga konvensional dan syariah tidak
// tercampur. Kegagalan satu rekening tidak menghentikan rekening lain; dilaporkan
// sebagai FAILED pada ringkasan.
func (s *savingsInterestService) AccrueAll(ctx context.Context, period time.Time, book domain.COABook, createdBy string) (*domain.SavingsInterestSummary, error) {
	from, to, daysInYear := savingsMonthBounds(period)

	accounts, err := s.repo.ListSavingsAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca rekening simpanan: %w", err)
	}
	opening, err := s.repo.OpeningBalances(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("saldo awal periode: %w", err)
	}
	changes, err := s.repo.DailyNetChanges(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("mutasi harian periode: %w", err)
	}
	byAccount := groupDailyChanges(changes)

	candidates := make([]domain.SavingsAccountInfo, 0, len(accounts))
	for _, a := range accounts {
		if a.Family != domain.FamilySavings {
			continue
		}
		if a.Status != domain.AccountStatusActive {
			continue
		}
		if book != "" && a.Book != book {
			continue
		}
		candidates = append(candidates, a)
	}

	// Rata-rata saldo harian per rekening; total mudharabah dipakai untuk alokasi
	// bagi hasil secara proporsional.
	avgs := make(map[uuid.UUID]decimal.Decimal, len(candidates))
	totalMudharabah := decimal.Zero
	for _, a := range candidates {
		daily := buildDailyBalances(a, opening, byAccount, from, to)
		avg := domain.AverageDailyBalance(daily)
		avgs[a.AccountID] = avg
		if isMudharabah(a.ProfitScheme) {
			totalMudharabah = totalMudharabah.Add(avg)
		}
	}

	pool := s.configDecimal(ctx, configMudharabahProfitPool, decimal.Zero)
	summary := &domain.SavingsInterestSummary{Period: from.Format("2006-01"), Book: book}

	for _, a := range candidates {
		summary.ProcessedAccounts++
		daily := buildDailyBalances(a, opening, byAccount, from, to)
		res, err := s.accrue(ctx, a, daily, avgs[a.AccountID], totalMudharabah, pool, daysInYear, createdBy)
		if err != nil {
			res = domain.SavingsInterestResult{
				AccountNumber:  a.AccountNumber,
				Book:           a.Book,
				ProfitScheme:   a.ProfitScheme,
				AverageBalance: avgs[a.AccountID],
				Status:         domain.BatchItemFailed,
				Message:        err.Error(),
			}
		}
		switch res.Status {
		case domain.BatchItemAccrued:
			summary.AccruedAccounts++
			summary.TotalInterest = summary.TotalInterest.Add(res.Amount)
		case domain.BatchItemSkipped:
			summary.SkippedAccounts++
		default:
			summary.FailedAccounts++
		}
		summary.Results = append(summary.Results, res)
	}

	return summary, nil
}

// AccrueAccount memproses satu rekening. Rekening non-aktif atau wadiah tidak
// diposting, tetapi tetap dikembalikan sebagai hasil berstatus SKIPPED.
func (s *savingsInterestService) AccrueAccount(ctx context.Context, accountNumber string, period time.Time, createdBy string) (*domain.SavingsInterestResult, error) {
	info, err := s.repo.GetSavingsAccount(ctx, accountNumber)
	if err != nil {
		return nil, err
	}

	from, to, daysInYear := savingsMonthBounds(period)
	res := domain.SavingsInterestResult{
		AccountNumber: info.AccountNumber,
		Book:          info.Book,
		ProfitScheme:  info.ProfitScheme,
		Status:        domain.BatchItemSkipped,
	}
	if info.Status != domain.AccountStatusActive {
		res.Message = "rekening tidak aktif (" + string(info.Status) + ")"
		return &res, nil
	}
	if info.Family != domain.FamilySavings {
		res.Message = "bukan rekening tabungan"
		return &res, nil
	}

	opening, err := s.repo.OpeningBalances(ctx, from)
	if err != nil {
		return nil, fmt.Errorf("saldo awal periode: %w", err)
	}
	changes, err := s.repo.DailyNetChanges(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("mutasi harian periode: %w", err)
	}
	byAccount := groupDailyChanges(changes)
	daily := buildDailyBalances(*info, opening, byAccount, from, to)
	avg := domain.AverageDailyBalance(daily)

	totalMudharabah := decimal.Zero
	if isMudharabah(info.ProfitScheme) {
		accounts, err := s.repo.ListSavingsAccounts(ctx)
		if err != nil {
			return nil, fmt.Errorf("membaca rekan mudharabah: %w", err)
		}
		for _, a := range accounts {
			if a.Status != domain.AccountStatusActive || a.Family != domain.FamilySavings {
				continue
			}
			if !isMudharabah(a.ProfitScheme) {
				continue
			}
			d := buildDailyBalances(a, opening, byAccount, from, to)
			totalMudharabah = totalMudharabah.Add(domain.AverageDailyBalance(d))
		}
	}

	pool := s.configDecimal(ctx, configMudharabahProfitPool, decimal.Zero)
	result, err := s.accrue(ctx, *info, daily, avg, totalMudharabah, pool, daysInYear, createdBy)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// accrue menghitung nominal satu rekening lalu memposting jurnalnya dalam satu
// transaksi bersama penanda idempotensi.
func (s *savingsInterestService) accrue(
	ctx context.Context,
	info domain.SavingsAccountInfo,
	daily []domain.DailyBalance,
	averageBalance decimal.Decimal,
	totalMudharabah decimal.Decimal,
	pool decimal.Decimal,
	daysInYear int,
	createdBy string,
) (domain.SavingsInterestResult, error) {
	res := domain.SavingsInterestResult{
		AccountNumber:  info.AccountNumber,
		Book:           info.Book,
		ProfitScheme:   info.ProfitScheme,
		AverageBalance: averageBalance,
		Status:         domain.BatchItemSkipped,
	}

	var amount decimal.Decimal
	switch info.ProfitScheme {
	case domain.SchemeWadiah:
		res.Message = "tabungan wadiah tanpa imbalan"
		return res, nil
	case domain.SchemeMudharabah, domain.SchemeMusyarakah:
		amount = domain.MudharabahShare(pool, info.ProfitSharingRatio, averageBalance, totalMudharabah)
		if amount.IsZero() {
			res.Message = "bagi hasil nihil (laba didistribusikan, nisbah, atau saldo nol)"
			return res, nil
		}
	case domain.SchemeInterest:
		rate := s.configDecimal(ctx, configSavingsRateAnnual, info.RateAnnual)
		amount = domain.SavingsInterest(daily, rate, daysInYear)
		if amount.IsZero() {
			res.Message = "bunga nihil"
			return res, nil
		}
	default:
		res.Message = fmt.Sprintf("skema imbal hasil %s tidak didukung akrual tabungan", info.ProfitScheme)
		return res, nil
	}

	product, err := s.productRepo.GetByID(ctx, info.ProductID)
	if err != nil {
		return res, fmt.Errorf("membaca produk %s: %w", info.ProductCode, err)
	}

	expenseCOA, payableCOA, mapped, err := s.resolveInterestCOAs(ctx, product, info.Book)
	if err != nil {
		return res, err
	}

	// Periode diturunkan dari tanggal saldo harian pertama (hari pertama bulan).
	periodStr := ""
	if len(daily) > 0 {
		periodStr = daily[0].Date.Format("2006-01")
	}

	// Akrual milik akhir bulan periode yang diakrual, bukan tanggal batch dijalankan.
	entryDate := time.Time{}
	if len(daily) > 0 {
		first := daily[0].Date
		entryDate = time.Date(first.Year(), first.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return res, err
	}
	defer tx.Rollback()

	// Penanda idempotensi disisipkan lebih dulu: hanya pemanggil pertama yang menang.
	inserted, err := s.repo.InsertInterestAccrual(ctx, tx, &domain.InterestAccrualRecord{
		AccountID:      info.AccountID,
		AccountNumber:  info.AccountNumber,
		Period:         periodStr,
		Book:           info.Book,
		ProfitScheme:   info.ProfitScheme,
		Amount:         amount,
		AverageBalance: averageBalance,
		ExpenseCOACode: expenseCOA,
		PayableCOACode: payableCOA,
	})
	if err != nil {
		return res, fmt.Errorf("menyimpan penanda akrual: %w", err)
	}
	if !inserted {
		res.Message = "akrual periode ini sudah pernah diposting"
		return res, nil
	}

	meta := PostingMeta{
		TransactionType: domain.TxTypeInterestAccrual,
		Description:     fmt.Sprintf("Akrual %s tabungan %s", interestLabel(info.ProfitScheme), info.AccountNumber),
		IdempotencyKey:  fmt.Sprintf("SAV-INT-%s-%s", info.AccountNumber, periodStr),
		CreatedBy:       createdBy,
		EntryDate:       entryDate,
	}

	var entry *domain.JournalEntry
	if mapped {
		// Pemetaan produk tersedia: pakai ProductPoster agar akun lawan tetap
		// sejalan dengan product_journal_mapping.
		entry, err = s.poster.PostEventTx(ctx, tx, product, domain.EventInterestAccrual, Amounts{Profit: amount}, meta)
	} else {
		entry, err = s.postInterestFallback(ctx, tx, expenseCOA, payableCOA, amount, meta)
	}
	if err != nil {
		return res, err
	}

	if err := s.repo.UpdateInterestAccrualJournal(ctx, tx, info.AccountID, periodStr, entry.ID); err != nil {
		return res, err
	}
	if err := tx.Commit(); err != nil {
		return res, err
	}

	res.Amount = amount
	res.ExpenseCOACode = expenseCOA
	res.PayableCOACode = payableCOA
	res.JournalReference = entry.ReferenceNumber
	res.Status = domain.BatchItemAccrued
	res.Message = ""
	return res, nil
}

// postInterestFallback memposting akrual saat produk tidak punya pemetaan
// INTEREST_ACCRUAL: nomor akun GL diresolusi dari kode COA hasil pemetaan/konfigurasi.
func (s *savingsInterestService) postInterestFallback(
	ctx context.Context,
	tx *sql.Tx,
	expenseCOA, payableCOA string,
	amount decimal.Decimal,
	meta PostingMeta,
) (*domain.JournalEntry, error) {
	expenseAcc, err := s.resolver.ResolveGLAccount(ctx, tx, expenseCOA)
	if err != nil {
		return nil, err
	}
	payableAcc, err := s.resolver.ResolveGLAccount(ctx, tx, payableCOA)
	if err != nil {
		return nil, err
	}
	return s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: meta.TransactionType,
		Description:     meta.Description,
		IdempotencyKey:  meta.IdempotencyKey,
		CreatedBy:       meta.CreatedBy,
		EntryDate:       meta.EntryDate,
		Lines: []domain.PostingLine{
			{AccountNumber: expenseAcc, Direction: domain.DirectionDebit, Amount: amount, Description: "Beban imbal hasil tabungan"},
			{AccountNumber: payableAcc, Direction: domain.DirectionCredit, Amount: amount, Description: "Kewajiban imbal hasil tabungan"},
		},
	})
}

// resolveInterestCOAs menentukan akun beban dan kewajiban. Pemetaan produk dipakai
// lebih dulu; bila tidak lengkap, fallback konfigurasi per buku.
func (s *savingsInterestService) resolveInterestCOAs(ctx context.Context, product *domain.BankingProduct, book domain.COABook) (string, string, bool, error) {
	var expense, payable string
	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventInterestAccrual)
		if err != nil {
			// Galat pembacaan pemetaan BUKAN "produk belum dipetakan". Tanpa pembedaan
			// ini, produk yang sudah memetakan jurnal bunganya dapat tanpa jejak
			// terjurnal ke COA bawaan saat ada galat sesaat. Tidak ada pemetaan
			// (rules kosong) tetap memakai COA konfigurasi di bawah.
			return "", "", false, fmt.Errorf("membaca pemetaan jurnal bunga tabungan produk %s: %w", product.Code, err)
		}
		for _, r := range rules {
			switch r.Direction {
			case domain.DirectionDebit:
				expense = r.COACode
			case domain.DirectionCredit:
				payable = r.COACode
			}
		}
	}
	if expense != "" && payable != "" {
		return expense, payable, true, nil
	}
	if book == domain.BookSyariah {
		return s.configString(ctx, configSavingsExpenseCOASyar, defaultSavingsExpenseCOASyar),
			s.configString(ctx, configSavingsPayableCOASyar, defaultSavingsPayableCOASyar), false, nil
	}
	return s.configString(ctx, configSavingsExpenseCOAConv, defaultSavingsExpenseCOAConv),
		s.configString(ctx, configSavingsPayableCOAConv, defaultSavingsPayableCOAConv), false, nil
}

// PayInterestToAccounts memindahkan akrual bunga/bagi hasil yang belum dibayar ke
// rekening nasabah: debit utang bunga, kredit rekening nasabah. Akrual disimpan
// sebagai utang saat EOM; tanpa langkah ini bunga tidak pernah diterima nasabah.
// Idempoten: akrual yang sudah ditandai dibayar tidak muncul lagi.
func (s *savingsInterestService) PayInterestToAccounts(ctx context.Context, period time.Time, createdBy string) (*domain.SavingsInterestSummary, error) {
	periodStr := period.Format("2006-01")

	records, err := s.repo.ListUnpaidAccruals(ctx, periodStr)
	if err != nil {
		return nil, fmt.Errorf("membaca akrual yang belum dibayar: %w", err)
	}

	summary := &domain.SavingsInterestSummary{Period: periodStr}
	for _, rec := range records {
		result := s.payAccrual(ctx, rec, period, createdBy)
		summary.Results = append(summary.Results, result)
		switch result.Status {
		case domain.BatchItemPaid:
			summary.ProcessedAccounts++
			summary.TotalInterest = summary.TotalInterest.Add(result.Amount)
			summary.TotalTax = summary.TotalTax.Add(result.TaxAmount)
		case domain.BatchItemFailed:
			summary.FailedAccounts++
		default:
			summary.SkippedAccounts++
		}
	}
	return summary, nil
}

// payAccrual membayarkan satu akrual ke rekening nasabah dalam satu transaksi
// bersama penandaan paid_at, sehingga jurnal dan penanda tidak pernah terpisah.
func (s *savingsInterestService) payAccrual(
	ctx context.Context,
	rec domain.InterestAccrualRecord,
	period time.Time,
	createdBy string,
) domain.SavingsInterestResult {
	res := domain.SavingsInterestResult{
		AccountNumber:  rec.AccountNumber,
		Book:           rec.Book,
		ProfitScheme:   rec.ProfitScheme,
		AverageBalance: rec.AverageBalance,
		Amount:         rec.Amount,
		ExpenseCOACode: rec.ExpenseCOACode,
		PayableCOACode: rec.PayableCOACode,
		Status:         domain.BatchItemFailed,
	}

	if !rec.Amount.IsPositive() {
		res.Status = domain.BatchItemSkipped
		res.Message = "nominal akrual nol"
		return res
	}

	// Bunga dibayakan pada akhir bulan periode, sama dengan biaya administrasi.
	entryDate := time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	// PPh final dipotong dari bunga yang dibayarkan ke nasabah. Ambang pembebasan
	// dibandingkan dengan saldo tabungan (PP 131/2000 Pasal 3 huruf a), jadi saldo
	// dibaca sebelum bunga dikreditkan.
	tax, err := s.savingsAccrualTax(ctx, rec)
	if err != nil {
		res.Message = fmt.Sprintf("menghitung PPh final: %v", err)
		return res
	}
	if tax.GreaterThan(rec.Amount) {
		res.Message = "PPh final melebihi bunga yang dibayarkan"
		return res
	}
	net := rec.Amount.Sub(tax)

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		res.Message = err.Error()
		return res
	}
	defer tx.Rollback()

	payableAcc, err := s.resolver.ResolveGLAccount(ctx, tx, rec.PayableCOACode)
	if err != nil {
		res.Message = fmt.Sprintf("akun utang bunga: %v", err)
		return res
	}

	lines := []domain.PostingLine{
		{AccountNumber: payableAcc, Direction: domain.DirectionDebit, Amount: rec.Amount, Description: "Pelunasan utang bunga tabungan"},
	}
	if net.IsPositive() {
		lines = append(lines, domain.PostingLine{
			AccountNumber: rec.AccountNumber,
			Direction:     domain.DirectionCredit,
			Amount:        net,
			Description:   "Bunga tabungan",
		})
	}
	if tax.IsPositive() {
		taxAcc, err := s.resolver.ResolveGLAccount(ctx, tx,
			s.configString(ctx, configSavingsTaxPayableCOA, defaultSavingsTaxPayableCOA))
		if err != nil {
			res.Message = fmt.Sprintf("akun utang pajak: %v", err)
			return res
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: taxAcc,
			Direction:     domain.DirectionCredit,
			Amount:        tax,
			Description:   "PPh final bunga tabungan",
		})
	}

	entry, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeInterestAccrual,
		Description:     fmt.Sprintf("Pembayaran bunga tabungan %s periode %s", rec.AccountNumber, rec.Period),
		IdempotencyKey:  fmt.Sprintf("SAV-INT-PAY-%s-%s", rec.AccountNumber, rec.Period),
		CreatedBy:       createdBy,
		EntryDate:       entryDate,
		Lines:           lines,
	})
	if err != nil {
		res.Message = fmt.Sprintf("posting pembayaran bunga: %v", err)
		return res
	}
	if err := s.repo.MarkAccrualPaid(ctx, tx, rec.ID, entry.ID); err != nil {
		res.Message = fmt.Sprintf("menandai akrual sudah dibayar: %v", err)
		return res
	}
	if err := tx.Commit(); err != nil {
		res.Message = err.Error()
		return res
	}

	res.Status = domain.BatchItemPaid
	res.JournalReference = entry.ReferenceNumber
	res.TaxAmount = tax
	res.Message = ""
	return res
}

// ChargeAdminFees memotong biaya administrasi bulanan setiap rekening aktif. Biaya
// diambil dari konfigurasi fee.admin.monthly, dengan fallback admin_fee produk.
// Rekening bersaldo kurang dari biaya tidak dipotong agar saldo tidak negatif.
func (s *savingsInterestService) ChargeAdminFees(ctx context.Context, period time.Time, createdBy string) (*domain.AdminFeeSummary, error) {
	from, _, _ := savingsMonthBounds(period)
	periodStr := from.Format("2006-01")

	accounts, err := s.repo.ListSavingsAccounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca rekening: %w", err)
	}

	summary := &domain.AdminFeeSummary{Period: periodStr}
	for _, info := range accounts {
		if info.Status != domain.AccountStatusActive {
			continue
		}
		fee := s.configDecimal(ctx, configAdminFeeMonthly, info.AdminFee)
		if !fee.IsPositive() {
			continue
		}
		summary.ProcessedAccounts++

		res := s.chargeAdminFee(ctx, info, fee, periodStr, createdBy)
		switch res.Status {
		case domain.BatchItemCharged:
			summary.ChargedAccounts++
			summary.TotalAdminFees = summary.TotalAdminFees.Add(res.Amount)
		case domain.BatchItemSkipped:
			summary.SkippedAccounts++
		default:
			summary.FailedAccounts++
		}
		summary.Results = append(summary.Results, res)
	}

	return summary, nil
}

func (s *savingsInterestService) chargeAdminFee(
	ctx context.Context,
	info domain.SavingsAccountInfo,
	fee decimal.Decimal,
	periodStr string,
	createdBy string,
) domain.AdminFeeResult {
	res := domain.AdminFeeResult{AccountNumber: info.AccountNumber, Amount: fee, Status: domain.BatchItemFailed}

	// Biaya administrasi milik akhir bulan periode, bukan tanggal batch dijalankan.
	entryDate := time.Time{}
	if period, perr := time.Parse("2006-01", periodStr); perr == nil {
		entryDate = time.Date(period.Year(), period.Month()+1, 0, 0, 0, 0, 0, time.UTC)
	}

	product, err := s.productRepo.GetByID(ctx, info.ProductID)
	if err != nil {
		res.Message = fmt.Sprintf("membaca produk: %v", err)
		return res
	}
	revenueCOA, err := s.adminFeeRevenueCOA(ctx, product, info.Book)
	if err != nil {
		res.Message = fmt.Sprintf("membaca pemetaan pendapatan administrasi: %v", err)
		return res
	}
	revenueAcc, err := s.resolver.ResolveGLAccount(ctx, nil, revenueCOA)
	if err != nil {
		res.Message = fmt.Sprintf("akun pendapatan administrasi: %v", err)
		return res
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		res.Message = err.Error()
		return res
	}
	defer tx.Rollback()

	// Kunci rekening lebih dulu lalu pastikan saldo cukup.
	acc, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, info.AccountNumber)
	if err != nil {
		res.Message = fmt.Sprintf("mengunci rekening: %v", err)
		return res
	}
	if acc.Balance.LessThan(fee) {
		res.Message = "saldo tidak cukup untuk biaya administrasi"
		return res
	}

	inserted, err := s.repo.InsertAdminFeeCharge(ctx, tx, &domain.AdminFeeChargeRecord{
		AccountID:      info.AccountID,
		AccountNumber:  info.AccountNumber,
		Period:         periodStr,
		Amount:         fee,
		RevenueCOACode: revenueCOA,
	})
	if err != nil {
		res.Message = fmt.Sprintf("menyimpan penanda biaya: %v", err)
		return res
	}
	if !inserted {
		res.Status = domain.BatchItemSkipped
		res.Message = "biaya administrasi periode ini sudah dipotong"
		return res
	}

	entry, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeFeeCharge,
		Description:     fmt.Sprintf("Biaya administrasi %s periode %s", info.AccountNumber, periodStr),
		IdempotencyKey:  fmt.Sprintf("EOM-ADM-%s-%s", info.AccountNumber, periodStr),
		CreatedBy:       createdBy,
		EntryDate:       entryDate,
		Lines: []domain.PostingLine{
			{AccountNumber: info.AccountNumber, Direction: domain.DirectionDebit, Amount: fee, Description: "Biaya administrasi bulanan"},
			{AccountNumber: revenueAcc, Direction: domain.DirectionCredit, Amount: fee, Description: "Pendapatan administrasi"},
		},
	})
	if err != nil {
		res.Message = fmt.Sprintf("posting biaya administrasi: %v", err)
		return res
	}
	if err := s.repo.UpdateAdminFeeChargeJournal(ctx, tx, info.AccountID, periodStr, entry.ID); err != nil {
		res.Message = err.Error()
		return res
	}
	if err := tx.Commit(); err != nil {
		res.Message = err.Error()
		return res
	}

	res.Status = domain.BatchItemCharged
	res.JournalReference = entry.ReferenceNumber
	res.Message = ""
	return res
}

// adminFeeRevenueCOA memilih akun pendapatan administrasi: pemetaan FEE_INCOME yang
// mengkredit, lalu konfigurasi per buku.
func (s *savingsInterestService) adminFeeRevenueCOA(ctx context.Context, product *domain.BankingProduct, book domain.COABook) (string, error) {
	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventFeeIncome)
		if err != nil {
			// Sama seperti resolveInterestCOAs: galat baca pemetaan tidak boleh
			// disamarkan sebagai "produk belum dipetakan" lalu jatuh ke COA bawaan.
			return "", fmt.Errorf("membaca pemetaan jurnal pendapatan administrasi produk %s: %w", product.Code, err)
		}
		for _, r := range rules {
			if r.Direction == domain.DirectionCredit {
				return r.COACode, nil
			}
		}
	}
	if book == domain.BookSyariah {
		return s.configString(ctx, configAdminFeeRevenueCOASyar, defaultAdminFeeRevenueCOASyar), nil
	}
	return s.configString(ctx, configAdminFeeRevenueCOAConv, defaultAdminFeeRevenueCOAConv), nil
}

// savingsAccrualTax menghitung PPh final atas satu akrual bunga tabungan. Ambang
// pembebasan dibandingkan dengan SALDO tabungan, bukan bunganya (PP 131/2000 Pasal 3
// huruf a). Bagi hasil mudharabah belum dipotong: ia bukan bunga, dan perlakuan
// pajaknya menunggu keputusan bank (dicatat di backlog).
func (s *savingsInterestService) savingsAccrualTax(ctx context.Context, rec domain.InterestAccrualRecord) (decimal.Decimal, error) {
	if rec.Book != domain.BookConventional {
		return decimal.Zero, nil
	}
	account, err := s.accountRepo.GetByID(ctx, rec.AccountID)
	if err != nil {
		return decimal.Zero, err
	}
	return domain.SavingsInterestTax(
		rec.Amount,
		account.Balance,
		s.configDecimal(ctx, configSavingsTaxRate, defaultSavingsTaxRate),
		s.configDecimal(ctx, configSavingsTaxExempt, defaultSavingsTaxExempt),
	), nil
}

func (s *savingsInterestService) configDecimal(ctx context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if s.configSvc == nil {
		return fallback
	}
	return s.configSvc.GetDecimal(ctx, key, fallback)
}

func (s *savingsInterestService) configString(ctx context.Context, key, fallback string) string {
	if s.configSvc == nil {
		return fallback
	}
	return s.configSvc.GetString(ctx, key, fallback)
}

// savingsMonthBounds mengembalikan hari pertama, hari terakhir, dan jumlah hari setahun
// (365/366) untuk periode satu bulan.
func savingsMonthBounds(period time.Time) (from, to time.Time, daysInYear int) {
	y, m, _ := period.Date()
	from = time.Date(y, m, 1, 0, 0, 0, 0, time.UTC)
	to = from.AddDate(0, 1, -1)
	daysInYear = 365
	if y%4 == 0 && (y%100 != 0 || y%400 == 0) {
		daysInYear = 366
	}
	return from, to, daysInYear
}

// buildDailyBalances menyusun saldo akhir setiap hari kalender dari saldo awal dan
// mutasi harian. Rekening tanpa jejak jurnal sama sekali memakai saldo tersimpan
// sebagai saldo tetap agar bunga atas saldo awal yang tidak dijurnal tidak hilang.
func buildDailyBalances(
	info domain.SavingsAccountInfo,
	opening map[uuid.UUID]decimal.Decimal,
	changes map[uuid.UUID]map[string]decimal.Decimal,
	from, to time.Time,
) []domain.DailyBalance {
	balance, hasOpening := opening[info.AccountID]
	perDay := changes[info.AccountID]
	if !hasOpening && len(perDay) == 0 && info.Balance.IsPositive() {
		balance = info.Balance
	}

	out := make([]domain.DailyBalance, 0, 31)
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		if perDay != nil {
			if delta, ok := perDay[d.Format("2006-01-02")]; ok {
				balance = balance.Add(delta)
			}
		}
		out = append(out, domain.DailyBalance{Date: d, Balance: balance})
	}
	return out
}

func groupDailyChanges(changes []domain.DailyNetChange) map[uuid.UUID]map[string]decimal.Decimal {
	out := make(map[uuid.UUID]map[string]decimal.Decimal)
	for _, c := range changes {
		key := c.Date.Format("2006-01-02")
		if out[c.AccountID] == nil {
			out[c.AccountID] = make(map[string]decimal.Decimal)
		}
		out[c.AccountID][key] = out[c.AccountID][key].Add(c.Delta)
	}
	return out
}

func isMudharabah(scheme domain.ProfitScheme) bool {
	return scheme == domain.SchemeMudharabah || scheme == domain.SchemeMusyarakah
}

func interestLabel(scheme domain.ProfitScheme) string {
	if isMudharabah(scheme) {
		return "bagi hasil"
	}
	return "bunga"
}

var _ domain.SavingsInterestService = (*savingsInterestService)(nil)
