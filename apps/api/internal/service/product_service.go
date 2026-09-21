package service

import (
	"context"
	"database/sql"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type productService struct {
	repo      domain.ProductParamRepository
	auditRepo domain.AuditRepository
	// txRunner membungkus penulisan parameter dan auditnya dalam SATU transaksi.
	// Lewat interface agar dapat diganti stub pada test unit tanpa database.
	txRunner ppapTxRunner
}

// NewProductService merakit layanan produk. auditRepo bersifat variadik agar
// pemanggil lama tanpa audit tetap dapat memakai konstruktor ini (auditRepo nil =
// penulisan audit dilewati, tidak menggagalkan aksi bisnis).
func NewProductService(db *sql.DB, repo domain.ProductParamRepository, auditSinks ...domain.AuditRepository) domain.ProductService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &productService{repo: repo, auditRepo: auditRepo, txRunner: sqlPPAPTxRunner{db: db}}
}

func (s *productService) ListProducts(ctx context.Context) ([]domain.BankingProduct, error) {
	return s.repo.List(ctx)
}

func (s *productService) GetProduct(ctx context.Context, id uuid.UUID) (*domain.BankingProduct, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *productService) GetMapping(ctx context.Context, productID uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return s.repo.GetMapping(ctx, productID, event)
}

