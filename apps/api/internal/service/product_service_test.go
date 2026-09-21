package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubProductParamRepo meniru repositori produk untuk jalur tulis parameter.
// Perubahan disimpan ke peta yang sama agar test dapat memeriksa nilai tersimpan.
type stubProductParamRepo struct {
	products map[string]*domain.BankingProduct
	// locked, bila tidak nil, adalah nilai yang dibaca GetByCodeTx. Dipakai untuk
	// meniru baris yang sudah berubah oleh permintaan lain sebelum lock didapat.
	locked    map[string]*domain.BankingProduct
	lastTx    any
	readTx    any
	updateErr error
}

func (s *stubProductParamRepo) List(context.Context) ([]domain.BankingProduct, error) {
	return nil, nil
}

func (s *stubProductParamRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.BankingProduct, error) {
	for _, p := range s.products {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, domain.ErrProductNotFound
}

func (s *stubProductParamRepo) GetByCode(_ context.Context, code string) (*domain.BankingProduct, error) {
	if p, ok := s.products[code]; ok {
		return p, nil
	}
	return nil, domain.ErrProductNotFound
}

func (s *stubProductParamRepo) GetMapping(context.Context, uuid.UUID, domain.PostingEvent) ([]domain.JournalMappingRule, error) {
	return nil, nil
}

func (s *stubProductParamRepo) GetByCodeTx(_ context.Context, tx any, code string) (*domain.BankingProduct, error) {
	s.readTx = tx
	if s.locked != nil {
		if p, ok := s.locked[code]; ok {
			return p, nil
		}
	}
	return s.GetByCode(context.Background(), code)
}

func (s *stubProductParamRepo) UpdateParamsTx(_ context.Context, tx any, p *domain.BankingProduct) error {
	s.lastTx = tx
	if s.updateErr != nil {
		return s.updateErr
	}
	s.products[p.Code] = p
	return nil
}

var _ domain.ProductParamRepository = (*stubProductParamRepo)(nil)

func productParamFixture() (*productService, *stubProductParamRepo, *stubAuditRepo) {
	repo := &stubProductParamRepo{products: map[string]*domain.BankingProduct{
		"PMB-MUDHARABAH": {
			ID:                         uuid.New(),
			Code:                       "PMB-MUDHARABAH",
			Name:                       "Pembiayaan Mudharabah",
			Family:                     domain.FamilyLoan,
			Book:                       domain.BookSyariah,
			ProfitScheme:               domain.SchemeMudharabah,
			ScheduleMethod:             domain.ScheduleBagiHasil,
			ProfitSharingRatio:         decimal.RequireFromString("0.4"),
			MinAmount:                  decimal.NewFromInt(1_000_000),
			MinTermMonths:              1,
			MaxTermMonths:              120,
			ProjectedRevenueRateAnnual: decimal.Zero,
		},
	}}
	audit := &stubAuditRepo{}
	return &productService{repo: repo, auditRepo: audit, txRunner: stubTxRunner{}}, repo, audit
}

func intPtr(v int) *int { return &v }

func decPtr(s string) *decimal.Decimal {
	d := decimal.RequireFromString(s)
	return &d
}

// Setiap parameter yang diizinkan harus benar-benar tersimpan, dan jejak auditnya
// memuat nilai sebelum -> sesudah untuk bidang yang berubah.
func TestProductServiceUpdateParams_SetiapBidangParameter(t *testing.T) {
	svc, repo, audit := productParamFixture()
	actor := domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}
	code := "PMB-MUDHARABAH"

	product, err := svc.UpdateParams(context.Background(), code, domain.UpdateProductParamsInput{
		RateAnnual:                 decPtr("14.5"),
		ProfitSharingRatio:         decPtr("0.55"),
		ProjectedRevenueRateAnnual: decPtr("11"),
		MinAmount:                  decPtr("2000000"),
		MaxAmount:                  decPtr("50000000"),
		MinTermMonths:              intPtr(3),
		MaxTermMonths:              intPtr(60),
		AdminFee:                   decPtr("25000"),
		TaxRate:                    decPtr("15"),
		EarlyWithdrawalPenaltyRate: decPtr("1.5"),
	}, actor)
	if err != nil {
		t.Fatalf("UpdateParams: %v", err)
	}

	stored := repo.products[code]
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"rate_annual", product.RateAnnual.String(), "14.5"},
		{"profit_sharing_ratio", product.ProfitSharingRatio.String(), "0.55"},
		{"projected_revenue_rate_annual", product.ProjectedRevenueRateAnnual.String(), "11"},
		{"min_amount", product.MinAmount.String(), "2000000"},
		{"max_amount", product.MaxAmount.String(), "50000000"},
		{"admin_fee", product.AdminFee.String(), "25000"},
		{"tax_rate", product.TaxRate.String(), "15"},
		{"early_withdrawal_penalty_rate", product.EarlyWithdrawalPenaltyRate.String(), "1.5"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Fatalf("%s = %s, ingin %s", c.name, c.got, c.want)
		}
	}
	if product.MinTermMonths != 3 || product.MaxTermMonths != 60 {
		t.Fatalf("tenor = %d..%d, ingin 3..60", product.MinTermMonths, product.MaxTermMonths)
	}
	if stored.RateAnnual.String() != "14.5" {
		t.Fatalf("nilai tersimpan = %s, ingin 14.5", stored.RateAnnual)
	}

	if len(audit.events) != 1 {
		t.Fatalf("audit = %+v, ingin satu event", audit.events)
	}
	event := audit.events[0]
	if event.Action != "UPDATE_PRODUCT_PARAMS" || event.ResourceType != "product" {
		t.Fatalf("audit = %+v, ingin aksi UPDATE_PRODUCT_PARAMS pada resource product", event)
	}
	if event.ResourceID != product.ID.String() {
		t.Fatalf("resource id audit = %s, ingin %s", event.ResourceID, product.ID)
	}
	if event.Changes["code"] != code {
		t.Fatalf("audit code = %v, ingin %s", event.Changes["code"], code)
	}
	changedFields := []string{
		"rate_annual", "profit_sharing_ratio", "projected_revenue_rate_annual",
		"min_amount", "max_amount", "min_term_months", "max_term_months",
		"admin_fee", "tax_rate", "early_withdrawal_penalty_rate",
	}
	for _, field := range changedFields {
		pair, ok := event.Changes[field].(map[string]any)
		if !ok {
			t.Fatalf("audit %s = %#v, ingin pasangan before/after", field, event.Changes[field])
		}
		if _, ok := pair["before"]; !ok {
			t.Fatalf("audit %s tidak memuat before", field)
		}
		if _, ok := pair["after"]; !ok {
			t.Fatalf("audit %s tidak memuat after", field)
		}
	}
	// Nilai sebelum untuk nisbah harus nilai lama, bukan nilai baru.
	if pair := event.Changes["profit_sharing_ratio"].(map[string]any); pair["before"].(decimal.Decimal).String() != "0.4" {
		t.Fatalf("before nisbah = %v, ingin 0.4", pair["before"])
	}
}

