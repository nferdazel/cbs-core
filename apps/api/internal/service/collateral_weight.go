package service

import (
	"context"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi gerbang aktivasi bobot agunan (keputusan panel butir 3). Keduanya
// di-seed migrasi 000094 dengan bawaan aman; selama bobot belum diaktifkan, nilai ini
// hanya dipakai gerbang, bukan perhitungan ATMR.
const (
	cfgCollateralWeightShadowMonths    = "collateral.weight.activation.shadow_months"
	cfgCollateralWeightCoverageMinFrac = "collateral.weight.activation.coverage_min_frac"
)

const (
	defaultCollateralWeightShadowMonths    = 2
	defaultCollateralWeightCoverageMinFrac = "0.90"
)

// CollateralWeightActivationPolicy adalah kebijakan gerbang yang dibaca dari konfigurasi.
type CollateralWeightActivationPolicy struct {
	ShadowMonths    int
	CoverageMinFrac decimal.Decimal
}

// collateralWeightActivationPolicy membaca kebijakan gerbang. Nilai tidak masuk akal
// (bulan negatif, cakupan di luar 0..1) jatuh ke bawaan aman, bukan dipakai diam-diam.
func collateralWeightActivationPolicy(ctx context.Context, config domain.SystemConfigService) CollateralWeightActivationPolicy {
	p := CollateralWeightActivationPolicy{
		ShadowMonths:    defaultCollateralWeightShadowMonths,
		CoverageMinFrac: mustDecimal(defaultCollateralWeightCoverageMinFrac),
	}
	if config == nil {
		return p
	}
	if v := config.GetInt(ctx, cfgCollateralWeightShadowMonths, p.ShadowMonths); v >= 0 {
		p.ShadowMonths = v
	}
	if raw := strings.TrimSpace(configStringOr(ctx, config, cfgCollateralWeightCoverageMinFrac, defaultCollateralWeightCoverageMinFrac)); raw != "" {
		if d, err := decimal.NewFromString(raw); err == nil && d.GreaterThanOrEqual(decimal.Zero) && d.LessThanOrEqual(decimal.NewFromInt(1)) {
			p.CoverageMinFrac = d
		}
	}
	return p
}

func mustDecimal(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		panic("konstanta desimal tidak sah: " + s)
	}
	return d
}

// CollateralWeightActivationApproval adalah permintaan aktivasi dari pemanggil.
type CollateralWeightActivationApproval struct {
	DireksiPolicyNumber string
	DireksiPolicyDate   *time.Time
	Maker               string
	Checker             string
	// AppliedWeightFrac nil berarti memakai bobot resmi (C7 lolos); diisi nilai lain
	// akan ditolak gerbang.
	AppliedWeightFrac   *decimal.Decimal
	HasConfigPermission bool
}

// CollateralWeightService adalah gerbang aktivasi bobot agunan. Kategori TIDAK dapat
// dinyalakan bila salah satu syarat C1-C9 / gerbang tambahan gagal.
type CollateralWeightService interface {
	Activate(ctx context.Context, categoryCode string, approval CollateralWeightActivationApproval, actor domain.Actor) (domain.CollateralWeightActivationResult, error)
}

type collateralWeightService struct {
	repo   domain.CollateralWeightRepository
	config domain.SystemConfigService
}

func NewCollateralWeightService(repo domain.CollateralWeightRepository, config domain.SystemConfigService) CollateralWeightService {
	return &collateralWeightService{repo: repo, config: config}
}

// Activate menjalankan gerbang lalu, HANYA bila lolos, menyalakan kategori. Data
// agunan dibaca lewat repository; keputusan tetap di domain agar satu tempat.
func (s *collateralWeightService) Activate(
	ctx context.Context,
	categoryCode string,
	approval CollateralWeightActivationApproval,
	actor domain.Actor,
) (domain.CollateralWeightActivationResult, error) {
	category, err := s.repo.GetCategory(ctx, categoryCode)
	if err != nil {
		return domain.CollateralWeightActivationResult{}, err
	}
	collaterals, exposure, err := s.repo.ListActiveCollateralsWithExposure(ctx)
	if err != nil {
		return domain.CollateralWeightActivationResult{}, err
	}
	policy := collateralWeightActivationPolicy(ctx, s.config)
	// Umur taksasi mengikuti kebijakan bank (kunci yang sama dengan MissingLampiranIIFields),
	// bukan bawaan kode, agar C3 menilai kedaluwarsa persis seperti penjagaan lain.
	validityMonths := domain.DefaultAppraisalValidityMonths
	if s.config != nil {
		validityMonths = domain.NormalizeAppraisalValidityMonths(
			s.config.GetInt(ctx, domain.AppraisalValidityMonthsConfigKey, domain.DefaultAppraisalValidityMonths))
	}

	applied := category.OfficialWeightFrac
	if approval.AppliedWeightFrac != nil {
		applied = *approval.AppliedWeightFrac
	}
	asOf := time.Now().UTC()

	req := domain.CollateralWeightActivationRequest{
		Category:             *category,
		CandidateAppliedFrac: applied,
		Collaterals:          collaterals,
		LoanExposure:         exposure,
		AsOf:                 asOf,
		ValidityMonths:       validityMonths,
		ShadowMonthsRequired: policy.ShadowMonths,
		CoverageMinFrac:      policy.CoverageMinFrac,
		DireksiPolicyNumber:  approval.DireksiPolicyNumber,
		DireksiPolicyDate:    approval.DireksiPolicyDate,
		Maker:                approval.Maker,
		Checker:              approval.Checker,
		HasConfigPermission:  approval.HasConfigPermission,
	}
	result := domain.EvaluateCollateralWeightActivation(req)
	if err := domain.CollateralWeightActivationRejection(result); err != nil {
		return result, err
	}

	if err := s.repo.EnableCategory(ctx, categoryCode, domain.CollateralWeightApproval{
		AppliedWeightFrac:   applied,
		DireksiPolicyNumber: strings.TrimSpace(approval.DireksiPolicyNumber),
		DireksiPolicyDate:   approval.DireksiPolicyDate,
		Maker:               strings.TrimSpace(approval.Maker),
		Checker:             strings.TrimSpace(approval.Checker),
		ActivatedAt:         asOf,
	}); err != nil {
		return result, err
	}
	return result, nil
}

var _ CollateralWeightService = (*collateralWeightService)(nil)
