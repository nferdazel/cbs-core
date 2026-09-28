package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// SavingsAccountAggregate adalah satu rekening tabungan (produk keluarga SAVINGS)
// pada posisi akhir periode, kebutuhan Form 11.00 "Daftar Tabungan".
//
// Hanya field yang benar-benar tersimpan yang diisi. Kolom Form 11.00 tanpa sumber
// (mis. Nomor Identitas, Jenis PEP, Risiko Nasabah, Status Data, nominal diblokir,
// biaya transaksi belum diamortisasi) TIDAK diturunkan di sini; form menulisnya "-"
// beserta alasan, bukan nol.
//
// CustomerTypeCode memakai sandi Lampiran 02 - Daftar Sandi Pihak Lawan
// (customers.ojk_pihak_lawan_code), sumber resmi kolom VI "Golongan Nasabah".
// Kosong berarti bank belum mengisi; laporan menulis "-", bukan menebak.
type SavingsAccountAggregate struct {
	AccountNumber   string
	CounterpartyCIF string
	// CustomerTypeCode adalah sandi Golongan Nasabah (Lampiran 02).
	CustomerTypeCode string
	// HubunganBankCode adalah sandi inline Hubungan dengan Bank (12/20).
	HubunganBankCode string
	// LocationCode adalah sandi Kabupaten/Kota nasabah (Lampiran 03).
	LocationCode string
	// ProfitScheme adalah skema imbal hasil produk (INTEREST/WADIAH/MUDHARABAH/...).
	// Hanya INTEREST yang dinyatakan sebagai suku bunga persen; skema lain ditulis "-".
	ProfitScheme string
	// InterestRateAnnual adalah rate_annual produk dalam persen per tahun. Hanya
	// bermakna bila ProfitScheme INTEREST.
	InterestRateAnnual decimal.Decimal
	// Balance adalah saldo tabungan pada akhir periode, direkonstruksi dari jurnal.
	Balance decimal.Decimal
}

// SavingsAccountReportRepository menyediakan baris per rekening tabungan pada posisi
// akhir periode (asOf), bukan keadaan saat ekspor dijalankan. Saldo direkonstruksi
// dari jurnal (entry_date <= asOf), sama seperti pola BankDepositRepository.
type SavingsAccountReportRepository interface {
	ListSavingsAccountsForOJK(ctx context.Context, asOf time.Time) ([]SavingsAccountAggregate, error)
}
