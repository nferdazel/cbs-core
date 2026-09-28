package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// aydaRegisterService menyediakan jalur baca dan tulis REGISTER AYDA (Agunan yang
// Diambil Alih) untuk Form 07.00.
//
// Ini BUKAN perubahan data operasional yang menyentuh uang: saldo buku besar AYDA tetap
// akun COA 10500. Register adalah rincian per kasus. Pembacaan bersifat bank-wide
// (aktor non-lintas cabang ditolak); penulisan dijaga izin system:config di rute dan
// diaudit dalam transaksi yang sama, mengikuti pola rekening administratif (000113).
type aydaRegisterService struct {
	repo   domain.AYDARegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewAYDARegisterService merakit layanan register AYDA. auditRepo variadik mengikuti
// pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi. db dipakai
// membuka transaksi tulis.
func NewAYDARegisterService(db *sql.DB, repo domain.AYDARegisterRepository, auditSinks ...domain.AuditRepository) domain.AYDARegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &aydaRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// AYDAReport menyusun laporan register AYDA untuk satu posisi. Bank-wide: aktor yang
// tidak berwenang atas seluruh bank ditolak.
func (s *aydaRegisterService) AYDAReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.AYDAReport, error) {
	report := domain.AYDAReport{AsOf: asOf.UTC(), Items: []domain.AYDAItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrAYDABankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register AYDA: repositori tidak tersedia")
	}
	items, err := s.repo.ListAYDAForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register AYDA: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *aydaRegisterService) ListItems(ctx context.Context) ([]domain.AYDAItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register AYDA: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register AYDA: %w", err)
	}
	if items == nil {
		items = []domain.AYDAItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu AYDA (id kosong = buat baru) dan menulis audit dalam satu
// transaksi.
func (s *aydaRegisterService) UpsertItem(ctx context.Context, input domain.UpdateAYDAItemInput, actor domain.Actor) (*domain.AYDAItem, error) {
	item, err := domain.BuildAYDAItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_AYDA_ITEM", "ayda_item",
			item.ID.String(), aydaAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu AYDA dan menulis audit. Baris yang tidak ada ditolak
// ErrAYDANotFound agar penghapusan tidak tampak berhasil.
func (s *aydaRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrAYDANotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_AYDA_ITEM", "ayda_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// aydaAuditChanges membangun catatan audit ringkas AYDA yang disimpan.
func aydaAuditChanges(i domain.AYDAItem) map[string]any {
	return map[string]any{
		"collateral_type_code":      i.CollateralTypeCode,
		"collateral_address":        i.CollateralAddress,
		"acquisition_date":          i.AcquisitionDate.Format("2006-01-02"),
		"initial_recognition_value": i.InitialRecognitionValue.String(),
		"accumulated_impairment":    i.AccumulatedImpairment.String(),
		"net_realizable_value":      i.NetRealizableValue.String(),
		"as_of":                     i.AsOf.Format("2006-01-02"),
		"status":                    i.Status,
	}
}

var _ domain.AYDARegisterService = (*aydaRegisterService)(nil)
