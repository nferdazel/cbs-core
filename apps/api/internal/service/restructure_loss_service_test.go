package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// lossLoanRepoStub hanya mengimplementasikan method yang dipanggil jalur kerugian
// restrukturisasi; sisanya memakai interface kosong agar tidak perlu menulis seluruh
// kontrak domain.LoanRepository di sini.
type lossLoanRepoStub struct {
	domain.LoanRepository
	loan      *domain.Loan
	schedules []domain.LoanSchedule

	plainRestructureCalled bool
	txRestructureCalled    bool
	eirStored              bool
	storedEIR              decimal.Decimal
	storedEIRMethod        string
}

func (s *lossLoanRepoStub) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	return s.loan, nil
}

func (s *lossLoanRepoStub) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	return s.loan, nil
}

func (s *lossLoanRepoStub) GetSchedulesTx(context.Context, any, uuid.UUID) ([]domain.LoanSchedule, error) {
	return s.schedules, nil
}

func (s *lossLoanRepoStub) UpdateRestructure(_ context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	s.plainRestructureCalled = true
	s.loan = l
	s.schedules = schedules
	return nil
}

func (s *lossLoanRepoStub) UpdateRestructureTx(_ context.Context, _ any, l *domain.Loan, schedules []domain.LoanSchedule) error {
	s.txRestructureCalled = true
	s.loan = l
	s.schedules = schedules
	return nil
}

func (s *lossLoanRepoStub) UpdateOriginalEIRTx(_ context.Context, _ any, _ uuid.UUID, monthly decimal.Decimal, method string, _ []byte, _ time.Time) error {
	s.eirStored = true
	s.storedEIR = monthly
	s.storedEIRMethod = method
	return nil
}

var _ domain.LoanRepository = (*lossLoanRepoStub)(nil)
var _ domain.RestructureTxWriter = (*lossLoanRepoStub)(nil)
var _ domain.LoanEIRWriter = (*lossLoanRepoStub)(nil)

// lossActor adalah aktor lintas cabang: cabang aktor (001) sengaja berbeda dari cabang
// kredit (002) agar atribusi cabang jurnal dapat diuji.
func lossActor() domain.Actor {
	return domain.Actor{Username: "uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}
}

func newLossService(repo *lossLoanRepoStub, products *stubProductRepo, posting *stubPosting, cfg *ckpnConfigStub) *loanService {
	return &loanService{
		loanRepo:    repo,
		productRepo: products,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      cfg,
		txRunner:    stubTxRunner{},
	}
}

func lossLoan() (*domain.Loan, *domain.BankingProduct, uuid.UUID) {
	productID := uuid.New()
	loan := &domain.Loan{
		ID:                   uuid.New(),
		LoanNumber:           "KRD-RESTRUCT-1",
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		BranchCode:           "002",
		Collectibility:       domain.CollectibilityKol1,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
		OriginalEIRMonthly:   decimal.NewFromFloat(0.01),
	}
	product := &domain.BankingProduct{
		ID:             productID,
		Code:           "KRD-FLAT",
		ProfitScheme:   domain.SchemeInterest,
		ScheduleMethod: domain.ScheduleFlat,
		RateAnnual:     decimal.NewFromInt(12),
	}
	return loan, product, productID
}

// Saklar mati berarti jalur lama dipakai persis: tidak ada jurnal, tidak ada penulisan
// transaksional, dan tidak ada EIR yang dihitung.
func TestRestructureLoss_SaklarMatiTidakMengubahApaPun(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{}} // loan.restructure.loss.enabled tidak ada -> false
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if !repo.plainRestructureCalled {
		t.Fatal("saklar mati harus memakai jalur UpdateRestructure lama")
	}
	if repo.txRestructureCalled || repo.eirStored {
		t.Fatal("saklar mati tidak boleh menulis transaksional atau menyimpan EIR")
	}
	if len(posting.requests) != 0 {
		t.Fatalf("saklar mati tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
}

// Parameter diskonto yang DIISI tetapi di luar rentang / salah format ditolak dengan
// pesan yang menyebut kuncinya, bukan dijatuhkan menjadi nol.
func TestRestructureLoss_ParameterSalahDitolak(t *testing.T) {
	cases := map[string]string{
		"nol":         "0",
		"lebih 100":   "100.5",
		"bukan angka": "dua belas",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			loan, product, _ := lossLoan()
			repo := &lossLoanRepoStub{loan: loan}
			cfg := &ckpnConfigStub{values: map[string]string{
				"loan.restructure.loss.enabled":              "true",
				"loan.restructure.loss.discount_rate_annual": value,
			}}
			svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

			_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
				LoanID:        loan.ID,
				NewTermMonths: 12,
			}, lossActor())
			if !errors.Is(err, domain.ErrRestructureLossParamInvalid) {
				t.Fatalf("mau ErrRestructureLossParamInvalid, dapat %v", err)
			}
			if !strings.Contains(err.Error(), "loan.restructure.loss.discount_rate_annual") {
				t.Fatalf("pesan harus menyebut kunci konfigurasi, dapat %q", err.Error())
			}
		})
	}
}

