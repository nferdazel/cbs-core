package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
)

type BranchHandler struct {
	service domain.BranchService
}

func NewBranchHandler(service domain.BranchService) *BranchHandler {
	return &BranchHandler{service: service}
}

func (h *BranchHandler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.service.ListBranches(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "gagal memuat daftar cabang")
		return
	}
	Success(w, http.StatusOK, "daftar cabang", branches)
}

// Create mendaftarkan cabang baru. Otorisasi ada di rute (branches:create);
// handler hanya merapikan masukan dan memetakan error bisnis.
func (h *BranchHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.CreateBranchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	branch, err := h.service.CreateBranch(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, domain.ErrBranchCodeExists) {
			status = http.StatusConflict
		}
		Fail(w, r, status, err)
		return
	}

	Success(w, http.StatusCreated, "cabang dibuat", branch)
}
