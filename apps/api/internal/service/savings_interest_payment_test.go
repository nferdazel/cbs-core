package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubSavingsRepo hanya melayani bagian yang diuji; sisanya tidak dipakai pada
// alur pembayaran bunga. Jalur yang menyentuh transaksi database diverifikasi
// end-to-end terhadap PostgreSQL, bukan lewat stub transaksi.
type stubSavingsRepo struct {
	unpaid []domain.InterestAccrualRecord
}

func (s *stubSavingsRepo) ListSavingsAccounts(context.Context) ([]domain.SavingsAccountInfo, error) {
	return nil, nil
}

func (s *stubSavingsRepo) GetSavingsAccount(context.Context, string) (*domain.SavingsAccountInfo, error) {
	return nil, nil
}

func (s *stubSavingsRepo) OpeningBalances(context.Context, time.Time) (map[uuid.UUID]decimal.Decimal, error) {
	return nil, nil
}

func (s *stubSavingsRepo) DailyNetChanges(context.Context, time.Time, time.Time) ([]domain.DailyNetChange, error) {
	return nil, nil
}

func (s *stubSavingsRepo) InsertInterestAccrual(context.Context, any, *domain.InterestAccrualRecord) (bool, error) {
	return false, nil
}

func (s *stubSavingsRepo) UpdateInterestAccrualJournal(context.Context, any, uuid.UUID, string, uuid.UUID) error {
	return nil
}

func (s *stubSavingsRepo) ListUnpaidAccruals(context.Context, string) ([]domain.InterestAccrualRecord, error) {
	return s.unpaid, nil
}

func (s *stubSavingsRepo) MarkAccrualPaid(context.Context, any, uuid.UUID, uuid.UUID) error {
	return nil
}

func (s *stubSavingsRepo) InsertAdminFeeCharge(context.Context, any, *domain.AdminFeeChargeRecord) (bool, error) {
	return false, nil
}

func (s *stubSavingsRepo) UpdateAdminFeeChargeJournal(context.Context, any, uuid.UUID, string, uuid.UUID) error {
	return nil
}

var _ domain.SavingsInterestRepository = (*stubSavingsRepo)(nil)

// Tanpa akrual yang belum dibayar, pembayaran tidak menyentuh database sama sekali
// dan ringkasannya nol. db nil membuktikan tidak ada transaksi yang dibuka.
func TestPayInterestToAccountsTanpaAkrualTidakMenyentuhDatabase(t *testing.T) {
	svc := &savingsInterestService{repo: &stubSavingsRepo{}}

	summary, err := svc.PayInterestToAccounts(context.Background(), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "tester")
	if err != nil {
		t.Fatalf("pembayaran gagal: %v", err)
	}
	if summary.Period != "2026-09" {
		t.Fatalf("periode %q, ingin 2026-09", summary.Period)
	}
	if summary.ProcessedAccounts != 0 || summary.FailedAccounts != 0 {
		t.Fatalf("ringkasan tidak nol: processed=%d failed=%d", summary.ProcessedAccounts, summary.FailedAccounts)
	}
	if !summary.TotalInterest.IsZero() {
		t.Fatalf("total bunga %s, ingin 0", summary.TotalInterest.String())
	}
}

// Akrual bernilai nol ditandai dilewati dan tidak diposting, sebelum transaksi dibuka.
func TestPayAccrualNominalNolDilewati(t *testing.T) {
	svc := &savingsInterestService{}

	result := svc.payAccrual(context.Background(), domain.InterestAccrualRecord{
		ID:            uuid.New(),
		AccountNumber: "0011010000000014",
		Period:        "2026-09",
		Amount:        decimal.Zero,
	}, time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), "tester")

	if result.Status != domain.BatchItemSkipped {
		t.Fatalf("status %q, ingin %q", result.Status, domain.BatchItemSkipped)
	}
	if result.JournalReference != "" {
		t.Fatalf("jurnal %q, ingin kosong", result.JournalReference)
	}
}

// stubTaxConfig menyediakan nilai konfigurasi pajak untuk test.
type stubTaxConfig struct {
	domain.SystemConfigService
	decimals map[string]decimal.Decimal
}

func (s stubTaxConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := s.decimals[key]; ok {
		return v
	}
	return fallback
}

// stubBalanceAccountRepo melayani rekening dari peta id.
type stubBalanceAccountRepo struct {
	domain.AccountRepository
	accounts map[uuid.UUID]*domain.Account
}

func (s *stubBalanceAccountRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Account, error) {
	acc, ok := s.accounts[id]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return acc, nil
}

// PPh final atas bunga tabungan diukur dari SALDO tabungan, bukan dari bunganya, dan
// memakai tarif serta ambang dari konfigurasi.
func TestSavingsAccrualTax_MemakaiSaldoDanKonfigurasi(t *testing.T) {
	accountID := uuid.New()
	accounts := map[uuid.UUID]*domain.Account{
		accountID: {ID: accountID, Balance: decimal.NewFromInt(50_000_000)},
	}
	svc := &savingsInterestService{
		accountRepo: &stubBalanceAccountRepo{accounts: accounts},
		configSvc: stubTaxConfig{decimals: map[string]decimal.Decimal{
			configSavingsTaxRate:   decimal.NewFromInt(20),
			configSavingsTaxExempt: decimal.NewFromInt(7_500_000),
		}},
	}
	accrual := domain.InterestAccrualRecord{
		AccountID: accountID,
		Book:      domain.BookConventional,
		Amount:    decimal.NewFromInt(750_000),
	}

	tax, err := svc.savingsAccrualTax(context.Background(), accrual)
	if err != nil {
		t.Fatalf("menghitung pajak: %v", err)
	}
	if !tax.Equal(decimal.NewFromInt(150_000)) {
		t.Fatalf("pajak %s, ingin 150000 (20%% dari 750000)", tax)
	}

	// Saldo di bawah ambang dibebaskan meski bunganya besar.
	accounts[accountID].Balance = decimal.NewFromInt(5_000_000)
	tax, err = svc.savingsAccrualTax(context.Background(), accrual)
	if err != nil {
		t.Fatalf("menghitung pajak: %v", err)
	}
	if !tax.IsZero() {
		t.Fatalf("saldo di bawah ambang harus bebas potongan, dapat %s", tax)
	}

	// Bagi hasil mudharabah bukan bunga: belum dipotong sampai bank memutuskan.
	accounts[accountID].Balance = decimal.NewFromInt(500_000_000)
	accrual.Book = domain.BookSyariah
	tax, err = svc.savingsAccrualTax(context.Background(), accrual)
	if err != nil {
		t.Fatalf("menghitung pajak: %v", err)
	}
	if !tax.IsZero() {
		t.Fatalf("bagi hasil syariah belum dipotong, dapat %s", tax)
	}
}
