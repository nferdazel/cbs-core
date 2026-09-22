package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type StaffHandler struct {
	staffSvc domain.StaffService
}

func NewStaffHandler(staffSvc domain.StaffService) *StaffHandler {
	return &StaffHandler{staffSvc: staffSvc}
}

// Create handles POST /api/v1/staff
func (h *StaffHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var input domain.CreateStaffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	if input.Username == "" || input.FullName == "" || input.Email == "" || input.Password == "" || input.Role == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgStaffFieldsRequired)
		return
	}

	user, err := h.staffSvc.CreateStaff(r.Context(), input, actor)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusCreated, i18n.MsgStaffCreated, user)
}

// List handles GET /api/v1/staff
func (h *StaffHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	users, total, err := h.staffSvc.ListStaff(r.Context(), page, pageSize)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	SuccessWithMeta(w, http.StatusOK, i18n.MsgStaffListed, users, PaginationMeta{
		Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages,
	})
}

// GetByID handles GET /api/v1/staff/{id}
func (h *StaffHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidStaffUserID)
		return
	}

	user, err := h.staffSvc.GetStaff(r.Context(), id)
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgStaffRetrieved, user)
}

// Update handles PUT /api/v1/staff/{id}
func (h *StaffHandler) Update(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidStaffUserID)
		return
	}

	var input domain.UpdateStaffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		return
	}

	user, err := h.staffSvc.UpdateStaff(r.Context(), id, input, actor)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgStaffUpdated, user)
}

// ChangePassword handles POST /api/v1/staff/me/change-password
func (h *StaffHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var input domain.ChangePasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		return
	}

	if err := h.staffSvc.ChangePassword(r.Context(), actor.UserID, input, actor); err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPasswordChanged, nil)
}

// ResetPassword handles POST /api/v1/staff/{id}/reset-password (Admin only)
func (h *StaffHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidStaffUserID)
		return
	}

	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewPassword == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgNewPasswordRequired)
		return
	}

	if err := h.staffSvc.ResetPassword(r.Context(), id, body.NewPassword, actor); err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPasswordReset, nil)
}
