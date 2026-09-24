package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kerugian restrukturisasi kredit menurut Pasal 32 POJK No. 1 Tahun 2024 jo.
// SEOJK No. 21/SEOJK.03/2024 (PA BPR) Bab 5.2:
//
//	Selisih kurang antara perubahan estimasi arus kas atas Restrukturisasi Kredit
//	dibandingkan dengan nilai tercatat diperhitungkan sebagai kerugian kredit.
//	Nilai kini dihitung dengan tingkat diskonto suku bunga efektif orisinal.
//
// Berkas ini menyediakan parameter kebijakan, perhitungan, dan jurnalnya. Yang berada
// di balik saklar loan.restructure.loss.enabled yang MATI secara bawaan hanyalah
// PENGAKUAN KERUGIAN beserta amortisasinya; saat mati tidak ada perhitungan kerugian,
// jurnal, atau perubahan perilaku restrukturisasi. Pencatatan EIR orisinal TIDAK lagi
// di belakang saklar: ia selalu dilakukan saat pencairan (lihat DisburseLoan) agar
// kredit yang cair sebelum bank menyalakan saklar tetap punya EIR.
const (
	cfgRestructureLossEnabled      = "loan.restructure.loss.enabled"
	cfgRestructureLossDiscountRate = "loan.restructure.loss.discount_rate_annual_pct"
	cfgRestructureLossExpenseCOA   = "loan.restructure.loss.coa.expense"
	cfgRestructureLossLoanCOA      = "loan.restructure.loss.coa.loan"
	// cfgRestructureLossAmortIncomeCOA adalah sisi KREDIT jurnal amortisasi
	// (pendapatan bunga). Dipisah dari COA beban karena jurnal amortisasi adalah
	// Db. Kredit yang diberikan / Kr. Pendapatan bunga (PA BPR hlm. 61), bukan jurnal
	// pembentukan kerugian.
	cfgRestructureLossAmortIncomeCOA = "loan.restructure.loss.amortization.coa.income"
)

// COA bawaan (di-seed migrasi 000043). Beban kerugian restrukturisasi sengaja TERPISAH
// dari beban CKPN (50301) dan PPAP (50200) karena PA BPR hlm. 144-145 menyajikan
// "beban kerugian restrukturisasi kredit" sebagai pos tersendiri. Sisi kredit adalah
// akun kredit yang diberikan per buku; produk syariah murabahah dapat memakai akun
// tersendiri lewat pemetaan jurnal produk.
const (
	fallbackRestructureLossExpenseConventional = "50401"
	fallbackRestructureLossExpenseSyariah      = "15902"
	fallbackRestructureLossLoanConventional    = "10301"
	fallbackRestructureLossLoanSyariah         = "11400"
	// Pendapatan bunga/bagi hasil untuk amortisasi. Konvensional memakai 40100
	// (Pendapatan Bunga Kredit); syariah memakai 14100 (Pendapatan Margin Murabahah).
	// Produk syariah lain dapat menggantinya lewat kunci konfigurasi di atas.
	fallbackRestructureLossAmortIncomeConventional = "40100"
	fallbackRestructureLossAmortIncomeSyariah      = "14100"
)

// restructureLossPolicy adalah parameter kebijakan yang dibaca dari system_config.
type restructureLossPolicy struct {
	Enabled bool
	// HasDiscountOverride true berarti bank mengisi suku bunga diskonto sendiri
	// (mis. kebijakan internal). Bila tidak, EIR orisinal tersimpan yang dipakai.
	HasDiscountOverride bool
	DiscountAnnual      decimal.Decimal
	// DiscountErr menyimpan nilai yang DIISI tetapi tidak sah (salah format/rentang),
	// agar perhitungan menolak dengan pesan jelas alih-alih memakai nol.
	DiscountErr error
}

