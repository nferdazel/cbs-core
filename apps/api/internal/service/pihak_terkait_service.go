package service

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// pihakTerkaitService menyediakan jalur baca dan tulis REGISTER PIHAK TERKAIT LAINNYA
// untuk Form 00.05 "Data Pihak Terkait Lainnya".
//
// Ini register pelaporan, bukan modul BMPK: pihak terkait di sini bisa BUKAN nasabah
// (lihat domain/form00_05_pihak_terkait.go). Pembacaan bersifat bank-wide (aktor
// non-lintas cabang ditolak); penulisan dijaga izin system:config di rute dan diaudit
// dalam transaksi yang sama.
//
// Form 00.05 tidak menetapkan nomor unik, sehingga penghapusan adalah DELETE fisik.
type pihakTerkaitService struct {
	repo   domain.PihakTerkaitRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewPihakTerkaitService merakit layanan register pihak terkait. auditSinks variadik
// mengikuti pola layanan lain: nil berarti audit dilewati. db dipakai membuka transaksi.
func NewPihakTerkaitService(db *sql.DB, repo domain.PihakTerkaitRepository, auditSinks ...domain.AuditRepository) domain.PihakTerkaitService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &pihakTerkaitService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// ListPihakTerkait menyusun laporan register pihak terkait. Bank-wide: aktor yang tidak
// berwenang atas seluruh bank ditolak.
func (s *pihakTerkaitService) ListPihakTerkait(ctx context.Context, actor domain.Actor) (domain.PihakTerkaitReport, error) {
	report := domain.PihakTerkaitReport{Items: []domain.PihakTerkaitItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrPihakTerkaitBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register pihak terkait: repositori tidak tersedia")
	}
	items, err := s.repo.ListPihakTerkaitForOJK(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca register pihak terkait: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik. Bukan
// laporan: tidak ada pemeriksaan bank-wide karena rute pengaturan dijaga system:config.
func (s *pihakTerkaitService) ListItems(ctx context.Context) ([]domain.PihakTerkaitItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register pihak terkait: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register pihak terkait: %w", err)
	}
	if items == nil {
		items = []domain.PihakTerkaitItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit.
func (s *pihakTerkaitService) UpsertItem(ctx context.Context, in domain.UpdatePihakTerkaitItemInput, actor domain.Actor) (*domain.PihakTerkaitItem, error) {
	item, err := domain.BuildPihakTerkaitItem(in)
	if err != nil {
		return nil, err
	}
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_PIHAK_TERKAIT_ITEM", "pihak_terkait_item",
			item.ID.String(), pihakTerkaitAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu baris (DELETE fisik) dan menulis audit.
func (s *pihakTerkaitService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPihakTerkaitNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_PIHAK_TERKAIT_ITEM", "pihak_terkait_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// pihakTerkaitAuditChanges membangun catatan audit ringkas. Kolom II No. Identitas tidak
// ikut karena memang tidak disimpan (keputusan privasi).
func pihakTerkaitAuditChanges(i domain.PihakTerkaitItem) map[string]any {
	return map[string]any{
		"nama":          i.Nama,
		"jenis_code":    i.JenisCode,
		"hubungan_code": i.HubunganCode,
	}
}

var _ domain.PihakTerkaitService = (*pihakTerkaitService)(nil)
