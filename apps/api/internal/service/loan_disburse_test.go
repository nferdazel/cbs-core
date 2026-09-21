package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// disburseLoanRepo membedakan snapshot GetByID dari baris hasil kunci LockLoanTx,
// sekaligus merekam nominal yang dipakai untuk menandai kredit cair.
type disburseLoanRepo struct {
	domain.LoanRepository
	snapshot *domain.Loan
	locked   *domain.Loan

	markedID          uuid.UUID
	markedOutstanding decimal.Decimal

	// schedules dipakai jalur EIR yang kini SELALU berjalan saat pencairan.
	schedules []domain.LoanSchedule
	// Penanda penyimpanan EIR: membedakan EIR tersimpan, EIR tidak tersedia, dan
	// tidak ada penulisan sama sekali.
	eirStored       bool
	storedEIR       decimal.Decimal
	storedEIRMethod string
	storedEIRBasis  []byte
}

func (r *disburseLoanRepo) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	return r.snapshot, nil
}

func (r *disburseLoanRepo) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	src := r.locked
	if src == nil {
		src = r.snapshot
	}
	cp := *src
	return &cp, nil
}

func (r *disburseLoanRepo) GetSchedulesTx(context.Context, any, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *disburseLoanRepo) UpdateOriginalEIRTx(_ context.Context, _ any, _ uuid.UUID, monthly decimal.Decimal, method string, basis []byte, _ time.Time) error {
	r.eirStored = true
	r.storedEIR = monthly
	r.storedEIRMethod = method
	r.storedEIRBasis = basis
	return nil
}

func (r *disburseLoanRepo) MarkDisbursedTx(_ context.Context, _ any, id uuid.UUID, outstanding decimal.Decimal) error {
	r.markedID = id
	r.markedOutstanding = outstanding
	return nil
}

// disburseAccountRepo mengembalikan satu rekening pencairan.
type disburseAccountRepo struct {
	domain.AccountRepository
	account *domain.Account
}

func (r *disburseAccountRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Account, error) {
	if r.account != nil && r.account.ID == id {
		return r.account, nil
	}
	return nil, domain.ErrAccountNotFound
}

// disburseFixture membangun layanan pencairan dengan produk yang punya pemetaan jurnal
// LOAN_DISBURSEMENT lengkap, termasuk akun rekening nasabah (COA 20100) agar override
// dipakai dan jurnal terbentuk.
func disburseFixture(t *testing.T, snapshot, locked *domain.Loan) (*loanService, *disburseLoanRepo, *stubPosting, uuid.UUID) {
	t.Helper()
	productID := snapshot.ProductID
	if productID == nil {
		t.Fatal("fixture memerlukan ProductID")
	}
	product := &domain.BankingProduct{
		ID:   *productID,
		Code: "KRD-FLAT",
		Name: "Kredit Flat",
	}
	products := &stubProductRepo{
		product: product,
		rules: map[domain.PostingEvent][]domain.JournalMappingRule{
			domain.EventLoanDisbursement: {
				{Event: domain.EventLoanDisbursement, Direction: domain.DirectionDebit, COACode: "10301", AmountSource: domain.AmountPrincipal},
				{Event: domain.EventLoanDisbursement, Direction: domain.DirectionCredit, COACode: "20100", AmountSource: domain.AmountPrincipal},
			},
		},
	}
	posting := &stubPosting{}
	repo := &disburseLoanRepo{snapshot: snapshot, locked: locked}
	acc := &domain.Account{
		ID:            snapshot.DisbursementAccountID,
		AccountNumber: "1110001",
		COACode:       "20100",
	}
	accRepo := &disburseAccountRepo{account: acc}
	cfg := &ckpnConfigStub{values: map[string]string{}} // saklar kerugian restrukturisasi mati

	svc := &loanService{
		loanRepo:    repo,
		productRepo: products,
		accountRepo: accRepo,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      cfg,
		txRunner:    stubTxRunner{},
	}
	return svc, repo, posting, *productID
}

