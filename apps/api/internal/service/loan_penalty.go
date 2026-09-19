package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi tarif denda kredit: per-mille (‰) per hari dari pokok tunggakan.
// Nilai 1 berarti 0,1% per hari. Tidak ada tarif bisnis yang dikarangkan di kode.
const cfgLoanPenaltyDailyRatePerMille = "loan.penalty.rate.daily.per_mille"

// loanPenaltyDailyRate membaca tarif denda harian dari system_config. Default 0 di
// sini HANYA fallback sementara untuk lingkungan yang belum di-provision: operator
// WAJIB mengisi kuncinya. Selama 0, tidak ada denda yang diakru dan ringkasan
// menandainya RateConfigured=false beserta Warning, bukan sukses diam-diam.
func loanPenaltyDailyRate(ctx context.Context, config domain.SystemConfigService) decimal.Decimal {
	return configDecimalOr(ctx, config, cfgLoanPenaltyDailyRatePerMille, decimal.Zero)
}

// AccruePenalties menghitung dan memposting denda atas tunggakan untuk tanggal asOf.
// Setiap kredit diproses dalam transaksinya sendiri: kegagalan satu kredit dicatat
// sebagai gagal di ringkasan dan tidak menghentikan kredit lain.
func (s *loanService) AccruePenalties(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.LoanPenaltySummary, error) {
	asOf = asOf.UTC()
	// Tanggal bisnis dipakai untuk basis DPD, kunci idempotensi, dan entry_date jurnal.
	day := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	rate := loanPenaltyDailyRate(ctx, s.config)

	candidates, err := s.loanRepo.ListPenaltyCandidates(ctx, day)
	if err != nil {
		return domain.LoanPenaltySummary{}, fmt.Errorf("mengambil daftar kredit menunggak: %w", err)
	}

	summary := domain.LoanPenaltySummary{
		AsOf:           asOf,
		RatePerMille:   rate,
		RateConfigured: rate.IsPositive(),
		Total:          len(candidates),
		Items:          []domain.LoanPenaltyItem{},
		Failures:       []domain.LoanPenaltyFailure{},
	}
	if !summary.RateConfigured {
		summary.Warning = fmt.Sprintf(
			"tarif denda harian %s masih 0; tidak ada denda yang diakru. Operator wajib mengisi tarifnya.",
			cfgLoanPenaltyDailyRatePerMille)
	}

	for _, c := range candidates {
		item := s.accruePenaltyForLoan(ctx, day, c, rate, actor)
		summary.Items = append(summary.Items, item)

		switch item.Status {
		case domain.BatchItemFailed:
			summary.Failed++
			summary.Failures = append(summary.Failures, domain.LoanPenaltyFailure{
				LoanID:     c.LoanID,
				LoanNumber: c.LoanNumber,
				Error:      item.Message,
			})
		case domain.BatchItemAccrued:
			summary.Processed++
			summary.Accrued++
			summary.TotalPenalty = summary.TotalPenalty.Add(item.Penalty)
		default:
			summary.Processed++
			summary.Skipped++
		}
	}
	return summary, nil
}

