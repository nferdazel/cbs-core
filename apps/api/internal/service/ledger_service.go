package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kas default per buku. Teller konvensional memakai kas teller; transaksi syariah
// memakai kas syariah supaya arus kas kedua buku tidak tercampur di laporan.
//
// configCashCOA* adalah SATU-SATUNYA kunci konfigurasi akun kas teller per buku,
// dipakai bersama oleh jurnal transaksi rekening (ledger_service) dan penerimaan
// angsuran tunai (loan_service). Kunci lama payment.cash.coa.* (migrasi 000034) sudah
// dihapus migrasi 000073 karena menyimpan konsep yang sama dengan nilai berbeda-beda
// berpotensi mengarahkan dua alur ke akun kas yang berbeda.
const (
	configCashCOAConventional = "cash.coa.conventional"
	configCashCOASYariah      = "cash.coa.syariah"

	defaultCashCOAConventional = "10101"
	defaultCashCOASYariah      = "11100"
)

// ledgerService mengorkestrasi transaksi rekening. Penulisan jurnal dan perhitungan
// saldo diserahkan sepenuhnya ke posting engine; service ini hanya menentukan akun
// lawan dan aturan bisnisnya.
type ledgerService struct {
	db          *sql.DB
	ledgerRepo  domain.LedgerRepository
	accountRepo domain.AccountRepository
	productRepo domain.ProductRepository
	resolver    domain.AccountResolver
	posting     domain.PostingService
	configSvc   domain.SystemConfigService
	limits      domain.TransactionLimitService
	approvals   domain.MakerCheckerService
	auditRepo   domain.AuditRepository
	// txRunner membuka transaksi untuk pembatalan. Lewat interface agar jalurnya dapat
	// diuji tanpa database, sama seperti pemrosesan PPAP dan pembayaran angsuran.
	txRunner ppapTxRunner
	// dateRepo dibaca LANGSUNG, bukan lewat layanan konfigurasi yang menyimpan nilai di
	// cache. Tanggal bisnis berubah setiap tutup hari, sedangkan cache konfigurasi berlaku
	// 60 detik: selama jeda itu pembatalan lintas hari akan dinilai sebagai transaksi hari
	// berjalan dan lolos tanpa pejabat kedua.
	dateRepo domain.BusinessDateRepository
}

func NewLedgerService(
	db *sql.DB,
	ledgerRepo domain.LedgerRepository,
	accountRepo domain.AccountRepository,
	productRepo domain.ProductRepository,
	resolver domain.AccountResolver,
	posting domain.PostingService,
	configSvc domain.SystemConfigService,
	limits domain.TransactionLimitService,
	approvals domain.MakerCheckerService,
	dateRepo domain.BusinessDateRepository,
	auditSinks ...domain.AuditRepository,
) domain.LedgerService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &ledgerService{
		db:          db,
		ledgerRepo:  ledgerRepo,
		accountRepo: accountRepo,
		productRepo: productRepo,
		resolver:    resolver,
		posting:     posting,
		configSvc:   configSvc,
		limits:      limits,
		approvals:   approvals,
		auditRepo:   auditRepo,
		txRunner:    sqlPPAPTxRunner{db: db},
		dateRepo:    dateRepo,
	}
}

// Types jenis aksi maker-checker untuk transaksi rekening. Dipakai juga sebagai
// kunci pendaftaran eksekutor.
const (
	ActionDeposit  = "DEPOSIT"
	ActionWithdraw = "WITHDRAWAL"
	ActionTransfer = "TRANSFER"
	// ActionReverse adalah kunci audit untuk pembatalan transaksi.
	ActionReverse = "REVERSE_TRANSACTION"
)

// guardLimit menegakkan batas transaksi. Tiga kemungkinan:
//   - lolos,
//   - ditolak karena melebihi batas (dikembalikan sebagai error),
//   - melewati ambang persetujuan, sehingga transaksi TIDAK diposting dan masuk
//     antrean maker-checker; pemanggil menerima PendingApprovalError (HTTP 202).
//
// Aturan ini ditegakkan di service, bukan di handler, karena menyangkut kewenangan
// pejabat bank dan tidak boleh bisa dilewati dengan memanggil service langsung.
func (s *ledgerService) guardLimit(ctx context.Context, actor domain.Actor, action, txType string, amount decimal.Decimal, payload map[string]any) error {
	return guardTransactionLimit(ctx, s.limits, s.approvals, actor, action, txType, amount, payload)
}

