package service_test

import (
	"context"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

func TestDocumentService_HTMLGenerators(t *testing.T) {
	docSvc := service.NewDocumentService(nil, nil, &stubLoanRepo{}, nil)

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
	loanHTML, err := docSvc.GenerateLoanAgreementHTML(context.Background(), uuid.New())
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
