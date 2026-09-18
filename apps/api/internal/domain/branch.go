package domain

import (
	"context"

	"github.com/google/uuid"
)

type Branch struct {
	ID           uuid.UUID `json:"id"`
	Code         string    `json:"code"`
	Name         string    `json:"name"`
	Address      string    `json:"address,omitempty"`
	Phone        string    `json:"phone,omitempty"`
	IsHeadOffice bool      `json:"is_head_office"`
	IsActive     bool      `json:"is_active"`
}

type BranchRepository interface {
	List(ctx context.Context) ([]Branch, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Branch, error)
	GetByCode(ctx context.Context, code string) (*Branch, error)
}

type BranchService interface {
	ListBranches(ctx context.Context) ([]Branch, error)
}