// guardTransactionLimit menegakkan batas transaksi dan mengalihkan transaksi yang
// melewati ambang persetujuan ke antrean maker-checker. Dipisah dari ledgerService
// agar penempatan deposito memakai penjaga yang SAMA PERSIS: satu tempat aturan,
// satu perilaku. Tiga kemungkinan keluarannya:
//   - lolos,
//   - ditolak karena melebihi batas (dikembalikan sebagai error),
//   - melewati ambang persetujuan, sehingga transaksi TIDAK diposting dan masuk
//     antrean maker-checker; pemanggil menerima PendingApprovalError (HTTP 202).
//
// Aturan ini ditegakkan di service, bukan di handler, karena menyangkut kewenangan
// pejabat bank dan tidak boleh bisa dilewati dengan memanggil service langsung.
// limits/approvals boleh nil (mis. pada test): penjaga dilewati tanpa mengubah
// perilaku bisnis.
func guardTransactionLimit(ctx context.Context, limits domain.TransactionLimitService, approvals domain.MakerCheckerService, actor domain.Actor, action, txType string, amount decimal.Decimal, payload map[string]any) error {
	if limits == nil {
		return nil
	}

	err := limits.Check(ctx, actor, txType, amount)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrRequiresApproval) {
		return err
	}
	if approvals == nil {
		return err
	}

	req, createErr := approvals.CreateRequest(ctx, domain.CreateMakerCheckerInput{
		ActionType: action,
		Amount:     amount,
		Payload:    payload,
		Notes:      "Nominal melewati ambang persetujuan pejabat",
	}, actor)
	if createErr != nil {
		return createErr
	}
	return &domain.PendingApprovalError{RequestID: req.ID, ActionType: action}
}

