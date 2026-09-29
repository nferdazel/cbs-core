package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// pinjamanOJKRepoStub mengimplementasikan domain.PinjamanRegisterRepository untuk
// menguji Form 00.07 lewat RepoSource tanpa basis data.
type pinjamanOJKRepoStub struct {
	domain.PinjamanRegisterRepository
	rows []domain.PinjamanItem
	err  error
}

func (s pinjamanOJKRepoStub) ListPinjamanForOJK(context.Context, time.Time) ([]domain.PinjamanItem, error) {
	return s.rows, s.err
}

var _ domain.PinjamanRegisterRepository = pinjamanOJKRepoStub{}

func pinjamanFormRows() []domain.PinjamanItem {
	return []domain.PinjamanItem{
		{
			CounterpartyID:             "KRT-001",
			CreditorGroupCode:          "600",
			BankCode:                   "123456",
			LocationCode:               "1101",
			JenisCode:                  "10",
			RelationshipCode:           "12",
			StartDate:                  time.Date(2025, time.January, 2, 0, 0, 0, 0, time.UTC),
			MaturityDate:               time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
			InterestRate:               decimal.NewFromInt(9),
			InterestCalcCode:           "11",
			Plafon:                     decimal.NewFromInt(1_000),
			CollateralTypeCode:         "02",
			CollateralAmount:           decimal.NewFromInt(2_000),
			BakiDebet:                  decimal.NewFromInt(800),
			UnamortizedTransactionCost: decimal.NewFromInt(50),
			UnamortizedDiscount:        decimal.NewFromInt(20),
		},
		{
			CounterpartyID:             "KRT-002",
			CreditorGroupCode:          "700",
			BankCode:                   "654321",
			LocationCode:               "1102",
			JenisCode:                  "20",
			RelationshipCode:           "20",
			StartDate:                  time.Date(2025, time.February, 1, 0, 0, 0, 0, time.UTC),
			MaturityDate:               time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC),
			InterestRate:               decimal.NewFromInt(11),
			InterestCalcCode:           "22",
			Plafon:                     decimal.NewFromInt(2_000),
			CollateralTypeCode:         domain.PinjamanTanpaAgunanCode,
			CollateralAmount:           decimal.Zero,
			BakiDebet:                  decimal.NewFromInt(1_500),
			UnamortizedTransactionCost: decimal.NewFromInt(100),
			UnamortizedDiscount:        decimal.Zero,
		},
	}
}

