package http_test

import (
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"github.com/go-chi/chi/v5"
)

// openapiGuardCKPNIndividualStub dan openapiGuardPPKAUmumStub menutup antarmuka
// opsional dengan menanamkannya; hanya dibutuhkan agar rute kondisional benar-benar
// terpasang saat router dibangun, bukan untuk menjalankan handler.
type openapiGuardCKPNIndividualStub struct{ domain.CKPNIndividualService }

type openapiGuardPPKAUmumStub struct{ domain.PPKAUmumService }

// openapiSpecPath menunjuk ke docs/openapi/openapi.yaml relatif terhadap direktori
// paket. go test menjalankan uji dengan working directory di paket
// (apps/api/internal/handler/http), sehingga lima tingkat ke atas sampai ke akar repo.
func openapiSpecPath() string {
	return filepath.Join("..", "..", "..", "..", "..", "docs", "openapi", "openapi.yaml")
}

// parseOpenAPIRoutes membaca blok paths: dari spesifikasi tanpa dependensi YAML.
// Yang diambil hanya kunci path ber-indentasi dua spasi dan kunci metode di bawahnya,
// sehingga cukup untuk membandingkan daftar rute. Format file dijaga seragam oleh
// spesifikasi ini (path selalu di akar blok paths, metode satu tingkat di bawahnya).
func parseOpenAPIRoutes(t *testing.T, specPath string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("baca spesifikasi %s: %v", specPath, err)
	}

	routes := make(map[string]bool)
	inPaths := false
	currentPath := ""
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimRight(line, " \t\r")

		switch {
		case line == "paths:":
			inPaths = true
			continue
		case !inPaths:
			continue
		}

		// Kunci top-level (mis. "components:") menandai akhir blok paths.
		if line != "" && !strings.HasPrefix(line, " ") {
			inPaths = false
			continue
		}

		if strings.HasPrefix(line, "  /") && strings.HasSuffix(line, ":") {
			currentPath = strings.TrimSuffix(strings.TrimPrefix(line, "  "), ":")
			continue
		}

		if currentPath == "" {
			continue
		}
		trimmed := strings.TrimPrefix(line, "    ")
		if trimmed == line {
			continue
		}
		if !strings.HasSuffix(trimmed, ":") {
			continue
		}
		method := strings.TrimSuffix(trimmed, ":")
		switch method {
		case "get", "post", "put", "delete", "patch", "head", "options":
			routes[strings.ToUpper(method)+" "+currentPath] = true
		}
	}
	if len(routes) == 0 {
		t.Fatalf("tidak ada rute terbaca dari %s", specPath)
	}
	return routes
}

// routerRoutes membangun router produksi dengan seluruh handler terisi lalu
// mengumpulkan rute terdaftar lewat chi.Walk. Rute kondisional (mis. CKPN individual
// dan monitoring) harus ikut, jadi handler opsionalnya diisi stub.
func routerRoutes(t *testing.T) map[string]bool {
	t.Helper()
	r := httpHandler.NewRouter(httpHandler.RouterParams{
		CustomerHandler:         &httpHandler.CustomerHandler{},
		AccountHandler:          &httpHandler.AccountHandler{},
		BranchHandler:           &httpHandler.BranchHandler{},
		ProductHandler:          &httpHandler.ProductHandler{},
		LedgerHandler:           &httpHandler.LedgerHandler{},
		AuthHandler:             &httpHandler.AuthHandler{},
		StaffHandler:            &httpHandler.StaffHandler{},
		LoanHandler:             &httpHandler.LoanHandler{},
		MakerCheckerHandler:     &httpHandler.MakerCheckerHandler{},
		ReportHandler:           &httpHandler.ReportHandler{},
		CollectionHandler:       &httpHandler.CollectionHandler{},
		IntegrationHandler:      &httpHandler.IntegrationHandler{},
		OJKReportHandler:        &httpHandler.OJKReportHandler{},
		KPMMHandler:             &httpHandler.KPMMHandler{},
		BatchProcessHandler:     &httpHandler.BatchProcessHandler{},
		EODDefinitionHandler:    &httpHandler.EODDefinitionHandler{},
		DocumentHandler:         &httpHandler.DocumentHandler{},
		DepositHandler:          &httpHandler.DepositHandler{},
		PPAPHandler:             httpHandler.NewPPAPHandler(nil, openapiGuardPPKAUmumStub{}),
		CKPNHandler:             &httpHandler.CKPNHandler{Individual: openapiGuardCKPNIndividualStub{}},
		LPSPlacementHandler:     &httpHandler.LPSPlacementHandler{},
		AuditHandler:            &httpHandler.AuditHandler{},
		CollateralHandler:       &httpHandler.CollateralHandler{},
		CollateralWeightHandler: &httpHandler.CollateralWeightHandler{},
		AppInfoHandler:          &httpHandler.AppInfoHandler{},
		BankProfileHandler:      &httpHandler.BankProfileHandler{},
		OJKProfileHandler:       &httpHandler.OJKProfileHandler{},
		CKPNActivationHandler:   &httpHandler.CKPNActivationHandler{},
		PermissionHandler:       &httpHandler.PermissionHandler{},
		MonitoringHandler:       &httpHandler.MonitoringHandler{},
	})

	routes := make(map[string]bool)
	if err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		routes[method+" "+route] = true
		return nil
	}); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	return routes
}

// TestOpenAPIMencakupSemuaRute menjamin kontrak OpenAPI tidak basi: setiap rute yang
// benar-benar terdaftar di NewRouter wajib punya entri metode+path di openapi.yaml,
// dan sebaliknya tidak boleh ada rute di spesifikasi yang sudah tidak terdaftar.
// Menambah rute baru tanpa memperbarui dokumen akan menggagalkan uji ini.
func TestOpenAPIMencakupSemuaRute(t *testing.T) {
	specRoutes := parseOpenAPIRoutes(t, openapiSpecPath())
	actualRoutes := routerRoutes(t)

	var belumTerdokumentasi, sudahTidakAdaDiRouter []string
	for route := range actualRoutes {
		if !specRoutes[route] {
			belumTerdokumentasi = append(belumTerdokumentasi, route)
		}
	}
	for route := range specRoutes {
		if !actualRoutes[route] {
			sudahTidakAdaDiRouter = append(sudahTidakAdaDiRouter, route)
		}
	}

	sort.Strings(belumTerdokumentasi)
	sort.Strings(sudahTidakAdaDiRouter)

	if len(belumTerdokumentasi) > 0 {
		t.Errorf("rute terdaftar tanpa dokumentasi di %s:\n  %s",
			openapiSpecPath(), strings.Join(belumTerdokumentasi, "\n  "))
	}
	if len(sudahTidakAdaDiRouter) > 0 {
		t.Errorf("spesifikasi memuat rute yang tidak ada di router (basi):\n  %s",
			strings.Join(sudahTidakAdaDiRouter, "\n  "))
	}
}