// ExecuteApproved mengeksekusi transaksi rekening yang sudah disetujui maker-checker.
// Dipanggil di dalam transaksi milik maker-checker service. Batas per transaksi dan
// ambang persetujuan TIDAK diperiksa ulang karena persetujuan itu sendiri yang menjadi
// kewenangannya; batas harian tetap dievaluasi ulang (lihat guardApprovedDaily) karena
// persetujuan tidak boleh melegalkannya.
func (s *ledgerService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	if payload == nil {
		return errors.New("payload persetujuan kosong")
	}
	accountNumber, _ := payload["account_number"].(string)
	amount, err := decimalFromPayload(payload["amount"])
	if err != nil {
		return err
	}
	description, _ := payload["description"].(string)
	idempotencyKey, _ := payload["idempotency_key"].(string)
	currency, _ := payload["currency"].(string)
	if currency == "" {
		currency = "IDR"
	}

	// Jurnal mencatat PEMBUAT transaksi, bukan pemeriksa. Peran pemeriksa tercatat
	// di audit log; mencatatnya sebagai pembuat akan menyesatkan penelusuran.
	createdBy := actor.DisplayName()
	if maker, ok := payload["maker_username"].(string); ok && maker != "" {
		createdBy = maker
	}

	// Batas harian dievaluasi ULANG saat pengajuan dieksekusi. Persetujuan pejabat
	// melegalkan nominal di atas ambang dan per transaksi, TETAPI tidak boleh melegalkan
	// pelanggaran batas harian: N pengajuan yang masing-masing di bawah batas dapat
	// disetujui semua sehingga total terposting hari itu melampaui batas. Bila sudah
	// melampaui, eksekusi gagal dengan galat jelas dan pengajuan tetap PENDING sehingga
	// dapat ditolak/ditinjau, bukan diam-diam jalan.
	if err := s.guardApprovedDaily(ctx, tx, actionType, payload, amount, actor); err != nil {
		return err
	}

	switch normalizeAction(actionType) {
	case ActionReverse:
		reference, _ := payload["reference"].(string)
		reason, _ := payload["reason"].(string)
		maker, _ := payload["maker_username"].(string)
		if reference == "" {
			return errors.New("referensi transaksi tidak ada pada payload persetujuan")
		}
		if maker == "" {
			maker = actor.DisplayName()
		}
		original, err := s.guardReversible(ctx, reference, actor, maker)
		if err != nil {
			return err
		}
		// Jurnal kontra ditulis di transaksi maker-checker: persetujuan dan
		// pembatalannya harus berhasil atau gagal bersama.
		_, err = s.postReversalTx(ctx, tx, original, reason, maker, actor)
		return err
	case ActionDeposit:
		return s.postDepositTx(ctx, tx, accountNumber, amount, currency, description, idempotencyKey, createdBy, actor)
	case ActionWithdraw:
		return s.postWithdrawTx(ctx, tx, accountNumber, amount, currency, description, idempotencyKey, createdBy, actor)
	case ActionTransfer:
		source, _ := payload["source_account_number"].(string)
		destination, _ := payload["destination_account_number"].(string)
		return s.postTransferTx(ctx, tx, source, destination, amount, currency, description, idempotencyKey, createdBy, actor)
	default:
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
}

// approvedLimitTxType memetakan jenis aksi maker-checker rekening ke jenis transaksi
// batas (sufiks limit.<peran>.<jenis>). Aksi tanpa batas harian (mis. pembatalan
// REVERSE_TRANSACTION, yang nominalnya diambil dari jurnal asal) mengembalikan "".
func approvedLimitTxType(actionType string) string {
	switch normalizeAction(actionType) {
	case ActionDeposit:
		return "deposit"
	case ActionWithdraw:
		return "withdrawal"
	case ActionTransfer:
		return "transfer"
	default:
		return ""
	}
}

// guardApprovedDaily mengevaluasi ulang batas harian untuk pengajuan rekening yang
// sudah disetujui. Pembuat permintaan dipulihkan dari payload (bukan pejabat yang
// menyetujui) karena batas dihitung per pembuat. Batas per transaksi dan ambang
// persetujuan tidak diperiksa ulang: persetujuan pejabat justru menjadi kewenangan
// untuk melewatinya. tx diteruskan agar evaluasi batas dikunci secara serial di dalam
// transaksi eksekusi yang sama. Untuk permintaan lama yang payload-nya belum memuat
// maker_role, peran jatuh ke pemeriksa (perilaku kompatibilitas); pengajuan baru selalu
// memuatnya.
func (s *ledgerService) guardApprovedDaily(ctx context.Context, tx any, actionType string, payload map[string]any, amount decimal.Decimal, actor domain.Actor) error {
	if s.limits == nil {
		return nil
	}
	txType := approvedLimitTxType(actionType)
	if txType == "" {
		return nil
	}
	if err := s.limits.CheckDailyAtExecution(ctx, tx, makerFromPayload(payload, actor), txType, amount); err != nil {
		return fmt.Errorf("pengajuan yang disetujui tidak dapat dieksekusi: %w", err)
	}
	return nil
}

// decimalFromPayload membaca nominal dari payload JSON, yang bisa berupa string
// (hasil .String()) atau angka.
func decimalFromPayload(raw any) (decimal.Decimal, error) {
	switch v := raw.(type) {
	case string:
		d, err := decimal.NewFromString(v)
		if err != nil {
			return decimal.Zero, fmt.Errorf("nominal tidak valid: %w", err)
		}
		return d, nil
	case float64:
		return decimal.NewFromFloat(v), nil
	case decimal.Decimal:
		return v, nil
	default:
		return decimal.Zero, errors.New("nominal tidak ada pada payload persetujuan")
	}
}

func (s *ledgerService) Deposit(ctx context.Context, req domain.DepositRequest) (*domain.JournalEntry, error) {
	if err := s.validateAmount(req.Amount); err != nil {
		return nil, err
	}

	acc, err := s.loadCreditAccount(ctx, req.AccountNumber)
	if err != nil {
		return nil, err
	}
	// Penegakan kepemilikan cabang; rekening tanpa cabang (data pra-migrasi)
	// dibiarkan agar operasional tidak terblokir.
	if !req.Actor.CanAccessBranch(acc.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Buku rekening ditegakkan sebelum jurnal disusun: teller satu buku tidak boleh
	// menyetor ke rekening buku lain. Rekening tanpa buku (data pra-migrasi) lolos.
	if !req.Actor.CanAccessBook(acc.COABook) {
		return nil, domain.ErrCrossBookAccess
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

	if err := s.guardLimit(ctx, req.Actor, ActionDeposit, "deposit", req.Amount, map[string]any{
		"account_number":  req.AccountNumber,
		"amount":          req.Amount.String(),
		"currency":        req.Currency,
		"description":     req.Description,
		"idempotency_key": req.IdempotencyKey,
		"maker_username":  req.Actor.DisplayName(),
	}); err != nil {
		return nil, err
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeDeposit,
		Source:          domain.SourceTeller,
		Description:     defaultDescription(req.Description, "Setoran tunai "+acc.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       actorName(req.Actor, req.CreatedBy),
		BranchCode:      acc.BranchCode,
		Lines: []domain.PostingLine{
			{AccountNumber: cashAccount, Direction: domain.DirectionDebit, Amount: req.Amount, Description: "Kas masuk"},
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionCredit, Amount: req.Amount, Description: req.Description},
		},
	})
}

func (s *ledgerService) Withdraw(ctx context.Context, req domain.WithdrawRequest) (*domain.JournalEntry, error) {
	if err := s.validateAmount(req.Amount); err != nil {
		return nil, err
	}

	acc, err := s.loadDebitAccount(ctx, req.AccountNumber)
	if err != nil {
		return nil, err
	}
	// Penegakan kepemilikan cabang; rekening tanpa cabang (data pra-migrasi)
	// dibiarkan agar pencairan data lama tidak terblokir.
	if !req.Actor.CanAccessBranch(acc.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Buku rekening ditegakkan sebelum jurnal disusun: teller satu buku tidak boleh
	// menarik dari rekening buku lain. Rekening tanpa buku (data pra-migrasi) lolos.
	if !req.Actor.CanAccessBook(acc.COABook) {
		return nil, domain.ErrCrossBookAccess
	}
	if acc.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return nil, err
	}

	if err := s.guardLimit(ctx, req.Actor, ActionWithdraw, "withdrawal", req.Amount, map[string]any{
		"account_number":  req.AccountNumber,
		"amount":          req.Amount.String(),
		"currency":        req.Currency,
		"description":     req.Description,
		"idempotency_key": req.IdempotencyKey,
		"maker_username":  req.Actor.DisplayName(),
	}); err != nil {
		return nil, err
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Source:          domain.SourceTeller,
		Description:     defaultDescription(req.Description, "Penarikan tunai "+acc.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       actorName(req.Actor, req.CreatedBy),
		BranchCode:      acc.BranchCode,
		Lines: []domain.PostingLine{
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionDebit, Amount: req.Amount, Description: req.Description},
			{AccountNumber: cashAccount, Direction: domain.DirectionCredit, Amount: req.Amount, Description: "Kas keluar"},
		},
	})
}

func (s *ledgerService) TransferInternal(ctx context.Context, req domain.TransferRequest) (*domain.JournalEntry, error) {
	if err := s.validateAmount(req.Amount); err != nil {
		return nil, err
	}
	if req.SourceAccountNumber == req.DestinationAccountNumber {
		return nil, domain.ErrSameAccountTransfer
	}

	src, err := s.loadDebitAccount(ctx, req.SourceAccountNumber)
	if err != nil {
		return nil, err
	}
	dest, err := s.loadCreditAccount(ctx, req.DestinationAccountNumber)
	if err != nil {
		return nil, err
	}
	// Transfer menyentuh dua rekening: keduanya harus berada di cabang aktor.
	// Rekening tanpa cabang (data pra-migrasi) dibiarkan agar tidak memblokir.
	if !req.Actor.CanAccessBranch(src.BranchCode) || !req.Actor.CanAccessBranch(dest.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	// Transfer menyentuh dua rekening: keduanya harus berada di buku aktor. Transfer
	// lintas buku ditolak, termasuk oleh aktor satu buku (aktor lintas buku lolos).
	if !req.Actor.CanAccessBook(src.COABook) || !req.Actor.CanAccessBook(dest.COABook) {
		return nil, domain.ErrCrossBookAccess
	}
	if src.AvailableBalance.LessThan(req.Amount) {
		return nil, domain.ErrInsufficientFunds
	}

	if err := s.guardLimit(ctx, req.Actor, ActionTransfer, "transfer", req.Amount, map[string]any{
		"source_account_number":      req.SourceAccountNumber,
		"destination_account_number": req.DestinationAccountNumber,
		"account_number":             req.SourceAccountNumber,
		"amount":                     req.Amount.String(),
		"currency":                   req.Currency,
		"description":                req.Description,
		"idempotency_key":            req.IdempotencyKey,
		"maker_username":             req.Actor.DisplayName(),
	}); err != nil {
		return nil, err
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: domain.TxTypeTransferInternal,
		Source:          domain.SourceTeller,
		Description:     defaultDescription(req.Description, "Transfer "+src.AccountNumber+" ke "+dest.AccountNumber),
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       actorName(req.Actor, req.CreatedBy),
		// Transfer menyentuh dua rekening; jurnal diatribusikan ke cabang rekening
		// asal secara deterministik. Transfer lintas cabang hanya mungkin dilakukan
		// aktor lintas cabang, dan sumber dana menandai cabang transaksi.
		BranchCode: src.BranchCode,
		Lines: []domain.PostingLine{
			{AccountNumber: src.AccountNumber, Direction: domain.DirectionDebit, Amount: req.Amount, Description: "Transfer keluar"},
			{AccountNumber: dest.AccountNumber, Direction: domain.DirectionCredit, Amount: req.Amount, Description: "Transfer masuk"},
		},
	})
}

// postDepositTx, postWithdrawTx, dan postTransferTx dipakai eksekusi persetujuan:
// jurnal ditulis pada transaksi pemanggil, bukan membuka transaksi baru.
func (s *ledgerService) postDepositTx(ctx context.Context, tx any, accountNumber string, amount decimal.Decimal, currency, description, idempotencyKey, createdBy string, actor domain.Actor) error {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return err
	}
	if err := domain.AccountCreditAllowed(acc.Status); err != nil {
		return err
	}
	// Jalur persetujuan tidak melewati Deposit, jadi penegakan buku diulang di sini.
	// Pemeriksa satu buku tidak boleh mengeksekusi setoran ke rekening buku lain.
	if !actor.CanAccessBook(acc.COABook) {
		return domain.ErrCrossBookAccess
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return err
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeDeposit,
		Source:          domain.SourceTeller,
		Description:     defaultDescription(description, "Setoran tunai "+acc.AccountNumber),
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       createdBy,
		BranchCode:      acc.BranchCode,
		Lines: []domain.PostingLine{
			{AccountNumber: cashAccount, Direction: domain.DirectionDebit, Amount: amount, Description: "Kas masuk"},
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionCredit, Amount: amount, Description: description},
		},
	})
	return err
}

