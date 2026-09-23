package domain_test

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Gerbang aktivasi bobot agunan (keputusan panel butir 3): sembilan syarat C1-C9
// ditambah mode bayangan dan cakupan. Uji ini murni (tanpa database) dan membuktikan
// tiap syarat benar-benar diperiksa mesin.

func cwDec(v string) decimal.Decimal {
	d, err := decimal.NewFromString(v)
	if err != nil {
		panic(err)
	}
	return d
}

func cwTimePtr(t time.Time) *time.Time { return &t }

func cwHasFailure(res domain.CollateralWeightActivationResult, kode string) bool {
	for _, f := range res.Failures {
		if strings.HasPrefix(f, kode+":") {
			return true
		}
	}
	return false
}

// validWeightRequest membentuk permintaan yang MEMENUHI seluruh syarat, sebagai basis
// untuk membuktikan tiap pelanggaran menghasilkan kegagalan yang tepat.
func validWeightRequest() domain.CollateralWeightActivationRequest {
	now := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	loanID := uuid.New()
	ct := domain.CollateralTanahBangunan
	bt := domain.BindingHakTanggungan
	ins := now.AddDate(0, 6, 0)
	return domain.CollateralWeightActivationRequest{
		Category: domain.CollateralWeightCategory{
			CategoryCode:       "TANAH_BANGUNAN_HAK_TANGGUNGAN",
			LampiranIIItem:     12,
			OfficialWeightFrac: cwDec("0.30"),
			AppliedWeightFrac:  cwDec("1.0"),
			CollateralType:     &ct,
			BindingType:        &bt,
			ShadowStartedAt:    cwTimePtr(now.AddDate(0, -3, 0)),
		},
		CandidateAppliedFrac: cwDec("0.30"),
		Collaterals: []domain.CollateralWeightCollateral{{
			LoanID:              loanID,
			CollateralType:      ct,
			BindingType:         &bt,
			InsuranceExpiryDate: &ins,
			AppraisalDate:       now.AddDate(0, 0, -5),
			AppraisalValue:      cwDec("100000"),
			BoundAmount:         cwDec("30000"),
			Status:              domain.CollateralActive,
		}},
		LoanExposure:         map[uuid.UUID]decimal.Decimal{loanID: cwDec("200000")},
		AsOf:                 now,
		ValidityMonths:       12,
		ShadowMonthsRequired: 2,
		CoverageMinFrac:      cwDec("0.90"),
		DireksiPolicyNumber:  "SK-DIR-012",
		DireksiPolicyDate:    cwTimePtr(now.AddDate(0, -1, 0)),
		Maker:                "maker.a",
		Checker:              "checker.b",
		HasConfigPermission:  true,
	}
}

func TestCollateralWeightActivation_LolosBilaSembilanSyaratTerpenuhi(t *testing.T) {
	res := domain.EvaluateCollateralWeightActivation(validWeightRequest())
	if !res.Allowed {
		t.Fatalf("sembilan syarat terpenuhi tetapi ditolak: %v", res.Failures)
	}
	if !res.CoverageFrac.Equal(cwDec("1")) {
		t.Fatalf("cakupan %s, mau 1", res.CoverageFrac)
	}
}

func TestCollateralWeightActivation_MenolakTiapSyaratYangGagal(t *testing.T) {
	// Kasus: nama, ubah, kode syarat yang harus muncul.
	kasus := []struct {
		nama string
		ubah func(*domain.CollateralWeightActivationRequest)
		kode string
	}{
		{"C1 binding_type kosong", func(r *domain.CollateralWeightActivationRequest) {
			r.Collaterals[0].BindingType = nil
		}, domain.CollateralWeightC1},
		{"C2 asuransi kedaluwarsa", func(r *domain.CollateralWeightActivationRequest) {
			lampau := r.AsOf.AddDate(0, 0, -1)
			r.Collaterals[0].InsuranceExpiryDate = &lampau
		}, domain.CollateralWeightC2},
		{"C3 taksasi kedaluwarsa", func(r *domain.CollateralWeightActivationRequest) {
			r.Collaterals[0].AppraisalDate = r.AsOf.AddDate(0, -13, 0)
		}, domain.CollateralWeightC3},
		{"C4 agunan sengketa", func(r *domain.CollateralWeightActivationRequest) {
			r.Collaterals[0].Disputed = true
		}, domain.CollateralWeightC4},
		{"C5 bobot resmi kosong", func(r *domain.CollateralWeightActivationRequest) {
			r.Category.OfficialWeightFrac = decimal.Zero
		}, domain.CollateralWeightC5},
		{"C6 pengurang melebihi eksposur", func(r *domain.CollateralWeightActivationRequest) {
			for id := range r.LoanExposure {
				r.LoanExposure[id] = cwDec("10000")
			}
			r.Collaterals[0].BoundAmount = cwDec("50000")
		}, domain.CollateralWeightC6},
		{"C7 applied beda dari official", func(r *domain.CollateralWeightActivationRequest) {
			r.CandidateAppliedFrac = cwDec("0.50")
		}, domain.CollateralWeightC7},
		{"C8 tanpa kebijakan Direksi", func(r *domain.CollateralWeightActivationRequest) {
			r.DireksiPolicyNumber = ""
			r.DireksiPolicyDate = nil
		}, domain.CollateralWeightC8},
		{"C9 maker sama dengan checker", func(r *domain.CollateralWeightActivationRequest) {
			r.Checker = r.Maker
		}, domain.CollateralWeightC9},
		{"SHADOW belum 2 bulan", func(r *domain.CollateralWeightActivationRequest) {
			r.Category.ShadowStartedAt = cwTimePtr(r.AsOf.AddDate(0, 0, -10))
		}, domain.CollateralWeightShadow},
	}

	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			req := validWeightRequest()
			k.ubah(&req)
			res := domain.EvaluateCollateralWeightActivation(req)
			if res.Allowed {
				t.Fatal("permintaan yang melanggar syarat justru diizinkan")
			}
			if !cwHasFailure(res, k.kode) {
				t.Fatalf("syarat %s tidak dilaporkan; failures=%v", k.kode, res.Failures)
			}
		})
	}
}

