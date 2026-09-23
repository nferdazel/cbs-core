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

// writeOffFixture menyiapkan satu kredit aktif beserta pemetaan jurnal hapus buku,
// termasuk kaki pelepasan piutang bunga (10400) dan piutang denda (10305).
// Ambang persetujuan disetel 0 agar seluruh hapus buku wajib disetujui pejabat kedua.
func writeOffFixture() (interestFixture, *loanService) {
	f := newInterestFixture()
	f.repo.loan = &domain.Loan{
		ID:                    f.loanID,
		LoanNumber:            "KRD-2026-0007",
		Status:                domain.LoanStatusDisbursed,
		ProductID:             &f.productID,
		DisbursementAccountID: f.accountID,
		OutstandingPrincipal:  decimal.NewFromInt(2_500_000),
		PenaltyAccrued:        decimal.NewFromInt(50_000),
		// Syarat hapus buku POJK: kualitas Macet dan cadangan sudah 100% dari nilai
		// tercatat. Tanpa keduanya, hapus buku ditolak sebelum jurnal apa pun.
		Collectibility: domain.CollectibilityKol5,
		RequiredPPAP:   decimal.NewFromInt(2_500_000),
		DPD:            400,
		// Nilai hapus buku yang tersimpan: pokok + bunga akru + denda = 2.610.000.
		// Batas pemulihan mengacu ke nilai ini.
		WrittenOffAmount: decimal.NewFromInt(2_610_000),
	}
	// Satu angsuran dengan bunga yang sudah diakui tetapi belum tertagih.
	f.repo.schedules = []domain.LoanSchedule{{
		ID:                  uuid.New(),
		LoanID:              f.loanID,
		InstallmentNo:       1,
		PrincipalAmount:     decimal.NewFromInt(2_500_000),
		ProfitAmount:        decimal.NewFromInt(60_000),
		ProfitAccruedAmount: decimal.NewFromInt(60_000),
		Status:              domain.InstallmentStatusPending,
	}}
	f.products.rules[domain.EventLoanWriteOff] = []domain.JournalMappingRule{
		{Direction: domain.DirectionDebit, COACode: "10900", AmountSource: domain.AmountPrincipal},
		{Direction: domain.DirectionCredit, COACode: "10301", AmountSource: domain.AmountPrincipal},
		{Direction: domain.DirectionDebit, COACode: "40100", AmountSource: domain.AmountProfit},
		{Direction: domain.DirectionCredit, COACode: "10400", AmountSource: domain.AmountProfit},
		{Direction: domain.DirectionDebit, COACode: "40500", AmountSource: domain.AmountPenalty},
		{Direction: domain.DirectionCredit, COACode: "10305", AmountSource: domain.AmountPenalty},
	}
	f.products.rules[domain.EventLoanRecovery] = []domain.JournalMappingRule{
		{Direction: domain.DirectionDebit, COACode: "20100", AmountSource: domain.AmountPrincipal},
		{Direction: domain.DirectionCredit, COACode: "50100", AmountSource: domain.AmountPrincipal},
	}
	svc := newInterestTestService(f)
	svc.approvals = &stubApprovals{threshold: map[string]decimal.Decimal{
		ActionLoanWriteOff: decimal.Zero,
		ActionLoanRecovery: decimal.Zero,
	}}
	return f, svc
}

func writeOffActor() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "ao.uji", Role: domain.RoleAO, BranchCode: "001"}
}

