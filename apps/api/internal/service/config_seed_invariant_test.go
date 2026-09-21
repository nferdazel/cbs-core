package service_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/service"
)

// Invarian seed konfigurasi.
//
// Masalah yang dicegah: kunci dibaca kode lewat SystemConfigService/Repository tetapi
// tidak pernah di-seed, sehingga penjaga batas atau tarif jatuh ke bawaan (atau nol)
// tanpa terlihat. Uji ini mengumpulkan kunci yang dibaca kode secara statis (literali
// string di sumber produksi) ditambah kunci yang dibangun saat runtime (keluarga
// limit.*, haircut, PD/LGD, tarif PPAP, ambang maker-checker), lalu membandingkannya
// dengan kunci yang di-seed berkas migrasi. Kunci yang dibaca tetapi tidak di-seed
// MENGGAGALKAN uji; kunci yang di-seed tetapi tidak dibaca juga menggagalkan uji.
//
// Menambah kunci baru di kode: seeder harus menambahkannya ke migrasi, atau bila kunci
// itu memang dibuat runtime (bukan kebijakan bank), tambahkan ke
// configKeyRuntimeExceptions dengan alasan. Jangan melonggarkan asersi untuk kunci
// kebijakan. Menambah keluarga kunci dinamis baru: tambahkan enumerasinya di
// runtimeGeneratedConfigKeys agar ikut diperiksa.

