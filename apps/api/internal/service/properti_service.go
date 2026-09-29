package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// propertiRegisterService menyediakan jalur baca dan tulis REGISTER PROPERTI
// TERBENGKALAI untuk Form 17.00 "Daftar Properti Terbengkalai".
//
// Ini BUKAN perubahan data operasional: register adalah rincian properti terbengkalai
// yang bank catat, bukan perhitungan saldo aset tetap. Pembacaan bersifat bank-wide
// (aktor non-lintas cabang ditolak); penulisan dijaga izin system:config di rute dan
// diaudit dalam transaksi yang sama, mengikuti pola register pinjaman yang diterima
// (000117).
//
// Nomor Register bersifat no reuse/no recycle (Form 17.00, PDF #page 231). Karena itu
// penghapusan dilakukan sebagai soft-delete (status -> NONAKTIF), tidak pernah DELETE
// fisik: baris tetap ada dan memegang nomornya.
type propertiRegisterService struct {
	repo   domain.PropertiRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewPropertiRegisterService merakit layanan register properti terbengkalai. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan
// aksi. db dipakai membuka transaksi tulis.
func NewPropertiRegisterService(db *sql.DB, repo domain.PropertiRegisterRepository, auditSinks ...domain.AuditRepository) domain.PropertiRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &propertiRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// PropertiReport menyusun laporan register properti terbengkalai untuk satu posisi.
// Bank-wide: aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *propertiRegisterService) PropertiReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PropertiReport, error) {
	report := domain.PropertiReport{AsOf: asOf.UTC(), Items: []domain.PropertiItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrPropertiBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register properti terbengkalai: repositori tidak tersedia")
	}
	items, err := s.repo.ListPropertiForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register properti terbengkalai: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *propertiRegisterService) ListItems(ctx context.Context) ([]domain.PropertiItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register properti terbengkalai: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register properti terbengkalai: %w", err)
	}
	if items == nil {
		items = []domain.PropertiItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu properti (id kosong = buat baru) dan menulis audit dalam
// satu transaksi. No. Register yang sudah pernah dipakai baris lain — termasuk baris
// NONAKTIF — ditolak repositori dengan ErrPropertiNoRegisterUsed.
func (s *propertiRegisterService) UpsertItem(ctx context.Context, input domain.UpdatePropertiItemInput, actor domain.Actor) (*domain.PropertiItem, error) {
	item, err := domain.BuildPropertiItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_PROPERTI_ITEM", "properti_item",
			item.ID.String(), propertiAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menonaktifkan satu properti (soft-delete, bukan DELETE fisik) dan menulis
// audit. Baris yang tidak ada ditolak ErrPropertiNotFound agar penghapusan tidak tampak
// berhasil.
func (s *propertiRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.SoftDeleteItemTx(ctx, tx, id, actor.UserID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPropertiNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_PROPERTI_ITEM", "properti_item",
			id.String(), map[string]any{"id": id.String(), "status": domain.PropertiStatusNonaktif})
	})
}

// propertiAuditChanges membangun catatan audit ringkas properti yang disimpan.
func propertiAuditChanges(i domain.PropertiItem) map[string]any {
	return map[string]any{
		"no_register":                          i.NoRegister,
		"jenis_properti_code":                  i.JenisPropertiCode,
		"alamat_properti":                      i.AlamatProperti,
		"koordinat":                            i.Koordinat,
		"tanggal_penetapan":                    i.TanggalPenetapan.Format("2006-01-02"),
		"biaya_perolehan_atau_nilai_wajar":     i.BiayaPerolehanNilaiWajar.String(),
		"akumulasi_penyusutan_atau_amortisasi": i.AkumulasiPenyusutanAmortisasi.String(),
		"metode_pengukuran_code":               i.MetodePengukuranCode,
		"as_of":                                i.AsOf.Format("2006-01-02"),
		"status":                               i.Status,
	}
}

var _ domain.PropertiRegisterService = (*propertiRegisterService)(nil)
