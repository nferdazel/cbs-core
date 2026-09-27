package domain

import (
	"context"

	"github.com/shopspring/decimal"
)

// BankDepositAggregate adalah satu baris agregasi Form 13.00 "Daftar Simpanan dari
// Bank Lain": simpanan (tabungan/giro) dan deposito berjangka milik nasabah yang
// bergolongan bank, dikelompokkan per bank lawan, kantor, dan jenis simpanan.
//
// Bank lawan dikenali dari sandi Lampiran 02 Daftar Sandi Pihak Lawan
// (customers.ojk_pihak_lawan_code): 001 Bank Indonesia, 600 BPR, 601 BPRS, 700 Bank
// Umum, 701 Bank Umum Syariah, dan 901 Unit Usaha Syariah. Nasabah tanpa sandi bank
// tidak dapat dikenali sebagai bank lawan dan tidak masuk agregasi ini.
//
// Jenis memakai sandi Form 13.00 kolom VII: "01" Tabungan, "02" Deposito.
type BankDepositAggregate struct {
	BranchCode       string
	CounterpartyCIF  string
	JenisBankCode    string
	HubunganBankCode string
	LocationCode     string
	Jenis            string
	// AccountCount adalah jumlah rekening/kontrak satuan pada kelompok ini.
	AccountCount int
	// TotalNominal adalah total saldo tabungan/giro atau nominal penempatan deposito
	// dalam rupiah penuh.
	TotalNominal decimal.Decimal
	// TotalBlocked adalah total saldo yang diblokir/dijaminkan (accounts.hold_balance)
	// pada kelompok ini.
	TotalBlocked decimal.Decimal
}

// BankDepositRepository menyediakan agregasi Form 13.00. Posisi yang dibaca adalah
// keadaan saat ini, bukan potret historis: sistem belum menyimpan riwayat saldo
// simpanan per akhir bulan, sehingga periode lampau tidak dapat direkonstruksi.
type BankDepositRepository interface {
	ListBankDeposits(ctx context.Context) ([]BankDepositAggregate, error)
}
