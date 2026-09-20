package domain

import (
	"errors"

	"github.com/google/uuid"
)

// ErrCrossBranchAccess menandai operasi tulis yang ditolak karena objeknya berada
// di cabang lain dari cabang aktor. Handlernya memetakan error ini ke HTTP 403
// dengan pesan yang jelas, tanpa membocorkan detail internal.
var ErrCrossBranchAccess = errors.New("akses lintas cabang ditolak: data berada di cabang lain")

// Actor adalah identitas pelaku sebuah aksi bisnis. Nilainya HANYA boleh dibangun
// dari JWT claim di middleware, tidak pernah dari body request. Ini yang mencegah
// pemalsuan identitas pada jurnal dan audit log.
type Actor struct {
	UserID     uuid.UUID
	Username   string
	Role       StaffRole
	BranchCode string
	SessionID  uuid.UUID
	IPAddress  string
	// RequestID menghubungkan aksi bisnis ke application log permintaan asalnya.
	RequestID string
}

// DisplayName mengembalikan nama yang disimpan di kolom created_by jurnal.
func (a Actor) DisplayName() string {
	if a.Username != "" {
		return a.Username
	}
	return a.UserID.String()
}

// IsCrossBranch melaporkan apakah aktor boleh membaca dan mengubah data lintas
// cabang. Hanya peran pengawas yang boleh: SUPERADMIN dan AUDITOR mengawasi
// seluruh bank. SYSTEM juga dianggap lintas cabang karena pekerjaan batch (ARO,
// PPAP, akrual denda, tutup buku) memang mengolah seluruh cabang; tanpa itu batch
// akan terblokir oleh pemeriksaan cabang. Peran operasional (TELLER, CS, AO,
// SUPERVISOR, ADMIN) terbatas pada cabangnya sendiri.
func (a Actor) IsCrossBranch() bool {
	switch a.Role {
	case RoleSuperAdmin, RoleAuditor, RoleSystem:
		return true
	default:
		return false
	}
}

// CanAccessBranch melaporkan apakah aktor berwenang atas data pada cabang
// branchCode. Aktor lintas cabang selalu boleh. Bila cabang objek kosong, akses
// diizinkan karena baris tersebut adalah data pra-migrasi yang cabangnya belum
// diketahui; memblokirnya akan menghentikan operasional atas data lama.
func (a Actor) CanAccessBranch(branchCode string) bool {
	if a.IsCrossBranch() {
		return true
	}
	if branchCode == "" {
		return true
	}
	return a.BranchCode != "" && a.BranchCode == branchCode
}

// CanManageStaff melaporkan apakah aktor berwenang mengubah akun staf dengan peran
// target. Peran setingkat tidak boleh saling mengubah — kecuali sesama SUPERADMIN,
// karena tidak ada satu akun pemilik tunggal — sehingga perubahan wewenang selalu
// turun dari tingkat di atasnya dan tidak ada jalur naik lewat rekan sejawat.
func (a Actor) CanManageStaff(target StaffRole) bool {
	if a.Role == RoleSuperAdmin {
		return true
	}
	return a.Role.PrivilegeRank() > target.PrivilegeRank()
}
