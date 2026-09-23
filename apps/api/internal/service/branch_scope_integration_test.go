package service_test

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

var branchScopeNikCounter int64

func branchScopeNik() string {
	return fmt.Sprintf("%016d", (time.Now().UnixNano()+atomic.AddInt64(&branchScopeNikCounter, 1))%10_000_000_000_000_000)
}

// Superadmin bertindak lintas cabang (semantik kantor pusat). Kode cabangnya bisa
// saja 'HO' yang tidak ada di tabel branches — kasus lapangan — dan operasi tulis
// seperti pendaftaran nasabah tidak boleh gagal karenanya. Nasabah diatribusikan
// ke cabang kantor pusat. Peran bercabang biasa tetap wajib punya cabang terdaftar.
func TestIntegrasiCabangSuperadminLintasCabang(t *testing.T) {
	e := newMoneyEnv(t)

	superadmin := domain.Actor{
		UserID:     e.actor.UserID,
		Username:   "superadmin.kantor.pusat",
		Role:       domain.RoleSuperAdmin,
		BranchCode: "HO",
	}
	cust, err := e.customerSvc.RegisterCustomer(e.ctx, domain.CreateCustomerInput{
		FullName:     "Nasabah Kantor Pusat",
		IDCardNumber: branchScopeNik(),
	}, superadmin)
	if err != nil {
		t.Fatalf("superadmin lintas cabang harus dapat mendaftarkan nasabah: %v", err)
	}
	if cust.BranchID == nil {
		t.Fatal("nasabah superadmin harus diatribusikan ke kantor pusat, bukan tanpa cabang")
	}
	var headOffice bool
	if err := e.db.QueryRowContext(e.ctx, `SELECT is_head_office FROM branches WHERE id = $1`, *cust.BranchID).Scan(&headOffice); err != nil {
		t.Fatalf("membaca cabang nasabah: %v", err)
	}
	if !headOffice {
		t.Fatal("cabang nasabah superadmin bukan kantor pusat")
	}

	// Peran bercabang biasa dengan kode cabang tak terdaftar tetap ditolak agar
	// nasabah tidak tercatat di cabang yang tidak sah.
	teller := domain.Actor{
		UserID:     e.actor.UserID,
		Username:   "teller.ho",
		Role:       domain.RoleTeller,
		BranchCode: "HO",
	}
	if _, err := e.customerSvc.RegisterCustomer(e.ctx, domain.CreateCustomerInput{
		FullName:     "Nasabah Teller HO",
		IDCardNumber: branchScopeNik(),
	}, teller); !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("teller dengan cabang tak terdaftar harus ditolak ErrBranchNotFound, dapat %v", err)
	}
}

