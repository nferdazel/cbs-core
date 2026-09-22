package service_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Uji integrasi rename kunci konfigurasi W3 (migrasi 000078) terhadap PostgreSQL
// sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// ckpn_coa_config_migration_integration_test.go.
//
//	TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiMigrasiRenameKunci -v
//
// Uji membuktikan: (1) migrasi memuat tepat seluruh pasangan rename yang diharapkan;
// (2) nama kunci berpindah tanpa mengubah NILAI maupun DESKRIPSI; (3) kunci yang tidak
// pernah ada tidak memunculkan baris sampah; (4) migrasi idempotent (dijalankan dua
// kali tidak menambah/mengubah baris).
//
// Daftar nama BARU ditulis di sini; nama LAMA dibaca dari berkas migrasi agar uji ini
// (a) tidak mengulang literal kunci lama di kode dan (b) gagal bila migrasi kehilangan
// satu pasangan rename.

const configRenameMigration = "000078_rename_config_keys.up.sql"

// configRenameNewKeys adalah nama baru yang wajib dihasilkan migrasi 000078.
var configRenameNewKeys = []string{
	"ckpn.pd_frac.gol_1",
	"ckpn.pd_frac.gol_2",
	"ckpn.pd_frac.gol_3",
	"ckpn.pd_frac.gol_4",
	"ckpn.pd_frac.gol_5",
	"ppap.rate_frac.gol_1",
	"ppap.rate_frac.gol_2",
	"ppap.rate_frac.gol_3",
	"ppap.rate_frac.gol_4",
	"ppap.rate_frac.gol_5",
	"ckpn.lgd_frac",
	"tax.deposit.rate_pct",
	"tax.savings.rate_pct",
	"loan.penalty.cap_pct",
	"savings.interest.rate_annual_pct",
	"loan.restructure.loss.discount_rate_annual_pct",
	"deposit.mudharabah.yield_annual_pct",
	"collateral.haircut.deposit_pct",
	"collateral.haircut.kendaraan_pct",
	"collateral.haircut.lainnya_pct",
	"collateral.haircut.mesin_peralatan_pct",
	"collateral.haircut.tanah_bangunan_pct",
}

// configRenameAbsentNew sengaja dihapus saat menyiapkan keadaan pra-rename untuk
// membuktikan kunci yang tidak pernah ada tidak menghasilkan baris sampah.
const configRenameAbsentNew = "collateral.haircut.lainnya_pct"

var migrasiRenamePattern = regexp.MustCompile(
	`(?i)UPDATE\s+system_config\s+SET\s+key\s*=\s*'([^']+)'\s+WHERE\s+key\s*=\s*'([^']+)'`)

// pasanganRename membaca berkas migrasi 000078 dan mengembalikan pasangan lama->baru.
func pasanganRename(t *testing.T) [][2]string {
	t.Helper()
	path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", configRenameMigration)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", configRenameMigration, err)
	}
	var pairs [][2]string
	for _, m := range migrasiRenamePattern.FindAllStringSubmatch(string(data), -1) {
		pairs = append(pairs, [2]string{m[2], m[1]})
	}
	return pairs
}

func jalankanMigrasiRenameKunci(t *testing.T, e *moneyEnv) {
	t.Helper()
	path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", configRenameMigration)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", configRenameMigration, err)
	}
	if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
		t.Fatalf("menjalankan migrasi %s: %v", configRenameMigration, err)
	}
}

type configMeta struct{ value, description string }

func bacaConfigMeta(t *testing.T, e *moneyEnv, key string) (configMeta, bool) {
	t.Helper()
	var m configMeta
	err := e.db.QueryRowContext(e.ctx,
		`SELECT value, COALESCE(description, '') FROM system_config WHERE key = $1`, key).
		Scan(&m.value, &m.description)
	if err != nil {
		return configMeta{}, false
	}
	return m, true
}

func hitungConfig(t *testing.T, e *moneyEnv) int {
	t.Helper()
	var n int
	if err := e.db.QueryRowContext(e.ctx, `SELECT count(*) FROM system_config`).Scan(&n); err != nil {
		t.Fatalf("menghitung system_config: %v", err)
	}
	return n
}

