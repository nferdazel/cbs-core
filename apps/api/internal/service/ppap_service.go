package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi COA PPAP. Tarif dan ambang DPD dibaca lewat helper bersama di
// collectibility_rules.go agar aturannya identik dengan jalur restrukturisasi kredit.
const (
	cfgPPAPExpenseCOA = "ppap.coa.expense"
	cfgPPAPReserveCOA = "ppap.coa.reserve"
)

// Fallback kode COA. Kode ini ADA di seed migrasi 000005:
//   - 50200 Beban Penyisihan Kerugian Kredit (EXPENSE, konvensional)
//   - 15200 Beban Penyisihan Kerugian Pembiayaan (EXPENSE, syariah)
//   - 10900 Cadangan Kerugian Penurunan Nilai / PPAP (ASSET contra, konvensional)
//   - 11900 Cadangan Kerugian Pembiayaan (ASSET contra, syariah)
//
// Kunci ppap.coa.expense/ppap.coa.reserve bersifat global (bukan per buku); nilai
// kosong atau tidak ada berarti fallback per buku di atas yang dipakai.
const (
	fallbackPPAPExpenseConventional = "50200"
	fallbackPPAPExpenseSyariah      = "15200"
	fallbackPPAPReserveConventional = "10900"
	fallbackPPAPReserveSyariah      = "11900"
)

// ppapTxRunner membuka transaksi per kredit. Interface ini membuat pemrosesan bisa
// diuji tanpa database: produksi memakai *sql.DB, test memakai runner palsu.
type ppapTxRunner interface {
	Run(ctx context.Context, fn func(tx any) error) error
}

type sqlPPAPTxRunner struct{ db *sql.DB }

func (r sqlPPAPTxRunner) Run(ctx context.Context, fn func(tx any) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type ppapService struct {
	txRunner    ppapTxRunner
	repo        domain.PPAPRepository
	productRepo domain.ProductRepository
	resolver    domain.AccountResolver
	poster      *ProductPoster
	posting     domain.PostingService
	config      domain.SystemConfigService
}

func NewPPAPService(
	db *sql.DB,
	repo domain.PPAPRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	poster *ProductPoster,
	posting domain.PostingService,
	config domain.SystemConfigService,
) domain.PPAPService {
	return &ppapService{
		txRunner:    sqlPPAPTxRunner{db: db},
		repo:        repo,
		productRepo: productRepo,
		resolver:    resolver,
		poster:      poster,
		posting:     posting,
		config:      config,
	}
}

// RunDaily menghitung kolektibilitas dan PPAP seluruh kredit aktif. Setiap kredit
// diproses dalam transaksinya sendiri: satu kredit gagal tidak menggagalkan batch.
func (s *ppapService) RunDaily(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PPAPRunSummary, error) {
	return s.run(ctx, asOf, actor, false)
}

// Preview menghitung tanpa memposting jurnal atau mengubah state kredit.
func (s *ppapService) Preview(ctx context.Context, asOf time.Time) (domain.PPAPRunSummary, error) {
	return s.run(ctx, asOf, domain.Actor{}, true)
}

func (s *ppapService) run(ctx context.Context, asOf time.Time, actor domain.Actor, preview bool) (domain.PPAPRunSummary, error) {
	asOf = asOf.UTC()

	snapshots, err := s.repo.ListDueLoans(ctx, asOf)
	if err != nil {
		return domain.PPAPRunSummary{}, fmt.Errorf("mengambil daftar kredit PPAP: %w", err)
	}

	thresholds := collectibilityThresholds(ctx, s.config)
	rates := collectibilityRates(ctx, s.config)

	summary := domain.PPAPRunSummary{
		AsOf:          asOf,
		Total:         len(snapshots),
		Preview:       preview,
		ReserveBefore: s.totalReserve(ctx),
	}

	for _, snap := range snapshots {
		item, err := s.processLoan(ctx, asOf, snap, thresholds, rates, actor, preview)
		if err != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, domain.PPAPRunFailure{
				LoanID:     snap.LoanID,
				LoanNumber: snap.LoanNumber,
				Error:      err.Error(),
			})
			continue
		}

		summary.Processed++
		summary.TotalAdjustment = summary.TotalAdjustment.Add(item.Adjustment)
		if !item.Posted && !item.CollectibilityChanged && item.DPD == snap.DPD && item.Target.Equal(snap.RequiredPPAP) {
			summary.Skipped++
		}
		summary.Items = append(summary.Items, item)
	}

	if !preview {
		summary.ReserveAfter = s.totalReserve(ctx)
	}
	return summary, nil
}