// restructureLossPolicy membaca parameter kebijakan. Saklar mati berarti tidak ada
// pembacaan parameter lain sama sekali.
func (s *loanService) restructureLossPolicy(ctx context.Context) restructureLossPolicy {
	var p restructureLossPolicy
	if s.config == nil {
		return p
	}
	p.Enabled = s.config.GetBool(ctx, cfgRestructureLossEnabled, false)
	if !p.Enabled {
		return p
	}

	v, set, err := configDecimal(ctx, s.config, cfgRestructureLossDiscountRate)
	switch {
	case err != nil:
		p.DiscountErr = fmt.Errorf("%w: suku bunga diskonto tahunan (kunci %s): %v",
			domain.ErrRestructureLossParamInvalid, cfgRestructureLossDiscountRate, err)
	case set && (v.LessThanOrEqual(decimal.Zero) || v.GreaterThan(decimal.NewFromInt(100))):
		p.DiscountErr = fmt.Errorf("%w: suku bunga diskonto tahunan (kunci %s) harus > 0 dan <= 100 (persen), bukan %s",
			domain.ErrRestructureLossParamInvalid, cfgRestructureLossDiscountRate, v)
	case set:
		p.HasDiscountOverride = true
		p.DiscountAnnual = v
	}
	return p
}

// restructureLossDiscountRate menentukan tingkat diskonto bulanan (fraksi). EIR
// orisinal tersimpan adalah sumber utama; override konfigurasi hanya dipakai bila bank
// benar-benar mengisinya. Kredit yang belum punya EIR ditolak dengan ErrEIRMissing,
// bukan dihitung dengan suku bunga kontraktual.
func (s *loanService) restructureLossDiscountRate(_ context.Context, loan *domain.Loan, policy restructureLossPolicy) (decimal.Decimal, error) {
	if policy.DiscountErr != nil {
		return decimal.Zero, policy.DiscountErr
	}
	if policy.HasDiscountOverride {
		// Konvensi sistem: persen per tahun dibagi 12, sama dengan pembentukan anuitas.
		return policy.DiscountAnnual.Div(decimal.NewFromInt(100)).Div(decimal.NewFromInt(12)), nil
	}
	if !loan.OriginalEIRMonthly.IsPositive() {
		return decimal.Zero, fmt.Errorf("%w: kredit %s; %s",
			domain.ErrEIRMissing, loan.LoanNumber, eirUnavailableHint(loan, cfgRestructureLossDiscountRate))
	}
	return loan.OriginalEIRMonthly, nil
}

// eirUnavailableHint menjelaskan mengapa EIR tidak dapat dipakai dan membedakan dua
// sebab yang berbeda agar pesannya tepat:
//   - Method = IRR_UNAVAILABLE: perhitungan sudah dicoba saat pencairan tetapi gagal;
//     alasannya tersimpan pada basis audit.
//   - selain itu: kredit lama yang dicairkan sebelum pencatatan EIR selalu aktif,
//     sehingga EIR memang belum pernah dihitung.
//
// Keduanya menyebut jalan keluar (override diskonto) agar operator tidak menebak.
func eirUnavailableHint(loan *domain.Loan, overrideKey string) string {
	if loan.OriginalEIRMethod == domain.RestructureLossEIRUnavailableMethod {
		if reason := eirBasisReason(loan.OriginalEIRBasis); reason != "" {
			return fmt.Sprintf("EIR tidak tersedia: perhitungan saat pencairan gagal (%s); isi %s sebagai override",
				reason, overrideKey)
		}
		return fmt.Sprintf("EIR tidak tersedia: perhitungan saat pencairan gagal; isi %s sebagai override", overrideKey)
	}
	return fmt.Sprintf("EIR tidak tersedia: kredit belum menyimpan EIR (dicairkan sebelum pencatatan EIR selalu aktif); isi %s sebagai override", overrideKey)
}

// eirBasisReason membaca alasan kegagalan dari basis audit. Galat parsing tidak
// menggagalkan pemanggil: basis rusak/kosong cukup menghasilkan pesan tanpa alasan.
func eirBasisReason(raw string) string {
	if raw == "" {
		return ""
	}
	var basis domain.EIRBasis
	if err := json.Unmarshal([]byte(raw), &basis); err != nil {
		return ""
	}
	return basis.Reason
}

