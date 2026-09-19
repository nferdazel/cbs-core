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

type penaltyLoanRepo struct {
	domain.LoanRepository
	candidates []domain.LoanPenaltyCandidate
	added      map[uuid.UUID]decimal.Decimal
	keys       map[string]bool
}

func (s *penaltyLoanRepo) ListPenaltyCandidates(context.Context, time.Time) ([]domain.LoanPenaltyCandidate, error) {
	return s.candidates, nil
}

func (s *penaltyLoanRepo) AddPenaltyAccruedTx(_ context.Context, _ any, loanID uuid.UUID, amount decimal.Decimal, key string, accruedOn time.Time) (bool, error) {
	if s.keys == nil {
		s.keys = map[string]bool{}
	}
	if s.keys[key] {
		return false, nil
	}
	s.keys[key] = true
	if s.added == nil {
		s.added = map[uuid.UUID]decimal.Decimal{}
	}
	s.added[loanID] = s.added[loanID].Add(amount)
	// Meniru kolom penalty_last_accrued_on: pemanggilan berikutnya melihat tanggal
	// ini sehingga hanya selisih harinya yang diakru.
	for i := range s.candidates {
		if s.candidates[i].LoanID == loanID {
			s.candidates[i].LastAccruedOn = &accruedOn
		}
	}
	return true, nil
}

var _ domain.LoanRepository = (*penaltyLoanRepo)(nil)

type penaltyProductRepo struct {
	domain.ProductRepository
	products map[uuid.UUID]*domain.BankingProduct
	rules    map[domain.PostingEvent][]domain.JournalMappingRule
}

func (s *penaltyProductRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.BankingProduct, error) {
	p, ok := s.products[id]
	if !ok {
		return nil, domain.ErrProductNotFound
	}
	return p, nil
}

func (s *penaltyProductRepo) GetMapping(_ context.Context, _ uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return s.rules[event], nil
}

var _ domain.ProductRepository = (*penaltyProductRepo)(nil)

type penaltyAccountRepo struct {
	domain.AccountRepository
	accounts map[uuid.UUID]*domain.Account
}

