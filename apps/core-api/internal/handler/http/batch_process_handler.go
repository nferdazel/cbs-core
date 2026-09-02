package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

type BatchProcessHandler struct {
	batchSvc domain.BatchProcessService
}

func NewBatchProcessHandler(batchSvc domain.BatchProcessService) *BatchProcessHandler {
	return &BatchProcessHandler{batchSvc: batchSvc}
}

// GetBusinessDate handles GET /api/v1/system/business-date
func (h *BatchProcessHandler) GetBusinessDate(w http.ResponseWriter, r *http.Request) {
	dateInfo, err := h.batchSvc.GetCurrentBusinessDate(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Success(w, http.StatusOK, "current system business date retrieved", dateInfo)
}

// RunEOD handles POST /api/v1/batch/eod (Supervisor / Admin)
func (h *BatchProcessHandler) RunEOD(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	result, err := h.batchSvc.RunEOD(r.Context(), claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "End of Day (EOD) executed successfully. System date advanced.", result)
}

// RunEOM handles POST /api/v1/batch/eom (Admin)
func (h *BatchProcessHandler) RunEOM(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		AdminFeeMonthly   decimal.Decimal `json:"admin_fee_monthly"`
		InterestRateMonth decimal.Decimal `json:"interest_rate_month"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	result, err := h.batchSvc.RunEOM(r.Context(), body.AdminFeeMonthly, body.InterestRateMonth, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "End of Month (EOM) batch process executed successfully.", result)
}

// RunEOY handles POST /api/v1/batch/eoy (Superadmin / Admin)
func (h *BatchProcessHandler) RunEOY(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		RetainedEarningsCOACode string `json:"retained_earnings_coa_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RetainedEarningsCOACode == "" {
		body.RetainedEarningsCOACode = "30201" // Default Laba Ditahan COA
	}

	result, err := h.batchSvc.RunEOY(r.Context(), body.RetainedEarningsCOACode, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "End of Year (EOY) Tutup Buku Akhir Tahun completed successfully.", result)
}
