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

// OJKLoanCodesHandler menyajikan pintu pengisian sandi referensi/inline OJK per
// kredit (Form 06.00 kolom VIII/XI/XXII) dari data yang sebelumnya hanya bisa diisi
// lewat SQL/seed. Perubahan diaudit satu transaksi; rute baca cukup GET kredit yang
// sudah menyertakan sandi ini.
type OJKLoanCodesHandler struct {
	svc domain.OJKLoanCodesService
}

func NewOJKLoanCodesHandler(svc domain.OJKLoanCodesService) *OJKLoanCodesHandler {
	return &OJKLoanCodesHandler{svc: svc}
}

// RegisterRoutes memasang rute pengisian sandi OJK kredit. Mengubah WAJIB
// system:config, izin yang sama dengan setelan OJK lain (profil Form 00.00); tidak
// ada izin baru.
func (h *OJKLoanCodesHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Put("/ojk/loan-codes/{loanId}", h.Update)
}

// Update handles PUT /api/v1/ojk/loan-codes/{loanId}. Decoder menolak bidang tak
// dikenal agar payload di luar kontrak ditolak 422, bukan diam-diam diabaikan.
func (h *OJKLoanCodesHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	loanID, err := uuid.Parse(chi.URLParam(r, "loanId"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var input domain.UpdateOJKLoanCodesInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	loan, err := h.svc.Update(r.Context(), loanID, input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeOJKLoanCodesError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOJKLoanCodesUpdated, loan)
}

// writeOJKLoanCodesError memetakan galat sandi OJK kredit. Kredit tak ditemukan
// adalah 404 (konsisten dengan endpoint kredit lain); permintaan kosong 400; aturan
// validasi/otorisasi 422/403 lewat Fail (termasuk lintas cabang yang selalu 403).
func writeOJKLoanCodesError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrLoanNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrOJKLoanCodesEmpty):
		ErrorCode(w, http.StatusBadRequest, i18n.MsgOJKLoanCodesEmpty)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}
