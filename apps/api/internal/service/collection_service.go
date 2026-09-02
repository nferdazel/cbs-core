package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

type collectionService struct {
	ledgerSvc domain.LedgerService
	loanSvc   domain.LoanService
}

func NewCollectionService(ledgerSvc domain.LedgerService, loanSvc domain.LoanService) domain.CollectionService {
	return &collectionService{
		ledgerSvc: ledgerSvc,
		loanSvc:   loanSvc,
	}
}

func (s *collectionService) ProcessMobileCollection(ctx context.Context, input domain.MobileCollectionInput) (*domain.MobileCollectionResult, error) {
	receiptNo := fmt.Sprintf("MBL-%s-%05d", time.Now().Format("20060102"), time.Now().Nanosecond()%100000)
	var refNo string

	switch input.CollectionType {
	case domain.CollectionSavingsDeposit:
		desc := fmt.Sprintf("Mobile Collector Deposit (%s) - Notes: %s", receiptNo, input.Notes)
		entry, err := s.ledgerSvc.Deposit(ctx, domain.DepositRequest{
			AccountNumber:  input.AccountNumber,
			Amount:         input.Amount,
			Currency:       "IDR",
			Description:    desc,
			IdempotencyKey: input.IdempotencyKey,
			CreatedBy:      input.CollectorID.String(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to process mobile deposit: %w", err)
		}
		refNo = entry.ReferenceNumber

	case domain.CollectionLoanInstallment:
		if input.LoanID == nil || input.InstallmentNo == nil {
			return nil, fmt.Errorf("loan_id and installment_no are required for loan installment collection")
		}
		_, err := s.loanSvc.PayInstallment(ctx, domain.PayInstallmentInput{
			LoanID:        *input.LoanID,
			InstallmentNo: *input.InstallmentNo,
			Amount:        input.Amount,
		}, input.CollectorID)
		if err != nil {
			return nil, fmt.Errorf("failed to process mobile loan payment: %w", err)
		}
		refNo = receiptNo

	default:
		return nil, fmt.Errorf("invalid collection_type")
	}

	return &domain.MobileCollectionResult{
		ReceiptNumber:   receiptNo,
		CollectionType:  input.CollectionType,
		AccountNumber:   input.AccountNumber,
		Amount:          input.Amount,
		CollectedAt:     time.Now().UTC(),
		CollectorID:     input.CollectorID,
		Latitude:        input.Latitude,
		Longitude:       input.Longitude,
		ReferenceNumber: refNo,
	}, nil
}
