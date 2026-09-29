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
)

// kelembagaanFor19Stub memenuhi kontrak laporan kelembagaan yang dipakai handler Form
// 00.19 lewat type assert.
type kelembagaanFor19Stub struct {
	ojkreport.RepoSource
	report domain.KelembagaanReport
	err    error
}

func (s kelembagaanFor19Stub) KelembagaanReport(_ context.Context, _ time.Time, _ domain.Actor) (domain.KelembagaanReport, error) {
	return s.report, s.err
}

// Form 00.19 disajikan sebagai dokumen HTML siap cetak, bukan JSON tabel.
func TestExportStrukturOrganisasiMenyajikanHTML(t *testing.T) {
	src := kelembagaanFor19Stub{report: domain.KelembagaanReport{
		Offices:    []domain.BankOffice{{Code: "001", Name: "KC Pusat", Status: "AKTIF"}},
		Management: []domain.BankManagement{{Category: "DIREKSI", Name: "Budi"}},
	}}
	h := &OJKReportHandler{source: src}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/struktur-organisasi?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rec := httptest.NewRecorder()

	h.ExportStrukturOrganisasi(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q, ingin text/html", ct)
	}
	body := rec.Body.String()
	for _, want := range []string{"STRUKTUR ORGANISASI BPR", "KC Pusat", "Budi", "Divisi atau Satuan Kerja"} {
		if !strings.Contains(body, want) {
			t.Errorf("dokumen tidak memuat %q", want)
		}
	}
}

// Tanpa sumber kelembagaan, dokumen tetap dibuat dengan peringatan (bukan gagal dan bukan
// diisi contoh).
func TestExportStrukturOrganisasiTanpaSumberTetapTerbit(t *testing.T) {
	h := &OJKReportHandler{source: ojkreport.RepoSource{}}
	req := ojkRequestWithClaims(http.MethodGet, "/api/v1/reports/ojk/struktur-organisasi?period=2026-03", "",
		&domain.JWTClaims{Role: domain.RoleAdmin})
	rec := httptest.NewRecorder()

	h.ExportStrukturOrganisasi(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin 200; body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "belum dikonfigurasi") {
		t.Fatal("dokumen tanpa sumber harus menyatakan sumber belum dikonfigurasi")
	}
}
