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

// pablCollectivePlacement membangun penempatan sah untuk uji mesin kolektif.
func pablCollectivePlacement(outstanding, guaranteed int64, k LPSPlacementCollectibility) LPSPlacement {
	return LPSPlacement{
		COACode:          "10200",
		CounterpartyBank: "Bank Y",
		PlacementType:    LPSPlacementDeposito,
		Outstanding:      decimal.NewFromInt(outstanding),
		LPSGuaranteed:    decimal.NewFromInt(guaranteed),
		Collectibility:   k,
	}
}

// pablParamsOK adalah parameter fraksi sah: PD 1%/10%/50%, LGD 50%.
func pablParamsOK() PABLCKPNParams {
	return PABLCKPNParams{
		PDFracGol1: decimal.NewFromFloat(0.01),
		PDFracGol3: decimal.NewFromFloat(0.10),
		PDFracGol5: decimal.NewFromFloat(0.50),
		LGDFrac:    decimal.NewFromFloat(0.5),
	}
}

// PD dipilih menurut kualitas dan dasar dikurangi bagian yang dijamin LPS: 8 juta.
func TestCalculatePABLCKPNCollective_PDDanPengurangLPS(t *testing.T) {
	kasus := []struct {
		nama string
		kol  LPSPlacementCollectibility
		want decimal.Decimal
	}{
		{"Lancar pakai golongan 1", LPSLancar, decimal.NewFromInt(40_000)},
		{"Kurang Lancar pakai golongan 3", LPSKurangLancar, decimal.NewFromInt(400_000)},
		{"Macet pakai golongan 5", LPSMacet, decimal.NewFromInt(2_000_000)},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			got, err := CalculatePABLCKPNCollective(
				pablCollectivePlacement(10_000_000, 2_000_000, k.kol), pablParamsOK())
			if err != nil {
				t.Fatalf("CalculatePABLCKPNCollective: %v", err)
			}
			if !got.Equal(k.want) {
				t.Fatalf("target %s, mau %s", got, k.want)
			}
		})
	}
}

// AsetBaikBentukCKPN true memakai outstanding penuh tanpa mengurangi jaminan LPS.
func TestCalculatePABLCKPNCollective_AsetBaikOutstandingPenuh(t *testing.T) {
	params := pablParamsOK()
	params.AsetBaikBentukCKPN = true
	got, err := CalculatePABLCKPNCollective(
		pablCollectivePlacement(10_000_000, 2_000_000, LPSLancar), params)
	if err != nil {
		t.Fatalf("CalculatePABLCKPNCollective: %v", err)
	}
	// 10 juta x 1% x 50% = 50.000 (bukan 40.000 yang dikurangi jaminan).
	if !got.Equal(decimal.NewFromInt(50_000)) {
		t.Fatalf("target %s, mau 50000", got)
	}
}

// Fraksi yang kosong/nol/negatif/di atas 1 ditolak dengan ErrPABLCKPNParameterInvalid,
// bukan dijadikan nol.
func TestCalculatePABLCKPNCollective_MenolakFraksiTidakSah(t *testing.T) {
	kasus := []struct {
		nama string
		ubah func(*PABLCKPNParams)
	}{
		{"PD golongan 1 nol", func(p *PABLCKPNParams) { p.PDFracGol1 = decimal.Zero }},
		{"PD golongan 1 negatif", func(p *PABLCKPNParams) { p.PDFracGol1 = decimal.NewFromInt(-1) }},
		{"PD golongan 1 di atas 1", func(p *PABLCKPNParams) { p.PDFracGol1 = decimal.NewFromInt(2) }},
		{"LGD nol", func(p *PABLCKPNParams) { p.LGDFrac = decimal.Zero }},
		{"LGD negatif", func(p *PABLCKPNParams) { p.LGDFrac = decimal.NewFromFloat(-0.1) }},
		{"LGD di atas 1", func(p *PABLCKPNParams) { p.LGDFrac = decimal.NewFromFloat(1.5) }},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			params := pablParamsOK()
			k.ubah(&params)
			_, err := CalculatePABLCKPNCollective(
				pablCollectivePlacement(10_000_000, 2_000_000, LPSLancar), params)
			if !errors.Is(err, ErrPABLCKPNParameterInvalid) {
				t.Fatalf("error %v, mau ErrPABLCKPNParameterInvalid", err)
			}
		})
	}
}

// Penempatan tidak sah ditolak lebih dulu agar tidak dihitung dari data kosong.
func TestCalculatePABLCKPNCollective_MenolakPenempatanTidakSah(t *testing.T) {
	p := pablCollectivePlacement(1_000_000, 0, LPSLancar)
	p.CounterpartyBank = ""
	if _, err := CalculatePABLCKPNCollective(p, pablParamsOK()); !errors.Is(err, ErrLPSPlacementInvalid) {
		t.Fatalf("error %v, mau ErrLPSPlacementInvalid", err)
	}
}

// Pembulatan memakai RoundToRupiah (Banker's Rounding), sama seperti CalculateCKPN:
// 100 x 33% x 50% = 16,5 dibulatkan ke bilangan genap 16.
func TestCalculatePABLCKPNCollective_PembulatanRoundToRupiah(t *testing.T) {
	params := pablParamsOK()
	params.PDFracGol1 = decimal.NewFromFloat(0.33)
	got, err := CalculatePABLCKPNCollective(
		pablCollectivePlacement(100, 0, LPSLancar), params)
	if err != nil {
		t.Fatalf("CalculatePABLCKPNCollective: %v", err)
	}
	if !got.Equal(decimal.NewFromInt(16)) {
		t.Fatalf("target %s, mau 16 (RoundToRupiah)", got)
	}
}
