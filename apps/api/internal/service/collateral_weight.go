package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// CollateralWeightActivationApproval adalah alias tipe domain. Disediakan agar
// pemanggil lama (dan uji) tetap memakai service.CollateralWeightActivationApproval
// setelah tipe dipindah ke domain demi antarmuka domain.CollateralWeightService.
type CollateralWeightActivationApproval = domain.CollateralWeightActivationApproval

type collateralWeightService struct {
	repo      domain.CollateralWeightRepository
	config    domain.SystemConfigService
	auditRepo domain.AuditRepository
}

// NewCollateralWeightService menyusun gerbang aktivasi. auditRepo opsional (variadic)
// agar pemanggil dan uji lama tetap berjalan; tanpa itu aktivasi tidak menulis audit.
func NewCollateralWeightService(repo domain.CollateralWeightRepository, config domain.SystemConfigService, audit ...domain.AuditRepository) domain.CollateralWeightService {
	var auditRepo domain.AuditRepository
	if len(audit) > 0 {
		auditRepo = audit[0]
	}
	return &collateralWeightService{repo: repo, config: config, auditRepo: auditRepo}
}

// gateInputs membaca kategori dan agunan, lalu menyusun masukan gerbang yang sama
// untuk penilaian maupun aktivasi. storedFallback true (endpoint baca) memakai
// kebijakan Direksi yang sudah tersimpan bila pemanggil tidak mengirimkannya; false
// (aktivasi) menuntut pemanggil menyebutkannya, supaya aktivasi tidak lolos C8 diam-diam.
func (s *collateralWeightService) gateInputs(
	ctx context.Context,
	categoryCode string,
	approval domain.CollateralWeightActivationApproval,
	storedFallback bool,
) (domain.CollateralWeightActivationRequest, error) {
	category, err := s.repo.GetCategory(ctx, categoryCode)
	if err != nil {
		return domain.CollateralWeightActivationRequest{}, err
	}
	collaterals, exposure, err := s.repo.ListActiveCollateralsWithExposure(ctx)
	if err != nil {
		return domain.CollateralWeightActivationRequest{}, err
	}
	policy := collateralWeightActivationPolicy(ctx, s.config)
	// Umur taksasi mengikuti kebijakan bank (kunci yang sama dengan MissingLampiranIIFields),
	// bukan bawaan kode, agar C3 menilai kedaluwarsa persis seperti penjagaan lain.
	validityMonths := domain.DefaultAppraisalValidityMonths
	if s.config != nil {
		validityMonths = domain.NormalizeAppraisalValidityMonths(
			s.config.GetInt(ctx, domain.AppraisalValidityMonthsConfigKey, domain.DefaultAppraisalValidityMonths))
	}

	policyNumber := strings.TrimSpace(approval.DireksiPolicyNumber)
	policyDate := approval.DireksiPolicyDate
	maker := strings.TrimSpace(approval.Maker)
	checker := strings.TrimSpace(approval.Checker)
	if storedFallback {
		if policyNumber == "" {
			policyNumber = category.DireksiPolicyNumber
		}
		if policyDate == nil {
			policyDate = category.DireksiPolicyDate
		}
		// Identitas maker/checker tidak boleh datang dari kueri. Untuk penilaian
		// baca-saja, keduanya dibaca dari jejak yang SUDAH tersimpan pada kategori
		// (hasil aktivasi sah sebelumnya), bukan dari pemanggil.
		if maker == "" {
			maker = strings.TrimSpace(category.ActivatedMaker)
		}
		if checker == "" {
			checker = strings.TrimSpace(category.ActivatedBy)
		}
	}

	applied := category.OfficialWeightFrac
	if approval.AppliedWeightFrac != nil {
		applied = *approval.AppliedWeightFrac
	}

	return domain.CollateralWeightActivationRequest{
		Category:             *category,
		CandidateAppliedFrac: applied,
		Collaterals:          collaterals,
		LoanExposure:         exposure,
		AsOf:                 time.Now().UTC(),
		ValidityMonths:       validityMonths,
		ShadowMonthsRequired: policy.ShadowMonths,
		CoverageMinFrac:      policy.CoverageMinFrac,
		DireksiPolicyNumber:  policyNumber,
		DireksiPolicyDate:    policyDate,
		Maker:                maker,
		Checker:              checker,
		HasConfigPermission:  approval.HasConfigPermission,
	}, nil
}

