package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

const testCustomerAccountNumber = "1110001234"

// cashKeyConfig mengembalikan nilai khusus untuk kunci kas tersatu (cash.coa.*) dan
// fallback untuk kunci lain. Dipakai membuktikan angsuran tunai benar-benar membaca
// kunci yang sama dengan jurnal transaksi rekening.
type cashKeyConfig struct {
	conventional string
	syariah      string
}

func (c cashKeyConfig) GetDecimal(context.Context, string, decimal.Decimal) decimal.Decimal {
	return decimal.Zero
}
func (c cashKeyConfig) GetInt(_ context.Context, _ string, fallback int) int { return fallback }
func (c cashKeyConfig) GetString(_ context.Context, key, fallback string) string {
	// Literal, bukan konstanta: test harus menangkap bila kode memindahkan pembacaan
	// ke kunci lain (mis. kembali ke payment.cash.coa.*).
	switch key {
	case "cash.coa.conventional":
		return c.conventional
	case "cash.coa.syariah":
		return c.syariah
	default:
		return fallback
	}
}
func (c cashKeyConfig) GetBool(_ context.Context, _ string, fallback bool) bool { return fallback }
func (c cashKeyConfig) Invalidate(string)                                       {}

var _ domain.SystemConfigService = cashKeyConfig{}

// debitAccounts mengumpulkan nomor akun yang didebit pada seluruh jurnal yang tercatat.
func debitAccounts(requests []domain.PostingRequest) map[string]bool {
	found := map[string]bool{}
	for _, req := range requests {
		for _, line := range req.Lines {
			if line.Direction == domain.DirectionDebit {
				found[line.AccountNumber] = true
			}
		}
	}
	return found
}

// Angsuran tunai harus mendebit kas teller, bukan rekening nasabah: uangnya diterima
// di kas, dan mendebit rekening nasabah akan mengurangi saldonya tanpa setoran.
func TestPayInstallment_TunaiMendebitKasTeller(t *testing.T) {
	f := paymentFixture(60, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Method: domain.LoanPaymentCash,
	}, domain.Actor{Username: "teller.uji"}); err != nil {
		t.Fatalf("PayInstallment tunai: %v", err)
	}

	if len(f.posting.requests) == 0 {
		t.Fatal("angsuran tunai tidak menghasilkan jurnal")
	}
	debits := debitAccounts(f.posting.requests)
	if !debits["10101"] {
		t.Fatalf("kas teller tidak didebit; debit: %v", debits)
	}
	if debits[testCustomerAccountNumber] {
		t.Fatalf("rekening nasabah ikut didebit pada pembayaran tunai: %v", debits)
	}
}

// Angsuran tunai dan jurnal rekening memakai SATU kunci kas per buku
// (cash.coa.conventional), bukan kunci lama payment.cash.coa.conventional. Mengubah
// nilai cash.coa.conventional harus mengubah akun kas angsuran tunai; kode lama
// mengabaikannya dan tetap memakai bawaan 10101 sehingga test ini gagal.
func TestPayInstallment_TunaiMemakaiKunciKasTersatu(t *testing.T) {
	f := paymentFixture(60, 100)
	svc := newInterestTestService(f)
	svc.config = cashKeyConfig{conventional: "10102", syariah: "11102"}

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Method: domain.LoanPaymentCash,
	}, domain.Actor{Username: "teller.uji"}); err != nil {
		t.Fatalf("PayInstallment tunai: %v", err)
	}
	debits := debitAccounts(f.posting.requests)
	if !debits["10102"] {
		t.Fatalf("kas dari cash.coa.conventional tidak didebit; debit: %v", debits)
	}
	if debits["10101"] {
		t.Fatalf("angsuran tunai masih memakai akun kas bawaan/lama: %v", debits)
	}
}

// Tanpa metode (perilaku lama) angsuran tetap mendebit rekening nasabah.
func TestPayInstallment_BawaanMendebitRekeningNasabah(t *testing.T) {
	f := paymentFixture(60, 100)
	svc := newInterestTestService(f)

	if _, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1,
	}, domain.Actor{Username: "teller.uji"}); err != nil {
		t.Fatalf("PayInstallment: %v", err)
	}

	debits := debitAccounts(f.posting.requests)
	if !debits[testCustomerAccountNumber] {
		t.Fatalf("rekening nasabah tidak didebit; debit: %v", debits)
	}
	if debits["10101"] {
		t.Fatalf("kas teller ikut didebit pada pembayaran lewat rekening: %v", debits)
	}
}

// Metode di luar ACCOUNT/CASH ditolak sebelum menyentuh kredit.
func TestPayInstallment_MetodeTidakDikenalDitolak(t *testing.T) {
	f := paymentFixture(60, 100)
	svc := newInterestTestService(f)

	_, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Method: domain.LoanPaymentMethod("TRANSFER"),
	}, domain.Actor{Username: "teller.uji"})
	if err == nil {
		t.Fatal("metode pembayaran tidak dikenal harus ditolak")
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}

// Nominal sebagian dicatat sebagai angsuran sebagian: sisa pokok angsuran tetap ada
// dan statusnya berubah, bukan ditandai lunas.
func TestPayInstallment_SebagianMencatatStatusPartial(t *testing.T) {
	f := paymentFixture(60, 100)
	svc := newInterestTestService(f)

	schedule, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Amount: decimal.NewFromInt(500),
	}, domain.Actor{Username: "teller.uji"})
	if err != nil {
		t.Fatalf("PayInstallment sebagian: %v", err)
	}

	if schedule.Status != domain.InstallmentStatusPartial {
		t.Fatalf("status angsuran %q, ingin %q", schedule.Status, domain.InstallmentStatusPartial)
	}
	if !f.repo.paidPrincipal.Equal(decimal.NewFromInt(400)) {
		t.Fatalf("pokok terbayar %s, ingin 400 (500 - 100 bunga)", f.repo.paidPrincipal)
	}
	if !f.repo.paidProfit.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("bunga terbayar %s, ingin 100", f.repo.paidProfit)
	}
	if schedule.PaidAt != nil {
		t.Fatal("angsuran sebagian belum boleh bertanggal lunas")
	}
}

// Nominal melebihi kewajiban ditolak: kelebihan pembayaran belum punya jalur.
func TestPayInstallment_KelebihanDitolak(t *testing.T) {
	f := paymentFixture(0, 100)
	svc := newInterestTestService(f)

	_, err := svc.PayInstallment(context.Background(), domain.PayInstallmentInput{
		LoanID: f.loanID, InstallmentNo: 1, Amount: decimal.NewFromInt(2_000),
	}, domain.Actor{Username: "teller.uji"})
	if err == nil {
		t.Fatal("kelebihan pembayaran harus ditolak")
	}
	if len(f.posting.requests) != 0 {
		t.Fatalf("jurnal %d, ingin 0", len(f.posting.requests))
	}
}
