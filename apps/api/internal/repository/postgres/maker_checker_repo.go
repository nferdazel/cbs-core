package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

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
	// Cabang pengajuan diisi lewat subquery di INSERT yang sama agar tidak
	// menambah round-trip. Bila BranchCode kosong, subquery menghasilkan NULL —
	// pengajuan tanpa cabang tetap sah dan terlihat semua cabang.
	// business_date diisi dari tanggal bisnis saat pengajuan dibuat; NULL hanya untuk
	// pengajuan yang dibuat saat sumber tanggal bisnis tidak tersedia (data lama), dan
	// kueri akumulasi harian memakai fallback tanggal kalender WIB untuk baris itu.
	_, err = sqlTx.ExecContext(ctx, `
		INSERT INTO maker_checker_requests
			(id, action_type, payload, status, maker_id, maker_notes, created_at, updated_at, branch_id, business_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, (SELECT id FROM branches WHERE code = $9), $10)`,
		req.ID, req.ActionType, payload, req.Status, req.MakerID,
		nullIfEmpty(req.MakerNotes), req.CreatedAt, req.UpdatedAt, req.BranchCode, nullDate(req.BusinessDate),
	)
	if err != nil {
		return fmt.Errorf("maker-checker: menyimpan permintaan: %w", err)
	}
	return nil
}

// nullDate mengirim NULL ke kolom DATE ketika tanggal belum diisi, dan tanggal apa
// adanya bila sudah. Dipisah agar pemanggil tidak perlu mengurus sql.NullTime.
func nullDate(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func (r *MakerCheckerRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.MakerCheckerRequest, error) {
	var req domain.MakerCheckerRequest
	var payload []byte
	var checkerID sql.NullString
	var reviewedAt sql.NullTime
	var businessDate sql.NullTime

	// branch_id diambil lewat join: pemeriksaan cabang di Approve/Reject
	// bergantung padanya, dan bila kosong pemeriksaan lintas cabang akan lolos
	// diam-diam. Baris lama tanpa cabang menghasilkan kode kosong.
	err := r.db.QueryRowContext(ctx, `
		SELECT mcr.id, mcr.action_type, mcr.payload, mcr.status, mcr.maker_id, mcr.checker_id,
		       COALESCE(mcr.maker_notes, ''), COALESCE(mcr.checker_notes, ''), mcr.reviewed_at, mcr.created_at, mcr.updated_at,
		       COALESCE(b.code, ''), mcr.business_date
		FROM maker_checker_requests mcr
		LEFT JOIN branches b ON b.id = mcr.branch_id
		WHERE mcr.id = $1`, id,
	).Scan(
		&req.ID, &req.ActionType, &payload, &req.Status, &req.MakerID, &checkerID,
		&req.MakerNotes, &req.CheckerNotes, &reviewedAt, &req.CreatedAt, &req.UpdatedAt,
		&req.BranchCode, &businessDate,
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
	if businessDate.Valid {
		req.BusinessDate = businessDate.Time
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

// buildPendingListQuery menyusun query daftar pengajuan PENDING beserta klausa
// filter cabang dan argumennya. Dipisah sebagai fungsi murni supaya penentuan
// scope cabang dapat diuji tanpa database; SQL akhirnya sendiri tetap hanya
// terverifikasi saat runtime. Aktor lintas cabang tidak difilter; aktor cabang
// hanya melihat cabangnya, dan baris branch_id NULL tetap terlihat.
func buildPendingListQuery(actor domain.Actor) (string, []any) {
	where, whereArgs := branchReadClause("branch_id", actor)
	// Buku pengajuan disimpan server-side di payload (maker_book). NULLIF kosong -> NULL
	// lalu dicor ke coa_book agar klausa buku yang sama (bookReadClause) dapat dipakai;
	// pengajuan lama tanpa maker_book tetap terlihat, mengikuti semantik CanAccessBook.
	bookColumn := "(NULLIF(payload->>'maker_book', '')::coa_book)"
	if clause, args := bookReadClause(bookColumn, actor, len(whereArgs)+1); clause != "" {
		whereArgs = append(whereArgs, args...)
		where = andCondition(where, clause)
	}

	query := `
		SELECT id, action_type, payload, status, maker_id, checker_id,
		       COALESCE(maker_notes, ''), COALESCE(checker_notes, ''), reviewed_at, created_at, updated_at, business_date
		FROM maker_checker_requests
		WHERE status = 'PENDING'`
	if where != "" {
		query += " AND " + where
	}
	query += " ORDER BY created_at ASC"
	return query, whereArgs
}

func (r *MakerCheckerRepository) ListPending(ctx context.Context, actor domain.Actor) ([]domain.MakerCheckerRequest, error) {
	query, args := buildPendingListQuery(actor)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var list []domain.MakerCheckerRequest
	for rows.Next() {
		var req domain.MakerCheckerRequest
		var payload []byte
		var checkerID sql.NullString
		var reviewedAt sql.NullTime
		var businessDate sql.NullTime
		if err := rows.Scan(
			&req.ID, &req.ActionType, &payload, &req.Status, &req.MakerID, &checkerID,
			&req.MakerNotes, &req.CheckerNotes, &reviewedAt, &req.CreatedAt, &req.UpdatedAt,
			&businessDate,
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
		if businessDate.Valid {
			req.BusinessDate = businessDate.Time
		}
		list = append(list, req)
	}
	return list, rows.Err()
}

var _ domain.MakerCheckerRepository = (*MakerCheckerRepository)(nil)
