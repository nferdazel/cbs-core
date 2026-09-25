package monitoring

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func boolPtr(v bool) *bool { return &v }

func businessDate(date string, status domain.BusinessDateStatus) *domain.SystemBusinessDate {
	t, _ := time.Parse("2006-01-02", date)
	return &domain.SystemBusinessDate{CurrentDate: t, Status: status}
}

func TestEvaluateBusinessDate(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		bd         *domain.SystemBusinessDate
		readErr    error
		wantCodes  []string
		wantFirst  Severity
		noFindings bool
	}{
		{
			name:       "tanggal sama dengan kalender tidak ada temuan",
			bd:         businessDate("2026-09-25", domain.BusinessDateStatusOpen),
			noFindings: true,
		},
		{
			name:      "tertinggal satu hari diperingatkan",
			bd:        businessDate("2026-09-24", domain.BusinessDateStatusOpen),
			wantCodes: []string{codeBusinessDateLagging},
			wantFirst: SeverityWarning,
		},
		{
			name:      "tertinggal lima hari kritis",
			bd:        businessDate("2026-09-20", domain.BusinessDateStatusOpen),
			wantCodes: []string{codeBusinessDateLagging},
			wantFirst: SeverityCritical,
		},
		{
			name:      "tanggal di depan kalender diperingatkan",
			bd:        businessDate("2026-09-27", domain.BusinessDateStatusOpen),
			wantCodes: []string{codeBusinessDateAhead},
			wantFirst: SeverityWarning,
		},
		{
			name:      "status in eod diperingatkan",
			bd:        businessDate("2026-09-25", domain.BusinessDateStatusEOD),
			wantCodes: []string{codeBusinessDateInEOD},
			wantFirst: SeverityWarning,
		},
		{
			name:      "gagal baca kritis",
			readErr:   errors.New("db down"),
			wantCodes: []string{codeBusinessDateUnavailable},
			wantFirst: SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateBusinessDate(now, tt.bd, tt.readErr)
			if tt.noFindings {
				if len(got) != 0 {
					t.Fatalf("ingin tanpa temuan, dapat %+v", got)
				}
				return
			}
			assertFindings(t, got, tt.wantCodes, tt.wantFirst)
		})
	}
}

func TestEvaluateEODRun(t *testing.T) {
	businessDate := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		steps             []domain.EODStepRunRecord
		readErr           error
		businessDateKnown bool
		wantCodes         []string
		wantFirst         Severity
		noFindings        bool
	}{
		{
			name:              "belum ada run diperingatkan",
			businessDateKnown: true,
			wantCodes:         []string{codeEODRunMissing},
			wantFirst:         SeverityWarning,
		},
		{
			name: "langkah gagal kritis",
			steps: []domain.EODStepRunRecord{
				{BusinessDate: businessDate, StepCode: "ppap", Status: domain.EODStepFailed, Reason: "timeout"},
			},
			businessDateKnown: true,
			wantCodes:         []string{codeEODStepFailed},
			wantFirst:         SeverityCritical,
		},
		{
			name: "kegagalan per kredit dari ringkasan",
			steps: []domain.EODStepRunRecord{
				{BusinessDate: businessDate, StepCode: "ckpn_comparison", Status: domain.EODStepRan,
					Summary: map[string]any{"failed": float64(3)}},
			},
			businessDateKnown: true,
			wantCodes:         []string{codeEODPerCredit},
			wantFirst:         SeverityWarning,
		},
		{
			name: "kredit gagal nol diabaikan",
			steps: []domain.EODStepRunRecord{
				{BusinessDate: businessDate, StepCode: "ckpn_comparison", Status: domain.EODStepRan,
					Summary: map[string]any{"failed": float64(0), "processed": float64(10)}},
			},
			businessDateKnown: true,
			noFindings:        true,
		},
		{
			name:      "gagal baca diperingatkan",
			readErr:   errors.New("db down"),
			wantCodes: []string{codeEODRunUnavailable},
			wantFirst: SeverityWarning,
		},
		{
			name:              "tanggal bisnis tak diketahui tidak melaporkan belum ada run",
			businessDateKnown: false,
			noFindings:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateEODRun(tt.steps, tt.readErr, tt.businessDateKnown)
			if tt.noFindings {
				if len(got) != 0 {
					t.Fatalf("ingin tanpa temuan, dapat %+v", got)
				}
				return
			}
			assertFindings(t, got, tt.wantCodes, tt.wantFirst)
		})
	}
}

