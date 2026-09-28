package ojkreport

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form18StubSource melengkapi stubSource dengan sumber arus kas dan neraca per tanggal
// agar Form 00.18 dapat diuji pada posisi Desember tanpa basis data.
type form18StubSource struct {
	*stubSource
	cashByDate map[string]decimal.Decimal
	flows      map[string]*domain.CashFlow
}

func (s *form18StubSource) GetBalanceSheet(_ context.Context, asOf time.Time, _ string) (*domain.BalanceSheet, error) {
	return &domain.BalanceSheet{
		TotalAssets: decimal.NewFromInt(100000),
		Rows: []domain.ReportRow{{
			AccountCode: "10101", AccountName: "Kas Teller",
			Amount: s.cashByDate[asOf.Format("2006-01-02")],
		}},
	}, nil
}

func (s *form18StubSource) GetCashFlow(_ context.Context, from, to time.Time, _ string) (*domain.CashFlow, error) {
	key := from.Format("2006-01-02") + ".." + to.Format("2006-01-02")
	if cf, ok := s.flows[key]; ok {
		return cf, nil
	}
	return &domain.CashFlow{}, nil
}

func arusKasRow(code, name string, amount int64) domain.ReportRow {
	return domain.ReportRow{AccountCode: code, AccountName: name, Amount: decimal.NewFromInt(amount)}
}

func newForm18StubSource() *form18StubSource {
	cfT := &domain.CashFlow{
		Operating: decimal.NewFromInt(800),
		Investing: decimal.NewFromInt(-200),
		Financing: decimal.NewFromInt(400),
		NetChange: decimal.NewFromInt(1000),
		Rows: []domain.ReportRow{
			arusKasRow("40100", "Pendapatan Bunga Kredit", 700),
			arusKasRow("50100", "Beban Bunga Deposito", -100),
			arusKasRow("20100", "Tabungan", 500),
			arusKasRow("10301", "Kredit - Pokok", -300),
			arusKasRow("10600", "Aset Tetap dan Inventaris", -200),
			arusKasRow("30100", "Modal Disetor", 400),
		},
	}
	cfT1 := &domain.CashFlow{
		Operating: decimal.NewFromInt(500),
		Investing: decimal.NewFromInt(-100),
		Financing: decimal.Zero,
		NetChange: decimal.NewFromInt(400),
		Rows: []domain.ReportRow{
			arusKasRow("40100", "Pendapatan Bunga Kredit", 300),
			arusKasRow("20100", "Tabungan", 200),
			arusKasRow("10600", "Aset Tetap dan Inventaris", -100),
		},
	}
	return &form18StubSource{
		stubSource: newStubSource(),
		cashByDate: map[string]decimal.Decimal{
			"2024-12-31": decimal.NewFromInt(600),
			"2025-12-31": decimal.NewFromInt(1000),
			"2026-12-31": decimal.NewFromInt(2000),
		},
		flows: map[string]*domain.CashFlow{
			"2026-01-01..2026-12-31": cfT,
			"2025-01-01..2025-12-31": cfT1,
		},
	}
}

// Susunan sandi Form 00.18 harus persis susunan resmi Form 00.18 – 1 (PDF #page 295-296).
func TestForm18SusunanSandiResmi(t *testing.T) {
	want := []string{
		"14010000", "14020000", "14030000", "14040000", "14050000", "14060000",
		"14070000", "14080000", "14090000", "14100000", "14110000", "14120000",
		"14130000", "14140000", "14150000", "14160000", "14170000", "14180000",
		"14190000", "14200000", "14210000", "14220000", "14230000", "14240000",
		"14250000", "14260000", "10000000",
		"21010000", "21020000", "21030000", "21040000", "21990000", "20000000",
		"31010000", "31020000", "31030000", "31990000", "30000000",
		"40000000", "50000000", "60000000",
	}
	got := make([]string, 0, len(form18Lines))
	for _, l := range form18Lines {
		got = append(got, l.Sandi)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sandi Form 00.18 = %v, ingin %v", got, want)
	}
}

