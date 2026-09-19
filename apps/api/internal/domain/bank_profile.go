package domain

import (
	"context"
	"strings"
	"time"
)

// BankProfile adalah identitas bank yang dipakai dokumen cetak. Data ini bersifat
// konfigurasi: operator mengisinya di database, bukan literal di kode. Nilai nama
// kosong berarti profil belum dikonfigurasi dan dokumen wajib menandainya.
type BankProfile struct {
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	City      string    `json:"city"`
	Phone     string    `json:"phone"`
	NPWP      string    `json:"npwp"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Configured melaporkan apakah identitas bank minimum (nama) sudah diisi.
func (p *BankProfile) Configured() bool {
	return p != nil && strings.TrimSpace(p.Name) != ""
}

type BankProfileRepository interface {
	// Get mengembalikan profil bank. (nil, nil) berarti baris profil belum ada,
	// sehingga pemanggil dapat membedakan "belum dikonfigurasi" dari error database.
	Get(ctx context.Context) (*BankProfile, error)
}
