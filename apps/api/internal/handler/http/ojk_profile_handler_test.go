package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// ojkProfileSvcStub melayani rute tanpa database dan merekam pemanggilan tulis.
type ojkProfileSvcStub struct {
	profile     *domain.OJKProfile
	updateCalls int
}

func (s *ojkProfileSvcStub) Get(context.Context) (*domain.OJKProfile, error) {
	return s.profile, nil
}

func (s *ojkProfileSvcStub) Update(_ context.Context, input domain.UpdateOJKProfileInput, _ domain.Actor) (*domain.OJKProfile, error) {
	s.updateCalls++
	if input.BankEmail != nil {
		s.profile.BankEmail = *input.BankEmail
	}
	return s.profile, nil
}

func newOJKProfileTestRouter(role domain.StaffRole) (http.Handler, *ojkProfileSvcStub) {
	svc := &ojkProfileSvcStub{profile: &domain.OJKProfile{BankEmail: "bpr@uji.id"}}
	router := NewRouter(RouterParams{
		CustomerHandler:     &CustomerHandler{},
		AccountHandler:      &AccountHandler{},
		BranchHandler:       &BranchHandler{},
		ProductHandler:      &ProductHandler{},
		LedgerHandler:       &LedgerHandler{},
		AuthHandler:         &AuthHandler{},
		StaffHandler:        &StaffHandler{},
		LoanHandler:         &LoanHandler{},
		MakerCheckerHandler: &MakerCheckerHandler{},
		ReportHandler:       NewReportHandler(reportSvcStub{}),
		CollectionHandler:   &CollectionHandler{},
		IntegrationHandler:  &IntegrationHandler{},
		BatchProcessHandler: &BatchProcessHandler{},
		DocumentHandler:     &DocumentHandler{},
		AppInfoHandler:      &AppInfoHandler{},
		OJKProfileHandler:   NewOJKProfileHandler(svc),
		AuthService:         reportAuthStub{role: role},
	})
	return router, svc
}

func ojkProfileRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer uji")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TELLER tidak memegang system:config:read maupun system:config: GET dan PUT harus 403.
func TestRuteOJKProfileTolakTeller(t *testing.T) {
	router, svc := newOJKProfileTestRouter(domain.RoleTeller)

	if code := ojkProfileRequest(t, router, http.MethodGet, "/api/v1/system/ojk-profile", "").Code; code != http.StatusForbidden {
		t.Fatalf("TELLER GET = %d, ingin 403", code)
	}
	if code := ojkProfileRequest(t, router, http.MethodPut, "/api/v1/system/ojk-profile", `{"bank_email":"x@y.id"}`).Code; code != http.StatusForbidden {
		t.Fatalf("TELLER PUT = %d, ingin 403", code)
	}
	if svc.updateCalls != 0 {
		t.Fatalf("service tulis dipanggil %d kali, ingin 0 saat ditolak", svc.updateCalls)
	}
}

// AUDITOR boleh membaca (system:config:read) tetapi TIDAK boleh mengubah.
func TestRuteOJKProfileAuditorBacaTanpaUbah(t *testing.T) {
	router, _ := newOJKProfileTestRouter(domain.RoleAuditor)

	if code := ojkProfileRequest(t, router, http.MethodGet, "/api/v1/system/ojk-profile", "").Code; code != http.StatusOK {
		t.Fatalf("AUDITOR GET = %d, ingin 200", code)
	}
	if code := ojkProfileRequest(t, router, http.MethodPut, "/api/v1/system/ojk-profile", `{"bank_email":"x@y.id"}`).Code; code != http.StatusForbidden {
		t.Fatalf("AUDITOR PUT = %d, ingin 403 (tanpa system:config)", code)
	}
}

// SUPERADMIN dapat mengubah; payload dengan bidang tak dikenal ditolak 422 agar
// kontrak field tetap terjaga.
func TestRuteOJKProfileSuperadminUbah(t *testing.T) {
	router, svc := newOJKProfileTestRouter(domain.RoleSuperAdmin)

	rec := ojkProfileRequest(t, router, http.MethodPut, "/api/v1/system/ojk-profile", `{"bank_email":"baru@uji.id"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("SUPERADMIN PUT = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.updateCalls != 1 || svc.profile.BankEmail != "baru@uji.id" {
		t.Fatalf("service tulis = %d panggilan, profil = %+v", svc.updateCalls, svc.profile)
	}

	rec = ojkProfileRequest(t, router, http.MethodPut, "/api/v1/system/ojk-profile", `{"unknown_field":"x"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bidang tak dikenal = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}
