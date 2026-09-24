package service_test

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// TAHAP T3 CKPN individual: pintu masuk jalur individual. Pemindaian melaporkan kredit
// yang memenuhi pemicu wajib (macet, restrukturisasi, DPD, agunan turun, bukti objektif)
// dan signifikansi (ambang nominal / TopN); penandaan mencatat keputusan pengelola ke
// loans.ckpn_method + jejak loan_ckpn_individual_assessments. Semuanya BAYANGAN:
// required_ckpn tidak pernah disentuh dan tidak ada jurnal.
//
// Ambang signifikansi dinaikkan menjadi Rp10.000.000 pada lingkungan uji ini sehingga
// kredit uji Rp1 jt tidak signifikan: jalur masuk murni lewat pemicu wajib.

func newIndividualSvcT3(e *moneyEnv) domain.CKPNIndividualService {
	return service.NewCKPNIndividualService(e.loanRepo, postgres.NewCKPNIndividualRepository(e.db), e.configSvc)
}

func TestIntegrasiCKPNIndividualT3PintuMasukDanPenandaan(t *testing.T) {
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.individual.significance_amount", "10000000")
	// Portofolio uji hanya beberapa kredit: TopN 20 membuat semuanya "terbesar".
	// TopN dimatikan pada uji ini agar jalur pemicu wajib terisolasi; jalur TopN
	// sudah diuji pada uji domain dan uji ambang di bawah.
	setCKPNConfig(t, e, "ckpn.individual.significance_top_n", "0")
	t.Cleanup(func() {
		setCKPNConfig(t, e, "ckpn.individual.significance_amount", "1000000000")
		setCKPNConfig(t, e, "ckpn.individual.significance_top_n", "20")
	})

	branchCode := ckpnTestBranchCode("T3")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T3")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit3", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T3", "t3-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(1_000_000), 12)
	svc := newIndividualSvcT3(e)

	// 1. Kredit lancar kecil: TIDAK masuk jalur individual.
	scan, err := svc.ScanEntries(e.ctx, actor)
	if err != nil {
		t.Fatalf("memindai portofolio: %v", err)
	}
	for _, c := range scan.Individual {
		if c.LoanID == loan.ID {
			t.Fatalf("kredit lancar kecil tidak boleh masuk jalur individual: %+v", c.Entry)
		}
	}

	// 2. Kredit dimacetkan: harus muncul dengan pemicu MACET.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans SET collectibility='5_MACET', dpd=95 WHERE id=$1`, loan.ID); err != nil {
		t.Fatalf("memacetkan kredit uji: %v", err)
	}
	scan, err = svc.ScanEntries(e.ctx, actor)
	if err != nil {
		t.Fatalf("memindai setelah macet: %v", err)
	}
	var kandidat *domain.CKPNIndividualCandidate
	for i := range scan.Individual {
		if scan.Individual[i].LoanID == loan.ID {
			kandidat = &scan.Individual[i]
			break
		}
	}
	if kandidat == nil {
		t.Fatalf("kredit macet harus masuk jalur individual")
	}
	adaMacet := false
	for _, tr := range kandidat.Entry.Triggers {
		if tr.Code == "MACET" && tr.Mandatory {
			adaMacet = true
		}
	}
	if !adaMacet {
		t.Fatalf("pemicu MACET wajib dilaporkan: %+v", kandidat.Entry.Triggers)
	}

	// 3. Penandaan keputusan pengelola: metode MAX + bukti objektif.
	entry, err := svc.MarkLoanEntry(e.ctx, loan.LoanNumber, domain.CKPNIndividualMethodMax,
		false, true, false, actor)
	if err != nil {
		t.Fatalf("menandai pintu masuk: %v", err)
	}
	if !entry.Individual || entry.SuggestedMethod != domain.CKPNIndividualMethodMax {
		t.Fatalf("keputusan penandaan harus memantul: %+v", entry)
	}
	var method string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT ckpn_method FROM loans WHERE id=$1`, loan.ID).Scan(&method); err != nil {
		t.Fatalf("membaca ckpn_method: %v", err)
	}
	if method != string(domain.CKPNIndividualMethodMax) {
		t.Fatalf("ckpn_method tersimpan %q, mau MAX", method)
	}

	// 4. Jejak audit tertulis pada tabel T0.
	var jejak int
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT COUNT(*) FROM loan_ckpn_individual_assessments WHERE loan_id=$1 AND method='MAX'`,
		loan.ID).Scan(&jejak); err != nil {
		t.Fatalf("membaca jejak penilaian: %v", err)
	}
	if jejak != 1 {
		t.Fatalf("jejak penilaian harus 1 baris, dapat %d", jejak)
	}

	// 5. required_ckpn tidak tersentuh (bayangan).
	var rc string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT required_ckpn FROM loans WHERE id=$1`, loan.ID).Scan(&rc); err != nil {
		t.Fatalf("membaca required_ckpn: %v", err)
	}
	if rc != "0" && rc != "0.00" && rc != "0.0000" {
		t.Fatalf("required_ckpn harus tetap 0, dapat %q", rc)
	}

	// 6. Penandaan pengecualian aset baik: kredit keluar dari kandidat.
	if _, err := svc.MarkLoanEntry(e.ctx, loan.LoanNumber, "", false, false, true, actor); err != nil {
		t.Fatalf("menandai pengecualian aset baik: %v", err)
	}
	scan, err = svc.ScanEntries(e.ctx, actor)
	if err != nil {
		t.Fatalf("memindai setelah pengecualian: %v", err)
	}
	for _, c := range scan.Individual {
		if c.LoanID == loan.ID {
			t.Fatalf("kredit yang dikecualikan tidak boleh jadi kandidat lagi")
		}
	}
	dikecualikan := false
	for _, nomor := range scan.ExcludedAsetBaik {
		if nomor == loan.LoanNumber {
			dikecualikan = true
		}
	}
	if !dikecualikan {
		t.Fatalf("kredit yang dikecualikan harus dilaporkan sebagai pengecualian: %+v", scan.ExcludedAsetBaik)
	}
	var methodEx string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT ckpn_method FROM loans WHERE id=$1`, loan.ID).Scan(&methodEx); err != nil {
		t.Fatalf("membaca ckpn_method pengecualian: %v", err)
	}
	if methodEx != string(domain.CKPNIndividualMethodExcludedAsetBaik) {
		t.Fatalf("ckpn_method pengecualian tersimpan %q", methodEx)
	}

	// 7. Penandaan tanpa alasan ditolak: kredit lancar di bawah ambang (TopN mati)
	// tidak boleh ditandai individual diam-diam; kredit macet di atas sah karena
	// pemicunya, jadi uji ini memakai kredit baru yang bersih. Nomor rekening harus
	// unik per kredit, jadi rekening kedua dibuat untuk kredit pembanding.
	accBersih := e.newAccountInBranch(t, cust.ID, branchID)
	loanBersih := e.disburseAs(t, actor, cust.ID, accBersih, idr(1_000_000), 12)
	if _, err := svc.MarkLoanEntry(e.ctx, loanBersih.LoanNumber, domain.CKPNIndividualMethodDCF,
		false, false, false, actor); err == nil {
		t.Fatalf("penandaan tanpa pemicu/signifikansi harus ditolak")
	}
}