func (s *ledgerService) postWithdrawTx(ctx context.Context, tx any, accountNumber string, amount decimal.Decimal, currency, description, idempotencyKey, createdBy string, actor domain.Actor) error {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return err
	}
	if err := domain.AccountDebitAllowed(acc.Status); err != nil {
		return err
	}
	// Jalur persetujuan tidak melewati Withdraw, jadi penegakan buku diulang di sini.
	if !actor.CanAccessBook(acc.COABook) {
		return domain.ErrCrossBookAccess
	}
	if acc.AvailableBalance.LessThan(amount) {
		return domain.ErrInsufficientFunds
	}
	cashAccount, err := s.resolveCashAccount(ctx, acc)
	if err != nil {
		return err
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeWithdrawal,
		Source:          domain.SourceTeller,
		Description:     defaultDescription(description, "Penarikan tunai "+acc.AccountNumber),
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       createdBy,
		BranchCode:      acc.BranchCode,
		Lines: []domain.PostingLine{
			{AccountNumber: acc.AccountNumber, Direction: domain.DirectionDebit, Amount: amount, Description: description},
			{AccountNumber: cashAccount, Direction: domain.DirectionCredit, Amount: amount, Description: "Kas keluar"},
		},
	})
	return err
}

func (s *ledgerService) postTransferTx(ctx context.Context, tx any, source, destination string, amount decimal.Decimal, currency, description, idempotencyKey, createdBy string, actor domain.Actor) error {
	if source == destination {
		return domain.ErrSameAccountTransfer
	}
	src, err := s.accountRepo.GetByNumber(ctx, source)
	if err != nil {
		return err
	}
	dest, err := s.accountRepo.GetByNumber(ctx, destination)
	if err != nil {
		return err
	}
	// Jalur persetujuan tidak melewati TransferInternal, jadi penegakan buku diulang:
	// kedua rekening harus berada di buku pemeriksa.
	if !actor.CanAccessBook(src.COABook) || !actor.CanAccessBook(dest.COABook) {
		return domain.ErrCrossBookAccess
	}
	// Transfer menyentuh dua rekening: sumber wajib ACTIVE (dana keluar), tujuan
	// boleh ACTIVE atau DORMANT (dana masuk tidak boleh tertahan).
	if err := domain.AccountDebitAllowed(src.Status); err != nil {
		return err
	}
	if err := domain.AccountCreditAllowed(dest.Status); err != nil {
		return err
	}
	if src.AvailableBalance.LessThan(amount) {
		return domain.ErrInsufficientFunds
	}

	_, err = s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeTransferInternal,
		Source:          domain.SourceTeller,
		Description:     defaultDescription(description, "Transfer "+src.AccountNumber+" ke "+dest.AccountNumber),
		IdempotencyKey:  idempotencyKey,
		CreatedBy:       createdBy,
		BranchCode:      src.BranchCode,
		Lines: []domain.PostingLine{
			{AccountNumber: src.AccountNumber, Direction: domain.DirectionDebit, Amount: amount, Description: "Transfer keluar"},
			{AccountNumber: dest.AccountNumber, Direction: domain.DirectionCredit, Amount: amount, Description: "Transfer masuk"},
		},
	})
	return err
}

