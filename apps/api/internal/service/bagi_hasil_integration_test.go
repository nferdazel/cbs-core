package service_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Pembiayaan mudharabah tidak boleh lagi menghasilkan jadwal ber-profit nol.
// Produk bagi hasil memakai tarif ekuivalen = nisbah x proyeksi pendapatan usaha
// (angka PROYEKSI, disesuaikan saat realisasi). Bila produk belum mengisi salah
// satu parameter, pengajuan ditolak dengan pesan yang menyebut apa yang harus
// diisi bank.
func TestIntegrasiBagiHasilMudharabahProyeksiNisbah(t *testing.T) {
	e := newMoneyEnv(t)

	productCode := fmt.Sprintf("PMB-UJI-%08d", (time.Now().UnixNano()/1e6)%100_000_000)
	var productID uuid.UUID
	if err := e.db.QueryRowContext(e.ctx, `
		INSERT INTO banking_products (code, name, family, book, profit_scheme, schedule_method,
			rate_annual, profit_sharing_ratio, projected_revenue_rate_annual,
			min_amount, min_term_months, max_term_months, is_active)
		VALUES ($1, 'Pembiayaan Uji Bagi Hasil', 'LOAN', 'SYARIAH', 'MUDHARABAH', 'BAGI_HASIL',
			0, 0.4, 12, 1000000, 1, 120, TRUE)
		RETURNING id`, productCode).Scan(&productID); err != nil {
		t.Fatalf("menyiapkan produk bagi hasil: %v", err)
	}
	product, err := e.productRepo.GetByID(e.ctx, productID)
	if err != nil {
		t.Fatalf("membaca produk bagi hasil: %v", err)
	}
	if !product.ProjectedRevenueRateAnnual.Equal(decimal.NewFromInt(12)) {
		t.Fatalf("kolom proyeksi tidak terbaca: %s", product.ProjectedRevenueRateAnnual)
	}

	cust := e.newCustomer(t, "Nasabah Bagi Hasil", fmt.Sprintf("bagihasil-%d@uji.local", time.Now().UnixNano()))
	acc := e.newAccount(t, cust.ID)

	apply := func() (*domain.Loan, error) {
		return e.loanSvc.ApplyLoan(e.ctx, domain.ApplyLoanInput{
			CustomerID:            cust.ID,
			ProductID:             product.ID,
			DisbursementAccountID: acc,
			PrincipalAmount:       decimal.NewFromInt(12_000_000),
			TermMonths:            12,
			Purpose:               "uji integrasi bagi hasil",
		}, e.actor)
	}

	loan, err := apply()
	if err != nil {
		t.Fatalf("pengajuan pembiayaan bagi hasil: %v", err)
	}
	if len(loan.Schedules) != 12 {
		t.Fatalf("jumlah angsuran %d, ingin 12", len(loan.Schedules))
	}
	sumProfit := decimal.Zero
	for _, sc := range loan.Schedules {
		if sc.ProfitType != domain.ProfitTypeBagiHasil {
			t.Fatalf("profit type angsuran %d = %s, ingin BAGI_HASIL", sc.InstallmentNo, sc.ProfitType)
		}
		if !sc.ProfitAmount.IsPositive() {
			t.Fatalf("angsuran %d ber-profit nol: proyeksi dari nisbah tidak dipakai", sc.InstallmentNo)
		}
		sumProfit = sumProfit.Add(sc.ProfitAmount)
	}
	// 12.000.000 x (0,4 x 12%) x 12/12 = 576.000.
	if !sumProfit.Equal(decimal.NewFromInt(576_000)) {
		t.Fatalf("total proyeksi bagi hasil %s, ingin 576000", sumProfit)
	}

	// Produk tanpa proyeksi pendapatan ditolak, bukan diterbitkan ber-profit nol.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE banking_products SET projected_revenue_rate_annual = 0 WHERE id = $1`, product.ID); err != nil {
		t.Fatalf("mengosongkan proyeksi produk: %v", err)
	}
	if _, err := apply(); !errors.Is(err, domain.ErrBagiHasilProjectionMissing) {
		t.Fatalf("produk tanpa proyeksi: err = %v, ingin ErrBagiHasilProjectionMissing", err)
	}

	// Produk tanpa nisbah juga ditolak.
	if _, err := e.db.ExecContext(e.ctx, `UPDATE banking_products SET projected_revenue_rate_annual = 12, profit_sharing_ratio = 0 WHERE id = $1`, product.ID); err != nil {
		t.Fatalf("mengosongkan nisbah produk: %v", err)
	}
	if _, err := apply(); !errors.Is(err, domain.ErrBagiHasilNisbahMissing) {
		t.Fatalf("produk tanpa nisbah: err = %v, ingin ErrBagiHasilNisbahMissing", err)
	}
}
