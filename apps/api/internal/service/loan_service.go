package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// loanLedger menggabungkan resolusi akun GL dengan operasi jurnal yang dibutuhkan
// pembatalan pencairan. Satu implementasi produksi (LedgerRepository postgres)
// memenuhi keduanya, sehingga service tidak perlu ketergantungan melingkar ke
// ledgerService hanya untuk membaca dan menandai jurnal.
type loanLedger interface {
	domain.AccountResolver
	domain.LedgerRepository
}

type loanService struct {
	db          *sql.DB
	loanRepo    domain.LoanRepository
	productRepo domain.ProductRepository
	accountRepo domain.AccountRepository
	resolver    domain.AccountResolver
	// ledgerRepo dipakai membalik jurnal pencairan saat pencairan dibatalkan.
	ledgerRepo domain.LedgerRepository
	poster     *ProductPoster
	posting    domain.PostingService
	references domain.ReferenceGenerator
	config     domain.SystemConfigService
	auditRepo  domain.AuditRepository
	// dates membaca tanggal bisnis berjalan. Jurnal kontra harus bertanggal bisnis,
	// bukan tanggal kalender; boleh nil, dan bila nil pembatalan ditolak alih-alih
	// menebak tanggal.
	dates domain.BusinessDateRepository
	// approvals menahan operasi kredit yang wajib disetujui pejabat kedua. Boleh nil
	// (mis. pada test atau bila maker-checker belum disiapkan): operasi berjalan
	// langsung seperti sebelumnya.
	approvals domain.MakerCheckerService
	// txRunner membuka transaksi per kredit untuk akrual denda/bunga. Dipakai agar
	// proses batch dan pembayaran bisa diuji tanpa database, sama seperti ppapService.
	txRunner ppapTxRunner
}

func NewLoanService(
	db *sql.DB,
	loanRepo domain.LoanRepository,
	productRepo domain.ProductRepository,
	accountRepo domain.AccountRepository,
	ledger loanLedger,
	poster *ProductPoster,
	posting domain.PostingService,
	references domain.ReferenceGenerator,
	config domain.SystemConfigService,
	approvals domain.MakerCheckerService,
	dates domain.BusinessDateRepository,
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
		resolver:    ledger,
		ledgerRepo:  ledger,
		poster:      poster,
		posting:     posting,
		references:  references,
		config:      config,
		auditRepo:   auditRepo,
		dates:       dates,
		approvals:   approvals,
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
		return nil, fmt.Errorf("%w %s (%s)", domain.ErrProductAmountBelowMin, product.Code, product.MinAmount.String())
	}
	if product.MaxAmount.IsPositive() && input.PrincipalAmount.GreaterThan(product.MaxAmount) {
		return nil, fmt.Errorf("%w %s (%s)", domain.ErrProductAmountAboveMax, product.Code, product.MaxAmount.String())
	}
	if input.TermMonths < product.MinTermMonths || (product.MaxTermMonths > 0 && input.TermMonths > product.MaxTermMonths) {
		return nil, fmt.Errorf("%w %s (%d-%d bulan)", domain.ErrProductTermOutOfRange, product.Code, product.MinTermMonths, product.MaxTermMonths)
	}

	acc, err := s.accountRepo.GetByID(ctx, input.DisbursementAccountID)
	if err != nil {
		return nil, domain.ErrDisbursementAccountNotFound
	}
	if acc.CustomerID == nil || *acc.CustomerID != input.CustomerID {
		return nil, domain.ErrDisbursementAccountNotOwned
	}
	// Penegakan kepemilikan cabang: pencairan ke rekening cabang lain ditolak.
	// Rekening tanpa cabang (data pra-migrasi) dibiarkan lewat.
	if !actor.CanAccessBranch(acc.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Pengajuan kredit mengikuti buku produk: pegawai satu buku tidak boleh
	// mengajukan produk buku lain. Produk tanpa buku (data lama) lolos.
	if !actor.CanAccessBook(product.Book) {
		return nil, domain.ErrCrossBookAccess
	}

	loanID := uuid.New()
	start := time.Now().UTC()

	method, profitType, margin, err := scheduleTermsFor(product, input.MarginAmount, input.PrincipalAmount, input.TermMonths)
	if err != nil {
		return nil, err
	}
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
	loanNumber, err := s.references.NextLoanNumber(ctx, product, now)
	if err != nil {
		return nil, fmt.Errorf("membuat nomor kredit: %w", err)
	}
	loan := &domain.Loan{
		ID:         loanID,
		LoanNumber: loanNumber,
		CustomerID: input.CustomerID,
		ProductID:  &product.ID,
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
		return nil, domain.ErrLoanAccountCOAMissing
	}
	if product != nil && product.Book != "" && acc.COABook != "" && product.Book != acc.COABook {
		return nil, fmt.Errorf(
			"rekening %s berada pada buku %s sedangkan produk %s berada pada buku %s; buku tidak boleh dicampur",
			acc.AccountNumber, acc.COABook, product.Code, product.Book)
	}
	return map[string]string{acc.COACode: acc.AccountNumber}, nil
}

