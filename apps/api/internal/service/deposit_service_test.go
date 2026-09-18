package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ── Stub repository & poster (tanpa database) ───────────────────────────────

type stubDepositRepo struct {
	deposit *domain.Deposit
}

func (s *stubDepositRepo) Create(ctx context.Context, tx any, d *domain.Deposit) error {
	s.deposit = d
	return nil
}
func (s *stubDepositRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Deposit, error) {
	return s.deposit, nil
}
func (s *stubDepositRepo) GetByIDForUpdate(ctx context.Context, tx any, id uuid.UUID) (*domain.Deposit, error) {
	return s.deposit, nil
}
func (s *stubDepositRepo) List(ctx context.Context, limit, offset int) ([]domain.Deposit, int, error) {
	return nil, 0, nil
}
func (s *stubDepositRepo) ListMaturedARO(ctx context.Context, asOf time.Time) ([]domain.Deposit, error) {
	return nil, nil
}
func (s *stubDepositRepo) AddAccrual(ctx context.Context, tx any, id uuid.UUID, profit, tax decimal.Decimal, asOf time.Time) error {
	return nil
}
func (s *stubDepositRepo) UpdateStatus(ctx context.Context, tx any, id uuid.UUID, status domain.DepositStatus, proceeds, paidProfit, paidTax decimal.Decimal) error {
	return nil
}
func (s *stubDepositRepo) Rollover(ctx context.Context, tx any, id uuid.UUID, newPrincipal, paidProfit, paidTax decimal.Decimal, newStart, newMaturity time.Time, resetAccrual bool) error {
	return nil
}

type stubDepositProductRepo struct {
	product *domain.BankingProduct
	err     error
}

func (s *stubDepositProductRepo) List(ctx context.Context) ([]domain.BankingProduct, error) {
	return nil, nil
}
func (s *stubDepositProductRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.BankingProduct, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.product, nil
}
func (s *stubDepositProductRepo) GetByCode(ctx context.Context, code string) (*domain.BankingProduct, error) {
	return s.product, s.err
}
func (s *stubDepositProductRepo) GetMapping(ctx context.Context, productID uuid.UUID, event domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return nil, nil
}

type stubDepositPoster struct {
	calls []domain.PostingEvent
}

func (s *stubDepositPoster) PostEventTx(ctx context.Context, tx any, product *domain.BankingProduct, event domain.PostingEvent, amounts Amounts, meta PostingMeta) (*domain.JournalEntry, error) {
	s.calls = append(s.calls, event)
	return &domain.JournalEntry{}, nil
}

type stubDepositConfig struct {
	values map[string]decimal.Decimal
}

func (c *stubDepositConfig) GetDecimal(ctx context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}
func (c *stubDepositConfig) GetInt(ctx context.Context, key string, fallback int) int {
	return fallback
}
func (c *stubDepositConfig) GetString(ctx context.Context, key, fallback string) string {
	return fallback
}
func (c *stubDepositConfig) GetBool(ctx context.Context, key string, fallback bool) bool {
	return fallback
}
func (c *stubDepositConfig) Invalidate(key string) {}

func newTestDepositService(product *domain.BankingProduct, cfg domain.SystemConfigService) *depositService {
	return &depositService{
		depositRepo: &stubDepositRepo{},
		productRepo: &stubDepositProductRepo{product: product},
		poster:      &stubDepositPoster{},
		configSvc:   cfg,
	}
}

// ── Validasi Place ──────────────────────────────────────────────────────────

func TestDepositPlaceMenolakNominalTidakPositif(t *testing.T) {
	svc := newTestDepositService(nil, nil)
	_, err := svc.Place(context.Background(), domain.PlaceDepositInput{
		PlacementAmount: decimal.Zero,
		TermMonths:      3,
	}, domain.Actor{})
	if !errors.Is(err, domain.ErrInvalidDepositAmount) {
		t.Fatalf("mau ErrInvalidDepositAmount, dapat %v", err)
	}
}

func TestDepositPlaceMenolakTenorTidakPositif(t *testing.T) {
	svc := newTestDepositService(nil, nil)
	_, err := svc.Place(context.Background(), domain.PlaceDepositInput{
		PlacementAmount: decimal.NewFromInt(1_000_000),
		TermMonths:      0,
	}, domain.Actor{})
	if !errors.Is(err, domain.ErrInvalidDepositTerm) {
		t.Fatalf("mau ErrInvalidDepositTerm, dapat %v", err)
	}
}

func TestDepositPlaceMenolakProdukBukanDeposito(t *testing.T) {
	product := &domain.BankingProduct{
		ID:       uuid.New(),
		Code:     "TAB-CONV",
		Family:   domain.FamilySavings,
		IsActive: true,
	}
	svc := newTestDepositService(product, nil)
	_, err := svc.Place(context.Background(), domain.PlaceDepositInput{
		PlacementAmount: decimal.NewFromInt(1_000_000),
		TermMonths:      3,
		ProductID:       product.ID,
	}, domain.Actor{})
	if !errors.Is(err, domain.ErrDepositProductInvalid) {
		t.Fatalf("mau ErrDepositProductInvalid, dapat %v", err)
	}
}

