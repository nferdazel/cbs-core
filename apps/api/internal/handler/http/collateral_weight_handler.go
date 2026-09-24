package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
)

// CollateralWeightHandler menyajikan gerbang aktivasi bobot risiko agunan (Lampiran II
// SEOJK No. 2/SEOJK.03/2025). Penilaian baca-saja untuk audit; aktivasi diajukan lewat
// antrean maker-checker dan baru menyalakan kategori setelah penyetuju LAIN menyetujui,
// sehingga seluruh syarat C1-C9 (termasuk dua orang berbeda) ditegakkan tanpa memercayai
// identitas dari body/kueri. Bobot TETAP MATI BAWAAN: hanya kategori yang lolos gerbang
// yang dinyalakan.
type CollateralWeightHandler struct {
	svc domain.CollateralWeightService
	// mc adalah antrean maker-checker yang sama dengan alur persetujuan lain di bank.
	// Pengaju (maker) tercatat dari token saat pengajuan; penyetuju (checker) dari token
	// saat menyetujui; pemisahan tugas ditegakkan service maker-checker.
	mc domain.MakerCheckerService
}

func NewCollateralWeightHandler(svc domain.CollateralWeightService, mc domain.MakerCheckerService) *CollateralWeightHandler {
	return &CollateralWeightHandler{svc: svc, mc: mc}
}

// RegisterRoutes memasang rute gerbang bobot agunan. Penilaian baca-saja cukup izin
// baca konfigurasi sistem agar pengawas dapat mengaudit tanpa wewenang menulis;
// aktivasi hanya lewat izin system:config, sesuai syarat C9.
func (h *CollateralWeightHandler) RegisterRoutes(r chi.Router) {
	// Urutan pendaftaran tidak menentukan: chi memilih rute statis
	// (/collateral/weights) lebih dulu daripada pola ber-{categoryCode}, sudah diuji.
	r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
		Get("/collateral/weights", h.List)
	r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
		Get("/collateral/weights/{categoryCode}", h.Assess)
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Post("/collateral/weights/{categoryCode}/activate", h.Activate)
}

// collateralWeightCategoryResponse adalah satu kategori pada daftar: kode dan label
// dibaca dari basis data, bukan dari daftar di kode klien.
type collateralWeightCategoryResponse struct {
	CategoryCode       string          `json:"category_code"`
	Label              string          `json:"label"`
	LampiranIIItem     int             `json:"lampiran_ii_item"`
	OfficialWeightFrac decimal.Decimal `json:"official_weight_frac"`
	AppliedWeightFrac  decimal.Decimal `json:"applied_weight_frac"`
	Enabled            bool            `json:"enabled"`
}

// List mengirim daftar kategori bobot agunan (GET /api/v1/collateral/weights).
// Izinnya sama dengan penilaian: cukup baca konfigurasi sistem.
func (h *CollateralWeightHandler) List(w http.ResponseWriter, r *http.Request) {
	categories, err := h.svc.ListCategories(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	out := make([]collateralWeightCategoryResponse, 0, len(categories))
	for _, c := range categories {
		out = append(out, collateralWeightCategoryResponse{
			CategoryCode:       c.CategoryCode,
			Label:              c.Label,
			LampiranIIItem:     c.LampiranIIItem,
			OfficialWeightFrac: c.OfficialWeightFrac,
			AppliedWeightFrac:  c.AppliedWeightFrac,
			Enabled:            c.Enabled,
		})
	}
	Success(w, http.StatusOK, i18n.MsgCollateralWeightCategories, out)
}

// collateralWeightAssessmentResponse adalah bentuk baca-saja hasil gerbang: syarat yang
// gagal, cakupan nilai agunan, dan umur mode bayangan.
type collateralWeightAssessmentResponse struct {
	CategoryCode         string          `json:"category_code"`
	Enabled              bool            `json:"enabled"`
	Allowed              bool            `json:"allowed"`
	Failures             []string        `json:"failures"`
	CoverageFrac         decimal.Decimal `json:"coverage_frac"`
	CoverageMinFrac      decimal.Decimal `json:"coverage_min_frac"`
	EligibleValue        decimal.Decimal `json:"eligible_value"`
	TotalValue           decimal.Decimal `json:"total_value"`
	ShadowStartedAt      *time.Time      `json:"shadow_started_at,omitempty"`
	ShadowMonthsElapsed  int             `json:"shadow_months_elapsed"`
	ShadowMonthsRequired int             `json:"shadow_months_required"`
}

func collateralWeightAssessmentDTO(res domain.CollateralWeightActivationResult) collateralWeightAssessmentResponse {
	failures := res.Failures
	if failures == nil {
		failures = []string{}
	}
	return collateralWeightAssessmentResponse{
		CategoryCode:         res.CategoryCode,
		Enabled:              res.CategoryEnabled,
		Allowed:              res.Allowed,
		Failures:             failures,
		CoverageFrac:         res.CoverageFrac,
		CoverageMinFrac:      res.CoverageMinFrac,
		EligibleValue:        res.EligibleValue,
		TotalValue:           res.TotalValue,
		ShadowStartedAt:      res.ShadowStartedAt,
		ShadowMonthsElapsed:  res.ShadowMonthsElapsed,
		ShadowMonthsRequired: res.ShadowMonthsRequired,
	}
}

// collateralWeightActivationRequest adalah isi permintaan PENGAJUAN aktivasi. Identitas
// TIDAK diterima dari body: pengaju (maker) diambil dari token pemanggil, dan penyetuju
// (checker) nanti dari token pemeriksa. Field apa pun bernama maker/checker diabaikan.
type collateralWeightActivationRequest struct {
	DireksiPolicyNumber string           `json:"direksi_policy_number"`
	DireksiPolicyDate   string           `json:"direksi_policy_date"`
	AppliedWeightFrac   *decimal.Decimal `json:"applied_weight_frac,omitempty"`
}

// collateralWeightActivationPendingResponse adalah balasan pengajuan: kategori belum
// dinyalakan; aktivasi menunggu persetujuan pemeriksa lain.
type collateralWeightActivationPendingResponse struct {
	RequestID    uuid.UUID                 `json:"request_id"`
	Status       domain.MakerCheckerStatus `json:"status"`
	CategoryCode string                    `json:"category_code"`
}

// Assess handles GET /api/v1/collateral/weights/{categoryCode}
// Menilai kesembilan syarat (C1-C9) beserta gerbang tambahan tanpa mengubah apa pun.
// Syarat yang gagal disebutkan pada `failures`; cakupan dan umur mode bayangan ikut
// dilaporkan. Parameter kueri opsional (direksi_policy_number, direksi_policy_date,
// applied_weight_frac) hanya untuk pratinjau, bukan penyimpanan; identitas maker/checker
// dibaca dari jejak aktivasi tersimpan, tidak pernah dari kueri.
func (h *CollateralWeightHandler) Assess(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	categoryCode := strings.TrimSpace(chi.URLParam(r, "categoryCode"))

	approval, err := collateralWeightApprovalFromQuery(r)
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgDateFormatInvalid)
		return
	}
	// Izin system:config menentukan C9; endpoint baca-saja tidak memberikannya.
	approval.HasConfigPermission = claims.HasPermission(domain.PermSystemConfig)

	res, err := h.svc.Assess(r.Context(), categoryCode, approval,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeCollateralWeightError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCollateralWeightAssessment, collateralWeightAssessmentDTO(res))
}

