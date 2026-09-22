package domain

import (
	"errors"

	"github.com/google/uuid"
)

// ErrCrossBranchAccess menandai operasi tulis yang ditolak karena objeknya berada
// di cabang lain dari cabang aktor. Handlernya memetakan error ini ke HTTP 403
// dengan pesan yang jelas, tanpa membocorkan detail internal.
var ErrCrossBranchAccess = errors.New("akses lintas cabang ditolak: data berada di cabang lain")

// ErrCrossBookAccess menandai operasi baca yang ditolak karena objeknya berada di
// buku COA (konvensional/syariah) lain dari buku aktor. Ia sejajar dengan
// ErrCrossBranchAccess sebagai batas keamanan sisi server: filter di klien hanya
// tampilan, bukan penentu akses.
var ErrCrossBookAccess = errors.New("akses lintas buku ditolak: data berada di buku lain")

// Actor adalah identitas pelaku sebuah aksi bisnis. Nilainya HANYA boleh dibangun
// dari JWT claim di middleware, tidak pernah dari body request. Ini yang mencegah
// pemalsuan identitas pada jurnal dan audit log.
type Actor struct {
	UserID     uuid.UUID
	Username   string
	Role       StaffRole
	BranchCode string
	// Book adalah buku COA (konvensional/syariah) pengguna, dibaca dari kolom
	// staff_users.book. Kosong berarti belum ditentukan; aktor dengan buku kosong
	// tidak dibatasi agar data lama tidak hilang dari operasional (lihat
	// CanAccessBook). Nilainya dipakai lapisan repository sebagai filter.
	Book      COABook
	SessionID uuid.UUID
	IPAddress string
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

// RequiresRegisteredBranch melaporkan apakah kode cabang aktor wajib terdaftar di
// tabel branches saat aktor menulis data baru. Aktor lintas cabang (SUPERADMIN,
// AUDITOR, SYSTEM) bertindak atas nama kantor pusat dan tidak terikat satu cabang
// operasional: kode bawaan 'HO' milik akun kantor pusat bukan baris di tabel
// branches, sehingga cabangnya tidak wajib terdaftar. Peran bercabang biasa
// (ADMIN, SUPERVISOR, TELLER, CS, AO) tetap wajib punya cabang sah agar nasabah
// dan rekening tidak tercatat di cabang yang tidak ditemukan.
func (a Actor) RequiresRegisteredBranch() bool {
	return !a.IsCrossBranch()
}

// IsCrossBook melaporkan apakah aktor boleh membaca dan mengubah data lintas buku
// COA. Meniru IsCrossBranch: hanya peran pengawas yang melayani kedua buku —
// SUPERADMIN dan AUDITOR mengawasi seluruh bank, dan SYSTEM karena pekerjaan batch
// (akrual, PPAP, CKPN, tutup buku) memang mengolah kredit/rekening kedua buku;
// tanpa penanda ini batch akan terblokir oleh pemeriksaan buku. Peran operasional
// (ADMIN, SUPERVISOR, TELLER, CS, AO) terbatas pada buku yang ditetapkan di kolom
// staff_users.book.
func (a Actor) IsCrossBook() bool {
	switch a.Role {
	case RoleSuperAdmin, RoleAuditor, RoleSystem:
		return true
	default:
		return false
	}
}

// CanAccessBook melaporkan apakah aktor berwenang atas data ber-buku book. Aktor
// lintas buku selalu boleh. Buku aktor yang kosong (belum ditentukan) dan buku
// objek yang kosong (mis. kredit lama tanpa produk) diizinkan karena memblokirnya
// akan menghentikan operasional atas data lama; ini mengikuti semantik
// CanAccessBranch.
func (a Actor) CanAccessBook(book COABook) bool {
	if a.IsCrossBook() {
		return true
	}
	if a.Book == "" || book == "" {
		return true
	}
	return a.Book == book
}

// CanAccessJournal melaporkan apakah aktor berwenang membaca jurnal ber-buku book.
// mixed menandai jurnal yang baris-baranya berada di lebih dari satu buku (mis. data
// lama yang kasnya belum terpisah): aktor satu buku TIDAK boleh membacanya karena satu
// barisnya membocorkan buku lain, sedangkan aktor lintas buku tetap boleh. Aktor yang
// bukunya belum ditentukan (kosong) tidak diblokir, mengikuti semantik CanAccessBook.
func (a Actor) CanAccessJournal(book COABook, mixed bool) bool {
	if a.IsCrossBook() || a.Book == "" {
		return true
	}
	if mixed {
		return false
	}
	return a.Book == book || book == ""
}

// ConstrainedBook membatasi buku yang diminta klien ke buku aktor. Dipakai laporan
// yang menerima query param book: aktor lintas buku atau yang bukunya belum
// ditentukan boleh memilih buku; aktor satu buku dipaksa ke bukunya sendiri agar
// pengguna konvensional tidak dapat membaca posisi syariah (dan sebaliknya).
func (a Actor) ConstrainedBook(requested string) string {
	if a.IsCrossBook() || a.Book == "" {
		return requested
	}
	return string(a.Book)
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
