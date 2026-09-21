package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Saklar ppap.collateral.enabled harus benar-benar menghentikan pembacaan agunan, bukan
// hanya membuat hasilnya nol: kalau query tetap jalan, tutup hari menanggung beban modul
// yang belum diaktifkan. Stub di sini tidak mengimplementasikan pembacaan agunan selain
// lewat penanda, sehingga pemanggilan tak terduga akan terlihat.
func TestPPAPCollateralValues_SaklarMatiTidakMembacaAgunan(t *testing.T) {
	repo := &collateralRepoStub{active: []domain.LoanCollateral{{ID: uuid.New(), LoanID: uuid.New()}}}
	svc := &ppapService{
		collateralRepo: repo,
		config:         &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "false"}},
	}

	values, err := svc.collateralValues(context.Background(), []domain.PPAPLoanSnapshot{{LoanID: uuid.New()}})
	if err != nil {
		t.Fatalf("kesalahan tak terduga: %v", err)
	}
	if values != nil {
		t.Fatalf("nilai agunan %v, mau tidak dibaca sama sekali", values)
	}
	if repo.listCalled {
		t.Fatal("agunan tidak boleh dibaca selama saklar pengurangan mati")
	}
}

func TestPPAPCollateralValues_SaklarHidupMembacaSekaliUntukSemuaKredit(t *testing.T) {
	loanA, loanB := uuid.New(), uuid.New()
	repo := &collateralRepoStub{active: []domain.LoanCollateral{
		{ID: uuid.New(), LoanID: loanA, Status: domain.CollateralActive},
		{ID: uuid.New(), LoanID: loanA, Status: domain.CollateralActive},
	}}
	svc := &ppapService{
		collateralRepo: repo,
		config:         &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "true"}},
	}

	values, err := svc.collateralValues(context.Background(), []domain.PPAPLoanSnapshot{
		{LoanID: loanA}, {LoanID: loanB},
	})
	if err != nil {
		t.Fatalf("kesalahan tak terduga: %v", err)
	}
	if !repo.listCalled {
		t.Fatal("agunan harus dibaca saat saklar pengurangan hidup")
	}
	if len(values[loanA]) != 2 {
		t.Fatalf("agunan kredit A %d, mau 2", len(values[loanA]))
	}
	if len(values[loanB]) != 0 {
		t.Fatalf("agunan kredit B %d, mau 0", len(values[loanB]))
	}
}

// Tanpa repositori agunan (mis. pemanggil lama yang belum menyambungkannya) jalur PPAP
// harus tetap berjalan seperti sebelumnya, bukan gagal.
func TestPPAPCollateralValues_TanpaRepositoriTetapBerjalan(t *testing.T) {
	svc := &ppapService{config: &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "true"}}}

	values, err := svc.collateralValues(context.Background(), []domain.PPAPLoanSnapshot{{LoanID: uuid.New()}})
	if err != nil {
		t.Fatalf("kesalahan tak terduga: %v", err)
	}
	if values != nil {
		t.Fatalf("nilai agunan %v, mau kosong", values)
	}
}

// Selama saklar mati, keberadaan agunan aktif tidak boleh mengubah hasil PPAP sama sekali:
// target, penyesuaian, dan state kredit harus identik dengan jalur tanpa modul agunan.
func TestPPAPRunDaily_SaklarAgunanMatiPerilakuTidakBerubah(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)
	loanID := uuid.New()

	snapshot := domain.PPAPLoanSnapshot{
		LoanID:         loanID,
		LoanNumber:     "KRD-AGUNAN-MATI",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolLancar,
		RequiredPPAP:   decimal.NewFromInt(50_000),
		LastDueDate:    &due,
	}
	agunan := domain.LoanCollateral{
		ID:             uuid.New(),
		LoanID:         loanID,
		CollateralType: domain.CollateralTanahBangunan,
		AppraisalValue: decimal.NewFromInt(1_000_000_000),
		AppraisalDate:  asOf.AddDate(0, 0, -10),
		HaircutPercent: decimal.Zero,
		Status:         domain.CollateralActive,
		Certified:      true,
		Mortgaged:      true,
		ExistsKnown:    true,
		Executable:     true,
	}

	// Jalur pembanding: tanpa repositori agunan sama sekali.
	repoTanpa, postingTanpa := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{snapshot}}, &stubPosting{}
	tanpaAgunan := newTestPPAPService(repoTanpa, &stubProductRepo{}, postingTanpa)
	totalTanpa, err := tanpaAgunan.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily tanpa agunan: %v", err)
	}

	// Jalur kedua: agunan aktif tersedia, tetapi saklar pengurangan mati.
	repo, posting := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{snapshot}}, &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunan}}
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "false"}}
	totalDengan, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily dengan agunan: %v", err)
	}

	if !totalTanpa.TotalAdjustment.Equal(totalDengan.TotalAdjustment) {
		t.Fatalf("penyesuaian berubah saat saklar mati: %s vs %s",
			totalTanpa.TotalAdjustment, totalDengan.TotalAdjustment)
	}
	if len(repoTanpa.updated) != 1 || len(repo.updated) != 1 {
		t.Fatalf("state kredit tidak diperbarui sekali: %d dan %d", len(repoTanpa.updated), len(repo.updated))
	}
	if !repoTanpa.updated[0].RequiredPPAP.Equal(repo.updated[0].RequiredPPAP) {
		t.Fatalf("required_ppap berubah saat saklar mati: %s vs %s",
			repoTanpa.updated[0].RequiredPPAP, repo.updated[0].RequiredPPAP)
	}
	if totalDengan.Items[0].CollateralValue.Sign() != 0 {
		t.Fatalf("pengurang agunan %s, mau nol saat saklar mati", totalDengan.Items[0].CollateralValue)
	}
}
