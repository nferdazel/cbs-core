package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
)

// OJKPlacementCodesHandler menyajikan pintu pengisian sandi referensi/inline OJK per
// penempatan pada bank lain (Form 05.00) dari data yang sebelumnya hanya bisa diisi
// lewat SQL/seed. Perubahan diaudit satu transaksi; rute baca mengembalikan nilai
// tersimpan agar UI dapat menampilkan isian saat ini.
type OJKPlacementCodesHandler struct {
	svc domain.OJKPlacementCodesService
}

func NewOJKPlacementCodesHandler(svc domain.OJKPlacementCodesService) *OJKPlacementCodesHandler {
	return &OJKPlacementCodesHandler{svc: svc}
}

// RegisterRoutes memasang rute pengisian sandi OJK penempatan. Mengubah dan membaca
// WAJIB system:config, izin yang sama dengan setelan OJK lain; tidak ada izin baru.
func (h *OJKPlacementCodesHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Put("/ojk/placement-codes/{placementId}", h.Update)
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Get("/ojk/placement-codes/{placementId}", h.Get)
}

// Update handles PUT /api/v1/ojk/placement-codes/{placementId}. Decoder menolak bidang
// tak dikenal agar payload di luar kontrak ditolak 422, bukan diam-diam diabaikan.
func (h *OJKPlacementCodesHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	placementID, err := uuid.Parse(chi.URLParam(r, "placementId"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidPlacementID)
		return
	}

	var input domain.UpdateOJKPlacementCodesInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	placement, err := h.svc.Update(r.Context(), placementID, input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeOJKPlacementCodesError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOJKPlacementCodesUpdated, domain.NewOJKPlacementCodesView(*placement))
}

// Get handles GET /api/v1/ojk/placement-codes/{placementId}. Mengembalikan nilai
// penempatan tersimpan sebagai proyeksi baca.
func (h *OJKPlacementCodesHandler) Get(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	placementID, err := uuid.Parse(chi.URLParam(r, "placementId"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidPlacementID)
		return
	}

	view, err := h.svc.Get(r.Context(), placementID,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// Pembacaan lintas cabang disamarkan 404 agar keberadaan data tidak bocor,
		// mengikuti GET /loans/{id}; operasi tulis tetap membalas 403.
		Fail(w, r, http.StatusNotFound, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOJKPlacementCodesRead, view)
}

// writeOJKPlacementCodesError memetakan galat sandi OJK penempatan. Penempatan tak
// ditemukan adalah 404; permintaan kosong 400; aturan validasi/otorisasi 422/403 lewat
// Fail (termasuk lintas cabang yang selalu 403).
func writeOJKPlacementCodesError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrLPSPlacementNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrOJKPlacementCodesEmpty):
		ErrorCode(w, http.StatusBadRequest, i18n.MsgOJKPlacementCodesEmpty)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}
