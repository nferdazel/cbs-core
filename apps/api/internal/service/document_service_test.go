package service_test

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubCustomerRepo menyediakan record nasabah (ciphertext) untuk test dokumen.
type stubCustomerRepo struct {
	record *domain.CustomerRecord
}

func (s *stubCustomerRepo) Create(ctx context.Context, record *domain.CustomerRecord) error {
	return nil
}

func (s *stubCustomerRepo) CreateTx(ctx context.Context, tx *sql.Tx, record *domain.CustomerRecord) error {
	return nil
}

func (s *stubCustomerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.CustomerRecord, error) {
	if s.record == nil || s.record.ID != id {
		return nil, domain.ErrCustomerNotFound
	}
	return s.record, nil
}

func (s *stubCustomerRepo) GetByCIF(ctx context.Context, cif string) (*domain.CustomerRecord, error) {
	return nil, domain.ErrCustomerNotFound
}

func (s *stubCustomerRepo) FindByIDCard(ctx context.Context, idCardIndex string) (*domain.CustomerRecord, error) {
	return nil, domain.ErrCustomerNotFound
}

func (s *stubCustomerRepo) GetByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]*domain.CustomerRecord, error) {
	result := make(map[uuid.UUID]*domain.CustomerRecord, len(ids))
	if s.record != nil {
		result[s.record.ID] = s.record
	}
	return result, nil
}

func (s *stubCustomerRepo) List(ctx context.Context, limit, offset int, q domain.CustomerQuery, actor domain.Actor) ([]domain.CustomerRecord, int, error) {
	return nil, 0, nil
}

func (s *stubCustomerRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.CustomerStatus) error {
	return nil
}

var _ domain.CustomerRepository = (*stubCustomerRepo)(nil)

// stubLedgerRepo melayani pencarian jurnal dari peta referensi; test tidak
// menyentuh database.
type stubLedgerRepo struct {
	domain.LedgerRepository
	entries map[string]*domain.JournalEntry
	err     error
}

func (s *stubLedgerRepo) GetJournalByRef(ctx context.Context, ref string) (*domain.JournalEntry, error) {
	if s.err != nil {
		return nil, s.err
	}
	if entry, ok := s.entries[ref]; ok && entry != nil {
		return entry, nil
	}
	return nil, errors.New("journal entry not found")
}

// stubAccountRepo mengembalikan rekening dari peta id; rekening di luar peta
// dianggap tidak ditemukan.
type stubAccountRepo struct {
	domain.AccountRepository
	accounts map[uuid.UUID]*domain.Account
}

func (s *stubAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Account, error) {
	if acc, ok := s.accounts[id]; ok && acc != nil {
		return acc, nil
	}
	return nil, domain.ErrAccountNotFound
}

// stubBankProfileRepo menggantikan pembacaan konfigurasi bank_profile.
type stubBankProfileRepo struct {
	profile *domain.BankProfile
	err     error
}

func (s *stubBankProfileRepo) Get(ctx context.Context) (*domain.BankProfile, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.profile, nil
}

func newTestCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	c, err := crypto.NewCipher("k1", base64.StdEncoding.EncodeToString(make([]byte, 32)), nil)
	if err != nil {
		t.Fatalf("membangun cipher: %v", err)
	}
	return c
}

// documentFixture menyatukan data jurnal, rekening, dan nasabah nyata untuk
// generator dokumen.
type documentFixture struct {
	svc       domain.DocumentService
	depRef    string
	wthRef    string
	accNumber string
	encName   string
	bankRepo  *stubBankProfileRepo
}