// storeOriginalEIR menghitung dan menyimpan EIR orisinal dari arus kas nyata pencairan.
// Selalu dipanggil saat pencairan, terlepas dari saklar kerugian restrukturisasi.
//
// Bila perhitungan TIDAK konvergen, EIR tidak boleh menggagalkan pencairan (transaksi
// riil tidak boleh batal demi data pelengkap): ketidaktersediaan DICATAT beserta
// alasannya, sehingga restrukturisasi berikutnya menolak dengan pesan yang tepat.
//
// PENTING (apa adanya): DisburseLoan saat ini TIDAK memotong provisi/biaya transaksi
// apa pun — jurnal LOAN_DISBURSEMENT mendebit pokok penuh (lihat migrasi 000005).
// Karena itu jumlah cair = pokok penuh dan biaya yang dipotong dicatat 0. EIR yang
// dihasilkan murni berasal dari bentuk jadwal angsuran; untuk jadwal anuitas/sliding
// nilainya mendekati suku bunga kontraktual, sedangkan untuk jadwal flat (bunga atas
// pokok penuh) nilai efektifnya memang lebih tinggi daripada nominal kontraktual.
func (s *loanService) storeOriginalEIR(ctx context.Context, tx any, loan *domain.Loan, schedules []domain.LoanSchedule) error {
	writer, ok := s.loanRepo.(domain.LoanEIRWriter)
	if !ok {
		return errors.New("repo kredit tidak mendukung penyimpanan suku bunga efektif orisinal")
	}
	rate, basis, err := domain.CalculateDisbursementEIR(loan.PrincipalAmount, decimal.Zero, schedules)
	if err != nil {
		return s.recordUnavailableEIR(ctx, writer, tx, loan, err)
	}
	raw, err := json.Marshal(basis)
	if err != nil {
		return fmt.Errorf("menyusun dasar audit EIR kredit %s: %w", loan.LoanNumber, err)
	}
	return writer.UpdateOriginalEIRTx(ctx, tx, loan.ID, rate, domain.RestructureLossEIRMethod, raw, basis.CalculatedAt)
}

// recordUnavailableEIR menyimpan baris "EIR tidak tersedia" (monthly=0, method penanda,
// alasan pada basis audit) di dalam transaksi pencairan. Dipakai khusus saat perhitungan
// numerik gagal, supaya pencairan tetap sah berjalan sementara penyebab kegagalan dapat
// dibaca kembali oleh penolakan restrukturisasi. Galat tulis dikembalikan: bila barisnya
// sendiri tidak bisa disimpan, transaksi harus rollback.
func (s *loanService) recordUnavailableEIR(ctx context.Context, writer domain.LoanEIRWriter, tx any, loan *domain.Loan, calcErr error) error {
	basis := domain.EIRBasis{
		Method:       domain.RestructureLossEIRUnavailableMethod,
		NetProceeds:  loan.PrincipalAmount,
		FeesDeducted: decimal.Zero,
		CalculatedAt: time.Now().UTC(),
		Reason:       calcErr.Error(),
	}
	raw, err := json.Marshal(basis)
	if err != nil {
		return fmt.Errorf("menyusun dasar audit EIR kredit %s: %w", loan.LoanNumber, err)
	}
	return writer.UpdateOriginalEIRTx(ctx, tx, loan.ID, decimal.Zero, domain.RestructureLossEIRUnavailableMethod, raw, basis.CalculatedAt)
}

