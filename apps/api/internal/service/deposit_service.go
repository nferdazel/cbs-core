package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ProductEventPoster adalah bagian ProductPoster yang dibutuhkan service deposito.
// Dibuat sebagai interface agar service dapat diuji tanpa mesin posting/database.
type ProductEventPoster interface {
	PostEventTx(ctx context.Context, tx any, product *domain.BankingProduct, event domain.PostingEvent, amounts Amounts, meta PostingMeta) (*domain.JournalEntry, error)
}

var defaultDepositTaxRate = decimal.NewFromInt(20)

// defaultDepositTaxExemptAmount adalah ambang jumlah deposito yang dibebaskan dari
// pemotongan PPh final (PP 131/2000 Pasal 3 huruf a). Nilai 0 berarti tanpa pembebasan.
var defaultDepositTaxExemptAmount = decimal.NewFromInt(7_500_000)

const (
	depositTaxRateConfigKey  = "tax.deposit.rate_pct"
	taxExemptAmountConfigKey = "tax.deposit.exempt_amount"
	mudharabahYieldConfigKey = "deposit.mudharabah.yield_annual_pct"

	// depositPenaltyCOAConfigKey menimpa akun pendapatan denda pencairan.
	// Migrasi 000005 menyediakan 40500 "Pendapatan Denda" hanya untuk buku
	// konvensional (buku syariah tidak punya akun denda baku), jadi fallback
	// konvensional 40500 dan syariah wajib diisi lewat konfigurasi.
	depositPenaltyCOAConfigKey   = "deposit.penalty.income.coa"
	defaultDepositPenaltyCOAConv = "40500"
)

type depositService struct {
	db           *sql.DB
	depositRepo  domain.DepositRepository
	productRepo  domain.ProductRepository
	accountRepo  domain.AccountRepository
	ledgerRepo   domain.LedgerRepository
	customerRepo domain.CustomerRepository
	branchRepo   domain.BranchRepository
	numbering    domain.AccountNumberGenerator
	poster       ProductEventPoster
	posting      domain.PostingService
	resolver     domain.AccountResolver
	configSvc    domain.SystemConfigService
	// limits dan approvals dipakai penjaga batas bersama. Boleh nil (mis. pada
	// test): penjaga dilewati dan penempatan langsung berjalan seperti sebelumnya.
	limits    domain.TransactionLimitService
	approvals domain.MakerCheckerService
	auditRepo domain.AuditRepository
}

// ActionPlaceDeposit adalah jenis aksi maker-checker untuk penempatan deposito yang
// melewati ambang persetujuan. Berbeda dari ActionDeposit (setoran tunai ke rekening
// yang sudah ada) karena eksekusinya membuka kontrak deposito sekaligus jurnalnya.
const ActionPlaceDeposit = "PLACE_DEPOSIT"

func NewDepositService(
	db *sql.DB,
	depositRepo domain.DepositRepository,
	productRepo domain.ProductRepository,
	accountRepo domain.AccountRepository,
	ledgerRepo domain.LedgerRepository,
	customerRepo domain.CustomerRepository,
	branchRepo domain.BranchRepository,
	numbering domain.AccountNumberGenerator,
	poster ProductEventPoster,
	posting domain.PostingService,
	resolver domain.AccountResolver,
	configSvc domain.SystemConfigService,
	limits domain.TransactionLimitService,
	approvals domain.MakerCheckerService,
	auditSinks ...domain.AuditRepository,
) domain.DepositService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &depositService{
		db:           db,
		depositRepo:  depositRepo,
		productRepo:  productRepo,
		accountRepo:  accountRepo,
		ledgerRepo:   ledgerRepo,
		customerRepo: customerRepo,
		branchRepo:   branchRepo,
		numbering:    numbering,
		poster:       poster,
		posting:      posting,
		resolver:     resolver,
		configSvc:    configSvc,
		limits:       limits,
		approvals:    approvals,
		auditRepo:    auditRepo,
	}
}

// depositPlacementPrep menampung hasil validasi dan perhitungan penempatan yang tidak
// menulis apa pun. Dengan memisahkannya, penjaga batas dapat dijalankan SEBELUM
// transaksi dibuka dan eksekutor persetujuan dapat memakai ulang jalur yang sama
// persis saat pengajuan disetujui.
type depositPlacementPrep struct {
	product     *domain.BankingProduct
	customer    *domain.CustomerRecord
	branch      *domain.Branch
	coa         *domain.ChartOfAccount
	start       time.Time
	maturity    time.Time
	now         time.Time
	profitType  domain.ProfitType
	profitRate  decimal.Decimal
	yieldRate   decimal.Decimal
	taxRate     decimal.Decimal
	instruction domain.AROInstruction
}

func normalizePlaceDepositInput(input *domain.PlaceDepositInput) {
	if input.Currency == "" {
		input.Currency = "IDR"
	}
}

func (s *depositService) Place(ctx context.Context, input domain.PlaceDepositInput, actor domain.Actor) (*domain.Deposit, error) {
	normalizePlaceDepositInput(&input)

	prep, err := s.preparePlacement(ctx, input, actor)
	if err != nil {
		return nil, err
	}

	// Penempatan deposito tunduk pada penjaga batas dan alur maker-checker yang SAMA
	// dengan transaksi setoran. Sebelumnya penempatan langsung menulis jurnal tanpa
	// memeriksa limit.<peran>.deposit.approval_above, sehingga nominal 150 juta yang
	// seharusnya butuh persetujuan bisa menempatkan dana lewat jalur deposit.
	payload, err := placeDepositPayload(input, actor)
	if err != nil {
		return nil, err
	}
	if err := guardTransactionLimit(ctx, s.limits, s.approvals, actor, ActionPlaceDeposit, "deposit", input.PlacementAmount, payload); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	deposit, err := s.persistPlacement(ctx, tx, input, actor, prep)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return deposit, nil
}

