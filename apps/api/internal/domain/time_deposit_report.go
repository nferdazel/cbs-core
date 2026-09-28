package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// TimeDepositAggregate adalah satu kontrak deposito berjangka pada posisi akhir
// periode, kebutuhan Form 12.00 "Daftar Deposito".
//
// Hanya field yang benar-benar tersimpan yang diisi. Kolom Form 12.00 tanpa sumber
// (mis. Nomor Identitas, Jenis PEP, Risiko Nasabah, Status Data, nominal diblokir,
// alasan diblokir, biaya transaksi belum diamortisasi) TIDAK diturunkan di sini; form
// menulisnya "-" beserta alasan, bukan nol.
//
// CustomerTypeCode memakai sandi Lampiran 02 - Daftar Sandi Pihak Lawan, sumber resmi
// kolom V "Golongan Nasabah". Kosong berarti bank belum mengisi; laporan menulis "-".
type TimeDepositAggregate struct {
	AccountNumber   string
	CounterpartyCIF string
	// CustomerTypeCode adalah sandi Golongan Nasabah (Lampiran 02).
	CustomerTypeCode string
	// HubunganBankCode adalah sandi inline Hubungan dengan Bank (12/20).
	HubunganBankCode string
	// LocationCode adalah sandi Kabupaten/Kota nasabah (Lampiran 03).
	LocationCode string
	// ProfitType adalah jenis imbal hasil kontrak (INTEREST/MARGIN/BAGI_HASIL).
	// Hanya INTEREST yang dinyatakan sebagai suku bunga persen; sisanya ditulis "-".
	ProfitType string
	// PlacementAmount adalah pokok penempatan deposito, sumber kolom IX Nominal.
	PlacementAmount decimal.Decimal
	// StartDate dan MaturityDate adalah tanggal mulai dan jatuh tempo perjanjian
	// terakhir, sumber kolom VII Jangka Waktu.
	StartDate    time.Time
	MaturityDate time.Time
	// ProfitRate adalah bunga tahunan (%) untuk INTEREST, atau nisbah (0-1) untuk
	// bagi hasil. Hanya bermakna sebagai persen bila ProfitType INTEREST.
	ProfitRate decimal.Decimal
}

// TimeDepositReportRepository menyediakan baris per kontrak deposito berjangka pada
// posisi akhir periode (asOf): kontrak yang sudah ditempatkan (start_date <= asOf) dan
// belum ditutup/dicairkan pada asOf (closed_at NULL atau setelah asOf).
type TimeDepositReportRepository interface {
	ListTimeDepositsForOJK(ctx context.Context, asOf time.Time) ([]TimeDepositAggregate, error)
}
