package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubDormantConfig adalah SystemConfigService in-memory; hanya GetInt yang dipakai.
type stubDormantConfig struct{ months int }

func (s stubDormantConfig) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (s stubDormantConfig) GetInt(context.Context, string, int) int          { return s.months }
func (s stubDormantConfig) GetString(context.Context, string, string) string { return "" }
func (s stubDormantConfig) GetBool(context.Context, string, bool) bool       { return false }
func (s stubDormantConfig) Invalidate(string)                                {}

func TestDormantAfterMonths(t *testing.T) {
	ctx := context.Background()

	// Konfigurasi tidak tersedia: fallback terdokumentasi dipakai (produksi selalu
	// menyediakan service, ini untuk lingkungan yang belum di-provision).
	if got, warn := dormantAfterMonths(ctx, nil); got != domain.DormantAfterMonthsFallback || warn != "" {
		t.Fatalf("config nil: dapat (%d, %q), mau (%d, \"\")", got, warn, domain.DormantAfterMonthsFallback)
	}

	// Nilai valid dipakai apa adanya.
	if got, warn := dormantAfterMonths(ctx, stubDormantConfig{months: 6}); got != 6 || warn != "" {
		t.Fatalf("nilai valid: dapat (%d, %q), mau (6, \"\")", got, warn)
	}

	// Nilai <= 0 tidak valid: fallback + peringatan, bukan lewat diam-diam.
	for _, m := range []int{0, -3} {
		got, warn := dormantAfterMonths(ctx, stubDormantConfig{months: m})
		if got != domain.DormantAfterMonthsFallback {
			t.Fatalf("nilai %d: dapat ambang %d, mau fallback %d", m, got, domain.DormantAfterMonthsFallback)
		}
		if warn == "" {
			t.Fatalf("nilai %d: harus menghasilkan peringatan", m)
		}
	}
}

// mapLedgerAccountRepo mengembalikan rekening berbeda per nomor, agar transfer
// sumber/tujuan dapat diuji dengan status berbeda.
type mapLedgerAccountRepo struct {
	domain.AccountRepository
	accounts map[string]*domain.Account
}

func (m *mapLedgerAccountRepo) GetByNumber(_ context.Context, accountNumber string) (*domain.Account, error) {
	if acc, ok := m.accounts[accountNumber]; ok {
		return acc, nil
	}
	return nil, domain.ErrAccountNotFound
}

func statusAccount(number string, status domain.AccountStatus) *domain.Account {
	return &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    number,
		AccountType:      domain.AccountTypeSavings,
		NormalBalance:    domain.BalanceTypeCredit,
		Status:           status,
		Currency:         "IDR",
		Balance:          decimal.NewFromInt(100_000),
		AvailableBalance: decimal.NewFromInt(100_000),
	}
}

func newLedgerWithAccounts(posting *stubPostingSvc, accounts ...*domain.Account) *ledgerService {
	m := make(map[string]*domain.Account, len(accounts))
	for _, a := range accounts {
		m[a.AccountNumber] = a
	}
	return &ledgerService{
		accountRepo: &mapLedgerAccountRepo{accounts: m},
		resolver:    stubLedgerResolver{},
		posting:     posting,
		limits:      stubLimits{},
		approvals:   &stubApprovals{},
	}
}

func TestDepositAllowsDormantAccount(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerWithAccounts(posting, statusAccount("A", domain.AccountStatusDormant))

	_, err := svc.Deposit(context.Background(), domain.DepositRequest{
		AccountNumber: "A", Amount: decimal.NewFromInt(1000), Currency: "IDR", Actor: testActor(),
	})
	if err != nil {
		t.Fatalf("setoran ke rekening dormant harus diterima: %v", err)
	}
	if posting.calls != 1 {
		t.Fatalf("jurnal harus diposting sekali, dapat %d", posting.calls)
	}
}

func TestWithdrawRejectsDormantAccount(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerWithAccounts(posting, statusAccount("A", domain.AccountStatusDormant))

	_, err := svc.Withdraw(context.Background(), domain.WithdrawRequest{
		AccountNumber: "A", Amount: decimal.NewFromInt(1000), Currency: "IDR", Actor: testActor(),
	})
	if !errors.Is(err, domain.ErrAccountDormant) {
		t.Fatalf("penarikan dari rekening dormant harus ErrAccountDormant, dapat %v", err)
	}
	if posting.calls != 0 {
		t.Fatal("tidak boleh ada jurnal untuk penarikan yang ditolak")
	}
}

func TestStatusRulesForFrozenAndClosed(t *testing.T) {
	posting := &stubPostingSvc{}
	for _, status := range []domain.AccountStatus{domain.AccountStatusFrozen, domain.AccountStatusClosed} {
		svc := newLedgerWithAccounts(posting, statusAccount("A", status))
		if _, err := svc.Deposit(context.Background(), domain.DepositRequest{
			AccountNumber: "A", Amount: decimal.NewFromInt(1000), Currency: "IDR", Actor: testActor(),
		}); !errors.Is(err, domain.ErrAccountInactive) {
			t.Fatalf("setoran %s harus ErrAccountInactive, dapat %v", status, err)
		}
		if _, err := svc.Withdraw(context.Background(), domain.WithdrawRequest{
			AccountNumber: "A", Amount: decimal.NewFromInt(1000), Currency: "IDR", Actor: testActor(),
		}); !errors.Is(err, domain.ErrAccountInactive) {
			t.Fatalf("penarikan %s harus ErrAccountInactive, dapat %v", status, err)
		}
	}
}

