package domain_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestValidateDoubleEntry(t *testing.T) {
	accA := uuid.New()
	accB := uuid.New()
	entryID := uuid.New()

	t.Run("Balanced Entry (Debit == Credit) should pass", func(t *testing.T) {
		lines := []domain.JournalLine{
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      accA,
				Direction:      domain.DirectionDebit,
				Amount:         decimal.NewFromInt(500000), // 500,000 IDR
			},
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      accB,
				Direction:      domain.DirectionCredit,
				Amount:         decimal.NewFromInt(500000), // 500,000 IDR
			},
		}

		err := domain.ValidateDoubleEntry(lines)
		if err != nil {
			t.Fatalf("expected valid entry, got: %v", err)
		}
	})

	t.Run("Unbalanced Entry (Debit != Credit) must fail", func(t *testing.T) {
		lines := []domain.JournalLine{
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      accA,
				Direction:      domain.DirectionDebit,
				Amount:         decimal.NewFromInt(500000),
			},
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      accB,
				Direction:      domain.DirectionCredit,
				Amount:         decimal.NewFromInt(490000), // Mismatch by 10,000!
			},
		}

		err := domain.ValidateDoubleEntry(lines)
		if err != domain.ErrLedgerUnbalanced {
			t.Fatalf("expected ErrLedgerUnbalanced, got: %v", err)
		}
	})

	t.Run("Negative or Zero Amount must fail", func(t *testing.T) {
		lines := []domain.JournalLine{
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      accA,
				Direction:      domain.DirectionDebit,
				Amount:         decimal.NewFromInt(-100000),
			},
			{
				ID:             uuid.New(),
				JournalEntryID: entryID,
				AccountID:      accB,
				Direction:      domain.DirectionCredit,
				Amount:         decimal.NewFromInt(-100000),
			},
		}

		err := domain.ValidateDoubleEntry(lines)
		if err != domain.ErrInvalidAmount {
			t.Fatalf("expected ErrInvalidAmount, got: %v", err)
		}
	})
}
