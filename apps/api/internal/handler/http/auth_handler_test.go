package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/middleware"
)

// /auth/me adalah sumber identitas web; cakupan buku instalasi harus ikut dilaporkan
// agar menu/halaman/pemilih buku disaring tanpa web menebak.
func TestAuthHandlerMeMelaporkanCakupanInstalasi(t *testing.T) {
	cases := []struct {
		scope     domain.InstitutionBookScope
		wantScope string
		wantBooks []string
	}{
		{domain.ScopeSyariah, "SYARIAH", []string{"SYARIAH"}},
		{domain.ScopeConventional, "KONVENSIONAL", []string{"CONVENTIONAL"}},
		{domain.ScopeDual, "DUAL", []string{"CONVENTIONAL", "SYARIAH"}},
		{"", "DUAL", []string{"CONVENTIONAL", "SYARIAH"}},
	}
	for _, tc := range cases {
		t.Run(tc.wantScope, func(t *testing.T) {
			h := httpHandler.NewAuthHandler(nil, middleware.CookieConfig{})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
			claims := &domain.JWTClaims{Username: "uji", Role: domain.RoleSuperAdmin, BookScope: tc.scope}
			req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
			rec := httptest.NewRecorder()

			h.Me(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d, ingin 200", rec.Code)
			}
			var body struct {
				Success bool `json:"success"`
				Data    struct {
					BookScope   string   `json:"book_scope"`
					ActiveBooks []string `json:"active_books"`
				} `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("respons bukan JSON valid: %v (%s)", err, rec.Body.String())
			}
			if !body.Success || body.Data.BookScope != tc.wantScope {
				t.Fatalf("book_scope=%q, ingin %q (body=%s)", body.Data.BookScope, tc.wantScope, rec.Body.String())
			}
			if len(body.Data.ActiveBooks) != len(tc.wantBooks) {
				t.Fatalf("active_books=%v, ingin %v", body.Data.ActiveBooks, tc.wantBooks)
			}
			for i := range body.Data.ActiveBooks {
				if body.Data.ActiveBooks[i] != tc.wantBooks[i] {
					t.Fatalf("active_books=%v, ingin %v", body.Data.ActiveBooks, tc.wantBooks)
				}
			}
		})
	}
}