func newDocumentFixture(t *testing.T) documentFixture {
	t.Helper()

	c := newTestCipher(t)
	encName, err := c.Encrypt("Siti Aminah")
	if err != nil {
		t.Fatalf("mengenkripsi nama: %v", err)
	}

	customerID := uuid.New()
	accountID := uuid.New()
	const accountNumber = "0011010000000099"

	custRepo := &stubCustomerRepo{record: &domain.CustomerRecord{ID: customerID, FullNameEnc: encName}}
	accRepo := &stubAccountRepo{accounts: map[uuid.UUID]*domain.Account{
		accountID: {
			ID:            accountID,
			AccountNumber: accountNumber,
			CustomerID:    &customerID,
			AccountType:   domain.AccountTypeSavings,
		},
	}}

	newEntry := func(ref string, txType domain.TransactionType, amount decimal.Decimal) *domain.JournalEntry {
		entryID := uuid.New()
		return &domain.JournalEntry{
			ID:              entryID,
			ReferenceNumber: ref,
			TransactionType: txType,
			Description:     "Setoran Tunai Simpanan Nasabah",
			Status:          domain.JournalStatusPosted,
			PostedAt:        time.Now().UTC(),
			CreatedBy:       "TELLER-01",
			Lines: []domain.JournalLine{
				{
					ID:             uuid.New(),
					JournalEntryID: entryID,
					AccountID:      accountID,
					AccountNumber:  accountNumber,
					Direction:      domain.DirectionCredit,
					Amount:         amount,
				},
			},
		}
	}

	depRef := "DEP-20260919-00001"
	wthRef := "WTH-20260919-00001"
	ledgerRepo := &stubLedgerRepo{entries: map[string]*domain.JournalEntry{
		depRef: newEntry(depRef, domain.TxTypeDeposit, decimal.NewFromInt(1500000)),
		wthRef: newEntry(wthRef, domain.TxTypeWithdrawal, decimal.NewFromInt(750000)),
	}}

	bankRepo := &stubBankProfileRepo{profile: &domain.BankProfile{
		Name:    "BPR Mitra Amanah",
		Address: "Jl. Merdeka No. 1",
		City:    "Bandung",
		Phone:   "022-1234567",
		NPWP:    "01.234.567.8-901.000",
	}}

	docSvc := service.NewDocumentService(ledgerRepo, accRepo, &stubLoanRepo{}, custRepo, bankRepo, c)
	return documentFixture{svc: docSvc, depRef: depRef, wthRef: wthRef, accNumber: accountNumber, encName: encName, bankRepo: bankRepo}
}

func TestDocumentService_HTMLGenerators(t *testing.T) {
	loanID := uuid.New()
	repo := &stubLoanRepo{
		loan: &domain.Loan{
			ID:                 loanID,
			LoanNumber:         "KRD-2026-00001",
			PrincipalAmount:    decimal.NewFromInt(50000000),
			InterestRateAnnual: decimal.NewFromInt(12),
			TermMonths:         12,
			TotalPayable:       decimal.NewFromInt(56000000),
			MonthlyInstallment: decimal.NewFromInt(4666667),
			Status:             domain.LoanStatusApproved,
			CreatedAt:          time.Now().UTC(),
		},
		schedules: []domain.LoanSchedule{
			{
				ID:               uuid.New(),
				InstallmentNo:    1,
				DueDate:          time.Now().UTC().AddDate(0, 1, 0),
				PrincipalAmount:  decimal.NewFromInt(4166667),
				ProfitAmount:     decimal.NewFromInt(500000),
				TotalInstallment: decimal.NewFromInt(4666667),
				ProfitType:       domain.ProfitTypeInterest,
				Status:           domain.InstallmentStatusPending,
			},
		},
	}
	fixture := newDocumentFixture(t)

	// 1. Test Deposit Slip HTML
	depHTML, err := fixture.svc.GenerateDepositSlipHTML(context.Background(), fixture.depRef)
	if err != nil {
		t.Fatalf("unexpected error generating deposit slip: %v", err)
	}
	if !strings.Contains(depHTML, "SLIP SETORAN TUNAI TELLER") {
		t.Fatal("expected deposit slip HTML to contain header title")
	}

	// 2. Test Withdrawal Slip HTML
	wthHTML, err := fixture.svc.GenerateWithdrawalSlipHTML(context.Background(), fixture.wthRef)
	if err != nil {
		t.Fatalf("unexpected error generating withdrawal slip: %v", err)
	}
	if !strings.Contains(wthHTML, "SLIP PENARIKAN TUNAI TELLER") {
		t.Fatal("expected withdrawal slip HTML to contain header title")
	}

	// 3. Test Loan Agreement HTML
	loanSvc := service.NewDocumentService(nil, nil, repo, nil, fixture.bankRepo, newTestCipher(t))
	loanHTML, err := loanSvc.GenerateLoanAgreementHTML(context.Background(), loanID)
	if err != nil {
		t.Fatalf("unexpected error generating loan agreement: %v", err)
	}
	if !strings.Contains(loanHTML, "SURAT PERJANJIAN KREDIT / AKAD PEMBIAYAAN") {
		t.Fatal("expected loan agreement HTML to contain title")
	}

	// 4. Test Thermal Receipt Text
	receiptText, err := fixture.svc.GenerateThermalReceiptText(context.Background(), fixture.depRef)
	if err != nil {
		t.Fatalf("unexpected error generating thermal receipt: %v", err)
	}
	if !strings.Contains(receiptText, "STRUK BUKTI PENERIMAAN KAS") {
		t.Fatal("expected thermal receipt to contain header text")
	}
}

