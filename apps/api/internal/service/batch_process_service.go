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

// COA laba ditahan baku BPR (migrasi 000005). Syariah memakai akun terpisah agar
// penutupan UUS tidak mencampur laba konvensional.
const (
	defaultRetainedEarningsCOAConv = "30200"
	defaultRetainedEarningsCOASyar = "13200"

	configRetainedEarningsCOAConv = "retained_earnings.coa.conventional"
	configRetainedEarningsCOASyar = "retained_earnings.coa.syariah"
)

type batchProcessService struct {
	dateRepo    domain.BusinessDateRepository
	batchRepo   domain.BatchActivityRepository
	savingsSvc  domain.SavingsInterestService
	yearEndRepo domain.YearEndRepository
	posting     domain.PostingService
	resolver    domain.AccountResolver
	configSvc   domain.SystemConfigService
	db          *sql.DB
}

func NewBatchProcessService(
	dateRepo domain.BusinessDateRepository,
	batchRepo domain.BatchActivityRepository,
	savingsSvc domain.SavingsInterestService,
	yearEndRepo domain.YearEndRepository,
	posting domain.PostingService,
	resolver domain.AccountResolver,
	configSvc domain.SystemConfigService,
	db *sql.DB,
) domain.BatchProcessService {
	return &batchProcessService{
		dateRepo:    dateRepo,
		batchRepo:   batchRepo,
		savingsSvc:  savingsSvc,
		yearEndRepo: yearEndRepo,
		posting:     posting,
		resolver:    resolver,
		configSvc:   configSvc,
		db:          db,
	}
}

func (s *batchProcessService) GetCurrentBusinessDate(ctx context.Context) (*domain.SystemBusinessDate, error) {
	return s.dateRepo.GetCurrentDate(ctx)
}

func (s *batchProcessService) RunEOD(ctx context.Context, executedBy uuid.UUID) (*domain.EODSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}

	if curDate.Status == domain.BusinessDateStatusClosed {
		return nil, domain.ErrEODAlreadyRunForDate
	}

	// 1. Mark status as IN_EOD_PROCESSING
	if err := s.dateRepo.SetStatus(ctx, domain.BusinessDateStatusEOD); err != nil {
		return nil, fmt.Errorf("failed to lock system for EOD: %w", err)
	}

	// 2. Calculate next business date (+1 day)
	nextDate := curDate.CurrentDate.AddDate(0, 0, 1)

	// 3. Advance system business date and reopen system
	if err := s.dateRepo.AdvanceDate(ctx, nextDate, executedBy); err != nil {
		return nil, fmt.Errorf("failed to advance business date: %w", err)
	}

	// Ringkasan dihitung dari jurnal tanggal bisnis yang ditutup, bukan konstanta.
	activity := &domain.DailyActivitySummary{}
	if s.batchRepo != nil {
		activity, err = s.batchRepo.DailyActivity(ctx, curDate.CurrentDate)
		if err != nil {
			return nil, fmt.Errorf("menghitung aktivitas harian: %w", err)
		}
	}

	return &domain.EODSummaryResult{
		ExecutedDate:               curDate.CurrentDate,
		NextBusinessDate:           nextDate,
		TotalPostedJournalsToday:   activity.PostedJournals,
		TotalDepositAmountToday:    activity.TotalDepositAmount,
		TotalWithdrawalAmountToday: activity.TotalWithdrawalAmount,
		ExecutedBy:                 executedBy,
		CompletedAt:                time.Now().UTC(),
	}, nil
}

// RunEOM menjalankan akrual bunga/bagi hasil tabungan periode berjalan dan memotong
// biaya administrasi bulanan. Tidak ada tarif yang diterima dari pemanggil: tarif
// dibaca dari produk dan system_config. Ringkasan diisi dari hasil nyata tiap rekening.
func (s *batchProcessService) RunEOM(ctx context.Context, executedBy uuid.UUID) (*domain.EOMSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}
	if s.savingsSvc == nil {
		return nil, errors.New("layanan akrual tabungan belum dikonfigurasi")
	}

	period := time.Date(curDate.CurrentDate.Year(), curDate.CurrentDate.Month(), 1, 0, 0, 0, 0, time.UTC)
	createdBy := executedBy.String()

	interest, err := s.savingsSvc.AccrueAll(ctx, period, "", createdBy)
	if err != nil {
		return nil, fmt.Errorf("akrual bunga tabungan: %w", err)
	}

	// Akrual disimpan sebagai utang; pembayaran memindahkannya ke rekening nasabah.
	payment, err := s.savingsSvc.PayInterestToAccounts(ctx, period, createdBy)
	if err != nil {
		return nil, fmt.Errorf("pembayaran bunga tabungan: %w", err)
	}

	fees, err := s.savingsSvc.ChargeAdminFees(ctx, period, createdBy)
	if err != nil {
		return nil, fmt.Errorf("pemotongan biaya administrasi: %w", err)
	}

	processed := interest.ProcessedAccounts
	if fees.ProcessedAccounts > processed {
		processed = fees.ProcessedAccounts
	}

	return &domain.EOMSummaryResult{
		ExecutedMonth:          period.Format("2006-01"),
		TotalAdminFeesDeducted: fees.TotalAdminFees,
		TotalInterestPaid:      payment.TotalInterest,
		ProcessedAccounts:      processed,
		FailedAccounts:         interest.FailedAccounts + payment.FailedAccounts + fees.FailedAccounts,
		CompletedAt:            time.Now().UTC(),
	}, nil
}

