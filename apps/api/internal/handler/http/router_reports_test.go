package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// reportSvcStub mengembalikan laporan kosong; cukup untuk membuktikan rute
// mencapainya (200) tanpa menyentuh database.
type reportSvcStub struct{}

func (reportSvcStub) GenerateTrialBalance(context.Context) (*domain.TrialBalanceReport, error) {
	return &domain.TrialBalanceReport{}, nil
}
func (reportSvcStub) GenerateBalanceSheet(context.Context, time.Time) (*domain.BalanceSheetReport, error) {
	return &domain.BalanceSheetReport{}, nil
}
func (reportSvcStub) GenerateIncomeStatement(context.Context, time.Time, time.Time) (*domain.IncomeStatementReport, error) {
	return &domain.IncomeStatementReport{}, nil
}
func (reportSvcStub) GetTrialBalance(context.Context, time.Time, time.Time, string) ([]domain.TrialBalanceRow, error) {
	return nil, nil
}
func (reportSvcStub) GetIncomeStatement(context.Context, time.Time, time.Time, string) (*domain.IncomeStatement, error) {
	return &domain.IncomeStatement{}, nil
}
func (reportSvcStub) GetBalanceSheet(context.Context, time.Time, string) (*domain.BalanceSheet, error) {
	return &domain.BalanceSheet{}, nil
}
func (reportSvcStub) GetCashFlow(context.Context, time.Time, time.Time, string) (*domain.CashFlow, error) {
	return &domain.CashFlow{}, nil
}
func (reportSvcStub) ListDueObligations(context.Context, time.Time, int, domain.Actor) ([]domain.DueObligation, error) {
	return nil, nil
}

// reportAuthStub menerima token apa pun dan mengembalikan klaim dengan peran tetap,
// supaya rute dapat diuji tanpa sesi nyata.
type reportAuthStub struct{ role domain.StaffRole }

func (reportAuthStub) Login(context.Context, domain.LoginInput) (*domain.LoginResponse, error) {
	return nil, nil
}
func (reportAuthStub) Refresh(context.Context, string) (*domain.LoginResponse, error) {
	return nil, nil
}
func (reportAuthStub) Logout(context.Context, uuid.UUID) error { return nil }
func (a reportAuthStub) ValidateAccessToken(context.Context, string) (*domain.JWTClaims, error) {
	return &domain.JWTClaims{Role: a.role}, nil
}

// newReportTestRouter merakit router produksi lengkap. Handler selain laporan cukup
// struct kosong: rute laporan tidak memanggilnya, sedangkan nilainya tidak boleh nil
// karena router mendaftarkan seluruh rute saat dibangun.
func newReportTestRouter(role domain.StaffRole) http.Handler {
	return NewRouter(RouterParams{
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
		AuthService:         reportAuthStub{role: role},
	})
}

func reportRequest(t *testing.T, router http.Handler, path string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer uji")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

// AO adalah peran cabang: keempat laporan keuangan bank-wide harus ditolak, tetapi
// laporan operasional (jatuh tempo) dan mutasi rekening tetap boleh.
func TestRuteLaporanKeuanganTolakAO(t *testing.T) {
	router := newReportTestRouter(domain.RoleAO)
	for _, path := range []string{
		"/api/v1/reports/trial-balance",
		"/api/v1/reports/balance-sheet",
		"/api/v1/reports/income-statement",
		"/api/v1/reports/cash-flow",
	} {
		if code := reportRequest(t, router, path); code != http.StatusForbidden {
			t.Fatalf("AO pada %s = %d, ingin 403 (tanpa reports:financial:read)", path, code)
		}
	}
	if code := reportRequest(t, router, "/api/v1/reports/due-obligations"); code != http.StatusOK {
		t.Fatalf("AO pada due-obligations = %d, ingin 200 (tetap memakai ledger:read)", code)
	}
}

// Pengawas yang berkepentingan tetap dapat membaca keempat laporan keuangan.
func TestRuteLaporanKeuanganIzinkanPengawas(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin, domain.RoleSupervisor, domain.RoleAuditor} {
		t.Run(string(role), func(t *testing.T) {
			router := newReportTestRouter(role)
			for _, path := range []string{
				"/api/v1/reports/trial-balance",
				"/api/v1/reports/balance-sheet",
				"/api/v1/reports/income-statement",
				"/api/v1/reports/cash-flow",
			} {
				if code := reportRequest(t, router, path); code != http.StatusOK {
					t.Fatalf("%s pada %s = %d, ingin 200", role, path, code)
				}
			}
		})
	}
}

// Peran cabang lain (TELLER/CS) juga tidak boleh membaca laporan keuangan bank-wide.
func TestRuteLaporanKeuanganTolakPeranCabangLain(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleTeller, domain.RoleCS} {
		router := newReportTestRouter(role)
		if code := reportRequest(t, router, "/api/v1/reports/balance-sheet"); code != http.StatusForbidden {
			t.Fatalf("%s pada balance-sheet = %d, ingin 403", role, code)
		}
	}
}
