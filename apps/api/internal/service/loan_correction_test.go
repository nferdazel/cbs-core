package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// correctionLoanRepo adalah LoanRepository minimal untuk koreksi nominal. Efek
// sampingnya dicatat di memori agar test dapat memastikan langkah mana yang
// benar-benar berjalan dan mana yang tidak.
type correctionLoanRepo struct {
	domain.LoanRepository

	loan       *domain.Loan
	schedules  []domain.LoanSchedule
	journalRef string

	corrected   []domain.LoanSchedule
	outstanding decimal.Decimal
	// correctionCount meniru kenaikan penghitung koreksi di transaksi yang sama;
	// nilainya masuk ke kunci idempotensi jurnal koreksi.
	correctionCount int
}

func (r *correctionLoanRepo) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	if r.loan == nil {
		return nil, domain.ErrLoanNotFound
	}
	return r.loan, nil
}

func (r *correctionLoanRepo) GetSchedules(context.Context, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *correctionLoanRepo) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	return r.loan, nil
}

func (r *correctionLoanRepo) GetSchedulesTx(context.Context, any, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *correctionLoanRepo) GetDisbursementJournalRefTx(context.Context, any, string) (string, error) {
	return r.journalRef, nil
}

func (r *correctionLoanRepo) CorrectLoanAmountTx(_ context.Context, _ any, l *domain.Loan, schedules []domain.LoanSchedule) error {
	r.corrected = schedules
	r.schedules = schedules
	r.outstanding = l.OutstandingPrincipal
	return nil
}

func (r *correctionLoanRepo) NextCorrectionCountTx(context.Context, any, uuid.UUID) (int, error) {
	r.correctionCount++
	return r.correctionCount, nil
}

var _ domain.LoanRepository = (*correctionLoanRepo)(nil)

const correctionDisbursementRef = "TRF-20260920-0002"

// correctionDisbursementEntry adalah jurnal pencairan asal: pokok kredit didebit
// (piutang kredit bertambah) melawan rekening nasabah yang dikredit (dana keluar).
// Arah ini mengikuti `disbursementLegs` di kode produksi; kebalikannya adalah dugaan
// yang paling mudah dibuat dan pernah menyesatkan komentar di sini.
func correctionDisbursementEntry() *domain.JournalEntry {
	return &domain.JournalEntry{
		ID:              uuid.New(),
		ReferenceNumber: correctionDisbursementRef,
		TransactionType: domain.TxTypeTransferInternal,
		Status:          domain.JournalStatusPosted,
		Description:     "Pencairan kredit",
		CreatedBy:       "ao.uji",
		BranchCode:      "001",
		Source:          domain.SourceLoan,
		Lines: []domain.JournalLine{
			{ID: uuid.New(), AccountNumber: "10301", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(10_000_000)},
			{ID: uuid.New(), AccountNumber: "1110001234", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(10_000_000)},
		},
	}
}

// correctionSchedules menghasilkan empat angsuran: satu sudah lunas (pokok penuh)
// dan tiga masih PENDING, sehingga koreksi punya riwayat yang harus dipertahankan.
func correctionSchedules(loanID uuid.UUID) []domain.LoanSchedule {
	paidAt := tanggalBisnisUji
	pending := func(no int) domain.LoanSchedule {
		return domain.LoanSchedule{
			ID:               uuid.New(),
			LoanID:           loanID,
			InstallmentNo:    no,
			DueDate:          tanggalBisnisUji.AddDate(0, no, 0),
			PrincipalAmount:  decimal.NewFromInt(2_500_000),
			ProfitAmount:     decimal.NewFromInt(100_000),
			TotalInstallment: decimal.NewFromInt(2_600_000),
			ProfitType:       domain.ProfitTypeInterest,
			Status:           domain.InstallmentStatusPending,
		}
	}
	return []domain.LoanSchedule{
		{
			ID:               uuid.New(),
			LoanID:           loanID,
			InstallmentNo:    1,
			DueDate:          tanggalBisnisUji,
			PrincipalAmount:  decimal.NewFromInt(2_500_000),
			ProfitAmount:     decimal.NewFromInt(100_000),
			TotalInstallment: decimal.NewFromInt(2_600_000),
			PaidPrincipal:    decimal.NewFromInt(2_500_000),
			PaidProfit:       decimal.NewFromInt(100_000),
			ProfitType:       domain.ProfitTypeInterest,
			Status:           domain.InstallmentStatusPaid,
			PaidAt:           &paidAt,
		},
		pending(2),
		pending(3),
		pending(4),
	}
}

