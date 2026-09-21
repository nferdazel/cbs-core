package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// AmortizeRestructureLoss memulihkan saldo kerugian restrukturisasi ke pendapatan bunga
// memakai metode suku bunga efektif (PA BPR Bab 5.2 hlm. 61): untuk setiap angsuran yang
// jatuh tempo (atau sisa saldo kredit yang sudah lunas/dihapusbukukan), diakui pendapatan
// bunga atas NILAI TERCATAT (pokok - saldo kerugian) pada EIR orisinal, lalu dicatat
// SELISIHNYA terhadap porsi bunga jadwal kontraktual dengan jurnal
// Db. Kredit yang diberikan / Kr. Pendapatan bunga.
//
// Mencegah pendapatan bunga dihitung dua kali: jalur akrual kontraktual (AccrueInterest,
// jurnal Db 10400 / Kr 40100) dan pelunasan (EventLoanProfitPay) tetap mengakui
// ContractualProfit persis seperti sebelumnya. Fungsi ini HANYA memposting
// eirInterest - ContractualProfit, bukan eirInterest penuh, sehingga total pendapatan
// yang diakui = ContractualProfit + (eirInterest - ContractualProfit) = eirInterest dan
// tidak ada bagian yang diakui dua kali. Karena itu jurnal di sini selalu memakai akun
// pendapatan yang sama (40100/14100), bukan memindahkan piutang bunga 10400.
//
// Seluruh perilaku berada di balik loan.restructure.loss.enabled. Saklar mati berarti
// tidak ada kueri kandidat, perhitungan, jurnal, atau perubahan saldo.
func (s *loanService) AmortizeRestructureLoss(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.RestructureLossAmortizationSummary, error) {
	asOf = asOf.UTC()
	// Tanggal bisnis dipakai untuk pemilihan angsuran jatuh tempo, kunci idempotensi,
	// dan entry_date jurnal, bukan jam eksekusi.
	day := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)

	summary := domain.RestructureLossAmortizationSummary{
		AsOf:     asOf,
		Items:    []domain.RestructureLossAmortizationItem{},
		Failures: []domain.RestructureLossAmortizationFailure{},
		Warnings: []string{},
	}
	policy := s.restructureLossPolicy(ctx)
	if !policy.Enabled {
		return summary, nil
	}
	// Parameter yang DIISI salah format/rentang ditolak dengan pesan jelas, bukan
	// diabaikan sehingga batch tampak sukses tanpa amortisasi.
	if policy.DiscountErr != nil {
		return summary, policy.DiscountErr
	}

	repo, ok := s.loanRepo.(domain.RestructureLossAmortizationRepository)
	if !ok {
		return summary, errors.New("repo kredit tidak mendukung amortisasi saldo kerugian restrukturisasi")
	}
	candidates, err := repo.ListRestructureLossAmortizationCandidates(ctx, day)
	if err != nil {
		return summary, fmt.Errorf("mengambil daftar kredit bersaldo kerugian: %w", err)
	}
	summary.Total = len(candidates)

	for _, c := range candidates {
		items, warning := s.amortizeRestructureLossForLoan(ctx, day, repo, c, policy, actor)
		if warning != "" {
			summary.Warnings = append(summary.Warnings, warning)
		}
		for _, item := range items {
			summary.Items = append(summary.Items, item)
			summary.Processed++
			switch item.Status {
			case domain.BatchItemAccrued:
				summary.Amortized++
				summary.TotalAmortized = summary.TotalAmortized.Add(item.Amount)
			case domain.BatchItemSkipped:
				summary.Skipped++
			default:
				summary.Failed++
				summary.Failures = append(summary.Failures, domain.RestructureLossAmortizationFailure{
					LoanID:     item.LoanID,
					LoanNumber: item.LoanNumber,
					Error:      item.Message,
				})
			}
		}
	}
	return summary, nil
}

