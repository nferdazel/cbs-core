package service

import (
	"context"
	"strings"
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

func (s *penaltyLoanRepo) ListPenaltyCandidates(context.Context, time.Time, domain.Actor) ([]domain.LoanPenaltyCandidate, error) {
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
			// Meniru penambahan loans.penalty_accrued: plafon diuji pada eksekusi
			// berikutnya memakai saldo ini.
			s.candidates[i].PenaltyAccrued = s.candidates[i].PenaltyAccrued.Add(amount)
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

type penaltyConfig struct {
	rate          decimal.Decimal
	capPercent    decimal.Decimal
	akadDisclosed bool
}

func (c penaltyConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	switch key {
	case cfgLoanPenaltyDailyRatePerMille:
		return c.rate
	case cfgLoanPenaltyCapPercent:
		if c.capPercent.IsZero() {
			return fallback
		}
		return c.capPercent
	default:
		return fallback
	}
}
func (penaltyConfig) GetInt(_ context.Context, _ string, fallback int) int { return fallback }
func (penaltyConfig) GetString(_ context.Context, _ string, fallback string) string {
	return fallback
}
func (c penaltyConfig) GetBool(_ context.Context, _ string, fallback bool) bool {
	if c.akadDisclosed {
		return true
	}
	return fallback
}
func (penaltyConfig) Invalidate(string) {}

var _ domain.SystemConfigService = penaltyConfig{}

func timePtr(t time.Time) *time.Time { return &t }

// newPenaltyTestService merangkai loanService tanpa database: transaksi dijalankan
// stubTxRunner, jurnal dicatat stubPosting.
func newPenaltyTestService(repo *penaltyLoanRepo, products *penaltyProductRepo, accounts *penaltyAccountRepo, posting *stubPosting, rate decimal.Decimal) *loanService {
	// akadDisclosed=true: sebagian besar uji memakai jalur syariah dan menguji hal
	// lain; gerbang akad diuji khusus di TestAccruePenalties_SyariahAkadGuard.
	return newPenaltyTestServiceWithConfig(repo, products, accounts, posting, penaltyConfig{rate: rate, akadDisclosed: true})
}

func newPenaltyTestServiceWithConfig(repo *penaltyLoanRepo, products *penaltyProductRepo, accounts *penaltyAccountRepo, posting *stubPosting, cfg penaltyConfig) *loanService {
	return &loanService{
		loanRepo:    repo,
		productRepo: products,
		accountRepo: accounts,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      cfg,
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
// 1.000.000 dan pemetaan LOAN_PENALTY standar (debit 10305 piutang denda, kredit
// 40500 pendapatan denda). Rekening nasabah tetap disiapkan untuk membuktikan
// akrual denda TIDAK menyentuhnya.
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
				{Event: domain.EventLoanPenalty, Direction: domain.DirectionDebit, COACode: "10305", AmountSource: domain.AmountPenalty},
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
		OutstandingPrincipal:  decimal.NewFromInt(50_000_000),
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

// Akrual dasar: denda 1.000.000 x 1‰ x 10 hari = 10.000, dan kaki debit jatuh ke
// piutang denda (10305) menurut pemetaan produk — BUKAN ke rekening nasabah.
// Denda adalah tagihan: dana nasabah tidak boleh berkurang saat denda diakru.
func TestAccruePenalties_AccruesToPenaltyReceivable(t *testing.T) {
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
	if lines[0].AccountNumber != "10305" || lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("kaki debit harus ke piutang denda 10305: %+v", lines[0])
	}
	if lines[1].AccountNumber != "40500" || lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kaki kredit pendapatan denda salah: %+v", lines[1])
	}
	// Regresi inti: rekening nasabah tidak boleh tersentuh akrual denda.
	for _, line := range lines {
		if line.AccountNumber == f.accounts.accounts[f.accountID].AccountNumber {
			t.Fatalf("akrual denda menyentuh rekening nasabah: %+v", line)
		}
	}
	if got := f.repo.added[f.loanID]; !got.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("penalty_accrued ditambah %s, ingin 10.000", got)
	}
}

// Kredit tidak lancar (kolektibilitas 3-5) tidak mengakru denda meskipun tarif
// denda diisi > 0: pendapatan denda atas kredit macet tidak diakui, konsisten
// dengan penghentian akrual bunga. Akruan yang sudah terbentuk tidak dibalik.
func TestAccruePenalties_SkipsNPL(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	// DPD 100 masuk Kurang Lancar (NPL) menurut ambang POJK default.
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -100))
	f.candidate.LastAccruedOn = nil
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("NPL harus dilewati: accrued=%d skipped=%d failed=%d", summary.Accrued, summary.Skipped, summary.Failed)
	}
	if len(f.posting.requests) != 0 {
		t.Fatal("NPL tidak boleh memposting jurnal denda (pendapatan 40500)")
	}
	if len(f.repo.added) != 0 {
		t.Fatal("NPL tidak boleh menambah penalty_accrued")
	}
}