// processLoan menghitung dan menerapkan PPAP satu kredit. preview=true hanya menghitung.
func (s *ppapService) processLoan(
	ctx context.Context,
	asOf time.Time,
	snap domain.PPAPLoanSnapshot,
	thresholds domain.CollectibilityThresholds,
	rates domain.PPAPRates,
	actor domain.Actor,
	preview bool,
) (domain.PPAPRunItem, error) {
	dpd := 0
	if snap.LastDueDate != nil {
		dpd = daysPastDue(asOf, *snap.LastDueDate)
	}
	col := domain.CollectibilityFromPosition(dpd, DaysPastMaturity(asOf, snap.FinalDueDate), thresholds)
	if snap.IsRestructured {
		// Pasal 23 POJK 1/2024: restrukturisasi tidak boleh menaikkan golongan sebelum
		// 3 periode pembayaran bersih berturut-turut.
		col = domain.RestructureCollectibility(snap.PreRestructureCollectibility, col, snap.CleanPeriods)
	}

	// Cadangan yang sudah ada untuk kredit ini adalah target terakhir yang tersimpan
	// di loans.required_ppap. Saldo akun GL cadangan bersifat agregat portofolio,
	// sehingga memakainya sebagai pengurang per kredit akan menggandakan/menghilangkan
	// selisih antar kredit. Saldo GL tetap dipakai untuk rekonsiliasi awal/akhir run.
	calc := domain.CalculatePPAP(snap.Outstanding, col, snap.RequiredPPAP, rates)

	stop := col.IsNPL() // golongan 3-5: akrual dihentikan (cash basis) sesuai POJK
	accrual := domain.AccrualStatusAccrual
	if stop {
		accrual = domain.AccrualStatusCash
	}

	item := domain.PPAPRunItem{
		LoanID:                snap.LoanID,
		LoanNumber:            snap.LoanNumber,
		DPD:                   dpd,
		Collectibility:        col,
		Outstanding:           snap.Outstanding,
		Target:                calc.Target,
		Existing:              calc.Existing,
		Adjustment:            calc.Adjustment,
		CollectibilityChanged: col != snap.Collectibility,
		StopAccrual:           stop,
	}

	changed := item.CollectibilityChanged || dpd != snap.DPD ||
		!calc.Adjustment.IsZero() || !calc.Target.Equal(snap.RequiredPPAP)
	if !changed || preview {
		return item, nil
	}

	// Pemetaan jurnal produk dipakai bila tersedia; jika tidak, fallback memakai COA
	// dari konfigurasi. Buku produk menentukan fallback konvensional/syariah.
	var product *domain.BankingProduct
	if snap.ProductID != nil {
		p, err := s.productRepo.GetByID(ctx, *snap.ProductID)
		if err == nil {
			product = p
		} else {
			slog.WarnContext(ctx, "produk kredit tidak terbaca; memakai fallback COA PPAP",
				"loan", snap.LoanNumber, "error", err)
		}
	}
	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}

	err := s.txRunner.Run(ctx, func(tx any) error {
		if !calc.Adjustment.IsZero() {
			if err := s.postAdjustment(ctx, tx, product, book, snap, calc.Adjustment, col, asOf, actor); err != nil {
				return err
			}
			item.Posted = true
		}
		return s.repo.UpdateLoanState(ctx, tx, domain.PPAPLoanUpdate{
			LoanID:         snap.LoanID,
			Collectibility: col,
			DPD:            dpd,
			AccrualStatus:  accrual,
			StopAccrual:    stop,
			RequiredPPAP:   calc.Target,
		})
	})
	if err != nil {
		return domain.PPAPRunItem{}, fmt.Errorf("kredit %s: %w", snap.LoanNumber, err)
	}
	return item, nil
}

