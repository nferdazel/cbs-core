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
func (s *stubDepositRepo) List(ctx context.Context, limit, offset int, actor domain.Actor) ([]domain.Deposit, int, error) {
	return nil, 0, nil
}
func (s *stubDepositRepo) ListMaturedARO(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.Deposit, error) {
	return nil, nil
}
func (s *stubDepositRepo) AddAccrual(ctx context.Context, tx any, id uuid.UUID, profit, tax decimal.Decimal, asOf time.Time) error {
	return nil
}
func (s *stubDepositRepo) UpdateStatus(ctx context.Context, tx any, id uuid.UUID, status domain.DepositStatus, proceeds, paidProfit, paidTax, penalty decimal.Decimal) error {
	return nil
}
func (s *stubDepositRepo) Rollover(ctx context.Context, tx any, id uuid.UUID, newPrincipal, paidProfit, paidTax decimal.Decimal, newStart, newMaturity time.Time, resetAccrual bool) error {
	return nil
}

type stubDepositProductRepo struct {
	product *domain.BankingProduct
	err     error
	// mappingRules/mappingErr menguji jalur pemetaan jurnal: membedakan "tidak ada
	// pemetaan" (boleh jatuh ke COA bawaan) dari "galat pembacaan" (harus gagal).
	mappingRules []domain.JournalMappingRule
	mappingErr   error
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
	return s.mappingRules, s.mappingErr
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
	profit, tax := depositDailyAccrual(dep, defaultDepositTaxExemptAmount)
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
	profit, tax := depositDailyAccrual(dep, defaultDepositTaxExemptAmount)
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

// ── Denda pencairan lebih awal ───────────────────────────────────────────────

func TestDepositWithdrawalAmountsDendaSebelumJatuhTempo(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		AccruedProfit:   decimal.NewFromInt(1_200_000),
		AccruedTax:      decimal.NewFromInt(240_000),
	}
	net, tax, penalty, proceeds, err := depositWithdrawalAmounts(dep, decimal.NewFromInt(100_000))
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if !net.Equal(decimal.NewFromInt(960_000)) {
		t.Fatalf("laba bersih = %s, mau 960000", net)
	}
	if !tax.Equal(decimal.NewFromInt(240_000)) {
		t.Fatalf("pajak = %s, mau 240000", tax)
	}
	if !penalty.Equal(decimal.NewFromInt(100_000)) {
		t.Fatalf("denda = %s, mau 100000", penalty)
	}
	if !proceeds.Equal(decimal.NewFromInt(10_860_000)) {
		t.Fatalf("hasil pencairan = %s, mau 10860000", proceeds)
	}
}

func TestDepositWithdrawalAmountsTanpaDendaSaatJatuhTempo(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		AccruedProfit:   decimal.NewFromInt(1_200_000),
		AccruedTax:      decimal.NewFromInt(240_000),
	}
	_, _, penalty, proceeds, err := depositWithdrawalAmounts(dep, decimal.Zero)
	if err != nil {
		t.Fatalf("tidak mau error: %v", err)
	}
	if !penalty.IsZero() {
		t.Fatalf("denda harus nol, dapat %s", penalty)
	}
	if !proceeds.Equal(decimal.NewFromInt(10_960_000)) {
		t.Fatalf("hasil pencairan = %s, mau 10960000", proceeds)
	}
}

func TestDepositWithdrawalAmountsDendaMelebihiHakNasabah(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(1_000_000),
		AccruedProfit:   decimal.NewFromInt(100_000),
		AccruedTax:      decimal.Zero,
	}
	// Denda 1,2 juta > pokok 1 juta + imbal hasil 100 ribu.
	_, _, _, proceeds, err := depositWithdrawalAmounts(dep, decimal.NewFromInt(1_200_000))
	if !errors.Is(err, domain.ErrDepositPenaltyExceedsProceeds) {
		t.Fatalf("mau ErrDepositPenaltyExceedsProceeds, dapat %v", err)
	}
	if !proceeds.IsZero() {
		t.Fatalf("proceeds saat error harus nol, dapat %s", proceeds)
	}
}

