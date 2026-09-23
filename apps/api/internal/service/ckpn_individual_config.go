package service

import (
	"context"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi CKPN individual TAHAP T0 (keputusan panel butir 4). Semuanya
// di-seed migrasi 000094 dengan bawaan aman; TIDAK ada perhitungan yang membacanya
// untuk mengubah required_ckpn. Pembaca ini adalah kerangka T0: ia mengubah nilai
// konfigurasi menjadi wadah kebijakan yang siap dipakai tahap T1+.
const (
	cfgCKPNIndividualEnabled                   = "ckpn.individual.enabled"
	cfgCKPNIndividualSignificanceAmount        = "ckpn.individual.significance_amount"
	cfgCKPNIndividualSignificanceTopN          = "ckpn.individual.significance_top_n"
	cfgCKPNIndividualMethod                    = "ckpn.individual.method"
	cfgCKPNIndividualDiscountRateAnnualPct     = "ckpn.individual.discount_rate_annual_pct"
	cfgCKPNIndividualMandatoryOnMacet          = "ckpn.individual.mandatory_on_macet"
	cfgCKPNIndividualMandatoryOnRestructured   = "ckpn.individual.mandatory_on_restructured"
	cfgCKPNIndividualMandatoryDPDDays          = "ckpn.individual.mandatory_dpd_days"
	cfgCKPNIndividualMandatoryOnCollateralDrop = "ckpn.individual.mandatory_on_collateral_drop"
	cfgCKPNIndividualMandatoryOnObjectiveEvi   = "ckpn.individual.mandatory_on_objective_evidence"
)

// CKPNIndividualDefaults adalah bawaan kerangka T0 bila konfigurasi tidak terbaca.
// Nilainya sama dengan seed migrasi 000094 dan TIDAK dianggap keputusan bank.
var (
	ckpnIndividualDefaultSignificanceAmount = domain.CKPNIndividualPolicy{
		Method:           domain.CKPNIndividualMethodMax,
		SignificanceTopN: 20,
	}
)

// ckpnIndividualPolicy membaca konfigurasi T0 CKPN individual menjadi wadah kebijakan.
// Fungsi ini TIDAK menghitung apa pun dan TIDAK mengubah state: nilainya hanya dipakai
// tahap berikutnya setelah bank menyalakan ckpn.individual.enabled.
func ckpnIndividualPolicy(ctx context.Context, config domain.SystemConfigService) domain.CKPNIndividualPolicy {
	p := domain.CKPNIndividualPolicy{
		Method:                       domain.CKPNIndividualMethodMax,
		SignificanceTopN:             ckpnIndividualDefaultSignificanceAmount.SignificanceTopN,
		MandatoryDPDDays:             90,
		MandatoryOnMacet:             true,
		MandatoryOnRestructured:      true,
		MandatoryOnCollateralDrop:    true,
		MandatoryOnObjectiveEvidence: true,
	}
	if config == nil {
		return p
	}
	p.Enabled = config.GetBool(ctx, cfgCKPNIndividualEnabled, false)
	p.SignificanceAmount = configDecimalOr(ctx, config, cfgCKPNIndividualSignificanceAmount, decimalSignificanceFloor())
	p.SignificanceTopN = configIntOr(ctx, config, cfgCKPNIndividualSignificanceTopN, p.SignificanceTopN)
	if method := domain.CKPNIndividualMethod(strings.ToUpper(strings.TrimSpace(
		configStringOr(ctx, config, cfgCKPNIndividualMethod, string(domain.CKPNIndividualMethodMax))))); method.Valid() {
		p.Method = method
	}
	// Override dibiarkan mentah: pemakaiannya (dan penolakan ErrEIRMissing) terjadi di
	// tahap T1 lewat domain.CKPNIndividualDiscountRateMonthly.
	p.DiscountRateAnnualPct = strings.TrimSpace(configStringOr(ctx, config, cfgCKPNIndividualDiscountRateAnnualPct, ""))
	p.MandatoryOnMacet = config.GetBool(ctx, cfgCKPNIndividualMandatoryOnMacet, p.MandatoryOnMacet)
	p.MandatoryOnRestructured = config.GetBool(ctx, cfgCKPNIndividualMandatoryOnRestructured, p.MandatoryOnRestructured)
	p.MandatoryDPDDays = configIntOr(ctx, config, cfgCKPNIndividualMandatoryDPDDays, p.MandatoryDPDDays)
	p.MandatoryOnCollateralDrop = config.GetBool(ctx, cfgCKPNIndividualMandatoryOnCollateralDrop, p.MandatoryOnCollateralDrop)
	p.MandatoryOnObjectiveEvidence = config.GetBool(ctx, cfgCKPNIndividualMandatoryOnObjectiveEvi, p.MandatoryOnObjectiveEvidence)
	return p
}

// ckpnIndividualDefaultSignificanceAmountValue adalah bawaan ambang signifikansi
// (Rp1.000.000.000) yang sama dengan seed 000094. Dipisah dari struct agar nol yang
// sengaja diisi bank tidak tertukar dengan bawaan.
func decimalSignificanceFloor() decimal.Decimal {
	return decimal.NewFromInt(1000000000)
}
