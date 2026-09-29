package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// penyertaanOJKRepoStub mengimplementasikan domain.PenyertaanRegisterRepository untuk
// menguji Form 16.00 lewat RepoSource tanpa basis data.
type penyertaanOJKRepoStub struct {
	domain.PenyertaanRegisterRepository
	rows []domain.PenyertaanItem
	err  error
}

func (s penyertaanOJKRepoStub) ListPenyertaanForOJK(context.Context, time.Time) ([]domain.PenyertaanItem, error) {
	return s.rows, s.err
}

var _ domain.PenyertaanRegisterRepository = penyertaanOJKRepoStub{}

func penyertaanFormItem(noRegister, counterparty, metode, kualitas string) domain.PenyertaanItem {
	return domain.PenyertaanItem{
		NoRegister:           noRegister,
		CounterpartyID:       counterparty,
		MetodePenyertaanCode: metode,
		KualitasCode:         kualitas,
		TujuanPenyertaanCode: domain.PenyertaanTujuanLembagaPenunjang,
		TanggalMulai:         time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC),
		PersentasePenyertaan: decimal.NewFromInt(25),
		Nominal:              decimal.NewFromInt(1_000_000),
		JumlahBulanLaporan:   decimal.NewFromInt(900_000),
		CKPN:                 decimal.NewFromInt(10_000),
		CKPNAsetBaik:         decimal.NewFromInt(5_000),
		CKPNAsetKurangBaik:   decimal.NewFromInt(3_000),
		CKPNAsetTidakBaik:    decimal.NewFromInt(2_000),
		JenisCKPNCode:        domain.PenyertaanJenisCKPNKolektif,
	}
}

// Satu baris register = satu baris form, tanpa baris JUMLAH, dan setiap nilai ditulis
// apa adanya (tidak ada kolom turunan yang dikarang).
func TestBuildForm16_00TanpaJumlahDanNilaiApaAdanya(t *testing.T) {
	rows := []domain.PenyertaanItem{
		penyertaanFormItem("PM-001", "PL-001", "1", "1"),
		penyertaanFormItem("PM-002", "PL-002", "2", "5"),
	}
	sec := BuildForm16_00(rows, ReportingOffice{Sandi: "1101"})

	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 tanpa baris JUMLAH", len(sec.Rows))
	}
	row := rowByKey(t, sec, "PM-001")
	for sandi, want := range map[string]string{
		"I": "1101", "II": "PM-001", "III": "PL-001", "IV": "1", "V": "1",
		"VI": "1", "VII": "01-06-2025", "VIII": "25.00", "IX": "1000000",
		"X": "900000", "XI": "10000", "XII": "5000", "XIII": "3000",
		"XIV": "2000", "XV": "2",
	} {
		if got := cellValue(row, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	// Tidak ada baris dengan kunci JUMLAH.
	for _, r := range sec.Rows {
		if r.Key == "JUMLAH" || strings.EqualFold(r.Key, "JUMLAH") {
			t.Fatal("Form 16.00 tidak boleh memiliki baris JUMLAH")
		}
	}
}

// Kolom X "Jumlah Bulan Laporan" diteruskan sebagai nilai bank, BUKAN dihitung dari
// Tanggal Mulai ke periodEnd (definisi PDF #page 228 adalah nilai tercatat rupiah).
func TestBuildForm16_00KolomXBukanTurunan(t *testing.T) {
	item := penyertaanFormItem("PM-001", "PL-001", "1", "1")
	item.JumlahBulanLaporan = decimal.NewFromInt(123_456_789)
	sec := BuildForm16_00([]domain.PenyertaanItem{item}, ReportingOffice{Sandi: "1101"})

	if got := cellValue(rowByKey(t, sec, "PM-001"), "X"); got != "123456789" {
		t.Fatalf("kolom X = %q, ingin nilai bank diteruskan apa adanya", got)
	}
}

// Tata kelola kolom: 15 kolom (I + II..XV) dibawa, kolom I dipasang dari kantor pelapor,
// dan tanpa kantor pasti kolom I didaftarkan tidak tersedia.
func TestBuildForm16_00KolomDanSandiKantor(t *testing.T) {
	sec := BuildForm16_00(
		[]domain.PenyertaanItem{penyertaanFormItem("PM-001", "PL-001", "1", "1")},
		ReportingOffice{Sandi: "1101"})
	if len(sec.Columns) != 15 {
		t.Fatalf("jumlah kolom = %d, ingin 15 (I + II..XV)", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[14].Sandi != "XV" {
		t.Fatalf("kolom pertama/terakhir = %+v/%+v", sec.Columns[0], sec.Columns[14])
	}

	tanpaKantor := BuildForm16_00(
		[]domain.PenyertaanItem{penyertaanFormItem("PM-001", "PL-001", "1", "1")},
		ReportingOffice{Reason: "kantor belum dikonfigurasi"})
	if !punyaUnavailable(tanpaKantor.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Catatan form menyebut ketiadaan baris JUMLAH dan bahwa nilai adalah isian bank.
func TestBuildForm16_00Catatan(t *testing.T) {
	sec := BuildForm16_00(nil, ReportingOffice{Sandi: "1101"})
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "TIDAK ada baris JUMLAH") {
		t.Fatalf("catatan harus menyatakan tidak ada baris JUMLAH: %q", gabung)
	}
	if !strings.Contains(gabung, "isian bank") {
		t.Fatalf("catatan harus menyatakan nilai adalah isian bank: %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 16.00 muncul di bundel bulanan.
func TestBuilderForm16_00LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		Penyertaan: penyertaanOJKRepoStub{rows: []domain.PenyertaanItem{
			penyertaanFormItem("PM-001", "PL-001", "1", "1"),
			penyertaanFormItem("PM-002", "PL-002", "2", "5"),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "16.00")
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 tanpa JUMLAH", len(sec.Rows))
	}
	if sec.Columns[0].Sandi != "II" {
		t.Fatalf("kolom pertama = %+v, ingin II (Sandi Kantor tidak tersedia tanpa bank_offices)", sec.Columns[0])
	}
}

// Register kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada SkippedForms
// dengan alasan spesifik.
func TestBuilderForm16_00RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), Penyertaan: penyertaanOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "16.00" {
			t.Fatal("Form 16.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "16.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 16.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 16.00 kosong harus tercatat pada SkippedForms")
}
