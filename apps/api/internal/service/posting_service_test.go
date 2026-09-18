package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubPostingRepo menangkap entri yang ditulis posting engine tanpa menyentuh DB.
// Embedding antarmuka memberi metode lain yang tidak dipakai test.
type stubPostingRepo struct {
	domain.PostingRepository
	inserted *domain.JournalEntry
}

func (s *stubPostingRepo) InsertJournal(ctx context.Context, tx any, entry *domain.JournalEntry) error {
	s.inserted = entry
	return nil
}

func (s *stubPostingRepo) FindJournalByIdempotencyKey(ctx context.Context, key string) (*domain.JournalEntry, error) {
	return nil, nil
}

// stubPostingAccountRepo hanya melayani kunci akun dan update saldo.
type stubPostingAccountRepo struct {
	domain.AccountRepository
	accounts map[string]*domain.Account
}

func (s *stubPostingAccountRepo) GetByNumberForUpdate(ctx context.Context, tx any, accountNumber string) (*domain.Account, error) {
	acc, ok := s.accounts[accountNumber]
	if !ok {
		return nil, domain.ErrAccountNotFound
	}
	return acc, nil
}

func (s *stubPostingAccountRepo) UpdateBalance(ctx context.Context, tx any, accountID uuid.UUID, balance, available decimal.Decimal, version int) error {
	return nil
}

// stubReferenceGen mengembalikan nomor tetap agar test tidak butuh sequence.
type stubReferenceGen struct{}

func (stubReferenceGen) Next(txType domain.TransactionType, at time.Time) (string, error) {
	return "REF-TEST-000001", nil
}

func (stubReferenceGen) NextTx(ctx context.Context, tx any, txType domain.TransactionType, at time.Time) (string, error) {
	return "REF-TEST-000001", nil
}

func newPostingServiceForTest(repo *stubPostingRepo) *postingService {
	return &postingService{
		postingRepo: repo,
		accountRepo: &stubPostingAccountRepo{accounts: map[string]*domain.Account{
			"10101": {ID: uuid.New(), AccountNumber: "10101", Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeDebit},
			"20100": {ID: uuid.New(), AccountNumber: "20100", Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeCredit},
		}},
		referenceGen: stubReferenceGen{},
	}
}

func balancedPostingRequest() domain.PostingRequest {
	return domain.PostingRequest{
		TransactionType: domain.TxTypeDeposit,
		Description:     "uji tanggal entri",
		CreatedBy:       "tester",
		Lines: []domain.PostingLine{
			{AccountNumber: "10101", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(100)},
			{AccountNumber: "20100", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(100)},
		},
	}
}

// EntryDate nol harus jatuh ke tanggal UTC hari ini agar pemanggil lama tidak berubah.
func TestPostTxEntryDateDefaultsToTodayUTC(t *testing.T) {
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)

	req := balancedPostingRequest()
	entry, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), req)
	if err != nil {
		t.Fatalf("posting gagal: %v", err)
	}

	want := time.Now().UTC().Format("2006-01-02")
	if got := entry.EntryDate.UTC().Format("2006-01-02"); got != want {
		t.Fatalf("entry_date default = %s, ingin hari ini UTC %s", got, want)
	}
	if repo.inserted == nil {
		t.Fatal("entri tidak sampai ke repository")
	}
	if !repo.inserted.EntryDate.Equal(entry.EntryDate) {
		t.Fatalf("entry_date tersimpan %s berbeda dari entri %s", repo.inserted.EntryDate, entry.EntryDate)
	}
}

// EntryDate yang diisi pemanggil harus tersimpan apa adanya, bukan ditimpa tanggal jalan.
func TestPostTxEntryDateUsesCallerValue(t *testing.T) {
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)

	explicit := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	req := balancedPostingRequest()
	req.EntryDate = explicit

	entry, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), req)
	if err != nil {
		t.Fatalf("posting gagal: %v", err)
	}
	if !entry.EntryDate.Equal(explicit) {
		t.Fatalf("entry_date = %s, ingin %s", entry.EntryDate, explicit)
	}
	if repo.inserted == nil || !repo.inserted.EntryDate.Equal(explicit) {
		t.Fatalf("entry_date tersimpan bukan nilai pemanggil: %+v", repo.inserted)
	}
}

// Penjaga status harus sadar arah: aturan dormant di mesin posting pernah hilang
// sehingga setoran ke rekening dormant ditolak padahal service pemanggil sudah
// mengizinkannya. Test ini mengunci aturan itu di satu-satunya pintu perubahan saldo.
func TestStatusGuardForDirection(t *testing.T) {
	cases := []struct {
		name      string
		direction domain.EntryDirection
		status    domain.AccountStatus
		wantErr   error
	}{
		{"kredit ke aktif", domain.DirectionCredit, domain.AccountStatusActive, nil},
		{"kredit ke dormant", domain.DirectionCredit, domain.AccountStatusDormant, nil},
		{"kredit ke frozen", domain.DirectionCredit, domain.AccountStatusFrozen, domain.ErrAccountInactive},
		{"kredit ke closed", domain.DirectionCredit, domain.AccountStatusClosed, domain.ErrAccountInactive},
		{"debit dari aktif", domain.DirectionDebit, domain.AccountStatusActive, nil},
		{"debit dari dormant", domain.DirectionDebit, domain.AccountStatusDormant, domain.ErrAccountDormant},
		{"debit dari frozen", domain.DirectionDebit, domain.AccountStatusFrozen, domain.ErrAccountInactive},
		{"debit dari closed", domain.DirectionDebit, domain.AccountStatusClosed, domain.ErrAccountInactive},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := statusGuardForDirection(tc.direction, tc.status)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("arah %s status %s: ingin lolos, dapat %v", tc.direction, tc.status, err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("arah %s status %s: ingin %v, dapat %v", tc.direction, tc.status, tc.wantErr, err)
			}
		})
	}
}
