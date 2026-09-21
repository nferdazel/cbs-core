package ojkreport

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

type stubSource struct {
	bs *domain.BalanceSheet
	is *domain.IncomeStatement

	gotBSAsOf          time.Time
	gotISFrom, gotISTo time.Time
	err                error
}

func (s *stubSource) GetBalanceSheet(_ context.Context, asOf time.Time, _ string) (*domain.BalanceSheet, error) {
	s.gotBSAsOf = asOf
	if s.err != nil {
		return nil, s.err
	}
	return s.bs, nil
}

func (s *stubSource) GetIncomeStatement(_ context.Context, from, to time.Time, _ string) (*domain.IncomeStatement, error) {
	s.gotISFrom, s.gotISTo = from, to
	if s.err != nil {
		return nil, s.err
	}
	return s.is, nil
}

func row(code, name string, amount int64) domain.ReportRow {
	return domain.ReportRow{AccountCode: code, AccountName: name, Amount: decimal.NewFromInt(amount)}
}

func newStubSource() *stubSource {
	return &stubSource{
		bs: &domain.BalanceSheet{
			Rows: []domain.ReportRow{
				row("10101", "Kas Teller", 1000),
				row("10301", "Kredit - Pokok", 5000),
				row("10900", "Cadangan Kerugian Penurunan Nilai (PPAP)", 200),
				row("20100", "Tabungan", 3000),
				row("30100", "Modal Disetor", 2000),
				row("30300", "Laba Tahun Berjalan", 800),
			},
		},
		is: &domain.IncomeStatement{
			Rows: []domain.ReportRow{
				row("40100", "Pendapatan Bunga Kredit", 700),
				row("50100", "Beban Bunga Deposito", 100),
			},
		},
	}
}

func findLine(t *testing.T, b *Bundle, form, sandi string) Line {
	t.Helper()
	for _, s := range b.Sections {
		if s.Form != form {
			continue
		}
		for _, l := range s.Lines {
			if l.Sandi == sandi {
				return l
			}
		}
	}
	t.Fatalf("baris %s pada form %s tidak ditemukan", sandi, form)
	return Line{}
}

func TestGenerateMonthlyMenghitungPosDanTotal(t *testing.T) {
	src := newStubSource()
	b, err := NewBuilder(src).GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}

	// Laporan dibaca per akhir bulan; laba rugi year-to-date sejak 1 Januari.
	if want := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC); !src.gotBSAsOf.Equal(want) {
		t.Fatalf("asOf = %s, ingin %s", src.gotBSAsOf, want)
	}
	if want := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC); !src.gotISFrom.Equal(want) {
		t.Fatalf("from = %s, ingin %s", src.gotISFrom, want)
	}
	if want := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC); !src.gotISTo.Equal(want) {
		t.Fatalf("to = %s, ingin %s", src.gotISTo, want)
	}

	cases := []struct {
		form, sandi string
		want        int64
	}{
		{"01.00", "1104010100", 5000}, // Kredit yang Diberikan
		{"01.00", "1104020000", -200}, // CKPN mengurangi (Sign -1)
		{"01.00", "2102010100", 3000}, // Tabungan
		{"01.00", "3101010000", 2000}, // Modal Disetor
		{"01.00", "3105020000", 800},  // Laba Tahun Berjalan
		{"01.00", "1000000000", 5800}, // TOTAL ASET
		{"01.00", "2000000000", 3000}, // Total Liabilitas
		{"01.00", "3000000000", 2800}, // Total Ekuitas
		{"01.00", "", 5800},           // TOTAL LIABILITAS DAN EKUITAS
		{"02.00", "4100000000", 700},  // Pendapatan Operasional
		{"02.00", "5100000000", 100},  // Beban Operasional
		{"02.00", "3104040100", 600},  // Laba Operasional
		{"02.00", "3104040400", 600},  // Jumlah Laba Tahun Berjalan
	}
	for _, c := range cases {
		line := findLine(t, b, c.form, c.sandi)
		if !line.Amount.Equal(decimal.NewFromInt(c.want)) {
			t.Errorf("form %s sandi %q = %s, ingin %d", c.form, c.sandi, line.Amount, c.want)
		}
	}

	// Pos tanpa nilai tetap hadir (bukan dihilangkan).
	if line := findLine(t, b, "01.00", "1101020000"); !line.Amount.IsZero() {
		t.Fatalf("Kas dalam Valuta Asing ingin 0, dapat %s", line.Amount)
	}

	if !b.Deadline.Equal(time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("tenggat = %s", b.Deadline)
	}
	if len(b.SkippedForms) == 0 {
		t.Fatal("form yang tidak dibangun harus didaftarkan")
	}
	if b.MappingStatus != MappingStatus {
		t.Fatalf("status pemetaan = %s", b.MappingStatus)
	}
}

func TestGenerateMonthlyMenolakPemetaanTidakLengkap(t *testing.T) {
	// Buang pemetaan 50100 agar baris beban deposito tak punya tujuan.
	var incomplete []MappingEntry
	for _, e := range COAMappingDraft {
		if e.COACode == "50100" {
			continue
		}
		incomplete = append(incomplete, e)
	}

	_, err := NewBuilderWithMapping(newStubSource(), incomplete).
		GenerateMonthly(context.Background(), time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), "")
	if err == nil {
		t.Fatal("pemetaan tidak lengkap harus ditolak")
	}
	if !errors.Is(err, ErrIncompleteMapping) {
		t.Fatalf("error = %v, ingin ErrIncompleteMapping", err)
	}
	if !strings.Contains(err.Error(), "50100") {
		t.Fatalf("pesan error harus menyebut kode yang belum dipetakan: %v", err)
	}
}

func TestGenerateMonthlyMenolakSumberTidakSeimbang(t *testing.T) {
	src := newStubSource()
	src.bs.Rows = append(src.bs.Rows, row("00000", "Penyeimbang (selisih jurnal)", 5))

	_, err := NewBuilder(src).GenerateMonthly(context.Background(), time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, ErrUnbalancedSource) {
		t.Fatalf("error = %v, ingin ErrUnbalancedSource", err)
	}
}

func TestGenerateMonthlyMenolakPemetaanGanda(t *testing.T) {
	dup := append([]MappingEntry{}, COAMappingDraft...)
	dup = append(dup, MappingEntry{COACode: "10101", Form: "01.00", Sandi: "1101010000", Sign: 1})

	_, err := NewBuilderWithMapping(newStubSource(), dup).
		GenerateMonthly(context.Background(), time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, ErrInvalidMapping) {
		t.Fatalf("error = %v, ingin ErrInvalidMapping", err)
	}
}

func TestGenerateMonthlyTanpaSumber(t *testing.T) {
	_, err := NewBuilder(nil).GenerateMonthly(context.Background(), time.Now(), "")
	if err == nil {
		t.Fatal("builder tanpa sumber harus menolak")
	}
}
