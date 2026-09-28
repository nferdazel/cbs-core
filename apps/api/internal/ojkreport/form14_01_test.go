package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// (a) Tidak terpicu: form memang tidak berlaku untuk periode itu, jadi tidak muncul di
// Tables dan tidak dicatat sebagai "TIDAK DIBANGUN" di SkippedForms.
func TestForm14_01TidakTerbitBilaTidakTerpicu(t *testing.T) {
	b := buildDenganRows(t,
		row("20300", "Giro", 300),
		row("20700", "Utang Lainnya", 100),
	)
	if adaTable(b, "14.01") {
		t.Fatal("Form 14.01 tidak boleh terbit bila pos Lainnya tidak melebihi 25%")
	}
	if adaSkipped(b, "14.01") {
		t.Fatal("form kondisional yang tidak berlaku tidak boleh ditulis TIDAK DIBANGUN")
	}
}

// (b) Terpicu: baris satu per akun COA yang dipetakan ke pos Lainnya, plus JUMLAH.
func TestForm14_01TerbitDenganBarisPerAkun(t *testing.T) {
	b := buildDenganRows(t,
		row("20300", "Giro", 299),
		row("20700", "Utang Lainnya", 101),
	)
	sec := tableByForm(t, b, "14.01")
	if len(sec.Rows) != 2 { // satu akun + JUMLAH
		t.Fatalf("baris Form 14.01 = %d, ingin 2", len(sec.Rows))
	}
	if got := cellValue(rowByKey(t, sec, "20700"), "II"); got != "Utang Lainnya" {
		t.Errorf("uraian 20700 = %q, ingin Utang Lainnya", got)
	}
	if got := findCell(t, sec, "20700", "III").Value; got != "101" {
		t.Errorf("jumlah 20700 = %q, ingin 101", got)
	}
	if got := findCell(t, sec, form14_01TotalKey, "III").Value; got != "101" {
		t.Errorf("JUMLAH Form 14.01 = %q, ingin 101", got)
	}
}

// (c) Pemicu memakai penyebut pos 2299000000 Form 01.00 (bukan jumlah baris Form 14.00)
// dan memakai perbandingan ketat: persis 25% tidak memicu, sedikit di atasnya memicu.
func TestForm14_01PemicuKetikKetatPada25Persen(t *testing.T) {
	tepat := buildDenganRows(t,
		row("20300", "Giro", 300),
		row("20700", "Utang Lainnya", 100),
	)
	if adaTable(tepat, "14.01") {
		t.Fatal("pos Lainnya tepat 25% dari jumlah liabilitas lainnya tidak boleh memicu (harus >, bukan >=)")
	}

	lebih := buildDenganRows(t,
		row("20300", "Giro", 299),
		row("20700", "Utang Lainnya", 101),
	)
	if !adaTable(lebih, "14.01") {
		t.Fatal("pos Lainnya di atas 25% harus memicu Form 14.01")
	}
}

// (d) Kolom III Jumlah = jumlah barisnya. PDF #page 217 tidak menegaskan ikatan wajib
// ke pos Lainnya Form 14.00, jadi tie-out itu tidak diklaim dan dicatat di Notes.
func TestForm14_01TotalJumlahBarisDanNotesTidakKlaimTieOut(t *testing.T) {
	b := buildDenganRows(t,
		row("20300", "Giro", 299),
		row("20700", "Utang Lainnya", 101),
	)
	sec := tableByForm(t, b, "14.01")
	total := findCell(t, sec, form14_01TotalKey, "III").Value
	baris := findCell(t, sec, "20700", "III").Value
	if total != baris {
		t.Fatalf("JUMLAH Form 14.01 = %s, ingin jumlah barisnya = %s", total, baris)
	}
	catatan := strings.Join(sec.Notes, " ")
	if !strings.Contains(catatan, "TIDAK menegaskan") {
		t.Fatalf("Notes harus menyatakan ikatan ke Form 14.00 tidak ditegaskan PDF, dapat %q", catatan)
	}
}

// (e) Kolom I Form 14.01 bersumber dari kantor pelapor yang sama dengan form lain.
func TestForm14_01KolomSandiKantorSamaSumbernya(t *testing.T) {
	base := newStubSource()
	base.bs.Rows = append(base.bs.Rows,
		row("20300", "Giro", 299),
		row("20700", "Utang Lainnya", 101),
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
	sec := tableByForm(t, b, "14.01")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Rows) == 0 {
		t.Fatal("Form 14.01 tidak punya baris")
	}
	for _, r := range sec.Rows {
		if got := cellValue(r, "I"); got != "001" {
			t.Errorf("baris %s kolom I = %q, ingin 001", r.Key, got)
		}
	}
}

// Terpicu tetapi ada akun tanpa saldo: baris ditulis "-" beralasan dan JUMLAH ditulis
// "-" agar angka sebagian tidak disalahartikan sebagai total.
func TestBuildForm14_01AkunTanpaSaldoDitulisMinus(t *testing.T) {
	sec := buildForm14_01([]rincianLainnya{{COACode: "20700"}},
		ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})
	baris := rowByKey(t, sec, "20700")
	if baris.Reason == "" {
		t.Fatal("baris akun tanpa saldo harus mencantumkan alasan")
	}
	if got := findCell(t, sec, "20700", "III").Value; got != "-" {
		t.Errorf("jumlah akun tanpa saldo = %q, ingin -", got)
	}
	total := rowByKey(t, sec, form14_01TotalKey)
	if total.Reason == "" {
		t.Fatal("JUMLAH harus beralasan saat masih ada akun tanpa saldo")
	}
	if got := findCell(t, sec, form14_01TotalKey, "III").Value; got != "-" {
		t.Errorf("JUMLAH = %q, ingin -", got)
	}
}
