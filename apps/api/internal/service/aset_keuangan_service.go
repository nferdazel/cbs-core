package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// asetKeuanganRegisterService menyediakan jalur baca dan tulis REGISTER ASET KEUANGAN
// LAINNYA untuk Form 18.00 "Daftar Aset Keuangan Lainnya".
//
// Ini BUKAN perubahan data operasional: register adalah rincian aset keuangan lainnya
// yang bank catat. Pembacaan bersifat bank-wide (aktor non-lintas cabang ditolak);
// penulisan dijaga izin system:config di rute dan diaudit dalam transaksi yang sama,
// mengikuti pola register penyertaan modal (000120) dan aset tetap (000119).
//
// No. Rekening bersifat unik dan "tidak boleh sama" (Form 18.00, PDF #page 237). Karena
// itu penghapusan dilakukan sebagai soft-delete (status -> NONAKTIF), tidak pernah DELETE
// fisik: baris tetap ada dan memegang nomor rekeningnya.
type asetKeuanganRegisterService struct {
	repo   domain.AsetKeuanganRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewAsetKeuanganRegisterService merakit layanan register aset keuangan lainnya.
// auditRepo variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa
// menggagalkan aksi. db dipakai membuka transaksi tulis.
func NewAsetKeuanganRegisterService(db *sql.DB, repo domain.AsetKeuanganRegisterRepository, auditSinks ...domain.AuditRepository) domain.AsetKeuanganRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &asetKeuanganRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// AsetKeuanganReport menyusun laporan register aset keuangan lainnya untuk satu posisi.
// Bank-wide: aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *asetKeuanganRegisterService) AsetKeuanganReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.AsetKeuanganReport, error) {
	report := domain.AsetKeuanganReport{AsOf: asOf.UTC(), Items: []domain.AsetKeuanganItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrAsetKeuanganBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register aset keuangan lainnya: repositori tidak tersedia")
	}
	items, err := s.repo.ListAsetKeuanganForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register aset keuangan lainnya: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *asetKeuanganRegisterService) ListItems(ctx context.Context) ([]domain.AsetKeuanganItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register aset keuangan lainnya: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register aset keuangan lainnya: %w", err)
	}
	if items == nil {
		items = []domain.AsetKeuanganItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu baris (id kosong = buat baru) dan menulis audit dalam satu
// transaksi. No. Rekening yang sudah pernah dipakai baris lain — termasuk baris NONAKTIF —
// ditolak repositori dengan ErrAsetKeuanganNoRekeningUsed.
func (s *asetKeuanganRegisterService) UpsertItem(ctx context.Context, input domain.UpdateAsetKeuanganItemInput, actor domain.Actor) (*domain.AsetKeuanganItem, error) {
	item, err := domain.BuildAsetKeuanganItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_ASET_KEUANGAN_ITEM", "aset_keuangan_item",
			item.ID.String(), asetKeuanganAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menonaktifkan satu baris (soft-delete, bukan DELETE fisik) dan menulis audit.
// Baris yang tidak ada ditolak ErrAsetKeuanganNotFound agar penghapusan tidak tampak
// berhasil.
func (s *asetKeuanganRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.SoftDeleteItemTx(ctx, tx, id, actor.UserID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrAsetKeuanganNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_ASET_KEUANGAN_ITEM", "aset_keuangan_item",
			id.String(), map[string]any{"id": id.String(), "status": domain.AsetKeuanganStatusNonaktif})
	})
}

// asetKeuanganAuditChanges membangun catatan audit ringkas baris yang disimpan.
func asetKeuanganAuditChanges(i domain.AsetKeuanganItem) map[string]any {
	return map[string]any{
		"no_rekening":                    i.NoRekening,
		"counterparty_id":                i.CounterpartyID,
		"jenis_code":                     i.JenisCode,
		"tanggal_mulai":                  i.TanggalMulai.Format("2006-01-02"),
		"tanggal_jatuh_tempo":            i.TanggalJatuhTempo.Format("2006-01-02"),
		"suku_bunga":                     i.SukuBunga.String(),
		"nominal":                        i.Nominal.String(),
		"nilai_agunan_diperhitungkan":    i.NilaiAgunanDiperhitungkan.String(),
		"ckpn":                           i.CKPN.String(),
		"ckpn_aset_baik":                 i.CKPNAsetBaik.String(),
		"ckpn_aset_kurang_baik":          i.CKPNAsetKurangBaik.String(),
		"ckpn_aset_tidak_baik":           i.CKPNAsetTidakBaik.String(),
		"klasifikasi_aset_keuangan_code": i.KlasifikasiAsetKeuanganCode,
		"jenis_ckpn_code":                i.JenisCKPNCode,
		"as_of":                          i.AsOf.Format("2006-01-02"),
		"status":                         i.Status,
	}
}

var _ domain.AsetKeuanganRegisterService = (*asetKeuanganRegisterService)(nil)