// preparePlacement memvalidasi produk, nasabah, cabang, akun kewajiban, dan menghitung
// syarat kontrak TANPA menulis. Semua jalur (langsung maupun setelah persetujuan)
// melewatinya, sehingga aturan yang sama berlaku pada keduanya.
func (s *depositService) preparePlacement(ctx context.Context, input domain.PlaceDepositInput, actor domain.Actor) (*depositPlacementPrep, error) {
	if input.PlacementAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidDepositAmount
	}
	if input.TermMonths <= 0 {
		return nil, domain.ErrInvalidDepositTerm
	}

	product, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Family != domain.FamilyTimeDeposit || !product.IsActive {
		return nil, domain.ErrDepositProductInvalid
	}
	// Penempatan mengikuti buku produk: pegawai satu buku tidak boleh menempatkan
	// (atau mempratinjau) deposito produk buku lain. Produk tanpa buku (data lama) lolos.
	if !actor.CanAccessBook(product.Book) {
		return nil, domain.ErrCrossBookAccess
	}
	if input.PlacementAmount.LessThan(product.MinAmount) {
		return nil, fmt.Errorf("%w %s (%s)", domain.ErrProductAmountBelowMin, product.Code, product.MinAmount.String())
	}
	if product.MaxAmount.IsPositive() && input.PlacementAmount.GreaterThan(product.MaxAmount) {
		return nil, fmt.Errorf("%w %s (%s)", domain.ErrProductAmountAboveMax, product.Code, product.MaxAmount.String())
	}
	if input.TermMonths < product.MinTermMonths || (product.MaxTermMonths > 0 && input.TermMonths > product.MaxTermMonths) {
		return nil, fmt.Errorf("%w %s (%d-%d bulan)", domain.ErrProductTermOutOfRange, product.Code, product.MinTermMonths, product.MaxTermMonths)
	}

	customer, err := s.customerRepo.GetByID(ctx, input.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("nasabah tidak valid: %w", err)
	}
	if customer.Status != domain.CustomerStatusActive {
		return nil, domain.ErrAccountInactive
	}

	// Identitas cabang HANYA dari JWT; branch_code pada body request diabaikan. Aktor
	// lintas cabang boleh memakai kode kantor pusat yang tidak terdaftar dan
	// diatribusikan ke kantor pusat, sedangkan peran bercabang biasa tetap wajib punya
	// cabang terdaftar.
	actorBranch, err := resolveActorBranch(ctx, s.branchRepo, actor)
	if err != nil {
		return nil, fmt.Errorf("cabang tidak valid: %w", err)
	}
	// Penegakan kepemilikan cabang: nasabah di luar cakupan unit aktor ditolak
	// (aktor area/wilayah boleh melayani cabang bawahannya). Nasabah tanpa cabang
	// (data pra-migrasi) dibiarkan agar operasional tidak terblokir. Deposito
	// mengikuti cabang nasabah, bukan cabang aktor, bila berbeda.
	branch, err := resolveCustomerBranch(ctx, s.branchRepo, actor, customer.BranchID, actorBranch)
	if err != nil {
		return nil, err
	}
	if branch == nil {
		return nil, fmt.Errorf("cabang tidak valid: %w", domain.ErrBranchNotFound)
	}
	if !branch.IsActive {
		return nil, fmt.Errorf("cabang %s sedang tidak aktif", branch.Code)
	}

	coaCode, err := liabilityCOAForProduct(product)
	if err != nil {
		return nil, err
	}
	coa, err := s.ledgerRepo.GetCOAByCode(ctx, coaCode)
	if err != nil {
		return nil, fmt.Errorf("akun kewajiban deposito %s tidak ditemukan: %w", coaCode, err)
	}

	start := time.Now().UTC()
	if input.StartDate != nil && !input.StartDate.IsZero() {
		start = input.StartDate.UTC()
	}
	start = depositDateOnly(start)
	maturity := domain.DepositMaturityDate(start, input.TermMonths)

	profitType, profitRate, yieldRate, taxRate := depositTerms(ctx, s.configSvc, product, input)

	instruction := input.AROInstruction
	if !input.ARO {
		instruction = domain.AROInstructionNone
	} else if instruction == "" || instruction == domain.AROInstructionNone {
		instruction = domain.AROInstructionPrincipal
	}
	if instruction != domain.AROInstructionPrincipal && instruction != domain.AROInstructionPrincipalAndProfit && instruction != domain.AROInstructionNone {
		return nil, errors.New("instruksi ARO tidak dikenal")
	}

	return &depositPlacementPrep{
		product:     product,
		customer:    customer,
		branch:      branch,
		coa:         coa,
		start:       start,
		maturity:    maturity,
		now:         time.Now().UTC(),
		profitType:  profitType,
		profitRate:  profitRate,
		yieldRate:   yieldRate,
		taxRate:     taxRate,
		instruction: instruction,
	}, nil
}

