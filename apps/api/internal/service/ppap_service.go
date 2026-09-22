package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi COA PPAP. Tarif dan ambang DPD dibaca lewat helper bersama di
// collectibility_rules.go agar aturannya identik dengan jalur restrukturisasi kredit.
const (
	cfgPPAPExpenseCOA = "ppap.coa.expense"
	cfgPPAPReserveCOA = "ppap.coa.reserve"
)

// Fallback kode COA. Kode ini ADA di seed migrasi 000005:
//   - 50200 Beban Penyisihan Kerugian Kredit (EXPENSE, konvensional)
//   - 15200 Beban Penyisihan Kerugian Pembiayaan (EXPENSE, syariah)
//   - 10900 Cadangan Kerugian Penurunan Nilai / PPAP (ASSET contra, konvensional)
//   - 11900 Cadangan Kerugian Pembiayaan (ASSET contra, syariah)
//
// Kunci ppap.coa.expense/ppap.coa.reserve bersifat global (bukan per buku); nilai
// kosong atau tidak ada berarti fallback per buku di atas yang dipakai.
const (
	fallbackPPAPExpenseConventional = "50200"
	fallbackPPAPExpenseSyariah      = "15200"
	fallbackPPAPReserveConventional = "10900"
	fallbackPPAPReserveSyariah      = "11900"
)

// ppapTxRunner membuka transaksi per kredit. Interface ini membuat pemrosesan bisa
// diuji tanpa database: produksi memakai *sql.DB, test memakai runner palsu.
type ppapTxRunner interface {
	Run(ctx context.Context, fn func(tx any) error) error
}

type sqlPPAPTxRunner struct{ db *sql.DB }

func (r sqlPPAPTxRunner) Run(ctx context.Context, fn func(tx any) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

type ppapService struct {
	txRunner       ppapTxRunner
	repo           domain.PPAPRepository
	collateralRepo domain.CollateralRepository
	productRepo    domain.ProductRepository
	resolver       domain.AccountResolver
	poster         *ProductPoster
	posting        domain.PostingService
	config         domain.SystemConfigService
	// runMarker mencatat tanggal bisnis run yang berhasil. Boleh nil (lingkungan uji
	// tanpa database); bila nil tidak ada penanda yang ditulis.
	runMarker domain.PPAPRunMarker
}

func NewPPAPService(
	db *sql.DB,
	repo domain.PPAPRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	poster *ProductPoster,
	posting domain.PostingService,
	config domain.SystemConfigService,
	runMarker domain.PPAPRunMarker,
	collateralSinks ...domain.CollateralRepository,
) domain.PPAPService {
	var collateralRepo domain.CollateralRepository
	if len(collateralSinks) > 0 {
		collateralRepo = collateralSinks[0]
	}
	return &ppapService{
		collateralRepo: collateralRepo,
		txRunner:       sqlPPAPTxRunner{db: db},
		repo:           repo,
		productRepo:    productRepo,
		resolver:       resolver,
		poster:         poster,
		posting:        posting,
		config:         config,
		runMarker:      runMarker,
	}
}

// RunDaily menghitung kolektibilitas dan PPAP seluruh kredit aktif. Setiap kredit
// diproses dalam transaksinya sendiri: satu kredit gagal tidak menggagalkan batch.
func (s *ppapService) RunDaily(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PPAPRunSummary, error) {
	return s.run(ctx, asOf, actor, false)
}

// Preview menghitung tanpa memposting jurnal atau mengubah state kredit. actor dipakai
// membatasi kredit yang dibaca pada cabang dan bukunya, sama seperti RunDaily.
func (s *ppapService) Preview(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PPAPRunSummary, error) {
	return s.run(ctx, asOf, actor, true)
}

func (s *ppapService) run(ctx context.Context, asOf time.Time, actor domain.Actor, preview bool) (domain.PPAPRunSummary, error) {
	asOf = asOf.UTC()

	snapshots, err := s.repo.ListDueLoans(ctx, asOf, actor)
	if err != nil {
		return domain.PPAPRunSummary{}, fmt.Errorf("mengambil daftar kredit PPAP: %w", err)
	}

	thresholds := collectibilityThresholds(ctx, s.config)
	rates := collectibilityRates(ctx, s.config)

	// Nilai agunan pengurang dibaca SEKALI untuk seluruh kredit dalam satu query. Bila
	// dibaca per kredit, pekerjaan harian ini menjadi N+1 tepat pada saat jumlah kredit
	// bertambah — persis saat kecepatannya paling dibutuhkan.
	collateralsByLoan, err := s.collateralValues(ctx, snapshots)
	if err != nil {
		return domain.PPAPRunSummary{}, err
	}

	summary := domain.PPAPRunSummary{
		AsOf:          asOf,
		Total:         len(snapshots),
		Preview:       preview,
		ReserveBefore: s.totalReserve(ctx),
	}

	for _, snap := range snapshots {
		item, err := s.processLoan(ctx, asOf, snap, thresholds, rates, collateralsByLoan[snap.LoanID], actor, preview)
		if err != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, domain.PPAPRunFailure{
				LoanID:     snap.LoanID,
				LoanNumber: snap.LoanNumber,
				Error:      err.Error(),
			})
			continue
		}

		summary.Processed++
		if item.Posted {
			summary.Adjusted++
		}
		summary.TotalAdjustment = summary.TotalAdjustment.Add(item.Adjustment)
		if !item.Posted && !item.CollectibilityChanged && item.DPD == snap.DPD && item.Target.Equal(snap.RequiredPPAP) {
			summary.Skipped++
		}
		summary.Items = append(summary.Items, item)
	}

	if !preview {
		summary.ReserveAfter = s.totalReserve(ctx)
		// Penanda ditulis SETELAH seluruh kredit diproses, bukan sebelum: bila run
		// berhenti di tengah, required_ppap sebagian kredit belum berasal dari tanggal
		// bisnis ini dan CKPN harus tetap menolak memakainya.
		if s.runMarker != nil {
			// updatedBy dari actor: kolom system_config.updated_by ber-FK ke
			// staff_users dan menolak uuid.Nil, sedangkan run ini sudah selesai.
			if err := s.runMarker.RecordRun(ctx, asOf, actor.UserID); err != nil {
				return summary, err
			}
		}
	}
	return summary, nil
}

