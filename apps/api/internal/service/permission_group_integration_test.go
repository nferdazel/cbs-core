package service_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// Uji integrasi grup pengguna, izin dari database, dan menu terhadap PostgreSQL
// sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// app_info_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiIzin -v
//
// Membuktikan: (1) seed migrasi mereproduksi RolePermissions sehingga pengguna lama
// tidak kehilangan akses; (2) mengubah pemetaan izin lewat maker-checker mengubah
// akses tanpa rilis kode; (3) perubahan teraudit sebelum->sesudah; (4) pembuat tidak
// dapat menyetujui pengajuannya sendiri; (5) pencabutan berdampak butuh konfirmasi;
// (6) menu mengikuti izin efektif, bukan daftar peran di kode.

func newPermissionDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi grup/izin dilewati")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("membuka database: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database tidak dapat dihubungi: %v", err)
	}
	return db, ctx
}

// seedTempUser menyisipkan pengguna uji beserta sesi yang masih berlaku, lalu
// mengembalikan ID pengguna dan ID sesi. Diatur agar dibersihkan otomatis.
func seedTempUser(t *testing.T, db *sql.DB, ctx context.Context, role domain.StaffRole) (uuid.UUID, uuid.UUID) {
	t.Helper()
	username := fmt.Sprintf("uji-izin-%s-%d", role, time.Now().UnixNano())
	now := time.Now().UTC()
	user := &domain.StaffUser{
		ID:                uuid.New(),
		EmployeeID:        fmt.Sprintf("EMP-UJI-%d", time.Now().UnixNano()),
		Username:          username,
		FullName:          "Pengguna Uji Izin",
		Email:             username + "@uji.local",
		PasswordHash:      "x",
		Role:              role,
		BranchCode:        "HO",
		Book:              domain.BookConventional,
		IsActive:          true,
		PasswordChangedAt: now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := postgres.NewStaffRepository(db).Create(ctx, user); err != nil {
		t.Fatalf("menyisipkan pengguna uji: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM staff_users WHERE id = $1`, user.ID)
	})

	sessionID := uuid.New()
	session := &domain.StaffSession{
		ID:               sessionID,
		UserID:           user.ID,
		RefreshTokenHash: uuid.NewString(),
		IPAddress:        "127.0.0.1",
		UserAgent:        "uji",
		ExpiresAt:        now.Add(time.Hour),
		CreatedAt:        now,
	}
	if err := postgres.NewSessionRepository(db).Create(ctx, session); err != nil {
		t.Fatalf("menyisipkan sesi uji: %v", err)
	}
	return user.ID, sessionID
}

func identityPermissions(t *testing.T, db *sql.DB, ctx context.Context, sessionID uuid.UUID) ([]domain.Permission, []string) {
	t.Helper()
	identity, err := postgres.NewSessionRepository(db).GetIdentity(ctx, sessionID)
	if err != nil {
		t.Fatalf("membaca identitas sesi: %v", err)
	}
	return identity.Permissions, identity.Menus
}

func permissionSet(perms []domain.Permission) map[string]bool {
	out := map[string]bool{}
	for _, p := range perms {
		out[string(p)] = true
	}
	return out
}

func containsMenu(menus []string, key string) bool {
	for _, m := range menus {
		if m == key {
			return true
		}
	}
	return false
}

// TestIntegrasiIzinSeedTidakMenghilangkanAkses membuktikan pengguna lama tetap
// memperoleh persis izin perannya setelah migrasi, lewat jalur resolusi nyata.
func TestIntegrasiIzinSeedTidakMenghilangkanAkses(t *testing.T) {
	db, ctx := newPermissionDB(t)

	roles := []domain.StaffRole{
		domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleSupervisor,
		domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor,
	}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			_, sessionID := seedTempUser(t, db, ctx, role)
			got, _ := identityPermissions(t, db, ctx, sessionID)

			want := map[string]bool{}
			for _, p := range domain.RolePermissions[role] {
				want[string(p)] = true
			}
			have := permissionSet(got)
			if len(have) != len(want) {
				t.Fatalf("izin efektif %s = %d, mau %d\n  punya: %v\n  mau  : %v",
					role, len(have), len(want), have, want)
			}
			for p := range want {
				if !have[p] {
					t.Errorf("%s kehilangan izin %s setelah migrasi", role, p)
				}
			}
		})
	}
}

