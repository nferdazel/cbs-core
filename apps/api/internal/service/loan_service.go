package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type loanService struct {
	db          *sql.DB
	loanRepo    domain.LoanRepository
	productRepo domain.ProductRepository
	accountRepo domain.AccountRepository
	poster      *ProductPoster
	references  domain.ReferenceGenerator
	config      domain.SystemConfigService
	auditRepo   domain.AuditRepository
	// txRunner membuka transaksi per kredit untuk akrual denda. Dipakai agar proses
	// batch bisa diuji tanpa database, sama seperti ppapService.
	txRunner ppapTxRunner
}

func NewLoanService(
	db *sql.DB,
	loanRepo domain.LoanRepository,
	productRepo domain.ProductRepository,
	accountRepo domain.AccountRepository,
	poster *ProductPoster,
	references domain.ReferenceGenerator,
	config domain.SystemConfigService,
	auditSinks ...domain.AuditRepository,
) domain.LoanService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &loanService{
		db:          db,
		loanRepo:    loanRepo,
		productRepo: productRepo,
		accountRepo: accountRepo,
		poster:      poster,
		references:  references,
		config:      config,
		auditRepo:   auditRepo,
		txRunner:    sqlPPAPTxRunner{db: db},
	}
}

func (s *loanService) ApplyLoan(ctx context.Context, input domain.ApplyLoanInput, actor domain.Actor) (*domain.Loan, error) {
	if input.PrincipalAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidLoanAmount
	}
	if input.TermMonths <= 0 {
		return nil, domain.ErrInvalidLoanTerm
	}

	product, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Family != domain.FamilyLoan {
		return nil, fmt.Errorf("produk %s bukan produk kredit/pembiayaan", product.Code)
	}
	if input.PrincipalAmount.LessThan(product.MinAmount) {
		return nil, fmt.Errorf("nominal di bawah minimum produk %s (%s)", product.Code, product.MinAmount.String())
	}
	if product.MaxAmount.IsPositive() && input.PrincipalAmount.GreaterThan(product.MaxAmount) {
		return nil, fmt.Errorf("nominal di atas maksimum produk %s (%s)", product.Code, product.MaxAmount.String())
	}
	if input.TermMonths < product.MinTermMonths || (product.MaxTermMonths > 0 && input.TermMonths > product.MaxTermMonths) {
		return nil, fmt.Errorf("jangka waktu di luar rentang produk %s (%d-%d bulan)", product.Code, product.MinTermMonths, product.MaxTermMonths)
	}

	acc, err := s.accountRepo.GetByID(ctx, input.DisbursementAccountID)
	if err != nil {
		return nil, errors.New("rekening pencairan tidak ditemukan")
	}
	if acc.CustomerID == nil || *acc.CustomerID != input.CustomerID {
		return nil, errors.New("rekening pencairan bukan milik nasabah yang mengajukan")
	}
	// Penegakan kepemilikan cabang: pencairan ke rekening cabang lain ditolak.
	// Rekening tanpa cabang (data pra-migrasi) dibiarkan lewat.
	if !actor.CanAccessBranch(acc.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}

	loanID := uuid.New()
	start := time.Now().UTC()

	method, profitType, margin := scheduleTermsFor(product, input.MarginAmount)
	schedules, totalPayable, monthly := domain.BuildSchedule(loanID, domain.ScheduleParams{
		Principal:  input.PrincipalAmount,
		AnnualRate: product.RateAnnual,
		Margin:     margin,
		TermMonths: input.TermMonths,
		StartDate:  start,
		Method:     method,
		ProfitType: profitType,
	})

	now := time.Now().UTC()
	loanNumber, err := s.references.Next(domain.TxTypeTransferInternal, now)
	if err != nil {
		return nil, fmt.Errorf("membuat nomor kredit: %w", err)
	}
	loan := &domain.Loan{
		ID:                    loanID,
		LoanNumber:            loanNumber,
		CustomerID:            input.CustomerID,
		ProductID:             &product.ID,
		// Cabang kredit mengikuti cabang rekening pencairan. Untuk aktor
		// non-lintas-cabang, penegakan di atas sudah memastikan cabang rekening
		// sama dengan cabang aktor; untuk aktor lintas cabang, cabang rekening
		// pencairan yang menjadi dasar. Bila rekening belum punya cabang (data
		// lama), dibiarkan NULL sampai migrasi backfill mengisinya.
		BranchID:              acc.BranchID,
		DisbursementAccountID: input.DisbursementAccountID,
		LoanType:              domain.LoanTypeFor(product),
		Status:                domain.LoanStatusPendingApproval,
		Collectibility:        domain.CollectibilityKol1,
		AccrualStatus:         domain.AccrualStatusAccrual,
		// required_ppap berarti cadangan yang SUDAH dibukukan untuk kredit ini, bukan
		// cadangan yang seharusnya. Kredit baru belum dicadangkan, jadi nilainya nol;
		// batch PPAP harian yang menetapkan target dan memposting selisihnya. Mengisi
		// angka di sini tanpa jurnal membuat GL cadangan tidak pernah menerima nominal
		// itu sehingga buku dan rekonsiliasi tidak cocok.
		RequiredPPAP:         decimal.Zero,
		PrincipalAmount:      input.PrincipalAmount,
		AcquisitionCost:      input.PrincipalAmount,
		DeferredMargin:       margin,
		InterestRateAnnual:   product.RateAnnual,
		MarginAmount:         margin,
		ProfitSharingRatio:   product.ProfitSharingRatio,
		TotalPayable:         totalPayable,
		TermMonths:           input.TermMonths,
		MonthlyInstallment:   monthly,
		OutstandingPrincipal: decimal.Zero,
		Purpose:              input.Purpose,
		AOID:                 &actor.UserID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := s.loanRepo.Create(ctx, loan, schedules); err != nil {
		return nil, fmt.Errorf("menyimpan pengajuan kredit: %w", err)
	}
	loan.Schedules = schedules
	return loan, nil
}

