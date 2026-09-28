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

// CKPNPABLActivationHandler menyajikan pintu pengaturan CKPN per penempatan pada bank
// lain (Form 05.00 kolom XII/XXI): fraksi PD/LGD kolektif, bentuk CKPN aset baik, dan
// saklar ckpn.pabl.enabled. Menggantikan pengisian lewat SQL menjadi perubahan berizin +
// teraudit. Rutenya TERPISAH dari jalur aktivasi CKPN utama agar setelan lanjutan tidak
// tertimpa. Menyalakan PABL tetap keputusan manusia: endpoint ini menolak penyalakan
// prematur, tidak pernah menyalakannya sendiri.
type CKPNPABLActivationHandler struct {
	svc domain.CKPNPABLActivationService
}

func NewCKPNPABLActivationHandler(svc domain.CKPNPABLActivationService) *CKPNPABLActivationHandler {
	return &CKPNPABLActivationHandler{svc: svc}
}

// RegisterRoutes memasang rute pengaturan CKPN PABL. Membaca cukup system:config:read
// (auditor dapat menilai tanpa wewenang menulis); mengubah WAJIB system:config penuh.
func (h *CKPNPABLActivationHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
		Get("/system/ckpn-pabl", h.Get)
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Put("/system/ckpn-pabl", h.Update)
}

// Get handles GET /api/v1/system/ckpn-pabl.
// Menyajikan nilai tiap kunci yang dikelola dan sisa penahan penyalakan. Baca-saja.
func (h *CKPNPABLActivationHandler) Get(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.Get(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNPABLActivation, res)
}

// Update handles PUT /api/v1/system/ckpn-pabl.
// Hanya bidang yang dikirim yang diubah. Penolakan aturan (fraksi tak sah, penyalakan
// prematur) dibalas 422 dengan alasan yang menyebut kunci bermasalah.
func (h *CKPNPABLActivationHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var body domain.UpdateCKPNPABLActivationInput
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	res, err := h.svc.Update(r.Context(), body, actor)
	if err != nil {
		writeCKPNPABLActivationError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCKPNPABLActivationUpdated, res)
}

// writeCKPNPABLActivationError memetakan galat pengaturan CKPN PABL. Aturan bisnis
// dibalas 422 agar pesan yang menyebut langkah perbaikan terlihat pengguna; permintaan
// kosong 400.
func writeCKPNPABLActivationError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrCKPNPABLActivationEmpty):
		Fail(w, r, http.StatusBadRequest, err)
	case errors.Is(err, domain.ErrPABLCKPNParameterInvalid),
		errors.Is(err, domain.ErrCKPNPABLActivationNotReady):
		Fail(w, r, http.StatusUnprocessableEntity, err)
	default:
		InternalError(w, r, err)
	}
}