// scheduleTermsFor menentukan metode jadwal dan jenis imbal hasil dari produk.
// margin nominal yang dikirim pemanggil dipakai apa adanya untuk murabahah/ijarah.
// Untuk akad bagi hasil (mudharabah/musyarakah), margin nominal hanya dipakai bila
// diisi eksplisit; selain itu proyeksi dihitung dari nisbah x proyeksi pendapatan
// usaha produk (angka PROYEKSI, disesuaikan saat realisasi bagi hasil aktual).
func scheduleTermsFor(product *domain.BankingProduct, requestedMargin, principal decimal.Decimal, termMonths int) (domain.ScheduleMethod, domain.ProfitType, decimal.Decimal, error) {
	switch product.ProfitScheme {
	case domain.SchemeMurabahah:
		return domain.ScheduleFlat, domain.ProfitTypeMargin, requestedMargin, nil
	case domain.SchemeMudharabah, domain.SchemeMusyarakah:
		if requestedMargin.IsPositive() {
			return domain.ScheduleBagiHasil, domain.ProfitTypeBagiHasil, requestedMargin, nil
		}
		margin, err := domain.ProjectBagiHasilMargin(principal, product.ProfitSharingRatio, product.ProjectedRevenueRateAnnual, termMonths)
		if err != nil {
			return "", "", decimal.Zero, err
		}
		return domain.ScheduleBagiHasil, domain.ProfitTypeBagiHasil, margin, nil
	case domain.SchemeIjarah:
		return domain.ScheduleFlat, domain.ProfitTypeMargin, requestedMargin, nil
	default:
		return product.ScheduleMethod, domain.ProfitTypeInterest, requestedMargin, nil
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
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if loan.Status != domain.LoanStatusPendingApproval {
		return nil, domain.ErrLoanAlreadyApproved
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

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

func (s *loanService) RejectLoan(ctx context.Context, loanID uuid.UUID, reason string, actor domain.Actor) (*domain.Loan, error) {
	// Alasan adalah bagian dari keputusan, bukan pelengkap: penolakan tanpa alasan
	// tidak dapat dipertanggungjawabkan, jadi ditolak sebelum menyentuh database.
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, domain.ErrLoanRejectionReasonRequired
	}
	if utf8.RuneCountInString(reason) > domain.MaxLoanRejectionReasonLen {
		return nil, domain.ErrLoanRejectionReasonTooLong
	}

	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if loan.Status != domain.LoanStatusPendingApproval {
		return nil, domain.ErrLoanAlreadyApproved
	}

	// Status dan alasan disimpan dalam satu transaksi bersama jejak audit: penolakan
	// tanpa alasan (atau alasan tanpa status) tidak boleh pernah commit terpisah.
	err = s.txRunner.Run(ctx, func(tx any) error {
		if err := s.loanRepo.RejectLoanTx(ctx, tx, loanID, reason); err != nil {
			return fmt.Errorf("menolak kredit: %w", err)
		}
		return writeAuditWithMetadata(ctx, s.auditRepo, tx, actor, "REJECT_LOAN", "loan", loan.ID.String(),
			map[string]any{
				"status": domain.LoanStatusRejected,
				"amount": loan.PrincipalAmount.String(),
				"reason": reason,
			},
			// Alasan juga ditulis ke metadata: pembaca audit lama membaca kolom itu,
			// sedangkan changes.reason tetap dipertahankan bagi pembaca yang sudah
			// bergantung padanya.
			map[string]any{
				"reason": reason,
			})
	})
	if err != nil {
		return nil, err
	}
	loan.Status = domain.LoanStatusRejected
	loan.RejectionReason = reason
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
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
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
		return nil, domain.ErrDisbursementAccountNotFound
	}
	overrides, err := customerAccountOverrides(product, acc)
	if err != nil {
		return nil, err
	}

	// Pencairan dan update status loan berada dalam satu transaksi: bila jurnal gagal,
	// status loan tidak boleh berubah. Baris kredit dikunci lebih dulu dan seluruh
	// angka diambil dari baris hasil kunci — snapshot di luar transaksi bisa sudah
	// basi (disiplin yang sama dengan pembayaran angsuran, pembatalan, dan koreksi).
	var result *domain.Loan
	err = s.txRunner.Run(ctx, func(tx any) error {
		fresh, err := s.loanRepo.LockLoanTx(ctx, tx, loan.ID)
		if err != nil {
			return fmt.Errorf("mengunci kredit %s: %w", loan.ID, err)
		}
		if !canAccessLoan(actor, fresh) {
			return domain.ErrCrossBranchAccess
		}
		if !s.canAccessLoanBook(ctx, actor, fresh) {
			return domain.ErrCrossBookAccess
		}
		// Pemeriksaan status di pemanggil hanya gagal-cepat; yang otoritatif adalah
		// baris hasil kunci. Kredit yang sudah cair tidak boleh dicairkan dua kali,
		// dan status selain APPROVED tetap ditolak.
		if fresh.Status == domain.LoanStatusDisbursed {
			return domain.ErrLoanAlreadyDisbursed
		}
		if fresh.Status != domain.LoanStatusApproved {
			return domain.ErrLoanNotApproved
		}

		// EIR orisinal (Pasal 32 POJK 1/2024 jo. PA BPR Bab 5.2) DIHITUNG DAN DISIMPAN
		// SELALU saat pencairan, terlepas dari saklar loan.restructure.loss.enabled.
		// Perhitungan ini murni pencatatan data: tidak ada jurnal, tidak mengubah
		// perilaku transaksi, dan tidak memengaruhi laporan. Saklar itu kini hanya
		// menyisakan pengakuan kerugian beserta amortisasinya (restructure_loss_service.go,
		// restructure_loss_amortization.go).
		//
		// ALASAN pemisahan: saklar itu MATI secara bawaan. Bila EIR ikut di belakangnya,
		// seluruh kredit yang cair sebelum bank menyalakannya tidak punya EIR; saat saklar
		// dinyalakan, restrukturisasi kredit-kredit itu ditolak ErrEIRMissing — dan EIR-nya
		// tidak bisa direkonstruksi setelah fakta karena arus kas aslinya sudah lewat.
		// Jalan buntu yang menunggu waktu.
		//
		// Pencairan adalah transaksi yang tidak boleh gagal karena data pelengkap: bila
		// perhitungan tidak konvergen, storeOriginalEIR MENCATAT ketidaktersediaan beserta
		// alasannya alih-alih mengembalikan galat. Kegagalan tulis infrastruktur tetap
		// menggagalkan transaksi — tanpa baris tersimpan, status EIR tidak dapat
		// direpresentasikan sama sekali dan rollback lebih jujur daripada diam-diam kosong.
		schedules, err := s.loanRepo.GetSchedulesTx(ctx, tx, fresh.ID)
		if err != nil {
			return fmt.Errorf("membaca jadwal angsuran untuk EIR: %w", err)
		}
		fresh.Schedules = schedules
		if err := s.storeOriginalEIR(ctx, tx, fresh, schedules); err != nil {
			return err
		}

		desc := fmt.Sprintf("Pencairan %s untuk nasabah rekening %s", product.Name, acc.AccountNumber)
		_, err = s.poster.PostEventTx(ctx, tx, product, domain.EventLoanDisbursement, Amounts{
			Principal: fresh.PrincipalAmount,
			Total:     fresh.PrincipalAmount,
		}, PostingMeta{
			TransactionType:  domain.TxTypeTransferInternal,
			Description:      desc,
			IdempotencyKey:   "DISB-" + fresh.LoanNumber,
			CreatedBy:        actor.DisplayName(),
			BranchCode:       actor.BranchCode,
			AccountOverrides: overrides,
		})
		if err != nil {
			return fmt.Errorf("jurnal pencairan: %w", err)
		}

		// Dana masuk ke rekening nasabah: debit akun nasabah pada jurnal mapping produk;
		// liabilitas bank bertambah. Loan outstanding diisi pokok penuh.
		if err := s.loanRepo.MarkDisbursedTx(ctx, tx, fresh.ID, fresh.PrincipalAmount); err != nil {
			return fmt.Errorf("menandai kredit dicairkan: %w", err)
		}
		if err := writeAudit(ctx, s.auditRepo, tx, actor, "DISBURSE_LOAN", "loan", fresh.ID.String(), map[string]any{
			"status": domain.LoanStatusDisbursed,
			"amount": fresh.PrincipalAmount.String(),
		}); err != nil {
			return err
		}

		fresh.Status = domain.LoanStatusDisbursed
		fresh.OutstandingPrincipal = fresh.PrincipalAmount
		now := time.Now().UTC()
		fresh.DisbursedAt = &now
		result = fresh
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *loanService) GetLoan(ctx context.Context, id uuid.UUID, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Kredit cabang lain ditolak. Pembacaan ini disamarkan menjadi 404 oleh
	// pemanggil (lihat Fail): status 404 dipertahankan agar keberadaan data tidak
	// bocor, sedangkan operasi tulis lintas cabang tetap dibalas 403.
	// Kredit tanpa cabang (data pra-migrasi) tetap boleh dibaca.
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Kredit buku lain juga ditolak; buku dibaca dari produk kredit.
	if !actor.CanAccessBook(s.loanBook(ctx, loan)) {
		return nil, domain.ErrCrossBookAccess
	}
	return loan, nil
}

// canAccessLoanBook menegakkan kepemilikan buku kredit. Dipakai bersama canAccessLoan
// pada SETIAP operasi tulis kredit (persetujuan, penolakan, pencairan, angsuran,
// restrukturisasi, hapus buku, recovery, pembatalan, dan koreksi nominal), sehingga
// pegawai satu buku tidak dapat mengubah kredit buku lain. Kredit tanpa produk (data
// lama) atau produk yang tidak terbaca mengembalikan buku kosong, yang oleh
// CanAccessBook diizinkan agar operasional data lama tidak terblokir.
func (s *loanService) canAccessLoanBook(ctx context.Context, actor domain.Actor, loan *domain.Loan) bool {
	return actor.CanAccessBook(s.loanBook(ctx, loan))
}

// loanBook mengembalikan buku produk kredit. Kredit tanpa produk (data lama) atau
// produk yang tidak terbaca mengembalikan buku kosong, yang oleh CanAccessBook
// diizinkan agar data lama tidak hilang dari operasional.
func (s *loanService) loanBook(ctx context.Context, loan *domain.Loan) domain.COABook {
	if loan.ProductID == nil {
		return ""
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil || product == nil {
		return ""
	}
	return product.Book
}

func (s *loanService) ListLoans(ctx context.Context, page, pageSize int, actor domain.Actor) ([]domain.Loan, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.loanRepo.List(ctx, pageSize, (page-1)*pageSize, actor)
}

// PayInstallment mencatat pembayaran angsuran. Menolak bila kredit belum dicairkan.
// Nominal yang dibayar diambil dari jadwal bila tidak diberikan.
func (s *loanService) PayInstallment(ctx context.Context, input domain.PayInstallmentInput, actor domain.Actor) (*domain.LoanSchedule, error) {
	method := input.Method
	if method == "" {
		method = domain.LoanPaymentAccount
	}
	if method != domain.LoanPaymentAccount && method != domain.LoanPaymentCash {
		return nil, fmt.Errorf("metode pembayaran %q tidak dikenal", input.Method)
	}

	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, domain.ErrLoanNotActive
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}

	payAccount, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, errors.New("rekening pembayaran angsuran tidak ditemukan")
	}

	// Hasil perhitungan di dalam transaksi disimpan ke variabel luar untuk dipakai
	// setelah commit (nilai kembalian dan pemeriksaan pelunasan).
	var (
		target            *domain.LoanSchedule
		allocation        domain.PaymentAllocation
		settle            decimal.Decimal
		installmentStatus domain.InstallmentStatus
		touchesSchedule   bool
		newPenalty        decimal.Decimal
	)

	err = s.txRunner.Run(ctx, func(tx any) error {
		// Kunci baris kredit lebih dulu, lalu baca jadwal di dalam kunci. Jadwal yang
		// dibaca di luar transaksi bisa sudah basi: koreksi nominal yang commit di
		// antaranya mengganti pokok tiap angsuran, sehingga pembayaran menghitung
		// kewajiban dari angka lama. Kunci yang sama juga menyerialkan pembayaran
		// dengan pembatalan pencairan, sehingga pembayaran tidak pernah jatuh pada
		// kredit yang sudah CANCELLED.
		locked, err := s.loanRepo.LockLoanTx(ctx, tx, loan.ID)
		if err != nil {
			return err
		}
		if locked.Status != domain.LoanStatusDisbursed {
			return domain.ErrLoanNotActive
		}
		loan = locked

		schedules, err := s.loanRepo.GetSchedulesTx(ctx, tx, loan.ID)
		if err != nil {
			return err
		}
		for i := range schedules {
			if schedules[i].InstallmentNo == input.InstallmentNo {
				target = &schedules[i]
				break
			}
		}
		if target == nil {
			return domain.ErrInstallmentNotFound
		}
		if target.Status == domain.InstallmentStatusPaid {
			return domain.ErrInstallmentAlreadyPaid
		}

		outstandingPrincipal := target.PrincipalAmount.Sub(target.PaidPrincipal)
		outstandingProfit := target.ProfitAmount.Sub(target.PaidProfit)
		if outstandingPrincipal.IsNegative() {
			outstandingPrincipal = decimal.Zero
		}
		if outstandingProfit.IsNegative() {
			outstandingProfit = decimal.Zero
		}

		// Denda adalah kewajiban tingkat kredit, bukan tingkat angsuran, sehingga seluruh
		// denda yang sudah diakru ditagih lebih dulu. Sebelum ini denda tidak punya jalur
		// pelunasan sama sekali: piutangnya menumpuk di 10305/11700 sementara kredit bisa
		// dinyatakan lunas dengan denda masih menggantung.
		penaltyDue := loan.PenaltyAccrued
		if penaltyDue.IsNegative() {
			penaltyDue = decimal.Zero
		}

		// Tanpa nominal, pemanggil meminta pelunasan penuh angsuran ini beserta dendanya.
		amount := input.Amount
		if !amount.IsPositive() {
			amount = penaltyDue.Add(outstandingProfit).Add(outstandingPrincipal)
		}
		allocation = domain.AllocateLoanPayment(amount, penaltyDue, outstandingProfit, outstandingPrincipal)
		if allocation.Unapplied.IsPositive() {
			return fmt.Errorf(
				"pembayaran %s melebihi kewajiban angsuran ke-%d kredit %s (%s); kelebihan belum dapat diterima",
				amount.StringFixed(2), input.InstallmentNo, loan.LoanNumber,
				penaltyDue.Add(outstandingProfit).Add(outstandingPrincipal).StringFixed(2))
		}

		// settle adalah porsi bunga yang diselesaikan dari akruan yang sudah terbentuk.
		// min(porsi bunga yang dibayar, profit_accrued_amount) memastikan piutang bunga
		// 10400 tidak pernah negatif dan pendapatan satu angsuran tidak melebihi porsi
		// bunga jadwal.
		settle = allocation.Profit
		if settle.GreaterThan(target.ProfitAccruedAmount) {
			settle = target.ProfitAccruedAmount
		}
		if settle.IsNegative() {
			settle = decimal.Zero
		}

		touchesSchedule = allocation.Profit.IsPositive() || allocation.Principal.IsPositive()
		installmentStatus = domain.InstallmentStatusPartial
		if allocation.Profit.Equal(outstandingProfit) && allocation.Principal.Equal(outstandingPrincipal) {
			installmentStatus = domain.InstallmentStatusPaid
		}

		newOutstanding := loan.OutstandingPrincipal.Sub(allocation.Principal)
		if newOutstanding.IsNegative() {
			newOutstanding = decimal.Zero
		}
		newPenalty = penaltyDue.Sub(allocation.Penalty)
		if newPenalty.IsNegative() {
			newPenalty = decimal.Zero
		}

		// Kunci idempotensi memuat total bayar kumulatif setelah pembayaran ini: percobaan
		// ulang atas pembayaran yang sama menghasilkan kunci yang sama (ditolak sebagai
		// duplikat), sedangkan angsuran sebagian berikutnya menghasilkan kunci berbeda
		// karena totalnya sudah berubah. Kunci lama (-PRIN/-PROF) tidak memuat itu sehingga
		// angsuran sebagian kedua akan tertolak sebagai duplikat.
		baseKey := fmt.Sprintf("INST-%s-%d-%s-%s", loan.LoanNumber, input.InstallmentNo,
			target.PaidPrincipal.Add(allocation.Principal).StringFixed(2),
			target.PaidProfit.Add(allocation.Profit).StringFixed(2))
		meta := PostingMeta{
			TransactionType: domain.TxTypeTransferInternal,
			Description:     fmt.Sprintf("Angsuran ke-%d kredit %s", input.InstallmentNo, loan.LoanNumber),
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}

		// Sumber dana ditentukan di dalam transaksi: penerimaan tunai mengarahkan kaki
		// debit ke kas teller, sedangkan pembayaran lewat rekening mengarahkannya ke
		// rekening nasabah (bawaan).
		funding, err := s.fundingSource(ctx, tx, product, payAccount, method)
		if err != nil {
			return err
		}
		meta.AccountOverrides = funding.overrides

		if allocation.Penalty.IsPositive() {
			penaltyMeta := meta
			penaltyMeta.IdempotencyKey = baseKey + "-PEN"
			penaltyMeta.Description = fmt.Sprintf("Pembayaran denda kredit %s", loan.LoanNumber)
			if err := s.postPenaltyCollection(ctx, tx, product, penaltyMeta, funding.accountNumber, allocation.Penalty); err != nil {
				return fmt.Errorf("jurnal pembayaran denda: %w", err)
			}
		}
		if allocation.Principal.IsPositive() {
			principalMeta := meta
			principalMeta.IdempotencyKey = baseKey + "-PRIN"
			if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanPrincipalPay, Amounts{
				Principal: allocation.Principal, Total: allocation.Principal,
			}, principalMeta); err != nil {
				return fmt.Errorf("jurnal angsuran pokok: %w", err)
			}
		}
		if allocation.Profit.IsPositive() {
			profitMeta := meta
			profitMeta.IdempotencyKey = baseKey + "-PROF"
			if settle.IsPositive() {
				// Sebagian/seluruh bunga sudah diakru: pendapatan tidak lagi diakui
				// penuh di sini, melainkan memindahkan piutang bunga 10400.
				if err := s.postProfitSettlement(ctx, tx, product, profitMeta, allocation.Profit, settle); err != nil {
					return fmt.Errorf("jurnal angsuran imbal hasil: %w", err)
				}
			} else if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanProfitPay, Amounts{
				Profit: allocation.Profit, Total: allocation.Profit,
			}, profitMeta); err != nil {
				return fmt.Errorf("jurnal angsuran imbal hasil: %w", err)
			}
		}

		if touchesSchedule {
			if err := s.loanRepo.UpdateSchedulePaymentTx(ctx, tx, target.ID, allocation.Principal, allocation.Profit, settle, installmentStatus); err != nil {
				return fmt.Errorf("mencatat pembayaran angsuran: %w", err)
			}
		}
		// Jejak audit ditulis di dalam transaksi yang sama: pembayaran angsuran adalah
		// bukti penerimaan kas, jadi catatan yang gagal ditulis harus membatalkan
		// pembayarannya, bukan tertinggal diam-diam.
		if err := writeAudit(ctx, s.auditRepo, tx, actor, "PAY_INSTALLMENT", "loan", loan.ID.String(), map[string]any{
			"loan_number":           loan.LoanNumber,
			"installment_no":        input.InstallmentNo,
			"payment_method":        string(method),
			"amount":                amount.StringFixed(2),
			"penalty":               allocation.Penalty.StringFixed(2),
			"profit":                allocation.Profit.StringFixed(2),
			"principal":             allocation.Principal.StringFixed(2),
			"installment_status":    string(installmentStatus),
			"outstanding_principal": newOutstanding.StringFixed(2),
			"penalty_remaining":     newPenalty.StringFixed(2),
		}); err != nil {
			return fmt.Errorf("audit pembayaran angsuran: %w", err)
		}
		return s.loanRepo.UpdateOutstandingTx(ctx, tx, loan.ID, newOutstanding, newPenalty)
	})
	if err != nil {
		return nil, err
	}

	target.PaidPrincipal = target.PaidPrincipal.Add(allocation.Principal)
	target.PaidProfit = target.PaidProfit.Add(allocation.Profit)
	target.ProfitAccruedAmount = target.ProfitAccruedAmount.Sub(settle)
	if target.ProfitAccruedAmount.IsNegative() {
		target.ProfitAccruedAmount = decimal.Zero
	}
	if touchesSchedule {
		target.Status = installmentStatus
		if installmentStatus == domain.InstallmentStatusPaid {
			now := time.Now().UTC()
			target.PaidAt = &now
		}
	}

	// Pelunasan hanya sah bila seluruh angsuran sudah dibayar DAN tidak ada denda yang
	// masih terakru: denda menggantung berarti kredit belum selesai.
	if !newPenalty.IsPositive() && s.allSchedulesPaid(ctx, loan.ID) {
		if err := s.loanRepo.UpdateStatus(ctx, loan.ID, domain.LoanStatusPaidOff, &actor.UserID); err != nil {
			return nil, fmt.Errorf("menandai kredit lunas: %w", err)
		}
	}

	return target, nil
}

