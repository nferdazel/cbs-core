package service_test

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
)

// Invarian batas-transaksi vs izin.
//
// Masalah yang dicegah: peran punya kunci batas `limit.<peran>.<jenis>.*` di seed
// (artinya bank menetapkan batas untuk peran itu pada jenis transaksi itu) TETAPI
// peran tersebut tidak punya izin melaksanakan transaksi itu. Akibatnya konfigurasi
// batas mati: AO berbatas 250 juta untuk deposit/withdrawal/transfer, tetapi setoran
// AO ditolak 403 karena tidak punya transactions:deposit. Batas yang tidak dapat
// dipakai adalah kontrol operasional palsu.
//
// Uji ini murni unit (tanpa database): ia membaca peta izin produksi lewat
// domain.RolePermissions/HasPermission dan membaca kunci yang benar-benar di-seed
// dari berkas migrasi (parser yang sama dengan TestConfigSeedInvariant), bukan daftar
// yang ditulis ulang manual. Peran yang punya batas tanpa izin MENGGAGALKAN uji,
// kecuali tercatat di limitPermissionExceptions dengan alasan eksplisit.

// limitPermissionExceptions mencatat peran yang sengaja punya kunci batas di seed
// tetapi TIDAK berizin melakukan transaksi, beserta alasannya. Hanya peran pengawas
// yang batasnya semata rujukan yang boleh ada di sini. Setiap pengecualian diperiksa
// tetap basi: bila peran itu tidak lagi punya kunci batas, uji gagal agar daftar tidak
// menumpuk tanpa guna.
var limitPermissionExceptions = map[domain.StaffRole]string{
	domain.RoleSupervisor: "peran persetujuan: tidak menerbitkan transaksi kas sendiri (hanya menyetujui/membatalkan), sehingga batasnya hanya nilai rujukan. Bila kelak SUPERVISOR diizinkan bertransaksi, hapus pengecualian ini.",
	domain.RoleAuditor:    "peran pengawas independen: hanya baca untuk audit dan tidak boleh menerbitkan transaksi, sehingga batasnya semata rujukan pengawasan.",
}

// requiredTransactionPermission memetakan jenis transaksi batas (segmen kedua kunci,
// huruf kecil) ke izin yang wajib dipegang pelaksananya. Jenis di luar tiga ini
// (mis. future) tidak diperiksa; keluarga kunci tetap dijaga TestConfigSeedInvariant.
func requiredTransactionPermission(txType string) (domain.Permission, bool) {
	switch strings.ToLower(strings.TrimSpace(txType)) {
	case "deposit":
		return domain.PermTransactionsDeposit, true
	case "withdrawal":
		return domain.PermTransactionsWithdraw, true
	case "transfer":
		return domain.PermTransactionsTransfer, true
	default:
		return "", false
	}
}

// TestLimitSeedRequiresTransactionPermission memastikan setiap peran yang batasnya
// di-seed untuk deposit/withdrawal/transfer benar-benar punya izin jenis transaksi itu.
func TestLimitSeedRequiresTransactionPermission(t *testing.T) {
	root := configSeedRepoRoot(t)
	seeded := parseSystemConfigSeed(t, filepath.Join(root, "packages", "db-migrations"))

	roleByName := map[string]domain.StaffRole{}
	for _, role := range service.TransactionLimitRoles() {
		roleByName[strings.ToLower(string(role))] = role
	}

	type lockedCombination struct {
		role   domain.StaffRole
		txType string
		key    string
	}
	combinations := map[string]lockedCombination{}
	for key := range seeded {
		parts := strings.Split(key, ".")
		if len(parts) != 4 || parts[0] != "limit" {
			continue
		}
		role, ok := roleByName[parts[1]]
		if !ok {
			// Peran yang tidak dibaca penjaga batas ditangkap TestConfigSeedInvariant.
			continue
		}
		id := string(role) + "/" + parts[2]
		combinations[id] = lockedCombination{role: role, txType: parts[2], key: key}
	}
	if len(combinations) == 0 {
		t.Fatal("tidak ada kunci limit.<peran>.<jenis>.* di seed; parser atau migrasi rusak")
	}
	if _, ok := combinations["AO/deposit"]; !ok {
		t.Fatal("seed limit.ao.deposit.* tidak terbaca; uji invarian ini tidak akan menangkap temuan AO")
	}

	var missing []string
	usedExceptions := map[domain.StaffRole]bool{}
	for _, c := range combinations {
		perm, required := requiredTransactionPermission(c.txType)
		if !required {
			continue
		}
		if reason, excused := limitPermissionExceptions[c.role]; excused {
			if strings.TrimSpace(reason) == "" {
				t.Errorf("pengecualian %s tanpa alasan", c.role)
			}
			usedExceptions[c.role] = true
			continue
		}
		if !c.role.HasPermission(perm) {
			missing = append(missing, fmt.Sprintf("%s punya kunci %s tetapi tidak punya izin %s", c.role, c.key, perm))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("batas di-seed tanpa izin pelaksana (%d):\n  %s\n"+
			"Beri peran itu izin transaksi yang selaras dengan batasnya, atau hapus kunci batasnya; "+
			"jangan menambah pengecualian kecuali peran itu memang pengawas yang batasnya hanya rujukan.",
			len(missing), strings.Join(missing, "\n  "))
	}

	var stale []string
	for role := range limitPermissionExceptions {
		if !usedExceptions[role] {
			stale = append(stale, string(role))
		}
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("pengecualian tanpa kunci batas di seed (%d): %s\n"+
			"Hapus pengecualian yang tidak lagi relevan.", len(stale), strings.Join(stale, ", "))
	}
}

// TestSystemLimitsReadPermissionSet mengunci siapa yang boleh membaca batas transaksi
// lewat izin system:config:read. Peran pengawas (Superadmin/Admin/Supervisor/Auditor)
// boleh; pelaksana operasional (Teller/CS/AO) tidak berkepentingan dan tetap ditolak.
// system:config penuh (yang juga membuka penulisan konfigurasi dan EOD/EOM/EOY) tidak
// diuji di sini karena rutenya tidak memakainya lagi.
func TestSystemLimitsReadPermissionSet(t *testing.T) {
	allowed := []domain.StaffRole{
		domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleSupervisor, domain.RoleAuditor,
	}
	denied := []domain.StaffRole{
		domain.RoleTeller, domain.RoleCS, domain.RoleAO,
	}

	for _, role := range allowed {
		if !role.HasPermission(domain.PermSystemConfigRead) {
			t.Errorf("%s tidak punya %s; pengawas ini seharusnya boleh melihat batas", role, domain.PermSystemConfigRead)
		}
	}
	for _, role := range denied {
		if role.HasPermission(domain.PermSystemConfigRead) {
			t.Errorf("%s punya %s; peran operasional tidak berkepentingan melihat batas semua peran", role, domain.PermSystemConfigRead)
		}
	}
}