// UpdateParams mengubah parameter produk (suku bunga/margin, nisbah, proyeksi
// pendapatan, batas plafon/tenor, biaya admin, pajak, penalti) berdasarkan kode
// produk. Hanya parameter yang disebut di payload yang diubah; bidang lain
// dipertahankan dari nilai tersimpan.
//
// Identitas produk (kode, jenis, mata uang/buku, skema laba, metode jadwal) dan
// penghapusan produk tidak didukung lewat jalur ini: keduanya mengubah arti transaksi
// lama. Perubahan hanya memengaruhi tampilan/operasi baru, tidak memigrasi ulang
// transaksi yang sudah tercatat.
//
// Setiap perubahan dan nilai sebelum -> sesudahnya dicatat dalam satu transaksi audit.
func (s *productService) UpdateParams(ctx context.Context, code string, input domain.UpdateProductParamsInput, actor domain.Actor) (*domain.BankingProduct, error) {
	if input.IsEmpty() {
		return nil, domain.ErrProductParamsEmpty
	}

	// Produk dibaca DI DALAM transaksi yang menulis (baris terkunci), lalu
	// dibandingkan dengan hasil penggabungan. Dengan begitu audit "before" selalu
	// mencerminkan keadaan yang benar-benar ditimpa, bukan potret di luar transaksi
	// yang bisa sudah basi saat permintaan bersamaan datang.
	var updated *domain.BankingProduct
	err := s.txRunner.Run(ctx, func(tx any) error {
		product, err := s.repo.GetByCodeTx(ctx, tx, code)
		if err != nil {
			return err
		}

		// Bekerja pada salinan: bila validasi gagal, objek yang dikembalikan
		// repositori tidak boleh tertinggal dalam keadaan separuh berubah.
		before := *product
		candidate := before
		if err := applyProductParams(&candidate, input); err != nil {
			return err
		}
		if err := validateProductParams(&candidate); err != nil {
			return err
		}

		changes := productParamChanges(before, candidate)
		if len(changes) == 0 {
			// Tidak ada parameter yang benar-benar berubah. PUT idempoten tetap
			// sukses, tetapi tidak menulis baris maupun event audit yang bisa
			// disalahbaca pembaca audit sebagai perubahan parameter.
			updated = &candidate
			return nil
		}
		changes["code"] = candidate.Code

		if err := s.repo.UpdateParamsTx(ctx, tx, &candidate); err != nil {
			return err
		}
		if err := writeAudit(ctx, s.auditRepo, tx, actor, "UPDATE_PRODUCT_PARAMS", "product", candidate.ID.String(), changes); err != nil {
			return err
		}
		updated = &candidate
		return nil
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// applyProductParams menyalin parameter yang dikirim ke produk, menolak nilai
// negatif dengan pesan yang menyebut satuannya. Rentang nisbah dan proyeksi bagi
// hasil memakai validator domain yang sama dengan pembentukan jadwal agar ambangnya
// satu sumber kebenaran.
func applyProductParams(p *domain.BankingProduct, in domain.UpdateProductParamsInput) error {
	if in.RateAnnual != nil {
		if err := domain.ValidateProductRateAnnual(*in.RateAnnual); err != nil {
			return err
		}
		p.RateAnnual = *in.RateAnnual
	}
	if in.ProfitSharingRatio != nil {
		if err := domain.ValidateProfitSharingRatio(*in.ProfitSharingRatio); err != nil {
			return err
		}
		p.ProfitSharingRatio = *in.ProfitSharingRatio
	}
	if in.ProjectedRevenueRateAnnual != nil {
		if err := domain.ValidateProjectedRevenueRateAnnual(*in.ProjectedRevenueRateAnnual); err != nil {
			return err
		}
		p.ProjectedRevenueRateAnnual = *in.ProjectedRevenueRateAnnual
	}
	if in.MinAmount != nil {
		if in.MinAmount.IsNegative() {
			return domain.ErrProductMinAmountNegative
		}
		p.MinAmount = *in.MinAmount
	}
	if in.MaxAmount != nil {
		if in.MaxAmount.IsNegative() {
			return domain.ErrProductMaxAmountNegative
		}
		p.MaxAmount = *in.MaxAmount
	}
	if in.MinTermMonths != nil {
		if *in.MinTermMonths < 0 {
			return domain.ErrProductTermNegative
		}
		p.MinTermMonths = *in.MinTermMonths
	}
	if in.MaxTermMonths != nil {
		if *in.MaxTermMonths < 0 {
			return domain.ErrProductTermNegative
		}
		p.MaxTermMonths = *in.MaxTermMonths
	}
	if in.AdminFee != nil {
		if err := domain.ValidateProductAdminFee(*in.AdminFee); err != nil {
			return err
		}
		p.AdminFee = *in.AdminFee
	}
	if in.TaxRate != nil {
		if err := domain.ValidateProductTaxRate(*in.TaxRate); err != nil {
			return err
		}
		p.TaxRate = *in.TaxRate
	}
	if in.EarlyWithdrawalPenaltyRate != nil {
		if err := domain.ValidateProductEarlyWithdrawalPenaltyRate(*in.EarlyWithdrawalPenaltyRate); err != nil {
			return err
		}
		p.EarlyWithdrawalPenaltyRate = *in.EarlyWithdrawalPenaltyRate
	}
	return nil
}

// validateProductParams memeriksa hubungan antar-bidang setelah penggabungan.
// max_amount 0 berarti tanpa batas; selain itu harus >= min_amount. Tenor maksimum
// 0 berarti tidak berlaku; selain itu minimum tidak boleh melebihi maksimum.
func validateProductParams(p *domain.BankingProduct) error {
	if p.MaxAmount.IsPositive() && p.MaxAmount.LessThan(p.MinAmount) {
		return domain.ErrProductAmountRange
	}
	if p.MaxTermMonths != 0 && p.MinTermMonths > p.MaxTermMonths {
		return domain.ErrProductTermRange
	}
	return nil
}

// productParamChanges membangun catatan audit nilai sebelum -> sesudah untuk setiap
// parameter yang benar-benar berubah.
func productParamChanges(before, after domain.BankingProduct) map[string]any {
	changes := map[string]any{}
	add := func(field string, b, a any, changed bool) {
		if changed {
			changes[field] = map[string]any{"before": b, "after": a}
		}
	}
	add("rate_annual", before.RateAnnual, after.RateAnnual, !before.RateAnnual.Equal(after.RateAnnual))
	add("profit_sharing_ratio", before.ProfitSharingRatio, after.ProfitSharingRatio, !before.ProfitSharingRatio.Equal(after.ProfitSharingRatio))
	add("projected_revenue_rate_annual", before.ProjectedRevenueRateAnnual, after.ProjectedRevenueRateAnnual, !before.ProjectedRevenueRateAnnual.Equal(after.ProjectedRevenueRateAnnual))
	add("min_amount", before.MinAmount, after.MinAmount, !before.MinAmount.Equal(after.MinAmount))
	add("max_amount", before.MaxAmount, after.MaxAmount, !before.MaxAmount.Equal(after.MaxAmount))
	add("min_term_months", before.MinTermMonths, after.MinTermMonths, before.MinTermMonths != after.MinTermMonths)
	add("max_term_months", before.MaxTermMonths, after.MaxTermMonths, before.MaxTermMonths != after.MaxTermMonths)
	add("admin_fee", before.AdminFee, after.AdminFee, !before.AdminFee.Equal(after.AdminFee))
	add("tax_rate", before.TaxRate, after.TaxRate, !before.TaxRate.Equal(after.TaxRate))
	add("early_withdrawal_penalty_rate", before.EarlyWithdrawalPenaltyRate, after.EarlyWithdrawalPenaltyRate, !before.EarlyWithdrawalPenaltyRate.Equal(after.EarlyWithdrawalPenaltyRate))
	return changes
}
