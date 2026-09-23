package domain

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrAccountNotFound = NewLocalizedError("account_not_found", "rekening tidak ditemukan")
	// ErrAccountDormant menandai rekening pasif yang tidak boleh didebit. Nasabah
	// harus melakukan reaktivasi di cabang sebelum dana keluar.
	ErrAccountDormant = NewLocalizedError("account_dormant", "rekening dormant: nasabah harus melakukan reaktivasi di cabang")
	// ErrAccountNotDormant dipakai endpoint reaktivasi: rekening yang statusnya bukan
	// DORMANT ditolak beserta status sebenarnya, bukan diam-diam dianggap sukses.
	ErrAccountNotDormant = NewLocalizedError("account_not_dormant", "rekening tidak berstatus dormant dan tidak dapat direaktivasi")
	// ErrAccountNotFrozen dipakai endpoint unfreeze: rekening yang statusnya bukan
	// FROZEN ditolak beserta status sebenarnya, bukan diam-diam dianggap aktif.
	// Melengkapi jalur masuk FROZEN (dibekukan) dengan jalur keluarnya.
	ErrAccountNotFrozen = NewLocalizedError("account_not_frozen", "rekening tidak berstatus dibekukan dan tidak dapat dibatalkan pembekuannya")
	// ErrAccountNotFreezable dipakai endpoint freeze: hanya rekening ACTIVE yang boleh
	// dibekukan. Membekukan rekening DORMANT/CLOSED/FROZEN akan menyamarkan status
	// sebenarnya, jadi ditolak beserta status saat ini.
	ErrAccountNotFreezable = NewLocalizedError("account_not_freezable", "rekening tidak berstatus aktif dan tidak dapat dibekukan")
	// ErrAccountUnfreezeSameActor menegakkan pemisahan tugas: pembekuan tidak boleh
	// dibatalkan oleh orang yang membekukannya. Dipakai langsung (bukan maker-checker)
	// karena yang dibutuhkan adalah orang KEDUA, bukan sekadar persetujuan.
	ErrAccountUnfreezeSameActor = NewLocalizedError("account_unfreeze_same_actor", "pembekuan tidak dapat dibatalkan oleh pelaksana pembekuan yang sama")
	// ErrAccountCloseBalance menolak penutupan rekening yang masih menyimpan saldo,
	// saldo tersedia, atau dana tertahan. Uang nasabah tidak boleh hilang karena
	// rekening ditutup sebelum bersih.
	ErrAccountCloseBalance = NewLocalizedError("account_close_balance", "rekening tidak dapat ditutup: masih ada saldo, saldo tersedia, atau dana tertahan")
	// ErrAccountNotClosable menolak penutupan rekening yang statusnya bukan ACTIVE
	// atau DORMANT (mis. sudah CLOSED/FROZEN), beserta status sebenarnya.
	ErrAccountNotClosable = NewLocalizedError("account_not_closable", "rekening tidak dapat ditutup pada status saat ini")
)

// DormantAfterMonthsFallback adalah ambang sementara agar proses tetap berjalan di
// lingkungan yang belum di-provision. Nilai produksi WAJIB diisi operator lewat
// system_config "account.dormant.after_months"; pemakaian fallback menghasilkan
// peringatan pada ringkasan EOD, bukan sukses diam-diam.
const DormantAfterMonthsFallback = 12

type AccountType string

const (
	AccountTypeSavings    AccountType = "SAVINGS"
	AccountTypeChecking   AccountType = "CHECKING"
	AccountTypeLoan       AccountType = "LOAN"
	AccountTypeInternalGL AccountType = "INTERNAL_GL"
)

type AccountStatus string

const (
	AccountStatusActive  AccountStatus = "ACTIVE"
	AccountStatusDormant AccountStatus = "DORMANT"
	AccountStatusFrozen  AccountStatus = "FROZEN"
	AccountStatusClosed  AccountStatus = "CLOSED"
)

