package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kunci konfigurasi kebijakan CKPN. Tidak ada angka asumsi yang ditanam di kode:
// PD dan LGD wajib diisi bank dari data historisnya (SEOJK No. 21/SEOJK.03/2024
// Bab XII butir 12.4.g.2.c dan 12.6–12.7).
const (
	cfgCKPNEnabled        = "ckpn.enabled"
	cfgCKPNLGD            = "ckpn.lgd"
	cfgCKPNAsetBaikMaxDPD = "ckpn.aset_baik.max_dpd"
	cfgCKPNExpenseCOA     = "ckpn.coa.expense"
	cfgCKPNReserveCOA     = "ckpn.coa.reserve"
)

// Fallback kode COA CKPN bila bank belum mengisi pemetaannya. Kode ini di-seed
// migrasi 000042; 10900/50200 tetap milik PPAP dan tidak boleh dicampur agar
// perbandingan CKPN vs PPKA tidak saling mengurangi.
const (
	fallbackCKPNExpenseConventional = "50301" // Beban Kerugian Penurunan Nilai - Kredit
	fallbackCKPNExpenseSyariah      = "15901" // Beban Kerugian Penurunan Nilai - Pembiayaan
	fallbackCKPNReserveConventional = "10950" // CKPN - Kredit
	fallbackCKPNReserveSyariah      = "11950" // CKPN - Pembiayaan
)

// ckpnTxRunner membuka transaksi per kredit; sama polanya dengan ppapTxRunner agar
// pemrosesan dapat diuji tanpa database.
type ckpnTxRunner interface {
	Run(ctx context.Context, fn func(tx any) error) error
}

type sqlCKPNTxRunner struct{ db *sql.DB }

func (r sqlCKPNTxRunner) Run(ctx context.Context, fn func(tx any) error) error {
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

// ckpnLoanLocker adalah irisan domain.LoanRepository yang dibutuhkan CKPN. Kredit
// yang menyentuh baris loans wajib dikunci lebih dulu (SELECT ... FOR UPDATE) sesuai
// disiplin LockLoanTx, tetapi service ini tidak perlu memegang seluruh antarmuka.
type ckpnLoanLocker interface {
	LockLoanTx(ctx context.Context, tx any, id uuid.UUID) (*domain.Loan, error)
}

type ckpnService struct {
	txRunner    ckpnTxRunner
	repo        domain.CKPNRepository
	productRepo domain.ProductRepository
	resolver    domain.AccountResolver
	poster      *ProductPoster
	posting     domain.PostingService
	config      domain.SystemConfigService
	locker      ckpnLoanLocker
}

func NewCKPNService(
	db *sql.DB,
	repo domain.CKPNRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	poster *ProductPoster,
	posting domain.PostingService,
	config domain.SystemConfigService,
	locker ckpnLoanLocker,
) domain.CKPNService {
	return &ckpnService{
		txRunner:    sqlCKPNTxRunner{db: db},
		repo:        repo,
		productRepo: productRepo,
		resolver:    resolver,
		poster:      poster,
		posting:     posting,
		config:      config,
		locker:      locker,
	}
}

// Compare menghitung dan membandingkan CKPN vs PPKA tanpa memposting atau menulis state.
func (s *ckpnService) Compare(ctx context.Context, asOf time.Time) (domain.CKPNComparisonSummary, error) {
	return s.run(ctx, asOf, domain.Actor{}, false)
}

// Run menghitung CKPN, memposting selisihnya, lalu menyimpan target per kredit.
func (s *ckpnService) Run(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.CKPNComparisonSummary, error) {
	return s.run(ctx, asOf, actor, true)
}

func (s *ckpnService) run(ctx context.Context, asOf time.Time, actor domain.Actor, post bool) (domain.CKPNComparisonSummary, error) {
	asOf = asOf.UTC()

	policy := s.policy(ctx)
	summary := domain.CKPNComparisonSummary{
		Enabled: policy.Enabled,
		AsOf:    asOf,
		Preview: !post,
	}

	// Saklar mati berarti tidak ada query tambahan sama sekali: kredit tidak dibaca dan
	// tidak ada state/jurnal yang disentuh. Ini pola yang sama dengan
	// ppap.collateral.enabled.
	if !policy.Enabled {
		return summary, nil
	}

	snapshots, err := s.repo.ListActiveLoans(ctx)
	if err != nil {
		return summary, fmt.Errorf("mengambil daftar kredit untuk CKPN: %w", err)
	}
	summary.Total = len(snapshots)

	for _, snap := range snapshots {
		calc, err := domain.CalculateCKPN(snap, policy)
		if err != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, domain.CKPNRunFailure{
				LoanID:     snap.LoanID,
				LoanNumber: snap.LoanNumber,
				Error:      err.Error(),
			})
			continue
		}

		if post && !calc.Adjustment.IsZero() {
			if err := s.apply(ctx, snap, calc, asOf, actor); err != nil {
				summary.Failed++
				summary.Failures = append(summary.Failures, domain.CKPNRunFailure{
					LoanID:     snap.LoanID,
					LoanNumber: snap.LoanNumber,
					Error:      err.Error(),
				})
				continue
			}
		}

		// Perbandingan memakai target, bukan saldo GL: target PPKA per kredit sudah
		// tersimpan di loans.required_ppap hasil jalur PPAP, dan target CKPN dihitung
		// di sini. Saldo GL bersifat agregat portofolio sehingga tidak dapat dipakai
		// per kredit.
		item := ckpnCompare(snap, calc)
		summary.Processed++
		summary.Items = append(summary.Items, item)
		summary.TotalCKPN = summary.TotalCKPN.Add(calc.Target)
		summary.TotalPPKA = summary.TotalPPKA.Add(snap.RequiredPPAP)
		if diff := snap.RequiredPPAP.Sub(calc.Target); diff.IsPositive() {
			summary.ModalIntiDeduction = summary.ModalIntiDeduction.Add(diff)
		}
	}

	return summary, nil
}

