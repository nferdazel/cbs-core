package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// --- Stub ---

type stubPPAPRepo struct {
	snapshots []domain.PPAPLoanSnapshot
	reserve   decimal.Decimal
	updated   []domain.PPAPLoanUpdate
	collect   []domain.Collectibility
	updateErr error
	listErr   error
}

func (s *stubPPAPRepo) ListDueLoans(context.Context, time.Time) ([]domain.PPAPLoanSnapshot, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.snapshots, nil
}

func (s *stubPPAPRepo) UpdateCollectibility(_ context.Context, _ any, _ uuid.UUID, c domain.Collectibility) error {
	s.collect = append(s.collect, c)
	return s.updateErr
}

func (s *stubPPAPRepo) UpdateLoanState(_ context.Context, _ any, u domain.PPAPLoanUpdate) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updated = append(s.updated, u)
	return nil
}

func (s *stubPPAPRepo) GetPPAPReserveBalance(context.Context, any, string) (decimal.Decimal, error) {
	return s.reserve, nil
}

var _ domain.PPAPRepository = (*stubPPAPRepo)(nil)

type stubResolver struct{}

func (stubResolver) ResolveGLAccount(_ context.Context, _ any, coaCode string) (string, error) {
	return coaCode, nil
}

var _ domain.AccountResolver = stubResolver{}

type stubPosting struct {
	requests []domain.PostingRequest
}

func (s *stubPosting) Post(_ context.Context, req domain.PostingRequest) (*domain.JournalEntry, error) {
	s.requests = append(s.requests, req)
	return &domain.JournalEntry{}, nil
}

func (s *stubPosting) PostTx(_ context.Context, _ any, req domain.PostingRequest) (*domain.JournalEntry, error) {
	s.requests = append(s.requests, req)
	return &domain.JournalEntry{}, nil
}

var _ domain.PostingService = (*stubPosting)(nil)

type stubProductRepo struct {
	product *domain.BankingProduct
	rules   map[domain.PostingEvent][]domain.JournalMappingRule
}

func (s *stubProductRepo) List(context.Context) ([]domain.BankingProduct, error) { return nil, nil }

func (s *stubProductRepo) GetByID(context.Context, uuid.UUID) (*domain.BankingProduct, error) {
	if s.product == nil {
		return nil, domain.ErrProductNotFound
	}
	return s.product, nil
}

func (s *stubProductRepo) GetByCode(context.Context, string) (*domain.BankingProduct, error) {
	if s.product == nil {
		return nil, domain.ErrProductNotFound
	}
	return s.product, nil
}

func (s *stubProductRepo) GetMapping(_ context.Context, _ uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return s.rules[event], nil
}

var _ domain.ProductRepository = (*stubProductRepo)(nil)

type stubTxRunner struct{}

func (stubTxRunner) Run(_ context.Context, fn func(tx any) error) error { return fn(nil) }

func newTestPPAPService(repo *stubPPAPRepo, products *stubProductRepo, posting *stubPosting) *ppapService {
	return &ppapService{
		txRunner:    stubTxRunner{},
		repo:        repo,
		productRepo: products,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      nil,
	}
}

