package domain

import (
	"context"
	"time"

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
}

// BankDepositRepository menyediakan agregasi Form 13.00 pada posisi akhir periode
// (asOf), bukan keadaan saat ekspor dijalankan. Saldo rekening simpanan direkonstruksi
// dari jurnal (entry_date <= asOf); kontrak deposito dipilih dari tanggal mulai dan
// penutupannya. Sistem TIDAK menyimpan riwayat saldo yang diblokir (accounts.hold_balance)
// per akhir bulan, sehingga komponen itu tidak ikut direkonstruksi.
type BankDepositRepository interface {
	ListBankDeposits(ctx context.Context, asOf time.Time) ([]BankDepositAggregate, error)
}
