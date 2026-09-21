package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
)

// stubAuditReader menangkap filter yang diterima repositori sekaligus mengembalikan
// sejumlah baris agar pemetaan respons dapat diperiksa.
type stubAuditReader struct {
	received domain.AuditLogFilter
	events   []domain.AuditEvent
	err      error
}

func (s *stubAuditReader) Query(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditEvent, error) {
	s.received = filter
	if s.err != nil {
		return nil, s.err
	}
	return s.events, nil
}

type auditListResponse struct {
	Success bool                `json:"success"`
	Data    []domain.AuditEvent `json:"data"`
	Meta    struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
		Count  int `json:"count"`
	} `json:"meta"`
}

func TestAuditHandler_ListMeneruskanFilter(t *testing.T) {
	reader := &stubAuditReader{events: []domain.AuditEvent{{
		ActorID: "teller1", Action: "PAY_INSTALLMENT", ResourceType: "LOAN", ResourceID: "L-1",
	}}}
	handler := httpHandler.NewAuditHandler(reader, nil)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet,
		"/api/v1/audit-logs?resource_type=LOAN&actor=teller1&action=PAY_INSTALLMENT&from=2026-09-01&to=2026-09-20&limit=10&offset=20", nil)

	handler.List(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}

	got := reader.received
	if got.ResourceType != "LOAN" || got.Actor != "teller1" || got.Action != "PAY_INSTALLMENT" {
		t.Fatalf("filter tidak diteruskan: %+v", got)
	}
	if got.Limit != 10 || got.Offset != 20 {
		t.Fatalf("limit/offset %d/%d, mau 10/20", got.Limit, got.Offset)
	}
	if got.From == nil || !got.From.Equal(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("from = %v, mau 2026-09-01T00:00:00Z", got.From)
	}
	// to=2026-09-20 berarti sampai dengan hari itu, sehingga batas atasnya 21 September
	// pukul 00:00 (eksklusif). Tanpa ini seluruh aksi pada tanggal 20 ikut terbuang.
	if got.To == nil || !got.To.Equal(time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("to = %v, mau 2026-09-21T00:00:00Z", got.To)
	}

	var body auditListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON valid: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ResourceID != "L-1" {
		t.Fatalf("data audit tidak sesuai: %+v", body.Data)
	}
	if body.Meta.Limit != 10 || body.Meta.Offset != 20 || body.Meta.Count != 1 {
		t.Fatalf("meta tidak sesuai: %+v", body.Meta)
	}
}

// Alasan penolakan kredit harus terbaca pembaca lama lewat metadata, di samping
// changes yang tetap dikirim. Bentuk respons tidak berubah, hanya bertambah field.
func TestAuditHandler_ListMengirimMetadataAlasan(t *testing.T) {
	reader := &stubAuditReader{events: []domain.AuditEvent{{
		ActorID: "spv1", Action: "REJECT_LOAN", ResourceType: "loan", ResourceID: "L-2",
		Changes:  map[string]any{"reason": "agunan tidak memenuhi syarat"},
		Metadata: map[string]any{"reason": "agunan tidak memenuhi syarat"},
	}}}
	handler := httpHandler.NewAuditHandler(reader, nil)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?resource_type=loan&action=REJECT_LOAN", nil)

	handler.List(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}
	var body auditListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON valid: %v", err)
	}
	if len(body.Data) != 1 {
		t.Fatalf("data audit %d baris, mau 1", len(body.Data))
	}
	if got, _ := body.Data[0].Metadata["reason"].(string); got != "agunan tidak memenuhi syarat" {
		t.Fatalf("metadata.reason %q, ingin alasan penolakan", got)
	}
}

func TestAuditHandler_ListMembatasiPermintaanBerlebihan(t *testing.T) {
	reader := &stubAuditReader{}
	handler := httpHandler.NewAuditHandler(reader, nil)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?limit=99999&offset=-5", nil)

	handler.List(rec, r)

	if reader.received.Limit != 200 {
		t.Fatalf("limit %d, mau dibatasi 200", reader.received.Limit)
	}
	if reader.received.Offset != 0 {
		t.Fatalf("offset %d, mau 0", reader.received.Offset)
	}
	if reader.received.From != nil || reader.received.To != nil {
		t.Fatalf("filter waktu kosong harus berarti tanpa batas: %+v", reader.received)
	}
}

func TestAuditHandler_ListMenolakWaktuDanRentangTidakSah(t *testing.T) {
	handler := httpHandler.NewAuditHandler(&stubAuditReader{}, nil)

	cases := []struct {
		name  string
		query string
	}{
		{"format tidak dikenal", "from=20-09-2026"},
		{"rentang terbalik", "from=2026-09-20&to=2026-09-01"},
		{"rentang kosong", "from=2026-09-20T00:00:00Z&to=2026-09-20T00:00:00Z"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?"+tc.query, nil)

			handler.List(rec, r)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, mau 400 (%s)", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestAuditHandler_ListMenerimaBatasWaktuRFC3339(t *testing.T) {
	reader := &stubAuditReader{}
	handler := httpHandler.NewAuditHandler(reader, nil)

	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/audit-logs?from=2026-09-20T07:30:00Z", nil)

	handler.List(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200 (%s)", rec.Code, rec.Body.String())
	}
	if reader.received.From == nil || !reader.received.From.Equal(time.Date(2026, 9, 20, 7, 30, 0, 0, time.UTC)) {
		t.Fatalf("from = %v, mau 2026-09-20T07:30:00Z", reader.received.From)
	}
	if reader.received.Limit != 50 {
		t.Fatalf("limit bawaan %d, mau 50", reader.received.Limit)
	}
}
