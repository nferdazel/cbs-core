package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// kepemilikanOJKRepoStub mengimplementasikan domain.KepemilikanRegisterRepository
// untuk menguji Form 00.01 lewat RepoSource tanpa basis data.
type kepemilikanOJKRepoStub struct {
	domain.KepemilikanRegisterRepository
	rows []domain.KepemilikanItem
	err  error
}

func (s kepemilikanOJKRepoStub) ListKepemilikanForOJK(context.Context, time.Time) ([]domain.KepemilikanItem, error) {
	return s.rows, s.err
}

var _ domain.KepemilikanRegisterRepository = kepemilikanOJKRepoStub{}

func kepemilikanFormRows() []domain.KepemilikanItem {
	return []domain.KepemilikanItem{
		{
			ShareholderName:       "Budi Santoso",
			ShareholderAddress:    "Jl. Merdeka 1",
			ShareholderTypeCode:   "01",
			ShareholderStatusCode: "01",
			NominalAmount:         decimal.NewFromInt(50_000_000),
			OwnershipPercentage:   decimal.NewFromInt(60),
			ChangeStatusCode:      "2",
		},
		{
			ShareholderName:       "PT Sejahtera",
			ShareholderAddress:    "Jl. Sudirman 2",
			ShareholderTypeCode:   "02",
			ShareholderStatusCode: "02",
			NominalAmount:         decimal.NewFromInt(20_000_000),
			OwnershipPercentage:   decimal.NewFromInt(40),
			ChangeStatusCode:      "9",
		},
	}
}

// Form 00.01 merinci tiap pemegang saham sebagai satu baris, delapan kolom (mulai dari
// Nama, tanpa Sandi Kantor), dan TANPA baris JUMLAH di kaki tabel.
func TestBuildForm00_01BarisPerPemegangTanpaJumlah(t *testing.T) {
	sec := BuildForm00_01(kepemilikanFormRows())

	if sec.Form != "00.01" {
		t.Fatalf("form = %q, ingin 00.01", sec.Form)
	}
	if len(sec.Columns) != 8 {
		t.Fatalf("jumlah kolom = %d, ingin 8", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Nama" {
		t.Fatalf("kolom pertama = %+v, ingin I Nama (bukan Sandi Kantor)", sec.Columns[0])
	}
	if sec.Columns[7].Sandi != "VIII" || sec.Columns[7].Nama != "Status Perubahan" {
		t.Fatalf("kolom terakhir = %+v, ingin VIII Status Perubahan", sec.Columns[7])
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 (tanpa JUMLAH)", len(sec.Rows))
	}
	for _, row := range sec.Rows {
		if row.Key == "JUMLAH" {
			t.Fatal("Form 00.01 tidak boleh punya baris JUMLAH")
		}
	}

	first := rowByKey(t, sec, "Budi Santoso|01")
	for sandi, want := range map[string]string{
		"I": "Budi Santoso", "II": "Jl. Merdeka 1", "III": "01",
		"V": "01", "VI": "50000000", "VII": "60.00", "VIII": "2",
	} {
		if got := cellValue(first, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
}

// Kolom IV No. Identitas selalu "-" dan alasannya menyebut keputusan privasi; tidak
// ada nilai identitas yang pernah mengalir ke keluaran.
func TestBuildForm00_01KolomIVSelaluDashBeralasan(t *testing.T) {
	sec := BuildForm00_01(kepemilikanFormRows())

	if !punyaUnavailable(sec.Unavailable, "IV") {
		t.Fatal("kolom IV No. Identitas harus didaftarkan belum tersedia")
	}
	var alasan string
	for _, u := range sec.Unavailable {
		if u.Sandi == "IV" {
			alasan = u.Reason
		}
	}
	if !strings.Contains(alasan, "privasi") || !strings.Contains(alasan, "KEPUTUSAN-OJK") {
		t.Fatalf("alasan kolom IV harus menyebut keputusan privasi: %q", alasan)
	}
	for _, row := range sec.Rows {
		if got := cellValue(row, "IV"); got != "-" {
			t.Errorf("kolom IV = %q, ingin - selalu", got)
		}
	}
}

// Alamat yang dikosongkan bank tidak diisi alasan "sumber tidak ada", melainkan
// mengikuti aturan form (<2% boleh dikosongkan).
func TestBuildForm00_01AlamatKosongMengikutiAturanForm(t *testing.T) {
	row := kepemilikanFormRows()[0]
	row.ShareholderAddress = ""
	sec := BuildForm00_01([]domain.KepemilikanItem{row})

	baris := rowByKey(t, sec, "Budi Santoso|01")
	if got := cellValue(baris, "II"); got != "-" {
		t.Fatalf("alamat kosong = %q, ingin -", got)
	}
	if baris.Reason == "" {
		t.Fatal("alamat kosong harus beralasan")
	}
	if !strings.Contains(baris.Reason, "kurang dari 2%") {
		t.Errorf("alasan alamat kosong harus mengikuti aturan form: %q", baris.Reason)
	}
	if strings.HasPrefix(strings.TrimSpace(baris.Reason), "sumber tidak ada") {
		t.Errorf("alamat kosong tidak boleh beralasan 'sumber tidak ada': %q", baris.Reason)
	}
}

// Lewat RepoSource dengan register terisi, Form 00.01 muncul di bundel bulanan dengan
// kolom pertama Nama (bukan Sandi Kantor) dan barisnya per pemegang saham.
func TestBuilderForm00_01LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source:      newStubSource(),
		Kepemilikan: kepemilikanOJKRepoStub{rows: kepemilikanFormRows()},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "00.01")
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(sec.Rows))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Nama" {
		t.Fatalf("kolom pertama = %+v, ingin I Nama", sec.Columns[0])
	}
}

// Register kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada
// SkippedForms dengan alasan spesifik.
func TestBuilderForm00_01RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), Kepemilikan: kepemilikanOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "00.01" {
			t.Fatal("Form 00.01 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "00.01" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 00.01 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 00.01 kosong harus tercatat pada SkippedForms")
}
