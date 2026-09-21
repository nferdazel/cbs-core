package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
)

// Uji bahwa token berpenanda kata sandi kedaluwarsa hanya boleh mengganti kata
// sandi. Rute lain harus dibalas 403 dengan pesan kedaluwarsa yang jelas — bukan
// pesan izin biasa — supaya antarmuka mengarahkan ke halaman ganti kata sandi.
func TestRequirePasswordChange(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	cases := []struct {
		name       string
		path       string
		claims     *domain.JWTClaims
		wantStatus int
	}{
		{
			name:       "kedaluwarsa boleh mengganti kata sandi",
			path:       "/api/v1/staff/me/change-password",
			claims:     &domain.JWTClaims{PasswordExpired: true},
			wantStatus: http.StatusOK,
		},
		{
			name:       "kedaluwarsa ditolak di rute bisnis",
			path:       "/api/v1/customers",
			claims:     &domain.JWTClaims{PasswordExpired: true},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "kedaluwarsa ditolak di rute staf lain",
			path:       "/api/v1/staff",
			claims:     &domain.JWTClaims{PasswordExpired: true},
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "token normal lolos di rute bisnis",
			path:       "/api/v1/customers",
			claims:     &domain.JWTClaims{PasswordExpired: false},
			wantStatus: http.StatusOK,
		},
		{
			name:       "tanpa claims ditolak",
			path:       "/api/v1/customers",
			claims:     nil,
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, nil)
			if tc.claims != nil {
				req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, tc.claims))
			}
			rec := httptest.NewRecorder()

			middleware.RequirePasswordChange(next).ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status %d, ingin %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantStatus == http.StatusForbidden {
				var body struct {
					Error string `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
					t.Fatalf("mengurai body: %v", err)
				}
				if !strings.Contains(body.Error, "kedaluwarsa") {
					t.Fatalf("pesan %q seharusnya menjelaskan kata sandi kedaluwarsa", body.Error)
				}
				if strings.Contains(body.Error, "does not have") {
					t.Fatalf("pesan tidak boleh tampak seperti galat izin biasa: %q", body.Error)
				}
			}
		})
	}
}
