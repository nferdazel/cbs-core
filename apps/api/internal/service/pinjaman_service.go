package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// pinjamanRegisterService menyediakan jalur baca dan tulis REGISTER PINJAMAN YANG
// DITERIMA untuk Form 00.07 "Daftar Pinjaman yang Diterima".
//
// Ini BUKAN perubahan data operasional: register adalah rincian pinjaman yang bank
// terima dari bank/Bank Indonesia/pihak ketiga bukan bank, bukan perhitungan saldo
// liabilitas. Pembacaan bersifat bank-wide (aktor non-lintas cabang ditolak);
// penulisan dijaga izin system:config di rute dan diaudit dalam transaksi yang sama,
// mengikuti pola register kepemilikan BPR (000116).
type pinjamanRegisterService struct {
	repo   domain.PinjamanRegisterRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewPinjamanRegisterService merakit layanan register pinjaman yang diterima. auditRepo
// variadik mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan
// aksi. db dipakai membuka transaksi tulis.
func NewPinjamanRegisterService(db *sql.DB, repo domain.PinjamanRegisterRepository, auditSinks ...domain.AuditRepository) domain.PinjamanRegisterService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &pinjamanRegisterService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// PinjamanReport menyusun laporan register pinjaman untuk satu posisi. Bank-wide:
// aktor yang tidak berwenang atas seluruh bank ditolak.
func (s *pinjamanRegisterService) PinjamanReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PinjamanReport, error) {
	report := domain.PinjamanReport{AsOf: asOf.UTC(), Items: []domain.PinjamanItem{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrPinjamanBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul register pinjaman yang diterima: repositori tidak tersedia")
	}
	items, err := s.repo.ListPinjamanForOJK(ctx, asOf)
	if err != nil {
		return report, fmt.Errorf("membaca register pinjaman yang diterima: %w", err)
	}
	if items != nil {
		report.Items = items
	}
	return report, nil
}

// ListItems membaca baris mentah register untuk UI edit, urutan deterministik dari
// repositori. Ini bukan laporan: tidak ada pemeriksaan bank-wide di sini karena rute
// pengaturan dijaga izin system:config.
func (s *pinjamanRegisterService) ListItems(ctx context.Context) ([]domain.PinjamanItem, error) {
	if s.repo == nil {
		return nil, fmt.Errorf("modul register pinjaman yang diterima: repositori tidak tersedia")
	}
	items, err := s.repo.ListItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca register pinjaman yang diterima: %w", err)
	}
	if items == nil {
		items = []domain.PinjamanItem{}
	}
	return items, nil
}

// UpsertItem menyimpan satu pinjaman (id kosong = buat baru) dan menulis audit dalam
// satu transaksi.
func (s *pinjamanRegisterService) UpsertItem(ctx context.Context, input domain.UpdatePinjamanItemInput, actor domain.Actor) (*domain.PinjamanItem, error) {
	item, err := domain.BuildPinjamanItem(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	item.CreatedAt, item.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertItemTx(ctx, tx, item, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_PINJAMAN_ITEM", "pinjaman_item",
			item.ID.String(), pinjamanAuditChanges(item))
	}); err != nil {
		return nil, err
	}
	return &item, nil
}

// DeleteItem menghapus satu pinjaman dan menulis audit. Baris yang tidak ada ditolak
// ErrPinjamanNotFound agar penghapusan tidak tampak berhasil.
func (s *pinjamanRegisterService) DeleteItem(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteItemTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrPinjamanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_PINJAMAN_ITEM", "pinjaman_item",
			id.String(), map[string]any{"id": id.String()})
	})
}

// pinjamanAuditChanges membangun catatan audit ringkas pinjaman yang disimpan.
func pinjamanAuditChanges(i domain.PinjamanItem) map[string]any {
	return map[string]any{
		"counterparty_id":              i.CounterpartyID,
		"creditor_group_code":          i.CreditorGroupCode,
		"bank_code":                    i.BankCode,
		"location_code":                i.LocationCode,
		"jenis_code":                   i.JenisCode,
		"relationship_code":            i.RelationshipCode,
		"start_date":                   i.StartDate.Format("2006-01-02"),
		"maturity_date":                i.MaturityDate.Format("2006-01-02"),
		"interest_rate":                i.InterestRate.String(),
		"interest_calc_code":           i.InterestCalcCode,
		"plafon":                       i.Plafon.String(),
		"collateral_type_code":         i.CollateralTypeCode,
		"collateral_amount":            i.CollateralAmount.String(),
		"baki_debet":                   i.BakiDebet.String(),
		"unamortized_transaction_cost": i.UnamortizedTransactionCost.String(),
		"unamortized_discount":         i.UnamortizedDiscount.String(),
		"as_of":                        i.AsOf.Format("2006-01-02"),
		"status":                       i.Status,
	}
}

var _ domain.PinjamanRegisterService = (*pinjamanRegisterService)(nil)
