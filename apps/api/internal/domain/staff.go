package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

// --- Errors ---
var (
	ErrInvalidCredentials  = errors.New("invalid username or password")
	ErrAccountLocked       = errors.New("account is temporarily locked due to too many failed login attempts")
	ErrAccountInactiveUser = errors.New("user account is inactive")
	ErrSessionExpired      = errors.New("session has expired")
	ErrSessionRevoked      = errors.New("session has been revoked")
	ErrInvalidToken        = errors.New("invalid or malformed token")
	ErrForbidden           = errors.New("you do not have permission to perform this action")
	ErrPasswordExpired     = errors.New("password kedaluwarsa, ganti password terlebih dahulu")
	// ErrStaffRoleNotManageable menolak perubahan atas akun staf yang perannya
	// setingkat atau lebih tinggi dari pelaku.
	ErrStaffRoleNotManageable = errors.New("peran Anda tidak berwenang mengubah akun ini")
)

// --- Staff Role & Permissions ---

type StaffRole string

const (
	RoleSuperAdmin StaffRole = "SUPERADMIN"
	RoleAdmin      StaffRole = "ADMIN"
	RoleSupervisor StaffRole = "SUPERVISOR"
	RoleTeller     StaffRole = "TELLER"
	RoleCS         StaffRole = "CS"
	RoleAO         StaffRole = "AO"
	RoleAuditor    StaffRole = "AUDITOR"
	// RoleSystem menandai pelaku non-manusia, yaitu pekerjaan batch terjadwal yang
	// tidak berasal dari permintaan HTTP. Nilainya tidak pernah disimpan ke kolom
	// peran pegawai; hanya dipakai pada Actor di dalam proses.
	RoleSystem StaffRole = "SYSTEM"
)

// PrivilegeRank mengurutkan kewenangan peran untuk pengelolaan akun staf. Tanpa
// urutan ini ADMIN dapat mereset kata sandi SUPERADMIN dan mengambil alih kendali
// penuh sistem, atau menaikkan rekan ke peran di atasnya.
//
// AUDITOR diberi peringkat setara ADMIN supaya pengelola akun tidak dapat mengubah
// akun pemeriksa: independensi pemeriksa hanya boleh disentuh SUPERADMIN.
func (r StaffRole) PrivilegeRank() int {
	switch r {
	case RoleSuperAdmin:
		return 50
	case RoleAdmin, RoleAuditor:
		return 40
	case RoleSupervisor:
		return 30
	case RoleTeller, RoleCS, RoleAO:
		return 10
	default:
		return 0
	}
}

type Permission string

const (
	// User management
	PermUsersCreate Permission = "users:create"
	PermUsersRead   Permission = "users:read"
	PermUsersUpdate Permission = "users:update"
	PermUsersDelete Permission = "users:delete"

	// Master data cabang. Pembuatan cabang adalah fungsi administratif kantor
	// pusat, jadi hanya SUPERADMIN yang memegangnya.
	PermBranchesCreate Permission = "branches:create"

	// Customer (CIF)
	PermCustomersCreate Permission = "customers:create"
	PermCustomersRead   Permission = "customers:read"
	PermCustomersUpdate Permission = "customers:update"

	// Accounts
	PermAccountsOpen   Permission = "accounts:open"
	PermAccountsRead   Permission = "accounts:read"
	PermAccountsFreeze Permission = "accounts:freeze"
	PermAccountsClose  Permission = "accounts:close"

	// Products — data referensi produk (suku bunga, margin, biaya) yang dipakai
	// layar rekening, deposito, dan kredit.
	PermProductsRead Permission = "products:read"

	// Transactions
	PermTransactionsDeposit  Permission = "transactions:deposit"
	PermTransactionsWithdraw Permission = "transactions:withdraw"
	PermTransactionsTransfer Permission = "transactions:transfer"
	PermTransactionsReverse  Permission = "transactions:reverse"

	// Loans & Financing (Kredit BPR / Pembiayaan BMT)
	PermLoansApply   Permission = "loans:apply"
	PermLoansRead    Permission = "loans:read"
	PermLoansApprove Permission = "loans:approve"
	// PermLoansCancel membatalkan pencairan kredit sebelum ada angsuran dibayar.
	// Lebih sensitif daripada pencairan, jadi hanya peran tertinggi yang memegangnya.
	PermLoansCancel Permission = "loans:cancel"
	// PermLoansCorrect mengoreksi nominal pokok kredit yang sudah berjalan tanpa
	// membatalkan pencairan. Mengubah tagihan nasabah, jadi sama sensitifnya dengan
	// pembatalan dan hanya dipegang peran tertinggi.
	PermLoansCorrect Permission = "loans:correct"

	// Field Collections
	PermCollectionsInput Permission = "collections:input"

	// Maker-Checker
	PermMakerCheckerApprove Permission = "maker_checker:approve"
	PermMakerCheckerReject  Permission = "maker_checker:reject"

	// Ledger
	PermLedgerRead Permission = "ledger:read"

	// Chart of Accounts
	PermCOAManage Permission = "coa:manage"

	// Agunan kredit
	PermCollateralRead   Permission = "collateral:read"
	PermCollateralManage Permission = "collateral:manage"

	// Audit & Reports
	PermAuditLogsRead Permission = "audit_logs:read"
	PermReportsExport Permission = "reports:export"
	// PermReportsFinancialRead memberi akses ke laporan keuangan bank-wide
	// (neraca saldo, neraca, laba rugi, arus kas). Dipisahkan dari PermLedgerRead
	// karena laporan ini mencakup seluruh bank: peran cabang (AO/TELLER/CS) memang
	// perlu membaca mutasi rekening, tetapi tidak berhak melihat posisi keuangan
	// bank secara keseluruhan. Laporan operasional seperti due-obligations tetap
	// memakai PermLedgerRead.
	PermReportsFinancialRead Permission = "reports:financial:read"

	// System
	PermSystemConfig Permission = "system:config"
	// PermSystemConfigRead hanya membaca konfigurasi sistem (mis. batas transaksi
	// per peran) tanpa mengubahnya. Dipisahkan dari PermSystemConfig supaya peran
	// pengawas dapat meninjau kebijakan batas tanpa ikut mendapat wewenang menulis
	// konfigurasi, menjalankan EOD/EOM/EOY, atau mengubah state sistem.
	PermSystemConfigRead Permission = "system:config:read"
)

