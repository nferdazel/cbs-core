package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
)

type CKPNHandler struct {
	ckpnSvc domain.CKPNService
}

func NewCKPNHandler(ckpnSvc domain.CKPNService) *CKPNHandler {
	return &CKPNHandler{ckpnSvc: ckpnSvc}
}

// Compare handles GET /api/v1/ckpn/comparison.
// Menyajikan perbandingan CKPN vs PPKA per kredit dan total tanpa menulis apa pun.
// Tanggal proses opsional lewat query `as_of` (lihat parseAsOf).
func (h *CKPNHandler) Compare(w http.ResponseWriter, r *http.Request) {
	summary, err := h.ckpnSvc.Compare(r.Context(), parseAsOf(r))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "perbandingan CKPN dan PPKA", summary)
}

// Run handles POST /api/v1/ckpn/run.
// Menghitung CKPN, memposting selisihnya, dan menyimpan target per kredit. Tanggal
// proses opsional lewat query `as_of`.
func (h *CKPNHandler) Run(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	summary, err := h.ckpnSvc.Run(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "perhitungan CKPN selesai", summary)
}

// RegisterRoutes memasang rute CKPN pada router yang sudah berada di dalam grup
// terautentikasi (AuthMiddleware). Menjalankan perhitungan (yang memposting jurnal)
// butuh wewenang approve kredit; perbandingan cukup wewenang baca.
func (h *CKPNHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ckpn", func(r chi.Router) {
		r.With(middleware.RequirePermission(domain.PermLoansApprove)).
			Post("/run", h.Run)
		r.With(middleware.RequirePermission(domain.PermLoansRead)).
			Get("/comparison", h.Compare)
	})
}
