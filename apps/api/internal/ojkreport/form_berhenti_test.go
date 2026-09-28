package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Form 00.09/00.10/00.12 adalah form KONDISIONAL berbasis PERISTIWA: baris muncul
// hanya bila tanggal peristiwanya jatuh pada bulan periode laporan (ended_at untuk
// manajemen, closed_at untuk kantor). Uji di bawah memakai perakit murni dan jalur
// bulanan lewat kelembagaanStubSource.

func tanggal(t *testing.T, s string) *time.Time {
	t.Helper()
	d := mustDate(t, s)
	return &d
}

// (a) Peristiwa pada bulan periode -> baris muncul dengan kolom yang benar.
func TestBuildForm00_09PeristiwaDalamBulan(t *testing.T) {
	rows := []domain.BankManagement{{
		Category:        domain.KelembagaanKategoriDireksi,
		Name:            "Budi",
		OJKPositionCode: "110",
		LicenseNumber:   "SR-1",
		StartedAt:       tanggal(t, "2020-01-01"),
		EndedAt:         tanggal(t, "2026-03-10"),
		Note:            "mengundurkan diri",
	}}
	sec, ada := buildForm00_09(rows, timeMarch2026())
	if !ada || len(sec.Rows) != 1 {
		t.Fatalf("baris Form 00.09 = %d (ada=%v), ingin 1", len(sec.Rows), ada)
	}
	cases := map[string]string{
		"I":   "Budi",
		"III": "110",
		"IV":  "2020-01-01",
		"IX":  "2026-03-10",
		"X":   "mengundurkan diri",
	}
	for sandi, want := range cases {
		if got := findCell(t, sec, "Budi", sandi).Value; got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
}

// (b) Peristiwa di luar bulan periode -> tidak muncul sebagai baris.
func TestBuildForm00_09PeristiwaLuarBulan(t *testing.T) {
	rows := []domain.BankManagement{
		{Category: domain.KelembagaanKategoriKomisaris, Name: "Ani", EndedAt: tanggal(t, "2026-02-10")},
		{Category: domain.KelembagaanKategoriKomisaris, Name: "Ana", EndedAt: tanggal(t, "2026-04-01")},
		{Category: domain.KelembagaanKategoriKomisaris, Name: "Belum Berhenti"},
	}
	if sec, ada := buildForm00_09(rows, timeMarch2026()); ada || len(sec.Rows) != 0 {
		t.Fatalf("peristiwa luar bulan harus tidak muncul, dapat %d baris (ada=%v)", len(sec.Rows), ada)
	}
}

// (d) NIK dan kolom tanpa sumber didaftarkan belum tersedia (ditulis "-" + alasan),
// bukan diisi angka/teks.
func TestBuildForm00_09KolomTanpaSumberBertanda(t *testing.T) {
	rows := []domain.BankManagement{{
		Category: domain.KelembagaanKategoriDireksi, Name: "Budi",
		EndedAt: tanggal(t, "2026-03-10"),
	}}
	sec, _ := buildForm00_09(rows, timeMarch2026())
	for _, sandi := range []string{"II", "V.1", "V.4", "VI", "VII", "VIII"} {
		u := unavailableColumn(t, sec, sandi)
		if strings.TrimSpace(u.Reason) == "" {
			t.Errorf("kolom %s harus mencantumkan alasan", sandi)
		}
	}
	// NIK tidak boleh muncul sebagai kolom terisi.
	for _, c := range sec.Columns {
		if c.Sandi == "II" {
			t.Fatal("NIK tidak boleh menjadi kolom bernilai")
		}
	}
}

// (e) note teks bebas tidak pernah menjadi sandi penyebab: kolom penyebab (VIII)
// tetap tidak tersedia, sedangkan note hanya menjadi kolom X Alasan.
func TestNoteTidakMenjadiSandiPenyebab(t *testing.T) {
	rows := []domain.BankManagement{{
		Category: domain.KelembagaanKategoriKomisaris, Name: "Ani",
		EndedAt: tanggal(t, "2026-03-11"), Note: "Meninggal dunia",
	}}
	sec, _ := buildForm00_09(rows, timeMarch2026())
	if !punyaUnavailable(sec.Unavailable, "VIII") {
		t.Fatal("sandi penyebab berhenti (VIII) harus tetap tidak tersedia")
	}
	alasan := findCell(t, sec, "Ani", "X").Value
	if alasan != "Meninggal dunia" {
		t.Fatalf("kolom X Alasan = %q, ingin note apa adanya", alasan)
	}
	if alasan == "3" {
		t.Fatal("note tidak boleh diterjemahkan menjadi sandi penyebab")
	}
	for _, c := range findCellRow(t, sec, "Ani").Cells {
		if c.Sandi == "VIII" {
			t.Fatalf("note menjadi sandi penyebab: %q", c.Value)
		}
	}
}

func TestBuildForm00_10PeristiwaDalamBulan(t *testing.T) {
	rows := []domain.BankManagement{{
		Category:      domain.KelembagaanKategoriPejabatEksekutif,
		Name:          "Citra",
		LicenseNumber: "SR-ANGKAT-9",
		LicenseDate:   tanggal(t, "2019-05-01"),
		StartedAt:     tanggal(t, "2019-06-01"),
		EndedAt:       tanggal(t, "2026-03-20"),
		Note:          "pemberhentian",
	}}
	sec, ada := buildForm00_10(rows, timeMarch2026())
	if !ada || len(sec.Rows) != 1 {
		t.Fatalf("baris Form 00.10 = %d (ada=%v), ingin 1", len(sec.Rows), ada)
	}
	cases := map[string]string{
		"I":    "Citra",
		"IV":   "2019-06-01",
		"V.1":  "SR-ANGKAT-9",
		"V.2":  "2019-05-01",
		"VIII": "pemberhentian",
	}
	for sandi, want := range cases {
		if got := findCell(t, sec, "Citra", sandi).Value; got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	// Surat Pemberhentian (IX) tidak boleh memakai ulang license_*.
	if u := unavailableColumn(t, sec, "IX"); !strings.Contains(u.Reason, "PENGANGKATAN") {
		t.Errorf("alasan Surat Pemberhentian harus menyebut license_* adalah pengangkatan, dapat: %s", u.Reason)
	}
	for _, sandi := range []string{"II", "III.1", "III.5", "VI.1", "VI.4", "VII"} {
		unavailableColumn(t, sec, sandi)
	}
}

func TestBuildForm00_10PeristiwaLuarBulanDanKategoriLain(t *testing.T) {
	rows := []domain.BankManagement{
		{Category: domain.KelembagaanKategoriPejabatEksekutif, Name: "Luar", EndedAt: tanggal(t, "2026-01-05")},
		{Category: domain.KelembagaanKategoriDireksi, Name: "Direksi", EndedAt: tanggal(t, "2026-03-05")},
	}
	if sec, ada := buildForm00_10(rows, timeMarch2026()); ada || len(sec.Rows) != 0 {
		t.Fatalf("Form 00.10 hanya direksi/luar bulan, dapat %d baris (ada=%v)", len(sec.Rows), ada)
	}
}

func TestBuildForm00_12PeristiwaDalamBulan(t *testing.T) {
	rows := []domain.BankOffice{{
		OfficeType:       "KANTOR_CABANG",
		Code:             "KC-01",
		Name:             "Kantor Cabang Uji",
		Address:          "Jl. Uji 1",
		OJKKabupatenCode: "0197",
		ClosedAt:         tanggal(t, "2026-03-15"),
		Status:           domain.KelembagaanKantorTutup,
	}}
	sec, ada := buildForm00_12(rows, timeMarch2026())
	if !ada || len(sec.Rows) != 1 {
		t.Fatalf("baris Form 00.12 = %d (ada=%v), ingin 1", len(sec.Rows), ada)
	}
	cases := map[string]string{
		"II":   "KC-01",
		"IV":   "Kantor Cabang Uji",
		"VI":   "Jl. Uji 1",
		"VII":  "2026-03-15",
		"VIII": "0197",
	}
	for sandi, want := range cases {
		if got := findCell(t, sec, "KC-01", sandi).Value; got != want {
			t.Errorf("kolom %s = %q, ingin %q", sandi, got, want)
		}
	}
	for _, sandi := range []string{"I", "III", "V"} {
		unavailableColumn(t, sec, sandi)
	}
}

func TestBuildForm00_12PeristiwaLuarBulan(t *testing.T) {
	rows := []domain.BankOffice{{Code: "KC-02", Name: "Lama", ClosedAt: tanggal(t, "2026-02-15")}}
	if sec, ada := buildForm00_12(rows, timeMarch2026()); ada || len(sec.Rows) != 0 {
		t.Fatalf("penutupan luar bulan harus tidak muncul, dapat %d baris (ada=%v)", len(sec.Rows), ada)
	}
}

// (d)/(6) Baris TPE mengosongkan Nama dan Alamat karena aturan form, bukan karena
// sumber tidak ada; sandi Jenis OJK tetap tidak dikarang dari office_type.
func TestBuildForm00_12TPEMengosongkanNamaAlamat(t *testing.T) {
	rows := []domain.BankOffice{{
		OfficeType: "TERMINAL_PERBANKAN_ELEKTRONIK", Code: "TPE-1",
		Name: "ATM Uji", Address: "Jl. TPE", ClosedAt: tanggal(t, "2026-03-01"),
	}}
	sec, ada := buildForm00_12(rows, timeMarch2026())
	if !ada {
		t.Fatal("baris TPE harus tetap muncul")
	}
	if got := findCell(t, sec, "TPE-1", "IV").Value; got != "-" {
		t.Errorf("Nama TPE = %q, ingin \"-\"", got)
	}
	if got := findCell(t, sec, "TPE-1", "VI").Value; got != "-" {
		t.Errorf("Alamat TPE = %q, ingin \"-\"", got)
	}
	if r := rowByKey(t, sec, "TPE-1"); !strings.Contains(r.Reason, "aturan form") {
		t.Errorf("alasan kolom TPE harus menyebut aturan form, dapat: %s", r.Reason)
	}
	if u := unavailableColumn(t, sec, "I"); !strings.Contains(u.Reason, "teks bebas") {
		t.Errorf("sandi Jenis OJK harus ditandai tidak dikarang dari office_type: %s", u.Reason)
	}
}

// (c) Tanpa peristiwa pada bulan periode, tidak ada tabel, tetapi ada entri
// SkippedForms ber-alasan spesifik untuk ketiga form.
func TestBuildKelembagaanPeristiwaTanpaPeristiwaDicatatSkipped(t *testing.T) {
	report := domain.KelembagaanReport{
		Management: []domain.BankManagement{
			{Category: domain.KelembagaanKategoriDireksi, Name: "Budi", EndedAt: tanggal(t, "2026-01-10")},
			{Category: domain.KelembagaanKategoriPejabatEksekutif, Name: "Citra", EndedAt: tanggal(t, "2026-04-10")},
		},
		Offices: []domain.BankOffice{
			{Code: "KC-01", Name: "Kantor", ClosedAt: tanggal(t, "2026-02-10")},
		},
	}
	tables, skipped := buildKelembagaanPeristiwaTables(report, timeMarch2026())
	if len(tables) != 0 {
		t.Fatalf("tanpa peristiwa tidak boleh ada tabel, dapat %d", len(tables))
	}
	want := map[string]bool{"00.09": false, "00.10": false, "00.12": false}
	for _, f := range skipped {
		if _, ok := want[f.Form]; !ok {
			continue
		}
		if !strings.Contains(f.UnavailableReason, "peristiwa") || !strings.Contains(f.UnavailableReason, "periode ini") {
			t.Errorf("alasan Form %s harus spesifik, dapat: %q", f.Form, f.UnavailableReason)
		}
		want[f.Form] = true
	}
	for form, found := range want {
		if !found {
			t.Errorf("Form %s tanpa peristiwa harus dicatat belum dibangun", form)
		}
	}
}

// (c) Jalur bulanan lewat KelembagaanSource: tanpa peristiwa, Form 00.09/00.10/00.12
// tidak tampil sebagai tabel kosong melainkan tercatat SkippedForms ber-alasan.
func TestGenerateMonthlyPeristiwaKosongDicatatSkipped(t *testing.T) {
	src := &kelembagaanStubSource{
		stubSource: newStubSource(),
		report: domain.KelembagaanReport{
			Management: []domain.BankManagement{
				{ID: uuid.New(), Category: domain.KelembagaanKategoriDireksi, Name: "Budi", EndedAt: tanggal(t, "2026-01-10")},
			},
		},
	}
	b, err := NewBuilder(src).GenerateMonthly(context.Background(),
		time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	for _, sec := range b.Tables {
		if sec.Form == "00.09" || sec.Form == "00.10" || sec.Form == "00.12" {
			t.Errorf("Form %s tidak boleh tampil sebagai tabel tanpa peristiwa", sec.Form)
		}
	}
	want := map[string]bool{"00.09": false, "00.10": false, "00.12": false}
	for _, f := range b.SkippedForms {
		if _, ok := want[f.Form]; !ok {
			continue
		}
		if f.UnavailableReason == "" {
			t.Errorf("Form %s harus punya alasan spesifik", f.Form)
		}
		want[f.Form] = true
	}
	for form, found := range want {
		if !found {
			t.Errorf("Form %s harus tercatat belum dibangun", form)
		}
	}
}

// (a) Jalur bulanan: peristiwa pada bulan periode melahirkan tabel Form 00.09/00.10/
// 00.12 dengan kolom terisi dan kolom tanpa sumber bertanda.
func TestGenerateMonthlyPeristiwaTerisi(t *testing.T) {
	src := &kelembagaanStubSource{
		stubSource: newStubSource(),
		report: domain.KelembagaanReport{
			Management: []domain.BankManagement{
				{ID: uuid.New(), Category: domain.KelembagaanKategoriDireksi, Name: "Budi",
					OJKPositionCode: "110", EndedAt: tanggal(t, "2026-03-10"), Note: "berhenti"},
				{ID: uuid.New(), Category: domain.KelembagaanKategoriPejabatEksekutif, Name: "Citra",
					LicenseNumber: "SR-9", EndedAt: tanggal(t, "2026-03-20")},
			},
			Offices: []domain.BankOffice{
				{ID: uuid.New(), OfficeType: "KANTOR_CABANG", Code: "KC-01", Name: "Cabang",
					ClosedAt: tanggal(t, "2026-03-15")},
			},
		},
	}
	b, err := NewBuilder(src).GenerateMonthly(context.Background(),
		time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	if got := findCell(t, tableByForm(t, b, "00.09"), "Budi", "III").Value; got != "110" {
		t.Errorf("Form 00.09 III = %q, ingin 110", got)
	}
	if got := findCell(t, tableByForm(t, b, "00.10"), "Citra", "V.1").Value; got != "SR-9" {
		t.Errorf("Form 00.10 V.1 = %q, ingin SR-9", got)
	}
	if got := findCell(t, tableByForm(t, b, "00.12"), "KC-01", "VII").Value; got != "2026-03-15" {
		t.Errorf("Form 00.12 VII = %q, ingin 2026-03-15", got)
	}
	for _, f := range b.SkippedForms {
		if f.Form == "00.09" || f.Form == "00.10" || f.Form == "00.12" {
			t.Errorf("Form %s berisi peristiwa, tidak boleh tercatat belum dibangun: %s", f.Form, f.UnavailableReason)
		}
	}
}

func findCellRow(t *testing.T, sec TableSection, key string) TableRow {
	t.Helper()
	return rowByKey(t, sec, key)
}
