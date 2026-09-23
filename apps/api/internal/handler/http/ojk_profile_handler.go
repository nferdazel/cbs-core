package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
)

// OJKProfileHandler melayani identitas Form 00.00 yang disimpan sebagai kunci
// system_config ojk.* (mis. surel, situs web, sandi kota/wilayah OJK, penanggung
// jawab laporan). Otorisasi ada di rute (system:config:read untuk baca,
// system:config untuk ubah); handler hanya merapikan masukan dan memetakan error
// bisnis ke status HTTP.
type OJKProfileHandler struct {
	svc domain.OJKProfileService
}

func NewOJKProfileHandler(svc domain.OJKProfileService) *OJKProfileHandler {
	return &OJKProfileHandler{svc: svc}
}

// Get mengirim profil OJK (GET /api/v1/system/ojk-profile).
func (h *OJKProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	profile, err := h.svc.Get(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOJKProfile, profile)
}

// Update mengubah profil OJK (PUT /api/v1/system/ojk-profile). Decoder menolak
// bidang tak dikenal agar payload di luar kontrak ditolak 422, bukan diam-diam
// diabaikan. Bidang yang dikirim kosong SAH: kosong berarti belum tersedia.
func (h *OJKProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.UpdateOJKProfileInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgOJKProfilePayloadInvalid, err.Error())
		return
	}

	profile, err := h.svc.Update(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOJKProfileUpdated, profile)
}
