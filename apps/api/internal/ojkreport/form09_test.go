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

// Invarian inti Form 09.00: cakupan COAMapping09Draft tepat sama dengan entri
// COAMappingDraft yang bersandi 1299000000 (pos Aset Lainnya Form 01.00). Dengan itu
// total Form 09.00 tidak mungkin berbeda dari pos Aset Lainnya Form 01.00.
func TestForm09PemetaanSejajarDenganForm01(t *testing.T) {
	if dup := DuplicateCOACodes(COAMapping09Draft); len(dup) > 0 {
		t.Fatalf("kode COA ganda pada pemetaan Form 09.00: %v", dup)
	}

	form01 := make(map[string]bool)
	for _, e := range COAMappingDraft {
		if e.Sandi == "1299000000" {
			form01[e.COACode] = true
		}
	}
	if len(form01) == 0 {
		t.Fatal("tidak ada entri Form 01.00 yang dipetakan ke Aset Lainnya 1299000000")
	}

	form09 := make(map[string]bool)
	for _, e := range COAMapping09Draft {
		if e.Form != "09.00" {
			t.Errorf("entri Form 09.00 %s form = %q, ingin 09.00", e.COACode, e.Form)
		}
		form09[e.COACode] = true
	}

	for code := range form01 {
		if !form09[code] {
			t.Errorf("COA %s dipetakan ke 1299000000 Form 01.00 tetapi tidak ada di COAMapping09Draft", code)
		}
	}
	for code := range form09 {
		if !form01[code] {
			t.Errorf("COA %s ada di COAMapping09Draft tetapi tidak dipetakan ke 1299000000 Form 01.00", code)
		}
	}
}

// Setiap kode COA Form 09.00 harus ada di seed akun daun, dan setiap sandi yang
// ditunjuknya harus ada di susunan resmi form09Lines. Ini menutup celah yang sama
// dengan yang pernah lolos pada akun CKPN/restrukturisasi migrasi 000042/000043.
func TestForm09KodeAdaDiSeedDanSandiAdaDiSusunan(t *testing.T) {
	seed := make(map[string]bool, len(seedLeafCOACodes))
	for _, c := range seedLeafCOACodes {
		seed[c] = true
	}
	sandi := make(map[string]bool, len(form09Lines))
	for _, l := range form09Lines {
		sandi[l.Sandi] = true
	}
	for _, e := range COAMapping09Draft {
		if !seed[e.COACode] {
			t.Errorf("COA %s pada COAMapping09Draft tidak ada di seedLeafCOACodes", e.COACode)
		}
		if !sandi[e.Sandi] {
			t.Errorf("COA %s menunjuk sandi %s yang tidak ada di form09Lines", e.COACode, e.Sandi)
		}
	}
}

// buildForm09 memakai nama pos resmi untuk baris yang punya sumber dan menandai baris
// yang belum punya akun COA sebagai tidak tersedia (bukan nol).
func TestBuildForm09MenandaiPosTanpaSumber(t *testing.T) {
	sec := buildForm09(map[string]decimal.Decimal{"1299010200": decimal.NewFromInt(200)},
		ReportingOffice{Reason: "belum ada kantor aktif ber-sandi"})
	if sec.Form != "09.00" {
		t.Fatalf("form = %q, ingin 09.00", sec.Form)
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom Sandi Kantor tidak ditandai belum tersedia")
	}
	for _, sandi := range []string{"1299010100", "1299010300", "1299010900"} {
		if reason := rowByKey(t, sec, sandi).Reason; strings.TrimSpace(reason) == "" {
			t.Errorf("baris %s harus mencantumkan alasan tidak tersedia", sandi)
		}
	}
	if got := cellValue(rowByKey(t, sec, "1299010200"), "II"); got != "b. Kredit yang Diberikan" {
		t.Errorf("nama pos 1299010200 = %q", got)
	}
	if got := findCell(t, sec, "1299010200", "IV").Value; got != "200" {
		t.Errorf("jumlah 1299010200 = %q, ingin 200", got)
	}
}

// Total Form 09.00 harus sama dengan pos Aset Lainnya Form 01.00 karena keduanya
// membaca saldo COA yang sama; baris induk 1299010000 adalah jumlah pos anaknya.
func TestForm09TotalSamaDenganAsetLainnyaForm01(t *testing.T) {
	src := newStubSource()
	src.bs.Rows = append(src.bs.Rows,
		row("10305", "Piutang Denda", 100),
		row("10310", "Premi Penjaminan LPS Dibayar di Muka", 500),
		row("10320", "Uang Muka Pajak", 600),
		row("10330", "Aset Pajak Tangguhan", 700),
		row("10340", "Biaya Dibayar di Muka", 800),
		row("10350", "Tagihan kepada Perusahaan Asuransi", 900),
		row("10360", "Uang Muka untuk Kegiatan Operasional", 1000),
		row("10400", "Bunga Kredit yang Masih Akan Diterima", 200),
		row("10999", "Akun Sementara", 300),
		row("11700", "Piutang Denda Syariah", 400),
	)
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}

	asetLainnya := findLine(t, b, "01.00", "1299000000").Amount
	sec := tableByForm(t, b, "09.00")

	// Jumlahkan pos daun (tanpa baris induk 1299010000) agar tidak dihitung dua kali.
	total := decimal.Zero
	for _, l := range form09Lines {
		if len(l.TotalFrom) > 0 {
			continue
		}
		total = total.Add(form09Value(t, sec, l.Sandi))
	}
	if !total.Equal(asetLainnya) {
		t.Fatalf("total Form 09.00 = %s, ingin sama dengan 1299000000 Form 01.00 = %s", total, asetLainnya)
	}

	// Baris induk 1299010000 = jumlah empat pos anaknya.
	anak := decimal.Zero
	for _, s := range []string{"1299010100", "1299010200", "1299010300", "1299010900"} {
		anak = anak.Add(form09Value(t, sec, s))
	}
	if induk := form09Value(t, sec, "1299010000"); !induk.Equal(anak) {
		t.Fatalf("1299010000 = %s, ingin jumlah anak = %s", induk, anak)
	}

	// Penampung resmi "Lainnya" memuat Piutang Denda + Akun Sementara + Piutang Denda
	// Syariah = 800; ketiganya tidak punya pos tersendiri di Form 09.00.
	if got := form09Value(t, sec, "1299990000"); !got.Equal(decimal.NewFromInt(800)) {
		t.Fatalf("1299990000 = %s, ingin 800", got)
	}
}

