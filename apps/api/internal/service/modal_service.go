package service

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// modalService menyediakan jalur baca dan tulis REGISTER MODAL untuk Form 00.06
// "Daftar Modal Disetor, Modal Sumbangan, dan Dana Setoran Modal - Ekuitas".
//
// Ini register peristiwa modal, bukan turunan saldo bagan akun: bank mencatat satu baris
// per setoran/sumbangan beserta bentuknya dan tanggal persetujuan otoritas (lihat
// domain/form00_06_modal.go). Pembacaan bank-wide; penulisan dijaga izin system:config di
// rute dan diaudit dalam transaksi yang sama.
//
// Form 00.06 tidak menetapkan nomor unik, sehingga penghapusan adalah DELETE fisik.
type modalService struct {
	repo   domain.ModalRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewModalService merakit layanan register modal. auditSinks variadik mengikuti pola
// layanan lain: nil berarti audit dilewati. db dipakai membuka transaksi.
func NewModalService(db *sql.DB, repo domain.ModalRepository, auditSinks ...domain.AuditRepository) domain.ModalService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &modalService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// ListModal menyusun laporan register modal. Bank-wide: aktor yang tidak berwenang atas
// seluruh bank ditolak.
func (s *modalService) ListModal(ctx context.Context, actor domain.Actor) (domain.ModalReport, error) {
	report := domain.ModalReport{Items: []domain.ModalItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrModalBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register modal: repositori tidak tersedia")
	}
	items, err := s.repo.ListModalForOJK(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca register modal: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik. Bukan
// laporan: tidak ada pemeriksaan bank-wide karena rute pengaturan dijaga system:config.
func (s *modalService) ListItems(ctx context.Context) ([]domain.ModalItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register modal: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register modal: %w", err)
	}
	if items == nil {
		items = []domain.ModalItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit.
func (s *modalService) UpsertItem(ctx context.Context, in domain.UpdateModalItemInput, actor domain.Actor) (*domain.ModalItem, error) {
	item, err := domain.BuildModalItem(in)
	if err != nil {
		return nil, err
	}
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_MODAL_ITEM", "modal_item",
			item.ID.String(), modalAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu baris (DELETE fisik) dan menulis audit.
func (s *modalService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrModalNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_MODAL_ITEM", "modal_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// modalAuditChanges membangun catatan audit ringkas.
func modalAuditChanges(i domain.ModalItem) map[string]any {
	return map[string]any{
		"jenis_code":       i.JenisCode,
		"jenis_modal_code": i.JenisModalCode,
		"jumlah":           i.Jumlah.String(),
	}
}

var _ domain.ModalService = (*modalService)(nil)
