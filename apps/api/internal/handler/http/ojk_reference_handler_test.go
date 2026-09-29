package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
)

// ojkReferenceStub menyajikan daftar sandi referensi OJK tetap, sudah terurut, untuk
// menguji handler tanpa basis data.
type ojkReferenceStub struct {
	creditors []domain.OJKReferenceOption
	regencies []domain.OJKReferenceOption
	err       error
}

func (s ojkReferenceStub) ListOJKCreditorGroups(context.Context) ([]domain.OJKReferenceOption, error) {
	return s.creditors, s.err
}

func (s ojkReferenceStub) ListOJKRegencies(context.Context) ([]domain.OJKReferenceOption, error) {
	return s.regencies, s.err
}

func newOJKReferenceHandler() *OJKReportHandler {
	return &OJKReportHandler{reference: ojkReferenceStub{
		creditors: []domain.OJKReferenceOption{
			{Code: "000", Name: "Tanpa penjamin"},
			{Code: "001", Name: "Bank Indonesia"},
			{Code: "600", Name: "BPR"},
		},
		regencies: []domain.OJKReferenceOption{
			{Code: "0102", Name: "Kab. Bekasi", Province: "Jawa Barat"},
			{Code: "0103", Name: "Kab. Purwakarta", Province: "Jawa Barat"},
		},
	}}
}

// Endpoint pihak lawan menyajikan item `code`/`name` berurutan menurut sandi.
func TestListCreditorGroupsMenyajikanDaftarTerurut(t *testing.T) {
	h := newOJKReferenceHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/reference/creditor-groups", nil)

	h.ListCreditorGroups(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Items []domain.OJKReferenceOption `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body bukan JSON: %v; body=%s", err, rec.Body.String())
	}
	if len(body.Data.Items) != 3 {
		t.Fatalf("jumlah item = %d, ingin 3", len(body.Data.Items))
	}
	if body.Data.Items[0].Code != "000" || body.Data.Items[2].Code != "600" {
		t.Fatalf("urutan item = %+v, ingin terurut menurut sandi", body.Data.Items)
	}
	if !strings.Contains(rec.Body.String(), `"name":"Bank Indonesia"`) {
		t.Fatalf("field name tidak disajikan: %s", rec.Body.String())
	}
}

// Endpoint kabupaten/kota menyajikan provinsi sekaligus, urut menurut sandi.
func TestListRegenciesMenyajikanProvinsiTerurut(t *testing.T) {
	h := newOJKReferenceHandler()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/reference/regencies", nil)

	h.ListRegencies(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Items []domain.OJKReferenceOption `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body bukan JSON: %v; body=%s", err, rec.Body.String())
	}
	if len(body.Data.Items) != 2 {
		t.Fatalf("jumlah item = %d, ingin 2", len(body.Data.Items))
	}
	if body.Data.Items[0].Code != "0102" || body.Data.Items[1].Code != "0103" {
		t.Fatalf("urutan item = %+v, ingin terurut menurut sandi", body.Data.Items)
	}
	if body.Data.Items[0].Province != "Jawa Barat" {
		t.Fatalf("provinsi = %q, ingin Jawa Barat", body.Data.Items[0].Province)
	}
}

// routeByMethodPath mengambil handler rute produksi (termasuk rantai middleware-nya)
// dari router yang sudah terpasang lewat RegisterRoutes.
func routeByMethodPath(t *testing.T, router chi.Router, method, pattern string) http.Handler {
	t.Helper()
	var found http.Handler
	if err := chi.Walk(router, func(m, route string, handler http.Handler, mws ...func(http.Handler) http.Handler) error {
		if m != method || route != pattern {
			return nil
		}
		h := handler
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		found = h
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	if found == nil {
		t.Fatalf("rute %s %s tidak terdaftar", method, pattern)
	}
	return found
}

// Rute referensi dijaga izin baca konfigurasi (system:config:read): tanpa izin 403,
// dengan izin diteruskan ke handler. Membuktikan penjagaan yang benar-benar terpasang,
// bukan sekadar ada di kode.
func TestRuteReferensiMemakaiIzinBacaKonfigurasi(t *testing.T) {
	router := chi.NewRouter()
	newOJKReferenceHandler().RegisterRoutes(router)

	cases := []struct {
		name   string
		claims *domain.JWTClaims
		want   int
	}{
		{
			name:   "peran tanpa izin ditolak",
			claims: &domain.JWTClaims{Role: domain.RoleTeller},
			want:   http.StatusForbidden,
		},
		{
			name: "izin baca konfigurasi diterima",
			claims: &domain.JWTClaims{
				Role: domain.RoleAuditor, PermissionsLoaded: true,
				Permissions: []domain.Permission{domain.PermSystemConfigRead},
			},
			want: http.StatusOK,
		},
	}

	for _, pattern := range []string{
		"/ojk/reference/creditor-groups",
		"/ojk/reference/regencies",
	} {
		handler := routeByMethodPath(t, router, http.MethodGet, pattern)
		for _, tc := range cases {
			t.Run(pattern+"/"+tc.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/api/v1/reports"+pattern, nil)
				req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, tc.claims))
				rec := httptest.NewRecorder()

				handler.ServeHTTP(rec, req)

				if rec.Code != tc.want {
					t.Fatalf("status = %d, ingin %d; body=%s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}
	}
}