// Hapus buku tidak langsung berefek: ia melepas tagihan dari neraca dan tidak dapat
// dibatalkan, jadi harus menunggu persetujuan pejabat kedua.
func TestWriteOffLoan_RequiresApprovalBeforePosting(t *testing.T) {
	f, svc := writeOffFixture()
	actor := writeOffActor()

	loan, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x + kunjungan lapangan",
	}, actor)
	if loan != nil {
		t.Fatal("hapus buku belum boleh menghasilkan kredit sebelum disetujui")
	}
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("hapus buku harus menunggu persetujuan, dapat: %v", err)
	}
	if pending.ActionType != ActionLoanWriteOff {
		t.Fatalf("jenis aksi %s, ingin %s", pending.ActionType, ActionLoanWriteOff)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0 sebelum disetujui", len(f.posting.requests))
	}
	if f.repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("status kredit berubah sebelum disetujui: %s", f.repo.loan.Status)
	}

	approvals, ok := svc.approvals.(*stubApprovals)
	if !ok || len(approvals.created) != 1 {
		t.Fatalf("permintaan persetujuan %d, ingin 1", len(approvals.created))
	}
	created := approvals.created[0]
	// Nominal persetujuan adalah seluruh yang dilepas: pokok + bunga akru + denda.
	if !created.Amount.Equal(decimal.NewFromInt(2_610_000)) {
		t.Fatalf("nominal permintaan %s, ingin 2610000", created.Amount)
	}
	if created.Payload["loan_id"] != f.loanID.String() {
		t.Fatalf("payload tanpa id kredit: %+v", created.Payload)
	}
	if created.Payload["maker_username"] != actor.Username {
		t.Fatalf("payload tanpa identitas pembuat: %+v", created.Payload)
	}
}

// Setelah disetujui, eksekutor menjalankan hapus buku: jurnal terposting, status
// berubah, dan jejak audit mencatat PEMBUAT keputusan, bukan pemeriksanya.
func TestWriteOffLoan_ExecuteApprovedPostsAndMarksLoan(t *testing.T) {
	f, svc := writeOffFixture()
	maker := writeOffActor()
	checker := domain.Actor{UserID: uuid.New(), Username: "kabag.uji", Role: domain.RoleSupervisor, BranchCode: "001"}
	audit := &stubAuditRepo{}
	svc.auditRepo = audit

	payload := map[string]any{
		"loan_id":            f.loanID.String(),
		"loan_number":        f.repo.loan.LoanNumber,
		"reason":             "debitor pailit",
		"collection_efforts": "somasi 3x + kunjungan lapangan",
		"maker_id":           maker.UserID.String(),
		"maker_username":     maker.Username,
		"maker_role":         string(maker.Role),
		"maker_branch":       maker.BranchCode,
	}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionLoanWriteOff, payload, checker); err != nil {
		t.Fatalf("ExecuteApproved: %v", err)
	}

	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(f.posting.requests))
	}
	if got := f.posting.requests[0].IdempotencyKey; got != "WOFF-"+f.repo.loan.LoanNumber {
		t.Fatalf("kunci idempotensi %q, ingin WOFF-%s", got, f.repo.loan.LoanNumber)
	}

	// Piutang bunga dan denda ikut dilepas: tanpa ini keduanya tetap tercatat di
	// neraca padahal kreditnya sudah tidak ada.
	lines := f.posting.requests[0].Lines
	if len(lines) != 6 {
		t.Fatalf("jurnal %d baris, ingin 6: %+v", len(lines), lines)
	}
	creditOf := func(coa string) decimal.Decimal {
		for _, line := range lines {
			if line.AccountNumber == coa && line.Direction == domain.DirectionCredit {
				return line.Amount
			}
		}
		return decimal.Zero
	}
	if got := creditOf("10301"); !got.Equal(decimal.NewFromInt(2_500_000)) {
		t.Fatalf("pokok dilepas %s, ingin 2500000", got)
	}
	if got := creditOf("10400"); !got.Equal(decimal.NewFromInt(60_000)) {
		t.Fatalf("piutang bunga dilepas %s, ingin 60000", got)
	}
	if got := creditOf("10305"); !got.Equal(decimal.NewFromInt(50_000)) {
		t.Fatalf("piutang denda dilepas %s, ingin 50000", got)
	}
	debit := sumDirection(lines, domain.DirectionDebit)
	if credit := sumDirection(lines, domain.DirectionCredit); !debit.Equal(credit) {
		t.Fatalf("jurnal tidak seimbang: debit %s != kredit %s", debit, credit)
	}
	if !f.repo.penalty.IsZero() {
		t.Fatalf("denda kredit masih %s setelah hapus buku", f.repo.penalty)
	}

	if f.repo.loan.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("status kredit %s, ingin WRITTEN_OFF", f.repo.loan.Status)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "WRITE_OFF_LOAN" {
		t.Fatalf("audit hapus buku tidak tercatat: %+v", audit.events)
	}
	if audit.events[0].ActorUsername != maker.Username {
		t.Fatalf("audit mencatat %q, ingin pembuat %q", audit.events[0].ActorUsername, maker.Username)
	}
	// Bukti syarat POJK harus tersimpan: kualitas aset, cadangan yang dibentuk, dan
	// upaya penagihan. Tanpanya, keputusan tidak dapat dipertanggungjawabkan ke Dekom.
	changes := audit.events[0].Changes
	if changes["collectibility"] != string(domain.CollectibilityKol5) {
		t.Fatalf("audit kualitas aset %v, ingin %s", changes["collectibility"], domain.CollectibilityKol5)
	}
	if changes["required_ppap"] != "2500000.00" {
		t.Fatalf("audit cadangan %v, ingin 2500000.00", changes["required_ppap"])
	}
	if changes["collection_efforts"] != "somasi 3x + kunjungan lapangan" {
		t.Fatalf("audit upaya penagihan %v, tidak tercatat", changes["collection_efforts"])
	}
}

