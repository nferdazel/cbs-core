package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// ckpnActivationSvcStub melayani rute tanpa database dan merekam pemanggilan tulis.
type ckpnActivationSvcStub struct {
	activation  *domain.CKPNActivation
	updateCalls int
	updateErr   error
}

func (s *ckpnActivationSvcStub) Get(context.Context) (*domain.CKPNActivation, error) {
	return s.activation, nil
}

func (s *ckpnActivationSvcStub) Update(_ context.Context, _ domain.UpdateCKPNActivationInput, _ domain.Actor) (*domain.CKPNActivation, error) {
	s.updateCalls++
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	return s.activation, nil
}

func newCKPNActivationTestRouter(role domain.StaffRole, svc *ckpnActivationSvcStub) http.Handler {
	return NewRouter(RouterParams{
		CustomerHandler:       &CustomerHandler{},
		AccountHandler:        &AccountHandler{},
		BranchHandler:         &BranchHandler{},
		ProductHandler:        &ProductHandler{},
		LedgerHandler:         &LedgerHandler{},
		AuthHandler:           &AuthHandler{},
		StaffHandler:          &StaffHandler{},
		LoanHandler:           &LoanHandler{},
		MakerCheckerHandler:   &MakerCheckerHandler{},
		ReportHandler:         NewReportHandler(reportSvcStub{}),
		CollectionHandler:     &CollectionHandler{},
		IntegrationHandler:    &IntegrationHandler{},
		BatchProcessHandler:   &BatchProcessHandler{},
		DocumentHandler:       &DocumentHandler{},
		AppInfoHandler:        &AppInfoHandler{},
		CKPNActivationHandler: NewCKPNActivationHandler(svc),
		AuthService:           reportAuthStub{role: role},
	})
}

func ckpnActivationRequest(t *testing.T, router http.Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/system/ckpn-activation", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer uji")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func stubCKPNActivation() *ckpnActivationSvcStub {
	return &ckpnActivationSvcStub{activation: &domain.CKPNActivation{
		Values:          map[string]string{domain.CKPNLGDFracKey: "0.45"},
		EnablementReady: false,
		EnablementGaps:  []string{"parameter belum FINAL"},
	}}
}

// TELLER tidak memegang system:config:read maupun system:config: GET dan PUT 403, dan
// service tulis tidak boleh tersentuh.
func TestRuteCKPNActivationTolakTeller(t *testing.T) {
	svc := stubCKPNActivation()
	router := newCKPNActivationTestRouter(domain.RoleTeller, svc)

	if code := ckpnActivationRequest(t, router, http.MethodGet, "").Code; code != http.StatusForbidden {
		t.Fatalf("TELLER GET = %d, ingin 403", code)
	}
	if code := ckpnActivationRequest(t, router, http.MethodPut, `{"lgd_frac":"0.45"}`).Code; code != http.StatusForbidden {
		t.Fatalf("TELLER PUT = %d, ingin 403", code)
	}
	if svc.updateCalls != 0 {
		t.Fatalf("service tulis dipanggil %d kali, ingin 0 saat ditolak", svc.updateCalls)
	}
}

// AUDITOR boleh membaca (system:config:read) tetapi TIDAK boleh mengubah.
func TestRuteCKPNActivationAuditorBacaTanpaUbah(t *testing.T) {
	svc := stubCKPNActivation()
	router := newCKPNActivationTestRouter(domain.RoleAuditor, svc)

	if code := ckpnActivationRequest(t, router, http.MethodGet, "").Code; code != http.StatusOK {
		t.Fatalf("AUDITOR GET = %d, ingin 200", code)
	}
	if code := ckpnActivationRequest(t, router, http.MethodPut, `{"lgd_frac":"0.45"}`).Code; code != http.StatusForbidden {
		t.Fatalf("AUDITOR PUT = %d, ingin 403 (tanpa system:config)", code)
	}
}

// SUPERADMIN dapat membaca dan mengubah.
func TestRuteCKPNActivationSuperadminUbah(t *testing.T) {
	svc := stubCKPNActivation()
	router := newCKPNActivationTestRouter(domain.RoleSuperAdmin, svc)

	rec := ckpnActivationRequest(t, router, http.MethodGet, "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "enablement_gaps") {
		t.Fatalf("SUPERADMIN GET = %d, body=%s", rec.Code, rec.Body.String())
	}

	rec = ckpnActivationRequest(t, router, http.MethodPut, `{"lgd_frac":"0.45"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("SUPERADMIN PUT = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.updateCalls != 1 {
		t.Fatalf("service tulis = %d panggilan, ingin 1", svc.updateCalls)
	}
}

// Penolakan aturan bisnis (penyalakan prematur) dibalas 422 dengan pesan yang terlihat.
func TestRuteCKPNActivationTolakPenyalakanPrematur(t *testing.T) {
	svc := stubCKPNActivation()
	svc.updateErr = domain.ErrCKPNActivationNotReady
	router := newCKPNActivationTestRouter(domain.RoleSuperAdmin, svc)

	rec := ckpnActivationRequest(t, router, http.MethodPut, `{"ckpn_enabled":true}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("penyalakan prematur = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}

// Permintaan kosong dibalas 400, bukan tercatat sebagai perubahan.
func TestRuteCKPNActivationTolakKosong(t *testing.T) {
	svc := stubCKPNActivation()
	svc.updateErr = domain.ErrCKPNActivationEmpty
	router := newCKPNActivationTestRouter(domain.RoleSuperAdmin, svc)

	rec := ckpnActivationRequest(t, router, http.MethodPut, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("permintaan kosong = %d, ingin 400; body=%s", rec.Code, rec.Body.String())
	}
}
