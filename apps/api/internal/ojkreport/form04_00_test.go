package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// suratBerhargaOJKRepoStub mengimplementasikan domain.SuratBerhargaRegisterRepository
// untuk menguji Form 04.00 lewat RepoSource tanpa basis data.
type suratBerhargaOJKRepoStub struct {
	domain.SuratBerhargaRegisterRepository
	rows []domain.SuratBerhargaItem
	err  error
}

func (s suratBerhargaOJKRepoStub) ListSuratBerhargaForOJK(context.Context, time.Time) ([]domain.SuratBerhargaItem, error) {
	return s.rows, s.err
}

var _ domain.SuratBerhargaRegisterRepository = suratBerhargaOJKRepoStub{}

func suratBerhargaFormItem(isin string, nominal int64) domain.SuratBerhargaItem {
	return domain.SuratBerhargaItem{
		KlasifikasiCode:                  domain.SuratBerhargaKlasifikasiTersediaUntukDijual,
		SukuBunga:                        decimal.NewFromInt(7),
		TanggalMulai:                     time.Date(2025, time.June, 1, 0, 0, 0, 0, time.UTC),
		TanggalJatuhTempo:                time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC),
		Nominal:                          decimal.NewFromInt(nominal),
		NominalDijaminkan:                decimal.NewFromInt(100),
		BiayaPerolehan:                   decimal.NewFromInt(200),
		DiskontoPremiumBelumDiamortisasi: decimal.NewFromInt(-10),
		BiayaTransaksiBelumDiamortisasi:  decimal.NewFromInt(5),
		LabaRugiBelumDirealisasi:         decimal.NewFromInt(3),
		BiayaPerolehanDiamortisasi:       decimal.NewFromInt(195),
		NomorSuratBerharga:               isin,
		CounterpartyID:                   "PL-001",
		JenisCode:                        domain.SuratBerhargaJenisPemerintah,
		KualitasCode:                     domain.SuratBerhargaKualitasLancar,
		CKPN:                             decimal.NewFromInt(1),
		LembagaPemeringkatCode:           domain.SuratBerhargaLembagaPemeringkatTanpaPeringkat,
		PeringkatSuratBerhargaCode:       domain.SuratBerhargaPeringkatTanpaPeringkat,
		TanggalPenerbitan:                time.Date(2025, time.May, 1, 0, 0, 0, 0, time.UTC),
		CKPNAsetBaik:                     decimal.NewFromInt(1),
		KlasifikasiAsetKeuanganCode:      domain.SuratBerhargaKlasifikasiAsetBiayaPerolehan,
		JenisCKPNCode:                    domain.SuratBerhargaJenisCKPNKolektif,
	}
}

// Form 04.00 menampilkan satu baris per surat berharga DITAMBAH satu baris JUMLAH, dan
// kolom angka pada JUMLAH dijumlahkan.
func TestBuildForm04_00AdaJumlahDanNilaiApaAdanya(t *testing.T) {
	rows := []domain.SuratBerhargaItem{
		suratBerhargaFormItem("ID0000000001", 1_000),
		suratBerhargaFormItem("ID0000000002", 2_000),
	}
	sec := BuildForm04_00(rows, ReportingOffice{Sandi: "1101"})

	if len(sec.Rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3 (2 surat berharga + JUMLAH)", len(sec.Rows))
	}
	row := rowByKey(t, sec, "ID0000000001")
	for sandi, want := range map[string]string{
		"I": "1101", "II": "1", "III": "7.00", "IV": "01-06-2025 s.d. 01-06-2026",
		"V": "1000", "VI": "100", "VII": "200", "VIII": "-10", "IX": "5", "X": "3",
		"XI": "195", "XII": "ID0000000001", "XIII": "PL-001", "XIV": "2", "XV": "1",
		"XVI": "1", "XVII": "9", "XVIII": "99", "XIX": "-", "XX": "01-05-2025",
		"XXI": "1", "XXII": "0", "XXIII": "0", "XXIV": "3", "XXV": "2",
	} {
		if got := cellValue(row, sandi); got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}

	jumlah := rowByKey(t, sec, "JUMLAH")
	if got := cellValue(jumlah, "II"); got != "JUMLAH" {
		t.Fatalf("baris JUMLAH kolom II = %q", got)
	}
	if got := cellValue(jumlah, "V"); got != "3000" {
		t.Fatalf("JUMLAH kolom V = %q, ingin 3000", got)
	}
	if got := cellValue(jumlah, "III"); got != "14.00" {
		t.Fatalf("JUMLAH kolom III = %q, ingin 14.00", got)
	}
	if got := cellValue(jumlah, "XII"); got != "-" {
		t.Fatalf("JUMLAH kolom teks XII = %q, ingin -", got)
	}
}

