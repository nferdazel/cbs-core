package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"github.com/shopspring/decimal"
)

// ojkCKPNConfigStub menyajikan kunci status parameter CKPN dari peta; kunci lain jatuh
// ke fallback. Dipakai membuktikan ekspor ditolak saat SEMENTARA dan boleh saat FINAL.
type ojkCKPNConfigStub struct{ values map[string]string }

func (s ojkCKPNConfigStub) GetString(_ context.Context, key, fallback string) string {
	if v, ok := s.values[key]; ok {
		return v
	}
	return fallback
}
func (ojkCKPNConfigStub) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (ojkCKPNConfigStub) GetInt(context.Context, string, int) int { return 0 }

// GetBool membaca peta yang sama dengan GetString: blokir ekspor OJK bergantung pada
// ckpn.enabled, jadi stub ini harus benar-benar membacanya agar uji bermakna.
func (s ojkCKPNConfigStub) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := s.values[key]; ok {
		return v == "true" || v == "1"
	}
	return fallback
}
func (ojkCKPNConfigStub) Invalidate(string) {}

var _ domain.SystemConfigService = ojkCKPNConfigStub{}

type stubOJKSource struct{}

func (stubOJKSource) GetBalanceSheet(_ context.Context, _ time.Time, _ string) (*domain.BalanceSheet, error) {
	return &domain.BalanceSheet{}, nil
}

func (stubOJKSource) GetIncomeStatement(_ context.Context, _, _ time.Time, _ string) (*domain.IncomeStatement, error) {
	return &domain.IncomeStatement{}, nil
}

func TestParseOJKPeriod(t *testing.T) {
	if got, err := parseOJKPeriod("2026-03"); err != nil || !got.Equal(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("YYYY-MM: got %s err %v", got, err)
	}
	if got, err := parseOJKPeriod("2026-03-18"); err != nil || !got.Equal(time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("YYYY-MM-DD: got %s err %v", got, err)
	}
	if _, err := parseOJKPeriod("2026/03"); err == nil {
		t.Fatal("format tidak valid harus ditolak")
	}

	got, err := parseOJKPeriod("")
	if err != nil {
		t.Fatalf("default tanpa period error: %v", err)
	}
	prev := time.Now().UTC().AddDate(0, -1, 0)
	if got.Day() != 1 || got.Month() != prev.Month() || got.Year() != prev.Year() {
		t.Fatalf("default period = %s, ingin awal bulan lalu", got)
	}
}

func newOJKHandler() *OJKReportHandler {
	return &OJKReportHandler{
		builder: ojkreport.NewBuilder(stubOJKSource{}),
		config: ojkCKPNConfigStub{values: map[string]string{
			domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusFinal,
		}},
	}
}

// newOJKHandlerSementara menyusun handler dengan parameter CKPN SEMENTARA: ekspor
// laporan OJK wajib ditolak.
func newOJKHandlerSementara() *OJKReportHandler {
	return ojKHandlerWithConfig(map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
		domain.ConfigKeyCKPNEnabled:          "true",
	})
}

// ojKHandlerWithConfig memakai stub konfigurasi yang benar-benar membaca peta, supaya
// saklar seperti ckpn.enabled ikut diuji (blokir ekspor OJK bergantung padanya).
func ojKHandlerWithConfig(values map[string]string) *OJKReportHandler {
	return &OJKReportHandler{
		builder: ojkreport.NewBuilder(stubOJKSource{}),
		config:  ojkCKPNConfigStub{values: values},
	}
}

// stubMappingReviewRepo menyimpan keputusan di memori untuk uji handler; idempotensi
// penyimpanan sungguhan diuji terpisah lewat integrasi basis data.
type stubMappingReviewRepo struct {
	saved []ojkreport.MappingReview
}

func (s *stubMappingReviewRepo) ListReviews(_ context.Context) ([]ojkreport.MappingReview, error) {
	return s.saved, nil
}

func (s *stubMappingReviewRepo) UpsertReview(_ context.Context, review ojkreport.MappingReview) error {
	s.saved = append(s.saved, review)
	return nil
}

