package ojkreport

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestFormatRupiahPenuhTanpaDesimal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"0", "0"},
		{"1000", "1000"},
		{"1000000000", "1000000000"},
		{"1500.75", "1501"},
		{"999.4", "999"},
		{"-250.5", "-251"},
		{"-0.0", "0"},
	}
	for _, c := range cases {
		got := FormatRupiah(decimal.RequireFromString(c.in))
		if got != c.want {
			t.Errorf("FormatRupiah(%s) = %s, ingin %s", c.in, got, c.want)
		}
	}
}

func TestWriteTextFormatDanNolTetapTertulis(t *testing.T) {
	b := &Bundle{
		Period:             time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:          time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC),
		Book:               "CONVENTIONAL",
		GeneratedAt:        time.Date(2026, time.April, 1, 0, 0, 0, 0, time.UTC),
		Deadline:           time.Date(2026, time.April, 10, 0, 0, 0, 0, time.UTC),
		CorrectionDeadline: time.Date(2026, time.April, 15, 0, 0, 0, 0, time.UTC),
		MappingStatus:      MappingStatus,
		Sections: []Section{
			{Form: "01.00", Name: "Laporan Posisi Keuangan", Lines: []Line{
				{Sandi: "1101010000", Name: "Kas dalam Rupiah", Amount: decimal.Zero},
				{Sandi: "1104010100", Name: "Kredit yang Diberikan (Baki Debet)", Amount: decimal.NewFromInt(5000)},
			}},
		},
		SkippedForms: []OJKFormDefinition{
			{Form: "00.00", Name: "Informasi Pokok BPR", UnavailableReason: "belum ada data"},
		},
	}

	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText error: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "FORM|SANDI|NAMA POS|JUMLAH") {
		t.Fatal("header kolom dengan pemisah konsisten tidak ditemukan")
	}
	if !strings.Contains(out, "01.00|1101010000|Kas dalam Rupiah|0") {
		t.Fatal("nominal nol harus tetap ditulis")
	}
	if !strings.Contains(out, "01.00|1104010100|Kredit yang Diberikan (Baki Debet)|5000") {
		t.Fatal("nominal harus rupiah penuh tanpa desimal")
	}
	if !strings.Contains(out, MappingStatus) {
		t.Fatal("status pemetaan draf harus tercantum di berkas")
	}
	if !strings.Contains(out, "# FORM 00.00 TIDAK DIBANGUN: belum ada data") {
		t.Fatal("form yang tidak dibangun harus tercantum beserta alasannya")
	}

	// Setiap baris data memakai pemisah yang sama persis: 3 pemisah -> 4 kolom.
	for _, line := range strings.Split(out, "\n") {
		if line == "" || strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "FORM"+ColumnSeparator) {
			continue
		}
		if n := strings.Count(line, ColumnSeparator); n != 3 {
			t.Fatalf("baris %q punya %d pemisah, ingin 3", line, n)
		}
	}
}
