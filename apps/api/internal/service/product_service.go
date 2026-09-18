package service

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type productService struct {
	repo domain.ProductRepository
}

func NewProductService(repo domain.ProductRepository) domain.ProductService {
	return &productService{repo: repo}
}

func (s *productService) ListProducts(ctx context.Context) ([]domain.BankingProduct, error) {
	return s.repo.List(ctx)
}

func (s *productService) GetProduct(ctx context.Context, id uuid.UUID) (*domain.BankingProduct, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *productService) GetMapping(ctx context.Context, productID uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return s.repo.GetMapping(ctx, productID, event)
}
