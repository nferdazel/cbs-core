package ojkreport

import (
	"strconv"
	"testing"
)

// seedLeafCOACodes adalah akun daun pada bagan akun baku (migrasi 000004/000005/
// 000019/000024). Daftar ini sengaja ditulis eksplisit sebagai jaring pengaman:
// bila kelak ada COA baru yang belum dipetakan, uji ini gagal dan ekspor tidak
// diam-diam menghasilkan laporan bolong.
var seedLeafCOACodes = []string{
	"10100", "10101", "10200", "10300", "10301", "10305", "10400", "10500", "10600",
	"10700", "10800", "10900", "10999",
	"11100", "11200", "11300", "11310", "11320", "11400", "11500", "11600", "11700", "11900",
	"12100", "12200", "12300", "12400", "12500", "12900",
	"13100", "13200",
	"14100", "14200", "14300", "14400", "14500", "14600", "14900",
	"15100", "15200", "15900",
	"20100", "20200", "20300", "20400", "20500", "20600", "20700", "20800",
	"30100", "30200", "30300", "30400",
	"40100", "40200", "40300", "40400", "40500", "40900",
	"50100", "50200", "50300", "50400", "50500", "50900",
	"60100",
}

func TestMappingTidakAdaKodeGanda(t *testing.T) {
	if dup := DuplicateCOACodes(COAMappingDraft); len(dup) > 0 {
		t.Fatalf("kode COA ganda pada pemetaan: %v", dup)
	}
}

func TestMappingMenutupSeluruhCOABaku(t *testing.T) {
	mapped := make(map[string]bool)
	for _, e := range COAMappingDraft {
		mapped[e.COACode] = true
	}
	for _, code := range seedLeafCOACodes {
		if !mapped[code] {
			t.Errorf("COA %s belum dipetakan ke pos OJK", code)
		}
	}
}

func TestMappingHanyaMemakaiFormDikenal(t *testing.T) {
	known := map[string]bool{}
	for _, f := range OJKBulananForms {
		known[f.Form] = true
	}
	for _, e := range COAMappingDraft {
		if !known[e.Form] {
			t.Errorf("COA %s dipetakan ke form tidak dikenal %q", e.COACode, e.Form)
		}
	}
}

// Sandi pada pemetaan harus benar-benar ada pada susunan form, supaya salah ketik
// tidak menghasilkan pos hantu.
func TestMappingSandiAdaDiForm(t *testing.T) {
	known := map[string]bool{}
	for _, l := range form01Lines {
		known["01.00/"+l.Sandi] = true
	}
	for _, l := range form02Lines {
		known["02.00/"+l.Sandi] = true
	}
	for _, e := range COAMappingDraft {
		if e.Sandi == "" {
			continue
		}
		if !known[e.Form+"/"+e.Sandi] {
			t.Errorf("COA %s menunjuk sandi %s yang tidak ada di form %s", e.COACode, e.Sandi, e.Form)
		}
	}
}

func TestMappingSandiSepuluhDigit(t *testing.T) {
	for _, e := range COAMappingDraft {
		if len(e.Sandi) != 10 {
			t.Errorf("COA %s sandi %q bukan 10 digit", e.COACode, e.Sandi)
			continue
		}
		if _, err := strconv.Atoi(e.Sandi); err != nil {
			t.Errorf("COA %s sandi %q bukan angka", e.COACode, e.Sandi)
		}
	}
}

func TestMappingSignHanyaSatuAtauMinusSatu(t *testing.T) {
	for _, e := range COAMappingDraft {
		if e.Sign != 1 && e.Sign != -1 {
			t.Errorf("COA %s Sign = %d, ingin 1 atau -1", e.COACode, e.Sign)
		}
	}
}

func TestCOACodesForForm(t *testing.T) {
	codes := COACodesForForm(COAMappingDraft, "01.00")
	if len(codes) == 0 {
		t.Fatal("form 01.00 tidak punya kode COA")
	}
	for i := 1; i < len(codes); i++ {
		if codes[i-1] > codes[i] {
			t.Fatalf("kode form 01.00 tidak terurut: %s > %s", codes[i-1], codes[i])
		}
	}
}
