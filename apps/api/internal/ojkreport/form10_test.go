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

// Setiap kode COA Form 10.00 harus ada di seed akun daun, dan setiap sandi yang
// ditunjuknya harus ada di susunan resmi form10Lines. Ini menutup celah yang sama
// dengan yang pernah lolos pada akun CKPN/restrukturisasi migrasi 000042/000043.
func TestForm10KodeAdaDiSeedDanSandiAdaDiSusunan(t *testing.T) {
	if dup := DuplicateCOACodes(COAMapping10Draft); len(dup) > 0 {
		t.Fatalf("kode COA ganda pada pemetaan Form 10.00: %v", dup)
	}
	seed := make(map[string]bool, len(seedLeafCOACodes))
	for _, c := range seedLeafCOACodes {
		seed[c] = true
	}
	sandi := make(map[string]bool, len(form10Lines))
	for _, l := range form10Lines {
		sandi[l.Sandi] = true
	}
	for _, e := range COAMapping10Draft {
		if e.Form != "10.00" {
			t.Errorf("entri Form 10.00 %s form = %q, ingin 10.00", e.COACode, e.Form)
		}
		if !seed[e.COACode] {
			t.Errorf("COA %s pada COAMapping10Draft tidak ada di seedLeafCOACodes", e.COACode)
		}
		if !sandi[e.Sandi] {
			t.Errorf("COA %s menunjuk sandi %s yang tidak ada di form10Lines", e.COACode, e.Sandi)
		}
	}
}

// buildForm10 memakai nama pos resmi untuk baris yang punya sumber dan menandai baris
// yang belum punya akun COA sebagai tidak tersedia (bukan nol). Baris JUMLAH pun tidak
// diisi selama masih ada pos tanpa akun.
func TestBuildForm10MenandaiPosTanpaSumber(t *testing.T) {
	sec := buildForm10(map[string]decimal.Decimal{
		"2101990000": decimal.NewFromInt(300),
	}, ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})

	if sec.Form != "10.00" {
		t.Fatalf("form = %q, ingin 10.00", sec.Form)
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom Sandi Kantor tidak ditandai belum tersedia")
	}
	// Pos tanpa akun COA: alasan wajib ada dan Jumlah ditulis "-", bukan nol.
	for _, sandi := range []string{"2101020000", "2101030000", "2101040000", "2101050000", "2101060000", "2101070000"} {
		row := rowByKey(t, sec, sandi)
		if strings.TrimSpace(row.Reason) == "" {
			t.Errorf("baris %s harus mencantumkan alasan tidak tersedia", sandi)
		}
		if got := cellValue(row, "IV"); got != "-" {
			t.Errorf("jumlah %s = %q, ingin -", sandi, got)
		}
	}
	// Pos bersumber: nama resmi dan jumlahnya terisi.
	if got := cellValue(rowByKey(t, sec, "2101990000"), "II"); got != "Lainnya" {
		t.Errorf("nama pos 2101990000 = %q", got)
	}
	if got := findCell(t, sec, "2101990000", "IV").Value; got != "300" {
		t.Errorf("jumlah 2101990000 = %q, ingin 300", got)
	}
	// JUMLAH tidak diisi selama ada pos tanpa akun.
	if total := rowByKey(t, sec, form10TotalKey); total.Reason == "" {
		t.Error("baris JUMLAH harus beralasan saat masih ada pos tanpa akun")
	}
}

// Angka Form 10.00 berasal dari saldo COA pada akhir periode (periodEnd), bukan waktu
// ekspor, dan pos tanpa akun tetap ditulis "-" di berkas ekspor.
func TestForm10AngkaDariSaldoCOAPeriodEnd(t *testing.T) {
	src := newStubSource()
	src.bs.Rows = append(src.bs.Rows,
		row("20400", "Bunga Deposito yang Masih Harus Dibayar", 100),
		row("20600", "Utang Bunga", 200),
		row("12400", "Bagi Hasil Masih Harus Dibayar", 300),
	)
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	if want := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC); !src.gotBSAsOf.Equal(want) {
		t.Fatalf("asOf = %s, ingin %s", src.gotBSAsOf, want)
	}

	sec := tableByForm(t, b, "10.00")
	if got := findCell(t, sec, "2101990000", "IV").Value; got != "600" {
		t.Errorf("2101990000 = %q, ingin 600 dari saldo COA 20400/20600/12400 periodEnd", got)
	}
	if got := findCell(t, sec, "2101030000", "IV").Value; got != "-" {
		t.Errorf("pos tanpa akun 2101030000 = %q, ingin -", got)
	}

	// Form 10.00 ikut keluaran bulanan dan alasan ketiadaan akun tercantum di berkas.
	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "# FORM 10.00") {
		t.Fatal("Form 10.00 harus muncul pada keluaran bulanan")
	}
	if !strings.Contains(out, "belum ada akun COA") {
		t.Fatal("alasan ketiadaan akun COA harus tercantum pada berkas")
	}
}

// Pos 1 (2101010000) SENGAJA tidak diisi dari saldo periodEnd: definisi PDF #page 192
// menuntut pajak "untuk periode sebelum bulan laporan yang dibayarkan pada bulan
// laporan" (arus), bukan saldo Utang Pajak (20500) pada akhir periode. Akun 20500 juga
// sengaja tidak dipindahkan ke Lainnya agar tidak salah golong dan tidak menggandakan
// pos Utang Pajak (2299020000) Form 14.00.
func TestForm10PosPajakTidakDipaksaDariSaldoPeriodEnd(t *testing.T) {
	src := newStubSource()
	src.bs.Rows = append(src.bs.Rows,
		row("20500", "Utang Pajak", 999),
		row("20400", "Bunga Deposito yang Masih Harus Dibayar", 100),
		row("20600", "Utang Bunga", 200),
		row("12400", "Bagi Hasil Masih Harus Dibayar", 300),
	)
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "10.00")

	pos1 := rowByKey(t, sec, "2101010000")
	if got := cellValue(pos1, "IV"); got != "-" {
		t.Errorf("pos 1 (pajak) = %q, ingin - karena definisinya arus pembayaran bulan berjalan", got)
	}
	if !strings.Contains(pos1.Reason, "dibayarkan pada bulan laporan") {
		t.Errorf("alasan pos 1 = %q, ingin menyebut aturan \"dibayarkan pada bulan laporan\"", pos1.Reason)
	}
	// Saldo 20500 tidak boleh menyelinap ke Lainnya: Lainnya hanya tiga akun non-pajak.
	if got := findCell(t, sec, "2101990000", "IV").Value; got != "600" {
		t.Errorf("2101990000 = %q, ingin 600 (20500 tidak ikut)", got)
	}
}

// Kolom I Form 10.00 bersumber dari kantor pelapor tunggal pada bank_offices, sama
// dengan Form 09.00/14.00. Lewat RepoSource, kolom I muncul sebagai kolom nyata paling
// kiri ber-sandi kantor itu dan tidak lagi didaftarkan sebagai tidak tersedia.
func TestBuilderForm10KolomSandiKantorLewatRepoSource(t *testing.T) {
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
	sec := tableByForm(t, b, "10.00")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if len(sec.Rows) == 0 {
		t.Fatal("Form 10.00 tidak punya baris")
	}
	for _, r := range sec.Rows {
		if got := cellValue(r, "I"); got != "001" {
			t.Errorf("baris %s kolom I = %q, ingin 001", r.Key, got)
		}
	}
}