// correctionFixture menyiapkan satu kredit DISBURSED dengan jurnal pencairan asal.
// Ambang persetujuan stub bawaan (50 juta) di atas nominal uji, sehingga jalur
// bahagia berjalan langsung lewat CorrectLoanAmount.
func correctionFixture() (*correctionLoanRepo, *stubPosting, *stubAuditRepo, *loanService) {
	loanID := uuid.New()
	repo := &correctionLoanRepo{
		loan: &domain.Loan{
			ID:                   loanID,
			LoanNumber:           "KRD-2026-0008",
			Status:               domain.LoanStatusDisbursed,
			BranchCode:           "001",
			PrincipalAmount:      decimal.NewFromInt(10_000_000),
			OutstandingPrincipal: decimal.NewFromInt(7_500_000),
			TotalPayable:         decimal.NewFromInt(10_400_000),
			MonthlyInstallment:   decimal.NewFromInt(2_600_000),
			TermMonths:           4,
		},
		schedules:  correctionSchedules(loanID),
		journalRef: correctionDisbursementRef,
	}
	posting := &stubPosting{}
	audit := &stubAuditRepo{}
	svc := &loanService{
		loanRepo:   repo,
		ledgerRepo: &reversalLedgerRepo{entry: correctionDisbursementEntry()},
		posting:    posting,
		auditRepo:  audit,
		txRunner:   stubTxRunner{},
		dates:      &reversalDateRepo{date: tanggalBisnisUji},
		approvals:  &stubApprovals{},
	}
	return repo, posting, audit, svc
}

func correctionActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "admin.uji", Role: domain.RoleAdmin, BranchCode: "001"}
}

// (a) Jalur bahagia nominal naik: jadwal belum dibayar dihitung ulang, sisa pokok
// benar, jurnal selisih searah pencairan awal, dan audit tercatat.
func TestCorrectLoanAmount_MenaikkanNominal(t *testing.T) {
	repo, posting, audit, svc := correctionFixture()
	before := repo.schedules[0]

	loan, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor())
	if err != nil {
		t.Fatalf("koreksi gagal: %v", err)
	}

	if !loan.PrincipalAmount.Equal(decimal.NewFromInt(12_000_000)) {
		t.Fatalf("nominal kredit %s, ingin 12000000", loan.PrincipalAmount)
	}
	if !loan.OutstandingPrincipal.Equal(decimal.NewFromInt(9_500_000)) {
		t.Fatalf("sisa pokok %s, ingin 9500000 (nominal baru - pokok dibayar)", loan.OutstandingPrincipal)
	}
	if !loan.MonthlyInstallment.Equal(decimal.NewFromInt(3_266_667)) {
		t.Fatalf("angsuran bulanan %s, ingin 3266667", loan.MonthlyInstallment)
	}
	if !loan.TotalPayable.Equal(decimal.NewFromInt(12_400_000)) {
		t.Fatalf("total tagihan %s, ingin 12400000", loan.TotalPayable)
	}

	totalPrincipal := decimal.Zero
	for _, s := range loan.Schedules {
		totalPrincipal = totalPrincipal.Add(s.PrincipalAmount)
	}
	if !totalPrincipal.Equal(decimal.NewFromInt(12_000_000)) {
		t.Fatalf("total pokok jadwal %s, ingin sama dengan nominal baru 12000000", totalPrincipal)
	}
	// Selisih pembulatan diserap angsuran belum dibayar terakhir.
	if !loan.Schedules[3].PrincipalAmount.Equal(decimal.NewFromInt(3_166_666)) {
		t.Fatalf("pokok angsuran terakhir %s, ingin 3166666", loan.Schedules[3].PrincipalAmount)
	}
	// Angsuran yang sudah dibayar tetap apa adanya.
	if loan.Schedules[0].ID != before.ID ||
		!loan.Schedules[0].PrincipalAmount.Equal(before.PrincipalAmount) ||
		!loan.Schedules[0].ProfitAmount.Equal(before.ProfitAmount) ||
		!loan.Schedules[0].PaidPrincipal.Equal(before.PaidPrincipal) ||
		loan.Schedules[0].Status != domain.InstallmentStatusPaid {
		t.Fatalf("angsuran lunas berubah: %+v", loan.Schedules[0])
	}

	if len(posting.requests) != 1 {
		t.Fatalf("jurnal koreksi %d, ingin 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.TransactionType != domain.TxTypeAdjustment {
		t.Fatalf("jenis transaksi %q, ingin ADJUSTMENT", req.TransactionType)
	}
	if req.IdempotencyKey != "CORR-KRD-2026-0008-12000000-1" {
		t.Fatalf("kunci idempotency %q", req.IdempotencyKey)
	}
	if req.BranchCode != "001" || req.Source != domain.SourceLoan {
		t.Fatalf("cabang/alur jurnal salah: %q/%q", req.BranchCode, req.Source)
	}
	if !req.EntryDate.Equal(tanggalBisnisUji) {
		t.Fatalf("tanggal jurnal %s, ingin tanggal bisnis %s", req.EntryDate, tanggalBisnisUji)
	}
	if len(req.Lines) != 2 {
		t.Fatalf("baris jurnal %d, ingin 2", len(req.Lines))
	}
	// Nominal naik: arah mengikuti pencairan awal — pokok didebit, rekening nasabah dikredit.
	if req.Lines[0].AccountNumber != "10301" || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("kaki pokok tidak sesuai: %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != "1110001234" || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kaki rekening nasabah tidak sesuai: %+v", req.Lines[1])
	}
	if !req.Lines[0].Amount.Equal(decimal.NewFromInt(2_000_000)) || !req.Lines[1].Amount.Equal(decimal.NewFromInt(2_000_000)) {
		t.Fatalf("nominal jurnal tidak sama dengan selisih: %+v", req.Lines)
	}

	if len(audit.events) != 1 {
		t.Fatalf("audit %d peristiwa, ingin 1", len(audit.events))
	}
	event := audit.events[0]
	if event.Action != "CORRECT_LOAN_AMOUNT" || event.ResourceType != "loan" || event.ResourceID != repo.loan.ID.String() {
		t.Fatalf("audit tidak sesuai: %+v", event)
	}
	if event.Changes["old_amount"] != "10000000.00" || event.Changes["new_amount"] != "12000000.00" || event.Changes["difference"] != "2000000.00" {
		t.Fatalf("nominal audit tidak sesuai: %+v", event.Changes)
	}
	if event.Changes["reason"] != "salah input nominal" {
		t.Fatalf("alasan tidak tercatat: %+v", event.Changes)
	}
	if _, ok := event.Changes["journal_reference"]; !ok {
		t.Fatalf("nomor referensi jurnal tidak tercatat: %+v", event.Changes)
	}
}