// collateralValues mengelompokkan agunan aktif per kredit. Tidak ada yang dibaca dan tidak
// ada query dijalankan selama ppap.collateral.enabled bernilai false, sehingga modul agunan
// yang belum diaktifkan tidak menambah beban maupun risiko pada tutup hari.
//
// Yang dikembalikan adalah daftar agunan, bukan total, karena pengurang harus dinilai per
// agunan lewat aturan Pasal 20 dan Pasal 21 POJK No. 1 Tahun 2024 (lihat domain).
//
// Kegagalan membaca agunan dikembalikan sebagai error, bukan diabaikan: mengabaikannya
// berarti menjalankan PPAP atas pokok penuh sambil melaporkan sukses, dan selisihnya baru
// terlihat saat rekonsiliasi cadangan.
func (s *ppapService) collateralValues(ctx context.Context, snapshots []domain.PPAPLoanSnapshot) (map[uuid.UUID][]domain.LoanCollateral, error) {
	if s.collateralRepo == nil || s.config == nil {
		return nil, nil
	}
	raw := strings.ToLower(strings.TrimSpace(s.config.GetString(ctx, domain.PPAPCollateralEnabledKey, "false")))
	if raw != "true" && raw != "1" && raw != "ya" {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(snapshots))
	for _, snap := range snapshots {
		ids = append(ids, snap.LoanID)
	}
	list, err := s.collateralRepo.ListActiveByLoans(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("mengambil nilai agunan untuk PPAP: %w", err)
	}

	byLoan := make(map[uuid.UUID][]domain.LoanCollateral, len(ids))
	for _, c := range list {
		byLoan[c.LoanID] = append(byLoan[c.LoanID], c)
	}
	return byLoan, nil
}

