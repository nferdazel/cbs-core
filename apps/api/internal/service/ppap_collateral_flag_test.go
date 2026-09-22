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

// Agunan tunai pun tidak boleh mengubah hasil selama saklar mati. Penanda is_cash hanya
// berarti bila pengurangan agunan aktif; sebelum itu, jalur PPAP harus identik dengan
// bank yang belum punya modul agunan.
func TestPPAPRunDaily_SaklarAgunanMatiTetapMengabaikanAgunanTunai(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -10)
	loanID := uuid.New()

	snapshot := domain.PPAPLoanSnapshot{
		LoanID:         loanID,
		LoanNumber:     "KRD-TUNAI-MATI",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolLancar,
		RequiredPPAP:   decimal.NewFromInt(50_000),
		LastDueDate:    &due,
	}
	akun := uuid.New()
	agunanTunai := domain.LoanCollateral{
		ID:             uuid.New(),
		LoanID:         loanID,
		CollateralType: domain.CollateralDeposit,
		IsCash:         true,
		CashAccountID:  &akun,
		AppraisalValue: decimal.NewFromInt(4_000_000),
		AppraisalDate:  asOf.AddDate(0, 0, -10),
		HaircutPercent: decimal.Zero,
		Status:         domain.CollateralActive,
		ExistsKnown:    true,
		Executable:     true,
	}

	repoTanpa, postingTanpa := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{snapshot}}, &stubPosting{}
	tanpaAgunan := newTestPPAPService(repoTanpa, &stubProductRepo{}, postingTanpa)
	totalTanpa, err := tanpaAgunan.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily tanpa agunan: %v", err)
	}

	repo, posting := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{snapshot}}, &stubPosting{}
	svc := newTestPPAPService(repo, &stubProductRepo{}, posting)
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunanTunai}}
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "false"}}
	totalDengan, err := svc.RunDaily(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("RunDaily dengan agunan tunai: %v", err)
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
		t.Fatalf("pengurang agunan tunai %s, mau nol saat saklar mati", totalDengan.Items[0].CollateralValue)
	}
	if !totalDengan.Items[0].Exposure.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("eksposur %s, mau baki penuh saat saklar mati", totalDengan.Items[0].Exposure)
	}
}

// agunanTunaiPPAP adalah agunan tunai aktif 4 juta yang dikaitkan ke rekening diblokir.
func agunanTunaiPPAP(loanID uuid.UUID, asOf time.Time) domain.LoanCollateral {
	akun := uuid.New()
	return domain.LoanCollateral{
		ID:             uuid.New(),
		LoanID:         loanID,
		CollateralType: domain.CollateralDeposit,
		IsCash:         true,
		CashAccountID:  &akun,
		AppraisalValue: decimal.NewFromInt(4_000_000),
		AppraisalDate:  asOf.AddDate(0, 0, -10),
		HaircutPercent: decimal.Zero,
		Status:         domain.CollateralActive,
		ExistsKnown:    true,
		Executable:     true,
	}
}

// Kualitas Lancar hanya dikenai PPKA umum. Bagian yang dijamin agunan tunai dikecualikan
// (Pasal 19 ayat (4) huruf b jo. Pasal 17): dasar 10 juta - 4 juta = 6 juta, tarif 0,5%.
func TestPPAPPreview_SaklarHidupMengecualikanAgunanTunaiDariPPKAUmum(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -10)
	loanID := uuid.New()

	repo := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{{
		LoanID:         loanID,
		LoanNumber:     "KRD-TUNAI-UMUM",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolLancar,
		RequiredPPAP:   decimal.NewFromInt(50_000),
		LastDueDate:    &due,
	}}}
	svc := newTestPPAPService(repo, &stubProductRepo{}, &stubPosting{})
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "true"}}
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunanTunaiPPAP(loanID, asOf)}}

	summary, err := svc.Preview(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	item := summary.Items[0]
	if !item.Exposure.Equal(decimal.NewFromInt(6_000_000)) {
		t.Fatalf("eksposur %s, mau 6000000", item.Exposure)
	}
	if !item.CollateralValue.Equal(decimal.NewFromInt(4_000_000)) {
		t.Fatalf("pengurang %s, mau 4000000", item.CollateralValue)
	}
	if !item.Target.Equal(decimal.NewFromInt(30_000)) {
		t.Fatalf("target %s, mau 30000", item.Target)
	}
}

// Kualitas Kurang Lancar dikenai PPKA khusus. Agunan tunai TIDAK mengurangi dasarnya
// (bukan daftar Pasal 20 ayat (1), lihat ayat (2)): dasar tetap 10 juta, tarif 10%.
func TestPPAPPreview_SaklarHidupAgunanTunaiTidakMengurangiPPKAKhusus(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -100)
	loanID := uuid.New()

	repo := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{{
		LoanID:         loanID,
		LoanNumber:     "KRD-TUNAI-KHUSUS",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolLancar,
		RequiredPPAP:   decimal.NewFromInt(50_000),
		LastDueDate:    &due,
	}}}
	svc := newTestPPAPService(repo, &stubProductRepo{}, &stubPosting{})
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "true"}}
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunanTunaiPPAP(loanID, asOf)}}

	summary, err := svc.Preview(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	item := summary.Items[0]
	if item.Collectibility != domain.KolKurangLancar {
		t.Fatalf("kolektibilitas %s, mau Kurang Lancar", item.Collectibility.Label())
	}
	if !item.Exposure.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("eksposur %s, mau baki penuh 10000000", item.Exposure)
	}
	if !item.CollateralValue.IsZero() {
		t.Fatalf("pengurang PPKA khusus %s, mau nol", item.CollateralValue)
	}
	if !item.Target.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("target %s, mau 1000000", item.Target)
	}
}

// Pasal 17 ayat (1) jo. Pasal 19 ayat (4) huruf b: porsi eksposur yang dijamin agunan
// tunai dibatasi pada eksposur. Jaminan tunai 15 juta atas baki 10 juta hanya
// mengecualikan 10 juta; kelebihannya tidak boleh membuat dasar negatif, dan porsi yang
// dijamin dilaporkan tidak melebihi baki.
func TestPPAPPreview_PorsiTunaiTidakMelebihiEksposur(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	due := asOf.AddDate(0, 0, -10)
	loanID := uuid.New()

	repo := &stubPPAPRepo{snapshots: []domain.PPAPLoanSnapshot{{
		LoanID:         loanID,
		LoanNumber:     "KRD-TUNAI-BATAS",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolLancar,
		RequiredPPAP:   decimal.NewFromInt(50_000),
		LastDueDate:    &due,
	}}}
	svc := newTestPPAPService(repo, &stubProductRepo{}, &stubPosting{})
	svc.config = &collateralConfigStub{values: map[string]string{"ppap.collateral.enabled": "true"}}
	agunan := agunanTunaiPPAP(loanID, asOf)
	agunan.AppraisalValue = decimal.NewFromInt(15_000_000)
	svc.collateralRepo = &collateralRepoStub{active: []domain.LoanCollateral{agunan}}

	summary, err := svc.Preview(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	item := summary.Items[0]
	if !item.CollateralValue.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("porsi dijamin %s, mau dibatasi 10000000", item.CollateralValue)
	}
	if !item.Exposure.IsZero() {
		t.Fatalf("eksposur %s, mau nol", item.Exposure)
	}
	if !item.Target.IsZero() {
		t.Fatalf("target %s, mau nol (tidak ada cadangan negatif)", item.Target)
	}
}
