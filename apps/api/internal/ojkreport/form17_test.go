package ojkreport

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form17StubSource mengembalikan baris ekuitas berbeda per tanggal agar Form 00.17
// dapat diuji membaca saldo 31 Des T-2/T-1/T tanpa basis data.
type form17StubSource struct {
	*stubSource
	rowsByDate map[string][]domain.ReportRow
}

func (s *form17StubSource) GetBalanceSheet(_ context.Context, asOf time.Time, _ string) (*domain.BalanceSheet, error) {
	return &domain.BalanceSheet{
		TotalAssets: decimal.NewFromInt(100000),
		Rows:        s.rowsByDate[asOf.Format("2006-01-02")],
	}, nil
}

func newForm17StubSource() *form17StubSource {
	eq := func(code, name string, amount int64) domain.ReportRow {
		return domain.ReportRow{AccountCode: code, AccountName: name, Amount: decimal.NewFromInt(amount)}
	}
	return &form17StubSource{
		stubSource: newStubSource(),
		rowsByDate: map[string][]domain.ReportRow{
			"2024-12-31": {
				eq("30100", "Modal Disetor", 1000),
				eq("30200", "Laba Ditahan", 5000),
				eq("30300", "Laba Tahun Berjalan", 200),
				eq("30400", "Cadangan Umum", 100),
			},
			"2025-12-31": {
				eq("30100", "Modal Disetor", 1500),
				eq("30200", "Laba Ditahan", 6000),
				eq("30300", "Laba Tahun Berjalan", 300),
				eq("30400", "Cadangan Umum", 150),
			},
			"2026-12-31": {
				eq("30100", "Modal Disetor", 2000),
				eq("13100", "Modal Disetor Syariah", 50),
				eq("30200", "Laba Ditahan", 7000),
				eq("30300", "Laba Tahun Berjalan", 400),
				eq("13200", "Laba Ditahan Syariah", 10),
				eq("30400", "Cadangan Umum", 200),
			},
		},
	}
}

func form17Amount(t *testing.T, sec TableSection, sandi, kolom string) decimal.Decimal {
	t.Helper()
	v := findCell(t, sec, sandi, kolom).Value
	d, err := decimal.NewFromString(v)
	if err != nil {
		t.Fatalf("nilai Form 00.17 sandi %s kolom %s bukan angka: %q", sandi, kolom, v)
	}
	return d
}

