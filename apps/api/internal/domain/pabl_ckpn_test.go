package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// Sandi kolom XXI Form 05.00: INDIVIDUAL_* = "1", COLLECTIVE/EXCLUDED_ASET_BAIK/kosong
// = "2" — cermin sandiJenisCKPN Form 06.00.
func TestSandiJenisCKPNPABL(t *testing.T) {
	kasus := []struct {
		method PABLCKPNMethod
		want   string
	}{
		{PABLCKPNMethodCollective, "2"},
		{PABLCKPNMethodIndividualDCF, "1"},
		{PABLCKPNMethodIndividualCollateral, "1"},
		{PABLCKPNMethodIndividualMax, "1"},
		{PABLCKPNMethodExcludedAsetBaik, "2"},
		{"", "2"},
		{"TIDAK_DIKENAL", "2"},
	}
	for _, k := range kasus {
		if got := SandiJenisCKPNPABL(k.method); got != k.want {
			t.Errorf("SandiJenisCKPNPABL(%q) = %q, ingin %q", k.method, got, k.want)
		}
	}
}

func TestPABLCKPNMethodValid(t *testing.T) {
	for _, m := range []PABLCKPNMethod{
		PABLCKPNMethodCollective, PABLCKPNMethodIndividualDCF,
		PABLCKPNMethodIndividualCollateral, PABLCKPNMethodIndividualMax,
		PABLCKPNMethodExcludedAsetBaik,
	} {
		if !m.Valid() {
			t.Errorf("metode %q harus valid", m)
		}
	}
	for _, m := range []PABLCKPNMethod{"", "DCF", "individual_dcf"} {
		if m.Valid() {
			t.Errorf("metode %q tidak boleh valid", m)
		}
	}
}

func pablInputValid() PABLCKPNInput {
	return PABLCKPNInput{
		Method:            PABLCKPNMethodIndividualDCF,
		Significant:       true,
		ObjectiveEvidence: true,
		RequiredCKPN:      decimal.NewFromInt(250_000),
		IndividualTarget:  decimal.NewFromInt(250_000),
		AsOf:              time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
	}
}

func TestPABLCKPNInputValidate(t *testing.T) {
	if err := pablInputValid().Validate(); err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}

	// Target individual nol TIDAK ditolak: perhitungan dapat menghasilkan nol.
	nol := pablInputValid()
	nol.IndividualTarget = decimal.Zero
	if err := nol.Validate(); err != nil {
		t.Fatalf("target individual nol ditolak: %v", err)
	}

	kasus := []struct {
		nama  string
		ubah  func(*PABLCKPNInput)
		pesan string
	}{
		{"metode tak dikenal", func(in *PABLCKPNInput) { in.Method = "DCF" }, "tidak dikenal"},
		{"CKPN negatif", func(in *PABLCKPNInput) { in.RequiredCKPN = decimal.NewFromInt(-1) }, "tidak boleh negatif"},
		{"target individu negatif", func(in *PABLCKPNInput) { in.IndividualTarget = decimal.NewFromInt(-1) }, "tidak boleh negatif"},
		{"tanggal nol", func(in *PABLCKPNInput) { in.AsOf = time.Time{} }, "tanggal asesmen wajib"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			in := pablInputValid()
			k.ubah(&in)
			err := in.Validate()
			if !errors.Is(err, ErrCKPNPABLInputInvalid) {
				t.Fatalf("error %v, mau ErrCKPNPABLInputInvalid", err)
			}
			if !strings.Contains(err.Error(), k.pesan) {
				t.Fatalf("pesan %q tidak memuat %q", err, k.pesan)
			}
		})
	}
}