func (s *penaltyAccountRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Account, error) {
	acc, ok := s.accounts[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return acc, nil
}

var _ domain.AccountRepository = (*penaltyAccountRepo)(nil)

type penaltyConfig struct{ rate decimal.Decimal }

func (c penaltyConfig) GetDecimal(_ context.Context, _ string, fallback decimal.Decimal) decimal.Decimal {
	if c.rate.IsZero() && fallback.IsZero() {
		return fallback
	}
	return c.rate
}
func (penaltyConfig) GetInt(_ context.Context, _ string, fallback int) int          { return fallback }
func (penaltyConfig) GetString(_ context.Context, _ string, fallback string) string { return fallback }
func (penaltyConfig) GetBool(_ context.Context, _ string, fallback bool) bool       { return fallback }
func (penaltyConfig) Invalidate(string)                                             {}

var _ domain.SystemConfigService = penaltyConfig{}

func timePtr(t time.Time) *time.Time { return &t }

// newPenaltyTestService merangkai loanService tanpa database: transaksi dijalankan
// stubTxRunner, jurnal dicatat stubPosting.
func newPenaltyTestService(repo *penaltyLoanRepo, products *penaltyProductRepo, accounts *penaltyAccountRepo, posting *stubPosting, rate decimal.Decimal) *loanService {
	return &loanService{
		loanRepo:    repo,
		productRepo: products,
		accountRepo: accounts,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      penaltyConfig{rate: rate},
		txRunner:    stubTxRunner{},
	}
}

type penaltyFixture struct {
	loanID    uuid.UUID
	productID uuid.UUID
	accountID uuid.UUID
	candidate domain.LoanPenaltyCandidate
	products  *penaltyProductRepo
	accounts  *penaltyAccountRepo
	repo      *penaltyLoanRepo
	posting   *stubPosting
}

// penaltyFixtureFor membentuk satu kredit menunggak 10 hari dengan pokok tunggakan
// 1.000.000 dan pemetaan LOAN_PENALTY standar (debit 20100, kredit 40500).
func penaltyFixtureFor(asOf time.Time) penaltyFixture {
	loanID := uuid.New()
	productID := uuid.New()
	accountID := uuid.New()
	due := asOf.AddDate(0, 0, -10)

	product := &domain.BankingProduct{ID: productID, Code: "KRD-FLAT"}
	products := &penaltyProductRepo{
		products: map[uuid.UUID]*domain.BankingProduct{productID: product},
		rules: map[domain.PostingEvent][]domain.JournalMappingRule{
			domain.EventLoanPenalty: {
				{Event: domain.EventLoanPenalty, Direction: domain.DirectionDebit, COACode: "20100", AmountSource: domain.AmountPenalty},
				{Event: domain.EventLoanPenalty, Direction: domain.DirectionCredit, COACode: "40500", AmountSource: domain.AmountPenalty},
			},
		},
	}
	accounts := &penaltyAccountRepo{
		accounts: map[uuid.UUID]*domain.Account{
			accountID: {ID: accountID, AccountNumber: "1110001234", COACode: "20100"},
		},
	}
	candidate := domain.LoanPenaltyCandidate{
		LoanID:                loanID,
		LoanNumber:            "KRD-2026-0001",
		ProductID:             &productID,
		DisbursementAccountID: accountID,
		Status:                domain.LoanStatusDisbursed,
		OverduePrincipal:      decimal.NewFromInt(1_000_000),
		OldestDueDate:         &due,
	}
	repo := &penaltyLoanRepo{candidates: []domain.LoanPenaltyCandidate{candidate}}
	return penaltyFixture{
		loanID:    loanID,
		productID: productID,
		accountID: accountID,
		candidate: candidate,
		products:  products,
		accounts:  accounts,
		repo:      repo,
		posting:   &stubPosting{},
	}
}

// Akrual dasar: denda 1.000.000 x 1‰ x 10 hari = 10.000 dan kaki debit diarahkan ke
// rekening nasabah, bukan akun kontrol 20100.
func TestAccruePenalties_AccruesAndOverridesCustomerAccount(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))

	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("ringkasan: accrued=%d skipped=%d failed=%d", summary.Accrued, summary.Skipped, summary.Failed)
	}
	if !summary.TotalPenalty.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("total denda %s, ingin 10.000", summary.TotalPenalty)
	}
	if summary.Items[0].DPD != 10 {
		t.Fatalf("DPD %d, ingin 10", summary.Items[0].DPD)
	}
	if summary.Items[0].DaysAccrued != 10 {
		t.Fatalf("DaysAccrued %d, ingin 10 (akrual pertama mengejar seluruh DPD)", summary.Items[0].DaysAccrued)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(f.posting.requests))
	}
	lines := f.posting.requests[0].Lines
	if len(lines) != 2 {
		t.Fatalf("jurnal %d baris, ingin 2", len(lines))
	}
	if lines[0].AccountNumber != "1110001234" || lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("kaki debit tidak diarahkan ke rekening nasabah: %+v", lines[0])
	}
	if lines[1].AccountNumber != "40500" || lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kaki kredit pendapatan denda salah: %+v", lines[1])
	}
	if got := f.repo.added[f.loanID]; !got.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("penalty_accrued ditambah %s, ingin 10.000", got)
	}
}

