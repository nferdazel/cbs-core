package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// bankProfileSvcStub melayani rute tanpa database dan merekam pemanggilan tulis,
// supaya bukti otorisasi tidak tercampur dengan efek samping.
type bankProfileSvcStub struct {
	profile     *domain.BankProfile
	updateCalls int
}

func (s *bankProfileSvcStub) Get(context.Context) (*domain.BankProfile, error) {
	return s.profile, nil
}

func (s *bankProfileSvcStub) Update(_ context.Context, input domain.UpdateBankProfileInput, _ domain.Actor) (*domain.BankProfile, error) {
	s.updateCalls++
	if input.Name != nil {
		s.profile.Name = *input.Name
	}
	return s.profile, nil
}

// newBankProfileTestRouter merakit router produksi dengan handler profil bank dan
// autentikasi tiruan berperan tetap.
func newBankProfileTestRouter(role domain.StaffRole) (http.Handler, *bankProfileSvcStub) {
	svc := &bankProfileSvcStub{profile: &domain.BankProfile{Name: "BPR Uji"}}
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
		BankProfileHandler:  NewBankProfileHandler(svc),
		AuthService:         reportAuthStub{role: role},
	})
	return router, svc
}

func bankProfileRequest(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
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

// TELLER tidak memegang system:config:read maupun system:config: GET dan PUT
// profil bank harus ditolak tanpa menyentuh service.
func TestRuteBankProfileTolakTeller(t *testing.T) {
	router, svc := newBankProfileTestRouter(domain.RoleTeller)

	if code := bankProfileRequest(t, router, http.MethodGet, "/api/v1/system/bank-profile", "").Code; code != http.StatusForbidden {
		t.Fatalf("TELLER GET = %d, ingin 403 (tanpa system:config:read)", code)
	}
	rec := bankProfileRequest(t, router, http.MethodPut, "/api/v1/system/bank-profile", `{"name":"BPR Baru"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("TELLER PUT = %d, ingin 403 (tanpa system:config)", rec.Code)
	}
	if svc.updateCalls != 0 {
		t.Fatalf("service tulis dipanggil %d kali, ingin 0 saat ditolak", svc.updateCalls)
	}
}

// AUDITOR boleh membaca (system:config:read) tetapi TIDAK boleh mengubah
// (system:config tetap khusus Superadmin) — membaca dan menulis dipisah.
func TestRuteBankProfileAuditorBacaTanpaUbah(t *testing.T) {
	router, _ := newBankProfileTestRouter(domain.RoleAuditor)

	if code := bankProfileRequest(t, router, http.MethodGet, "/api/v1/system/bank-profile", "").Code; code != http.StatusOK {
		t.Fatalf("AUDITOR GET = %d, ingin 200", code)
	}
	if code := bankProfileRequest(t, router, http.MethodPut, "/api/v1/system/bank-profile", `{"name":"BPR Baru"}`).Code; code != http.StatusForbidden {
		t.Fatalf("AUDITOR PUT = %d, ingin 403 (tanpa system:config)", code)
	}
}

// SUPERADMIN dapat mengubah profil, dan payload dengan bidang tak dikenal ditolak
// 422 agar kolom lain (id/updated_at) tidak dapat disentuh.
func TestRuteBankProfileSuperadminUbah(t *testing.T) {
	router, svc := newBankProfileTestRouter(domain.RoleSuperAdmin)

	rec := bankProfileRequest(t, router, http.MethodPut, "/api/v1/system/bank-profile", `{"name":"BPR Baru Sejahtera"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("SUPERADMIN PUT = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if svc.updateCalls != 1 || svc.profile.Name != "BPR Baru Sejahtera" {
		t.Fatalf("service tulis = %d panggilan, profil = %+v", svc.updateCalls, svc.profile)
	}

	rec = bankProfileRequest(t, router, http.MethodPut, "/api/v1/system/bank-profile", `{"updated_at":"2020-01-01"}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bidang tak dikenal = %d, ingin 422; body=%s", rec.Code, rec.Body.String())
	}
}