// Assess menilai seluruh syarat C1-C9 dan gerbang tambahan TANPA mengubah apa pun.
// Hasilnya menyebut syarat yang gagal, cakupan nilai agunan, dan umur mode bayangan,
// sehingga gerbang dapat diaudit dari API.
func (s *collateralWeightService) Assess(
	ctx context.Context,
	categoryCode string,
	approval domain.CollateralWeightActivationApproval,
	actor domain.Actor,
) (domain.CollateralWeightActivationResult, error) {
	req, err := s.gateInputs(ctx, categoryCode, approval, true)
	if err != nil {
		return domain.CollateralWeightActivationResult{}, err
	}
	return domain.EvaluateCollateralWeightActivation(req), nil
}

// ExecuteApproved menjalankan gerbang aktivasi saat pengajuan maker-checker disetujui,
// di dalam transaksi milik maker-checker service sehingga keputusan dan efeknya commit
// bersama. Identitas diambil dari sumber terautentikasi: pembuat dari payload yang
// diisi server-side saat pengajuan (CreateRequest), penyetuju dari aktor yang
// menyetujui. Keduanya tidak pernah dibaca dari body/kueri permintaan aktivasi.
func (s *collateralWeightService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	if normalizeAction(actionType) != domain.CollateralWeightActivateAction {
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("transaksi persetujuan aktivasi bobot agunan tidak valid")
	}
	categoryCode := strings.TrimSpace(stringPayload(payload, "category_code"))
	if categoryCode == "" {
		return errors.New("pengajuan aktivasi bobot agunan tanpa kategori")
	}

	approval := domain.CollateralWeightActivationApproval{
		DireksiPolicyNumber: stringPayload(payload, "direksi_policy_number"),
		Maker:               stringPayload(payload, "maker_username"),
		// Penyetuju adalah aktor terautentikasi yang menyetujui, bukan nama bebas.
		Checker:             actor.DisplayName(),
		HasConfigPermission: true,
	}
	if raw := strings.TrimSpace(stringPayload(payload, "direksi_policy_date")); raw != "" {
		d, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return fmt.Errorf("tanggal kebijakan Direksi pada pengajuan tidak sah: %w", err)
		}
		approval.DireksiPolicyDate = &d
	}
	if raw := strings.TrimSpace(stringPayload(payload, "applied_weight_frac")); raw != "" {
		d, err := decimal.NewFromString(raw)
		if err != nil {
			return fmt.Errorf("applied_weight_frac pada pengajuan tidak sah: %w", err)
		}
		approval.AppliedWeightFrac = &d
	}

	req, err := s.gateInputs(ctx, categoryCode, approval, false)
	if err != nil {
		return err
	}
	result := domain.EvaluateCollateralWeightActivation(req)
	if err := domain.CollateralWeightActivationRejection(result); err != nil {
		return err
	}

	if err := s.repo.EnableCategoryTx(ctx, sqlTx, categoryCode, domain.CollateralWeightApproval{
		AppliedWeightFrac:   req.CandidateAppliedFrac,
		DireksiPolicyNumber: strings.TrimSpace(req.DireksiPolicyNumber),
		DireksiPolicyDate:   req.DireksiPolicyDate,
		Maker:               strings.TrimSpace(req.Maker),
		Checker:             strings.TrimSpace(req.Checker),
		ActivatedAt:         req.AsOf,
	}); err != nil {
		return err
	}

	// Audit ikut transaksi persetujuan. Maker dan checker diambil dari jejak gerbang
	// (yang berasal dari token) agar identitas yang tercatat adalah yang BENAR (C9).
	return writeAudit(ctx, s.auditRepo, sqlTx, actor, "COLLATERAL_WEIGHT_ACTIVATED", "collateral_weight_category", categoryCode, map[string]any{
		"applied_weight_frac":   req.CandidateAppliedFrac.String(),
		"direksi_policy_number": strings.TrimSpace(req.DireksiPolicyNumber),
		"maker":                 strings.TrimSpace(req.Maker),
		"checker":               strings.TrimSpace(req.Checker),
		"coverage_frac":         result.CoverageFrac.String(),
	})
}

// stringPayload membaca nilai string dari payload maker-checker. Payload diserialkan
// ke JSONB, sehingga nilai non-string tetap dikembalikan kosong, bukan panic.
func stringPayload(payload map[string]any, key string) string {
	v, _ := payload[key].(string)
	return strings.TrimSpace(v)
}

var _ domain.CollateralWeightService = (*collateralWeightService)(nil)