// processLoan menghitung dan menerapkan PPAP satu kredit. preview=true hanya menghitung.
func (s *ppapService) processLoan(
	ctx context.Context,
	asOf time.Time,
	snap domain.PPAPLoanSnapshot,
	thresholds domain.CollectibilityThresholds,
	rates domain.PPAPRates,
	collaterals []domain.LoanCollateral,
	actor domain.Actor,
	preview bool,
) (domain.PPAPRunItem, error) {
	// Dasar PPKA adalah NILAI TERCATAT kredit setelah kerugian restrukturisasi. Menurut
	// Pasal 32 POJK 1/2024 jo. PA BPR Bab 5.2 hlm. 60-61, kerugian restrukturisasi sudah
	// mengurangi nilai tercatat Kredit, sehingga PPKA tidak boleh dihitung atas pokok
	// bruto. required_ppap TETAP berarti cadangan yang sudah diakui; yang berubah hanya
	// dasar perhitungannya, bukan arti kolom.
	carrying := domain.PPAPCarryingAmount(snap.Outstanding, snap.RestructureLoss)

	// Kredit tidak aktif (PAID_OFF/WRITTEN_OFF/CANCELLED) yang masih menyimpan cadangan
	// tidak punya eksposur lagi: targetnya dipaksa nol agar pemulihan dilepas lewat jalur
	// yang sudah ada. Status kosong (pemanggil lama/uji) diperlakukan aktif.
	inactive := snap.Status != "" && !snap.Status.IsPPAPActive()

	appliedCollateral := decimal.Zero
	exposure := carrying
	col := snap.Collectibility
	dpd := snap.DPD
	var calc domain.PPAPCalculation

	if inactive {
		calc = domain.PPAPCalculation{
			Outstanding:    carrying,
			Collectibility: col,
			Target:         decimal.Zero,
			Existing:       snap.RequiredPPAP,
			Adjustment:     domain.PPAPAdjustment(decimal.Zero, snap.RequiredPPAP),
		}
	} else {
		dpd = 0
		if snap.LastDueDate != nil {
			dpd = daysPastDue(asOf, *snap.LastDueDate)
		}
		col = domain.CollectibilityFromPosition(dpd, DaysPastMaturity(asOf, snap.FinalDueDate), thresholds)
		if snap.IsRestructured {
			// Pasal 31 POJK 1/2024: restrukturisasi tidak boleh menaikkan golongan sebelum
			// 3 periode pembayaran bersih berturut-turut.
			col = domain.RestructureCollectibility(snap.PreRestructureCollectibility, col, snap.CleanPeriods)
		}

		// Cadangan yang sudah ada untuk kredit ini adalah target terakhir yang tersimpan
		// di loans.required_ppap. Saldo akun GL cadangan bersifat agregat portofolio,
		// sehingga memakainya sebagai pengurang per kredit akan menggandakan/menghilangkan
		// selisih antar kredit. Saldo GL tetap dipakai untuk rekonsiliasi awal/akhir run.
		//
		// Agunan mengurangi eksposur yang dikenai tarif, bukan cadangan yang sudah ada.
		// Pengurang dihitung per agunan lewat domain: hanya agunan yang lolos Pasal 20 dan
		// Pasal 21 POJK No. 1 Tahun 2024 yang dipakai. Selama ppap.collateral.enabled false,
		// daftar agunan kosong sehingga perilaku PPAP identik dengan sebelum modul agunan ada.
		collateralValue := domain.PPAPCollateralDeductionTotal(collaterals, domain.PPAPPasal20Context{
			AsOf:           asOf,
			Collectibility: col,
			MacetAt:        snap.MacetAt,
			Outstanding:    carrying,
		})
		// Dua rezim yang TIDAK boleh disamakan:
		//   - PPKA umum (kualitas Lancar, Pasal 19 ayat (2)): bagian yang dijamin agunan tunai
		//     dikecualikan (Pasal 19 ayat (4) huruf b jo. Pasal 17). Pengurang Pasal 20 justru
		//     mengatur PPKA khusus (Pasal 19 ayat (3)), sehingga tidak dikurangkan dari dasar
		//     PPKA umum.
		//   - PPKA khusus: dikurangi pengurang Pasal 20 ayat (1). Agunan tunai tidak masuk
		//     daftar itu, jadi tidak mengurangi PPKA khusus (Pasal 20 ayat (2)).
		appliedCollateral = collateralValue
		exposure = domain.PPAPExposure(carrying, collateralValue)
		if col == domain.KolLancar {
			// Pemisahan porsi eksplisit (Pasal 17 ayat (1) jo. Pasal 19 ayat (4) huruf b):
			// hanya bagian yang benar-benar dijamin agunan tunai yang dikecualikan dari
			// PPKA umum, dan porsi itu dibatasi pada eksposur. Porsi yang tidak dijamin
			// tetap dihitung dengan tarif umumnya.
			cashValue := domain.PPAPCashCollateralTotal(collaterals)
			guaranteed, unguaranteed := domain.PPAPLancarPortions(carrying, cashValue)
			appliedCollateral = guaranteed
			exposure = unguaranteed
		}
		calc = domain.CalculatePPAP(exposure, col, snap.RequiredPPAP, rates)
	}

	stop := col.IsNPL() // golongan 3-5: akrual dihentikan (cash basis) sesuai POJK
	accrual := domain.AccrualStatusAccrual
	if stop {
		accrual = domain.AccrualStatusCash
	}

	item := domain.PPAPRunItem{
		LoanID:                snap.LoanID,
		LoanNumber:            snap.LoanNumber,
		DPD:                   dpd,
		Collectibility:        col,
		Outstanding:           snap.Outstanding,
		CollateralValue:       appliedCollateral,
		Exposure:              exposure,
		CarryingAmount:        carrying,
		Target:                calc.Target,
		Existing:              calc.Existing,
		Adjustment:            calc.Adjustment,
		CollectibilityChanged: col != snap.Collectibility,
		StopAccrual:           stop,
	}

	changed := item.CollectibilityChanged || dpd != snap.DPD ||
		!calc.Adjustment.IsZero() || !calc.Target.Equal(snap.RequiredPPAP)
	if !changed || preview {
		return item, nil
	}

	// Pemetaan jurnal produk dipakai bila tersedia; jika tidak, fallback memakai COA
	// dari konfigurasi. Buku produk menentukan fallback konvensional/syariah.
	var product *domain.BankingProduct
	if snap.ProductID != nil {
		p, err := s.productRepo.GetByID(ctx, *snap.ProductID)
		if err == nil {
			product = p
		} else {
			slog.WarnContext(ctx, "produk kredit tidak terbaca; memakai fallback COA PPAP",
				"loan", snap.LoanNumber, "error", err)
		}
	}
	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}

	err := s.txRunner.Run(ctx, func(tx any) error {
		if !calc.Adjustment.IsZero() {
			if err := s.postAdjustment(ctx, tx, product, book, snap, calc.Adjustment, col, asOf, actor); err != nil {
				return err
			}
			item.Posted = true
		}
		return s.repo.UpdateLoanState(ctx, tx, domain.PPAPLoanUpdate{
			LoanID:         snap.LoanID,
			Collectibility: col,
			DPD:            dpd,
			AccrualStatus:  accrual,
			StopAccrual:    stop,
			RequiredPPAP:   calc.Target,
		})
	})
	if err != nil {
		return domain.PPAPRunItem{}, fmt.Errorf("kredit %s: %w", snap.LoanNumber, err)
	}
	return item, nil
}

