package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// LPSPlacementHandler menyajikan perhitungan PPKA penempatan pada bank lain yang dijamin
// LPS menurut Pasal 23 POJK No. 1 Tahun 2024 dan asesmen CKPN per penempatan (kolom
// XII/XXI Form 05.00).
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
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	summary, err := h.lpsSvc.Calculate(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPPKAPlacementCalculated, summary)
}

// AssessCKPN handles POST /api/v1/lps-placements/{id}/ckpn.
// Menyimpan asesmen CKPN satu penempatan (target, metode, penanda, tanggal). Selama
// saklar ckpn.pabl.enabled mati, permintaan ditolak 409; asesmen tidak memposting jurnal.
func (h *LPSPlacementHandler) AssessCKPN(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	placementID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	var input domain.PABLCKPNInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	placement, err := h.lpsSvc.AssessCKPN(r.Context(), placementID, input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		h.failAssessCKPN(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNPABLAssessed, placement)
}

// failAssessCKPN memetakan galat asesmen CKPN ke status yang dapat ditindaklanjuti:
// penempatan tidak ditemukan 404, penolakan keamanan 403 (Fail), saklar mati 409, dan
// masukan tidak sah 422; sisanya kegagalan server.
func (h *LPSPlacementHandler) failAssessCKPN(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrLPSPlacementNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrCrossBranchAccess), errors.Is(err, domain.ErrCrossBookAccess):
		Fail(w, r, http.StatusForbidden, err)
	case errors.Is(err, domain.ErrCKPNPABLDisabled):
		Fail(w, r, http.StatusConflict, err)
	case errors.Is(err, domain.ErrCKPNPABLInputInvalid):
		Fail(w, r, http.StatusUnprocessableEntity, err)
	default:
		InternalError(w, r, err)
	}
}

// RegisterRoutes memasang rute pada router yang sudah berada di dalam grup
// terautentikasi (AuthMiddleware). Baca cukup wewenang loans:read; asesmen CKPN menulis
// state keuangan, jadi memakai loans:approve (bukan loans:read) mengikuti preseden
// endpoint tulis CKPN individual lain di modul ini.
func (h *LPSPlacementHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermLoansRead)).
		Get("/lps-placements", h.Calculate)
	r.With(middleware.RequirePermission(domain.PermLoansApprove)).
		Post("/lps-placements/{id}/ckpn", h.AssessCKPN)
}