// ckpnCompare menyusun perbandingan satu kredit. Difference = PPKA - CKPN; positif
// berarti PPKA lebih besar dan selisih itu menjadi pengurang modal inti (SEOJK
// No. 21/SEOJK.03/2024 butir 1.1.6).
func ckpnCompare(snap domain.CKPNLoanSnapshot, calc domain.CKPNCalculation) domain.CKPNComparisonItem {
	larger := domain.CKPNLargerSame
	switch {
	case snap.RequiredPPAP.GreaterThan(calc.Target):
		larger = domain.CKPNLargerPPKA
	case calc.Target.GreaterThan(snap.RequiredPPAP):
		larger = domain.CKPNLargerCKPN
	}
	return domain.CKPNComparisonItem{
		LoanID:         snap.LoanID,
		LoanNumber:     snap.LoanNumber,
		Outstanding:    snap.Outstanding,
		Collectibility: snap.Collectibility,
		IsAsetBaik:     calc.IsAsetBaik,
		CKPN:           calc.Target,
		PPKA:           snap.RequiredPPAP,
		Difference:     snap.RequiredPPAP.Sub(calc.Target),
		Larger:         larger,
	}
}

// apply memposting selisih dan menyimpan target CKPN dalam satu transaksi. Kredit
// dikunci lebih dulu karena baris loans disentuh (disiplin LockLoanTx).
func (s *ckpnService) apply(ctx context.Context, snap domain.CKPNLoanSnapshot, calc domain.CKPNCalculation, asOf time.Time, actor domain.Actor) error {
	err := s.txRunner.Run(ctx, func(tx any) error {
		if s.locker != nil {
			if _, err := s.locker.LockLoanTx(ctx, tx, snap.LoanID); err != nil {
				return fmt.Errorf("mengunci kredit %s: %w", snap.LoanNumber, err)
			}
		}
		if err := s.postAdjustment(ctx, tx, snap, calc, asOf, actor); err != nil {
			return err
		}
		return s.repo.UpdateRequiredCKPN(ctx, tx, snap.LoanID, calc.Target)
	})
	if err != nil {
		return fmt.Errorf("kredit %s: %w", snap.LoanNumber, err)
	}
	return nil
}

// postAdjustment memposting selisih CKPN. Positif = pembentukan (debit beban, kredit
// CKPN); negatif = pemulihan (debit CKPN, kredit beban), sesuai SEOJK 21/2024 butir
// 12.5 dan contoh jurnal butir 12.10.
func (s *ckpnService) postAdjustment(ctx context.Context, tx any, snap domain.CKPNLoanSnapshot, calc domain.CKPNCalculation, asOf time.Time, actor domain.Actor) error {
	event := domain.EventCKPNProvision
	if calc.Adjustment.IsNegative() {
		event = domain.EventCKPNReversal
	}

	meta := PostingMeta{
		TransactionType: domain.TxTypeAdjustment,
		Description:     fmt.Sprintf("CKPN kredit %s golongan %s (%s)", snap.LoanNumber, calc.Collectibility.Label(), calc.Adjustment.String()),
		IdempotencyKey:  fmt.Sprintf("CKPN-%s-%s-%s", snap.LoanNumber, asOf.Format("2006-01-02"), calc.Adjustment.String()),
		CreatedBy:       actor.DisplayName(),
		BranchCode:      actor.BranchCode,
	}
	amount := calc.Adjustment.Abs()

	// Jalur utama: pemetaan jurnal produk untuk peristiwa CKPN_*. Bila produk belum
	// punya pemetaan (mis. migrasi 000042 tidak dapat menanamnya karena nilai enum baru
	// tidak boleh dipakai dalam transaksi yang sama), jatuh ke COA konfigurasi.
	var product *domain.BankingProduct
	if snap.ProductID != nil {
		p, err := s.productRepo.GetByID(ctx, *snap.ProductID)
		if err == nil {
			product = p
		} else {
			slog.WarnContext(ctx, "produk kredit tidak terbaca; memakai fallback COA CKPN",
				"loan", snap.LoanNumber, "error", err)
		}
	}

	if product != nil {
		rules, err := s.productRepo.GetMapping(ctx, product.ID, event)
		if err == nil && len(rules) > 0 {
			if _, err := s.poster.PostEventTx(ctx, tx, product, event, Amounts{
				Principal: amount,
				Total:     amount,
			}, meta); err != nil {
				return fmt.Errorf("jurnal CKPN produk %s: %w", product.Code, err)
			}
			return nil
		}
	}

	book := domain.BookConventional
	if product != nil && product.Book == domain.BookSyariah {
		book = domain.BookSyariah
	}
	return s.postAdjustmentFallback(ctx, tx, amount, calc.Adjustment.IsPositive(), book, meta)
}

