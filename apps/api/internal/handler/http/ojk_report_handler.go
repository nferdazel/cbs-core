package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"cbs-core/apps/core-api/internal/ojkreport"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// OJKCOALister membaca bagan akun untuk melengkapi peninjauan pemetaan dengan nama akun.
// Kontraknya sengaja sempit agar handler tidak bergantung pada layanan ledger penuh.
type OJKCOALister interface {
	GetCOAList(ctx context.Context) ([]domain.ChartOfAccount, error)
}

// OJKReportHandler melayani fondasi ekspor laporan OJK (APOLO). Angka diambil
// dari laporan journal-based yang sudah ada; handler hanya menyajikan.
type OJKReportHandler struct {
	builder *ojkreport.Builder
	// source disimpan untuk menghitung kode COA yang belum terpetakan pada peninjauan.
	source ojkreport.Source
	// coa dan reviews dipakai endpoint peninjauan pemetaan; boleh nil pada uji lama.
	coa     OJKCOALister
	reviews ojkreport.MappingReviewRepository
}

// NewOJKReportHandler menyusun handler ekspor sekaligus peninjauan pemetaan. coa dan
// reviews boleh nil, tetapi endpoint peninjauan membutuhkannya.
func NewOJKReportHandler(source ojkreport.Source, coa OJKCOALister, reviews ojkreport.MappingReviewRepository) *OJKReportHandler {
	return &OJKReportHandler{
		builder: ojkreport.NewBuilder(source),
		source:  source,
		coa:     coa,
		reviews: reviews,
	}
}

// RegisterRoutes memasang rute di bawah /reports/ojk. Izin reports:export dipakai
// karena seluruh endpoint menghasilkan data laporan untuk diekspor.
//
// Menandai keputusan bank bukan tindakan membaca laporan, melainkan pengaturan pedoman
// konversi bagan akun. Karena itu endpoint keputusan memakai coa:manage yang menurut
// domain.RolePermissions hanya dipegang SUPERADMIN dan ADMIN; AUDITOR tetap dapat
// membaca pemetaan untuk memeriksanya tanpa dapat mengubah keputusan bank.
func (h *OJKReportHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ojk", func(r chi.Router) {
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/definitions", h.Definitions)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/monthly", h.ExportMonthly)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/mapping", h.Mapping)
		r.With(middleware.RequirePermission(domain.PermCOAManage)).
			Post("/mapping/decision", h.DecideMapping)
	})
}

// Definitions mengembalikan definisi laporan, periodisitas/tenggat, daftar form
// bulanan, dan status pemetaan. Laporan yang belum dapat dibangun ikut didaftarkan.
func (h *OJKReportHandler) Definitions(w http.ResponseWriter, r *http.Request) {
	Success(w, http.StatusOK, i18n.MsgOJKReportDefinitions, map[string]any{
		"reports":        ojkreport.OJKReportDefinitions,
		"monthly_forms":  ojkreport.OJKBulananForms,
		"mapping_status": ojkreport.MappingStatus,
	})
}