func TestDepositWithdrawalAmountsHasilTidakNegatif(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(1_000_000),
		AccruedProfit:   decimal.NewFromInt(100_000),
		AccruedTax:      decimal.Zero,
	}
	// Denda tepat sama dengan pokok + imbal hasil: hasil 0, bukan negatif.
	_, _, _, proceeds, err := depositWithdrawalAmounts(dep, decimal.NewFromInt(1_100_000))
	if err != nil {
		t.Fatalf("denda = hak nasabah harus boleh, dapat error %v", err)
	}
	if proceeds.IsNegative() {
		t.Fatalf("hasil pencairan tidak boleh negatif, dapat %s", proceeds)
	}
	if !proceeds.IsZero() {
		t.Fatalf("hasil pencairan = %s, mau 0", proceeds)
	}
}

func TestDepositPenaltySplitMenyerapBungaLaluPokok(t *testing.T) {
	fromProfit, fromPrincipal := depositPenaltySplit(decimal.NewFromInt(100_000), decimal.NewFromInt(60_000))
	if !fromProfit.Equal(decimal.NewFromInt(60_000)) || !fromPrincipal.IsZero() {
		t.Fatalf("denda <= bunga harus dari bunga: profit=%s pokok=%s", fromProfit, fromPrincipal)
	}
	fromProfit, fromPrincipal = depositPenaltySplit(decimal.NewFromInt(100_000), decimal.NewFromInt(150_000))
	if !fromProfit.Equal(decimal.NewFromInt(100_000)) || !fromPrincipal.Equal(decimal.NewFromInt(50_000)) {
		t.Fatalf("sisanya harus dari pokok: profit=%s pokok=%s", fromProfit, fromPrincipal)
	}
}

// ── Batas jatuh tempo & perpanjangan ARO ────────────────────────────────────

func TestDepositIsEarlySebelumDanSesudahJatuhTempo(t *testing.T) {
	maturity := time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)
	if !depositIsEarly(time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC), maturity) {
		t.Fatal("sehari sebelum jatuh tempo harus dianggap lebih awal")
	}
	if depositIsEarly(maturity, maturity) {
		t.Fatal("tepat jatuh tempo bukan pencairan lebih awal")
	}
	if depositIsEarly(time.Date(2026, time.March, 16, 0, 0, 0, 0, time.UTC), maturity) {
		t.Fatal("setelah jatuh tempo bukan pencairan lebih awal")
	}
}

func TestDepositRolloverTermsPokokDanProfit(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		TermMonths:      3,
		MaturityDate:    time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC),
		AROInstruction:  domain.AROInstructionPrincipalAndProfit,
	}
	newPrincipal, start, maturity, capitalise := depositRolloverTerms(dep, decimal.NewFromInt(120_000))
	if !newPrincipal.Equal(decimal.NewFromInt(10_120_000)) {
		t.Fatalf("pokok ARO = %s, mau 10120000", newPrincipal)
	}
	if !start.Equal(time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("awal ARO = %s, mau 2026-03-15", start.Format("2006-01-02"))
	}
	if !maturity.Equal(time.Date(2026, time.June, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("jatuh tempo ARO = %s, mau 2026-06-15", maturity.Format("2006-01-02"))
	}
	if !capitalise {
		t.Fatal("PRINCIPAL_AND_PROFIT harus mengapitalisasi imbal hasil")
	}
}

func TestDepositRolloverTermsPokokSaja(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(10_000_000),
		TermMonths:      6,
		MaturityDate:    time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC),
		AROInstruction:  domain.AROInstructionPrincipal,
	}
	newPrincipal, _, maturity, capitalise := depositRolloverTerms(dep, decimal.NewFromInt(120_000))
	if !newPrincipal.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("pokok ARO principal = %s, mau 10000000", newPrincipal)
	}
	if !maturity.Equal(time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("jatuh tempo ARO = %s, mau 2026-09-15", maturity.Format("2006-01-02"))
	}
	if capitalise {
		t.Fatal("PRINCIPAL tidak boleh mengapitalisasi imbal hasil")
	}
}

// ── Penegakan cabang pada Place ─────────────────────────────────────────────

