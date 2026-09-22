package service_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// seedRowPattern membaca baris INSERT eod_step_definitions di migrasi 000077:
//
//	('aro', 10, TRUE, FALSE, 'OPERASIONAL', '[]',
//	 'deskripsi ...')
var seedRowPattern = regexp.MustCompile(`\('([^']+)',\s*(\d+),\s*(TRUE|FALSE),\s*(TRUE|FALSE),\s*'([^']*)',\s*'(\[[^']*\])'`)

// TestEODSeedReproducesLegacyOrderAndSwitches membuktikan seed migrasi 000077
// mereproduksi urutan, saklar, kategori, dan prasyarat yang berlaku sekarang (oracle
// lama). Tanpa ini, migrasi bisa mengubah perilaku produksi tanpa terlihat.
func TestEODSeedReproducesLegacyOrderAndSwitches(t *testing.T) {
	root := configSeedRepoRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "packages", "db-migrations", "000077_eod_orchestration.up.sql"))
	if err != nil {
		t.Fatalf("membaca migrasi 000077: %v", err)
	}

	type row struct {
		Code          string
		Sequence      int
		Enabled       bool
		Core          bool
		Category      string
		Prerequisites []string
	}
	normalize := func(defs []domain.EODStepDefinition) []row {
		out := make([]row, 0, len(defs))
		for _, def := range defs {
			prereq := def.Prerequisites
			if prereq == nil {
				prereq = []string{}
			}
			out = append(out, row{
				Code:          def.Code,
				Sequence:      def.Sequence,
				Enabled:       def.Enabled,
				Core:          def.Core,
				Category:      def.Category,
				Prerequisites: prereq,
			})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
		return out
	}

	var seeded []domain.EODStepDefinition
	for _, m := range seedRowPattern.FindAllStringSubmatch(string(raw), -1) {
		def := domain.EODStepDefinition{
			Code:     m[1],
			Sequence: atoiT(t, m[2]),
			Enabled:  m[3] == "TRUE",
			Core:     m[4] == "TRUE",
			Category: m[5],
		}
		if err := json.Unmarshal([]byte(m[6]), &def.Prerequisites); err != nil {
			t.Fatalf("prasyarat %s tidak sah: %v", def.Code, err)
		}
		seeded = append(seeded, def)
	}
	if len(seeded) == 0 {
		t.Fatal("tidak ada baris seed yang terbaca; pola berubah?")
	}

	got := normalize(seeded)
	want := normalize(unitEODDefinitions())
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("seed migrasi 000077 TIDAK sama dengan urutan lama.\nseed: %+v\nlama: %+v", got, want)
	}
}

// atoiT mengubah digit hasil regex menjadi int tanpa membawa dependensi tambahan.
func atoiT(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("angka seed tidak sah: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
