package service

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// hapusBukuService menyediakan jalur baca dan tulis REGISTER ASET PRODUKTIF YANG DIHAPUS
// BUKU untuk Form 15.00.
//
// Ini register pelaporan, bukan modul hapus buku operasional: memuat kredit maupun
// penempatan pada bank lain yang dihapus buku, beserta dimensi pelaporan (jenis aset,
// pemisahan pokok/bunga, akumulasi tertagih, agunan) yang tidak disimpan pada baris loans.
// Pembacaan bank-wide; penulisan dijaga izin system:config di rute dan diaudit dalam
// transaksi yang sama. Form 15.00 tidak menetapkan nomor unik, sehingga hapus = DELETE fisik.
type hapusBukuService struct {
	repo   domain.HapusBukuRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewHapusBukuService merakit layanan register hapus buku. auditSinks variadik mengikuti
// pola layanan lain: nil berarti audit dilewati. db dipakai membuka transaksi.
func NewHapusBukuService(db *sql.DB, repo domain.HapusBukuRepository, auditSinks ...domain.AuditRepository) domain.HapusBukuService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &hapusBukuService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// ListHapusBuku menyusun laporan register hapus buku. Bank-wide: aktor yang tidak
// berwenang atas seluruh bank ditolak.
func (s *hapusBukuService) ListHapusBuku(ctx context.Context, actor domain.Actor) (domain.HapusBukuReport, error) {
	report := domain.HapusBukuReport{Items: []domain.HapusBukuItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrHapusBukuBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register hapus buku: repositori tidak tersedia")
	}
	items, err := s.repo.ListHapusBukuForOJK(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca register hapus buku: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik. Bukan
// laporan: tidak ada pemeriksaan bank-wide karena rute pengaturan dijaga system:config.
func (s *hapusBukuService) ListItems(ctx context.Context) ([]domain.HapusBukuItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register hapus buku: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register hapus buku: %w", err)
	}
	if items == nil {
		items = []domain.HapusBukuItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit.
func (s *hapusBukuService) UpsertItem(ctx context.Context, in domain.UpdateHapusBukuItemInput, actor domain.Actor) (*domain.HapusBukuItem, error) {
	item, err := domain.BuildHapusBukuItem(in)
	if err != nil {
		return nil, err
	}
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_HAPUS_BUKU_ITEM", "hapus_buku_item",
			item.ID.String(), hapusBukuAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu baris (DELETE fisik) dan menulis audit.
func (s *hapusBukuService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrHapusBukuNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_HAPUS_BUKU_ITEM", "hapus_buku_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// hapusBukuAuditChanges membangun catatan audit ringkas.
func hapusBukuAuditChanges(i domain.HapusBukuItem) map[string]any {
	return map[string]any{
		"jenis_aset_code":    i.JenisAsetCode,
		"nomor_rekening":     i.NomorRekening,
		"tanggal_hapus_buku": i.TanggalHapusBuku.Format("2006-01-02"),
	}
}

var _ domain.HapusBukuService = (*hapusBukuService)(nil)