// Dimensi jatuh tempo Kredit juga menentukan: DPD angsuran kecil (Lancar) tetapi
// Kredit sudah lewat jatuh tempo cukup lama tetap NPL, sehingga denda tidak diakru.
// Ini membuktikan FinalDueDate benar-benar dipakai, bukan hanya DPD.
func TestAccruePenalties_SkipsNPLByMaturity(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	// DPD 5 (Lancar menurut tunggakan), tetapi jatuh tempo Kredit 40 hari lalu
	// -> Diragukan (NPL) menurut dimensi maturity POJK.
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -5))
	f.candidate.FinalDueDate = timePtr(asOf.AddDate(0, 0, -40))
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 1 {
		t.Fatalf("NPL via maturity harus dilewati: accrued=%d skipped=%d", summary.Accrued, summary.Skipped)
	}
	if len(f.posting.requests) != 0 || len(f.repo.added) != 0 {
		t.Fatal("NPL via maturity tidak boleh mengakru denda")
	}
}

// Kredit lancar (kol 1-2) tetap mengakru denda meskipun tanggal jatuh tempo Kredit
// sudah diketahui dan masih jauh; gerbang NPL tidak boleh menyaingi kredit sehat.
func TestAccruePenalties_AccruesForPerformingLoanWithFinalDueDate(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -10))
	f.candidate.FinalDueDate = timePtr(asOf.AddDate(0, 12, 0))
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("kredit lancar harus diakru: accrued=%d skipped=%d failed=%d", summary.Accrued, summary.Skipped, summary.Failed)
	}
	if !summary.TotalPenalty.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("total denda %s, ingin 10.000", summary.TotalPenalty)
	}
}

// Tarif 0 tidak mengakru apa pun, termasuk untuk kredit NPL: peringatan tarif tetap
// muncul karena ada tunggakan, dan tidak ada jurnal/penambahan.
func TestAccruePenalties_ZeroRateSkipsNPL(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.candidate.OldestDueDate = timePtr(asOf.AddDate(0, 0, -100))
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.Zero)
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 0 || len(f.posting.requests) != 0 || len(f.repo.added) != 0 {
		t.Fatal("tarif 0 tidak boleh mengakru denda apa pun, termasuk NPL")
	}
	if summary.Warning == "" || summary.Overdue != 1 {
		t.Fatalf("peringatan tarif 0 harus tetap muncul: warning=%q overdue=%d", summary.Warning, summary.Overdue)
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
	// Peringatan harus menyebut jumlah kredit terdampak, bukan sekadar "ada tunggakan".
	if !strings.Contains(summary.Warning, "1 kredit") {
		t.Fatalf("peringatan harus menyebut 1 kredit menunggak, dapat %q", summary.Warning)
	}
	if summary.Overdue != 1 {
		t.Fatalf("overdue=%d, mau 1", summary.Overdue)
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

// Tarif 0 dengan beberapa kredit menunggak: peringatan menyebut jumlah yang terdampak
// agar operator tahu besarnya paparan, bukan hanya bahwa tidak ada denda diakru.
func TestAccruePenalties_ZeroRateWarningReportsAffectedCount(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)

	second := f.candidate
	second.LoanID = uuid.New()
	second.LoanNumber = "KRD-2026-0002"
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate, second}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.Zero)
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Overdue != 2 {
		t.Fatalf("overdue=%d, mau 2", summary.Overdue)
	}
	if !strings.Contains(summary.Warning, "2 kredit") {
		t.Fatalf("peringatan harus menyebut 2 kredit menunggak, dapat %q", summary.Warning)
	}
	if !strings.Contains(summary.Warning, "tidak ada denda yang diakru") {
		t.Fatalf("peringatan harus menyatakan tidak ada denda diakru, dapat %q", summary.Warning)
	}
}