// configKeyShape cocok dengan pola kunci konfigurasi: minimal dua segmen huruf kecil
// dipisah titik, mis. limit.teller.deposit.per_transaction.
var configKeyShape = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z0-9_]+)+$`)

// sqlEmbeddedKey cocok dengan kunci yang ditanam di dalam query SQL, mis.
// "SELECT value FROM system_config WHERE key = 'system.business_date'".
var sqlEmbeddedKey = regexp.MustCompile(`'([a-z][a-z0-9_]*(?:\.[a-z0-9_]+)+)'`)

// configKeyRuntimeExceptions adalah kunci yang sengaja TIDAK di-seed karena dibuat
// saat runtime oleh aplikasi, bukan kebijakan bank yang dapat ditetapkan lebih dulu.
var configKeyRuntimeExceptions = map[string]string{
	"system.business_date":        "tanggal bisnis dibuat/diubah oleh tutup hari, bukan nilai seed",
	"system.business_date_status": "status tanggal bisnis dibuat/diubah oleh tutup hari",
	"ppap.last_run_business_date": "penanda run PPAP ditulis batch saat PPAP berjalan (migrasi 000052 milik perubahan lain)",
}

// configKeySeedExceptions sudah tidak diperlukan. Satu-satunya pengecualian dulu
// adalah auth.password_expiry_days, yang di-seed tetapi belum dibaca kode. Setelah
// penegakan kedaluwarsa diimplementasikan (auth_service.go membacanya), kunci itu
// benar-benar dipakai sehingga pengecualian seed dihapus. Bila kelak ada kunci seed
// yang sengaja dipertahankan tanpa pembaca, kembalikan peta pengecualian di sini
// beserta alasannya.

func configSeedRepoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("menentukan akar repo: %v", err)
	}
	return root
}

// TestConfigSeedInvariant memastikan setiap kunci konfigurasi yang dibaca kode ada di
// seed, dan tidak ada kunci seed yang basi tanpa pengecualian beralasan.
func TestConfigSeedInvariant(t *testing.T) {
	root := configSeedRepoRoot(t)

	read := collectCodeConfigKeys(t, filepath.Join(root, "apps", "api"))
	for _, key := range runtimeGeneratedConfigKeys() {
		if _, exists := read[key]; !exists {
			read[key] = "dibangun runtime"
		}
	}
	seeded := parseSystemConfigSeed(t, filepath.Join(root, "packages", "db-migrations"))

	var missing []string
	for key, origin := range read {
		if _, ok := seeded[key]; ok {
			continue
		}
		if _, excused := configKeyRuntimeExceptions[key]; excused {
			continue
		}
		missing = append(missing, fmt.Sprintf("%s (%s)", key, origin))
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("kunci dibaca kode tetapi tidak di-seed (%d):\n  %s\nTambahkan ke packages/db-migrations agar nilainya eksplisit dan terlihat bank; "+
			"jangan tambahkan ke pengecualian kecuali kunci itu memang dibuat runtime.",
			len(missing), strings.Join(missing, "\n  "))
	}

	var stale []string
	for key := range seeded {
		if _, ok := read[key]; ok {
			continue
		}
		stale = append(stale, key)
	}
	sort.Strings(stale)
	if len(stale) > 0 {
		t.Fatalf("kunci di-seed tetapi tidak dibaca kode (%d):\n  %s\nHapus dari seed lewat migrasi, atau tambahkan pembacanya; jangan biarkan kunci basi.",
			len(stale), strings.Join(stale, "\n  "))
	}
}

// runtimeGeneratedConfigKeys mengembalikan kunci yang dibangun kode saat runtime
// sehingga tidak terlihat sebagai literali utuh oleh pemindai statis. Enumerasinya
// harus mengikuti kode produksi; bila keluarga kunci baru muncul, tambahkan di sini.
func runtimeGeneratedConfigKeys() []string {
	var keys []string

	// limitKey() di limit_service.go: limit.<peran>.<jenis>.<sufiks>.
	for _, role := range service.TransactionLimitRoles() {
		for _, txType := range service.TransactionLimitTypes() {
			for _, suffix := range []string{"per_transaction", "daily", "approval_above"} {
				keys = append(keys, fmt.Sprintf("limit.%s.%s.%s",
					strings.ToLower(string(role)), strings.ToLower(txType), suffix))
			}
		}
	}

	// HaircutConfigKey(): collateral.haircut.<jenis>; jenis tak dikenal jatuh ke "lainnya".
	for _, kind := range []string{"deposit", "kendaraan", "lainnya", "mesin_peralatan", "tanah_bangunan"} {
		keys = append(keys, "collateral.haircut."+kind)
	}

	// ckpnPDKey() dan collectibilityRates(): lima golongan kolektibilitas.
	for i := 1; i <= 5; i++ {
		keys = append(keys, fmt.Sprintf("ckpn.pd.%d", i))
		keys = append(keys, fmt.Sprintf("ppap.rate.%d", i))
	}

	// maker_checkerService.Threshold(): maker_checker.<aksi>.threshold. Daftar aksi
	// mengikuti konstanta Action* di ledger_service.go dan loan_service.go.
	for _, action := range []string{
		"deposit", "withdrawal", "transfer", "reverse_transaction",
		"loan_write_off", "loan_recovery", "loan_correction",
	} {
		keys = append(keys, "maker_checker."+action+".threshold")
	}

	return keys
}

// collectCodeConfigKeys memindai seluruh berkas Go produksi (bukan _test.go) di bawah
// moduleDir dan mengumpulkan literali string yang berbentuk kunci konfigurasi, termasuk
// kunci yang ditanam di dalam string SQL. Nilai map adalah lokasi asal untuk pesan galat.
func collectCodeConfigKeys(t *testing.T, moduleDir string) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	found := map[string]string{}

	record := func(key, origin string) {
		if !configKeyShape.MatchString(key) || strings.HasSuffix(key, ".branch_id") {
			return
		}
		if _, exists := found[key]; !exists {
			found[key] = origin
		}
	}

	err := filepath.WalkDir(moduleDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "bin", "vendor":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("mengurai %s: %v", path, perr)
		}
		rel, _ := filepath.Rel(moduleDir, path)
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				return true
			}
			origin := fmt.Sprintf("%s:%d", rel, fset.Position(lit.Pos()).Line)
			record(value, origin)
			for _, m := range sqlEmbeddedKey.FindAllStringSubmatch(value, -1) {
				record(m[1], origin)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("menelusuri kode di %s: %v", moduleDir, err)
	}
	return found
}

var (
	insertSystemConfig = regexp.MustCompile(`(?i)insert\s+into\s+system_config`)
	deleteSystemConfig = regexp.MustCompile(`(?i)delete\s+from\s+system_config\s+where\s+key\s*=\s*'([^']+)'`)
	deleteSystemLike   = regexp.MustCompile(`(?i)delete\s+from\s+system_config\s+where\s+key\s+like\s*'([^']+)'`)
)

