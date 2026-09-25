package service_test

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/crypto"
	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi penegakan buku COA pada jalur TULIS terhadap PostgreSQL sungguhan.
// Auditor menemukan jalur baca sudah tertutup (book_scope_integration_test.go), tetapi
// jalur tulis masih terbuka: pegawai satu buku dapat mengubah data buku lain. Berkas
// ini membuktikan setiap operasi tulis yang dijaga menolak percobaan lintas buku, dan
// tetap mengizinkan aktor lintas buku serta operasi buku yang sama.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiBukuTulis -v
//
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola moneyflow_integration_test.go.

// bookWriteEnv membangun grafik service penuh di atas satu koneksi database, karena
// moneyEnv (jalur uang) hanya menyediakan sebagian service.
type bookWriteEnv struct {
	*moneyEnv
	ledgerSvc     domain.LedgerService
	depositSvc    domain.DepositService
	ckpnSvc       domain.CKPNService
	batchSvc      domain.BatchProcessService
	mcSvc         domain.MakerCheckerService
	collateralSvc domain.CollateralService
	docSvc        domain.DocumentService
	ledgerRepo    *postgres.LedgerRepository
	accountRepo   *postgres.AccountRepository
	ckpnRepo      *postgres.CKPNRepository
	ppapRepo      *postgres.PPAPRepository
}

