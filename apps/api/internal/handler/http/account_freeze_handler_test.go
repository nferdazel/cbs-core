package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// freezeStubAccountSvc hanya melayani FreezeAccount; metode lain diambil dari
// antarmuka sehingga test handler tidak menyiapkan seluruh layanan rekening.
type freezeStubAccountSvc struct {
	domain.AccountService
	err error
}

func (s freezeStubAccountSvc) FreezeAccount(context.Context, string, string, domain.Actor) (*domain.Account, error) {
	return nil, s.err
}

// Pembekuan rekening berstatus selain ACTIVE harus dibalas 422 dengan pesan domain,
// bukan 500; ini juga membuktikan rute memakai izin accounts:freeze yang sudah ada.
func TestFreezeHandler_StatusTidakSah422(t *testing.T) {
	h := NewAccountHandler(freezeStubAccountSvc{err: domain.ErrAccountNotFreezable})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/123/freeze", strings.NewReader(`{}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountNumber", "123")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.ContextKeyClaims, &domain.JWTClaims{
		Username: "uji-freeze", Role: domain.RoleSupervisor, BranchCode: "001",
	})

	rec := httptest.NewRecorder()
	h.Freeze(rec, req.WithContext(ctx))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422 (body: %s)", rec.Code, rec.Body.String())
	}
}
