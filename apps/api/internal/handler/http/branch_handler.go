package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
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
		ErrorCode(w, http.StatusInternalServerError, i18n.MsgBranchListFailed)
		return
	}
	Success(w, http.StatusOK, i18n.MsgBranchList, branches)
}

// Create mendaftarkan cabang baru. Otorisasi ada di rute (branches:create);
// handler hanya merapikan masukan dan memetakan error bisnis.
func (h *BranchHandler) Create(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.CreateBranchInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
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

	Success(w, http.StatusCreated, i18n.MsgBranchCreated, branch)
}

// ListOrgUnits menampilkan seluruh unit organisasi (CABANG/AREA/WILAYAH) beserta
// atasannya. Bila bank tidak memakai area/wilayah, isinya sama dengan daftar
// cabang, sehingga tidak ada perilaku baru.
func (h *BranchHandler) ListOrgUnits(w http.ResponseWriter, r *http.Request) {
	units, err := h.service.ListOrgUnits(r.Context())
	if err != nil {
		ErrorCode(w, http.StatusInternalServerError, i18n.MsgOrgUnitListFailed)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOrgUnitList, units)
}

// CreateOrgUnit menambah cabang/area/wilayah. Otorisasi ada di rute
// (branches:create): susunan organisasi adalah fungsi administratif kantor pusat,
// sama seperti pembuatan cabang.
func (h *BranchHandler) CreateOrgUnit(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.CreateOrgUnitInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	unit, err := h.service.CreateOrgUnit(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusCreated, i18n.MsgOrgUnitCreated, unit)
}

// SetOrgUnitParent memindahkan unit ke bawah atasan lain, atau melepasnya menjadi
// puncak bila parent_code kosong.
func (h *BranchHandler) SetOrgUnitParent(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.SetOrgUnitParentInput
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&input)
	}

	unit, err := h.service.SetOrgUnitParent(r.Context(), chi.URLParam(r, "code"), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// Pemindahan yang berdampak pada pengguna aktif bukan kegagalan: ia masuk
		// antrean maker-checker dan dibalas 202 beserta id permintaannya.
		var pending *domain.PendingApprovalError
		if errors.As(err, &pending) {
			Success(w, http.StatusAccepted, i18n.MsgTransactionPendingApproval, map[string]any{
				"request_id":  pending.RequestID,
				"action_type": pending.ActionType,
				"status":      "PENDING_APPROVAL",
			})
			return
		}
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOrgUnitParentUpdated, unit)
}
