package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/i18n"
)

// TestPesanDomainSelarasDenganKatalog menjaga agar setiap galat domain berkode
// (NewLocalizedError) benar-benar punya terjemahan katalog, dan bahwa makna pesan
// Indonesia lama tidak berubah: katalog ID harus sama persis dengan pesan domain
// (kecuali untuk sentinel yang memang berbahasa Inggris, yang dicocokkan lewat EN).
//
// Penjaga ini membaca kode sumber, bukan daftar manual, sehingga galat lokal baru
// yang lupa didaftarkan ke katalog langsung tertangkap.
func TestPesanDomainSelarasDenganKatalog(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal menemukan lokasi uji")
	}
	root := filepath.Dir(thisFile)

	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("membaca %s: %v", root, err)
	}

	diperiksa := 0
	for _, ent := range entries {
		nama := ent.Name()
		if !strings.HasSuffix(nama, ".go") || strings.HasSuffix(nama, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		berkas, err := parser.ParseFile(fset, filepath.Join(root, nama), nil, 0)
		if err != nil {
			t.Fatalf("mem-parse %s: %v", nama, err)
		}
		ast.Inspect(berkas, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn, ok := call.Fun.(*ast.Ident)
			if !ok || fn.Name != "NewLocalizedError" || len(call.Args) < 2 {
				return true
			}
			codeLit, ok1 := call.Args[0].(*ast.BasicLit)
			msgLit, ok2 := call.Args[1].(*ast.BasicLit)
			if !ok1 || !ok2 {
				pos := fset.Position(call.Pos())
				t.Errorf("%s: argumen NewLocalizedError harus literal agar dapat diperiksa", pos)
				return true
			}
			diperiksa++
			code, _ := strconv.Unquote(codeLit.Value)
			msg, _ := strconv.Unquote(msgLit.Value)

			id := i18n.T(i18n.ID, i18n.Code(code))
			en := i18n.T(i18n.EN, i18n.Code(code))
			if id == code {
				pos := fset.Position(call.Pos())
				t.Errorf("%s: kode %q tidak ada di katalog i18n", pos, code)
				return true
			}
			// Makna lama dipertahankan: pesan domain harus muncul apa adanya di ID
			// (galat Indonesia) atau di EN (galat yang memang berbahasa Inggris).
			if id != msg && en != msg {
				pos := fset.Position(call.Pos())
				t.Errorf("%s: pesan domain %q tidak selaras dengan katalog (ID=%q, EN=%q)",
					pos, msg, id, en)
			}
			return true
		})
	}

	if diperiksa == 0 {
		t.Fatal("tidak ada NewLocalizedError yang diperiksa; penjaga tidak bekerja")
	}
}