// permissionTestRig merakit layanan nyata (repo + maker-checker) di atas DB uji.
type permissionTestRig struct {
	db    *sql.DB
	ctx   context.Context
	perm  domain.PermissionService
	mc    domain.MakerCheckerService
	audit domain.AuditRepository
}

func newPermissionRig(t *testing.T, db *sql.DB, ctx context.Context) *permissionTestRig {
	t.Helper()
	configRepo := postgres.NewSystemConfigRepository(db)
	configSvc := service.NewSystemConfigService(configRepo)
	auditRepo := postgres.NewAuditRepository(db)
	executors := service.NewExecutorRegistry()
	mcRepo := postgres.NewMakerCheckerRepository(db)
	mcSvc := service.NewMakerCheckerService(db, mcRepo, auditRepo, configSvc, executors, postgres.NewBusinessDateRepository(db), postgres.NewBranchRepository(db))
	permSvc := service.NewPermissionService(postgres.NewPermissionRepository(db), auditRepo, mcSvc)
	executors.Register(service.ActionPermissionChange, permSvc)
	return &permissionTestRig{db: db, ctx: ctx, perm: permSvc, mc: mcSvc, audit: auditRepo}
}

// actorFor membangun pelaku dari pengguna uji nyata: audit_logs.staff_user_id
// berelasi ke staff_users, jadi pelaku sintetis akan ditolak foreign key.
func actorFor(id uuid.UUID, username string, role domain.StaffRole) domain.Actor {
	return domain.Actor{UserID: id, Username: username, Role: role}
}

func (r *permissionTestRig) cleanupPermission(t *testing.T, groupCode string, p domain.Permission) {
	t.Helper()
	t.Cleanup(func() {
		_, _ = r.db.ExecContext(context.Background(), `
			DELETE FROM group_permissions gp USING user_groups g
			WHERE gp.group_id = g.id AND g.code = $1 AND gp.permission = $2`, groupCode, p)
	})
}

