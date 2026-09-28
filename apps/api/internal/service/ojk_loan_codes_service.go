package service

import (
	"context"
	"database/sql"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// ojk_loan_codes_service.go menyediakan pintu masuk PENGATURAN sandi OJK per kredit
// tanpa SQL: jenis penggunaan (VIII, inline), periode pembayaran (XI, inline), dan
// kabupaten lokasi penggunaan (XXII, FK ojk_kabupaten). Menggantikan pengisian lewat
// SQL/seed menjadi perubahan berizin + teraudit.
//
// Ini BUKAN perubahan data kredit yang menyentuh uang: hanya atribut laporan. Karena
// itu layanan ini berdiri sendiri dan tidak mengubah mesin kredit/jadwal angsuran.
type ojkLoanCodesService struct {
	db     *sql.DB
	store  domain.OJKLoanCodesStore
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewOJKLoanCodesService merakit layanan sandi OJK kredit. auditRepo variadik
// mengikuti pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi.
func NewOJKLoanCodesService(db *sql.DB, store domain.OJKLoanCodesStore, auditSinks ...domain.AuditRepository) domain.OJKLoanCodesService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &ojkLoanCodesService{db: db, store: store, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// Update memvalidasi, menyimpan, dan mengaudit sandi OJK satu kredit dalam satu
// transaksi. Bidang yang tidak dikirim tidak diubah; kredit di luar cakupan unit
// aktor ditolak ErrCrossBranchAccess.
func (s *ojkLoanCodesService) Update(ctx context.Context, loanID uuid.UUID, input domain.UpdateOJKLoanCodesInput, actor domain.Actor) (*domain.Loan, error) {
	if input.IsEmpty() {
		return nil, domain.ErrOJKLoanCodesEmpty
	}
	if err := input.Validate(); err != nil {
		return nil, err
	}

	loan, err := s.store.GetByID(ctx, loanID)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessBranch(loan.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}

	// Sandi referensi diperiksa terhadap tabel ojk_* lebih dulu agar pesannya
	// menyebut kolom; foreign key tetap menjadi penjaga terakhir.
	if s.db != nil && input.OJKKabupatenCode != nil {
		if err := requireOJKReference(ctx, s.db, domain.OJKRefKabupaten, *input.OJKKabupatenCode, domain.OJKKabupatenField); err != nil {
			return nil, err
		}
	}
	if s.db != nil && input.OJKPenjaminCode != nil {
		if err := requireOJKReference(ctx, s.db, domain.OJKRefPihakLawan, *input.OJKPenjaminCode, domain.OJKPenjaminField); err != nil {
			return nil, err
		}
	}

	changes := ojkLoanCodesChanges(loan, input)
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.store.UpdateOJKLoanCodesTx(ctx, tx, loanID, input); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPDATE_OJK_LOAN_CODES", "loan", loanID.String(), changes)
	}); err != nil {
		return nil, err
	}
	return s.store.GetByID(ctx, loanID)
}

// ojkLoanCodesChanges mencatat nilai sebelum -> sesudah untuk bidang yang benar-benar
// berubah, sehingga audit tidak memuat perubahan semu.
func ojkLoanCodesChanges(before *domain.Loan, input domain.UpdateOJKLoanCodesInput) map[string]any {
	out := map[string]any{}
	if before == nil {
		before = &domain.Loan{}
	}
	add := func(field, old string, next *string) {
		if next == nil {
			return
		}
		nv := strings.TrimSpace(*next)
		if nv != old {
			out[field] = map[string]any{"before": old, "after": nv}
		}
	}
	add(domain.OJKJenisPenggunaanField, before.OJKJenisPenggunaanCode, input.OJKJenisPenggunaanCode)
	add(domain.OJKPeriodePembayaranField, before.OJKPeriodePembayaranCode, input.OJKPeriodePembayaranCode)
	add(domain.OJKKabupatenField, before.OJKKabupatenCode, input.OJKKabupatenCode)
	add(domain.OJKKelompokKreditField, before.OJKKelompokKreditCode, input.OJKKelompokKreditCode)
	add(domain.OJKSumberDanaField, before.OJKSumberDanaCode, input.OJKSumberDanaCode)
	add(domain.OJKKategoriUsahaField, before.OJKKategoriUsahaCode, input.OJKKategoriUsahaCode)
	add(domain.OJKSifatKreditField, before.OJKSifatKreditCode, input.OJKSifatKreditCode)
	add(domain.OJKPenjaminField, before.OJKPenjaminCode, input.OJKPenjaminCode)
	add(domain.OJKKlasifikasiAsetField, before.OJKKlasifikasiAsetCode, input.OJKKlasifikasiAsetCode)
	ojkAddPercentageChange(out, domain.OJKPenjaminBagianPctField, before.OJKPenjaminBagianPct, input.OJKPenjaminBagianPct)
	ojkAddDateChange(out, domain.OJKTanggalMulaiMacetField, before.OJKTanggalMulaiMacet, input.OJKTanggalMulaiMacet)
	ojkAddAmountChange(out, domain.OJKAgunanPPKAAmountField, before.OJKAgunanPPKAAmount, input.OJKAgunanPPKAAmount)
	ojkAddAmountChange(out, domain.OJKKelonggaranTarikAmountField, before.OJKKelonggaranTarikAmount, input.OJKKelonggaranTarikAmount)
	ojkAddAmountChange(out, domain.OJKProvisiBelumDiamortisasiAmountField, before.OJKProvisiBelumDiamortisasiAmount, input.OJKProvisiBelumDiamortisasiAmount)
	ojkAddAmountChange(out, domain.OJKBiayaTransaksiBelumDiamortisasiAmountField, before.OJKBiayaTransaksiBelumDiamortisasiAmount, input.OJKBiayaTransaksiBelumDiamortisasiAmount)
	ojkAddAmountChange(out, domain.OJKPendapatanBungaDitangguhkanAmountField, before.OJKPendapatanBungaDitangguhkanAmount, input.OJKPendapatanBungaDitangguhkanAmount)
	ojkAddAmountChange(out, domain.OJKCadanganKerugianRestrukturisasiAmountField, before.OJKCadanganKerugianRestrukturisasiAmount, input.OJKCadanganKerugianRestrukturisasiAmount)
	return out
}
