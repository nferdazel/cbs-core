package service

import (
	"context"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// Peringatan pemetaan akun CKPN syariah harus muncul pada SETIAP cakupan yang
// benar-benar memproses pembiayaan syariah — SYARIAH maupun DUAL — bukan hanya
// SYARIAH. DUAL adalah nilai awal produksi (migrasi 000074), jadi memeriksa SYARIAH
// saja membuat peringatan ini tidak pernah muncul di instalasi nyata.
func TestInstallationValidationWarningsAkunCKPNSyariah(t *testing.T) {
	base := map[string]string{
		domain.ConfigKeyInstitutionBookScope: string(domain.ScopeDual),
	}

	cases := []struct {
		name   string
		values map[string]string
		want   int
	}{
		{
			name:   "KONVENSIONAL tidak perlu akun syariah",
			values: map[string]string{domain.ConfigKeyInstitutionBookScope: string(domain.ScopeConventional)},
			want:   0,
		},
		{
			name:   "SYARIAH tanpa pemetaan syariah: dua peringatan",
			values: map[string]string{domain.ConfigKeyInstitutionBookScope: string(domain.ScopeSyariah)},
			want:   2,
		},
		{
			name:   "DUAL tanpa pemetaan syariah: dua peringatan",
			values: map[string]string{domain.ConfigKeyInstitutionBookScope: string(domain.ScopeDual)},
			want:   2,
		},
		{
			name:   "cakupan kosong diperlakukan DUAL: dua peringatan",
			values: map[string]string{},
			want:   2,
		},
		{
			name: "DUAL dengan pemetaan lengkap: tanpa peringatan",
			values: map[string]string{
				domain.ConfigKeyInstitutionBookScope: string(domain.ScopeDual),
				"ckpn.coa.expense.syariah":           "15901",
				"ckpn.coa.reserve.syariah":           "11950",
			},
			want: 0,
		},
		{
			name: "DUAL hanya cadangan terisi: peringatan beban saja",
			values: map[string]string{
				domain.ConfigKeyInstitutionBookScope: string(domain.ScopeDual),
				"ckpn.coa.reserve.syariah":           "11950",
			},
			want: 1,
		},
		{
			name: "nilai hanya spasi dianggap kosong",
			values: map[string]string{
				domain.ConfigKeyInstitutionBookScope: string(domain.ScopeDual),
				"ckpn.coa.expense.syariah":           "   ",
				"ckpn.coa.reserve.syariah":           "11950",
			},
			want: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			values := tc.values
			if values == nil {
				values = base
			}
			got := InstallationValidationWarnings(context.Background(), &ckpnConfigStub{values: values})
			if len(got) != tc.want {
				t.Fatalf("peringatan %d, mau %d: %v", len(got), tc.want, got)
			}
			for _, w := range got {
				if !strings.Contains(w, "CKPN") {
					t.Fatalf("peringatan tidak menyebut CKPN: %q", w)
				}
			}
		})
	}

	if got := InstallationValidationWarnings(context.Background(), nil); got != nil {
		t.Fatalf("cfg nil harus tanpa peringatan, dapat %v", got)
	}
}
