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

type accountService struct {
	db           *sql.DB
	accountRepo  domain.AccountRepository
	customerRepo domain.CustomerRepository
	customerSvc  domain.CustomerService
	productRepo  domain.ProductRepository
	branchRepo   domain.BranchRepository
	numbering    domain.AccountNumberGenerator
	configSvc    domain.SystemConfigService
	auditRepo    domain.AuditRepository
}

func NewAccountService(
	db *sql.DB,
	accountRepo domain.AccountRepository,
	customerRepo domain.CustomerRepository,
	customerSvc domain.CustomerService,
	productRepo domain.ProductRepository,
	branchRepo domain.BranchRepository,
	numbering domain.AccountNumberGenerator,
	configSvc domain.SystemConfigService,
	auditSinks ...domain.AuditRepository,
) domain.AccountService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &accountService{
		db:           db,
		accountRepo:  accountRepo,
		customerRepo: customerRepo,
		customerSvc:  customerSvc,
		productRepo:  productRepo,
		branchRepo:   branchRepo,
		numbering:    numbering,
		configSvc:    configSvc,
		auditRepo:    auditRepo,
	}
}

// liabilityCOAForProduct memetakan produk simpanan ke akun kewajiban di COA.
// Buku (konvensional/syariah) memisahkan akun karena pelaporan keduanya tidak
// boleh dicampur, meski sama-sama kewajiban kepada nasabah.
func liabilityCOAForProduct(p *domain.BankingProduct) (string, error) {
	switch p.Book {
	case domain.BookConventional:
		switch p.Family {
		case domain.FamilySavings:
			return "20100", nil // Tabungan
		case domain.FamilyTimeDeposit:
			return "20200", nil // Deposito Berjangka
		case domain.FamilyCurrentAccount:
			return "20300", nil // Giro
		}
	case domain.BookSyariah:
		switch p.Family {
		case domain.FamilySavings:
			return "12100", nil // Tabungan Wadiah
		case domain.FamilyTimeDeposit:
			return "12300", nil // Deposito Mudharabah
		case domain.FamilyCurrentAccount:
			return "12300", nil // Giro (belum ada akun giro syariah terpisah)
		}
	}
	return "", fmt.Errorf("produk %s tidak untuk pembukaan rekening simpanan", p.Code)
}

func accountTypeForFamily(f domain.ProductFamily) domain.AccountType {
	switch f {
	case domain.FamilySavings:
		return domain.AccountTypeSavings
	case domain.FamilyCurrentAccount:
		return domain.AccountTypeChecking
	case domain.FamilyLoan:
		return domain.AccountTypeLoan
	default:
		return domain.AccountTypeSavings
	}
}

func (s *accountService) OpenAccount(ctx context.Context, input domain.OpenAccountInput, actor domain.Actor) (*domain.Account, error) {
	if input.Currency == "" {
		input.Currency = "IDR"
	}
	// Identitas cabang HANYA dari JWT. branch_code pada body request diabaikan
	// agar pemanggil tidak dapat menulis rekening di cabang lain.
	if actor.BranchCode == "" {
		return nil, fmt.Errorf("kode cabang aktor wajib diisi")
	}

	customer, err := s.customerRepo.GetByID(ctx, input.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("nasabah tidak valid: %w", err)
	}
	if customer.Status != domain.CustomerStatusActive {
		return nil, domain.ErrAccountInactive
	}

	product, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if !product.IsActive {
		return nil, fmt.Errorf("produk %s sedang tidak aktif", product.Code)
	}

	actorBranch, err := s.branchRepo.GetByCode(ctx, actor.BranchCode)
	if err != nil {
		return nil, fmt.Errorf("cabang tidak valid: %w", err)
	}
	// Penegakan kepemilikan cabang: nasabah di cabang lain ditolak. Nasabah tanpa
	// cabang (data pra-migrasi) dibiarkan agar operasional tidak terblokir.
	if !actor.IsCrossBranch() && customer.BranchID != nil && *customer.BranchID != actorBranch.ID {
		return nil, domain.ErrCrossBranchAccess
	}
	// Rekening mengikuti cabang nasabah. Aktor lintas cabang yang melayani nasabah
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

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	accountNumber, err := s.numbering.NextAccountNumber(ctx, tx, product, branch.Code)
	if err != nil {
		return nil, err
	}

	coaID, err := s.resolveCOAID(ctx, tx, coaCode)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	account := &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    accountNumber,
		CustomerID:       &customer.ID,
		ProductID:        &product.ID,
		BranchID:         &branch.ID,
		COAID:            coaID,
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
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("nomor rekening sudah terpakai, silakan coba lagi")
		}
		return nil, err
	}

	// Audit hanya memuat data non-pribadi rekening; nama/NIK nasabah tidak dicatat.
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "OPEN_ACCOUNT", "account", account.ID.String(), map[string]any{
		"account_number": account.AccountNumber,
		"product":        product.Code,
		"branch":         branch.Code,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return account, nil
}

