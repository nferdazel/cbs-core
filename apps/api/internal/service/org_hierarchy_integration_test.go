package service_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// seedOrgHierarchy menyiapkan jenjang wilayah -> area -> cabang yang dipakai uji
// cakupan (W14). Idempotent (ON CONFLICT) supaya suite dapat dijalankan berulang
// pada database uji yang sama. Bank tanpa area/wilayah tidak tersentuh: unit ini
// hanya ada bila sebuah bank menyalakan hierarki.
func seedOrgHierarchy(t *testing.T, e *actorBranchEnv) {
	t.Helper()
	stmts := []string{
		`INSERT INTO branches (id, code, name, is_head_office, is_active, unit_level, parent_id)
		 VALUES (uuid_generate_v4(), 'R900', 'Wilayah Uji', FALSE, TRUE, 'WILAYAH', NULL)
		 ON CONFLICT (code) DO UPDATE SET unit_level='WILAYAH', parent_id=NULL`,
		`INSERT INTO branches (id, code, name, is_head_office, is_active, unit_level, parent_id)
		 VALUES (uuid_generate_v4(), 'A810', 'Area Uji', FALSE, TRUE, 'AREA',
		         (SELECT id FROM branches WHERE code='R900'))
		 ON CONFLICT (code) DO UPDATE SET unit_level='AREA',
		         parent_id=(SELECT id FROM branches WHERE code='R900')`,
		`INSERT INTO branches (id, code, name, is_head_office, is_active, unit_level, parent_id)
		 VALUES (uuid_generate_v4(), '811', 'Cabang 811 Uji', FALSE, TRUE, 'CABANG',
		         (SELECT id FROM branches WHERE code='A810'))
		 ON CONFLICT (code) DO UPDATE SET unit_level='CABANG',
		         parent_id=(SELECT id FROM branches WHERE code='A810')`,
		`INSERT INTO branches (id, code, name, is_head_office, is_active, unit_level, parent_id)
		 VALUES (uuid_generate_v4(), '812', 'Cabang 812 Uji', FALSE, TRUE, 'CABANG',
		         (SELECT id FROM branches WHERE code='A810'))
		 ON CONFLICT (code) DO UPDATE SET unit_level='CABANG',
		         parent_id=(SELECT id FROM branches WHERE code='A810')`,
		`INSERT INTO branches (id, code, name, is_head_office, is_active, unit_level, parent_id)
		 VALUES (uuid_generate_v4(), '821', 'Cabang 821 Uji', FALSE, TRUE, 'CABANG',
		         (SELECT id FROM branches WHERE code='R900'))
		 ON CONFLICT (code) DO UPDATE SET unit_level='CABANG',
		         parent_id=(SELECT id FROM branches WHERE code='R900')`,
	}
	for _, stmt := range stmts {
		if _, err := e.money.db.ExecContext(e.money.ctx, stmt); err != nil {
			t.Fatalf("menyiapkan hierarki: %v", err)
		}
	}
}