// customerAccountOverrides memvalidasi bahwa rekening nasabah berada pada buku yang
// sama dengan produk, lalu mengarahkan baris jurnal yang pada pemetaan produk menunjuk
// akun kontrol (mis. 20100 Tabungan) ke rekening nasabah yang sebenarnya. Tanpa ini
// dana tercatat di akun kontrol dan saldo rekening nasabah tidak pernah bergerak:
// pencairan tidak bisa dipakai dan angsuran tidak terpotong.
//
// Validasi buku penting karena dana UUS tidak boleh bercampur dengan konvensional.
// Bila produk syariah mencairkan ke rekening konvensional, kode COA rekening tidak
// akan cocok dengan pemetaan produk dan jurnal akan jatuh ke buku yang salah.
func customerAccountOverrides(product *domain.BankingProduct, acc *domain.Account) (map[string]string, error) {
	if acc == nil || acc.COACode == "" {
		return nil, errors.New("rekening nasabah tidak punya kode COA; jurnal tidak dapat dipetakan")
	}
	if product != nil && product.Book != "" && acc.COABook != "" && product.Book != acc.COABook {
		return nil, fmt.Errorf(
			"rekening %s berada pada buku %s sedangkan produk %s berada pada buku %s; buku tidak boleh dicampur",
			acc.AccountNumber, acc.COABook, product.Code, product.Book)
	}
	return map[string]string{acc.COACode: acc.AccountNumber}, nil
}

// scheduleTermsFor menentukan metode jadwal dan jenis imbal hasil dari produk.
func scheduleTermsFor(product *domain.BankingProduct, requestedMargin decimal.Decimal) (domain.ScheduleMethod, domain.ProfitType, decimal.Decimal) {
	switch product.ProfitScheme {
	case domain.SchemeMurabahah:
		return domain.ScheduleFlat, domain.ProfitTypeMargin, requestedMargin
	case domain.SchemeMudharabah, domain.SchemeMusyarakah:
		// Bagi hasil: margin diproyeksikan dari nisbah bila tidak diberikan eksplisit.
		return domain.ScheduleBagiHasil, domain.ProfitTypeBagiHasil, requestedMargin
	case domain.SchemeIjarah:
		return domain.ScheduleFlat, domain.ProfitTypeMargin, requestedMargin
	default:
		return product.ScheduleMethod, domain.ProfitTypeInterest, requestedMargin
	}
}

// canAccessLoan menegakkan kepemilikan cabang kredit. Kredit dengan branch_id
// NULL adalah data pra-migrasi yang cabangnya belum diketahui; baris seperti itu
// sengaja TIDAK ditolak agar operasional atas data lama tidak terblokir.
func canAccessLoan(actor domain.Actor, loan *domain.Loan) bool {
	return actor.CanAccessBranch(loan.BranchCode)
}