// Kredit dengan DPD 100 (Kurang Lancar, tarif 10%) yang sebelumnya diakui 0,5%
// harus memposting selisih target - cadangan lama, bukan target penuh.
func TestPPAPRunDaily_PostsDifferenceAndStopsAccrual(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-2026-0001",
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			DPD:            0,
			AccrualStatus:  domain.AccrualStatusAccrual,
			RequiredPPAP:   decimal.NewFromInt(50_000),
			LastDueDate:    &due,
		}},
	}
	products := &stubProductRepo{}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, products, posting)

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if summary.Processed != 1 || summary.Failed != 0 {
		t.Fatalf("ringkasan: processed=%d failed=%d", summary.Processed, summary.Failed)
	}
	// Tunggakan 100 hari = Kurang Lancar; 10% x 10.000.000 - 50.000 cadangan lama.
	if !summary.TotalAdjustment.Equal(decimal.NewFromInt(950_000)) {
		t.Fatalf("total penyesuaian %s, ingin 950.000", summary.TotalAdjustment)
	}

	if len(posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(posting.requests))
	}
	lines := posting.requests[0].Lines
	if len(lines) != 2 {
		t.Fatalf("jurnal punya %d baris, ingin 2", len(lines))
	}
	if lines[0].AccountNumber != "50200" || lines[0].Direction != domain.DirectionDebit ||
		!lines[0].Amount.Equal(decimal.NewFromInt(950_000)) {
		t.Fatalf("baris debit salah: %+v", lines[0])
	}
	if lines[1].AccountNumber != "10900" || lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("baris kredit salah: %+v", lines[1])
	}

	if len(repo.updated) != 1 {
		t.Fatalf("state kredit diperbarui %d kali, ingin 1", len(repo.updated))
	}
	upd := repo.updated[0]
	if upd.Collectibility != domain.KolKurangLancar {
		t.Errorf("kolektibilitas %s, ingin Kurang Lancar", upd.Collectibility.Label())
	}
	// Kurang Lancar 10% x 10.000.000 = 1.000.000 (POJK 1/2024 Pasal 19).
	if !upd.RequiredPPAP.Equal(decimal.NewFromInt(1_000_000)) {
		t.Errorf("required_ppap %s, ingin 1.000.000", upd.RequiredPPAP)
	}
	if !upd.StopAccrual || upd.AccrualStatus != domain.AccrualStatusCash {
		t.Errorf("Macet/NPL harus stop accrual: %+v", upd)
	}
}

func TestPPAPRunDaily_MacetTriggersStopAccrual(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -400)
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-2026-0002",
			Outstanding:    decimal.NewFromInt(2_000_000),
			Collectibility: domain.KolDPK,
			DPD:            400,
			AccrualStatus:  domain.AccrualStatusAccrual,
			RequiredPPAP:   decimal.Zero,
			LastDueDate:    &due,
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if summary.Failed != 0 {
		t.Fatalf("run gagal: %+v", summary.Failures)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("state diperbarui %d kali, ingin 1", len(repo.updated))
	}
	upd := repo.updated[0]
	if upd.Collectibility != domain.KolMacet {
		t.Fatalf("kolektibilitas %s, ingin Macet", upd.Collectibility.Label())
	}
	if !upd.StopAccrual || upd.AccrualStatus != domain.AccrualStatusCash {
		t.Fatalf("Macet harus stop accrual: %+v", upd)
	}
	// Target 100% dari pokok terutang, cadangan lama nol.
	if !upd.RequiredPPAP.Equal(decimal.NewFromInt(2_000_000)) {
		t.Fatalf("required_ppap %s, ingin 2.000.000", upd.RequiredPPAP)
	}
}

// Pasal 23 POJK 1/2024: kredit yang direstrukturisasi tidak boleh kembali Lancar
// hanya karena DPD-nya nol; batasnya lepas setelah 3 periode pembayaran bersih.
func TestPPAPRunDaily_RestructuredLoanCannotImprove(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf // tidak ada tunggakan: DPD 0
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:                       loanID,
			LoanNumber:                   "KRD-RESTRUKTUR",
			Outstanding:                  decimal.NewFromInt(10_000_000),
			Collectibility:               domain.KolLancar,
			AccrualStatus:                domain.AccrualStatusAccrual,
			RequiredPPAP:                 decimal.NewFromInt(50_000),
			LastDueDate:                  &due,
			IsRestructured:               true,
			PreRestructureCollectibility: domain.KolMacet,
			CleanPeriods:                 0,
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("state diperbarui %d kali, ingin 1", len(repo.updated))
	}
	upd := repo.updated[0]
	if upd.Collectibility != domain.KolKurangLancar {
		t.Fatalf("mantan Macet yang direstrukturisasi harus tetap Kurang Lancar, dapat %s",
			upd.Collectibility.Label())
	}
	if !upd.StopAccrual || upd.AccrualStatus != domain.AccrualStatusCash {
		t.Fatalf("Kurang Lancar harus cash basis: %+v", upd)
	}
	// 10% x 10.000.000 = 1.000.000, dikurangi cadangan lama 50.000.
	if !summary.TotalAdjustment.Equal(decimal.NewFromInt(950_000)) {
		t.Fatalf("total penyesuaian %s, ingin 950.000", summary.TotalAdjustment)
	}
}

