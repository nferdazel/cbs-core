package service_test

import (
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
)

// Uji integrasi batas keamanan buku COA terhadap PostgreSQL sungguhan. Di-skip
// kecuali CBS_TEST_DB_DSN diisi, mengikuti pola moneyflow_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiFilterBuku -v
//
// Membuktikan empat kasus pada data nyata:
//   (a) aktor buku SYARIAH tidak melihat kredit/rekening/deposito konvensional;
//   (b) aktor buku CONVENTIONAL tidak melihat pembiayaan/rekening/deposito syariah;
//   (c) aktor lintas buku (AUDITOR) melihat keduanya;
//   (d) aktor lama tanpa buku (book kosong) tidak kehilangan akses.
//
// Penegakan berada di repository: endpoint /api/v1/loans, /api/v1/accounts,
// /api/v1/deposits memanggil List/ListAll/List dengan Actor yang membawa Book.

// TestIntegrasiFilterBukuKreditRekeningDeposito menyiapkan satu kredit konvensional
// dan satu pembiayaan syariah beserta rekening dan deposito tiap buku, lalu memastikan
// daftar per buku hanya memuat baris bukunya.
func TestIntegrasiFilterBukuKreditRekeningDeposito(t *testing.T) {
	e := newMoneyEnv(t)

	// ── Data konvensional ───────────────────────────────────────────────────
	custConv := e.newCustomer(t, "Nasabah Konvensional Scope", fmt.Sprintf("scope-conv-%d@uji.local", time.Now().UnixNano()))
	accConv := e.newAccount(t, custConv.ID)
	loanConv := e.disburse(t, custConv.ID, accConv, idr(5_000_000), 6)
	depConv := e.insertDeposit(t, e.accountNumberFor(t, accConv), custConv.ID, "DEP-CONV")

	// ── Data syariah ────────────────────────────────────────────────────────
	custSyr := e.newCustomer(t, "Nasabah Syariah Scope", fmt.Sprintf("scope-syr-%d@uji.local", time.Now().UnixNano()))
	accSyr := e.newSyariahAccount(t, custSyr.ID)
	loanSyr := e.disburseWithProduct(t, "PMB-MURABAHAH", custSyr.ID, accSyr, idr(5_000_000), 6)
	depSyr := e.insertDeposit(t, e.accountNumberFor(t, accSyr), custSyr.ID, "DEP-SYAR")

	loanRepo := postgres.NewLoanRepository(e.db)
	accountRepo := postgres.NewAccountRepository(e.db)
	depositRepo := postgres.NewDepositRepository(e.db)

	const branch = "001"
	base := domain.Actor{UserID: e.actor.UserID, BranchCode: branch}
	syrActor := base
	syrActor.Role, syrActor.Book = domain.RoleAO, domain.BookSyariah
	convActor := base
	convActor.Role, convActor.Book = domain.RoleAO, domain.BookConventional
	crossActor := base
	crossActor.Role, crossActor.Book = domain.RoleAuditor, domain.BookConventional
	legacyActor := base
	legacyActor.Role = domain.RoleAO // Book kosong = belum ditentukan

	type scope struct {
		name     string
		actor    domain.Actor
		wantConv bool
		wantSyr  bool
	}
	scopes := []scope{
		{"a: aktor syariah", syrActor, false, true},
		{"b: aktor konvensional", convActor, true, false},
		{"c: aktor lintas buku", crossActor, true, true},
		{"d: aktor tanpa buku", legacyActor, true, true},
	}

	for _, s := range scopes {
		t.Run("kredit/"+s.name, func(t *testing.T) {
			loans, _, err := loanRepo.List(e.ctx, 100, 0, s.actor)
			if err != nil {
				t.Fatalf("daftar kredit: %v", err)
			}
			got := loanIDSet(loans)
			if got[loanConv.ID] != s.wantConv {
				t.Errorf("kredit konvensional terlihat=%v, ingin %v", got[loanConv.ID], s.wantConv)
			}
			if got[loanSyr.ID] != s.wantSyr {
				t.Errorf("pembiayaan syariah terlihat=%v, ingin %v", got[loanSyr.ID], s.wantSyr)
			}
		})

		t.Run("rekening/"+s.name, func(t *testing.T) {
			accounts, _, err := accountRepo.ListAll(e.ctx, 100, 0, "", s.actor)
			if err != nil {
				t.Fatalf("daftar rekening: %v", err)
			}
			got := accountIDSet(accounts)
			if got[accConv] != s.wantConv {
				t.Errorf("rekening konvensional terlihat=%v, ingin %v", got[accConv], s.wantConv)
			}
			if got[accSyr] != s.wantSyr {
				t.Errorf("rekening syariah terlihat=%v, ingin %v", got[accSyr], s.wantSyr)
			}
		})

		t.Run("deposito/"+s.name, func(t *testing.T) {
			deposits, _, err := depositRepo.List(e.ctx, 100, 0, s.actor)
			if err != nil {
				t.Fatalf("daftar deposito: %v", err)
			}
			got := depositIDSet(deposits)
			if got[depConv] != s.wantConv {
				t.Errorf("deposito konvensional terlihat=%v, ingin %v", got[depConv], s.wantConv)
			}
			if got[depSyr] != s.wantSyr {
				t.Errorf("deposito syariah terlihat=%v, ingin %v", got[depSyr], s.wantSyr)
			}
		})
	}
}