func TestTransferRejectsDormantSource(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerWithAccounts(posting,
		statusAccount("SRC", domain.AccountStatusDormant),
		statusAccount("DST", domain.AccountStatusActive),
	)

	_, err := svc.TransferInternal(context.Background(), domain.TransferRequest{
		SourceAccountNumber:      "SRC",
		DestinationAccountNumber: "DST",
		Amount:                   decimal.NewFromInt(1000),
		Currency:                 "IDR",
		Actor:                    testActor(),
	})
	if !errors.Is(err, domain.ErrAccountDormant) {
		t.Fatalf("sumber dormant harus ErrAccountDormant, dapat %v", err)
	}
	if posting.calls != 0 {
		t.Fatal("tidak boleh ada jurnal untuk transfer yang ditolak")
	}
}

func TestTransferAllowsDormantDestination(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerWithAccounts(posting,
		statusAccount("SRC", domain.AccountStatusActive),
		statusAccount("DST", domain.AccountStatusDormant),
	)

	_, err := svc.TransferInternal(context.Background(), domain.TransferRequest{
		SourceAccountNumber:      "SRC",
		DestinationAccountNumber: "DST",
		Amount:                   decimal.NewFromInt(1000),
		Currency:                 "IDR",
		Actor:                    testActor(),
	})
	if err != nil {
		t.Fatalf("tujuan dormant harus diterima: %v", err)
	}
	if posting.calls != 1 {
		t.Fatalf("jurnal harus diposting sekali, dapat %d", posting.calls)
	}
}

func TestTransferRejectsFrozenDestination(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerWithAccounts(posting,
		statusAccount("SRC", domain.AccountStatusActive),
		statusAccount("DST", domain.AccountStatusFrozen),
	)

	_, err := svc.TransferInternal(context.Background(), domain.TransferRequest{
		SourceAccountNumber:      "SRC",
		DestinationAccountNumber: "DST",
		Amount:                   decimal.NewFromInt(1000),
		Currency:                 "IDR",
		Actor:                    testActor(),
	})
	if !errors.Is(err, domain.ErrAccountInactive) {
		t.Fatalf("tujuan FROZEN harus ErrAccountInactive, dapat %v", err)
	}
	if posting.calls != 0 {
		t.Fatal("tidak boleh ada jurnal untuk transfer yang ditolak")
	}
}

// Gerbang reaktivasi: rekening yang bukan DORMANT ditolak, tidak boleh diaktifkan
// diam-diam (termasuk FROZEN/CLOSED). Jalur sukses butuh database, tidak diuji di sini.
func TestReactivateRejectsNonDormant(t *testing.T) {
	for _, status := range []domain.AccountStatus{
		domain.AccountStatusActive, domain.AccountStatusFrozen, domain.AccountStatusClosed,
	} {
		svc := &accountService{accountRepo: &stubLedgerAccountRepo{acc: statusAccount("A", status)}}
		_, err := svc.ReactivateAccount(context.Background(), "A", "", testActor())
		if !errors.Is(err, domain.ErrAccountNotDormant) {
			t.Fatalf("status %s harus ErrAccountNotDormant, dapat %v", status, err)
		}
	}
}

func TestReactivateRejectsCrossBranch(t *testing.T) {
	acc := statusAccount("A", domain.AccountStatusDormant)
	acc.BranchCode = "002"
	svc := &accountService{accountRepo: &stubLedgerAccountRepo{acc: acc}}

	// Aktor cabang 001; rekening di cabang 002 ditolak 403 sebelum menyentuh database.
	_, err := svc.ReactivateAccount(context.Background(), "A", "", testActor())
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("reaktivasi lintas cabang harus ErrCrossBranchAccess, dapat %v", err)
	}
}

// Gerbang unfreeze: rekening yang bukan FROZEN ditolak, tidak boleh diaktifkan
// diam-diam (termasuk ACTIVE/DORMANT/CLOSED). Jalur sukses butuh database.
func TestUnfreezeRejectsNonFrozen(t *testing.T) {
	for _, status := range []domain.AccountStatus{
		domain.AccountStatusActive, domain.AccountStatusDormant, domain.AccountStatusClosed,
	} {
		svc := &accountService{accountRepo: &stubLedgerAccountRepo{acc: statusAccount("A", status)}}
		_, err := svc.UnfreezeAccount(context.Background(), "A", "", testActor())
		if !errors.Is(err, domain.ErrAccountNotFrozen) {
			t.Fatalf("status %s harus ErrAccountNotFrozen, dapat %v", status, err)
		}
	}
}
