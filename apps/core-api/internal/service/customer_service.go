package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type customerService struct {
	repo domain.CustomerRepository
}

func NewCustomerService(repo domain.CustomerRepository) domain.CustomerService {
	return &customerService{repo: repo}
}

func (s *customerService) RegisterCustomer(ctx context.Context, input domain.CreateCustomerInput) (*domain.Customer, error) {
	cifNumber := fmt.Sprintf("CIF%s%04d", time.Now().Format("20060102"), time.Now().Nanosecond()%10000)

	customer := &domain.Customer{
		ID:           uuid.New(),
		CIFNumber:    cifNumber,
		FullName:     input.FullName,
		IDCardNumber: input.IDCardNumber,
		Email:        input.Email,
		PhoneNumber:  input.PhoneNumber,
		Address:      input.Address,
		Status:       domain.CustomerStatusActive,
		Metadata:     input.Metadata,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, customer); err != nil {
		return nil, err
	}

	return customer, nil
}

func (s *customerService) GetCustomer(ctx context.Context, id uuid.UUID) (*domain.Customer, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *customerService) ListCustomers(ctx context.Context, page, pageSize int) ([]domain.Customer, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.repo.List(ctx, pageSize, offset)
}