// Nisbah 40 hampir pasti maksudnya 40% (0,4). Nilai ini harus ditolak dengan pesan
// yang menyebut satuannya (pecahan), bukan disimpan dan melipatgandakan imbal hasil.
func TestProductServiceUpdateParams_TolakNisbahSalahSatuan(t *testing.T) {
	svc, repo, audit := productParamFixture()

	_, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{ProfitSharingRatio: decPtr("40")},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin})
	if !errors.Is(err, domain.ErrBagiHasilNisbahOutOfRange) {
		t.Fatalf("err = %v, ingin ErrBagiHasilNisbahOutOfRange", err)
	}
	if !strings.Contains(err.Error(), "pecahan") || !strings.Contains(err.Error(), "0,4 untuk 40%") {
		t.Fatalf("pesan = %q, ingin menyebut satuan pecahan/40%%", err.Error())
	}
	if repo.products["PMB-MUDHARABAH"].ProfitSharingRatio.String() != "0.4" {
		t.Fatal("nisbah tidak boleh berubah saat validasi gagal")
	}
	if len(audit.events) != 0 {
		t.Fatalf("audit ditulis untuk perubahan yang ditolak: %+v", audit.events)
	}
}

// Proyeksi 1200 hampir pasti maksudnya 12%. Nilai di luar 0..100 harus ditolak
// dengan pesan yang menyebut persen per tahun; batas atas tepat 100 tetap diterima.
func TestProductServiceUpdateParams_TolakProyeksiSalahSatuan(t *testing.T) {
	svc, _, _ := productParamFixture()

	_, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{ProjectedRevenueRateAnnual: decPtr("1200")},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin})
	if !errors.Is(err, domain.ErrBagiHasilProjectionOutOfRange) {
		t.Fatalf("err = %v, ingin ErrBagiHasilProjectionOutOfRange", err)
	}
	if !strings.Contains(err.Error(), "persen per tahun") || !strings.Contains(err.Error(), "12 untuk 12%") {
		t.Fatalf("pesan = %q, ingin menyebut persen per tahun", err.Error())
	}

	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{ProjectedRevenueRateAnnual: decPtr("100")},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}); err != nil {
		t.Fatalf("proyeksi tepat 100 harus diterima: %v", err)
	}
}

