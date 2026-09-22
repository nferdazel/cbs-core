package domain

import (
	"errors"
	"strings"
	"testing"
)

// validEODDefinitions adalah himpunan definisi yang sah: bentuk seed migrasi 000077.
func validEODDefinitions() []EODStepDefinition {
	return []EODStepDefinition{
		{Code: "aro", Sequence: 10, Enabled: true, Category: "OPERASIONAL"},
		{Code: "loan_interest_accrual", Sequence: 20, Enabled: true, Core: true, Category: "AKUNTANSI"},
		{Code: "restructure_loss_amortization", Sequence: 30, Enabled: true, Core: true, Category: "AKUNTANSI",
			Prerequisites: []string{"loan_interest_accrual"}},
		{Code: "ppap", Sequence: 40, Enabled: true, Core: true, Category: "AKUNTANSI",
			Prerequisites: []string{"restructure_loss_amortization"}},
		{Code: "ckpn_comparison", Sequence: 50, Enabled: true, Core: true, Category: "PELAPORAN",
			Prerequisites: []string{"ppap"}},
		{Code: "loan_penalty_accrual", Sequence: 60, Enabled: true, Category: "AKUNTANSI"},
		{Code: "dormant", Sequence: 70, Enabled: true, Category: "OPERASIONAL"},
	}
}

func withDef(t *testing.T, defs []EODStepDefinition, mutate func([]EODStepDefinition) []EODStepDefinition) []EODStepDefinition {
	t.Helper()
	out := make([]EODStepDefinition, len(defs))
	copy(out, defs)
	return mutate(out)
}

func findDef(defs []EODStepDefinition, code string) *EODStepDefinition {
	for i := range defs {
		if defs[i].Code == code {
			return &defs[i]
		}
	}
	return nil
}

func TestValidateEODStepDefinitions_AcceptsSeedShape(t *testing.T) {
	if err := ValidateEODStepDefinitions(validEODDefinitions()); err != nil {
		t.Fatalf("definisi seed harus sah, dapat: %v", err)
	}
}

func TestValidateEODStepDefinitions_Rejects(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition
		wantErr error
		wantMsg string
	}{
		{
			name:    "urutan ganda",
			wantErr: ErrEODDuplicateSequence,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "dormant").Sequence = 10
				return defs
			},
		},
		{
			name:    "urutan kosong",
			wantErr: ErrEODInvalidSequence,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "dormant").Sequence = 0
				return defs
			},
		},
		{
			name:    "urutan negatif",
			wantErr: ErrEODInvalidSequence,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "dormant").Sequence = -1
				return defs
			},
		},
		{
			name:    "prasyarat tidak dikenal",
			wantErr: ErrEODUnknownPrerequisite,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "ppap").Prerequisites = []string{"tidak-ada"}
				return defs
			},
		},
		{
			name:    "prasyarat siklus",
			wantErr: ErrEODPrerequisiteCycle,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "loan_interest_accrual").Prerequisites = []string{"ppap"}
				return defs
			},
		},
		{
			name:    "langkah aktif bergantung langkah nonaktif",
			wantErr: ErrEODActiveDependsOnDisabled,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "loan_interest_accrual").Enabled = false
				// akrual nonaktif, tetapi amortisasi (aktif) tetap memerlukannya.
				return defs
			},
		},
		{
			name:    "langkah inti hilang",
			wantErr: ErrEODCoreStepRequired,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				// Hapus PPAP sekaligus lepaskan prasyarat CKPN agar error yang muncul
				// benar-benar tentang langkah inti yang hilang.
				findDef(defs, "ckpn_comparison").Prerequisites = nil
				out := make([]EODStepDefinition, 0, len(defs)-1)
				for _, def := range defs {
					if def.Code != "ppap" {
						out = append(out, def)
					}
				}
				return out
			},
		},
		{
			name:    "langkah inti nonaktif",
			wantErr: ErrEODCoreStepDisabled,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				// Nonaktifkan inti sekaligus cabut dependennya agar error yang muncul
				// benar-benar tentang inti yang nonaktif.
				findDef(defs, "ppap").Enabled = false
				findDef(defs, "ckpn_comparison").Enabled = false
				return defs
			},
		},
		{
			name:    "langkah inti kehilangan penanda core",
			wantErr: ErrEODCoreStepRequired,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				findDef(defs, "ppap").Core = false
				return defs
			},
		},
		{
			name:    "kode tidak dikenal mesin",
			wantErr: ErrEODDefinitionsUnavailable,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				defs[0].Code = "langkah_karangan"
				return defs
			},
		},
		{
			name:    "definisi kosong",
			wantErr: ErrEODDefinitionsUnavailable,
			mutate: func(t *testing.T, defs []EODStepDefinition) []EODStepDefinition {
				return nil
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defs := withDef(t, validEODDefinitions(), func(d []EODStepDefinition) []EODStepDefinition {
				return tc.mutate(t, d)
			})
			err := ValidateEODStepDefinitions(defs)
			if err == nil {
				t.Fatalf("mau error %v, dapat nil", tc.wantErr)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("mau error %v, dapat: %v", tc.wantErr, err)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("pesan %q tidak memuat %q", err.Error(), tc.wantMsg)
			}
		})
	}
}

func TestSortEODStepDefinitions(t *testing.T) {
	defs := validEODDefinitions()
	defs[0], defs[len(defs)-1] = defs[len(defs)-1], defs[0]
	sorted := SortEODStepDefinitions(defs)
	if sorted[0].Code != "aro" || sorted[len(sorted)-1].Code != "dormant" {
		t.Fatalf("urutan salah: %s ... %s", sorted[0].Code, sorted[len(sorted)-1].Code)
	}
	// Salinan: irisan masukan tidak boleh ikut terurut.
	if defs[0].Code == "aro" {
		t.Fatal("SortEODStepDefinitions tidak boleh mengubah irisan masukan")
	}
}
