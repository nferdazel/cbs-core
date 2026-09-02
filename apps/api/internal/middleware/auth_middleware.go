package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
)

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": msg})
}

// AuthMiddleware validates the Bearer JWT and injects claims into context.
func AuthMiddleware(authSvc domain.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				writeError(w, http.StatusUnauthorized, "missing or invalid authorization header")
				return
			}

			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
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