// (b) Nominal turun: arah baris jurnal dibalik dari pencairan awal.
func TestCorrectLoanAmount_MenurunkanNominalMembalikArah(t *testing.T) {
	repo, posting, _, svc := correctionFixture()

	loan, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(8_000_000),
		Reason:    "kelebihan pencairan",
	}, correctionActor())
	if err != nil {
		t.Fatalf("koreksi gagal: %v", err)
	}

	if !loan.OutstandingPrincipal.Equal(decimal.NewFromInt(5_500_000)) {
		t.Fatalf("sisa pokok %s, ingin 5500000", loan.OutstandingPrincipal)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal koreksi %d, ingin 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.IdempotencyKey != "CORR-KRD-2026-0008-8000000-1" {
		t.Fatalf("kunci idempotency %q", req.IdempotencyKey)
	}
	if req.Lines[0].Direction != domain.DirectionCredit || req.Lines[1].Direction != domain.DirectionDebit {
		t.Fatalf("arah baris tidak dibalik: %+v", req.Lines)
	}
	if !req.Lines[0].Amount.Equal(decimal.NewFromInt(2_000_000)) {
		t.Fatalf("nominal jurnal %s, ingin 2000000", req.Lines[0].Amount)
	}
}

// (c) Angsuran yang sudah dibayar tidak boleh berubah sedikit pun nilai barisnya.
func TestCorrectLoanAmount_AngsuranDibayarTidakBerubah(t *testing.T) {
	repo, _, _, svc := correctionFixture()
	before := repo.schedules[0]

	if _, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor()); err != nil {
		t.Fatalf("koreksi gagal: %v", err)
	}

	after := repo.schedules[0]
	if after.ID != before.ID ||
		!after.PrincipalAmount.Equal(before.PrincipalAmount) ||
		!after.ProfitAmount.Equal(before.ProfitAmount) ||
		!after.TotalInstallment.Equal(before.TotalInstallment) ||
		!after.PaidPrincipal.Equal(before.PaidPrincipal) ||
		!after.PaidProfit.Equal(before.PaidProfit) ||
		after.Status != before.Status {
		t.Fatalf("baris angsuran lunas berubah:\nsebelum %+v\nsesudah %+v", before, after)
	}
	if after.PaidAt == nil || !after.PaidAt.Equal(*before.PaidAt) {
		t.Fatalf("tanggal bayar angsuran lunas berubah: %v", after.PaidAt)
	}
}

