package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// AccrueInterest mengakru pendapatan bunga kredit konvensional berbasis jadwal
// angsuran untuk tanggal asOf. Kandidat adalah angsuran yang sudah jatuh tempo dan
// belum diakru; setiap kredit diproses dalam transaksinya sendiri sehingga kegagalan
// satu kredit tidak menghentikan kredit lain. Akrual kredit tidak lancar (kolektibilitas
// 3-5) dihentikan tanpa membalik akruan yang sudah terbentuk.
func (s *loanService) AccrueInterest(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.LoanInterestAccrualSummary, error) {
	asOf = asOf.UTC()
	// Tanggal bisnis dipakai untuk basis DPD, kunci idempotensi, dan entry_date jurnal.
	day := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)

	candidates, err := s.loanRepo.ListInterestAccrualCandidates(ctx, day)
	if err != nil {
		return domain.LoanInterestAccrualSummary{}, fmt.Errorf("mengambil daftar angsuran bunga: %w", err)
	}

	summary := domain.LoanInterestAccrualSummary{
		AsOf:     asOf,
		Total:    len(candidates),
		Items:    []domain.LoanInterestAccrualItem{},
		Failures: []domain.LoanInterestAccrualFailure{},
		Warnings: []string{},
	}

	// Kandidat dikelompokkan per kredit: satu transaksi menampung seluruh angsuran
	// kredit tersebut agar perubahan sisa akruan dan jurnalnya konsisten.
	var order []uuid.UUID
	groups := make(map[uuid.UUID][]domain.LoanInterestAccrualCandidate)
	for _, c := range candidates {
		if _, ok := groups[c.LoanID]; !ok {
			order = append(order, c.LoanID)
		}
		groups[c.LoanID] = append(groups[c.LoanID], c)
	}

	for _, loanID := range order {
		items, warning := s.accrueInterestForLoan(ctx, day, groups[loanID], actor)
		if warning != "" {
			summary.Warnings = append(summary.Warnings, warning)
		}
		for _, item := range items {
			summary.Items = append(summary.Items, item)
			summary.Processed++
			switch item.Status {
			case domain.BatchItemAccrued:
				summary.Accrued++
				summary.TotalAccrued = summary.TotalAccrued.Add(item.Amount)
			case domain.BatchItemSkipped:
				summary.Skipped++
			default:
				summary.Failed++
				summary.Failures = append(summary.Failures, domain.LoanInterestAccrualFailure{
					LoanID:     item.LoanID,
					LoanNumber: item.LoanNumber,
					Error:      item.Message,
				})
			}
		}
	}
	return summary, nil
}

