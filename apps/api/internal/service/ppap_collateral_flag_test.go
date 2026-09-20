package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Saklar ppap.collateral.enabled harus benar-benar menghentikan pembacaan agunan, bukan
// hanya membuat hasilnya nol: kalau query tetap jalan, tutup hari menanggung beban modul
// yang belum diaktifkan. Stub di sini tidak mengimplementasikan agregat selain lewat
// penanda, sehingga pemanggilan tak terduga akan terlihat.
func TestPPAPCollateralValues_SaklarMatiTidakMembacaAgunan(t *testing.T) {
	repo := &collateralRepoStub{sums: map[uuid.UUID]decimal.Decimal{uuid.New(): decimal.NewFromInt(1000)}}
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
	if repo.sumCalled {
		t.Fatal("agunan tidak boleh dibaca selama saklar pengurangan mati")
	}
}

func TestPPAPCollateralValues_SaklarHidupMembacaSekaliUntukSemuaKredit(t *testing.T) {
	loanA, loanB := uuid.New(), uuid.New()
	repo := &collateralRepoStub{sums: map[uuid.UUID]decimal.Decimal{
		loanA: decimal.NewFromInt(250000),
		loanB: decimal.NewFromInt(0),
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
	if !repo.sumCalled {
		t.Fatal("agunan harus dibaca saat saklar pengurangan hidup")
	}
	if !values[loanA].Equal(decimal.NewFromInt(250000)) {
		t.Fatalf("nilai agunan kredit A %s, mau 250000", values[loanA])
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
