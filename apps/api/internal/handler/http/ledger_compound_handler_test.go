package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// compoundLedgerStub hanya menyediakan PostCompoundJournal. Metode LedgerService
// lain dibiarkan lewat embedded interface karena rute yang diuji tidak memanggilnya.
type compoundLedgerStub struct {
	domain.LedgerService
	lastReq domain.CustomJournalRequest
	err     error
}

func (s *compoundLedgerStub) PostCompoundJournal(_ context.Context, req domain.CustomJournalRequest) (*domain.JournalEntry, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return &domain.JournalEntry{ReferenceNumber: "JM-2026-0001", TransactionType: req.TransactionType}, nil
}

const compoundBody = `{"transaction_type":"ADJUSTMENT","description":"koreksi uji",` +
	`"lines":[{"account_number":"10101","direction":"DEBIT","amount":"1000"},` +
	`{"account_number":"40100","direction":"CREDIT","amount":"1000"}]}`

// newCompoundTestRouter merakit router produksi dengan handler jurnal yang di-stub.
// Handler lain cukup struct kosong: rutenya tidak dipanggil oleh uji ini, tetapi tidak
// boleh nil karena router mendaftarkannya saat dibangun.
func newCompoundTestRouter(role domain.StaffRole, svc domain.LedgerService) http.Handler {
	return NewRouter(RouterParams{
		CustomerHandler:     &CustomerHandler{},
		AccountHandler:      &AccountHandler{},
		BranchHandler:       &BranchHandler{},
		ProductHandler:      &ProductHandler{},
		LedgerHandler:       NewLedgerHandler(svc),
		AuthHandler:         &AuthHandler{},
		StaffHandler:        &StaffHandler{},
		LoanHandler:         &LoanHandler{},
		MakerCheckerHandler: &MakerCheckerHandler{},
		ReportHandler:       &ReportHandler{},
		CollectionHandler:   &CollectionHandler{},
		IntegrationHandler:  &IntegrationHandler{},
		BatchProcessHandler: &BatchProcessHandler{},
		DocumentHandler:     &DocumentHandler{},
		AuthService:         reportAuthStub{role: role},
	})
}

func compoundRouterRequest(t *testing.T, role domain.StaffRole, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := newCompoundTestRouter(role, &compoundLedgerStub{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/journals", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer uji")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// Jurnal majemuk menulis langsung ke buku besar, jadi peran cabang tanpa coa:manage
// harus ditolak meski lolos autentikasi.
func TestRuteJurnalMajemukTolakPeranCabang(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleAO, domain.RoleTeller, domain.RoleCS, domain.RoleSupervisor} {
		rec := compoundRouterRequest(t, role, compoundBody)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s pada POST /transactions/journals = %d, ingin 403 (tanpa coa:manage)", role, rec.Code)
		}
	}
}

// Pemegang coa:manage (ADMIN/SUPERADMIN) mencapai handler dan jurnal diterima 201.
func TestRuteJurnalMajemukIzinkanAdmin(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleAdmin, domain.RoleSuperAdmin} {
		rec := compoundRouterRequest(t, role, compoundBody)
		if rec.Code != http.StatusCreated {
			t.Fatalf("%s = %d, ingin 201 (%s)", role, rec.Code, rec.Body.String())
		}
	}
}

func compoundRequestWithClaims(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/journals", strings.NewReader(body))
	claims := &domain.JWTClaims{UserID: uuid.New(), Username: "admin01", Role: domain.RoleAdmin, BranchCode: "001"}
	return req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
}

// Kurang dari dua baris ditolak 400 dengan pesan katalog, tanpa memanggil service.
func TestPostCompoundJournalKurangDuaBaris(t *testing.T) {
	stub := &compoundLedgerStub{}
	handler := NewLedgerHandler(stub)

	rec := httptest.NewRecorder()
	handler.PostCompoundJournal(rec, compoundRequestWithClaims(`{"lines":[{"account_number":"10101"}]}`))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, ingin 400 (%s)", rec.Code, rec.Body.String())
	}
	if stub.lastReq.Lines != nil {
		t.Fatal("service tidak boleh dipanggil saat baris kurang dari dua")
	}
}

// Identitas pencatat diambil dari JWT, bukan dari body, dan kunci idempotensi dari
// header diteruskan ke service.
func TestPostCompoundJournalIdentitasDariJWT(t *testing.T) {
	stub := &compoundLedgerStub{}
	handler := NewLedgerHandler(stub)

	req := compoundRequestWithClaims(compoundBody)
	req.Header.Set("Idempotency-Key", "IDEM-JM-1")
	rec := httptest.NewRecorder()
	handler.PostCompoundJournal(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, ingin 201 (%s)", rec.Code, rec.Body.String())
	}
	if stub.lastReq.CreatedBy != "admin01" {
		t.Fatalf("created_by = %q, ingin identitas JWT admin01", stub.lastReq.CreatedBy)
	}
	if stub.lastReq.IdempotencyKey != "IDEM-JM-1" {
		t.Fatalf("idempotency_key = %q, ingin dari header", stub.lastReq.IdempotencyKey)
	}
	if len(stub.lastReq.Lines) != 2 {
		t.Fatalf("baris diteruskan = %d, ingin 2", len(stub.lastReq.Lines))
	}
}