// Bidang numerik tidak boleh negatif; setiap bidang mengembalikan pesan bersatuan.
func TestProductServiceUpdateParams_TolakNilaiNegatif(t *testing.T) {
	svc, _, _ := productParamFixture()
	actor := domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}

	cases := []struct {
		name  string
		input domain.UpdateProductParamsInput
		want  error
		unit  string
	}{
		{"rate", domain.UpdateProductParamsInput{RateAnnual: decPtr("-1")}, domain.ErrProductRateNegative, "persen"},
		{"admin_fee", domain.UpdateProductParamsInput{AdminFee: decPtr("-1")}, domain.ErrProductAdminFeeNegative, "rupiah"},
		{"tax", domain.UpdateProductParamsInput{TaxRate: decPtr("-0.5")}, domain.ErrProductTaxRateNegative, "persen"},
		{"penalty", domain.UpdateProductParamsInput{EarlyWithdrawalPenaltyRate: decPtr("-1")}, domain.ErrProductPenaltyRateNegative, "persen"},
		{"min_amount", domain.UpdateProductParamsInput{MinAmount: decPtr("-1")}, domain.ErrProductMinAmountNegative, "rupiah"},
		{"max_amount", domain.UpdateProductParamsInput{MaxAmount: decPtr("-1")}, domain.ErrProductMaxAmountNegative, "rupiah"},
		{"min_term", domain.UpdateProductParamsInput{MinTermMonths: intPtr(-1)}, domain.ErrProductTermNegative, "bulan"},
		{"max_term", domain.UpdateProductParamsInput{MaxTermMonths: intPtr(-1)}, domain.ErrProductTermNegative, "bulan"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH", c.input, actor)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, ingin %v", err, c.want)
			}
			if !strings.Contains(err.Error(), c.unit) {
				t.Fatalf("pesan = %q, ingin menyebut satuan %q", err.Error(), c.unit)
			}
		})
	}
}

// Hubungan antar-bidang: plafon maksimum 0 berarti tanpa batas, selain itu harus
// >= minimum; tenor minimum tidak boleh melebihi maksimum.
func TestProductServiceUpdateParams_TolakRentangAntarBidang(t *testing.T) {
	svc, _, _ := productParamFixture()
	actor := domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}

	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{MinAmount: decPtr("100000"), MaxAmount: decPtr("50000")}, actor); !errors.Is(err, domain.ErrProductAmountRange) {
		t.Fatalf("plafon min>max: err = %v, ingin ErrProductAmountRange", err)
	}
	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{MinTermMonths: intPtr(60), MaxTermMonths: intPtr(24)}, actor); !errors.Is(err, domain.ErrProductTermRange) {
		t.Fatalf("tenor min>max: err = %v, ingin ErrProductTermRange", err)
	}
	// max_amount 0 tetap sah (tanpa batas) walau min_amount besar.
	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{MinAmount: decPtr("9000000"), MaxAmount: decPtr("0")}, actor); err != nil {
		t.Fatalf("max_amount 0 harus sah: %v", err)
	}
}

// Payload kosong bukan perubahan yang sah: ditolak 422 dan tidak menyentuh repositori.
func TestProductServiceUpdateParams_PayloadKosong(t *testing.T) {
	svc, repo, audit := productParamFixture()

	_, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{}, domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin})
	if !errors.Is(err, domain.ErrProductParamsEmpty) {
		t.Fatalf("err = %v, ingin ErrProductParamsEmpty", err)
	}
	if repo.lastTx != nil {
		t.Fatal("repositori tidak boleh dipanggil untuk payload kosong")
	}
	if len(audit.events) != 0 {
		t.Fatalf("audit tidak boleh ditulis: %+v", audit.events)
	}
}

