package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// interestLoanRepo adalah LoanRepository minimal untuk akrual dan pembayaran bunga:
// kandidat akrual dikembalikan apa adanya, penambahan akruan idempoten per kunci
// jurnal, dan pembayaran angsuran dicatat untuk diperiksa test.
type interestLoanRepo struct {
	domain.LoanRepository

	candidates []domain.LoanInterestAccrualCandidate
	added      map[string]decimal.Decimal

	loan      *domain.Loan
	schedules []domain.LoanSchedule

	paidPrincipal decimal.Decimal
	paidProfit    decimal.Decimal
	settled       decimal.Decimal

	// Saldo yang disimpan kembali lewat UpdateOutstanding (mis. saat hapus buku).
	outstanding decimal.Decimal
	penalty     decimal.Decimal
}

func (r *interestLoanRepo) ListInterestAccrualCandidates(context.Context, time.Time, domain.Actor) ([]domain.LoanInterestAccrualCandidate, error) {
	return r.candidates, nil
}

func (r *interestLoanRepo) AddScheduleProfitAccruedTx(_ context.Context, _ any, _ uuid.UUID, amount decimal.Decimal, idempotencyKey string, _ time.Time) (bool, error) {
	if r.added == nil {
		r.added = map[string]decimal.Decimal{}
	}
	if _, exists := r.added[idempotencyKey]; exists {
		return false, nil
	}
	r.added[idempotencyKey] = amount
	return true, nil
}

func (r *interestLoanRepo) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	return r.loan, nil
}

func (r *interestLoanRepo) GetSchedules(context.Context, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *interestLoanRepo) GetSchedulesTx(context.Context, any, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *interestLoanRepo) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	return r.loan, nil
}

func (r *interestLoanRepo) UpdateSchedulePayment(_ context.Context, _ uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, _ domain.InstallmentStatus) error {
	r.paidPrincipal = paidPrincipal
	r.paidProfit = paidProfit
	r.settled = settleAccrued
	return nil
}

func (r *interestLoanRepo) UpdateSchedulePaymentTx(ctx context.Context, _ any, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status domain.InstallmentStatus) error {
	return r.UpdateSchedulePayment(ctx, scheduleID, paidPrincipal, paidProfit, settleAccrued, status)
}

func (r *interestLoanRepo) UpdateOutstandingTx(ctx context.Context, _ any, id uuid.UUID, outstanding, penalty decimal.Decimal) error {
	return r.UpdateOutstanding(ctx, id, outstanding, penalty)
}

func (r *interestLoanRepo) UpdateOutstanding(_ context.Context, _ uuid.UUID, outstanding, penalty decimal.Decimal) error {
	r.outstanding = outstanding
	r.penalty = penalty
	return nil
}

func (r *interestLoanRepo) UpdateStatus(context.Context, uuid.UUID, domain.LoanStatus, *uuid.UUID) error {
	return nil
}

var _ domain.LoanRepository = (*interestLoanRepo)(nil)

// interestFixture merangkai satu kredit konvensional dengan pemetaan jurnal lengkap.
type interestFixture struct {
	loanID    uuid.UUID
	productID uuid.UUID
	accountID uuid.UUID
	products  *penaltyProductRepo
	accounts  *penaltyAccountRepo
	repo      *interestLoanRepo
	posting   *stubPosting
}

