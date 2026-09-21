package ojkreport

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

func TestBuildMappingReviewRowsMelengkapiNamaDanKeputusan(t *testing.T) {
	entries := []MappingEntry{
		{COACode: "10100", Form: "01.00", Sandi: "1101010000", Sign: 1, Note: "Kas"},
		{COACode: "99999", Form: "01.00", Sandi: "1101010000", Sign: -1},
	}
	names := map[string]string{"10100": "Kas"}
	decidedAt := time.Date(2026, time.March, 2, 3, 4, 5, 0, time.UTC)
	reviews := []MappingReview{
		{Form: "01.00", COACode: "10100", Decision: ReviewApproved, Note: "setuju", DecidedBy: "admin", DecidedAt: decidedAt},
	}

	rows := BuildMappingReviewRows(entries, names, reviews)
	if len(rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(rows))
	}
	first := rows[0]
	if first.COACode != "10100" {
		t.Fatalf("baris pertama = %s, ingin terurut 10100", first.COACode)
	}
	if first.COAName != "Kas" || !first.COAFound {
		t.Errorf("nama COA = %q found=%v, ingin Kas/true", first.COAName, first.COAFound)
	}
	if first.PosName != "Kas dalam Rupiah" {
		t.Errorf("nama pos = %q, ingin Kas dalam Rupiah", first.PosName)
	}
	if first.Decision != ReviewApproved || first.ReviewNote != "setuju" || first.DecidedBy != "admin" {
		t.Errorf("keputusan tidak menempel: %+v", first)
	}
	if first.DecidedAt == nil || !first.DecidedAt.Equal(decidedAt) {
		t.Errorf("decided_at = %v, ingin %v", first.DecidedAt, decidedAt)
	}

	// Kode yang tidak ada di bagan akun harus ditandai, bukan diberi nama kosong diam-diam.
	second := rows[1]
	if second.COAFound {
		t.Errorf("COA 99999 tidak ada di bagan akun, COAFound harus false")
	}
	if second.Decision != "" {
		t.Errorf("baris tanpa keputusan harus kosong, dapat %q", second.Decision)
	}
}

func TestUnmappedPositionsMenemukanPosTanpaSumber(t *testing.T) {
	positions := UnmappedPositions(COAMappingDraft)
	// "Kas dalam Valuta Asing" tidak pernah dipetakan COA mana pun pada draf.
	found := false
	for _, p := range positions {
		if p.Form == "01.00" && p.Sandi == "1101020000" {
			found = true
			if p.PosName != "Kas dalam Valuta Asing" {
				t.Errorf("nama pos = %q", p.PosName)
			}
		}
		// Baris total turunan bukan pos yang butuh sumber COA.
		if p.Sandi == "1000000000" || p.Sandi == "2000000000" || p.Sandi == "3000000000" {
			t.Errorf("baris total %s tidak boleh dianggap belum punya sumber", p.Sandi)
		}
	}
	if !found {
		t.Fatalf("pos 1101020000 seharusnya terdaftar belum punya sumber COA")
	}
}

func TestUnmappedCOACodesMenemukanKodeTanpaPemetaan(t *testing.T) {
	bs := &domain.BalanceSheet{Rows: []domain.ReportRow{
		{AccountCode: "10100", AccountName: "Kas"},
		{AccountCode: "99999", AccountName: "Akun Baru"},
		{AccountCode: "00000", AccountName: "Selisih"},
	}}
	is := &domain.IncomeStatement{Rows: []domain.ReportRow{
		{AccountCode: "88888", AccountName: "Pendapatan Baru"},
	}}

	gaps := UnmappedCOACodes(bs, is, COAMappingDraft)
	if len(gaps) != 2 {
		t.Fatalf("celah = %+v, ingin 2 (99999 dan 88888)", gaps)
	}
	// Terurut form lalu kode: 01.00/99999 lalu 02.00/88888.
	if gaps[0].Form != "01.00" || gaps[0].COACode != "99999" || gaps[0].COAName != "Akun Baru" {
		t.Errorf("celah pertama = %+v", gaps[0])
	}
	if gaps[1].Form != "02.00" || gaps[1].COACode != "88888" {
		t.Errorf("celah kedua = %+v", gaps[1])
	}
	// Kode "00000" bukan celah pemetaan, melainkan jurnal tidak seimbang.
	for _, g := range gaps {
		if g.COACode == "00000" {
			t.Errorf("00000 tidak boleh masuk daftar celah pemetaan")
		}
	}
}

func TestHasImbalance(t *testing.T) {
	if HasImbalance(&domain.BalanceSheet{}, &domain.IncomeStatement{}) {
		t.Fatal("laporan kosong seharusnya tidak dianggap tidak seimbang")
	}
	bs := &domain.BalanceSheet{Rows: []domain.ReportRow{{AccountCode: "00000"}}}
	if !HasImbalance(bs, &domain.IncomeStatement{}) {
		t.Fatal("baris 00000 harus menandai sumber tidak seimbang")
	}
}

func TestKnownMappingLine(t *testing.T) {
	if !KnownMappingLine("01.00", "10100") {
		t.Fatal("baris pemetaan yang ada harus dikenali")
	}
	if KnownMappingLine("01.00", "99999") {
		t.Fatal("kode yang tidak ada di pemetaan harus ditolak")
	}
	if KnownMappingLine("09.99", "10100") {
		t.Fatal("form yang tidak ada di pemetaan harus ditolak")
	}
}

// Semua sandi pada pemetaan harus punya nama pos, agar antarmuka tidak menampilkan
// sandi tanpa keterangan.
func TestSemuaSandiPemetaanPunyaNamaPos(t *testing.T) {
	for _, e := range COAMappingDraft {
		if e.Sandi == "" {
			continue
		}
		if PositionName(e.Form, e.Sandi) == "" {
			t.Errorf("sandi %s form %s tidak punya nama pos", e.Sandi, e.Form)
		}
	}
}