// accruePenaltyForLoan memproses satu kredit. Kredit yang tidak memenuhi syarat
// dikembalikan sebagai SKIPPED dengan alasan; kegagalan menjadi FAILED.
func (s *loanService) accruePenaltyForLoan(
	ctx context.Context,
	day time.Time,
	c domain.LoanPenaltyCandidate,
	rate decimal.Decimal,
	actor domain.Actor,
) domain.LoanPenaltyItem {
	item := domain.LoanPenaltyItem{
		LoanID:           c.LoanID,
		LoanNumber:       c.LoanNumber,
		OverduePrincipal: c.OverduePrincipal,
	}

	if !domain.LoanPenaltyEligible(c.Status) {
		return skipPenalty(item, "status kredit "+string(c.Status)+" tidak dikenai denda")
	}
	if c.OldestDueDate == nil || !c.OverduePrincipal.IsPositive() {
		return skipPenalty(item, "tidak ada tunggakan pokok")
	}

	dpd := daysPastDue(day, *c.OldestDueDate)
	item.DPD = dpd
	if dpd <= 0 {
		return skipPenalty(item, "tunggakan belum melewati jatuh tempo")
	}

	// Delta berbasis tanggal: hanya hari yang belum diakru yang ditagih. Akrual
	// pertama (belum pernah diakru) mengejar seluruh DPD karena penalty_accrued
	// masih 0; setelah itu selisih sejak akrual terakhir. Tanpa ini, EOD harian
	// menambah DPD penuh setiap hari dan totalnya menjadi kuadratik (1+2+...+n).
	daysToAccrue := dpd
	if c.LastAccruedOn != nil {
		daysToAccrue = daysPastDue(day, *c.LastAccruedOn)
	}
	if daysToAccrue > dpd {
		// Tidak boleh menagih melebihi masa tunggakan, misalnya bila ada tanggal
		// akrual lama yang hilang.
		daysToAccrue = dpd
	}
	if daysToAccrue <= 0 {
		return skipPenalty(item, "tidak ada hari baru untuk diakru (sudah diakru sampai tanggal ini)")
	}

	// Tarif 0 sengaja tidak menghasilkan jurnal apa pun, tetapi alasan skip harus
	// terlihat jelas di ringkasan.
	if !rate.IsPositive() {
		return skipPenalty(item, "tarif denda harian 0; tidak ada denda diakru")
	}
	item.DaysAccrued = daysToAccrue

	penalty := domain.LoanPenaltyAmount(c.OverduePrincipal, daysToAccrue, rate)
	item.Penalty = penalty
	if !penalty.IsPositive() {
		return skipPenalty(item, "hasil perhitungan denda nol")
	}

	if c.ProductID == nil {
		return failPenalty(item, "kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *c.ProductID)
	if err != nil {
		return failPenalty(item, "produk kredit tidak ditemukan: "+err.Error())
	}

	// Kunci idempotensi per kredit per tanggal. Kolom idempotency_key jurnal unik,
	// sehingga batch kedua pada tanggal yang sama tidak menggandakan denda.
	idempotencyKey := fmt.Sprintf("PENALTY-%s-%s", c.LoanNumber, day.Format("2006-01-02"))

	replayed := false
	err = s.txRunner.Run(ctx, func(tx any) error {
		// Penambahan penalty_accrued dikunci lebih dulu dan hanya berlaku bila jurnal
		// tanggal ini belum ada. Bila sudah ada (replay), tidak ada yang ditambah dan
		// transaksi ini menjadi no-op.
		added, err := s.loanRepo.AddPenaltyAccruedTx(ctx, tx, c.LoanID, penalty, idempotencyKey, day)
		if err != nil {
			return err
		}
		if !added {
			replayed = true
			return nil
		}

		entry, err := s.poster.PostEventTx(ctx, tx, product, domain.EventLoanPenalty, Amounts{
			Penalty: penalty,
			Total:   penalty,
		}, PostingMeta{
			TransactionType: domain.TxTypeAdjustment,
			Description:     fmt.Sprintf("Akrual denda kredit %s DPD %d (%d hari)", c.LoanNumber, dpd, daysToAccrue),
			IdempotencyKey:  idempotencyKey,
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
			// Jurnal masuk ke tanggal bisnis yang diproses, bukan jam eksekusi.
			EntryDate: day,
			// Tanpa AccountOverrides: denda adalah TAGIHAN, sehingga kaki debit jatuh
			// ke akun piutang denda menurut pemetaan produk (migrasi 000024), bukan ke
			// rekening nasabah. Dana nasabah baru berkurang saat denda dibayar.
		})
		if err != nil {
			// Jurnal gagal: penambahan penalty_accrued ikut dibatalkan oleh rollback.
			return err
		}
		item.JournalReference = entry.ReferenceNumber
		return nil
	})
	if err != nil {
		return failPenalty(item, err.Error())
	}
	if replayed {
		return skipPenalty(item, "denda tanggal ini sudah diakru (idempoten)")
	}

	item.Status = domain.BatchItemAccrued
	item.Message = "denda diakru"
	return item
}

func skipPenalty(item domain.LoanPenaltyItem, message string) domain.LoanPenaltyItem {
	item.Status = domain.BatchItemSkipped
	item.Message = message
	return item
}

func failPenalty(item domain.LoanPenaltyItem, message string) domain.LoanPenaltyItem {
	item.Status = domain.BatchItemFailed
	item.Message = message
	return item
}

var _ domain.LoanPenaltyService = (*loanService)(nil)
