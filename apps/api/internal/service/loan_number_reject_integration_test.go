package service_test

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi terhadap PostgreSQL sungguhan untuk dua temuan simulasi produksi:
// alasan penolakan kredit tersimpan, dan nomor kredit memakai sequence sendiri
// sehingga tidak menggeser urutan referensi transaksi. Di-skip kecuali CBS_TEST_DB_DSN
// diisi, mengikuti pola moneyflow_integration_test.go yang menyediakan newMoneyEnv.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiKreditW2 -v

func w2ApplyLoan(t *testing.T, e *moneyEnv, productCode string, customerID, accountID uuid.UUID, amount decimal.Decimal) *domain.Loan {
	t.Helper()
	product, err := e.productRepo.GetByCode(e.ctx, productCode)
	if err != nil {
		t.Fatalf("membaca produk %s: %v", productCode, err)
	}
	loan, err := e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
		CustomerID:            customerID,
		ProductID:             product.ID,
		DisbursementAccountID: accountID,
		PrincipalAmount:       amount,
		TermMonths:            12,
		Purpose:               "uji integrasi w2",
	}, e.actor)
	if err != nil {
		t.Fatalf("pengajuan kredit: %v", err)
	}
	return loan
}

// Alasan penolakan yang dikirim lewat layanan harus terbaca kembali dari database dan
// tercatat di audit. Alasan kosong ditolak tanpa mengubah baris kredit.
func TestIntegrasiKreditW2TolakMenyimpanAlasan(t *testing.T) {
	e := newMoneyEnv(t)
	cust := e.newCustomer(t, "Nasabah Tolak W2", "tolak.w2@uji.local")
	accID := e.newAccount(t, cust.ID)
	loan := w2ApplyLoan(t, e, "KRD-FLAT", cust.ID, accID, decimal.NewFromInt(3_000_000))

	if _, err := e.loanSvc.RejectLoan(e.ctx, loan.ID, "   ", e.actor); !errors.Is(err, domain.ErrLoanRejectionReasonRequired) {
		t.Fatalf("penolakan tanpa alasan: mau ErrLoanRejectionReasonRequired, dapat %v", err)
	}

	reason := "agunan tidak memenuhi syarat"
	rejected, err := e.loanSvc.RejectLoan(e.ctx, loan.ID, reason, e.actor)
	if err != nil {
		t.Fatalf("RejectLoan: %v", err)
	}
	if rejected.RejectionReason != reason {
		t.Fatalf("alasan pada hasil %q, ingin %q", rejected.RejectionReason, reason)
	}

	var status, dbReason string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT status::text, rejection_reason FROM loans WHERE id = $1`, loan.ID).
		Scan(&status, &dbReason); err != nil {
		t.Fatalf("membaca kredit: %v", err)
	}
	if status != string(domain.LoanStatusRejected) {
		t.Fatalf("status di database %s, ingin REJECTED", status)
	}
	if dbReason != reason {
		t.Fatalf("alasan di database %q, ingin %q", dbReason, reason)
	}

	var auditReason, auditMetadataReason string
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT changes->>'reason', metadata->>'reason' FROM audit_logs
		WHERE resource_type = 'loan' AND resource_id = $1 AND action = 'REJECT_LOAN'
		ORDER BY created_at DESC LIMIT 1`, loan.ID.String()).Scan(&auditReason, &auditMetadataReason); err != nil {
		t.Fatalf("membaca audit penolakan: %v", err)
	}
	if auditReason != reason {
		t.Fatalf("alasan pada audit changes %q, ingin %q", auditReason, reason)
	}
	// Pembaca audit lama membaca metadata; alasan harus terbaca di sana juga.
	if auditMetadataReason != reason {
		t.Fatalf("alasan pada audit metadata %q, ingin %q", auditMetadataReason, reason)
	}

	// Jalur baca repositori (kolom metadata ikut dipetakan) juga harus membawa alasan.
	auditEvents, err := postgres.NewAuditRepository(e.db).List(e.ctx, "loan", loan.ID.String(), 10)
	if err != nil {
		t.Fatalf("membaca audit lewat repositori: %v", err)
	}
	ditemukan := false
	for _, ev := range auditEvents {
		if ev.Action != "REJECT_LOAN" {
			continue
		}
		ditemukan = true
		if got, _ := ev.Metadata["reason"].(string); got != reason {
			t.Fatalf("alasan metadata dari repositori audit %q, ingin %q", got, reason)
		}
	}
	if !ditemukan {
		t.Fatal("audit REJECT_LOAN tidak ditemukan lewat repositori audit")
	}
}

