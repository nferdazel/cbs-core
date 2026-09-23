package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"github.com/shopspring/decimal"
)

// staleCKPNService meniru modul CKPN yang menolak perbandingan karena PPKA milik
// tanggal bisnis lain. Dipakai untuk membuktikan handler memetakannya ke 4xx yang
// jelas, bukan 500 generik.
type staleCKPNService struct {
	domain.CKPNService
}

func (staleCKPNService) Compare(context.Context, time.Time, domain.Actor) (domain.CKPNComparisonSummary, error) {
	return domain.CKPNComparisonSummary{}, domain.ErrCKPNStalePPAP
}

func (staleCKPNService) Run(context.Context, time.Time, domain.Actor) (domain.CKPNComparisonSummary, error) {
	return domain.CKPNComparisonSummary{}, domain.ErrCKPNStalePPAP
}

func ckpnRequestWithClaims(t *testing.T, method, target string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	claims := &domain.JWTClaims{
		UserID:   [16]byte{1},
		Username: "uji-ckpn",
		Role:     domain.RoleSuperAdmin,
	}
	return req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
}

// PPKA basi adalah kondisi operator (tanggal bisnis PPAP berbeda), jadi harus
// menjadi 409 dengan pesan yang menyebut sebabnya. Dengan InternalError lama,
// statusnya 500 dan pesannya generik — uji ini gagal.
func TestCKPNPPAPBasiDibalas409(t *testing.T) {
	h := httpHandler.NewCKPNHandler(staleCKPNService{}, nil)

	for _, tc := range []struct {
		name   string
		method string
		target string
		call   func(w http.ResponseWriter, r *http.Request)
	}{
		{"compare", http.MethodGet, "/api/v1/ckpn/comparison", h.Compare},
		{"run", http.MethodPost, "/api/v1/ckpn/run", h.Run},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.call(rec, ckpnRequestWithClaims(t, tc.method, tc.target))

			if rec.Code != http.StatusConflict {
				t.Fatalf("status = %d, ingin 409", rec.Code)
			}
			if body := rec.Body.String(); !strings.Contains(body, domain.ErrCKPNStalePPAP.Error()) {
				t.Fatalf("pesan tidak menyebut PPKA basi: %s", body)
			}
		})
	}
}

// ckpnStatusConfigStub menyajikan status parameter CKPN dari peta; kunci lain jatuh
// ke fallback.
type ckpnStatusConfigStub struct{ values map[string]string }

func (s ckpnStatusConfigStub) GetString(_ context.Context, key, fallback string) string {
	if v, ok := s.values[key]; ok {
		return v
	}
	return fallback
}
func (ckpnStatusConfigStub) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (ckpnStatusConfigStub) GetInt(context.Context, string, int) int { return 0 }

// GetBool membaca peta yang sama dengan GetString supaya saklar seperti ckpn.enabled
// benar-benar diuji di handler ini (blokir ekspor OJK bergantung padanya).
func (s ckpnStatusConfigStub) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := s.values[key]; ok {
		return v == "true" || v == "1"
	}
	return fallback
}
func (ckpnStatusConfigStub) Invalidate(string) {}

// Endpoint status parameter dapat diperiksa tanpa menjalankan EOD: SEMENTARA + CKPN
// resmi menyala wajib menandai blokir OJK; SEMENTARA selama CKPN masih mati tidak
// (laporan OJK memuat PPKA), dan FINAL tidak pernah memblokir.
func TestCKPNHandlerStatusParameter(t *testing.T) {
	h := httpHandler.NewCKPNHandler(nil, ckpnStatusConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
		domain.ConfigKeyCKPNEnabled:          "true",
	}})
	rec := httptest.NewRecorder()
	h.Status(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ckpn/status", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"SEMENTARA"`) || !strings.Contains(body, `"ojk_export_blocked":true`) {
		t.Fatalf("status SEMENTARA + CKPN nyala harus memblokir OJK: %s", body)
	}
	if !strings.Contains(body, "FINAL") {
		t.Fatalf("alasan harus menyebut langkah FINAL: %s", body)
	}

	// CKPN masih mati: laporan OJK memuat PPKA, jadi belum ditutup.
	h = httpHandler.NewCKPNHandler(nil, ckpnStatusConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
		domain.ConfigKeyCKPNEnabled:          "false",
	}})
	rec = httptest.NewRecorder()
	h.Status(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ckpn/status", nil))
	if !strings.Contains(rec.Body.String(), `"ojk_export_blocked":false`) {
		t.Fatalf("CKPN mati tidak boleh menutup ekspor: %s", rec.Body.String())
	}

	h = httpHandler.NewCKPNHandler(nil, ckpnStatusConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusFinal,
		domain.ConfigKeyCKPNEnabled:          "true",
	}})
	rec = httptest.NewRecorder()
	h.Status(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ckpn/status", nil))
	if !strings.Contains(rec.Body.String(), `"ojk_export_blocked":false`) {
		t.Fatalf("status FINAL tidak boleh memblokir: %s", rec.Body.String())
	}
}