func TestIntegrasiMigrasiRenameKunciKonfigurasi(t *testing.T) {
	e := newMoneyEnv(t)

	// 1. Migrasi harus memuat tepat pasangan yang diharapkan: jumlah sama, nama baru
	// cocok, dan tidak ada nama baru yang bertabrakan dengan nama lama.
	pairs := pasanganRename(t)
	if len(pairs) != len(configRenameNewKeys) {
		t.Fatalf("migrasi memuat %d pasangan rename, mau %d", len(pairs), len(configRenameNewKeys))
	}
	wantNew := map[string]bool{}
	for _, k := range configRenameNewKeys {
		wantNew[k] = true
	}
	oldSet := map[string]bool{}
	for _, p := range pairs {
		if !wantNew[p[1]] {
			t.Fatalf("migrasi menghasilkan nama baru %q yang tidak diharapkan", p[1])
		}
		oldSet[p[0]] = true
	}
	for _, p := range pairs {
		if oldSet[p[1]] {
			t.Fatalf("nama baru %q juga dipakai sebagai nama lama", p[1])
		}
	}

	// Nama lama dari kunci yang sengaja dihapus dibaca dari migrasi, bukan ditulis
	// ulang, agar kode uji tidak menyimpan literal kunci lama.
	oldAbsent := ""
	for _, p := range pairs {
		if p[1] == configRenameAbsentNew {
			oldAbsent = p[0]
		}
	}
	if oldAbsent == "" {
		t.Fatalf("migrasi tidak memuat pasangan rename untuk %s", configRenameAbsentNew)
	}

	// Cleanup didaftarkan SEBELUM mutasi apa pun agar database uji tetap konsisten
	// walau uji gagal di tengah: kunci yang sengaja dihapus dihadirkan kembali dengan
	// nilai seed, lalu migrasi mengembalikan nama lama yang sempat dikembalikan.
	t.Cleanup(func() {
		if _, err := e.db.ExecContext(e.ctx, `
			INSERT INTO system_config (key, value, description)
			SELECT 'collateral.haircut.lainnya_pct', '100',
			       'Bagian nilai taksasi agunan jenis lain yang tidak dihitung sebagai pengurang PPAP (100 = tanpa pengurangan).'
			WHERE NOT EXISTS (SELECT 1 FROM system_config WHERE key = $1)
			ON CONFLICT (key) DO NOTHING`, oldAbsent); err != nil {
			t.Fatalf("memulihkan kunci %s: %v", configRenameAbsentNew, err)
		}
		e.configSvc.Invalidate(configRenameAbsentNew)
		jalankanMigrasiRenameKunci(t, e)
	})

	// 2. Siapkan keadaan pra-000078: kembalikan tiap kunci baru ke nama lama di database
	// uji, catat nilai+deskripsinya, lalu hapus satu kunci supaya ia benar-benar tidak
	// pernah ada.
	sebelum := map[string]configMeta{}
	for _, p := range pairs {
		oldKey, newKey := p[0], p[1]
		meta, ok := bacaConfigMeta(t, e, newKey)
		if !ok {
			t.Fatalf("kunci %s tidak ada sebelum rename", newKey)
		}
		if _, err := e.db.ExecContext(e.ctx,
			`UPDATE system_config SET key = $1 WHERE key = $2`, oldKey, newKey); err != nil {
			t.Fatalf("mengembalikan %s ke %s: %v", newKey, oldKey, err)
		}
		if newKey == configRenameAbsentNew {
			if _, err := e.db.ExecContext(e.ctx, `DELETE FROM system_config WHERE key = $1`, oldKey); err != nil {
				t.Fatalf("menghapus %s: %v", oldKey, err)
			}
			continue
		}
		sebelum[oldKey] = meta
	}

	countAwal := hitungConfig(t, e)

	jalankanMigrasiRenameKunci(t, e)

	// 3. Setiap kunci hasil rename harus membawa nilai dan deskripsi yang identik.
	for _, p := range pairs {
		oldKey, newKey := p[0], p[1]
		if newKey == configRenameAbsentNew {
			if _, ok := bacaConfigMeta(t, e, newKey); ok {
				t.Fatalf("kunci %s yang tidak pernah ada malah dibuat migrasi", newKey)
			}
			continue
		}
		meta, ok := bacaConfigMeta(t, e, newKey)
		if !ok {
			t.Fatalf("kunci hasil rename %s tidak ada", newKey)
		}
		if meta.value != sebelum[oldKey].value {
			t.Fatalf("nilai %s berubah: %q -> %q", newKey, sebelum[oldKey].value, meta.value)
		}
		if meta.description != sebelum[oldKey].description {
			t.Fatalf("deskripsi %s berubah/terhapus: %q -> %q", newKey, sebelum[oldKey].description, meta.description)
		}
		if _, ok := bacaConfigMeta(t, e, oldKey); ok {
			t.Fatalf("nama lama %s masih ada setelah rename", oldKey)
		}
	}

	if countAkhir := hitungConfig(t, e); countAkhir != countAwal {
		t.Fatalf("jumlah baris system_config berubah %d -> %d", countAwal, countAkhir)
	}

	// 4. Jalan kedua harus tidak mengubah apa pun (idempotent).
	jalankanMigrasiRenameKunci(t, e)
	if countKedua := hitungConfig(t, e); countKedua != countAwal {
		t.Fatalf("migrasi rename tidak idempotent: %d -> %d baris", countAwal, countKedua)
	}
	for oldKey, meta := range sebelum {
		newKey := ""
		for _, p := range pairs {
			if p[0] == oldKey {
				newKey = p[1]
			}
		}
		got, ok := bacaConfigMeta(t, e, newKey)
		if !ok || got != meta {
			t.Fatalf("jalan kedua mengubah %s: %+v", newKey, got)
		}
	}
}