// validateAmount memastikan nominal positif dan mata uang terisi.
func (s *ledgerService) validateAmount(amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.ErrInvalidAmount
	}
	return nil
}

// loadDebitAccount mengambil rekening dan memastikan rekening boleh didebit.
// Rekening dormant ditolak ErrAccountDormant; FROZEN/CLOSED tetap ErrAccountInactive.
func (s *ledgerService) loadDebitAccount(ctx context.Context, accountNumber string) (*domain.Account, error) {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	if err := domain.AccountDebitAllowed(acc.Status); err != nil {
		return nil, err
	}
	return acc, nil
}

// loadCreditAccount mengambil rekening dan memastikan rekening boleh dikredit.
// Rekening ACTIVE maupun DORMANT diterima; FROZEN/CLOSED ditolak ErrAccountInactive.
func (s *ledgerService) loadCreditAccount(ctx context.Context, accountNumber string) (*domain.Account, error) {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}
	if err := domain.AccountCreditAllowed(acc.Status); err != nil {
		return nil, err
	}
	return acc, nil
}

// actorName memakai identitas dari JWT; CreatedBy hanya dipertahankan sebagai
// kompatibilitas pemanggil lama dan tidak boleh menjadi sumber identitas.
func actorName(actor domain.Actor, fallback string) string {
	if actor.UserID != uuid.Nil || actor.Username != "" {
		return actor.DisplayName()
	}
	return fallback
}