// (d) Nominal sama dengan yang lama ditolak: tidak ada jurnal dan jadwal tidak ditulis.
func TestCorrectLoanAmount_MenolakNominalSama(t *testing.T) {
	repo, posting, _, svc := correctionFixture()

	_, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: repo.loan.PrincipalAmount,
		Reason:    "tanpa perubahan",
	}, correctionActor())
	if !errors.Is(err, domain.ErrLoanAmountUnchanged) {
		t.Fatalf("kesalahan %v, ingin ErrLoanAmountUnchanged", err)
	}
	if len(posting.requests) != 0 || len(repo.corrected) != 0 {
		t.Fatal("tidak boleh ada jurnal atau penulisan jadwal saat nominal sama")
	}
}

// (e) Kredit yang bukan DISBURSED tidak boleh dikoreksi.
func TestCorrectLoanAmount_MenolakStatusBukanDisbursed(t *testing.T) {
	repo, posting, _, svc := correctionFixture()
	repo.loan.Status = domain.LoanStatusApproved

	_, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor())
	if !errors.Is(err, domain.ErrLoanNotCancellable) {
		t.Fatalf("kesalahan %v, ingin ErrLoanNotCancellable", err)
	}
	if len(posting.requests) != 0 || len(repo.corrected) != 0 {
		t.Fatal("tidak boleh ada jurnal saat status bukan DISBURSED")
	}
}

// (f) Kredit cabang lain disamarkan sebagai tidak ditemukan.
func TestCorrectLoanAmount_MenyamarkanCabangLain(t *testing.T) {
	repo, posting, _, svc := correctionFixture()
	repo.loan.BranchCode = "999"

	_, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor())
	if !errors.Is(err, domain.ErrLoanNotFound) {
		t.Fatalf("kesalahan %v, ingin ErrLoanNotFound", err)
	}
	if len(posting.requests) != 0 {
		t.Fatal("tidak boleh ada jurnal untuk cabang lain")
	}
}

// (g) Tanpa layanan persetujuan, koreksi tidak dijalankan: dikembalikan sebagai
// menunggu persetujuan dan tidak ada jurnal yang ditulis.
func TestCorrectLoanAmount_TanpaLayananPersetujuanDitolak(t *testing.T) {
	repo, posting, _, svc := correctionFixture()
	svc.approvals = nil

	loan, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor())
	if loan != nil {
		t.Fatal("koreksi tidak boleh menghasilkan kredit tanpa persetujuan")
	}
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("koreksi harus menunggu persetujuan, dapat: %v", err)
	}
	if pending.ActionType != ActionLoanCorrection {
		t.Fatalf("jenis aksi %s, ingin %s", pending.ActionType, ActionLoanCorrection)
	}
	if len(posting.requests) != 0 || len(repo.corrected) != 0 {
		t.Fatal("tidak boleh ada jurnal saat menunggu persetujuan")
	}
}

// (h) Tanggal bisnis tidak terbaca: operasi ditolak sebelum satu pun baris berubah.
func TestCorrectLoanAmount_TanggalBisnisTidakTerbaca(t *testing.T) {
	repo, posting, _, svc := correctionFixture()
	svc.dates = &reversalDateRepo{err: errors.New("tanggal bisnis tidak terbaca")}

	_, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor())
	if err == nil {
		t.Fatal("koreksi seharusnya ditolak saat tanggal bisnis tidak terbaca")
	}
	if len(posting.requests) != 0 || len(repo.corrected) != 0 {
		t.Fatalf("tidak boleh ada jurnal saat tanggal bisnis tidak terbaca: %+v", posting.requests)
	}
	if !repo.loan.PrincipalAmount.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("nominal kredit berubah menjadi %s", repo.loan.PrincipalAmount)
	}
}