// Tanpa registri, Form 04.00 tetap membawa baris JUMLAH dengan sel "-" dan tidak
// mengarang total.
func TestBuildForm04_00KosongTetapBawaJumlah(t *testing.T) {
	sec := BuildForm04_00(nil, ReportingOffice{Sandi: "1101"})
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1 (JUMLAH kosong)", len(sec.Rows))
	}
	jumlah := rowByKey(t, sec, "JUMLAH")
	if got := cellValue(jumlah, "V"); got != "-" {
		t.Fatalf("JUMLAH tanpa baris kolom V = %q, ingin -", got)
	}
}

// Tata kelola kolom: 25 kolom (I + II..XXV) dibawa; tanpa kantor pasti kolom I didaftarkan
// tidak tersedia.
func TestBuildForm04_00KolomDanSandiKantor(t *testing.T) {
	sec := BuildForm04_00(
		[]domain.SuratBerhargaItem{suratBerhargaFormItem("ID0000000001", 100)},
		ReportingOffice{Sandi: "1101"})
	if len(sec.Columns) != 25 {
		t.Fatalf("jumlah kolom = %d, ingin 25 (I + II..XXV)", len(sec.Columns))
	}
	if sec.Columns[0].Sandi != "I" || sec.Columns[24].Sandi != "XXV" {
		t.Fatalf("kolom pertama/terakhir = %+v/%+v", sec.Columns[0], sec.Columns[24])
	}

	tanpaKantor := BuildForm04_00(
		[]domain.SuratBerhargaItem{suratBerhargaFormItem("ID0000000001", 100)},
		ReportingOffice{Reason: "kantor belum dikonfigurasi"})
	if !punyaUnavailable(tanpaKantor.Unavailable, "I") {
		t.Fatal("tanpa kantor pelapor, kolom I harus terdaftar tidak tersedia")
	}
}

// Catatan form menyebut baris JUMLAH dan bahwa nilai adalah isian bank.
func TestBuildForm04_00Catatan(t *testing.T) {
	sec := BuildForm04_00(nil, ReportingOffice{Sandi: "1101"})
	gabung := strings.Join(sec.Notes, " ")
	if !strings.Contains(gabung, "JUMLAH") {
		t.Fatalf("catatan harus menyebut baris JUMLAH: %q", gabung)
	}
	if !strings.Contains(gabung, "isian bank") {
		t.Fatalf("catatan harus menyatakan nilai adalah isian bank: %q", gabung)
	}
}

// Lewat RepoSource dengan register terisi, Form 04.00 muncul di bundel bulanan.
func TestBuilderForm04_00LewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		SuratBerharga: suratBerhargaOJKRepoStub{rows: []domain.SuratBerhargaItem{
			suratBerhargaFormItem("ID0000000001", 1_000),
			suratBerhargaFormItem("ID0000000002", 2_000),
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "04.00")
	if len(sec.Rows) != 3 {
		t.Fatalf("jumlah baris = %d, ingin 3 (2 + JUMLAH)", len(sec.Rows))
	}
}

// Register kosong tidak ditampilkan sebagai tabel (termasuk JUMLAH kosong); form dicatat
// pada SkippedForms dengan alasan spesifik.
func TestBuilderForm04_00RegisterKosongDicatatSkipped(t *testing.T) {
	src := RepoSource{Source: newStubSource(), SuratBerharga: suratBerhargaOJKRepoStub{}}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "04.00" {
			t.Fatal("Form 04.00 tidak boleh tampil sebagai tabel saat register kosong")
		}
	}
	for _, f := range b.SkippedForms {
		if f.Form == "04.00" {
			if f.UnavailableReason == "" {
				t.Fatal("Form 04.00 kosong harus punya alasan spesifik")
			}
			return
		}
	}
	t.Fatal("Form 04.00 kosong harus tercatat pada SkippedForms")
}
