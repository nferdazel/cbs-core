package postgres

import (
	"regexp"
	"strconv"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

var placeholderRe = regexp.MustCompile(`\$(\d+)`)

// maxPlaceholder mengembalikan nomor placeholder $n terbesar pada klausa (0 bila tak ada).
func maxPlaceholder(clause string) int {
	max := 0
	for _, m := range placeholderRe.FindAllStringSubmatch(clause, -1) {
		if n, err := strconv.Atoi(m[1]); err == nil && n > max {
			max = n
		}
	}
	return max
}

// Guard anti-injeksi untuk helper klausa scope (P1-002).
//
// Usulan audit: pastikan `branchReadClause`/`bookReadClause` hanya menghasilkan klausa
// yang memakai placeholder `$n` + ekspresi kolom, bukan menyisipkan NILAI. Karena kolom
// memang disisipkan (literal dari pemanggil, bukan input pengguna), yang benar-benar
// dapat diuji adalah dua invarian berikut:
//
//  1. Setiap nilai yang diserahkan helper masuk sebagai PARAMETER (args), bukan literal
//     di dalam teks klausa. Diuji dengan kode "umpan" yang mencolok: bila klausa memuat
//     kode itu mentah-mentah, berarti nilai bocor ke SQL (kandidat injeksi).
//  2. Nomor placeholder tidak menggantung: $n terbanyak wajib sama dengan jumlah args,
//     sehingga penggabungan dengan filter lain tidak menghasilkan parameter hilang.
//
// Uji ini gagal bila helper diubah untuk menyisipkan nilai langsung ke string query.

func TestBranchReadClauseTidakMenyisipkanNilai(t *testing.T) {
	// Kode umpan sengaja tidak mungkin muncul alami di ekspresi kolom.
	const umpan = "ZZ-INJEKSI-9"

	kasus := []struct {
		nama  string
		aktor domain.Actor
	}{
		{"jalur lama (kode cabang aktor)", domain.Actor{Role: domain.RoleSupervisor, BranchCode: umpan}},
		{"cakupan hierarki", domain.Actor{
			Role: domain.RoleSupervisor, BranchCode: "810",
			BranchScope: domain.NewBranchScope([]string{umpan, "811"}),
		}},
	}

	for _, tc := range kasus {
		t.Run(tc.nama, func(t *testing.T) {
			clause, args := branchReadClause("l.branch_id", tc.aktor)
			if clause == "" {
				t.Fatal("aktor terbatas cabang harus menghasilkan klausa, bukan kosong")
			}
			if containsValue(clause, umpan) {
				t.Fatalf("nilai kode cabang bocor ke teks SQL (kandidat injeksi): %q", clause)
			}
			assertPlaceholderSesuaiStartArg(t, clause, args, 1)
		})
	}
}

func TestBranchCodeReadClauseTidakMenyisipkanNilai(t *testing.T) {
	const umpan = "ZZ-INJEKSI-9"
	clause, args := branchCodeReadClause("branch_code", domain.Actor{
		Role: domain.RoleSupervisor, BranchCode: umpan,
	}, 1)
	if clause == "" {
		t.Fatal("aktor terbatas cabang harus menghasilkan klausa")
	}
	if containsValue(clause, umpan) {
		t.Fatalf("nilai kode cabang bocor ke teks SQL: %q", clause)
	}
	assertPlaceholderSesuaiStartArg(t, clause, args, 1)
}

func TestBookReadClauseTidakMenyisipkanNilai(t *testing.T) {
	// Buku hanya punya dua nilai sah; yang diuji di sini adalah nilai selalu lewat
	// parameter (bukan literal), dan placeholder tidak menggantung saat digabung.
	aktor := domain.Actor{Role: domain.RoleSupervisor, BranchCode: "001", Book: domain.BookSyariah}
	clause, args := bookReadClause("p.book", aktor, 3)
	if clause == "" {
		t.Fatal("aktor terbatas buku harus menghasilkan klausa")
	}
	if containsValue(clause, string(domain.BookSyariah)) {
		t.Fatalf("nilai buku bocor ke teks SQL: %q", clause)
	}
	if !regexp.MustCompile(`\$3\b`).MatchString(clause) {
		t.Fatalf("klausa harus memakai placeholder startArg ($3): %q", clause)
	}
	assertPlaceholderSesuaiStartArg(t, clause, args, 3)
}

func TestJournalBookFilterTidakMenyisipkanNilai(t *testing.T) {
	aktor := domain.Actor{Role: domain.RoleSupervisor, BranchCode: "001", Book: domain.BookSyariah}
	clause, args := journalBookFilter("je", aktor, 2)
	if clause == "" {
		t.Fatal("aktor terbatas buku harus menghasilkan klausa")
	}
	if containsValue(clause, string(domain.BookSyariah)) {
		t.Fatalf("nilai buku bocor ke teks SQL: %q", clause)
	}
	if !regexp.MustCompile(`\$2\b`).MatchString(clause) {
		t.Fatalf("klausa harus memakai placeholder startArg ($2): %q", clause)
	}
	assertPlaceholderSesuaiStartArg(t, clause, args, 2)
}

// containsValue melaporkan apakah sebuah nilai muncul sebagai literal di klausa.
// Placeholder ($n) dan ekspresi kolom tidak dihitung sebagai nilai.
func containsValue(clause, value string) bool {
	return value != "" && regexp.MustCompile(regexp.QuoteMeta(value)).MatchString(clause)
}

// assertPlaceholderSesuaiStartArg memastikan klausa memakai tepat SEKITAR startArg:
// placeholder tertinggi == startArg + len(args) - 1 untuk helper ber-argumen tunggal
// (semua helper scope di sini mengembalikan satu argumen). Ini menangkap placeholder
// menggantung ($n tanpa argumen) maupun argumen menganggur tanpa placeholder.
func assertPlaceholderSesuaiStartArg(t *testing.T, clause string, args []any, startArg int) {
	t.Helper()
	if len(args) == 0 {
		t.Fatalf("klausa berisi tetapi args kosong: %q", clause)
	}
	want := startArg + len(args) - 1
	if got := maxPlaceholder(clause); got != want {
		t.Fatalf("placeholder tertinggi $%d, ingin $%d (startArg %d + %d argumen); klausa=%q args=%v",
			got, want, startArg, len(args), clause, args)
	}
}
