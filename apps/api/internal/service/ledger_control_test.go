package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubLimits menggantikan pemeriksa batas: test cukup menentukan hasilnya.
type stubLimits struct{ checkErr error }

func (s stubLimits) ForActor(ctx context.Context, actor domain.Actor, txType string) (domain.TransactionLimit, error) {
	return domain.TransactionLimit{}, nil
}

func (s stubLimits) Check(ctx context.Context, actor domain.Actor, txType string, amount decimal.Decimal) error {
	return s.checkErr
}

// stubApprovals mencatat permintaan persetujuan yang diajukan.
type stubApprovals struct {
	created []domain.CreateMakerCheckerInput
	err     error
	// threshold mengatur ambang persetujuan per jenis aksi. Bila nil, dipakai ambang
	// default 50 juta agar test lain tidak berubah perilaku.
	threshold map[string]decimal.Decimal
}

func (s *stubApprovals) Threshold(_ context.Context, actionType string) decimal.Decimal {
	if s.threshold != nil {
		if v, ok := s.threshold[actionType]; ok {
			return v
		}
	}
	return decimal.NewFromInt(50_000_000)
}

func (s *stubApprovals) CreateRequest(ctx context.Context, input domain.CreateMakerCheckerInput, actor domain.Actor) (*domain.MakerCheckerRequest, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.created = append(s.created, input)
	return &domain.MakerCheckerRequest{ID: uuid.New(), ActionType: input.ActionType, Status: domain.MakerCheckerPending}, nil
}

func (s *stubApprovals) Approve(ctx context.Context, id uuid.UUID, actor domain.Actor, notes string) error {
	return nil
}
func (s *stubApprovals) Reject(ctx context.Context, id uuid.UUID, actor domain.Actor, notes string) error {
	return nil
}
func (s *stubApprovals) ListPending(ctx context.Context, actor domain.Actor) ([]domain.MakerCheckerRequest, error) {
	return nil, nil
}

// stubPostingSvc menangkap permintaan posting tanpa menyentuh database.
type stubPostingSvc struct {
	lastAnyTx any
	last      domain.PostingRequest
	calls     int
}

func (s *stubPostingSvc) Post(ctx context.Context, req domain.PostingRequest) (*domain.JournalEntry, error) {
	s.calls++
	s.last = req
	s.lastAnyTx = nil
	return &domain.JournalEntry{ID: uuid.New(), ReferenceNumber: "REF-TEST"}, nil
}

func (s *stubPostingSvc) PostTx(ctx context.Context, tx any, req domain.PostingRequest) (*domain.JournalEntry, error) {
	s.calls++
	s.last = req
	s.lastAnyTx = tx
	return &domain.JournalEntry{ID: uuid.New(), ReferenceNumber: "REF-TEST"}, nil
}

type stubLedgerAccountRepo struct {
	domain.AccountRepository
	acc *domain.Account
}

func (s *stubLedgerAccountRepo) GetByNumber(ctx context.Context, accountNumber string) (*domain.Account, error) {
	if s.acc == nil {
		return nil, domain.ErrAccountNotFound
	}
	return s.acc, nil
}

type stubLedgerResolver struct{}

func (stubLedgerResolver) ResolveGLAccount(ctx context.Context, tx any, coaCode string) (string, error) {
	return "GL-" + coaCode, nil
}

func testActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "teller01", Role: domain.RoleTeller, BranchCode: "001"}
}

func savingsAccount() *domain.Account {
	return &domain.Account{
		ID:               uuid.New(),
		AccountNumber:    "0011010000000014",
		AccountType:      domain.AccountTypeSavings,
		NormalBalance:    domain.BalanceTypeCredit,
		Status:           domain.AccountStatusActive,
		Currency:         "IDR",
		Balance:          decimal.NewFromInt(1_000_000),
		AvailableBalance: decimal.NewFromInt(1_000_000),
	}
}

func newLedgerForTest(limits stubLimits, approvals *stubApprovals, posting *stubPostingSvc) *ledgerService {
	return &ledgerService{
		accountRepo: &stubLedgerAccountRepo{acc: savingsAccount()},
		resolver:    stubLedgerResolver{},
		posting:     posting,
		limits:      limits,
		approvals:   approvals,
	}
}

