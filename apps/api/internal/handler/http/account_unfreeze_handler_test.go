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

// unfreezeStubAccountSvc hanya melayani UnfreezeAccount; metode lain diambil dari
// antarmuka sehingga test handler tidak menyiapkan seluruh layanan rekening.
type unfreezeStubAccountSvc struct {
	domain.AccountService
	err error
}

func (s unfreezeStubAccountSvc) UnfreezeAccount(context.Context, string, string, domain.Actor) (*domain.Account, error) {
	return nil, s.err
}

// Rekening berstatus selain FROZEN harus dibalas 422 dengan pesan domain, bukan 500.
func TestUnfreezeHandler_StatusTidakSah422(t *testing.T) {
	h := NewAccountHandler(unfreezeStubAccountSvc{err: domain.ErrAccountNotFrozen})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/123/unfreeze", strings.NewReader(`{}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("accountNumber", "123")
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.ContextKeyClaims, &domain.JWTClaims{
		Username: "uji-unfreeze", Role: domain.RoleSuperAdmin, BranchCode: "001",
	})

	rec := httptest.NewRecorder()
	h.Unfreeze(rec, req.WithContext(ctx))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422 (body: %s)", rec.Code, rec.Body.String())
	}
}
