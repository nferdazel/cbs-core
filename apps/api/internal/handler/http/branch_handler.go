package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
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
