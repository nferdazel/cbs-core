package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
)

// CKPNActivationHandler menyajikan pintu pengaturan aktivasi CKPN: parameter PD/LGD,
// akun syariah, status SEMENTARA/FINAL, bukti ratifikasi, mode bayangan, dan saklar
// ckpn.enabled. Menggantikan pengisian lewat SQL menjadi perubahan berizin + teraudit.
// Menyalakan CKPN tetap keputusan manusia: endpoint ini menolak penyalakan prematur,
// tidak pernah menyalakannya sendiri.
type CKPNActivationHandler struct {
	svc domain.CKPNActivationService
}

func NewCKPNActivationHandler(svc domain.CKPNActivationService) *CKPNActivationHandler {
	return &CKPNActivationHandler{svc: svc}
}

// RegisterRoutes memasang rute pengaturan aktivasi. Membaca cukup system:config:read
// (auditor dapat menilai tanpa wewenang menulis); mengubah WAJIB system:config penuh.
func (h *CKPNActivationHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
		Get("/system/ckpn-activation", h.Get)
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Put("/system/ckpn-activation", h.Update)
}

// Get handles GET /api/v1/system/ckpn-activation.
// Menyajikan nilai tiap kunci yang dikelola, status parameter (SEMENTARA/FINAL, batas
// ratifikasi, blokir ekspor OJK), dan sisa penahan penyalakan. Baca-saja.
func (h *CKPNActivationHandler) Get(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Get(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNActivation, res)
}

// Update handles PUT /api/v1/system/ckpn-activation.
// Hanya bidang yang dikirim yang diubah. Penolakan aturan (fraksi tak sah, FINAL tanpa
// bukti, penyalakan prematur) dibalas 422 dengan alasan yang menyebut kunci bermasalah.
func (h *CKPNActivationHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var body domain.UpdateCKPNActivationInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	res, err := h.svc.Update(r.Context(), body, actor)
	if err != nil {
		writeCKPNActivationError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNActivationUpdated, res)
}

// writeCKPNActivationError memetakan galat pengaturan aktivasi. Aturan bisnis dibalas 422
// agar pesan yang menyebut langkah perbaikan terlihat pengguna; permintaan kosong 400.
func writeCKPNActivationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrCKPNActivationEmpty):
		Fail(w, r, http.StatusBadRequest, err)
	case errors.Is(err, domain.ErrCKPNActivationFractionInvalid),
		errors.Is(err, domain.ErrCKPNActivationStatusInvalid),
		errors.Is(err, domain.ErrCKPNActivationDateInvalid),
		errors.Is(err, domain.ErrCKPNActivationRatificationIncomplete),
		errors.Is(err, domain.ErrCKPNActivationNotReady):
		Fail(w, r, http.StatusUnprocessableEntity, err)
	default:
		InternalError(w, r, err)
	}
}