func newInterestFixture() interestFixture {
	loanID := uuid.New()
	productID := uuid.New()
	accountID := uuid.New()

	product := &domain.BankingProduct{ID: productID, Code: "KRD-FLAT", Book: domain.BookConventional}
	products := &penaltyProductRepo{
		products: map[uuid.UUID]*domain.BankingProduct{productID: product},
		rules: map[domain.PostingEvent][]domain.JournalMappingRule{
			domain.EventLoanPrincipalPay: {
				{Direction: domain.DirectionDebit, COACode: "20100", AmountSource: domain.AmountPrincipal},
				{Direction: domain.DirectionCredit, COACode: "10301", AmountSource: domain.AmountPrincipal},
			},
			domain.EventLoanProfitPay: {
				{Direction: domain.DirectionDebit, COACode: "20100", AmountSource: domain.AmountProfit},
				{Direction: domain.DirectionCredit, COACode: "40100", AmountSource: domain.AmountProfit},
			},
			domain.EventInterestAccrual: {
				{Direction: domain.DirectionDebit, COACode: "10400", AmountSource: domain.AmountProfit},
				{Direction: domain.DirectionCredit, COACode: "40100", AmountSource: domain.AmountProfit},
			},
			domain.EventLoanPenalty: {
				{Direction: domain.DirectionDebit, COACode: "10305", AmountSource: domain.AmountPenalty},
				{Direction: domain.DirectionCredit, COACode: "40500", AmountSource: domain.AmountPenalty},
			},
		},
	}
	accounts := &penaltyAccountRepo{
		accounts: map[uuid.UUID]*domain.Account{
			accountID: {ID: accountID, AccountNumber: "1110001234", COACode: "20100"},
		},
	}
	return interestFixture{
		loanID:    loanID,
		productID: productID,
		accountID: accountID,
		products:  products,
		accounts:  accounts,
		repo:      &interestLoanRepo{},
		posting:   &stubPosting{},
	}
}

func newInterestTestService(f interestFixture) *loanService {
	return &loanService{
		loanRepo:    f.repo,
		productRepo: f.products,
		accountRepo: f.accounts,
		resolver:    stubResolver{},
		poster:      NewProductPoster(f.products, stubResolver{}, f.posting),
		posting:     f.posting,
		config:      penaltyConfig{},
		txRunner:    stubTxRunner{},
	}
}

func accrualCandidate(f interestFixture, due time.Time, profit int64) domain.LoanInterestAccrualCandidate {
	return domain.LoanInterestAccrualCandidate{
		LoanID:            f.loanID,
		LoanNumber:        "KRD-2026-0001",
		ProductID:         &f.productID,
		LoanType:          domain.LoanTypeConventionalFlat,
		LoanStatus:        domain.LoanStatusDisbursed,
		ScheduleID:        uuid.New(),
		InstallmentNo:     1,
		DueDate:           due,
		OutstandingProfit: decimal.NewFromInt(profit),
		OldestDueDate:     &due,
	}
}

func sumDirection(lines []domain.PostingLine, dir domain.EntryDirection) decimal.Decimal {
	total := decimal.Zero
	for _, l := range lines {
		if l.Direction == dir {
			total = total.Add(l.Amount)
		}
	}
	return total
}

// Angsuran jatuh tempo diakru sekali: jurnal 10400/40100 dan sisa akruan bertambah.
func TestAccrueInterest_AccruesDueInstallment(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := newInterestFixture()
	f.repo.candidates = []domain.LoanInterestAccrualCandidate{
		accrualCandidate(f, asOf.AddDate(0, 0, -10), 250_000),
	}
	svc := newInterestTestService(f)

	summary, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("AccrueInterest: %v", err)
	}
	if summary.Accrued != 1 || summary.Skipped != 0 || summary.Failed != 0 {
		t.Fatalf("ringkasan: accrued=%d skipped=%d failed=%d", summary.Accrued, summary.Skipped, summary.Failed)
	}
	if !summary.TotalAccrued.Equal(decimal.NewFromInt(250_000)) {
		t.Fatalf("total akruan %s, ingin 250.000", summary.TotalAccrued)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(f.posting.requests))
	}
	lines := f.posting.requests[0].Lines
	if len(lines) != 2 {
		t.Fatalf("jurnal akrual %d baris, ingin 2", len(lines))
	}
	if lines[0].AccountNumber != "10400" || lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("kaki debit piutang bunga salah: %+v", lines[0])
	}
	if lines[1].AccountNumber != "40100" || lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kaki kredit pendapatan bunga salah: %+v", lines[1])
	}
	if got := f.repo.added["ACCR-KRD-2026-0001-1"]; !got.Equal(decimal.NewFromInt(250_000)) {
		t.Fatalf("sisa akruan bertambah %s, ingin 250.000", got)
	}
}

