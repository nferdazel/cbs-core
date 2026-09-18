package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// defaultMakerCheckerThreshold dipakai bila ambang per jenis aksi belum diisi di
// system_config. Ini tempat sementara: nilai produksi wajib diisi saat provisioning.
var defaultMakerCheckerThreshold = decimal.NewFromInt(50_000_000)

type makerCheckerService struct {
	db        *sql.DB
	repo      domain.MakerCheckerRepository
	auditRepo domain.AuditRepository
	config    domain.SystemConfigService
	executors domain.MakerCheckerExecutor
}

func NewMakerCheckerService(
	db *sql.DB,
	repo domain.MakerCheckerRepository,
	auditRepo domain.AuditRepository,
	config domain.SystemConfigService,
	executors domain.MakerCheckerExecutor,
) domain.MakerCheckerService {
	return &makerCheckerService{db: db, repo: repo, auditRepo: auditRepo, config: config, executors: executors}
}

// Threshold membaca ambang persetujuan untuk satu jenis aksi dari system_config.
// Key: maker_checker.<action_type>.<threshold>, mis. maker_checker.deposit.threshold.
func (s *makerCheckerService) Threshold(ctx context.Context, actionType string) decimal.Decimal {
	key := "maker_checker." + strings.ToLower(strings.TrimSpace(actionType)) + ".threshold"
	return s.config.GetDecimal(ctx, key, defaultMakerCheckerThreshold)
}

func (s *makerCheckerService) CreateRequest(ctx context.Context, input domain.CreateMakerCheckerInput, actor domain.Actor) (*domain.MakerCheckerRequest, error) {
	if strings.TrimSpace(input.ActionType) == "" {
		return nil, errors.New("jenis aksi maker-checker wajib diisi")
	}

	threshold := s.Threshold(ctx, input.ActionType)
	requiresApproval := input.Amount.IsPositive() && input.Amount.GreaterThanOrEqual(threshold)

	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	payload["amount"] = input.Amount.String()
	payload["threshold"] = threshold.String()
	payload["requires_approval"] = requiresApproval

	now := time.Now().UTC()
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		ActionType: input.ActionType,
		Payload:    payload,
		Status:     domain.MakerCheckerPending,
		MakerID:    actor.UserID.String(),
		MakerNotes: input.Notes,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := s.repo.CreateTx(ctx, tx, req); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "CREATE_MAKER_CHECKER", "maker_checker", req.ID.String(), map[string]any{
		"action_type": req.ActionType,
		"amount":      input.Amount.String(),
		"status":      string(req.Status),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *makerCheckerService) Approve(ctx context.Context, id uuid.UUID, actor domain.Actor, notes string) error {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if req.Status != domain.MakerCheckerPending {
		return domain.ErrMakerCheckerNotPending
	}
	// Pemisahan tugas: pembuat tidak boleh menjadi pemeriksa permintaannya sendiri.
	if req.MakerID == actor.UserID.String() {
		return domain.ErrCannotSelfApprove
	}
	return s.process(ctx, req, actor, domain.MakerCheckerApproved, notes, "APPROVE_MAKER_CHECKER")
}

func (s *makerCheckerService) Reject(ctx context.Context, id uuid.UUID, actor domain.Actor, notes string) error {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if req.Status != domain.MakerCheckerPending {
		return domain.ErrMakerCheckerNotPending
	}
	return s.process(ctx, req, actor, domain.MakerCheckerRejected, notes, "REJECT_MAKER_CHECKER")
}

// process mengeksekusi transaksi yang disetujui sekaligus mencatat keputusannya.
// Status, posting jurnal, dan audit berada dalam SATU transaksi: keputusan tidak
// pernah tercatat tanpa efeknya, dan efek tidak pernah terjadi tanpa jejak keputusan.
func (s *makerCheckerService) process(ctx context.Context, req *domain.MakerCheckerRequest, actor domain.Actor, status domain.MakerCheckerStatus, notes, action string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Kunci permintaan agar dua pemeriksa tidak memproses bersamaan.
	if err := s.repo.UpdateStatusTx(ctx, tx, req.ID, status, actor.UserID.String(), notes); err != nil {
		return err
	}

	// Hanya persetujuan yang mengeksekusi transaksi; penolakan berhenti di status.
	if status == domain.MakerCheckerApproved {
		if s.executors == nil {
			return domain.ErrNoExecutorForAction
		}
		if err := s.executors.ExecuteApproved(ctx, tx, req.ActionType, req.Payload, actor); err != nil {
			return err
		}
	}

	if err := writeAudit(ctx, s.auditRepo, tx, actor, action, "maker_checker", req.ID.String(), map[string]any{
		"action_type": req.ActionType,
		"status":      string(status),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *makerCheckerService) ListPending(ctx context.Context) ([]domain.MakerCheckerRequest, error) {
	return s.repo.ListPending(ctx)
}
