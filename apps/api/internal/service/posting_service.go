package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// postingService adalah satu-satunya tempat jurnal ditulis. Semua modul memanggil
// Post/PostTx, tidak ada lagi insert journal_entries/journal_lines di service lain.
type postingService struct {
	db           *sql.DB
	postingRepo  domain.PostingRepository
	accountRepo  domain.AccountRepository
	resolver     domain.AccountResolver
	referenceGen domain.ReferenceGenerator
	dates        domain.BusinessDateRepository
}

func NewPostingService(
	db *sql.DB,
	postingRepo domain.PostingRepository,
	accountRepo domain.AccountRepository,
	resolver domain.AccountResolver,
	referenceGen domain.ReferenceGenerator,
	dates domain.BusinessDateRepository,
) domain.PostingService {
	return &postingService{
		db:           db,
		postingRepo:  postingRepo,
		accountRepo:  accountRepo,
		resolver:     resolver,
		referenceGen: referenceGen,
		dates:        dates,
	}
}

func (s *postingService) Post(ctx context.Context, req domain.PostingRequest) (*domain.JournalEntry, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	entry, err := s.PostTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit posting: %w", err)
	}
	return entry, nil
}

// PostTx memposting jurnal di dalam transaksi yang disediakan pemanggil, sehingga
// beberapa operasi bisnis bisa atomik dalam satu transaksi.
func (s *postingService) PostTx(ctx context.Context, tx any, req domain.PostingRequest) (*domain.JournalEntry, error) {
	sqlTx, ok := tx.(*sql.Tx)
	if !ok {
		return nil, errors.New("posting: transaksi tidak valid")
	}

	// Idempotency: bila kunci sudah pernah diproses, kembalikan jurnal yang lama.
	if req.IdempotencyKey != "" {
		existing, err := s.postingRepo.FindJournalByIdempotencyKey(ctx, req.IdempotencyKey)
		if err != nil {
			return nil, err
		}
		if existing != nil {
			return existing, nil
		}
	}

	journalID := uuid.New()
	now := time.Now().UTC()

	// Tanggal akuntansi entri. Nol berarti tanggal bisnis berjalan, bukan tanggal
	// kalender: bila tutup hari tertinggal, tanggal kalender jatuh di periode yang
	// belum dibuka sehingga jurnal terlewat oleh tutup hari dan buku cabang berbeda
	// dari kenyataan. Tanggal bisnis yang tidak terbaca menolak posting, bukan
	// menebak tanggal kalender.
	entryDate := req.EntryDate
	if entryDate.IsZero() {
		businessDate, err := s.businessDate(ctx)
		if err != nil {
			return nil, err
		}
		entryDate = businessDate
	}

	refNumber := req.ReferenceNumber
	if refNumber == "" {
		// Nomor diambil di dalam transaksi yang sama agar kegagalan sequence ikut
		// membatalkan jurnal, bukan menulis jurnal tanpa nomor.
		generated, err := s.referenceGen.NextTx(ctx, sqlTx, req.TransactionType, now)
		if err != nil {
			return nil, fmt.Errorf("membuat nomor referensi jurnal: %w", err)
		}
		refNumber = generated
	}

	var idemKeyPtr *string
	if req.IdempotencyKey != "" {
		idemKeyPtr = &req.IdempotencyKey
	}

	// Kunci semua akun yang terlibat (urut leksikografis) lalu hitung saldo baru.
	lines, err := s.buildLines(ctx, sqlTx, journalID, req, now)
	if err != nil {
		return nil, err
	}

	if err := domain.ValidateDoubleEntry(lines); err != nil {
		return nil, err
	}

	entry := &domain.JournalEntry{
		ID:              journalID,
		ReferenceNumber: refNumber,
		IdempotencyKey:  idemKeyPtr,
		TransactionType: req.TransactionType,
		Description:     req.Description,
		Status:          domain.JournalStatusPosted,
		PostedAt:        now,
		EntryDate:       entryDate,
		CreatedBy:       req.CreatedBy,
		BranchCode:      req.BranchCode,
		// Source wajib diteruskan: tanpa ini seluruh jurnal tersimpan tanpa penanda alur,
		// dan pembatalan transaksi menolak semuanya karena alur yang tidak dikenal memang
		// tidak boleh dibatalkan.
		Source:    req.Source,
		Lines:     lines,
		CreatedAt: now,
	}

	if err := s.postingRepo.InsertJournal(ctx, sqlTx, entry); err != nil {
		return nil, err
	}
	return entry, nil
}

// businessDate membaca tanggal bisnis berjalan dari repositori tanggal, bukan dari
// layanan konfigurasi yang nilainya bisa basi. Tanpa tanggal yang pasti, posting
// ditolak: menebak tanggal kalender berisiko menulis jurnal di periode yang belum
// dibuka dan membuat buku cabang berbeda dari kenyataan.
func (s *postingService) businessDate(ctx context.Context) (time.Time, error) {
	if s.dates == nil {
		return time.Time{}, errors.New("sumber tanggal bisnis belum terpasang")
	}
	current, err := s.dates.GetCurrentDate(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("membaca tanggal bisnis: %w", err)
	}
	if current == nil || current.CurrentDate.IsZero() {
		return time.Time{}, errors.New("tanggal bisnis tidak tersedia")
	}
	return current.CurrentDate, nil
}