// stubPlaceCustomerRepo hanya melayani GetByID untuk test penegakan cabang;
// method lain dibiarkan dari interface agar tidak dipakai.
type stubPlaceCustomerRepo struct {
	domain.CustomerRepository
	customer *domain.CustomerRecord
}

func (s *stubPlaceCustomerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.CustomerRecord, error) {
	if s.customer == nil {
		return nil, domain.ErrCustomerNotFound
	}
	return s.customer, nil
}

type stubPlaceBranchRepo struct {
	domain.BranchRepository
	branch *domain.Branch
}

func (s *stubPlaceBranchRepo) GetByCode(ctx context.Context, code string) (*domain.Branch, error) {
	return s.branch, nil
}

func (s *stubPlaceBranchRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Branch, error) {
	return s.branch, nil
}

func TestDepositPlaceMenolakNasabahCabangLain(t *testing.T) {
	productID := uuid.New()
	customerID := uuid.New()
	customerBranchID := uuid.New()
	actorBranchID := uuid.New()

	svc := &depositService{
		productRepo: &stubDepositProductRepo{product: &domain.BankingProduct{
			ID:        productID,
			Code:      "DEP-CONV",
			Family:    domain.FamilyTimeDeposit,
			IsActive:  true,
			MinAmount: decimal.NewFromInt(1_000_000),
		}},
		customerRepo: &stubPlaceCustomerRepo{customer: &domain.CustomerRecord{
			ID:       customerID,
			Status:   domain.CustomerStatusActive,
			BranchID: &customerBranchID,
		}},
		branchRepo: &stubPlaceBranchRepo{branch: &domain.Branch{
			ID:       actorBranchID,
			Code:     "001",
			IsActive: true,
		}},
	}

	_, err := svc.Place(context.Background(), domain.PlaceDepositInput{
		CustomerID:      customerID,
		ProductID:       productID,
		PlacementAmount: decimal.NewFromInt(1_000_000),
		TermMonths:      1,
	}, domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("mau ErrCrossBranchAccess, dapat %v", err)
	}
}

// GetByID menerapkan filter cabang pada jalur baca deposito.
func TestDepositGetByID_MenolakCabangLain(t *testing.T) {
	depositID := uuid.New()
	branchID := uuid.New()
	svc := &depositService{depositRepo: &stubDepositRepo{deposit: &domain.Deposit{ID: depositID, BranchID: &branchID, BranchCode: "002"}}}

	t.Run("teller cabang berbeda ditolak", func(t *testing.T) {
		_, err := svc.GetByID(context.Background(), depositID, domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
		if !errors.Is(err, domain.ErrCrossBranchAccess) {
			t.Fatalf("mau ErrCrossBranchAccess, dapat %v", err)
		}
	})

	t.Run("teller cabang sama boleh", func(t *testing.T) {
		dep, err := svc.GetByID(context.Background(), depositID, domain.Actor{Role: domain.RoleTeller, BranchCode: "002"})
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if dep.ID != depositID {
			t.Fatalf("deposit id %s, ingin %s", dep.ID, depositID)
		}
	})

	t.Run("system lintas cabang boleh", func(t *testing.T) {
		if _, err := svc.GetByID(context.Background(), depositID, domain.SystemActor(uuid.New())); err != nil {
			t.Fatalf("batch seharusnya boleh membaca lintas cabang: %v", err)
		}
	})

	t.Run("deposito bercabang NULL tetap terlihat", func(t *testing.T) {
		svcNull := &depositService{depositRepo: &stubDepositRepo{deposit: &domain.Deposit{ID: depositID}}}
		if _, err := svcNull.GetByID(context.Background(), depositID, domain.Actor{Role: domain.RoleTeller, BranchCode: "001"}); err != nil {
			t.Fatalf("data pra-migrasi tanpa cabang tidak boleh ditolak: %v", err)
		}
	})
}

// Deposito yang jumlahnya tidak melebihi ambang dibebaskan dari pemotongan PPh final
// (PP 131/2000 Pasal 3 huruf a). Memotongnya berarti menahan hak nasabah kecil.
func TestDepositDailyAccrualBebasPajakDiBawahAmbang(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(7_500_000),
		ProfitType:      domain.ProfitTypeInterest,
		ProfitRate:      decimal.NewFromInt(4),
		TaxRate:         decimal.NewFromInt(20),
	}
	profit, tax := depositDailyAccrual(dep, defaultDepositTaxExemptAmount)
	if !profit.IsPositive() {
		t.Fatalf("bunga harian harus tetap diakru, dapat %s", profit)
	}
	if !tax.IsZero() {
		t.Fatalf("deposito tepat di ambang tidak boleh dipotong, dapat %s", tax)
	}

	// Satu rupiah di atas ambang kembali dikenakan tarif bruto.
	dep.PlacementAmount = decimal.NewFromInt(7_500_001)
	_, tax = depositDailyAccrual(dep, defaultDepositTaxExemptAmount)
	if !tax.IsPositive() {
		t.Fatal("deposito di atas ambang harus dipotong PPh final")
	}
}

