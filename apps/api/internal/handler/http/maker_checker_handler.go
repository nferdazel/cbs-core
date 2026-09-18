package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type makerCheckerResponse struct {
	ID           uuid.UUID                 `json:"id"`
	ActionType   string                    `json:"action_type"`
	Payload      map[string]any            `json:"payload,omitempty"`
	Status       domain.MakerCheckerStatus `json:"status"`
	MakerID      string                    `json:"maker_id"`
	CheckerID    *string                   `json:"checker_id,omitempty"`
	MakerNotes   string                    `json:"maker_notes,omitempty"`
	CheckerNotes string                    `json:"checker_notes,omitempty"`
	ReviewedAt   *time.Time                `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time                 `json:"created_at"`
}

func toMakerCheckerResponse(req *domain.MakerCheckerRequest) makerCheckerResponse {
	return makerCheckerResponse{
		ID:           req.ID,
		ActionType:   req.ActionType,
		Payload:      req.Payload,
		Status:       req.Status,
		MakerID:      req.MakerID,
		CheckerID:    req.CheckerID,
		MakerNotes:   req.MakerNotes,
		CheckerNotes: req.CheckerNotes,
		ReviewedAt:   req.ReviewedAt,
		CreatedAt:    req.CreatedAt,
	}
}

type MakerCheckerHandler struct {
	svc domain.MakerCheckerService
}

// NewMakerCheckerHandler menerima service yang sudah dirakit, bukan koneksi database.
// Pemeriksa dan penulis audit dijalankan service; handler hanya menerjemahkan HTTP.
// Eksekutor transaksi disuntikkan lewat service agar persetujuan benar-benar
// memposting jurnalnya, bukan sekadar mengubah status.
func NewMakerCheckerHandler(svc domain.MakerCheckerService) *MakerCheckerHandler {
	return &MakerCheckerHandler{svc: svc}
}

// ListPending handles GET /api/v1/maker-checker/pending
func (h *MakerCheckerHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	requests, err := h.svc.ListPending(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}

	list := make([]makerCheckerResponse, 0, len(requests))
	for i := range requests {
		list = append(list, toMakerCheckerResponse(&requests[i]))
	}
	Success(w, http.StatusOK, "pending maker-checker requests", list)
}

// Approve handles POST /api/v1/maker-checker/{id}/approve
func (h *MakerCheckerHandler) Approve(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid request id")
		return
	}

	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	if err := h.svc.Approve(r.Context(), id, actor, decodeNotes(r)); err != nil {
		writeMakerCheckerError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "request approved successfully", nil)
}

// Reject handles POST /api/v1/maker-checker/{id}/reject
func (h *MakerCheckerHandler) Reject(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid request id")
		return
	}

	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	if err := h.svc.Reject(r.Context(), id, actor, decodeNotes(r)); err != nil {
		writeMakerCheckerError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "request rejected", nil)
}

// decodeNotes membaca catatan pemeriksa; body opsional.
func decodeNotes(r *http.Request) string {
	var body struct {
		Notes string `json:"notes"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&body)
	}
	return body.Notes
}

func writeMakerCheckerError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrCannotSelfApprove):
		Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrMakerCheckerNotFound):
		Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrMakerCheckerNotPending):
		Error(w, http.StatusConflict, err.Error())
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}