// RolePermissions is the canonical permission map — configurable via DB overrides.
var RolePermissions = map[StaffRole][]Permission{
	RoleSuperAdmin: {
		PermProductsRead,
		PermUsersCreate, PermUsersRead, PermUsersUpdate, PermUsersDelete,
		PermBranchesCreate,
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsOpen, PermAccountsRead, PermAccountsFreeze, PermAccountsClose,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer, PermTransactionsReverse,
		PermLoansApply, PermLoansRead, PermLoansApprove, PermCollectionsInput,
		PermLoansCancel, PermLoansCorrect,
		PermCollateralRead, PermCollateralManage,
		PermMakerCheckerApprove, PermMakerCheckerReject,
		PermLedgerRead, PermCOAManage,
		PermAuditLogsRead, PermReportsExport, PermReportsFinancialRead,
		PermSystemConfig, PermSystemConfigRead,
	},
	RoleAdmin: {
		PermProductsRead,
		PermUsersCreate, PermUsersRead, PermUsersUpdate,
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsOpen, PermAccountsRead, PermAccountsFreeze, PermAccountsClose,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer, PermTransactionsReverse,
		PermLoansApply, PermLoansRead, PermLoansApprove, PermCollectionsInput,
		PermLoansCancel, PermLoansCorrect,
		PermCollateralRead, PermCollateralManage,
		PermMakerCheckerApprove, PermMakerCheckerReject,
		PermLedgerRead, PermCOAManage,
		PermAuditLogsRead, PermReportsExport, PermReportsFinancialRead,
		PermSystemConfigRead,
	},
	RoleSupervisor: {
		PermProductsRead,
		PermUsersRead,
		PermCustomersRead, PermCustomersUpdate,
		PermAccountsRead, PermAccountsFreeze,
		PermTransactionsReverse,
		PermLoansRead, PermLoansApprove,
		PermCollateralRead, PermCollateralManage,
		PermMakerCheckerApprove, PermMakerCheckerReject,
		PermLedgerRead,
		PermAuditLogsRead, PermReportsExport, PermReportsFinancialRead,
		PermSystemConfigRead,
	},
	RoleTeller: {
		PermProductsRead,
		PermCustomersCreate, PermCustomersRead,
		PermAccountsOpen, PermAccountsRead,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer,
		PermCollectionsInput,
		PermLedgerRead,
	},
	RoleCS: {
		PermProductsRead,
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsRead,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer,
		PermLedgerRead,
	},
	RoleAO: {
		PermProductsRead,
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsRead,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer,
		PermLoansApply, PermLoansRead,
		PermCollectionsInput,
		PermLedgerRead,
	},
	RoleAuditor: {
		PermProductsRead,
		PermUsersRead,
		PermCustomersRead,
		PermAccountsRead,
		PermLoansRead,
		PermLedgerRead,
		PermAuditLogsRead, PermReportsExport, PermReportsFinancialRead,
		PermSystemConfigRead,
	},
}

