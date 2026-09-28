package domain

import "testing"

// TestUpdateOJKLoanCodesValidateFieldsBaru menegakkan himpunan inline dan aturan
// nominal/persentase/tanggal untuk kolom K1/K2 Form 06.00 yang baru dibuka.
func TestUpdateOJKLoanCodesValidateFieldsBaru(t *testing.T) {
	valid := func(in UpdateOJKLoanCodesInput) {
		t.Helper()
		if err := in.Validate(); err != nil {
			t.Fatalf("masukan sah ditolak: %v", err)
		}
	}
	invalid := func(in UpdateOJKLoanCodesInput, wantCode string) {
		t.Helper()
		code, _, ok := LocalizedMessage(in.Validate())
		if !ok || code != wantCode {
			t.Fatalf("err = %v (kode %q), ingin kode %q", in.Validate(), code, wantCode)
		}
	}

	sumber := "10"
	valid(UpdateOJKLoanCodesInput{OJKSumberDanaCode: &sumber})
	sumber = "11"
	invalid(UpdateOJKLoanCodesInput{OJKSumberDanaCode: &sumber}, "ojk_inline_code_invalid")

	kategori := "4"
	valid(UpdateOJKLoanCodesInput{OJKKategoriUsahaCode: &kategori})
	kategori = "5"
	invalid(UpdateOJKLoanCodesInput{OJKKategoriUsahaCode: &kategori}, "ojk_inline_code_invalid")

	sifat := "9"
	valid(UpdateOJKLoanCodesInput{OJKSifatKreditCode: &sifat})
	sifat = "1"
	invalid(UpdateOJKLoanCodesInput{OJKSifatKreditCode: &sifat}, "ojk_inline_code_invalid")

	pctOK := "100.00"
	valid(UpdateOJKLoanCodesInput{OJKPenjaminBagianPct: &pctOK})
	pctBad := "100.001"
	invalid(UpdateOJKLoanCodesInput{OJKPenjaminBagianPct: &pctBad}, "ojk_percentage_invalid")
	pctNeg := "-0.01"
	invalid(UpdateOJKLoanCodesInput{OJKPenjaminBagianPct: &pctNeg}, "ojk_percentage_invalid")

	tanggalOK := "2024-01-31"
	valid(UpdateOJKLoanCodesInput{OJKTanggalMulaiMacet: &tanggalOK})
	tanggalBad := "2024-13-01"
	invalid(UpdateOJKLoanCodesInput{OJKTanggalMulaiMacet: &tanggalBad}, "ojk_date_invalid")

	amountOK := "0"
	valid(UpdateOJKLoanCodesInput{OJKAgunanPPKAAmount: &amountOK})
	amountBad := "-1"
	invalid(UpdateOJKLoanCodesInput{OJKAgunanPPKAAmount: &amountBad}, "ojk_amount_invalid")
	amountText := "bukan-angka"
	invalid(UpdateOJKLoanCodesInput{OJKKelonggaranTarikAmount: &amountText}, "ojk_amount_invalid")

	// String kosong berarti "kosongkan" dan tetap sah untuk semua bidang.
	kosong := ""
	valid(UpdateOJKLoanCodesInput{
		OJKSumberDanaCode:    &kosong,
		OJKPenjaminBagianPct: &kosong,
		OJKAgunanPPKAAmount:  &kosong,
	})
}

// TestParseOJKAmountNullable membuktikan tiga keadaan nominal: kosong -> NULL,
// nol tetap nilai sah, dan negatif ditolak.
func TestParseOJKAmountNullable(t *testing.T) {
	nd, err := ParseOJKAmount(OJKBlockedAmountField, "")
	if err != nil || nd.Valid {
		t.Fatalf("kosong = %+v, %v; ingin NULL", nd, err)
	}
	nd, err = ParseOJKAmount(OJKBlockedAmountField, "0")
	if err != nil || !nd.Valid || !nd.Decimal.IsZero() {
		t.Fatalf("nol = %+v, %v; ingin nilai nol sah", nd, err)
	}
	if _, err := ParseOJKAmount(OJKBlockedAmountField, "-0.01"); err == nil {
		t.Fatal("nominal negatif harus ditolak")
	}
}

// TestUpdateOJKPlacementCodesValidate menegakkan sandi inline hubungan bank
// penempatan (12/20) dan nominal nullable.
func TestUpdateOJKPlacementCodesValidate(t *testing.T) {
	hubungan := "12"
	if err := (UpdateOJKPlacementCodesInput{OJKHubunganBankCode: &hubungan}).Validate(); err != nil {
		t.Fatalf("12 harus diterima: %v", err)
	}
	hubungan = "11"
	code, _, ok := LocalizedMessage(UpdateOJKPlacementCodesInput{OJKHubunganBankCode: &hubungan}.Validate())
	if !ok || code != "ojk_inline_code_invalid" {
		t.Fatalf("11 (sandi nasabah) tidak boleh diterima untuk penempatan: %v", code)
	}
	if !(UpdateOJKPlacementCodesInput{}).IsEmpty() {
		t.Fatal("masukan tanpa bidang harus kosong")
	}
	amount := "0"
	if err := (UpdateOJKPlacementCodesInput{BlockedAmount: &amount}).Validate(); err != nil {
		t.Fatalf("nominal nol harus diterima: %v", err)
	}
	neg := "-5"
	if _, _, ok := LocalizedMessage(UpdateOJKPlacementCodesInput{AccruedInterestPending: &neg}.Validate()); !ok {
		t.Fatal("nominal negatif harus ditolak")
	}
}