// postPenaltyCollection memindahkan denda yang sudah diakru dari piutang denda ke
// rekening nasabah:
//
//	DEBIT  rekening nasabah            sebesar denda yang dibayar
//	CREDIT akun piutang denda          sebesar yang sama
//
// Akun piutang diambil dari kaki DEBIT pemetaan LOAN_PENALTY produk (10305
// konvensional, 11700 syariah) sehingga tidak ada kode akun yang dikarang di sini.
// Kaki CREDIT pemetaan itu adalah pendapatan denda / dana kebajikan dan hanya dipakai
// saat AKRUAL; memakainya lagi di sini akan mengakui pendapatan dua kali.
// Sumber dana pembayaran angsuran. Pemetaan produk memakai akun kontrol tabungan untuk
// kaki debit pembayaran, sehingga override-nya dikunci per kode COA rekening nasabah.
// fundingSource adalah asal dana satu pembayaran angsuran: nomor akun yang didebit dan
// override pemetaan yang membuat jurnal produk mengarah ke akun itu.
//
// Akun kas tunai memakai kunci yang sama dengan jurnal transaksi rekening
// (configCashCOAConventional/Syariah, migrasi 000073 menyatukan kunci lama
// payment.cash.coa.*): satu kunci per buku, bukan dua kunci yang bisa berbeda.
type fundingSource struct {
	accountNumber string
	overrides     map[string]string
}

// configString membaca konfigurasi teks dengan nilai cadangan; cfg nil (mis. pada test)
// mengembalikan cadangan.
func (s *loanService) configString(ctx context.Context, key, fallback string) string {
	if s.config == nil {
		return fallback
	}
	return s.config.GetString(ctx, key, fallback)
}

// fundingSource menentukan dari mana angsuran dibayar. Pembayaran tunai mengarahkan
// kaki debit ke kas teller sesuai buku produk; rekening nasabah tidak disentuh sama
// sekali, karena uangnya diterima di kas, bukan dipindahkan dari rekeningnya.
func (s *loanService) fundingSource(ctx context.Context, tx any, product *domain.BankingProduct, account *domain.Account, method domain.LoanPaymentMethod) (fundingSource, error) {
	overrides, err := customerAccountOverrides(product, account)
	if err != nil {
		return fundingSource{}, err
	}
	if method != domain.LoanPaymentCash {
		return fundingSource{accountNumber: account.AccountNumber, overrides: overrides}, nil
	}

	key, fallback := configCashCOAConventional, defaultCashCOAConventional
	if product.Book == domain.BookSyariah {
		key, fallback = configCashCOASYariah, defaultCashCOASYariah
	}
	cashAccount, err := s.resolver.ResolveGLAccount(ctx, tx, s.configString(ctx, key, fallback))
	if err != nil {
		return fundingSource{}, fmt.Errorf("akun kas teller: %w", err)
	}
	for coa := range overrides {
		overrides[coa] = cashAccount
	}
	return fundingSource{accountNumber: cashAccount, overrides: overrides}, nil
}

func (s *loanService) postPenaltyCollection(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	meta PostingMeta,
	fundingAccount string,
	amount decimal.Decimal,
) error {
	rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventLoanPenalty)
	if err != nil {
		return fmt.Errorf("membaca pemetaan %s: %w", domain.EventLoanPenalty, err)
	}
	var receivableCOA string
	for _, r := range rules {
		if r.Direction == domain.DirectionDebit {
			receivableCOA = r.COACode
			break
		}
	}
	if receivableCOA == "" {
		return fmt.Errorf("pemetaan %s produk %s tidak punya kaki debit piutang denda",
			domain.EventLoanPenalty, product.Code)
	}
	receivableAcc, err := s.resolver.ResolveGLAccount(ctx, tx, receivableCOA)
	if err != nil {
		return err
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: meta.TransactionType,
		Description:     meta.Description,
		IdempotencyKey:  meta.IdempotencyKey,
		CreatedBy:       meta.CreatedBy,
		BranchCode:      meta.BranchCode,
		EntryDate:       meta.EntryDate,
		Lines: []domain.PostingLine{
			{
				AccountNumber: fundingAccount,
				Direction:     domain.DirectionDebit,
				Amount:        amount,
				Description:   "Pembayaran denda kredit",
			},
			{
				AccountNumber: receivableAcc,
				Direction:     domain.DirectionCredit,
				Amount:        amount,
				Description:   "Pelunasan piutang denda kredit",
			},
		},
	})
	return err
}

// postProfitSettlement memposting pembayaran porsi bunga angsuran saat sebagian atau
// seluruhnya sudah diakru. Jurnalnya satu entri:
//
//	DEBIT  rekening nasabah            sebesar P (total porsi bunga)
//	CREDIT akun piutang bunga 10400    sebesar settle
//	CREDIT akun pendapatan bunga 40100 sebesar P - settle
//
// Akun piutang diambil dari kaki DEBIT pemetaan INTEREST_ACCRUAL produk (bukan
// hardcode), sedangkan akun pendapatan dari kaki CREDIT pemetaan LOAN_PROFIT_PAYMENT.
// Bila settle == P baris pendapatan nol dan tidak ditulis, sehingga jumlah debit dan
// kredit tetap seimbang.
func (s *loanService) postProfitSettlement(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	meta PostingMeta,
	totalProfit, settled decimal.Decimal,
) error {
	profitRules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventLoanProfitPay)
	if err != nil {
		return fmt.Errorf("membaca pemetaan %s: %w", domain.EventLoanProfitPay, err)
	}
	accrualRules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventInterestAccrual)
	if err != nil {
		return fmt.Errorf("membaca pemetaan %s: %w", domain.EventInterestAccrual, err)
	}

	var debitCOA, revenueCOA, receivableCOA string
	for _, r := range profitRules {
		switch r.Direction {
		case domain.DirectionDebit:
			if debitCOA == "" {
				debitCOA = r.COACode
			}
		case domain.DirectionCredit:
			if revenueCOA == "" {
				revenueCOA = r.COACode
			}
		}
	}
	for _, r := range accrualRules {
		if r.Direction == domain.DirectionDebit {
			receivableCOA = r.COACode
			break
		}
	}
	if debitCOA == "" || revenueCOA == "" || receivableCOA == "" {
		return fmt.Errorf("pemetaan produk %s tidak lengkap untuk penyelesaian akruan bunga", product.Code)
	}

	// Rekening nasabah diambil dari override pemetaan debit (mis. 20100 -> nomor
	// rekening), bukan dari akun kontrol GL. Override yang tidak dipakai berarti
	// pemetaan tidak memuat akun rekening nasabah; tolak agar dana tidak diam-diam
	// jatuh ke akun kontrol.
	debitAcc := meta.AccountOverrides[debitCOA]
	for coa := range meta.AccountOverrides {
		if coa != debitCOA {
			return fmt.Errorf(
				"pemetaan %s/%s tidak memuat akun rekening nasabah (COA %s); periksa kesesuaian buku produk dan buku rekening",
				product.Code, domain.EventLoanProfitPay, coa)
		}
	}
	if debitAcc == "" {
		debitAcc, err = s.resolver.ResolveGLAccount(ctx, tx, debitCOA)
		if err != nil {
			return err
		}
	}
	revenueAcc, err := s.resolver.ResolveGLAccount(ctx, tx, revenueCOA)
	if err != nil {
		return err
	}
	receivableAcc, err := s.resolver.ResolveGLAccount(ctx, tx, receivableCOA)
	if err != nil {
		return err
	}

	lines := []domain.PostingLine{
		{AccountNumber: debitAcc, Direction: domain.DirectionDebit, Amount: totalProfit, Description: meta.Description},
		{AccountNumber: receivableAcc, Direction: domain.DirectionCredit, Amount: settled, Description: "Pelunasan bunga kredit yang masih akan diterima"},
	}
	if remaining := totalProfit.Sub(settled); remaining.IsPositive() {
		lines = append(lines, domain.PostingLine{
			AccountNumber: revenueAcc,
			Direction:     domain.DirectionCredit,
			Amount:        remaining,
			Description:   "Pendapatan bunga kredit",
		})
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
	// Saklar kerugian restrukturisasi (Pasal 32 POJK 1/2024) mati secara bawaan. Saat
	// mati, jalur di bawah ini persis seperti sebelumnya: tidak ada perhitungan EIR,
	// nilai kini, jurnal, atau perubahan perilaku restrukturisasi.
	if policy := s.restructureLossPolicy(ctx); policy.Enabled {
		return s.restructureLoanWithLoss(ctx, input, actor, policy)
	}

	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, domain.ErrRestructureOnlyActiveLoan
	}
	if input.NewTermMonths <= 0 {
		return nil, domain.ErrRestructureNewTermPositive
	}
	if loan.ProductID == nil {
		return nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	// Kualitas sebelum restrukturisasi disimpan sebelum apa pun berubah: Pasal 31
	// POJK 1/2024 membatasi kualitas sesudahnya berdasarkan nilai ini, dan proses
	// harian memakainya lagi lewat pre_restructure_collectibility.
	before := domain.CollectibilityFromOJK(loan.Collectibility)
	loan.PreRestructureCollectibility = loan.Collectibility
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
	// PPAP harian, bukan aturan tersendiri, lalu dibatasi Pasal 31 POJK 1/2024:
	// restrukturisasi tidak boleh menaikkan golongan sebelum 3 periode pembayaran
	// bersih berturut-turut. required_ppap sengaja TIDAK dihitung ulang di sini:
	// nilainya berarti cadangan yang sudah dibukukan, dan hanya batch PPAP yang boleh
	// mengubahnya karena ia pula yang memposting selisih jurnalnya.
	//
	// Catatan: blok ini adalah jalur SAKLAR-MATI. Perlakuan akuntansi kerugian
	// restrukturisasi (Pasal 32 POJK 1/2024 jo. PA BPR Bab 5.2) TIDAK diposting di
	// sini; jalurnya ada di restructureLoanWithLoss yang aktif hanya bila
	// loan.restructure.loss.enabled=true dan memakai EIR orisinal, bukan suku bunga
	// kontraktual. required_ppap tetap tidak dihitung ulang di kedua jalur: hanya batch
	// PPAP yang boleh mengubahnya karena batch itulah yang memposting selisih jurnalnya.
	col := CollectibilityForPosition(ctx, s.config, loan.DPD, DaysPastMaturity(now, loan.FinalDueDate))
	col = domain.RestructureCollectibility(before, col, 0)
	loan.Collectibility = col.OJKCode()
	loan.AccrualStatus = AccrualForCollectibility(col)

	method, profitType, margin, err := scheduleTermsFor(product, loan.MarginAmount, loan.OutstandingPrincipal, loan.TermMonths)
	if err != nil {
		return nil, err
	}
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
	// Restrukturisasi mengubah jadwal angsuran sekaligus menahan kualitas kredit pada
	// batas Pasal 31. Tanpa jejak audit, perubahan yang berimplikasi pada PPAP dan
	// kualitas aset tidak dapat direkonstruksi.
	if err := writeAudit(ctx, s.auditRepo, nil, actor, "RESTRUCTURE_LOAN", "loan", loan.ID.String(), map[string]any{
		"loan_number":           loan.LoanNumber,
		"reason":                input.Reason,
		"collectibility_before": before.OJKCode(),
		"collectibility_after":  loan.Collectibility,
		"term_months":           input.NewTermMonths,
		"total_payable":         loan.TotalPayable.StringFixed(2),
		"monthly_installment":   loan.MonthlyInstallment.StringFixed(2),
	}); err != nil {
		return nil, fmt.Errorf("audit restrukturisasi: %w", err)
	}
	return loan, nil
}

