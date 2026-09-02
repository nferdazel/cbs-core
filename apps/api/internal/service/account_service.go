package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type accountService struct {
	accountRepo  domain.AccountRepository
	customerRepo domain.CustomerRepository
}

func NewAccountService(accountRepo domain.AccountRepository, customerRepo domain.CustomerRepository) domain.AccountService {
	return &accountService{
		accountRepo:  accountRepo,
		customerRepo: customerRepo,
	}
}

func (s *accountService) OpenAccount(ctx context.Context, input domain.OpenAccountInput) (*domain.Account, error) {
	// 1. Verify customer exists
	customer, err := s.customerRepo.GetByID(ctx, input.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("invalid customer: %w", err)
	}

	if customer.Status != domain.CustomerStatusActive {
		return nil, domain.ErrAccountInactive
	}

	// Generate 12-digit account number (prefix 10 + timestamp/random)
	accountNumber := fmt.Sprintf("10%s%04d", time.Now().Format("06010215"), time.Now().Nanosecond()%10000)

	account := &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    accountNumber,
		CustomerID:       &customer.ID,
		CustomerName:     customer.FullName,
		COAID:            input.COAID,
		AccountType:      input.AccountType,
		Currency:         input.Currency,
		Balance:          decimal.Zero,
		AvailableBalance: decimal.Zero,
		HoldBalance:      decimal.Zero,
		Status:           domain.AccountStatusActive,
		Version:          1,
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}

	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, err
	}

	return account, nil
}

func (s *accountService) GetAccountByNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	return s.accountRepo.GetByNumber(ctx, accountNumber)
}

func (s *accountService) ListAccounts(ctx context.Context, page, pageSize int) ([]domain.Account, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.accountRepo.ListAll(ctx, pageSize, offset)
}