// RunEOY memposting jurnal penutup tahun buku: saldo akun pendapatan dan beban
// dipindahkan ke laba ditahan. book kosong berarti seluruh buku diproses, tetapi
// setiap buku ditutup terpisah agar konvensional dan syariah tidak tercampur.
func (s *batchProcessService) RunEOY(ctx context.Context, book string, executedBy uuid.UUID) (*domain.EOYSummaryResult, error) {
	curDate, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, err
	}
	if s.yearEndRepo == nil || s.posting == nil || s.db == nil {
		return nil, errors.New("dependensi tutup buku belum dikonfigurasi")
	}

	books, err := resolveClosingBooks(book)
	if err != nil {
		return nil, err
	}

	fiscalYear := curDate.CurrentDate.Year()
	createdBy := executedBy.String()

	results := make([]domain.EOYBookResult, 0, len(books))
	var refs []string
	totalRevenue := decimal.Zero
	totalExpense := decimal.Zero
	netIncome := decimal.Zero

	for _, b := range books {
		result, err := s.closeBook(ctx, fiscalYear, b, createdBy)
		if err != nil {
			return nil, err
		}
		results = append(results, *result)
		totalRevenue = totalRevenue.Add(result.TotalRevenueClosed)
		totalExpense = totalExpense.Add(result.TotalExpenseClosed)
		netIncome = netIncome.Add(result.NetRetainedEarnings)
		if result.ClosingJournalRef != "" {
			refs = append(refs, result.ClosingJournalRef)
		}
	}

	return &domain.EOYSummaryResult{
		FiscalYear:          fiscalYear,
		TotalRevenueClosed:  totalRevenue,
		TotalExpenseClosed:  totalExpense,
		NetRetainedEarnings: netIncome,
		ClosingJournalRef:   strings.Join(refs, ", "),
		Books:               results,
		CompletedAt:         time.Now().UTC(),
	}, nil
}

