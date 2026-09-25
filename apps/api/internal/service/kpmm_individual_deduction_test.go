package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// kpmm_individual_deduction_test.go mengunci penyambungan jalur CKPN individual ke
// pengurang modal inti KPMM tanpa basis data. Alur yang dibuktikan:
//
//	segel loans.ckpn_method (T3/T4) -> mesin CKPN memakai target INDIVIDUAL (bukan
//	EADxPDxLGD kolektif) -> summary.ModalIntiDeduction (Σ max(PPKA-CKPN,0) PER KREDIT)
//	-> kpmmService.pengurangModalInti memakainya sebagai pengurang modal inti.
//
// Kredit kolektif TIDAK boleh berubah oleh keberadaan jalur individual.

// kpmmCKPNItem mengambil item satu kredit dari ringkasan, atau menggagalkan uji.
func kpmmCKPNItem(t *testing.T, summary domain.CKPNComparisonSummary, loanID uuid.UUID) domain.CKPNComparisonItem {
	t.Helper()
	for _, it := range summary.Items {
		if it.LoanID == loanID {
			return it
		}
	}
	t.Fatalf("kredit %s tidak ada di hasil CKPN (failures: %+v)", loanID, summary.Failures)
	return domain.CKPNComparisonItem{}
}

// stubIndividualSvc mensimulasikan penilaian CKPN individual (T1/T2) tanpa basis data.
// Hanya EvaluateForLoan yang dipakai jalur baca-saja Compare; method lain diwarisi nil
// dari interface sehingga tidak sengaja terpanggil.
type stubIndividualSvc struct {
	domain.CKPNIndividualService
	assessment domain.CKPNIndividualAssessment
	err        error
}

func (s stubIndividualSvc) EvaluateForLoan(_ context.Context, loan *domain.Loan, asOf time.Time) (domain.CKPNIndividualAssessment, error) {
	if s.err != nil {
		return domain.CKPNIndividualAssessment{}, s.err
	}
	a := s.assessment
	a.LoanID = loan.ID
	a.LoanNumber = loan.LoanNumber
	a.AsOf = asOf
	return a, nil
}

// kpmmIndividuAssessment hasil penilaian individual yang sah untuk metode MAX: proyeksi
// wajib ada (kalau kosong, CKPNIndividualTargetForEOD sengaja GAGAL), FinalTarget adalah
// angka yang menjadi target resmi.
func kpmmIndividuAssessment(target int64) domain.CKPNIndividualAssessment {
	return domain.CKPNIndividualAssessment{
		Method:      domain.CKPNIndividualMethodMax,
		FinalTarget: decimal.NewFromInt(target),
		Projections: []domain.CKPNCashflowProjection{
			{Period: 1, Amount: decimal.NewFromInt(110_000)},
		},
	}
}

// kpmmIndividuConfig parameter kolektif yang sengaja akan menghasilkan angka BERBEDA
// dari target individual: 10.000.000 x 10% x 50% = 500.000.
func kpmmIndividuConfig() *ckpnConfigStub {
	return &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
	}}
}