// TestIntegrasiUbahIzinTanpaRilisKode mengubah izin grup lewat maker-checker,
// memastikan izin berlaku langsung, teraudit sebelum->sesudah, pembuat tidak dapat
// menyetujui sendiri, dan pencabutan berdampak butuh konfirmasi.
func TestIntegrasiUbahIzinTanpaRilisKode(t *testing.T) {
	db, ctx := newPermissionDB(t)
	rig := newPermissionRig(t, db, ctx)
	// Hanya reports:export yang benar-benar ditambahkan uji ini; ledger:read adalah
	// izin seed yang TIDAK boleh dihapus oleh pembersihan (permintaan pencabutannya
	// sengaja tidak pernah disetujui).
	rig.cleanupPermission(t, "ROLE_TELLER", domain.PermReportsExport)

	_, sessionID := seedTempUser(t, db, ctx, domain.RoleTeller)
	makerID, _ := seedTempUser(t, db, ctx, domain.RoleAdmin)
	checkerID, _ := seedTempUser(t, db, ctx, domain.RoleSuperAdmin)
	maker := actorFor(makerID, "pembuat-uji", domain.RoleAdmin)
	checker := actorFor(checkerID, "pemeriksa-uji", domain.RoleSuperAdmin)

	// 1. Sebelum: TELLER tidak punya reports:export.
	before, _ := identityPermissions(t, db, ctx, sessionID)
	if permissionSet(before)[string(domain.PermReportsExport)] {
		t.Fatal("prasyarat salah: TELLER sudah punya reports:export sebelum uji")
	}

	// 2. Ajukan GRANT (pembuat ADMIN), lalu setujui oleh pemeriksa SUPERADMIN.
	req, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode:  "ROLE_TELLER",
		Permission: domain.PermReportsExport,
		Operation:  domain.PermissionChangeGrant,
	}, maker)
	if err != nil {
		t.Fatalf("mengajukan perubahan izin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM maker_checker_requests WHERE id = $1`, req.ID)
	})

	// 3. Pembuat tidak boleh menyetujui pengajuannya sendiri.
	if _, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode:  "ROLE_TELLER",
		Permission: domain.PermCOAManage,
		Operation:  domain.PermissionChangeGrant,
	}, maker); err != nil {
		t.Fatalf("mengajukan perubahan kedua: %v", err)
	}
	pending, err := rig.mc.ListPending(ctx, maker)
	if err != nil {
		t.Fatalf("membaca antrean: %v", err)
	}
	var selfReq *domain.MakerCheckerRequest
	for i := range pending {
		if pending[i].MakerID == maker.UserID.String() {
			selfReq = &pending[i]
		}
	}
	if selfReq == nil {
		t.Fatal("pengajuan pembuat tidak ada di antrean")
	}
	if err := rig.mc.Approve(ctx, selfReq.ID, maker, ""); err == nil {
		t.Fatal("pembuat berhasil menyetujui pengajuannya sendiri; seharusnya ditolak")
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM maker_checker_requests WHERE id = $1`, selfReq.ID)
	})

	// 4. Setujui pengajuan pertama oleh pemeriksa lain.
	if err := rig.mc.Approve(ctx, req.ID, checker, "uji"); err != nil {
		t.Fatalf("menyetujui perubahan izin: %v", err)
	}

	after, _ := identityPermissions(t, db, ctx, sessionID)
	if !permissionSet(after)[string(domain.PermReportsExport)] {
		t.Fatal("reports:export belum berlaku setelah persetujuan; izin tidak mengikuti database")
	}

	// 5. Audit sebelum->sesudah.
	var changesRaw []byte
	err = db.QueryRowContext(ctx, `
		SELECT changes FROM audit_logs
		WHERE resource_type = 'user_group' AND resource_id = 'ROLE_TELLER' AND action = 'PERMISSION_MAPPING_CHANGE'
		ORDER BY created_at DESC LIMIT 1`).Scan(&changesRaw)
	if err != nil {
		t.Fatalf("membaca audit perubahan izin: %v", err)
	}
	var changes map[string]any
	if err := json.Unmarshal(changesRaw, &changes); err != nil {
		t.Fatalf("audit changes bukan JSON: %v", err)
	}
	if changes["before"] != false || changes["after"] != true {
		t.Fatalf("jejak audit salah: %+v", changes)
	}

	// 6. Pencabutan ledger:read akan mencabut akses TELLER; tanpa konfirmasi ditolak.
	_, err = rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode:  "ROLE_TELLER",
		Permission: domain.PermLedgerRead,
		Operation:  domain.PermissionChangeRevoke,
	}, maker)
	var loss *domain.AccessLossError
	if err == nil {
		t.Fatal("pencabutan berdampak diterima tanpa konfirmasi; seharusnya ditolak")
	}
	if !errors.As(err, &loss) {
		t.Fatalf("err = %v, mau AccessLossError", err)
	}

	// Dengan konfirmasi eksplisit, pengajuan diterima.
	confirmReq, err := rig.perm.RequestChange(ctx, domain.PermissionChangeInput{
		GroupCode:         "ROLE_TELLER",
		Permission:        domain.PermLedgerRead,
		Operation:         domain.PermissionChangeRevoke,
		ConfirmAccessLoss: true,
	}, maker)
	if err != nil {
		t.Fatalf("pencabutan dengan konfirmasi seharusnya diterima: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM maker_checker_requests WHERE id = $1`, confirmReq.ID)
	})
}