// Kredit yang statusnya berubah antara pengajuan dan persetujuan harus ditolak saat
// eksekusi: jadwal bisa saja sudah lunas, sehingga hapus buku tidak lagi berlaku.
func TestWriteOffLoan_ExecuteApprovedRejectsChangedLoan(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusPaidOff

	payload := map[string]any{"loan_id": f.loanID.String(), "reason": "debitor pailit"}
	err := svc.ExecuteApproved(context.Background(), nil, ActionLoanWriteOff, payload, writeOffActor())
	if err == nil {
		t.Fatal("hapus buku atas kredit yang tidak lagi aktif harus ditolak")
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// Recovery hapus buku melewati ambang yang sama: menunggu persetujuan, lalu
// dieksekusi dengan payload yang diajukan.
func TestRecoverWrittenOffLoan_RequiresApprovalThenExecutes(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	maker := writeOffActor()

	loan, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000),
	}, maker)
	var pending *domain.PendingApprovalError
	if loan != nil || !errors.As(err, &pending) {
		t.Fatalf("recovery harus menunggu persetujuan, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0 sebelum disetujui", len(f.posting.requests))
	}

	approvals := svc.approvals.(*stubApprovals)
	payload := approvals.created[0].Payload
	payload["maker_id"] = maker.UserID.String()
	payload["maker_username"] = maker.Username
	payload["maker_role"] = string(maker.Role)
	payload["maker_branch"] = maker.BranchCode

	checker := domain.Actor{UserID: uuid.New(), Username: "kabag.uji", Role: domain.RoleSupervisor, BranchCode: "001"}
	if err := svc.ExecuteApproved(context.Background(), nil, ActionLoanRecovery, payload, checker); err != nil {
		t.Fatalf("ExecuteApproved recovery: %v", err)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1", len(f.posting.requests))
	}
	if f.repo.loan.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("status kredit berubah oleh recovery: %s", f.repo.loan.Status)
	}
}

// Ambang positif mengizinkan operasi di bawahnya berjalan langsung tanpa persetujuan.
func TestWriteOffLoan_BelowThresholdExecutesDirectly(t *testing.T) {
	f, svc := writeOffFixture()
	svc.approvals = &stubApprovals{threshold: map[string]decimal.Decimal{
		ActionLoanWriteOff: decimal.NewFromInt(10_000_000),
	}}
	svc.txRunner = stubTxRunner{}

	loan, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x + kunjungan lapangan",
	}, writeOffActor())
	if err != nil {
		t.Fatalf("hapus buku di bawah ambang: %v", err)
	}
	if loan == nil || loan.Status != domain.LoanStatusWrittenOff {
		t.Fatalf("kredit tidak ditandai hapus buku: %+v", loan)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(f.posting.requests))
	}
}