// Setiap kode COA daun pada bagan akun baku (kecuali kas dan setara kas) harus punya
// pemetaan Form 00.18, dan setiap sandi tujuan harus ada di susunan resmi. Kas dan
// setara kas sengaja TIDAK dipetakan karena sumber mengeluarkannya dari lawan jurnal.
func TestForm18PemetaanSejajarSeedDanSandi(t *testing.T) {
	if dup := DuplicateCOACodes(COAMapping18Draft); len(dup) > 0 {
		t.Fatalf("kode COA ganda pada pemetaan Form 00.18: %v", dup)
	}

	cash := make(map[string]bool, len(form18CashCOACodes))
	for _, c := range form18CashCOACodes {
		cash[c] = true
	}
	seed := make(map[string]bool, len(seedLeafCOACodes))
	for _, c := range seedLeafCOACodes {
		seed[c] = true
	}

	// Sandi tujuan harus berupa baris rincian (bukan baris neto turunan).
	rincian := make(map[string]bool)
	for _, l := range form18Lines {
		if len(l.TotalFrom) == 0 && l.Sandi != "50000000" && l.Sandi != "60000000" {
			rincian[l.Sandi] = true
		}
	}

	mapped := make(map[string]bool, len(COAMapping18Draft))
	for _, e := range COAMapping18Draft {
		mapped[e.COACode] = true
		if e.Form != "00.18" {
			t.Errorf("COA %s form = %q, ingin 00.18", e.COACode, e.Form)
		}
		if e.Sign != 1 {
			t.Errorf("COA %s Sign = %d; arus kas sudah bertanda sehingga ingin 1", e.COACode, e.Sign)
		}
		if !rincian[e.Sandi] {
			t.Errorf("COA %s menunjuk sandi %s yang bukan baris rincian form18Lines", e.COACode, e.Sandi)
		}
		if strings.TrimSpace(e.Note) == "" {
			t.Errorf("COA %s tanpa catatan penalaran", e.COACode)
		}
	}

	for _, code := range seedLeafCOACodes {
		if cash[code] {
			if mapped[code] {
				t.Errorf("kas dan setara kas %s tidak boleh dipetakan ke baris arus kas", code)
			}
			continue
		}
		if !mapped[code] {
			t.Errorf("COA %s belum dipetakan ke baris Form 00.18", code)
		}
	}
	for code := range mapped {
		if !seed[code] {
			t.Errorf("COA %s pada COAMapping18Draft tidak ada di seedLeafCOACodes", code)
		}
	}
}

