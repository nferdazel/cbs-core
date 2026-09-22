package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
)

// BankProfileHandler melayani identitas bank tingkat instalasi: dibaca halaman
// pengaturan dan diubah bank tanpa SQL. Otorisasi ada di rute (system:config:read
// untuk baca, system:config untuk ubah); handler hanya merapikan masukan dan
// memetakan error bisnis ke status HTTP.
type BankProfileHandler struct {
	svc domain.BankProfileService
}

func NewBankProfileHandler(svc domain.BankProfileService) *BankProfileHandler {
	return &BankProfileHandler{svc: svc}
}

// Get mengirim profil bank (GET /api/v1/system/bank-profile).
func (h *BankProfileHandler) Get(w http.ResponseWriter, r *http.Request) {
	profile, err := h.svc.Get(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "profil bank", profile)
}

// Update mengubah profil bank (PUT /api/v1/system/bank-profile). Decoder menolak
// bidang tak dikenal agar payload yang mencoba menyentuh kolom lain (mis. id atau
// updated_at) ditolak 422, bukan diam-diam diabaikan.
func (h *BankProfileHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.UpdateBankProfileInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		Error(w, http.StatusUnprocessableEntity, "payload profil bank tidak valid: "+err.Error())
		return
	}

	profile, err := h.svc.Update(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusOK, "profil bank diperbarui", profile)
}