// Dijalankan ulang tidak mengakru lagi (idempoten).
func TestAccrueInterest_Idempotent(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := newInterestFixture()
	f.repo.candidates = []domain.LoanInterestAccrualCandidate{
		accrualCandidate(f, asOf.AddDate(0, 0, -10), 250_000),
	}
	svc := newInterestTestService(f)

	first, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("run pertama: %v", err)
	}
	second, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("run kedua: %v", err)
	}
	if first.Accrued != 1 || second.Accrued != 0 || second.Skipped != 1 {
		t.Fatalf("run kedua harus melewati: accrued=%d skipped=%d", second.Accrued, second.Skipped)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(f.posting.requests))
	}
}

// Angsuran yang belum jatuh tempo dilewati dan tidak dijurnal.
func TestAccrueInterest_SkipsNotYetDue(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := newInterestFixture()
	f.repo.candidates = []domain.LoanInterestAccrualCandidate{
		accrualCandidate(f, asOf.AddDate(0, 0, 5), 250_000),
	}
	svc := newInterestTestService(f)

	summary, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccrueInterest: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 1 {
		t.Fatalf("angsuran belum jatuh tempo harus dilewati: accrued=%d skipped=%d", summary.Accrued, summary.Skipped)
	}
	if len(f.posting.requests) != 0 {
		t.Fatal("angsuran belum jatuh tempo tidak boleh dijurnal")
	}
}

// Kredit tidak lancar (kolektibilitas 3-5) menghentikan akrual tanpa membalik
// akruan yang sudah terbentuk.
func TestAccrueInterest_SkipsNPL(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := newInterestFixture()
	// DPD 100 dengan ambang default POJK masuk Diragukan (NPL).
	f.repo.candidates = []domain.LoanInterestAccrualCandidate{
		accrualCandidate(f, asOf.AddDate(0, 0, -100), 250_000),
	}
	svc := newInterestTestService(f)

	summary, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccrueInterest: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("NPL harus dilewati: accrued=%d skipped=%d failed=%d", summary.Accrued, summary.Skipped, summary.Failed)
	}
	if len(f.posting.requests) != 0 {
		t.Fatal("NPL tidak boleh diakru")
	}
	if f.repo.added != nil {
		t.Fatal("NPL tidak boleh menambah sisa akruan")
	}
}

// Produk tanpa pemetaan INTEREST_ACCRUAL dilewati dengan Warning, bukan gagal diam.
func TestAccrueInterest_SkipsProductWithoutMapping(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := newInterestFixture()
	delete(f.products.rules, domain.EventInterestAccrual)
	f.repo.candidates = []domain.LoanInterestAccrualCandidate{
		accrualCandidate(f, asOf.AddDate(0, 0, -10), 250_000),
	}
	svc := newInterestTestService(f)

	summary, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccrueInterest: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 1 || summary.Failed != 0 {
		t.Fatalf("tanpa pemetaan harus dilewati: accrued=%d skipped=%d failed=%d", summary.Accrued, summary.Skipped, summary.Failed)
	}
	if len(summary.Warnings) != 1 {
		t.Fatalf("harus ada 1 Warning, dapat %v", summary.Warnings)
	}
	if len(f.posting.requests) != 0 {
		t.Fatal("tanpa pemetaan tidak boleh diakru")
	}
}