// TestIntegrasiMenuMengikutiIzin membuktikan menu dihitung dari izin efektif, bukan
// dari daftar peran di kode: menambah izin ke grup membuka menu tanpa rilis kode.
func TestIntegrasiMenuMengikutiIzin(t *testing.T) {
	db, ctx := newPermissionDB(t)

	// CS tidak berhak loans:read sehingga menu kredit/pembiayaan tertutup,
	// tetapi menu tanpa izin (beranda) terbuka.
	_, sessionID := seedTempUser(t, db, ctx, domain.RoleCS)
	_, menusBefore := identityPermissions(t, db, ctx, sessionID)
	if !containsMenu(menusBefore, "beranda") {
		t.Fatalf("menu beranda seharusnya terbuka untuk semua pengguna: %v", menusBefore)
	}
	if containsMenu(menusBefore, "kredit") {
		t.Fatalf("menu kredit terbuka padahal CS tidak punya loans:read: %v", menusBefore)
	}

	// Beri izin loans:read langsung ke grup CS, lalu menu kredit harus terbuka.
	_, err := db.ExecContext(ctx, `
		INSERT INTO group_permissions (group_id, permission)
		SELECT id, $1 FROM user_groups WHERE code = 'ROLE_CS'
		ON CONFLICT DO NOTHING`, domain.PermLoansRead)
	if err != nil {
		t.Fatalf("memberi izin loans:read: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `
			DELETE FROM group_permissions gp USING user_groups g
			WHERE gp.group_id = g.id AND g.code = 'ROLE_CS' AND gp.permission = $1`, domain.PermLoansRead)
	})

	_, menusAfter := identityPermissions(t, db, ctx, sessionID)
	if !containsMenu(menusAfter, "kredit") {
		t.Fatalf("menu kredit belum terbuka setelah izin diberikan: %v", menusAfter)
	}
}

// TestIntegrasiPeranLimitSeedMenyamaiPeran membuktikan kaitan
// user_groups.approval_limit_role (000079) mereproduksi pemetaan efektif sekarang:
// untuk tiap peran, resolusi grup dari database mengembalikan peran itu sendiri,
// dan batas efektif sesudah kaitan identik dengan sebelum kaitan. Tidak ada
// transaksi yang berubah persetujuannya.
func TestIntegrasiPeranLimitSeedMenyamaiPeran(t *testing.T) {
	db, ctx := newPermissionDB(t)
	permRepo := postgres.NewPermissionRepository(db)
	configSvc := service.NewSystemConfigService(postgres.NewSystemConfigRepository(db))

	roles := []domain.StaffRole{
		domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleSupervisor,
		domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor,
	}
	for _, role := range roles {
		t.Run(string(role), func(t *testing.T) {
			userID, _ := seedTempUser(t, db, ctx, role)

			resolved, err := permRepo.ResolveApprovalLimitRole(ctx, userID, role)
			if err != nil {
				t.Fatalf("ResolveApprovalLimitRole: %v", err)
			}
			if resolved != role {
				t.Fatalf("peran limit %s = %s, ingin %s (seed harus menyamai peran)", role, resolved, role)
			}

			sebelum := service.NewTransactionLimitService(configSvc, nil, nil)
			sesudah := service.NewTransactionLimitService(configSvc, nil, nil, permRepo)
			for _, txType := range service.TransactionLimitTypes() {
				actor := domain.Actor{UserID: userID, Username: string(role), Role: role}
				a, err := sebelum.ForActor(ctx, actor, txType)
				if err != nil {
					t.Fatalf("sebelum %s: %v", txType, err)
				}
				b, err := sesudah.ForActor(ctx, actor, txType)
				if err != nil {
					t.Fatalf("sesudah %s: %v", txType, err)
				}
				if !a.PerTransaction.Equal(b.PerTransaction) ||
					!a.DailyAmount.Equal(b.DailyAmount) ||
					!a.RequiresApprovalAbove.Equal(b.RequiresApprovalAbove) {
					t.Fatalf("%s/%s berubah: sebelum %+v, sesudah %+v", role, txType, a, b)
				}
				if txType == "DEPOSIT" {
					t.Logf("%-11s per_transaksi sebelum=%s sesudah=%s | ambang sebelum=%s sesudah=%s",
						role, a.PerTransaction, b.PerTransaction, a.RequiresApprovalAbove, b.RequiresApprovalAbove)
				}
			}
		})
	}
}

// TestIntegrasiPeranLimitGrupTertinggiMenang membuktikan keanggotaan grup lain
// (aditif) dapat menaikkan jenjang kewenangan: TELLER yang dijadikan anggota grup
// SUPERVISOR memakai baris matriks SUPERVISOR. Peran tertinggi yang menang, sejalan
// dengan semantik izin efektif yang juga aditif.
func TestIntegrasiPeranLimitGrupTertinggiMenang(t *testing.T) {
	db, ctx := newPermissionDB(t)
	permRepo := postgres.NewPermissionRepository(db)

	userID, _ := seedTempUser(t, db, ctx, domain.RoleTeller)
	if _, err := db.ExecContext(ctx, `
		INSERT INTO user_group_members (user_id, group_id)
		SELECT $1, id FROM user_groups WHERE code = 'ROLE_SUPERVISOR'
		ON CONFLICT DO NOTHING`, userID); err != nil {
		t.Fatalf("menambah keanggotaan grup: %v", err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM user_group_members WHERE user_id = $1`, userID)
	})

	resolved, err := permRepo.ResolveApprovalLimitRole(ctx, userID, domain.RoleTeller)
	if err != nil {
		t.Fatalf("ResolveApprovalLimitRole: %v", err)
	}
	if resolved != domain.RoleSupervisor {
		t.Fatalf("peran limit = %s, ingin SUPERVISOR (peran tertinggi dari grup yang diikuti)", resolved)
	}
}