func newBookWriteEnv(t *testing.T) *bookWriteEnv {
	t.Helper()
	me := newMoneyEnv(t)
	db := me.db
	configSvc, productRepo, loanRepo := me.configSvc, me.productRepo, me.loanRepo

	ledgerRepo := postgres.NewLedgerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	customerRepo := postgres.NewCustomerRepository(db)
	branchRepo := postgres.NewBranchRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	dateRepo := postgres.NewBusinessDateRepository(db)
	referenceGen := postgres.NewReferenceGenerator(db)
	numberingRepo := postgres.NewNumberingRepository(db)
	depositRepo := postgres.NewDepositRepository(db)
	yearEndRepo := postgres.NewYearEndRepository(db)
	configRepo := postgres.NewSystemConfigRepository(db)

	postingSvc := service.NewPostingService(db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(productRepo, ledgerRepo, postingSvc)
	executors := service.NewExecutorRegistry()
	mcSvc := service.NewMakerCheckerService(db, postgres.NewMakerCheckerRepository(db), auditRepo, configSvc, executors, dateRepo, branchRepo)
	limitSvc := service.NewTransactionLimitService(configSvc, ledgerRepo, dateRepo)

	ledgerSvc := service.NewLedgerService(db, ledgerRepo, accountRepo, productRepo, ledgerRepo, postingSvc, configSvc, limitSvc, mcSvc, dateRepo, auditRepo)
	executors.Register(service.ActionDeposit, ledgerSvc)
	executors.Register(service.ActionWithdraw, ledgerSvc)
	executors.Register(service.ActionTransfer, ledgerSvc)
	executors.Register(service.ActionReverse, ledgerSvc)

	depositSvc := service.NewDepositService(db, depositRepo, productRepo, accountRepo, ledgerRepo, customerRepo, branchRepo, numberingRepo, poster, postingSvc, ledgerRepo, configSvc, limitSvc, mcSvc, auditRepo)
	executors.Register(service.ActionPlaceDeposit, depositSvc)

	runMarker := service.NewPPAPRunMarker(configRepo)
	ckpnSvc := service.NewCKPNService(db, postgres.NewCKPNRepository(db), productRepo, ledgerRepo, poster, postingSvc, configSvc, loanRepo, runMarker,
		service.NewCKPNIndividualService(loanRepo, postgres.NewCKPNIndividualRepository(db), configSvc))
	batchSvc := service.NewBatchProcessService(dateRepo, nil, nil, yearEndRepo, postingSvc, ledgerRepo, configSvc, db, nil, nil, nil, nil, nil, nil, postgres.NewEODStepRepository(db))
	collateralSvc := service.NewCollateralService(me.collateralRepo, configSvc, branchRepo, auditRepo)

	cipher, err := crypto.NewCipher("e2e", base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef")), nil)
	if err != nil {
		t.Fatalf("membangun cipher: %v", err)
	}
	docSvc := service.NewDocumentService(ledgerRepo, accountRepo, loanRepo, customerRepo, postgres.NewBankProfileRepository(db), cipher)

	return &bookWriteEnv{
		moneyEnv:      me,
		ledgerSvc:     ledgerSvc,
		depositSvc:    depositSvc,
		ckpnSvc:       ckpnSvc,
		batchSvc:      batchSvc,
		mcSvc:         mcSvc,
		collateralSvc: collateralSvc,
		docSvc:        docSvc,
		ledgerRepo:    ledgerRepo,
		accountRepo:   accountRepo,
		ckpnRepo:      postgres.NewCKPNRepository(db),
		ppapRepo:      postgres.NewPPAPRepository(db),
	}
}

// ---------- aktor uji ----------

func (e *bookWriteEnv) convTeller() domain.Actor {
	return domain.Actor{UserID: e.actor.UserID, Username: "teller.konvensional", Role: domain.RoleTeller, BranchCode: "001", Book: domain.BookConventional}
}

func (e *bookWriteEnv) syrTeller() domain.Actor {
	return domain.Actor{UserID: e.actor.UserID, Username: "teller.syariah", Role: domain.RoleTeller, BranchCode: "001", Book: domain.BookSyariah}
}

func (e *bookWriteEnv) auditorActor() domain.Actor {
	return domain.Actor{UserID: e.actor.UserID, Username: "auditor.buku", Role: domain.RoleAuditor, BranchCode: "001", Book: domain.BookConventional}
}

func (e *bookWriteEnv) setBalance(t *testing.T, accountID uuid.UUID, amount decimal.Decimal) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE accounts SET balance = $2, available_balance = $2 WHERE id = $1`, accountID, amount); err != nil {
		t.Fatalf("menyetel saldo rekening: %v", err)
	}
}

func (e *bookWriteEnv) syariahCustomerAccount(t *testing.T) (uuid.UUID, string) {
	t.Helper()
	cust := e.newCustomer(t, "Nasabah Tulis Syariah", fmt.Sprintf("tulis-syr-%d@uji.local", time.Now().UnixNano()))
	accID := e.newSyariahAccount(t, cust.ID)
	return accID, e.accountNumberFor(t, accID)
}

// ---------- Temuan 1: tulis kas lintas buku ----------

func TestIntegrasiBukuTulisSetoranLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	accSyr, accSyrNum := e.syariahCustomerAccount(t)

	_, err := e.ledgerSvc.Deposit(e.ctx, domain.DepositRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-DEP-REJECT-%d", time.Now().UnixNano()),
		Actor:          e.convTeller(),
	})
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("setoran lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.accountBalance(t, accSyr); !got.IsZero() {
		t.Fatalf("saldo rekening syariah berubah %s meski setoran ditolak", got)
	}

	// Aktor lintas buku tetap boleh menyetor ke buku syariah.
	if _, err := e.ledgerSvc.Deposit(e.ctx, domain.DepositRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-DEP-ALLOW-%d", time.Now().UnixNano()),
		Actor:          e.auditorActor(),
	}); err != nil {
		t.Fatalf("setoran oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisPenarikanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	accSyr, accSyrNum := e.syariahCustomerAccount(t)
	e.setBalance(t, accSyr, idr(5_000_000))

	_, err := e.ledgerSvc.Withdraw(e.ctx, domain.WithdrawRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-WD-REJECT-%d", time.Now().UnixNano()),
		Actor:          e.convTeller(),
	})
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("penarikan lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.accountBalance(t, accSyr); !got.Equal(idr(5_000_000)) {
		t.Fatalf("saldo rekening syariah berubah %s meski penarikan ditolak", got)
	}

	if _, err := e.ledgerSvc.Withdraw(e.ctx, domain.WithdrawRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-WD-ALLOW-%d", time.Now().UnixNano()),
		Actor:          e.auditorActor(),
	}); err != nil {
		t.Fatalf("penarikan oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisTransferLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	cust := e.newCustomer(t, "Transfer Lintas Buku", "")
	accConv := e.newAccount(t, cust.ID)
	_, accSyrNum := e.syariahCustomerAccount(t)
	e.setBalance(t, accConv, idr(5_000_000))

	_, err := e.ledgerSvc.TransferInternal(e.ctx, domain.TransferRequest{
		SourceAccountNumber:      e.accountNumberFor(t, accConv),
		DestinationAccountNumber: accSyrNum,
		Amount:                   idr(1_000_000),
		Currency:                 "IDR",
		IdempotencyKey:           fmt.Sprintf("CB-TRF-REJECT-%d", time.Now().UnixNano()),
		Actor:                    e.convTeller(),
	})
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("transfer lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.accountBalance(t, accConv); !got.Equal(idr(5_000_000)) {
		t.Fatalf("saldo rekening sumber berubah %s meski transfer ditolak", got)
	}

	if _, err := e.ledgerSvc.TransferInternal(e.ctx, domain.TransferRequest{
		SourceAccountNumber:      e.accountNumberFor(t, accConv),
		DestinationAccountNumber: accSyrNum,
		Amount:                   idr(1_000_000),
		Currency:                 "IDR",
		IdempotencyKey:           fmt.Sprintf("CB-TRF-ALLOW-%d", time.Now().UnixNano()),
		Actor:                    e.auditorActor(),
	}); err != nil {
		t.Fatalf("transfer oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisPembatalanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	_, accSyrNum := e.syariahCustomerAccount(t)

	jurnal, err := e.ledgerSvc.Deposit(e.ctx, domain.DepositRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-REV-SRC-%d", time.Now().UnixNano()),
		Actor:          e.auditorActor(),
	})
	if err != nil {
		t.Fatalf("menyiapkan jurnal syariah: %v", err)
	}

	_, err = e.ledgerSvc.Reverse(e.ctx, domain.ReversalRequest{
		Reference: jurnal.ReferenceNumber,
		Reason:    "coba lintas buku",
		Actor:     e.convTeller(),
	})
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("pembatalan lintas buku: %v, mau ErrCrossBookAccess", err)
	}

	// Aktor lintas buku lain (bukan pencatat) boleh membatalkan.
	if _, err := e.ledgerSvc.Reverse(e.ctx, domain.ReversalRequest{
		Reference: jurnal.ReferenceNumber,
		Reason:    "pembatalan sah aktor lintas buku",
		Actor:     e.actor,
	}); err != nil {
		t.Fatalf("pembatalan oleh aktor lintas buku: %v", err)
	}
}

// ---------- Temuan 2: tulis kredit lintas buku ----------

// syariahLoan mengajukan, menyetujui, dan mencairkan PMB MURABAHAH memakai aktor
// lintas buku, lalu mengembalikan kredit yang sudah DISBURSED.
func (e *bookWriteEnv) syariahLoan(t *testing.T) *domain.Loan {
	t.Helper()
	cust := e.newCustomer(t, "Nasabah Tulis Kredit Syariah", fmt.Sprintf("tulis-krd-syr-%d@uji.local", time.Now().UnixNano()))
	acc := e.newSyariahAccount(t, cust.ID)
	return e.disburseWithProduct(t, "PMB-MURABAHAH", cust.ID, acc, idr(10_000_000), 6)
}

func TestIntegrasiBukuTulisKreditPersetujuanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	cust := e.newCustomer(t, "Persetujuan Kredit Syariah", "")
	acc := e.newSyariahAccount(t, cust.ID)
	product, err := e.productRepo.GetByCode(e.ctx, "PMB-MURABAHAH")
	if err != nil {
		t.Fatalf("membaca produk: %v", err)
	}
	loan, err := e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
		CustomerID: cust.ID, ProductID: product.ID, DisbursementAccountID: acc,
		PrincipalAmount: idr(5_000_000), TermMonths: 6, Purpose: "uji lintas buku",
	}, e.actor)
	if err != nil {
		t.Fatalf("pengajuan: %v", err)
	}

	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("persetujuan lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusPendingApproval) {
		t.Fatalf("status berubah menjadi %q meski persetujuan ditolak", got)
	}
	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, e.actor); err != nil {
		t.Fatalf("persetujuan oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditPencairanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	cust := e.newCustomer(t, "Pencairan Kredit Syariah", "")
	acc := e.newSyariahAccount(t, cust.ID)
	product, err := e.productRepo.GetByCode(e.ctx, "PMB-MURABAHAH")
	if err != nil {
		t.Fatalf("membaca produk: %v", err)
	}
	loan, err := e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
		CustomerID: cust.ID, ProductID: product.ID, DisbursementAccountID: acc,
		PrincipalAmount: idr(5_000_000), TermMonths: 6, Purpose: "uji lintas buku",
	}, e.actor)
	if err != nil {
		t.Fatalf("pengajuan: %v", err)
	}
	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, e.actor); err != nil {
		t.Fatalf("persetujuan: %v", err)
	}

	if _, err := e.loanSvc.DisburseLoan(e.ctx, loan.ID, e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("pencairan lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusApproved) {
		t.Fatalf("status berubah menjadi %q meski pencairan ditolak", got)
	}
	if _, err := e.loanSvc.DisburseLoan(e.ctx, loan.ID, e.actor); err != nil {
		t.Fatalf("pencairan oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditAngsuranLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan := e.syariahLoan(t)

	_, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{LoanID: loan.ID, InstallmentNo: 1}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("angsuran lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if s := e.schedule(t, loan.ID, 1); s.Status == domain.InstallmentStatusPaid {
		t.Fatal("angsuran terbayar meski pembayaran ditolak")
	}
	if _, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{LoanID: loan.ID, InstallmentNo: 1}, e.auditorActor()); err != nil {
		t.Fatalf("angsuran oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditRestrukturisasiLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan := e.syariahLoan(t)

	_, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
		LoanID: loan.ID, NewTermMonths: 12, Reason: "coba lintas buku",
	}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("restrukturisasi lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.loanInt(t, loan.ID, "term_months"); got == 12 {
		t.Fatal("jangka waktu berubah meski restrukturisasi ditolak")
	}
	if _, err := e.loanSvc.RestructureLoan(e.ctx, domain.RestructureLoanInput{
		LoanID: loan.ID, NewTermMonths: 12, Reason: "restrukturisasi sah aktor lintas buku",
	}, e.auditorActor()); err != nil {
		t.Fatalf("restrukturisasi oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditHapusBukuLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan := e.syariahLoan(t)

	_, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{LoanID: loan.ID, Reason: "coba lintas buku"}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("hapus buku lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusDisbursed) {
		t.Fatalf("status berubah menjadi %q meski hapus buku ditolak", got)
	}
	// Hapus buku selalu melalui persetujuan (threshold 0); yang penting aktor lintas
	// buku TIDAK tertahan oleh penjagaan buku.
	if _, err := e.loanSvc.WriteOffLoan(e.ctx, domain.WriteOffLoanInput{LoanID: loan.ID, Reason: "hapus buku sah"}, e.auditorActor()); errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("hapus buku oleh aktor lintas buku tertahan penjagaan buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditRecoveryLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan := e.syariahLoan(t)
	// Siapkan status hapus buku langsung: penjagaan buku berada di depan, sebelum
	// validasi status, sehingga setup tidak perlu menembus alur persetujuan.
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loans SET status = $2, outstanding_principal = 0 WHERE id = $1`,
		loan.ID, string(domain.LoanStatusWrittenOff)); err != nil {
		t.Fatalf("menyiapkan status hapus buku: %v", err)
	}

	_, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: idr(500_000),
	}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("recovery lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	// Recovery selalu melalui persetujuan (threshold 0); aktor lintas buku tidak
	// tertahan penjagaan buku.
	if _, err := e.loanSvc.RecoverWrittenOffLoan(e.ctx, domain.RecoverWrittenOffLoanInput{
		LoanID: loan.ID, RecoveryAmount: idr(500_000),
	}, e.auditorActor()); errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("recovery oleh aktor lintas buku tertahan penjagaan buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditPembatalanPencairanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan := e.syariahLoan(t)

	_, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{LoanID: loan.ID, Reason: "coba lintas buku"}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("pembatalan pencairan lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.loanStatus(t, loan.ID); got != string(domain.LoanStatusDisbursed) {
		t.Fatalf("status berubah menjadi %q meski pembatalan ditolak", got)
	}
	if _, err := e.loanSvc.CancelDisbursementLoan(e.ctx, domain.CancelLoanInput{LoanID: loan.ID, Reason: "pembatalan sah"}, e.auditorActor()); err != nil {
		t.Fatalf("pembatalan oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuTulisKreditKoreksiNominalLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loan := e.syariahLoan(t)

	_, err := e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
		LoanID: loan.ID, NewAmount: idr(8_000_000), Reason: "coba lintas buku",
	}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("koreksi nominal lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if got := e.loanDecimal(t, loan.ID, "principal_amount"); !got.Equal(idr(10_000_000)) {
		t.Fatalf("nominal berubah menjadi %s meski koreksi ditolak", got)
	}
	if _, err := e.loanSvc.CorrectLoanAmount(e.ctx, domain.CorrectLoanAmountInput{
		LoanID: loan.ID, NewAmount: idr(8_000_000), Reason: "koreksi sah",
	}, e.auditorActor()); err != nil {
		t.Fatalf("koreksi oleh aktor lintas buku: %v", err)
	}
}

// Aktor satu buku tetap dapat mengubah kredit buku yang sama.
func TestIntegrasiBukuTulisKreditBukuSamaTetapBisa(t *testing.T) {
	e := newBookWriteEnv(t)
	cust := e.newCustomer(t, "Kredit Buku Sama", "")
	acc := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, acc, idr(5_000_000), 6)

	conv := e.convTeller()
	if _, err := e.loanSvc.PayInstallment(e.ctx, domain.PayInstallmentInput{LoanID: loan.ID, InstallmentNo: 1}, conv); err != nil {
		t.Fatalf("angsuran buku sama ditolak: %v", err)
	}
	if s := e.schedule(t, loan.ID, 1); s.Status != domain.InstallmentStatusPaid {
		t.Fatalf("status angsuran %q, mau PAID", s.Status)
	}
}

// ---------- Temuan 3: EOY menerima book dari body ----------

func TestIntegrasiBukuTulisEOYMenolakBukuLain(t *testing.T) {
	e := newBookWriteEnv(t)

	_, err := e.batchSvc.RunEOY(e.ctx, "SYARIAH", e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("EOY buku syariah oleh aktor konvensional: %v, mau ErrCrossBookAccess", err)
	}

	// Aktor lintas buku boleh menutup buku syariah.
	if _, err := e.batchSvc.RunEOY(e.ctx, "SYARIAH", e.auditorActor()); err != nil {
		t.Fatalf("EOY buku syariah oleh aktor lintas buku: %v", err)
	}

	// Book kosong oleh aktor satu buku dipaksa ke bukunya sendiri, bukan kedua buku.
	res, err := e.batchSvc.RunEOY(e.ctx, "", e.convTeller())
	if err != nil {
		t.Fatalf("EOY book kosong aktor konvensional: %v", err)
	}
	for _, b := range res.Books {
		if b.Book == domain.BookSyariah {
			t.Fatalf("EOY aktor konvensional menutup buku syariah: %+v", b)
		}
	}
}

// ---------- Temuan 4: CKPN ----------

func TestIntegrasiBukuCKPNTidakMembacaKreditBukuLain(t *testing.T) {
	e := newBookWriteEnv(t)
	custConv := e.newCustomer(t, "CKPN Konvensional", "")
	accConv := e.newAccount(t, custConv.ID)
	loanConv := e.disburse(t, custConv.ID, accConv, idr(5_000_000), 6)
	loanSyr := e.syariahLoan(t)

	convLoans, err := e.ckpnRepo.ListActiveLoans(e.ctx, e.convTeller())
	if err != nil {
		t.Fatalf("ListActiveLoans aktor konvensional: %v", err)
	}
	if !ckpnContains(convLoans, loanConv.ID) {
		t.Fatal("kredit konvensional tidak terlihat oleh aktor konvensional")
	}
	if ckpnContains(convLoans, loanSyr.ID) {
		t.Fatal("kredit syariah terlihat oleh aktor konvensional")
	}

	crossLoans, err := e.ckpnRepo.ListActiveLoans(e.ctx, e.auditorActor())
	if err != nil {
		t.Fatalf("ListActiveLoans aktor lintas buku: %v", err)
	}
	if !ckpnContains(crossLoans, loanSyr.ID) {
		t.Fatal("kredit syariah tidak terlihat oleh aktor lintas buku")
	}
}

func ckpnContains(list []domain.CKPNLoanSnapshot, id uuid.UUID) bool {
	for _, s := range list {
		if s.LoanID == id {
			return true
		}
	}
	return false
}

// ---------- Temuan 5: PPAP ----------

func TestIntegrasiBukuPPAPTidakMemprosesKreditBukuLain(t *testing.T) {
	e := newBookWriteEnv(t)
	asOf := time.Now().UTC()

	custConv := e.newCustomer(t, "PPAP Konvensional", "")
	accConv := e.newAccount(t, custConv.ID)
	loanConv := e.disburse(t, custConv.ID, accConv, idr(5_000_000), 6)
	loanSyr := e.syariahLoan(t)
	e.setOldestDueDate(t, loanConv.ID, asOf.AddDate(0, 0, -100))
	e.setOldestDueDate(t, loanSyr.ID, asOf.AddDate(0, 0, -100))

	due, err := e.ppapRepo.ListDueLoans(e.ctx, asOf, e.convTeller())
	if err != nil {
		t.Fatalf("ListDueLoans aktor konvensional: %v", err)
	}
	if !ppapContains(due, loanConv.ID) || ppapContains(due, loanSyr.ID) {
		t.Fatalf("filter PPAP salah: konvensional=%v syariah=%v",
			ppapContains(due, loanConv.ID), ppapContains(due, loanSyr.ID))
	}

	// RunDaily aktor konvensional tidak boleh menyentuh kredit syariah.
	if _, err := e.ppapSvc.RunDaily(e.ctx, asOf, e.convTeller()); err != nil {
		t.Fatalf("RunDaily PPAP aktor konvensional: %v", err)
	}
	if got := e.loanDecimal(t, loanSyr.ID, "required_ppap"); got.IsPositive() {
		t.Fatalf("required_ppap kredit syariah berubah %s oleh aktor konvensional", got)
	}
	if got := e.loanDecimal(t, loanConv.ID, "required_ppap"); !got.IsPositive() {
		t.Fatalf("required_ppap kredit konvensional tidak dihitung: %s", got)
	}

	// Preview aktor konvensional juga tidak memuat kredit syariah.
	prev, err := e.ppapSvc.Preview(e.ctx, asOf, e.convTeller())
	if err != nil {
		t.Fatalf("Preview PPAP aktor konvensional: %v", err)
	}
	if ppapSummaryContains(prev, loanSyr.ID) {
		t.Fatal("preview PPAP aktor konvensional memuat kredit syariah")
	}
}

func ppapContains(list []domain.PPAPLoanSnapshot, id uuid.UUID) bool {
	for _, s := range list {
		if s.LoanID == id {
			return true
		}
	}
	return false
}

func ppapSummaryContains(sum domain.PPAPRunSummary, id uuid.UUID) bool {
	for _, f := range sum.Failures {
		if f.LoanID == id {
			return true
		}
	}
	for _, it := range sum.Items {
		if it.LoanID == id {
			return true
		}
	}
	return false
}

// ---------- Temuan 6: maker-checker ----------

func TestIntegrasiBukuMakerCheckerMenolakBukuLain(t *testing.T) {
	e := newBookWriteEnv(t)

	req, err := e.mcSvc.CreateRequest(e.ctx, domain.CreateMakerCheckerInput{
		ActionType: "UJI_LINTAS_BUKU",
		Amount:     idr(1_000_000),
		Payload:    map[string]any{"catatan": "uji"},
	}, e.syrTeller())
	if err != nil {
		t.Fatalf("membuat pengajuan syariah: %v", err)
	}

	// Antrean buku lain tidak terlihat.
	pendingConv, err := e.mcSvc.ListPending(e.ctx, e.convTeller())
	if err != nil {
		t.Fatalf("ListPending konvensional: %v", err)
	}
	for _, p := range pendingConv {
		if p.ID == req.ID {
			t.Fatal("pengajuan syariah terlihat di antrean aktor konvensional")
		}
	}
	pendingCross, err := e.mcSvc.ListPending(e.ctx, e.auditorActor())
	if err != nil {
		t.Fatalf("ListPending lintas buku: %v", err)
	}
	if !mcContains(pendingCross, req.ID) {
		t.Fatal("pengajuan syariah tidak terlihat oleh aktor lintas buku")
	}

	// Persetujuan buku lain ditolak sebelum eksekutor dipanggil.
	if err := e.mcSvc.Approve(e.ctx, req.ID, e.convTeller(), ""); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("Approve lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	// Aktor lintas buku lolos penjagaan buku (boleh gagal karena alasan lain, mis.
	// eksekutor tidak terdaftar untuk aksi uji ini).
	if err := e.mcSvc.Approve(e.ctx, req.ID, e.auditorActor(), ""); errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("Approve aktor lintas buku ditolak karena buku: %v", err)
	}
}

func mcContains(list []domain.MakerCheckerRequest, id uuid.UUID) bool {
	for _, r := range list {
		if r.ID == id {
			return true
		}
	}
	return false
}

// ---------- Temuan 7: baca jurnal & dokumen lintas buku ----------

func TestIntegrasiBukuDaftarJurnalTidakBocorLintasBuku(t *testing.T) {
	e := newBookWriteEnv(t)
	_, accSyrNum := e.syariahCustomerAccount(t)

	jurnal, err := e.ledgerSvc.Deposit(e.ctx, domain.DepositRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-JRN-%d", time.Now().UnixNano()),
		Actor:          e.auditorActor(),
	})
	if err != nil {
		t.Fatalf("menyiapkan jurnal syariah: %v", err)
	}

	convJournals, _, err := e.ledgerSvc.ListJournals(e.ctx, 1, 100, e.convTeller())
	if err != nil {
		t.Fatalf("ListJournals konvensional: %v", err)
	}
	if journalContains(convJournals, jurnal.ReferenceNumber) {
		t.Fatal("jurnal syariah terlihat di daftar jurnal aktor konvensional")
	}

	crossJournals, _, err := e.ledgerSvc.ListJournals(e.ctx, 1, 100, e.auditorActor())
	if err != nil {
		t.Fatalf("ListJournals lintas buku: %v", err)
	}
	if !journalContains(crossJournals, jurnal.ReferenceNumber) {
		t.Fatal("jurnal syariah tidak terlihat oleh aktor lintas buku")
	}
}