// Produk tidak ditemukan diteruskan sebagai ErrProductNotFound (dipetakan 404).
func TestProductServiceUpdateParams_ProdukTidakDitemukan(t *testing.T) {
	svc, _, _ := productParamFixture()

	_, err := svc.UpdateParams(context.Background(), "TIDAK-ADA",
		domain.UpdateProductParamsInput{RateAnnual: decPtr("10")},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin})
	if !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("err = %v, ingin ErrProductNotFound", err)
	}
}

// Penulisan parameter dan auditnya harus berada di satu transaksi: bila salah satu
// gagal, keduanya tidak terlihat.
func TestProductServiceUpdateParams_MemakaiSatuTransaksi(t *testing.T) {
	repo := &stubProductParamRepo{products: map[string]*domain.BankingProduct{
		"PMB-MUDHARABAH": {ID: uuid.New(), Code: "PMB-MUDHARABAH", ProfitSharingRatio: decimal.RequireFromString("0.4")},
	}}
	audit := &txAwareAuditRepo{}
	runner := &recordingTxRunner{tx: "TX-PRODUK"}
	svc := &productService{repo: repo, auditRepo: audit, txRunner: runner}

	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{ProfitSharingRatio: decPtr("0.5")},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}); err != nil {
		t.Fatalf("UpdateParams: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("transaksi dibuka %d kali, ingin tepat 1", runner.calls)
	}
	if repo.lastTx != runner.tx {
		t.Fatalf("repo menulis di luar transaksi: %v", repo.lastTx)
	}
	if audit.lastTx != runner.tx {
		t.Fatalf("audit menulis di luar transaksi: %v", audit.lastTx)
	}
}

// Produk dibaca DI DALAM transaksi penulisan dengan baris terkunci, sehingga audit
// before mencerminkan keadaan yang benar-benar ditimpa. Bila nilai baris sudah
// berubah oleh permintaan lain sebelum lock didapat, yang dicatat adalah nilai
// terbaru itu, bukan potret lama di luar transaksi.
func TestProductServiceUpdateParams_AuditBeforeDariBarisTerkunci(t *testing.T) {
	repo := &stubProductParamRepo{
		products: map[string]*domain.BankingProduct{
			"PMB-MUDHARABAH": {ID: uuid.New(), Code: "PMB-MUDHARABAH", RateAnnual: decimal.NewFromInt(10)},
		},
		// Keadaan baris saat dikunci sudah 20, bukan 10 yang terlihat di luar.
		locked: map[string]*domain.BankingProduct{
			"PMB-MUDHARABAH": {ID: uuid.New(), Code: "PMB-MUDHARABAH", RateAnnual: decimal.NewFromInt(20)},
		},
	}
	audit := &txAwareAuditRepo{}
	runner := &recordingTxRunner{tx: "TX-PRODUK"}
	svc := &productService{repo: repo, auditRepo: audit, txRunner: runner}

	rate := decimal.NewFromInt(30)
	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{RateAnnual: &rate},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}); err != nil {
		t.Fatalf("UpdateParams: %v", err)
	}
	if repo.readTx != runner.tx {
		t.Fatalf("pembacaan produk di luar transaksi penulisan: %v", repo.readTx)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit = %+v, ingin satu event", audit.events)
	}
	pair, ok := audit.events[0].Changes["rate_annual"].(map[string]any)
	if !ok {
		t.Fatalf("audit rate = %#v, ingin pasangan before/after", audit.events[0].Changes["rate_annual"])
	}
	if pair["before"].(decimal.Decimal).String() != "20" {
		t.Fatalf("before rate = %v, ingin nilai baris terkunci 20", pair["before"])
	}
	if pair["after"].(decimal.Decimal).String() != "30" {
		t.Fatalf("after rate = %v, ingin 30", pair["after"])
	}
}

// PUT yang tidak mengubah apa pun tetap sukses, tetapi tidak boleh menulis event
// audit yang seolah-olah ada parameter berubah.
func TestProductServiceUpdateParams_NoOpTidakMenulisAudit(t *testing.T) {
	svc, repo, audit := productParamFixture()

	same := decimal.RequireFromString("0.4")
	product, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{ProfitSharingRatio: &same},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin})
	if err != nil {
		t.Fatalf("UpdateParams no-op harus sukses: %v", err)
	}
	if product == nil {
		t.Fatal("respons no-op tidak boleh nil")
	}
	if len(audit.events) != 0 {
		t.Fatalf("no-op menulis audit: %+v", audit.events)
	}
	if repo.lastTx != nil {
		t.Fatal("no-op tidak boleh menulis baris produk")
	}
}