// postAdjustment memposting selisih PPAP. Positif = provisi (debit beban, kredit
// cadangan); negatif = reversal (debit cadangan, kredit beban).
func (s *ppapService) postAdjustment(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	book domain.COABook,
	snap domain.PPAPLoanSnapshot,
	adjustment decimal.Decimal,
	col domain.Collectibility,
	asOf time.Time,
	actor domain.Actor,
) error {
	event := domain.EventPPAPProvision
	if adjustment.IsNegative() {
		event = domain.EventPPAPReversal
	}

	// Jurnal diatribusikan ke cabang KREDIT, bukan cabang aktor. Kredit cabang lain
	// yang ikut diproses run aktor cabang S tidak boleh mencatat cadangannya di cabang
	// aktor. Cabang kredit kosong (data lama/uji) jatuh ke cabang aktor.
	branchCode := snap.BranchCode
	if branchCode == "" {
		branchCode = actor.BranchCode
	}

	meta := PostingMeta{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("PPAP kredit %s golongan %s (%s)", snap.LoanNumber, col.Label(), adjustment.String()),
		IdempotencyKey:  fmt.Sprintf("PPAP-%s-%s-%s", snap.LoanNumber, asOf.Format("2006-01-02"), adjustment.String()),
		CreatedBy:       actor.DisplayName(),
		BranchCode:      branchCode,
	}
	amount := adjustment.Abs()

	// Jalur utama: pemetaan jurnal produk. Seed 000005 memetakan PPAP_PROVISION untuk
	// produk kredit contoh; PPAP_REVERSAL biasanya belum dipetakan sehingga jatuh ke
	// fallback COA konfigurasi di bawah.
	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, event)
		if err != nil {
			// Galat pembacaan pemetaan BUKAN "produk belum dipetakan". Menjatuhkannya
			// ke COA fallback membuat produk yang sudah memetakan jurnalnya tanpa jejak
			// terjurnal ke akun bawaan saat ada galat sesaat. Tidak ada pemetaan
			// (len==0) tetap memakai fallback.
			return fmt.Errorf("membaca pemetaan jurnal PPAP produk %s: %w", product.Code, err)
		}
		if len(rules) > 0 {
			if _, err := s.poster.PostEventTx(ctx, tx, product, event, Amounts{
				Principal: amount,
				Total:     amount,
			}, meta); err != nil {
				return fmt.Errorf("jurnal PPAP produk %s: %w", product.Code, err)
			}
			return nil
		}
	}

	return s.postAdjustmentFallback(ctx, tx, amount, adjustment.IsPositive(), book, meta)
}