// (a)+(b) Resolusi cakupan menurun dan bank tanpa area tetap satu unit.
func TestIntegrasiResolusiCakupanHierarki(t *testing.T) {
	e := newActorBranchEnv(t)
	seedOrgHierarchy(t, e)
	branchRepo := postgres.NewBranchRepository(e.money.db)

	cases := []struct {
		code string
		want []string
	}{
		{"811", []string{"811"}},
		{"A810", []string{"A810", "811", "812"}},
		{"R900", []string{"R900", "A810", "811", "812", "821"}},
		// Bank tanpa area/wilayah: kantor pusat hanya unitnya sendiri, tidak ada
		// turunan yang tiba-tiba terbuka karena kehadiran hierarki.
		{"001", []string{"001"}},
	}
	for _, tc := range cases {
		got, err := branchRepo.ResolveScopeCodes(e.money.ctx, tc.code)
		if err != nil {
			t.Fatalf("ResolveScopeCodes(%s): %v", tc.code, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("ResolveScopeCodes(%s) = %v, ingin %v", tc.code, got, tc.want)
		}
		set := map[string]bool{}
		for _, c := range got {
			set[c] = true
		}
		for _, c := range tc.want {
			if !set[c] {
				t.Fatalf("ResolveScopeCodes(%s) = %v, kehilangan %s", tc.code, got, c)
			}
		}
	}

	unknown, err := branchRepo.ResolveScopeCodes(e.money.ctx, "ZZZ-TIDAK-ADA")
	if err != nil {
		t.Fatalf("ResolveScopeCodes kode tak dikenal: %v", err)
	}
	if len(unknown) != 0 {
		t.Fatalf("kode tak dikenal harus menghasilkan cakupan kosong, dapat %v", unknown)
	}
}

// (b) Baca: cabang hanya cabangnya, area seluruh cabang di areanya, wilayah
// seluruh wilayahnya; luar cakupan ditolak 403.
func TestIntegrasiBacaCakupanAreaWilayah(t *testing.T) {
	e := newActorBranchEnv(t)
	seedOrgHierarchy(t, e)

	// Nasabah didaftarkan oleh pelaku cabang 811 (jalur produksi) sehingga
	// diatribusikan ke cabang itu.
	branch811 := domain.Actor{UserID: e.money.actor.UserID, Username: "ao.811", Role: domain.RoleAO, BranchCode: "811", BranchScope: domain.NewBranchScope([]string{"811"})}
	cust, err := e.money.customerSvc.RegisterCustomer(e.money.ctx, domain.CreateCustomerInput{
		FullName:     "Nasabah Cakupan Area",
		IDCardNumber: branchScopeNik(),
	}, branch811)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah: %v", err)
	}

	area := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.area", Role: domain.RoleSupervisor, BranchCode: "A810", BranchScope: domain.NewBranchScope([]string{"A810", "811", "812"})}
	region := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.wilayah", Role: domain.RoleSupervisor, BranchCode: "R900", BranchScope: domain.NewBranchScope([]string{"R900", "A810", "811", "812", "821"})}
	sibling := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.812", Role: domain.RoleSupervisor, BranchCode: "812", BranchScope: domain.NewBranchScope([]string{"812"})}
	outside := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.001", Role: domain.RoleSupervisor, BranchCode: "001", BranchScope: domain.NewBranchScope([]string{"001"})}

	if _, err := e.money.customerSvc.GetCustomer(e.money.ctx, cust.ID, area); err != nil {
		t.Fatalf("area harus membaca nasabah cabang bawahannya: %v", err)
	}
	if _, err := e.money.customerSvc.GetCustomer(e.money.ctx, cust.ID, region); err != nil {
		t.Fatalf("wilayah harus membaca nasabah di wilayahnya: %v", err)
	}
	if _, err := e.money.customerSvc.GetCustomer(e.money.ctx, cust.ID, branch811); err != nil {
		t.Fatalf("cabang harus membaca nasabahnya: %v", err)
	}
	if _, err := e.money.customerSvc.GetCustomer(e.money.ctx, cust.ID, sibling); !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("cabang lain di area yang sama: err = %v, ingin ErrCrossBranchAccess", err)
	}
	if _, err := e.money.customerSvc.GetCustomer(e.money.ctx, cust.ID, outside); !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("luar cakupan: err = %v, ingin ErrCrossBranchAccess", err)
	}
}

// (b)+(d) Tulis: area boleh membuka rekening nasabah cabang bawahannya, cabang di
// luar cakupan ditolak, dan cakupan buku tetap menolak rekening lini usaha lain.
func TestIntegrasiTulisCakupanAreaDanBuku(t *testing.T) {
	e := newActorBranchEnv(t)
	seedOrgHierarchy(t, e)

	actor811 := domain.Actor{UserID: e.money.actor.UserID, Username: "ao.811", Role: domain.RoleAO, BranchCode: "811", BranchScope: domain.NewBranchScope([]string{"811"})}
	cust, err := e.money.customerSvc.RegisterCustomer(e.money.ctx, domain.CreateCustomerInput{
		FullName:     "Nasabah Tulis Area",
		IDCardNumber: branchScopeNik(),
	}, actor811)
	if err != nil {
		t.Fatalf("mendaftarkan nasabah: %v", err)
	}
	product, err := e.money.productRepo.GetByCode(e.money.ctx, "TAB-CONV")
	if err != nil {
		t.Fatalf("membaca produk tabungan: %v", err)
	}

	area := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.area", Role: domain.RoleSupervisor, BranchCode: "A810", BranchScope: domain.NewBranchScope([]string{"A810", "811", "812"}), Book: domain.BookConventional}
	acc, err := e.accountSvc.OpenAccount(e.money.ctx, domain.OpenAccountInput{CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR"}, area)
	if err != nil {
		t.Fatalf("area harus dapat membuka rekening nasabah cabang bawahannya: %v", err)
	}
	var branchCode string
	if err := e.money.db.QueryRowContext(e.money.ctx, `SELECT code FROM branches WHERE id=$1`, *acc.BranchID).Scan(&branchCode); err != nil {
		t.Fatalf("membaca cabang rekening: %v", err)
	}
	if branchCode != "811" {
		t.Fatalf("rekening harus di cabang nasabah 811, dapat %s", branchCode)
	}

	// Tulis dari cabang di luar cakupan ditolak sebelum menyentuh akuntansi.
	outside := domain.Actor{UserID: e.money.actor.UserID, Username: "spv.001", Role: domain.RoleSupervisor, BranchCode: "001", BranchScope: domain.NewBranchScope([]string{"001"})}
	if _, err := e.accountSvc.OpenAccount(e.money.ctx, domain.OpenAccountInput{CustomerID: cust.ID, ProductID: product.ID, Currency: "IDR"}, outside); !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("cabang luar cakupan menulis: err = %v, ingin ErrCrossBranchAccess", err)
	}

	// (d) Sumbu buku tetap berlaku bersamaan: aktor area dengan buku SYARIAH tidak
	// boleh membaca rekening konvensional, walau cabangnya dalam cakupan.
	areaSyariah := area
	areaSyariah.Book = domain.BookSyariah
	if _, err := e.accountSvc.GetAccountByNumber(e.money.ctx, acc.AccountNumber, areaSyariah); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("buku lain: err = %v, ingin ErrCrossBookAccess", err)
	}
	if _, err := e.accountSvc.GetAccountByNumber(e.money.ctx, acc.AccountNumber, area); err != nil {
		t.Fatalf("buku sendiri di cakupan area harus boleh dibaca: %v", err)
	}
}