// accrueInterestForLoan memproses seluruh angsuran satu kredit. Kredit yang tidak
// memenuhi syarat menghasilkan item SKIPPED (bukan kegagalan); kegagalan teknis
// menghasilkan item FAILED. warning diisi bila produk tidak punya pemetaan
// INTEREST_ACCRUAL sehingga angsuran dilewati, bukan diakru diam-diam.
func (s *loanService) accrueInterestForLoan(
	ctx context.Context,
	day time.Time,
	group []domain.LoanInterestAccrualCandidate,
	actor domain.Actor,
) ([]domain.LoanInterestAccrualItem, string) {
	items := make([]domain.LoanInterestAccrualItem, len(group))
	for i, c := range group {
		items[i] = domain.LoanInterestAccrualItem{
			LoanID:        c.LoanID,
			LoanNumber:    c.LoanNumber,
			InstallmentNo: c.InstallmentNo,
		}
	}

	first := group[0]

	// Hanya kredit konvensional yang diakru. Query kandidat sudah menyaring, tetapi
	// pemeriksaan di sini menjaga bila kandidat datang dari jalur lain.
	if !isConventionalLoanType(first.LoanType) {
		return skipInterestItems(items, "jenis kredit "+string(first.LoanType)+" tidak diakru"), ""
	}

	// Kolektibilitas memakai aturan yang sama dengan PPAP harian. Kredit tidak lancar
	// (3-5) menghentikan akrual, bukan membalik yang sudah terbentuk.
	dpd := 0
	if first.OldestDueDate != nil {
		dpd = daysPastDue(day, *first.OldestDueDate)
	}
	col := CollectibilityForPosition(ctx, s.config, dpd, DaysPastMaturity(day, first.FinalDueDate))
	if col.IsNPL() {
		return skipInterestItems(items, "kolektibilitas "+col.Label()+" (NPL): akrual dihentikan"), ""
	}
	for i := range items {
		items[i].DPD = dpd
	}

	if first.ProductID == nil {
		return failInterestItems(items, "kredit tidak terhubung ke produk"), ""
	}
	product, err := s.productRepo.GetByID(ctx, *first.ProductID)
	if err != nil {
		return failInterestItems(items, "produk kredit tidak ditemukan: "+err.Error()), ""
	}
	rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventInterestAccrual)
	if err != nil {
		return failInterestItems(items, "membaca pemetaan akrual bunga: "+err.Error()), ""
	}
	if len(rules) == 0 {
		return skipInterestItems(items, "produk tanpa pemetaan INTEREST_ACCRUAL"),
			fmt.Sprintf("produk %s tidak punya pemetaan %s; akrual bunga dilewati", product.Code, domain.EventInterestAccrual)
	}

	err = s.txRunner.Run(ctx, func(tx any) error {
		for i := range group {
			c := group[i]
			if c.DueDate.After(day) {
				items[i] = skipInterestItem(items[i], "angsuran belum jatuh tempo")
				continue
			}
			amount := c.OutstandingProfit
			if !amount.IsPositive() {
				items[i] = skipInterestItem(items[i], "porsi bunga sudah tidak tersisa")
				continue
			}
			// Kunci idempotensi unik per angsuran. Kolom idempotency_key jurnal unik,
			// sehingga batch kedua pada angsuran yang sama tidak menggandakan akrual.
			key := fmt.Sprintf("ACCR-%s-%d", c.LoanNumber, c.InstallmentNo)

			added, err := s.loanRepo.AddScheduleProfitAccruedTx(ctx, tx, c.ScheduleID, amount, key, day)
			if err != nil {
				return err
			}
			if !added {
				items[i] = skipInterestItem(items[i], "angsuran ini sudah diakru (idempoten)")
				continue
			}

			entry, err := s.poster.PostEventTx(ctx, tx, product, domain.EventInterestAccrual, Amounts{
				Profit: amount,
			}, PostingMeta{
				TransactionType: domain.TxTypeInterestAccrual,
				Description:     fmt.Sprintf("Akrual bunga kredit %s angsuran ke-%d", c.LoanNumber, c.InstallmentNo),
				IdempotencyKey:  key,
				CreatedBy:       actor.DisplayName(),
				BranchCode:      actor.BranchCode,
				// Jurnal masuk ke tanggal bisnis yang diproses, bukan jam eksekusi.
				EntryDate: day,
			})
			if err != nil {
				// Jurnal gagal: penambahan profit_accrued_amount ikut dibatalkan rollback.
				return err
			}
			items[i].Amount = amount
			items[i].JournalReference = entry.ReferenceNumber
			items[i].Status = domain.BatchItemAccrued
			items[i].Message = "bunga diakru"
		}
		return nil
	})
	if err != nil {
		return failInterestItems(items, err.Error()), ""
	}
	return items, ""
}

// isConventionalLoanType menentukan kredit yang boleh diakru bunganya. Daftar positif
// disengaja agar jenis kredit baru (mis. syariah) tidak otomatis diakru.
func isConventionalLoanType(lt domain.LoanType) bool {
	switch lt {
	case domain.LoanTypeConventionalFlat, domain.LoanTypeConventionalAnnuity:
		return true
	default:
		return false
	}
}

func skipInterestItems(items []domain.LoanInterestAccrualItem, message string) []domain.LoanInterestAccrualItem {
	for i := range items {
		items[i] = skipInterestItem(items[i], message)
	}
	return items
}

func failInterestItems(items []domain.LoanInterestAccrualItem, message string) []domain.LoanInterestAccrualItem {
	for i := range items {
		items[i] = failInterestItem(items[i], message)
	}
	return items
}

func skipInterestItem(item domain.LoanInterestAccrualItem, message string) domain.LoanInterestAccrualItem {
	item.Status = domain.BatchItemSkipped
	item.Message = message
	return item
}

func failInterestItem(item domain.LoanInterestAccrualItem, message string) domain.LoanInterestAccrualItem {
	item.Status = domain.BatchItemFailed
	item.Message = message
	return item
}

var _ domain.LoanInterestAccrualRunner = (*loanService)(nil)