// Jenis aksi maker-checker untuk operasi kredit yang efeknya tidak dapat dibatalkan
// begitu jurnalnya terposting. Nilai ini juga menjadi kunci pendaftaran eksekutor.
const (
	ActionLoanWriteOff = "LOAN_WRITE_OFF"
	ActionLoanRecovery = "LOAN_RECOVERY"
	// ActionLoanCorrection mengoreksi nominal pokok kredit yang sudah berjalan.
	// Perubahannya menyentuh tagihan nasabah, jadi tidak boleh langsung berlaku
	// tanpa persetujuan pejabat kedua.
	ActionLoanCorrection = "LOAN_CORRECTION"
)

// guardLoanApproval menahan operasi kredit yang wajib disetujui pejabat kedua.
// Ambangnya dibaca dari system_config (maker_checker.<aksi>.threshold); nilai 0
// berarti setiap nominal wajib disetujui. Bila layanan persetujuan belum tersedia
// (mis. pada test), operasi berjalan langsung seperti sebelumnya.
func (s *loanService) guardLoanApproval(ctx context.Context, actor domain.Actor, action string, amount decimal.Decimal, payload map[string]any) error {
	if s.approvals == nil {
		return nil
	}
	threshold := s.approvals.Threshold(ctx, action)
	if threshold.IsPositive() && amount.LessThan(threshold) {
		return nil
	}
	payload["maker_id"] = actor.UserID.String()
	payload["maker_username"] = actor.Username
	payload["maker_role"] = string(actor.Role)
	payload["maker_branch"] = actor.BranchCode

	req, err := s.approvals.CreateRequest(ctx, domain.CreateMakerCheckerInput{
		ActionType: action,
		Amount:     amount,
		Payload:    payload,
		Notes:      "Operasi kredit wajib disetujui pejabat berwenang",
	}, actor)
	if err != nil {
		return err
	}
	return &domain.PendingApprovalError{RequestID: req.ID, ActionType: action}
}

// makerFromPayload memulihkan identitas pembuat keputusan dari payload persetujuan.
// Jurnal mencatat PEMBUAT, bukan pemeriksa: pengesahan pemeriksa sudah tercatat di
// audit maker-checker, dan mencatatnya sebagai pembuat akan menyesatkan penelusuran.
func makerFromPayload(payload map[string]any, fallback domain.Actor) domain.Actor {
	maker := fallback
	if v, ok := payload["maker_id"].(string); ok {
		if id, err := uuid.Parse(v); err == nil {
			maker.UserID = id
		}
	}
	if v, ok := payload["maker_username"].(string); ok && v != "" {
		maker.Username = v
	}
	if v, ok := payload["maker_role"].(string); ok && v != "" {
		maker.Role = domain.StaffRole(v)
	}
	if v, ok := payload["maker_branch"].(string); ok && v != "" {
		maker.BranchCode = v
	}
	return maker
}

func loanIDFromPayload(payload map[string]any) (uuid.UUID, error) {
	raw, ok := payload["loan_id"].(string)
	if !ok {
		return uuid.Nil, errors.New("payload persetujuan tidak memuat kredit")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("id kredit pada payload tidak valid: %w", err)
	}
	return id, nil
}

// ExecuteApproved menjalankan hapus buku atau recovery yang sudah disetujui, di dalam
// transaksi milik maker-checker service sehingga keputusan dan efeknya commit bersama.
func (s *loanService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	loanID, err := loanIDFromPayload(payload)
	if err != nil {
		return err
	}
	loan, err := s.loanRepo.GetByID(ctx, loanID)
	if err != nil {
		return err
	}
	if loan == nil {
		return errors.New("kredit pada permintaan persetujuan tidak ditemukan")
	}
	// Persetujuan adalah jalur tulis: pemeriksa satu buku tidak boleh mengeksekusi
	// operasi atas kredit buku lain, walau pembuatnya berbeda buku.
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return domain.ErrCrossBookAccess
	}
	maker := makerFromPayload(payload, actor)

	switch normalizeAction(actionType) {
	case ActionLoanWriteOff:
		product, terms, err := s.writeOffTarget(ctx, loan)
		if err != nil {
			return err
		}
		reason, _ := payload["reason"].(string)
		collectionEfforts, _ := payload["collection_efforts"].(string)
		return s.writeOffTx(ctx, tx, loan, product, terms, reason, collectionEfforts, maker)
	case ActionLoanRecovery:
		amount, err := decimalFromPayload(payload["recovery_amount"])
		if err != nil {
			return err
		}
		if !amount.IsPositive() {
			return domain.ErrRecoveryAmountPositive
		}
		product, overrides, err := s.recoveryTarget(ctx, loan)
		if err != nil {
			return err
		}
		idempotencyKey, _ := payload["idempotency_key"].(string)
		return s.recoveryTx(ctx, tx, loan, product, amount, overrides, maker, idempotencyKey)
	case ActionLoanCorrection:
		amount, err := decimalFromPayload(payload["new_amount"])
		if err != nil {
			return err
		}
		reason, _ := payload["reason"].(string)
		return s.correctLoanAmountTx(ctx, tx, loan, domain.CorrectLoanAmountInput{
			LoanID:    loan.ID,
			NewAmount: amount,
			Reason:    reason,
		}, maker)
	default:
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
}

// writeOffTerms adalah nominal yang dilepas dari neraca saat hapus buku: pokok,
// bunga yang sudah diakui tetapi belum tertagih, dan denda yang masih tercatat.
type writeOffTerms struct {
	Principal     decimal.Decimal
	AccruedProfit decimal.Decimal
	Penalty       decimal.Decimal
}

func (t writeOffTerms) Total() decimal.Decimal {
	return t.Principal.Add(t.AccruedProfit).Add(t.Penalty)
}

// accruedProfitFor menghitung piutang bunga yang masih tercatat untuk satu kredit:
// jumlah akruan yang sudah diakui dan belum diselesaikan pembayaran. Nilainya harus
// dilepas saat hapus buku; kalau tidak, piutangnya tetap di neraca tanpa kredit yang
// menopangnya, dan bank terus melaporkan aset yang tidak mungkin tertagih.
func (s *loanService) accruedProfitFor(ctx context.Context, loanID uuid.UUID) (decimal.Decimal, error) {
	schedules, err := s.loanRepo.GetSchedules(ctx, loanID)
	if err != nil {
		return decimal.Zero, err
	}
	total := decimal.Zero
	for _, schedule := range schedules {
		if schedule.ProfitAccruedAmount.IsPositive() {
			total = total.Add(schedule.ProfitAccruedAmount)
		}
	}
	return total, nil
}

// requireWriteOffLegs menolak hapus buku yang pemetaan produknya belum memuat kaki
// untuk nominal yang dilepas. Poster hanya melewati nominal yang tidak punya kaki,
// sehingga tanpa pemeriksaan ini piutang bunga atau denda tetap tercatat di neraca
// tanpa satu pun peringatan.
func requireWriteOffLegs(rules []domain.JournalMappingRule, terms writeOffTerms) error {
	declared := func(source domain.AmountSource) bool {
		if source == "" {
			return true
		}
		for _, rule := range rules {
			if rule.AmountSource == source {
				return true
			}
		}
		return false
	}
	if terms.AccruedProfit.IsPositive() && !declared(domain.AmountProfit) {
		return errors.New("pemetaan hapus buku produk belum memuat kaki piutang bunga (PROFIT); piutang bunga kredit tidak dapat dilepas")
	}
	if terms.Penalty.IsPositive() && !declared(domain.AmountPenalty) {
		return errors.New("pemetaan hapus buku produk belum memuat kaki piutang denda (PENALTY); piutang denda kredit tidak dapat dilepas")
	}
	return nil
}

