package service

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
)

type branchService struct {
	repo domain.BranchRepository
}

func NewBranchService(repo domain.BranchRepository) domain.BranchService {
	return &branchService{repo: repo}
}

func (s *branchService) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	return s.repo.List(ctx)
}