// persistPlacement menulis rekening, kontrak deposito, jurnal penempatan, dan audit
// memakai transaksi pemanggil. Tidak membuka transaksi sendiri supaya, saat dijalankan
// dari maker-checker, status persetujuan dan efeknya commit bersama.
func (s *depositService) persistPlacement(ctx context.Context, tx *sql.Tx, input domain.PlaceDepositInput, actor domain.Actor, prep *depositPlacementPrep) (*domain.Deposit, error) {
	accountNumber, err := s.numbering.NextAccountNumber(ctx, tx, prep.product, prep.branch.Code)
	if err != nil {
		return nil, err
	}

	// Rekening deposito nasabah. Saldo jurnal kewajiban tetap di akun kontrol GL
	// karena posting memakai pemetaan produk; baris akun ini menautkan bilyet ke
	// nasabah dan menjadi nomor rekening yang sah.
	account := &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    accountNumber,
		CustomerID:       &prep.customer.ID,
		ProductID:        &prep.product.ID,
		BranchID:         &prep.branch.ID,
		COAID:            prep.coa.ID,
		AccountType:      accountTypeForFamily(prep.product.Family),
		Currency:         input.Currency,
		Balance:          decimal.Zero,
		AvailableBalance: decimal.Zero,
		HoldBalance:      decimal.Zero,
		Status:           domain.AccountStatusActive,
		Version:          1,
		OpenedAt:         &prep.now,
		CreatedAt:        prep.now,
		UpdatedAt:        prep.now,
	}
	if err := s.accountRepo.CreateTx(ctx, tx, account); err != nil {
		return nil, fmt.Errorf("membuat rekening deposito: %w", err)
	}

	deposit := &domain.Deposit{
		ID:              uuid.New(),
		AccountNumber:   accountNumber,
		CustomerID:      prep.customer.ID,
		ProductID:       prep.product.ID,
		BranchID:        &prep.branch.ID,
		PlacementAmount: input.PlacementAmount,
		Currency:        input.Currency,
		TermMonths:      input.TermMonths,
		StartDate:       prep.start,
		MaturityDate:    prep.maturity,
		ProfitRate:      prep.profitRate,
		YieldRate:       prep.yieldRate,
		ProfitType:      prep.profitType,
		TaxRate:         prep.taxRate,
		ARO:             input.ARO,
		AROInstruction:  prep.instruction,
		Status:          domain.DepositStatusPlaced,
		CreatedAt:       prep.now,
		UpdatedAt:       prep.now,
	}
	if err := s.depositRepo.Create(ctx, tx, deposit); err != nil {
		return nil, err
	}

	idemKey := input.IdempotencyKey
	if idemKey == "" {
		idemKey = "DEP-PLACE-" + deposit.ID.String()
	}
	_, err = s.poster.PostEventTx(ctx, tx, prep.product, domain.EventDeposit, Amounts{
		Principal: input.PlacementAmount,
		Total:     input.PlacementAmount,
	}, PostingMeta{
		// Penempatan deposito memakai jenis tersendiri (DEPOSIT_PLACEMENT) agar tidak
		// lagi tercampur dengan setoran tunai teller (DEPOSIT) di laporan dan
		// rekonsiliasi; prefix referensinya mengikuti jenis ini (DPL).
		//
		// Jurnal penempatan LAMA sengaja tetap bertipe DEPOSIT dan TIDAK
		// direklasifikasi: baris lama tidak menyimpan penanda pasti untuk
		// membedakan penempatan dari setoran tunai, sehingga menebak berdasarkan
		// deskripsi/prefix justru berisiko salah kelompok pada data produksi.
		TransactionType: domain.TxTypeDepositPlacement,
		Description:     fmt.Sprintf("Penempatan deposito %s %s", prep.product.Name, accountNumber),
		IdempotencyKey:  idemKey,
		CreatedBy:       actor.DisplayName(),
		// Cabang diambil dari cabang yang sudah diselesaikan preparePlacement
		// (resolveActorBranch), bukan dari kode mentah aktor. Aktor lintas cabang
		// berkode 'HO' yang tidak terdaftar sebelumnya menghasilkan jurnal
		// branch_id NULL; kini diatribusikan ke kantor pusat yang sama dengan
		// rekening dan kontrak depositonya.
		BranchCode: prep.branch.Code,
	})
	if err != nil {
		return nil, fmt.Errorf("jurnal penempatan deposito: %w", err)
	}

	if err := writeAudit(ctx, s.auditRepo, tx, actor, "PLACE_DEPOSIT", "deposit", deposit.ID.String(), map[string]any{
		"account_number": accountNumber,
		"product":        prep.product.Code,
		"amount":         input.PlacementAmount.String(),
		"term_months":    input.TermMonths,
	}); err != nil {
		return nil, err
	}
	return deposit, nil
}

// placeDepositPayload mengubah input penempatan menjadi payload persetujuan yang dapat
// dipulihkan utuh saat disetujui, plus identitas pembuat agar jurnal dan audit tetap
// mencatat teller yang mengajukan, bukan pejabat yang menyetujui.
func placeDepositPayload(input domain.PlaceDepositInput, actor domain.Actor) (map[string]any, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("menyiapkan payload persetujuan deposito: %w", err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("menyiapkan payload persetujuan deposito: %w", err)
	}
	payload["maker_id"] = actor.UserID.String()
	payload["maker_username"] = actor.Username
	payload["maker_role"] = string(actor.Role)
	payload["maker_branch"] = actor.BranchCode
	return payload, nil
}

// placeDepositInputFromPayload memulihkan input penempatan dan identitas pembuat dari
// payload persetujuan. Identitas pembuat wajib: tanpa itu jurnal akan mencatat pejabat
// yang menyetujui sebagai pencatat transaksi, menyesatkan penelusuran.
func placeDepositInputFromPayload(payload map[string]any, fallback domain.Actor) (domain.PlaceDepositInput, domain.Actor, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return domain.PlaceDepositInput{}, fallback, fmt.Errorf("membaca payload persetujuan deposito: %w", err)
	}
	var input domain.PlaceDepositInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return domain.PlaceDepositInput{}, fallback, fmt.Errorf("membaca payload persetujuan deposito: %w", err)
	}
	if input.CustomerID == uuid.Nil || input.ProductID == uuid.Nil {
		return domain.PlaceDepositInput{}, fallback, errors.New("payload persetujuan deposito tidak lengkap")
	}
	if !input.PlacementAmount.IsPositive() || input.TermMonths <= 0 {
		return domain.PlaceDepositInput{}, fallback, errors.New("nominal atau tenor pada payload persetujuan deposito tidak valid")
	}

	maker := fallback
	if v, ok := payload["maker_id"].(string); ok {
		if id, err := uuid.Parse(v); err == nil {
			maker.UserID = id
		}
	}
	if v, ok := payload["maker_username"].(string); ok && v != "" {
		maker.Username = v
	}
	if v, ok := payload["maker_role"].(string); ok && v != "" {
		maker.Role = domain.StaffRole(v)
	}
	if v, ok := payload["maker_branch"].(string); ok && v != "" {
		maker.BranchCode = v
	}
	return input, maker, nil
}

