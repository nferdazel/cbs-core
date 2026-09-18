package observability_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/observability"
)

func TestRedactMasksSensitiveValues(t *testing.T) {
	cases := []struct {
		key   string
		value string
		want  string
	}{
		{"password", "rahasia123", "***"},
		{"user_password", "rahasia123", "***"},
		{"Authorization", "Bearer abc.def", "***"},
		{"access_token", "abc", "***"},
		{"nik", "3271234567890001", "***"},
		{"id_card_number", "3271234567890001", "***"},
		{"email", "nasabah@example.com", "***"},
		{"phone_number", "081234567890", "***"},
		{"address", "Jl. Merdeka 1", "***"},
	}

	for _, c := range cases {
		if got := observability.Redact(c.key, c.value); got != c.want {
			t.Errorf("Redact(%q) = %q, mau %q", c.key, got, c.want)
		}
	}
}

func TestRedactKeepsAccountNumberTraceable(t *testing.T) {
	got := observability.Redact("account_number", "0011010000000017")
	if got != "************0017" {
		t.Fatalf("nomor rekening harus menyisakan 4 digit terakhir, dapat %q", got)
	}
}

func TestRedactLeavesOrdinaryValues(t *testing.T) {
	if got := observability.Redact("status", "ACTIVE"); got != "ACTIVE" {
		t.Fatalf("nilai biasa tidak boleh diubah, dapat %q", got)
	}
}

func TestIsSensitiveKeyIsCaseInsensitive(t *testing.T) {
	if !observability.IsSensitiveKey("CUSTOMER_EMAIL") {
		t.Fatal("nama field huruf besar harus dikenali sebagai sensitif")
	}
	if observability.IsSensitiveKey("branch_code") {
		t.Fatal("branch_code bukan data sensitif")
	}
}
