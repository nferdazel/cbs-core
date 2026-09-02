package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CollectionType string

const (
	CollectionSavingsDeposit CollectionType = "SAVINGS_DEPOSIT"
	CollectionLoanInstallment CollectionType = "LOAN_INSTALLMENT"
)

type MobileCollectionInput struct {
	CollectorID    uuid.UUID       `json:"collector_id"`
	AccountNumber  string          `json:"account_number"`
	LoanID         *uuid.UUID      `json:"loan_id,omitempty"`
	InstallmentNo  *int            `json:"installment_no,omitempty"`
	CollectionType CollectionType `json:"collection_type"`
	Amount         decimal.Decimal `json:"amount"`
	Latitude       float64         `json:"latitude"`
	Longitude      float64         `json:"longitude"`
	Notes          string          `json:"notes"`
	IdempotencyKey string          `json:"idempotency_key"`
}

type MobileCollectionResult struct {
	ReceiptNumber   string          `json:"receipt_number"`
	CollectionType  CollectionType `json:"collection_type"`
	AccountNumber   string          `json:"account_number"`
	Amount          decimal.Decimal `json:"amount"`
	CollectedAt     time.Time       `json:"collected_at"`
	CollectorID     uuid.UUID       `json:"collector_id"`
	Latitude        float64         `json:"latitude"`
	Longitude       float64         `json:"longitude"`
	ReferenceNumber string          `json:"reference_number"`
}

type CollectionService interface {
	ProcessMobileCollection(ctx context.Context, input MobileCollectionInput) (*MobileCollectionResult, error)
}