// resolveCOAID mencari id akun COA berdasarkan kode. Dilakukan di dalam transaksi
// pembukaan rekening agar konsisten dengan insert.
func (s *accountService) resolveCOAID(ctx context.Context, tx *sql.Tx, code string) (uuid.UUID, error) {
	var id uuid.UUID
	err := tx.QueryRowContext(ctx, `SELECT id FROM chart_of_accounts WHERE code = $1`, code).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, fmt.Errorf("akun COA %s tidak ditemukan", code)
		}
		return uuid.Nil, err
	}
	return id, nil
}

// isUniqueViolation mendeteksi pelanggaran unique constraint. Driver pgx
// membungkusnya sebagai error SQLSTATE 23505; pengecekan string dipakai agar
// tidak menambah ketergantungan pada tipe driver tertentu.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "23505") || strings.Contains(msg, "duplicate key")
}

func (s *accountService) GetAccountByNumber(ctx context.Context, accountNumber string, actor domain.Actor) (*domain.Account, error) {
	account, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	// Rekening cabang lain ditolak tegas dengan 403, bukan disamarkan menjadi 404.
	// Rekening tanpa cabang (data pra-migrasi) tetap boleh dibaca.
	if !actor.CanAccessBranch(account.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	s.fillCustomerNames(ctx, []domain.Account{*account}, func(acc *domain.Account, name string) {
		account.CustomerName = name
	})
	return account, nil
}

func (s *accountService) ListAccounts(ctx context.Context, page, pageSize int, search string, actor domain.Actor) ([]domain.Account, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	accounts, total, err := s.accountRepo.ListAll(ctx, pageSize, offset, search, actor)
	if err != nil {
		return nil, 0, err
	}
	s.fillCustomerNames(ctx, accounts, func(acc *domain.Account, name string) {
		acc.CustomerName = name
	})
	return accounts, total, nil
}

// fillCustomerNames melengkapi nama nasabah pada daftar rekening dengan satu query
// batch, lalu mendekripsinya lewat customer service. Kegagalan pelengkapan nama
// tidak boleh menggagalkan operasi rekening; nama dibiarkan kosong.
func (s *accountService) fillCustomerNames(ctx context.Context, accounts []domain.Account, set func(*domain.Account, string)) {
	if s.customerSvc == nil || len(accounts) == 0 {
		return
	}
	ids := make([]uuid.UUID, 0, len(accounts))
	for i := range accounts {
		if accounts[i].CustomerID != nil {
			ids = append(ids, *accounts[i].CustomerID)
		}
	}
	if len(ids) == 0 {
		return
	}

	names, err := s.customerSvc.NamesByIDs(ctx, ids)
	if err != nil {
		return
	}
	for i := range accounts {
		if accounts[i].CustomerID == nil {
			continue
		}
		if name, ok := names[*accounts[i].CustomerID]; ok {
			set(&accounts[i], name)
		}
	}
}

// Kunci konfigurasi ambang dormant. Nilai produksi WAJIB diisi operator lewat
// system_config; fallback di domain.DormantAfterMonthsFallback hanya agar sistem
// tetap berjalan di lingkungan yang belum di-provision.
const cfgAccountDormantAfterMonths = "account.dormant.after_months"

// dormantAfterMonths membaca ambang dormant dari konfigurasi. Nilai <= 0 dianggap
// tidak valid sehingga fallback terdokumentasi dipakai disertai peringatan, bukan
// diam-diam melewati pekerjaan. Nilai non-numerik sudah ditangani SystemConfigService
// (memakai fallback + log), sehingga di sini cukup menjaga batas bawah.
func dormantAfterMonths(ctx context.Context, config domain.SystemConfigService) (int, string) {
	months := configIntOr(ctx, config, cfgAccountDormantAfterMonths, domain.DormantAfterMonthsFallback)
	if months <= 0 {
		return domain.DormantAfterMonthsFallback, fmt.Sprintf(
			"konfigurasi %s tidak valid (%d); memakai fallback %d bulan",
			cfgAccountDormantAfterMonths, months, domain.DormantAfterMonthsFallback)
	}
	return months, ""
}

// MarkDormant menandai rekening nasabah yang tidak ada aktivitas dalam ambang
// tertentu sebagai DORMANT. Pekerjaan ini bank-wide, jadi tidak ada filter cabang.
// Penandaan idempoten: hanya rekening ACTIVE yang disentuh, dan penjalanan ulang
// tidak mengubah apa pun. Tidak ada jurnal — dormant bukan peristiwa ekonomi.
func (s *accountService) MarkDormant(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.DormantRunSummary, error) {
	months, warning := dormantAfterMonths(ctx, s.configSvc)
	summary := domain.DormantRunSummary{Warning: warning}

	candidates, err := s.accountRepo.ListDormantCandidates(ctx)
	if err != nil {
		return summary, fmt.Errorf("mengambil kandidat rekening dormant: %w", err)
	}

	cutoff := domain.DormantCutoff(asOf, months)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return summary, err
	}
	defer tx.Rollback()

	for _, acc := range candidates {
		base, ok := domain.AccountActivityBase(acc.LastActivityAt, acc.OpenedAt, acc.CreatedAt)
		if !ok {
			// Tidak ada waktu sama sekali: dilewati, bukan dikarang agar tidak menandai
			// rekening yang sebenarnya baru.
			summary.Skipped++
			continue
		}
		if !domain.IsDormantDue(base, cutoff) {
			continue
		}
		marked, err := s.accountRepo.MarkDormant(ctx, tx, acc.ID)
		if err != nil {
			return summary, fmt.Errorf("menandai rekening %s dormant: %w", acc.AccountNumber, err)
		}
		if marked {
			summary.Marked++
		}
	}

	if err := tx.Commit(); err != nil {
		return summary, err
	}
	return summary, nil
}

