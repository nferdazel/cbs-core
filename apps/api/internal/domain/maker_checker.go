package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrMakerCheckerNotFound   = errors.New("permintaan maker-checker tidak ditemukan")
	ErrMakerCheckerNotPending = errors.New("permintaan maker-checker sudah diproses")
	ErrCannotSelfApprove      = errors.New("pembuat permintaan tidak boleh menyetujui permintaannya sendiri")
)

type MakerCheckerStatus string

const (
	MakerCheckerPending  MakerCheckerStatus = "PENDING"
	MakerCheckerApproved MakerCheckerStatus = "APPROVED"
	MakerCheckerRejected MakerCheckerStatus = "REJECTED"
)

// MakerCheckerRequest adalah permintaan persetujuan berjenjang. Identitas pembuat dan
// pemeriksa disimpan sebagai string uuid agar sesuai kolom maker_id/checker_id.
type MakerCheckerRequest struct {
	ID           uuid.UUID
	ActionType   string
	Payload      map[string]any
	Status       MakerCheckerStatus
	MakerID      string
	CheckerID    *string
	MakerNotes   string
	CheckerNotes string
	ReviewedAt   *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateMakerCheckerInput struct {
	ActionType string
	Amount     decimal.Decimal
	Payload    map[string]any
	Notes      string
}

type MakerCheckerRepository interface {
	CreateTx(ctx context.Context, tx any, req *MakerCheckerRequest) error
	GetByID(ctx context.Context, id uuid.UUID) (*MakerCheckerRequest, error)
	UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status MakerCheckerStatus, checkerID, notes string) error
	ListPending(ctx context.Context) ([]MakerCheckerRequest, error)
}

type MakerCheckerService interface {
	CreateRequest(ctx context.Context, input CreateMakerCheckerInput, actor Actor) (*MakerCheckerRequest, error)
	Approve(ctx context.Context, id uuid.UUID, actor Actor, notes string) error
	Reject(ctx context.Context, id uuid.UUID, actor Actor, notes string) error
	ListPending(ctx context.Context) ([]MakerCheckerRequest, error)
}
