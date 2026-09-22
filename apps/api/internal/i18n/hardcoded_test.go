package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// hardcodedFuncArg memetakan helper respons ke indeks argumen pesan/kode. Hanya
// pemanggilan fungsi paket (ast.Ident) yang diperiksa; method seperti
// logger.Error(...) tidak termasuk.
var hardcodedFuncArg = map[string]int{
	"Success":         2,
	"SuccessWithMeta": 2,
	"Error":           2,
	"ErrorCode":       2,
	"ErrorCodef":      2,
	"writeError":      2,
	"writeErrorCode":  2,
	"writeErrorCodef": 2,
}

// allowedLiteralSites mendaftarkan pengecualian yang disengaja, dengan alasan.
// Kunci: "namafile.go|NamaFungsi". Daftar kosong berarti tidak ada literal yang
// dibiarkan; tambahkan entri hanya dengan alasan yang jelas agar penjaga ini tidak
// berubah menjadi pengaman palsu.
var allowedLiteralSites = map[string]string{}

// containsStringLiteral melaporkan apakah ekspresi membawa literal teks, termasuk
// penggabungan seperti "prefix" + err.Error(). Ini menutup celah hardcode sebagian.
func containsStringLiteral(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.BinaryExpr:
		return containsStringLiteral(e.X) || containsStringLiteral(e.Y)
	case *ast.ParenExpr:
		return containsStringLiteral(e.X)
	}
	return false
}

// TestTanpaPesanHardcoded menolak pesan teks literal baru pada helper respons API.
// Pesan tetap wajib lewat katalog i18n; yang dinamis (mis. err.Error()) tetap boleh.
func TestTanpaPesanHardcoded(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal menemukan lokasi uji")
	}
	root := filepath.Dir(thisFile)
	sumber := []string{
		filepath.Join(root, "..", "handler", "http"),
		filepath.Join(root, "..", "middleware"),
	}

	diperiksa := 0
	for _, dir := range sumber {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("membaca %s: %v", dir, err)
		}
		for _, ent := range entries {
			nama := ent.Name()
			if !strings.HasSuffix(nama, ".go") || strings.HasSuffix(nama, "_test.go") {
				// Berkas uji memang penuh teks harapan, bukan pesan produksi.
				continue
			}
			path := filepath.Join(dir, nama)
			fset := token.NewFileSet()
			berkas, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("mem-parse %s: %v", path, err)
			}
			ast.Inspect(berkas, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				fn, ok := call.Fun.(*ast.Ident)
				if !ok {
					return true
				}
				idx, ok := hardcodedFuncArg[fn.Name]
				if !ok || len(call.Args) <= idx {
					return true
				}
				diperiksa++
				if !containsStringLiteral(call.Args[idx]) {
					return true
				}
				key := nama + "|" + fn.Name
				if _, diizinkan := allowedLiteralSites[key]; diizinkan {
					return true
				}
				pos := fset.Position(call.Args[idx].Pos())
				t.Errorf("%s: %s memakai pesan teks literal; pakai konstanta i18n (mis. i18n.Msg...)",
					pos, fn.Name)
				return true
			})
		}
	}

	if diperiksa == 0 {
		// Tanpa ini, penjaga bisa "lulus" karena berkas tidak ditemukan.
		t.Fatal("tidak ada pemanggilan helper respons yang diperiksa; penjaga tidak bekerja")
	}
}
