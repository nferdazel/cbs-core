package service

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// applyDirection adalah satu-satunya tempat saldo akun berubah. Salah di sini berarti
// saldo seluruh bank salah, jadi perilakunya dikunci dengan test.
func TestApplyDirectionDebitNormalAccount(t *testing.T) {
	balance := decimal.NewFromInt(1000)

	got, err := applyDirection(domain.BalanceTypeDebit, balance, domain.DirectionDebit, decimal.NewFromInt(250))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(decimal.NewFromInt(1250)) {
		t.Fatalf("akun debit bertambah saat didebit: dapat %s", got)
	}

	got, err = applyDirection(domain.BalanceTypeDebit, balance, domain.DirectionCredit, decimal.NewFromInt(250))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(decimal.NewFromInt(750)) {
		t.Fatalf("akun debit berkurang saat dikredit: dapat %s", got)
	}
}

func TestApplyDirectionCreditNormalAccount(t *testing.T) {
	balance := decimal.NewFromInt(1000)

	got, err := applyDirection(domain.BalanceTypeCredit, balance, domain.DirectionCredit, decimal.NewFromInt(250))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(decimal.NewFromInt(1250)) {
		t.Fatalf("akun kredit bertambah saat dikredit: dapat %s", got)
	}

	got, err = applyDirection(domain.BalanceTypeCredit, balance, domain.DirectionDebit, decimal.NewFromInt(250))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(decimal.NewFromInt(750)) {
		t.Fatalf("akun kredit berkurang saat didebit: dapat %s", got)
	}
}

// Kewajiban bank kepada nasabah bersaldo normal kredit: setoran menambah kewajiban.
func TestSavingsLiabilityGrowsOnDeposit(t *testing.T) {
	got, _ := applyDirection(domain.BalanceTypeCredit, decimal.Zero, domain.DirectionCredit, decimal.NewFromInt(500000))
	if !got.Equal(decimal.NewFromInt(500000)) {
		t.Fatalf("setoran harus menambah saldo kewajiban: dapat %s", got)
	}
}

// Baris jurnal harus selalu diproses dalam urutan nomor akun yang sama, karena urutan
// pengambilan kunci baris menentukan apakah transaksi paralel bisa deadlock.
func TestSortPostingLinesOrdersByAccountNumber(t *testing.T) {
	lines := []domain.PostingLine{
		{AccountNumber: "20100", Direction: domain.DirectionCredit, Amount: decimal.NewFromInt(1)},
		{AccountNumber: "10101", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(1)},
		{AccountNumber: "11100", Direction: domain.DirectionDebit, Amount: decimal.NewFromInt(1)},
	}

	sorted := sortPostingLines(lines)
	want := []string{"10101", "11100", "20100"}
	for i, w := range want {
		if sorted[i].AccountNumber != w {
			t.Fatalf("posisi %d = %s, mau %s (urutan: %v)", i, sorted[i].AccountNumber, w, accountNumbers(sorted))
		}
	}

	// Input asli tidak boleh berubah: pemanggil mungkin masih memakainya.
	if lines[0].AccountNumber != "20100" {
		t.Fatal("sortPostingLines tidak boleh mengubah slice masukan")
	}
}

// Akun kas ditentukan buku produk: transaksi syariah tidak boleh menyentuh kas
// konvensional, karena laporan arus kas kedua buku dipisah.
func TestLiabilityCOAForProductSeparatesBooks(t *testing.T) {
	savings := &domain.BankingProduct{Code: "TAB-CONV", Family: domain.FamilySavings, Book: domain.BookConventional}
	savingsSyariah := &domain.BankingProduct{Code: "TAB-SYAR", Family: domain.FamilySavings, Book: domain.BookSyariah}

	conv, err := liabilityCOAForProduct(savings)
	if err != nil {
		t.Fatal(err)
	}
	shar, err := liabilityCOAForProduct(savingsSyariah)
	if err != nil {
		t.Fatal(err)
	}
	if conv == shar {
		t.Fatalf("tabungan konvensional dan syariah tidak boleh memakai akun kewajiban yang sama (%s)", conv)
	}

	// Produk kredit tidak boleh dipakai membuka rekening simpanan.
	loan := &domain.BankingProduct{Code: "KRD-FLAT", Family: domain.FamilyLoan, Book: domain.BookConventional}
	if _, err := liabilityCOAForProduct(loan); err == nil {
		t.Fatal("produk kredit harus ditolak saat membuka rekening simpanan")
	}
}

func accountNumbers(lines []domain.PostingLine) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = l.AccountNumber
	}
	return out
}
