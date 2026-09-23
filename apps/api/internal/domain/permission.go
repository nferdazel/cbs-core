package domain

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
	// ErrUserNotFound dikembalikan bila pengguna yang akan dijadikan anggota tidak ada.
	ErrUserNotFound = errors.New("pengguna tidak ditemukan")
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
	// Members adalah pengguna yang tergabung dalam grup ini. Ditampilkan halaman
	// pengelolaan izin agar bank dapat melihat siapa yang terdampak sebelum
	// mengubah pemetaan; keanggotaan bersifat aditif dan tidak mengikat pekerjaan.
	Members []GroupMember
}

// GroupMember adalah satu pengguna yang tergabung dalam sebuah grup. Dibaca dari
// staff_users; hanya field yang aman ditampilkan (tanpa kata sandi).
type GroupMember struct {
	UserID   uuid.UUID
	Username string
	FullName string
	Role     StaffRole
	IsActive bool
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
	// PermissionChangeAddMember dan PermissionChangeRemoveMember mengelola
	// keanggotaan pengguna pada grup. Operasi ini tidak membawa Permission; yang
	// berubah adalah siapa yang tergabung, sehingga izin efektifnya ikut berubah
	// lewat jalur maker-checker yang sama dengan perubahan pemetaan izin.
	PermissionChangeAddMember    PermissionChangeOperation = "ADD_MEMBER"
	PermissionChangeRemoveMember PermissionChangeOperation = "REMOVE_MEMBER"
)

// PermissionChangeInput adalah isi pengajuan perubahan pemetaan izin atau keanggotaan
// grup. UserID diisi untuk operasi anggota (ADD_MEMBER/REMOVE_MEMBER) dan diabaikan
// untuk operasi izin.
type PermissionChangeInput struct {
	GroupCode  string
	Permission Permission
	Operation  PermissionChangeOperation
	// UserID adalah pengguna yang ditambahkan/dikeluarkan dari grup (string UUID).
	UserID string
	// ConfirmAccessLoss wajib TRUE bila pencabutan akan membuat pengguna
	// kehilangan akses. Semantik perubahan tetap aditif: tanpa konfirmasi,
	// pencabutan yang berdampak ditolak, bukan dijalankan diam-diam.
	ConfirmAccessLoss bool
	Notes             string
}

// IsMemberOperation melaporkan apakah operasi mengelola keanggotaan grup.
func (o PermissionChangeOperation) IsMemberOperation() bool {
	return o == PermissionChangeAddMember || o == PermissionChangeRemoveMember
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

// KnownPermissions mengembalikan seluruh izin yang ditegakkan kode, terurut dan
// tanpa duplikat. Dipakai halaman pengelolaan izin sebagai daftar pilihan sehingga
// web tidak menyimpan salinan izin di kode dan hanya bisa mengajukan izin yang
// benar-benar diperiksa rute.
func KnownPermissions() []Permission {
	seen := map[Permission]bool{}
	out := []Permission{}
	for _, perms := range RolePermissions {
		for _, p := range perms {
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// NormalizePermissionOperation memvalidasi dan menormalkan operasi perubahan.
func NormalizePermissionOperation(raw string) (PermissionChangeOperation, error) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case string(PermissionChangeGrant):
		return PermissionChangeGrant, nil
	case string(PermissionChangeRevoke):
		return PermissionChangeRevoke, nil
	case string(PermissionChangeAddMember):
		return PermissionChangeAddMember, nil
	case string(PermissionChangeRemoveMember):
		return PermissionChangeRemoveMember, nil
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
	// CountAccessLossOnMemberRemoval menghitung pengguna yang kehilangan izin
	// efektif bila userID dikeluarkan dari grup groupCode. Hasilnya 0 atau 1:
	// penghapusan hanya menyentuh satu pengguna. Dipakai agar penghapusan anggota
	// yang mencabut akses dituntut konfirmasi, sama seperti pencabutan izin.
	CountAccessLossOnMemberRemoval(ctx context.Context, groupCode string, userID uuid.UUID) (int, error)
	// UserExists melaporkan apakah pengguna staf ada. Mencegah pengajuan keanggotaan
	// untuk pengguna yang tidak dikenal.
	UserExists(ctx context.Context, userID uuid.UUID) (bool, error)
	// UserInGroup melaporkan apakah pengguna sudah menjadi anggota grup.
	UserInGroup(ctx context.Context, groupCode string, userID uuid.UUID) (bool, error)
	// AddMemberTx dan RemoveMemberTx mengubah keanggotaan di dalam transaksi
	// maker-checker yang sama, sehingga persetujuan dan efeknya commit bersama.
	AddMemberTx(ctx context.Context, tx any, groupCode string, userID uuid.UUID) error
	RemoveMemberTx(ctx context.Context, tx any, groupCode string, userID uuid.UUID) error
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
