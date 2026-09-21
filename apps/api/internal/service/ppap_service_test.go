package service

import (
	"context"
	"errors"
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
	// mappingErr memaksa GetMapping gagal, dipakai uji membedakan "tidak ada
	// pemetaan" (boleh fallback COA) dari "galat pembacaan pemetaan" (harus gagal).
	mappingErr error
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
	if s.mappingErr != nil {
		return nil, s.mappingErr
	}
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

// Pasal 31 POJK 1/2024: kredit yang direstrukturisasi tidak boleh kembali Lancar
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

// Dimensi "Kredit telah jatuh tempo" ikut menentukan golongan: kredit yang sudah
// lewat jatuh tempo 45 hari tetap Diragukan meski tidak ada tunggakan angsuran.
func TestPPAPRunDaily_MaturedLoanUsesMaturityDimension(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	finalDue := asOf.AddDate(0, 0, -45)
	lastDue := asOf // DPD angsuran nol
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-JATUH-TEMPO",
			Outstanding:    decimal.NewFromInt(2_000_000),
			Collectibility: domain.KolLancar,
			AccrualStatus:  domain.AccrualStatusAccrual,
			RequiredPPAP:   decimal.NewFromInt(10_000),
			LastDueDate:    &lastDue,
			FinalDueDate:   &finalDue,
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
	if upd.Collectibility != domain.KolDiragukan {
		t.Fatalf("jatuh tempo 45 hari harus Diragukan meski DPD 0, dapat %s",
			upd.Collectibility.Label())
	}
	if !upd.StopAccrual || upd.AccrualStatus != domain.AccrualStatusCash {
		t.Fatalf("Diragukan harus cash basis: %+v", upd)
	}
	// 50% x 2.000.000 = 1.000.000, dikurangi cadangan lama 10.000.
	if !summary.TotalAdjustment.Equal(decimal.NewFromInt(990_000)) {
		t.Fatalf("total penyesuaian %s, ingin 990.000", summary.TotalAdjustment)
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

// PPKA dihitung atas saldo SETELAH kerugian restrukturisasi (Pasal 32 POJK 1/2024 jo.
// PA BPR Bab 5.2): pokok 10.000.000 dikurangi saldo kerugian 2.000.000 = 8.000.000,
// sehingga DPK 3% menjadi 240.000 (bukan 300.000).
func TestPPAPRunDaily_MemakaiSaldoSetelahKerugianRestrukturisasi(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -40) // DPK
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:          loanID,
			LoanNumber:      "KRD-SETELAH-KERUGIAN",
			Outstanding:     decimal.NewFromInt(10_000_000),
			RestructureLoss: decimal.NewFromInt(2_000_000),
			Collectibility:  domain.KolLancar,
			RequiredPPAP:    decimal.NewFromInt(1_000_000),
			LastDueDate:     &due,
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
	if !upd.RequiredPPAP.Equal(decimal.NewFromInt(240_000)) {
		t.Fatalf("required_ppap %s, ingin 240.000 (3%% dari 8.000.000)", upd.RequiredPPAP)
	}
	if len(summary.Items) != 1 || !summary.Items[0].CarryingAmount.Equal(decimal.NewFromInt(8_000_000)) {
		t.Fatalf("dasar nilai tercatat tidak tercatat: %+v", summary.Items)
	}
}

// Celah cadangan menggantung ditutup: kredit yang sudah tidak aktif tetapi masih
// menyimpan required_ppap bukan nol ikut diproses dengan target nol, dan pelepasannya
// lewat jalur PPAP yang sudah ada (debit cadangan, kredit beban).
func TestPPAPRunDaily_MelepasCadanganKreditTidakAktif(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	loanID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         loanID,
			LoanNumber:     "KRD-LUNAS",
			Status:         domain.LoanStatusPaidOff,
			Outstanding:    decimal.NewFromInt(5_000_000), // sisa pokok basi; tetap tidak boleh dicadangkan
			Collectibility: domain.KolLancar,
			RequiredPPAP:   decimal.NewFromInt(500_000),
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
	if len(repo.updated) != 1 || !repo.updated[0].RequiredPPAP.IsZero() {
		t.Fatalf("required_ppap kredit tidak aktif harus nol: %+v", repo.updated)
	}
	if !summary.TotalAdjustment.Equal(decimal.NewFromInt(-500_000)) {
		t.Fatalf("total penyesuaian %s, ingin -500.000", summary.TotalAdjustment)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal pelepasan %d, ingin 1", len(posting.requests))
	}
	lines := posting.requests[0].Lines
	if lines[0].AccountNumber != "10900" || lines[0].Direction != domain.DirectionDebit ||
		!lines[0].Amount.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("baris debit pelepasan salah: %+v", lines[0])
	}
	if lines[1].AccountNumber != "50200" || lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("baris kredit pelepasan salah: %+v", lines[1])
	}
}