// Activate handles POST /api/v1/collateral/weights/{categoryCode}/activate
// Mengajukan aktivasi ke antrean maker-checker. Pengaju (maker) adalah aktor
// terautentikasi; identitas dari body/kueri DIABAIKAN (bukan ditolak) supaya klien
// lama tidak patah dan tidak ada jalur yang tampak menerima identitas. Kategori baru
// dinyalakan setelah pemeriksa LAIN menyetujui pengajuan lewat antrean maker-checker.
func (h *CollateralWeightHandler) Activate(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	categoryCode := strings.TrimSpace(chi.URLParam(r, "categoryCode"))
	if categoryCode == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidRequestID)
		return
	}

	// Body hanya membawa data kebijakan Direksi dan kandidat bobot. Field bernama
	// maker/checker sengaja tidak ada di struct ini, sehingga nilai apa pun di body
	// tidak pernah terbaca.
	var body collateralWeightActivationRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	policyDate, err := parseOptionalDate(body.DireksiPolicyDate, "tanggal kebijakan Direksi")
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgDateFormatInvalid)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	payload := map[string]any{
		"category_code":         categoryCode,
		"direksi_policy_number": strings.TrimSpace(body.DireksiPolicyNumber),
		// Izin system:config sudah ditegakkan rute ini; pengaju adalah pemegangnya.
		"has_config_permission": true,
	}
	if policyDate != nil {
		payload["direksi_policy_date"] = policyDate.Format("2006-01-02")
	}
	if body.AppliedWeightFrac != nil {
		payload["applied_weight_frac"] = body.AppliedWeightFrac.String()
	}

	req, err := h.mc.CreateRequest(r.Context(), domain.CreateMakerCheckerInput{
		ActionType: domain.CollateralWeightActivateAction,
		Payload:    payload,
	}, actor)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusAccepted, i18n.MsgPendingMakerChecker, collateralWeightActivationPendingResponse{
		RequestID:    req.ID,
		Status:       req.Status,
		CategoryCode: categoryCode,
	})
}

// collateralWeightApprovalFromQuery membaca parameter pratinjau opsional. Kosong
// berarti gerbang memakai keadaan yang tersimpan (lihat gateInputs storedFallback).
// Identitas maker/checker TIDAK dibaca dari kueri; pratinjau memakai jejak tersimpan.
func collateralWeightApprovalFromQuery(r *http.Request) (domain.CollateralWeightActivationApproval, error) {
	q := r.URL.Query()
	approval := domain.CollateralWeightActivationApproval{
		DireksiPolicyNumber: strings.TrimSpace(q.Get("direksi_policy_number")),
	}
	if raw := strings.TrimSpace(q.Get("direksi_policy_date")); raw != "" {
		d, err := parseOptionalDate(raw, "tanggal kebijakan Direksi")
		if err != nil {
			return approval, err
		}
		approval.DireksiPolicyDate = d
	}
	if raw := strings.TrimSpace(q.Get("applied_weight_frac")); raw != "" {
		d, err := decimal.NewFromString(raw)
		if err != nil {
			return approval, fmt.Errorf("applied_weight_frac harus desimal 0..1")
		}
		approval.AppliedWeightFrac = &d
	}
	return approval, nil
}

// writeCollateralWeightError memetakan galat gerbang. Penolakan gerbang adalah aturan
// bisnis: pesannya (menyebut syarat yang gagal) harus terlihat pengguna.
func writeCollateralWeightError(w http.ResponseWriter, r *http.Request, err error) {
	Fail(w, r, http.StatusUnprocessableEntity, err)
}