func TestEvaluateCKPNParameters(t *testing.T) {
	tests := []struct {
		name            string
		status          domain.CKPNParametersStatus
		wantCodes       []string
		wantFirst       Severity
		wantAnyCritical bool
	}{
		{
			name:      "final dan sehat tanpa temuan",
			status:    domain.CKPNParametersStatus{Sementara: false},
			wantCodes: nil,
		},
		{
			name:      "sementara diperingatkan",
			status:    domain.CKPNParametersStatus{Sementara: true, RatificationMonths: 12},
			wantCodes: []string{codeCKPNProvisional},
			wantFirst: SeverityWarning,
		},
		{
			name: "melewati batas kritis",
			status: domain.CKPNParametersStatus{
				Sementara: true, RatificationMonths: 12,
				TemporarySince: "2024-01-01", Deadline: "2025-01-01", DeadlinePassed: true,
			},
			wantCodes:       []string{codeCKPNOverdue},
			wantAnyCritical: true,
		},
		{
			name:      "ekspor ojK diblokir kritis",
			status:    domain.CKPNParametersStatus{Sementara: false, OJKExportBlocked: true},
			wantCodes: []string{codeCKPNExport},
			wantFirst: SeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCKPNParameters(tt.status)
			if tt.wantCodes == nil {
				if len(got) != 0 {
					t.Fatalf("ingin tanpa temuan, dapat %+v", got)
				}
				return
			}
			assertContainsCodes(t, got, tt.wantCodes)
			if tt.wantFirst != "" && firstSeverity(got) != tt.wantFirst {
				t.Fatalf("severity pertama = %s, ingin %s", firstSeverity(got), tt.wantFirst)
			}
			if tt.wantAnyCritical && !hasSeverity(got, SeverityCritical) {
				t.Fatalf("ingin minimal satu temuan CRITICAL, dapat %+v", got)
			}
		})
	}
}

func TestEvaluatePPAPFreshness(t *testing.T) {
	bd := businessDate("2026-09-25", domain.BusinessDateStatusOpen)
	operational := boolPtr(true)
	last := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name        string
		bd          *domain.SystemBusinessDate
		last        time.Time
		known       bool
		readErr     error
		operational *bool
		wantCodes   []string
		noFindings  bool
	}{
		{
			name: "run terbaru segar",
			bd:   bd, last: last, known: true, operational: operational,
			noFindings: true,
		},
		{
			name: "belum ada run diperingatkan",
			bd:   bd, known: false, operational: operational,
			wantCodes: []string{codePPAPMissing},
		},
		{
			name: "run mendahului tanggal bisnis",
			bd:   bd, last: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), known: true, operational: operational,
			wantCodes: []string{codePPAPLagging},
		},
		{
			name: "instalasi belum beroperasi diabaikan",
			bd:   bd, known: false, operational: boolPtr(false),
			noFindings: true,
		},
		{
			name: "gagal baca penanda",
			bd:   bd, readErr: errors.New("db down"), operational: operational,
			wantCodes: []string{codePPAPUnavailable},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluatePPAPFreshness(tt.bd, tt.last, tt.known, tt.readErr, tt.operational)
			if tt.noFindings {
				if len(got) != 0 {
					t.Fatalf("ingin tanpa temuan, dapat %+v", got)
				}
				return
			}
			assertContainsCodes(t, got, tt.wantCodes)
		})
	}
}

func TestEvaluateCKPNReadiness(t *testing.T) {
	tests := []struct {
		name        string
		operational *bool
		readErr     error
		ckpnEnabled bool
		wantCodes   []string
		noFindings  bool
	}{
		{name: "belum beroperasi diabaikan", operational: boolPtr(false), noFindings: true},
		{name: "operasional dan ckpn mati diperingatkan", operational: boolPtr(true), wantCodes: []string{codeCKPNDisabled}},
		{name: "operasional dan ckpn menyala aman", operational: boolPtr(true), ckpnEnabled: true, noFindings: true},
		{name: "status tak diketahui tidak menyimpulkan", operational: nil, noFindings: true},
		{name: "gagal baca status diperingatkan", readErr: errors.New("db down"), wantCodes: []string{codeOperationalUnavailable}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateCKPNReadiness(tt.operational, tt.readErr, tt.ckpnEnabled)
			if tt.noFindings {
				if len(got) != 0 {
					t.Fatalf("ingin tanpa temuan, dapat %+v", got)
				}
				return
			}
			assertContainsCodes(t, got, tt.wantCodes)
		})
	}
}

func TestEvaluateSortsCriticalFirst(t *testing.T) {
	data := Data{
		Now:          time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
		BusinessDate: businessDate("2026-09-24", domain.BusinessDateStatusOpen),
		EODSteps: []domain.EODStepRunRecord{
			{BusinessDate: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), StepCode: "ppap", Status: domain.EODStepFailed},
		},
		CKPN: domain.CKPNParametersStatus{Sementara: true, RatificationMonths: 12},
	}
	got := Evaluate(data)
	if len(got) < 2 {
		t.Fatalf("ingin minimal dua temuan, dapat %+v", got)
	}
	if got[0].Severity != SeverityCritical {
		t.Fatalf("temuan pertama harus CRITICAL, dapat %+v", got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Severity == SeverityWarning && got[i].Severity == SeverityCritical {
			t.Fatalf("urutan tidak menempatkan CRITICAL lebih dulu: %+v", got)
		}
	}
}

// --- Stub untuk uji Collector ---

