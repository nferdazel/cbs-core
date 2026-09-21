package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"cbs-core/apps/core-api/internal/ojkreport"
	"github.com/go-chi/chi/v5"
)

// OJKReportHandler melayani fondasi ekspor laporan OJK (APOLO). Angka diambil
// dari laporan journal-based yang sudah ada; handler hanya menyajikan.
type OJKReportHandler struct {
	builder *ojkreport.Builder
}

func NewOJKReportHandler(source ojkreport.Source) *OJKReportHandler {
	return &OJKReportHandler{builder: ojkreport.NewBuilder(source)}
}

// RegisterRoutes memasang rute di bawah /reports/ojk. Izin reports:export dipakai
// karena seluruh endpoint menghasilkan data laporan untuk diekspor.
func (h *OJKReportHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ojk", func(r chi.Router) {
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/definitions", h.Definitions)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/monthly", h.ExportMonthly)
	})
}

// Definitions mengembalikan definisi laporan, periodisitas/tenggat, daftar form
// bulanan, dan status pemetaan. Laporan yang belum dapat dibangun ikut didaftarkan.
func (h *OJKReportHandler) Definitions(w http.ResponseWriter, r *http.Request) {
	Success(w, http.StatusOK, "definisi laporan OJK", map[string]any{
		"reports":        ojkreport.OJKReportDefinitions,
		"monthly_forms":  ojkreport.OJKBulananForms,
		"mapping_status": ojkreport.MappingStatus,
	})
}

// ExportMonthly menulis berkas teks Laporan Bulanan BPR: form 01.00, 02.00, 00.08,
// dan — bila sumbernya tersedia — form daftar 00.00, 05.00, dan 06.00 untuk periode
// YYYY-MM. Query book opsional (mis. CONVENTIONAL/SYARIAH).
func (h *OJKReportHandler) ExportMonthly(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	// Laporan OJK bersifat bank-wide: membatasinya pada satu cabang menghasilkan
	// laporan yang salah, sementara memberikannya ke aktor yang hanya berwenang
	// atas cabangnya membuka data lintas cabang. Karena itu hanya aktor lintas
	// cabang (pengawas) yang boleh mengekspor.
	if !actor.IsCrossBranch() {
		Fail(w, r, http.StatusForbidden, fmt.Errorf(
			"%w: laporan OJK bersifat bank-wide dan hanya dapat diekspor peran lintas cabang",
			domain.ErrCrossBranchAccess))
		return
	}

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}

	bundle, err := h.builder.GenerateMonthlyForActor(r.Context(), period, r.URL.Query().Get("book"), actor)
	if err != nil {
		if errors.Is(err, ojkreport.ErrOJKBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, ojkreport.ErrIncompleteMapping) ||
			errors.Is(err, ojkreport.ErrInvalidMapping) ||
			errors.Is(err, ojkreport.ErrUnbalancedSource) {
			Fail(w, r, http.StatusUnprocessableEntity, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	filename := "ojk-bulanan-" + period.Format("2006-01") + ".txt"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	// Header sudah terkirim; kegagalan menulis hanya dapat dicatat ke log.
	if err := ojkreport.WriteText(w, bundle); err != nil {
		observability.FromContext(r.Context()).Error("gagal menulis berkas ekspor OJK", "error", err)
	}
}

// parseOJKPeriod membaca periode YYYY-MM atau YYYY-MM-DD. Bila kosong, dipakai
// bulan sebelumnya karena laporan bulanan disampaikan pada bulan berikutnya.
func parseOJKPeriod(raw string) (time.Time, error) {
	if raw == "" {
		prev := time.Now().UTC().AddDate(0, -1, 0)
		return time.Date(prev.Year(), prev.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	for _, layout := range []string{"2006-01", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, errors.New("format period tidak valid: gunakan YYYY-MM")
}