// ReactivateAccount memulihkan rekening dormant ke ACTIVE. Reaktivasi adalah tindakan
// eksplisit di cabang: setoran tidak pernah mengaktifkan rekening. Rekening yang
// berstatus selain DORMANT (termasuk FROZEN/CLOSED) ditolak dengan pesan status
// sebenarnya agar operator tidak salah paham, bukan diam-diam sukses.
func (s *accountService) ReactivateAccount(ctx context.Context, accountNumber, notes string, actor domain.Actor) (*domain.Account, error) {
	account, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	// Reaktivasi terikat cabang rekening; aktor cabang lain ditolak 403, sama seperti
	// operasi rekening lain. Rekening tanpa cabang (data pra-migrasi) tetap boleh.
	if !actor.CanAccessBranch(account.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	if account.Status != domain.AccountStatusDormant {
		return nil, fmt.Errorf("%w: status rekening saat ini %s", domain.ErrAccountNotDormant, account.Status)
	}

	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	changed, err := s.accountRepo.Reactivate(ctx, tx, account.ID, now)
	if err != nil {
		return nil, err
	}
	if !changed {
		// Status berubah antara pembacaan dan penulisan; operator perlu memuat ulang.
		return nil, fmt.Errorf("%w: status rekening berubah, silakan muat ulang", domain.ErrAccountNotDormant)
	}

	// Audit mengikuti pola OpenAccount: non-pribadi, di dalam transaksi bisnis.
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "REACTIVATE_ACCOUNT", "account", account.ID.String(), map[string]any{
		"account_number": account.AccountNumber,
		"status_before":  string(domain.AccountStatusDormant),
		"status_after":   string(domain.AccountStatusActive),
		"notes":          notes,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	account.Status = domain.AccountStatusActive
	account.DormantAt = nil
	account.LastActivityAt = &now
	account.UpdatedAt = now
	return account, nil
}