type stubDates struct {
	bd  *domain.SystemBusinessDate
	err error
}

func (s stubDates) GetCurrentDate(context.Context) (*domain.SystemBusinessDate, error) {
	return s.bd, s.err
}

type stubEOD struct {
	steps []domain.EODStepRunRecord
	err   error
}

func (s stubEOD) ListStepRuns(context.Context, time.Time) ([]domain.EODStepRunRecord, error) {
	return s.steps, s.err
}

type stubPPAP struct {
	date  time.Time
	known bool
	err   error
}

func (s stubPPAP) RecordRun(context.Context, time.Time, uuid.UUID) error { return nil }

func (s stubPPAP) LastRunBusinessDate(context.Context) (time.Time, bool, error) {
	return s.date, s.known, s.err
}

type stubActivity struct {
	v   bool
	err error
}

func (s stubActivity) HasOperationalActivity(context.Context) (bool, error) { return s.v, s.err }

type stubConfig struct {
	values map[string]string
}

func (s stubConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	return fallback
}

func (s stubConfig) GetInt(_ context.Context, key string, fallback int) int { return fallback }

func (s stubConfig) GetString(_ context.Context, key string, fallback string) string {
	if v, ok := s.values[key]; ok {
		return v
	}
	return fallback
}

func (s stubConfig) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := s.values[key]; ok {
		return v == "true"
	}
	return fallback
}

func (s stubConfig) Invalidate(string) {}

func TestCollectorCollect(t *testing.T) {
	now := time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)
	collector := NewCollector(Deps{
		Dates:    stubDates{bd: businessDate("2026-09-24", domain.BusinessDateStatusOpen)},
		EOD:      stubEOD{steps: []domain.EODStepRunRecord{{BusinessDate: time.Date(2026, 9, 24, 0, 0, 0, 0, time.UTC), StepCode: "ppap", Status: domain.EODStepFailed}}},
		PPAP:     stubPPAP{date: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), known: true},
		Activity: stubActivity{v: true},
		Config:   stubConfig{values: map[string]string{domain.ConfigKeyCKPNParametersStatus: "SEMENTARA"}},
		Now:      func() time.Time { return now },
	})

	snap := collector.Collect(context.Background())
	if !snap.EvaluatedAt.Equal(now.UTC()) {
		t.Fatalf("EvaluatedAt = %s, ingin %s", snap.EvaluatedAt, now.UTC())
	}
	if snap.Findings == nil {
		t.Fatal("Findings harus non-nil agar JSON stabil")
	}
	if snap.Summary.Critical == 0 || snap.Summary.Warning == 0 {
		t.Fatalf("ringkasan tidak mencerminkan temuan: %+v", snap.Summary)
	}
	for _, code := range []string{codeBusinessDateLagging, codeEODStepFailed, codePPAPLagging, codeCKPNProvisional, codeCKPNDisabled} {
		if !hasCode(snap.Findings, code) {
			t.Errorf("temuan %s tidak ditemukan pada %+v", code, snap.Findings)
		}
	}
}

func TestCollectorReportsUnreadableSources(t *testing.T) {
	collector := NewCollector(Deps{
		Dates: stubDates{err: errors.New("db down")},
		EOD:   stubEOD{err: errors.New("db down")},
		Now:   func() time.Time { return time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC) },
	})
	snap := collector.Collect(context.Background())
	if !hasCode(snap.Findings, codeBusinessDateUnavailable) {
		t.Fatalf("temuan %s tidak ditemukan: %+v", codeBusinessDateUnavailable, snap.Findings)
	}
	// Dates gagal -> tanggal bisnis nil, sehingga riwayat EOD tidak dibaca tanpa
	// dilaporkan sebagai "belum ada run".
	if hasCode(snap.Findings, codeEODRunMissing) {
		t.Fatalf("ketiadaan data tidak boleh dilaporkan sebagai eod_run_missing: %+v", snap.Findings)
	}
}

// --- Bantuan asersi ---

func assertFindings(t *testing.T, got []Finding, wantCodes []string, wantFirst Severity) {
	t.Helper()
	assertContainsCodes(t, got, wantCodes)
	if firstSeverity(got) != wantFirst {
		t.Fatalf("severity pertama = %s, ingin %s (%+v)", firstSeverity(got), wantFirst, got)
	}
}

func assertContainsCodes(t *testing.T, got []Finding, wantCodes []string) {
	t.Helper()
	for _, code := range wantCodes {
		if !hasCode(got, code) {
			t.Fatalf("temuan %s tidak ditemukan pada %+v", code, got)
		}
	}
}

func hasCode(findings []Finding, code string) bool {
	for _, f := range findings {
		if f.Code == code {
			return true
		}
	}
	return false
}

func hasSeverity(findings []Finding, severity Severity) bool {
	for _, f := range findings {
		if f.Severity == severity {
			return true
		}
	}
	return false
}

func firstSeverity(findings []Finding) Severity {
	if len(findings) == 0 {
		return ""
	}
	return findings[0].Severity
}