// Jurnal pencairan dan penandaan status memakai nilai baris hasil kunci, bukan snapshot
// di luar transaksi.
func TestDisburseLoan_MemakaiBarisHasilKunci(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-DISB-1",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(10_000_000),
		DisbursementAccountID: uuid.New(),
	}
	locked := *snapshot
	locked.PrincipalAmount = decimal.NewFromInt(7_000_000)

	svc, repo, posting, _ := disburseFixture(t, snapshot, &locked)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	returned, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor)
	if err != nil {
		t.Fatalf("DisburseLoan: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(posting.requests))
	}
	if got := posting.requests[0].Lines[0].Amount; !got.Equal(locked.PrincipalAmount) {
		t.Fatalf("jurnal memakai pokok %s, mau pokok hasil kunci %s (snapshot %s)",
			got, locked.PrincipalAmount, snapshot.PrincipalAmount)
	}
	if !repo.markedOutstanding.Equal(locked.PrincipalAmount) {
		t.Fatalf("penandaan memakai pokok %s, mau pokok hasil kunci %s",
			repo.markedOutstanding, locked.PrincipalAmount)
	}
	if !returned.OutstandingPrincipal.Equal(locked.PrincipalAmount) {
		t.Fatalf("kredit kembali %s, mau pokok hasil kunci %s",
			returned.OutstandingPrincipal, locked.PrincipalAmount)
	}
}

// Status otoritatif dibaca dari baris hasil kunci: kredit yang sudah cair tidak boleh
// dicairkan dua kali meskipun snapshot lama masih APPROVED.
func TestDisburseLoan_TolakSudahCairDariBarisHasilKunci(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-DISB-2",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(10_000_000),
		DisbursementAccountID: uuid.New(),
	}
	locked := *snapshot
	locked.Status = domain.LoanStatusDisbursed

	svc, repo, posting, _ := disburseFixture(t, snapshot, &locked)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	_, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor)
	if !errors.Is(err, domain.ErrLoanAlreadyDisbursed) {
		t.Fatalf("mau ErrLoanAlreadyDisbursed, dapat %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("kredit yang sudah cair tidak boleh dijurnal lagi, dapat %d", len(posting.requests))
	}
	if !repo.markedOutstanding.IsZero() {
		t.Fatalf("kredit yang sudah cair tidak boleh ditandai lagi, dapat %s", repo.markedOutstanding)
	}
}

// Status selain APPROVED/DISBURSED juga ditolak dari baris hasil kunci.
func TestDisburseLoan_TolakStatusBukanApprovedDariBarisHasilKunci(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-DISB-3",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(10_000_000),
		DisbursementAccountID: uuid.New(),
	}
	locked := *snapshot
	locked.Status = domain.LoanStatusPendingApproval

	svc, _, posting, _ := disburseFixture(t, snapshot, &locked)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	_, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor)
	if !errors.Is(err, domain.ErrLoanNotApproved) {
		t.Fatalf("mau ErrLoanNotApproved, dapat %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("kredit belum APPROVED tidak boleh dijurnal, dapat %d", len(posting.requests))
	}
}

// anuitasSederhana membuat jadwal angsuran dengan jumlah yang menghasilkan arus kas
// bertanda (pokok keluar di periode 0, angsuran masuk) sehingga IRR dapat dihitung.
func anuitasSederhana(n int, perAngsuran int64) []domain.LoanSchedule {
	schedules := make([]domain.LoanSchedule, 0, n)
	for i := 1; i <= n; i++ {
		schedules = append(schedules, domain.LoanSchedule{
			InstallmentNo:    i,
			TotalInstallment: decimal.NewFromInt(perAngsuran),
		})
	}
	return schedules
}