func newOJKReviewHandler(repo ojkreport.MappingReviewRepository) *OJKReportHandler {
	return &OJKReportHandler{
		builder: ojkreport.NewBuilder(stubOJKSource{}),
		source:  stubOJKSource{},
		reviews: repo,
	}
}

func ojkRequestWithClaims(method, target, body string, claims *domain.JWTClaims) *http.Request {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	return req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
}

// Bank hanya boleh menandai baris yang benar-benar ada pada pemetaan draf dan memakai
// keputusan yang dikenal. Pengiriman sah harus menyimpan siapa dan kapan memutuskan.
func TestDecideMappingMenyimpanKeputusanSah(t *testing.T) {
	repo := &stubMappingReviewRepo{}
	h := newOJKReviewHandler(repo)
	before := time.Now().UTC()

	req := ojkRequestWithClaims(http.MethodPost, "/api/v1/reports/ojk/mapping/decision",
		`{"form":"01.00","coa_code":"10100","decision":"DISETUJUI","note":"sesuai pedoman"}`,
		&domain.JWTClaims{Username: "admin.uji", Role: domain.RoleAdmin})
	rec := httptest.NewRecorder()

	h.DecideMapping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.saved) != 1 {
		t.Fatalf("keputusan tersimpan = %d, ingin 1", len(repo.saved))
	}
	saved := repo.saved[0]
	if saved.DecidedBy != "admin.uji" {
		t.Errorf("decided_by = %q, ingin admin.uji", saved.DecidedBy)
	}
	if saved.DecidedAt.Before(before) {
		t.Errorf("decided_at %s lebih awal dari permintaan %s", saved.DecidedAt, before)
	}
	if saved.Decision != ojkreport.ReviewApproved || saved.Note != "sesuai pedoman" {
		t.Errorf("keputusan = %+v", saved)
	}
}

func TestDecideMappingMenolakMasukanTidakSah(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"keputusan asing", `{"form":"01.00","coa_code":"10100","decision":"MUNGKIN"}`},
		{"baris tidak dikenal", `{"form":"01.00","coa_code":"99999","decision":"DISETUJUI"}`},
		{"form tidak dikenal", `{"form":"09.99","coa_code":"10100","decision":"DISETUJUI"}`},
		{"catatan keberatan kosong", `{"form":"01.00","coa_code":"10100","decision":"DICATAT","note":"  "}`},
		{"body bukan JSON", `bukan-json`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubMappingReviewRepo{}
			h := newOJKReviewHandler(repo)
			req := ojkRequestWithClaims(http.MethodPost, "/api/v1/reports/ojk/mapping/decision",
				tc.body, &domain.JWTClaims{Username: "admin.uji", Role: domain.RoleAdmin})
			rec := httptest.NewRecorder()

			h.DecideMapping(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, ingin 400; body=%s", rec.Code, rec.Body.String())
			}
			if len(repo.saved) != 0 {
				t.Fatalf("keputusan tidak sah tidak boleh tersimpan: %+v", repo.saved)
			}
		})
	}
}

// stubCOALister mengembalikan bagan akun tetap untuk menguji pelengkapan nama akun.
type stubCOALister struct {
	list []domain.ChartOfAccount
}

func (s stubCOALister) GetCOAList(_ context.Context, _ domain.Actor) ([]domain.ChartOfAccount, error) {
	return s.list, nil
}

