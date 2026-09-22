package postgres

import (
	"context"
	"database/sql"
	"errors"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type BranchRepository struct {
	db *sql.DB
}

func NewBranchRepository(db *sql.DB) *BranchRepository {
	return &BranchRepository{db: db}
}

const branchColumns = `id, code, name, COALESCE(address, ''), COALESCE(phone, ''), is_head_office, is_active, parent_id, unit_level`

func (r *BranchRepository) List(ctx context.Context) ([]domain.Branch, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+branchColumns+` FROM branches ORDER BY unit_level DESC, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []domain.Branch
	for rows.Next() {
		b, err := scanBranch(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *b)
	}
	return list, rows.Err()
}

func (r *BranchRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Branch, error) {
	return r.scanOne(ctx, `SELECT `+branchColumns+` FROM branches WHERE id = $1`, id)
}

func (r *BranchRepository) GetByCode(ctx context.Context, code string) (*domain.Branch, error) {
	return r.scanOne(ctx, `SELECT `+branchColumns+` FROM branches WHERE code = $1`, code)
}

// Create menyimpan cabang baru. Keunikan kode ditegakkan unique constraint
// branches.code; pemanggil menerjemahkan pelanggarannya menjadi error bisnis.
func (r *BranchRepository) Create(ctx context.Context, b *domain.Branch) error {
	return r.CreateTx(ctx, r.db, b)
}

// CreateTx menyimpan cabang di dalam transaksi pemanggil. Dipakai agar penulisan
// cabang dan audit log commit bersama. parent_id dan unit_level ikut disimpan
// supaya hierarki organisasi (W14) dibangun lewat jalur produksi yang sama.
func (r *BranchRepository) CreateTx(ctx context.Context, tx any, b *domain.Branch) error {
	exec, ok := tx.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	})
	if !ok {
		return errors.New("branch: transaksi tidak valid")
	}
	level := b.UnitLevel
	if level == "" {
		level = domain.UnitLevelBranch
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO branches (id, code, name, address, phone, is_head_office, is_active, parent_id, unit_level)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::org_unit_level)`,
		b.ID, b.Code, b.Name, b.Address, b.Phone, b.IsHeadOffice, b.IsActive, b.ParentID, level)
	return err
}

// ResolveScopeCodes mengembalikan kode unit yang boleh diakses kode code: unit
// itu sendiri dan seluruh turunannya, mengikuti rantai parent_id. Satu query
// rekursif menjaga pemetaan tetap di database, bukan di kode pemanggil.
// Unit nonaktif dikecualikan agar unit yang dinonaktifkan tidak lagi membuka akses.
func (r *BranchRepository) ResolveScopeCodes(ctx context.Context, code string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, `
		WITH RECURSIVE scope AS (
			SELECT id, code FROM branches WHERE code = $1 AND is_active
			UNION ALL
			SELECT b.id, b.code FROM branches b
			JOIN scope s ON b.parent_id = s.id
			WHERE b.is_active
		)
		SELECT code FROM scope`, code)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var codes []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		codes = append(codes, c)
	}
	return codes, rows.Err()
}

// SetParentTx memindahkan unit ke bawah atasan (parentID nil = puncak) di dalam
// transaksi pemanggil, agar perubahan susunan dan auditnya commit bersama.
func (r *BranchRepository) SetParentTx(ctx context.Context, tx any, id uuid.UUID, parentID *uuid.UUID) error {
	exec, ok := tx.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	})
	if !ok {
		return errors.New("branch: transaksi tidak valid")
	}
	_, err := exec.ExecContext(ctx, `UPDATE branches SET parent_id = $2, updated_at = NOW() WHERE id = $1`, id, parentID)
	return err
}

// ScopeImpactUsers menghitung pengguna aktif yang cakupannya berubah bila target
// dipindahkan ke atasan baru. Cakupan seorang staf adalah cabangnya ditambah seluruh
// turunannya, jadi hanya staf pada leluhur target yang berubah: leluhur lama yang
// bukan leluhur baru kehilangan target, dan sebaliknya. Staf di unit target sendiri
// (dan turunannya) selalu memuat target, sehingga tidak pernah ikut kehilangan.
func (r *BranchRepository) ScopeImpactUsers(ctx context.Context, targetID uuid.UUID, oldParentID, newParentID *uuid.UUID) (int, int, error) {
	var losing, gaining int
	err := r.db.QueryRowContext(ctx, `
		WITH RECURSIVE old_chain AS (
			SELECT id, code, parent_id FROM branches WHERE id = $1
			UNION ALL
			SELECT b.id, b.code, b.parent_id FROM branches b JOIN old_chain c ON b.id = c.parent_id
		),
		new_chain AS (
			SELECT id, code, parent_id FROM branches WHERE id = $2
			UNION ALL
			SELECT b.id, b.code, b.parent_id FROM branches b JOIN new_chain c ON b.id = c.parent_id
		),
		losing AS (SELECT code FROM old_chain EXCEPT SELECT code FROM new_chain),
		gaining AS (SELECT code FROM new_chain EXCEPT SELECT code FROM old_chain)
		SELECT
			(SELECT COUNT(*) FROM staff_users u WHERE u.is_active AND u.branch_code IN (SELECT code FROM losing)),
			(SELECT COUNT(*) FROM staff_users u WHERE u.is_active AND u.branch_code IN (SELECT code FROM gaining))`,
		oldParentID, newParentID).Scan(&losing, &gaining)
	if err != nil {
		return 0, 0, err
	}
	return losing, gaining, nil
}

func (r *BranchRepository) scanOne(ctx context.Context, query string, arg any) (*domain.Branch, error) {
	return scanBranch(r.db.QueryRowContext(ctx, query, arg))
}

// branchScanner menyeragamkan pemindaian baris (QueryRow atau Rows) yang keduanya
// menyediakan Scan dengan bentuk kolom yang sama.
type branchScanner interface {
	Scan(dest ...any) error
}

func scanBranch(row branchScanner) (*domain.Branch, error) {
	var b domain.Branch
	var level string
	err := row.Scan(
		&b.ID, &b.Code, &b.Name, &b.Address, &b.Phone, &b.IsHeadOffice, &b.IsActive, &b.ParentID, &level,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrBranchNotFound
		}
		return nil, err
	}
	b.UnitLevel = domain.OrgUnitLevel(level)
	return &b, nil
}
