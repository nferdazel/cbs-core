package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"cbs-core/apps/core-api/internal/monitoring"
)

type stubMonitoringService struct {
	snapshot monitoring.Snapshot
}

func (s stubMonitoringService) Collect(context.Context) monitoring.Snapshot { return s.snapshot }

func TestMonitoringHandlerGet(t *testing.T) {
	svc := stubMonitoringService{snapshot: monitoring.Snapshot{
		Findings: []monitoring.Finding{
			{Code: "business_date_lagging", Severity: monitoring.SeverityCritical, Message: "tertinggal"},
		},
		Summary: monitoring.Summary{Critical: 1},
	}}
	h := NewMonitoringHandler(svc)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/system/monitoring", nil)
	h.Get(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, ingin %d", rec.Code, http.StatusOK)
	}

	var payload struct {
		Success bool                `json:"success"`
		Data    monitoring.Snapshot `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("badan respons bukan JSON: %v", err)
	}
	if !payload.Success {
		t.Fatalf("success = false, badan: %s", rec.Body.String())
	}
	if len(payload.Data.Findings) != 1 || payload.Data.Findings[0].Code != "business_date_lagging" {
		t.Fatalf("data temuan tidak diteruskan apa adanya: %+v", payload.Data)
	}
	if payload.Data.Summary.Critical != 1 {
		t.Fatalf("ringkasan tidak diteruskan: %+v", payload.Data.Summary)
	}
}