// Pengelolaan susunan hierarki lewat jalur produksi: wilayah -> area -> cabang,
// tercatat audit, dan atasan lintas jenjang divalidasi.
func TestIntegrasiCreateOrgUnitProduksi(t *testing.T) {
	e := newActorBranchEnv(t)
	branchSvc := service.NewBranchService(e.money.db, postgres.NewBranchRepository(e.money.db), postgres.NewAuditRepository(e.money.db))
	actor := domain.Actor{UserID: e.money.actor.UserID, Username: "super.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}
	codes := []string{"899", "898", "R998", "A997"}
	for _, c := range codes {
		if _, err := e.money.db.ExecContext(e.money.ctx, `DELETE FROM branches WHERE code = $1`, c); err != nil {
			t.Fatalf("bersihkan unit uji: %v", err)
		}
	}
	t.Cleanup(func() {
		for _, c := range codes {
			_, _ = e.money.db.ExecContext(e.money.ctx, `DELETE FROM branches WHERE code = $1`, c)
		}
	})

	region, err := branchSvc.CreateOrgUnit(e.money.ctx, domain.CreateOrgUnitInput{Code: "R998", Name: "Wilayah Produksi", Level: domain.UnitLevelRegion}, actor)
	if err != nil {
		t.Fatalf("membuat wilayah: %v", err)
	}
	area, err := branchSvc.CreateOrgUnit(e.money.ctx, domain.CreateOrgUnitInput{Code: "A997", Name: "Area Produksi", Level: domain.UnitLevelArea, ParentCode: region.Code}, actor)
	if err != nil {
		t.Fatalf("membuat area: %v", err)
	}
	branch, err := branchSvc.CreateOrgUnit(e.money.ctx, domain.CreateOrgUnitInput{Code: "899", Name: "Cabang Produksi", Level: domain.UnitLevelBranch, ParentCode: area.Code}, actor)
	if err != nil {
		t.Fatalf("membuat cabang: %v", err)
	}
	if branch.ParentID == nil || *branch.ParentID != area.ID {
		t.Fatalf("cabang harus di bawah area, dapat %+v", branch)
	}

	// Atasan setingkat/lebih rendah ditolak.
	if _, err := branchSvc.CreateOrgUnit(e.money.ctx, domain.CreateOrgUnitInput{Code: "898", Name: "Area Salah", Level: domain.UnitLevelArea, ParentCode: branch.Code}, actor); !errors.Is(err, domain.ErrOrgUnitParentInvalid) {
		t.Fatalf("area di bawah cabang: err = %v, ingin ErrOrgUnitParentInvalid", err)
	}

	var auditCount int
	if err := e.money.db.QueryRowContext(e.money.ctx, `SELECT count(*) FROM audit_logs WHERE action='CREATE_ORG_UNIT' AND resource_id IN ($1,$2,$3)`, region.ID.String(), area.ID.String(), branch.ID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("membaca audit: %v", err)
	}
	if auditCount != 3 {
		t.Fatalf("audit CREATE_ORG_UNIT = %d, ingin 3", auditCount)
	}

	resolved, err := postgres.NewBranchRepository(e.money.db).ResolveScopeCodes(e.money.ctx, region.Code)
	if err != nil {
		t.Fatalf("resolusi wilayah produksi: %v", err)
	}
	if len(resolved) != 3 {
		t.Fatalf("wilayah produksi harus mencakup wilayah+area+cabang, dapat %v", resolved)
	}
}

// seedOrgStaffAt menyisipkan pengguna aktif di cabang tertentu (memakai penyisip
// pengguna uji izin) lalu memindahkan cabangnya. Dibersihkan otomatis.
func seedOrgStaffAt(t *testing.T, db *sql.DB, ctx context.Context, branchCode string) uuid.UUID {
	t.Helper()
	userID, _ := seedTempUser(t, db, ctx, domain.RoleSupervisor)
	if _, err := db.ExecContext(ctx, `UPDATE staff_users SET branch_code = $2 WHERE id = $1`, userID, branchCode); err != nil {
		t.Fatalf("memindahkan cabang pengguna uji: %v", err)
	}
	return userID
}

// Pemindahan cabang yang punya pengguna TIDAK mengunci pengguna di cabang itu,
// tetapi mengubah cakupan pengguna di unit atasan. Perubahan itu wajib dikonfirmasi
// agar tidak mengubah akses secara tak sengaja; setelah dikonfirmasi, staf di
// cabang yang dipindah tetap dapat mengakses cabangnya sendiri.
func TestIntegrasiPemindahanUnitKonfirmasiCakupanTanpaMengunci(t *testing.T) {
	e := newActorBranchEnv(t)
	seedOrgHierarchy(t, e)
	db, ctx := e.money.db, e.money.ctx
	branchSvc := service.NewBranchService(db, postgres.NewBranchRepository(db), postgres.NewAuditRepository(db))
	actor := domain.Actor{UserID: e.money.actor.UserID, Username: "super.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	// Pengguna di area A810 akan kehilangan cabang 811; pengguna di 811 tetap.
	seedOrgStaffAt(t, db, ctx, "A810")
	branchUser := seedOrgStaffAt(t, db, ctx, "811")

	// Kembalikan hierarki setelah uji agar tidak mempengaruhi uji lain.
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(),
			`UPDATE branches SET parent_id = (SELECT id FROM branches WHERE code='A810') WHERE code='811'`)
	})

	_, err := branchSvc.SetOrgUnitParent(ctx, "811", domain.SetOrgUnitParentInput{ParentCode: "R900"}, actor)
	var scope *domain.ScopeChangeError
	if !errors.As(err, &scope) {
		t.Fatalf("err = %v, ingin ScopeChangeError", err)
	}
	if scope.Losing < 1 {
		t.Fatalf("harus ada pengguna yang kehilangan cakupan, dapat %+v", scope)
	}
	if scope.Gaining != 0 {
		t.Fatalf("tidak ada yang mendapat cakupan, dapat gaining=%d", scope.Gaining)
	}
	t.Logf("dampak pemindahan 811: kehilangan=%d mendapat=%d", scope.Losing, scope.Gaining)

	var stillOldParent bool
	if err := db.QueryRowContext(ctx, `
		SELECT (SELECT parent_id FROM branches WHERE code='811') = (SELECT id FROM branches WHERE code='A810')`).Scan(&stillOldParent); err != nil {
		t.Fatalf("membaca parent: %v", err)
	}
	if !stillOldParent {
		t.Fatal("pemindahan ditolak tetapi parent sudah berubah")
	}

	if _, err := branchSvc.SetOrgUnitParent(ctx, "811", domain.SetOrgUnitParentInput{ParentCode: "R900", ConfirmScopeChange: true}, actor); err != nil {
		t.Fatalf("dengan konfirmasi seharusnya diterima: %v", err)
	}
	codes, err := postgres.NewBranchRepository(db).ResolveScopeCodes(ctx, "811")
	if err != nil {
		t.Fatalf("ResolveScopeCodes: %v", err)
	}
	userActor := domain.Actor{UserID: branchUser, Username: "spv.811", Role: domain.RoleSupervisor, BranchCode: "811", BranchScope: domain.NewBranchScope(codes)}
	if !userActor.CanAccessBranch("811") {
		t.Fatalf("staf di cabang yang dipindah kehilangan cakupan cabangnya sendiri: %v", codes)
	}
}