// Ambang persetujuan 0 berarti setiap koreksi wajib disetujui: permintaan dibuat
// dengan jenis aksi yang tepat dan efeknya belum berjalan.
func TestCorrectLoanAmount_MenungguPersetujuan(t *testing.T) {
	repo, posting, _, svc := correctionFixture()
	svc.approvals = &stubApprovals{threshold: map[string]decimal.Decimal{
		ActionLoanCorrection: decimal.Zero,
	}}

	_, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID:    repo.loan.ID,
		NewAmount: decimal.NewFromInt(12_000_000),
		Reason:    "salah input nominal",
	}, correctionActor())
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("koreksi harus menunggu persetujuan, dapat: %v", err)
	}
	if pending.ActionType != ActionLoanCorrection {
		t.Fatalf("jenis aksi %s, ingin %s", pending.ActionType, ActionLoanCorrection)
	}
	if len(posting.requests) != 0 || len(repo.corrected) != 0 {
		t.Fatal("tidak boleh ada jurnal sebelum disetujui")
	}
}

// ExecuteApproved menjalankan koreksi yang sudah disetujui di dalam transaksi
// pemanggil: jurnal terposting dan jadwal tersimpan.
func TestCorrectLoanAmount_ExecuteApproved(t *testing.T) {
	repo, posting, _, svc := correctionFixture()
	maker := correctionActor()
	checker := domain.Actor{UserID: uuid.New(), Username: "kabag.uji", Role: domain.RoleSupervisor, BranchCode: "001"}

	payload := map[string]any{
		"loan_id":        repo.loan.ID.String(),
		"loan_number":    repo.loan.LoanNumber,
		"new_amount":     "12000000",
		"reason":         "salah input nominal",
		"maker_id":       maker.UserID.String(),
		"maker_username": maker.Username,
		"maker_role":     string(maker.Role),
		"maker_branch":   maker.BranchCode,
	}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionLoanCorrection, payload, checker); err != nil {
		t.Fatalf("ExecuteApproved: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal koreksi %d, ingin 1", len(posting.requests))
	}
	if !repo.loan.PrincipalAmount.Equal(decimal.NewFromInt(12_000_000)) {
		t.Fatalf("nominal kredit %s, ingin 12000000", repo.loan.PrincipalAmount)
	}
	if len(repo.corrected) != len(repo.schedules) || len(repo.corrected) == 0 {
		t.Fatal("jadwal koreksi tidak tersimpan")
	}
}

// Wewenang: koreksi nominal hanya untuk peran tertinggi, sama seperti pembatalan.
func TestCorrectLoanAmount_HanyaPeranTertinggi(t *testing.T) {
	for _, role := range []domain.StaffRole{domain.RoleSuperAdmin, domain.RoleAdmin} {
		if !role.HasPermission(domain.PermLoansCorrect) {
			t.Fatalf("%s harus berwenang mengoreksi nominal", role)
		}
	}
	if domain.RoleSupervisor.HasPermission(domain.PermLoansCorrect) {
		t.Fatal("supervisor tidak boleh berwenang mengoreksi nominal")
	}
}

// Jurnal pencairan dengan lebih dari satu baris per arah tidak boleh ditafsirkan:
// mengambil baris pertama akan mengarahkan koreksi ke akun yang salah tanpa peringatan.
// Lebih baik ditolak dan diperiksa manusia.
func TestCorrectLoanAmount_JurnalPencairanAmbiguDitolak(t *testing.T) {
	repo, posting, _, svc := correctionFixture()

	// Tambahkan kaki kredit kedua (mis. pencairan dengan biaya yang tercatat di jurnal
	// yang sama) sehingga sisi kredit tidak lagi tunggal.
	entry := correctionDisbursementEntry()
	entry.Lines = append(entry.Lines, domain.JournalLine{
		ID:            uuid.New(),
		AccountNumber: "40100",
		Direction:     domain.DirectionCredit,
		Amount:        decimal.NewFromInt(50_000),
	})
	svc.ledgerRepo = &reversalLedgerRepo{entry: entry}

	_, err := svc.CorrectLoanAmount(context.Background(), domain.CorrectLoanAmountInput{
		LoanID: repo.loan.ID, NewAmount: decimal.NewFromInt(12_000_000), Reason: "salah input nominal",
	}, correctionActor())
	if err == nil {
		t.Fatal("koreksi seharusnya ditolak saat jurnal pencairan ambigu")
	}
	if len(posting.requests) != 0 {
		t.Fatalf("tidak boleh ada jurnal saat jurnal pencairan ambigu: %+v", posting.requests)
	}
	if repo.loan.PrincipalAmount.Equal(decimal.NewFromInt(12_000_000)) {
		t.Fatal("nominal kredit berubah padahal koreksi ditolak")
	}
}
