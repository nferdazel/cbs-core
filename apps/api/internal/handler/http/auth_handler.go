package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
)

type AuthHandler struct {
	authSvc domain.AuthService
	cookies middleware.CookieConfig
}

func NewAuthHandler(authSvc domain.AuthService, cookies middleware.CookieConfig) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, cookies: cookies}
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
			Fail(w, r, http.StatusUnauthorized, err)
		case domain.ErrAccountLocked:
			Fail(w, r, http.StatusTooManyRequests, err)
		case domain.ErrAccountInactiveUser:
			Fail(w, r, http.StatusForbidden, err)
		default:
			Error(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	csrfToken, err := middleware.GenerateCSRFToken()
	if err != nil {
		Error(w, http.StatusInternalServerError, "login failed")
		return
	}

	// Token tidak lagi dikirim di body: ditulis sebagai httpOnly cookie.
	h.cookies.SetSessionCookies(w, r, resp.AccessToken, resp.RefreshToken, resp.ExpiresIn, resp.RefreshExpiresIn)
	h.cookies.SetCSRFCookie(w, r, csrfToken, resp.RefreshExpiresIn)

	Success(w, http.StatusOK, "login successful", resp)
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := h.cookies.RefreshTokenFromRequest(r)

	// Kompatibilitas integrasi non-browser yang masih mengirim di body.
	// Cookie tetap diprioritaskan bila ada.
	if refreshToken == "" {
		var body struct {
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
			refreshToken = body.RefreshToken
		}
	}

	if refreshToken == "" {
		Error(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	resp, err := h.authSvc.Refresh(r.Context(), refreshToken)
	if err != nil {
		switch err {
		case domain.ErrSessionExpired, domain.ErrSessionRevoked, domain.ErrInvalidToken:
			Fail(w, r, http.StatusUnauthorized, err)
		default:
			Error(w, http.StatusInternalServerError, "token refresh failed")
		}
		return
	}

	csrfToken, err := middleware.GenerateCSRFToken()
	if err != nil {
		Error(w, http.StatusInternalServerError, "token refresh failed")
		return
	}

	// Rotasi: kedua cookie diperbarui, termasuk token CSRF.
	h.cookies.SetSessionCookies(w, r, resp.AccessToken, resp.RefreshToken, resp.ExpiresIn, resp.RefreshExpiresIn)
	h.cookies.SetCSRFCookie(w, r, csrfToken, resp.RefreshExpiresIn)

	Success(w, http.StatusOK, "token refreshed", resp)
}

// Logout handles POST /api/v1/auth/logout.
//
// Endpoint ini tidak berada di belakang AuthMiddleware: cookie harus tetap bisa
// dihapus walau access token sudah kedaluwarsa. Pencabutan sesi dilakukan
// best-effort bila token yang dikirim masih valid. CSRF tetap diwajibkan karena
// logout mengubah state.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if token := h.cookies.AccessTokenFromRequest(r); token != "" {
		if claims, err := h.authSvc.ValidateAccessToken(r.Context(), token); err == nil {
			_ = h.authSvc.Logout(r.Context(), claims.SessionID)
		}
	}

	h.cookies.ClearSessionCookies(w, r)
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