// postAdjustment memposting selisih PPAP. Positif = provisi (debit beban, kredit
// cadangan); negatif = reversal (debit cadangan, kredit beban).
func (s *ppapService) postAdjustment(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	book domain.COABook,
	snap domain.PPAPLoanSnapshot,
	adjustment decimal.Decimal,
	col domain.Collectibility,
	asOf time.Time,
	actor domain.Actor,
) error {
	event := domain.EventPPAPProvision
	if adjustment.IsNegative() {
		event = domain.EventPPAPReversal
	}

	meta := PostingMeta{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("PPAP kredit %s golongan %s (%s)", snap.LoanNumber, col.Label(), adjustment.String()),
		IdempotencyKey:  fmt.Sprintf("PPAP-%s-%s-%s", snap.LoanNumber, asOf.Format("2006-01-02"), adjustment.String()),
		CreatedBy:       actor.DisplayName(),
		BranchCode:      actor.BranchCode,
	}
	amount := adjustment.Abs()

	// Jalur utama: pemetaan jurnal produk. Seed 000005 memetakan PPAP_PROVISION untuk
	// produk kredit contoh; PPAP_REVERSAL biasanya belum dipetakan sehingga jatuh ke
	// fallback COA konfigurasi di bawah.
	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, event)
		if err == nil && len(rules) > 0 {
			if _, err := s.poster.PostEventTx(ctx, tx, product, event, Amounts{
				Principal: amount,
				Total:     amount,
			}, meta); err != nil {
				return fmt.Errorf("jurnal PPAP produk %s: %w", product.Code, err)
			}
			return nil
		}
	}

	return s.postAdjustmentFallback(ctx, tx, amount, adjustment.IsPositive(), book, meta)
}

// postAdjustmentFallback memposting PPAP memakai kode COA dari konfigurasi
// (ppap.coa.expense/ppap.coa.reserve) dengan fallback per buku produk.
func (s *ppapService) postAdjustmentFallback(
	ctx context.Context,
	tx any,
	amount decimal.Decimal,
	provision bool,
	book domain.COABook,
	meta PostingMeta,
) error {
	expenseCOA := s.expenseCOA(ctx, book)
	reserveCOA := s.reserveCOA(ctx, book)

	expenseAcc, err := s.resolver.ResolveGLAccount(ctx, tx, expenseCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrPPAPExpenseNotFound, expenseCOA, err)
	}
	reserveAcc, err := s.resolver.ResolveGLAccount(ctx, tx, reserveCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrPPAPReserveNotFound, reserveCOA, err)
	}

	var debitAcc, creditAcc string
	if provision {
		debitAcc, creditAcc = expenseAcc, reserveAcc
	} else {
		debitAcc, creditAcc = reserveAcc, expenseAcc
	}

	lines := []domain.PostingLine{
		{AccountNumber: debitAcc, Direction: domain.DirectionDebit, Amount: amount, Description: meta.Description},
		{AccountNumber: creditAcc, Direction: domain.DirectionCredit, Amount: amount, Description: meta.Description},
	}
	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: meta.TransactionType,
		Description:     meta.Description,
		IdempotencyKey:  meta.IdempotencyKey,
		CreatedBy:       meta.CreatedBy,
		BranchCode:      meta.BranchCode,
		Lines:           lines,
	})
	return err
}

func (s *ppapService) totalReserve(ctx context.Context) decimal.Decimal {
	total := decimal.Zero
	for _, book := range []domain.COABook{domain.BookConventional, domain.BookSyariah} {
		code := s.reserveCOA(ctx, book)
		balance, err := s.repo.GetPPAPReserveBalance(ctx, nil, code)
		if err != nil {
			slog.WarnContext(ctx, "saldo cadangan PPAP tidak terbaca", "coa", code, "error", err)
			continue
		}
		total = total.Add(balance)
	}
	return total
}

func (s *ppapService) expenseCOA(ctx context.Context, book domain.COABook) string {
	fallback := fallbackPPAPExpenseConventional
	if book == domain.BookSyariah {
		fallback = fallbackPPAPExpenseSyariah
	}
	return configStringOr(ctx, s.config, cfgPPAPExpenseCOA, fallback)
}

func (s *ppapService) reserveCOA(ctx context.Context, book domain.COABook) string {
	fallback := fallbackPPAPReserveConventional
	if book == domain.BookSyariah {
		fallback = fallbackPPAPReserveSyariah
	}
	return configStringOr(ctx, s.config, cfgPPAPReserveCOA, fallback)
}

// daysPastDue menghitung selisih hari kalender (bukan jam) antara asOf dan jatuh tempo.
func daysPastDue(asOf, due time.Time) int {
	a := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	d := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC)
	days := int(a.Sub(d).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

var _ domain.PPAPService = (*ppapService)(nil)
