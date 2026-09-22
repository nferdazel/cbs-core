package service_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Uji integrasi sisa kepatuhan agunan (Item 6) terhadap PostgreSQL sungguhan: dasar
// NJOP huruf d/e, pembatasan porsi agunan tunai, dan penanda kriteria penjamin
// BUMN/BUMD huruf i. Mengikuti pola moneyflow_integration_test.go: di-skip kecuali
// CBS_TEST_DB_DSN diisi, sehingga `go test ./...` tetap hijau tanpa database.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55441/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiJalurUangAgunanItem6 -v

// addCollateralItem6 mencatat agunan uji dengan atribut Pasal 20 yang eksplisit dan
// membacanya kembali dari database, supaya kolom baru benar-benar teruji round-trip.
func (e *moneyEnv) addCollateralItem6(t *testing.T, c *domain.LoanCollateral) *domain.LoanCollateral {
	t.Helper()
	c.Description = "agunan uji Item 6"
	c.DocumentNumber = fmt.Sprintf("DOC6-%d", time.Now().UnixNano())
	c.OwnerName = "Pemilik Uji Item 6"
	c.AppraisalDate = time.Now().UTC().AddDate(0, 0, -10)
	c.HaircutPercent = decimal.Zero
	c.Status = domain.CollateralActive
	c.ExistsKnown = true
	c.Executable = true
	if err := e.collateralRepo.Create(e.ctx, c, "001"); err != nil {
		t.Fatalf("mencatat agunan uji Item 6: %v", err)
	}
	got, err := e.collateralRepo.GetByID(e.ctx, c.ID)
	if err != nil {
		t.Fatalf("membaca ulang agunan uji Item 6: %v", err)
	}
	return got
}

// TestIntegrasiJalurUangAgunanItem6NJOP memverifikasi angka di database: pengurang
// huruf d/e dihitung dari NJOP (bukan taksasi), dan agunan huruf d/e tanpa NJOP
// menghasilkan nol tanpa memakai taksasi sebagai gantinya.
func TestIntegrasiJalurUangAgunanItem6NJOP(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setCollateralEnabled(t, true)

	sumberNJOP := domain.NJOPSourceTax

	// Kredit Kurang Lancar agar pengurang Pasal 20 berlaku (bukan PPKA umum).
	custD := e.newCustomer(t, "Dedi NJOP", "")
	loanD := e.disburse(t, custD.ID, e.newAccount(t, custD.ID), idr(10_000_000), 4)
	e.setOldestDueDate(t, loanD.ID, asOf.AddDate(0, 0, -100))

	// Huruf d: tanah bersertifikat TANPA hak tanggungan. NJOP 8 juta, taksasi 9 juta:
	// pengurang = 60% x 8 juta = 4,8 juta (bukan 60% x 9 juta = 5,4 juta).
	tanahD := e.addCollateralItem6(t, &domain.LoanCollateral{
		LoanID:         loanD.ID,
		CollateralType: domain.CollateralTanahBangunan,
		AppraisalValue: idr(9_000_000),
		Certified:      true,
		Mortgaged:      false,
		NJOPValue:      idr(8_000_000),
		NJOPSource:     &sumberNJOP,
	})
	if tanahD.NJOPSource == nil || *tanahD.NJOPSource != domain.NJOPSourceTax {
		t.Fatalf("asal NJOP tersimpan %v, mau NJOP", tanahD.NJOPSource)
	}
	if !tanahD.NJOPValue.Equal(idr(8_000_000)) {
		t.Fatalf("NJOP tersimpan %s, mau 8000000", tanahD.NJOPValue)
	}

	custE := e.newCustomer(t, "Eka Tanah Adat", "")
	loanE := e.disburse(t, custE.ID, e.newAccount(t, custE.ID), idr(10_000_000), 4)
	e.setOldestDueDate(t, loanE.ID, asOf.AddDate(0, 0, -100))

	// Huruf e: tanah adat. NJOP 6 juta -> pengurang 50% x 6 juta = 3 juta.
	e.addCollateralItem6(t, &domain.LoanCollateral{
		LoanID:         loanE.ID,
		CollateralType: domain.CollateralTanahAdat,
		AppraisalValue: idr(9_000_000),
		NJOPValue:      idr(6_000_000),
		NJOPSource:     &sumberNJOP,
	})

	custF := e.newCustomer(t, "Fani Tanpa NJOP", "")
	loanF := e.disburse(t, custF.ID, e.newAccount(t, custF.ID), idr(10_000_000), 4)
	e.setOldestDueDate(t, loanF.ID, asOf.AddDate(0, 0, -100))

	// Huruf e tanpa NJOP: taksasi 9 juta TIDAK dipakai sebagai NJOP -> pengurang nol.
	e.addCollateralItem6(t, &domain.LoanCollateral{
		LoanID:         loanF.ID,
		CollateralType: domain.CollateralTanahAdat,
		AppraisalValue: idr(9_000_000),
	})

	summary, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}

	itemD := ppapItem(t, summary, loanD.ID)
	if want := idr(4_800_000); !itemD.CollateralValue.Equal(want) {
		t.Fatalf("huruf d: pengurang %s, mau 60%% NJOP = %s", itemD.CollateralValue, want)
	}
	if want := idr(10_000_000).Sub(idr(4_800_000)); !itemD.Exposure.Equal(want) {
		t.Fatalf("huruf d: eksposur %s, mau %s", itemD.Exposure, want)
	}

	itemE := ppapItem(t, summary, loanE.ID)
	if want := idr(3_000_000); !itemE.CollateralValue.Equal(want) {
		t.Fatalf("huruf e: pengurang %s, mau 50%% NJOP = %s", itemE.CollateralValue, want)
	}

	itemF := ppapItem(t, summary, loanF.ID)
	if !itemF.CollateralValue.IsZero() {
		t.Fatalf("huruf e tanpa NJOP: pengurang %s, mau nol (taksasi tidak dipakai)", itemF.CollateralValue)
	}
	if !itemF.Exposure.Equal(idr(10_000_000)) {
		t.Fatalf("huruf e tanpa NJOP: eksposur %s, mau baki penuh 10000000", itemF.Exposure)
	}
}