// Pemetaan produk yang belum memuat kaki piutang bunga harus menolak hapus buku,
// bukan melewati nominalnya diam-diam dan meninggalkan piutang di neraca.
func TestWriteOffLoan_RejectsUnmappedAccruedProfit(t *testing.T) {
	f, svc := writeOffFixture()
	svc.approvals = nil
	// Pemetaan gaya lama: hanya pokok yang punya kaki jurnal.
	f.products.rules[domain.EventLoanWriteOff] = f.products.rules[domain.EventLoanWriteOff][:2]

	_, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit", CollectionEfforts: "somasi 3x + kunjungan lapangan",
	}, writeOffActor())
	if err == nil || !strings.Contains(err.Error(), "piutang bunga") {
		t.Fatalf("hapus buku tanpa kaki piutang bunga harus ditolak, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0 saat pemetaan tidak lengkap", len(f.posting.requests))
	}
	if f.repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("status kredit berubah meski hapus buku ditolak: %s", f.repo.loan.Status)
	}
}

// Retry recovery tidak boleh menggandakan jurnal. Kunci idempotensi harus
// DETERMINISTIK; kode lama memakai time.Now().Unix() sehingga dua permintaan identik
// mendapat kunci berbeda dan posting engine menulis jurnal kedua. Test ini gagal
// (unix timestamp != kunci deterministik) bila perilaku lama kembali.
func TestRecoverWrittenOffLoan_IdempotencyKeyDeterministic(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	// Jalur langsung (di bawah ambang / tanpa maker-checker) agar posting terjadi.
	svc.approvals = nil

	input := domain.RecoverWrittenOffLoanInput{LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000)}
	for i := 0; i < 2; i++ {
		if _, err := svc.RecoverWrittenOffLoan(context.Background(), input, writeOffActor()); err != nil {
			t.Fatalf("RecoverWrittenOffLoan ke-%d: %v", i+1, err)
		}
	}
	if len(f.posting.requests) != 2 {
		t.Fatalf("permintaan posting %d, ingin 2", len(f.posting.requests))
	}
	want := "RECOV-" + f.repo.loan.LoanNumber + "-500000"
	for i, req := range f.posting.requests {
		if req.IdempotencyKey != want {
			t.Fatalf("kunci idempotensi permintaan ke-%d %q, ingin %q (bukan timestamp)", i+1, req.IdempotencyKey, want)
		}
	}
	if len(distinctKeys(f.posting.requests)) != 1 {
		t.Fatalf("dua permintaan identik menghasilkan %d kunci berbeda, ingin 1", len(distinctKeys(f.posting.requests)))
	}
}

// Kunci dari pemanggil dipakai apa adanya sehingga dua recovery sah dengan nominal
// sama dapat dibedakan, dan pengulangan dengan kunci sama tetap idempoten.
func TestRecoverWrittenOffLoan_UsesCallerIdempotencyKey(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	svc.approvals = nil

	if _, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000), IdempotencyKey: "kuitansi-777",
	}, writeOffActor()); err != nil {
		t.Fatalf("RecoverWrittenOffLoan: %v", err)
	}
	if got := f.posting.requests[0].IdempotencyKey; got != "RECOV-"+f.repo.loan.LoanNumber+"-kuitansi-777" {
		t.Fatalf("kunci idempotensi %q, ingin memakai kunci pemanggil", got)
	}
}

// Pemulihan sebagian (di bawah nilai hapus buku) tetap boleh: yang dilarang hanya
// akumulasi yang MELEBIHI nilai hapus buku.
func TestRecoverWrittenOffLoan_MenerimaPemulihanSebagian(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	svc.approvals = nil

	if _, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(400_000),
	}, writeOffActor()); err != nil {
		t.Fatalf("pemulihan sebagian harus boleh: %v", err)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1", len(f.posting.requests))
	}
}