// Slip setoran harus memuat nama nasabah hasil dekripsi, nomor rekening, dan
// nominal dari jurnal, bukan data karangan.
func TestDocumentService_DepositSlipUsesRealCustomerData(t *testing.T) {
	fixture := newDocumentFixture(t)

	html, err := fixture.svc.GenerateDepositSlipHTML(context.Background(), fixture.depRef)
	if err != nil {
		t.Fatalf("generate deposit slip: %v", err)
	}
	if !strings.Contains(html, "Siti Aminah") {
		t.Fatal("nama nasabah nyata hasil dekripsi tidak muncul di slip setoran")
	}
	if !strings.Contains(html, fixture.accNumber) {
		t.Fatalf("nomor rekening nyata %s tidak muncul di slip setoran", fixture.accNumber)
	}
	if !strings.Contains(html, "1500000.00") {
		t.Fatal("nominal jurnal tidak muncul di slip setoran")
	}
	if !strings.Contains(html, "BPR Mitra Amanah") {
		t.Fatal("nama bank dari konfigurasi tidak muncul di slip setoran")
	}
	if strings.Contains(html, fixture.encName) {
		t.Fatal("nilai nama terenkripsi tidak boleh ikut tercetak di slip setoran")
	}
}

// Slip penarikan harus memakai data jurnal yang sama, bukan literal.
func TestDocumentService_WithdrawalSlipUsesRealCustomerData(t *testing.T) {
	fixture := newDocumentFixture(t)

	html, err := fixture.svc.GenerateWithdrawalSlipHTML(context.Background(), fixture.wthRef)
	if err != nil {
		t.Fatalf("generate withdrawal slip: %v", err)
	}
	if !strings.Contains(html, "Siti Aminah") {
		t.Fatal("nama nasabah nyata hasil dekripsi tidak muncul di slip penarikan")
	}
	if !strings.Contains(html, fixture.accNumber) {
		t.Fatalf("nomor rekening nyata %s tidak muncul di slip penarikan", fixture.accNumber)
	}
	if !strings.Contains(html, "750000.00") {
		t.Fatal("nominal jurnal tidak muncul di slip penarikan")
	}
}

// Struk thermal memakai data jurnal yang sama.
func TestDocumentService_ThermalReceiptUsesRealCustomerData(t *testing.T) {
	fixture := newDocumentFixture(t)

	text, err := fixture.svc.GenerateThermalReceiptText(context.Background(), fixture.depRef)
	if err != nil {
		t.Fatalf("generate thermal receipt: %v", err)
	}
	if !strings.Contains(text, "Siti Aminah") {
		t.Fatal("nama nasabah nyata tidak muncul di struk thermal")
	}
	if !strings.Contains(text, fixture.accNumber) {
		t.Fatalf("nomor rekening nyata %s tidak muncul di struk thermal", fixture.accNumber)
	}
	if !strings.Contains(text, "BPR Mitra Amanah") {
		t.Fatal("nama bank dari konfigurasi tidak muncul di struk thermal")
	}
}