// Tanpa tunggakan, tarif 0 tidak boleh menghasilkan peringatan: tidak ada denda yang
// seharusnya diakru, sehingga peringatan hanya kebisingan. Kredit yang jatuh tempo
// HARI INI belum menunggak (DPD 0) dan tidak boleh dihitung terdampak.
func TestAccruePenalties_ZeroRateNoOverdueNoWarning(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	dueToday := asOf
	f.candidate.OldestDueDate = &dueToday
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.Zero)
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Overdue != 0 {
		t.Fatalf("overdue=%d, mau 0 (jatuh tempo hari ini belum menunggak)", summary.Overdue)
	}
	if summary.Warning != "" {
		t.Fatalf("tanpa tunggakan tidak boleh ada peringatan, dapat %q", summary.Warning)
	}
}

// Plafon denda memotong nominal yang melewati batas dan menghentikan akrual
// berikutnya, serta terlihat di ringkasan. Basis: 1% dari pokok tunggakan
// 1.000.000 = 10.000. Denda 20 hari (20.000) terpotong ke plafon 10.000; hari
// berikutnya ruang plafon habis sehingga akrual dihentikan dan dihitung Capped.
func TestAccruePenalties_CapTruncatesAndStopsAccrual(t *testing.T) {
	day0 := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(day0)
	f.candidate.OverduePrincipal = decimal.NewFromInt(1_000_000)
	f.candidate.OldestDueDate = timePtr(day0.AddDate(0, 0, -20))
	f.candidate.LastAccruedOn = nil
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestServiceWithConfig(f.repo, f.products, f.accounts, f.posting,
		penaltyConfig{rate: decimal.NewFromInt(1), capPercent: decimal.NewFromInt(1)})

	first, err := svc.AccruePenalties(context.Background(), day0, domain.Actor{})
	if err != nil {
		t.Fatalf("run pertama: %v", err)
	}
	item := first.Items[0]
	if item.Status != domain.BatchItemAccrued {
		t.Fatalf("status %q, ingin ACCRUED dengan nominal terpotong plafon", item.Status)
	}
	if !item.Penalty.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("denda %s, ingin terpotong ke plafon 10.000", item.Penalty)
	}
	if !item.Capped || !item.CapAmount.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("item harus ditandai Capped dengan plafon 10.000: capped=%v cap=%s", item.Capped, item.CapAmount)
	}
	if first.Capped != 1 || !first.TotalPenalty.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("ringkasan: capped=%d total=%s, ingin 1/10.000", first.Capped, first.TotalPenalty)
	}

	// Hari berikutnya: ruang plafon sudah nol, akrual dihentikan dan wajib terlihat.
	second, err := svc.AccruePenalties(context.Background(), day0.AddDate(0, 0, 1), domain.Actor{})
	if err != nil {
		t.Fatalf("run kedua: %v", err)
	}
	if second.Accrued != 0 || second.Skipped != 1 || second.Capped != 1 {
		t.Fatalf("run kedua: accrued=%d skipped=%d capped=%d, ingin 0/1/1", second.Accrued, second.Skipped, second.Capped)
	}
	if !strings.Contains(second.Warning, "plafon denda") {
		t.Fatalf("ringkasan harus menyebut plafon, dapat %q", second.Warning)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1 (hari kedua dihentikan plafon)", len(f.posting.requests))
	}
	if got := f.repo.added[f.loanID]; !got.Equal(decimal.NewFromInt(10_000)) {
		t.Fatalf("total penalty_accrued %s, ingin tetap 10.000", got)
	}
}