// Produk syariah tidak diakru meskipun muncul sebagai kandidat.
func TestAccrueInterest_SkipsSyariah(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	f := newInterestFixture()
	c := accrualCandidate(f, asOf.AddDate(0, 0, -10), 250_000)
	c.LoanType = domain.LoanTypeSyariahMurabahah
	f.repo.candidates = []domain.LoanInterestAccrualCandidate{c}
	svc := newInterestTestService(f)

	summary, err := svc.AccrueInterest(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("AccrueInterest: %v", err)
	}
	if summary.Accrued != 0 || summary.Skipped != 1 {
		t.Fatalf("syariah harus dilewati: accrued=%d skipped=%d", summary.Accrued, summary.Skipped)
	}
	if len(f.posting.requests) != 0 {
		t.Fatal("produk syariah tidak boleh diakru")
	}
}

// paymentFixture menyiapkan satu kredit dengan satu angsuran yang bunganya sudah
// diakru sebagian, untuk menguji jalur pembayaran.
func paymentFixture(profitAccrued int64, profit int64) interestFixture {
	f := newInterestFixture()
	f.repo.loan = &domain.Loan{
		ID:                    f.loanID,
		LoanNumber:            "KRD-2026-0001",
		Status:                domain.LoanStatusDisbursed,
		ProductID:             &f.productID,
		DisbursementAccountID: f.accountID,
		OutstandingPrincipal:  decimal.NewFromInt(1_000),
	}
	f.repo.schedules = []domain.LoanSchedule{{
		ID:                  uuid.New(),
		LoanID:              f.loanID,
		InstallmentNo:       1,
		PrincipalAmount:     decimal.NewFromInt(1_000),
		ProfitAmount:        decimal.NewFromInt(profit),
		Status:              domain.InstallmentStatusPending,
		ProfitAccruedAmount: decimal.NewFromInt(profitAccrued),
	}}
	return f
}

// Angsuran yang sudah diakru sebagian: jurnal bunga tiga baris seimbang, sisa akruan
// diselesaikan sebesar settle.
func TestPayInstallment_SettlesAccruedProfitWithThreeBalancedLines(t *testing.T) {
	f := paymentFixture(60, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1,
	}, domain.Actor{Username: "tester"}); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}

	if len(f.posting.requests) != 2 {
		t.Fatalf("jurnal %d, ingin 2 (pokok + bunga)", len(f.posting.requests))
	}
	profitReq := f.posting.requests[1]
	if len(profitReq.Lines) != 3 {
		t.Fatalf("jurnal penyelesaian %d baris, ingin 3: %+v", len(profitReq.Lines), profitReq.Lines)
	}
	debit := sumDirection(profitReq.Lines, domain.DirectionDebit)
	credit := sumDirection(profitReq.Lines, domain.DirectionCredit)
	if !debit.Equal(credit) {
		t.Fatalf("jurnal tidak seimbang: debit %s != kredit %s", debit, credit)
	}
	if !debit.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("debit rekening nasabah %s, ingin 100", debit)
	}
	if !f.repo.settled.Equal(decimal.NewFromInt(60)) {
		t.Fatalf("settle %s, ingin 60", f.repo.settled)
	}
	if !f.repo.paidProfit.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("paid_profit %s, ingin 100", f.repo.paidProfit)
	}
	// Piutang bunga dikredit 60 dan pendapatan 40.
	if !profitReq.Lines[1].Amount.Equal(decimal.NewFromInt(60)) || profitReq.Lines[1].AccountNumber != "10400" {
		t.Fatalf("baris kredit piutang salah: %+v", profitReq.Lines[1])
	}
	if !profitReq.Lines[2].Amount.Equal(decimal.NewFromInt(40)) || profitReq.Lines[2].AccountNumber != "40100" {
		t.Fatalf("baris kredit pendapatan salah: %+v", profitReq.Lines[2])
	}
}