// writeOffTarget memvalidasi syarat hapus buku (POJK 1/2024 Pasal 42-43 / POJK
// 24/2024 Pasal 50-51) dan menghitung nominal yang dilepas dari neraca.
//
// Dua syarat yang ditegakkan di sini, bukan sekadar konfigurasi:
//  1. kualitas aset harus Macet (kolektibilitas 5);
//  2. cadangan/penyisihan harus sudah 100% dari nilai tercatat kredit.
//
// Nilai tercatat memakai basis yang sama dengan mesin PPAP (PPAPCarryingAmount),
// sehingga saldo kerugian restrukturisasi tidak dihitung sebagai eksposur yang perlu
// dicadangkan. Pengurang agunan (Pasal 20) tidak diterapkan di sini karena layanan
// kredit tidak memegang modul agunan; karena itu syarat cadangan 100% bersifat lebih
// konservatif untuk kredit beragunan.
func (s *loanService) writeOffTarget(ctx context.Context, loan *domain.Loan) (*domain.BankingProduct, writeOffTerms, error) {
	// Kredit tidak aktif (PAID_OFF/WRITTEN_OFF/CANCELLED) sudah tidak punya eksposur.
	// DEFAULTED tetap boleh karena ia masih punya eksposur dan justru golongan macet.
	if loan.Status != domain.LoanStatusDisbursed && loan.Status != domain.LoanStatusDefaulted {
		return nil, writeOffTerms{}, errors.New("hanya kredit aktif yang dapat dihapus buku")
	}
	if loan.ProductID == nil {
		return nil, writeOffTerms{}, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, writeOffTerms{}, err
	}
	// Syarat 1: hanya aset macet. Kolektibilitas disimpan sebagai kode lama (mis.
	// "5_MACET"); domain.CollectibilityFromOJK menormalkannya.
	if c := domain.CollectibilityFromOJK(loan.Collectibility); c != domain.KolMacet {
		return nil, writeOffTerms{}, fmt.Errorf("%w: kredit %s berkualitas %s",
			domain.ErrWriteOffNotMacet, loan.LoanNumber, c.Label())
	}
	// Tanpa fallback ke PrincipalAmount: hapus buku harus melepas SELURUH sisa pokok
	// yang tercatat (POJK 1/2024 Pasal 42 ayat (2)). Sisa pokok nol berarti tidak ada
	// lagi yang bisa dihapus buku.
	if !loan.OutstandingPrincipal.IsPositive() {
		return nil, writeOffTerms{}, errors.New("kredit tidak memiliki sisa pokok yang dapat dihapus buku")
	}
	principal := loan.OutstandingPrincipal
	// Syarat 2: cadangan 100% dari nilai tercatat. Pihak bank boleh menetapkan tarif
	// PPAP lebih tinggi, tetapi POJK menetapkan lantai 100% untuk kualitas Macet,
	// sehingga lantai itu dipakai langsung (bukan tarif yang bisa lebih rendah).
	carrying := domain.PPAPCarryingAmount(principal, loan.RestructureLossBalance)
	requiredReserve := domain.RoundToRupiah(carrying)
	if loan.RequiredPPAP.LessThan(requiredReserve) {
		return nil, writeOffTerms{}, fmt.Errorf("%w: kredit %s baru dicadangkan %s dari %s yang diwajibkan",
			domain.ErrWriteOffReserveIncomplete, loan.LoanNumber,
			loan.RequiredPPAP.StringFixed(2), requiredReserve.StringFixed(2))
	}
	accruedProfit, err := s.accruedProfitFor(ctx, loan.ID)
	if err != nil {
		return nil, writeOffTerms{}, err
	}
	penalty := loan.PenaltyAccrued
	if penalty.IsNegative() {
		penalty = decimal.Zero
	}
	return product, writeOffTerms{Principal: principal, AccruedProfit: accruedProfit, Penalty: penalty}, nil
}

// recoveryTarget memvalidasi kredit hapus buku dan menyiapkan rekening penerimaannya.
func (s *loanService) recoveryTarget(ctx context.Context, loan *domain.Loan) (*domain.BankingProduct, map[string]string, error) {
	if loan.Status != domain.LoanStatusWrittenOff {
		return nil, nil, errors.New("kredit tidak berstatus hapus buku")
	}
	if loan.ProductID == nil {
		return nil, nil, errors.New("kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *loan.ProductID)
	if err != nil {
		return nil, nil, err
	}
	recoveryAccount, err := s.accountRepo.GetByID(ctx, loan.DisbursementAccountID)
	if err != nil {
		return nil, nil, errors.New("rekening recovery tidak ditemukan")
	}
	overrides, err := customerAccountOverrides(product, recoveryAccount)
	if err != nil {
		return nil, nil, err
	}
	return product, overrides, nil
}

// writeOffReserveOverride menentukan pengarahan kaki debit cadangan jurnal hapus buku.
//
// Semenjak ckpn.enabled menyala, cadangan yang dibukukan untuk kredit adalah CKPN
// (10950 konvensional / 11950 syariah), bukan PPAP (10900/11900). Pemetaan produk
// LOAN_WRITE_OFF di-seed melepas akun PPAP, sehingga tanpa pengarahan ini hapus buku
// melepas akun PPAP yang saldonya milik dasar lain dan cadangan CKPN kredit itu tidak
// pernah dilepas. Saat ckpn.enabled mati, pemetaan produk dipakai apa adanya (perilaku
// lama) — pengarahan ini sengaja hanya berlaku pada jalur CKPN aktif.
//
// Nilai kembalian adalah AccountOverrides (kunci kode COA -> nomor akun GL) yang siap
// diserahkan ke ProductPoster. Pemeriksaan apakah kode benar-benar ada di bagan akun
// dilakukan ResolveGLAccount, sehingga pemetaan yang salah ditolak, bukan terjurnal ke
// akun yang tidak ada.
func (s *loanService) writeOffReserveOverride(ctx context.Context, tx any, product *domain.BankingProduct, rules []domain.JournalMappingRule) (map[string]string, error) {
	if s.config == nil || !s.config.GetBool(ctx, cfgCKPNEnabled, false) {
		return nil, nil
	}
	// Kaki debit pokok adalah akun cadangan yang dilepas; kaki kredit pokok adalah
	// piutang kredit. Hanya kaki debit yang diarahkan.
	reserveCOA := ""
	for _, rule := range rules {
		if rule.Direction == domain.DirectionDebit && rule.AmountSource == domain.AmountPrincipal {
			reserveCOA = rule.COACode
			break
		}
	}
	if reserveCOA == "" {
		return nil, nil
	}
	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}
	ckpnReserve := resolveCKPNCOA(ctx, s.config, book,
		cfgCKPNReserveCOASyariah, cfgCKPNReserveCOA,
		fallbackCKPNReserveSyariah, fallbackCKPNReserveConventional)
	// Pemetaan produk sudah memakai akun CKPN: tidak ada yang perlu diubah.
	if reserveCOA == ckpnReserve {
		return nil, nil
	}
	acc, err := s.resolver.ResolveGLAccount(ctx, tx, ckpnReserve)
	if err != nil {
		return nil, fmt.Errorf("%w: COA %s: %v", domain.ErrCKPNReserveNotFound, ckpnReserve, err)
	}
	return map[string]string{reserveCOA: acc}, nil
}

// redirectWriteOffReserve menyalin pemetaan hapus buku dan mengubah SATU kaki debit
// pokok (cadangan) menjadi sumber nominal AmountReserve, sehingga yang dilepas dari
// akun cadangan adalah required_ckpn kredit — bukan pokok penuh. Kaki kredit pokok
// tetap memakai AmountPrincipal agar seluruh pokok tetap keluar dari neraca.
func redirectWriteOffReserve(rules []domain.JournalMappingRule) []domain.JournalMappingRule {
	out := make([]domain.JournalMappingRule, len(rules))
	copy(out, rules)
	for i := range out {
		if out[i].Direction == domain.DirectionDebit && out[i].AmountSource == domain.AmountPrincipal {
			out[i].AmountSource = domain.AmountReserve
			break
		}
	}
	return out
}

// writeOffLossCOA memilih akun beban kerugian penurunan nilai per buku untuk selisih
// pokok yang belum dicadangkan saat hapus buku CKPN. Akunnya sengaja memakai kunci
// kebijakan ckpn.coa.expense yang sudah ada (bank dapat mengarahkannya ke akun
// tersendiri), bukan akun baru yang ditanam di kode.
func (s *loanService) writeOffLossCOA(ctx context.Context, product *domain.BankingProduct) string {
	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}
	return resolveCKPNCOA(ctx, s.config, book,
		cfgCKPNExpenseCOASyariah, cfgCKPNExpenseCOA,
		fallbackCKPNExpenseSyariah, fallbackCKPNExpenseConventional)
}

// writeOffTx menjalankan hapus buku di dalam transaksi pemanggil: jurnal, status, sisa
// pokok, dan jejak auditnya commit bersama. Selain pokok, piutang bunga dan denda yang
// masih tercatat ikut dilepas supaya tidak ada aset tanpa penopang yang tertinggal.
//
// Auditnya menyimpan bukti syarat saat keputusan: kualitas aset, DPD, nilai cadangan
// yang sudah dibentuk, dasar pertimbangan, dan upaya penagihan yang terdokumentasi.
// Bukti ini tidak dapat diubah (audit_logs append-only) dan menjadi rujukan pelaporan
// pengawasan Dekom/OJK.
func (s *loanService) writeOffTx(ctx context.Context, tx any, loan *domain.Loan, product *domain.BankingProduct, terms writeOffTerms, reason, collectionEfforts string, actor domain.Actor) error {
	rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventLoanWriteOff)
	if err != nil {
		return fmt.Errorf("membaca pemetaan hapus buku: %w", err)
	}
	if err := requireWriteOffLegs(rules, terms); err != nil {
		return err
	}
	postRules := rules
	amounts := Amounts{
		Principal: terms.Principal,
		Profit:    terms.AccruedProfit,
		Penalty:   terms.Penalty,
		Total:     terms.Total(),
	}
	// Jalur CKPN aktif: yang dibukukan sebagai cadangan kredit bukan PPAP, melainkan
	// CKPN. Nilai yang boleh dilepas dari akun CKPN karena itu HANYA sebesar cadangan
	// yang benar-benar terkait kredit ini (loans.required_ckpn), bukan sebesar pokok.
	// Melepas sebesar pokok (benar pada jalur PPAP karena cadangan 100% pokok memang
	// diwajibkan) akan MENYISAKAN cadangan bila required_ckpn > pokok, atau membuat
	// saldo CKPN NEGATIF bila required_ckpn < pokok. Selisih pokok yang belum
	// dicadangkan diakui sebagai beban kerugian penurunan nilai (ckpn.coa.expense buku
	// kredit) supaya jurnal tetap seimbang dan pokok tetap dilepas penuh.
	ckpnActive := s.config != nil && s.config.GetBool(ctx, cfgCKPNEnabled, false)
	ckpnReleased, writeOffLoss := decimal.Zero, decimal.Zero
	if ckpnActive {
		ckpnReleased = loan.RequiredCKPN
		if ckpnReleased.IsNegative() {
			ckpnReleased = decimal.Zero
		}
		if ckpnReleased.GreaterThan(terms.Principal) {
			// Cadangan tidak pernah lebih besar dari pokok (EAD = pokok - kerugian
			// restrukturisasi), jadi ini hanya penjaga defensif: jangan melepas lebih
			// dari yang dilepas neraca.
			ckpnReleased = terms.Principal
		}
		writeOffLoss = terms.Principal.Sub(ckpnReleased)
		amounts.Reserve = ckpnReleased
		postRules = redirectWriteOffReserve(rules)
		if writeOffLoss.IsPositive() {
			amounts.Loss = writeOffLoss
			postRules = append(append([]domain.JournalMappingRule{}, postRules...), domain.JournalMappingRule{
				Direction:    domain.DirectionDebit,
				COACode:      s.writeOffLossCOA(ctx, product),
				AmountSource: domain.AmountLoss,
			})
		}
	}
	// Saat CKPN aktif DAN ada cadangan yang dilepas, kaki debit cadangan diarahkan ke
	// akun CKPN per buku (bukan akun PPAP pada pemetaan produk). Bila tidak ada
	// cadangan (mis. CKPN belum pernah dijalankan untuk kredit ini), tidak ada kaki
	// cadangan yang tersisa sehingga overrides juga kosong. Bila CKPN mati, overrides
	// nil dan pemetaan produk dipakai apa adanya — perilaku lama tidak berubah.
	var overrides map[string]string
	if ckpnReleased.IsPositive() {
		overrides, err = s.writeOffReserveOverride(ctx, tx, product, rules)
		if err != nil {
			return err
		}
	}

	if _, err := s.poster.postEventWithRulesTx(ctx, tx, product, domain.EventLoanWriteOff, postRules, amounts, PostingMeta{
		TransactionType:  domain.TxTypeAdjustment,
		Description:      fmt.Sprintf("Hapus buku kredit %s: %s", loan.LoanNumber, reason),
		IdempotencyKey:   "WOFF-" + loan.LoanNumber,
		CreatedBy:        actor.DisplayName(),
		BranchCode:       actor.BranchCode,
		AccountOverrides: overrides,
	}); err != nil {
		return fmt.Errorf("jurnal hapus buku: %w", err)
	}
	if err := s.loanRepo.UpdateStatusTx(ctx, tx, loan.ID, domain.LoanStatusWrittenOff, &actor.UserID); err != nil {
		return err
	}
	if err := s.loanRepo.UpdateOutstandingTx(ctx, tx, loan.ID, decimal.Zero, decimal.Zero); err != nil {
		return err
	}
	// Nilai yang dilepas disimpan pada baris kredit, satu transaksi dengan jurnal dan
	// statusnya. Inilah satu-satunya sumber batas pemulihan setelah hapus buku; tanpa
	// simpanan ini pemulihan hanya berbatas kejujuran operator.
	if err := s.loanRepo.SetWrittenOffAmountTx(ctx, tx, loan.ID, terms.Total()); err != nil {
		return err
	}
	changes := map[string]any{
		"loan_number":           loan.LoanNumber,
		"reason":                reason,
		"collection_efforts":    collectionEfforts,
		"principal":             terms.Principal.StringFixed(2),
		"accrued_profit":        terms.AccruedProfit.StringFixed(2),
		"penalty":               terms.Penalty.StringFixed(2),
		"amount":                terms.Total().StringFixed(2),
		"outstanding_principal": loan.OutstandingPrincipal.StringFixed(2),
		"penalty_outstanding":   loan.PenaltyAccrued.StringFixed(2),
		// Bukti syarat POJK pada saat keputusan: kualitas aset wajib Macet dan
		// cadangan yang sudah dibentuk wajib 100% dari nilai tercatat.
		"collectibility": string(loan.Collectibility),
		"dpd":            loan.DPD,
		"required_ppap":  loan.RequiredPPAP.StringFixed(2),
	}
	// Jalur CKPN aktif menyimpan dasar pelepasan: cadangan yang benar-benar dilepas
	// dan selisih pokok yang diakui sebagai beban. Bidang ini TIDAK ditambahkan saat
	// CKPN mati agar jejak audit jalur lama persis tidak berubah.
	if ckpnActive {
		changes["required_ckpn"] = loan.RequiredCKPN.StringFixed(2)
		changes["ckpn_released"] = ckpnReleased.StringFixed(2)
		changes["write_off_loss"] = writeOffLoss.StringFixed(2)
	}
	return writeAudit(ctx, s.auditRepo, tx, actor, "WRITE_OFF_LOAN", "loan", loan.ID.String(), changes)
}

