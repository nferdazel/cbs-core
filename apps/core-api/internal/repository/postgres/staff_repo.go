package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type StaffRepository struct {
	db *sql.DB
}

func NewStaffRepository(db *sql.DB) *StaffRepository {
	return &StaffRepository{db: db}
}

func scanStaffUser(row interface{ Scan(...any) error }) (*domain.StaffUser, error) {
	var u domain.StaffUser
	var createdBy sql.NullString
	var lockedUntil sql.NullTime
	var lastLoginAt sql.NullTime

	err := row.Scan(
		&u.ID, &u.EmployeeID, &u.Username, &u.FullName, &u.Email,
		&u.PasswordHash, &u.Role, &u.BranchCode, &u.IsActive,
		&lastLoginAt, &u.PasswordChangedAt,
		&u.FailedLoginCount, &lockedUntil,
		&createdBy, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastLoginAt.Valid {
		u.LastLoginAt = &lastLoginAt.Time
	}
	if lockedUntil.Valid {
		u.LockedUntil = &lockedUntil.Time
	}
	if createdBy.Valid {
		id, _ := uuid.Parse(createdBy.String)
		u.CreatedBy = &id
	}
	return &u, nil
}

func (r *StaffRepository) Create(ctx context.Context, u *domain.StaffUser) error {
	q := `INSERT INTO staff_users
		(id, employee_id, username, full_name, email, password_hash, role, branch_code,
		 is_active, password_changed_at, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`
	_, err := r.db.ExecContext(ctx, q,
		u.ID, u.EmployeeID, u.Username, u.FullName, u.Email, u.PasswordHash,
		u.Role, u.BranchCode, u.IsActive, u.PasswordChangedAt, u.CreatedBy, u.CreatedAt, u.UpdatedAt,
	)
	return err
}

func (r *StaffRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.StaffUser, error) {
	q := `SELECT id, employee_id, username, full_name, email, password_hash, role, branch_code,
		is_active, last_login_at, password_changed_at, failed_login_count, locked_until,
		created_by, created_at, updated_at
		FROM staff_users WHERE id = $1`
	row := r.db.QueryRowContext(ctx, q, id)
	u, err := scanStaffUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("staff user not found")
	}
	return u, err
}

func (r *StaffRepository) GetByUsername(ctx context.Context, username string) (*domain.StaffUser, error) {
	q := `SELECT id, employee_id, username, full_name, email, password_hash, role, branch_code,
		is_active, last_login_at, password_changed_at, failed_login_count, locked_until,
		created_by, created_at, updated_at
		FROM staff_users WHERE username = $1`
	row := r.db.QueryRowContext(ctx, q, username)
	u, err := scanStaffUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New("staff user not found")
	}
	return u, err
}

func (r *StaffRepository) List(ctx context.Context, limit, offset int) ([]domain.StaffUser, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM staff_users").Scan(&total); err != nil {
		return nil, 0, err
	}
	q := `SELECT id, employee_id, username, full_name, email, password_hash, role, branch_code,
		is_active, last_login_at, password_changed_at, failed_login_count, locked_until,
		created_by, created_at, updated_at
		FROM staff_users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.db.QueryContext(ctx, q, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var list []domain.StaffUser
	for rows.Next() {
		u, err := scanStaffUser(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *u)
	}
	return list, total, nil
}

func (r *StaffRepository) Update(ctx context.Context, u *domain.StaffUser) error {
	q := `UPDATE staff_users
		SET full_name=$1, email=$2, role=$3, branch_code=$4, is_active=$5, updated_at=NOW()
		WHERE id=$6`
	_, err := r.db.ExecContext(ctx, q, u.FullName, u.Email, u.Role, u.BranchCode, u.IsActive, u.ID)
	return err
}

func (r *StaffRepository) IncrementFailedLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_users SET failed_login_count = failed_login_count + 1, updated_at=NOW() WHERE id=$1", id)
	return err
}

func (r *StaffRepository) LockAccount(ctx context.Context, id uuid.UUID, until time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_users SET locked_until=$1, updated_at=NOW() WHERE id=$2", until, id)
	return err
}

func (r *StaffRepository) ResetFailedLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_users SET failed_login_count=0, locked_until=NULL, updated_at=NOW() WHERE id=$1", id)
	return err
}

func (r *StaffRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_users SET last_login_at=NOW(), updated_at=NOW() WHERE id=$1", id)
	return err
}

func (r *StaffRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE staff_users SET password_hash=$1, password_changed_at=NOW(), failed_login_count=0, locked_until=NULL, updated_at=NOW() WHERE id=$2",
		hash, id)
	return err
}