// Angsuran yang belum pernah diakru mempertahankan perilaku lama: dua baris, kredit
// pendapatan penuh, dan settle nol.
func TestPayInstallment_NotAccruedKeepsTwoLineBehavior(t *testing.T) {
	f := paymentFixture(0, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1,
	}, domain.Actor{Username: "tester"}); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}

	if len(f.posting.requests) != 2 {
		t.Fatalf("jurnal %d, ingin 2", len(f.posting.requests))
	}
	profitReq := f.posting.requests[1]
	if len(profitReq.Lines) != 2 {
		t.Fatalf("jurnal bunga %d baris, ingin 2: %+v", len(profitReq.Lines), profitReq.Lines)
	}
	if !profitReq.Lines[1].Amount.Equal(decimal.NewFromInt(100)) || profitReq.Lines[1].AccountNumber != "40100" {
		t.Fatalf("kredit pendapatan penuh salah: %+v", profitReq.Lines[1])
	}
	if !f.repo.settled.IsZero() {
		t.Fatalf("settle %s, ingin 0", f.repo.settled)
	}
}

// settle tidak pernah melebihi akruan yang tersedia, sehingga piutang bunga 10400
// tidak berubah negatif.
func TestPayInstallment_SettlementNeverExceedsAccrued(t *testing.T) {
	f := paymentFixture(150, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1,
	}, domain.Actor{Username: "tester"}); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}
	if !f.repo.settled.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("settle %s, ingin dibatasi ke porsi bunga 100", f.repo.settled)
	}
	profitReq := f.posting.requests[1]
	if len(profitReq.Lines) != 2 {
		t.Fatalf("bunga terakru penuh: %d baris, ingin 2 (tanpa baris pendapatan nol)", len(profitReq.Lines))
	}
	credit := sumDirection(profitReq.Lines, domain.DirectionCredit)
	if !credit.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("kredit %s, ingin 100", credit)
	}
}

// Waterfall: denda ditagih lebih dulu, lalu bunga, terakhir pokok. Sebelum ini denda
// tidak punya jalur pelunasan sama sekali sehingga piutangnya menumpuk.
func TestPayInstallment_MelunasiDendaLaluBungaLaluPokok(t *testing.T) {
	f := paymentFixture(0, 100)
	f.repo.loan.PenaltyAccrued = decimal.NewFromInt(25)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Amount: decimal.NewFromInt(200),
	}, domain.Actor{Username: "tester"}); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}

	// 200 dibagi 25 denda + 100 bunga + 75 pokok.
	if len(f.posting.requests) != 3 {
		t.Fatalf("jurnal %d, ingin 3 (denda + pokok + bunga)", len(f.posting.requests))
	}
	penaltyReq := f.posting.requests[0]
	if len(penaltyReq.Lines) != 2 {
		t.Fatalf("jurnal denda %d baris, ingin 2", len(penaltyReq.Lines))
	}
	if penaltyReq.Lines[0].AccountNumber != "1110001234" ||
		penaltyReq.Lines[0].Direction != domain.DirectionDebit ||
		!penaltyReq.Lines[0].Amount.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("debit rekening nasabah untuk denda salah: %+v", penaltyReq.Lines[0])
	}
	if penaltyReq.Lines[1].AccountNumber != "10305" ||
		penaltyReq.Lines[1].Direction != domain.DirectionCredit ||
		!penaltyReq.Lines[1].Amount.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("kredit piutang denda salah: %+v", penaltyReq.Lines[1])
	}

	principalReq := f.posting.requests[1]
	if !sumDirection(principalReq.Lines, domain.DirectionDebit).Equal(decimal.NewFromInt(75)) {
		t.Fatalf("pokok dibayar %s, ingin 75", sumDirection(principalReq.Lines, domain.DirectionDebit))
	}
	profitReq := f.posting.requests[2]
	if !sumDirection(profitReq.Lines, domain.DirectionDebit).Equal(decimal.NewFromInt(100)) {
		t.Fatalf("bunga dibayar %s, ingin 100", sumDirection(profitReq.Lines, domain.DirectionDebit))
	}

	if !f.repo.paidPrincipal.Equal(decimal.NewFromInt(75)) {
		t.Fatalf("paid_principal %s, ingin 75", f.repo.paidPrincipal)
	}
	if !f.repo.paidProfit.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("paid_profit %s, ingin 100", f.repo.paidProfit)
	}
}