// Form 00.07 merinci tiap pinjaman sebagai satu baris, tujuh belas kolom (sub-kolom
// Jangka Waktu/Suku Bunga dihitung terpisah; mulai dari ID Pihak Lawan, tanpa Sandi
// Kantor), dan menghitung Baki Debet Neto XV.
func TestBuildForm00_07KolomBarisDanNeto(t *testing.T) {
	sec := BuildForm00_07(pinjamanFormRows())

	if sec.Form != "00.07" {
		t.Fatalf("form = %q, ingin 00.07", sec.Form)
	}
	if len(sec.Columns) != 17 {
		t.Fatalf("jumlah kolom = %d, ingin 17", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "ID Pihak Lawan" {
		t.Fatalf("kolom pertama = %+v, ingin I ID Pihak Lawan (bukan Sandi Kantor)", sec.Columns[0])
	}
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("Form 00.07 tidak boleh menandai Sandi Kantor: kolom I-nya ID Pihak Lawan")
	}
	if len(sec.Rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3 (dua pinjaman + JUMLAH)", len(sec.Rows))
	}

	first := rowByKey(t, sec, "KRT-001")
	for sandi, want := range map[string]string{
		"I": "KRT-001", "II": "600", "III": "123456", "IV": "1101", "V": "10",
		"VI": "12", "VII.1": "2025-01-02", "VII.2": "2026-01-02",
		"VIII.1": "9.00", "VIII.2": "11", "IX": "1000", "X": "02", "XI": "2000",
		"XII": "800", "XIII": "50", "XIV": "20", "XV": "730",
	} {
		if got := cellValue(first, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}

	// Pinjaman tanpa agunan (XII kolom X/XI): sandi 299 dan nominal 0.
	kedua := rowByKey(t, sec, "KRT-002")
	if got := cellValue(kedua, "X"); got != "299" {
		t.Errorf("jenis agunan tanpa agunan = %q, ingin 299", got)
	}
	if got := cellValue(kedua, "XI"); got != "0" {
		t.Errorf("nominal agunan tanpa agunan = %q, ingin 0", got)
	}
}

// Baris JUMLAH menjumlahkan kolom angka IX, XI-XV bila seluruh baris lengkap.
func TestBuildForm00_07JumlahLengkap(t *testing.T) {
	sec := BuildForm00_07(pinjamanFormRows())
	total := rowByKey(t, sec, form00_07TotalKey)
	if total.Reason != "" {
		t.Fatalf("JUMLAH harus terisi saat semua baris lengkap: %s", total.Reason)
	}
	for sandi, want := range map[string]string{
		"IX": "3000", "XI": "2000", "XII": "2300",
		"XIII": "150", "XIV": "20", "XV": "2130",
	} {
		if got := cellValue(total, sandi); got != want {
			t.Errorf("JUMLAH kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
}

// Bila baki debet neto tidak dapat dihitung (biaya/diskonto melebihi baki debet),
// kolom XV ditulis "-" beralasan dan JUMLAH tidak diisi angka sebagian.
func TestBuildForm00_07NetoTidakLengkap(t *testing.T) {
	row := pinjamanFormRows()[0]
	row.UnamortizedTransactionCost = decimal.NewFromInt(900)
	row.UnamortizedDiscount = decimal.Zero
	sec := BuildForm00_07([]domain.PinjamanItem{row})

	baris := rowByKey(t, sec, "KRT-001")
	if got := cellValue(baris, "XV"); got != "-" {
		t.Fatalf("neto tidak lengkap = %q, ingin -", got)
	}
	if baris.Reason == "" {
		t.Fatal("baris tidak lengkap harus beralasan")
	}
	total := rowByKey(t, sec, form00_07TotalKey)
	if total.Reason == "" {
		t.Fatal("JUMLAH harus beralasan saat ada baris tidak lengkap")
	}
	for _, sandi := range []string{"IX", "XI", "XII", "XIII", "XIV", "XV"} {
		if got := cellValue(total, sandi); got != "-" {
			t.Errorf("JUMLAH kolom %s = %q, ingin -", sandi, got)
		}
	}
}

// Catatan form menyebut aturan tanpa agunan (sandi 299, nominal 0) supaya pembaca
// berkas tahu aturannya ditegakkan saat pengisian, bukan ditambal laporan.
func TestBuildForm00_07CatatanTanpaAgunan(t *testing.T) {
	sec := BuildForm00_07(nil)
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "299") || !strings.Contains(gabung, "tanpa agunan") {
		t.Fatalf("catatan harus menyebut aturan tanpa agunan (299): %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 00.07 muncul di bundel bulanan dengan
// kolom pertama ID Pihak Lawan dan barisnya per pinjaman.
func TestBuilderForm00_07LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source:   newStubSource(),
		Pinjaman: pinjamanOJKRepoStub{rows: pinjamanFormRows()},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.07")
	if len(sec.Rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3", len(sec.Rows))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "ID Pihak Lawan" {
		t.Fatalf("kolom pertama = %+v, ingin I ID Pihak Lawan", sec.Columns[0])
	}
}

// Register kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada
// SkippedForms dengan alasan spesifik.
func TestBuilderForm00_07RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), Pinjaman: pinjamanOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "00.07" {
			t.Fatal("Form 00.07 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "00.07" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 00.07 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 00.07 kosong harus tercatat pada SkippedForms")
}