// amortizeRestructureLossForLoan memproses seluruh angsuran satu kredit dalam SATU
// transaksi. Baris kredit dikunci lebih dulu (LockLoanTx) dan seluruh angka dihitung
// dari baris hasil kunci, bukan snapshot di luar transaksi. Angsuran diproses urut
// nomor angsuran; saldo kerugian yang dipakai periode berikutnya adalah saldo tersimpan
// setelah pengurangan periode sebelumnya.
func (s *loanService) amortizeRestructureLossForLoan(
	ctx context.Context,
	day time.Time,
	repo domain.RestructureLossAmortizationRepository,
	candidate domain.LoanRestructureLossAmortizationCandidate,
	policy restructureLossPolicy,
	actor domain.Actor,
) ([]domain.RestructureLossAmortizationItem, string) {
	items := []domain.RestructureLossAmortizationItem{}

	err := s.txRunner.Run(ctx, func(tx any) error {
		fresh, err := s.loanRepo.LockLoanTx(ctx, tx, candidate.LoanID)
		if err != nil {
			return fmt.Errorf("mengunci kredit %s: %w", candidate.LoanID, err)
		}
		if !fresh.RestructureLossBalance.IsPositive() {
			// Saldo sudah nol: tidak ada lagi yang diamortisasi.
			return nil
		}

		var product *domain.BankingProduct
		if fresh.ProductID != nil {
			product, err = s.productRepo.GetByID(ctx, *fresh.ProductID)
			if err != nil {
				return fmt.Errorf("produk kredit %s: %w", fresh.LoanNumber, err)
			}
		}
		discount, err := s.restructureLossDiscountRate(ctx, fresh, policy)
		if err != nil {
			return err
		}
		schedules, err := s.loanRepo.GetSchedulesTx(ctx, tx, fresh.ID)
		if err != nil {
			return fmt.Errorf("jadwal kredit %s: %w", fresh.LoanNumber, err)
		}

		maxNo := 0
		totalPrincipal := decimal.Zero
		for _, sc := range schedules {
			totalPrincipal = totalPrincipal.Add(sc.PrincipalAmount)
			if sc.InstallmentNo > maxNo {
				maxNo = sc.InstallmentNo
			}
		}

		closed := fresh.Status == domain.LoanStatusPaidOff || fresh.Status == domain.LoanStatusWrittenOff
		lossBefore := fresh.RestructureLossBalance
		prefixPrincipal := decimal.Zero
		for _, sc := range schedules {
			principalBefore := totalPrincipal.Sub(prefixPrincipal)
			prefixPrincipal = prefixPrincipal.Add(sc.PrincipalAmount)

			if sc.RestructureLossAmortizedAt != nil {
				continue
			}
			// Kredit aktif hanya memproses angsuran yang sudah jatuh tempo; kredit
			// lunas/dihapusbukukan memproses sisanya tanpa menunggu jatuh tempo agar
			// saldo ditutup tepat nol.
			if !closed && sc.DueDate.After(day) {
				continue
			}

			result, err := domain.CalculateRestructureLossAmortization(domain.RestructureLossAmortizationInput{
				PrincipalBefore:   principalBefore,
				LossBalanceBefore: lossBefore,
				EIRMonthly:        discount,
				ContractualProfit: sc.ProfitAmount,
				FinalPeriod:       sc.InstallmentNo == maxNo,
			})
			if err != nil {
				return fmt.Errorf("menghitung amortisasi kredit %s angsuran ke-%d: %w", fresh.LoanNumber, sc.InstallmentNo, err)
			}
			if !result.Amortization.IsPositive() {
				// Bunga kontraktual menutupi bunga efektif periode ini: tidak ada
				// amortisasi dan tidak ada jurnal. Angsuran sengaja tidak ditandai
				// karena tidak ada state yang berubah; pengulangan tidak berakibat.
				items = append(items, amortizationItem(fresh, sc.InstallmentNo,
					domain.BatchItemSkipped, "tidak ada selisih untuk diamortisasi", decimal.Zero, ""))
				continue
			}

			key := fmt.Sprintf("LOSSAMORT-%s-%d", fresh.LoanNumber, sc.InstallmentNo)
			applied, err := repo.ApplyRestructureLossAmortizationTx(ctx, tx, fresh.ID, &sc.ID, result.Amortization, key, day)
			if err != nil {
				return fmt.Errorf("menerapkan amortisasi kredit %s: %w", fresh.LoanNumber, err)
			}
			if !applied {
				// Kunci jurnal sudah ada: replay idempoten, jangan posting ulang.
				items = append(items, amortizationItem(fresh, sc.InstallmentNo,
					domain.BatchItemSkipped, "angsuran ini sudah diamortisasi (idempoten)", decimal.Zero, ""))
				continue
			}
			entry, err := s.postRestructureLossAmortization(ctx, tx, product, fresh, result.Amortization, sc.InstallmentNo, key, day, actor)
			if err != nil {
				return err
			}
			lossBefore = result.LossBalanceAfter
			items = append(items, amortizationItem(fresh, sc.InstallmentNo,
				domain.BatchItemAccrued, "saldo kerugian diamortisasi", result.Amortization, entry.ReferenceNumber))
		}

		// Penutupan sisa: kredit sudah lunas/dihapusbukukan tetapi saldo masih tersisa
		// (mis. pelunasan dipercepat sebelum tenor berakhir). Sisa dihabiskan agar akun
		// piutang tidak pernah negatif setelah pokok penuh dikredit.
		if closed && lossBefore.IsPositive() {
			key := fmt.Sprintf("LOSSAMORT-%s-CLOSE", fresh.LoanNumber)
			applied, err := repo.ApplyRestructureLossAmortizationTx(ctx, tx, fresh.ID, nil, lossBefore, key, day)
			if err != nil {
				return fmt.Errorf("menutup sisa saldo kerugian kredit %s: %w", fresh.LoanNumber, err)
			}
			if applied {
				entry, err := s.postRestructureLossAmortization(ctx, tx, product, fresh, lossBefore, 0, key, day, actor)
				if err != nil {
					return err
				}
				items = append(items, amortizationItem(fresh, 0,
					domain.BatchItemAccrued, "sisa saldo kerugian ditutup", lossBefore, entry.ReferenceNumber))
				lossBefore = decimal.Zero
			}
		}
		return nil
	})
	if err != nil {
		return []domain.RestructureLossAmortizationItem{amortizationItem(candidateLoan(candidate),
			0, domain.BatchItemFailed, err.Error(), decimal.Zero, "")}, ""
	}
	return items, ""
}