// ── Perhitungan murni lewat helper service ──────────────────────────────────

func TestDepositDailyAccrualKonvensional(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		ProfitType:      domain.ProfitTypeInterest,
		ProfitRate:      decimal.NewFromInt(4),
		TaxRate:         decimal.NewFromInt(20),
	}
	profit, tax := depositDailyAccrual(dep)
	if !profit.Equal(decimal.NewFromInt(1096)) {
		t.Fatalf("bunga harian = %s, mau 1096", profit)
	}
	if !tax.Equal(decimal.NewFromInt(219)) {
		t.Fatalf("pajak harian = %s, mau 219", tax)
	}
}

func TestDepositDailyAccrualBagiHasilTanpaPajak(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		ProfitType:      domain.ProfitTypeBagiHasil,
		ProfitRate:      decimal.NewFromFloat(0.6),
		YieldRate:       decimal.NewFromInt(6),
		TaxRate:         decimal.Zero,
	}
	profit, tax := depositDailyAccrual(dep)
	if !profit.Equal(decimal.NewFromInt(986)) {
		t.Fatalf("bagi hasil harian = %s, mau 986", profit)
	}
	if !tax.IsZero() {
		t.Fatalf("syariah tidak boleh dipotong pajak, dapat %s", tax)
	}
}

func TestDepositPayoutAmountsMemotongPajak(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		AccruedProfit:   decimal.NewFromInt(1_200_000),
		AccruedTax:      decimal.NewFromInt(240_000),
	}
	net, tax, proceeds := depositPayoutAmounts(dep)
	if !net.Equal(decimal.NewFromInt(960_000)) {
		t.Fatalf("laba bersih = %s, mau 960000", net)
	}
	if !tax.Equal(decimal.NewFromInt(240_000)) {
		t.Fatalf("pajak = %s, mau 240000", tax)
	}
	if !proceeds.Equal(decimal.NewFromInt(10_960_000)) {
		t.Fatalf("hak jatuh tempo = %s, mau 10960000", proceeds)
	}
}

// ── Aturan tarif & pajak ────────────────────────────────────────────────────

func TestDepositTermsPajakDefaultKonvensional(t *testing.T) {
	product := &domain.BankingProduct{
		Book:         domain.BookConventional,
		ProfitScheme: domain.SchemeInterest,
		RateAnnual:   decimal.NewFromInt(4),
		TaxRate:      decimal.Zero,
	}
	_, rate, _, tax := depositTerms(context.Background(), &stubDepositConfig{}, product, domain.PlaceDepositInput{})
	if !tax.Equal(decimal.NewFromInt(20)) {
		t.Fatalf("pajak default = %s, mau 20", tax)
	}
	if !rate.Equal(decimal.NewFromInt(4)) {
		t.Fatalf("bunga = %s, mau 4", rate)
	}
}

func TestDepositTermsPajakDariKonfigurasi(t *testing.T) {
	product := &domain.BankingProduct{
		Book:         domain.BookConventional,
		ProfitScheme: domain.SchemeInterest,
		RateAnnual:   decimal.NewFromInt(4),
	}
	cfg := &stubDepositConfig{values: map[string]decimal.Decimal{
		depositTaxRateConfigKey: decimal.NewFromInt(15),
	}}
	_, _, _, tax := depositTerms(context.Background(), cfg, product, domain.PlaceDepositInput{})
	if !tax.Equal(decimal.NewFromInt(15)) {
		t.Fatalf("pajak dari konfigurasi = %s, mau 15", tax)
	}
}

func TestDepositTermsSyariahTanpaPajakDanNisbah(t *testing.T) {
	product := &domain.BankingProduct{
		Book:               domain.BookSyariah,
		ProfitScheme:       domain.SchemeMudharabah,
		ProfitSharingRatio: decimal.NewFromFloat(0.6),
		TaxRate:            decimal.NewFromInt(20),
	}
	profitType, rate, yieldRate, tax := depositTerms(context.Background(), &stubDepositConfig{}, product, domain.PlaceDepositInput{})
	if profitType != domain.ProfitTypeBagiHasil {
		t.Fatalf("profit type = %s, mau BAGI_HASIL", profitType)
	}
	if !rate.Equal(decimal.NewFromFloat(0.6)) {
		t.Fatalf("nisbah = %s, mau 0.6", rate)
	}
	if !yieldRate.IsZero() {
		t.Fatalf("yield belum dikonfigurasi harus 0, dapat %s", yieldRate)
	}
	if !tax.IsZero() {
		t.Fatalf("syariah bebas pajak, dapat %s", tax)
	}
}
