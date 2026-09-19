package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Batas bawah saldo: rekening nasabah tidak boleh negatif, akun GL internal boleh.
// Akun kontra seperti cadangan PPAP (10900) memang bersaldo negatif menurut normal
// balance-nya, sehingga pengecualian GL bukan kelonggaran melainkan keharusan.
func TestAccountBalanceFloorBreached(t *testing.T) {
	cases := []struct {
		name        string
		accountType domain.AccountType
		newBalance  string
		want        bool
	}{
		{"tabungan negatif", domain.AccountTypeSavings, "-0.0001", true},
		{"tabungan nol", domain.AccountTypeSavings, "0", false},
		{"tabungan positif", domain.AccountTypeSavings, "150000", false},
		{"giro negatif", domain.AccountTypeChecking, "-1", true},
		{"kredit negatif", domain.AccountTypeLoan, "-250000", true},
		{"kredit nol", domain.AccountTypeLoan, "0", false},
		{"gl internal negatif", domain.AccountTypeInternalGL, "-999999999", false},
		{"gl internal positif", domain.AccountTypeInternalGL, "1000", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			balance, err := decimal.NewFromString(tc.newBalance)
			if err != nil {
				t.Fatalf("nilai uji tidak valid: %v", err)
			}
			if got := domain.AccountBalanceFloorBreached(tc.accountType, balance); got != tc.want {
				t.Fatalf("AccountBalanceFloorBreached(%s, %s) = %v, ingin %v", tc.accountType, balance, got, tc.want)
			}
		})
	}
}