// Form 00.17 pada posisi Desember harus terbit dengan 17 baris dan 12 kolom (6 kolom
// tersedia + 6 kolom tidak tersedia), serta susunan sandi resmi.
func TestBuildForm17Desember17Baris12Kolom(t *testing.T) {
	b, err := NewBuilder(newForm17StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.17")

	if len(sec.Rows) != 17 {
		t.Fatalf("Form 00.17 harus 17 baris, dapat %d", len(sec.Rows))
	}
	if got := len(sec.Columns) + len(sec.Unavailable); got != 12 {
		t.Fatalf("Form 00.17 harus 12 kolom (tersedia+tidak tersedia), dapat %d", got)
	}

	want := []string{
		"10000000", "10100000", "10200000", "10300000", "10400000", "10500000",
		"10600000", "19900000", "20000000", "20100000", "20200000", "20300000",
		"20400000", "20500000", "20600000", "29900000", "30000000",
	}
	got := make([]string, 0, len(sec.Rows))
	for _, r := range sec.Rows {
		got = append(got, r.Key)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sandi Form 00.17 = %v, ingin %v", got, want)
	}
}

// Kolom bersumber harus membaca saldo akhir tahun yang benar: baris 10000000 dari 31
// Des T-2, 20000000 dari 31 Des T-1, dan 30000000 dari 31 Des T.
func TestBuildForm17SaldoTahunYangBenar(t *testing.T) {
	b, err := NewBuilder(newForm17StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.17")

	cases := []struct {
		sandi string
		kolom string
		want  int64
	}{
		{"10000000", form17SandiModalDisetor, 1000},
		{"10000000", form17SandiCadUmum, 100},
		{"10000000", form17SandiSaldoLaba, 5200}, // 5000 + 200
		{"20000000", form17SandiModalDisetor, 1500},
		{"20000000", form17SandiCadUmum, 150},
		{"20000000", form17SandiSaldoLaba, 6300},    // 6000 + 300
		{"30000000", form17SandiModalDisetor, 2050}, // 2000 + 13100 syariah 50
		{"30000000", form17SandiCadUmum, 200},
		{"30000000", form17SandiSaldoLaba, 7410}, // 7000 + 400 + 13200 syariah 10
	}
	for _, c := range cases {
		got := form17Amount(t, sec, c.sandi, c.kolom)
		if !got.Equal(decimal.NewFromInt(c.want)) {
			t.Errorf("Form 00.17 %s kolom %s = %s, ingin %d", c.sandi, c.kolom, got, c.want)
		}
	}
}

// Kolom yang belum punya akun COA didaftarkan sebagai kolom tidak tersedia dengan
// alasan, dan baris mutasi ditulis "-" (bukan nol) beserta alasannya.
func TestBuildForm17KolomTanpaSumberDanBarisMutasi(t *testing.T) {
	b, err := NewBuilder(newForm17StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.17")

	for _, sandi := range []string{
		form17SandiTambahan, form17SandiSumbangan, form17SandiDSM,
		form17SandiLabaRugi, form17SandiRevaluasi, form17SandiCadTujuan,
	} {
		if strings.TrimSpace(unavailableColumn(t, sec, sandi).Reason) == "" {
			t.Errorf("kolom %s tanpa sumber harus mencantumkan alasan", sandi)
		}
	}

	// Baris mutasi: kolom bersumber tetap ditulis "-", bukan nol, dan beralasan.
	row := rowByKey(t, sec, "10100000")
	if strings.TrimSpace(row.Reason) == "" {
		t.Fatal("baris mutasi 10100000 harus mencantumkan alasan")
	}
	for _, kolom := range []string{form17SandiModalDisetor, form17SandiCadUmum, form17SandiSaldoLaba} {
		if got := findCell(t, sec, "10100000", kolom).Value; got != "-" {
			t.Errorf("baris mutasi 10100000 kolom %s = %q, ingin \"-\"", kolom, got)
		}
	}
	// Pos Penambah/Pengurang Lainnya tidak diisi residu.
	if got := findCell(t, sec, "19900000", form17SandiSaldoLaba).Value; got != "-" {
		t.Errorf("baris 19900000 kolom XI = %q, ingin \"-\" (bukan residu)", got)
	}
}

// Kolom XII hanya diisi bila seluruh kolom angka baris itu bernilai; karena kolom
// IV-IX belum punya akun, seluruh baris menuliskan Jumlah "-". Logika penjumlahannya
// diuji langsung pada fungsi murni.
func TestBuildForm17JumlahHanyaBilaLengkap(t *testing.T) {
	b, err := NewBuilder(newForm17StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.December, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.17")

	for _, sandi := range []string{"10000000", "20000000", "30000000", "10100000"} {
		if got := findCell(t, sec, sandi, form17SandiJumlah).Value; got != "-" {
			t.Errorf("baris %s kolom XII = %q, ingin \"-\" karena baris belum lengkap", sandi, got)
		}
	}

	if _, ok := form17HitungJumlah([]string{"1000", "2000", "-"}); ok {
		t.Fatal("Jumlah tidak boleh dihitung bila ada kolom bernilai \"-\"")
	}
	total, ok := form17HitungJumlah([]string{"1000", "2000", "3000"})
	if !ok || !total.Equal(decimal.NewFromInt(6000)) {
		t.Fatalf("Jumlah baris lengkap = %s (ok=%v), ingin 6000", total, ok)
	}
}

// Form 00.17 hanya terbit sebagai tabel pada posisi Desember; posisi lain mencatatnya
// sebagai TIDAK DIBANGUN dengan alasan, bukan menghilangkannya.
func TestGenerateMonthlyForm17HanyaDesember(t *testing.T) {
	b, err := NewBuilder(newForm17StubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly Maret: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "00.17" {
			t.Fatal("Form 00.17 tidak boleh dibangun untuk posisi Maret")
		}
	}
	alasan := skippedReason(t, b, "00.17")
	if !strings.Contains(alasan, "Desember") {
		t.Fatalf("alasan Form 00.17 non-Desember = %q, ingin menyebut Desember", alasan)
	}

	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.Contains(buf.String(), "# FORM 00.17 TIDAK DIBANGUN:") {
		t.Fatal("posisi non-Desember harus menulis Form 00.17 sebagai TIDAK DIBANGUN")
	}
}
