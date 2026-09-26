package ojkreport

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatalf("tanggal uji %q: %v", s, err)
	}
	return d
}

func timeMarch2026() time.Time {
	return time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
}

// findCell mencari nilai satu kolom pada satu baris tabel berdasarkan kunci baris.
func findCell(t *testing.T, sec TableSection, key, sandi string) TableCell {
	t.Helper()
	for _, row := range sec.Rows {
		if row.Key != key {
			continue
		}
		for _, c := range row.Cells {
			if c.Sandi == sandi {
				return c
			}
		}
		t.Fatalf("kolom %s pada baris %s tidak ditemukan", sandi, key)
	}
	t.Fatalf("baris %s tidak ditemukan", key)
	return TableCell{}
}

func unavailableColumn(t *testing.T, sec TableSection, sandi string) ColumnUnavailable {
	t.Helper()
	for _, u := range sec.Unavailable {
		if u.Sandi == sandi {
			return u
		}
	}
	t.Fatalf("kolom %s tidak terdaftar sebagai tidak tersedia", sandi)
	return ColumnUnavailable{}
}

func rowByKey(t *testing.T, sec TableSection, key string) TableRow {
	t.Helper()
	for _, r := range sec.Rows {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("baris %s tidak ditemukan", key)
	return TableRow{}
}

// Form 06.00 mengisi hanya kolom yang bersumber dari baris kredit dan mendaftarkan
// kolom yang tidak punya sumber, bukan mengisinya nol.
func TestBuildForm06MengisiKolomTersediaDanMenandaiYangTidak(t *testing.T) {
	akad := mustDate(t, "2025-01-15")
	jatuh := mustDate(t, "2027-01-15")
	rows := []LoanRow{{
		Status: "DISBURSED", BranchCode: "001", LoanNumber: "LN-001",
		Collectibility: "3_KURANG_LANCAR", DPD: 45,
		Outstanding: decimal.NewFromInt(5_000_000), RequiredCKPN: decimal.NewFromInt(250_000),
		IsRestructured: true, RestructuredCount: 2,
		InterestRateAnnual: decimal.RequireFromString("12.5"),
		PrincipalAmount:    decimal.NewFromInt(8_000_000),
		AkadDate:           &akad, FinalDueDate: &jatuh,
	}}

	sec := buildForm06(rows)
	if sec.Form != "06.00" {
		t.Fatalf("form = %s", sec.Form)
	}
	cases := []struct{ sandi, want string }{
		{form06SandiKantor, "001"},
		{form06SandiRestruktur, "21"}, // restrukturisasi ke-2
		{form06SandiKualitas, "3"},    // kurang lancar
		{form06SandiHariTunggakan, "45"},
		{form06SandiBakiDebet, "5000000"},
		{form06SandiCKPN, "250000"},
		{form06SandiSukuBunga, "12.50"},
		{form06SandiAkadAwal, "2025-01-15"},
		{form06SandiJangkaWaktu, "2027-01-15"},
		{form06SandiPlafon, "8000000"},
		// Sandi referensi OJK kosong ditulis "-", bukan dikosongkan atau ditebak.
		{form06SandiJenisDebitur, "-"},
		{form06SandiSektor, "-"},
	}
	for _, c := range cases {
		got := findCell(t, sec, "LN-001", c.sandi)
		if got.Value != c.want {
			t.Errorf("kolom %s = %q, ingin %q", c.sandi, got.Value, c.want)
		}
	}

	// Kolom tanpa sumber wajib terdaftar beserta alasan, bukan tampil sebagai nol.
	for _, sandi := range []string{form06SandiIDPihakLawan, form06SandiNoIdentitas, form06SandiBMPK, form06SandiBakiNeto} {
		u := unavailableColumn(t, sec, sandi)
		if strings.TrimSpace(u.Reason) == "" {
			t.Errorf("kolom %s tanpa alasan", sandi)
		}
	}
}

// Kredit yang tidak punya eksposur berjalan tidak dilaporkan pada Form 06.00.
func TestBuildForm06MelewatiKreditTidakAktif(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-AKTIF", Outstanding: decimal.NewFromInt(100)},
		{Status: "PAID_OFF", LoanNumber: "LN-LUNAS", Outstanding: decimal.Zero},
		{Status: "PENDING_APPROVAL", LoanNumber: "LN-PENDING"},
	}
	sec := buildForm06(rows)
	if len(sec.Rows) != 1 || sec.Rows[0].Key != "LN-AKTIF" {
		t.Fatalf("hanya kredit aktif yang dilaporkan; rows=%+v", sec.Rows)
	}
}