// Kredit yang belum menyimpan EIR orisinal ditolak, bukan dihitung dengan suku bunga
// kontraktual.
func TestRestructureLoss_EIRHilangDitolak(t *testing.T) {
	loan, product, _ := lossLoan()
	loan.OriginalEIRMonthly = decimal.Zero
	repo := &lossLoanRepoStub{loan: loan}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

	_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:        loan.ID,
		NewTermMonths: 12,
	}, lossActor())
	if !errors.Is(err, domain.ErrEIRMissing) {
		t.Fatalf("mau ErrEIRMissing, dapat %v", err)
	}
}

// Jalur aktif: kerugian dihitung dari EIR orisinal, dijurnal Db 50401 / Kr 10301 dengan
// cabang KREDIT, dan saldo kerugian disimpan pada baris kredit.
func TestRestructureLoss_MempostingJurnalDanMenyimpanSaldo(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	// Turunkan suku bunga kontraktual: nilai kini arus kas baru < nilai tercatat.
	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if !repo.txRestructureCalled {
		t.Fatal("jalur aktif harus menyimpan lewat transaksi pemanggil")
	}
	// Cabang jurnal = cabang kredit (002), bukan cabang aktor (001).
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.BranchCode != "002" {
		t.Fatalf("cabang jurnal %q, mau cabang kredit 002", req.BranchCode)
	}
	if len(req.Lines) != 2 {
		t.Fatalf("jurnal %d baris, ingin 2", len(req.Lines))
	}
	if req.Lines[0].AccountNumber != fallbackRestructureLossExpenseConventional ||
		req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("baris debit salah: %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != fallbackRestructureLossLoanConventional ||
		req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("baris kredit salah: %+v", req.Lines[1])
	}
	if !req.Lines[0].Amount.IsPositive() || !req.Lines[0].Amount.Equal(req.Lines[1].Amount) {
		t.Fatalf("nominal jurnal tidak seimbang: %+v", req.Lines)
	}
	// Saldo kerugian tersimpan sama dengan nominal yang dijurnal.
	if !repo.loan.RestructureLossBalance.Equal(req.Lines[0].Amount) {
		t.Fatalf("saldo kerugian %s, mau %s", repo.loan.RestructureLossBalance, req.Lines[0].Amount)
	}
	if strings.Contains(req.IdempotencyKey, "  ") {
		t.Fatalf("kunci idempotensi tidak wajar: %q", req.IdempotencyKey)
	}
}

// Bila pemetaan produk untuk LOAN_RESTRUCTURE_LOSS belum ada, COA konfigurasi dipakai.
func TestRestructureLoss_CoaKonfigurasiDipakai(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{
		"loan.restructure.loss.enabled":     "true",
		"loan.restructure.loss.coa.expense": "59999",
		"loan.restructure.loss.coa.loan":    "10999",
	}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(posting.requests))
	}
	lines := posting.requests[0].Lines
	if lines[0].AccountNumber != "59999" || lines[1].AccountNumber != "10999" {
		t.Fatalf("COA konfigurasi tidak dipakai: %+v", lines)
	}
}

// Keuntungan modifikasi (nilai kini > nilai tercatat) tidak diposting sebagai kerugian
// dan tidak menambah saldo kerugian.
func TestRestructureLoss_KeuntunganTidakDijurnal(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         24,
		NewInterestRateAnnual: decimal.NewFromInt(100),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("keuntungan tidak boleh dijurnal, dapat %d jurnal", len(posting.requests))
	}
	if !repo.loan.RestructureLossBalance.IsZero() {
		t.Fatalf("saldo kerugian %s, mau tetap 0", repo.loan.RestructureLossBalance)
	}
}

// Kredit cabang lain tetap ditolak pada jalur aktif, dan pemeriksaan memakai data hasil
// kunci (LockLoanTx).
func TestRestructureLoss_TolakCabangLain(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

	_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:        loan.ID,
		NewTermMonths: 12,
	}, domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("mau ErrCrossBranchAccess, dapat %v", err)
	}
}
