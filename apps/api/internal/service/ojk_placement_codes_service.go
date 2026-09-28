package service

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ojk_placement_codes_service.go menyediakan pintu masuk PENGATURAN sandi OJK per
// penempatan pada bank lain (Form 05.00) tanpa SQL: lokasi bank lawan (III),
// hubungan bank (V), alasan diblokir (XI), CIF pihak lawan (XVI), klasifikasi aset
// (XX), serta nominal diblokir (X) dan pendapatan bunga akrual/penyelesaian
// (XIII/XIV). Menggantikan pengisian lewat SQL/seed menjadi perubahan berizin +
// teraudit.
//
// Ini BUKAN perubahan data yang menyentuh uang: hanya atribut laporan. Layanan ini
// tidak mengubah asesmen CKPN penempatan maupun perhitungan PPKA.
type ojkPlacementCodesService struct {
	db     *sql.DB
	store  domain.OJKPlacementCodesStore
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewOJKPlacementCodesService merakit layanan sandi OJK penempatan. auditRepo variadik
// mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi.
func NewOJKPlacementCodesService(db *sql.DB, store domain.OJKPlacementCodesStore, auditSinks ...domain.AuditRepository) domain.OJKPlacementCodesService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &ojkPlacementCodesService{db: db, store: store, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// Update memvalidasi, menyimpan, dan mengaudit sandi OJK satu penempatan dalam satu
// transaksi. Bidang yang tidak dikirim tidak diubah; penempatan di luar cakupan unit
// aktor ditolak ErrCrossBranchAccess mengikuti pola scope lps_placement.
func (s *ojkPlacementCodesService) Update(ctx context.Context, placementID uuid.UUID, input domain.UpdateOJKPlacementCodesInput, actor domain.Actor) (*domain.LPSPlacement, error) {
	if input.IsEmpty() {
		return nil, domain.ErrOJKPlacementCodesEmpty
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}

	placement, err := s.store.GetPlacementByID(ctx, placementID)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessBranch(placement.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}

	// Sandi referensi diperiksa terhadap tabel ojk_* lebih dulu agar pesannya menyebut
	// kolom; foreign key tetap menjadi penjaga terakhir.
	if s.db != nil && input.OJKKabupatenCode != nil {
		if err := requireOJKReference(ctx, s.db, domain.OJKRefKabupaten, *input.OJKKabupatenCode, domain.OJKKabupatenField); err != nil {
			return nil, err
		}
	}

	changes := ojkPlacementCodesChanges(placement, input)
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.store.UpdateOJKPlacementCodesTx(ctx, tx, placementID, input); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPDATE_OJK_PLACEMENT_CODES", "lps_placement", placementID.String(), changes)
	}); err != nil {
		return nil, err
	}
	return s.store.GetPlacementByID(ctx, placementID)
}

// Get mengembalikan nilai sandi OJK satu penempatan yang tersimpan. Penempatan di
// luar cakupan unit aktor ditolak ErrCrossBranchAccess.
func (s *ojkPlacementCodesService) Get(ctx context.Context, placementID uuid.UUID, actor domain.Actor) (domain.OJKPlacementCodesView, error) {
	placement, err := s.store.GetPlacementByID(ctx, placementID)
	if err != nil {
		return domain.OJKPlacementCodesView{}, err
	}
	if !actor.CanAccessBranch(placement.BranchCode) {
		return domain.OJKPlacementCodesView{}, domain.ErrCrossBranchAccess
	}
	return domain.NewOJKPlacementCodesView(*placement), nil
}

// ojkPlacementCodesChanges mencatat nilai sebelum -> sesudah untuk bidang yang
// benar-benar berubah, sehingga audit tidak memuat perubahan semu.
func ojkPlacementCodesChanges(before *domain.LPSPlacement, input domain.UpdateOJKPlacementCodesInput) map[string]any {
	out := map[string]any{}
	if before == nil {
		before = &domain.LPSPlacement{}
	}
	ojkAddCodeChange(out, domain.OJKKabupatenField, before.OJKKabupatenCode, input.OJKKabupatenCode)
	ojkAddCodeChange(out, domain.OJKHubunganBankField, before.OJKHubunganBankCode, input.OJKHubunganBankCode)
	ojkAddCodeChange(out, domain.OJKAlasanDiblokirField, before.OJKAlasanDiblokirCode, input.OJKAlasanDiblokirCode)
	ojkAddCodeChange(out, domain.OJKCounterpartyCIFField, before.CounterpartyCIF, input.CounterpartyCIF)
	ojkAddCodeChange(out, domain.OJKKlasifikasiAsetField, before.OJKKlasifikasiAsetCode, input.OJKKlasifikasiAsetCode)
	ojkAddAmountChange(out, domain.OJKBlockedAmountField, before.BlockedAmount, input.BlockedAmount)
	ojkAddAmountChange(out, domain.OJKAccruedInterestReceivableField, before.AccruedInterestReceivable, input.AccruedInterestReceivable)
	ojkAddAmountChange(out, domain.OJKAccruedInterestPendingField, before.AccruedInterestPending, input.AccruedInterestPending)
	return out
}

// ojkAddCodeChange mencatat perubahan sandi/teks bila nilai barunya berbeda.
func ojkAddCodeChange(out map[string]any, field, old string, next *string) {
	if next == nil {
		return
	}
	nv := strings.TrimSpace(*next)
	if nv != old {
		out[field] = map[string]any{"before": old, "after": nv}
	}
}

// ojkAddAmountChange mencatat perubahan nominal (string masukan -> desimal nullable).
// Nilai yang gagal diurai diabaikan karena validasi sudah berjalan lebih dulu.
func ojkAddAmountChange(out map[string]any, field string, old *decimal.Decimal, next *string) {
	if next == nil {
		return
	}
	nd, err := domain.ParseOJKAmount(field, *next)
	if err != nil {
		return
	}
	var nv *decimal.Decimal
	if nd.Valid {
		v := nd.Decimal
		nv = &v
	}
	before, after := ojkDecimalString(old), ojkDecimalString(nv)
	if before == after {
		return
	}
	out[field] = map[string]any{"before": before, "after": after}
}

// ojkAddPercentageChange mencatat perubahan persentase nullable.
func ojkAddPercentageChange(out map[string]any, field string, old *decimal.Decimal, next *string) {
	if next == nil {
		return
	}
	nd, err := domain.ParseOJKPercentage(field, *next)
	if err != nil {
		return
	}
	var nv *decimal.Decimal
	if nd.Valid {
		v := nd.Decimal
		nv = &v
	}
	before, after := ojkDecimalString(old), ojkDecimalString(nv)
	if before == after {
		return
	}
	out[field] = map[string]any{"before": before, "after": after}
}

// ojkAddDateChange mencatat perubahan tanggal nullable (format YYYY-MM-DD).
func ojkAddDateChange(out map[string]any, field string, old *time.Time, next *string) {
	if next == nil {
		return
	}
	nt, err := domain.ParseOJKDate(field, *next)
	if err != nil {
		return
	}
	before, after := ojkDateString(old), ojkDateString(nt)
	if before == after {
		return
	}
	out[field] = map[string]any{"before": before, "after": after}
}

// ojkDecimalString mengubah nominal nullable menjadi string untuk audit; nil tetap nil.
func ojkDecimalString(d *decimal.Decimal) any {
	if d == nil {
		return nil
	}
	return d.String()
}

// ojkDateString mengubah tanggal nullable menjadi YYYY-MM-DD untuk audit; nil tetap nil.
func ojkDateString(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Format("2006-01-02")
}
