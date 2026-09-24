package service_test

import (
	"os"
	"path/filepath"
	"testing"
)

// Uji integrasi pengisian COA penjurnalan CKPN (migrasi 000069) terhadap PostgreSQL
// sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// audit_metadata_backfill_integration_test.go.
//
//	TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiMigrasiCOACKPN -v
//
// Uji membuktikan: (1) kedua kunci terisi kode akun yang benar dan akunnya memang ada
// di bagan akun; (2) migrasi idempotent (dijalankan dua kali tidak berubah); (3) nilai
// yang sudah disesuaikan operator TIDAK ditimpa.

const ckpnCOAMigration = "000069_ckpn_coa_config.up.sql"

// ckpnCOASyariahMigration menambahkan kunci per unit syariah dengan nilai KOSONG.
const ckpnCOASyariahMigration = "000071_ckpn_coa_syariah.up.sql"

// jalankanMigrasiCkpnCOA menjalankan berkas migrasi apa adanya.
func jalankanMigrasiCkpnCOA(t *testing.T, e *moneyEnv) {
	t.Helper()
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnCOAMigration)
}

// jalankanMigrasiCkpnCOABerkas menjalankan satu berkas migrasi COA CKPN apa adanya.
func jalankanMigrasiCkpnCOABerkas(t *testing.T, e *moneyEnv, nama string) {
	t.Helper()
	path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", nama)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", nama, err)
	}
	if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
		t.Fatalf("menjalankan migrasi %s: %v", nama, err)
	}
}

// setConfigCKPNCOA memaksa nilai kunci; Insert agar baris selalu ada pada DB uji.
// Nilai lama dipulihkan lewat t.Cleanup saat pertama kunci disentuh.
func setConfigCKPNCOA(t *testing.T, e *moneyEnv, key, value string) {
	t.Helper()
	e.simpanPulihkanConfigKunci(t, key)
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji migrasi COA CKPN')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

func bacaConfigCKPNCOA(t *testing.T, e *moneyEnv, key string) string {
	t.Helper()
	var value string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT value FROM system_config WHERE key = $1`, key).Scan(&value); err != nil {
		t.Fatalf("membaca konfigurasi %s: %v", key, err)
	}
	return value
}

func TestIntegrasiMigrasiCOACKPNIdempotent(t *testing.T) {
	e := newMoneyEnv(t)

	// Tiru keadaan pra-000069: kedua kunci ada tetapi masih kosong.
	setConfigCKPNCOA(t, e, "ckpn.coa.expense", "")
	setConfigCKPNCOA(t, e, "ckpn.coa.reserve", "")

	assertKodeBenar := func(tahap string) {
		t.Helper()
		if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense"); got != "50301" {
			t.Fatalf("%s: ckpn.coa.expense = %q, mau 50301", tahap, got)
		}
		if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.reserve"); got != "10950" {
			t.Fatalf("%s: ckpn.coa.reserve = %q, mau 10950", tahap, got)
		}
	}

	jalankanMigrasiCkpnCOA(t, e)
	assertKodeBenar("setelah jalan pertama")

	// Jalan kedua harus tidak mengubah apa pun (idempotent).
	jalankanMigrasiCkpnCOA(t, e)
	assertKodeBenar("setelah jalan kedua")

	// Kode akun harus benar-benar ada di bagan akun dengan jenis/saldo normal yang
	// tepat, dan punya akun GL internal sehingga penjurnalan tidak gagal.
	var expName, expType, expNorm string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT name, type, normal_balance FROM chart_of_accounts WHERE code = '50301'`).
		Scan(&expName, &expType, &expNorm); err != nil {
		t.Fatalf("akun beban CKPN 50301 tidak ada: %v", err)
	}
	if expType != "EXPENSE" || expNorm != "DEBIT" {
		t.Fatalf("akun 50301 = %s/%s/%s, mau EXPENSE/DEBIT", expName, expType, expNorm)
	}
	var resName, resType, resNorm string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT name, type, normal_balance FROM chart_of_accounts WHERE code = '10950'`).
		Scan(&resName, &resType, &resNorm); err != nil {
		t.Fatalf("akun cadangan CKPN 10950 tidak ada: %v", err)
	}
	if resType != "ASSET" || resNorm != "CREDIT" {
		t.Fatalf("akun 10950 = %s/%s/%s, mau ASSET/CREDIT", resName, resType, resNorm)
	}
	for _, code := range []string{"50301", "10950"} {
		var n int
		if err := e.db.QueryRowContext(e.ctx,
			`SELECT count(*) FROM accounts WHERE account_number = $1 AND account_type = 'INTERNAL_GL'`, code).Scan(&n); err != nil {
			t.Fatalf("menghitung akun GL %s: %v", code, err)
		}
		if n == 0 {
			t.Fatalf("akun GL internal %s tidak ada; penjurnalan CKPN akan gagal", code)
		}
	}

	// Nilai yang sudah disesuaikan operator tidak boleh ditimpa.
	setConfigCKPNCOA(t, e, "ckpn.coa.expense", "59999")
	jalankanMigrasiCkpnCOA(t, e)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense"); got != "59999" {
		t.Fatalf("nilai operator ckpn.coa.expense ditimpa menjadi %q, mau tetap 59999", got)
	}
	// Kunci lain yang masih kosong tetap diisi migrasi.
	setConfigCKPNCOA(t, e, "ckpn.coa.reserve", "")
	jalankanMigrasiCkpnCOA(t, e)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.reserve"); got != "10950" {
		t.Fatalf("ckpn.coa.reserve kosong tidak diisi: %q", got)
	}
}

// Uji migrasi 000071 terhadap PostgreSQL sungguhan: kunci unit syariah di-seed
// KOSONG (artinya mengikuti kunci global), idempotent, dan nilai operator tidak
// ditimpa. Kosongnya nilai seed penting: bank yang belum mengisi kunci syariah
// harus mendapat perilaku yang sama seperti sebelum kunci ini ada.
func TestIntegrasiMigrasiCOACKPNSyariahIdempotent(t *testing.T) {
	e := newMoneyEnv(t)

	// Tiru keadaan sesudah migrasi: kedua kunci ada dan kosong.
	setConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah", "")
	setConfigCKPNCOA(t, e, "ckpn.coa.reserve.syariah", "")

	assertKosong := func(tahap string) {
		t.Helper()
		if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah"); got != "" {
			t.Fatalf("%s: ckpn.coa.expense.syariah = %q, mau kosong (ikut kunci global)", tahap, got)
		}
		if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.reserve.syariah"); got != "" {
			t.Fatalf("%s: ckpn.coa.reserve.syariah = %q, mau kosong (ikut kunci global)", tahap, got)
		}
	}

	jalankanMigrasiCkpnCOABerkas(t, e, ckpnCOASyariahMigration)
	assertKosong("setelah jalan pertama")

	// Jalan kedua tetap kosong, tidak mengubah apa pun.
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnCOASyariahMigration)
	assertKosong("setelah jalan kedua")

	// Nilai yang sudah disesuaikan operator tidak boleh ditimpa.
	setConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah", "15901")
	jalankanMigrasiCkpnCOABerkas(t, e, ckpnCOASyariahMigration)
	if got := bacaConfigCKPNCOA(t, e, "ckpn.coa.expense.syariah"); got != "15901" {
		t.Fatalf("nilai operator ckpn.coa.expense.syariah ditimpa menjadi %q, mau tetap 15901", got)
	}
}
