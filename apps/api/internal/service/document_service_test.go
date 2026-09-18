package service_test

import (
	"context"
	"database/sql"
	"encoding/base64"
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

func (s *stubCustomerRepo) List(ctx context.Context, limit, offset int) ([]domain.CustomerRecord, int, error) {
	return nil, 0, nil
}

func (s *stubCustomerRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.CustomerStatus) error {
	return nil
}

var _ domain.CustomerRepository = (*stubCustomerRepo)(nil)

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
	docSvc := service.NewDocumentService(nil, nil, repo, nil)

	// 1. Test Deposit Slip HTML
	depHTML, err := docSvc.GenerateDepositSlipHTML(context.Background(), "DEP-20260902-001")
	if err != nil {
		t.Fatalf("unexpected error generating deposit slip: %v", err)
	}
	if !strings.Contains(depHTML, "SLIP SETORAN TUNAI TELLER") {
		t.Fatal("expected deposit slip HTML to contain header title")
	}

	// 2. Test Withdrawal Slip HTML
	wthHTML, err := docSvc.GenerateWithdrawalSlipHTML(context.Background(), "WTH-20260902-001")
	if err != nil {
		t.Fatalf("unexpected error generating withdrawal slip: %v", err)
	}
	if !strings.Contains(wthHTML, "SLIP PENARIKAN TUNAI TELLER") {
		t.Fatal("expected withdrawal slip HTML to contain header title")
	}

	// 3. Test Loan Agreement HTML
	loanHTML, err := docSvc.GenerateLoanAgreementHTML(context.Background(), loanID)
	if err != nil {
		t.Fatalf("unexpected error generating loan agreement: %v", err)
	}
	if !strings.Contains(loanHTML, "SURAT PERJANJIAN KREDIT / AKAD PEMBIAYAAN") {
		t.Fatal("expected loan agreement HTML to contain title")
	}

	// 4. Test Thermal Receipt Text
	receiptText, err := docSvc.GenerateThermalReceiptText(context.Background(), "MBL-20260902-00001")
	if err != nil {
		t.Fatalf("unexpected error generating thermal receipt: %v", err)
	}
	if !strings.Contains(receiptText, "STRUK BUKTI PENERIMAAN KAS") {
		t.Fatal("expected thermal receipt to contain header text")
	}
}

// Kredit yang tidak ada harus menghasilkan error, bukan dokumen berisi data palsu.
func TestDocumentService_LoanAgreementRejectsMissingLoan(t *testing.T) {
	docSvc := service.NewDocumentService(nil, nil, &stubLoanRepo{}, nil)
	if _, err := docSvc.GenerateLoanAgreementHTML(context.Background(), uuid.New()); err == nil {
		t.Fatal("diharapkan error saat kredit tidak ditemukan")
	}
}

// Nama debitur harus diambil dari ciphertext lalu didekripsi, bukan dari placeholder.
func TestDocumentService_LoanAgreementUsesDecryptedCustomerName(t *testing.T) {
	c, err := crypto.NewCipher("k1", base64.StdEncoding.EncodeToString(make([]byte, 32)), nil)
	if err != nil {
		t.Fatalf("membangun cipher: %v", err)
	}
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

	docSvc := service.NewDocumentService(nil, nil, loanRepo, custRepo, c)
	loanHTML, err := docSvc.GenerateLoanAgreementHTML(context.Background(), loanID)
	if err != nil {
		t.Fatalf("generate loan agreement: %v", err)
	}
	if !strings.Contains(loanHTML, "Siti Aminah") {
		t.Fatal("nama debitur hasil dekripsi tidak muncul di surat perjanjian")
	}
}
