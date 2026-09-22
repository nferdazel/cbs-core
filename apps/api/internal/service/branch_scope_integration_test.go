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
