package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// asetTetapOJKRepoStub mengimplementasikan domain.AsetTetapRegisterRepository untuk
// menguji Form 08.00 lewat RepoSource tanpa basis data.
type asetTetapOJKRepoStub struct {
	domain.AsetTetapRegisterRepository
	rows []domain.AsetTetapItem
	err  error
}

func (s asetTetapOJKRepoStub) ListAsetTetapForOJK(context.Context, time.Time) ([]domain.AsetTetapItem, error) {
	return s.rows, s.err
}

var _ domain.AsetTetapRegisterRepository = asetTetapOJKRepoStub{}

func asetTetapFormItem(jenis, sumber, status, metode string, biaya, akum, rugi int64) domain.AsetTetapItem {
	return domain.AsetTetapItem{
		JenisAsetCode:                   jenis,
		SumberPerolehanCode:             sumber,
		StatusAsetCode:                  status,
		BiayaPerolehan:                  decimal.NewFromInt(biaya),
		AkumulasiPenyusutanAmortisasi:   decimal.NewFromInt(akum),
		AkumulasiKerugianPenurunanNilai: decimal.NewFromInt(rugi),
		MetodePengukuranCode:            metode,
	}
}

// Dua aset dengan kombinasi sama (II/III/IV/IX) digabung menjadi SATU baris dengan kolom
// nominal dijumlahkan, sesuai aturan penggabungan Form 08.00 (PDF #page 184).
func TestBuildForm08_00KelompokkanPerKombinasi(t *testing.T) {
	rows := []domain.AsetTetapItem{
		asetTetapFormItem("101", "01", "1", "1", 1_000, 200, 0),
		asetTetapFormItem("101", "01", "1", "1", 2_000, 400, 100),
	}
	sec := BuildForm08_00(rows, ReportingOffice{Sandi: "1101"})

	// Satu baris kombinasi + satu baris JUMLAH.
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 (1 kombinasi + JUMLAH)", len(sec.Rows))
	}
	row := rowByKey(t, sec, "101|01|1|1")
	for sandi, want := range map[string]string{
		"I": "1101", "II": "101", "III": "01", "IV": "1",
		"V": "3000", "VI": "600", "VII": "100", "VIII": "2300", "IX": "1",
	} {
		if got := cellValue(row, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	if row.Reason != "" {
		t.Fatalf("baris lengkap tidak boleh beralasan: %q", row.Reason)
	}

	// Baris JUMLAH menjumlahkan kolom nominal bila seluruh baris lengkap.
	total := rowByKey(t, sec, form08_00TotalKey)
	for sandi, want := range map[string]string{
		"II": "JUMLAH", "V": "3000", "VI": "600", "VII": "100", "VIII": "2300",
	} {
		if got := cellValue(total, sandi); got != want {
			t.Errorf("JUMLAH kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	if total.Reason != "" {
		t.Fatalf("JUMLAH lengkap tidak boleh beralasan: %q", total.Reason)
	}
}

// Kolom VIII Nilai Tercatat = V - VI - VII persis; hasil negatif ditulis "-" beralasan
// dan membuat baris JUMLAH ditulis "-" juga.
func TestBuildForm08_00NilaiTercatatDanNegatif(t *testing.T) {
	sec := BuildForm08_00([]domain.AsetTetapItem{
		asetTetapFormItem("101", "01", "1", "1", 1_000, 200, 50),
	}, ReportingOffice{Sandi: "1101"})
	if got := cellValue(rowByKey(t, sec, "101|01|1|1"), "VIII"); got != "750" {
		t.Fatalf("nilai tercatat = %q, ingin 750", got)
	}

	negatif := BuildForm08_00([]domain.AsetTetapItem{
		asetTetapFormItem("101", "01", "1", "1", 100, 200, 0),
	}, ReportingOffice{Sandi: "1101"})
	baris := rowByKey(t, negatif, "101|01|1|1")
	if got := cellValue(baris, "VIII"); got != "-" {
		t.Fatalf("nilai tercatat negatif = %q, ingin -", got)
	}
	if baris.Reason == "" {
		t.Fatal("baris nilai tercatat negatif harus beralasan")
	}
	total := rowByKey(t, negatif, form08_00TotalKey)
	if got := cellValue(total, "VIII"); got != "-" {
		t.Fatalf("JUMLAH dengan baris tidak lengkap = %q, ingin -", got)
	}
	if total.Reason == "" {
		t.Fatal("JUMLAH dengan baris tidak lengkap harus beralasan")
	}
}

// Status Aset (IV) dikosongkan untuk aset tidak berwujud, bahkan bila register sempat
// menyimpan sandi; aset berwujud tanpa status ditulis "-" beralasan, bukan ditebak.
func TestBuildForm08_00StatusAset(t *testing.T) {
	sec := BuildForm08_00([]domain.AsetTetapItem{
		asetTetapFormItem("201", "02", "", "1", 1_000, 100, 0),
		asetTetapFormItem("202", "02", "1", "1", 2_000, 100, 0),
		asetTetapFormItem("101", "01", "", "1", 3_000, 100, 0),
	}, ReportingOffice{Sandi: "1101"})

	// Tidak berwujud: kolom IV kosong. Baris 201 dan 202 BERBEDA kombinasi karena
	// jenisnya beda, sehingga tetap dua baris.
	if got := cellValue(rowByKey(t, sec, "201|02||1"), "IV"); got != "" {
		t.Errorf("status aset tidak berwujud = %q, ingin kosong", got)
	}
	if got := cellValue(rowByKey(t, sec, "202|02||1"), "IV"); got != "" {
		t.Errorf("status aset tidak berwujud yang tersimpan harus dikosongkan, dapat %q", got)
	}
	// Berwujud tanpa status: "-" + alasan.
	berwujud := rowByKey(t, sec, "101|01||1")
	if got := cellValue(berwujud, "IV"); got != "-" {
		t.Errorf("status aset berwujud kosong = %q, ingin -", got)
	}
	if berwujud.Reason == "" {
		t.Error("baris berwujud tanpa status harus beralasan")
	}
}

// Tata kelola kolom: daftar kolom II..IX tetap dibawa, kolom I dipasang dari kantor
// pelapor, dan tanpa kantor pasti kolom I didaftarkan tidak tersedia.
func TestBuildForm08_00KolomDanSandiKantor(t *testing.T) {
	sec := BuildForm08_00([]domain.AsetTetapItem{
		asetTetapFormItem("101", "01", "1", "1", 1_000, 0, 0),
	}, ReportingOffice{Sandi: "1101"})
	if len(sec.Columns) != 9 {
		t.Fatalf("jumlah kolom = %d, ingin 9 (I + II..IX)", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[8].Sandi != "IX" {
		t.Fatalf("kolom pertama/terakhir = %+v/%+v", sec.Columns[0], sec.Columns[8])
	}

	tanpaKantor := BuildForm08_00(
		[]domain.AsetTetapItem{asetTetapFormItem("101", "01", "1", "1", 1_000, 0, 0)},
		ReportingOffice{Reason: "kantor belum dikonfigurasi"})
	if !punyaUnavailable(tanpaKantor.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Catatan form menyebut tidak adanya tie otomatis ke neraca dan aturan baris JUMLAH.
func TestBuildForm08_00Catatan(t *testing.T) {
	sec := BuildForm08_00(nil, ReportingOffice{Sandi: "1101"})
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "TIDAK memaksa") {
		t.Fatalf("catatan harus menyatakan tie ke neraca tidak dipaksa: %q", gabung)
	}
	if !strings.Contains(gabung, "JUMLAH") {
		t.Fatalf("catatan harus menyebut aturan baris JUMLAH: %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 08.00 muncul di bundel bulanan dengan
// baris tergabung; tanpa kantor pelapor kolom I tidak dikarang.
func TestBuilderForm08_00LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		AsetTetap: asetTetapOJKRepoStub{rows: []domain.AsetTetapItem{
			asetTetapFormItem("101", "01", "1", "1", 1_000, 200, 0),
			asetTetapFormItem("101", "01", "1", "1", 2_000, 400, 0),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "08.00")
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 (1 kombinasi + JUMLAH)", len(sec.Rows))
	}
	if sec.Columns[0].Sandi != "II" {
		t.Fatalf("kolom pertama = %+v, ingin II (Sandi Kantor tidak tersedia tanpa bank_offices)", sec.Columns[0])
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Register kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada SkippedForms
// dengan alasan spesifik.
func TestBuilderForm08_00RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), AsetTetap: asetTetapOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "08.00" {
			t.Fatal("Form 08.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "08.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 08.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 08.00 kosong harus tercatat pada SkippedForms")
}
