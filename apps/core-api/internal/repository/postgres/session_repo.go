package postgres

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// HashToken returns the SHA-256 hex hash of a raw token string.
func HashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", h)
}

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, s *domain.StaffSession) error {
	q := `INSERT INTO staff_sessions
		(id, user_id, refresh_token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := r.db.ExecContext(ctx, q,
		s.ID, s.UserID, s.RefreshTokenHash, s.IPAddress, s.UserAgent, s.ExpiresAt, s.CreatedAt,
	)
	return err
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, hash string) (*domain.StaffSession, error) {
	q := `SELECT id, user_id, refresh_token_hash, ip_address, user_agent, expires_at, revoked_at, created_at
		FROM staff_sessions WHERE refresh_token_hash = $1`
	var s domain.StaffSession
	var revokedAt sql.NullTime
	err := r.db.QueryRowContext(ctx, q, hash).Scan(
		&s.ID, &s.UserID, &s.RefreshTokenHash, &s.IPAddress, &s.UserAgent,
		&s.ExpiresAt, &revokedAt, &s.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("session not found")
	}
	if err != nil {
		return nil, err
	}
	if revokedAt.Valid {
		s.RevokedAt = &revokedAt.Time
	}
	return &s, nil
}

func (r *SessionRepository) RevokeByID(ctx context.Context, sessionID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_sessions SET revoked_at=NOW() WHERE id=$1", sessionID)
	return err
}

func (r *SessionRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_sessions SET revoked_at=NOW() WHERE user_id=$1 AND revoked_at IS NULL", userID)
	return err
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		"DELETE FROM staff_sessions WHERE expires_at < NOW()")
	return err
}