// ExecuteApproved menjalankan penempatan deposito yang sudah disetujui, di dalam
// transaksi milik maker-checker service sehingga keputusan dan efeknya commit bersama.
func (s *depositService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	if normalizeAction(actionType) != ActionPlaceDeposit {
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
	input, maker, err := placeDepositInputFromPayload(payload, actor)
	if err != nil {
		return err
	}
	normalizePlaceDepositInput(&input)

	// Persetujuan dijalankan di dalam transaksi milik maker-checker service. Transaksi
	// diambil sebelum evaluasi batas agar evaluasi itu dapat dikunci serial di dalam
	// transaksi yang sama.
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return errors.New("transaksi persetujuan penempatan deposito tidak valid")
	}

	// Batas harian dievaluasi ULANG saat penempatan yang disetujui dieksekusi, sama
	// seperti transaksi rekening lain: persetujuan pejabat tidak boleh melegalkan
	// pelanggaran batas harian. Bila sudah melampaui, eksekusi gagal dan pengajuan
	// tetap PENDING sehingga dapat ditolak/ditinjau.
	if s.limits != nil {
		if err := s.limits.CheckDailyAtExecution(ctx, sqlTx, maker, "deposit", input.PlacementAmount); err != nil {
			return fmt.Errorf("pengajuan yang disetujui tidak dapat dieksekusi: %w", err)
		}
	}

	prep, err := s.preparePlacement(ctx, input, maker)
	if err != nil {
		return err
	}
	_, err = s.persistPlacement(ctx, sqlTx, input, maker, prep)
	return err
}

// Preview menghitung proyeksi deposito tanpa menyimpan apa pun, agar teller dapat
// memeriksa nominal, bunga/bagi hasil, pajak, dan nilai jatuh tempo sebelum menekan
// simpan. Validasi produk dan nasabah disamakan dengan Place supaya kesalahan
// ketahuan lebih awal; cabang tidak diuji ulang karena tidak ada yang ditulis.
//
// Proyeksi memakai akrual harian yang sama dengan Accrue lalu dikalikan jumlah hari
// tenor, sehingga angkanya konsisten dengan posting riil. Tidak ada rumus baru yang
// diduplikasi di frontend.
func (s *depositService) Preview(ctx context.Context, input domain.PlaceDepositInput, actor domain.Actor) (*domain.DepositPreview, error) {
	if input.PlacementAmount.LessThanOrEqual(decimal.Zero) {
		return nil, domain.ErrInvalidDepositAmount
	}
	if input.TermMonths <= 0 {
		return nil, domain.ErrInvalidDepositTerm
	}
	if input.Currency == "" {
		input.Currency = "IDR"
	}

	product, err := s.productRepo.GetByID(ctx, input.ProductID)
	if err != nil {
		return nil, err
	}
	if product.Family != domain.FamilyTimeDeposit || !product.IsActive {
		return nil, domain.ErrDepositProductInvalid
	}
	if input.PlacementAmount.LessThan(product.MinAmount) {
		return nil, fmt.Errorf("%w %s (%s)", domain.ErrProductAmountBelowMin, product.Code, product.MinAmount.String())
	}
	if product.MaxAmount.IsPositive() && input.PlacementAmount.GreaterThan(product.MaxAmount) {
		return nil, fmt.Errorf("%w %s (%s)", domain.ErrProductAmountAboveMax, product.Code, product.MaxAmount.String())
	}
	if input.TermMonths < product.MinTermMonths || (product.MaxTermMonths > 0 && input.TermMonths > product.MaxTermMonths) {
		return nil, fmt.Errorf("%w %s (%d-%d bulan)", domain.ErrProductTermOutOfRange, product.Code, product.MinTermMonths, product.MaxTermMonths)
	}

	customer, err := s.customerRepo.GetByID(ctx, input.CustomerID)
	if err != nil {
		return nil, fmt.Errorf("nasabah tidak valid: %w", err)
	}
	if customer.Status != domain.CustomerStatusActive {
		return nil, domain.ErrAccountInactive
	}

	start := time.Now().UTC()
	if input.StartDate != nil && !input.StartDate.IsZero() {
		start = input.StartDate.UTC()
	}
	start = depositDateOnly(start)
	maturity := domain.DepositMaturityDate(start, input.TermMonths)

	profitType, profitRate, yieldRate, taxRate := depositTerms(ctx, s.configSvc, product, input)

	exempt := defaultDepositTaxExemptAmount
	if s.configSvc != nil {
		exempt = s.configSvc.GetDecimal(ctx, taxExemptAmountConfigKey, defaultDepositTaxExemptAmount)
	}
	probe := &domain.Deposit{
		PlacementAmount: input.PlacementAmount,
		ProfitRate:      profitRate,
		YieldRate:       yieldRate,
		ProfitType:      profitType,
		TaxRate:         taxRate,
	}
	dailyProfit, dailyTax := depositDailyAccrual(probe, exempt)
	days := decimal.NewFromInt(int64(maturity.Sub(start).Hours() / 24))
	estimatedProfit := dailyProfit.Mul(days)
	estimatedTax := dailyTax.Mul(days)

	return &domain.DepositPreview{
		PlacementAmount:  input.PlacementAmount,
		TermMonths:       input.TermMonths,
		StartDate:        start,
		MaturityDate:     maturity,
		ProfitType:       profitType,
		ProfitRate:       profitRate,
		YieldRate:        yieldRate,
		TaxRate:          taxRate,
		EstimatedProfit:  estimatedProfit,
		EstimatedTax:     estimatedTax,
		MaturityProceeds: domain.DepositMaturityProceeds(input.PlacementAmount, estimatedProfit, estimatedTax),
	}, nil
}

