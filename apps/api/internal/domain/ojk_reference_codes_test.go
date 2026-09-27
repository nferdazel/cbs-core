package domain

import "testing"

func TestValidateOJKHubunganBankCode(t *testing.T) {
	for _, kode := range []string{"", "11", "12", "20", " 11 "} {
		if err := ValidateOJKHubunganBankCode(kode); err != nil {
			t.Errorf("kode %q: %v, ingin diterima", kode, err)
		}
	}
	for _, kode := range []string{"10", "99", "1"} {
		err := ValidateOJKHubunganBankCode(kode)
		code, _, ok := LocalizedMessage(err)
		if !ok || code != "ojk_inline_code_invalid" {
			t.Errorf("kode %q: err = %v, ingin ojk_inline_code_invalid", kode, err)
		}
	}
}

func TestUpdateOJKLoanCodesValidate(t *testing.T) {
	jenis := "39"
	periode := "1"
	in := UpdateOJKLoanCodesInput{OJKJenisPenggunaanCode: &jenis, OJKPeriodePembayaranCode: &periode}
	if err := in.Validate(); err != nil {
		t.Fatalf("himpunan sah ditolak: %v", err)
	}
	if in.IsEmpty() {
		t.Fatal("input berisi tidak boleh dilaporkan kosong")
	}

	badJenis := "11"
	code, _, ok := LocalizedMessage(UpdateOJKLoanCodesInput{OJKJenisPenggunaanCode: &badJenis}.Validate())
	if !ok || code != "ojk_inline_code_invalid" {
		t.Fatalf("jenis tidak sah: err = %v", code)
	}

	badPeriode := "9"
	code, _, ok = LocalizedMessage(UpdateOJKLoanCodesInput{OJKPeriodePembayaranCode: &badPeriode}.Validate())
	if !ok || code != "ojk_inline_code_invalid" {
		t.Fatalf("periode tidak sah: err = %v", code)
	}

	if !(UpdateOJKLoanCodesInput{}).IsEmpty() {
		t.Fatal("input tanpa bidang harus kosong")
	}
}