// postAdjustmentFallback memposting PPAP memakai kode COA dari konfigurasi
// (ppap.coa.expense/ppap.coa.reserve) dengan fallback per buku produk.
func (s *ppapService) postAdjustmentFallback(
	ctx context.Context,
	tx any,
	amount decimal.Decimal,
	provision bool,
	book domain.COABook,
	meta PostingMeta,
) error {
	expenseCOA := s.expenseCOA(ctx, book)
	reserveCOA := s.reserveCOA(ctx, book)

	expenseAcc, err := s.resolver.ResolveGLAccount(ctx, tx, expenseCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrPPAPExpenseNotFound, expenseCOA, err)
	}
	reserveAcc, err := s.resolver.ResolveGLAccount(ctx, tx, reserveCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrPPAPReserveNotFound, reserveCOA, err)
	}

	var debitAcc, creditAcc string
	if provision {
		debitAcc, creditAcc = expenseAcc, reserveAcc
	} else {
		debitAcc, creditAcc = reserveAcc, expenseAcc
	}

	lines := []domain.PostingLine{
		{AccountNumber: debitAcc, Direction: domain.DirectionDebit, Amount: amount, Description: meta.Description},
		{AccountNumber: creditAcc, Direction: domain.DirectionCredit, Amount: amount, Description: meta.Description},
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

func (s *ppapService) totalReserve(ctx context.Context) decimal.Decimal {
	total := decimal.Zero
	for _, book := range []domain.COABook{domain.BookConventional, domain.BookSyariah} {
		code := s.reserveCOA(ctx, book)
		balance, err := s.repo.GetPPAPReserveBalance(ctx, nil, code)
		if err != nil {
			slog.WarnContext(ctx, "saldo cadangan PPAP tidak terbaca", "coa", code, "error", err)
			continue
		}
		total = total.Add(balance)
	}
	return total
}

func (s *ppapService) expenseCOA(ctx context.Context, book domain.COABook) string {
	fallback := fallbackPPAPExpenseConventional
	if book == domain.BookSyariah {
		fallback = fallbackPPAPExpenseSyariah
	}
	return configStringOr(ctx, s.config, cfgPPAPExpenseCOA, fallback)
}

func (s *ppapService) reserveCOA(ctx context.Context, book domain.COABook) string {
	fallback := fallbackPPAPReserveConventional
	if book == domain.BookSyariah {
		fallback = fallbackPPAPReserveSyariah
	}
	return configStringOr(ctx, s.config, cfgPPAPReserveCOA, fallback)
}

// daysPastDue menghitung selisih hari kalender (bukan jam) antara asOf dan jatuh tempo.
func daysPastDue(asOf, due time.Time) int {
	a := time.Date(asOf.Year(), asOf.Month(), asOf.Day(), 0, 0, 0, 0, time.UTC)
	d := time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, time.UTC)
	days := int(a.Sub(d).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

var _ domain.PPAPService = (*ppapService)(nil)
