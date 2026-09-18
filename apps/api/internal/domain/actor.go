package domain

import (
	"github.com/google/uuid"
)

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
}

// DisplayName mengembalikan nama yang disimpan di kolom created_by jurnal.
func (a Actor) DisplayName() string {
	if a.Username != "" {
		return a.Username
	}
	return a.UserID.String()
}
