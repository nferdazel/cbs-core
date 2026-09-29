package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// propertiOJKRepoStub mengimplementasikan domain.PropertiRegisterRepository untuk
// menguji Form 17.00 lewat RepoSource tanpa basis data.
type propertiOJKRepoStub struct {
	domain.PropertiRegisterRepository
	rows []domain.PropertiItem
	err  error
}

func (s propertiOJKRepoStub) ListPropertiForOJK(context.Context, time.Time) ([]domain.PropertiItem, error) {
	return s.rows, s.err
}

var _ domain.PropertiRegisterRepository = propertiOJKRepoStub{}

func propertiFormRows() []domain.PropertiItem {
	return []domain.PropertiItem{
		{
			NoRegister:                    "PRP-001",
			JenisPropertiCode:             "1",
			AlamatProperti:                "Jl. Contoh No. 1",
			Koordinat:                     "-6.2, 106.8",
			TanggalPenetapan:              time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC),
			BiayaPerolehanNilaiWajar:      decimal.NewFromInt(1_000),
			AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(200),
			MetodePengukuranCode:          "1",
		},
		{
			NoRegister:                    "PRP-002",
			JenisPropertiCode:             "3",
			AlamatProperti:                "Jl. Contoh No. 2",
			Koordinat:                     "",
			TanggalPenetapan:              time.Date(2025, time.July, 15, 0, 0, 0, 0, time.UTC),
			BiayaPerolehanNilaiWajar:      decimal.NewFromInt(5_000),
			AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(500),
			MetodePengukuranCode:          "2",
		},
	}
}

// Form 17.00 merinci tiap properti sebagai satu baris, sepuluh kolom (Sandi Kantor +
// II..X), TIDAK punya baris JUMLAH, dan menghitung kolom IX Jumlah dari VII - VIII.
func TestBuildForm17_00KolomBarisDanJumlah(t *testing.T) {
	kantor := ReportingOffice{Sandi: "1101"}
	sec := BuildForm17_00(propertiFormRows(), kantor)

	if sec.Form != "17.00" {
		t.Fatalf("form = %q, ingin 17.00", sec.Form)
	}
	if len(sec.Columns) != 10 {
		t.Fatalf("jumlah kolom = %d, ingin 10 (Sandi Kantor + II..X)", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns[0])
	}
	if sec.Columns[9].Sandi != "X" || sec.Columns[9].Nama != "Metode Pengukuran" {
		t.Fatalf("kolom terakhir = %+v, ingin X Metode Pengukuran", sec.Columns[9])
	}
	// Tidak ada baris JUMLAH: jumlah baris sama dengan jumlah properti.
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2 (tanpa baris JUMLAH)", len(sec.Rows))
	}
	for _, r := range sec.Rows {
		if r.Key == "JUMLAH" {
			t.Fatal("Form 17.00 tidak boleh punya baris JUMLAH")
		}
	}

	first := rowByKey(t, sec, "PRP-001")
	for sandi, want := range map[string]string{
		"I": "1101", "II": "PRP-001", "III": "1", "IV": "Jl. Contoh No. 1",
		"V": "-6.2, 106.8", "VI": "01-06-2025", "VII": "1000", "VIII": "200",
		"IX": "800", "X": "1",
	} {
		if got := cellValue(first, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	if first.Reason != "" {
		t.Fatalf("baris lengkap tidak boleh beralasan: %q", first.Reason)
	}

	// Koordinat kosong ditulis "-", bukan dikarang.
	kedua := rowByKey(t, sec, "PRP-002")
	if got := cellValue(kedua, "V"); got != "-" {
		t.Errorf("koordinat kosong = %q, ingin -", got)
	}
	if got := cellValue(kedua, "IX"); got != "4500" {
		t.Errorf("jumlah PRP-002 = %q, ingin 4500", got)
	}
}

// Bila akumulasi melebihi biaya, kolom IX ditulis "-" beralasan; tidak ada angka
// negatif yang menyesatkan.
func TestBuildForm17_00JumlahTidakValid(t *testing.T) {
	row := propertiFormRows()[0]
	row.AkumulasiPenyusutanAmortisasi = decimal.NewFromInt(1_500)
	sec := BuildForm17_00([]domain.PropertiItem{row}, ReportingOffice{Sandi: "1101"})

	baris := rowByKey(t, sec, "PRP-001")
	if got := cellValue(baris, "IX"); got != "-" {
		t.Fatalf("jumlah tidak valid = %q, ingin -", got)
	}
	if baris.Reason == "" {
		t.Fatal("baris tidak valid harus beralasan")
	}
}

// Tanpa kantor pelapor yang pasti, kolom I didaftarkan sebagai tidak tersedia beserta
// alasannya, bukan diisi "-" tanpa alasan.
func TestBuildForm17_00SandiKantorTidakTersedia(t *testing.T) {
	sec := BuildForm17_00(propertiFormRows(), ReportingOffice{Reason: "kantor belum dikonfigurasi"})
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I harus terdaftar tidak tersedia bila kantor tidak pasti")
	}
	for _, c := range sec.Columns {
		if c.Sandi == "I" {
			t.Fatal("kolom I tidak boleh menjadi kolom bernilai bila kantor tidak pasti")
		}
	}
}

// Register kosong tetap membawa daftar kolom (bukan baris kosong palsu).
func TestBuildForm17_00KosongTetapBawaKolom(t *testing.T) {
	sec := BuildForm17_00(nil, ReportingOffice{Sandi: "1101"})
	if len(sec.Rows) != 0 {
		t.Fatalf("register kosong tidak boleh punya baris, dapat %d", len(sec.Rows))
	}
	if len(sec.Columns) != 10 {
		t.Fatalf("daftar kolom tetap dibawa: %d, ingin 10", len(sec.Columns))
	}
}

// Catatan form menyebut aturan no reuse/no recycle dan tidak adanya baris JUMLAH.
func TestBuildForm17_00Catatan(t *testing.T) {
	sec := BuildForm17_00(nil, ReportingOffice{Sandi: "1101"})
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "no reuse/no recycle") {
		t.Fatalf("catatan harus menyebut aturan no reuse/no recycle: %q", gabung)
	}
	if !strings.Contains(gabung, "TIDAK ada baris JUMLAH") {
		t.Fatalf("catatan harus menyebut ketiadaan baris JUMLAH: %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 17.00 muncul di bundel bulanan dengan
// barisnya per properti dan tanpa baris JUMLAH.
func TestBuilderForm17_00LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source:   newStubSource(),
		Properti: propertiOJKRepoStub{rows: propertiFormRows()},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "17.00")
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(sec.Rows))
	}
	// Stub sumber tidak menyediakan kantor pelapor, jadi kolom I tidak dikarang:
	// didaftarkan sebagai tidak tersedia dan kolom pertama yang bernilai adalah II.
	if sec.Columns[0].Sandi != "II" {
		t.Fatalf("kolom pertama = %+v, ingin II (Sandi Kantor tidak tersedia tanpa bank_offices)", sec.Columns[0])
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Register kosong tidak ditampilkan sebagai tabel kosong; form dicatat pada SkippedForms
// dengan alasan spesifik.
func TestBuilderForm17_00RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), Properti: propertiOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "17.00" {
			t.Fatal("Form 17.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "17.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 17.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 17.00 kosong harus tercatat pada SkippedForms")
}