// Mapping menyajikan pemetaan COA DRAF sebagai data untuk ditinjau bank: setiap baris
// memuat kode COA, nama akun, sandi dan nama pos OJK, status verifikasi draf, serta
// keputusan bank bila sudah ada. Disertakan pula kode COA pada laporan sumber yang belum
// terpetakan (penyebab ekspor ditolak) dan pos OJK yang belum punya sumber COA.
//
// Query period (YYYY-MM, default bulan lalu) dan book menentukan laporan sumber yang
// dipakai mendeteksi celah pemetaan; tanpa angka jurnal, celah tidak dapat dikarang.
func (h *OJKReportHandler) Mapping(w http.ResponseWriter, r *http.Request) {
	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	book := r.URL.Query().Get("book")

	entries := ojkreport.COAMappingDraft

	coaNames := make(map[string]string)
	if h.coa != nil {
		list, err := h.coa.GetCOAList(r.Context())
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca bagan akun: %w", err))
			return
		}
		for _, c := range list {
			coaNames[c.Code] = c.Name
		}
	}

	var reviews []ojkreport.MappingReview
	if h.reviews != nil {
		reviews, err = h.reviews.ListReviews(r.Context())
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca keputusan pemetaan: %w", err))
			return
		}
	}

	// Celah dihitung dari laporan sumber periode ini, sama seperti ekspor. Bila sumber
	// tidak tersedia, celah dibiarkan kosong alih-alih menuduh pemetaan bolong.
	var unmappedCOA []ojkreport.UnmappedCOA
	unbalanced := false
	if h.source != nil {
		periodEnd := ojkreport.MonthEnd(period)
		yearStart := time.Date(period.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		bs, err := h.source.GetBalanceSheet(r.Context(), periodEnd, book)
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca posisi keuangan: %w", err))
			return
		}
		is, err := h.source.GetIncomeStatement(r.Context(), yearStart, periodEnd, book)
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca laba rugi: %w", err))
			return
		}
		unmappedCOA = ojkreport.UnmappedCOACodes(bs, is, entries)
		unbalanced = ojkreport.HasImbalance(bs, is)
	}
	if unmappedCOA == nil {
		unmappedCOA = []ojkreport.UnmappedCOA{}
	}

	Success(w, http.StatusOK, i18n.MsgOJKCOAMapping, map[string]any{
		"mapping_status":     ojkreport.MappingStatus,
		"period":             period,
		"book":               book,
		"rows":               ojkreport.BuildMappingReviewRows(entries, coaNames, reviews),
		"unmapped_coa":       unmappedCOA,
		"unmapped_positions": ojkreport.UnmappedPositions(entries),
		"duplicate_coa":      ojkreport.DuplicateCOACodes(entries),
		"source_unbalanced":  unbalanced,
	})
}

// mappingDecisionRequest adalah keputusan bank atas satu baris pemetaan.
type mappingDecisionRequest struct {
	Form     string `json:"form"`
	COACode  string `json:"coa_code"`
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// DecideMapping menyimpan persetujuan bank atau catatan keberatannya atas satu baris
// pemetaan. Baris diidentifikasi (form, coa_code) dan hanya baris yang benar-benar ada
// pada pemetaan draf yang diterima. Keputusan disimpan idempoten di basis data beserta
// pelaku dan waktunya; status pemetaan global tidak berubah.
func (h *OJKReportHandler) DecideMapping(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	if h.reviews == nil {
		InternalError(w, r, errors.New("penyimpan keputusan pemetaan belum dikonfigurasi"))
		return
	}

	var req mappingDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, r, http.StatusBadRequest, errors.New("body JSON tidak valid"))
		return
	}
	req.Form = strings.TrimSpace(req.Form)
	req.COACode = strings.TrimSpace(req.COACode)
	decision := ojkreport.ReviewDecision(strings.TrimSpace(req.Decision))

	if !decision.Valid() {
		Fail(w, r, http.StatusBadRequest, errors.New("decision harus DISETUJUI atau DICATAT"))
		return
	}
	if !ojkreport.KnownMappingLine(req.Form, req.COACode) {
		Fail(w, r, http.StatusBadRequest, errors.New("baris pemetaan (form, coa_code) tidak dikenal"))
		return
	}
	note := strings.TrimSpace(req.Note)
	if decision == ojkreport.ReviewNoted && note == "" {
		Fail(w, r, http.StatusBadRequest, errors.New("catatan wajib diisi bila mencatat keberatan"))
		return
	}

	review := ojkreport.MappingReview{
		Form:      req.Form,
		COACode:   req.COACode,
		Decision:  decision,
		Note:      note,
		DecidedBy: claims.Username,
		DecidedAt: time.Now().UTC(),
	}
	if claims.UserID != uuid.Nil {
		review.DecidedByID = claims.UserID.String()
	}
	if err := h.reviews.UpsertReview(r.Context(), review); err != nil {
		InternalError(w, r, fmt.Errorf("menyimpan keputusan pemetaan: %w", err))
		return
	}

	Success(w, http.StatusOK, i18n.MsgOJKMappingDecisionSaved, review)
}

// ExportMonthly menulis berkas teks Laporan Bulanan BPR: form 01.00, 02.00, 00.08,
// dan — bila sumbernya tersedia — form daftar 00.00, 05.00, dan 06.00 untuk periode
// YYYY-MM. Query book opsional (mis. CONVENTIONAL/SYARIAH).
func (h *OJKReportHandler) ExportMonthly(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
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
