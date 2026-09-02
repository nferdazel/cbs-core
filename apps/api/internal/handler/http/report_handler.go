package http

import (
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

type ReportHandler struct {
	reportSvc domain.ReportService
}

func NewReportHandler(reportSvc domain.ReportService) *ReportHandler {
	return &ReportHandler{reportSvc: reportSvc}
}

// GetTrialBalance handles GET /api/v1/reports/trial-balance
func (h *ReportHandler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	report, err := h.reportSvc.GenerateTrialBalance(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Success(w, http.StatusOK, "Trial Balance report generated", report)
}

// GetBalanceSheet handles GET /api/v1/reports/balance-sheet
func (h *ReportHandler) GetBalanceSheet(w http.ResponseWriter, r *http.Request) {
	asOf := time.Now().UTC()
	report, err := h.reportSvc.GenerateBalanceSheet(r.Context(), asOf)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Success(w, http.StatusOK, "Balance Sheet report generated", report)
}

// GetIncomeStatement handles GET /api/v1/reports/income-statement
func (h *ReportHandler) GetIncomeStatement(w http.ResponseWriter, r *http.Request) {
	endDate := time.Now().UTC()
	startDate := time.Date(endDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	report, err := h.reportSvc.GenerateIncomeStatement(r.Context(), startDate, endDate)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Success(w, http.StatusOK, "Income Statement report generated", report)
}