// depositTerms menentukan jenis imbal hasil, tarif/nisbah, proyeksi yield syariah,
// dan tarif pajak. Pajak hanya berlaku untuk buku konvensional dan diambil dari
// produk bila diisi, jika tidak dari konfigurasi database (default 20%).
func depositTerms(ctx context.Context, cfg domain.SystemConfigService, product *domain.BankingProduct, input domain.PlaceDepositInput) (domain.ProfitType, decimal.Decimal, decimal.Decimal, decimal.Decimal) {
	profitType := domain.ProfitTypeInterest
	switch product.ProfitScheme {
	case domain.SchemeMurabahah:
		profitType = domain.ProfitTypeMargin
	case domain.SchemeMudharabah, domain.SchemeMusyarakah:
		profitType = domain.ProfitTypeBagiHasil
	}

	profitRate := product.RateAnnual
	yieldRate := decimal.Zero
	if profitType == domain.ProfitTypeBagiHasil {
		// Nisbah adalah porsi pemilik dana; imbal hasil dihitung dari yield proyeksi.
		profitRate = product.ProfitSharingRatio
		if cfg != nil {
			yieldRate = cfg.GetDecimal(ctx, mudharabahYieldConfigKey, decimal.Zero)
		}
	}
	if input.ProfitRate.IsPositive() {
		profitRate = input.ProfitRate
	}
	if input.YieldRate.IsPositive() {
		yieldRate = input.YieldRate
	}

	taxRate := decimal.Zero
	if product.Book != domain.BookSyariah {
		taxRate = product.TaxRate
		if !taxRate.IsPositive() {
			taxRate = defaultDepositTaxRate
			if cfg != nil {
				taxRate = cfg.GetDecimal(ctx, depositTaxRateConfigKey, defaultDepositTaxRate)
			}
		}
	}
	return profitType, profitRate, yieldRate, taxRate
}

// depositDailyAccrual adalah perhitungan murni akrual satu hari. Dipisah agar
// dapat diuji tanpa database dan dipakai konsisten oleh Accrue.
// depositDailyAccrual menghitung imbal hasil dan PPh final harian deposito.
//
// Ambang pembebasan PPh dibandingkan dengan JUMLAH DEPOSITONYA, bukan dengan bunga:
// PP 131/2000 Pasal 3 huruf a membebaskan pemotongan sepanjang "jumlah deposito dan
// tabungan ... tidak melebihi Rp 7.500.000", dan Pasal 2 hanya mengenakan tarif 20%
// bila jumlah deposito MELAMPAUI angka itu. Deposito kecil karena itu tidak boleh
// dipotong sama sekali; memotongnya berarti menahan hak nasabah dan menambah utang
// pajak yang tidak seharusnya ada.
func depositDailyAccrual(d *domain.Deposit, taxExemptAmount decimal.Decimal) (profit, tax decimal.Decimal) {
	profit = domain.DepositDailyProfit(d.PlacementAmount, d.ProfitRate, d.YieldRate, domain.IsBagiHasilProfit(d.ProfitType))
	if taxExemptAmount.IsPositive() && d.PlacementAmount.LessThanOrEqual(taxExemptAmount) {
		return profit, decimal.Zero
	}
	tax = domain.DepositDailyTax(profit, d.TaxRate)
	return profit, tax
}

// depositPayoutAmounts menghitung hak bersih nasabah saat pencairan.
func depositPayoutAmounts(d *domain.Deposit) (netProfit, tax, proceeds decimal.Decimal) {
	tax = d.AccruedTax
	netProfit = d.AccruedProfit.Sub(d.AccruedTax)
	if netProfit.IsNegative() {
		netProfit = decimal.Zero
	}
	proceeds = domain.DepositMaturityProceeds(d.PlacementAmount, d.AccruedProfit, d.AccruedTax)
	return netProfit, tax, proceeds
}

// depositIsEarly melaporkan apakah asOf masih sebelum tanggal jatuh tempo.
func depositIsEarly(asOf, maturity time.Time) bool {
	return depositDateOnly(asOf).Before(depositDateOnly(maturity))
}

// depositWithdrawalAmounts menerapkan denda pada hak pencairan. Proceeds tidak
// pernah negatif: bila denda melebihi pokok + imbal hasil bersih, kembalikan
// ErrDepositPenaltyExceedsProceeds. appliedPenalty di-nol-kan saat tidak valid.
func depositWithdrawalAmounts(d *domain.Deposit, penalty decimal.Decimal) (netProfit, tax, appliedPenalty, proceeds decimal.Decimal, err error) {
	netProfit, tax, proceeds = depositPayoutAmounts(d)
	appliedPenalty = penalty
	if appliedPenalty.IsNegative() {
		appliedPenalty = decimal.Zero
	}
	if appliedPenalty.GreaterThan(proceeds) {
		return netProfit, tax, decimal.Zero, decimal.Zero, domain.ErrDepositPenaltyExceedsProceeds
	}
	proceeds = proceeds.Sub(appliedPenalty)
	return netProfit, tax, appliedPenalty, proceeds, nil
}

// depositPenaltySplit membagi denda: sebesar mungkin diserap porsi imbal hasil
// bersih lebih dulu, sisanya mengurangi pokok. Ini menjaga setiap jurnal tetap
// seimbang tanpa nilai kas negatif.
func depositPenaltySplit(netProfit, penalty decimal.Decimal) (fromProfit, fromPrincipal decimal.Decimal) {
	if penalty.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero
	}
	if penalty.LessThanOrEqual(netProfit) {
		return penalty, decimal.Zero
	}
	return netProfit, penalty.Sub(netProfit)
}

