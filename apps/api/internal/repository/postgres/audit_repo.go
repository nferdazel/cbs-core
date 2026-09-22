package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
			ip_address, changes, metadata, staff_user_id, staff_role,
			request_id, user_agent, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := exec.ExecContext(ctx, query,
		actorLabel,
		event.ActorRole,
		event.Action,
		event.ResourceType,
		event.ResourceID,
		event.IPAddress,
		event.ChangesJSON(),
		event.MetadataJSON(),
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
// AuditLogPageSize membatasi jumlah baris yang dapat diminta dari audit log dalam satu
// permintaan. Batas atas ini menjaga agar laporan audit tidak menarik seluruh tabel.
const (
	AuditLogDefaultPageSize = 50
	AuditLogMaxPageSize     = 200
)

// Query membaca audit log dengan filter opsional, terbaru lebih dulu. Semua nilai filter
// dikirim sebagai parameter; tidak ada nilai yang dirangkai ke dalam SQL.
func (r *AuditRepository) Query(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditEvent, error) {
	limit := filter.Limit
	if limit < 1 || limit > AuditLogMaxPageSize {
		limit = AuditLogDefaultPageSize
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var conditions []string
	var args []any
	// "\$?" diisi nomor parameter, sehingga satu nilai dapat dipakai di beberapa kolom.
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.ReplaceAll(condition, "$?", fmt.Sprintf("$%d", len(args))))
	}

	if filter.ResourceType != "" {
		add("resource_type = $?", filter.ResourceType)
	}
	if filter.ResourceID != "" {
		add("resource_id = $?", filter.ResourceID)
	}
	if filter.Actor != "" {
		add("(actor_id = $? OR staff_user_id::text = $?)", filter.Actor)
	}
	if filter.Action != "" {
		add("action = $?", filter.Action)
	}
	if filter.From != nil {
		add("created_at >= $?", *filter.From)
	}
	if filter.To != nil {
		add("created_at < $?", *filter.To)
	}

	query := `SELECT actor_id, COALESCE(actor_role, ''), action, resource_type, resource_id,
		       COALESCE(ip_address, ''), COALESCE(changes, '{}'::jsonb), COALESCE(metadata, '{}'::jsonb),
		       COALESCE(request_id, ''), COALESCE(user_agent, ''), created_at
		FROM audit_logs`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	// id ikut diurutkan agar urutan stabil untuk aksi yang tercatat pada detik yang sama.
	query += fmt.Sprintf(" ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	events := make([]domain.AuditEvent, 0, limit)
	for rows.Next() {
		var e domain.AuditEvent
		var changes, metadata []byte
		if err := rows.Scan(
			&e.ActorID, &e.ActorRole, &e.Action, &e.ResourceType, &e.ResourceID,
			&e.IPAddress, &changes, &metadata, &e.RequestID, &e.UserAgent, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			_ = json.Unmarshal(changes, &e.Changes)
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &e.Metadata)
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (r *AuditRepository) List(ctx context.Context, resourceType, resourceID string, limit int) ([]domain.AuditEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT actor_id, COALESCE(actor_role, ''), action, resource_type, resource_id,
		       COALESCE(ip_address, ''), COALESCE(changes, '{}'::jsonb), COALESCE(metadata, '{}'::jsonb),
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
	defer func() { _ = rows.Close() }()

	var events []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var changes, metadata []byte
		if err := rows.Scan(
			&e.ActorID, &e.ActorRole, &e.Action, &e.ResourceType, &e.ResourceID,
			&e.IPAddress, &changes, &metadata, &e.RequestID, &e.UserAgent, &e.CreatedAt,
		); err != nil {
			return nil, err
		}
		if len(changes) > 0 {
			_ = json.Unmarshal(changes, &e.Changes)
		}
		if len(metadata) > 0 {
			_ = json.Unmarshal(metadata, &e.Metadata)
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
