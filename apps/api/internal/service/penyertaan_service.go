package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// penyertaanRegisterService menyediakan jalur baca dan tulis REGISTER PENYERTAAN MODAL
// untuk Form 16.00 "Daftar Penyertaan Modal".
//
// Ini BUKAN perubahan data operasional: register adalah rincian penyertaan modal yang
// bank catat. Pembacaan bersifat bank-wide (aktor non-lintas cabang ditolak); penulisan
// dijaga izin system:config di rute dan diaudit dalam transaksi yang sama, mengikuti pola
// register properti terbengkalai (000118) dan aset tetap (000119).
//
// Nomor Register bersifat no reuse/no recycle (Form 16.00, PDF #page 227). Karena itu
// penghapusan dilakukan sebagai soft-delete (status -> NONAKTIF), tidak pernah DELETE
// fisik: baris tetap ada dan memegang nomornya.
type penyertaanRegisterService struct {
	repo   domain.PenyertaanRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewPenyertaanRegisterService merakit layanan register penyertaan modal. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan
// aksi. db dipakai membuka transaksi tulis.
func NewPenyertaanRegisterService(db *sql.DB, repo domain.PenyertaanRegisterRepository, auditSinks ...domain.AuditRepository) domain.PenyertaanRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &penyertaanRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// PenyertaanReport menyusun laporan register penyertaan modal untuk satu posisi.
// Bank-wide: aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *penyertaanRegisterService) PenyertaanReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PenyertaanReport, error) {
	report := domain.PenyertaanReport{AsOf: asOf.UTC(), Items: []domain.PenyertaanItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrPenyertaanBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register penyertaan modal: repositori tidak tersedia")
	}
	items, err := s.repo.ListPenyertaanForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register penyertaan modal: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *penyertaanRegisterService) ListItems(ctx context.Context) ([]domain.PenyertaanItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register penyertaan modal: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register penyertaan modal: %w", err)
	}
	if items == nil {
		items = []domain.PenyertaanItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu penyertaan (id kosong = buat baru) dan menulis audit dalam
// satu transaksi. No. Register yang sudah pernah dipakai baris lain — termasuk baris
// NONAKTIF — ditolak repositori dengan ErrPenyertaanNoRegisterUsed.
func (s *penyertaanRegisterService) UpsertItem(ctx context.Context, input domain.UpdatePenyertaanItemInput, actor domain.Actor) (*domain.PenyertaanItem, error) {
	item, err := domain.BuildPenyertaanItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_PENYERTAAN_ITEM", "penyertaan_item",
			item.ID.String(), penyertaanAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menonaktifkan satu penyertaan (soft-delete, bukan DELETE fisik) dan menulis
// audit. Baris yang tidak ada ditolak ErrPenyertaanNotFound agar penghapusan tidak tampak
// berhasil.
func (s *penyertaanRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.SoftDeleteItemTx(ctx, tx, id, actor.UserID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPenyertaanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_PENYERTAAN_ITEM", "penyertaan_item",
			id.String(), map[string]any{"id": id.String(), "status": domain.PenyertaanStatusNonaktif})
	})
}

// penyertaanAuditChanges membangun catatan audit ringkas penyertaan yang disimpan.
func penyertaanAuditChanges(i domain.PenyertaanItem) map[string]any {
	return map[string]any{
		"no_register":            i.NoRegister,
		"counterparty_id":        i.CounterpartyID,
		"metode_penyertaan_code": i.MetodePenyertaanCode,
		"kualitas_code":          i.KualitasCode,
		"tujuan_penyertaan_code": i.TujuanPenyertaanCode,
		"tanggal_mulai":          i.TanggalMulai.Format("2006-01-02"),
		"persentase_penyertaan":  i.PersentasePenyertaan.String(),
		"nominal":                i.Nominal.String(),
		"jumlah_bulan_laporan":   i.JumlahBulanLaporan.String(),
		"ckpn":                   i.CKPN.String(),
		"ckpn_aset_baik":         i.CKPNAsetBaik.String(),
		"ckpn_aset_kurang_baik":  i.CKPNAsetKurangBaik.String(),
		"ckpn_aset_tidak_baik":   i.CKPNAsetTidakBaik.String(),
		"jenis_ckpn_code":        i.JenisCKPNCode,
		"as_of":                  i.AsOf.Format("2006-01-02"),
		"status":                 i.Status,
	}
}

var _ domain.PenyertaanRegisterService = (*penyertaanRegisterService)(nil)
