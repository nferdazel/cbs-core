package service_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Matriks izin final (keputusan pemilik sistem):
//   - loans:write_off — SUPERVISOR ke atas (hapus buku melepas aset dari neraca).
//     SUPERADMIN/ADMIN tetap punya agar tidak ada yang terkunci.
//   - loans:recover — TELLER/AO boleh (pencatatan uang masuk, bukan penghapusan);
//     di atas ambang nominal wajib persetujuan atasan (diuji terpisah).
//
// Uji ini mengunci keputusan itu; uji invarian seed (TestPermissionSeedInvariant)
// memastikan kode dan migrasi 000079 tetap sinkron.
func TestMatriksIzinFinalHapusBukuDanRecovery(t *testing.T) {
	writeOffBoleh := []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleSupervisor}
	writeOffTidak := []domain.StaffRole{domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor}
	for _, role := range writeOffBoleh {
		if !role.HasPermission(domain.PermLoansWriteOff) {
			t.Errorf("%s harus punya %s (SUPERVISOR ke atas)", role, domain.PermLoansWriteOff)
		}
	}
	for _, role := range writeOffTidak {
		if role.HasPermission(domain.PermLoansWriteOff) {
			t.Errorf("%s tidak boleh punya %s", role, domain.PermLoansWriteOff)
		}
	}

	recoverBoleh := []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleTeller, domain.RoleAO}
	recoverTidak := []domain.StaffRole{domain.RoleCS, domain.RoleAuditor}
	for _, role := range recoverBoleh {
		if !role.HasPermission(domain.PermLoansRecover) {
			t.Errorf("%s harus punya %s", role, domain.PermLoansRecover)
		}
	}
	for _, role := range recoverTidak {
		if role.HasPermission(domain.PermLoansRecover) {
			t.Errorf("%s tidak boleh punya %s", role, domain.PermLoansRecover)
		}
	}
}

// Ambang recovery wajib kunci konfigurasi (bukan angka di kode) dengan nilai awal
// 10.000.000. Migrasi 000085 yang mengisinya, dan hanya mengubah bila masih bawaan 0.
func TestAmbangRecoveryAwalAdalahKunciKonfigurasi(t *testing.T) {
	const migrasi = "000085_write_off_amount_and_recovery_threshold.up.sql"
	data, err := os.ReadFile(filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", migrasi))
	if err != nil {
		t.Fatalf("membaca %s: %v", migrasi, err)
	}
	sql := string(data)
	if !strings.Contains(sql, "maker_checker.loan_recovery.threshold") {
		t.Fatalf("%s tidak menyetel kunci maker_checker.loan_recovery.threshold", migrasi)
	}
	if !strings.Contains(sql, "'10000000'") {
		t.Fatalf("%s tidak memuat nilai awal 10000000 untuk ambang recovery", migrasi)
	}
}
