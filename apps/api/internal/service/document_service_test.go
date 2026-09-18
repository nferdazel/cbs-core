package service_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