// TestIntegrasiJalurUangAgunanItem6PorsiTunai memverifikasi bahwa porsi agunan tunai
// yang dikecualikan dari PPKA umum dibatasi pada eksposur: jaminan 15 juta atas baki
// 10 juta hanya mengecualikan 10 juta.
func TestIntegrasiJalurUangAgunanItem6PorsiTunai(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setCollateralEnabled(t, true)

	cust := e.newCustomer(t, "Tina Tunai", "")
	tabungan := e.newAccount(t, cust.ID)
	loan := e.disburse(t, cust.ID, tabungan, idr(10_000_000), 4) // Lancar (DPD 0)

	e.addCollateralItem6(t, &domain.LoanCollateral{
		LoanID:         loan.ID,
		CollateralType: domain.CollateralDeposit,
		AppraisalValue: idr(15_000_000),
		IsCash:         true,
		CashAccountID:  &tabungan,
	})

	summary, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	item := ppapItem(t, summary, loan.ID)
	if want := idr(10_000_000); !item.CollateralValue.Equal(want) {
		t.Fatalf("porsi dijamin %s, mau dibatasi pada eksposur %s", item.CollateralValue, want)
	}
	if !item.Exposure.IsZero() {
		t.Fatalf("eksposur %s, mau nol", item.Exposure)
	}
	if !item.Target.IsZero() {
		t.Fatalf("target %s, mau nol (tidak ada cadangan negatif)", item.Target)
	}
}

// TestIntegrasiJalurUangAgunanItem6Bumn memverifikasi penanda kriteria penjamin
// BUMN/BUMD: tanpa penanda nilainya bukan pengurang, dan dengan penanda beserta bukti
// barulah tarif 50% huruf i berlaku. Kolom penanda juga diuji langsung di database.
func TestIntegrasiJalurUangAgunanItem6Bumn(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := time.Now().UTC()
	e.setCollateralEnabled(t, true)

	custTanpa := e.newCustomer(t, "Budi BUMN Tanpa", "")
	loanTanpa := e.disburse(t, custTanpa.ID, e.newAccount(t, custTanpa.ID), idr(10_000_000), 4)
	e.setOldestDueDate(t, loanTanpa.ID, asOf.AddDate(0, 0, -100))
	e.addCollateralItem6(t, &domain.LoanCollateral{
		LoanID:         loanTanpa.ID,
		CollateralType: domain.CollateralJaminanBumnBumd,
		AppraisalValue: idr(4_000_000),
		// BumnBumdCriteriaMet sengaja dibiarkan false.
	})

	custDengan := e.newCustomer(t, "Bella BUMN Dengan", "")
	loanDengan := e.disburse(t, custDengan.ID, e.newAccount(t, custDengan.ID), idr(10_000_000), 4)
	e.setOldestDueDate(t, loanDengan.ID, asOf.AddDate(0, 0, -100))
	bumn := e.addCollateralItem6(t, &domain.LoanCollateral{
		LoanID:              loanDengan.ID,
		CollateralType:      domain.CollateralJaminanBumnBumd,
		AppraisalValue:      idr(4_000_000),
		BumnBumdCriteriaMet: true,
		BumnBumdEvidence:    "Surat jaminan penjamin BUMN",
	})

	// Kolom penanda dan bukti benar-benar tersimpan di database.
	var terpenuhi bool
	var bukti sql.NullString
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT bumn_bumd_criteria_met, bumn_bumd_evidence FROM loan_collaterals WHERE id = $1`,
		bumn.ID).Scan(&terpenuhi, &bukti); err != nil {
		t.Fatalf("membaca penanda BUMN/BUMD: %v", err)
	}
	if !terpenuhi {
		t.Fatal("penanda kriteria BUMN/BUMD tidak tersimpan true")
	}
	if !bukti.Valid || bukti.String == "" {
		t.Fatal("bukti kriteria BUMN/BUMD tidak tersimpan")
	}

	summary, err := e.ppapSvc.Preview(e.ctx, asOf, e.actor)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	itemTanpa := ppapItem(t, summary, loanTanpa.ID)
	if !itemTanpa.CollateralValue.IsZero() {
		t.Fatalf("BUMN/BUMD tanpa penanda: pengurang %s, mau nol", itemTanpa.CollateralValue)
	}
	itemDengan := ppapItem(t, summary, loanDengan.ID)
	if want := idr(2_000_000); !itemDengan.CollateralValue.Equal(want) {
		t.Fatalf("BUMN/BUMD berpenanda: pengurang %s, mau 50%% x 4000000 = %s", itemDengan.CollateralValue, want)
	}
}