// Transaksi di bawah ambang harus lolos tanpa membuat permintaan persetujuan.
func TestGuardLimitAllowsWithinThreshold(t *testing.T) {
	approvals := &stubApprovals{}
	svc := newLedgerForTest(stubLimits{checkErr: nil}, approvals, &stubPostingSvc{})

	if err := svc.guardLimit(context.Background(), testActor(), ActionDeposit, "deposit", decimal.NewFromInt(1000), nil); err != nil {
		t.Fatalf("transaksi di bawah ambang harus lolos, dapat %v", err)
	}
	if len(approvals.created) != 0 {
		t.Fatal("tidak boleh ada permintaan persetujuan untuk transaksi di bawah ambang")
	}
}

// Melebihi batas keras harus DITOLAK, bukan dialihkan ke persetujuan: pejabat pun
// tidak berwenang melampaui batas yang ditetapkan bank.
func TestGuardLimitRejectsHardLimit(t *testing.T) {
	approvals := &stubApprovals{}
	svc := newLedgerForTest(stubLimits{checkErr: domain.ErrLimitPerTransaction}, approvals, &stubPostingSvc{})

	err := svc.guardLimit(context.Background(), testActor(), ActionDeposit, "deposit", decimal.NewFromInt(999_000_000), nil)
	if !errors.Is(err, domain.ErrLimitPerTransaction) {
		t.Fatalf("harus ditolak dengan ErrLimitPerTransaction, dapat %v", err)
	}
	if len(approvals.created) != 0 {
		t.Fatal("batas keras tidak boleh dialihkan menjadi permintaan persetujuan")
	}
}

// Melewati ambang persetujuan harus mengalihkan transaksi ke maker-checker dan
// mengembalikan PendingApprovalError berisi id permintaan.
func TestGuardLimitDivertsToMakerChecker(t *testing.T) {
	approvals := &stubApprovals{}
	svc := newLedgerForTest(stubLimits{checkErr: domain.ErrRequiresApproval}, approvals, &stubPostingSvc{})

	payload := map[string]any{"account_number": "0011010000000014", "amount": "25000000"}
	err := svc.guardLimit(context.Background(), testActor(), ActionDeposit, "deposit", decimal.NewFromInt(25_000_000), payload)

	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("harus mengembalikan PendingApprovalError, dapat %v", err)
	}
	if pending.RequestID == uuid.Nil {
		t.Fatal("id permintaan persetujuan harus terisi")
	}
	if pending.ActionType != ActionDeposit {
		t.Fatalf("jenis aksi = %s, mau %s", pending.ActionType, ActionDeposit)
	}
	if len(approvals.created) != 1 {
		t.Fatalf("harus ada tepat satu permintaan persetujuan, dapat %d", len(approvals.created))
	}
	if approvals.created[0].Payload["account_number"] != "0011010000000014" {
		t.Fatal("payload persetujuan harus memuat rincian transaksi")
	}
}

// Persetujuan harus benar-benar mengeksekusi transaksi, bukan sekadar mengubah status.
func TestExecuteApprovedDepositPostsJournal(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerForTest(stubLimits{}, &stubApprovals{}, posting)

	err := svc.ExecuteApproved(context.Background(), "tx-handle", ActionDeposit, map[string]any{
		"account_number": "0011010000000014",
		"amount":         "25000000",
		"description":    "Setoran disetujui",
	}, testActor())
	if err != nil {
		t.Fatalf("eksekusi persetujuan gagal: %v", err)
	}
	if posting.calls != 1 {
		t.Fatalf("jurnal harus diposting tepat sekali, dapat %d", posting.calls)
	}
	// Harus memakai transaksi pemanggil agar persetujuan dan posting commit bersama.
	if posting.lastAnyTx != "tx-handle" {
		t.Fatalf("harus memakai transaksi pemanggil, dapat %v", posting.lastAnyTx)
	}
	if posting.last.TransactionType != domain.TxTypeDeposit {
		t.Fatalf("jenis transaksi = %s, mau DEPOSIT", posting.last.TransactionType)
	}
	if len(posting.last.Lines) != 2 {
		t.Fatalf("setoran harus dua baris jurnal, dapat %d", len(posting.last.Lines))
	}
	// Identitas pelaku diambil dari actor yang menyetujui, bukan dari payload.
	if posting.last.CreatedBy == "" {
		t.Fatal("created_by harus terisi dari identitas pelaku")
	}
}