// restructureLoanWithLoss menjalankan restrukturisasi kredit beserta pengakuan kerugian
// penurunan nilainya di dalam SATU transaksi, dengan LockLoanTx di awal dan seluruh
// angka dihitung dari baris hasil kunci (bukan snapshot di luar transaksi).
func (s *loanService) restructureLoanWithLoss(ctx context.Context, input domain.RestructureLoanInput, actor domain.Actor, policy restructureLossPolicy) (*domain.Loan, error) {
	now := time.Now().UTC()
	var result *domain.Loan
	var before domain.Collectibility
	var loss domain.RestructureLossResult

	err := s.txRunner.Run(ctx, func(tx any) error {
		fresh, err := s.loanRepo.LockLoanTx(ctx, tx, input.LoanID)
		if err != nil {
			return fmt.Errorf("mengunci kredit %s: %w", input.LoanID, err)
		}
		if !canAccessLoan(actor, fresh) {
			return domain.ErrCrossBranchAccess
		}
		if !s.canAccessLoanBook(ctx, actor, fresh) {
			return domain.ErrCrossBookAccess
		}
		if fresh.Status != domain.LoanStatusDisbursed {
			return domain.ErrRestructureOnlyActiveLoan
		}
		if input.NewTermMonths <= 0 {
			return domain.ErrRestructureNewTermPositive
		}
		if fresh.ProductID == nil {
			return domain.ErrLoanProductMissing
		}
		product, err := s.productRepo.GetByID(ctx, *fresh.ProductID)
		if err != nil {
			return err
		}

		// Kualitas sebelum restrukturisasi disimpan sebelum apa pun berubah (Pasal 31).
		before = domain.CollectibilityFromOJK(fresh.Collectibility)
		fresh.PreRestructureCollectibility = fresh.Collectibility
		fresh.IsRestructured = true
		fresh.RestructuredCount++
		fresh.RestructuredAt = &now
		fresh.RestructuringReason = input.Reason
		fresh.TermMonths = input.NewTermMonths
		if input.NewInterestRateAnnual.IsPositive() {
			fresh.InterestRateAnnual = input.NewInterestRateAnnual
		}
		if input.NewMarginAmount.IsPositive() {
			fresh.MarginAmount = input.NewMarginAmount
			fresh.DeferredMargin = input.NewMarginAmount
		}

		col := CollectibilityForPosition(ctx, s.config, fresh.DPD, DaysPastMaturity(now, fresh.FinalDueDate))
		col = domain.RestructureCollectibility(before, col, 0)
		fresh.Collectibility = col.OJKCode()
		fresh.AccrualStatus = AccrualForCollectibility(col)

		method, profitType, margin, err := scheduleTermsFor(product, fresh.MarginAmount, fresh.OutstandingPrincipal, fresh.TermMonths)
		if err != nil {
			return err
		}
		schedules, totalPayable, monthly := domain.BuildSchedule(fresh.ID, domain.ScheduleParams{
			Principal:  fresh.OutstandingPrincipal,
			AnnualRate: fresh.InterestRateAnnual,
			Margin:     margin,
			TermMonths: fresh.TermMonths,
			StartDate:  now,
			Method:     method,
			ProfitType: profitType,
		})
		fresh.TotalPayable = totalPayable
		fresh.MonthlyInstallment = monthly

		// Nilai tercatat = pokok terutang dikurangi saldo kerugian restrukturisasi yang
		// belum diamortisasi (bila restrukturisasi ini bukan yang pertama).
		carrying := domain.PPAPCarryingAmount(fresh.OutstandingPrincipal, fresh.RestructureLossBalance)
		discount, err := s.restructureLossDiscountRate(ctx, fresh, policy)
		if err != nil {
			return err
		}
		loss, err = domain.CalculateRestructureLoss(carrying, domain.CashFlowsFromSchedules(schedules), discount)
		if err != nil {
			return fmt.Errorf("menghitung kerugian restrukturisasi kredit %s: %w", fresh.LoanNumber, err)
		}

		// Hanya kerugian (Loss > 0) yang dijurnal. Keuntungan modifikasi (Loss < 0) belum
		// diimplementasikan dan tidak boleh diposting sebagai beban negatif; saldo
		// kerugian tidak berubah. Lihat laporan keterbatasan.
		if loss.Loss.IsPositive() {
			if err := s.postRestructureLoss(ctx, tx, product, fresh, loss, actor); err != nil {
				return err
			}
			fresh.RestructureLossBalance = fresh.RestructureLossBalance.Add(loss.Loss)
		}

		writer, ok := s.loanRepo.(domain.RestructureTxWriter)
		if !ok {
			return errors.New("repo kredit tidak mendukung penyimpanan restrukturisasi transaksional")
		}
		fresh.Schedules = schedules
		if err := writer.UpdateRestructureTx(ctx, tx, fresh, schedules); err != nil {
			return fmt.Errorf("menyimpan restrukturisasi: %w", err)
		}
		result = fresh

		// Audit ditulis DI DALAM transaksi yang sama dengan jurnal kerugian. Bila
		// ditulis setelah commit lalu gagal, pemanggil menganggap operasi gagal dan
		// mencoba lagi — padahal jurnal kerugiannya sudah commit, sehingga percobaan
		// ulang menerbitkan jurnal kerugian KEDUA dengan kunci idempotensi berbeda
		// (nomor urut restrukturisasi naik). Satu transaksi membuat keduanya batal
		// bersama.
		if err := writeAudit(ctx, s.auditRepo, tx, actor, "RESTRUCTURE_LOAN", "loan", result.ID.String(), map[string]any{
			"loan_number":            result.LoanNumber,
			"reason":                 input.Reason,
			"collectibility_before":  before.OJKCode(),
			"collectibility_after":   result.Collectibility,
			"term_months":            input.NewTermMonths,
			"total_payable":          result.TotalPayable.StringFixed(2),
			"monthly_installment":    result.MonthlyInstallment.StringFixed(2),
			"restructure_loss":       loss.Loss.StringFixed(2),
			"carrying_amount":        loss.CarryingAmount.StringFixed(2),
			"present_value":          loss.PresentValue.StringFixed(2),
			"discount_rate_monthly":  loss.DiscountRateMonthly.String(),
			"restructure_loss_total": result.RestructureLossBalance.StringFixed(2),
		}); err != nil {
			return fmt.Errorf("audit restrukturisasi: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// postRestructureLoss memposting kerugian restrukturisasi: Db. Beban kerugian
// penurunan nilai; Kr. Kredit yang diberikan (PA BPR hlm. 61). Pemetaan produk untuk
// peristiwa LOAN_RESTRUCTURE_LOSS dipakai lebih dulu bila bank sudah mengisinya; jika
// tidak, jatuh ke COA konfigurasi. Migrasi 000043 tidak menanam pemetaan produk karena
// nilai enum baru tidak boleh dipakai dalam transaksi yang sama di PostgreSQL.
//
// Jurnal diatribusikan ke cabang KREDIT (loan.BranchCode), bukan cabang aktor.
func (s *loanService) postRestructureLoss(ctx context.Context, tx any, product *domain.BankingProduct, loan *domain.Loan, result domain.RestructureLossResult, actor domain.Actor) error {
	amount := result.Loss
	meta := PostingMeta{
		TransactionType: domain.TxTypeAdjustment,
		Description: fmt.Sprintf("Kerugian restrukturisasi kredit %s (nilai tercatat %s - nilai kini %s)",
			loan.LoanNumber, result.CarryingAmount, result.PresentValue),
		IdempotencyKey: fmt.Sprintf("RESTRUCT-LOSS-%s-%d", loan.LoanNumber, loan.RestructuredCount),
		CreatedBy:      actor.DisplayName(),
		BranchCode:     loan.BranchCode,
	}

	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventLoanRestructureLoss)
		if err != nil {
			// Galat pembacaan pemetaan BUKAN "produk belum dipetakan". Tanpa pembedaan
			// ini, produk yang sudah memetakan jurnal kerugiannya bisa tanpa jejak
			// terjurnal ke COA bawaan saat ada galat sesaat. Tidak ada pemetaan
			// (len==0) tetap memakai COA konfigurasi di bawah.
			return fmt.Errorf("membaca pemetaan jurnal kerugian restrukturisasi produk %s: %w", product.Code, err)
		}
		if len(rules) > 0 {
			if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanRestructureLoss, Amounts{
				Principal: amount,
				Total:     amount,
			}, meta); err != nil {
				return fmt.Errorf("jurnal kerugian restrukturisasi produk %s: %w", product.Code, err)
			}
			return nil
		}
	}

	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}
	expenseCOA := fallbackRestructureLossExpenseConventional
	loanCOA := fallbackRestructureLossLoanConventional
	if book == domain.BookSyariah {
		expenseCOA = fallbackRestructureLossExpenseSyariah
		loanCOA = fallbackRestructureLossLoanSyariah
	}
	expenseCOA = configStringOr(ctx, s.config, cfgRestructureLossExpenseCOA, expenseCOA)
	loanCOA = configStringOr(ctx, s.config, cfgRestructureLossLoanCOA, loanCOA)

	expenseAcc, err := s.resolver.ResolveGLAccount(ctx, tx, expenseCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrRestructureLossExpenseNotFound, expenseCOA, err)
	}
	loanAcc, err := s.resolver.ResolveGLAccount(ctx, tx, loanCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrRestructureLossLoanAccountNotFound, loanCOA, err)
	}

	lines := []domain.PostingLine{
		{AccountNumber: expenseAcc, Direction: domain.DirectionDebit, Amount: amount, Description: meta.Description},
		{AccountNumber: loanAcc, Direction: domain.DirectionCredit, Amount: amount, Description: meta.Description},
	}
	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: meta.TransactionType,
		Description:     meta.Description,
		IdempotencyKey:  meta.IdempotencyKey,
		CreatedBy:       meta.CreatedBy,
		BranchCode:      meta.BranchCode,
		Lines:           lines,
	})
	return err
}
