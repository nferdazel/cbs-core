package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrLimitPerTransaction = NewLocalizedError("limit_per_transaction", "nominal melebihi batas per transaksi")
	ErrLimitDaily          = NewLocalizedError("limit_daily", "akumulasi transaksi harian melebihi batas")
	ErrRequiresApproval    = NewLocalizedError("requires_approval", "nominal melebihi ambang yang memerlukan persetujuan")
)

// TransactionLimit adalah batas transaksi seorang pelaku untuk satu jenis transaksi.
type TransactionLimit struct {
	PerTransaction        decimal.Decimal
	DailyAmount           decimal.Decimal
	RequiresApprovalAbove decimal.Decimal
}

// TransactionLimitView adalah satu baris batas efektif untuk dibaca antarmuka.
// Nilai uang dikirim sebagai decimal agar antena JSON menulisnya sebagai string
// (shopspring/decimal mempertahankan presisi dengan mengutip angka), dan Configured
// menandai apakah nilai berasal dari system_config atau masih bawaan aplikasi.
type TransactionLimitView struct {
	Role            StaffRole       `json:"role"`
	TransactionType string          `json:"transaction_type"`
	PerTransaction  decimal.Decimal `json:"per_transaction"`
	DailyLimit      decimal.Decimal `json:"daily_limit"`
	ApprovalAbove   decimal.Decimal `json:"approval_above"`
	Configured      bool            `json:"configured"`
}

// DailyDebitSumReader menyediakan bahan akumulasi harian penjaga batas tanpa memuat
// seluruh repository jurnal. Dipakai menghitung akumulasi harian yang mengikat pelaku.
//
// Seluruh parameter businessDate adalah TANGGAL BISNIS bank (WIB), bukan tanggal
// kalender UTC: jurnal membawa entry_date dari tanggal bisnis, sehingga menyaring
// dengan created_at/tanggal UTC membuat jurnal hari bisnis berjalan tidak terhitung
// saat tutup hari tertinggal.
type DailyDebitSumReader interface {
	SumDebitByCreatedByAndDate(ctx context.Context, createdBy string, businessDate time.Time) (decimal.Decimal, error)
	// SumPendingDebitByMakerAndAction menjumlahkan nominal pengajuan maker-checker yang
	// masih PENDING milik satu pembuat untuk satu jenis aksi pada satu TANGGAL BISNIS.
	// Tanpa ini N pengajuan yang masing-masing di bawah batas harian dapat disetujui
	// semua sehingga total yang terposting hari itu melampaui batas.
	SumPendingDebitByMakerAndAction(ctx context.Context, maker, actionType string, businessDate time.Time) (decimal.Decimal, error)
	// LockDailyEvaluation menyerialkan evaluasi batas harian terhadap evaluasi lain
	// untuk pembuat, jenis transaksi, dan tanggal bisnis yang sama, di dalam transaksi
	// eksekusi. Kunci diambil SEBELUM jurnal ditulis; dua persetujuan bersamaan tidak
	// boleh sama-sama membaca snapshot "masih di bawah batas" lalu sama-sama posting.
	LockDailyEvaluation(ctx context.Context, tx any, maker, txType string, businessDate time.Time) error
}

// ApprovalLimitRoleResolver menyelesaikan peran pada matriks limit 000051 yang
// menjadi jenjang kewenangan persetujuan seorang pelaku, dibaca dari grup pengguna
// (user_groups.approval_limit_role, migrasi 000079). Ini KAIT ke matriks yang sudah
// ada, bukan matriks limit kedua: nilai batas tetap dibaca dari kunci
// limit.<peran>.<jenis>.* yang sama. Bila tidak ada grup yang menunjuk peran lain,
// peran pelaku sendiri yang dipakai, sehingga perilaku lama tidak berubah.
type ApprovalLimitRoleResolver interface {
	ResolveApprovalLimitRole(ctx context.Context, userID uuid.UUID, role StaffRole) (StaffRole, error)
}

type TransactionLimitService interface {
	// ForActor mengembalikan batas yang berlaku bagi role pelaku pada jenis transaksi.
	ForActor(ctx context.Context, actor Actor, txType string) (TransactionLimit, error)
	// Check memvalidasi nominal terhadap batas per transaksi, akumulasi harian, dan
	// ambang persetujuan. Pelanggaran dikembalikan sebagai sentinel error di domain.
	Check(ctx context.Context, actor Actor, txType string, amount decimal.Decimal) error
	// CheckDailyAtExecution mengevaluasi ULANG hanya batas harian ketika pengajuan yang
	// sudah disetujui dieksekusi, di dalam transaksi eksekusi (tx). Persetujuan pejabat
	// melegalkan nominal di atas ambang dan batas per transaksi, tetapi tidak boleh
	// melegalkan pelanggaran batas harian; karena itu ambang persetujuan dan batas per
	// transaksi tidak diperiksa di sini. maker adalah pembuat pengajuan, bukan pejabat
	// yang menyetujui. tx dipakai mengunci evaluasi secara serial terhadap eksekusi lain
	// untuk pembuat/jenis/tanggal bisnis yang sama; nil diperbolehkan hanya pada stub uji.
	CheckDailyAtExecution(ctx context.Context, tx any, maker Actor, txType string, amount decimal.Decimal) error
	// List mengembalikan batas efektif seluruh peran x jenis transaksi yang dijaga
	// penjaga batas, beserta penanda configured. Dipakai endpoint baca system/limits.
	List(ctx context.Context) ([]TransactionLimitView, error)
}