func journalContains(list []domain.JournalEntry, ref string) bool {
	for _, j := range list {
		if j.ReferenceNumber == ref {
			return true
		}
	}
	return false
}

func TestIntegrasiBukuDokumenSlipTidakBocorLintasBuku(t *testing.T) {
	e := newBookWriteEnv(t)
	_, accSyrNum := e.syariahCustomerAccount(t)

	jurnal, err := e.ledgerSvc.Deposit(e.ctx, domain.DepositRequest{
		AccountNumber:  accSyrNum,
		Amount:         idr(1_000_000),
		Currency:       "IDR",
		IdempotencyKey: fmt.Sprintf("CB-DOC-%d", time.Now().UnixNano()),
		Actor:          e.auditorActor(),
	})
	if err != nil {
		t.Fatalf("menyiapkan jurnal syariah: %v", err)
	}

	if _, err := e.docSvc.GenerateDepositSlipHTML(e.ctx, jurnal.ReferenceNumber, e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("slip setoran lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if _, err := e.docSvc.GenerateDepositSlipHTML(e.ctx, jurnal.ReferenceNumber, e.auditorActor()); err != nil {
		t.Fatalf("slip setoran oleh aktor lintas buku: %v", err)
	}
}

// ---------- Temuan 8: tulis deposito ----------

func (e *bookWriteEnv) placeDeposit(t *testing.T, productCode string, actor domain.Actor, amount decimal.Decimal) *domain.Deposit {
	t.Helper()
	cust := e.newCustomer(t, "Nasabah Tulis Deposito", fmt.Sprintf("tulis-dep-%d@uji.local", time.Now().UnixNano()))
	product, err := e.productRepo.GetByCode(e.ctx, productCode)
	if err != nil {
		t.Fatalf("membaca produk deposito %s: %v", productCode, err)
	}
	dep, err := e.depositSvc.Place(e.ctx, domain.PlaceDepositInput{
		CustomerID:      cust.ID,
		ProductID:       product.ID,
		PlacementAmount: amount,
		TermMonths:      3,
		Currency:        "IDR",
		StartDate:       ptrTime(time.Now().UTC().AddDate(0, 0, -10)),
	}, actor)
	if err != nil {
		t.Fatalf("penempatan deposito %s: %v", productCode, err)
	}
	return dep
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestIntegrasiBukuDepositoTempatLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	cust := e.newCustomer(t, "Tempat Deposito Lintas Buku", "")
	product, err := e.productRepo.GetByCode(e.ctx, "DEP-SYAR")
	if err != nil {
		t.Fatalf("membaca produk deposito syariah: %v", err)
	}

	_, err = e.depositSvc.Place(e.ctx, domain.PlaceDepositInput{
		CustomerID: cust.ID, ProductID: product.ID, PlacementAmount: idr(1_000_000), TermMonths: 3, Currency: "IDR",
	}, e.convTeller())
	if !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("penempatan deposito lintas buku: %v, mau ErrCrossBookAccess", err)
	}

	if _, err := e.depositSvc.Place(e.ctx, domain.PlaceDepositInput{
		CustomerID: cust.ID, ProductID: product.ID, PlacementAmount: idr(1_000_000), TermMonths: 3, Currency: "IDR",
	}, e.auditorActor()); err != nil {
		t.Fatalf("penempatan oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuDepositoAkrualLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	dep := e.placeDeposit(t, "DEP-SYAR", e.auditorActor(), idr(1_000_000))

	if _, err := e.depositSvc.Accrue(e.ctx, dep.ID, time.Now().UTC(), e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("akrual deposito lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if _, err := e.depositSvc.Accrue(e.ctx, dep.ID, time.Now().UTC(), e.auditorActor()); err != nil {
		t.Fatalf("akrual oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuDepositoPencairanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	dep := e.placeDeposit(t, "DEP-SYAR", e.auditorActor(), idr(1_000_000))

	if _, err := e.depositSvc.MatureOrWithdraw(e.ctx, dep.ID, e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("pencairan deposito lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if _, err := e.depositSvc.MatureOrWithdraw(e.ctx, dep.ID, e.auditorActor()); err != nil {
		t.Fatalf("pencairan oleh aktor lintas buku: %v", err)
	}
}

func TestIntegrasiBukuDepositoAROLintasBukuTidakDiproses(t *testing.T) {
	e := newBookWriteEnv(t)
	dep := e.placeDeposit(t, "DEP-SYAR", e.auditorActor(), idr(1_000_000))

	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE time_deposits SET aro = TRUE, status = 'MATURED', maturity_date = CURRENT_DATE - 1 WHERE id = $1`, dep.ID); err != nil {
		t.Fatalf("menyiapkan ARO: %v", err)
	}
	var maturityBefore time.Time
	if err := e.db.QueryRowContext(e.ctx, `SELECT maturity_date FROM time_deposits WHERE id = $1`, dep.ID).Scan(&maturityBefore); err != nil {
		t.Fatalf("membaca maturity: %v", err)
	}

	if _, err := e.depositSvc.RunARO(e.ctx, time.Now().UTC(), e.convTeller()); err != nil {
		t.Fatalf("RunARO aktor konvensional: %v", err)
	}
	var maturityAfter time.Time
	if err := e.db.QueryRowContext(e.ctx, `SELECT maturity_date FROM time_deposits WHERE id = $1`, dep.ID).Scan(&maturityAfter); err != nil {
		t.Fatalf("membaca maturity setelah: %v", err)
	}
	if !maturityAfter.Equal(maturityBefore) {
		t.Fatalf("deposito syariah diperpanjang aktor konvensional: %s -> %s", maturityBefore, maturityAfter)
	}

	if _, err := e.depositSvc.RunARO(e.ctx, time.Now().UTC(), e.auditorActor()); err != nil {
		t.Fatalf("RunARO aktor lintas buku: %v", err)
	}
	var maturityCross time.Time
	if err := e.db.QueryRowContext(e.ctx, `SELECT maturity_date FROM time_deposits WHERE id = $1`, dep.ID).Scan(&maturityCross); err != nil {
		t.Fatalf("membaca maturity pasca lintas buku: %v", err)
	}
	if maturityCross.Equal(maturityBefore) {
		t.Fatal("deposito syariah tidak diperpanjang oleh aktor lintas buku")
	}
}

// ---------- Temuan 9: agunan berkunci loanId ----------

func (e *bookWriteEnv) collateralInput(loanID uuid.UUID) domain.CollateralInput {
	exists := true
	return domain.CollateralInput{
		LoanID:         loanID,
		CollateralType: domain.CollateralTanahBangunan,
		Description:    "agunan uji lintas buku",
		DocumentNumber: fmt.Sprintf("SHM-UJI-%d", time.Now().UnixNano()),
		OwnerName:      "Pemilik Uji",
		AppraisalValue: idr(1_000_000),
		AppraisalDate:  time.Now().UTC().AddDate(0, 0, -1),
		Certified:      true,
		Mortgaged:      true,
		ExistsKnown:    &exists,
		Executable:     &exists,
	}
}

func TestIntegrasiBukuAgunanLintasBukuDitolak(t *testing.T) {
	e := newBookWriteEnv(t)
	loanSyr := e.syariahLoan(t)

	if _, err := e.collateralSvc.Create(e.ctx, e.collateralInput(loanSyr.ID), e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("agunan lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if _, err := e.collateralSvc.ListByLoan(e.ctx, loanSyr.ID, e.convTeller()); !errors.Is(err, domain.ErrCrossBookAccess) {
		t.Fatalf("daftar agunan lintas buku: %v, mau ErrCrossBookAccess", err)
	}
	if _, err := e.collateralSvc.Create(e.ctx, e.collateralInput(loanSyr.ID), e.auditorActor()); err != nil {
		t.Fatalf("agunan oleh aktor lintas buku: %v", err)
	}

	// Nasabah sengaja lintas buku (CIF bersama): aktor konvensional tetap dapat
	// mencatat/membaca agunan kredit konvensional milik nasabah yang sama.
	cust := e.newCustomer(t, "Nasabah CIF Bersama", "")
	accConv := e.newAccount(t, cust.ID)
	loanConv := e.disburse(t, cust.ID, accConv, idr(5_000_000), 6)
	convCollateral, err := e.collateralSvc.Create(e.ctx, e.collateralInput(loanConv.ID), e.convTeller())
	if err != nil {
		t.Fatalf("agunan kredit konvensional ditolak: %v", err)
	}
	list, err := e.collateralSvc.ListByLoan(e.ctx, loanConv.ID, e.convTeller())
	if err != nil {
		t.Fatalf("daftar agunan kredit konvensional: %v", err)
	}
	found := false
	for _, c := range list {
		if c.ID == convCollateral.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("agunan kredit konvensional tidak terlihat oleh aktor konvensional")
	}
}
