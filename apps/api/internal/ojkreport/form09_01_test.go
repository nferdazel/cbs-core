package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// adaTable dan adaSkipped dipakai bersama oleh uji Form 09.01/14.01 (lihat juga
// form14_01_test.go).
func adaTable(b *Bundle, form string) bool {
	for _, sec := range b.Tables {
		if sec.Form == form {
			return true
		}
	}
	return false
}

func adaSkipped(b *Bundle, form string) bool {
	for _, f := range b.SkippedForms {
		if f.Form == form {
			return true
		}
	}
	return false
}

// buildDenganRows menyusun bundel bulanan dengan baris neraca tambahan; baris stok
// bawaan newStubSource tidak memetakan ke pos Aset/Liabilitas Lainnya sehingga tidak
// memengaruhi ambang Form 09.01/14.01.
func buildDenganRows(t *testing.T, rows ...domain.ReportRow) *Bundle {
	t.Helper()
	src := newStubSource()
	src.bs.Rows = append(src.bs.Rows, rows...)
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	return b
}

// (a) Tidak terpicu: form memang tidak berlaku untuk periode itu, jadi tidak muncul di
// Tables dan tidak dicatat sebagai "TIDAK DIBANGUN" di SkippedForms.
func TestForm09_01TidakTerbitBilaTidakTerpicu(t *testing.T) {
	b := buildDenganRows(t,
		row("10305", "Piutang Denda", 100),
		row("10310", "Premi Penjaminan LPS Dibayar di Muka", 300),
	)
	if adaTable(b, "09.01") {
		t.Fatal("Form 09.01 tidak boleh terbit bila pos Lainnya tidak melebihi 25%")
	}
	if adaSkipped(b, "09.01") {
		t.Fatal("form kondisional yang tidak berlaku tidak boleh ditulis TIDAK DIBANGUN")
	}
}

// (b) Terpicu: baris satu per akun COA yang dipetakan ke pos Lainnya, plus JUMLAH.
func TestForm09_01TerbitDenganBarisPerAkun(t *testing.T) {
	b := buildDenganRows(t,
		row("10305", "Piutang Denda", 500),
		row("10999", "Akun Sementara", 300),
		row("11700", "Piutang Denda Syariah", 200),
	)
	sec := tableByForm(t, b, "09.01")
	if len(sec.Rows) != 4 { // tiga akun + JUMLAH
		t.Fatalf("baris Form 09.01 = %d, ingin 4", len(sec.Rows))
	}
	cases := []struct{ code, uraian, jumlah string }{
		{"10305", "Piutang Denda", "500"},
		{"10999", "Akun Sementara", "300"},
		{"11700", "Piutang Denda Syariah", "200"},
	}
	for _, c := range cases {
		if got := cellValue(rowByKey(t, sec, c.code), "II"); got != c.uraian {
			t.Errorf("uraian %s = %q, ingin %q", c.code, got, c.uraian)
		}
		if got := findCell(t, sec, c.code, "III").Value; got != c.jumlah {
			t.Errorf("jumlah %s = %q, ingin %q", c.code, got, c.jumlah)
		}
	}
	if got := findCell(t, sec, form09_01TotalKey, "III").Value; got != "1000" {
		t.Errorf("JUMLAH Form 09.01 = %q, ingin 1000", got)
	}
}

// (c) Pemicu memakai penyebut pos 1299000000 Form 01.00 dan memakai perbandingan
// ketat: persis 25% tidak memicu, sedikit di atasnya memicu.
func TestForm09_01PemicuKetikKetatPada25Persen(t *testing.T) {
	tepat := buildDenganRows(t,
		row("10305", "Piutang Denda", 100),
		row("10310", "Premi Penjaminan LPS Dibayar di Muka", 300),
	)
	if adaTable(tepat, "09.01") {
		t.Fatal("pos Lainnya tepat 25% dari jumlah aset lainnya tidak boleh memicu (harus >, bukan >=)")
	}

	lebih := buildDenganRows(t,
		row("10305", "Piutang Denda", 101),
		row("10310", "Premi Penjaminan LPS Dibayar di Muka", 299),
	)
	if !adaTable(lebih, "09.01") {
		t.Fatal("pos Lainnya di atas 25% harus memicu Form 09.01")
	}
}

// (d) Kolom III Jumlah Form 09.01 harus sama dengan pos Lainnya (1299990000) Form 09.00.
func TestForm09_01TotalSamaDenganPosLainnyaForm09(t *testing.T) {
	b := buildDenganRows(t,
		row("10305", "Piutang Denda", 500),
		row("10999", "Akun Sementara", 300),
		row("11700", "Piutang Denda Syariah", 200),
	)
	posLainnya := form09Value(t, tableByForm(t, b, "09.00"), "1299990000")
	total := findCell(t, tableByForm(t, b, "09.01"), form09_01TotalKey, "III").Value
	if total != FormatRupiah(posLainnya) {
		t.Fatalf("total Form 09.01 = %s, ingin sama dengan 1299990000 Form 09.00 = %s",
			total, FormatRupiah(posLainnya))
	}
}

// (e) Kolom I Form 09.01 bersumber dari kantor pelapor yang sama dengan form lain.
func TestForm09_01KolomSandiKantorSamaSumbernya(t *testing.T) {
	base := newStubSource()
	base.bs.Rows = append(base.bs.Rows,
		row("10305", "Piutang Denda", 500),
		row("10999", "Akun Sementara", 300),
		row("11700", "Piutang Denda Syariah", 200),
	)
	src := RepoSource{
		Source: base,
		Kelembagaan: kelembagaanRepoStub{offices: []domain.BankOffice{
			{Code: "001", Name: "Kantor Pusat", Status: domain.KelembagaanKantorAktif},
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "09.01")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Rows) == 0 {
		t.Fatal("Form 09.01 tidak punya baris")
	}
	for _, r := range sec.Rows {
		if got := cellValue(r, "I"); got != "001" {
			t.Errorf("baris %s kolom I = %q, ingin 001", r.Key, got)
		}
	}
}

// Terpicu tetapi ada akun tanpa saldo: baris ditulis "-" beralasan dan JUMLAH ditulis
// "-" agar angka sebagian tidak disalahartikan sebagai total.
func TestBuildForm09_01AkunTanpaSaldoDitulisMinus(t *testing.T) {
	sec := buildForm09_01([]rincianLainnya{{COACode: "10999"}},
		ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})
	baris := rowByKey(t, sec, "10999")
	if baris.Reason == "" {
		t.Fatal("baris akun tanpa saldo harus mencantumkan alasan")
	}
	if got := findCell(t, sec, "10999", "III").Value; got != "-" {
		t.Errorf("jumlah akun tanpa saldo = %q, ingin -", got)
	}
	total := rowByKey(t, sec, form09_01TotalKey)
	if total.Reason == "" {
		t.Fatal("JUMLAH harus beralasan saat masih ada akun tanpa saldo")
	}
	if got := findCell(t, sec, form09_01TotalKey, "III").Value; got != "-" {
		t.Errorf("JUMLAH = %q, ingin -", got)
	}
}
