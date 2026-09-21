package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"github.com/shopspring/decimal"
)

// stubLimitSvc menggantikan penjaga batas: test hanya perlu List untuk memeriksa
// bentuk respons endpoint.
type stubLimitSvc struct {
	views []domain.TransactionLimitView
	err   error
}

func (s *stubLimitSvc) ForActor(context.Context, domain.Actor, string) (domain.TransactionLimit, error) {
	return domain.TransactionLimit{}, nil
}

func (s *stubLimitSvc) Check(context.Context, domain.Actor, string, decimal.Decimal) error {
	return nil
}

func (s *stubLimitSvc) List(context.Context) ([]domain.TransactionLimitView, error) {
	return s.views, s.err
}

// TestListTransactionLimitsShape mengunci bentuk respons GET /system/limits: source
// "config" dan nilai uang sebagai STRING desimal (bukan angka JSON), sesuai kontrak
// yang dipakai antarmuka.
func TestListTransactionLimitsShape(t *testing.T) {
	svc := &stubLimitSvc{views: []domain.TransactionLimitView{{
		Role:            domain.RoleTeller,
		TransactionType: "DEPOSIT",
		PerTransaction:  decimal.NewFromInt(50_000_000),
		DailyLimit:      decimal.NewFromInt(500_000_000),
		ApprovalAbove:   decimal.NewFromInt(100_000_000),
		Configured:      true,
	}}}
	handler := httpHandler.NewAuditHandler(&stubAuditReader{}, svc)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/system/limits", nil)
	handler.ListTransactionLimits(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}

	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Source string `json:"source"`
			Limits []struct {
				Role            string          `json:"role"`
				TransactionType string          `json:"transaction_type"`
				PerTransaction  json.RawMessage `json:"per_transaction"`
				DailyLimit      json.RawMessage `json:"daily_limit"`
				ApprovalAbove   json.RawMessage `json:"approval_above"`
				Configured      bool            `json:"configured"`
			} `json:"limits"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON valid: %v (%s)", err, rec.Body.String())
	}
	if !body.Success || body.Data.Source != "config" {
		t.Fatalf("envelope salah: success=%v source=%q", body.Success, body.Data.Source)
	}
	if len(body.Data.Limits) != 1 {
		t.Fatalf("jumlah limit %d, mau 1", len(body.Data.Limits))
	}
	row := body.Data.Limits[0]
	if row.Role != "TELLER" || row.TransactionType != "DEPOSIT" || !row.Configured {
		t.Fatalf("baris salah: %+v", row)
	}
	// Nilai uang HARUS string agar presisi desimal tidak hilang.
	if string(row.PerTransaction) != `"50000000"` {
		t.Fatalf("per_transaction = %s, mau \"50000000\" (string)", row.PerTransaction)
	}
	if string(row.DailyLimit) != `"500000000"` {
		t.Fatalf("daily_limit = %s, mau \"500000000\" (string)", row.DailyLimit)
	}
	if string(row.ApprovalAbove) != `"100000000"` {
		t.Fatalf("approval_above = %s, mau \"100000000\" (string)", row.ApprovalAbove)
	}
}