func TestExecuteApprovedRejectsUnknownPayload(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerForTest(stubLimits{}, &stubApprovals{}, posting)

	if err := svc.ExecuteApproved(context.Background(), nil, ActionDeposit, nil, testActor()); err == nil {
		t.Fatal("payload kosong harus ditolak")
	}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionDeposit, map[string]any{"account_number": "x", "amount": "bukan-angka"}, testActor()); err == nil {
		t.Fatal("nominal tidak valid harus ditolak")
	}
	if posting.calls != 0 {
		t.Fatal("jurnal tidak boleh diposting saat payload tidak valid")
	}
}

// Registry harus menolak aksi yang tidak punya eksekutor, bukan diam-diam
// menyetujui tanpa efek.
func TestExecutorRegistryRejectsUnknownAction(t *testing.T) {
	registry := NewExecutorRegistry()
	err := registry.ExecuteApproved(context.Background(), nil, "ACTION_TIDAK_DIKENAL", nil, testActor())
	if !errors.Is(err, domain.ErrNoExecutorForAction) {
		t.Fatalf("harus ErrNoExecutorForAction, dapat %v", err)
	}
}

func TestExecutorRegistryDispatchesToRegisteredExecutor(t *testing.T) {
	registry := NewExecutorRegistry()
	posting := &stubPostingSvc{}
	svc := newLedgerForTest(stubLimits{}, &stubApprovals{}, posting)
	registry.Register("deposit", svc) // sengaja huruf kecil: normalisasi harus menangkap

	err := registry.ExecuteApproved(context.Background(), nil, ActionDeposit, map[string]any{
		"account_number": "0011010000000014",
		"amount":         "1000",
	}, testActor())
	if err != nil {
		t.Fatalf("eksekusi lewat registry gagal: %v", err)
	}
	if posting.calls != 1 {
		t.Fatalf("eksekutor terdaftar harus dipanggil, dapat %d", posting.calls)
	}
}

// Jurnal harus mencatat PEMBUAT transaksi, bukan pejabat yang menyetujui. Peran
// pemeriksa tercatat di audit log; mencatatnya sebagai pembuat menyesatkan penelusuran.
func TestExecuteApprovedRecordsMakerNotApprover(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerForTest(stubLimits{}, &stubApprovals{}, posting)

	approver := domain.Actor{UserID: uuid.New(), Username: "supervisor01", Role: domain.RoleSupervisor}
	err := svc.ExecuteApproved(context.Background(), nil, ActionDeposit, map[string]any{
		"account_number": "0011010000000014",
		"amount":         "25000000",
		"maker_username": "teller01",
	}, approver)
	if err != nil {
		t.Fatalf("eksekusi gagal: %v", err)
	}
	if posting.last.CreatedBy != "teller01" {
		t.Fatalf("created_by = %q, mau pembuat transaksi (teller01)", posting.last.CreatedBy)
	}
}

// Tanpa identitas pembuat pada payload (permintaan lama), pelaku yang menyetujui
// dipakai agar jurnal tetap punya jejak, bukan kosong.
func TestExecuteApprovedFallsBackToActingUser(t *testing.T) {
	posting := &stubPostingSvc{}
	svc := newLedgerForTest(stubLimits{}, &stubApprovals{}, posting)

	approver := domain.Actor{UserID: uuid.New(), Username: "supervisor01", Role: domain.RoleSupervisor}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionDeposit, map[string]any{
		"account_number": "0011010000000014",
		"amount":         "1000",
	}, approver); err != nil {
		t.Fatalf("eksekusi gagal: %v", err)
	}
	if posting.last.CreatedBy != "supervisor01" {
		t.Fatalf("created_by = %q, mau pelaku yang menyetujui", posting.last.CreatedBy)
	}
}