// recoveryKey membangun kunci idempotensi recovery yang DETERMINISTIK. Kunci dari
// pemanggil dipakai apa adanya bila ada, sehingga klien dapat mengulang permintaan
// dengan aman. Tanpa kunci pemanggil, kuncinya adalah nomor kredit + nominal + tanggal
// bisnis: pengulangan pada hari bisnis yang sama menghasilkan kunci yang sama dan
// posting engine mengembalikan jurnal lama, bukan menulis jurnal kedua. Tanggal bisnis
// (bukan time.Now()) dipakai agar dua penerimaan yang sah dengan nominal sama pada hari
// berbeda tetap tercatat sebagai dua jurnal. Bila sumber tanggal bisnis tidak tersedia
// (mis. test unit), kunci memuat nomor kredit dan nominal saja.
func (s *loanService) recoveryKey(ctx context.Context, loanNumber string, amount decimal.Decimal, provided string) string {
	if key := strings.TrimSpace(provided); key != "" {
		return "RECOV-" + loanNumber + "-" + key
	}
	key := fmt.Sprintf("RECOV-%s-%s", loanNumber, amount.String())
	if s.dates != nil {
		if current, err := s.dates.GetCurrentDate(ctx); err == nil && current != nil && !current.CurrentDate.IsZero() {
			key += "-" + current.CurrentDate.Format("20060102")
		}
	}
	return key
}

// recoveryTx mencatat penerimaan kas atas kredit yang sudah dihapus buku. Sebelum
// mencatat, akumulasi pemulihan diperiksa terhadap nilai hapus buku yang tersimpan:
// pemulihan tidak boleh melebihi nilai yang pernah dilepas. Baris kredit dikunci lebih
// dulu (SELECT ... FOR UPDATE) agar dua pemulihan bersamaan tidak sama-sama lolos batas.
func (s *loanService) recoveryTx(ctx context.Context, tx any, loan *domain.Loan, product *domain.BankingProduct, amount decimal.Decimal, overrides map[string]string, actor domain.Actor, providedKey string) error {
	locked, err := s.loanRepo.LockLoanTx(ctx, tx, loan.ID)
	if err != nil {
		return err
	}
	if locked != nil {
		loan = locked
	}
	// Kunci dibangun sebelum batas diperiksa supaya jurnal dari permintaan yang SAMA
	// (retry idempoten) tidak dihitung sebagai pemulihan lain.
	idempotencyKey := s.recoveryKey(ctx, loan.LoanNumber, amount, providedKey)
	if err := s.guardRecoveryCap(ctx, tx, loan, amount, idempotencyKey); err != nil {
		return err
	}
	if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanRecovery, Amounts{
		Principal: amount, Total: amount,
	}, PostingMeta{
		TransactionType:  domain.TxTypeAdjustment,
		Description:      fmt.Sprintf("Recovery kredit hapus buku %s", loan.LoanNumber),
		IdempotencyKey:   idempotencyKey,
		CreatedBy:        actor.DisplayName(),
		BranchCode:       actor.BranchCode,
		AccountOverrides: overrides,
	}); err != nil {
		return fmt.Errorf("jurnal recovery: %w", err)
	}
	return writeAudit(ctx, s.auditRepo, tx, actor, "RECOVER_WRITTEN_OFF_LOAN", "loan", loan.ID.String(), map[string]any{
		"loan_number":     loan.LoanNumber,
		"recovery_amount": amount.StringFixed(2),
	})
}

// guardRecoveryCap menolak pemulihan yang membuat akumulasi pemulihan melebihi nilai
// hapus buku kredit. Nilai hapus buku tersimpan di baris kredit saat eksekusi hapus
// buku (written_off_amount); data lama diisi migrasi 000085 dari audit/jurnal. Bila
// nilainya tidak diketahui (mis. data lama yang tidak dapat dipastikan), pemulihan
// DITOLAK dengan pesan jelas — menolak lebih aman daripada membiarkan pemulihan tanpa
// batas. Nilai yang tepat sama dengan batas tetap boleh (tidak "melebihi").
func (s *loanService) guardRecoveryCap(ctx context.Context, tx any, loan *domain.Loan, amount decimal.Decimal, idempotencyKey string) error {
	if !loan.WrittenOffAmount.IsPositive() {
		return fmt.Errorf("%w: kredit %s", domain.ErrWriteOffAmountUnavailable, loan.LoanNumber)
	}
	recovered, err := s.loanRepo.SumRecoveredAmountTx(ctx, tx, loan.LoanNumber, idempotencyKey)
	if err != nil {
		return err
	}
	if recovered.Add(amount).GreaterThan(loan.WrittenOffAmount) {
		return fmt.Errorf("%w: kredit %s sudah dipulihkan %s, ditambah %s melebihi nilai hapus buku %s",
			domain.ErrRecoveryExceedsWriteOff, loan.LoanNumber,
			recovered.StringFixed(2), amount.StringFixed(2), loan.WrittenOffAmount.StringFixed(2))
	}
	return nil
}

// WriteOffLoan mengajukan hapus buku. Karena melepas tagihan dari neraca dan tidak
// dapat dibatalkan begitu jurnalnya terposting, operasinya wajib disetujui pejabat
// kedua: efeknya baru terjadi saat persetujuan diproses lewat ExecuteApproved.
func (s *loanService) WriteOffLoan(ctx context.Context, input domain.WriteOffLoanInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	product, terms, err := s.writeOffTarget(ctx, loan)
	if err != nil {
		return nil, err
	}
	// POJK 1/2024 Pasal 43 mewajibkan upaya memperoleh kembali dan dasar
	// pertimbangannya terdokumentasi. Keduanya diminta SEBELUM keputusan dibuat dan
	// ikut tersimpan pada permintaan persetujuan, bukan hanya pada audit eksekusi.
	if strings.TrimSpace(input.Reason) == "" {
		return nil, domain.ErrWriteOffReasonRequired
	}
	if strings.TrimSpace(input.CollectionEfforts) == "" {
		return nil, domain.ErrWriteOffCollectionEffortsRequired
	}
	// Tidak boleh hapus buku sebagian (POJK 1/2024 Pasal 42 ayat (2)). Pengosongan
	// Amount berarti seluruh eksposur; nilai yang diisi harus sama dengan sisa pokok.
	if input.Amount.IsPositive() && !input.Amount.Equal(terms.Principal) {
		return nil, fmt.Errorf("%w: diminta %s, kewajiban hapus buku seluruh sisa pokok %s",
			domain.ErrWriteOffPartial, input.Amount.StringFixed(2), terms.Principal.StringFixed(2))
	}

	if err := s.guardLoanApproval(ctx, actor, ActionLoanWriteOff, terms.Total(), map[string]any{
		"loan_id":     loan.ID.String(),
		"loan_number": loan.LoanNumber,
		"reason":      input.Reason,
		// Bukti syarat dikirim ke pemeriksa pada saat pengajuan, sehingga keputusan
		// dapat dinilai dari keadaan kredit saat itu, bukan saat eksekusi.
		"collection_efforts": input.CollectionEfforts,
		"collectibility":     string(loan.Collectibility),
		"dpd":                loan.DPD,
		"required_ppap":      loan.RequiredPPAP.String(),
		"amount":             terms.Total().String(),
	}); err != nil {
		return nil, err
	}

	// Transaksi dibuka lewat txRunner yang sama dengan pembayaran angsuran: isolasi
	// ReadCommitted dan jalur uji tanpa database tetap satu pintu.
	if err := s.txRunner.Run(ctx, func(tx any) error {
		return s.writeOffTx(ctx, tx, loan, product, terms, input.Reason, input.CollectionEfforts, actor)
	}); err != nil {
		return nil, err
	}

	loan.Status = domain.LoanStatusWrittenOff
	loan.OutstandingPrincipal = decimal.Zero
	loan.WrittenOffAmount = terms.Total()
	return loan, nil
}

// RecoverWrittenOffLoan mengajukan penerimaan kas atas kredit hapus buku, dengan alur
// persetujuan yang sama seperti hapus buku.
func (s *loanService) RecoverWrittenOffLoan(ctx context.Context, input domain.RecoverWrittenOffLoanInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrCrossBranchAccess
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if input.RecoveryAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrRecoveryAmountPositive
	}
	product, overrides, err := s.recoveryTarget(ctx, loan)
	if err != nil {
		return nil, err
	}

	if err := s.guardLoanApproval(ctx, actor, ActionLoanRecovery, input.RecoveryAmount, map[string]any{
		"loan_id":         loan.ID.String(),
		"loan_number":     loan.LoanNumber,
		"recovery_amount": input.RecoveryAmount.String(),
		// Kunci idempotensi dibawa ke payload persetujuan agar eksekusi memakai kunci
		// yang sama dengan pengajuan; pengulangan pengajuan yang diulang-eksekusi tidak
		// menggandakan jurnal.
		"idempotency_key": input.IdempotencyKey,
	}); err != nil {
		return nil, err
	}

	if err := s.txRunner.Run(ctx, func(tx any) error {
		return s.recoveryTx(ctx, tx, loan, product, input.RecoveryAmount, overrides, actor, input.IdempotencyKey)
	}); err != nil {
		return nil, err
	}
	return loan, nil
}