// Berkas ekspor menulis sel Jumlah pos tanpa akun COA sebagai "-" (bukan nol) lengkap
// dengan alasan konkret ketiadaan akun COA.
func TestWriteTextForm09SelTidakTersediaDitulisMinus(t *testing.T) {
	b, err := NewBuilder(newStubSource()).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	var buf bytes.Buffer
	if err := WriteText(&buf, b); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "09.00|1299010100|IV Jumlah|-") {
		t.Fatal("sel Jumlah pos tanpa akun COA harus ditulis \"-\", bukan nol")
	}
	if !strings.Contains(out, "TIDAK TERSEDIA:") || !strings.Contains(out, "belum ada akun COA") {
		t.Fatal("alasan ketiadaan akun COA harus tercantum pada berkas")
	}
}

// form09Value mengembalikan nilai kolom IV satu baris Form 09.00; baris tanpa sumber
// (Reason terisi) bernilai nol karena memang tidak tersedia, bukan angka karangan.
func form09Value(t *testing.T, sec TableSection, sandi string) decimal.Decimal {
	t.Helper()
	for _, r := range sec.Rows {
		if r.Key != sandi {
			continue
		}
		if r.Reason != "" {
			return decimal.Zero
		}
		for _, c := range r.Cells {
			if c.Sandi != "IV" {
				continue
			}
			v, err := decimal.NewFromString(c.Value)
			if err != nil {
				t.Fatalf("nilai Form 09.00 sandi %s bukan angka: %q", sandi, c.Value)
			}
			return v
		}
		t.Fatalf("baris Form 09.00 sandi %s tidak punya kolom IV", sandi)
	}
	t.Fatalf("baris Form 09.00 sandi %s tidak ditemukan", sandi)
	return decimal.Zero
}

// Kolom I Form 09.00 kini bersumber dari kantor pelapor tunggal pada bank_offices.
// Lewat RepoSource, kolom I muncul sebagai kolom nyata paling kiri ber-sandi kantor
// itu dan tidak lagi didaftarkan sebagai tidak tersedia.
func TestBuilderForm09KolomSandiKantorLewatRepoSource(t *testing.T) {
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
	sec := tableByForm(t, b, "09.00")
	if punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom I tidak boleh tidak tersedia saat kantor pelapor tunggal tersedia")
	}
	if len(sec.Columns) == 0 || sec.Columns[0].Sandi != "I" || sec.Columns[0].Nama != "Sandi Kantor" {
		t.Fatalf("kolom pertama = %+v, ingin I Sandi Kantor", sec.Columns)
	}
	if len(sec.Rows) == 0 {
		t.Fatal("Form 09.00 tidak punya baris")
	}
	for _, r := range sec.Rows {
		if got := cellValue(r, "I"); got != "001" {
			t.Errorf("baris %s kolom I = %q, ingin 001", r.Key, got)
		}
	}
}

// Lebih dari satu kantor aktif ber-sandi: kolom I tetap tidak tersedia dengan alasan
// yang menyebut jumlahnya; sistem tidak memilih kantor pelapor sendiri.
func TestBuilderForm09KolomSandiKantorBanyakKantor(t *testing.T) {
	src := RepoSource{
		Source: newStubSource(),
		Kelembagaan: kelembagaanRepoStub{offices: []domain.BankOffice{
			{Code: "001", Name: "Kantor A", Status: domain.KelembagaanKantorAktif},
			{Code: "002", Name: "Kantor B", Status: domain.KelembagaanKantorAktif},
		}},
	}
	b, err := NewBuilder(src).GenerateMonthly(
		context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}
	sec := tableByForm(t, b, "09.00")
	u := unavailableColumn(t, sec, "I")
	if !strings.Contains(u.Reason, "2 kantor aktif ber-sandi") {
		t.Fatalf("alasan kolom I = %q, ingin menyebut 2 kantor aktif ber-sandi", u.Reason)
	}
	for _, c := range sec.Columns {
		if c.Sandi == "I" {
			t.Fatal("kolom I tidak boleh tersedia saat kantor pelapor ambigu")
		}
	}
}