// buildLines mengunci tiap akun, menghitung saldo baru berdasarkan normal balance
// COA, menulis perubahan saldo, dan mengembalikan baris jurnal.
func (s *postingService) buildLines(
	ctx context.Context,
	tx *sql.Tx,
	journalID uuid.UUID,
	req domain.PostingRequest,
	now time.Time,
) ([]domain.JournalLine, error) {
	if len(req.Lines) < 2 {
		return nil, errors.New("jurnal memerlukan minimal dua baris")
	}

	sorted := sortPostingLines(req.Lines)

	// Saldo berjalan per akun agar beberapa baris pada akun yang sama dihitung berurutan.
	running := make(map[string]*domain.Account)
	var lines []domain.JournalLine

	for i, pl := range sorted {
		acc, ok := running[pl.AccountNumber]
		if !ok {
			locked, err := s.accountRepo.GetByNumberForUpdate(ctx, tx, pl.AccountNumber)
			if err != nil {
				return nil, fmt.Errorf("akun %s tidak ditemukan: %w", pl.AccountNumber, err)
			}
			if err := statusGuardForDirection(pl.Direction, locked.Status); err != nil {
				return nil, fmt.Errorf("%w: akun %s", err, pl.AccountNumber)
			}
			running[pl.AccountNumber] = locked
			acc = locked
		}

		newBalance, err := applyDirection(acc.NormalBalance, acc.Balance, pl.Direction, pl.Amount)
		if err != nil {
			return nil, err
		}

		// Batas bawah saldo diperiksa SETELAH baris rekening dikunci, sehingga
		// transaksi paralel tidak dapat sama-sama lolos (lihat komentar domain).
		if domain.AccountBalanceFloorBreached(acc.AccountType, newBalance) {
			return nil, fmt.Errorf("%w: rekening %s tidak mencukupi", domain.ErrInsufficientFunds, pl.AccountNumber)
		}

		newAvailable := newBalance
		if err := s.accountRepo.UpdateBalance(ctx, tx, acc.ID, newBalance, newAvailable, acc.Version); err != nil {
			return nil, fmt.Errorf("update saldo akun %s: %w", pl.AccountNumber, err)
		}
		acc.Balance = newBalance
		acc.Version++

		lines = append(lines, domain.JournalLine{
			ID:             uuid.New(),
			JournalEntryID: journalID,
			AccountID:      acc.ID,
			AccountNumber:  acc.AccountNumber,
			Direction:      pl.Direction,
			Amount:         pl.Amount,
			Currency:       "IDR",
			BalanceAfter:   newBalance,
			Sequence:       i + 1,
			Description:    pl.Description,
			CreatedAt:      now,
		})
	}
	return lines, nil
}

// applyDirection menambah atau mengurangi saldo sesuai normal balance akun.
// Akun bersaldo normal DEBIT bertambah saat didebit; akun CREDIT bertambah saat dikredit.
// statusGuardForDirection memastikan status akun boleh menerima baris jurnal sesuai
// arahnya. Pemeriksaan ini wajib ada di mesin posting, bukan hanya di service
// pemanggil: mesin posting adalah satu-satunya pintu yang mengubah saldo, sehingga
// aturan yang hanya ditegakkan di pemanggil dapat dilewati jalur lain.
//
// Kredit boleh masuk ke rekening dormant agar dana pensiun/upah tidak tertahan;
// debit tetap ditolak. FROZEN dan CLOSED ditolak untuk kedua arah.
func statusGuardForDirection(direction domain.EntryDirection, status domain.AccountStatus) error {
	if direction == domain.DirectionDebit {
		return domain.AccountDebitAllowed(status)
	}
	return domain.AccountCreditAllowed(status)
}

func applyDirection(normal domain.BalanceType, balance decimal.Decimal, dir domain.EntryDirection, amount decimal.Decimal) (decimal.Decimal, error) {
	if normal == domain.BalanceTypeDebit {
		if dir == domain.DirectionDebit {
			return balance.Add(amount), nil
		}
		return balance.Sub(amount), nil
	}
	if dir == domain.DirectionCredit {
		return balance.Add(amount), nil
	}
	return balance.Sub(amount), nil
}

// sortPostingLines mengurutkan baris berdasarkan nomor akun agar kunci baris selalu
// diambil dalam urutan yang sama. Ini yang mencegah deadlock antar transaksi paralel.
func sortPostingLines(lines []domain.PostingLine) []domain.PostingLine {
	out := make([]domain.PostingLine, len(lines))
	copy(out, lines)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].AccountNumber > out[j].AccountNumber; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
