package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

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

// reversalConfig menyediakan tanggal bisnis untuk pemisahan same-day dan lintas hari.
type reversalConfig struct {
	domain.SystemConfigService
	date string
}

func (c *reversalConfig) GetString(ctx context.Context, key, fallback string) string {
	if key == "system.business_date" && c.date != "" {
		return c.date
	}
	return fallback
}

// reversalApprovals mencatat permintaan persetujuan yang dibuat.
type reversalApprovals struct {
	domain.MakerCheckerService
	created int
	err     error
}

func (a *reversalApprovals) CreateRequest(ctx context.Context, input domain.CreateMakerCheckerInput, actor domain.Actor) (*domain.MakerCheckerRequest, error) {
	if a.err != nil {
		return nil, a.err
	}
	a.created++
	return &domain.MakerCheckerRequest{ID: uuid.New(), ActionType: input.ActionType, Payload: input.Payload}, nil
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
		Source:          domain.SourceTeller,
		EntryDate:       time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC),
		Lines: []domain.JournalLine{
			{AccountNumber: "10101", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(500000), Currency: "IDR"},
			{AccountNumber: "2010000001", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(500000), Currency: "IDR"},
		},
	}
}

type reversalFixture struct {
	svc       *ledgerService
	repo      *reversalLedgerRepo
	posting   *stubPostingSvc
	audit     *reversalAuditRepo
	tx        *reversalTxRunner
	actor     domain.Actor
	approvals *reversalApprovals
}