// Nomor kredit memakai sequence loan_number_seq (prefix KRD/PMB) dan pengajuan kredit
// tidak menyentuh journal_reference_seq. Dibandingkan dua pembangkit: kredit vs jurnal.
func TestIntegrasiKreditW2NomorTidakMenggeserReferensiTransaksi(t *testing.T) {
	e := newMoneyEnv(t)
	cust := e.newCustomer(t, "Nasabah Nomor W2", "nomor.w2@uji.local")
	accID := e.newAccount(t, cust.ID)

	seqBefore := w2PrimeJournalSeq(t, e)
	loan := w2ApplyLoan(t, e, "KRD-FLAT", cust.ID, accID, decimal.NewFromInt(3_000_000))
	seqAfterApply := w2PeekJournalSeq(t, e)

	if !regexp.MustCompile(`^KRD-\d{8}-\d{6}$`).MatchString(loan.LoanNumber) {
		t.Fatalf("nomor kredit %q tidak memakai format KRD-YYYYMMDD-NNNNNN", loan.LoanNumber)
	}
	if seqAfterApply != seqBefore {
		t.Fatalf("pengajuan kredit menggeser journal_reference_seq dari %d ke %d", seqBefore, seqAfterApply)
	}

	// Pembangkit referensi transaksi tetap memakai sequence jurnal, dan nomornya
	// berbeda domain dari nomor kredit.
	ref, err := postgres.NewReferenceGenerator(e.db).Next(domain.TxTypeTransferInternal, time.Now().UTC())
	if err != nil {
		t.Fatalf("membuat referensi transaksi: %v", err)
	}
	if !regexp.MustCompile(`^TRF-\d{8}-\d{6}$`).MatchString(ref) {
		t.Fatalf("referensi transaksi %q tidak memakai format TRF-YYYYMMDD-NNNNNN", ref)
	}
	if seqNow := w2PeekJournalSeq(t, e); seqNow == seqBefore {
		t.Fatal("pembangkit referensi transaksi tidak menaikkan journal_reference_seq")
	}

	// Produk syariah memakai prefix berbeda.
	syarCust := e.newCustomer(t, "Nasabah Syariah W2", "syariah.w2@uji.local")
	syarAcc := e.newAccount(t, syarCust.ID)
	syarLoan := w2ApplyLoan(t, e, "PMB-MURABAHAH", syarCust.ID, syarAcc, decimal.NewFromInt(3_000_000))
	if !regexp.MustCompile(`^PMB-\d{8}-\d{6}$`).MatchString(syarLoan.LoanNumber) {
		t.Fatalf("nomor pembiayaan syariah %q tidak berprefix PMB", syarLoan.LoanNumber)
	}
}

// w2PrimeJournalSeq memastikan sequence sudah pernah dipakai (is_called=true) sebelum
// dijadikan acuan. Tanpa ini, pengambilan pertama cukup mengubah is_called tanpa
// menaikkan last_value sehingga pergeseran tidak terdeteksi.
func w2PrimeJournalSeq(t *testing.T, e *moneyEnv) int64 {
	t.Helper()
	var v int64
	if err := e.db.QueryRowContext(context.Background(),
		`SELECT nextval('journal_reference_seq')`).Scan(&v); err != nil {
		t.Fatalf("memanaskan journal_reference_seq: %v", err)
	}
	return w2PeekJournalSeq(t, e)
}

func w2PeekJournalSeq(t *testing.T, e *moneyEnv) int64 {
	t.Helper()
	var v int64
	if err := e.db.QueryRowContext(context.Background(),
		`SELECT last_value FROM journal_reference_seq`).Scan(&v); err != nil {
		t.Fatalf("membaca journal_reference_seq: %v", err)
	}
	return v
}
