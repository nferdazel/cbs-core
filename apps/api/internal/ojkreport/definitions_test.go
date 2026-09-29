package ojkreport

import (
	"strings"
	"testing"
	"time"
)

func TestDeadlineBulanan(t *testing.T) {
	def := reportByCode("LAPORAN_BULANAN_BPR")
	period := time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)

	due := def.Deadline(period)
	if want := time.Date(2026, time.February, 10, 0, 0, 0, 0, time.UTC); !due.Equal(want) {
		t.Fatalf("tenggat = %s, ingin %s", due.Format("2006-01-02"), want.Format("2006-01-02"))
	}
	corr := def.CorrectionDeadline(period)
	if want := time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC); !corr.Equal(want) {
		t.Fatalf("batas koreksi = %s, ingin %s", corr.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

// Desember harus menggulir ke Januari tahun berikutnya, bukan bulan ke-13.
func TestDeadlineLintasTahun(t *testing.T) {
	def := reportByCode("LAPORAN_BULANAN_BPR")
	period := time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC)

	due := def.Deadline(period)
	if want := time.Date(2026, time.January, 10, 0, 0, 0, 0, time.UTC); !due.Equal(want) {
		t.Fatalf("tenggat = %s, ingin %s", due.Format("2006-01-02"), want.Format("2006-01-02"))
	}
}

func TestCorrectionDeadlineMengikutiTenggatBilaKosong(t *testing.T) {
	def := OJKReportDefinition{DueDay: 10, CorrectionDay: 0}
	period := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	if !def.CorrectionDeadline(period).Equal(def.Deadline(period)) {
		t.Fatal("batas koreksi tanpa CorrectionDay harus sama dengan tenggat")
	}
}

func TestDefinisiTenggatSesuaiSEOJK(t *testing.T) {
	cases := []struct {
		code        string
		periodicity OJKPeriodicity
		dueDay      int
	}{
		{"LAPORAN_BULANAN_BPR", OJKBulanan, 10},
		{"LAPORAN_KEUANGAN_PUBLIKASI_TRIWULANAN", OJKTribulanan, 10},
		{"LAPORAN_LAKU_PANDAI", OJKTribulanan, 15},
	}
	for _, c := range cases {
		def := reportByCode(c.code)
		if def.Code != c.code {
			t.Fatalf("definisi %s tidak ditemukan", c.code)
		}
		if def.Periodicity != c.periodicity {
			t.Fatalf("%s periodisitas = %s, ingin %s", c.code, def.Periodicity, c.periodicity)
		}
		if def.DueDay != c.dueDay {
			t.Fatalf("%s DueDay = %d, ingin %d", c.code, def.DueDay, c.dueDay)
		}
		if def.Channel != OJKChannelAPOLO {
			t.Fatalf("%s kanal = %s, ingin APOLO", c.code, def.Channel)
		}
	}
}

// Laporan yang belum bisa dibangun wajib mencantumkan alasan, dan yang bisa
// dibangun wajib punya tenggat. Ini mencegah kekurangan data disembunyikan.
func TestDefinisiLengkap(t *testing.T) {
	for _, d := range OJKReportDefinitions {
		if d.Buildable && d.DueDay <= 0 {
			t.Fatalf("%s buildable tetapi DueDay tidak diisi", d.Code)
		}
		if !d.Buildable && d.UnavailableReason == "" {
			t.Fatalf("%s tidak buildable tetapi tanpa alasan", d.Code)
		}
	}
	for _, f := range OJKBulananForms {
		if !f.Buildable && f.UnavailableReason == "" {
			t.Fatalf("form %s tidak buildable tetapi tanpa alasan", f.Form)
		}
	}
}

func TestBuildableForms(t *testing.T) {
	forms := BuildableForms()
	if len(forms) != 36 {
		t.Fatalf("ingin 36 form buildable, dapat %d", len(forms))
	}
	want := []string{"00.00", "00.08", "01.00", "01.01", "02.00", "05.00", "06.00", "09.00", "09.01", "13.00", "00.01", "00.02", "00.03", "00.04", "00.05", "00.07", "00.09", "00.10", "00.11", "00.12", "00.17", "00.18", "03.00", "04.00", "06.01", "06.02", "07.00", "08.00", "10.00", "11.00", "12.00", "14.00", "14.01", "16.00", "17.00", "18.00"}
	for i, w := range want {
		if forms[i].Form != w {
			t.Fatalf("form buildable[%d] = %s, ingin %s", i, forms[i].Form, w)
		}
	}
}

// Form 09.01/14.01 kini buildable sebagai form KONDISIONAL: pendaftarannya tidak boleh
// menyimpan alasan "belum dibangun" karena perakitnya memang hanya terbit saat ambang
// 25% terlampaui (bukan karena kekurangan data).
func TestForm09_01Dan14_01Buildable(t *testing.T) {
	for _, form := range []string{"09.01", "14.01"} {
		var def *OJKFormDefinition
		for i := range OJKBulananForms {
			if OJKBulananForms[i].Form == form {
				def = &OJKBulananForms[i]
				break
			}
		}
		if def == nil {
			t.Fatalf("Form %s tidak terdaftar", form)
		}
		if !def.Buildable {
			t.Fatalf("Form %s harus buildable", form)
		}
		if def.UnavailableReason != "" {
			t.Fatalf("Form %s buildable tidak boleh menyimpan alasan: %q", form, def.UnavailableReason)
		}
	}
}

// Form 14.00 kini dapat dibangun dari saldo COA Liabilitas Lainnya. Daftar sandinya
// harus persis susunan resmi Form 14.00 (PDF #page 213).
func TestForm14BuildableDenganSandiResmi(t *testing.T) {
	var def *OJKFormDefinition
	for i := range OJKBulananForms {
		if OJKBulananForms[i].Form == "14.00" {
			def = &OJKBulananForms[i]
			break
		}
	}
	if def == nil {
		t.Fatal("Form 14.00 tidak terdaftar")
	}
	if !def.Buildable {
		t.Fatal("Form 14.00 harus buildable setelah pemetaan COA ke pos 14.00 tersedia")
	}
	if def.UnavailableReason != "" {
		t.Fatalf("Form 14.00 buildable tidak boleh menyimpan alasan: %q", def.UnavailableReason)
	}

	want := []string{
		"2299010100", "2299010201", "2299010202", "2299010301", "2299010302",
		"2299010401", "2299010402", "2299010501", "2299010502", "2299019900",
		"2299020000", "2299030000", "2299040000", "2299050000", "2299060000",
		"2299070000", "2299990000",
	}
	got := make([]string, 0, len(form14Lines))
	for _, l := range form14Lines {
		got = append(got, l.Sandi)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sandi Form 14.00 = %v, ingin %v", got, want)
	}
}

// Laporan BMPK kini dapat dibangun karena endpoint ekspornya sudah tersedia; alasan
// "belum tersedia" harus hilang agar definisi tidak bertentangan dengan kenyataan.
func TestDefinisiLaporanBMPKBuildable(t *testing.T) {
	def := reportByCode("LAPORAN_BMPK")
	if !def.Buildable {
		t.Fatal("LAPORAN_BMPK harus buildable setelah endpoint ekspor dirangkai")
	}
	if def.UnavailableReason != "" {
		t.Fatalf("LAPORAN_BMPK buildable tidak boleh menyimpan alasan: %q", def.UnavailableReason)
	}
}

// Form 13.00 dapat dibangun dari agregasi simpanan bank lawan.
func TestForm13Buildable(t *testing.T) {
	for _, f := range OJKBulananForms {
		if f.Form != "13.00" {
			continue
		}
		if !f.Buildable {
			t.Fatal("Form 13.00 harus buildable")
		}
		if f.UnavailableReason != "" {
			t.Fatalf("Form 13.00 buildable tidak boleh menyimpan alasan: %q", f.UnavailableReason)
		}
		return
	}
	t.Fatal("Form 13.00 tidak terdaftar")
}

// Form 09.00 kini dapat dibangun dari saldo COA Aset Lainnya. Daftar sandinya harus
// persis susunan resmi Form 09.00 - 1 (PDF #page 187).
func TestForm09BuildableDenganSandiResmi(t *testing.T) {
	var def *OJKFormDefinition
	for i := range OJKBulananForms {
		if OJKBulananForms[i].Form == "09.00" {
			def = &OJKBulananForms[i]
			break
		}
	}
	if def == nil {
		t.Fatal("Form 09.00 tidak terdaftar")
	}
	if !def.Buildable {
		t.Fatal("Form 09.00 harus buildable setelah pemetaan COA ke pos 09.00 tersedia")
	}
	if def.UnavailableReason != "" {
		t.Fatalf("Form 09.00 buildable tidak boleh menyimpan alasan: %q", def.UnavailableReason)
	}

	want := []string{
		"1299010000", "1299010100", "1299010200", "1299010300", "1299010900",
		"1299020000", "1299030000", "1299040000", "1299050000", "1299060000",
		"1299070000", "1299990000",
	}
	got := make([]string, 0, len(form09Lines))
	for _, l := range form09Lines {
		got = append(got, l.Sandi)
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("sandi Form 09.00 = %v, ingin %v", got, want)
	}
}