// Cakupan < 90% menghalangi kategori: satu agunan sengketa bernilai besar menurunkan
// cakupan walau agunan lain lolos.
func TestCollateralWeightActivation_CakupanDiBawah90Menghalangi(t *testing.T) {
	req := validWeightRequest()
	loanID := uuid.New()
	req.Collaterals = append(req.Collaterals, domain.CollateralWeightCollateral{
		LoanID:         loanID,
		CollateralType: *req.Category.CollateralType,
		BindingType:    req.Collaterals[0].BindingType,
		Disputed:       true,
		// Nilai sengketa 300.000 vs layak 100.000 -> cakupan 25%.
		AppraisalValue: cwDec("300000"),
		BoundAmount:    cwDec("0"),
		Status:         domain.CollateralActive,
	})
	req.LoanExposure[loanID] = cwDec("300000")

	res := domain.EvaluateCollateralWeightActivation(req)
	if res.Allowed {
		t.Fatal("cakupan di bawah 90% harus menghalangi aktivasi")
	}
	if !cwHasFailure(res, domain.CollateralWeightCoverage) {
		t.Fatalf("kegagalan COVERAGE tidak dilaporkan: %v", res.Failures)
	}
}

// Data agunan tidak lengkap tetap 100% per agunan: sengketa dan polis kedaluwarsa
// tidak boleh memakai bobot resmi.
func TestCollateralRiskWeightFrac_SengketaDanPolisKedaluwarsaTetap100(t *testing.T) {
	asOf := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	ikatan := domain.BindingHakTanggungan
	ins := asOf.AddDate(0, -1, 0) // polis kedaluwarsa
	taksasiSah := asOf.AddDate(0, 6, 0)
	c := &domain.LoanCollateral{
		LoanID:              uuid.New(),
		CollateralType:      domain.CollateralTanahBangunan,
		DocumentNumber:      "DOC-1",
		OwnerName:           "Pemilik",
		AppraisalValue:      cwDec("100000"),
		AppraisalDate:       asOf.AddDate(0, 0, -5),
		AppraisalValidUntil: &taksasiSah,
		HaircutPercent:      decimal.Zero,
		Status:              domain.CollateralActive,
		BindingType:         &ikatan,
		InsuranceExpiryDate: &ins,
	}
	if got := domain.CollateralRiskWeightFrac(c, asOf, 12, cwDec("0.30")); !got.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("polis kedaluwarsa harus 100%%, dapat %s", got)
	}

	// Polis sah, tetapi agunan sengketa -> tetap 100%.
	insSah := asOf.AddDate(0, 6, 0)
	c.InsuranceExpiryDate = &insSah
	c.Disputed = true
	if got := domain.CollateralRiskWeightFrac(c, asOf, 12, cwDec("0.30")); !got.Equal(decimal.NewFromInt(1)) {
		t.Fatalf("agunan sengketa harus 100%%, dapat %s", got)
	}

	// Lengkap dan tidak sengketa -> memakai bobot resmi.
	c.Disputed = false
	if got := domain.CollateralRiskWeightFrac(c, asOf, 12, cwDec("0.30")); !got.Equal(cwDec("0.30")) {
		t.Fatalf("agunan lengkap harus memakai bobot resmi, dapat %s", got)
	}
}
