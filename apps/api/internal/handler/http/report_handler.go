package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
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
	Success(w, http.StatusOK, i18n.MsgReportTrialBalanceGenerated, report)
}

// GetBalanceSheet handles GET /api/v1/reports/balance-sheet
func (h *ReportHandler) GetBalanceSheet(w http.ResponseWriter, r *http.Request) {
	asOf, _ := reportToday()
	report, err := h.reportSvc.GenerateBalanceSheet(r.Context(), asOf)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgReportBalanceSheetGenerated, report)
}

// GetIncomeStatement handles GET /api/v1/reports/income-statement
func (h *ReportHandler) GetIncomeStatement(w http.ResponseWriter, r *http.Request) {
	endDate, _ := reportToday()
	startDate := time.Date(endDate.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	report, err := h.reportSvc.GenerateIncomeStatement(r.Context(), startDate, endDate)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgReportIncomeStatementGenerated, report)
}

// reportToday mengembalikan awal hari dan awal bulan berjalan menurut TANGGAL
// BISNIS bank (WIB), bukan tanggal UTC. Tanggal bisnis sistem memakai
// domain.BankZone (lihat batch_process_service.go dan domain.Cron), sehingga antara
// 00:00–07:00 WIB laporan tidak lagi tampak kosong karena masih menyebut hari UTC
// kemarin. Nilai yang dikembalikan bertipe UTC pada tengah malam agar bentuknya sama
// dengan hasil parse parameter tanggal eksplisit (?from/?to), sementara komponen
// tanggalnya mengikuti kalender WIB.
func reportToday() (today, firstOfMonth time.Time) {
	return businessDay(time.Now())
}

// businessDay memisahkan perhitungan tanggal dari jam sistem agar dapat diuji
// deterministik pada jendela 00:00–07:00 WIB, saat tanggal WIB berbeda dari UTC.
func businessDay(now time.Time) (today, firstOfMonth time.Time) {
	wib := now.In(domain.BankZone)
	today = time.Date(wib.Year(), wib.Month(), wib.Day(), 0, 0, 0, 0, time.UTC)
	firstOfMonth = time.Date(wib.Year(), wib.Month(), 1, 0, 0, 0, 0, time.UTC)
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
	Success(w, http.StatusOK, i18n.MsgReportTrialBalanceGenerated, report)
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
	Success(w, http.StatusOK, i18n.MsgReportIncomeStatementGenerated, report)
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
	Success(w, http.StatusOK, i18n.MsgReportBalanceSheetGenerated, report)
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
	Success(w, http.StatusOK, i18n.MsgReportCashFlowGenerated, report)
}

// DueObligations handles GET /api/v1/reports/due-obligations.
// Query opsional: days (default 30). Daftar mencakup kewajiban yang sudah lewat
// tanggal sehingga penandanya tetap terlihat walau horizons-nya pendek.
func (h *ReportHandler) DueObligations(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	withinDays := 30
	if raw := strings.TrimSpace(r.URL.Query().Get("days")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			ErrorCode(w, http.StatusBadRequest, i18n.MsgDaysInvalid)
			return
		}
		withinDays = parsed
	}

	items, err := h.reportSvc.ListDueObligations(
		r.Context(),
		time.Now().UTC(),
		withinDays,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())),
	)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgDueObligationsListed, items)
}