func newReversalFixture(entry *domain.JournalEntry) reversalFixture {
	repo := &reversalLedgerRepo{entry: entry}
	posting := &stubPostingSvc{}
	audit := &reversalAuditRepo{}
	runner := &reversalTxRunner{}
	approvals := &reversalApprovals{}
	return reversalFixture{
		svc: &ledgerService{
			ledgerRepo: repo,
			posting:    posting,
			auditRepo:  audit,
			txRunner:   runner,
			approvals:  approvals,
			// Tanggal bisnis sama dengan EntryDate fixture: jalur langsung.
			configSvc: &reversalConfig{date: "2026-09-20"},
		},
		repo:      repo,
		posting:   posting,
		audit:     audit,
		tx:        runner,
		approvals: approvals,
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
	// Jurnal kontra dibuat alur yang sama dengan jurnal asal yang dibatalkannya.
	if f.posting.last.Source != domain.SourceTeller {
		t.Fatalf("alur jurnal kontra %q, mau TELLER", f.posting.last.Source)
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
			name:    "jurnal akrual",
			entry:   asesmen(func(e *domain.JournalEntry) { e.TransactionType = domain.TxTypeInterestAccrual }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "jurnal penyesuaian batch",
			entry:   asesmen(func(e *domain.JournalEntry) { e.TransactionType = domain.TxTypeAdjustment }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "jurnal yang dibuat sistem",
			entry:   asesmen(func(e *domain.JournalEntry) { e.CreatedBy = "SYSTEM" }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "jurnal alur kredit",
			entry:   asesmen(func(e *domain.JournalEntry) { e.Source = domain.SourceLoan }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "jurnal alur deposito",
			entry:   asesmen(func(e *domain.JournalEntry) { e.Source = domain.SourceDeposit }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "jurnal alur batch",
			entry:   asesmen(func(e *domain.JournalEntry) { e.Source = domain.SourceBatch }),
			wantErr: domain.ErrReversalNotAllowed,
		},
		{
			name:    "jurnal tanpa penanda alur",
			entry:   asesmen(func(e *domain.JournalEntry) { e.Source = "" }),
			wantErr: domain.ErrReversalNotAllowed,
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

// Transaksi lintas hari mengubah angka yang sudah masuk laporan tutup buku, jadi wajib
// lewat pejabat kedua meskipun pelakunya supervisor.
func TestReverse_LintasHariWajibPersetujuan(t *testing.T) {
	f := newReversalFixture(reversalEntry())
	f.svc.configSvc = &reversalConfig{date: "2026-09-21"} // jurnal kemarin

	_, err := f.svc.Reverse(context.Background(), f.request())

	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("kesalahan %v, mau menunggu persetujuan", err)
	}
	if pending.ActionType != ActionReverse {
		t.Fatalf("jenis aksi %q, mau %q", pending.ActionType, ActionReverse)
	}
	if f.approvals.created != 1 {
		t.Fatalf("permintaan persetujuan %d, mau 1", f.approvals.created)
	}
	if f.posting.calls != 0 || len(f.repo.markedRef) != 0 {
		t.Fatal("jurnal kontra tidak boleh dibuat sebelum disetujui")
	}
}

// Tanggal bisnis yang tidak terbaca diperlakukan sebagai lintas hari: pembatalan yang
// tidak dapat dipastikan berasal dari hari berjalan tidak boleh lolos tanpa persetujuan.
func TestReverse_TanggalBisnisTidakTerbacaDianggapLintasHari(t *testing.T) {
	f := newReversalFixture(reversalEntry())
	f.svc.configSvc = &reversalConfig{} // tanpa tanggal bisnis

	_, err := f.svc.Reverse(context.Background(), f.request())

	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("kesalahan %v, mau menunggu persetujuan", err)
	}
	if f.posting.calls != 0 {
		t.Fatal("jurnal kontra tidak boleh dibuat tanpa persetujuan")
	}
}

// Bila layanan persetujuan tidak tersedia, pembatalan lintas hari harus ditolak, bukan
// diloloskan tanpa pejabat kedua.
func TestReverse_LintasHariTanpaLayananPersetujuanDitolak(t *testing.T) {
	f := newReversalFixture(reversalEntry())
	f.svc.configSvc = &reversalConfig{date: "2026-09-21"}
	f.svc.approvals = nil

	_, err := f.svc.Reverse(context.Background(), f.request())
	if !errors.Is(err, domain.ErrReversalNotAllowed) {
		t.Fatalf("kesalahan %v, mau penolakan", err)
	}
	if f.posting.calls != 0 {
		t.Fatal("jurnal kontra tidak boleh dibuat")
	}
}

// Eksekusi setelah disetujui memakai transaksi milik maker-checker dan mencatat pembuat
// permintaan sebagai pencatat jurnal, bukan pejabat yang menyetujui.
func TestReverse_EksekusiPersetujuanMenulisDiTransaksiPenyetuju(t *testing.T) {
	f := newReversalFixture(reversalEntry())
	f.svc.configSvc = &reversalConfig{date: "2026-09-21"}

	approved := false
	err := f.svc.ExecuteApproved(context.Background(), "tx-persetujuan", ActionReverse, map[string]any{
		"reference":      "DEP-20260920-0001",
		"reason":         "salah rekening tujuan",
		"amount":         "500000.00",
		"maker_username": "supervisor1",
	}, domain.Actor{UserID: uuid.New(), Username: "admin1", Role: domain.RoleAdmin, BranchCode: "KC001"})
	if err != nil {
		t.Fatalf("eksekusi persetujuan gagal: %v", err)
	}
	approved = true

	if !approved || f.posting.calls != 1 {
		t.Fatalf("jurnal kontra %d kali, mau 1", f.posting.calls)
	}
	if f.posting.lastAnyTx != "tx-persetujuan" {
		t.Fatalf("jurnal kontra tidak ditulis di transaksi maker-checker: %v", f.posting.lastAnyTx)
	}
	if f.posting.last.CreatedBy != "supervisor1" {
		t.Fatalf("pencatat jurnal %q, mau pembuat permintaan supervisor1", f.posting.last.CreatedBy)
	}
	if len(f.repo.markedRef) != 1 {
		t.Fatal("jurnal asal harus ditandai dibatalkan")
	}
}
