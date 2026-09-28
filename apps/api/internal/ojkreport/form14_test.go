package ojkreport

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Setiap kode COA Form 14.00 harus ada di seed akun daun, dan setiap sandi yang
// ditunjuknya harus ada di susunan resmi form14Lines. Ini menutup celah yang sama
// dengan yang pernah lolos pada akun CKPN/restrukturisasi migrasi 000042/000043.
func TestForm14KodeAdaDiSeedDanSandiAdaDiSusunan(t *testing.T) {
	if dup := DuplicateCOACodes(COAMapping14Draft); len(dup) > 0 {
		t.Fatalf("kode COA ganda pada pemetaan Form 14.00: %v", dup)
	}
	seed := make(map[string]bool, len(seedLeafCOACodes))
	for _, c := range seedLeafCOACodes {
		seed[c] = true
	}
	sandi := make(map[string]bool, len(form14Lines))
	for _, l := range form14Lines {
		sandi[l.Sandi] = true
	}
	for _, e := range COAMapping14Draft {
		if e.Form != "14.00" {
			t.Errorf("entri Form 14.00 %s form = %q, ingin 14.00", e.COACode, e.Form)
		}
		if !seed[e.COACode] {
			t.Errorf("COA %s pada COAMapping14Draft tidak ada di seedLeafCOACodes", e.COACode)
		}
		if !sandi[e.Sandi] {
			t.Errorf("COA %s menunjuk sandi %s yang tidak ada di form14Lines", e.COACode, e.Sandi)
		}
	}
}

// buildForm14 memakai nama pos resmi untuk baris yang punya sumber dan menandai baris
// yang belum punya akun COA sebagai tidak tersedia (bukan nol). Baris JUMLAH pun tidak
// diisi selama masih ada pos tanpa akun.
func TestBuildForm14MenandaiPosTanpaSumber(t *testing.T) {
	sec := buildForm14(map[string]decimal.Decimal{
		"2299020000": decimal.NewFromInt(200),
		"2299990000": decimal.NewFromInt(100),
	}, ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})

	if sec.Form != "14.00" {
		t.Fatalf("form = %q, ingin 14.00", sec.Form)
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom Sandi Kantor tidak ditandai belum tersedia")
	}
	// Header grup "Utang Bunga" tanpa sandi sendiri.
	header := rowByKey(t, sec, form14HeaderKey)
	if got := cellValue(header, "II"); got != "Utang Bunga" {
		t.Errorf("header grup = %q, ingin Utang Bunga", got)
	}
	if got := cellValue(header, "III"); got != "-" {
		t.Errorf("sandi header grup = %q, ingin -", got)
	}
	// Pos tanpa akun COA: alasan wajib ada dan Jumlah ditulis "-", bukan nol.
	tanpaAkun := rowByKey(t, sec, "2299030000")
	if strings.TrimSpace(tanpaAkun.Reason) == "" {
		t.Error("baris 2299030000 harus mencantumkan alasan tidak tersedia")
	}
	if got := cellValue(tanpaAkun, "IV"); got != "-" {
		t.Errorf("jumlah 2299030000 = %q, ingin -", got)
	}
	// Pos bersumber: nama resmi dan jumlahnya terisi.
	if got := cellValue(rowByKey(t, sec, "2299020000"), "II"); got != "Utang Pajak" {
		t.Errorf("nama pos 2299020000 = %q", got)
	}
	if got := findCell(t, sec, "2299020000", "IV").Value; got != "200" {
		t.Errorf("jumlah 2299020000 = %q, ingin 200", got)
	}
	// JUMLAH tidak diisi selama ada pos tanpa akun.
	if total := rowByKey(t, sec, form14TotalKey); total.Reason == "" {
		t.Error("baris JUMLAH harus beralasan saat masih ada pos tanpa akun")
	}
}

// Angka Form 14.00 berasal dari saldo COA pada akhir periode (periodEnd), bukan waktu
// ekspor, dan pos tanpa akun tetap ditulis "-" di berkas ekspor.
func TestForm14AngkaDariSaldoCOAPeriodEnd(t *testing.T) {
	src := newStubSource()
	src.bs.Rows = append(src.bs.Rows,
		row("20500", "Utang Pajak", 1234),
		row("20700", "Utang Lainnya", 5678),
	)
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	if want := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC); !src.gotBSAsOf.Equal(want) {
		t.Fatalf("asOf = %s, ingin %s", src.gotBSAsOf, want)
	}

	sec := tableByForm(t, b, "14.00")
	if got := findCell(t, sec, "2299020000", "IV").Value; got != "1234" {
		t.Errorf("2299020000 = %q, ingin 1234 dari saldo COA 20500 periodEnd", got)
	}
	if got := findCell(t, sec, "2299990000", "IV").Value; got != "5678" {
		t.Errorf("2299990000 = %q, ingin 5678 dari saldo COA 20700 periodEnd", got)
	}
	if got := findCell(t, sec, "2299030000", "IV").Value; got != "-" {
		t.Errorf("pos tanpa akun 2299030000 = %q, ingin -", got)
	}

	// Form 14.00 ikut keluaran bulanan dan alasan ketiadaan akun tercantum di berkas.
	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# FORM 14.00") {
		t.Fatal("Form 14.00 harus muncul pada keluaran bulanan")
	}
	if !strings.Contains(out, "belum ada akun COA") {
		t.Fatal("alasan ketiadaan akun COA harus tercantum pada berkas")
	}
}

// Kolom I Form 14.00 bersumber dari kantor pelapor tunggal pada bank_offices, sama
// dengan Form 09.00/01.01. Lewat RepoSource, kolom I muncul sebagai kolom nyata paling
// kiri ber-sandi kantor itu dan tidak lagi didaftarkan sebagai tidak tersedia.
func TestBuilderForm14KolomSandiKantorLewatRepoSource(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		Kelembagaan: kelembagaanRepoStub{offices: []domain.BankOffice{
			{Code: "001", Name: "Kantor Pusat", Status: domain.KelembagaanKantorAktif},
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "14.00")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if len(sec.Rows) == 0 {
		t.Fatal("Form 14.00 tidak punya baris")
	}
	for _, r := range sec.Rows {
		if got := cellValue(r, "I"); got != "001" {
			t.Errorf("baris %s kolom I = %q, ingin 001", r.Key, got)
		}
	}
}
