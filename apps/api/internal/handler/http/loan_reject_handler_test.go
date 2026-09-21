package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// w2RejectLoanSvc hanya melayani RejectLoan; metode lain diambil dari antarmuka
// sehingga test handler tidak perlu menyiapkan seluruh layanan kredit.
type w2RejectLoanSvc struct {
	domain.LoanService
	gotReason string
}

func (s *w2RejectLoanSvc) RejectLoan(_ context.Context, id uuid.UUID, reason string, _ domain.Actor) (*domain.Loan, error) {
	s.gotReason = reason
	if strings.TrimSpace(reason) == "" {
		return nil, domain.ErrLoanRejectionReasonRequired
	}
	return &domain.Loan{ID: id, Status: domain.LoanStatusRejected, RejectionReason: reason}, nil
}

func w2LoanRejectRequest(id uuid.UUID, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/loans/"+id.String()+"/reject", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", id.String())
	ctx := context.WithValue(req.Context(), chi.RouteCtxKey, rctx)
	ctx = context.WithValue(ctx, domain.ContextKeyClaims, &domain.JWTClaims{
		Username: "spv.uji", Role: domain.RoleSuperAdmin, BranchCode: "001",
	})
	return req.WithContext(ctx)
}

// Body permintaan penolakan harus benar-benar dibaca; sebelumnya alasan diabaikan.
func TestRejectHandler_MembacaAlasanDariBody(t *testing.T) {
	svc := &w2RejectLoanSvc{}
	h := &LoanHandler{loanSvc: svc}
	id := uuid.New()
	rec := httptest.NewRecorder()

	h.Reject(rec, w2LoanRejectRequest(id, `{"reason":"agunan tidak memenuhi syarat"}`))

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, ingin 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.gotReason != "agunan tidak memenuhi syarat" {
		t.Fatalf("alasan yang diteruskan ke service %q", svc.gotReason)
	}
}

// Body kosong bukan galat parsing: alasan kosong diteruskan agar service menolaknya
// dengan pesan domain yang jelas (422), bukan 400 "invalid request body".
func TestRejectHandler_BodyKosongDiteruskanSebagaiAlasanKosong(t *testing.T) {
	svc := &w2RejectLoanSvc{}
	h := &LoanHandler{loanSvc: svc}
	rec := httptest.NewRecorder()

	h.Reject(rec, w2LoanRejectRequest(uuid.New(), ""))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d, ingin 422 (body: %s)", rec.Code, rec.Body.String())
	}
	if svc.gotReason != "" {
		t.Fatalf("alasan body kosong diteruskan sebagai %q", svc.gotReason)
	}
}
