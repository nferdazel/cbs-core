package http

import (
	"fmt"
	"net/http"
	"strings"
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
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "Trial Balance report generated", report)
}

// GetBalanceSheet handles GET /api/v1/reports/balance-sheet
func (h *ReportHandler) GetBalanceSheet(w http.ResponseWriter, r *http.Request) {
	asOf := time.Now().UTC()
	report, err := h.reportSvc.GenerateBalanceSheet(r.Context(), asOf)
	if err != nil {
		InternalError(w, r, err)
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
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "Income Statement report generated", report)
}

// reportToday mengembalikan awal hari UTC dan awal bulan berjalan UTC.
func reportToday() (today, firstOfMonth time.Time) {
	now := time.Now().UTC()
	today = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	firstOfMonth = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return today, firstOfMonth
}

// parseReportDate membaca query param tanggal format YYYY-MM-DD, atau default bila kosong.
func parseReportDate(r *http.Request, key string, def time.Time) (time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return def, nil
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("format %s tidak valid: gunakan YYYY-MM-DD", key)
	}
	return parsed, nil
}

// reportRange membaca rentang from/to dengan default awal bulan berjalan s/d hari ini.
func reportRange(r *http.Request) (from, to time.Time, err error) {
	today, firstOfMonth := reportToday()
	if from, err = parseReportDate(r, "from", firstOfMonth); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if to, err = parseReportDate(r, "to", today); err != nil {
		return time.Time{}, time.Time{}, err
	}
	return from, to, nil
}

// TrialBalance handles GET /api/v1/reports/trial-balance
func (h *ReportHandler) TrialBalance(w http.ResponseWriter, r *http.Request) {
	from, to, err := reportRange(r)
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	report, err := h.reportSvc.GetTrialBalance(r.Context(), from, to, r.URL.Query().Get("book"))
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "Trial Balance report generated", report)
}

// IncomeStatement handles GET /api/v1/reports/income-statement
func (h *ReportHandler) IncomeStatement(w http.ResponseWriter, r *http.Request) {
	from, to, err := reportRange(r)
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	report, err := h.reportSvc.GetIncomeStatement(r.Context(), from, to, r.URL.Query().Get("book"))
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "Income Statement report generated", report)
}

// BalanceSheet handles GET /api/v1/reports/balance-sheet
func (h *ReportHandler) BalanceSheet(w http.ResponseWriter, r *http.Request) {
	today, _ := reportToday()
	asOf, err := parseReportDate(r, "as_of", today)
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	report, err := h.reportSvc.GetBalanceSheet(r.Context(), asOf, r.URL.Query().Get("book"))
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "Balance Sheet report generated", report)
}

// CashFlow handles GET /api/v1/reports/cash-flow
func (h *ReportHandler) CashFlow(w http.ResponseWriter, r *http.Request) {
	from, to, err := reportRange(r)
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	report, err := h.reportSvc.GetCashFlow(r.Context(), from, to, r.URL.Query().Get("book"))
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "Cash Flow report generated", report)
}