// Pratinjau menampilkan proyeksi dari jalur perhitungan akrual yang sama tanpa
// menyimpan apa pun: nominal, tenor, tanggal jatuh tempo, estimasi bunga & pajak,
// serta nilai jatuh tempo.
func TestDepositPreviewMenghitungProyeksi(t *testing.T) {
	productID := uuid.New()
	customerID := uuid.New()
	product := &domain.BankingProduct{
		ID:         productID,
		Code:       "DEP-CONV",
		Family:     domain.FamilyTimeDeposit,
		Book:       domain.BookConventional,
		IsActive:   true,
		MinAmount:  decimal.NewFromInt(1_000_000),
		RateAnnual: decimal.NewFromInt(12),
		TaxRate:    decimal.NewFromInt(20),
	}
	svc := &depositService{
		productRepo: &stubDepositProductRepo{product: product},
		customerRepo: &stubPlaceCustomerRepo{customer: &domain.CustomerRecord{
			ID:     customerID,
			Status: domain.CustomerStatusActive,
		}},
		configSvc: &stubDepositConfig{values: map[string]decimal.Decimal{}},
	}

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	preview, err := svc.Preview(context.Background(), domain.PlaceDepositInput{
		CustomerID:      customerID,
		ProductID:       productID,
		PlacementAmount: decimal.NewFromInt(12_000_000),
		TermMonths:      3,
		StartDate:       &start,
	}, domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
	if err != nil {
		t.Fatalf("Preview tak terduga: %v", err)
	}
	if got := preview.MaturityDate.Format("2006-01-02"); got != "2026-04-01" {
		t.Fatalf("jatuh tempo = %s, ingin 2026-04-01", got)
	}
	// Bunga harian = 12.000.000 * 12% / 365 = 3.945 (dibulatkan bank).
	// 90 hari tenor: bunga 355.050 dan pajak 71.010; hak nasabah 12.284.040.
	if !preview.EstimatedProfit.Equal(decimal.NewFromInt(355_050)) {
		t.Fatalf("estimasi bunga = %s, ingin 355050", preview.EstimatedProfit)
	}
	if !preview.EstimatedTax.Equal(decimal.NewFromInt(71_010)) {
		t.Fatalf("estimasi pajak = %s, ingin 71010", preview.EstimatedTax)
	}
	if !preview.MaturityProceeds.Equal(decimal.NewFromInt(12_284_040)) {
		t.Fatalf("nilai jatuh tempo = %s, ingin 12284040", preview.MaturityProceeds)
	}
}

// Ambang 0 mematikan pembebasan: seluruh deposito dikenakan tarif produk.
func TestDepositDailyAccrualTanpaAmbangPembebasan(t *testing.T) {
	dep := &domain.Deposit{
		PlacementAmount: decimal.NewFromInt(1_000_000),
		ProfitType:      domain.ProfitTypeInterest,
		ProfitRate:      decimal.NewFromInt(4),
		TaxRate:         decimal.NewFromInt(20),
	}
	_, tax := depositDailyAccrual(dep, decimal.Zero)
	if !tax.IsPositive() {
		t.Fatal("ambang 0 harus tetap memotong pajak")
	}
}
