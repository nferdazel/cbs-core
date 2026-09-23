package service_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// N1: nomor pegawai diambil dari sequence database (migrasi 000089), bukan potongan
// waktu, sehingga beberapa pembuatan berturut-turut menghasilkan nomor yang berbeda.
func TestIntegrasiNomorPegawaiUnikDariUrutan(t *testing.T) {
	e := newMoneyEnv(t)
	svc := service.NewStaffService(
		postgres.NewStaffRepository(e.db),
		postgres.NewBranchRepository(e.db),
		nil,
	)
	actor := domain.Actor{UserID: e.actor.UserID, Username: "superadmin.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	seen := map[string]bool{}
	for i := 0; i < 5; i++ {
		suffix := uuid.NewString()[:8]
		user, err := svc.CreateStaff(e.ctx, domain.CreateStaffInput{
			Username:   "staf.urut." + suffix,
			FullName:   "Staf Urut",
			Email:      "staf.urut." + suffix + "@uji.local",
			Password:   "Rahasia#2026",
			Role:       domain.RoleTeller,
			BranchCode: "001",
		}, actor)
		if err != nil {
			t.Fatalf("CreateStaff ke-%d: %v", i+1, err)
		}
		if user.EmployeeID == "" {
			t.Fatalf("nomor pegawai kosong pada pembuatan ke-%d", i+1)
		}
		if seen[user.EmployeeID] {
			t.Fatalf("nomor pegawai berulang: %s", user.EmployeeID)
		}
		seen[user.EmployeeID] = true
	}
}

// N4: query peringatan start menemukan staf aktif yang branch_code-nya tidak terdaftar,
// supaya operator dapat memperbaiki data lama tanpa memperluas cakupan diam-diam.
func TestIntegrasiDaftarKetidakcocokanCabangStaf(t *testing.T) {
	e := newMoneyEnv(t)
	id := uuid.New()
	username := "adminho." + uuid.NewString()[:8]
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO staff_users (id, employee_id, username, full_name, email, password_hash, role, branch_code, is_active)
		VALUES ($1, $2, $3, 'Admin HO', $4, 'x', 'ADMIN', 'HO', TRUE)`,
		id, "EMP-UJI-"+id.String()[:8], username, username+"@uji.local"); err != nil {
		t.Fatalf("menyisipkan staf HO: %v", err)
	}

	repo := postgres.NewStaffRepository(e.db)
	list, err := repo.ListBranchScopeMismatches(e.ctx)
	if err != nil {
		t.Fatalf("membaca ketidakcocokan cabang: %v", err)
	}
	found := false
	for _, m := range list {
		if m.Username == username {
			found = true
			if m.BranchCode != "HO" || m.Role != domain.RoleAdmin {
				t.Fatalf("baris ketidakcocokan salah: %+v", m)
			}
		}
	}
	if !found {
		t.Fatalf("staf %q bercabang HO tidak terdeteksi", username)
	}
}