// Denda pembiayaan syariah diposting ke Dana Kebajikan (12500), BUKAN pendapatan.
// Sikap sementara menunggu keputusan DPS.
func TestAccruePenalties_SyariahCreditsSocialFundNotIncome(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.products.products[f.productID].Book = domain.BookSyariah
	f.products.rules[domain.EventLoanPenalty] = []domain.JournalMappingRule{
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionDebit, COACode: "11700", AmountSource: domain.AmountPenalty},
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionCredit, COACode: "12500", AmountSource: domain.AmountPenalty},
	}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 1 || summary.SyariahSocialFund != 1 {
		t.Fatalf("accrued=%d syariahSocialFund=%d, ingin 1/1", summary.Accrued, summary.SyariahSocialFund)
	}
	if !summary.Items[0].SyariahSocialFund {
		t.Fatal("item syariah harus ditandai SyariahSocialFund")
	}
	lines := f.posting.requests[0].Lines
	if lines[0].AccountNumber != "11700" || lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("kaki debit harus piutang denda syariah 11700: %+v", lines[0])
	}
	if lines[1].AccountNumber != "12500" || lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kaki kredit harus Dana Kebajikan 12500: %+v", lines[1])
	}
	for _, line := range lines {
		if line.AccountNumber == "40500" || line.AccountNumber == "14600" {
			t.Fatalf("denda syariah tidak boleh masuk akun pendapatan: %+v", line)
		}
	}
}

// Pemetaan syariah yang mengkredit akun pendapatan DITOLAK, bukan diam-diam diakui.
func TestAccruePenalties_SyariahRejectsIncomeMapping(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	f.products.products[f.productID].Book = domain.BookSyariah
	f.products.rules[domain.EventLoanPenalty] = []domain.JournalMappingRule{
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionDebit, COACode: "11700", AmountSource: domain.AmountPenalty},
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionCredit, COACode: "14600", AmountSource: domain.AmountPenalty},
	}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Failed != 1 || summary.Accrued != 0 {
		t.Fatalf("failed=%d accrued=%d, ingin 1/0", summary.Failed, summary.Accrued)
	}
	if len(f.posting.requests) != 0 || len(f.repo.added) != 0 {
		t.Fatal("pemetaan syariah ke pendapatan tidak boleh memposting jurnal atau menambah penalty_accrued")
	}
	if len(summary.Failures) != 1 || !strings.Contains(summary.Failures[0].Error, "tidak boleh diakui sebagai pendapatan") {
		t.Fatalf("kegagalan harus menjelaskan larangan pendapatan: %+v", summary.Failures)
	}
}