// Reverse membatalkan jurnal yang sudah diposting dengan jurnal kontra.
//
// Aturan yang ditegakkan di sini:
//   - Jurnal kontra bertanggal bisnis BERJALAN, bukan tanggal jurnal asal. Periode asal
//     sudah membentuk laporan, PPAP, dan dasar pajak; menyisipkan entri ke sana mengubah
//     angka yang mungkin sudah dilaporkan. Jejak koreksinya ada di deskripsi jurnal.
//   - Nominal diambil dari jurnal asal, tidak dari permintaan, sehingga pembatalan tidak
//     dapat dipakai memindahkan dana dengan jumlah karangan. Karena itu pula batas
//     transaksi tidak diperiksa ulang: tidak ada nominal yang bisa dipilih pemanggil.
//   - Hanya jurnal POSTED, dan jurnal pembatalan tidak dapat dibatalkan lagi.
//   - Pencatat transaksi asal tidak boleh membatalkannya sendiri.
//   - Hanya untuk transaksi yang tidak menyalakan state domain (setoran, penarikan,
//     transfer, biaya). Jurnal yang mengiringi jadwal angsuran, pencairan kredit, atau
//     pencairan deposito tidak boleh dibatalkan di sini: jurnal dan data domainnya akan
//     tidak sinkron. Koreksinya harus lewat fitur domain masing-masing.
func (s *ledgerService) Reverse(ctx context.Context, req domain.ReversalRequest) (*domain.JournalEntry, error) {
	original, err := s.guardReversible(ctx, req.Reference, req.Actor, req.Actor.DisplayName())
	if err != nil {
		return nil, err
	}

	// Lintas hari berarti mengubah angka yang sudah masuk laporan tutup buku, jadi
	// pembatalannya wajib lewat pejabat kedua meskipun pelakunya sudah supervisor.
	// Transaksi hari berjalan masih operasional dan cukup disetujui supervisor.
	if s.isCrossDate(ctx, original) {
		if s.approvals == nil {
			return nil, fmt.Errorf("%w: pembatalan lintas hari memerlukan persetujuan pejabat kedua, tetapi layanan persetujuan tidak tersedia", domain.ErrReversalNotAllowed)
		}
		approval, err := s.approvals.CreateRequest(ctx, domain.CreateMakerCheckerInput{
			ActionType: ActionReverse,
			Amount:     debitTotal(original),
			Payload: map[string]any{
				"reference": original.ReferenceNumber,
				"reason":    req.Reason,
				// Nominal disertakan agar pejabat penyetuju melihat nilai yang
				// dibatalkan; nilai yang benar-benar dipakai tetap dihitung dari jurnal
				// asal, bukan dari payload ini.
				"amount":         debitTotal(original).StringFixed(2),
				"maker_username": req.Actor.DisplayName(),
			},
			Notes: "Pembatalan transaksi lintas hari",
		}, req.Actor)
		if err != nil {
			return nil, err
		}
		return nil, &domain.PendingApprovalError{RequestID: approval.ID, ActionType: ActionReverse}
	}

	var entry *domain.JournalEntry
	err = s.txRunner.Run(ctx, func(tx any) error {
		posted, err := s.postReversalTx(ctx, tx, original, req.Reason, req.Actor.DisplayName(), req.Actor)
		if err != nil {
			return err
		}
		entry = posted
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// guardReversible memuat jurnal asal dan menegakkan seluruh syarat pembatalan.
// requesterName adalah pencatat permintaan; pada jalur persetujuan yang diperiksa
// sebagai pencatat transaksi asal adalah pembuat permintaan, bukan pejabat penyetuju.
func (s *ledgerService) guardReversible(ctx context.Context, reference string, actor domain.Actor, requesterName string) (*domain.JournalEntry, error) {
	original, err := s.ledgerRepo.GetJournalByRef(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", domain.ErrJournalNotFound, reference)
	}
	// Cabang lain dilaporkan sebagai tidak ditemukan: membedakannya memberi tahu
	// pemanggil bahwa referensi tertentu memang ada di bank ini.
	if !actor.CanAccessBranch(original.BranchCode) {
		return nil, fmt.Errorf("%w: %s", domain.ErrJournalNotFound, reference)
	}
	// Buku jurnal diturunkan dari akun barisnya (original.Book, original.BookMixed).
	// Pembatalan jurnal buku lain ditolak tegas: jurnal kontra adalah tulisan lintas
	// buku bila lolos.
	if !actor.CanAccessJournal(original.Book, original.BookMixed) {
		return nil, domain.ErrCrossBookAccess
	}

	switch original.Status {
	case domain.JournalStatusPosted:
	case domain.JournalStatusReversed:
		return nil, fmt.Errorf("%w: %s", domain.ErrJournalAlreadyReversed, original.ReferenceNumber)
	default:
		return nil, fmt.Errorf("%w (status %s)", domain.ErrJournalNotPosted, original.Status)
	}
	if original.TransactionType == domain.TxTypeReversal {
		return nil, fmt.Errorf("%w: %s", domain.ErrReversalNotAllowed, original.ReferenceNumber)
	}
	// Akrual dan penyesuaian selalu membawa state domain atau periode: jurnal kontra
	// tidak mengembalikan bunga yang sudah diakru, jadwal yang sudah dibentuk, atau
	// periode yang sudah ditutup.
	switch original.TransactionType {
	case domain.TxTypeInterestAccrual, domain.TxTypeAdjustment:
		return nil, fmt.Errorf("%w: jurnal %s tidak dapat dibatalkan lewat jalur ini",
			domain.ErrReversalNotAllowed, original.TransactionType)
	}
	if createdBy := strings.ToUpper(strings.TrimSpace(original.CreatedBy)); createdBy == "" || createdBy == "SYSTEM" {
		return nil, fmt.Errorf("%w: jurnal sistem", domain.ErrReversalNotAllowed)
	}
	// Penegakan cakupan memakai penanda alur, bukan transaction_type: DEPOSIT dan
	// WITHDRAWAL dipakai bersama oleh transaksi teller dan transaksi deposito berjangka,
	// sehingga daftar putih atas label akan meloloskan justru jurnal yang membawa state
	// domain — deposito berstatus CLOSED, jadwal angsuran, atau akrual. Jurnal tanpa
	// penanda alur juga ditolak: alur yang belum dikenal tidak boleh dibatalkan.
	if original.Source != domain.SourceTeller {
		source := string(original.Source)
		if source == "" {
			source = "tidak bertanda"
		}
		return nil, fmt.Errorf("%w: alur jurnal %s bukan transaksi teller", domain.ErrReversalNotAllowed, source)
	}
	if maker := original.CreatedBy; maker != "" && (maker == requesterName || maker == actor.Username) {
		return nil, fmt.Errorf("%w: %s", domain.ErrSelfReversal, original.ReferenceNumber)
	}
	if len(original.Lines) == 0 {
		return nil, fmt.Errorf("jurnal %s tidak memiliki baris untuk dibalik", original.ReferenceNumber)
	}
	return original, nil
}

// postReversalTx menulis jurnal kontra, menandai jurnal asal, dan menulis audit di dalam
// transaksi pemanggil. Dipakai jalur langsung maupun jalur persetujuan, sehingga
// keduanya tidak dapat berbeda perilaku.
func (s *ledgerService) postReversalTx(ctx context.Context, tx any, original *domain.JournalEntry, reason, createdBy string, actor domain.Actor) (*domain.JournalEntry, error) {
	// Kontra: arah ditukar, nominal tetap penuh. Tidak ada pembatalan sebagian —
	// koreksi sebagian adalah transaksi baru, bukan pembatalan.
	lines := make([]domain.PostingLine, 0, len(original.Lines))
	for _, line := range original.Lines {
		if line.AccountNumber == "" {
			return nil, fmt.Errorf("baris jurnal %s tidak memiliki nomor rekening", line.ID)
		}
		direction := domain.DirectionDebit
		if line.Direction == domain.DirectionDebit {
			direction = domain.DirectionCredit
		}
		lines = append(lines, domain.PostingLine{
			AccountNumber: line.AccountNumber,
			Direction:     direction,
			Amount:        line.Amount,
			Description:   "Pembatalan " + original.ReferenceNumber,
		})
	}
	total := debitTotal(original)
	if total.IsZero() {
		return nil, fmt.Errorf("jurnal %s tidak memiliki sisi debit", original.ReferenceNumber)
	}

	// Tanggal bisnis berjalan, bukan tanggal kalender. Pembatalan yang jatuh di periode
	// yang belum dibuka tidak akan ikut terhitung tutup hari, sehingga buku cabang
	// berbeda dari kenyataan sampai tutup hari menyusul.
	var entryDate time.Time
	if s.dateRepo == nil {
		return nil, errors.New("sumber tanggal bisnis belum terpasang")
	}
	current, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca tanggal bisnis: %w", err)
	}
	if current == nil || current.CurrentDate.IsZero() {
		return nil, errors.New("tanggal bisnis tidak tersedia")
	}
	entryDate = current.CurrentDate

	posted, err := s.posting.PostTx(ctx, tx, domain.PostingRequest{
		TransactionType: domain.TxTypeReversal,
		EntryDate:       entryDate,
		Description:     fmt.Sprintf("Pembatalan %s — %s", original.ReferenceNumber, reason),
		// Kunci tetap per jurnal asal: permintaan ulang mengembalikan jurnal kontra yang
		// sama, bukan membuat pembalikan kedua.
		IdempotencyKey: "REV-" + original.ReferenceNumber,
		CreatedBy:      createdBy,
		// Cabang mengikuti jurnal asal. Tanpa ini jurnal kontra tersimpan dengan
		// branch_id NULL, yaitu jurnal bank-wide, sehingga laporan per cabang dan
		// pemeriksaan cabang atas jurnal itu tidak lagi mencerminkan asalnya.
		BranchCode: original.BranchCode,
		Source:     domain.SourceTeller,
		Lines:      lines,
	})
	if err != nil {
		return nil, err
	}

	// Penandaan status asal berada di transaksi yang sama dengan jurnal kontranya.
	// Terpisah, kegagalan di antaranya meninggalkan jurnal yang masih POSTED padahal
	// sudah ada kontranya, dan pembalikan kedua bisa lolos.
	if err := s.ledgerRepo.MarkJournalReversed(ctx, tx, original.ReferenceNumber); err != nil {
		return nil, err
	}

	// Pembatalan tanpa catatan siapa dan mengapa adalah perubahan angka yang tidak
	// dapat dipertanggungjawabkan.
	if err := writeAudit(ctx, s.auditRepo, tx, actor, ActionReverse, "JOURNAL", original.ReferenceNumber, map[string]any{
		"reversal_reference": posted.ReferenceNumber,
		"reason":             reason,
		"amount":             total.StringFixed(2),
		"transaction_type":   string(original.TransactionType),
		"created_by":         createdBy,
	}); err != nil {
		return nil, err
	}
	return posted, nil
}

// isCrossDate melaporkan apakah jurnal asal berasal dari tanggal bisnis sebelum hari ini.
// Bila tanggal bisnis tidak dapat dibaca, jawabannya ya: pembatalan yang tidak dapat
// dipastikan berasal dari hari berjalan diperlakukan sebagai lintas hari.
func (s *ledgerService) isCrossDate(ctx context.Context, original *domain.JournalEntry) bool {
	if original.EntryDate.IsZero() || s.dateRepo == nil {
		return true
	}
	current, err := s.dateRepo.GetCurrentDate(ctx)
	if err != nil || current == nil {
		return true
	}
	by, bm, bd := current.CurrentDate.Date()
	ey, em, ed := original.EntryDate.Date()
	return by != ey || bm != em || bd != ed
}

// debitTotal menjumlahkan sisi debit jurnal, yaitu nominal yang dipindahkan.
func debitTotal(original *domain.JournalEntry) decimal.Decimal {
	total := decimal.Zero
	for _, line := range original.Lines {
		if line.Direction == domain.DirectionDebit {
			total = total.Add(line.Amount)
		}
	}
	return total
}

// PostCompoundJournal memposting jurnal majemuk yang disusun operator. Posting engine
// yang menghitung saldo dan menegakkan keseimbangan (domain.ValidateDoubleEntry);
// service ini tidak menebak sifat saldo dari nomor akun. Izin pemanggilnya adalah
// pengelolaan data master akuntansi (coa:manage) karena menyentuh buku besar di luar
// alur domain.
func (s *ledgerService) PostCompoundJournal(ctx context.Context, req domain.CustomJournalRequest) (*domain.JournalEntry, error) {
	if len(req.Lines) < 2 {
		return nil, fmt.Errorf("jurnal majemuk memerlukan minimal dua baris")
	}

	lines := make([]domain.PostingLine, 0, len(req.Lines))
	for _, l := range req.Lines {
		if l.Amount.LessThanOrEqual(decimal.Zero) {
			return nil, domain.ErrInvalidAmount
		}
		lines = append(lines, domain.PostingLine(l))
	}

	return s.posting.Post(ctx, domain.PostingRequest{
		TransactionType: req.TransactionType,
		Description:     req.Description,
		IdempotencyKey:  req.IdempotencyKey,
		CreatedBy:       req.CreatedBy,
		Lines:           lines,
	})
}

// resolveCashAccount menentukan akun kas lawan berdasarkan buku produk rekening.
// Syariah memakai kas syariah agar laporan arus kas kedua buku tetap terpisah.
func (s *ledgerService) resolveCashAccount(ctx context.Context, acc *domain.Account) (string, error) {
	coaCode := defaultCashCOAConventional
	if s.isSyariahAccount(ctx, acc) {
		coaCode = defaultCashCOASYariah
	}
	if s.configSvc != nil {
		key := configCashCOAConventional
		if coaCode == defaultCashCOASYariah {
			key = configCashCOASYariah
		}
		coaCode = s.configSvc.GetString(ctx, key, coaCode)
	}
	return s.resolver.ResolveGLAccount(ctx, nil, coaCode)
}

// isSyariahAccount menentukan buku rekening dari produknya, dan dari buku COA
// rekening sebagai cadangan bila produk tidak ada atau tidak terbaca. Rekening lama
// (tanpa produk) tetap punya COA ber-book, sehingga sifat syariahnya tidak lagi
// menghilang; sebaliknya data yang produknya ada tetapi bukunya kosong tetap dinilai
// dari produk.
func (s *ledgerService) isSyariahAccount(ctx context.Context, acc *domain.Account) bool {
	if acc.ProductID != nil && s.productRepo != nil {
		product, err := s.productRepo.GetByID(ctx, *acc.ProductID)
		if err == nil {
			return product.Book == domain.BookSyariah
		}
	}
	return acc.COABook == domain.BookSyariah
}

func (s *ledgerService) GetJournalByReference(ctx context.Context, ref string) (*domain.JournalEntry, error) {
	return s.ledgerRepo.GetJournalByRef(ctx, ref)
}

func (s *ledgerService) ListJournals(ctx context.Context, page, pageSize int, actor domain.Actor) ([]domain.JournalEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.ledgerRepo.ListJournals(ctx, pageSize, offset, actor)
}

func (s *ledgerService) GetAccountStatement(ctx context.Context, accountNumber string, page, pageSize int, actor domain.Actor) ([]domain.JournalLine, int, error) {
	acc, err := s.accountRepo.GetByNumber(ctx, accountNumber)
	if err != nil {
		return nil, 0, err
	}

	// Mutasi rekening cabang lain ditolak 403 lewat Fail (batas keamanan), bukan
	// dikembalikan sebagai daftar kosong yang menyamarkan penolakan sebagai
	// "tidak ada mutasi". Berbeda dari detail rekening, yang menyamarkan baca
	// lintas cabang sebagai 404 agar keberadaan rekening tidak bocor.
	if !actor.CanAccessBranch(acc.BranchCode) {
		return nil, 0, domain.ErrCrossBranchAccess
	}
	// Mutasi rekening buku lain ditolak dengan aturan batas keamanan yang sama.
	if !actor.CanAccessBook(acc.COABook) {
		return nil, 0, domain.ErrCrossBookAccess
	}

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize
	return s.ledgerRepo.ListAccountStatements(ctx, acc.ID, pageSize, offset, actor)
}

func (s *ledgerService) GetChartOfAccounts(ctx context.Context) ([]domain.ChartOfAccount, error) {
	return s.ledgerRepo.GetCOAList(ctx)
}

func defaultDescription(desc, fallback string) string {
	if desc != "" {
		return desc
	}
	return fallback
}
