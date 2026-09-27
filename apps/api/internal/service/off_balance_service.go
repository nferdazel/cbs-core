package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// offBalanceService menyediakan jalur baca dan tulis REGISTER REKENING ADMINISTRATIF
// (pos komitmen/kontinjensi off-balance) untuk Form 01.01.
//
// Ini BUKAN perubahan data operasional yang menyentuh uang: rekening administratif
// tidak pernah menjadi saldo bagan akun, hanya atribut laporan. Pembacaan bersifat
// bank-wide (aktor non-lintas cabang ditolak); penulisan dijaga izin system:config di
// rute dan diaudit dalam transaksi yang sama, mengikuti pola kelembagaan/OJK loan codes.
type offBalanceService struct {
	db     *sql.DB
	repo   domain.OffBalanceRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewOffBalanceService merakit layanan register rekening administratif. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan
// aksi. db dipakai memeriksa nasabah lawan sebelum menyimpan; nil melewati pemeriksaan
// itu (foreign key tetap penjaga terakhir).
func NewOffBalanceService(db *sql.DB, repo domain.OffBalanceRepository, auditSinks ...domain.AuditRepository) domain.OffBalanceService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &offBalanceService{db: db, repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// OffBalanceReport menyusun laporan register rekening administratif untuk satu posisi.
// Bank-wide: aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *offBalanceService) OffBalanceReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.OffBalanceReport, error) {
	report := domain.OffBalanceReport{
		AsOf:       asOf.UTC(),
		Items:      []domain.OffBalanceItem{},
		Aggregates: []domain.OffBalanceAggregate{},
	}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrOffBalanceBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul rekening administratif: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca register rekening administratif: %w", err)
	}
	aggregates, err := s.repo.ListAggregates(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("mengagregasi register rekening administratif: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	if aggregates != nil {
		report.Aggregates = aggregates
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *offBalanceService) ListItems(ctx context.Context) ([]domain.OffBalanceItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul rekening administratif: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register rekening administratif: %w", err)
	}
	if items == nil {
		items = []domain.OffBalanceItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu pos (id kosong = buat baru) dan menulis audit dalam satu
// transaksi.
func (s *offBalanceService) UpsertItem(ctx context.Context, input domain.UpdateOffBalanceItemInput, actor domain.Actor) (*domain.OffBalanceItem, error) {
	item, err := domain.BuildOffBalanceItem(input)
	if err != nil {
		return nil, err
	}
	if err := s.periksaLawan(ctx, item.CounterpartyCustomerID); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_OFF_BALANCE_ITEM", "off_balance_item",
			item.ID.String(), offBalanceAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu pos dan menulis audit. Baris yang tidak ada ditolak
// ErrOffBalanceNotFound agar penghapusan tidak tampak berhasil.
func (s *offBalanceService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrOffBalanceNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_OFF_BALANCE_ITEM", "off_balance_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// periksaLawan menolak nasabah lawan yang tidak ada dengan pesan yang menyebut
// kolomnya. Nasabah kosong lolos; db nil melewati pemeriksaan (foreign key tetap
// penjaga terakhir).
func (s *offBalanceService) periksaLawan(ctx context.Context, id *uuid.UUID) error {
	if s.db == nil || id == nil {
		return nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM customers WHERE id = $1)`, *id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: counterparty_customer_id tidak ditemukan", domain.ErrOffBalanceInputInvalid)
	}
	return nil
}

// offBalanceAuditChanges membangun catatan audit ringkas pos yang disimpan.
func offBalanceAuditChanges(i domain.OffBalanceItem) map[string]any {
	return map[string]any{
		"position_code": i.PositionCode,
		"category":      i.Category,
		"description":   i.Description,
		"amount":        i.Amount.String(),
		"as_of":         i.AsOf.Format("2006-01-02"),
		"status":        i.Status,
	}
}

var _ domain.OffBalanceService = (*offBalanceService)(nil)