// parseSystemConfigSeed membaca seluruh berkas migrasi *.up.sql dan mengembalikan peta
// kunci -> nilai seed. INSERT diproses berurutan (kemunculan pertama menang), lalu
// DELETE diterapkan agar kunci yang sengaja dibuang migrasi tidak dianggap masih basi.
func parseSystemConfigSeed(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatalf("mendaftar migrasi di %s: %v", dir, err)
	}
	sort.Strings(files)

	out := map[string]string{}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("membaca %s: %v", path, err)
		}
		s := string(data)
		for _, loc := range insertSystemConfig.FindAllStringIndex(s, -1) {
			if inSQLLineComment(s, loc[0]) {
				continue
			}
			for _, pair := range parseSQLValuesTuples(s, loc[1]) {
				if _, exists := out[pair[0]]; !exists {
					out[pair[0]] = pair[1]
				}
			}
		}
		for _, m := range deleteSystemConfig.FindAllStringSubmatch(s, -1) {
			delete(out, m[1])
		}
		for _, m := range deleteSystemLike.FindAllStringSubmatch(s, -1) {
			for key := range out {
				if sqlLikeMatch(m[1], key) {
					delete(out, key)
				}
			}
		}
	}
	return out
}

// parseSQLValuesTuples mengambil tuple VALUES ('kunci', 'nilai', ...) dari satu
// pernyataan INSERT. Semicolon di dalam deskripsi tidak memotong pernyataan karena
// pemindaian melacak tanda kutip; tanda kutip ganda SQL (” ) diperlakukan sebagai
// escape.
func parseSQLValuesTuples(s string, start int) [][2]string {
	var tuples [][2]string
	var elems []string
	depth := 0

	for i := start; i < len(s); {
		c := s[i]
		switch {
		case c == '\'':
			j := i + 1
			var b strings.Builder
			for j < len(s) {
				if s[j] == '\'' {
					if j+1 < len(s) && s[j+1] == '\'' {
						b.WriteByte('\'')
						j += 2
						continue
					}
					break
				}
				b.WriteByte(s[j])
				j++
			}
			if depth == 1 {
				elems = append(elems, b.String())
			}
			i = j + 1
			continue
		case c == '(':
			depth++
			if depth == 1 {
				elems = nil
			}
		case c == ')':
			if depth == 1 {
				if len(elems) >= 2 {
					tuples = append(tuples, [2]string{elems[0], elems[1]})
				}
				elems = nil
			}
			if depth > 0 {
				depth--
			}
		case c == ';':
			if depth <= 0 {
				return tuples
			}
		}
		i++
	}
	return tuples
}

// inSQLLineComment melaporkan apakah offset berada di dalam komentar baris (-- ...).
func inSQLLineComment(s string, offset int) bool {
	lineStart := strings.LastIndexByte(s[:offset], '\n')
	if lineStart < 0 {
		lineStart = 0
	}
	return strings.Contains(s[lineStart:offset], "--")
}

// sqlLikeMatch mencocokkan pola SQL LIKE sederhana: % sebagai wildcard apa pun dan _
// sebagai satu karakter. Cukup untuk pola migrasi seperti role_limit.%.
func sqlLikeMatch(pattern, key string) bool {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '%':
			b.WriteString(".*")
		case '_':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String()).MatchString(key)
}
