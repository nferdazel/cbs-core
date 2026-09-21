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
	"github.com/google/uuid"
)

// stubDepositSvc menyediakan permukaan DepositService untuk menguji terjemahan HTTP.
type stubDepositSvc struct {
	placeErr error
}

func (s *stubDepositSvc) Place(context.Context, domain.PlaceDepositInput, domain.Actor) (*domain.Deposit, error) {
	if s.placeErr != nil {
		return nil, s.placeErr
	}
	return &domain.Deposit{ID: uuid.New()}, nil
}

func (s *stubDepositSvc) Preview(context.Context, domain.PlaceDepositInput, domain.Actor) (*domain.DepositPreview, error) {
	return nil, nil
}

func (s *stubDepositSvc) Accrue(context.Context, uuid.UUID, time.Time, domain.Actor) (*domain.Deposit, error) {
	return nil, nil
}

func (s *stubDepositSvc) MatureOrWithdraw(context.Context, uuid.UUID, domain.Actor) (*domain.Deposit, error) {
	return nil, nil
}

func (s *stubDepositSvc) RunARO(context.Context, time.Time, domain.Actor) (int, error) { return 0, nil }

func (s *stubDepositSvc) GetByID(context.Context, uuid.UUID, domain.Actor) (*domain.Deposit, error) {
	return nil, nil
}

func (s *stubDepositSvc) List(context.Context, int, int, domain.Actor) ([]domain.Deposit, int, error) {
	return nil, 0, nil
}

func (s *stubDepositSvc) ExecuteApproved(context.Context, any, string, map[string]any, domain.Actor) error {
	return nil
}

func placeRequestWithClaims(t *testing.T) *http.Request {
	t.Helper()
	body := `{"customer_id":"` + uuid.New().String() + `","product_id":"` + uuid.New().String() +
		`","placement_amount":"150000000","term_months":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/deposits/place", strings.NewReader(body))
	claims := &domain.JWTClaims{
		UserID:     uuid.New(),
		Username:   "teller01",
		Role:       domain.RoleTeller,
		BranchCode: "001",
	}
	return req.WithContext(context.WithValue(req.Context(), domain.ContextKeyClaims, claims))
}

// Penempatan di atas ambang persetujuan harus dibalas 202 PENDING_APPROVAL, bukan
// 422: ia diterima untuk direview, sama seperti transaksi setoran.
func TestDepositPlace_OverThresholdMapsTo202(t *testing.T) {
	pending := &domain.PendingApprovalError{RequestID: uuid.New(), ActionType: "PLACE_DEPOSIT"}
	handler := httpHandler.NewDepositHandler(&stubDepositSvc{placeErr: pending})

	rec := httptest.NewRecorder()
	handler.Place(rec, placeRequestWithClaims(t))

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, mau 202 (%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "PENDING_APPROVAL") || !strings.Contains(rec.Body.String(), pending.RequestID.String()) {
		t.Fatalf("badan respons tidak memuat status/id pengajuan: %s", rec.Body.String())
	}
}

// Penempatan yang tidak butuh persetujuan tetap dibalas 201.
func TestDepositPlace_WithinThresholdStays201(t *testing.T) {
	handler := httpHandler.NewDepositHandler(&stubDepositSvc{})

	rec := httptest.NewRecorder()
	handler.Place(rec, placeRequestWithClaims(t))

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, mau 201 (%s)", rec.Code, rec.Body.String())
	}
}