// Menjalankan batch dua kali pada tanggal yang sama tidak menggandakan denda.
func TestAccruePenalties_IdempotentOnSameDate(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	actor := domain.Actor{Username: "tester"}

	first, err := svc.AccruePenalties(context.Background(), asOf, actor)
	if err != nil {
		t.Fatalf("run pertama: %v", err)
	}
	second, err := svc.AccruePenalties(context.Background(), asOf, actor)
	if err != nil {
		t.Fatalf("run kedua: %v", err)
	}
	if first.Accrued != 1 || second.Accrued != 0 || second.Skipped != 1 {
		t.Fatalf("run kedua harus melewati: accrued=%d skipped=%d", second.Accrued, second.Skipped)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(f.posting.requests))
	}
	if got := f.repo.added[f.loanID]; !got.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("penalty_accrued %s, ingin tetap 10.000", got)
	}
}

// Tarif 0 wajib terlihat jelas: RateConfigured=false, ada Warning, dan tidak ada
// jurnal/penambahan sama sekali.
func TestAccruePenalties_ZeroRateSkipsWithWarning(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.Zero)

	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.RateConfigured {
		t.Fatal("rate 0 harus menandai RateConfigured=false")
	}
	if summary.Warning == "" {
		t.Fatal("rate 0 harus punya peringatan eksplisit")
	}
	if summary.Accrued != 0 || summary.Skipped != 1 {
		t.Fatalf("rate 0: accrued=%d skipped=%d", summary.Accrued, summary.Skipped)
	}
	if len(f.posting.requests) != 0 || len(f.repo.added) != 0 {
		t.Fatal("rate 0 tidak boleh memposting jurnal atau menambah penalty_accrued")
	}
}

// Kredit lunas dan hapus buku dilewati meskipun masih muncul di daftar.
func TestAccruePenalties_SkipsPaidOffAndWrittenOff(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)

	paidOff := f.candidate
	paidOff.LoanID = uuid.New()
	paidOff.LoanNumber = "KRD-2026-0002"
	paidOff.Status = domain.LoanStatusPaidOff
	writtenOff := f.candidate
	writtenOff.LoanID = uuid.New()
	writtenOff.LoanNumber = "KRD-2026-0003"
	writtenOff.Status = domain.LoanStatusWrittenOff
	f.repo.candidates = []domain.LoanPenaltyCandidate{paidOff, writtenOff}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 2 {
		t.Fatalf("lunas/hapus buku harus dilewati: accrued=%d skipped=%d", summary.Accrued, summary.Skipped)
	}
	if len(f.posting.requests) != 0 {
		t.Fatal("tidak boleh ada jurnal untuk kredit lunas/hapus buku")
	}
}

// Akrual pertama (LastAccruedOn nil) mengejar seluruh tunggakan: DPD 12 -> 12 hari.
func TestAccruePenalties_FirstAccrualCatchesUpWholeDPD(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -12))
	f.candidate.LastAccruedOn = nil
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	item := summary.Items[0]
	if item.DPD != 12 || item.DaysAccrued != 12 {
		t.Fatalf("DPD=%d DaysAccrued=%d, ingin 12/12", item.DPD, item.DaysAccrued)
	}
	// 1.000.000 x 1‰ x 12 = 12.000
	if !item.Penalty.Equal(decimal.NewFromInt(12_000)) {
		t.Fatalf("denda %s, ingin 12.000", item.Penalty)
	}
}