// depositRolloverTerms menghitung syarat kontrak ARO berikutnya tanpa menyentuh
// database: pokok, awal baru (tanggal jatuh tempo lama), jatuh tempo berikutnya,
// dan apakah imbal hasil dikapitalisasi ke pokok.
func depositRolloverTerms(d *domain.Deposit, netProfit decimal.Decimal) (newPrincipal decimal.Decimal, newStart, newMaturity time.Time, capitalise bool) {
	newStart = depositDateOnly(d.MaturityDate)
	newMaturity = domain.DepositMaturityDate(newStart, d.TermMonths)
	newPrincipal = d.PlacementAmount
	if d.AROInstruction == domain.AROInstructionPrincipalAndProfit && netProfit.IsPositive() {
		newPrincipal = d.PlacementAmount.Add(netProfit)
		capitalise = true
	}
	return newPrincipal, newStart, newMaturity, capitalise
}

func (s *depositService) Accrue(ctx context.Context, depositID uuid.UUID, asOf time.Time, actor domain.Actor) (*domain.Deposit, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	dep, err := s.depositRepo.GetByIDForUpdate(ctx, tx, depositID)
	if err != nil {
		return nil, err
	}
	if dep.Status != domain.DepositStatusPlaced && dep.Status != domain.DepositStatusMatured {
		return nil, domain.ErrDepositNotActive
	}
	// Penegakan kepemilikan cabang. Deposito tanpa cabang (data pra-migrasi)
	// dibiarkan lewat; batch memakai RoleSystem yang lintas cabang.
	if !actor.CanAccessBranch(dep.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Buku deposito ikut ditegakkan: akrual buku lain tidak boleh dipicu pegawai satu
	// buku. Batch memakai RoleSystem (lintas buku) sehingga jurnal tetap berjalan.
	if !actor.CanAccessBook(s.depositBook(ctx, dep)) {
		return nil, domain.ErrCrossBookAccess
	}

	accrualDate := depositDateOnly(asOf)
	// Idempotent per hari: hari yang sudah diakrual tidak diposting ulang.
	if dep.LastAccrualDate != nil && !accrualDate.After(depositDateOnly(*dep.LastAccrualDate)) {
		return dep, nil
	}
	if accrualDate.Before(depositDateOnly(dep.StartDate)) {
		return dep, nil
	}

	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil {
		return nil, err
	}

	profit, tax := depositDailyAccrual(dep,
		s.configSvc.GetDecimal(ctx, taxExemptAmountConfigKey, defaultDepositTaxExemptAmount))
	if profit.IsPositive() {
		_, err = s.poster.PostEventTx(ctx, tx, product, domain.EventInterestAccrual, Amounts{
			Profit: profit,
			Total:  profit,
		}, PostingMeta{
			TransactionType: domain.TxTypeInterestAccrual,
			Description:     fmt.Sprintf("Akrual imbal hasil deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-ACCR-" + dep.ID.String() + "-" + accrualDate.Format("20060102"),
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		})
		if err != nil {
			return nil, fmt.Errorf("jurnal akrual deposito: %w", err)
		}
	}

	if err := s.depositRepo.AddAccrual(ctx, tx, dep.ID, profit, tax, accrualDate); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	dep.AccruedProfit = dep.AccruedProfit.Add(profit)
	dep.AccruedTax = dep.AccruedTax.Add(tax)
	dep.LastAccrualDate = &accrualDate
	return dep, nil
}

func (s *depositService) MatureOrWithdraw(ctx context.Context, depositID uuid.UUID, actor domain.Actor) (*domain.Deposit, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	dep, err := s.depositRepo.GetByIDForUpdate(ctx, tx, depositID)
	if err != nil {
		return nil, err
	}
	if dep.Status == domain.DepositStatusClosed || dep.Status == domain.DepositStatusBroken {
		return nil, domain.ErrDepositAlreadyClosed
	}
	// Penegakan kepemilikan cabang. Deposito tanpa cabang (data pra-migrasi)
	// dibiarkan lewat agar pencairan data lama tidak terblokir.
	if !actor.CanAccessBranch(dep.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Buku deposito ikut ditegakkan: pencairan/pematangan deposito buku lain ditolak.
	if !actor.CanAccessBook(s.depositBook(ctx, dep)) {
		return nil, domain.ErrCrossBookAccess
	}

	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	// Sebelum jatuh tempo kontrak dicairkan lebih awal (BROKEN) dan dikenai denda
	// sesuai tarif produk. Denda hanya memotong hasil pencairan, tidak mengubah
	// akrual imbal hasil yang sudah diakui.
	early := depositIsEarly(now, dep.MaturityDate)
	penalty := decimal.Zero
	if early {
		penalty = domain.DepositEarlyWithdrawalPenalty(dep.PlacementAmount, product.EarlyWithdrawalPenaltyRate)
	}
	netProfit, tax, penalty, proceeds, err := depositWithdrawalAmounts(dep, penalty)
	if err != nil {
		return nil, err
	}
	fromProfit, fromPrincipal := depositPenaltySplit(netProfit, penalty)

	status := domain.DepositStatusClosed
	if early {
		status = domain.DepositStatusBroken
	}

	// 1. Potong PPh final: kurangi utang bunga, akui utang pajak (20500).
	if tax.IsPositive() {
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventTaxWithholding, Amounts{
			Tax: tax, Total: tax,
		}, PostingMeta{
			TransactionType: domain.TxTypeWithdrawal,
			Description:     fmt.Sprintf("Potongan PPh final deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-TAX",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}); err != nil {
			return nil, fmt.Errorf("jurnal potongan pajak deposito: %w", err)
		}
	}

	// 2. Akui pendapatan denda: kurangi kewajiban imbal hasil/pokok sebesar denda
	// yang tidak dibayarkan ke nasabah.
	if penalty.IsPositive() {
		if err := s.postDepositPenalty(ctx, tx, product, dep, actor, fromProfit, fromPrincipal, penalty); err != nil {
			return nil, err
		}
	}

	// 3. Bayar imbal hasil bersih setelah denda porsi bunga.
	if interestPaid := netProfit.Sub(fromProfit); interestPaid.IsPositive() {
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventInterestPayment, Amounts{
			Profit: interestPaid, Total: interestPaid,
		}, PostingMeta{
			TransactionType: domain.TxTypeWithdrawal,
			Description:     fmt.Sprintf("Pembayaran imbal hasil deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-INT",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}); err != nil {
			return nil, fmt.Errorf("jurnal pembayaran imbal hasil deposito: %w", err)
		}
	}

	// 4. Bayar pokok setelah denda porsi pokok (bila denda melebihi bunga bersih).
	if principalPaid := dep.PlacementAmount.Sub(fromPrincipal); principalPaid.IsPositive() {
		if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventWithdrawal, Amounts{
			Principal: principalPaid, Total: principalPaid,
		}, PostingMeta{
			TransactionType: domain.TxTypeWithdrawal,
			Description:     fmt.Sprintf("Pencairan pokok deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-PRIN",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
		}); err != nil {
			return nil, fmt.Errorf("jurnal pencairan pokok deposito: %w", err)
		}
	}

	if err := s.depositRepo.UpdateStatus(ctx, tx, dep.ID, status, proceeds, netProfit, tax, penalty); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "WITHDRAW_DEPOSIT", "deposit", dep.ID.String(), map[string]any{
		"status":   status,
		"proceeds": proceeds.String(),
		"tax":      tax.String(),
		"penalty":  penalty.String(),
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	dep.Status = status
	dep.MaturityProceeds = proceeds
	dep.PaidProfit = netProfit
	dep.PaidTax = tax
	dep.EarlyWithdrawalPenalty = penalty
	dep.ClosedAt = &now
	return dep, nil
}

func (s *depositService) RunARO(ctx context.Context, asOf time.Time, actor domain.Actor) (int, error) {
	// Daftar sudah dibatasi cabang dan buku aktor di query, jadi aktor satu buku tidak
	// pernah menerima deposito buku lain untuk diproses; aktor lintas buku (SYSTEM
	// saat EOD) menerima seluruh bank.
	matured, err := s.depositRepo.ListMaturedARO(ctx, asOf, actor)
	if err != nil {
		return 0, err
	}
	processed := 0
	for i := range matured {
		if err := s.rolloverOne(ctx, matured[i].ID, actor); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

// rolloverOne memperpanjang satu kontrak deposito ber-ARO. Tidak ada dana keluar:
// kewajiban pokok tetap di GL. Bila imbal hasil dikapitalisasi, saldo kewajiban
// imbal hasil direklas ke kewajiban pokok lewat posting engine.
func (s *depositService) rolloverOne(ctx context.Context, depositID uuid.UUID, actor domain.Actor) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	dep, err := s.depositRepo.GetByIDForUpdate(ctx, tx, depositID)
	if err != nil {
		return err
	}
	if !dep.ARO || dep.Status == domain.DepositStatusClosed || dep.Status == domain.DepositStatusBroken {
		return nil
	}

	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil {
		return err
	}
	// Pertahanan berlapis: RunARO sudah menyaring buku di query, tetapi rolloverOne
	// juga tidak boleh menulis jurnal atas deposito buku lain bila dipanggil dari jalur
	// lain. Buku dibaca dari produk yang sudah dimuat, tanpa kueri tambahan.
	if !actor.CanAccessBook(product.Book) {
		return domain.ErrCrossBookAccess
	}

	netProfit, tax, _ := depositPayoutAmounts(dep)
	newPrincipal, start, maturity, capitalise := depositRolloverTerms(dep, netProfit)
	var paidProfit, paidTax decimal.Decimal

	if capitalise {
		if tax.IsPositive() {
			if _, err := s.poster.PostEventTx(ctx, tx, product, domain.EventTaxWithholding, Amounts{
				Tax: tax, Total: tax,
			}, PostingMeta{
				TransactionType: domain.TxTypeAdjustment,
				Description:     fmt.Sprintf("Potongan PPh final ARO deposito %s", dep.AccountNumber),
				IdempotencyKey:  "DEP-ARO-" + dep.ID.String() + "-" + dep.MaturityDate.Format("20060102") + "-TAX",
				CreatedBy:       actor.DisplayName(),
				BranchCode:      actor.BranchCode,
			}); err != nil {
				return fmt.Errorf("jurnal potongan pajak ARO deposito: %w", err)
			}
		}

		payableAccount, err := s.mappingAccount(ctx, tx, product.ID, domain.EventInterestAccrual, domain.DirectionCredit)
		if err != nil {
			return err
		}
		liabilityAccount, err := s.mappingAccount(ctx, tx, product.ID, domain.EventDeposit, domain.DirectionCredit)
		if err != nil {
			return err
		}
		if _, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
			TransactionType: domain.TxTypeAdjustment,
			Description:     fmt.Sprintf("Kapitalisasi imbal hasil ARO deposito %s", dep.AccountNumber),
			IdempotencyKey:  "DEP-ARO-" + dep.ID.String() + "-" + dep.MaturityDate.Format("20060102") + "-CAP",
			CreatedBy:       actor.DisplayName(),
			BranchCode:      actor.BranchCode,
			Lines: []domain.PostingLine{
				{AccountNumber: payableAccount, Direction: domain.DirectionDebit, Amount: netProfit, Description: "Reklas imbal hasil ke pokok"},
				{AccountNumber: liabilityAccount, Direction: domain.DirectionCredit, Amount: netProfit, Description: "Perpanjangan pokok ARO"},
			},
		}); err != nil {
			return fmt.Errorf("jurnal kapitalisasi ARO deposito: %w", err)
		}

		paidProfit = netProfit
		paidTax = tax
	}

	if err := s.depositRepo.Rollover(ctx, tx, dep.ID, newPrincipal, paidProfit, paidTax, start, maturity, capitalise); err != nil {
		return err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "ROLLOVER_DEPOSIT", "deposit", dep.ID.String(), map[string]any{
		"new_principal": newPrincipal.String(),
		"new_maturity":  maturity.Format("2006-01-02"),
		"capitalised":   capitalise,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// mappingAccount mengambil nomor akun GL untuk sebuah peristiwa produk pada arah
// tertentu. Dipakai kapitalisasi ARO agar tidak menebak kode COA di service.
func (s *depositService) mappingAccount(ctx context.Context, tx any, productID uuid.UUID, event domain.PostingEvent, dir domain.EntryDirection) (string, error) {
	rules, err := s.productRepo.GetMapping(ctx, productID, event)
	if err != nil {
		return "", err
	}
	for _, rule := range rules {
		if rule.Direction == dir {
			return s.resolver.ResolveGLAccount(ctx, tx, rule.COACode)
		}
	}
	return "", fmt.Errorf("produk tidak punya pemetaan %s arah %s", event, dir)
}

// depositPenaltyCOA menentukan kode COA pendapatan denda. Urutan: override
// system_config deposit.penalty.income.coa, lalu fallback 40500 "Pendapatan
// Denda" (buku konvensional, migrasi 000005). Buku syariah tidak punya akun
// denda baku, sehingga mengembalikan string kosong bila tidak dikonfigurasi.
func (s *depositService) depositPenaltyCOA(ctx context.Context, product *domain.BankingProduct) string {
	fallback := ""
	if product.Book == domain.BookConventional {
		fallback = defaultDepositPenaltyCOAConv
	}
	if s.configSvc == nil {
		return fallback
	}
	code := strings.TrimSpace(s.configSvc.GetString(ctx, depositPenaltyCOAConfigKey, fallback))
	if code == "" {
		code = fallback
	}
	return code
}

// postDepositPenalty menulis jurnal pendapatan denda. Debit mengikuti dari sisi
// mana denda diserap (kewajiban imbal hasil dan/atau pokok), kredit ke akun
// pendapatan denda. Idempotency key tetap per kontrak agar pencairan ganda tidak
// memposting denda dua kali.
func (s *depositService) postDepositPenalty(
	ctx context.Context,
	tx any,
	product *domain.BankingProduct,
	dep *domain.Deposit,
	actor domain.Actor,
	fromProfit, fromPrincipal, penalty decimal.Decimal,
) error {
	coa := s.depositPenaltyCOA(ctx, product)
	if coa == "" {
		return fmt.Errorf("%w: atur %s untuk buku %s", domain.ErrDepositPenaltyCOAUnavailable, depositPenaltyCOAConfigKey, product.Book)
	}
	penaltyAccount, err := s.resolver.ResolveGLAccount(ctx, tx, coa)
	if err != nil {
		return fmt.Errorf("akun pendapatan denda %s: %w", coa, err)
	}

	lines := make([]domain.PostingLine, 0, 3)
	if fromProfit.IsPositive() {
		account, err := s.mappingAccount(ctx, tx, product.ID, domain.EventInterestPayment, domain.DirectionDebit)
		if err != nil {
			return err
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: account, Direction: domain.DirectionDebit, Amount: fromProfit,
			Description: "Denda pencairan deposito - imbal hasil",
		})
	}
	if fromPrincipal.IsPositive() {
		account, err := s.mappingAccount(ctx, tx, product.ID, domain.EventWithdrawal, domain.DirectionDebit)
		if err != nil {
			return err
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: account, Direction: domain.DirectionDebit, Amount: fromPrincipal,
			Description: "Denda pencairan deposito - pokok",
		})
	}
	lines = append(lines, domain.PostingLine{
		AccountNumber: penaltyAccount, Direction: domain.DirectionCredit, Amount: penalty,
		Description: "Pendapatan denda pencairan deposito",
	})

	if _, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     fmt.Sprintf("Denda pencairan lebih awal deposito %s", dep.AccountNumber),
		IdempotencyKey:  "DEP-MAT-" + dep.ID.String() + "-PEN",
		CreatedBy:       actor.DisplayName(),
		BranchCode:      actor.BranchCode,
		Lines:           lines,
	}); err != nil {
		return fmt.Errorf("jurnal denda pencairan deposito: %w", err)
	}
	return nil
}

func (s *depositService) GetByID(ctx context.Context, id uuid.UUID, actor domain.Actor) (*domain.Deposit, error) {
	dep, err := s.depositRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	// Deposito cabang lain ditolak. Pembacaan ini disamarkan menjadi 404 oleh
	// pemanggil (lihat Fail): status 404 dipertahankan agar keberadaan data tidak
	// bocor, sedangkan operasi tulis lintas cabang tetap dibalas 403.
	// Deposito tanpa cabang (data pra-migrasi) tetap boleh dibaca.
	if !actor.CanAccessBranch(dep.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Deposito buku lain juga ditolak; buku dibaca dari produk deposito.
	if !actor.CanAccessBook(s.depositBook(ctx, dep)) {
		return nil, domain.ErrCrossBookAccess
	}
	return dep, nil
}

// depositBook mengembalikan buku produk deposito. Deposito tanpa produk (data lama)
// atau produk yang tidak terbaca mengembalikan buku kosong, yang oleh CanAccessBook
// diizinkan agar data lama tidak hilang dari operasional.
func (s *depositService) depositBook(ctx context.Context, dep *domain.Deposit) domain.COABook {
	if dep.ProductID == uuid.Nil {
		return ""
	}
	product, err := s.productRepo.GetByID(ctx, dep.ProductID)
	if err != nil || product == nil {
		return ""
	}
	return product.Book
}

func (s *depositService) List(ctx context.Context, page, pageSize int, actor domain.Actor) ([]domain.Deposit, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return s.depositRepo.List(ctx, pageSize, (page-1)*pageSize, actor)
}

// dateOnly menormalkan waktu ke tanggal UTC tanpa jam, agar akrual dan jatuh
// tempo dibandingkan per hari.
func depositDateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

var _ domain.DepositService = (*depositService)(nil)
