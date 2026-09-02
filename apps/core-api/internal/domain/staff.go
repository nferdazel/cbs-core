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
	ErrPasswordExpired     = errors.New("password has expired, please change it")
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
)

type Permission string

const (
	// User management
	PermUsersCreate Permission = "users:create"
	PermUsersRead   Permission = "users:read"
	PermUsersUpdate Permission = "users:update"
	PermUsersDelete Permission = "users:delete"

	// Customer (CIF)
	PermCustomersCreate Permission = "customers:create"
	PermCustomersRead   Permission = "customers:read"
	PermCustomersUpdate Permission = "customers:update"

	// Accounts
	PermAccountsOpen   Permission = "accounts:open"
	PermAccountsRead   Permission = "accounts:read"
	PermAccountsFreeze Permission = "accounts:freeze"
	PermAccountsClose  Permission = "accounts:close"

	// Transactions
	PermTransactionsDeposit  Permission = "transactions:deposit"
	PermTransactionsWithdraw Permission = "transactions:withdraw"
	PermTransactionsTransfer Permission = "transactions:transfer"
	PermTransactionsReverse  Permission = "transactions:reverse"

	// Loans & Financing (Kredit BPR / Pembiayaan BMT)
	PermLoansApply   Permission = "loans:apply"
	PermLoansRead    Permission = "loans:read"
	PermLoansApprove Permission = "loans:approve"

	// Field Collections
	PermCollectionsInput Permission = "collections:input"

	// Maker-Checker
	PermMakerCheckerApprove Permission = "maker_checker:approve"
	PermMakerCheckerReject  Permission = "maker_checker:reject"

	// Ledger
	PermLedgerRead Permission = "ledger:read"

	// Chart of Accounts
	PermCOAManage Permission = "coa:manage"

	// Audit & Reports
	PermAuditLogsRead  Permission = "audit_logs:read"
	PermReportsExport  Permission = "reports:export"

	// System
	PermSystemConfig Permission = "system:config"
)

// RolePermissions is the canonical permission map — configurable via DB overrides.
var RolePermissions = map[StaffRole][]Permission{
	RoleSuperAdmin: {
		PermUsersCreate, PermUsersRead, PermUsersUpdate, PermUsersDelete,
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsOpen, PermAccountsRead, PermAccountsFreeze, PermAccountsClose,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer, PermTransactionsReverse,
		PermLoansApply, PermLoansRead, PermLoansApprove, PermCollectionsInput,
		PermMakerCheckerApprove, PermMakerCheckerReject,
		PermLedgerRead, PermCOAManage,
		PermAuditLogsRead, PermReportsExport,
		PermSystemConfig,
	},
	RoleAdmin: {
		PermUsersCreate, PermUsersRead, PermUsersUpdate,
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsOpen, PermAccountsRead, PermAccountsFreeze, PermAccountsClose,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer, PermTransactionsReverse,
		PermLoansApply, PermLoansRead, PermLoansApprove, PermCollectionsInput,
		PermMakerCheckerApprove, PermMakerCheckerReject,
		PermLedgerRead, PermCOAManage,
		PermAuditLogsRead, PermReportsExport,
	},
	RoleSupervisor: {
		PermUsersRead,
		PermCustomersRead, PermCustomersUpdate,
		PermAccountsRead, PermAccountsFreeze,
		PermTransactionsReverse,
		PermLoansRead, PermLoansApprove,
		PermMakerCheckerApprove, PermMakerCheckerReject,
		PermLedgerRead,
		PermAuditLogsRead, PermReportsExport,
	},
	RoleTeller: {
		PermCustomersCreate, PermCustomersRead,
		PermAccountsOpen, PermAccountsRead,
		PermTransactionsDeposit, PermTransactionsWithdraw, PermTransactionsTransfer,
		PermCollectionsInput,
		PermLedgerRead,
	},
	RoleCS: {
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsRead,
		PermLedgerRead,
	},
	RoleAO: {
		PermCustomersCreate, PermCustomersRead, PermCustomersUpdate,
		PermAccountsRead,
		PermLoansApply, PermLoansRead,
		PermCollectionsInput,
	},
	RoleAuditor: {
		PermUsersRead,
		PermCustomersRead,
		PermAccountsRead,
		PermLoansRead,
		PermLedgerRead,
		PermAuditLogsRead, PermReportsExport,
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
	ID                  uuid.UUID  `json:"id"`
	EmployeeID          string     `json:"employee_id"`
	Username            string     `json:"username"`
	FullName            string     `json:"full_name"`
	Email               string     `json:"email"`
	PasswordHash        string     `json:"-"` // never serialised
	Role                StaffRole  `json:"role"`
	BranchCode          string     `json:"branch_code"`
	IsActive            bool       `json:"is_active"`
	LastLoginAt         *time.Time `json:"last_login_at,omitempty"`
	PasswordChangedAt   time.Time  `json:"password_changed_at"`
	FailedLoginCount    int        `json:"-"`
	LockedUntil         *time.Time `json:"-"`
	CreatedBy           *uuid.UUID `json:"created_by,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
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
	SessionID  uuid.UUID `json:"sid"`
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

type LoginResponse struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	ExpiresIn    int        `json:"expires_in"` // seconds
	User         *StaffUser `json:"user"`
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
	CreateStaff(ctx context.Context, input CreateStaffInput, createdBy uuid.UUID) (*StaffUser, error)
	GetStaff(ctx context.Context, id uuid.UUID) (*StaffUser, error)
	ListStaff(ctx context.Context, page, pageSize int) ([]StaffUser, int, error)
	UpdateStaff(ctx context.Context, id uuid.UUID, input UpdateStaffInput) (*StaffUser, error)
	ChangePassword(ctx context.Context, id uuid.UUID, input ChangePasswordInput) error
	ResetPassword(ctx context.Context, id uuid.UUID, newPassword string, resetBy uuid.UUID) error
}
