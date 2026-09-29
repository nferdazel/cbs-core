package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// suratBerhargaRegisterService menyediakan jalur baca dan tulis REGISTER SURAT BERHARGA
// untuk Form 04.00 "Daftar Surat Berharga".
//
// Ini BUKAN perubahan data operasional: register adalah rincian surat berharga yang bank
// miliki. Pembacaan bersifat bank-wide (aktor non-lintas cabang ditolak); penulisan dijaga
// izin system:config di rute dan diaudit dalam transaksi yang sama.
//
// Form 04.00 tidak menetapkan nomor register unik (tidak ada aturan no reuse/no recycle),
// sehingga penghapusan adalah DELETE fisik, bukan soft-delete.
type suratBerhargaRegisterService struct {
	repo   domain.SuratBerhargaRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewSuratBerhargaRegisterService merakit layanan register surat berharga. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan
// aksi. db dipakai membuka transaksi tulis.
func NewSuratBerhargaRegisterService(db *sql.DB, repo domain.SuratBerhargaRegisterRepository, auditSinks ...domain.AuditRepository) domain.SuratBerhargaRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &suratBerhargaRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// SuratBerhargaReport menyusun laporan register surat berharga untuk satu posisi.
// Bank-wide: aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *suratBerhargaRegisterService) SuratBerhargaReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.SuratBerhargaReport, error) {
	report := domain.SuratBerhargaReport{AsOf: asOf.UTC(), Items: []domain.SuratBerhargaItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrSuratBerhargaBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register surat berharga: repositori tidak tersedia")
	}
	items, err := s.repo.ListSuratBerhargaForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register surat berharga: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *suratBerhargaRegisterService) ListItems(ctx context.Context) ([]domain.SuratBerhargaItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register surat berharga: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register surat berharga: %w", err)
	}
	if items == nil {
		items = []domain.SuratBerhargaItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit dalam satu
// transaksi.
func (s *suratBerhargaRegisterService) UpsertItem(ctx context.Context, input domain.UpdateSuratBerhargaItemInput, actor domain.Actor) (*domain.SuratBerhargaItem, error) {
	item, err := domain.BuildSuratBerhargaItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_SURAT_BERHARGA_ITEM", "surat_berharga_item",
			item.ID.String(), suratBerhargaAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu baris (DELETE fisik) dan menulis audit. Baris yang tidak ada
// ditolak ErrSuratBerhargaNotFound agar penghapusan tidak tampak berhasil.
func (s *suratBerhargaRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrSuratBerhargaNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_SURAT_BERHARGA_ITEM", "surat_berharga_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// suratBerhargaAuditChanges membangun catatan audit ringkas baris yang disimpan.
func suratBerhargaAuditChanges(i domain.SuratBerhargaItem) map[string]any {
	return map[string]any{
		"klasifikasi_code":               i.KlasifikasiCode,
		"suku_bunga":                     i.SukuBunga.String(),
		"tanggal_mulai":                  i.TanggalMulai.Format("2006-01-02"),
		"tanggal_jatuh_tempo":            i.TanggalJatuhTempo.Format("2006-01-02"),
		"nominal":                        i.Nominal.String(),
		"nominal_dijaminkan":             i.NominalDijaminkan.String(),
		"biaya_perolehan":                i.BiayaPerolehan.String(),
		"nomor_surat_berharga":           i.NomorSuratBerharga,
		"counterparty_id":                i.CounterpartyID,
		"jenis_code":                     i.JenisCode,
		"kualitas_code":                  i.KualitasCode,
		"lembaga_pemeringkat_code":       i.LembagaPemeringkatCode,
		"peringkat_surat_berharga_code":  i.PeringkatSuratBerhargaCode,
		"klasifikasi_aset_keuangan_code": i.KlasifikasiAsetKeuanganCode,
		"jenis_ckpn_code":                i.JenisCKPNCode,
		"as_of":                          i.AsOf.Format("2006-01-02"),
		"status":                         i.Status,
	}
}

var _ domain.SuratBerhargaRegisterService = (*suratBerhargaRegisterService)(nil)
