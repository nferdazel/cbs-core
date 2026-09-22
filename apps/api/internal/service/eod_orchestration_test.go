package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubARORunner menggantikan perpanjangan otomatis deposito pada uji urutan.
type stubARORunner struct {
	rolled int
	err    error
	order  *[]string
	called *bool
}

func (s stubARORunner) RunARO(context.Context, time.Time, domain.Actor) (int, error) {
	if s.called != nil {
		*s.called = true
	}
	if s.order != nil {
		*s.order = append(*s.order, "aro")
	}
	return s.rolled, s.err
}

func openEODDateRepo() *stubBusinessDateRepo {
	return &stubBusinessDateRepo{
		currentDate: time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		status:      domain.BusinessDateStatusOpen,
	}
}

func newEODTestService(dateRepo domain.BusinessDateRepository, repo domain.EODStepRepository,
	aro domain.ARORunner, ppap domain.PPAPRunner, penalty domain.LoanPenaltyService,
	dormant domain.DormantRunner, accrual domain.LoanInterestAccrualRunner, ckpn domain.CKPNService) domain.BatchProcessService {
	return service.NewBatchProcessService(dateRepo, nil, nil, nil, nil, nil, nil, nil, aro, ppap, penalty, dormant, accrual, ckpn, repo)
}

// Uji terpenting orkestrasi: urutan langkah yang dijalankan HARUS datang dari definisi
// database, bukan dari kode. Dua langkah independen (aro & dormant) sengaja ditukar
// posisinya; bila mesin masih memakai urutan hardcode, urutan pemanggilan tidak berubah.
func TestRunEODUsesDefinitionOrderFromDatabase(t *testing.T) {
	defs := unitEODDefinitions()
	for i := range defs {
		switch defs[i].Code {
		case "dormant":
			// Pindahkan dormant ke depan aro untuk membuktikan urutan mengikuti tabel.
			defs[i].Sequence = 5
		}
	}
	repo := &eodDefRepoStub{defs: defs}

	var order []string
	aro := stubARORunner{rolled: 1, order: &order}
	dormant := stubDormantRunner{marked: 2, order: &order}

	svc := newEODTestService(openEODDateRepo(), repo, aro, nil, nil, dormant, nil, nil)
	if _, err := svc.RunEOD(context.Background(), uuid.New()); err != nil {
		t.Fatalf("RunEOD: %v", err)
	}
	if got := strings.Join(order, "->"); got != "dormant->aro" {
		t.Fatalf("urutan pemanggilan %q, mau %q (definisi database harus dipakai)", got, "dormant->aro")
	}
}

// Langkah nonaktif di definisi dilewati: pelaksana tidak dipanggil, status SKIPPED, dan
// ada peringatan agar tidak senyap.
func TestRunEODSkipsDisabledStep(t *testing.T) {
	defs := unitEODDefinitions()
	for i := range defs {
		if defs[i].Code == "dormant" {
			defs[i].Enabled = false
		}
	}
	repo := &eodDefRepoStub{defs: defs}
	dormantCalled := false

	svc := newEODTestService(openEODDateRepo(), repo, nil, nil, nil, stubDormantRunner{called: &dormantCalled}, nil, nil)
	res, err := svc.RunEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("RunEOD: %v", err)
	}
	if dormantCalled {
		t.Fatal("langkah nonaktif tidak boleh dipanggil")
	}
	step := eodStepStatus(t, res, "dormant")
	if step.Status != domain.EODStepSkipped {
		t.Fatalf("status dormant = %s, mau SKIPPED", step.Status)
	}
	found := false
	for _, w := range res.Warnings {
		if strings.Contains(w, "dormant dinonaktifkan") {
			found = true
		}
	}
	if !found {
		t.Fatalf("langkah nonaktif harus berisik, dapat %v", res.Warnings)
	}
}

