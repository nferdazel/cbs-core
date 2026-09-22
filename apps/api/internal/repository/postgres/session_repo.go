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

// GetIdentity mengambil sesi yang masih berlaku beserta pengguna pemiliknya dalam satu
// query. Sesi yang dicabut, kedaluwarsa, atau tidak ada diperlakukan sama: tidak ada
// identitas, sehingga token yang mengacu padanya ditolak.
func (r *SessionRepository) GetIdentity(ctx context.Context, sessionID uuid.UUID) (*domain.SessionIdentity, error) {
	q := `SELECT s.id, s.user_id, u.username, u.role, u.branch_code, u.book, u.is_active, u.locked_until
		FROM staff_sessions s
		JOIN staff_users u ON u.id = s.user_id
		WHERE s.id = $1 AND s.revoked_at IS NULL AND s.expires_at > NOW()`

	var identity domain.SessionIdentity
	var lockedUntil sql.NullTime
	var book sql.NullString
	err := r.db.QueryRowContext(ctx, q, sessionID).Scan(
		&identity.SessionID, &identity.UserID, &identity.Username, &identity.Role,
		&identity.BranchCode, &book, &identity.IsActive, &lockedUntil,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, domain.ErrSessionExpired
	}
	if err != nil {
		return nil, fmt.Errorf("mengambil identitas sesi: %w", err)
	}
	if book.Valid {
		identity.Book = domain.COABook(book.String)
	}
	if lockedUntil.Valid {
		identity.LockedUntil = &lockedUntil.Time
	}

	// Izin & menu dibaca dari DATABASE setiap permintaan, bukan dari kode dan bukan
	// dari token: perubahan pemetaan izin langsung berlaku, dan token lama tidak
	// dapat menahan izin yang sudah dicabut. Bila tabel grup belum ada (database
	// belum dimigrasi), identitas gagal dibaca — lebih baik terlihat daripada
	// diam-diam jatuh ke pemetaan kode.
	permissions, err := r.effectivePermissions(ctx, identity.UserID, identity.Role)
	if err != nil {
		return nil, err
	}
	identity.Permissions = permissions
	menus, err := r.allowedMenus(ctx, identity.UserID, identity.Role)
	if err != nil {
		return nil, err
	}
	identity.Menus = menus
	return &identity, nil
}

// effectivePermissions menghitung izin efektif pengguna: izin grup ROLE_<peran>
// digabung keanggotaan grup lain. Semantiknya ADITIF, sehingga menambah grup tidak
// pernah mencabut akses; pengguna tanpa keanggotaan tetap memperoleh izin perannya
// lewat cabang role group.
func (r *SessionRepository) effectivePermissions(ctx context.Context, userID uuid.UUID, role domain.StaffRole) ([]domain.Permission, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT gp.permission
		FROM group_permissions gp
		JOIN user_groups g ON g.id = gp.group_id
		WHERE g.code = 'ROLE_' || $2
		UNION
		SELECT DISTINCT gp.permission
		FROM user_group_members m
		JOIN group_permissions gp ON gp.group_id = m.group_id
		WHERE m.user_id = $1
		ORDER BY 1`, userID, string(role))
	if err != nil {
		return nil, fmt.Errorf("membaca izin efektif: %w", err)
	}
	defer rows.Close()

	perms := []domain.Permission{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		perms = append(perms, domain.Permission(p))
	}
	return perms, rows.Err()
}

// allowedMenus mengembalikan kunci menu yang terbuka oleh izin efektif pengguna.
// Menu tanpa pemetaan izin selalu tampil (mis. beranda). Penyaringan buku dilakukan
// handler memakai cakupan instalasi yang hanya diketahui middleware.
func (r *SessionRepository) allowedMenus(ctx context.Context, userID uuid.UUID, role domain.StaffRole) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH effective AS (
			SELECT gp.permission
			FROM group_permissions gp
			JOIN user_groups g ON g.id = gp.group_id
			WHERE g.code = 'ROLE_' || $2
			UNION
			SELECT gp.permission
			FROM user_group_members m
			JOIN group_permissions gp ON gp.group_id = m.group_id
			WHERE m.user_id = $1
		)
		SELECT m.menu_key
		FROM menu_catalog m
		WHERE NOT EXISTS (SELECT 1 FROM menu_permissions mp WHERE mp.menu_key = m.menu_key)
		   OR EXISTS (SELECT 1 FROM menu_permissions mp
		              WHERE mp.menu_key = m.menu_key
		                AND mp.permission IN (SELECT permission FROM effective))
		ORDER BY m.menu_key ASC`, userID, string(role))
	if err != nil {
		return nil, fmt.Errorf("membaca menu: %w", err)
	}
	defer rows.Close()

	menus := []string{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		menus = append(menus, key)
	}
	return menus, rows.Err()
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