func TestIntegrasiCKPNIndividualT3AmbangSignifikansi(t *testing.T) {
	e := newMoneyEnv(t)
	// Ambang diturunkan menjadi Rp500.000: kredit uji Rp1 jt menjadi signifikan
	// walau lancar; inilah pintu signifikansi (praktik bank), terpisah dari wajib.
	setCKPNConfig(t, e, "ckpn.individual.significance_amount", "500000")
	t.Cleanup(func() { setCKPNConfig(t, e, "ckpn.individual.significance_amount", "1000000000") })

	branchCode := ckpnTestBranchCode("T3B")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji T3B")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit3b", Role: domain.RoleAdmin, BranchCode: branchCode}
	cust := e.newCustomer(t, "Nasabah T3B", "t3b-"+branchCode+"@uji.local")
	acc := e.newAccountInBranch(t, cust.ID, branchID)
	loan := e.disburseAs(t, actor, cust.ID, acc, idr(1_000_000), 12)
	svc := newIndividualSvcT3(e)

	scan, err := svc.ScanEntries(e.ctx, actor)
	if err != nil {
		t.Fatalf("memindai portofolio: %v", err)
	}
	ketemu := false
	for _, c := range scan.Individual {
		if c.LoanID != loan.ID {
			continue
		}
		ketemu = true
		if !c.Entry.Significance {
			t.Fatalf("kredit di atas ambang harus signifikan: %+v", c.Entry)
		}
		for _, tr := range c.Entry.Triggers {
			if tr.Mandatory {
				t.Fatalf("kredit lancar tidak boleh punya pemicu wajib: %+v", c.Entry.Triggers)
			}
		}
	}
	if !ketemu {
		t.Fatalf("kredit signifikan harus masuk daftar kandidat")
	}
	// Penandaan signifikan lancar sah (praktik bank memilih menilai lebih awal).
	if _, err := svc.MarkLoanEntry(e.ctx, loan.LoanNumber, domain.CKPNIndividualMethodMax,
		true, false, false, actor); err != nil {
		t.Fatalf("menandai kredit signifikan: %v", err)
	}
}