// CancelDisbursementLoan membatalkan pencairan kredit sepenuhnya. Hanya kredit
// DISBURSED yang belum menerima angsuran yang boleh dibatalkan: setelah uang
// masuk, jurnal dan jadwal sudah berjalan dan koreksinya bukan pembatalan.
//
// Seluruh efek berada dalam satu transaksi: jurnal pencairan dibalik, jurnal
// asal ditandai REVERSED, jadwal angsuran dihapus, status CANCELLED, sisa pokok
// dinolkan, dan audit ditulis. Terpisah, kegagalan di antaranya akan meninggalkan
// kredit yang jurnalnya sudah dibatalkan tetapi tagihannya masih menggantung.
func (s *loanService) CancelDisbursementLoan(ctx context.Context, input domain.CancelLoanInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	// Pembatalan pencairan lebih sensitif daripada pencairan. Kredit cabang lain
	// disamarkan sebagai tidak ditemukan agar keberadaannya tidak terbocor.
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrLoanNotFound
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, domain.ErrLoanNotCancellable
	}
	if input.Reason == "" {
		return nil, domain.ErrDisbursementCancelReasonRequired
	}

	if err := s.txRunner.Run(ctx, func(tx any) error {
		// Kunci baris kredit lebih dulu. Pemeriksaan di pemanggil hanyalah gagal-cepat:
		// setelah kunci diperoleh, status dan ada/tidaknya pembayaran diperiksa ulang
		// dari keadaan segar, sehingga pembayaran atau koreksi yang commit lebih dulu
		// pasti terlihat dan pembatalan tidak menghapus jadwal miliknya.
		locked, err := s.loanRepo.LockLoanTx(ctx, tx, loan.ID)
		if err != nil {
			return err
		}
		if locked.Status != domain.LoanStatusDisbursed {
			return domain.ErrLoanNotCancellable
		}
		paid, err := s.loanRepo.HasInstallmentPaymentTx(ctx, tx, loan.ID)
		if err != nil {
			return err
		}
		if paid {
			return domain.ErrLoanHasInstallmentPayments
		}
		// Akrual bunga/denda/PPAP yang sudah terposting tidak akan dibalik oleh
		// pembatalan ini, jadi kredit semacam itu ditolak alih-alih dibiarkan
		// menggantung. Memeriksa tepat setelah kunci membuat hasilnya otoritatif:
		// jalur akrual juga memegang kunci baris kredit yang sama.
		accrued, err := s.loanRepo.HasAccrualPostingsTx(ctx, tx, loan.ID)
		if err != nil {
			return err
		}
		if accrued {
			return domain.ErrLoanHasAccrualPostings
		}

		ref, err := s.loanRepo.GetDisbursementJournalRefTx(ctx, tx, loan.LoanNumber)
		if err != nil {
			return err
		}
		original, err := s.ledgerRepo.GetJournalByRef(ctx, ref)
		if err != nil {
			return fmt.Errorf("membaca jurnal pencairan %s: %w", ref, err)
		}

		posted, err := s.postLoanReversalTx(ctx, tx, original, loan, input.Reason, actor)
		if err != nil {
			return err
		}
		if err := s.loanRepo.DeleteSchedulesTx(ctx, tx, loan.ID); err != nil {
			return fmt.Errorf("menghapus jadwal angsuran: %w", err)
		}
		if err := s.loanRepo.UpdateStatusTx(ctx, tx, loan.ID, domain.LoanStatusCancelled, &actor.UserID); err != nil {
			return fmt.Errorf("membatalkan status kredit: %w", err)
		}
		// Sisa pokok dan denda dinolkan karena tagihan yang mendasarinya tidak ada.
		if err := s.loanRepo.UpdateOutstandingTx(ctx, tx, loan.ID, decimal.Zero, decimal.Zero); err != nil {
			return fmt.Errorf("menolkan sisa pokok kredit: %w", err)
		}
		return writeAudit(ctx, s.auditRepo, tx, actor, "CANCEL_DISBURSEMENT_LOAN", "loan", loan.ID.String(), map[string]any{
			"loan_number":        loan.LoanNumber,
			"reason":             input.Reason,
			"reversed_reference": original.ReferenceNumber,
			"reversal_reference": posted.ReferenceNumber,
			"amount":             loan.OutstandingPrincipal.StringFixed(2),
		})
	}); err != nil {
		return nil, err
	}

	loan.Status = domain.LoanStatusCancelled
	loan.OutstandingPrincipal = decimal.Zero
	loan.PenaltyAccrued = decimal.Zero
	loan.Schedules = nil
	return loan, nil
}

// CorrectLoanAmount mengoreksi nominal pokok kredit yang sudah dicairkan tanpa
// membatalkan pencairan. Berbeda dari pembatalan, angsuran yang sudah dibayar tetap
// apa adanya; hanya angsuran yang belum dibayar yang pokoknya dihitung ulang. Karena
// perubahannya mengubah tagihan nasabah yang sedang berjalan, operasinya wajib lewat
// persetujuan pejabat kedua: efeknya baru terjadi saat ExecuteApproved dipanggil.
func (s *loanService) CorrectLoanAmount(ctx context.Context, input domain.CorrectLoanAmountInput, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByID(ctx, input.LoanID)
	if err != nil {
		return nil, err
	}
	// Sama seperti pembatalan: kredit cabang lain disamarkan sebagai tidak ditemukan
	// agar keberadaannya tidak terbocor.
	if !canAccessLoan(actor, loan) {
		return nil, domain.ErrLoanNotFound
	}
	if !s.canAccessLoanBook(ctx, actor, loan) {
		return nil, domain.ErrCrossBookAccess
	}
	if loan.Status != domain.LoanStatusDisbursed {
		return nil, domain.ErrLoanNotCancellable
	}
	if input.NewAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidLoanAmount
	}
	if input.NewAmount.Equal(loan.PrincipalAmount) {
		return nil, domain.ErrLoanAmountUnchanged
	}
	if input.Reason == "" {
		return nil, domain.ErrLoanCorrectionReasonRequired
	}

	// Koreksi tidak boleh dijalankan langsung tanpa layanan persetujuan: tidak ada
	// pejabat yang bisa memeriksanya. Dikembalikan sebagai menunggu persetujuan,
	// bukan sebagai keberhasilan, agar pemanggil tidak mengira tagihan sudah berubah.
	if s.approvals == nil {
		return nil, &domain.PendingApprovalError{ActionType: ActionLoanCorrection}
	}
	if err := s.guardLoanApproval(ctx, actor, ActionLoanCorrection, input.NewAmount, map[string]any{
		"loan_id":     loan.ID.String(),
		"loan_number": loan.LoanNumber,
		"new_amount":  input.NewAmount.String(),
		"reason":      input.Reason,
	}); err != nil {
		return nil, err
	}

	if err := s.txRunner.Run(ctx, func(tx any) error {
		return s.correctLoanAmountTx(ctx, tx, loan, input, actor)
	}); err != nil {
		return nil, err
	}
	return loan, nil
}

// correctLoanAmountTx menjalankan koreksi di dalam transaksi pemanggil: membaca
// jadwal, menghitung ulang angsuran yang belum dibayar, memposting jurnal selisih,
// memperbarui nominal, dan menulis audit. Tanggal bisnis dibaca paling awal agar
// koreksi ditolak sebelum satu pun baris berubah bila tanggal tidak tersedia.
func (s *loanService) correctLoanAmountTx(ctx context.Context, tx any, loan *domain.Loan, input domain.CorrectLoanAmountInput, actor domain.Actor) error {
	// Kunci baris kredit lebih dulu, sebelum jadwal dibaca. Tanpa kunci, dua koreksi
	// bersamaan menghitung selisih dari data basi dan menerbitkan jurnal selisih ganda,
	// dan koreksi pada kredit yang baru saja CANCELLED dapat menghidupkan kembali
	// jadwalnya. Pemeriksaan status di pemanggil hanya gagal-cepat; yang otoritatif
	// adalah keadaan setelah kunci diperoleh.
	locked, err := s.loanRepo.LockLoanTx(ctx, tx, loan.ID)
	if err != nil {
		return err
	}
	if locked.Status != domain.LoanStatusDisbursed {
		return domain.ErrLoanNotCancellable
	}
	// Nominal yang diminta bisa saja sudah menjadi nominal berjalan karena koreksi lain
	// commit lebih dulu. Tanpa pemeriksaan ulang ini, selisihnya nol tetapi koreksi
	// tetap menulis audit dan menimpa jadwal tanpa manfaat.
	if input.NewAmount.Equal(locked.PrincipalAmount) {
		return domain.ErrLoanAmountUnchanged
	}
	// Salin keadaan segar ke objek pemanggil agar mutasi di bawah (nominal, jadwal)
	// tetap terlihat pada kredit yang dikembalikan.
	*loan = *locked

	entryDate, err := s.businessDate(ctx)
	if err != nil {
		return err
	}

	schedules, err := s.loanRepo.GetSchedulesTx(ctx, tx, loan.ID)
	if err != nil {
		return fmt.Errorf("membaca jadwal angsuran: %w", err)
	}

	oldAmount := loan.PrincipalAmount
	updated, outstanding, totalPayable, monthly, err := recalculateSchedules(schedules, input.NewAmount)
	if err != nil {
		return err
	}

	diff := input.NewAmount.Sub(oldAmount)
	journalRef := ""
	if !diff.IsZero() {
		ref, err := s.loanRepo.GetDisbursementJournalRefTx(ctx, tx, loan.LoanNumber)
		if err != nil {
			return err
		}
		original, err := s.ledgerRepo.GetJournalByRef(ctx, ref)
		if err != nil {
			return fmt.Errorf("membaca jurnal pencairan %s: %w", ref, err)
		}
		// Nomor koreksi dinaikkan di dalam transaksi ini, bukan sebelumnya: bila
		// transaksi gagal, kenaikannya ikut ter-rollback dan nomor tidak terpakai.
		sequence, err := s.loanRepo.NextCorrectionCountTx(ctx, tx, loan.ID)
		if err != nil {
			return fmt.Errorf("menaikkan penghitung koreksi nominal: %w", err)
		}
		journalRef, err = s.postLoanCorrectionTx(ctx, tx, original, loan, input, diff, sequence, entryDate, actor)
		if err != nil {
			return err
		}
	}

	loan.PrincipalAmount = input.NewAmount
	loan.TotalPayable = totalPayable
	loan.MonthlyInstallment = monthly
	loan.OutstandingPrincipal = outstanding
	loan.Schedules = updated
	if err := s.loanRepo.CorrectLoanAmountTx(ctx, tx, loan, updated); err != nil {
		return fmt.Errorf("menyimpan koreksi nominal: %w", err)
	}

	return writeAudit(ctx, s.auditRepo, tx, actor, "CORRECT_LOAN_AMOUNT", "loan", loan.ID.String(), map[string]any{
		"loan_number":       loan.LoanNumber,
		"old_amount":        oldAmount.StringFixed(2),
		"new_amount":        input.NewAmount.StringFixed(2),
		"difference":        diff.StringFixed(2),
		"journal_reference": journalRef,
		"reason":            input.Reason,
	})
}