// Akumulasi pemulihan yang MELEBIHI nilai hapus buku ditolak dengan galat spesifik;
// tidak ada jurnal yang ditulis untuk permintaan yang ditolak.
func TestRecoverWrittenOffLoan_MenolakMelebihiNilaiHapusBuku(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	f.repo.loan.WrittenOffAmount = decimal.NewFromInt(1_000_000)
	f.repo.recovered = decimal.NewFromInt(600_000)
	svc.approvals = nil

	_, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000),
	}, writeOffActor())
	if !errors.Is(err, domain.ErrRecoveryExceedsWriteOff) {
		t.Fatalf("pemulihan yang melebihi nilai hapus buku harus ditolak, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0 saat ditolak", len(f.posting.requests))
	}
}

// Akumulasi pemulihan yang TEPAT SAMA dengan nilai hapus buku masih boleh: "melebihi"
// berarti lebih besar, bukan sama dengan.
func TestRecoverWrittenOffLoan_MenerimaTepatNilaiHapusBuku(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	f.repo.loan.WrittenOffAmount = decimal.NewFromInt(1_000_000)
	f.repo.recovered = decimal.NewFromInt(500_000)
	svc.approvals = nil

	if _, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000),
	}, writeOffActor()); err != nil {
		t.Fatalf("pemulihan tepat pada batas harus boleh: %v", err)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal recovery %d, ingin 1", len(f.posting.requests))
	}
}

// Kredit hapus buku lama yang nilai hapus bukunya tidak dapat dipastikan (0) tidak
// boleh dipulihkan tanpa batas: ditolak dengan pesan jelas.
func TestRecoverWrittenOffLoan_MenolakNilaiHapusBukuTidakDiketahui(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	f.repo.loan.WrittenOffAmount = decimal.Zero
	svc.approvals = nil

	_, err := svc.RecoverWrittenOffLoan(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(100_000),
	}, writeOffActor())
	if !errors.Is(err, domain.ErrWriteOffAmountUnavailable) {
		t.Fatalf("nilai hapus buku tidak diketahui harus ditolak, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// Kredit yang belum Macet tidak boleh dihapus buku (POJK 1/2024 Pasal 42 ayat (1)).
// Penolakan harus menyebut syaratnya (ErrWriteOffNotMacet), bukan galat umum.
func TestWriteOffLoan_RejectsNonMacet(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Collectibility = domain.CollectibilityKol4

	_, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "coba", CollectionEfforts: "telepon",
	}, writeOffActor())
	if !errors.Is(err, domain.ErrWriteOffNotMacet) {
		t.Fatalf("kredit Diragukan harus ditolak sebagai bukan macet, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 || f.repo.loan.Status != domain.LoanStatusDisbursed {
		t.Fatalf("tidak boleh ada jurnal/status berubah, dapat %d jurnal status %s",
			len(f.posting.requests), f.repo.loan.Status)
	}
}

// Cadangan yang belum 100% menolak hapus buku walaupun kualitasnya sudah Macet.
func TestWriteOffLoan_RejectsIncompleteReserve(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.RequiredPPAP = decimal.NewFromInt(1_000_000) // < 2.5 juta

	_, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "coba", CollectionEfforts: "telepon",
	}, writeOffActor())
	if !errors.Is(err, domain.ErrWriteOffReserveIncomplete) {
		t.Fatalf("cadangan belum 100%% harus ditolak, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// Nilai tercatat hanya dihitung dari pokok dikurangi saldo kerugian restrukturisasi;
// cadangan yang menutup nilai tercatat itu sudah dianggap 100%.
func TestWriteOffLoan_ReserveCountsCarryingAmount(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.RestructureLossBalance = decimal.NewFromInt(500_000)
	f.repo.loan.RequiredPPAP = decimal.NewFromInt(2_000_000) // 2.5jt - 0.5jt
	svc.approvals = nil

	if _, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "coba", CollectionEfforts: "telepon",
	}, writeOffActor()); err != nil {
		t.Fatalf("cadangan atas nilai tercatat harus diterima, dapat: %v", err)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(f.posting.requests))
	}
}

