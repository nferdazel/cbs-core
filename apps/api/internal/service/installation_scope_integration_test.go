package service_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Uji integrasi cakupan buku tingkat INSTALASI terhadap PostgreSQL sungguhan.
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola moneyflow_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiCakupanInstalasi -v
//
// Membuktikan lini usaha yang tidak aktif tidak dapat dibaca (repository) maupun
// ditulis (service) bahkan oleh peran lintas buku; DUAL tetap identik dengan perilaku
// sebelum setelan ini ada.
//
// pengaturanScopeInstalasi mensimulasikan yang dilakukan middleware
// BookScopeMiddleware: cakupan dari config, dan pada instalasi satu buku actor.Book
// dipaksa ke buku aktif.

func scopeActor(base domain.Actor, scope domain.InstitutionBookScope) domain.Actor {
	base.BookScope = scope
	if book := scope.SingleBook(); book != "" {
		base.Book = book
	}
	return base
}

// TestIntegrasiCakupanInstalasiBacaMenolakLiniTidakAktif memastikan daftar kredit
// dan rekening hanya memuat buku yang aktif di instalasi, termasuk bagi SUPERADMIN.
func TestIntegrasiCakupanInstalasiBacaMenolakLiniTidakAktif(t *testing.T) {
	e := newBookWriteEnv(t)

	custConv := e.newCustomer(t, "Nasabah Cakupan Konvensional", fmt.Sprintf("cakupan-conv-%d@uji.local", time.Now().UnixNano()))
	accConv := e.newAccount(t, custConv.ID)
	loanConv := e.disburse(t, custConv.ID, accConv, idr(5_000_000), 6)

	custSyr := e.newCustomer(t, "Nasabah Cakupan Syariah", fmt.Sprintf("cakupan-syr-%d@uji.local", time.Now().UnixNano()))
	accSyr := e.newSyariahAccount(t, custSyr.ID)
	loanSyr := e.disburseWithProduct(t, "PMB-MURABAHAH", custSyr.ID, accSyr, idr(5_000_000), 6)

	base := domain.Actor{UserID: e.actor.UserID, Username: "superadmin.cakupan", Role: domain.RoleSuperAdmin, BranchCode: "001"}
	dualActor := scopeActor(base, domain.ScopeDual)
	syrActor := scopeActor(base, domain.ScopeSyariah)
	convActor := scopeActor(base, domain.ScopeConventional)

	cases := []struct {
		name     string
		actor    domain.Actor
		wantConv bool
		wantSyr  bool
	}{
		{"SYARIAH: hanya syariah", syrActor, false, true},
		{"KONVENSIONAL: hanya konvensional", convActor, true, false},
		{"DUAL: keduanya (identik dengan sebelumnya)", dualActor, true, true},
	}
	for _, tc := range cases {
		t.Run("kredit/"+tc.name, func(t *testing.T) {
			loans, _, err := e.loanRepo.List(e.ctx, 200, 0, tc.actor)
			if err != nil {
				t.Fatalf("daftar kredit: %v", err)
			}
			got := loanIDSet(loans)
			if got[loanConv.ID] != tc.wantConv {
				t.Errorf("kredit konvensional terlihat=%v, ingin %v", got[loanConv.ID], tc.wantConv)
			}
			if got[loanSyr.ID] != tc.wantSyr {
				t.Errorf("pembiayaan syariah terlihat=%v, ingin %v", got[loanSyr.ID], tc.wantSyr)
			}
		})
		t.Run("rekening/"+tc.name, func(t *testing.T) {
			accounts, _, err := e.accountRepo.ListAll(e.ctx, 200, 0, "", tc.actor)
			if err != nil {
				t.Fatalf("daftar rekening: %v", err)
			}
			got := accountIDSet(accounts)
			if got[accConv] != tc.wantConv {
				t.Errorf("rekening konvensional terlihat=%v, ingin %v", got[accConv], tc.wantConv)
			}
			if got[accSyr] != tc.wantSyr {
				t.Errorf("rekening syariah terlihat=%v, ingin %v", got[accSyr], tc.wantSyr)
			}
		})
	}
}