// postLoanCorrectionTx memposting jurnal selisih koreksi lewat posting engine.
// Akunnya tidak diambil dari COA keras melainkan dari jurnal pencairan asli: kaki
// kas/rekening adalah sisi debit jurnal asal dan lawannya adalah sisi kredit (akun
// pokok), sehingga uang kembali ke/keluar dari tempat yang sama seperti pencairan
// awal. Koreksi naik mengikuti arah pencairan, koreksi turun membalik arahnya.
func (s *loanService) postLoanCorrectionTx(ctx context.Context, tx any, original *domain.JournalEntry, loan *domain.Loan, input domain.CorrectLoanAmountInput, diff decimal.Decimal, sequence int, entryDate time.Time, actor domain.Actor) (string, error) {
	debitLeg, creditLeg, err := disbursementLegs(original)
	if err != nil {
		return "", err
	}
	posted, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeAdjustment,
		EntryDate:       entryDate,
		Description:     fmt.Sprintf("Koreksi nominal kredit %s — %s", loan.LoanNumber, input.Reason),
		// Kunci memuat urutan koreksi per kredit, bukan hanya nominal baru. Rangkaian
		// 10jt -> 12jt -> 10jt -> 12jt memakai kembali kunci "-12000000" pada langkah
		// terakhir bila urutannya tidak disertakan; posting engine lalu mengembalikan
		// jurnal lama tanpa menulis selisih, padahal state kredit tetap berubah.
		// Penghitung dinaikkan di dalam transaksi yang sama (correctLoanAmountTx) agar
		// transaksi gagal tidak menghabiskan nomor.
		//
		// Batas jaminan: penghitung ini membedakan peristiwa koreksi yang berbeda, bukan
		// mengenali ulangan permintaan yang sama. Pengulangan permintaan yang sama
		// setelah transaksi sukses tetap dapat menghasilkan jurnal kedua; jaminan penuh
		// butuh kunci idempotensi dari pemanggil (mis. ID permintaan API), yang saat ini
		// belum diteruskan ke sini. Validasi ErrLoanAmountUnchanged hanya menolak ulangan
		// selama pokok kredit belum berubah.
		IdempotencyKey: "CORR-" + loan.LoanNumber + "-" + input.NewAmount.String() + "-" + strconv.Itoa(sequence),
		CreatedBy:      actor.DisplayName(),
		// Cabang mewarisi jurnal pencairan asal agar laporan per cabang tetap benar.
		BranchCode: original.BranchCode,
		Source:     domain.SourceLoan,
		Lines:      correctionLines(debitLeg, creditLeg, diff.Abs(), diff.IsPositive(), loan.LoanNumber),
	})
	if err != nil {
		return "", fmt.Errorf("jurnal koreksi nominal: %w", err)
	}
	return posted.ReferenceNumber, nil
}

// disbursementLegs memisahkan jurnal pencairan menjadi kaki debit dan kaki kredit.
// Nama "kas" dan "pokok" sengaja tidak dipakai: pada pencairan kredit, kaki DEBIT adalah
// piutang kredit (aset bertambah) dan kaki KREDIT adalah rekening nasabah (kewajiban
// bertambah) — kebalikan dari dugaan yang paling mudah dibuat, dan penamaan yang keliru
// di sini akan menyesatkan orang berikutnya yang menyentuh kode ini. Koreksi hanya perlu
// mempertahankan arah ASLI jurnal (saat nominal naik) atau membalikkannya (saat turun),
// sehingga tidak perlu tahu akun mana yang mana.
//
// Jumlah baris diperiksa ketat. Bila jurnal pencairan memuat lebih dari satu baris per
// arah (mis. biaya administrasi atau akrual ikut tercatat di jurnal yang sama), mengambil
// baris pertama akan mengarahkan koreksi ke akun yang salah tanpa peringatan apa pun —
// lebih baik menolak dan meminta mata manusia melihat jurnalnya.
func disbursementLegs(original *domain.JournalEntry) (debit, credit domain.JournalLine, err error) {
	for _, line := range original.Lines {
		if line.AccountNumber == "" {
			return debit, credit, fmt.Errorf("baris jurnal %s tidak memiliki nomor rekening", line.ID)
		}
		switch line.Direction {
		case domain.DirectionDebit:
			if debit.AccountNumber != "" {
				return debit, credit, fmt.Errorf("jurnal pencairan %s memiliki lebih dari satu kaki debit", original.ReferenceNumber)
			}
			debit = line
		case domain.DirectionCredit:
			if credit.AccountNumber != "" {
				return debit, credit, fmt.Errorf("jurnal pencairan %s memiliki lebih dari satu kaki kredit", original.ReferenceNumber)
			}
			credit = line
		}
	}
	if debit.AccountNumber == "" || credit.AccountNumber == "" {
		return debit, credit, fmt.Errorf("jurnal pencairan %s tidak memiliki kaki debit dan kredit yang lengkap", original.ReferenceNumber)
	}
	return debit, credit, nil
}

func correctionLines(debit, credit domain.JournalLine, amount decimal.Decimal, increase bool, loanNumber string) []domain.PostingLine {
	debitDirection := domain.DirectionDebit
	creditDirection := domain.DirectionCredit
	if !increase {
		debitDirection = domain.DirectionCredit
		creditDirection = domain.DirectionDebit
	}
	description := "Koreksi nominal kredit " + loanNumber
	return []domain.PostingLine{
		{AccountNumber: debit.AccountNumber, Direction: debitDirection, Amount: amount, Description: description},
		{AccountNumber: credit.AccountNumber, Direction: creditDirection, Amount: amount, Description: description},
	}
}

// recalculateSchedules menghitung ulang hanya angsuran yang belum dibayar. Jumlah
// angsuran dan tanggal jatuh tempo tetap sama; pokok angsuran yang belum dibayar
// disesuaikan agar total pokok seluruh jadwal sama dengan nominal baru, dan selisih
// pembulatan diserap angsuran belum dibayar terakhir. Angsuran yang sudah dibayar
// disalin apa adanya.
//
// Margin/bunga per angsuran TIDAK diubah. Untuk produk syariah, margin adalah bagian
// akad yang melekat pada harga jual; mengubahnya berarti mengubah akad, sedangkan
// koreksi ini hanya mengoreksi nominal pokok.
func recalculateSchedules(schedules []domain.LoanSchedule, newAmount decimal.Decimal) ([]domain.LoanSchedule, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	updated := make([]domain.LoanSchedule, len(schedules))
	copy(updated, schedules)

	paidPrincipal := decimal.Zero
	frozenPrincipal := decimal.Zero
	unpaid := make([]int, 0, len(updated))
	for i := range updated {
		s := &updated[i]
		if s.PaidPrincipal.IsZero() && s.PaidProfit.IsZero() && s.Status == domain.InstallmentStatusPending {
			unpaid = append(unpaid, i)
			continue
		}
		paidPrincipal = paidPrincipal.Add(s.PaidPrincipal)
		frozenPrincipal = frozenPrincipal.Add(s.PrincipalAmount)
	}
	if len(unpaid) == 0 {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, errors.New("tidak ada angsuran belum dibayar yang dapat disesuaikan")
	}
	// Sisa pokok baru adalah nominal baru dikurangi pokok yang sudah dibayar.
	outstanding := newAmount.Sub(paidPrincipal)
	if outstanding.IsNegative() {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, errors.New("nominal baru lebih kecil daripada pokok yang sudah dibayar")
	}
	// Pokok yang dibagi ke angsuran belum dibayar adalah nominal baru dikurangi
	// pokok jadwal yang dibekukan, supaya total pokok seluruh jadwal tepat nominal baru.
	distributable := newAmount.Sub(frozenPrincipal)
	if distributable.IsNegative() {
		return nil, decimal.Zero, decimal.Zero, decimal.Zero, errors.New("nominal baru lebih kecil daripada pokok jadwal yang sudah dibayar")
	}
	base := domain.RoundToRupiah(distributable.Div(decimal.NewFromInt(int64(len(unpaid)))))
	allocated := decimal.Zero
	for pos, i := range unpaid {
		s := &updated[i]
		principalPart := base
		if pos == len(unpaid)-1 {
			principalPart = distributable.Sub(allocated)
		}
		allocated = allocated.Add(principalPart)
		s.PrincipalAmount = principalPart
		s.TotalInstallment = principalPart.Add(s.ProfitAmount)
	}

	totalPayable := decimal.Zero
	for _, s := range updated {
		totalPayable = totalPayable.Add(s.TotalInstallment)
	}
	return updated, outstanding, totalPayable, updated[unpaid[0]].TotalInstallment, nil
}

// businessDate membaca tanggal bisnis berjalan dari repositori tanggal, bukan dari
// layanan konfigurasi yang menyimpan nilainya di cache: nilai yang basi membuat jurnal
// kontra jatuh di periode yang salah.
func (s *loanService) businessDate(ctx context.Context) (time.Time, error) {
	if s.dates == nil {
		return time.Time{}, errors.New("sumber tanggal bisnis belum terpasang")
	}
	current, err := s.dates.GetCurrentDate(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("membaca tanggal bisnis: %w", err)
	}
	if current == nil || current.CurrentDate.IsZero() {
		return time.Time{}, errors.New("tanggal bisnis tidak tersedia")
	}
	return current.CurrentDate, nil
}

// postLoanReversalTx menulis jurnal kontra atas jurnal pencairan di dalam transaksi
// pemanggil. Ini salinan kecil dari ledgerService.postReversalTx: kedua service
// tidak boleh saling bergantung, dan pembatalan pencairan punya konsekuensi domain
// sendiri. Arah baris ditukar dan nominal tetap penuh — tidak ada pembatalan sebagian.
func (s *loanService) postLoanReversalTx(ctx context.Context, tx any, original *domain.JournalEntry, loan *domain.Loan, reason string, actor domain.Actor) (*domain.JournalEntry, error) {
	if len(original.Lines) == 0 {
		return nil, fmt.Errorf("jurnal pencairan %s tidak memiliki baris untuk dibalik", original.ReferenceNumber)
	}
	lines := make([]domain.PostingLine, 0, len(original.Lines))
	for _, line := range original.Lines {
		if line.AccountNumber == "" {
			return nil, fmt.Errorf("baris jurnal %s tidak memiliki nomor rekening", line.ID)
		}
		direction := domain.DirectionDebit
		if line.Direction == domain.DirectionDebit {
			direction = domain.DirectionCredit
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: line.AccountNumber,
			Direction:     direction,
			Amount:        line.Amount,
			Description:   "Pembatalan pencairan " + loan.LoanNumber,
		})
	}

	// Tanggal bisnis berjalan, bukan tanggal kalender. Pembatalan yang jatuh di periode
	// yang belum dibuka tidak akan ikut terhitung tutup hari, sehingga buku cabang
	// berbeda dari kenyataan sampai tutup hari menyusul.
	entryDate, err := s.businessDate(ctx)
	if err != nil {
		return nil, err
	}

	posted, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeReversal,
		EntryDate:       entryDate,
		Description:     fmt.Sprintf("Pembatalan pencairan kredit %s — %s", loan.LoanNumber, reason),
		// Kunci tetap per jurnal asal: pengulangan mengembalikan jurnal kontra yang
		// sama, bukan membuat pembalikan kedua.
		IdempotencyKey: "REV-" + original.ReferenceNumber,
		CreatedBy:      actor.DisplayName(),
		// Cabang mewarisi jurnal asal; tanpa ini kontra tersimpan bank-wide dan
		// laporan per cabang tidak lagi mencerminkan asalnya.
		BranchCode: original.BranchCode,
		Source:     domain.SourceLoan,
		Lines:      lines,
	})
	if err != nil {
		return nil, fmt.Errorf("jurnal kontra pencairan: %w", err)
	}

	// Penandaan status asal berada di transaksi yang sama dengan jurnal kontranya:
	// terpisah, kegagalan di antaranya meninggalkan jurnal POSTED padahal sudah ada
	// kontranya, dan pembalikan kedua bisa lolos.
	if err := s.ledgerRepo.MarkJournalReversed(ctx, tx, original.ReferenceNumber); err != nil {
		return nil, err
	}
	return posted, nil
}
