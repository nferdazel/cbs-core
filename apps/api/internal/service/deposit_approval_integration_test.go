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

// Uji integrasi penempatan deposito di atas ambang persetujuan terhadap PostgreSQL
// sungguhan. Mengikuti pola moneyflow_integration_test.go: di-skip kecuali
// CBS_TEST_DB_DSN diisi, sehingga `go test ./...` tetap hijau tanpa database.
//
//	CBS_TEST_DB_DSN='postgres://cbs_app:...@127.0.0.1:55471/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiPenempatanDeposito -v
//
// Yang dibuktikan: penempatan 150 juta (di atas limit.teller.deposit.approval_above
// = 100 juta) tidak menulis kontrak/jurnal apa pun dan mengembalikan pengajuan
// persetujuan; dana baru masuk setelah pengajuan disetujui.

type depositApprovalEnv struct {
	money *moneyEnv
	// teller memakai UserID staf nyata (FK audit) tetapi peran/branch teller, agar
	// limit.teller.deposit.* yang teruji -- bukan limit.superadmin.
	teller  domain.Actor
	checker domain.Actor
	svc     domain.DepositService
	mcSvc   domain.MakerCheckerService
}

func newDepositApprovalEnv(t *testing.T) *depositApprovalEnv {
	t.Helper()
	money := newMoneyEnv(t)
	ctx := money.ctx

	// Batas harian teller dinolkan agar uji fokus pada urutan ambang persetujuan
	// versus batas per transaksi. Tanpa ini, pengajuan yang disetujui pada jalan
	// sebelumnya menumpuk di akumulasi harian dan membuat uji tidak deterministik.
	setLimitConfig(t, money, "limit.teller.deposit.daily", "0")
	t.Cleanup(func() { setLimitConfig(t, money, "limit.teller.deposit.daily", "500000000") })

	auditRepo := postgres.NewAuditRepository(money.db)
	accountRepo := postgres.NewAccountRepository(money.db)
	ledgerRepo := postgres.NewLedgerRepository(money.db)
	customerRepo := postgres.NewCustomerRepository(money.db)
	branchRepo := postgres.NewBranchRepository(money.db)
	numberingRepo := postgres.NewNumberingRepository(money.db)
	depositRepo := postgres.NewDepositRepository(money.db)
	dateRepo := postgres.NewBusinessDateRepository(money.db)
	referenceGen := postgres.NewReferenceGenerator(money.db)

	postingSvc := service.NewPostingService(money.db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(money.productRepo, ledgerRepo, postingSvc)
	limitSvc := service.NewTransactionLimitService(money.configSvc, ledgerRepo, postgres.NewBusinessDateRepository(money.db))

	executors := service.NewExecutorRegistry()
	mcSvc := service.NewMakerCheckerService(money.db, postgres.NewMakerCheckerRepository(money.db), auditRepo, money.configSvc, executors, postgres.NewBusinessDateRepository(money.db))
	depositSvc := service.NewDepositService(
		money.db, depositRepo, money.productRepo, accountRepo, ledgerRepo, customerRepo,
		branchRepo, numberingRepo, poster, postingSvc, ledgerRepo, money.configSvc,
		limitSvc, mcSvc, auditRepo,
	)
	executors.Register(service.ActionPlaceDeposit, depositSvc)

	teller := money.actor
	teller.Role = domain.RoleTeller
	teller.Username = "teller.depo.uji"

	// Pemeriksa harus staf nyata (FK audit) dan bukan pembuat pengajuan.
	checkerID := uuid.MustParse("c0000000-0000-0000-0000-0000000000aa")
	if _, err := money.db.ExecContext(ctx, `
		INSERT INTO staff_users (id, employee_id, username, full_name, email, password_hash, role, branch_code, is_active)
		VALUES ($1, 'EMP-2026-999', 'checker.depo.uji', 'Checker Uji', 'checker.depo.uji@cbs.local', 'x', 'SUPERVISOR', '001', TRUE)
		ON CONFLICT (id) DO NOTHING`, checkerID); err != nil {
		t.Fatalf("menyiapkan pemeriksa: %v", err)
	}
	checker := domain.Actor{UserID: checkerID, Username: "checker.depo.uji", Role: domain.RoleSupervisor, BranchCode: "001"}

	return &depositApprovalEnv{money: money, teller: teller, checker: checker, svc: depositSvc, mcSvc: mcSvc}
}

func (e *depositApprovalEnv) depositCount(t *testing.T, customerID uuid.UUID) int {
	t.Helper()
	var n int
	if err := e.money.db.QueryRowContext(e.money.ctx,
		`SELECT COUNT(*) FROM deposits WHERE customer_id = $1`, customerID).Scan(&n); err != nil {
		t.Fatalf("menghitung deposito: %v", err)
	}
	return n
}

func TestIntegrasiPenempatanDepositoLewatAmbangButuhPersetujuan(t *testing.T) {
	e := newDepositApprovalEnv(t)
	cust := e.money.newCustomer(t, "Deposan Uji", "")
	product, err := e.money.productRepo.GetByCode(e.money.ctx, "DEP-CONV")
	if err != nil {
		t.Fatalf("membaca produk deposito: %v", err)
	}

	input := domain.PlaceDepositInput{
		CustomerID:      cust.ID,
		ProductID:       product.ID,
		PlacementAmount: idr(150_000_000),
		TermMonths:      3,
		Currency:        "IDR",
		BranchCode:      "001",
	}

	// 150 juta > limit.teller.deposit.approval_above (100 juta): harus menjadi
	// pengajuan persetujuan, dan dana belum boleh masuk.
	_, err = e.svc.Place(e.money.ctx, input, e.teller)
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("penempatan 150 juta harus menunggu persetujuan, dapat %v", err)
	}
	if got := e.depositCount(t, cust.ID); got != 0 {
		t.Fatalf("deposito sudah tertulis sebelum persetujuan: %d baris", got)
	}

	// Setelah disetujui, barulah kontrak dan jurnal penempatan ditulis.
	if err := e.mcSvc.Approve(e.money.ctx, pending.RequestID, e.checker, "disetujui uji"); err != nil {
		t.Fatalf("persetujuan pengajuan: %v", err)
	}

	var depID uuid.UUID
	var amount decimal.Decimal
	var status string
	var createdBy string
	if err := e.money.db.QueryRowContext(e.money.ctx, `
		SELECT d.id, d.placement_amount, d.status, j.created_by
		FROM deposits d
		JOIN journal_entries j ON j.idempotency_key = 'DEP-PLACE-' || d.id::text
		WHERE d.customer_id = $1`, cust.ID).Scan(&depID, &amount, &status, &createdBy); err != nil {
		t.Fatalf("membaca deposito hasil persetujuan: %v", err)
	}
	if !amount.Equal(idr(150_000_000)) {
		t.Fatalf("nominal tersimpan %s, mau 150000000", amount)
	}
	if status != string(domain.DepositStatusPlaced) {
		t.Fatalf("status tersimpan %q, mau PLACED", status)
	}
	// Jurnal mencatat PEMBUAT (teller), bukan pejabat yang menyetujui.
	if createdBy != e.teller.Username {
		t.Fatalf("created_by jurnal %q, mau %q", createdBy, e.teller.Username)
	}
}

// Nominal di atas batas per transaksi tetapi belum melewati ambang persetujuan tetap
// ditolak: 60 juta > 50 juta per transaksi, di bawah 100 juta ambang persetujuan.
func TestIntegrasiPenempatanDepositoDitolakBatasPerTransaksi(t *testing.T) {
	e := newDepositApprovalEnv(t)
	cust := e.money.newCustomer(t, "Deposan Batas", "")
	product, err := e.money.productRepo.GetByCode(e.money.ctx, "DEP-CONV")
	if err != nil {
		t.Fatalf("membaca produk deposito: %v", err)
	}

	_, err = e.svc.Place(e.money.ctx, domain.PlaceDepositInput{
		CustomerID:      cust.ID,
		ProductID:       product.ID,
		PlacementAmount: idr(60_000_000),
		TermMonths:      3,
		Currency:        "IDR",
		BranchCode:      "001",
	}, e.teller)
	if !errors.Is(err, domain.ErrLimitPerTransaction) {
		t.Fatalf("60 juta harus ditolak batas per transaksi, dapat %v", err)
	}
	if got := e.depositCount(t, cust.ID); got != 0 {
		t.Fatalf("deposito tertulis padahal ditolak: %d baris", got)
	}
}