// Mapping harus menyajikan nama akun dan daftar celah tanpa menyalakan apa pun:
// status tetap DRAF.
func TestMappingMenyajikanNamaAkunDanCelah(t *testing.T) {
	h := &OJKReportHandler{
		builder: ojkreport.NewBuilder(stubOJKSource{}),
		source:  stubOJKSource{},
		coa:     stubCOALister{list: []domain.ChartOfAccount{{Code: "10100", Name: "Kas"}}},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/mapping?period=2026-03", nil)
	rec := httptest.NewRecorder()

	h.Mapping(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"mapping_status":"`+ojkreport.MappingStatus+`"`) {
		t.Fatalf("status pemetaan tidak draf: %s", body)
	}
	if !strings.Contains(body, `"coa_name":"Kas"`) {
		t.Fatalf("nama akun COA tidak disajikan: %s", body)
	}
	if !strings.Contains(body, `"pos_name":"Kas dalam Rupiah"`) {
		t.Fatalf("nama pos OJK tidak disajikan: %s", body)
	}
	if !strings.Contains(body, `"unmapped_positions"`) {
		t.Fatalf("daftar pos tanpa sumber tidak disajikan: %s", body)
	}
}

// Aktor yang hanya berwenang atas cabangnya tidak boleh mengekspor laporan
// bank-wide; permintaan harus ditolak 403.
func TestExportMonthlyMenolakAktorNonLintasCabang(t *testing.T) {
	h := newOJKHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/monthly?period=2026-03", nil)
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims,
		&domain.JWTClaims{Role: domain.RoleTeller, BranchCode: "001"}))
	rec := httptest.NewRecorder()

	h.ExportMonthly(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, ingin 403", rec.Code)
	}
}

// Aktor lintas cabang boleh mengekspor; keluaran adalah berkas teks lampiran.
func TestExportMonthlyMengizinkanAktorLintasCabang(t *testing.T) {
	h := newOJKHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/monthly?period=2026-03", nil)
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims,
		&domain.JWTClaims{Role: domain.RoleSuperAdmin}))
	rec := httptest.NewRecorder()

	h.ExportMonthly(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Fatalf("content-type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "ojk-bulanan-2026-03.txt") {
		t.Fatalf("content-disposition = %q", cd)
	}
	if !strings.Contains(rec.Body.String(), "FORM|SANDI|NAMA POS|JUMLAH") {
		t.Fatal("berkas teks tidak memuat header kolom")
	}
}

// Selama parameter CKPN SEMENTARA, berkas laporan OJK TIDAK boleh dibangun: jumlahnya
// berasal dari parameter yang belum diratifikasi dan dilarang menjadi dasar laporan
// OJK/APOLO. Bentuknya MENOLAK KERAS (422) — bukan menandai berkas, karena berkas
// bertanda masih dapat terkirim. Pesannya wajib menyebut sebab dan langkah perbaikan.
func TestExportMonthlyMenolakSaatParameterCKPNSementara(t *testing.T) {
	h := newOJKHandlerSementara()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/monthly?period=2026-03", nil)
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims,
		&domain.JWTClaims{Role: domain.RoleSuperAdmin}))
	rec := httptest.NewRecorder()

	h.ExportMonthly(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, ingin 422 (ekspor ditolak selama SEMENTARA); body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"SEMENTARA", "FINAL", "ratifikasi"} {
		if !strings.Contains(body, want) {
			t.Fatalf("pesan penolakan harus memuat %q: %s", want, body)
		}
	}
	if strings.Contains(body, "FORM|SANDI|NAMA POS") {
		t.Fatal("berkas laporan tidak boleh terbentuk saat parameter SEMENTARA")
	}
}

// Selama CKPN resmi MASIH MATI, laporan OJK memuat angka PPKA (bukan CKPN dari PD/LGD
// sementara), sehingga menutup laporannya justru memblokir hal yang sehat. Blokir hanya
// berlaku saat angka sementara benar-benar mengalir ke laporan.
func TestExportMonthlyTetapJalanSaatParameterSementaraDanCKPNMati(t *testing.T) {
	h := ojKHandlerWithConfig(map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
		domain.ConfigKeyCKPNEnabled:          "false",
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports/ojk/monthly?period=2026-03", nil)
	req = req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims,
		&domain.JWTClaims{Role: domain.RoleSuperAdmin}))
	rec := httptest.NewRecorder()

	h.ExportMonthly(rec, req)

	if rec.Code == http.StatusUnprocessableEntity {
		t.Fatalf("CKPN mati tidak boleh menutup ekspor OJK; body=%s", rec.Body.String())
	}
}