// Form 05.00 memetakan jenis dan kualitas ke sandi Lampiran II dan memakai nama bank
// lawan sebagai kunci baris.
func TestBuildForm05MemetakanJenisDanKualitas(t *testing.T) {
	rows := []PlacementRow{
		{BranchCode: "001", CounterpartyBank: "Bank Uji A", PlacementType: "DEPOSITO", Collectibility: "LANCAR", Outstanding: decimal.NewFromInt(10_000_000)},
		{CounterpartyBank: "Bank Uji B", PlacementType: "GIRO", Collectibility: "MACET", Outstanding: decimal.NewFromInt(2_000_000)},
		{CounterpartyBank: "Bank Uji C", PlacementType: "KREDIT", Collectibility: "KURANG_LANCAR", Outstanding: decimal.NewFromInt(1_000_000)},
	}
	sec := buildForm05(rows)

	if got := findCell(t, sec, "Bank Uji A", form05SandiJenis).Value; got != "30" {
		t.Errorf("jenis deposito = %s, ingin 30", got)
	}
	if got := findCell(t, sec, "Bank Uji A", form05SandiKualitas).Value; got != "1" {
		t.Errorf("kualitas lancar = %s, ingin 1", got)
	}
	if got := findCell(t, sec, "Bank Uji B", form05SandiKualitas).Value; got != "5" {
		t.Errorf("kualitas macet = %s, ingin 5", got)
	}
	if got := findCell(t, sec, "Bank Uji B", form05SandiJumlah).Value; got != "2000000" {
		t.Errorf("jumlah = %s, ingin 2000000", got)
	}
	// Jenis yang tidak punya sandi pada Form 05.00 – 2 ditulis "-", bukan angka karangan.
	if got := findCell(t, sec, "Bank Uji C", form05SandiJenis).Value; got != "-" {
		t.Errorf("jenis kredit = %s, ingin -", got)
	}
	if u := unavailableColumn(t, sec, form05SandiBank); strings.TrimSpace(u.Reason) == "" {
		t.Error("kolom sandi bank harus terdaftar tidak tersedia dengan alasan")
	}
}

// Penulis berkas menuliskan baris form daftar tetap empat kolom, dan baris/kolom
// yang tidak tersedia ditulis sebagai komentar dengan alasan.
func TestWriteTextMenulisFormDaftarDanAlasan(t *testing.T) {
	b := &Bundle{
		Period:    timeMarch2026(),
		PeriodEnd: timeMarch2026(),
		Tables: []TableSection{
			{Form: "06.00", Name: "Daftar Kredit yang Diberikan", KeyLabel: "No. Rekening",
				Columns: []TableColumn{{Sandi: "XXVIII", Nama: "Baki Debet"}},
				Rows: []TableRow{{Key: "LN-001", Cells: []TableCell{
					{Sandi: "XXVIII", Nama: "Baki Debet", Value: "5000"}}}},
				Unavailable: []ColumnUnavailable{{Sandi: "III", Nama: "No. Identitas", Reason: "identitas terenkripsi"}}},
		},
	}
	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "06.00|LN-001|XXVIII Baki Debet|5000") {
		t.Fatalf("baris tabel tidak ditulis dengan format 4 kolom; keluaran=%s", out)
	}
	if !strings.Contains(out, "III No. Identitas KOLOM TIDAK TERSEDIA: identitas terenkripsi") {
		t.Fatalf("alasan kolom tidak tersedia tidak ditulis; keluaran=%s", out)
	}
	// Setiap baris data tetap konsisten 4 kolom.
	for _, line := range strings.Split(out, "\n") {
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "FORM"+ColumnSeparator) {
			continue
		}
		if n := strings.Count(line, ColumnSeparator); n != 3 {
			t.Fatalf("baris %q punya %d pemisah, ingin 3", line, n)
		}
	}
}