// postRestructureLossAmortization memposting amortisasi satu periode:
// Db. Kredit yang diberikan / Kr. Pendapatan bunga (PA BPR hlm. 61). Jurnal diatribusikan
// ke cabang KREDIT (loan.BranchCode), bukan cabang aktor, dan bertanggal bisnis (day).
func (s *loanService) postRestructureLossAmortization(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	loan *domain.Loan,
	amount decimal.Decimal,
	installmentNo int,
	idempotencyKey string,
	day time.Time,
	actor domain.Actor,
) (*domain.JournalEntry, error) {
	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}
	loanCOA := fallbackRestructureLossLoanConventional
	incomeCOA := fallbackRestructureLossAmortIncomeConventional
	if book == domain.BookSyariah {
		loanCOA = fallbackRestructureLossLoanSyariah
		incomeCOA = fallbackRestructureLossAmortIncomeSyariah
	}
	// Kredit yang disajikan memakai kunci COA yang sama dengan jurnal kerugian
	// (loan.restructure.loss.coa.loan), sehingga sisi piutang kedua jurnal selalu
	// menunjuk akun yang sama.
	loanCOA = configStringOr(ctx, s.config, cfgRestructureLossLoanCOA, loanCOA)
	incomeCOA = configStringOr(ctx, s.config, cfgRestructureLossAmortIncomeCOA, incomeCOA)

	loanAcc, err := s.resolver.ResolveGLAccount(ctx, tx, loanCOA)
	if err != nil {
		return nil, fmt.Errorf("%w: COA %s: %v", domain.ErrRestructureLossLoanAccountNotFound, loanCOA, err)
	}
	incomeAcc, err := s.resolver.ResolveGLAccount(ctx, tx, incomeCOA)
	if err != nil {
		return nil, fmt.Errorf("%w: COA %s: %v", domain.ErrRestructureLossIncomeAccountNotFound, incomeCOA, err)
	}

	scope := fmt.Sprintf("angsuran ke-%d", installmentNo)
	if installmentNo == 0 {
		scope = "akhir tenor"
	}
	description := fmt.Sprintf("Amortisasi saldo kerugian restrukturisasi kredit %s (%s)", loan.LoanNumber, scope)
	return s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeAdjustment,
		Description:     description,
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       actor.DisplayName(),
		// Cabang jurnal = cabang kredit, bukan cabang aktor.
		BranchCode: loan.BranchCode,
		EntryDate:  day,
		Lines: []domain.PostingLine{
			{AccountNumber: loanAcc, Direction: domain.DirectionDebit, Amount: amount, Description: description},
			{AccountNumber: incomeAcc, Direction: domain.DirectionCredit, Amount: amount, Description: description},
		},
	})
}

func amortizationItem(loan *domain.Loan, installmentNo int, status, message string, amount decimal.Decimal, ref string) domain.RestructureLossAmortizationItem {
	return domain.RestructureLossAmortizationItem{
		LoanID:           loan.ID,
		LoanNumber:       loan.LoanNumber,
		InstallmentNo:    installmentNo,
		Amount:           amount,
		JournalReference: ref,
		Status:           status,
		Message:          message,
	}
}

// candidateLoan memulihkan identitas kredit minimal untuk item gagal saat kunci kredit
// sendiri tidak berhasil dibaca.
func candidateLoan(c domain.LoanRestructureLossAmortizationCandidate) *domain.Loan {
	return &domain.Loan{ID: c.LoanID, LoanNumber: c.LoanNumber}
}

var _ domain.RestructureLossAmortizationRunner = (*loanService)(nil)