// TestKPMMKreditIndividualMemakaiCKPNIndividual membuktikan kredit tersegel individual
// memakai target individual-nya, BUKAN angka kolektif, dan bahwa angka itu mengalir ke
// pengurang modal inti KPMM. Kredit kolektif di portofolio yang sama tidak berubah.
func TestKPMMKreditIndividualMemakaiCKPNIndividual(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	// Kredit individual: PPKA 1.000.000, target individual 50.000. Rumus kolektif akan
	// memberi 500.000; kalau mesin salah jalur, pengurangnya jadi 500.000, bukan 950.000.
	individu := ckpnLoan()
	individu.LoanNumber = "KRD-2026-INDIV"
	individu.CKPNMethod = string(domain.CKPNIndividualMethodMax)
	individu.RequiredPPAP = decimal.NewFromInt(1_000_000)

	// Kredit kolektif: PPKA 200.000 < CKPN kolektif 500.000 -> pengurang 0 (kredit ini
	// tidak boleh mengurangi kelebihan kredit lain, dan tidak terpengaruh jalur individual).
	kolektif := ckpnLoan()
	kolektif.LoanNumber = "KRD-2026-KOLEKTIF"
	kolektif.RequiredPPAP = decimal.NewFromInt(200_000)

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{individu, kolektif}}
	ckpnSvc, _, _ := newTestCKPNService(repo, kpmmIndividuConfig())
	ckpnSvc.individual = stubIndividualSvc{assessment: kpmmIndividuAssessment(50_000)}

	summary, err := ckpnSvc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Failed != 0 || len(summary.Items) != 2 {
		t.Fatalf("failed=%d items=%d (%+v), mau 0/2", summary.Failed, len(summary.Items), summary.Failures)
	}

	itemIndividu := kpmmCKPNItem(t, summary, individu.LoanID)
	if !itemIndividu.CKPN.Equal(decimal.NewFromInt(50_000)) {
		t.Fatalf("kredit individual memakai CKPN %s, mau 50000 (target individual, bukan kolektif 500000)", itemIndividu.CKPN)
	}
	itemKolektif := kpmmCKPNItem(t, summary, kolektif.LoanID)
	if !itemKolektif.CKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("kredit kolektif memakai CKPN %s, mau 500000 (perilaku kolektif tidak berubah)", itemKolektif.CKPN)
	}

	if summary.IndividualCount != 1 || !summary.IndividualTotalCKPN.Equal(decimal.NewFromInt(50_000)) {
		t.Fatalf("ringkasan individual count=%d total=%s, mau 1/50000",
			summary.IndividualCount, summary.IndividualTotalCKPN)
	}
	if !summary.TotalCKPN.Equal(decimal.NewFromInt(550_000)) {
		t.Fatalf("TotalCKPN %s, mau 550000 (individual 50000 + kolektif 500000, tanpa dobel hitung)", summary.TotalCKPN)
	}
	// Σ per kredit = (1.000.000-50.000) + max(200.000-500.000,0) = 950.000.
	if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(950_000)) {
		t.Fatalf("ModalIntiDeduction %s, mau 950000", summary.ModalIntiDeduction)
	}
	// Agregat = 1.200.000 - 550.000 = 650.000; sengaja berbeda agar pilihan basis terlihat.
	if !summary.Difference.Equal(decimal.NewFromInt(650_000)) {
		t.Fatalf("Difference agregat %s, mau 650000", summary.Difference)
	}

	// Titik sambung KPMM: pengurang modal inti memakai ModalIntiDeduction (per kredit),
	// bukan Difference (agregat), sehingga kredit individual ikut menentukan.
	kpmm := &kpmmService{ckpn: ckpnSvc, config: newKPMMConfig()}
	var gaps []string
	komponen, basis := kpmm.pengurangModalInti(context.Background(), asOf, domain.Actor{}, &gaps)
	if basis != "per_kredit" {
		t.Fatalf("basis %q, mau per_kredit", basis)
	}
	if !komponen.Tersedia || !komponen.Nilai.Equal(decimal.NewFromInt(950_000)) {
		t.Fatalf("pengurang modal inti KPMM %+v, mau tersedia 950000", komponen)
	}

	// Basis agregat tetap tersedia sebagai pilihan kebijakan dan memakai selisih agregat.
	basisAgregat := newKPMMConfig()
	basisAgregat.values["kpmm.deduction_basis"] = "agregat"
	kpmmAgregat := &kpmmService{ckpn: ckpnSvc, config: basisAgregat}
	komponenAgregat, basisEfektif := kpmmAgregat.pengurangModalInti(context.Background(), asOf, domain.Actor{}, &gaps)
	if basisEfektif != "agregat" || !komponenAgregat.Nilai.Equal(decimal.NewFromInt(650_000)) {
		t.Fatalf("basis agregat %q nilai %s, mau agregat/650000", basisEfektif, komponenAgregat.Nilai)
	}
}

// TestKPMMKreditKolektifTidakMemanggilJalurIndividual membuktikan kredit tanpa segel
// tetap kolektif dan TIDAK menyentuh layanan individual: layanan yang sengaja selalu
// gagal pun tidak boleh muncul sebagai kegagalan saat hanya ada kredit kolektif.
func TestKPMMKreditKolektifTidakMemanggilJalurIndividual(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	kolektif := ckpnLoan()
	kolektif.RequiredPPAP = decimal.NewFromInt(1_000_000)

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{kolektif}}
	ckpnSvc, _, _ := newTestCKPNService(repo, kpmmIndividuConfig())
	ckpnSvc.individual = stubIndividualSvc{err: errors.New("jalur individual tidak boleh dipanggil")}

	summary, err := ckpnSvc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Failed != 0 || len(summary.Items) != 1 {
		t.Fatalf("kolektif gagal=%d items=%d (%+v), mau 0/1", summary.Failed, len(summary.Items), summary.Failures)
	}
	if !summary.Items[0].CKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("CKPN kolektif %s, mau 500000", summary.Items[0].CKPN)
	}
	// Pengurang = 1.000.000 - 500.000 = 500.000; jalur individual tidak mengubahnya.
	if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("ModalIntiDeduction kolektif %s, mau 500000", summary.ModalIntiDeduction)
	}
}
