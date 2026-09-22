package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi tarif denda kredit: per-mille (‰) per hari dari pokok tunggakan.
// Nilai 1 berarti 0,1% per hari. Tidak ada tarif bisnis yang dikarangkan di kode.
const cfgLoanPenaltyDailyRatePerMille = "loan.penalty.rate.daily.per_mille"

// Kunci plafon denda: persen dari pokok tunggakan yang menjadi batas atas denda
// terakru per kredit. Nilai aplikasi 10 (10%). 0 = plafon dimatikan.
const cfgLoanPenaltyCapPercent = "loan.penalty.cap_pct"

// Sikap sementara menunggu keputusan DPS: denda pembiayaan syariah (ta'zir) TIDAK
// diakui sebagai pendapatan bank, melainkan dana sosial (Dana Kebajikan). Akun
// kewajiban itu sudah ada di bagan akun (migrasi 000024: 12500 "Dana Kebajikan",
// note LIABILITY/CREDIT) dan pemetaan LOAN_PENALTY produk syariah menunjuk ke sana
// (debit 11700 Piutang Denda Syariah / kredit 12500). Kode TIDAK mengarang akun:
// nilainya dibaca dari kunci konfigurasi ini, dengan bawaan 12500, dan pemetaan
// produk yang mengkredit akun lain akan DITOLAK, bukan diam-diam diakui sebagai
// pendapatan. Ganti nilai ini hanya setelah DPS menetapkan perlakuan resminya.
const cfgLoanPenaltySyariahSocialFundCOA = "loan.penalty.syariah.social_fund.coa"

const defaultLoanPenaltySyariahSocialFundCOA = "12500"

// loanPenaltyDailyRate membaca tarif denda harian dari system_config. Default 0 di
// sini HANYA fallback sementara untuk lingkungan yang belum di-provision: operator
// WAJIB mengisi kuncinya. Selama 0, tidak ada denda yang diakru dan ringkasan
// menandainya RateConfigured=false beserta Warning, bukan sukses diam-diam.
func loanPenaltyDailyRate(ctx context.Context, config domain.SystemConfigService) decimal.Decimal {
	return configDecimalOr(ctx, config, cfgLoanPenaltyDailyRatePerMille, decimal.Zero)
}

// loanPenaltyCapPercent membaca plafon denda (persen dari pokok tunggakan). Bawaan
// kode 10% dipakai bila kunci tidak ada; 0 berarti plafon dimatikan.
func loanPenaltyCapPercent(ctx context.Context, config domain.SystemConfigService) decimal.Decimal {
	return configDecimalOr(ctx, config, cfgLoanPenaltyCapPercent, decimal.NewFromInt(10))
}

// syariahPenaltyCreditCOA membaca kaki kredit pemetaan LOAN_PENALTY produk syariah,
// yaitu tempat denda syariah diakui. Dipakai memastikan denda syariah tidak pernah
// jatuh ke akun pendapatan.
func (s *loanService) syariahPenaltyCreditCOA(ctx context.Context, product *domain.BankingProduct) (string, error) {
	rules, err := s.productRepo.GetMapping(ctx, product.ID, domain.EventLoanPenalty)
	if err != nil {
		return "", fmt.Errorf("membaca pemetaan denda syariah: %w", err)
	}
	for _, r := range rules {
		if r.Direction == domain.DirectionCredit {
			return r.COACode, nil
		}
	}
	return "", fmt.Errorf("pemetaan denda syariah produk %s tidak punya kaki kredit", product.Code)
}

