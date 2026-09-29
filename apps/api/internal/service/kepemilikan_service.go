package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// kepemilikanRegisterService menyediakan jalur baca dan tulis REGISTER PEMEGANG
// SAHAM BPR untuk Form 00.01 "Data Kepemilikan BPR".
//
// Ini BUKAN perubahan data operasional: register adalah rincian pemegang saham yang
// bank catat, bukan perhitungan modal. Pembacaan bersifat bank-wide (aktor
// non-lintas cabang ditolak); penulisan dijaga izin system:config di rute dan
// diaudit dalam transaksi yang sama, mengikuti pola register AYDA (000115).
type kepemilikanRegisterService struct {
	repo   domain.KepemilikanRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewKepemilikanRegisterService merakit layanan register kepemilikan BPR. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa
// menggagalkan aksi. db dipakai membuka transaksi tulis.
func NewKepemilikanRegisterService(db *sql.DB, repo domain.KepemilikanRegisterRepository, auditSinks ...domain.AuditRepository) domain.KepemilikanRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &kepemilikanRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// KepemilikanReport menyusun laporan register pemegang saham untuk satu posisi.
// Bank-wide: aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *kepemilikanRegisterService) KepemilikanReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.KepemilikanReport, error) {
	report := domain.KepemilikanReport{AsOf: asOf.UTC(), Items: []domain.KepemilikanItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrKepemilikanBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register kepemilikan BPR: repositori tidak tersedia")
	}
	items, err := s.repo.ListKepemilikanForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register kepemilikan BPR: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *kepemilikanRegisterService) ListItems(ctx context.Context) ([]domain.KepemilikanItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register kepemilikan BPR: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register kepemilikan BPR: %w", err)
	}
	if items == nil {
		items = []domain.KepemilikanItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu pemegang saham (id kosong = buat baru) dan menulis audit
// dalam satu transaksi.
func (s *kepemilikanRegisterService) UpsertItem(ctx context.Context, input domain.UpdateKepemilikanItemInput, actor domain.Actor) (*domain.KepemilikanItem, error) {
	item, err := domain.BuildKepemilikanItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_KEPEMILIKAN_ITEM", "kepemilikan_item",
			item.ID.String(), kepemilikanAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu pemegang saham dan menulis audit. Baris yang tidak ada
// ditolak ErrKepemilikanNotFound agar penghapusan tidak tampak berhasil.
func (s *kepemilikanRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrKepemilikanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_KEPEMILIKAN_ITEM", "kepemilikan_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// kepemilikanAuditChanges membangun catatan audit ringkas pemegang saham yang
// disimpan. Identitas (kolom IV) tidak ikut karena memang tidak pernah disimpan.
func kepemilikanAuditChanges(i domain.KepemilikanItem) map[string]any {
	return map[string]any{
		"shareholder_name":        i.ShareholderName,
		"shareholder_address":     i.ShareholderAddress,
		"shareholder_type_code":   i.ShareholderTypeCode,
		"shareholder_status_code": i.ShareholderStatusCode,
		"nominal_amount":          i.NominalAmount.String(),
		"ownership_percentage":    i.OwnershipPercentage.String(),
		"change_status_code":      i.ChangeStatusCode,
		"as_of":                   i.AsOf.Format("2006-01-02"),
		"status":                  i.Status,
	}
}

var _ domain.KepemilikanRegisterService = (*kepemilikanRegisterService)(nil)
