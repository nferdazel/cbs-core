package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// SavingsCustomerAggregate adalah satu baris agregasi laporan internal jenis nasabah
// per produk (dulu diberi nomor Form 00.14 — nomor itu tidak ada di SEOJK 16/2024,
// lihat docs/CELAH-FORM-OJK.md §6): jumlah rekening dan total nominal per produk
// simpanan menurut golongan nasabah.
//
// Golongan nasabah memakai sandi Lampiran 02 – Daftar Sandi Pihak Lawan, sama dengan
// "Golongan Nasabah" pada Form 11.00/12.00 SEOJK 16/2024 dan kolom "Jenis Debitur"
// Form 06.00, sehingga tidak ada kolom baru. CustomerTypeCode kosong berarti bank
// belum mengisi sandinya lewat jalur nasabah; laporan menulis "-", bukan menebak dari
// nama atau jenis identitas.
type SavingsCustomerAggregate struct {
	ProductFamily    ProductFamily
	ProductCode      string
	ProductName      string
	CustomerTypeCode string
	// AccountCount adalah jumlah rekening/kontrak simpanan pada kelompok ini.
	AccountCount int
	// TotalAmount adalah total saldo tabungan/giro atau nominal penempatan deposito
	// dalam rupiah penuh.
	TotalAmount decimal.Decimal
}

// SavingsCustomerRepository menyediakan agregasi laporan internal jenis nasabah per
// produk (dulu diberi nomor Form 00.14; lihat docs/CELAH-FORM-OJK.md §6) pada posisi
// akhir periode (asOf), bukan keadaan saat ekspor dijalankan. Saldo rekening simpanan
// direkonstruksi dari jurnal (entry_date <= asOf); kontrak deposito dipilih dari
// tanggal mulai dan penutupannya.
type SavingsCustomerRepository interface {
	ListSavingsCustomerAggregates(ctx context.Context, asOf time.Time) ([]SavingsCustomerAggregate, error)
}
