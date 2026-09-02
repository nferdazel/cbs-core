package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
)

type AuthHandler struct {
	authSvc domain.AuthService
}

func NewAuthHandler(authSvc domain.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Username == "" || body.Password == "" {
		Error(w, http.StatusBadRequest, "username and password are required")
		return
	}

	input := domain.LoginInput{
		Username:  body.Username,
		Password:  body.Password,
		IPAddress: r.RemoteAddr,
		UserAgent: r.UserAgent(),
	}

	resp, err := h.authSvc.Login(r.Context(), input)
	if err != nil {
		switch err {
		case domain.ErrInvalidCredentials:
			Error(w, http.StatusUnauthorized, err.Error())
		case domain.ErrAccountLocked:
			Error(w, http.StatusTooManyRequests, err.Error())
		case domain.ErrAccountInactiveUser:
			Error(w, http.StatusForbidden, err.Error())
		default:
			Error(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	Success(w, http.StatusOK, "login successful", resp)
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.RefreshToken == "" {
		Error(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	resp, err := h.authSvc.Refresh(r.Context(), body.RefreshToken)
	if err != nil {
		switch err {
		case domain.ErrSessionExpired, domain.ErrSessionRevoked, domain.ErrInvalidToken:
			Error(w, http.StatusUnauthorized, err.Error())
		default:
			Error(w, http.StatusInternalServerError, "token refresh failed")
		}
		return
	}

	Success(w, http.StatusOK, "token refreshed", resp)
}

// Logout handles POST /api/v1/auth/logout (requires valid access token)
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	if err := h.authSvc.Logout(r.Context(), claims.SessionID); err != nil {
		Error(w, http.StatusInternalServerError, "logout failed")
		return
	}

	Success(w, http.StatusOK, "logged out successfully", nil)
}

// Me handles GET /api/v1/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	Success(w, http.StatusOK, "current user", map[string]any{
		"user_id":     claims.UserID,
		"username":    claims.Username,
		"role":        claims.Role,
		"branch_code": claims.BranchCode,
		"permissions": domain.RolePermissions[claims.Role],
	})
}
