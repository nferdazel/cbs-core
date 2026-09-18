package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
)

// AuditRepository menulis audit log. Hanya ada operasi tulis dan baca; tidak ada
// update maupun delete, karena audit log adalah bukti yang tidak boleh berubah.
type AuditRepository struct {
	db *sql.DB
}

func NewAuditRepository(db *sql.DB) *AuditRepository {
	return &AuditRepository{db: db}
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// Write menyimpan event audit. Bila tx diberikan, penulisan ikut transaksi bisnis
// sehingga audit sukses hanya bila aksinya sukses.
func (r *AuditRepository) Write(ctx context.Context, tx any, event domain.AuditEvent) error {
	var exec execer = r.db
	if tx != nil {
		sqlTx, ok := tx.(*sql.Tx)
		if !ok {
			return fmt.Errorf("konteks transaksi audit tidak valid")
		}
		exec = sqlTx
	}

	// actor_id adalah kolom NOT NULL dari skema awal: dipakai sebagai nama tampilan
	// pelaku. staff_user_id menyimpan uuid-nya untuk relasi yang bisa di-join.
	actorLabel := event.ActorUsername
	if actorLabel == "" {
		actorLabel = event.ActorID
	}
	if actorLabel == "" {
		actorLabel = "system"
	}

	var staffUserID any
	if event.ActorID != "" {
		staffUserID = event.ActorID
	}

	query := `
		INSERT INTO audit_logs (
			actor_id, actor_role, action, resource_type, resource_id,
			ip_address, changes, staff_user_id, staff_role,
			request_id, user_agent, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err := exec.ExecContext(ctx, query,
		actorLabel,
		event.ActorRole,
		event.Action,
		event.ResourceType,
		event.ResourceID,
		event.IPAddress,
		event.ChangesJSON(),
		staffUserID,
		event.ActorRole,
		nullIfEmpty(event.RequestID),
		nullIfEmpty(event.UserAgent),
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("gagal menulis audit log: %w", err)
	}
	return nil
}

// List mengambil audit log untuk satu objek, terbaru lebih dahulu.
func (r *AuditRepository) List(ctx context.Context, resourceType, resourceID string, limit int) ([]domain.AuditEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT actor_id, COALESCE(actor_role, ''), action, resource_type, resource_id,
		       COALESCE(ip_address, ''), COALESCE(changes, '{}'::jsonb),
		       COALESCE(request_id, ''), COALESCE(user_agent, ''), created_at
		FROM audit_logs
		WHERE resource_type = $1 AND resource_id = $2
		ORDER BY created_at DESC
		LIMIT $3
	`
	rows, err := r.db.QueryContext(ctx, query, resourceType, resourceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var changes []byte
		if err := rows.Scan(
			&e.ActorID, &e.ActorRole, &e.Action, &e.ResourceType, &e.ResourceID,
			&e.IPAddress, &changes, &e.RequestID, &e.UserAgent, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			_ = json.Unmarshal(changes, &e.Changes)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