// TestIntegrasiCakupanInstalasiTulisMenolakLiniTidakAktif memastikan transaksi pada
// buku yang tidak aktif ditolak tegas, dan DUAL tetap mengizinkannya.
func TestIntegrasiCakupanInstalasiTulisMenolakLiniTidakAktif(t *testing.T) {
	e := newBookWriteEnv(t)

	custConv := e.newCustomer(t, "Nasabah Tulis Cakupan Konvensional", fmt.Sprintf("tcakupan-conv-%d@uji.local", time.Now().UnixNano()))
	accConv := e.newAccount(t, custConv.ID)
	accConvNum := e.accountNumberFor(t, accConv)

	custSyr := e.newCustomer(t, "Nasabah Tulis Cakupan Syariah", fmt.Sprintf("tcakupan-syr-%d@uji.local", time.Now().UnixNano()))
	accSyr := e.newSyariahAccount(t, custSyr.ID)
	accSyrNum := e.accountNumberFor(t, accSyr)

	base := domain.Actor{UserID: e.actor.UserID, Username: "superadmin.cakupan", Role: domain.RoleSuperAdmin, BranchCode: "001"}
	syrActor := scopeActor(base, domain.ScopeSyariah)
	convActor := scopeActor(base, domain.ScopeConventional)
	dualActor := scopeActor(base, domain.ScopeDual)

	deposit := func(t *testing.T, accNum string, actor domain.Actor, tag string) error {
		t.Helper()
		_, err := e.ledgerSvc.Deposit(e.ctx, domain.DepositRequest{
			AccountNumber:  accNum,
			Amount:         idr(1_000_000),
			Currency:       "IDR",
			IdempotencyKey: fmt.Sprintf("%s-%d", tag, time.Now().UnixNano()),
			Actor:          actor,
		})
		return err
	}

	t.Run("SYARIAH menolak setoran rekening konvensional", func(t *testing.T) {
		if err := deposit(t, accConvNum, syrActor, "CAK-SYR-CONV"); !errors.Is(err, domain.ErrCrossBookAccess) {
			t.Fatalf("setoran konvensional lewat instalasi SYARIAH: %v, mau ErrCrossBookAccess", err)
		}
	})
	t.Run("KONVENSIONAL menolak setoran rekening syariah", func(t *testing.T) {
		if err := deposit(t, accSyrNum, convActor, "CAK-CONV-SYR"); !errors.Is(err, domain.ErrCrossBookAccess) {
			t.Fatalf("setoran syariah lewat instalasi KONVENSIONAL: %v, mau ErrCrossBookAccess", err)
		}
	})
	t.Run("DUAL mengizinkan kedua buku (identik dengan sebelumnya)", func(t *testing.T) {
		if err := deposit(t, accConvNum, dualActor, "CAK-DUAL-CONV"); err != nil {
			t.Fatalf("DUAL setoran konvensional: %v", err)
		}
		if err := deposit(t, accSyrNum, dualActor, "CAK-DUAL-SYR"); err != nil {
			t.Fatalf("DUAL setoran syariah: %v", err)
		}
	})

	// Pengajuan kredit juga ditolak lintas lini lewat pemeriksaan buku produk.
	t.Run("SYARIAH menolak pengajuan produk konvensional", func(t *testing.T) {
		product, err := e.productRepo.GetByCode(e.ctx, "KRD-FLAT")
		if err != nil {
			t.Fatalf("membaca produk: %v", err)
		}
		_, err = e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
			CustomerID:            custSyr.ID,
			ProductID:             product.ID,
			DisbursementAccountID: accSyr,
			PrincipalAmount:       idr(1_000_000),
			TermMonths:            6,
			Purpose:               "uji cakupan instalasi",
		}, syrActor)
		if !errors.Is(err, domain.ErrCrossBookAccess) {
			t.Fatalf("pengajuan produk konvensional di instalasi SYARIAH: %v, mau ErrCrossBookAccess", err)
		}
	})
}