// Permintaan yang lebih kecil dari seluruh sisa pokok adalah hapus buku sebagian dan
// dilarang (POJK 1/2024 Pasal 42 ayat (2)).
func TestWriteOffLoan_RejectsPartialRequest(t *testing.T) {
	f, svc := writeOffFixture()

	_, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "coba", CollectionEfforts: "telepon",
		Amount: decimal.NewFromInt(1_000_000),
	}, writeOffActor())
	if !errors.Is(err, domain.ErrWriteOffPartial) {
		t.Fatalf("hapus buku sebagian harus ditolak, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// Upaya penagihan wajib terdokumentasi (POJK 1/2024 Pasal 43); yang kosong ditolak.
func TestWriteOffLoan_RejectsMissingCollectionEfforts(t *testing.T) {
	f, svc := writeOffFixture()

	_, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "coba",
	}, writeOffActor())
	if !errors.Is(err, domain.ErrWriteOffCollectionEffortsRequired) {
		t.Fatalf("upaya penagihan kosong harus ditolak, dapat: %v", err)
	}
}

// Bukti syarat dikirim ke pemeriksa sejak pengajuan: kualitas, DPD, cadangan, dan
// upaya penagihan harus ikut di payload persetujuan, bukan hanya di audit eksekusi.
func TestWriteOffLoan_ApprovalPayloadCarriesEvidence(t *testing.T) {
	f, svc := writeOffFixture()

	_, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit", CollectionEfforts: "somasi + kunjungan",
	}, writeOffActor())
	var pending *domain.PendingApprovalError
	if !errors.As(err, &pending) {
		t.Fatalf("harus menunggu persetujuan, dapat: %v", err)
	}
	approvals := svc.approvals.(*stubApprovals)
	payload := approvals.created[0].Payload
	if payload["collectibility"] != string(domain.CollectibilityKol5) {
		t.Fatalf("payload kualitas aset %v", payload["collectibility"])
	}
	if payload["required_ppap"] != "2500000" {
		t.Fatalf("payload cadangan %v, ingin 2500000", payload["required_ppap"])
	}
	if payload["collection_efforts"] != "somasi + kunjungan" {
		t.Fatalf("payload upaya penagihan %v", payload["collection_efforts"])
	}
}

func distinctKeys(reqs []domain.PostingRequest) map[string]bool {
	keys := map[string]bool{}
	for _, r := range reqs {
		keys[r.IdempotencyKey] = true
	}
	return keys
}

// hasWriteOffLine melaporkan apakah jurnal memuat baris akun/arah tertentu.
func hasWriteOffLine(lines []domain.PostingLine, account string, dir domain.EntryDirection) bool {
	for _, l := range lines {
		if l.AccountNumber == account && l.Direction == dir {
			return true
		}
	}
	return false
}

// Saat ckpn.enabled menyala, hapus buku melepas cadangan CKPN (10950), bukan akun PPAP
// (10900) pada pemetaan produk LOAN_WRITE_OFF. Tanpa pengarahan ini saldo CKPN kredit
// tidak pernah dilepas dan akun PPAP dasar lain ikut terpotong. Test ini gagal bila
// pengarahan tidak aktif (kaki debit tetap 10900).
func TestWriteOffLoan_CKPNMenyalaMelepasAkunCKPN(t *testing.T) {
	f, svc := writeOffFixture()
	svc.approvals = nil
	svc.config = &ckpnConfigStub{values: map[string]string{cfgCKPNEnabled: "true"}}

	if _, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit", CollectionEfforts: "somasi",
	}, writeOffActor()); err != nil {
		t.Fatalf("hapus buku dengan CKPN aktif: %v", err)
	}
	if len(f.posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(f.posting.requests))
	}
	lines := f.posting.requests[0].Lines
	if !hasWriteOffLine(lines, "10950", domain.DirectionDebit) {
		t.Fatalf("cadangan CKPN 10950 tidak dilepas saat CKPN aktif: %+v", lines)
	}
	if hasWriteOffLine(lines, "10900", domain.DirectionDebit) {
		t.Fatalf("akun PPAP 10900 masih dilepas saat CKPN aktif: %+v", lines)
	}
}