type Account struct {
	ID            uuid.UUID  `json:"id"`
	AccountNumber string     `json:"account_number"`
	CustomerID    *uuid.UUID `json:"customer_id,omitempty"`
	CustomerName  string     `json:"customer_name,omitempty"`
	ProductID     *uuid.UUID `json:"product_id,omitempty"`
	BranchID      *uuid.UUID `json:"branch_id,omitempty"`
	BranchCode    string     `json:"branch_code,omitempty"` // kode cabang rekening; kosong = data lama
	COAID         uuid.UUID  `json:"coa_id"`
	COACode       string     `json:"coa_code,omitempty"`
	// COABook adalah buku COA rekening (konvensional/syariah). Dipakai untuk
	// menolak transaksi yang mencampur buku, mis. pembiayaan syariah yang
	// mencairkan dana ke rekening konvensional.
	COABook          COABook         `json:"coa_book,omitempty"`
	NormalBalance    BalanceType     `json:"normal_balance"`
	AccountType      AccountType     `json:"account_type"`
	Currency         string          `json:"currency"`
	Balance          decimal.Decimal `json:"balance"`
	AvailableBalance decimal.Decimal `json:"available_balance"`
	HoldBalance      decimal.Decimal `json:"hold_balance"`
	Status           AccountStatus   `json:"status"`
	Version          int             `json:"version"`
	OpenedAt         *time.Time      `json:"opened_at,omitempty"`
	// LastActivityAt adalah waktu transaksi berhasil terakhir pada rekening. Menjadi
	// dasar penilaian dormant; nil berarti belum ada transaksi sehingga opened_at
	// atau created_at dipakai sebagai gantinya.
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
	// DormantAt adalah waktu rekening ditandai dormant; nil bila tidak sedang dormant.
	DormantAt *time.Time `json:"dormant_at,omitempty"`
	// FrozenBy adalah pelaksana pembekuan terakhir. Dipakai menegakkan pemisahan tugas:
	// pembekuan tidak boleh dibatalkan oleh orang yang sama (lihat ErrAccountUnfreezeSameActor).
	// Nil berarti pembekuan lama (pra-migrasi) atau rekening tidak dibekukan lewat API.
	FrozenBy *uuid.UUID `json:"frozen_by,omitempty"`
	// FrozenAt adalah waktu pembekuan terakhir; nil bila tidak sedang dibekukan.
	FrozenAt  *time.Time `json:"frozen_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// AccountDebitAllowed mengembalikan error bila status rekening tidak boleh didebit.
// Debit (penarikan, transfer keluar) hanya boleh dari rekening ACTIVE: rekening
// dormant harus direaktivasi lebih dulu di cabang. FROZEN dan CLOSED tetap ditolak
// dengan ErrAccountInactive, sehingga pesan keduanya tidak tertukar.
func AccountDebitAllowed(status AccountStatus) error {
	switch status {
	case AccountStatusActive:
		return nil
	case AccountStatusDormant:
		return ErrAccountDormant
	default:
		return ErrAccountInactive
	}
}

// AccountCreditAllowed mengembalikan error bila status rekening tidak boleh dikredit.
// Kredit (setoran, transfer masuk) diterima untuk ACTIVE maupun DORMANT: dana masuk
// tidak boleh tertahan hanya karena rekening pasif, sementara justru dana keluarlah
// yang perlu dilindungi. Setoran TIDAK mengaktifkan kembali rekening; reaktivasi
// tetap harus eksplisit di cabang. FROZEN dan CLOSED tetap ditolak.
func AccountCreditAllowed(status AccountStatus) error {
	switch status {
	case AccountStatusActive, AccountStatusDormant:
		return nil
	default:
		return ErrAccountInactive
	}
}

// AccountBalanceFloorBreached melaporkan apakah saldo baru sebuah rekening nasabah
// akan turun di bawah nol. Rekening GL internal dikecualikan karena akun kontra
// seperti cadangan PPAP (10900) memang bersaldo negatif menurut normal balance-nya,
// dan saldo GL agregat bukan dana nasabah.
//
// Aturan ini ditegakkan di mesin posting, bukan hanya di service pemanggil.
// Pemeriksaan di pemanggil membaca saldo di luar transaksi, sehingga dua penarikan
// paralel dapat sama-sama lolos dan mengeksekusi rekening menjadi negatif; hanya
// pemeriksaan setelah baris rekening dikunci yang benar-benar mengikat.
func AccountBalanceFloorBreached(accountType AccountType, newBalance decimal.Decimal) bool {
	if accountType == AccountTypeInternalGL {
		return false
	}
	return newBalance.IsNegative()
}

// DormantCutoff menghitung batas waktu penandaan dormant: rekening yang aktivitas
// terakhirnya sebelum cutoff dianggap sudah pasif. afterMonths dijamin > 0 oleh
// pemanggil (lihat dormantAfterMonths).
func DormantCutoff(asOf time.Time, afterMonths int) time.Time {
	return asOf.AddDate(0, -afterMonths, 0)
}

// AccountActivityBase menentukan dasar waktu aktivitas rekening untuk penilaian
// dormant: COALESCE(last_activity_at, opened_at, created_at). Hasil kedua false
// berarti tidak ada waktu yang bisa dipakai (rekening lama tanpa opened_at dan
// created_at); rekening seperti itu dilewati, bukan dikarang waktunya.
func AccountActivityBase(lastActivityAt, openedAt *time.Time, createdAt time.Time) (time.Time, bool) {
	if lastActivityAt != nil {
		return *lastActivityAt, true
	}
	if openedAt != nil {
		return *openedAt, true
	}
	if !createdAt.IsZero() {
		return createdAt, true
	}
	return time.Time{}, false
}

// IsDormantDue menentukan apakah rekening sudah melewati ambang dormant.
// Perbandingan ketat (Before) membuat aktivitas tepat di ambang belum dianggap
// dormant; batas eksklusif ini disengaja agar tidak ada penandaan lebih awal.
func IsDormantDue(base, cutoff time.Time) bool {
	return base.Before(cutoff)
}

// OpenAccountInput membuka rekening dari produk. COA kewajiban ditentukan produk,
// bukan dipilih teller, supaya pemetaan akuntansi konsisten.
type OpenAccountInput struct {
	CustomerID uuid.UUID `json:"customer_id"`
	ProductID  uuid.UUID `json:"product_id"`
	Currency   string    `json:"currency"`
	BranchCode string    `json:"branch_code"`
}

type AccountRepository interface {
	Create(ctx context.Context, account *Account) error
	CreateTx(ctx context.Context, tx *sql.Tx, account *Account) error
	GetByID(ctx context.Context, id uuid.UUID) (*Account, error)
	GetByNumber(ctx context.Context, accountNumber string) (*Account, error)
	GetByNumberForUpdate(ctx context.Context, tx any, accountNumber string) (*Account, error)
	ListByCustomer(ctx context.Context, customerID uuid.UUID) ([]Account, error)
	// ListAll mengembalikan daftar rekening yang boleh dibaca aktor. Filter cabang
	// diterapkan di query agar pagination dan total tetap benar.
	ListAll(ctx context.Context, limit, offset int, search string, actor Actor) ([]Account, int, error)
	UpdateBalance(ctx context.Context, tx any, accountID uuid.UUID, balance, available decimal.Decimal, version int) error
	// ListDormantCandidates mengembalikan rekening nasabah (SAVINGS/CHECKING) yang
	// masih ACTIVE untuk dinilai dormannya saat EOD. actor membatasi hasil pada buku
	// yang aktif di instalasi.
	ListDormantCandidates(ctx context.Context, actor Actor) ([]Account, error)
	// MarkDormant menandai satu rekening DORMANT hanya bila masih ACTIVE, sehingga
	// aman dijalankan ulang (idempoten). Hasil false berarti tidak ada yang diubah.
	MarkDormant(ctx context.Context, tx any, accountID uuid.UUID) (bool, error)
	// Reactivate memulihkan rekening DORMANT ke ACTIVE. Hasil false berarti rekening
	// tidak lagi DORMANT (mis. balapan dengan aksi lain).
	Reactivate(ctx context.Context, tx any, accountID uuid.UUID, reactivatedAt time.Time) (bool, error)
	// Freeze membekukan rekening ACTIVE. Penjaga status = 'ACTIVE' menolak rekening
	// DORMANT/CLOSED/FROZEN, sehingga status tidak tersamar. frozenBy dicatat agar
	// pembatalan pembekuan oleh orang yang sama dapat ditolak.
	Freeze(ctx context.Context, tx any, accountID, frozenBy uuid.UUID, frozenAt time.Time) (bool, error)
	// Unfreeze memulihkan rekening FROZEN ke ACTIVE. Hasil false berarti rekening
	// tidak lagi FROZEN (mis. balapan dengan aksi lain).
	Unfreeze(ctx context.Context, tx any, accountID uuid.UUID) (bool, error)
	// Close menutup rekening ACTIVE/DORMANT yang sudah bersih (saldo nol). Hasil
	// false berarti status tidak lagi dapat ditutup (mis. sudah CLOSED).
	Close(ctx context.Context, tx any, accountID uuid.UUID) (bool, error)
}

type AccountService interface {
	OpenAccount(ctx context.Context, input OpenAccountInput, actor Actor) (*Account, error)
	// GetAccountByNumber menolak rekening cabang lain dengan ErrCrossBranchAccess,
	// bukan menyamarkannya sebagai tidak ditemukan.
	GetAccountByNumber(ctx context.Context, accountNumber string, actor Actor) (*Account, error)
	ListAccounts(ctx context.Context, page, pageSize int, search string, actor Actor) ([]Account, int, error)
	// ReactivateAccount memulihkan rekening dormant ke ACTIVE. Rekening yang bukan
	// DORMANT ditolak ErrAccountNotDormant; cabang lain ditolak ErrCrossBranchAccess.
	ReactivateAccount(ctx context.Context, accountNumber, notes string, actor Actor) (*Account, error)
	// FreezeAccount membekukan rekening ACTIVE. Rekening yang bukan ACTIVE ditolak
	// ErrAccountNotFreezable; cabang/buku lain ditolak sebelum status diperiksa.
	// Pelaksana dicatat sebagai FrozenBy agar unfreeze oleh orang yang sama ditolak.
	FreezeAccount(ctx context.Context, accountNumber, notes string, actor Actor) (*Account, error)
	// UnfreezeAccount memulihkan rekening FROZEN ke ACTIVE, jalur keluar dari
	// pembekuan. Rekening yang bukan FROZEN ditolak ErrAccountNotFrozen; cabang lain
	// ditolak ErrCrossBranchAccess; pelaksana pembekuan yang sama ditolak
	// ErrAccountUnfreezeSameActor.
	UnfreezeAccount(ctx context.Context, accountNumber, notes string, actor Actor) (*Account, error)
	// CloseAccount menutup rekening ACTIVE/DORMANT yang bersih (saldo nol). Rekening
	// dengan saldo/dana tertahan ditolak ErrAccountCloseBalance, status yang tidak
	// dapat ditutup ditolak ErrAccountNotClosable, dan cabang lain ditolak
	// ErrCrossBranchAccess.
	CloseAccount(ctx context.Context, accountNumber, notes string, actor Actor) (*Account, error)
	// MarkDormant menandai rekening pasif bank-wide. Dipanggil batch EOD, bukan HTTP.
	MarkDormant(ctx context.Context, asOf time.Time, actor Actor) (DormantRunSummary, error)
}
