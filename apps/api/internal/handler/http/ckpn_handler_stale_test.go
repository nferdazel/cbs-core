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
	h := httpHandler.NewCKPNHandler(staleCKPNService{})

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