// HasPermission checks if a role has a given permission.
func (r StaffRole) HasPermission(p Permission) bool {
	perms, ok := RolePermissions[r]
	if !ok {
		return false
	}
	for _, perm := range perms {
		if perm == p {
			return true
		}
	}
	return false
}

// --- Staff User Entity ---

type StaffUser struct {
	ID           uuid.UUID `json:"id"`
	EmployeeID   string    `json:"employee_id"`
	Username     string    `json:"username"`
	FullName     string    `json:"full_name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialised
	Role         StaffRole `json:"role"`
	BranchCode   string    `json:"branch_code"`
	// Book adalah buku COA (konvensional/syariah) akun staf. Kosong berarti belum
	// ditentukan; nilainya dibaca ulang setiap permintaan lewat SessionIdentity,
	// sehingga perubahan buku berlaku pada permintaan berikutnya.
	Book              COABook    `json:"book,omitempty"`
	IsActive          bool       `json:"is_active"`
	LastLoginAt       *time.Time `json:"last_login_at,omitempty"`
	PasswordChangedAt time.Time  `json:"password_changed_at"`
	FailedLoginCount  int        `json:"-"`
	LockedUntil       *time.Time `json:"-"`
	CreatedBy         *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

// IsLocked returns true if the account is currently locked.
func (u *StaffUser) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// --- Staff Session Entity ---

type StaffSession struct {
	ID               uuid.UUID  `json:"id"`
	UserID           uuid.UUID  `json:"user_id"`
	RefreshTokenHash string     `json:"-"`
	IPAddress        string     `json:"ip_address"`
	UserAgent        string     `json:"user_agent"`
	ExpiresAt        time.Time  `json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// SessionIdentity adalah keadaan sesi dan akun saat sebuah permintaan diverifikasi.
// Nilainya dibaca dari basis data, bukan dari klaim token: token dibuat sekali dan tidak
// dapat mencerminkan logout, penguncian akun, penonaktifan pengguna, pencabutan sesi,
// atau perubahan peran yang terjadi setelahnya.
type SessionIdentity struct {
	SessionID   uuid.UUID
	UserID      uuid.UUID
	Username    string
	Role        StaffRole
	BranchCode  string
	Book        COABook
	IsActive    bool
	LockedUntil *time.Time
}

// IsLocked menandai akun yang sedang terkunci karena percobaan masuk gagal berulang.
func (i *SessionIdentity) IsLocked() bool {
	return i.LockedUntil != nil && i.LockedUntil.After(time.Now())
}

// IsValid returns true if the session is not expired and not revoked.
func (s *StaffSession) IsValid() bool {
	return s.RevokedAt == nil && s.ExpiresAt.After(time.Now())
}

// --- JWT Claims ---

type JWTClaims struct {
	UserID     uuid.UUID `json:"uid"`
	Username   string    `json:"username"`
	Role       StaffRole `json:"role"`
	BranchCode string    `json:"branch"`
	// Book adalah buku COA pengguna pada saat token diterbitkan (dan disegarkan
	// saat validasi dari baris staff_users). Kosong = belum ditentukan.
	Book COABook `json:"book,omitempty"`
	// BookScope adalah cakupan buku tingkat instalasi. TIDAK ditanam di token:
	// middleware mengisinya setiap permintaan dari system_config
	// institution.book_scope sehingga perubahan setelan langsung berlaku dan token
	// lama tidak dapat melanggarnya.
	BookScope InstitutionBookScope `json:"-"`
	SessionID uuid.UUID            `json:"sid"`
	// PasswordExpired menandai token yang diterbitkan saat kata sandi sudah
	// kedaluwarsa. Token semacam ini hanya boleh dipakai untuk mengganti kata
	// sandi (lihat middleware.RequirePasswordChange), bukan ditolak saat login,
	// supaya pengguna tetap punya jalur pulih.
	PasswordExpired bool `json:"pwd_expired"`
}

// ToActor membangun identitas pelaku dari claims. IP address diisi terpisah oleh
// handler dari koneksi, bukan dari token.
// ToActor membangun identitas pelaku dari claims. ip dan requestID berasal dari
// permintaan HTTP; requestID dipakai untuk mengkorelasikan aksi bisnis dengan
// application log permintaan asalnya.
func (c *JWTClaims) ToActor(ip, requestID string) Actor {
	return Actor{
		UserID:     c.UserID,
		Username:   c.Username,
		Role:       c.Role,
		BranchCode: c.BranchCode,
		Book:       c.Book,
		BookScope:  c.BookScope,
		SessionID:  c.SessionID,
		IPAddress:  ip,
		RequestID:  requestID,
	}
}

// ContextKey for storing claims in request context
type contextKey string

const ContextKeyClaims contextKey = "staff_claims"

// ClaimsFromContext extracts JWT claims from context.
func ClaimsFromContext(ctx context.Context) (*JWTClaims, bool) {
	v, ok := ctx.Value(ContextKeyClaims).(*JWTClaims)
	return v, ok
}

// --- Input / Output DTOs ---

type LoginInput struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	IPAddress string `json:"-"`
	UserAgent string `json:"-"`
}