// Invarian inti: neto tiap aktivitas + saldo kas awal = saldo kas akhir, dan neto
// keseluruhan sama dengan sumber (domain.CashFlow.NetChange). Kolom T-1 diuji dengan
// jalur yang sama.
func TestBuildForm18InvariantArusKas(t *testing.T) {
	b, err := NewBuilder(newForm18StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.18")

	cases := []struct {
		sandi string
		kolom string
		want  int64
	}{
		{"14010000", form18SandiT, 700},
		{"14060000", form18SandiT, -100},
		{"14200000", form18SandiT, 500},
		{"14150000", form18SandiT, -300},
		{"10000000", form18SandiT, 800},
		{"21010000", form18SandiT, -200},
		{"20000000", form18SandiT, -200},
		{"31990000", form18SandiT, 400},
		{"30000000", form18SandiT, 400},
		{"40000000", form18SandiT, 1000},
		{"50000000", form18SandiT, 1000},
		{"60000000", form18SandiT, 2000},
		{"14010000", form18SandiT1, 300},
		{"10000000", form18SandiT1, 500},
		{"20000000", form18SandiT1, -100},
		{"50000000", form18SandiT1, 600},
		{"60000000", form18SandiT1, 1000},
	}
	for _, c := range cases {
		got := form18Amount(t, sec, c.sandi, c.kolom)
		if !got.Equal(decimal.NewFromInt(c.want)) {
			t.Errorf("Form 00.18 %s kolom %s = %s, ingin %d", c.sandi, c.kolom, got, c.want)
		}
	}

	src := newForm18StubSource()
	for _, kolom := range []string{form18SandiT, form18SandiT1} {
		operasi := form18Amount(t, sec, "10000000", kolom)
		investasi := form18Amount(t, sec, "20000000", kolom)
		pendanaan := form18Amount(t, sec, "30000000", kolom)
		perubahan := form18Amount(t, sec, "40000000", kolom)
		awal := form18Amount(t, sec, "50000000", kolom)
		akhir := form18Amount(t, sec, "60000000", kolom)
		if !operasi.Add(investasi).Add(pendanaan).Equal(perubahan) {
			t.Errorf("kolom %s: neto operasi+investasi+pendanaan %s != Peningkatan Arus Kas %s",
				kolom, operasi.Add(investasi).Add(pendanaan), perubahan)
		}
		if !awal.Add(perubahan).Equal(akhir) {
			t.Errorf("kolom %s: kas awal %s + perubahan %s != kas akhir %s", kolom, awal, perubahan, akhir)
		}
		// Konsisten dengan sumber: neto keseluruhan = NetChange periode itu.
		var netChange decimal.Decimal
		if kolom == form18SandiT {
			netChange = src.flows["2026-01-01..2026-12-31"].NetChange
		} else {
			netChange = src.flows["2025-01-01..2025-12-31"].NetChange
		}
		if !perubahan.Equal(netChange) {
			t.Errorf("kolom %s: Peningkatan Arus Kas %s != NetChange sumber %s", kolom, perubahan, netChange)
		}
	}
}

// Baris tanpa akun COA sumber harus ditulis "-", bukan 0, dan alasannya tercantum.
func TestBuildForm18BarisTanpaSumberDitulisMinus(t *testing.T) {
	b, err := NewBuilder(newForm18StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.18")

	row := rowByKey(t, sec, "14030000")
	if strings.TrimSpace(row.Reason) == "" {
		t.Fatal("baris 14030000 tanpa sumber harus mencantumkan alasan")
	}
	for _, kolom := range []string{form18SandiT, form18SandiT1} {
		if got := form18Cell(t, sec, "14030000", kolom); got != "-" {
			t.Errorf("baris tanpa sumber %s kolom %s = %q, ingin \"-\"", "14030000", kolom, got)
		}
	}

	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "00.18|14030000|III 31 Des Tahun T|-") {
		t.Fatal("sel Jumlah baris tanpa sumber harus ditulis \"-\" pada berkas")
	}
	if !strings.Contains(out, "TIDAK TERSEDIA:") {
		t.Fatal("alasan baris tanpa sumber harus tercantum pada berkas")
	}
}

// Kode arus kas yang belum dipetakan harus menolak ekspor Desember, bukan menghasilkan
// neto yang tidak tie-out.
func TestBuildForm18MenolakCOABelumDipetakan(t *testing.T) {
	src := newForm18StubSource()
	src.flows["2026-01-01..2026-12-31"].Rows = append(
		src.flows["2026-01-01..2026-12-31"].Rows,
		arusKasRow("99999", "Akun Baru", 10),
	)
	_, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "")
	if !errors.Is(err, ErrIncompleteMapping) {
		t.Fatalf("COA belum dipetakan harus menolak ekspor dengan ErrIncompleteMapping, dapat %v", err)
	}
	if !strings.Contains(err.Error(), "18.00") && !strings.Contains(err.Error(), "00.18") {
		t.Fatalf("galat harus menyebut form 00.18: %v", err)
	}
}

// Form 00.18 hanya terbit sebagai tabel pada posisi Desember; posisi lain mencatatnya
// sebagai TIDAK DIBANGUN, bukan menghilangkannya.
func TestGenerateMonthlyForm18HanyaDesember(t *testing.T) {
	bMaret, err := NewBuilder(newForm18StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly Maret: %v", err)
	}
	for _, sec := range bMaret.Tables {
		if sec.Form == "00.18" {
			t.Fatal("Form 00.18 tidak boleh dibangun untuk posisi Maret")
		}
	}
	alasan := skippedReason(t, bMaret, "00.18")
	if !strings.Contains(alasan, "Desember") {
		t.Fatalf("alasan Form 00.18 non-Desember = %q, ingin menyebut Desember", alasan)
	}

	var buf bytes.Buffer
	if err := WriteText(&buf, bMaret); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(buf.String(), "# FORM 00.18 TIDAK DIBANGUN:") {
		t.Fatal("posisi non-Desember harus menulis Form 00.18 sebagai TIDAK DIBANGUN")
	}
}

// form18Cell mengembalikan nilai satu kolom Form 00.18.
func form18Cell(t *testing.T, sec TableSection, sandi, kolom string) string {
	t.Helper()
	return findCell(t, sec, sandi, kolom).Value
}

// form18Amount mengembalikan nilai numerik satu kolom Form 00.18.
func form18Amount(t *testing.T, sec TableSection, sandi, kolom string) decimal.Decimal {
	t.Helper()
	v := form18Cell(t, sec, sandi, kolom)
	d, err := decimal.NewFromString(v)
	if err != nil {
		t.Fatalf("nilai Form 00.18 sandi %s kolom %s bukan angka: %q", sandi, kolom, v)
	}
	return d
}