// Saat ckpn.enabled mati, perilaku lama dipertahankan: pemetaan produk melepas akun
// PPAP 10900 dan tidak menyentuh akun CKPN 10950.
func TestWriteOffLoan_CKPNMatiTetapLepasAkunPPAP(t *testing.T) {
	f, svc := writeOffFixture()
	svc.approvals = nil

	if _, err := svc.WriteOffLoan(context.Background(), domain.WriteOffLoanInput{
		LoanID: f.loanID, Reason: "debitor pailit", CollectionEfforts: "somasi",
	}, writeOffActor()); err != nil {
		t.Fatalf("hapus buku dengan CKPN mati: %v", err)
	}
	lines := f.posting.requests[0].Lines
	if !hasWriteOffLine(lines, "10900", domain.DirectionDebit) {
		t.Fatalf("perilaku lama berubah: akun PPAP 10900 tidak dilepas: %+v", lines)
	}
	if hasWriteOffLine(lines, "10950", domain.DirectionDebit) {
		t.Fatalf("akun CKPN 10950 dipakai padahal CKPN mati: %+v", lines)
	}
}

// N3: permintaan pengajuan yang melebihi batas pemulihan ditolak SAAT PENGAJUAN
// (lewat penjaga yang dipanggil handler), sehingga tidak mengisi antrean maker-checker
// (dulu 202 lalu 422 saat approve).
func TestRecoverWrittenOffLoan_MenolakMelebihiBatasSaatPengajuan(t *testing.T) {
	f, svc := writeOffFixture() // ambang 0: bila lolos, akan masuk antrean persetujuan
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	f.repo.loan.WrittenOffAmount = decimal.NewFromInt(1_000_000)
	f.repo.recovered = decimal.NewFromInt(600_000)

	err := svc.CheckRecoveryCapAtSubmission(context.Background(), domain.RecoverWrittenOffLoanInput{
		LoanID: f.loanID, RecoveryAmount: decimal.NewFromInt(500_000),
	}, writeOffActor())
	if !errors.Is(err, domain.ErrRecoveryExceedsWriteOff) {
		t.Fatalf("harus ditolak saat pengajuan, dapat: %v", err)
	}
	if approvals := svc.approvals.(*stubApprovals); len(approvals.created) != 0 {
		t.Fatalf("penjaga pengajuan tidak boleh membuat permintaan persetujuan: %d", len(approvals.created))
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// N3: batas tetap diperiksa ULANG saat persetujuan karena keadaan bisa berubah.
func TestRecoverWrittenOffLoan_MenolakMelebihiBatasSaatPersetujuan(t *testing.T) {
	f, svc := writeOffFixture()
	f.repo.loan.Status = domain.LoanStatusWrittenOff
	f.repo.loan.WrittenOffAmount = decimal.NewFromInt(1_000_000)
	f.repo.recovered = decimal.NewFromInt(600_000)

	maker := writeOffActor()
	payload := map[string]any{
		"loan_id":         f.loanID.String(),
		"loan_number":     f.repo.loan.LoanNumber,
		"recovery_amount": "500000",
		"idempotency_key": "",
		"maker_id":        maker.UserID.String(),
		"maker_username":  maker.Username,
		"maker_role":      string(maker.Role),
		"maker_branch":    maker.BranchCode,
	}
	checker := domain.Actor{UserID: uuid.New(), Username: "kabag.uji", Role: domain.RoleSupervisor, BranchCode: "001"}
	err := svc.ExecuteApproved(context.Background(), nil, ActionLoanRecovery, payload, checker)
	if !errors.Is(err, domain.ErrRecoveryExceedsWriteOff) {
		t.Fatalf("persetujuan harus memeriksa ulang batas, dapat: %v", err)
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}
