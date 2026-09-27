package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// agunanTanahPPAP adalah agunan tanah/bangunan aktif yang lolos syarat Pasal 20/21
// (dinilai, bersertifikat, terikat, ada, dapat dieksekusi), dipakai menguji penyimpanan
// nilai agunan PPKA. Haircut 0 agar pengurangnya jelas > 0.
func agunanTanahPPAP(loanID uuid.UUID, asOf time.Time) domain.LoanCollateral {
	return domain.LoanCollateral{
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
}

// snapshotKurangLancarPPAP adalah kredit 10 juta DPD 100 (Kurang Lancar, tarif 10%)
// dengan cadangan lama 0,5%: run pasti mengubah state sehingga nilai agunan ikut
// tersimpan.
func snapshotKurangLancarPPAP(loanID uuid.UUID, asOf time.Time) domain.PPAPLoanSnapshot {
	due := asOf.AddDate(0, 0, -100)
	return domain.PPAPLoanSnapshot{
		LoanID:         loanID,
		LoanNumber:     "KRD-AGUNAN-PPKA",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolLancar,
		RequiredPPAP:   decimal.NewFromInt(50_000),
		LastDueDate:    &due,
	}
}

// Saat saklar agunan hidup, nilai pengurang yang dipakai run (CollateralValue) harus
// tersimpan per kredit sebagai sumber Form 06.00 kolom XXV, bukan hanya hidup di hasil run.
func TestPPAPRunDaily_SaklarAgunanHidupMenyimpanAgunanPPKA(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	loanID := uuid.New()

	repo, posting := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{snapshotKurangLancarPPAP(loanID, asOf)}}, &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "true"}}
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunanTanahPPAP(loanID, asOf)}}

	summary, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("state kredit diperbarui %d kali, ingin 1", len(repo.updated))
	}
	pengurang := summary.Items[0].CollateralValue
	if pengurang.Sign() <= 0 {
		t.Fatalf("pengurang agunan %s, ingin > 0 agar penyimpanan teruji", pengurang)
	}
	got := repo.updated[0].OJKAgunanPPKAAmount
	if got == nil {
		t.Fatal("nilai agunan PPKA tidak tersimpan saat saklar agunan hidup")
	}
	if !got.Equal(pengurang) {
		t.Fatalf("nilai agunan PPKA tersimpan %s, ingin sama dengan CollateralValue %s", got, pengurang)
	}
}

// Saat saklar agunan mati, sistem tidak menghitung pengurang sama sekali: service harus
// mengirim nil (repositori lalu tidak mengubah kolom lewat COALESCE), sehingga isian bank
// tidak terhapus dan laporan menulis "-" selama belum ada isian, bukan diklaim nol.
func TestPPAPRunDaily_SaklarAgunanMatiTidakMenyimpanAgunanPPKA(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	loanID := uuid.New()

	repo, posting := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{snapshotKurangLancarPPAP(loanID, asOf)}}, &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "false"}}
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunanTanahPPAP(loanID, asOf)}}

	if _, err := svc.RunDaily(context.Background(), asOf, domain.Actor{}); err != nil {
		t.Fatalf("RunDaily: %v", err)
	}
	if len(repo.updated) != 1 {
		t.Fatalf("state kredit diperbarui %d kali, ingin 1", len(repo.updated))
	}
	if repo.updated[0].OJKAgunanPPKAAmount != nil {
		t.Fatalf("nilai agunan PPKA %v, ingin NULL saat saklar agunan mati",
			repo.updated[0].OJKAgunanPPKAAmount)
	}
}
