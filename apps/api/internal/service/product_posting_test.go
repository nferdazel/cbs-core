package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

const (
	controlSavingsCOA = "20100"
	customerAccountNo = "0011010000000014"
)

func disbursementProduct() *stubProductRepo {
	return &stubProductRepo{
		product: &domain.BankingProduct{ID: uuid.New(), Code: "KRD-FLAT"},
		rules: map[domain.PostingEvent][]domain.JournalMappingRule{
			domain.EventLoanDisbursement: {
				{Event: domain.EventLoanDisbursement, Direction: domain.DirectionDebit, COACode: "10301", AmountSource: domain.AmountPrincipal},
				{Event: domain.EventLoanDisbursement, Direction: domain.DirectionCredit, COACode: controlSavingsCOA, AmountSource: domain.AmountPrincipal},
			},
		},
	}
}

func postDisbursement(t *testing.T, meta PostingMeta) domain.PostingRequest {
	t.Helper()
	products := disbursementProduct()
	posting := &stubPosting{}
	poster := NewProductPoster(products, stubResolver{}, posting)

	amount := decimal.NewFromInt(1000)
	if _, err := poster.PostEventTx(context.Background(), nil, products.product, domain.EventLoanDisbursement,
		Amounts{Principal: amount, Total: amount}, meta); err != nil {
		t.Fatalf("posting gagal: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jumlah jurnal %d, ingin 1", len(posting.requests))
	}
	return posting.requests[0]
}

// Tanpa override, kaki kredit tetap menunjuk akun kontrol sesuai pemetaan produk.
func TestProductPosterWithoutOverrideUsesControlAccount(t *testing.T) {
	req := postDisbursement(t, PostingMeta{IdempotencyKey: "DISB-1"})

	var credit string
	for _, line := range req.Lines {
		if line.Direction == domain.DirectionCredit {
			credit = line.AccountNumber
		}
	}
	if credit != controlSavingsCOA {
		t.Fatalf("kredit ke %q, ingin akun kontrol %q", credit, controlSavingsCOA)
	}
}

// Dengan override, kaki kredit harus menunjuk rekening nasabah agar saldonya
// bergerak. Kaki lain tidak boleh ikut berubah.
func TestProductPosterAccountOverrideReplacesControlAccount(t *testing.T) {
	req := postDisbursement(t, PostingMeta{
		IdempotencyKey:   "DISB-2",
		AccountOverrides: map[string]string{controlSavingsCOA: customerAccountNo},
	})

	var debit, credit string
	for _, line := range req.Lines {
		switch line.Direction {
		case domain.DirectionDebit:
			debit = line.AccountNumber
		case domain.DirectionCredit:
			credit = line.AccountNumber
		}
	}
	if credit != customerAccountNo {
		t.Fatalf("kredit ke %q, ingin rekening nasabah %q", credit, customerAccountNo)
	}
	if debit != "10301" {
		t.Fatalf("debit ke %q, ingin tetap 10301", debit)
	}
}