func (s *loanService) ApproveLoan(ctx context.Context, loanID uuid.UUID, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status != domain.LoanStatusPendingApproval {
		return nil, domain.ErrLoanAlreadyApproved
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.loanRepo.UpdateStatusTx(ctx, tx, loanID, domain.LoanStatusApproved, &actor.UserID); err != nil {
		return nil, fmt.Errorf("menyetujui kredit: %w", err)
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "APPROVE_LOAN", "loan", loan.ID.String(), map[string]any{
		"status": domain.LoanStatusApproved,
		"amount": loan.PrincipalAmount.String(),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	loan.Status = domain.LoanStatusApproved
	loan.ApprovedBy = &actor.UserID
	now := time.Now().UTC()
	loan.ApprovedAt = &now
	return loan, nil
}

func (s *loanService) RejectLoan(ctx context.Context, loanID uuid.UUID, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status != domain.LoanStatusPendingApproval {
		return nil, domain.ErrLoanAlreadyApproved
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.loanRepo.UpdateStatusTx(ctx, tx, loanID, domain.LoanStatusRejected, &actor.UserID); err != nil {
		return nil, fmt.Errorf("menolak kredit: %w", err)
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "REJECT_LOAN", "loan", loan.ID.String(), map[string]any{
		"status": domain.LoanStatusRejected,
		"amount": loan.PrincipalAmount.String(),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	loan.Status = domain.LoanStatusRejected
	return loan, nil
}

func (s *loanService) DisburseLoan(ctx context.Context, loanID uuid.UUID, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status == domain.LoanStatusDisbursed {
		return nil, domain.ErrLoanAlreadyDisbursed
	}
	if loan.Status != domain.LoanStatusApproved {
		return nil, domain.ErrLoanNotApproved
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}

	acc, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, errors.New("rekening pencairan tidak ditemukan")
	}
	overrides, err := customerAccountOverrides(product, acc)
	if err != nil {
		return nil, err
	}

	// Pencairan dan update status loan berada dalam satu transaksi: bila jurnal gagal,
	// status loan tidak boleh berubah.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	desc := fmt.Sprintf("Pencairan %s untuk nasabah rekening %s", product.Name, acc.AccountNumber)
	_, err = s.poster.PostEventTx(ctx, tx, product, domain.EventLoanDisbursement, Amounts{
		Principal: loan.PrincipalAmount,
		Total:     loan.PrincipalAmount,
	}, PostingMeta{
		TransactionType:  domain.TxTypeTransferInternal,
		Description:      desc,
		IdempotencyKey:   "DISB-" + loan.LoanNumber,
		CreatedBy:        actor.DisplayName(),
		BranchCode:       actor.BranchCode,
		AccountOverrides: overrides,
	})
	if err != nil {
		return nil, fmt.Errorf("jurnal pencairan: %w", err)
	}

	// Dana masuk ke rekening nasabah: debit akun nasabah pada jurnal mapping produk;
	// liabilitas bank bertambah. Loan outstanding diisi pokok penuh.
	if err := s.loanRepo.MarkDisbursedTx(ctx, tx, loan.ID, loan.PrincipalAmount); err != nil {
		return nil, fmt.Errorf("menandai kredit dicairkan: %w", err)
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "DISBURSE_LOAN", "loan", loan.ID.String(), map[string]any{
		"status": domain.LoanStatusDisbursed,
		"amount": loan.PrincipalAmount.String(),
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	loan.Status = domain.LoanStatusDisbursed
	loan.OutstandingPrincipal = loan.PrincipalAmount
	now := time.Now().UTC()
	loan.DisbursedAt = &now
	return loan, nil
}

func (s *loanService) GetLoan(ctx context.Context, id uuid.UUID) (*domain.Loan, error) {
	return s.loanRepo.GetByID(ctx, id)
}

func (s *loanService) ListLoans(ctx context.Context, page, pageSize int) ([]domain.Loan, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.loanRepo.List(ctx, pageSize, (page-1)*pageSize)
}

// PayInstallment mencatat pembayaran angsuran. Menolak bila kredit belum dicairkan.
// Nominal yang dibayar diambil dari jadwal bila tidak diberikan.
func (s *loanService) PayInstallment(ctx context.Context, input domain.PayInstallmentInput, actor domain.Actor) (*domain.LoanSchedule, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, errors.New("kredit tidak dalam status aktif")
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}

	schedules, err := s.loanRepo.GetSchedules(ctx, loan.ID)
	if err != nil {
		return nil, err
	}
	payAccount, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, errors.New("rekening pembayaran angsuran tidak ditemukan")
	}
	overrides, err := customerAccountOverrides(product, payAccount)
	if err != nil {
		return nil, err
	}
	var target *domain.LoanSchedule
	for i := range schedules {
		if schedules[i].InstallmentNo == input.InstallmentNo {
			target = &schedules[i]
			break
		}
	}
	if target == nil {
		return nil, errors.New("jadwal angsuran tidak ditemukan")
	}
	if target.Status == domain.InstallmentStatusPaid {
		return nil, errors.New("angsuran ini sudah dibayar penuh")
	}

	outstandingPrincipal := target.PrincipalAmount.Sub(target.PaidPrincipal)
	outstandingProfit := target.ProfitAmount.Sub(target.PaidProfit)

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Pokok dan imbal hasil dijurnal terpisah karena akun pendapatannya berbeda.
	// Kunci idempotency wajib berbeda per jurnal: kolomnya unik, sehingga memakai
	// kunci yang sama untuk keduanya membuat jurnal kedua ditolak.
	baseKey := fmt.Sprintf("INST-%s-%d", loan.LoanNumber, input.InstallmentNo)
	meta := PostingMeta{
		TransactionType:  domain.TxTypeTransferInternal,
		Description:      fmt.Sprintf("Angsuran ke-%d kredit %s", input.InstallmentNo, loan.LoanNumber),
		IdempotencyKey:   baseKey + "-PRIN",
		CreatedBy:        actor.DisplayName(),
		BranchCode:       actor.BranchCode,
		AccountOverrides: overrides,
	}

	if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanPrincipalPay, Amounts{
		Principal: outstandingPrincipal, Total: outstandingPrincipal,
	}, meta); err != nil {
		return nil, fmt.Errorf("jurnal angsuran pokok: %w", err)
	}
	if outstandingProfit.IsPositive() {
		meta.IdempotencyKey = baseKey + "-PROF"
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanProfitPay, Amounts{
			Profit: outstandingProfit, Total: outstandingProfit,
		}, meta); err != nil {
			return nil, fmt.Errorf("jurnal angsuran imbal hasil: %w", err)
		}
	}

	if err := s.loanRepo.UpdateSchedulePayment(ctx, target.ID, outstandingPrincipal, outstandingProfit, domain.InstallmentStatusPaid); err != nil {
		return nil, fmt.Errorf("mencatat pembayaran angsuran: %w", err)
	}

	newOutstanding := loan.OutstandingPrincipal.Sub(outstandingPrincipal)
	if newOutstanding.IsNegative() {
		newOutstanding = decimal.Zero
	}
	if err := s.loanRepo.UpdateOutstanding(ctx, loan.ID, newOutstanding, loan.PenaltyAccrued); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	target.PaidPrincipal = target.PrincipalAmount
	target.PaidProfit = target.ProfitAmount
	target.Status = domain.InstallmentStatusPaid
	now := time.Now().UTC()
	target.PaidAt = &now

	// Pelunasan: bila seluruh jadwal sudah dibayar, kredit menjadi lunas.
	if s.allSchedulesPaid(ctx, loan.ID) {
		_ = s.loanRepo.UpdateStatus(ctx, loan.ID, domain.LoanStatusPaidOff, &actor.UserID)
		s.loanRepo.UpdateOutstanding(ctx, loan.ID, decimal.Zero, loan.PenaltyAccrued)
	}

	return target, nil
}

func (s *loanService) allSchedulesPaid(ctx context.Context, loanID uuid.UUID) bool {
	schedules, err := s.loanRepo.GetSchedules(ctx, loanID)
	if err != nil {
		return false
	}
	for _, sc := range schedules {
		if sc.Status != domain.InstallmentStatusPaid {
			return false
		}
	}
	return len(schedules) > 0
}

func (s *loanService) RestructureLoan(ctx context.Context, input domain.RestructureLoanInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, errors.New("hanya kredit aktif yang dapat direstrukturisasi")
	}
	if input.NewTermMonths <= 0 {
		return nil, errors.New("jangka waktu baru harus positif")
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	loan.IsRestructured = true
	loan.RestructuredCount++
	loan.RestructuredAt = &now
	loan.RestructuringReason = input.Reason
	loan.TermMonths = input.NewTermMonths

	if input.NewInterestRateAnnual.IsPositive() {
		loan.InterestRateAnnual = input.NewInterestRateAnnual
	}
	if input.NewMarginAmount.IsPositive() {
		loan.MarginAmount = input.NewMarginAmount
		loan.DeferredMargin = input.NewMarginAmount
	}

	// Kolektibilitas mengikuti DPD kredit dari konfigurasi yang sama dengan proses
	// PPAP harian, bukan aturan tersendiri. required_ppap sengaja TIDAK dihitung ulang
	// di sini: nilainya berarti cadangan yang sudah dibukukan, dan hanya batch PPAP
	// yang boleh mengubahnya karena ia pula yang memposting selisih jurnalnya.
	col, accrual := CollectibilityForDPD(ctx, s.config, loan.DPD)
	loan.Collectibility = col.OJKCode()
	loan.AccrualStatus = accrual

	method, profitType, margin := scheduleTermsFor(product, loan.MarginAmount)
	schedules, totalPayable, monthly := domain.BuildSchedule(loan.ID, domain.ScheduleParams{
		Principal:  loan.OutstandingPrincipal,
		AnnualRate: loan.InterestRateAnnual,
		Margin:     margin,
		TermMonths: loan.TermMonths,
		StartDate:  now,
		Method:     method,
		ProfitType: profitType,
	})
	loan.TotalPayable = totalPayable
	loan.MonthlyInstallment = monthly
	loan.Schedules = schedules

	if err := s.loanRepo.UpdateRestructure(ctx, loan, schedules); err != nil {
		return nil, fmt.Errorf("menyimpan restrukturisasi: %w", err)
	}
	return loan, nil
}

func (s *loanService) WriteOffLoan(ctx context.Context, input domain.WriteOffLoanInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, errors.New("hanya kredit aktif yang dapat dihapus buku")
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	amount := loan.OutstandingPrincipal
	if !amount.IsPositive() {
		amount = loan.PrincipalAmount
	}
	if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanWriteOff, Amounts{
		Principal: amount, Total: amount,
	}, PostingMeta{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("Hapus buku kredit %s: %s", loan.LoanNumber, input.Reason),
		IdempotencyKey:  "WOFF-" + loan.LoanNumber,
		CreatedBy:       actor.DisplayName(),
		BranchCode:      actor.BranchCode,
	}); err != nil {
		return nil, fmt.Errorf("jurnal hapus buku: %w", err)
	}

	if err := s.loanRepo.UpdateStatus(ctx, loan.ID, domain.LoanStatusWrittenOff, &actor.UserID); err != nil {
		return nil, err
	}
	if err := s.loanRepo.UpdateOutstanding(ctx, loan.ID, decimal.Zero, loan.PenaltyAccrued); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	loan.Status = domain.LoanStatusWrittenOff
	loan.OutstandingPrincipal = decimal.Zero
	return loan, nil
}

func (s *loanService) RecoverWrittenOffLoan(ctx context.Context, input domain.RecoverWrittenOffLoanInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if loan.Status != domain.LoanStatusWrittenOff {
		return nil, errors.New("kredit tidak berstatus hapus buku")
	}
	if input.RecoveryAmount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("nominal recovery harus positif")
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}
	recoveryAccount, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, errors.New("rekening recovery tidak ditemukan")
	}
	overrides, err := customerAccountOverrides(product, recoveryAccount)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanRecovery, Amounts{
		Principal: input.RecoveryAmount, Total: input.RecoveryAmount,
	}, PostingMeta{
		TransactionType:  domain.TxTypeAdjustment,
		Description:      fmt.Sprintf("Recovery kredit hapus buku %s", loan.LoanNumber),
		IdempotencyKey:   fmt.Sprintf("RECOV-%s-%d", loan.LoanNumber, time.Now().Unix()),
		CreatedBy:        actor.DisplayName(),
		BranchCode:       actor.BranchCode,
		AccountOverrides: overrides,
	}); err != nil {
		return nil, fmt.Errorf("jurnal recovery: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return loan, nil
}
