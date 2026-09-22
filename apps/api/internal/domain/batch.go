package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// DailyActivitySummary adalah aktivitas jurnal satu tanggal bisnis, dihitung dari
// journal_entries/journal_lines agar EOD melaporkan angka nyata, bukan konstanta.
type DailyActivitySummary struct {
	PostedJournals int
	// TotalDepositAmount hanya setoran tunai teller (transaction_type = DEPOSIT),
	// bukan penempatan deposito berjangka. Penempatan dilaporkan terpisah lewat
	// DepositPlacementCount/TotalDepositPlacementAmount.
	TotalDepositAmount    decimal.Decimal
	TotalWithdrawalAmount decimal.Decimal
	// DepositPlacementCount dan TotalDepositPlacementAmount adalah penempatan deposito
	// berjangka (transaction_type = DEPOSIT_PLACEMENT) pada tanggal yang sama.
	//
	// Keterbatasan data lama: jurnal penempatan yang dibuat sebelum jenis
	// DEPOSIT_PLACEMENT ada (migrasi 000057) tetap bertipe DEPOSIT dan ikut terhitung
	// sebagai setoran tunai. Baris lama tidak punya penanda pasti untuk dipisahkan,
	// sehingga reklasifikasi berbasis tebakan sengaja tidak dilakukan.
	DepositPlacementCount       int
	TotalDepositPlacementAmount decimal.Decimal
	// SocialFundBalance adalah saldo akun 12500 "Dana Kebajikan" (LIABILITY, saldo
	// normal kredit) sampai dan termasuk tanggal bisnis ini: sisi kredit dikurangi
	// debit. Akun ini menampung denda keterlambatan pembiayaan syariah (ta'zir),
	// bukan pendapatan bank, dan menunggu keputusan penyaluran oleh Dewan Pengawas
	// Syariah. Kode 12500 berasal dari bagan akun (migrasi 000024/000068), sama
	// dengan bawaan kunci konfigurasi loan.penalty.syariah.social_fund.coa.
	SocialFundBalance decimal.Decimal
}

// BatchActivityRepository membaca aktivitas harian untuk ringkasan EOD.
type BatchActivityRepository interface {
	DailyActivity(ctx context.Context, date time.Time) (*DailyActivitySummary, error)
}

// ARORunner menjalankan perpanjangan otomatis deposito yang jatuh tempo. Dipisah
// dari DepositService agar batch tidak bergantung pada seluruh permukaan layanan
// deposito, sekaligus menghindari siklus dependensi.
type ARORunner interface {
	RunARO(ctx context.Context, asOf time.Time, actor Actor) (int, error)
}

// PPAPRunner menjalankan perhitungan kolektibilitas dan cadangan PPAP harian.
type PPAPRunner interface {
	RunDaily(ctx context.Context, asOf time.Time, actor Actor) (PPAPRunSummary, error)
}

// PPAPRunMarker mencatat tanggal bisnis run PPAP terakhir yang berhasil dan
// membacanya kembali. Perbandingan CKPN memakai required_ppap yang disimpan PPAP;
// tanpa penanda ini perbandingan tidak dapat membedakan angka tanggal bisnis berjalan
// dari angka run sebelumnya, sehingga pemanggilan manual di luar tutup hari bisa
// menyajikan dasar kemarin seolah sah.
//
// Dipisah dari PPAPService agar CKPN tidak memegang seluruh permukaan layanan PPAP,
// mengikuti pola ARORunner/PPAPRunner yang sempit.
type PPAPRunMarker interface {
	// RecordRun menyimpan tanggal bisnis run PPAP yang berhasil. Dipanggil hanya
	// pada jalur non-preview setelah seluruh kredit selesai diproses.
	//
	// updatedBy wajib staf yang benar-benar ada: system_config.updated_by punya FK
	// ke staff_users, sehingga uuid.Nil akan ditolak database dan menggagalkan run
	// PPAP yang sebenarnya sudah selesai menghitung.
	RecordRun(ctx context.Context, businessDate time.Time, updatedBy uuid.UUID) error
	// LastRunBusinessDate mengembalikan tanggal bisnis run PPAP terakhir. ok=false
	// berarti belum pernah ada run PPAP yang berhasil (bukan error).
	LastRunBusinessDate(ctx context.Context) (time.Time, bool, error)
}

// DormantRunSummary merangkum satu kali penandaan rekening dormant pada tutup hari.
// Warning diisi bila konfigurasi ambang tidak valid sehingga fallback terpakai;
// tanpa itu pekerjaan dapat tampak berjalan padahal memakai asumsi operator.
type DormantRunSummary struct {
	Marked  int    `json:"marked"`
	Skipped int    `json:"skipped"`
	Warning string `json:"warning,omitempty"`
}

// DormantRunner menandai rekening nasabah yang tidak ada aktivitas sebagai DORMANT.
// Mengikuti pola ARORunner/PPAPRunner: interface sempit agar batch tidak bergantung
// pada seluruh permukaan AccountService.
type DormantRunner interface {
	MarkDormant(ctx context.Context, asOf time.Time, actor Actor) (DormantRunSummary, error)
}

// LoanInterestAccrualRunner mengakru pendapatan bunga kredit konvensional berbasis
// jadwal angsuran. Mengikuti pola ARORunner/PPAPRunner/DormantRunner: interface sempit
// agar batch tidak bergantung pada seluruh permukaan LoanService.
//
// Amortisasi saldo kerugian restrukturisasi ikut di sini karena ia berjalan pada batch
// yang sama dan menyentuh kredit yang sama; pemisahan field baru hanya akan menambah
// parameter konstruktor tanpa manfaat.
type LoanInterestAccrualRunner interface {
	AccrueInterest(ctx context.Context, asOf time.Time, actor Actor) (LoanInterestAccrualSummary, error)
	RestructureLossAmortizationRunner
}

// RestructureLossAmortizationRunner memulihkan saldo kerugian restrukturisasi ke
// pendapatan bunga memakai metode suku bunga efektif (PA BPR Bab 5.2). Dipisah agar
// pemanggil yang hanya butuh amortisasi tidak bergantung pada akrual.
type RestructureLossAmortizationRunner interface {
	AmortizeRestructureLoss(ctx context.Context, asOf time.Time, actor Actor) (RestructureLossAmortizationSummary, error)
}

// SystemActor membangun identitas pelaku untuk pekerjaan batch yang tidak berasal
// dari permintaan HTTP. Username diisi id pengguna yang menjalankan batch supaya
// jurnal dan audit tetap dapat ditelusuri ke orang yang memicunya.
func SystemActor(executedBy uuid.UUID) Actor {
	return Actor{UserID: executedBy, Username: executedBy.String(), Role: RoleSystem}
}
