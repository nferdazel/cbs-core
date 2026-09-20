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

// reversalLedgerRepo melayani pembacaan jurnal dan mencatat penandaan status asal,
// tanpa menyentuh database.
type reversalLedgerRepo struct {
	domain.LedgerRepository
	entry     *domain.JournalEntry
	entryErr  error
	markErr   error
	markedRef []string
}

func (r *reversalLedgerRepo) GetJournalByRef(ctx context.Context, ref string) (*domain.JournalEntry, error) {
	if r.entryErr != nil {
		return nil, r.entryErr
	}
	if r.entry == nil {
		return nil, errors.New("journal entry not found")
	}
	return r.entry, nil
}

func (r *reversalLedgerRepo) MarkJournalReversed(ctx context.Context, tx any, reference string) error {
	if r.markErr != nil {
		return r.markErr
	}
	r.markedRef = append(r.markedRef, reference)
	return nil
}

// reversalTxRunner menjalankan fn tanpa database dan menyimpan kesalahannya. Dipakai
// untuk memastikan kegagalan di tengah memang menggagalkan transaksi (rollback), bukan
// hanya diteruskan sebagai respons.
type reversalTxRunner struct{ err error }

func (r *reversalTxRunner) Run(ctx context.Context, fn func(tx any) error) error {
	r.err = fn(nil)
	return r.err
}

type reversalAuditRepo struct {
	events []domain.AuditEvent
}

func (r *reversalAuditRepo) Write(ctx context.Context, tx any, event domain.AuditEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *reversalAuditRepo) List(ctx context.Context, resourceType, resourceID string, limit int) ([]domain.AuditEvent, error) {
	return nil, nil
}

func reversalEntry() *domain.JournalEntry {
	return &domain.JournalEntry{
		ID:              uuid.New(),
		ReferenceNumber: "DEP-20260920-0001",
		TransactionType: domain.TxTypeDeposit,
		Status:          domain.JournalStatusPosted,
		Description:     "Setoran tunai",
		CreatedBy:       "teller1",
		BranchCode:      "KC001",
		Lines: []domain.JournalLine{
			{AccountNumber: "10101", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(500000), Currency: "IDR"},
			{AccountNumber: "2010000001", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(500000), Currency: "IDR"},
		},
	}
}

type reversalFixture struct {
	svc     *ledgerService
	repo    *reversalLedgerRepo
	posting *stubPostingSvc
	audit   *reversalAuditRepo
	tx      *reversalTxRunner
	actor   domain.Actor
}

func newReversalFixture(entry *domain.JournalEntry) reversalFixture {
	repo := &reversalLedgerRepo{entry: entry}
	posting := &stubPostingSvc{}
	audit := &reversalAuditRepo{}
	runner := &reversalTxRunner{}
	return reversalFixture{
		svc: &ledgerService{
			ledgerRepo: repo,
			posting:    posting,
			auditRepo:  audit,
			txRunner:   runner,
		},
		repo:    repo,
		posting: posting,
		audit:   audit,
		tx:      runner,
		actor: domain.Actor{
			UserID:     uuid.New(),
			Username:   "supervisor1",
			Role:       domain.RoleSupervisor,
			BranchCode: "KC001",
		},
	}
}

func (f reversalFixture) request() domain.ReversalRequest {
	return domain.ReversalRequest{
		Reference: "DEP-20260920-0001",
		Reason:    "salah rekening tujuan",
		Actor:     f.actor,
	}
}

func TestReverse_MembuatKontraDanMenandaiJurnalAsal(t *testing.T) {
	f := newReversalFixture(reversalEntry())

	entry, err := f.svc.Reverse(context.Background(), f.request())
	if err != nil {
		t.Fatalf("pembatalan gagal: %v", err)
	}
	if entry == nil {
		t.Fatal("jurnal kontra tidak dikembalikan")
	}

	if f.posting.last.TransactionType != domain.TxTypeReversal {
		t.Fatalf("jenis transaksi %q, mau REVERSAL", f.posting.last.TransactionType)
	}
	if f.posting.last.IdempotencyKey != "REV-DEP-20260920-0001" {
		t.Fatalf("kunci idempotency %q, mau mengacu jurnal asal", f.posting.last.IdempotencyKey)
	}
	if !strings.Contains(f.posting.last.Description, "DEP-20260920-0001") ||
		!strings.Contains(f.posting.last.Description, "salah rekening tujuan") {
		t.Fatalf("deskripsi harus memuat referensi asal dan alasan, dapat %q", f.posting.last.Description)
	}

	// Arah ditukar, nominal tetap penuh: kaki debit asal menjadi kredit di kontra.
	if len(f.posting.last.Lines) != 2 {
		t.Fatalf("jumlah baris %d, mau 2", len(f.posting.last.Lines))
	}
	if f.posting.last.Lines[0].Direction != domain.DirectionCredit ||
		!f.posting.last.Lines[0].Amount.Equal(decimal.NewFromInt(500000)) ||
		f.posting.last.Lines[0].AccountNumber != "10101" {
		t.Fatalf("baris pertama kontra tidak sesuai: %+v", f.posting.last.Lines[0])
	}
	if f.posting.last.Lines[1].Direction != domain.DirectionDebit ||
		f.posting.last.Lines[1].AccountNumber != "2010000001" {
		t.Fatalf("baris kedua kontra tidak sesuai: %+v", f.posting.last.Lines[1])
	}
	// Jurnal kontra harus mewarisi cabang jurnal asal; kalau tidak, ia tersimpan sebagai
	// jurnal bank-wide dan tidak lagi terlihat sebagai transaksi cabang itu.
	if f.posting.last.BranchCode != "KC001" {
		t.Fatalf("cabang kontra %q, mau mengikuti jurnal asal KC001", f.posting.last.BranchCode)
	}

	if len(f.repo.markedRef) != 1 || f.repo.markedRef[0] != "DEP-20260920-0001" {
		t.Fatalf("jurnal asal tidak ditandai dibatalkan: %+v", f.repo.markedRef)
	}

	if len(f.audit.events) != 1 {
		t.Fatalf("audit %d peristiwa, mau 1", len(f.audit.events))
	}
	event := f.audit.events[0]
	if event.Action != ActionReverse || event.ResourceType != "JOURNAL" || event.ResourceID != "DEP-20260920-0001" {
		t.Fatalf("audit tidak sesuai: %+v", event)
	}
	if event.Changes["reason"] != "salah rekening tujuan" || event.Changes["transaction_type"] != string(domain.TxTypeDeposit) {
		t.Fatalf("alasan dan jenis transaksi asal harus tercatat: %+v", event.Changes)
	}
}

