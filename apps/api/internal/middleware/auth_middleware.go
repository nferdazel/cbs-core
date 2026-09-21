package middleware

import (
	"context"
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
)

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg})
}

// AuthMiddleware validates the access token and injects claims into context.
// Token diambil dari httpOnly cookie (prioritas), lalu dari header
// Authorization: Bearer untuk kompatibilitas integrasi non-browser.
func AuthMiddleware(authSvc domain.AuthService, cookies CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := cookies.AccessTokenFromRequest(r)
			if tokenStr == "" {
				writeError(w, http.StatusUnauthorized, "missing or invalid access token")
				return
			}

			claims, err := authSvc.ValidateAccessToken(r.Context(), tokenStr)
			if err != nil {
				switch err {
				case domain.ErrSessionExpired:
					writeError(w, http.StatusUnauthorized, "access token expired")
				default:
					writeError(w, http.StatusUnauthorized, "invalid access token")
				}
				return
			}

			// Inject claims into context
			ctx := context.WithValue(r.Context(), domain.ContextKeyClaims, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// passwordExpiredAllowedPaths adalah rute yang tetap boleh diakses saat kata
// sandi kedaluwarsa. Hanya ganti kata sandi; lihat RequirePasswordChange untuk
// alasan rute profil (/api/v1/auth/me) dan logout tidak ada di sini.
var passwordExpiredAllowedPaths = map[string]bool{
	"/api/v1/staff/me/change-password": true,
}

// RequirePasswordChange membatasi token yang diterbitkan saat kata sandi sudah
// kedaluwarsa (klaim pwd_expired). Token semacam itu bukan ditolak saat login —
// menolak login akan mengunci pengguna tanpa jalur pulih — melainkan hanya boleh
// dipakai untuk mengganti kata sandi; rute lain dibalas 403 dengan pesan yang
// jelas, bukan galat izin biasa, supaya antarmuka mengarahkan ke halaman ganti
// kata sandi. Lihat profil sendiri (/api/v1/auth/me) dan logout sengaja berada
// di luar jangkauan middleware ini (router.go), sehingga tetap dapat dipakai.
// Harus dipasang SETELAH AuthMiddleware.
func RequirePasswordChange(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := domain.ClaimsFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if !claims.PasswordExpired || passwordExpiredAllowedPaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		writeError(w, http.StatusForbidden, domain.ErrPasswordExpired.Error())
	})
}

// RequirePermission returns a middleware that checks if the authenticated user
// has the specified permission. Must be used AFTER AuthMiddleware.
func RequirePermission(perm domain.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := domain.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			if !claims.Role.HasPermission(perm) {
				writeError(w, http.StatusForbidden,
					"forbidden: your role ("+string(claims.Role)+") does not have '"+string(perm)+"' permission")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole restricts access to one or more specific roles.
func RequireRole(roles ...domain.StaffRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := domain.ClaimsFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "authentication required")
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, http.StatusForbidden, "forbidden: insufficient role")
		})
	}
}