// Jurnal PPAP diatribusikan ke cabang KREDIT, bukan cabang aktor. Ini mencegah run
// oleh aktor cabang lain mencatat pelepasan cadangan kredit cabang S di cabang aktor.
func TestPPAPRunDaily_JurnalMengikutiCabangKredit(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         uuid.New(),
			LoanNumber:     "KRD-CABANG-1",
			BranchCode:     "002",
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			RequiredPPAP:   decimal.NewFromInt(50_000),
			LastDueDate:    &due,
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	if _, err := svc.RunDaily(context.Background(), asOf, domain.Actor{Username: "aktor", BranchCode: "001"}); err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(posting.requests))
	}
	if got := posting.requests[0].BranchCode; got != "002" {
		t.Fatalf("cabang jurnal %q, mau cabang kredit 002 (bukan cabang aktor 001)", got)
	}
}

// Bila cabang kredit tidak tersedia (data lama/uji), cabang jurnal jatuh ke cabang aktor
// alih-alih dibiarkan kosong.
func TestPPAPRunDaily_CabangKreditKosongJatuhKeAktor(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         uuid.New(),
			LoanNumber:     "KRD-CABANG-KOSONG",
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			RequiredPPAP:   decimal.NewFromInt(50_000),
			LastDueDate:    &due,
		}},
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)

	if _, err := svc.RunDaily(context.Background(), asOf, domain.Actor{Username: "aktor", BranchCode: "001"}); err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(posting.requests))
	}
	if got := posting.requests[0].BranchCode; got != "001" {
		t.Fatalf("cabang jurnal %q, mau fallback cabang aktor 001", got)
	}
}

// Galat pembacaan pemetaan jurnal produk tidak boleh diam-diam jatuh ke COA fallback:
// kredit dilaporkan gagal agar tidak ada jurnal ke akun bawaan tanpa jejak.
func TestPPAPRunDaily_GalatPemetaanTidakJatuhKeFallback(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)
	productID := uuid.New()

	repo := &stubPPAPRepo{
		snapshots: []domain.PPAPLoanSnapshot{{
			LoanID:         uuid.New(),
			LoanNumber:     "KRD-PEMETAAN-GAGAL",
			ProductID:      &productID,
			Outstanding:    decimal.NewFromInt(10_000_000),
			Collectibility: domain.KolLancar,
			RequiredPPAP:   decimal.NewFromInt(50_000),
			LastDueDate:    &due,
		}},
	}
	products := &stubProductRepo{
		product:    &domain.BankingProduct{ID: productID, Code: "KRD-FLAT"},
		mappingErr: errors.New("koneksi database terputus"),
	}
	posting := &stubPosting{}
	svc := newTestPPAPService(repo, products, posting)

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("kredit harus dilaporkan gagal, dapat failed=%d failures=%+v", summary.Failed, summary.Failures)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("tidak boleh ada jurnal fallback saat pemetaan gagal dibaca, dapat %d", len(posting.requests))
	}
}
