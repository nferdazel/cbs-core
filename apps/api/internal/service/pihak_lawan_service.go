package service

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// pihakLawanService menyediakan jalur baca dan tulis REGISTER PIHAK LAWAN untuk Form 00.16.
//
// Memuat seluruh pihak lawan bank maupun bukan bank yang bertransaksi dengan BPR. Kolom III
// Nomor Identitas dan VI NPWP sengaja tidak disimpan (keputusan privasi). Pembacaan
// bank-wide; penulisan dijaga izin system:config di rute dan diaudit dalam transaksi yang
// sama. Form 00.16 tidak menetapkan nomor unik, sehingga hapus = DELETE fisik.
type pihakLawanService struct {
	repo   domain.PihakLawanRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewPihakLawanService merakit layanan register pihak lawan. auditSinks variadik mengikuti
// pola layanan lain: nil berarti audit dilewati. db dipakai membuka transaksi.
func NewPihakLawanService(db *sql.DB, repo domain.PihakLawanRepository, auditSinks ...domain.AuditRepository) domain.PihakLawanService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &pihakLawanService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// ListPihakLawan menyusun laporan register pihak lawan. Bank-wide: aktor yang tidak
// berwenang atas seluruh bank ditolak.
func (s *pihakLawanService) ListPihakLawan(ctx context.Context, actor domain.Actor) (domain.PihakLawanReport, error) {
	report := domain.PihakLawanReport{Items: []domain.PihakLawanItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrPihakLawanBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register pihak lawan: repositori tidak tersedia")
	}
	items, err := s.repo.ListPihakLawanForOJK(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca register pihak lawan: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik. Bukan
// laporan: tidak ada pemeriksaan bank-wide karena rute pengaturan dijaga system:config.
func (s *pihakLawanService) ListItems(ctx context.Context) ([]domain.PihakLawanItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register pihak lawan: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register pihak lawan: %w", err)
	}
	if items == nil {
		items = []domain.PihakLawanItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit.
func (s *pihakLawanService) UpsertItem(ctx context.Context, in domain.UpdatePihakLawanItemInput, actor domain.Actor) (*domain.PihakLawanItem, error) {
	item, err := domain.BuildPihakLawanItem(in)
	if err != nil {
		return nil, err
	}
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_PIHAK_LAWAN_ITEM", "pihak_lawan_item",
			item.ID.String(), pihakLawanAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu baris (DELETE fisik) dan menulis audit.
func (s *pihakLawanService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPihakLawanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_PIHAK_LAWAN_ITEM", "pihak_lawan_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// pihakLawanAuditChanges membangun catatan audit ringkas. Kolom III Nomor Identitas dan VI
// NPWP tidak ikut karena memang tidak disimpan (keputusan privasi).
func pihakLawanAuditChanges(i domain.PihakLawanItem) map[string]any {
	return map[string]any{
		"pihak_lawan_id":     i.PihakLawanID,
		"nama":               i.Nama,
		"golongan_code":      i.GolonganCode,
		"hubungan_bank_code": i.HubunganBankCode,
	}
}

var _ domain.PihakLawanService = (*pihakLawanService)(nil)