// Langkah inti tidak boleh dinonaktifkan. EOD menolak SEBELUM mengklaim tanggal, jadi
// status tanggal bisnis tidak berubah.
func TestRunEODFailsLoudWhenCoreStepDisabled(t *testing.T) {
	defs := unitEODDefinitions()
	for i := range defs {
		if defs[i].Code == "ppap" || defs[i].Code == "ckpn_comparison" {
			defs[i].Enabled = false
		}
	}
	repo := &eodDefRepoStub{defs: defs}
	dateRepo := openEODDateRepo()
	svc := newEODTestService(dateRepo, repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.RunEOD(context.Background(), uuid.New())
	if !errors.Is(err, domain.ErrEODCoreStepDisabled) {
		t.Fatalf("mau ErrEODCoreStepDisabled, dapat %v", err)
	}
	if dateRepo.status != domain.BusinessDateStatusOpen {
		t.Fatalf("status tanggal tidak boleh berubah, dapat %s", dateRepo.status)
	}
}

// Definisi yang tidak dapat dibaca harus MENGGAGALKAN tutup hari, bukan diam-diam
// melewati langkah.
func TestRunEODFailsLoudWhenDefinitionsUnavailable(t *testing.T) {
	cases := map[string]domain.EODStepRepository{
		"repositori nil":   nil,
		"repositori error": &eodDefRepoStub{listErr: errors.New("database mati")},
		"urutan ganda": &eodDefRepoStub{defs: func() []domain.EODStepDefinition {
			defs := unitEODDefinitions()
			defs[len(defs)-1].Sequence = defs[0].Sequence
			return defs
		}()},
	}
	for name, repo := range cases {
		t.Run(name, func(t *testing.T) {
			dateRepo := openEODDateRepo()
			svc := newEODTestService(dateRepo, repo, nil, nil, nil, nil, nil, nil)
			_, err := svc.RunEOD(context.Background(), uuid.New())
			if err == nil {
				t.Fatal("EOD harus gagal saat definisi tidak tersedia/tidak sah")
			}
			if dateRepo.status != domain.BusinessDateStatusOpen {
				t.Fatalf("status tanggal tidak boleh berubah, dapat %s", dateRepo.status)
			}
		})
	}
}

// Riwayat per langkah harus tercatat: satu baris per definisi, statusnya sesuai hasil,
// dan angkanya ikut tersimpan.
func TestRunEODRecordsStepHistory(t *testing.T) {
	repo := &eodDefRepoStub{}
	svc := newEODTestService(openEODDateRepo(), repo,
		stubARORunner{rolled: 4},
		stubPPAPRunner{processed: 3, adjusted: 1},
		stubPenaltyRunner{summary: domain.LoanPenaltySummary{Accrued: 2, TotalPenalty: decimal.NewFromInt(50_000)}},
		stubDormantRunner{marked: 5},
		stubInterestAccrualRunner{accrued: 6, amount: decimal.NewFromInt(120_000), amortized: 7},
		stubCKPNService{summary: domain.CKPNComparisonSummary{Enabled: true, Processed: 3}},
	)

	if _, err := svc.RunEOD(context.Background(), uuid.New()); err != nil {
		t.Fatalf("RunEOD: %v", err)
	}
	if len(repo.runs) != len(unitEODDefinitions()) {
		t.Fatalf("riwayat %d langkah, mau %d", len(repo.runs), len(unitEODDefinitions()))
	}
	for _, rec := range repo.runs {
		if rec.Trigger != "MANUAL" {
			t.Fatalf("trigger %s, mau MANUAL", rec.Trigger)
		}
		if rec.Status != domain.EODStepRan {
			t.Fatalf("langkah %s status %s (%s), mau RAN", rec.StepCode, rec.Status, rec.Reason)
		}
		if len(rec.Summary) == 0 {
			t.Fatalf("langkah %s harus menyertakan angka penting pada riwayat", rec.StepCode)
		}
	}
}

// Jalur terjadwal harus menolak bila tidak ada pemicu jatuh tempo.
func TestRunScheduledEODRejectsWithoutDueTrigger(t *testing.T) {
	repo := &eodDefRepoStub{}
	svc := newEODTestService(openEODDateRepo(), repo, nil, nil, nil, nil, nil, nil)
	if _, err := svc.RunScheduledEOD(context.Background(), uuid.New()); !errors.Is(err, domain.ErrEODNoDueTrigger) {
		t.Fatalf("mau ErrEODNoDueTrigger, dapat %v", err)
	}
}

// Pemicu terjadwal yang jatuh tempo menjalankan EOD lewat mesin yang sama dan riwayatnya
// ditandai SCHEDULED; pemicunya ditandai sudah jalan.
func TestRunScheduledEODRunsDueTrigger(t *testing.T) {
	repo := &eodDefRepoStub{due: []domain.EODTrigger{{
		Name:        "scheduled-daily",
		TriggerType: "SCHEDULED",
		Enabled:     true,
	}}}
	svc := newEODTestService(openEODDateRepo(), repo,
		stubARORunner{rolled: 1},
		stubPPAPRunner{processed: 1},
		stubPenaltyRunner{},
		stubDormantRunner{marked: 1},
		stubInterestAccrualRunner{accrued: 1, amortized: 1},
		stubCKPNService{summary: domain.CKPNComparisonSummary{Enabled: true, Processed: 1}},
	)

	res, err := svc.RunScheduledEOD(context.Background(), uuid.New())
	if err != nil {
		t.Fatalf("RunScheduledEOD: %v", err)
	}
	if res == nil {
		t.Fatal("ringkasan EOD harus dikembalikan")
	}
	if len(repo.runs) == 0 {
		t.Fatal("riwayat langkah harus tercatat")
	}
	for _, rec := range repo.runs {
		if rec.Trigger != "SCHEDULED" {
			t.Fatalf("trigger %s, mau SCHEDULED", rec.Trigger)
		}
	}
	if len(repo.marked) != 1 || repo.marked[0] != "scheduled-daily" {
		t.Fatalf("pemicu harus ditandai sudah jalan, dapat %v", repo.marked)
	}
}
