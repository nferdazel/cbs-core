package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// kasValasRegisterService menyediakan jalur baca dan tulis REGISTER KAS VALUTA ASING
// untuk Form 03.00 "Daftar Kas dalam Valuta Asing".
//
// Ini BUKAN perubahan data operasional: register adalah rincian kas valas yang bank
// perdagangkan. Pembacaan bersifat bank-wide (aktor non-lintas cabang ditolak); penulisan
// dijaga izin system:config di rute dan diaudit dalam transaksi yang sama.
//
// Form 03.00 tidak menetapkan nomor unik, sehingga penghapusan adalah DELETE fisik.
type kasValasRegisterService struct {
	repo   domain.KasValasRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewKasValasRegisterService merakit layanan register kas valas. auditRepo variadik
// mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi. db
// dipakai membuka transaksi tulis.
func NewKasValasRegisterService(db *sql.DB, repo domain.KasValasRegisterRepository, auditSinks ...domain.AuditRepository) domain.KasValasRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &kasValasRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// KasValasReport menyusun laporan register kas valas untuk satu posisi. Bank-wide: aktor
// yang tidak berwenang atas seluruh bank ditolak.
func (s *kasValasRegisterService) KasValasReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.KasValasReport, error) {
	report := domain.KasValasReport{AsOf: asOf.UTC(), Items: []domain.KasValasItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrKasValasBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register kas valuta asing: repositori tidak tersedia")
	}
	items, err := s.repo.ListKasValasForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register kas valuta asing: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *kasValasRegisterService) ListItems(ctx context.Context) ([]domain.KasValasItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register kas valuta asing: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register kas valuta asing: %w", err)
	}
	if items == nil {
		items = []domain.KasValasItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit dalam satu
// transaksi.
func (s *kasValasRegisterService) UpsertItem(ctx context.Context, input domain.UpdateKasValasItemInput, actor domain.Actor) (*domain.KasValasItem, error) {
	item, err := domain.BuildKasValasItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_KAS_VALAS_ITEM", "kas_valas_item",
			item.ID.String(), kasValasAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu baris (DELETE fisik) dan menulis audit. Baris yang tidak ada
// ditolak ErrKasValasNotFound agar penghapusan tidak tampak berhasil.
func (s *kasValasRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrKasValasNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_KAS_VALAS_ITEM", "kas_valas_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// kasValasAuditChanges membangun catatan audit ringkas baris yang disimpan.
func kasValasAuditChanges(i domain.KasValasItem) map[string]any {
	return map[string]any{
		"jenis_valas_code": i.JenisValasCode,
		"nominal":          i.Nominal.String(),
		"kurs_tengah":      i.KursTengah.String(),
		"as_of":            i.AsOf.Format("2006-01-02"),
		"status":           i.Status,
	}
}

var _ domain.KasValasRegisterService = (*kasValasRegisterService)(nil)