func loanIDSet(loans []domain.Loan) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(loans))
	for _, l := range loans {
		set[l.ID] = true
	}
	return set
}

func accountIDSet(accounts []domain.Account) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(accounts))
	for _, a := range accounts {
		set[a.ID] = true
	}
	return set
}

func depositIDSet(deposits []domain.Deposit) map[uuid.UUID]bool {
	set := make(map[uuid.UUID]bool, len(deposits))
	for _, d := range deposits {
		set[d.ID] = true
	}
	return set
}

// accountNumberFor membaca nomor rekening dari id, dipakai menyiapkan baris deposito
// yang ber-FK ke accounts.account_number.
func (e *moneyEnv) accountNumberFor(t *testing.T, id uuid.UUID) string {
	t.Helper()
	var number string
	if err := e.db.QueryRowContext(e.ctx, `SELECT account_number FROM accounts WHERE id = $1`, id).Scan(&number); err != nil {
		t.Fatalf("membaca nomor rekening %s: %v", id, err)
	}
	return number
}

// insertDeposit menyisipkan kontrak deposito langsung pada cabang kantor pusat memakai
// produk DEP-CONV/DEP-SYAR, cukup untuk menguji filter buku di repository.
func (e *moneyEnv) insertDeposit(t *testing.T, accountNumber string, customerID uuid.UUID, productCode string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := e.db.QueryRowContext(e.ctx, `
		INSERT INTO time_deposits (account_number, customer_id, product_id, branch_id,
			placement_amount, term_months, start_date, maturity_date)
		VALUES ($1, $2, (SELECT id FROM banking_products WHERE code = $3),
			(SELECT id FROM branches WHERE code = '001'), 1000000, 3, CURRENT_DATE, CURRENT_DATE + 90)
		RETURNING id`, accountNumber, customerID, productCode).Scan(&id)
	if err != nil {
		t.Fatalf("menyiapkan deposito %s: %v", productCode, err)
	}
	return id
}

// TestIntegrasiFilterBukuBaganAkun membuktikan bagan akun disaring buku dengan semantik
// yang sama seperti kredit/rekening/deposito: aktor terikat satu buku tidak melihat bagan
// akun lini usaha lain, aktor lintas buku dan aktor lama tanpa buku melihat keduanya.
func TestIntegrasiFilterBukuBaganAkun(t *testing.T) {
	e := newMoneyEnv(t)
	repo := postgres.NewLedgerRepository(e.db)

	var syrCode, convCode string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT code FROM chart_of_accounts WHERE book = 'SYARIAH' ORDER BY code LIMIT 1`).Scan(&syrCode); err != nil {
		t.Fatalf("membaca COA syariah: %v", err)
	}
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT code FROM chart_of_accounts WHERE book = 'CONVENTIONAL' ORDER BY code LIMIT 1`).Scan(&convCode); err != nil {
		t.Fatalf("membaca COA konvensional: %v", err)
	}

	codesFor := func(actor domain.Actor) map[string]bool {
		list, err := repo.GetCOAList(e.ctx, actor)
		if err != nil {
			t.Fatalf("GetCOAList: %v", err)
		}
		codes := make(map[string]bool, len(list))
		for _, c := range list {
			codes[c.Code] = true
		}
		return codes
	}

	syr := codesFor(domain.Actor{UserID: e.actor.UserID, Role: domain.RoleAO, Book: domain.BookSyariah})
	if !syr[syrCode] {
		t.Errorf("aktor syariah tidak melihat COA syariah %s", syrCode)
	}
	if syr[convCode] {
		t.Errorf("aktor syariah melihat COA konvensional %s", convCode)
	}

	conv := codesFor(domain.Actor{UserID: e.actor.UserID, Role: domain.RoleAO, Book: domain.BookConventional})
	if !conv[convCode] {
		t.Errorf("aktor konvensional tidak melihat COA konvensional %s", convCode)
	}
	if conv[syrCode] {
		t.Errorf("aktor konvensional melihat COA syariah %s", syrCode)
	}

	cross := codesFor(domain.Actor{UserID: e.actor.UserID, Role: domain.RoleAuditor, Book: domain.BookConventional})
	if !cross[syrCode] || !cross[convCode] {
		t.Errorf("aktor lintas buku harus melihat kedua buku (syr=%v conv=%v)", cross[syrCode], cross[convCode])
	}

	legacy := codesFor(domain.Actor{UserID: e.actor.UserID, Role: domain.RoleAO})
	if !legacy[syrCode] || !legacy[convCode] {
		t.Errorf("aktor tanpa buku harus melihat kedua buku (syr=%v conv=%v)", legacy[syrCode], legacy[convCode])
	}
}
