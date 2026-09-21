package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
)

// LPSPlacementHandler menyajikan perhitungan PPKA penempatan pada bank lain yang dijamin
// LPS menurut Pasal 23 POJK No. 1 Tahun 2024. Operasi baca-saja.
type LPSPlacementHandler struct {
	lpsSvc domain.LPSPlacementService
}

func NewLPSPlacementHandler(lpsSvc domain.LPSPlacementService) *LPSPlacementHandler {
	return &LPSPlacementHandler{lpsSvc: lpsSvc}
}

// Calculate handles GET /api/v1/lps-placements.
// Menghitung PPKA umum dan khusus atas penempatan setelah dikurangi bagian yang dijamin
// LPS. Tanggal proses opsional lewat query `as_of` (lihat parseAsOf). Aktor diambil dari
// claims agar aktor cabang hanya membaca penempatan cabangnya; aktor lintas cabang
// (SUPERADMIN/AUDITOR) membaca seluruh bank.
func (h *LPSPlacementHandler) Calculate(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	summary, err := h.lpsSvc.Calculate(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "perhitungan PPKA penempatan pada bank lain (Pasal 23 POJK 1/2024)", summary)
}

// RegisterRoutes memasang rute pada router yang sudah berada di dalam grup
// terautentikasi (AuthMiddleware). Cukup wewenang baca karena tidak menulis apa pun.
func (h *LPSPlacementHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermLoansRead)).
		Get("/lps-placements", h.Calculate)
}
