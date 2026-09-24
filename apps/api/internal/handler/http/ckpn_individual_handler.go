package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
)

// CKPN individual TAHAP T1 (keputusan panel butir 4). Endpoint ini BACA-SAJA untuk
// penilaian dan hanya menulis proyeksi arus kas (data operasional bank); required_ckpn
// TIDAK pernah disentuh di sini.

// IndividualAssessment handles GET /api/v1/ckpn/individual/{loanNumber}/assessment.
// Menghitung DCF satu kredit memakai EIR orisinal dan proyeksi arus kas manual.
func (h *CKPNHandler) IndividualAssessment(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	loanNumber := chi.URLParam(r, "loanNumber")
	assessment, err := h.Individual.Evaluate(r.Context(), loanNumber, parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		h.failIndividual(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNIndividualAssessment, assessment)
}

// ReplaceIndividualProjections handles
// PUT /api/v1/ckpn/individual/{loanNumber}/projections.
// Payload: {"projections":[{"period":1,"amount":"1000000"}]}. Proyeksi divalidasi
// sebelum disimpan; proyeksi salah ditolak, bukan disimpan diam-diam.
func (h *CKPNHandler) ReplaceIndividualProjections(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input struct {
		Projections []domain.CKPNCashflowProjection `json:"projections"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	loanNumber := chi.URLParam(r, "loanNumber")
	if err := h.Individual.ReplaceProjections(r.Context(), loanNumber, parseAsOf(r), input.Projections,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		h.failIndividual(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNIndividualProjectionsSaved, map[string]any{
		"loan_number": loanNumber,
		"as_of":       parseAsOf(r).Format("2006-01-02"),
		"count":       len(input.Projections),
	})
}

// failIndividual memetakan galat T1 ke status yang dapat ditindaklanjuti: kredit tidak
// ditemukan 404, penolakan keamanan ditangani Fail (403), dan masukan/parameter tidak
// sah 422; sisanya kegagalan server.
func (h *CKPNHandler) failIndividual(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrLoanNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrCrossBranchAccess), errors.Is(err, domain.ErrCrossBookAccess):
		Fail(w, r, http.StatusForbidden, err)
	case errors.Is(err, domain.ErrCKPNProjectionInvalid),
		errors.Is(err, domain.ErrCKPNParameterInvalid),
		errors.Is(err, domain.ErrEIRMissing):
		Fail(w, r, http.StatusUnprocessableEntity, err)
	default:
		InternalError(w, r, err)
	}
}

// SetIndividualDisposalCost handles
// PUT /api/v1/ckpn/individual/{loanNumber}/collaterals/{collateralID}/disposal-cost.
// T2: menyimpan estimasi biaya pelepasan satu agunan. Payload: {"disposal_cost":"150000"}
// atau {"disposal_cost":null} untuk mengosongkan (NRV memakai bound_amount penuh).
func (h *CKPNHandler) SetIndividualDisposalCost(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input struct {
		// pointer agar "null" dan "hilang" dapat dibedakan: null = kosongkan.
		DisposalCost *decimal.Decimal `json:"disposal_cost"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	collateralID, err := uuid.Parse(chi.URLParam(r, "collateralID"))
	if err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr,
			"collateralID bukan UUID yang sah")
		return
	}
	loanNumber := chi.URLParam(r, "loanNumber")
	if err := h.Individual.SetDisposalCost(r.Context(), loanNumber, collateralID, input.DisposalCost,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		h.failIndividual(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNIndividualDisposalCostSaved, map[string]any{
		"loan_number":   loanNumber,
		"collateral_id": collateralID,
	})
}
