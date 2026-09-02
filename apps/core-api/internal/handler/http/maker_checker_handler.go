package http

import (
	"database/sql"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type MakerCheckerRequest struct {
	ID              uuid.UUID       `json:"id"`
	MakerID         uuid.UUID       `json:"maker_id"`
	CheckerID       *uuid.UUID      `json:"checker_id,omitempty"`
	TransactionType string          `json:"transaction_type"`
	Amount          decimal.Decimal `json:"amount"`
	Payload         string          `json:"payload"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	ProcessedAt     *time.Time      `json:"processed_at,omitempty"`
}

type MakerCheckerHandler struct {
	db *sql.DB
}

func NewMakerCheckerHandler(db *sql.DB) *MakerCheckerHandler {
	return &MakerCheckerHandler{db: db}
}

// ListPending handles GET /api/v1/maker-checker/pending
func (h *MakerCheckerHandler) ListPending(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, maker_id, transaction_type, amount, payload, status, created_at
		FROM maker_checker_requests WHERE status = 'PENDING' ORDER BY created_at ASC`

	rows, err := h.db.QueryContext(r.Context(), q)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	var list []MakerCheckerRequest
	for rows.Next() {
		var req MakerCheckerRequest
		if err := rows.Scan(
			&req.ID, &req.MakerID, &req.TransactionType, &req.Amount,
			&req.Payload, &req.Status, &req.CreatedAt,
		); err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		list = append(list, req)
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

	q := `UPDATE maker_checker_requests
		SET status = 'APPROVED', checker_id = $1, processed_at = NOW()
		WHERE id = $2 AND status = 'PENDING'`

	res, err := h.db.ExecContext(r.Context(), q, claims.UserID, id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		Error(w, http.StatusNotFound, "pending maker-checker request not found")
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

	q := `UPDATE maker_checker_requests
		SET status = 'REJECTED', checker_id = $1, processed_at = NOW()
		WHERE id = $2 AND status = 'PENDING'`

	res, err := h.db.ExecContext(r.Context(), q, claims.UserID, id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		Error(w, http.StatusNotFound, "pending maker-checker request not found")
		return
	}

	Success(w, http.StatusOK, "request rejected", nil)
}
