package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
)

// orgUnitStubBranchService hanya melayani CreateOrgUnit; sisanya ditutup interface.
type orgUnitStubBranchService struct {
	domain.BranchService
	err error
}

func (s orgUnitStubBranchService) CreateOrgUnit(context.Context, domain.CreateOrgUnitInput, domain.Actor) (*domain.Branch, error) {
	return nil, s.err
}

// Kode unit yang melebihi lebar kolom harus ditolak 422 dengan pesan yang menyebut
// batas panjangnya, bukan 500. Bila handler memakai InternalError, status menjadi
// 500 dan uji ini gagal.
func TestCreateOrgUnitKodeTerlaluPanjang422(t *testing.T) {
	h := httpHandler.NewBranchHandler(orgUnitStubBranchService{err: domain.ErrOrgUnitCodeTooLong})

	body := strings.NewReader(`{"code":"AREA-JABAR","level":"AREA","name":"Area Jabar"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/branches/org-units", body)
	claims := &domain.JWTClaims{UserID: [16]byte{2}, Username: "uji-area", Role: domain.RoleSuperAdmin}
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))

	rec := httptest.NewRecorder()
	h.CreateOrgUnit(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422", rec.Code)
	}
	var resp struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("respons bukan JSON: %v", err)
	}
	if !strings.Contains(resp.Error, "8 karakter") {
		t.Fatalf("pesan tidak menyebut batas panjang: %q", resp.Error)
	}
}
