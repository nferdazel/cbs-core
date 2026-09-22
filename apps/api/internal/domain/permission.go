package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Izin kini bersumber dari database: pemetaan grup -> izin (group_permissions)
// dan keanggotaan pengguna -> grup (user_group_members). domain.RolePermissions
// hanya menjadi SEED AWAL migrasi 000079, bukan sumber kedua yang dibaca runtime.
// Bila keduanya berbeda, database yang menang; ketidaksinkronannya terlihat lewat
// uji invarian seed.
var (
	// ErrGroupNotFound dikembalikan bila kode grup tidak ada di user_groups.
	ErrGroupNotFound = errors.New("grup pengguna tidak ditemukan")
	// ErrUnknownPermission menolak pengajuan izin yang bukan konstanta Perm* kode.
	// Izin yang tidak ditegakkan kode tidak punya arti; menolaknya lebih jelas
	// daripada menyimpan baris yang tak pernah berpengaruh.
	ErrUnknownPermission = errors.New("izin tidak dikenal")
	// ErrPermissionChangeNeedsConfirmation menandai pencabutan izin yang akan
	// membuat pengguna kehilangan akses mendadak. Pemanggil harus menyetel
	// ConfirmAccessLoss agar keputusan itu disengaja, bukan kecelakaan.
	ErrPermissionChangeNeedsConfirmation = errors.New("pencabutan izin akan mencabut akses pengguna; konfirmasi diperlukan")
)

// AccessLossError membawa jumlah pengguna yang akan kehilangan izin sehingga
// pesan ke pembuat pengajuan menyebut angkanya, bukan sekadar "ditolak".
type AccessLossError struct {
	AffectedUsers int
}

func (e *AccessLossError) Error() string {
	return fmt.Sprintf("%s: %d pengguna akan kehilangan akses bila perubahan ini disetujui",
		ErrPermissionChangeNeedsConfirmation.Error(), e.AffectedUsers)
}

func (e *AccessLossError) Is(target error) bool {
	return target == ErrPermissionChangeNeedsConfirmation
}

// UserGroup adalah grup pengguna. Grup dipakai untuk akses menu dan tiering
// kewenangan persetujuan, BUKAN cakupan buku (lini usaha pada data).
type UserGroup struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	IsSystem    bool
	// ApprovalLimitRole menunjuk peran pada matriks limit 000051 yang menjadi
	// jenjang kewenangan grup ini. Kosong berarti peran pengguna sendiri.
	ApprovalLimitRole StaffRole
	Permissions       []Permission
}

// MenuDefinition adalah satu menu aplikasi beserta izin yang membukanya. Nol izin
// berarti menu tampil untuk semua pengguna terautentikasi (mis. beranda).
type MenuDefinition struct {
	MenuKey     string
	Permissions []Permission
}

// PermissionChangeOperation membedakan penambahan dari pencabutan izin.
type PermissionChangeOperation string

const (
	PermissionChangeGrant  PermissionChangeOperation = "GRANT"
	PermissionChangeRevoke PermissionChangeOperation = "REVOKE"
)

// PermissionChangeInput adalah isi pengajuan perubahan pemetaan izin.
type PermissionChangeInput struct {
	GroupCode  string
	Permission Permission
	Operation  PermissionChangeOperation
	// ConfirmAccessLoss wajib TRUE bila pencabutan akan membuat pengguna
	// kehilangan akses. Semantik perubahan tetap aditif: tanpa konfirmasi,
	// pencabutan yang berdampak ditolak, bukan dijalankan diam-diam.
	ConfirmAccessLoss bool
	Notes             string
}

// KnownPermission melaporkan apakah p adalah izin yang ditegakkan kode. Himpunan
// diambil dari RolePermissions (seed) karena setiap konstanta Perm* memang dipakai
// setidaknya satu peran; izin di luar itu tidak akan pernah diperiksa rute mana pun.
func KnownPermission(p Permission) bool {
	for _, perms := range RolePermissions {
		for _, known := range perms {
			if known == p {
				return true
			}
		}
	}
	return false
}

// NormalizePermissionOperation memvalidasi dan menormalkan operasi perubahan.
func NormalizePermissionOperation(raw string) (PermissionChangeOperation, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(PermissionChangeGrant):
		return PermissionChangeGrant, nil
	case string(PermissionChangeRevoke):
		return PermissionChangeRevoke, nil
	default:
		return "", fmt.Errorf("operasi perubahan izin tidak dikenal: %q", raw)
	}
}

// PermissionRepository membaca dan mengubah pemetaan grup/izin serta katalog menu.
// Operasi tulis yang menempel pada transaksi maker-checker menerima tx apa adanya
// agar persetujuan dan efeknya commit bersama.
type PermissionRepository interface {
	ListGroups(ctx context.Context) ([]UserGroup, error)
	ListMenus(ctx context.Context) ([]MenuDefinition, error)
	GroupExists(ctx context.Context, groupCode string) (bool, error)
	GroupHasPermission(ctx context.Context, groupCode string, p Permission) (bool, error)
	GrantPermissionTx(ctx context.Context, tx any, groupCode string, p Permission) error
	RevokePermissionTx(ctx context.Context, tx any, groupCode string, p Permission) error
	// CountUsersLosingPermission menghitung pengguna aktif yang izin efektifnya
	// akan kehilangan p bila p dicabut dari grup groupCode.
	CountUsersLosingPermission(ctx context.Context, groupCode string, p Permission) (int, error)
}

// PermissionService mengelola grup/izin dan mengajukan perubahannya lewat
// maker-checker. ExecuteApproved menjadikannya eksekutor action_type
// permission_change pada registry maker-checker.
type PermissionService interface {
	ListGroups(ctx context.Context) ([]UserGroup, error)
	ListMenus(ctx context.Context) ([]MenuDefinition, error)
	RequestChange(ctx context.Context, input PermissionChangeInput, actor Actor) (*MakerCheckerRequest, error)
	ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor Actor) error
}
