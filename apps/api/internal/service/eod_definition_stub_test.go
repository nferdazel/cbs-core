package service_test

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// unitEODDefinitions mereproduksi seed migrasi 000077: urutan, saklar, dan prasyarat
// yang berlaku sekarang. Dipakai sebagai oracle unit test (bukan sumber produksi) agar
// perilaku lama tetap teruji tanpa database.
func unitEODDefinitions() []domain.EODStepDefinition {
	return []domain.EODStepDefinition{
		{Code: "aro", Sequence: 10, Enabled: true, Category: "OPERASIONAL", Description: "ARO deposito"},
		{Code: "loan_interest_accrual", Sequence: 20, Enabled: true, Core: true, Category: "AKUNTANSI", Description: "akrual bunga"},
		{Code: "restructure_loss_amortization", Sequence: 30, Enabled: true, Core: true, Category: "AKUNTANSI",
			Prerequisites: []string{"loan_interest_accrual"}, Description: "amortisasi saldo kerugian"},
		{Code: "ppap", Sequence: 40, Enabled: true, Core: true, Category: "AKUNTANSI",
			Prerequisites: []string{"restructure_loss_amortization"}, Description: "PPAP"},
		{Code: "ckpn_comparison", Sequence: 50, Enabled: true, Core: true, Category: "PELAPORAN",
			Prerequisites: []string{"ppap"}, Description: "CKPN"},
		{Code: "loan_penalty_accrual", Sequence: 60, Enabled: true, Category: "AKUNTANSI", Description: "denda"},
		{Code: "dormant", Sequence: 70, Enabled: true, Category: "OPERASIONAL", Description: "dormant"},
	}
}

// eodDefRepoStub menggantikan repositori definisi EOD pada unit test tanpa database.
// Zero value mengembalikan definisi kanonik sehingga uji lama berperilaku sama.
type eodDefRepoStub struct {
	defs     []domain.EODStepDefinition
	saved    []domain.EODStepDefinition
	runs     []domain.EODStepRunRecord
	due      []domain.EODTrigger
	marked   []string
	listErr  error
	writeErr error
}

func (s *eodDefRepoStub) ListDefinitions(context.Context) ([]domain.EODStepDefinition, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	if s.defs == nil {
		return unitEODDefinitions(), nil
	}
	return append([]domain.EODStepDefinition(nil), s.defs...), nil
}

func (s *eodDefRepoStub) SaveDefinitions(_ context.Context, _ any, defs []domain.EODStepDefinition, _ uuid.UUID) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.saved = append([]domain.EODStepDefinition(nil), defs...)
	s.defs = append([]domain.EODStepDefinition(nil), defs...)
	return nil
}

func (s *eodDefRepoStub) RecordStepResult(_ context.Context, rec domain.EODStepRunRecord) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.runs = append(s.runs, rec)
	return nil
}

func (s *eodDefRepoStub) ListStepRuns(_ context.Context, _ time.Time) ([]domain.EODStepRunRecord, error) {
	return s.runs, s.listErr
}

func (s *eodDefRepoStub) ListDueScheduledTriggers(_ context.Context, _ time.Time) ([]domain.EODTrigger, error) {
	return s.due, s.listErr
}

func (s *eodDefRepoStub) MarkTriggerRun(_ context.Context, name string, _ time.Time, _ *time.Time) error {
	if s.writeErr != nil {
		return s.writeErr
	}
	s.marked = append(s.marked, name)
	return nil
}
