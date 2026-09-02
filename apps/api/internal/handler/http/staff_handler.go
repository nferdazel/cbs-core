package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
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
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.CreateStaffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.Username == "" || input.FullName == "" || input.Email == "" || input.Password == "" || input.Role == "" {
		Error(w, http.StatusBadRequest, "username, full_name, email, password, and role are required")
		return
	}

	user, err := h.staffSvc.CreateStaff(r.Context(), input, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusCreated, "staff user created successfully", user)
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
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	SuccessWithMeta(w, http.StatusOK, "staff users listed", users, PaginationMeta{
		Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages,
	})
}

// GetByID handles GET /api/v1/staff/{id}
func (h *StaffHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid staff user id")
		return
	}

	user, err := h.staffSvc.GetStaff(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	Success(w, http.StatusOK, "staff user retrieved", user)
}

// Update handles PUT /api/v1/staff/{id}
func (h *StaffHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid staff user id")
		return
	}

	var input domain.UpdateStaffInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.staffSvc.UpdateStaff(r.Context(), id, input)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "staff user updated", user)
}

// ChangePassword handles POST /api/v1/staff/me/change-password
func (h *StaffHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.ChangePasswordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.staffSvc.ChangePassword(r.Context(), claims.UserID, input); err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "password changed successfully", nil)
}

// ResetPassword handles POST /api/v1/staff/{id}/reset-password (Admin only)
func (h *StaffHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid staff user id")
		return
	}

	var body struct {
		NewPassword string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NewPassword == "" {
		Error(w, http.StatusBadRequest, "new_password is required")
		return
	}

	if err := h.staffSvc.ResetPassword(r.Context(), id, body.NewPassword, claims.UserID); err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "password reset successfully", nil)
}