func TestReverse_MenolakPembatalanYangTidakSah(t *testing.T) {
	asesmen := func(ubah func(*domain.JournalEntry)) *domain.JournalEntry {
		entry := reversalEntry()
		ubah(entry)
		return entry
	}

	cases := []struct {
		name string
		// wantErr nil berarti penolakan diperiksa lewat ketiadaan jurnal kontra,
		// bukan lewat sentinel tertentu.
		wantErr error
		entry   *domain.JournalEntry
	}{
		{
			name:    "jurnal sudah pernah dibatalkan",
			entry:   asesmen(func(e *domain.JournalEntry) { e.Status = domain.JournalStatusReversed }),
			wantErr: domain.ErrJournalAlreadyReversed,
		},
		{
			name:    "jurnal belum diposting",
			entry:   asesmen(func(e *domain.JournalEntry) { e.Status = domain.JournalStatusPendingApproval }),
			wantErr: domain.ErrJournalNotPosted,
		},
		{
			name:    "jurnal pembatalan tidak dapat dibatalkan lagi",
			entry:   asesmen(func(e *domain.JournalEntry) { e.TransactionType = domain.TxTypeReversal }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "pencatat transaksi membatalkan transaksinya sendiri",
			entry:   asesmen(func(e *domain.JournalEntry) { e.CreatedBy = "supervisor1" }),
			wantErr: domain.ErrSelfReversal,
		},
		{
			name:  "jurnal tanpa baris",
			entry: asesmen(func(e *domain.JournalEntry) { e.Lines = nil }),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReversalFixture(tc.entry)
			_, err := f.svc.Reverse(context.Background(), f.request())
			if err == nil {
				t.Fatal("pembatalan seharusnya ditolak")
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("kesalahan %v, ingin %v", err, tc.wantErr)
			}
			if f.posting.calls != 0 {
				t.Fatal("jurnal kontra tidak boleh dibuat saat penolakan")
			}
			if len(f.repo.markedRef) != 0 {
				t.Fatal("jurnal asal tidak boleh ditandai saat penolakan")
			}
			if len(f.audit.events) != 0 {
				t.Fatal("audit tidak boleh ditulis saat penolakan")
			}
		})
	}
}

// Jurnal di cabang lain dilaporkan sebagai tidak ditemukan: membedakannya memberi tahu
// pemanggil bahwa referensi tertentu memang ada di bank ini.
func TestReverse_MenyamarkanJurnalCabangLain(t *testing.T) {
	entry := reversalEntry()
	entry.BranchCode = "KC999"
	f := newReversalFixture(entry)

	_, err := f.svc.Reverse(context.Background(), f.request())
	if !errors.Is(err, domain.ErrJournalNotFound) {
		t.Fatalf("kesalahan %v, ingin ErrJournalNotFound", err)
	}
	if f.posting.calls != 0 {
		t.Fatal("jurnal kontra tidak boleh dibuat untuk cabang lain")
	}
}

func TestReverse_MeneruskanKegagalanPenandaanStatus(t *testing.T) {
	f := newReversalFixture(reversalEntry())
	f.repo.markErr = domain.ErrJournalAlreadyReversed

	// Kegagalan menandai status harus membatalkan seluruh transaksi; jurnal kontra yang
	// terlanjur dibuat tidak boleh dianggap berhasil.
	_, err := f.svc.Reverse(context.Background(), f.request())
	if !errors.Is(err, domain.ErrJournalAlreadyReversed) {
		t.Fatalf("kesalahan %v, ingin kegagalan penandaan diteruskan", err)
	}
	if f.tx.err == nil {
		t.Fatal("transaksi harus berakhir gagal (rollback), bukan sukses")
	}
}

func TestReverse_TidakAdaPeranTanpaIzinDibalokDiHandler(t *testing.T) {
	// Kontrak wewenang: pembatalan hanya untuk Supervisor ke atas. Teller tidak punya
	// permission-nya, dan pemeriksaan itu ditegakkan middleware, bukan service.
	teller := domain.RoleTeller
	for _, perm := range domain.RolePermissions[teller] {
		if perm == domain.PermTransactionsReverse {
			t.Fatal("teller tidak boleh memiliki wewenang membatalkan transaksi")
		}
	}
	supervisor := domain.RolePermissions[domain.RoleSupervisor]
	found := false
	for _, perm := range supervisor {
		if perm == domain.PermTransactionsReverse {
			found = true
		}
	}
	if !found {
		t.Fatal("supervisor harus berwenang membatalkan transaksi")
	}
}
