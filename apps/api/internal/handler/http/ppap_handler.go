package http

import (
	"errors"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
)

type PPAPHandler struct {
	ppapSvc domain.PPAPService
	// ppkaUmum menghitung PPKA umum (0,5% aset produktif lancar) dari kredit dan
	// penempatan. Opsional: bila nil, rutenya tidak dipasang (perilaku lama).
	ppkaUmum domain.PPKAUmumService
}

// NewPPAPHandler menyusun handler PPAP. ppkaUmumOpsional boleh kosong; tanpa itu
// hanya rute run-daily/preview yang dipasang, tanpa mengubah perilaku lama.
func NewPPAPHandler(ppapSvc domain.PPAPService, ppkaUmumOpsional ...domain.PPKAUmumService) *PPAPHandler {
	var ppka domain.PPKAUmumService
	if len(ppkaUmumOpsional) > 0 {
		ppka = ppkaUmumOpsional[0]
	}
	return &PPAPHandler{ppapSvc: ppapSvc, ppkaUmum: ppka}
}

// RunDaily handles POST /api/v1/ppap/run-daily.
// Menjalankan perhitungan kolektibilitas dan penyisihan PPAP untuk seluruh kredit
// aktif. Tanggal proses opsional lewat query `as_of` (YYYY-MM-DD atau RFC3339).
func (h *PPAPHandler) RunDaily(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	asOf := parseAsOf(r)
	summary, err := h.ppapSvc.RunDaily(r.Context(), asOf, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPPAPCalculated, summary)
}

// Preview handles GET /api/v1/ppap/preview.
// Menghitung target dan selisih PPAP tanpa memposting jurnal.
func (h *PPAPHandler) Preview(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	summary, err := h.ppapSvc.Preview(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPPAPPreview, summary)
}

// PPKAUmum handles GET /api/v1/ppap/ppka-umum.
// Menghitung PPKA umum minimum 0,5% aset produktif lancar (POJK 1/2024 Pasal 19
// ayat (2)) dari kredit lancar dan penempatan pada bank lain, tanpa memposting apa pun.
func (h *PPAPHandler) PPKAUmum(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	if h.ppkaUmum == nil {
		InternalError(w, r, errors.New("modul PPKA umum belum tersambung"))
		return
	}

	summary, err := h.ppkaUmum.Hitung(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPPKAUmumCalculated, summary)
}

// RegisterRoutes memasang rute PPAP pada router yang sudah berada di dalam grup
// terautentikasi (AuthMiddleware). Jalankan perhitungan butuh wewenang approve kredit;
// pratinjau dan PPKA umum cukup wewenang baca.
func (h *PPAPHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ppap", func(r chi.Router) {
		r.With(middleware.RequirePermission(domain.PermLoansApprove)).
			Post("/run-daily", h.RunDaily)
		r.With(middleware.RequirePermission(domain.PermLoansRead)).
			Get("/preview", h.Preview)
		if h.ppkaUmum != nil {
			r.With(middleware.RequirePermission(domain.PermLoansRead)).
				Get("/ppka-umum", h.PPKAUmum)
		}
	})
}

// parseAsOf membaca tanggal proses; default waktu sekarang (UTC).
func parseAsOf(r *http.Request) time.Time {
	raw := r.URL.Query().Get("as_of")
	if raw == "" {
		return time.Now().UTC()
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC()
	}
	return time.Now().UTC()
}
