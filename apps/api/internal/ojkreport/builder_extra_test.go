package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// ojkStubSource melengkapi stubSource dengan sumber data opsional (kredit, profil
// bank, penempatan) untuk menguji jalur form daftar dan komponen rasio.
type ojkStubSource struct {
	*stubSource
	loans      []LoanRow
	profile    *BankProfileConfig
	placements []PlacementRow
}

func (s *ojkStubSource) ListLoansForOJK(_ context.Context, _ time.Time, _ domain.Actor) ([]LoanRow, error) {
	return s.loans, nil
}

func (s *ojkStubSource) GetBankProfileConfig(_ context.Context) (*BankProfileConfig, error) {
	return s.profile, nil
}

func (s *ojkStubSource) ListPlacementsForOJK(_ context.Context, _ time.Time, _ domain.Actor) ([]PlacementRow, error) {
	return s.placements, nil
}

func tableByForm(t *testing.T, b *Bundle, form string) TableSection {
	t.Helper()
	for _, sec := range b.Tables {
		if sec.Form == form {
			return sec
		}
	}
	t.Fatalf("tabel form %s tidak ditemukan", form)
	return TableSection{}
}

// Dengan sumber lengkap, form 00.00/05.00/06.00 muncul dan NPL/ROA terisi dari data.
func TestGenerateMonthlyDenganSumberLengkap(t *testing.T) {
	base := newStubSource()
	base.bs.TotalAssets = decimal.NewFromInt(3000)
	src := &ojkStubSource{
		stubSource: base,
		loans: []LoanRow{
			{Status: "DISBURSED", LoanNumber: "LN-001", Collectibility: "1_LANCAR", Outstanding: decimal.NewFromInt(600)},
			{Status: "DISBURSED", LoanNumber: "LN-002", Collectibility: "3_KURANG_LANCAR", Outstanding: decimal.NewFromInt(400), RequiredCKPN: decimal.NewFromInt(40)},
		},
		profile:    &BankProfileConfig{Name: "BPR Uji", Configured: true},
		placements: []PlacementRow{{CounterpartyBank: "Bank Uji", PlacementType: "DEPOSITO", Collectibility: "LANCAR", Outstanding: decimal.NewFromInt(1_000_000)}},
	}

	b, err := NewBuilder(src).GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}

	form06 := tableByForm(t, b, "06.00")
	if got := findCell(t, form06, "LN-002", form06SandiBakiDebet).Value; got != "400" {
		t.Errorf("Form 06.00 baki debet = %q, ingin 400", got)
	}
	form05 := tableByForm(t, b, "05.00")
	if got := findCell(t, form05, "Bank Uji", form05SandiJumlah).Value; got != "1000000" {
		t.Errorf("Form 05.00 jumlah = %q, ingin 1000000", got)
	}
	_ = tableByForm(t, b, "00.00")

	// NPL gross = 400/1000 = 40%; neto = (400-40)/1000 = 36%.
	hasil := RasioKeuangan(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), nil, nil,
		KomponenRasio{
			Kredit:                    NPLDariLoanRows(src.loans),
			RataRataTotalAset:         decimal.NewFromInt(3000),
			RataRataTotalAsetTersedia: true,
		})
	if g := rasioBySandi(hasil, sandiNPLGross); !g.Tersedia || !g.NilaiPersen.Equal(decimal.NewFromInt(40)) {
		t.Errorf("NPL gross = %s tersedia=%v, ingin 40", g.NilaiPersen, g.Tersedia)
	}
	if n := rasioBySandi(hasil, sandiNPLNeto); !n.Tersedia || !n.NilaiPersen.Equal(decimal.NewFromInt(36)) {
		t.Errorf("NPL neto = %s tersedia=%v, ingin 36", n.NilaiPersen, n.Tersedia)
	}

	// Baris Form 00.08 NPL pada bundle harus terisi (bukan "-").
	nplLine := findLine(t, b, "00.08", sandiNPLGross)
	if nplLine.UnavailableReason != "" {
		t.Errorf("NPL gross harus terisi, tetapi: %s", nplLine.UnavailableReason)
	}
}

// Tanpa sumber opsional, form daftar didaftarkan sebagai belum dapat dibangun.
func TestGenerateMonthlyTanpaSumberOpsionalMencatatFormDaftar(t *testing.T) {
	b, err := NewBuilder(newStubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	want := map[string]bool{"00.00": false, "05.00": false, "06.00": false}
	for _, f := range b.SkippedForms {
		if _, ok := want[f.Form]; ok {
			if f.UnavailableReason == "" {
				t.Errorf("form %s tidak dapat dibangun tanpa alasan", f.Form)
			}
			want[f.Form] = true
		}
	}
	for form, found := range want {
		if !found {
			t.Errorf("form %s seharusnya dicatat belum dapat dibangun", form)
		}
	}
	if len(b.Tables) != 0 {
		t.Fatalf("tanpa sumber opsional tidak boleh ada tabel, dapat %d", len(b.Tables))
	}
}
