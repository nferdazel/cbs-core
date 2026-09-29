package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// asetTetapRegisterService menyediakan jalur baca dan tulis REGISTER ASET TETAP,
// INVENTARIS, DAN ASET TIDAK BERWUJUD untuk Form 08.00 "Daftar Aset Tetap, Inventaris,
// dan Aset Tidak Berwujud".
//
// Ini BUKAN perubahan data operasional: register adalah rincian aset yang bank catat,
// bukan perhitungan saldo aset tetap. Pembacaan bersifat bank-wide (aktor non-lintas
// cabang ditolak); penulisan dijaga izin system:config di rute dan diaudit dalam
// transaksi yang sama, mengikuti pola register pinjaman yang diterima (000117).
//
// Form 08.00 tidak mengatur no reuse/no recycle nomor register, sehingga penghapusan
// adalah DELETE fisik lewat DeleteItemTx (pola register pinjaman).
type asetTetapRegisterService struct {
	repo   domain.AsetTetapRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewAsetTetapRegisterService merakit layanan register aset. auditRepo variadik
// mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi. db
// dipakai membuka transaksi tulis.
func NewAsetTetapRegisterService(db *sql.DB, repo domain.AsetTetapRegisterRepository, auditSinks ...domain.AuditRepository) domain.AsetTetapRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &asetTetapRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// AsetTetapReport menyusun laporan register aset untuk satu posisi. Bank-wide: aktor
// yang tidak berwenang atas seluruh bank ditolak.
func (s *asetTetapRegisterService) AsetTetapReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.AsetTetapReport, error) {
	report := domain.AsetTetapReport{AsOf: asOf.UTC(), Items: []domain.AsetTetapItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrAsetTetapBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register aset tetap: repositori tidak tersedia")
	}
	items, err := s.repo.ListAsetTetapForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register aset tetap: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *asetTetapRegisterService) ListItems(ctx context.Context) ([]domain.AsetTetapItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register aset tetap: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register aset tetap: %w", err)
	}
	if items == nil {
		items = []domain.AsetTetapItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu aset (id kosong = buat baru) dan menulis audit dalam satu
// transaksi.
func (s *asetTetapRegisterService) UpsertItem(ctx context.Context, input domain.UpdateAsetTetapItemInput, actor domain.Actor) (*domain.AsetTetapItem, error) {
	item, err := domain.BuildAsetTetapItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_ASET_TETAP_ITEM", "aset_tetap_item",
			item.ID.String(), asetTetapAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu aset dan menulis audit. Baris yang tidak ada ditolak
// ErrAsetTetapNotFound agar penghapusan tidak tampak berhasil.
func (s *asetTetapRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrAsetTetapNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_ASET_TETAP_ITEM", "aset_tetap_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// asetTetapAuditChanges membangun catatan audit ringkas aset yang disimpan.
func asetTetapAuditChanges(i domain.AsetTetapItem) map[string]any {
	return map[string]any{
		"jenis_aset_code":                    i.JenisAsetCode,
		"sumber_perolehan_code":              i.SumberPerolehanCode,
		"status_aset_code":                   i.StatusAsetCode,
		"biaya_perolehan":                    i.BiayaPerolehan.String(),
		"akumulasi_penyusutan_amortisasi":    i.AkumulasiPenyusutanAmortisasi.String(),
		"akumulasi_kerugian_penurunan_nilai": i.AkumulasiKerugianPenurunanNilai.String(),
		"metode_pengukuran_code":             i.MetodePengukuranCode,
		"as_of":                              i.AsOf.Format("2006-01-02"),
		"status":                             i.Status,
	}
}

var _ domain.AsetTetapRegisterService = (*asetTetapRegisterService)(nil)