// postAdjustmentFallback memposting CKPN memakai COA dari konfigurasi
// (ckpn.coa.expense/ckpn.coa.reserve) dengan fallback per buku produk.
func (s *ckpnService) postAdjustmentFallback(ctx context.Context, tx any, amount decimal.Decimal, provision bool, book domain.COABook, meta PostingMeta) error {
	expenseCOA := s.expenseCOA(ctx, book)
	reserveCOA := s.reserveCOA(ctx, book)

	expenseAcc, err := s.resolver.ResolveGLAccount(ctx, tx, expenseCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrCKPNExpenseNotFound, expenseCOA, err)
	}
	reserveAcc, err := s.resolver.ResolveGLAccount(ctx, tx, reserveCOA)
	if err != nil {
		return fmt.Errorf("%w: COA %s: %v", domain.ErrCKPNReserveNotFound, reserveCOA, err)
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

// policy membaca parameter kebijakan bank dari konfigurasi. Kunci PD/LGD yang belum
// diisi sengaja tidak diberi nilai default: kredit yang membutuhkannya akan gagal
// dengan ErrCKPNParameterMissing, bukan dihitung dengan nol.
func (s *ckpnService) policy(ctx context.Context) domain.CKPNPolicy {
	p := domain.CKPNPolicy{
		PD:             make(map[domain.Collectibility]decimal.Decimal),
		AsetBaikMaxDPD: domain.CKPNAsetBaikMaxDPDDefault,
	}
	if s.config == nil {
		return p
	}

	p.Enabled = s.config.GetBool(ctx, cfgCKPNEnabled, false)
	if !p.Enabled {
		return p
	}

	if v := s.config.GetInt(ctx, cfgCKPNAsetBaikMaxDPD, domain.CKPNAsetBaikMaxDPDDefault); v >= 0 {
		p.AsetBaikMaxDPD = v
	}
	for _, c := range []domain.Collectibility{
		domain.KolLancar, domain.KolDPK, domain.KolKurangLancar, domain.KolDiragukan, domain.KolMacet,
	} {
		if v, ok := configDecimalSet(ctx, s.config, ckpnPDKey(c)); ok {
			p.PD[c] = v
		}
	}
	if v, ok := configDecimalSet(ctx, s.config, cfgCKPNLGD); ok {
		p.LGD = v
		p.LGDIsSet = true
	}
	return p
}

func ckpnPDKey(c domain.Collectibility) string {
	return "ckpn.pd." + strconv.Itoa(int(c))
}

// configDecimalSet membaca nilai desimal dan membedakan "kosong/tidak valid" dari
// "nol yang diisi sengaja". GetDecimal tidak dapat membedakannya karena fallback
// dikembalikan pada kedua keadaan.
func configDecimalSet(ctx context.Context, config domain.SystemConfigService, key string) (decimal.Decimal, bool) {
	raw := strings.TrimSpace(config.GetString(ctx, key, ""))
	if raw == "" {
		return decimal.Zero, false
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, false
	}
	return d, true
}

func (s *ckpnService) expenseCOA(ctx context.Context, book domain.COABook) string {
	fallback := fallbackCKPNExpenseConventional
	if book == domain.BookSyariah {
		fallback = fallbackCKPNExpenseSyariah
	}
	return configStringOr(ctx, s.config, cfgCKPNExpenseCOA, fallback)
}

func (s *ckpnService) reserveCOA(ctx context.Context, book domain.COABook) string {
	fallback := fallbackCKPNReserveConventional
	if book == domain.BookSyariah {
		fallback = fallbackCKPNReserveSyariah
	}
	return configStringOr(ctx, s.config, cfgCKPNReserveCOA, fallback)
}

var _ domain.CKPNService = (*ckpnService)(nil)
