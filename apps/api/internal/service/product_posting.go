package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// ProductPoster menerjemahkan peristiwa produk menjadi baris jurnal konkret dengan
// membaca product_journal_mapping, lalu menyerahkannya ke posting engine. Service
// produk tidak perlu mengetahui kode akun sama sekali.
type ProductPoster struct {
	productRepo domain.ProductRepository
	resolver    domain.AccountResolver
	posting     domain.PostingService
}

func NewProductPoster(
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	posting domain.PostingService,
) *ProductPoster {
	return &ProductPoster{
		productRepo: productRepo,
		resolver:    resolver,
		posting:     posting,
	}
}

// Amounts adalah nilai-nilai yang tersedia untuk dipetakan ke sisi jurnal.
type Amounts struct {
	Principal decimal.Decimal
	Profit    decimal.Decimal
	Fee       decimal.Decimal
	Tax       decimal.Decimal
	Penalty   decimal.Decimal
	Total     decimal.Decimal
}

func (a Amounts) pick(source domain.AmountSource) decimal.Decimal {
	switch source {
	case domain.AmountPrincipal:
		return a.Principal
	case domain.AmountProfit:
		return a.Profit
	case domain.AmountFee:
		return a.Fee
	case domain.AmountTax:
		return a.Tax
	case domain.AmountPenalty:
		return a.Penalty
	case domain.AmountTotal:
		return a.Total
	default:
		return decimal.Zero
	}
}

// PostEventTx memposting jurnal untuk sebuah peristiwa produk di dalam transaksi yang
// disediakan pemanggil. Baris dengan nominal nol dilewati agar jurnal tidak memuat
// sisi kosong (mis. produk tanpa fee tidak menulis baris fee).
func (p *ProductPoster) PostEventTx(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	event domain.PostingEvent,
	amounts Amounts,
	meta PostingMeta,
) (*domain.JournalEntry, error) {
	rules, err := p.productRepo.GetMapping(ctx, product.ID, event)
	if err != nil {
		return nil, fmt.Errorf("membaca pemetaan jurnal produk %s: %w", product.Code, err)
	}
	if len(rules) == 0 {
		return nil, fmt.Errorf("produk %s belum punya pemetaan jurnal untuk peristiwa %s", product.Code, event)
	}

	lines := make([]domain.PostingLine, 0, len(rules))
	overridesUsed := make(map[string]bool, len(meta.AccountOverrides))
	for _, rule := range rules {
		override, hasOverride := meta.AccountOverrides[rule.COACode]
		if hasOverride && override != "" {
			overridesUsed[rule.COACode] = true
		}

		amount := amounts.pick(rule.AmountSource)
		if amount.IsZero() {
			continue
		}
		if amount.IsNegative() {
			return nil, fmt.Errorf("nominal negatif pada pemetaan %s/%s", product.Code, event)
		}

		var accountNumber string
		if hasOverride && override != "" {
			accountNumber = override
		} else {
			accountNumber, err = p.resolver.ResolveGLAccount(ctx, tx, rule.COACode)
			if err != nil {
				return nil, err
			}
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: accountNumber,
			Direction:     rule.Direction,
			Amount:        amount,
			Description:   meta.Description,
		})
	}

	// Override yang tidak dipakai berarti pemetaan produk tidak memuat akun rekening
	// nasabah pada kode COA itu — biasanya karena produk dan rekening berada di buku
	// berbeda. Tanpa pemeriksaan ini jurnal akan diam-diam jatuh ke akun kontrol GL
	// dan dana nasabah tidak pernah masuk rekeningnya.
	for coa := range meta.AccountOverrides {
		if !overridesUsed[coa] {
			return nil, fmt.Errorf(
				"pemetaan %s/%s tidak memuat akun rekening nasabah (COA %s); periksa kesesuaian buku produk dan buku rekening",
				product.Code, event, coa)
		}
	}

	if len(lines) < 2 {
		return nil, fmt.Errorf("pemetaan %s/%s menghasilkan %d baris setelah nominal nol dibuang; perlu minimal 2", product.Code, event, len(lines))
	}

	return p.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: meta.TransactionType,
		Description:     meta.Description,
		IdempotencyKey:  meta.IdempotencyKey,
		CreatedBy:       meta.CreatedBy,
		BranchCode:      meta.BranchCode,
		EntryDate:       meta.EntryDate,
		Lines:           lines,
	})
}

// PostingMeta membawa konteks non-nominal untuk jurnal. EntryDate nol berarti
// posting engine memakai TANGGAL BISNIS berjalan (bukan tanggal kalender UTC).
//
// AccountOverrides mengganti tujuan satu kode COA dengan nomor rekening konkret.
// Pemetaan produk memakai akun kontrol (mis. 20100 Tabungan), padahal untuk
// transaksi yang menyentuh rekening nasabah yang benar adalah rekening nasabah itu
// sendiri agar saldonya ikut bergerak. Kuncinya kode COA, nilainya nomor rekening.
type PostingMeta struct {
	TransactionType  domain.TransactionType
	Description      string
	IdempotencyKey   string
	CreatedBy        string
	BranchCode       string
	EntryDate        time.Time
	AccountOverrides map[string]string
}