// LoginResponse membawa token ke handler (yang menuliskannya sebagai httpOnly
// cookie) tetapi token TIDAK diserialisasi ke body. Hanya expires_in dan profil
// user yang dikirim ke klien.
type LoginResponse struct {
	AccessToken      string `json:"-"`
	RefreshToken     string `json:"-"`
	ExpiresIn        int    `json:"expires_in"`         // access token, detik
	RefreshExpiresIn int    `json:"refresh_expires_in"` // refresh token, detik
	// PasswordExpired memberitahu klien bahwa token yang baru diterbitkan hanya
	// boleh dipakai untuk mengganti kata sandi, sehingga antarmuka dapat
	// mengarahkan ke halaman ganti kata sandi.
	PasswordExpired bool       `json:"password_expired"`
	User            *StaffUser `json:"user"`
}

type RefreshInput struct {
	RefreshToken string `json:"refresh_token"`
}

type CreateStaffInput struct {
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	Email      string    `json:"email"`
	Password   string    `json:"password"`
	Role       StaffRole `json:"role"`
	BranchCode string    `json:"branch_code"`
}

type UpdateStaffInput struct {
	FullName   *string    `json:"full_name,omitempty"`
	Email      *string    `json:"email,omitempty"`
	Role       *StaffRole `json:"role,omitempty"`
	BranchCode *string    `json:"branch_code,omitempty"`
	IsActive   *bool      `json:"is_active,omitempty"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// --- Repository & Service Interfaces ---

type StaffRepository interface {
	Create(ctx context.Context, user *StaffUser) error
	GetByID(ctx context.Context, id uuid.UUID) (*StaffUser, error)
	GetByUsername(ctx context.Context, username string) (*StaffUser, error)
	List(ctx context.Context, limit, offset int) ([]StaffUser, int, error)
	Update(ctx context.Context, user *StaffUser) error
	IncrementFailedLogin(ctx context.Context, id uuid.UUID) error
	LockAccount(ctx context.Context, id uuid.UUID, until time.Time) error
	ResetFailedLogin(ctx context.Context, id uuid.UUID) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error
}

type SessionRepository interface {
	Create(ctx context.Context, session *StaffSession) error
	GetByTokenHash(ctx context.Context, hash string) (*StaffSession, error)
	// GetIdentity mengambil sesi yang masih berlaku beserta pengguna pemiliknya.
	// Mengembalikan ErrSessionExpired bila sesi tidak ada, sudah dicabut, atau kedaluwarsa.
	GetIdentity(ctx context.Context, sessionID uuid.UUID) (*SessionIdentity, error)
	RevokeByID(ctx context.Context, sessionID uuid.UUID) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

type SystemConfigRepository interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, updatedBy uuid.UUID) error
	GetAll(ctx context.Context) (map[string]string, error)
}

type AuthService interface {
	Login(ctx context.Context, input LoginInput) (*LoginResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	ValidateAccessToken(ctx context.Context, tokenString string) (*JWTClaims, error)
}

type StaffService interface {
	CreateStaff(ctx context.Context, input CreateStaffInput, actor Actor) (*StaffUser, error)
	GetStaff(ctx context.Context, id uuid.UUID) (*StaffUser, error)
	ListStaff(ctx context.Context, page, pageSize int) ([]StaffUser, int, error)
	UpdateStaff(ctx context.Context, id uuid.UUID, input UpdateStaffInput, actor Actor) (*StaffUser, error)
	ChangePassword(ctx context.Context, id uuid.UUID, input ChangePasswordInput, actor Actor) error
	ResetPassword(ctx context.Context, id uuid.UUID, newPassword string, actor Actor) error
}