// Tidak ada satu pun dokumen yang boleh memuat string DEMO atau identitas bank
// karangan. Inilah bukti langsung bahwa data palsu sudah hilang dari keluaran.
func TestDocumentService_DocumentsContainNoDemoData(t *testing.T) {
	fixture := newDocumentFixture(t)
	ctx := context.Background()

	depHTML, err := fixture.svc.GenerateDepositSlipHTML(ctx, fixture.depRef)
	if err != nil {
		t.Fatalf("generate deposit slip: %v", err)
	}
	wthHTML, err := fixture.svc.GenerateWithdrawalSlipHTML(ctx, fixture.wthRef)
	if err != nil {
		t.Fatalf("generate withdrawal slip: %v", err)
	}
	thermal, err := fixture.svc.GenerateThermalReceiptText(ctx, fixture.depRef)
	if err != nil {
		t.Fatalf("generate thermal receipt: %v", err)
	}

	outputs := map[string]string{
		"slip setoran":   depHTML,
		"slip penarikan": wthHTML,
		"struk thermal":  thermal,
	}
	for name, out := range outputs {
		if strings.Contains(out, "DEMO") {
			t.Fatalf("%s masih memuat string DEMO", name)
		}
		if strings.Contains(out, "201001002003") {
			t.Fatalf("%s masih memuat nomor rekening palsu", name)
		}
		if strings.Contains(out, "BANK PEREKONOMIAN RAKYAT") {
			t.Fatalf("%s masih memuat identitas bank hardcoded", name)
		}
	}
}

// Nomor struk yang tidak tertaut ke jurnal harus ditolak dengan error, bukan
// dicetak dengan data karangan.
func TestDocumentService_ThermalReceiptRejectsUnknownNumber(t *testing.T) {
	fixture := newDocumentFixture(t)
	ledgerRepo := &stubLedgerRepo{err: errors.New("journal entry not found")}
	docSvc := service.NewDocumentService(ledgerRepo, nil, nil, nil, fixture.bankRepo, newTestCipher(t))

	if _, err := docSvc.GenerateThermalReceiptText(context.Background(), "MBL-20260919-00001"); err == nil {
		t.Fatal("diharapkan error saat nomor struk tidak tertaut ke jurnal")
	}
}

// Jurnal yang tidak ditemukan harus menghasilkan error, bukan dokumen mock.
func TestDocumentService_DepositSlipRejectsMissingJournal(t *testing.T) {
	fixture := newDocumentFixture(t)
	ledgerRepo := &stubLedgerRepo{err: errors.New("journal entry not found")}
	docSvc := service.NewDocumentService(ledgerRepo, nil, nil, nil, fixture.bankRepo, newTestCipher(t))

	if _, err := docSvc.GenerateDepositSlipHTML(context.Background(), "DEP-TIDAK-ADA"); err == nil {
		t.Fatal("diharapkan error saat jurnal tidak ditemukan")
	}
}

// Jurnal tanpa kaki rekening nasabah tidak boleh dicetak dengan data tebakan.
func TestDocumentService_DepositSlipRejectsJournalWithoutCustomerLine(t *testing.T) {
	fixture := newDocumentFixture(t)
	c := newTestCipher(t)
	encName, err := c.Encrypt("Siti Aminah")
	if err != nil {
		t.Fatalf("mengenkripsi nama: %v", err)
	}

	glAccountID := uuid.New()
	customerID := uuid.New()
	custRepo := &stubCustomerRepo{record: &domain.CustomerRecord{ID: customerID, FullNameEnc: encName}}
	accRepo := &stubAccountRepo{accounts: map[uuid.UUID]*domain.Account{
		glAccountID: {
			ID:          glAccountID,
			AccountType: domain.AccountTypeInternalGL,
			// CustomerID nil menandai akun GL, bukan rekening nasabah.
		},
	}}
	ref := "DEP-20260919-00002"
	ledgerRepo := &stubLedgerRepo{entries: map[string]*domain.JournalEntry{
		ref: {
			ReferenceNumber: ref,
			TransactionType: domain.TxTypeDeposit,
			Lines: []domain.JournalLine{
				{AccountID: glAccountID, Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(1000)},
			},
		},
	}}
	docSvc := service.NewDocumentService(ledgerRepo, accRepo, nil, custRepo, fixture.bankRepo, c)

	if _, err := docSvc.GenerateDepositSlipHTML(context.Background(), ref); err == nil {
		t.Fatal("diharapkan error saat jurnal tidak punya kaki rekening nasabah")
	}
}

