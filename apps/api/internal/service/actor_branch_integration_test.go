package service_test

import (
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// actorBranchEnv merakit jalur tulis produksi yang menyelesaikan cabang pelaku:
// pembukaan rekening dan penempatan deposito, di atas database sungguhan.
type actorBranchEnv struct {
	money      *moneyEnv
	accountSvc domain.AccountService
	depositSvc domain.DepositService
}

func newActorBranchEnv(t *testing.T) *actorBranchEnv {
	t.Helper()
	m := newMoneyEnv(t)

	auditRepo := postgres.NewAuditRepository(m.db)
	accountRepo := postgres.NewAccountRepository(m.db)
	ledgerRepo := postgres.NewLedgerRepository(m.db)
	customerRepo := postgres.NewCustomerRepository(m.db)
	branchRepo := postgres.NewBranchRepository(m.db)
	numberingRepo := postgres.NewNumberingRepository(m.db)
	depositRepo := postgres.NewDepositRepository(m.db)
	dateRepo := postgres.NewBusinessDateRepository(m.db)
	referenceGen := postgres.NewReferenceGenerator(m.db)

	postingSvc := service.NewPostingService(m.db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(m.productRepo, ledgerRepo, postingSvc)
	limitSvc := service.NewTransactionLimitService(m.configSvc, ledgerRepo, dateRepo)
	mcSvc := service.NewMakerCheckerService(m.db, postgres.NewMakerCheckerRepository(m.db), auditRepo, m.configSvc, service.NewExecutorRegistry(), dateRepo, branchRepo)

	accountSvc := service.NewAccountService(m.db, accountRepo, customerRepo, m.customerSvc, m.productRepo, branchRepo, numberingRepo, m.configSvc, auditRepo)
	depositSvc := service.NewDepositService(
		m.db, depositRepo, m.productRepo, accountRepo, ledgerRepo, customerRepo,
		branchRepo, numberingRepo, poster, postingSvc, ledgerRepo, m.configSvc,
		limitSvc, mcSvc, auditRepo,
	)
	return &actorBranchEnv{money: m, accountSvc: accountSvc, depositSvc: depositSvc}
}

func (e *actorBranchEnv) superadminHO() domain.Actor {
	return domain.Actor{
		UserID:     e.money.actor.UserID,
		Username:   "superadmin.kantor.pusat",
		Role:       domain.RoleSuperAdmin,
		BranchCode: "HO",
	}
}

func (e *actorBranchEnv) headOfficeID(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := e.money.db.QueryRowContext(e.money.ctx,
		`SELECT id FROM branches WHERE is_head_office = TRUE ORDER BY code LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("membaca kantor pusat: %v", err)
	}
	return id
}

// Superadmin dengan kode cabang 'HO' yang tidak terdaftar harus dapat membuka
// rekening; rekening diatribusikan ke kantor pusat, bukan cabang NULL.
func TestIntegrasiBukaRekeningSuperadminTanpaCabangTerdaftar(t *testing.T) {
	e := newActorBranchEnv(t)
	superadmin := e.superadminHO()
	cust, err := e.money.customerSvc.RegisterCustomer(e.money.ctx, domain.CreateCustomerInput{
		FullName:     "Nasabah Rekening Kantor Pusat",
		IDCardNumber: branchScopeNik(),
	}, superadmin)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah superadmin: %v", err)
	}
	product, err := e.money.productRepo.GetByCode(e.money.ctx, "TAB-CONV")
	if err != nil {
		t.Fatalf("membaca produk tabungan: %v", err)
	}
	input := domain.OpenAccountInput{CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR"}

	acc, err := e.accountSvc.OpenAccount(e.money.ctx, input, superadmin)
	if err != nil {
		t.Fatalf("superadmin lintas cabang harus dapat membuka rekening: %v", err)
	}
	if acc.BranchID == nil {
		t.Fatal("rekening superadmin harus punya cabang, bukan NULL")
	}
	if *acc.BranchID != e.headOfficeID(t) {
		t.Fatalf("rekening harus diatribusikan ke kantor pusat, dapat %s", acc.BranchID)
	}

	// Peran bercabang biasa dengan kode cabang tak terdaftar tetap ditolak.
	teller := domain.Actor{UserID: e.money.actor.UserID, Username: "teller.ho", Role: domain.RoleTeller, BranchCode: "HO"}
	if _, err := e.accountSvc.OpenAccount(e.money.ctx, input, teller); !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("teller HO harus ditolak ErrBranchNotFound, dapat %v", err)
	}
}

// Superadmin dengan kode cabang 'HO' yang tidak terdaftar harus dapat menempatkan
// deposito; kontraknya diatribusikan ke kantor pusat.
func TestIntegrasiTempatkanDepositoSuperadminTanpaCabangTerdaftar(t *testing.T) {
	e := newActorBranchEnv(t)
	superadmin := e.superadminHO()
	cust, err := e.money.customerSvc.RegisterCustomer(e.money.ctx, domain.CreateCustomerInput{
		FullName:     "Nasabah Deposito Kantor Pusat",
		IDCardNumber: branchScopeNik(),
	}, superadmin)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah superadmin: %v", err)
	}
	product, err := e.money.productRepo.GetByCode(e.money.ctx, "DEP-CONV")
	if err != nil {
		t.Fatalf("membaca produk deposito: %v", err)
	}
	input := domain.PlaceDepositInput{
		CustomerID:      cust.ID,
		ProductID:       product.ID,
		PlacementAmount: decimal.NewFromInt(1_000_000),
		TermMonths:      3,
		Currency:        "IDR",
	}

	dep, err := e.depositSvc.Place(e.money.ctx, input, superadmin)
	if err != nil {
		t.Fatalf("superadmin lintas cabang harus dapat menempatkan deposito: %v", err)
	}
	if dep.BranchID == nil {
		t.Fatal("deposito superadmin harus punya cabang, bukan NULL")
	}
	if *dep.BranchID != e.headOfficeID(t) {
		t.Fatalf("deposito harus diatribusikan ke kantor pusat, dapat %s", dep.BranchID)
	}

	// Jurnal penempatan juga harus memakai cabang yang sudah diselesaikan, bukan
	// kode 'HO' mentah yang menghasilkan branch_id NULL dan menghilangkan baris dari
	// laporan per cabang. Scan ke string gagal bila nilainya NULL.
	var journalBranchID string
	if err := e.money.db.QueryRowContext(e.money.ctx,
		`SELECT branch_id FROM journal_entries WHERE transaction_type = 'DEPOSIT_PLACEMENT' ORDER BY created_at DESC LIMIT 1`).Scan(&journalBranchID); err != nil {
		t.Fatalf("membaca cabang jurnal penempatan (NULL = gagal): %v", err)
	}
	if journalBranchID != e.headOfficeID(t).String() {
		t.Fatalf("jurnal penempatan harus diatribusikan ke kantor pusat %s, dapat %s", e.headOfficeID(t), journalBranchID)
	}

	// Peran bercabang biasa dengan kode cabang tak terdaftar tetap ditolak.
	teller := domain.Actor{UserID: e.money.actor.UserID, Username: "teller.depo.ho", Role: domain.RoleTeller, BranchCode: "HO"}
	if _, err := e.depositSvc.Place(e.money.ctx, input, teller); !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("teller HO harus ditolak ErrBranchNotFound, dapat %v", err)
	}
}