// Regresi paling penting: EOD harian harus menambah denda secara LINIER, bukan
// kuadratik. Perilaku lama menambah pokok x tarif x DPD penuh setiap hari, sehingga
// setelah DPD 1,2,3 totalnya 1+2+3 = 6 x pokok x tarif (kuadratik). Sekarang tiap
// hari hanya menambah selisih 1 hari, total 3 x pokok x tarif.
func TestAccruePenalties_DailyRunsAreLinearNotQuadratic(t *testing.T) {
	base := decimal.NewFromInt(1_000_000)
	rate := decimal.NewFromInt(1) // 1‰ per hari
	day0 := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	f := penaltyFixtureFor(day0)
	f.candidate.OverduePrincipal = base
	// Jatuh tempo sehari sebelum hari pertama, sehingga DPD = 1, 2, 3.
	f.candidate.OldestDueDate = timePtr(day0.AddDate(0, 0, -1))
	f.candidate.LastAccruedOn = nil
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, rate)

	for i, asOf := range []time.Time{day0, day0.AddDate(0, 0, 1), day0.AddDate(0, 0, 2)} {
		summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
		if err != nil {
			t.Fatalf("run hari ke-%d: %v", i+1, err)
		}
		if summary.Accrued != 1 {
			t.Fatalf("run hari ke-%d: accrued=%d, ingin 1", i+1, summary.Accrued)
		}
		item := summary.Items[0]
		if item.DaysAccrued != 1 {
			t.Fatalf("run hari ke-%d: DaysAccrued=%d, ingin 1 (hanya selisih)", i+1, item.DaysAccrued)
		}
		// 1.000.000 x 1‰ x 1 hari = 1.000 setiap run.
		if !item.Penalty.Equal(decimal.NewFromInt(1_000)) {
			t.Fatalf("run hari ke-%d: denda %s, ingin 1.000", i+1, item.Penalty)
		}
	}

	// Linier: 1.000 x 3 hari = 3.000. Kuadratik lama: 1.000 x (1+2+3) = 6.000.
	got := f.repo.added[f.loanID]
	wantLinear := decimal.NewFromInt(3_000)
	wantOldQuadratic := decimal.NewFromInt(6_000)
	if !got.Equal(wantLinear) {
		t.Fatalf("total denda %s, ingin linier %s (kuadratik lama %s)", got, wantLinear, wantOldQuadratic)
	}
	if got.Equal(wantOldQuadratic) {
		t.Fatal("total denda kuadratik: regresi akrual delta kembali terjadi")
	}
}

// Hari yang terlewat menyembuhkan diri: akrual terakhir 3 hari lalu menagih 3 hari.
func TestAccruePenalties_MissedDaysSelfHeal(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -10))
	f.candidate.LastAccruedOn = timePtr(asOf.AddDate(0, 0, -3))
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	item := summary.Items[0]
	if item.DPD != 10 || item.DaysAccrued != 3 {
		t.Fatalf("DPD=%d DaysAccrued=%d, ingin 10/3", item.DPD, item.DaysAccrued)
	}
	if !item.Penalty.Equal(decimal.NewFromInt(3_000)) {
		t.Fatalf("denda %s, ingin 3.000", item.Penalty)
	}
}

// Selisih hari tidak boleh melebihi masa tunggakan (mis. penanda tanggal hilang).
func TestAccruePenalties_DaysToAccrueCappedAtDPD(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -10))
	f.candidate.LastAccruedOn = timePtr(asOf.AddDate(0, 0, -50))
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	item := summary.Items[0]
	if item.DaysAccrued != 10 {
		t.Fatalf("DaysAccrued=%d, ingin dibatasi ke DPD 10", item.DaysAccrued)
	}
	if !item.Penalty.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("denda %s, ingin 10.000", item.Penalty)
	}
}

// Kegagalan satu kredit tidak menghentikan kredit lain.
func TestAccruePenalties_FailureDoesNotStopOtherLoans(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)

	broken := f.candidate
	broken.LoanID = uuid.New()
	broken.LoanNumber = "KRD-2026-0009"
	broken.ProductID = nil // tidak terhubung produk -> gagal
	f.repo.candidates = []domain.LoanPenaltyCandidate{broken, f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Failed != 1 || summary.Accrued != 1 {
		t.Fatalf("failed=%d accrued=%d, ingin 1/1", summary.Failed, summary.Accrued)
	}
	if len(summary.Failures) != 1 || summary.Failures[0].LoanNumber != "KRD-2026-0009" {
		t.Fatalf("kegagalan tidak tercatat: %+v", summary.Failures)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1 dari kredit yang sehat", len(f.posting.requests))
	}
}
