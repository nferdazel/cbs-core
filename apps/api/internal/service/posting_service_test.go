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
			"10101": {ID: uuid.New(), AccountNumber: "10101", AccountType: domain.AccountTypeInternalGL, Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeDebit},
			"20100": {ID: uuid.New(), AccountNumber: "20100", AccountType: domain.AccountTypeInternalGL, Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeCredit},
		}},
		referenceGen: stubReferenceGen{},
		dates:        &reversalDateRepo{date: tanggalBisnisUji},
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

// EntryDate nol harus memakai tanggal bisnis berjalan, bukan tanggal kalender:
// bila tutup hari tertinggal, tanggal kalender jatuh di periode yang belum dibuka.
func TestPostTxEntryDateDefaultsToBusinessDate(t *testing.T) {
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)

	req := balancedPostingRequest()
	entry, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), req)
	if err != nil {
		t.Fatalf("posting gagal: %v", err)
	}

	if !entry.EntryDate.Equal(tanggalBisnisUji) {
		t.Fatalf("entry_date default = %s, ingin tanggal bisnis %s", entry.EntryDate, tanggalBisnisUji)
	}
	if repo.inserted == nil {
		t.Fatal("entri tidak sampai ke repository")
	}
	if !repo.inserted.EntryDate.Equal(tanggalBisnisUji) {
		t.Fatalf("entry_date tersimpan %s, ingin tanggal bisnis %s", repo.inserted.EntryDate, tanggalBisnisUji)
	}
}

// Sumber tanggal bisnis yang tidak terbaca harus menolak posting, bukan menebak
// tanggal kalender dan bukan menulis jurnal di periode yang salah.
func TestPostTxEntryDateRejectsMissingBusinessDate(t *testing.T) {
	cases := []struct {
		name  string
		dates domain.BusinessDateRepository
	}{
		{"repositori nil", nil},
		{"pembacaan gagal", &reversalDateRepo{err: errors.New("database tanggal tidak dapat dihubungi")}},
		{"tanggal nol", &reversalDateRepo{date: time.Time{}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubPostingRepo{}
			svc := newPostingServiceForTest(repo)
			svc.dates = tc.dates

			if _, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), balancedPostingRequest()); err == nil {
				t.Fatal("posting harus ditolak saat tanggal bisnis tidak terbaca")
			}
			if repo.inserted != nil {
				t.Fatal("jurnal tidak boleh tertulis saat tanggal bisnis tidak terbaca")
			}
		})
	}
}

// EntryDate yang diisi pemanggil harus tersimpan apa adanya, bukan ditimpa tanggal jalan.
// Sumber tanggal bisnis sengaja dibuat gagal: jalur eksplisit tidak boleh bergantung padanya.
func TestPostTxEntryDateUsesCallerValue(t *testing.T) {
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)
	svc.dates = &reversalDateRepo{err: errors.New("sumber tanggal tidak dipakai untuk tanggal eksplisit")}

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

// Penarikan yang melebihi saldo rekening nasabah wajib ditolak di mesin posting,
// bukan hanya di service pemanggil: pemeriksaan di pemanggil memakai saldo di luar
// transaksi sehingga dua penarikan paralel dapat sama-sama lolos.
func TestPostTxRejectsNegativeCustomerBalance(t *testing.T) {
	const customer = "2010010001"
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)
	svc.accountRepo = &stubPostingAccountRepo{accounts: map[string]*domain.Account{
		"10101": {ID: uuid.New(), AccountNumber: "10101", AccountType: domain.AccountTypeInternalGL, Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeDebit},
		customer: {
			ID: uuid.New(), AccountNumber: customer,
			AccountType: domain.AccountTypeSavings, Status: domain.AccountStatusActive,
			NormalBalance: domain.BalanceTypeCredit, Balance: decimal.NewFromInt(50),
		},
	}}

	req := domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     "uji batas saldo rekening nasabah",
		CreatedBy:       "tester",
		Lines: []domain.PostingLine{
			{AccountNumber: customer, Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(100)},
			{AccountNumber: "10101", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(100)},
		},
	}

	_, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), req)
	if !errors.Is(err, domain.ErrInsufficientFunds) {
		t.Fatalf("penarikan melebihi saldo harus ditolak ErrInsufficientFunds, dapat %v", err)
	}
	if repo.inserted != nil {
		t.Fatal("jurnal tidak boleh tersimpan saat saldo rekening nasabah tidak mencukupi")
	}
}

// Saldo tepat nol masih sah; yang dilarang hanya saldo negatif.
func TestPostTxAllowsExactZeroCustomerBalance(t *testing.T) {
	const customer = "2010010002"
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)
	svc.accountRepo = &stubPostingAccountRepo{accounts: map[string]*domain.Account{
		"10101": {ID: uuid.New(), AccountNumber: "10101", AccountType: domain.AccountTypeInternalGL, Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeDebit},
		customer: {
			ID: uuid.New(), AccountNumber: customer,
			AccountType: domain.AccountTypeSavings, Status: domain.AccountStatusActive,
			NormalBalance: domain.BalanceTypeCredit, Balance: decimal.NewFromInt(100),
		},
	}}

	req := domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Description:     "uji saldo nol",
		CreatedBy:       "tester",
		Lines: []domain.PostingLine{
			{AccountNumber: customer, Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(100)},
			{AccountNumber: "10101", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(100)},
		},
	}

	if _, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), req); err != nil {
		t.Fatalf("penarikan sebesar saldo harus lolos, dapat %v", err)
	}
	if repo.inserted == nil {
		t.Fatal("jurnal seharusnya tersimpan")
	}
}

// Akun GL internal dikecualikan: akun kontra seperti cadangan PPAP (10900) memang
// bersaldo negatif menurut normal balance-nya, dan saldo GL agregat bukan dana nasabah.
func TestPostTxAllowsNegativeInternalGL(t *testing.T) {
	repo := &stubPostingRepo{}
	svc := newPostingServiceForTest(repo)
	svc.accountRepo = &stubPostingAccountRepo{accounts: map[string]*domain.Account{
		"10101": {ID: uuid.New(), AccountNumber: "10101", AccountType: domain.AccountTypeInternalGL, Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeDebit},
		"10900": {ID: uuid.New(), AccountNumber: "10900", AccountType: domain.AccountTypeInternalGL, Status: domain.AccountStatusActive, NormalBalance: domain.BalanceTypeCredit},
	}}

	req := domain.PostingRequest{
		TransactionType: domain.TxTypeAdjustment,
		Description:     "uji akun kontra GL",
		CreatedBy:       "tester",
		Lines: []domain.PostingLine{
			{AccountNumber: "10900", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(500)},
			{AccountNumber: "10101", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(500)},
		},
	}

	if _, err := svc.PostTx(context.Background(), (*sql.Tx)(nil), req); err != nil {
		t.Fatalf("akun GL internal harus boleh bersaldo negatif, dapat %v", err)
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