// Hanya bidang yang dikirim yang berubah; sisanya tetap nilai lama (partial-PUT).
func TestProductServiceUpdateParams_PartialPutMempertahankanBidangLain(t *testing.T) {
	svc, repo, _ := productParamFixture()
	code := "PMB-MUDHARABAH"
	before := *repo.products[code]

	if _, err := svc.UpdateParams(context.Background(), code,
		domain.UpdateProductParamsInput{RateAnnual: decPtr("14.5")},
		domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}); err != nil {
		t.Fatalf("UpdateParams: %v", err)
	}

	stored := repo.products[code]
	if stored.RateAnnual.String() != "14.5" {
		t.Fatalf("rate = %s, ingin 14.5", stored.RateAnnual)
	}
	if !stored.ProfitSharingRatio.Equal(before.ProfitSharingRatio) {
		t.Fatalf("nisbah berubah = %s, ingin tetap %s", stored.ProfitSharingRatio, before.ProfitSharingRatio)
	}
	if !stored.ProjectedRevenueRateAnnual.Equal(before.ProjectedRevenueRateAnnual) {
		t.Fatalf("proyeksi berubah = %s, ingin tetap %s", stored.ProjectedRevenueRateAnnual, before.ProjectedRevenueRateAnnual)
	}
	if !stored.MinAmount.Equal(before.MinAmount) || stored.MinTermMonths != before.MinTermMonths || stored.MaxTermMonths != before.MaxTermMonths {
		t.Fatalf("batas/tenor berubah: %+v, ingin tetap %+v", stored, before)
	}
	if !stored.AdminFee.Equal(before.AdminFee) || !stored.TaxRate.Equal(before.TaxRate) || !stored.EarlyWithdrawalPenaltyRate.Equal(before.EarlyWithdrawalPenaltyRate) {
		t.Fatalf("biaya/pajak/penalti berubah: %+v, ingin tetap %+v", stored, before)
	}
}

// Tarif berbasis persen dan biaya admin tidak boleh melewati batas wajar; pesannya
// menyebut satuan. Batas tepat 100 tetap diterima.
func TestProductServiceUpdateParams_TolakTarifDiAtasBatas(t *testing.T) {
	svc, _, audit := productParamFixture()
	actor := domain.Actor{Username: "admin.uji", Role: domain.RoleAdmin}

	cases := []struct {
		name  string
		input domain.UpdateProductParamsInput
		want  error
		unit  string
	}{
		{"rate", domain.UpdateProductParamsInput{RateAnnual: decPtr("101")}, domain.ErrProductRateOutOfRange, "persen per tahun"},
		{"tax", domain.UpdateProductParamsInput{TaxRate: decPtr("9999")}, domain.ErrProductTaxRateOutOfRange, "persen"},
		{"penalty", domain.UpdateProductParamsInput{EarlyWithdrawalPenaltyRate: decPtr("101")}, domain.ErrProductPenaltyRateOutOfRange, "persen per tahun"},
		{"admin_fee", domain.UpdateProductParamsInput{AdminFee: decPtr("1000000000001")}, domain.ErrProductAdminFeeOutOfRange, "rupiah"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH", c.input, actor)
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, ingin %v", err, c.want)
			}
			if !strings.Contains(err.Error(), c.unit) {
				t.Fatalf("pesan = %q, ingin menyebut satuan %q", err.Error(), c.unit)
			}
		})
	}
	if len(audit.events) != 0 {
		t.Fatalf("audit ditulis untuk nilai yang ditolak: %+v", audit.events)
	}

	// Batas atas tepat 100 diterima.
	rate := decimal.NewFromInt(100)
	if _, err := svc.UpdateParams(context.Background(), "PMB-MUDHARABAH",
		domain.UpdateProductParamsInput{RateAnnual: &rate}, actor); err != nil {
		t.Fatalf("rate tepat 100 harus diterima: %v", err)
	}
}

// Izin perubahan parameter produk adalah administratif: ADMIN dan SUPERADMIN saja.
func TestPermProdukParameterAdministratif(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin} {
		if !role.HasPermission(domain.PermCOAManage) {
			t.Fatalf("%s harus memegang izin pengelolaan parameter produk", role)
		}
	}
	for _, role := range []domain.StaffRole{domain.RoleSupervisor, domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor} {
		if role.HasPermission(domain.PermCOAManage) {
			t.Fatalf("%s tidak boleh memegang izin pengelolaan parameter produk", role)
		}
	}
}
