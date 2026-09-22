package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// fakeEODDefinitionRepo adalah repositori definisi in-memory untuk uji layanan admin.
type fakeEODDefinitionRepo struct {
	defs    []domain.EODStepDefinition
	saved   []domain.EODStepDefinition
	saveErr error
	listErr error
}

func (f *fakeEODDefinitionRepo) ListDefinitions(context.Context) ([]domain.EODStepDefinition, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]domain.EODStepDefinition(nil), f.defs...), nil
}

func (f *fakeEODDefinitionRepo) SaveDefinitions(_ context.Context, _ any, defs []domain.EODStepDefinition, _ uuid.UUID) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append([]domain.EODStepDefinition(nil), defs...)
	return nil
}

func (f *fakeEODDefinitionRepo) RecordStepResult(context.Context, domain.EODStepRunRecord) error {
	return nil
}
func (f *fakeEODDefinitionRepo) ListStepRuns(context.Context, time.Time) ([]domain.EODStepRunRecord, error) {
	return nil, nil
}
func (f *fakeEODDefinitionRepo) ListDueScheduledTriggers(context.Context, time.Time) ([]domain.EODTrigger, error) {
	return nil, nil
}
func (f *fakeEODDefinitionRepo) MarkTriggerRun(context.Context, string, time.Time, *time.Time) error {
	return nil
}

type fakeEODTxRunner struct{ ran bool }

func (f *fakeEODTxRunner) Run(_ context.Context, fn func(tx any) error) error {
	f.ran = true
	return fn(nil)
}

type fakeEODAuditRepo struct{ events []domain.AuditEvent }

func (f *fakeEODAuditRepo) Write(_ context.Context, _ any, event domain.AuditEvent) error {
	f.events = append(f.events, event)
	return nil
}
func (f *fakeEODAuditRepo) List(context.Context, string, string, int) ([]domain.AuditEvent, error) {
	return nil, nil
}

func validEODDefs() []domain.EODStepDefinition {
	return []domain.EODStepDefinition{
		{Code: "aro", Sequence: 10, Enabled: true, Category: "OPERASIONAL"},
		{Code: "loan_interest_accrual", Sequence: 20, Enabled: true, Core: true, Category: "AKUNTANSI"},
		{Code: "restructure_loss_amortization", Sequence: 30, Enabled: true, Core: true, Category: "AKUNTANSI",
			Prerequisites: []string{"loan_interest_accrual"}},
		{Code: "ppap", Sequence: 40, Enabled: true, Core: true, Category: "AKUNTANSI",
			Prerequisites: []string{"restructure_loss_amortization"}},
		{Code: "ckpn_comparison", Sequence: 50, Enabled: true, Core: true, Category: "PELAPORAN",
			Prerequisites: []string{"ppap"}},
		{Code: "loan_penalty_accrual", Sequence: 60, Enabled: true, Category: "AKUNTANSI"},
		{Code: "dormant", Sequence: 70, Enabled: true, Category: "OPERASIONAL"},
	}
}

func newEODDefinitionTestService(repo *fakeEODDefinitionRepo, audit *fakeEODAuditRepo) (*eodDefinitionService, *fakeEODTxRunner) {
	tx := &fakeEODTxRunner{}
	return &eodDefinitionService{repo: repo, auditRepo: audit, txRunner: tx}, tx
}

// Perubahan definisi yang sah tersimpan dan TERAUDIT. Tanpa audit, perubahan urutan
// akuntansi tidak dapat ditelusuri siapa pelakunya.
func TestUpdateEODDefinitionsSavesAndAudits(t *testing.T) {
	repo := &fakeEODDefinitionRepo{defs: validEODDefs()}
	audit := &fakeEODAuditRepo{}
	svc, tx := newEODDefinitionTestService(repo, audit)

	defs := validEODDefs()
	// Tukar posisi dua langkah independen untuk membuktikan urutan tersimpan.
	defs[0].Sequence, defs[len(defs)-1].Sequence = defs[len(defs)-1].Sequence, defs[0].Sequence
	got, err := svc.UpdateDefinitions(context.Background(), defs, domain.Actor{UserID: uuid.New(), Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("UpdateDefinitions: %v", err)
	}
	if !tx.ran {
		t.Fatal("pembaruan harus dibungkus satu transaksi")
	}
	if len(repo.saved) != len(defs) {
		t.Fatalf("tersimpan %d langkah, mau %d", len(repo.saved), len(defs))
	}
	if got[0].Code != "dormant" {
		t.Fatalf("hasil harus terurut: %s di awal", got[0].Code)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "UPDATE_EOD_STEP_DEFINITIONS" {
		t.Fatalf("audit perubahan definisi tidak tercatat: %+v", audit.events)
	}
	if audit.events[0].ResourceType != "eod_step_definitions" {
		t.Fatalf("resource_type audit %q, mau eod_step_definitions", audit.events[0].ResourceType)
	}
}

// Konfigurasi tidak sah (inti dinonaktifkan) harus ditolak SEBELUM menyentuh database
// dan tidak meninggalkan audit.
func TestUpdateEODDefinitionsRejectsCoreDisabledBeforeWrite(t *testing.T) {
	repo := &fakeEODDefinitionRepo{defs: validEODDefs()}
	audit := &fakeEODAuditRepo{}
	svc, tx := newEODDefinitionTestService(repo, audit)

	defs := validEODDefs()
	for i := range defs {
		if defs[i].Code == "ppap" || defs[i].Code == "ckpn_comparison" {
			defs[i].Enabled = false
		}
	}
	if _, err := svc.UpdateDefinitions(context.Background(), defs, domain.Actor{UserID: uuid.New()}); !errors.Is(err, domain.ErrEODCoreStepDisabled) {
		t.Fatalf("mau ErrEODCoreStepDisabled, dapat %v", err)
	}
	if tx.ran || len(repo.saved) != 0 || len(audit.events) != 0 {
		t.Fatal("konfigurasi tidak sah tidak boleh menyentuh database atau audit")
	}
}

// Langkah tidak boleh ditambah atau dihapus lewat API: daftar kode harus sama dengan
// yang ada. Ini menutup jalur penghapusan langkah inti.
func TestUpdateEODDefinitionsRejectsAddOrRemove(t *testing.T) {
	repo := &fakeEODDefinitionRepo{defs: validEODDefs()}
	svc, tx := newEODDefinitionTestService(repo, &fakeEODAuditRepo{})

	removed := validEODDefs()
	removed = removed[:len(removed)-1]
	if _, err := svc.UpdateDefinitions(context.Background(), removed, domain.Actor{UserID: uuid.New()}); err == nil {
		t.Fatal("menghapus langkah lewat API harus ditolak")
	}

	added := append(validEODDefs(), domain.EODStepDefinition{Code: "langkah_baru", Sequence: 80, Enabled: true})
	if _, err := svc.UpdateDefinitions(context.Background(), added, domain.Actor{UserID: uuid.New()}); err == nil {
		t.Fatal("menambah langkah lewat API harus ditolak")
	}
	if tx.ran {
		t.Fatal("tidak boleh ada penulisan saat daftar kode tidak sama")
	}
}