// K1: staf bercabang dengan cakupan teresolusi (seperti yang dipasang middleware)
// memanggil endpoint daftar. Sebelum perbaikan, branchReadClause membandingkan
// kolom branch_id (uuid) dengan himpunan kode cabang sehingga PostgreSQL menolak
// dengan SQLSTATE 22P02 dan endpoint mengembalikan 500 untuk SEMUA peran
// non-lintas-cabang. Uji ini mengunci tipe yang benar pada seluruh pemakaian
// klausa cakupan, sekaligus membuktikan cakupan baca tetap terpisah.
func TestIntegrasiDaftarCakupanCabangMemakaiIdBukanKode(t *testing.T) {
	e := newActorBranchEnv(t)
	seedOrgHierarchy(t, e)
	db, ctx := e.money.db, e.money.ctx

	customerRepo := postgres.NewCustomerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	loanRepo := postgres.NewLoanRepository(db)
	depositRepo := postgres.NewDepositRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	mcRepo := postgres.NewMakerCheckerRepository(db)
	dueRepo := postgres.NewDueRepository(db)
	ckpnRepo := postgres.NewCKPNRepository(db)
	ppapRepo := postgres.NewPPAPRepository(db)
	lpsRepo := postgres.NewLPSPlacementRepository(db)

	// Dua nasabah pada cabang berbeda: 811 (di bawah area A810) dan 821 (di bawah
	// wilayah R900, di luar area A810).
	actor811 := domain.Actor{UserID: e.money.actor.UserID, Username: "ao.811.k1", Role: domain.RoleAO, BranchCode: "811", BranchScope: domain.NewBranchScope([]string{"811"})}
	cust811, err := e.money.customerSvc.RegisterCustomer(ctx, domain.CreateCustomerInput{
		FullName: "Nasabah Cabang 811 K1", IDCardNumber: branchScopeNik(),
	}, actor811)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah 811: %v", err)
	}
	actor821 := domain.Actor{UserID: e.money.actor.UserID, Username: "ao.821.k1", Role: domain.RoleAO, BranchCode: "821", BranchScope: domain.NewBranchScope([]string{"821"})}
	cust821, err := e.money.customerSvc.RegisterCustomer(ctx, domain.CreateCustomerInput{
		FullName: "Nasabah Cabang 821 K1", IDCardNumber: branchScopeNik(),
	}, actor821)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah 821: %v", err)
	}

	product, err := e.money.productRepo.GetByCode(ctx, "TAB-CONV")
	if err != nil {
		t.Fatalf("membaca produk tabungan: %v", err)
	}
	acc811, err := e.accountSvc.OpenAccount(ctx, domain.OpenAccountInput{
		CustomerID: cust811.ID, ProductID: product.ID, Currency: "IDR",
	}, actor811)
	if err != nil {
		t.Fatalf("membuka rekening nasabah 811: %v", err)
	}

	customerIDs := func(actor domain.Actor) map[string]bool {
		list, _, err := customerRepo.List(ctx, 200, 0, domain.CustomerQuery{}, actor)
		if err != nil {
			t.Fatalf("daftar nasabah untuk %s (cabang %s): %v", actor.Username, actor.BranchCode, err)
		}
		ids := map[string]bool{}
		for _, rec := range list {
			ids[rec.ID.String()] = true
		}
		return ids
	}
	accountNumbers := func(actor domain.Actor) map[string]bool {
		list, _, err := accountRepo.ListAll(ctx, 200, 0, "", actor)
		if err != nil {
			t.Fatalf("daftar rekening untuk %s (cabang %s): %v", actor.Username, actor.BranchCode, err)
		}
		nums := map[string]bool{}
		for _, a := range list {
			nums[a.AccountNumber] = true
		}
		return nums
	}

	teller811 := domain.Actor{UserID: e.money.actor.UserID, Username: "teller.811.k1", Role: domain.RoleTeller, BranchCode: "811", BranchScope: domain.NewBranchScope([]string{"811"})}
	area810 := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.a810.k1", Role: domain.RoleSupervisor, BranchCode: "A810", BranchScope: domain.NewBranchScope([]string{"A810", "811", "812"})}
	region900 := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.r900.k1", Role: domain.RoleSupervisor, BranchCode: "R900", BranchScope: domain.NewBranchScope([]string{"R900", "A810", "811", "812", "821"})}
	outside := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.001.k1", Role: domain.RoleSupervisor, BranchCode: "001", BranchScope: domain.NewBranchScope([]string{"001"})}
	cross := domain.Actor{UserID: e.money.actor.UserID, Username: "super.k1", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	if ids := customerIDs(teller811); !ids[cust811.ID.String()] || ids[cust821.ID.String()] {
		t.Fatalf("cabang 811 harus melihat nasabahnya dan bukan 821: %v", ids)
	}
	if nums := accountNumbers(teller811); !nums[acc811.AccountNumber] {
		t.Fatalf("cabang 811 harus melihat rekening cabangnya: %v", nums)
	}
	if ids := customerIDs(area810); !ids[cust811.ID.String()] || ids[cust821.ID.String()] {
		t.Fatalf("area A810 harus melihat 811 dan bukan 821: %v", ids)
	}
	if ids := customerIDs(region900); !ids[cust811.ID.String()] || !ids[cust821.ID.String()] {
		t.Fatalf("wilayah R900 harus melihat 811 dan 821: %v", ids)
	}
	if ids := customerIDs(outside); ids[cust811.ID.String()] || ids[cust821.ID.String()] {
		t.Fatalf("cabang 001 tidak boleh melihat nasabah cakupan lain: %v", ids)
	}
	if nums := accountNumbers(outside); nums[acc811.AccountNumber] {
		t.Fatalf("cabang 001 tidak boleh melihat rekening cakupan lain: %v", nums)
	}
	if ids := customerIDs(cross); !ids[cust811.ID.String()] || !ids[cust821.ID.String()] {
		t.Fatalf("aktor lintas cabang harus melihat semua: %v", ids)
	}

	// Seluruh pemakaian branchReadClause lain harus lolos tipe (uri tidak 500),
	// termasuk jalur statement dan batch yang mengolah seluruh portofolio.
	until := time.Now().AddDate(0, 1, 0)
	if _, _, err := loanRepo.List(ctx, 50, 0, teller811); err != nil {
		t.Fatalf("daftar kredit: %v", err)
	}
	if _, _, err := depositRepo.List(ctx, 50, 0, teller811); err != nil {
		t.Fatalf("daftar deposito: %v", err)
	}
	if _, _, err := ledgerRepo.ListJournals(ctx, 50, 0, teller811); err != nil {
		t.Fatalf("daftar jurnal: %v", err)
	}
	if _, _, err := ledgerRepo.ListAccountStatements(ctx, acc811.ID, 50, 0, teller811); err != nil {
		t.Fatalf("statement rekening: %v", err)
	}
	if _, err := mcRepo.ListPending(ctx, teller811); err != nil {
		t.Fatalf("daftar pengajuan maker-checker: %v", err)
	}
	if _, err := dueRepo.ListDueLoanInstallments(ctx, time.Now(), until, teller811); err != nil {
		t.Fatalf("daftar jatuh tempo kredit: %v", err)
	}
	if _, err := dueRepo.ListDueDeposits(ctx, time.Now(), until, teller811); err != nil {
		t.Fatalf("daftar jatuh tempo deposito: %v", err)
	}
	if _, err := ckpnRepo.ListActiveLoans(ctx, teller811); err != nil {
		t.Fatalf("daftar kredit CKPN: %v", err)
	}
	if _, err := ppapRepo.ListDueLoans(ctx, time.Now(), teller811); err != nil {
		t.Fatalf("daftar kredit PPAP: %v", err)
	}
	if _, err := lpsRepo.ListPlacements(ctx, time.Now(), teller811); err != nil {
		t.Fatalf("daftar penempatan LPS: %v", err)
	}

	// Tulis: area boleh melayani cabang bawahan, cabang luar cakupan ditolak;
	// nasabah cabang lain (821) tidak boleh dilayani area A810.
	if _, err := e.accountSvc.OpenAccount(ctx, domain.OpenAccountInput{
		CustomerID: cust811.ID, ProductID: product.ID, Currency: "IDR",
	}, area810); err != nil {
		t.Fatalf("area A810 harus dapat membuka rekening cabang bawahannya: %v", err)
	}
	if _, err := e.accountSvc.OpenAccount(ctx, domain.OpenAccountInput{
		CustomerID: cust821.ID, ProductID: product.ID, Currency: "IDR",
	}, area810); !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("area A810 pada nasabah 821: err = %v, ingin ErrCrossBranchAccess", err)
	}
}