// Gerbang akad (keputusan panel butir 2.1(3)): tanpa klausul akad yang dinyatakan
// tercantum, ta'zir pembiayaan syariah TIDAK diakru dan alasannya terlihat.
func TestAccruePenalties_SyariahAkadGuard(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	// Kasus A: kunci akad_disclosed masih false -> ditolak.
	f := penaltyFixtureFor(asOf)
	f.products.products[f.productID].Book = domain.BookSyariah
	f.products.rules[domain.EventLoanPenalty] = []domain.JournalMappingRule{
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionDebit, COACode: "11700", AmountSource: domain.AmountPenalty},
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionCredit, COACode: "12500", AmountSource: domain.AmountPenalty},
	}
	svc := newPenaltyTestServiceWithConfig(f.repo, f.products, f.accounts, f.posting,
		penaltyConfig{rate: decimal.NewFromInt(1), akadDisclosed: false})
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Failed != 1 || summary.Accrued != 0 {
		t.Fatalf("tanpa akad_disclosed: failed=%d accrued=%d, ingin 1/0", summary.Failed, summary.Accrued)
	}
	if len(f.posting.requests) != 0 || len(f.repo.added) != 0 {
		t.Fatal("ta'zir tanpa klausul akad tidak boleh dijurnal atau menambah penalty_accrued")
	}
	if !strings.Contains(summary.Failures[0].Error, cfgLoanPenaltySyariahAkadDisclosed) {
		t.Fatalf("galat harus menyebut kunci yang harus diisi: %q", summary.Failures[0].Error)
	}

	// Kasus B: kunci menyala -> ta'zir diakru ke dana kebajikan (bukan ditolak).
	f2 := penaltyFixtureFor(asOf)
	f2.products.products[f2.productID].Book = domain.BookSyariah
	f2.products.rules[domain.EventLoanPenalty] = []domain.JournalMappingRule{
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionDebit, COACode: "11700", AmountSource: domain.AmountPenalty},
		{Event: domain.EventLoanPenalty, Direction: domain.DirectionCredit, COACode: "12500", AmountSource: domain.AmountPenalty},
	}
	svc2 := newPenaltyTestServiceWithConfig(f2.repo, f2.products, f2.accounts, f2.posting,
		penaltyConfig{rate: decimal.NewFromInt(1), akadDisclosed: true})
	summary2, err := svc2.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary2.Failed != 0 || summary2.Accrued != 1 || summary2.SyariahSocialFund != 1 {
		t.Fatalf("akad_disclosed true: failed=%d accrued=%d socialfund=%d, ingin 0/1/1",
			summary2.Failed, summary2.Accrued, summary2.SyariahSocialFund)
	}
}

// Total denda tidak boleh melebihi sisa pokok (keputusan panel butir 2.1(2)):
// denda dipotong ke sisa pokok dan ditandai Capped, bukan melewatinya.
func TestAccruePenalties_CapAtRemainingPrincipal(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	// Sisa pokok 5.000 < denda 10.000 (1jt x 1‰ x 10 hari).
	f.candidate.OutstandingPrincipal = decimal.NewFromInt(5_000)
	f.repo.candidates = []domain.LoanPenaltyCandidate{f.candidate}

	svc := newPenaltyTestService(f.repo, f.products, f.accounts, f.posting, decimal.NewFromInt(1))
	summary, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccruePenalties: %v", err)
	}
	if summary.Accrued != 1 {
		t.Fatalf("accrued=%d, ingin 1 dengan nominal dipotong ke sisa pokok", summary.Accrued)
	}
	item := summary.Items[0]
	if !item.Penalty.Equal(decimal.NewFromInt(5_000)) {
		t.Fatalf("denda %s, ingin dipotong ke sisa pokok 5.000", item.Penalty)
	}
	if !item.Capped {
		t.Fatal("pemotongan oleh sisa pokok harus menandai Capped")
	}
}

// Plafon di atas 100% ditolak keras: bukan dipakai, bukan dipotong diam-diam.
func TestAccruePenalties_RejectsCapAbove100(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := penaltyFixtureFor(asOf)
	svc := newPenaltyTestServiceWithConfig(f.repo, f.products, f.accounts, f.posting,
		penaltyConfig{rate: decimal.NewFromInt(1), capPercent: decimal.NewFromInt(101), akadDisclosed: true})
	if _, err := svc.AccruePenalties(context.Background(), asOf, domain.Actor{}); err == nil {
		t.Fatal("cap_pct 101 harus ditolak")
	} else if !strings.Contains(err.Error(), "100") {
		t.Fatalf("galat cap harus menyebut batas 100: %v", err)
	}
	if len(f.posting.requests) != 0 || len(f.repo.added) != 0 {
		t.Fatal("konfigurasi plafon tidak sah tidak boleh menghasilkan jurnal")
	}
}