// INTI PERBAIKAN: EIR disimpan saat pencairan meski saklar kerugian restrukturisasi
// MATI. Saklar mati tidak boleh berarti "tidak ada EIR"; ia hanya menahan jurnal
// kerugian. Karena itu pencairan ini juga tidak boleh memunculkan jurnal kerugian.
func TestDisburseLoan_MencatatEIRTanpaBergantungSaklar(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-EIR-1",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(12_000_000),
		DisbursementAccountID: uuid.New(),
	}

	svc, repo, posting, _ := disburseFixture(t, snapshot, nil)
	// Saklar kerugian restrukturisasi mati: ckpnConfigStub kosong pada fixture.
	repo.schedules = anuitasSederhana(12, 1_100_000)
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	if _, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor); err != nil {
		t.Fatalf("DisburseLoan: %v", err)
	}
	if !repo.eirStored {
		t.Fatal("EIR harus disimpan saat pencairan meski saklar kerugian restrukturisasi mati")
	}
	if !repo.storedEIR.IsPositive() {
		t.Fatalf("EIR tersimpan %s, mau positif", repo.storedEIR)
	}
	if repo.storedEIRMethod != domain.RestructureLossEIRMethod {
		t.Fatalf("metode EIR %q, mau %q", repo.storedEIRMethod, domain.RestructureLossEIRMethod)
	}
	var basis domain.EIRBasis
	if err := json.Unmarshal(repo.storedEIRBasis, &basis); err != nil {
		t.Fatalf("basis audit EIR bukan JSON sah: %v", err)
	}
	if !basis.NetProceeds.Equal(snapshot.PrincipalAmount) {
		t.Fatalf("basis net_proceeds %s, mau pokok %s", basis.NetProceeds, snapshot.PrincipalAmount)
	}
	if len(basis.Installments) != 12 {
		t.Fatalf("basis menyimpan %d angsuran, mau 12", len(basis.Installments))
	}
	// Saklar mati tetap berarti tidak ada jurnal kerugian: hanya jurnal pencairan.
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, mau hanya 1 (pencairan); saklar mati tidak boleh memposting kerugian", len(posting.requests))
	}
	if !repo.markedOutstanding.Equal(snapshot.PrincipalAmount) {
		t.Fatalf("kredit ditandai cair dengan pokok %s, mau %s", repo.markedOutstanding, snapshot.PrincipalAmount)
	}
}

// Bila perhitungan EIR gagal konvergen, pencairan yang sah TIDAK boleh digagalkan.
// Yang dilakukan: catat EIR tidak tersedia beserta alasannya, agar restrukturisasi
// berikutnya menolak dengan pesan yang tepat alih-alih menebak.
func TestDisburseLoan_EIRGagalTidakMenggagalkanPencairan(t *testing.T) {
	productID := uuid.New()
	snapshot := &domain.Loan{
		ID:                    uuid.New(),
		LoanNumber:            "KRD-EIR-2",
		Status:                domain.LoanStatusApproved,
		ProductID:             &productID,
		BranchCode:            "001",
		PrincipalAmount:       decimal.NewFromInt(5_000_000),
		DisbursementAccountID: uuid.New(),
	}

	svc, repo, posting, _ := disburseFixture(t, snapshot, nil)
	// Jadwal kosong: arus kas tidak bertanda, perhitungan EIR pasti gagal.
	repo.schedules = nil
	actor := domain.Actor{Username: "teller.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}

	if _, err := svc.DisburseLoan(context.Background(), snapshot.ID, actor); err != nil {
		t.Fatalf("kegagalan konvergensi EIR tidak boleh menggagalkan pencairan, dapat %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal pencairan %d, mau tetap 1", len(posting.requests))
	}
	if !repo.markedOutstanding.Equal(snapshot.PrincipalAmount) {
		t.Fatalf("kredit harus tetap ditandai cair, outstanding %s", repo.markedOutstanding)
	}
	if !repo.eirStored {
		t.Fatal("ketidaktersediaan EIR harus tetap dicatat")
	}
	if !repo.storedEIR.IsZero() {
		t.Fatalf("EIR saat gagal hitung harus 0, dapat %s", repo.storedEIR)
	}
	if repo.storedEIRMethod != domain.RestructureLossEIRUnavailableMethod {
		t.Fatalf("metode %q, mau %q", repo.storedEIRMethod, domain.RestructureLossEIRUnavailableMethod)
	}
	var basis domain.EIRBasis
	if err := json.Unmarshal(repo.storedEIRBasis, &basis); err != nil {
		t.Fatalf("basis audit EIR bukan JSON sah: %v", err)
	}
	if !strings.Contains(basis.Reason, domain.ErrIRRNotConverged.Error()) {
		t.Fatalf("alasan pada basis %q, mau memuat %q", basis.Reason, domain.ErrIRRNotConverged.Error())
	}
}