// Pembuatan cabang lewat jalur produksi benar-benar tersimpan dan tercatat di
// audit log. Kode ganda ditolak sebagai sentinel bisnis, bukan galat database.
func TestIntegrasiBuatCabangDanAudit(t *testing.T) {
	e := newMoneyEnv(t)
	if _, err := e.db.ExecContext(e.ctx, `DELETE FROM branches WHERE code = '909'`); err != nil {
		t.Fatalf("bersihkan cabang uji: %v", err)
	}

	branchSvc := service.NewBranchService(
		e.db,
		postgres.NewBranchRepository(e.db),
		nil,
		postgres.NewAuditRepository(e.db),
	)
	actor := domain.Actor{UserID: e.actor.UserID, Username: "superadmin.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	branch, err := branchSvc.CreateBranch(e.ctx, domain.CreateBranchInput{Code: "909", Name: "Cabang Uji 909"}, actor)
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if branch.Code != "909" || !branch.IsActive {
		t.Fatalf("cabang tersimpan %+v", branch)
	}

	var name string
	var active bool
	if err := e.db.QueryRowContext(e.ctx, `SELECT name, is_active FROM branches WHERE code = '909'`).Scan(&name, &active); err != nil {
		t.Fatalf("membaca cabang: %v", err)
	}
	if name != "Cabang Uji 909" || !active {
		t.Fatalf("baris cabang name=%q active=%v", name, active)
	}

	var auditCount int
	if err := e.db.QueryRowContext(e.ctx, `SELECT count(*) FROM audit_logs WHERE action = 'CREATE_BRANCH' AND resource_id = $1`, branch.ID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("membaca audit: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit CREATE_BRANCH = %d, ingin 1", auditCount)
	}

	if _, err := branchSvc.CreateBranch(e.ctx, domain.CreateBranchInput{Code: "909", Name: "Duplikat"}, actor); !errors.Is(err, domain.ErrBranchCodeExists) {
		t.Fatalf("kode ganda: err = %v, ingin ErrBranchCodeExists", err)
	}
}

// N4: staf bercabang biasa yang cakupannya teresolusi KOSONG (branch_code tidak cocok
// dengan unit mana pun) tidak boleh melihat data apa pun. Memperluas cakupan diam-diam
// akan membuka kebocoran; yang benar adalah menolak/memperbaiki, bukan melebarkan.
func TestIntegrasiCakupanKosongTidakMembocorkanData(t *testing.T) {
	e := newMoneyEnv(t)

	superadmin := domain.Actor{
		UserID: e.actor.UserID, Username: "superadmin.uji",
		Role: domain.RoleSuperAdmin, BranchCode: "001",
	}
	cust, err := e.customerSvc.RegisterCustomer(e.ctx, domain.CreateCustomerInput{
		FullName: "Nasabah 001", IDCardNumber: branchScopeNik(),
	}, superadmin)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah: %v", err)
	}

	// Cakupan kosong: meniru middleware yang gagal mencocokkan branch_code 'HO'
	// dengan unit mana pun (Loaded=true, himpunan kode kosong).
	empty := domain.Actor{
		UserID: e.actor.UserID, Username: "adminho", Role: domain.RoleAdmin,
		BranchCode: "HO", BranchScope: domain.NewBranchScope(nil),
	}
	repo := postgres.NewCustomerRepository(e.db)
	list, _, err := repo.List(e.ctx, 200, 0, domain.CustomerQuery{}, empty)
	if err != nil {
		t.Fatalf("daftar nasabah cakupan kosong: %v", err)
	}
	for _, rec := range list {
		if rec.ID == cust.ID {
			t.Fatal("cakupan kosong membocorkan nasabah cabang lain")
		}
	}
}