// TestIntegrasiCakupanInstalasiBatchTidakMemprosesLiniTidakAktif memastikan kandidat
// batch (denda/akrual) hanya memuat buku yang aktif, sehingga EOD tidak menjalankan
// langkah untuk lini usaha yang tidak dilayani instalasi.
func TestIntegrasiCakupanInstalasiBatchTidakMemprosesLiniTidakAktif(t *testing.T) {
	e := newBookWriteEnv(t)

	custConv := e.newCustomer(t, "Nasabah Batch Cakupan Konvensional", fmt.Sprintf("bcakupan-conv-%d@uji.local", time.Now().UnixNano()))
	accConv := e.newAccount(t, custConv.ID)
	loanConv := e.disburse(t, custConv.ID, accConv, idr(4_000_000), 6)

	custSyr := e.newCustomer(t, "Nasabah Batch Cakupan Syariah", fmt.Sprintf("bcakupan-syr-%d@uji.local", time.Now().UnixNano()))
	accSyr := e.newSyariahAccount(t, custSyr.ID)
	loanSyr := e.disburseWithProduct(t, "PMB-MURABAHAH", custSyr.ID, accSyr, idr(4_000_000), 6)

	// Jauh di masa depan: seluruh angsuran sudah jatuh tempo sehingga keduanya
	// menjadi kandidat bila tidak difilter.
	asOf := time.Now().UTC().AddDate(10, 0, 0)

	base := domain.Actor{UserID: e.actor.UserID, Username: "system.cakupan", Role: domain.RoleSystem, BranchCode: "001"}
	syrCandidates, err := e.loanRepo.ListPenaltyCandidates(e.ctx, asOf, scopeActor(base, domain.ScopeSyariah))
	if err != nil {
		t.Fatalf("kandidat denda SYARIAH: %v", err)
	}
	if hasPenaltyCandidate(syrCandidates, loanConv.ID) {
		t.Error("instalasi SYARIAH masih menjadikan kredit konvensional kandidat batch")
	}
	if !hasPenaltyCandidate(syrCandidates, loanSyr.ID) {
		t.Error("instalasi SYARIAH harus tetap memproses pembiayaan syariah")
	}

	convCandidates, err := e.loanRepo.ListPenaltyCandidates(e.ctx, asOf, scopeActor(base, domain.ScopeConventional))
	if err != nil {
		t.Fatalf("kandidat denda KONVENSIONAL: %v", err)
	}
	if hasPenaltyCandidate(convCandidates, loanSyr.ID) {
		t.Error("instalasi KONVENSIONAL masih menjadikan pembiayaan syariah kandidat batch")
	}

	dualCandidates, err := e.loanRepo.ListPenaltyCandidates(e.ctx, asOf, scopeActor(base, domain.ScopeDual))
	if err != nil {
		t.Fatalf("kandidat denda DUAL: %v", err)
	}
	if !hasPenaltyCandidate(dualCandidates, loanConv.ID) || !hasPenaltyCandidate(dualCandidates, loanSyr.ID) {
		t.Error("DUAL harus tetap menjadikan kedua buku kandidat (identik dengan sebelumnya)")
	}
}

func hasPenaltyCandidate(list []domain.LoanPenaltyCandidate, loanID uuid.UUID) bool {
	for _, c := range list {
		if c.LoanID == loanID {
			return true
		}
	}
	return false
}

// TestIntegrasiCakupanInstalasiDualTidakBerubah membandingkan aktor DUAL eksplisit
// dengan aktor tanpa cakupan (perilaku lama) pada query repository yang sama; hasilnya
// harus sama supaya DUAL benar-benar identik.
func TestIntegrasiCakupanInstalasiDualTidakBerubah(t *testing.T) {
	e := newBookWriteEnv(t)

	custConv := e.newCustomer(t, "Nasabah Dual Identik Konvensional", fmt.Sprintf("dual-id-conv-%d@uji.local", time.Now().UnixNano()))
	accConv := e.newAccount(t, custConv.ID)
	loanConv := e.disburse(t, custConv.ID, accConv, idr(3_000_000), 6)

	custSyr := e.newCustomer(t, "Nasabah Dual Identik Syariah", fmt.Sprintf("dual-id-syr-%d@uji.local", time.Now().UnixNano()))
	accSyr := e.newSyariahAccount(t, custSyr.ID)
	e.disburseWithProduct(t, "PMB-MURABAHAH", custSyr.ID, accSyr, idr(3_000_000), 6)

	legacy := domain.Actor{UserID: e.actor.UserID, Username: "auditor.lama", Role: domain.RoleAuditor, BranchCode: "001"}
	dual := legacy
	dual.BookScope = domain.ScopeDual

	legacyLoans, _, err := e.loanRepo.List(e.ctx, 200, 0, legacy)
	if err != nil {
		t.Fatalf("daftar kredit tanpa cakupan: %v", err)
	}
	dualLoans, _, err := e.loanRepo.List(e.ctx, 200, 0, dual)
	if err != nil {
		t.Fatalf("daftar kredit DUAL: %v", err)
	}
	if len(legacyLoans) != len(dualLoans) {
		t.Fatalf("jumlah kredit tanpa cakupan=%d, DUAL=%d; DUAL harus identik", len(legacyLoans), len(dualLoans))
	}
	if !loanIDSet(dualLoans)[loanConv.ID] {
		t.Fatal("DUAL harus tetap melihat kredit konvensional")
	}
}