// Angsuran sebagian kini mungkin: nominal kecil hanya menutup sebagian bunga, pokok
// belum tersentuh, dan angsurannya tidak ditandai lunas.
func TestPayInstallment_AngsuranSebagian(t *testing.T) {
	f := paymentFixture(0, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Amount: decimal.NewFromInt(50),
	}, domain.Actor{Username: "tester"}); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}

	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1 (hanya bunga)", len(f.posting.requests))
	}
	if !f.repo.paidPrincipal.IsZero() {
		t.Fatalf("pokok seharusnya belum dibayar, dapat %s", f.repo.paidPrincipal)
	}
	if !f.repo.paidProfit.Equal(decimal.NewFromInt(50)) {
		t.Fatalf("paid_profit %s, ingin 50", f.repo.paidProfit)
	}
}

// Kelebihan pembayaran ditolak dengan jelas, bukan disimpan diam-diam.
func TestPayInstallment_KelebihanPembayaranDitolak(t *testing.T) {
	f := paymentFixture(0, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Amount: decimal.NewFromInt(2_000),
	}, domain.Actor{Username: "tester"}); err == nil {
		t.Fatal("pembayaran melebihi kewajiban harus ditolak")
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("tidak boleh ada jurnal saat pembayaran ditolak, dapat %d", len(f.posting.requests))
	}
}

// stubAuditRepo merekam event audit agar test dapat memeriksa jejaknya.
type stubAuditRepo struct {
	events []domain.AuditEvent
}

func (s *stubAuditRepo) Write(_ context.Context, _ any, event domain.AuditEvent) error {
	s.events = append(s.events, event)
	return nil
}

func (s *stubAuditRepo) List(context.Context, string, string, int) ([]domain.AuditEvent, error) {
	return s.events, nil
}

// Angsuran wajib meninggalkan jejak audit. Tanpa catatan ini penerimaan kas teller
// hanya bisa direkonstruksi dari jurnal, bukan dari aksi siapa pun yang melakukannya.
func TestPayInstallment_WritesAuditEvent(t *testing.T) {
	f := paymentFixture(60, 100)
	audit := &stubAuditRepo{}
	svc := newInterestTestService(f)
	svc.auditRepo = audit

	actor := domain.Actor{UserID: uuid.New(), Username: "teller.uji", Role: domain.RoleTeller, BranchCode: "001"}
	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1,
	}, actor); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}

	if len(audit.events) != 1 {
		t.Fatalf("event audit %d, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "PAY_INSTALLMENT" || event.ResourceType != "loan" || event.ResourceID != f.loanID.String() {
		t.Fatalf("event audit tidak sesuai: %+v", event)
	}
	if event.ActorUsername != "teller.uji" || event.ActorRole != string(domain.RoleTeller) {
		t.Fatalf("identitas pelaku tidak tercatat: %+v", event)
	}
	if got := event.Changes["amount"]; got != "1100.00" {
		t.Fatalf("nominal audit %v, ingin 1100.00", got)
	}
	if got := event.Changes["installment_status"]; got != string(domain.InstallmentStatusPaid) {
		t.Fatalf("status angsuran audit %v, ingin %s", got, domain.InstallmentStatusPaid)
	}
}

// UpdateStatusTx mencatat perubahan status kredit agar test hapus buku dapat
// memastikan status tidak berubah sebelum persetujuan dan berubah sesudahnya.
func (r *interestLoanRepo) UpdateStatusTx(_ context.Context, _ any, id uuid.UUID, status domain.LoanStatus, _ *uuid.UUID) error {
	if r.loan != nil && r.loan.ID == id {
		r.loan.Status = status
	}
	return nil
}