// AccruePenalties menghitung dan memposting denda atas tunggakan untuk tanggal asOf.
// Setiap kredit diproses dalam transaksinya sendiri: kegagalan satu kredit dicatat
// sebagai gagal di ringkasan dan tidak menghentikan kredit lain.
func (s *loanService) AccruePenalties(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.LoanPenaltySummary, error) {
	asOf = asOf.UTC()
	// Tanggal bisnis dipakai untuk basis DPD, kunci idempotensi, dan entry_date jurnal.
	day := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	rate := loanPenaltyDailyRate(ctx, s.config)
	capPercent := loanPenaltyCapPercent(ctx, s.config)

	candidates, err := s.loanRepo.ListPenaltyCandidates(ctx, day, actor)
	if err != nil {
		return domain.LoanPenaltySummary{}, fmt.Errorf("mengambil daftar kredit menunggak: %w", err)
	}

	summary := domain.LoanPenaltySummary{
		AsOf:           asOf,
		RatePerMille:   rate,
		RateConfigured: rate.IsPositive(),
		CapPercent:     capPercent,
		Total:          len(candidates),
		Items:          []domain.LoanPenaltyItem{},
		Failures:       []domain.LoanPenaltyFailure{},
	}

	for _, c := range candidates {
		item := s.accruePenaltyForLoan(ctx, day, c, rate, actor)
		summary.Items = append(summary.Items, item)
		// DPD positif berarti kredit benar-benar melewati jatuh tempo dengan pokok
		// tunggakan; inilah kredit yang akan dikenai denda bila tarif diisi.
		if item.DPD > 0 {
			summary.Overdue++
		}
		if item.Capped {
			summary.Capped++
		}
		if item.SyariahSocialFund {
			summary.SyariahSocialFund++
		}

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

	// Tarif 0 hanya diperingatkan bila ada kredit menunggak: tanpa angka yang
	// terdampak, "tidak ada denda yang diakru" tidak dapat ditindaklanjuti; tanpa
	// tunggakan, peringatan itu hanya kebisingan.
	warnings := []string{}
	if !summary.RateConfigured && summary.Overdue > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"tarif denda harian %s masih 0; tidak ada denda yang diakru atas %d kredit yang menunggak (jatuh tempo terlewat). Operator wajib mengisi tarifnya.",
			cfgLoanPenaltyDailyRatePerMille, summary.Overdue))
	}
	// Plafon yang menghentikan akrual wajib terlihat: tanpa ini, denda berhenti
	// bertambah tanpa penjelasan dan operator menyangka semuanya normal.
	if summary.Capped > 0 {
		warnings = append(warnings, fmt.Sprintf(
			"plafon denda %s%% dari pokok tunggakan tercapai pada %d kredit; akrual denda dibatasi/dihentikan agar tidak melebihi kewajiban pokoknya.",
			capPercent, summary.Capped))
	}
	summary.Warning = strings.Join(warnings, "; ")
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

	// Kolektibilitas memakai jalur yang sama dengan akrual bunga dan PPAP harian.
	// Akrual denda dihentikan untuk kredit tidak lancar (kolektibilitas 3-5): denda
	// atas kredit macet tidak boleh diakui sebagai pendapatan secara akrual, sejalan
	// dengan penghentian akrual bunga (cash basis). Selama NPL, tidak ada akruan baru
	// dan LastAccruedOn tidak maju; akruan yang sudah terbentuk tidak dibalik.
	//
	// Konsekuensi saat kredit SEMBUH dari NPL: gerbang ini tidak lagi menahan, dan
	// daysToAccrue di bawah mengejar seluruh hari sejak akrual terakhir — TERMASUK
	// hari-hari masa NPL. Ini disengaja dan konsisten dengan pola akrual bunga: yang
	// dihentikan hanyalah PENGAKUAN selama masa NPL, bukan hak atas denda setelah
	// kualitas membaik. Karena itu kalimat di atas tidak berarti "hanya akruan ke depan
	// yang dihentikan".
	col := CollectibilityForPosition(ctx, s.config, dpd, DaysPastMaturity(day, c.FinalDueDate))
	if col.IsNPL() {
		return skipPenalty(item, "kolektibilitas "+col.Label()+" (NPL): akrual denda dihentikan")
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
	if !penalty.IsPositive() {
		return skipPenalty(item, "hasil perhitungan denda nol")
	}

	// Plafon denda per kredit: denda terakru yang belum dibayar tidak boleh melewati
	// capPercent persen dari pokok tunggakan saat ini. Tanpa plafon, tunggakan
	// berbulan-bulan (DPD ratusan) dapat membuat denda melewati kewajiban pokoknya.
	// Saat plafon tersentuh, akrual dibatasi/dihentikan dan item.Capped membuatnya
	// terlihat di ringkasan EOD, bukan berhenti diam-diam.
	capPercent := loanPenaltyCapPercent(ctx, s.config)
	if capPercent.IsPositive() {
		capAmount := domain.LoanPenaltyCap(c.OverduePrincipal, capPercent)
		item.CapAmount = capAmount
		remaining := capAmount.Sub(c.PenaltyAccrued)
		if !remaining.IsPositive() {
			item.Capped = true
			return skipPenalty(item, fmt.Sprintf(
				"plafon denda tercapai: denda terakru %s sudah mencapai/melampaui plafon %s (%s%% dari pokok tunggakan %s)",
				c.PenaltyAccrued, capAmount, capPercent, c.OverduePrincipal))
		}
		if penalty.GreaterThan(remaining) {
			// Sisa ruang plafon lebih kecil dari denda hari ini: tagih hanya sebesar
			// sisa itu, lalu tandai terpotong plafon.
			penalty = remaining
			item.Capped = true
		}
	}
	item.Penalty = penalty

	if c.ProductID == nil {
		return failPenalty(item, "kredit tidak terhubung ke produk")
	}
	product, err := s.productRepo.GetByID(ctx, *c.ProductID)
	if err != nil {
		return failPenalty(item, "produk kredit tidak ditemukan: "+err.Error())
	}

	// Denda pembiayaan syariah (ta'zir) tidak boleh diakui sebagai pendapatan bank;
	// dana itu milik sosial. Sikap sementara menunggu keputusan DPS: pastikan kaki
	// kredit pemetaan menunjuk akun Dana Kebajikan, dan TOLAK (bukan diam-diam
	// diakui) bila pemetaan produk mengarah ke akun lain.
	if product.Book == domain.BookSyariah {
		creditCOA, err := s.syariahPenaltyCreditCOA(ctx, product)
		if err != nil {
			return failPenalty(item, err.Error())
		}
		socialFundCOA := configStringOr(ctx, s.config, cfgLoanPenaltySyariahSocialFundCOA, defaultLoanPenaltySyariahSocialFundCOA)
		if creditCOA != socialFundCOA {
			return failPenalty(item, fmt.Sprintf(
				"pemetaan denda syariah produk %s mengkredit %s, bukan dana kebajikan %s: denda syariah tidak boleh diakui sebagai pendapatan. Sikap sementara menunggu keputusan DPS.",
				product.Code, creditCOA, socialFundCOA))
		}
		item.SyariahSocialFund = true
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