// closeBook menutup satu buku: hitung saldo nominal dari jurnal, bentuk jurnal
// penutup, lalu posting sekaligus dengan penanda idempotensi. Jurnal yang tidak
// seimbang ditolak sebelum menyentuh database.
func (s *batchProcessService) closeBook(ctx context.Context, fiscalYear int, book domain.COABook, createdBy string) (*domain.EOYBookResult, error) {
	existing, err := s.yearEndRepo.GetYearEndClosing(ctx, fiscalYear, book)
	if err != nil {
		return nil, fmt.Errorf("membaca penanda tutup buku: %w", err)
	}
	if existing != nil {
		return existingClosingResult(existing), nil
	}

	start := time.Date(fiscalYear, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(fiscalYear, 12, 31, 0, 0, 0, 0, time.UTC)

	balances, err := s.yearEndRepo.NominalBalances(ctx, start, end, book)
	if err != nil {
		return nil, fmt.Errorf("membaca saldo akun nominal: %w", err)
	}

	retainedCOA := s.retainedEarningsCOA(ctx, book)
	entries, totalRevenue, totalExpense, netIncome, err := domain.ComputeClosingEntries(balances, retainedCOA)
	if err != nil {
		if errors.Is(err, domain.ErrNoNominalAccounts) {
			// Tidak ada yang perlu ditutup; tidak ada jurnal dan tidak ada penanda.
			return &domain.EOYBookResult{Book: book, RetainedEarningsCOACode: retainedCOA}, nil
		}
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	inserted, err := s.yearEndRepo.InsertYearEndClosing(ctx, tx, &domain.YearEndClosingRecord{
		FiscalYear:          fiscalYear,
		Book:                book,
		TotalRevenue:        totalRevenue,
		TotalExpense:        totalExpense,
		NetIncome:           netIncome,
		RetainedEarningsCOA: retainedCOA,
	})
	if err != nil {
		return nil, fmt.Errorf("menyimpan penanda tutup buku: %w", err)
	}
	if !inserted {
		// Balapan dengan eksekusi lain: pakai penanda yang sudah ada.
		tx.Rollback()
		existing, err := s.yearEndRepo.GetYearEndClosing(ctx, fiscalYear, book)
		if err != nil || existing == nil {
			return nil, fmt.Errorf("tutup buku %d/%s sudah berjalan tetapi penanda tidak terbaca", fiscalYear, book)
		}
		return existingClosingResult(existing), nil
	}

	lines := make([]domain.PostingLine, 0, len(entries))
	for _, e := range entries {
		accountNumber, err := s.resolver.ResolveGLAccount(ctx, tx, e.COACode)
		if err != nil {
			return nil, fmt.Errorf("akun GL %s: %w", e.COACode, err)
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: accountNumber,
			Direction:     e.Direction,
			Amount:        e.Amount,
			Description:   fmt.Sprintf("Penutupan tahun buku %d", fiscalYear),
		})
	}

	entry, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("Jurnal penutup tahun buku %d (%s)", fiscalYear, book),
		IdempotencyKey:  fmt.Sprintf("EOY-CLOSE-%d-%s", fiscalYear, book),
		CreatedBy:       createdBy,
		// Jurnal penutup harus masuk ke tahun fiskal yang ditutup, bukan tahun
		// saat batch dijalankan (bisa sudah lewat 31 Desember).
		EntryDate: time.Date(fiscalYear, 12, 31, 0, 0, 0, 0, time.UTC),
		Lines:     lines,
	})
	if err != nil {
		return nil, fmt.Errorf("posting jurnal penutup: %w", err)
	}

	if err := s.yearEndRepo.UpdateYearEndClosingJournal(ctx, tx, fiscalYear, book, entry.ID); err != nil {
		return nil, fmt.Errorf("menautkan jurnal penutup: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return &domain.EOYBookResult{
		Book:                    book,
		TotalRevenueClosed:      totalRevenue,
		TotalExpenseClosed:      totalExpense,
		NetRetainedEarnings:     netIncome,
		RetainedEarningsCOACode: retainedCOA,
		ClosingJournalRef:       entry.ReferenceNumber,
	}, nil
}

// retainedEarningsCOA menentukan akun laba ditahan per buku, dengan override konfigurasi.
func (s *batchProcessService) retainedEarningsCOA(ctx context.Context, book domain.COABook) string {
	fallback := defaultRetainedEarningsCOAConv
	key := configRetainedEarningsCOAConv
	if book == domain.BookSyariah {
		fallback = defaultRetainedEarningsCOASyar
		key = configRetainedEarningsCOASyar
	}
	if s.configSvc == nil {
		return fallback
	}
	return s.configSvc.GetString(ctx, key, fallback)
}

// resolveClosingBooks menerjemahkan parameter buku menjadi daftar buku yang ditutup.
// Kosong berarti kedua buku; nilai lain harus CONVENTIONAL atau SYARIAH.
func resolveClosingBooks(book string) ([]domain.COABook, error) {
	switch strings.ToUpper(strings.TrimSpace(book)) {
	case "":
		return []domain.COABook{domain.BookConventional, domain.BookSyariah}, nil
	case string(domain.BookConventional):
		return []domain.COABook{domain.BookConventional}, nil
	case string(domain.BookSyariah):
		return []domain.COABook{domain.BookSyariah}, nil
	default:
		return nil, fmt.Errorf("buku %q tidak dikenal; gunakan CONVENTIONAL atau SYARIAH", book)
	}
}

// existingClosingResult mengubah penanda tutup buku yang sudah ada menjadi ringkasan.
func existingClosingResult(rec *domain.YearEndClosingRecord) *domain.EOYBookResult {
	result := &domain.EOYBookResult{
		Book:                    rec.Book,
		TotalRevenueClosed:      rec.TotalRevenue,
		TotalExpenseClosed:      rec.TotalExpense,
		NetRetainedEarnings:     rec.NetIncome,
		RetainedEarningsCOACode: rec.RetainedEarningsCOA,
		AlreadyClosed:           true,
		ClosingJournalRef:       rec.JournalReference,
	}
	return result
}

var _ domain.BatchProcessService = (*batchProcessService)(nil)
