package service_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi CKPN per unit usaha syariah terhadap PostgreSQL sungguhan. Di-skip
// kecuali CBS_TEST_DB_DSN diisi, mengikuti pola ckpn_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiCKPNSyariah -v
//
// Membuktikan tiga kasus pada database nyata:
//   (a) kredit syariah + kunci ckpn.coa.*.syariah terisi -> jurnal memakai akun syariah;
//   (b) kunci syariah kosong -> jurnal memakai kunci global 50301/10950 (perilaku lama);
//   (c) kode akun syariah tidak ada di bagan akun -> kredit ditolak dengan galat jelas.

// syariahGlobalCOA dan syariahUnitCOA adalah akun yang benar-benar di-seed migrasi
// 000042: 50301/10950 konvensional, 15901/11950 pembiayaan syariah.
const (
	syariahGlobalExpense  = "50301"
	syariahGlobalReserve  = "10950"
	syariahUnitExpense    = "15901"
	syariahUnitReserve    = "11950"
	syariahMissingExpense = "99999"
)

// syariahCKPNEnv menyiapkan satu kredit pembiayaan syariah di cabang uji tersendiri
// beserta aktor yang terbatas pada cabang itu, lalu mengembalikan service CKPN dan
// nomor kredit. Aktor terbatas membuat ringkasan hanya memuat kredit uji ini.
func syariahCKPNEnv(t *testing.T, productCode string) (*moneyEnv, domain.CKPNService, *domain.Loan, domain.Actor) {
	t.Helper()
	e := newMoneyEnv(t)

	branchCode := ckpnTestBranchCode("S")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji CKPN Syariah")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujickpnsy", Role: domain.RoleAdmin, BranchCode: branchCode}

	cust := e.newCustomer(t, "Nasabah CKPN Syariah", fmt.Sprintf("ckpn-sy-%d@uji.local", time.Now().UnixNano()))
	acc := e.newSyariahAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseWithProductAs(t, actor, productCode, cust.ID, acc, decimal.NewFromInt(10_000_000), 12)
	if loan.BranchCode != branchCode {
		t.Fatalf("cabang kredit %q, mau %q", loan.BranchCode, branchCode)
	}

	// State kredit dipaksa seperti hasil jalur PPAP: golongan 3, tunggakan 100 hari,
	// PPKA 1.000.000. Target CKPN = 10.000.000 x 10% x 50% = 500.000.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility = '3_KURANG_LANCAR', dpd = 100, required_ppap = $1
		WHERE id = $2`, decimal.NewFromInt(1_000_000), loan.ID); err != nil {
		t.Fatalf("menyiapkan state kredit: %v", err)
	}

	setCKPNConfig(t, e, "ckpn.enabled", "true")
	setCKPNConfig(t, e, "ckpn.pd.3", "0.10")
	setCKPNConfig(t, e, "ckpn.lgd", "0.50")
	setCKPNConfig(t, e, "ckpn.coa.expense", syariahGlobalExpense)
	setCKPNConfig(t, e, "ckpn.coa.reserve", syariahGlobalReserve)
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.coa.expense.syariah", "")
		setCKPNConfig(t, e, "ckpn.coa.reserve.syariah", "")
	})

	return e, newCKPNSvcForTest(e), loan, actor
}

// newSyariahAccountInBranch menyisipkan rekening tabungan syariah (COA 12100) pada
// cabang tertentu agar pencairan PMB lolos pemeriksaan kecocokan buku dan kredit
// mewarisi cabang itu.
func (e *moneyEnv) newSyariahAccountInBranch(t *testing.T, customerID, branchID uuid.UUID) uuid.UUID {
	t.Helper()
	accountNumber := fmt.Sprintf("E2E-CKPNSY-%d", time.Now().UnixNano())
	var id uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `
		INSERT INTO accounts (account_number, coa_id, account_type, status, balance, available_balance, currency, customer_id, branch_id)
		VALUES ($1, (SELECT id FROM chart_of_accounts WHERE code = '12100'),
		        'SAVINGS', 'ACTIVE', 0, 0, 'IDR', $2, $3)
		RETURNING id`, accountNumber, customerID, branchID).Scan(&id); err != nil {
		t.Fatalf("menyiapkan rekening syariah uji: %v", err)
	}
	return id
}

// disburseWithProductAs menjalankan siklus produksi penuh produk tertentu dengan aktor
// tertentu, sehingga kredit mewarisi cabang aktor/rekening.
func (e *moneyEnv) disburseWithProductAs(t *testing.T, actor domain.Actor, productCode string, customerID, accountID uuid.UUID, amount decimal.Decimal, term int) *domain.Loan {
	t.Helper()
	product, err := e.productRepo.GetByCode(e.ctx, productCode)
	if err != nil {
		t.Fatalf("membaca produk kredit %s: %v", productCode, err)
	}
	loan, err := e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
		CustomerID:            customerID,
		ProductID:             product.ID,
		DisbursementAccountID: accountID,
		PrincipalAmount:       amount,
		TermMonths:            term,
		Purpose:               "uji integrasi CKPN syariah",
	}, actor)
	if err != nil {
		t.Fatalf("pengajuan kredit %s: %v", productCode, err)
	}
	if _, err := e.loanSvc.ApproveLoan(e.ctx, loan.ID, actor); err != nil {
		t.Fatalf("persetujuan kredit %s: %v", productCode, err)
	}
	disbursed, err := e.loanSvc.DisburseLoan(e.ctx, loan.ID, actor)
	if err != nil {
		t.Fatalf("pencairan kredit %s: %v", productCode, err)
	}
	return disbursed
}

// (a) Kedua kunci syariah terisi: jurnal memakai akun syariah 15901/11950.
func TestIntegrasiCKPNSyariahKunciTerisiPakaiAkunSyariah(t *testing.T) {
	e, svc, loan, actor := syariahCKPNEnv(t, "PMB-MURABAHAH")
	setCKPNConfig(t, e, "ckpn.coa.expense.syariah", syariahUnitExpense)
	setCKPNConfig(t, e, "ckpn.coa.reserve.syariah", syariahUnitReserve)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := svc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}
	item := ckpnItem(t, summary, loan.ID)
	if !item.CKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("CKPN %s, mau 500000", item.CKPN)
	}

	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(500_000))
	// Akun yang benar-benar dipakai adalah akun syariah, bukan global 50301/10950.
	assertCKPNJournal(t, e, key, syariahUnitExpense, "DEBIT", syariahUnitReserve, "CREDIT", decimal.NewFromInt(500_000))
}

// (b) Kedua kunci syariah kosong: jurnal memakai kunci global 50301/10950, identik
// dengan perilaku sebelum kunci syariah ada.
func TestIntegrasiCKPNSyariahKunciKosongPakaiAkunGlobal(t *testing.T) {
	e, svc, loan, actor := syariahCKPNEnv(t, "PMB-MURABAHAH")
	setCKPNConfig(t, e, "ckpn.coa.expense.syariah", "")
	setCKPNConfig(t, e, "ckpn.coa.reserve.syariah", "")
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := svc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}
	if summary.Failed != 0 {
		t.Fatalf("kunci syariah kosong tidak boleh menggagalkan kredit: %+v", summary.Failures)
	}

	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(500_000))
	assertCKPNJournal(t, e, key, syariahGlobalExpense, "DEBIT", syariahGlobalReserve, "CREDIT", decimal.NewFromInt(500_000))
}

// (c) Kode akun syariah tidak ada di bagan akun: kredit ditolak dengan galat jelas dan
// tidak ada jurnal yang ditulis.
func TestIntegrasiCKPNSyariahKodeAkunTidakAdaDitolak(t *testing.T) {
	e, svc, loan, actor := syariahCKPNEnv(t, "PMB-MURABAHAH")
	setCKPNConfig(t, e, "ckpn.coa.expense.syariah", syariahMissingExpense)
	setCKPNConfig(t, e, "ckpn.coa.reserve.syariah", syariahUnitReserve)
	asOf := ckpnTestAsOf()
	e.recordPPAPRun(t, asOf)

	summary, err := svc.Run(e.ctx, asOf, actor)
	if err != nil {
		t.Fatalf("Run CKPN: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("akun tidak ada harus menggagalkan kredit, dapat failed=%d processed=%d", summary.Failed, summary.Processed)
	}
	msg := summary.Failures[0].Error
	if !strings.Contains(msg, syariahMissingExpense) || !strings.Contains(msg, domain.ErrCKPNExpenseNotFound.Error()) {
		t.Fatalf("pesan gagal %q harus menyebut kode %s dan galat akun tidak ditemukan", msg, syariahMissingExpense)
	}

	// Tidak boleh ada jurnal yang menunjuk akun tidak ada.
	key := ckpnIdempotencyKey(loan.LoanNumber, asOf, decimal.NewFromInt(500_000))
	if n := countJournals(t, e, key); n != 0 {
		t.Fatalf("kredit dengan akun tidak ada tidak boleh dijurnal, dapat %d", n)
	}
	// required_ckpn juga tidak boleh tersimpan, karena transaksi kredit dibatalkan.
	if got := e.loanDecimal(t, loan.ID, "required_ckpn"); !got.IsZero() {
		t.Fatalf("required_ckpn %s, mau nol karena kredit gagal", got)
	}
}