// Profil bank kosong tetap ditandai eksplisit pada dokumen perjanjian kredit yang
// tidak membutuhkan penelusuran rekening.
func TestDocumentService_LoanAgreementMarksMissingBankProfile(t *testing.T) {
	loanID := uuid.New()
	loanRepo := &stubLoanRepo{loan: &domain.Loan{
		ID:                 loanID,
		LoanNumber:         "KRD-2026-00077",
		PrincipalAmount:    decimal.NewFromInt(1000000),
		InterestRateAnnual: decimal.NewFromInt(12),
		TermMonths:         6,
		TotalPayable:       decimal.NewFromInt(1060000),
		MonthlyInstallment: decimal.NewFromInt(176667),
		Status:             domain.LoanStatusApproved,
		CreatedAt:          time.Now().UTC(),
	}}
	docSvc := service.NewDocumentService(nil, nil, loanRepo, nil, &stubBankProfileRepo{}, newTestCipher(t))

	html, err := docSvc.GenerateLoanAgreementHTML(context.Background(), loanID)
	if err != nil {
		t.Fatalf("generate loan agreement: %v", err)
	}
	if !strings.Contains(html, "PROFIL BANK BELUM DIKONFIGURASI") {
		t.Fatal("profil bank yang belum diisi tidak ditandai di surat perjanjian kredit")
	}
}

// Kredit yang tidak ada harus menghasilkan error, bukan dokumen berisi data palsu.
func TestDocumentService_LoanAgreementRejectsMissingLoan(t *testing.T) {
	docSvc := service.NewDocumentService(nil, nil, &stubLoanRepo{}, nil, nil, newTestCipher(t))
	if _, err := docSvc.GenerateLoanAgreementHTML(context.Background(), uuid.New()); err == nil {
		t.Fatal("diharapkan error saat kredit tidak ditemukan")
	}
}

// Nama debitur harus diambil dari ciphertext lalu didekripsi, bukan dari placeholder.
func TestDocumentService_LoanAgreementUsesDecryptedCustomerName(t *testing.T) {
	c := newTestCipher(t)
	encName, err := c.Encrypt("Siti Aminah")
	if err != nil {
		t.Fatalf("mengenkripsi nama: %v", err)
	}

	customerID := uuid.New()
	loanID := uuid.New()
	loanRepo := &stubLoanRepo{
		loan: &domain.Loan{
			ID:                 loanID,
			LoanNumber:         "KRD-2026-00009",
			CustomerID:         customerID,
			PrincipalAmount:    decimal.NewFromInt(1000000),
			InterestRateAnnual: decimal.NewFromInt(12),
			TermMonths:         6,
			TotalPayable:       decimal.NewFromInt(1060000),
			MonthlyInstallment: decimal.NewFromInt(176667),
			Status:             domain.LoanStatusApproved,
			CreatedAt:          time.Now().UTC(),
		},
	}
	custRepo := &stubCustomerRepo{record: &domain.CustomerRecord{ID: customerID, FullNameEnc: encName}}

	docSvc := service.NewDocumentService(nil, nil, loanRepo, custRepo, nil, c)
	loanHTML, err := docSvc.GenerateLoanAgreementHTML(context.Background(), loanID)
	if err != nil {
		t.Fatalf("generate loan agreement: %v", err)
	}
	if !strings.Contains(loanHTML, "Siti Aminah") {
		t.Fatal("nama debitur hasil dekripsi tidak muncul di surat perjanjian")
	}
}
