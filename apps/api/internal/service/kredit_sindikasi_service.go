package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// kreditSindikasiRegisterService menyediakan jalur baca dan tulis REGISTER KREDIT
// SINDIKASI untuk Form 06.02 "Daftar Kredit Sindikasi".
//
// Ini BUKAN perubahan data operasional: register adalah rincian fasilitas kredit sindikasi
// yang bank laporkan. Pembacaan bersifat bank-wide (aktor non-lintas cabang ditolak);
// penulisan dijaga izin system:config di rute dan diaudit dalam transaksi yang sama.
//
// Form 06.02 menetapkan No. Rekening unik dan "tidak boleh sama" (PDF #page 177), sehingga
// penghapusan adalah soft-delete: nomor rekening tetap terpakai.
type kreditSindikasiRegisterService struct {
	repo   domain.SindikasiRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewKreditSindikasiRegisterService merakit layanan register kredit sindikasi. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi.
// db dipakai membuka transaksi tulis.
func NewKreditSindikasiRegisterService(db *sql.DB, repo domain.SindikasiRegisterRepository, auditSinks ...domain.AuditRepository) domain.SindikasiRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &kreditSindikasiRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// SindikasiReport menyusun laporan register kredit sindikasi untuk satu posisi. Bank-wide:
// aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *kreditSindikasiRegisterService) SindikasiReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.SindikasiReport, error) {
	report := domain.SindikasiReport{AsOf: asOf.UTC(), Items: []domain.SindikasiItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrSindikasiBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register kredit sindikasi: repositori tidak tersedia")
	}
	items, err := s.repo.ListSindikasiForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register kredit sindikasi: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *kreditSindikasiRegisterService) ListItems(ctx context.Context) ([]domain.SindikasiItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register kredit sindikasi: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register kredit sindikasi: %w", err)
	}
	if items == nil {
		items = []domain.SindikasiItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit dalam satu
// transaksi. Nomor rekening yang sudah dipakai baris lain ditolak
// ErrSindikasiNoRekeningUsed dari repositori.
func (s *kreditSindikasiRegisterService) UpsertItem(ctx context.Context, input domain.UpdateSindikasiItemInput, actor domain.Actor) (*domain.SindikasiItem, error) {
	item, err := domain.BuildSindikasiItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_KREDIT_SINDIKASI_ITEM", "kredit_sindikasi_item",
			item.ID.String(), kreditSindikasiAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menonaktifkan satu baris (soft-delete NONAKTIF) dan menulis audit. Baris yang
// tidak ada ditolak ErrSindikasiNotFound agar penghapusan tidak tampak berhasil.
func (s *kreditSindikasiRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.SoftDeleteItemTx(ctx, tx, id, actor.UserID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrSindikasiNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_KREDIT_SINDIKASI_ITEM", "kredit_sindikasi_item",
			id.String(), map[string]any{"id": id.String(), "status": "NONAKTIF"})
	})
}

// kreditSindikasiAuditChanges membangun catatan audit ringkas baris yang disimpan. Kolom III
// No. Identitas tidak ikut karena memang tidak disimpan (keputusan privasi).
func kreditSindikasiAuditChanges(i domain.SindikasiItem) map[string]any {
	return map[string]any{
		"counterparty_id":         i.CounterpartyID,
		"no_rekening":             i.NoRekening,
		"nomor_perjanjian_induk":  i.NomorPerjanjianInduk,
		"status_kepesertaan_code": i.StatusKepesertaanCode,
		"kualitas_code":           i.KualitasCode,
		"as_of":                   i.AsOf.Format("2006-01-02"),
		"status":                  i.Status,
	}
}

var _ domain.SindikasiRegisterService = (*kreditSindikasiRegisterService)(nil)
