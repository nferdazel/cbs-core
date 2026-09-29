package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// asetKeuanganOJKRepoStub mengimplementasikan domain.AsetKeuanganRegisterRepository untuk
// menguji Form 18.00 lewat RepoSource tanpa basis data.
type asetKeuanganOJKRepoStub struct {
	domain.AsetKeuanganRegisterRepository
	rows []domain.AsetKeuanganItem
	err  error
}

func (s asetKeuanganOJKRepoStub) ListAsetKeuanganForOJK(context.Context, time.Time) ([]domain.AsetKeuanganItem, error) {
	return s.rows, s.err
}

var _ domain.AsetKeuanganRegisterRepository = asetKeuanganOJKRepoStub{}

func asetKeuanganFormItem(noRekening, counterparty, jenis string) domain.AsetKeuanganItem {
	return domain.AsetKeuanganItem{
		NoRekening:                  noRekening,
		CounterpartyID:              counterparty,
		JenisCode:                   jenis,
		TanggalMulai:                time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC),
		TanggalJatuhTempo:           time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		SukuBunga:                   decimal.NewFromInt(7),
		Nominal:                     decimal.NewFromInt(1_000_000),
		NilaiAgunanDiperhitungkan:   decimal.NewFromInt(400_000),
		CKPN:                        decimal.NewFromInt(10_000),
		CKPNAsetBaik:                decimal.NewFromInt(5_000),
		CKPNAsetKurangBaik:          decimal.NewFromInt(3_000),
		CKPNAsetTidakBaik:           decimal.NewFromInt(2_000),
		KlasifikasiAsetKeuanganCode: domain.AsetKeuanganKlasifikasiBiayaPerolehanDiamortasi,
		JenisCKPNCode:               domain.AsetKeuanganJenisCKPNKolektif,
	}
}

// Satu baris register = satu baris form, tanpa baris JUMLAH, dan setiap nilai ditulis apa
// adanya (tidak ada kolom turunan yang dikarang).
func TestBuildForm18_00TanpaJumlahDanNilaiApaAdanya(t *testing.T) {
	rows := []domain.AsetKeuanganItem{
		asetKeuanganFormItem("AKL-001", "PL-001", "10"),
		asetKeuanganFormItem("AKL-002", "PL-002", "99"),
	}
	sec := BuildForm18_00(rows, ReportingOffice{Sandi: "1101"})

	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 tanpa baris JUMLAH", len(sec.Rows))
	}
	row := rowByKey(t, sec, "AKL-001")
	for sandi, want := range map[string]string{
		"I": "1101", "II": "AKL-001", "III": "PL-001", "IV": "10",
		"V": "01-06-2025", "VI": "01-06-2026", "VII": "7.00", "VIII": "1000000",
		"IX": "400000", "X": "10000", "XI": "5000", "XII": "3000",
		"XIII": "2000", "XIV": "3", "XV": "2",
	} {
		if got := cellValue(row, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	for _, r := range sec.Rows {
		if strings.EqualFold(r.Key, "JUMLAH") {
			t.Fatal("Form 18.00 tidak boleh memiliki baris JUMLAH")
		}
	}
}

// Format tanggal Form 18.00 adalah TT-MM-TTTT (PDF #page 235), sama seperti form lain di
// sini tetapi dinyatakan eksplisit karena form ini menyimpang dari konvensi umum.
func TestBuildForm18_00FormatTanggalTTMMTTTT(t *testing.T) {
	item := asetKeuanganFormItem("AKL-001", "PL-001", "10")
	item.TanggalMulai = time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)
	sec := BuildForm18_00([]domain.AsetKeuanganItem{item}, ReportingOffice{Sandi: "1101"})

	if got := cellValue(rowByKey(t, sec, "AKL-001"), "V"); got != "31-12-2025" {
		t.Fatalf("kolom V = %q, ingin 31-12-2025", got)
	}
}

// Tata kelola kolom: 15 kolom (I + II..XV) dibawa, kolom I dipasang dari kantor pelapor,
// dan tanpa kantor pasti kolom I didaftarkan tidak tersedia.
func TestBuildForm18_00KolomDanSandiKantor(t *testing.T) {
	sec := BuildForm18_00(
		[]domain.AsetKeuanganItem{asetKeuanganFormItem("AKL-001", "PL-001", "10")},
		ReportingOffice{Sandi: "1101"})
	if len(sec.Columns) != 15 {
		t.Fatalf("jumlah kolom = %d, ingin 15 (I + II..XV)", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[14].Sandi != "XV" {
		t.Fatalf("kolom pertama/terakhir = %+v/%+v", sec.Columns[0], sec.Columns[14])
	}

	tanpaKantor := BuildForm18_00(
		[]domain.AsetKeuanganItem{asetKeuanganFormItem("AKL-001", "PL-001", "10")},
		ReportingOffice{Reason: "kantor belum dikonfigurasi"})
	if !punyaUnavailable(tanpaKantor.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Catatan form menyebut ketiadaan baris JUMLAH dan bahwa nilai adalah isian bank.
func TestBuildForm18_00Catatan(t *testing.T) {
	sec := BuildForm18_00(nil, ReportingOffice{Sandi: "1101"})
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "TIDAK ada baris JUMLAH") {
		t.Fatalf("catatan harus menyatakan tidak ada baris JUMLAH: %q", gabung)
	}
	if !strings.Contains(gabung, "isian bank") {
		t.Fatalf("catatan harus menyatakan nilai adalah isian bank: %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 18.00 muncul di bundel bulanan.
func TestBuilderForm18_00LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		AsetKeuangan: asetKeuanganOJKRepoStub{rows: []domain.AsetKeuanganItem{
			asetKeuanganFormItem("AKL-001", "PL-001", "10"),
			asetKeuanganFormItem("AKL-002", "PL-002", "99"),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "18.00")
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 tanpa JUMLAH", len(sec.Rows))
	}
}

// Register kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada SkippedForms
// dengan alasan spesifik.
func TestBuilderForm18_00RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), AsetKeuangan: asetKeuanganOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "18.00" {
			t.Fatal("Form 18.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "18.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 18.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 18.00 kosong harus tercatat pada SkippedForms")
}