// Kredit yang sudah sesuai target tidak diposting ulang dan tidak diubah.
func TestPPAPRunDaily_UnchangedIsSkipped(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-2026-0003",
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			DPD:            0,
			AccrualStatus:  domain.AccrualStatusAccrual,
			RequiredPPAP:   decimal.NewFromInt(50_000),
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("tidak boleh ada jurnal, dapat %d", len(posting.requests))
	}
	if len(repo.updated) != 0 {
		t.Fatalf("tidak boleh ada update, dapat %d", len(repo.updated))
	}
	if summary.Skipped != 1 {
		t.Fatalf("skipped %d, ingin 1", summary.Skipped)
	}
}

// Pemetaan jurnal produk harus dipakai lebih dulu daripada fallback COA konfigurasi.
func TestPPAPRunDaily_UsesProductMappingWhenPresent(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -40)
	loanID := uuid.New()
	productID := uuid.New()

	product := &domain.BankingProduct{ID: productID, Code: "KRD-FLAT", Book: domain.BookConventional}
	products := &stubProductRepo{
		product: product,
		rules: map[domain.PostingEvent][]domain.JournalMappingRule{
			domain.EventPPAPProvision: {
				{Event: domain.EventPPAPProvision, Direction: domain.DirectionDebit, COACode: "59900", AmountSource: domain.AmountPrincipal},
				{Event: domain.EventPPAPProvision, Direction: domain.DirectionCredit, COACode: "10999", AmountSource: domain.AmountPrincipal},
			},
		},
	}

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-2026-0004",
			ProductID:      &productID,
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			RequiredPPAP:   decimal.NewFromInt(50_000),
			LastDueDate:    &due,
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, products, posting)

	if _, err := svc.RunDaily(context.Background(), asOf, domain.Actor{}); err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(posting.requests))
	}
	lines := posting.requests[0].Lines
	if len(lines) != 2 || lines[0].AccountNumber != "59900" || lines[1].AccountNumber != "10999" {
		t.Fatalf("pemetaan produk tidak dipakai: %+v", lines)
	}
}

// Preview hanya menghitung, tidak memposting dan tidak mengubah state.
func TestPPAPPreview_DoesNotPostOrUpdate(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-2026-0005",
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			RequiredPPAP:   decimal.NewFromInt(50_000),
			LastDueDate:    &due,
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	summary, err := svc.Preview(context.Background(), asOf)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if !summary.Preview {
		t.Fatal("ringkasan harus ditandai preview")
	}
	if len(posting.requests) != 0 || len(repo.updated) != 0 {
		t.Fatalf("preview tidak boleh memposting/mengubah: jurnal=%d update=%d", len(posting.requests), len(repo.updated))
	}
	// Tunggakan 100 hari = Kurang Lancar; 10% x 10.000.000 - 50.000 cadangan lama.
	if !summary.TotalAdjustment.Equal(decimal.NewFromInt(950_000)) {
		t.Fatalf("total penyesuaian %s, ingin 950.000", summary.TotalAdjustment)
	}
}

// Kegagalan satu kredit tidak boleh menggagalkan batch; kredit lain tetap diproses.
func TestPPAPRunDaily_OneFailureDoesNotAbortBatch(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -40)

	repo := &stubPPAPRepo{
		updateErr: errStubPPAP,
		snapshots: []domain.PPAPLoanSnapshot{
			{
				LoanID:         uuid.New(),
				LoanNumber:     "KRD-GAGAL",
				Outstanding:    decimal.NewFromInt(1_000_000),
				Collectibility: domain.KolLancar,
				RequiredPPAP:   decimal.Zero,
				LastDueDate:    &due,
			},
			{
				LoanID:         uuid.New(),
				LoanNumber:     "KRD-SUKSES",
				Outstanding:    decimal.NewFromInt(1_000_000),
				Collectibility: domain.KolLancar,
				RequiredPPAP:   decimal.NewFromInt(5_000),
			},
		},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("failed %d, ingin 1", summary.Failed)
	}
	if summary.Processed != 1 {
		t.Fatalf("processed %d, ingin 1 (kredit kedua tetap diproses)", summary.Processed)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].LoanNumber != "KRD-GAGAL" {
		t.Fatalf("failure tidak tercatat: %+v", summary.Failures)
	}
}

type stubPPAPError struct{}

func (stubPPAPError) Error() string { return "stub ppap update gagal" }

var errStubPPAP = stubPPAPError{}