func TestIntegrasiCKPNIndividualT3IsolasiCabang(t *testing.T) {
	// Pemindaian tunduk pada cakupan cabang: aktor cabang A tidak melihat kredit cabang B.
	e := newMoneyEnv(t)
	setCKPNConfig(t, e, "ckpn.individual.significance_amount", "10000000")
	t.Cleanup(func() { setCKPNConfig(t, e, "ckpn.individual.significance_amount", "1000000000") })

	branchA := ckpnTestBranchCode("T3A")
	branchB := ckpnTestBranchCode("T3C")
	idA := e.ensureBranch(t, branchA, "Cabang Uji T3-A")
	idB := e.ensureBranch(t, branchB, "Cabang Uji T3-B")
	actorA := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit3a", Role: domain.RoleAdmin, BranchCode: branchA}
	custA := e.newCustomer(t, "Nasabah T3-A", "t3a-"+branchA+"@uji.local")
	accA := e.newAccountInBranch(t, custA.ID, idA)
	loanA := e.disburseAs(t, actorA, custA.ID, accA, idr(1_000_000), 12)
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loans SET collectibility='5_MACET', branch_id=$2 WHERE id=$1`, loanA.ID, idB); err != nil {
		t.Fatalf("memindahkan kredit ke cabang B: %v", err)
	}

	svc := newIndividualSvcT3(e)
	scan, err := svc.ScanEntries(e.ctx, actorA)
	if err != nil {
		t.Fatalf("memindai sebagai cabang A: %v", err)
	}
	for _, c := range scan.Individual {
		if c.LoanID == loanA.ID {
			t.Fatalf("aktor cabang A tidak boleh melihat kredit macet cabang B")
		}
	}
	actorB := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujit3c", Role: domain.RoleAdmin, BranchCode: branchB}
	custB := e.newCustomer(t, "Nasabah T3-C", "t3c-"+branchB+"@uji.local")
	accB := e.newAccountInBranch(t, custB.ID, idB)
	loanB := e.disburseAs(t, actorB, custB.ID, accB, idr(2_000_000), 12)
	if _, err := e.db.ExecContext(e.ctx,
		`UPDATE loans SET collectibility='5_MACET' WHERE id=$1`, loanB.ID); err != nil {
		t.Fatalf("memacetkan kredit cabang B: %v", err)
	}
	scan, err = svc.ScanEntries(e.ctx, actorB)
	if err != nil {
		t.Fatalf("memindai sebagai cabang B: %v", err)
	}
	dilihat := false
	for _, c := range scan.Individual {
		if c.LoanID == loanA.ID || c.LoanID == loanB.ID {
			dilihat = true
		}
	}
	if !dilihat {
		t.Fatalf("aktor cabang B harus melihat kredit macet cabang B")
	}
}
