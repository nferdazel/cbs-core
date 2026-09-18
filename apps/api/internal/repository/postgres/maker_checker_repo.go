package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type MakerCheckerRepository struct {
	db *sql.DB
}

func NewMakerCheckerRepository(db *sql.DB) *MakerCheckerRepository {
	return &MakerCheckerRepository{db: db}
}

func (r *MakerCheckerRepository) CreateTx(ctx context.Context, tx any, req *domain.MakerCheckerRequest) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("maker-checker: transaksi tidak valid")
	}
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return fmt.Errorf("maker-checker: payload tidak bisa diserialkan: %w", err)
	}
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO maker_checker_requests
			(id, action_type, payload, status, maker_id, maker_notes, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		req.ID, req.ActionType, payload, req.Status, req.MakerID,
		nullIfEmpty(req.MakerNotes), req.CreatedAt, req.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("maker-checker: menyimpan permintaan: %w", err)
	}
	return nil
}

func (r *MakerCheckerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.MakerCheckerRequest, error) {
	var req domain.MakerCheckerRequest
	var payload []byte
	var checkerID sql.NullString
	var reviewedAt sql.NullTime

	err := r.db.QueryRowContext(ctx, `
		SELECT id, action_type, payload, status, maker_id, checker_id,
		       COALESCE(maker_notes, ''), COALESCE(checker_notes, ''), reviewed_at, created_at, updated_at
		FROM maker_checker_requests WHERE id = $1`, id,
	).Scan(
		&req.ID, &req.ActionType, &payload, &req.Status, &req.MakerID, &checkerID,
		&req.MakerNotes, &req.CheckerNotes, &reviewedAt, &req.CreatedAt, &req.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrMakerCheckerNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &req.Payload)
	}
	if checkerID.Valid {
		req.CheckerID = &checkerID.String
	}
	if reviewedAt.Valid {
		req.ReviewedAt = &reviewedAt.Time
	}
	return &req, nil
}

func (r *MakerCheckerRepository) UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status domain.MakerCheckerStatus, checkerID, notes string) error {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("maker-checker: transaksi tidak valid")
	}
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE maker_checker_requests
		SET status = $1, checker_id = $2, checker_notes = $3, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $4 AND status = 'PENDING'`,
		status, checkerID, nullIfEmpty(notes), id,
	)
	if err != nil {
		return fmt.Errorf("maker-checker: mengubah status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return domain.ErrMakerCheckerNotPending
	}
	return nil
}

func (r *MakerCheckerRepository) ListPending(ctx context.Context) ([]domain.MakerCheckerRequest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, action_type, payload, status, maker_id, checker_id,
		       COALESCE(maker_notes, ''), COALESCE(checker_notes, ''), reviewed_at, created_at, updated_at
		FROM maker_checker_requests
		WHERE status = 'PENDING'
		ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.MakerCheckerRequest
	for rows.Next() {
		var req domain.MakerCheckerRequest
		var payload []byte
		var checkerID sql.NullString
		var reviewedAt sql.NullTime
		if err := rows.Scan(
			&req.ID, &req.ActionType, &payload, &req.Status, &req.MakerID, &checkerID,
			&req.MakerNotes, &req.CheckerNotes, &reviewedAt, &req.CreatedAt, &req.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &req.Payload)
		}
		if checkerID.Valid {
			req.CheckerID = &checkerID.String
		}
		if reviewedAt.Valid {
			req.ReviewedAt = &reviewedAt.Time
		}
		list = append(list, req)
	}
	return list, rows.Err()
}

var _ domain.MakerCheckerRepository = (*MakerCheckerRepository)(nil)
