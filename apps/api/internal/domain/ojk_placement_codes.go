package domain

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ojk_placement_codes.go membuka pengisian sandi OJK penempatan pada bank lain
// (Form 05.00) lewat API untuk kolom yang sebelumnya hanya dapat diisi lewat SQL/seed
// (migrasi 000106 dan 000108). Berbeda dari jalur tulis CKPN penempatan, berkas ini
// TIDAK menghitung apa pun: hanya atribut laporan yang disimpan.
//
// Semantik masukan seragam dengan sandi OJK kredit: absen/null = jangan ubah; string
// kosong = kosongkan (NULL); selain itu divalidasi lalu disimpan. Sandi referensi
// (kabupaten) diperiksa service terhadap tabel ojk_*.

// Nama kolom sandi OJK penempatan, dipakai pada pesan galat agar pengguna tahu sandi
// mana yang bermasalah.
const (
	OJKAlasanDiblokirField            = "ojk_alasan_diblokir_code"
	OJKCounterpartyCIFField           = "counterparty_cif"
	OJKBlockedAmountField             = "blocked_amount"
	OJKAccruedInterestReceivableField = "accrued_interest_receivable"
	OJKAccruedInterestPendingField    = "accrued_interest_pending"
)

// UpdateOJKPlacementCodesInput adalah isi PUT sandi OJK per penempatan. Semua bidang
// pointer-string nullable dengan semantik "absen = jangan ubah, kosong = NULL".
type UpdateOJKPlacementCodesInput struct {
	OJKKabupatenCode          *string `json:"ojk_kabupaten_code,omitempty"`
	OJKHubunganBankCode       *string `json:"ojk_hubungan_bank_code,omitempty"`
	OJKAlasanDiblokirCode     *string `json:"ojk_alasan_diblokir_code,omitempty"`
	CounterpartyCIF           *string `json:"counterparty_cif,omitempty"`
	OJKKlasifikasiAsetCode    *string `json:"ojk_klasifikasi_aset_code,omitempty"`
	BlockedAmount             *string `json:"blocked_amount,omitempty"`
	AccruedInterestReceivable *string `json:"accrued_interest_receivable,omitempty"`
	AccruedInterestPending    *string `json:"accrued_interest_pending,omitempty"`
}

// IsEmpty melaporkan apakah tidak ada satu bidang pun yang dikirim.
func (in UpdateOJKPlacementCodesInput) IsEmpty() bool {
	return in.OJKKabupatenCode == nil &&
		in.OJKHubunganBankCode == nil &&
		in.OJKAlasanDiblokirCode == nil &&
		in.CounterpartyCIF == nil &&
		in.OJKKlasifikasiAsetCode == nil &&
		in.BlockedAmount == nil &&
		in.AccruedInterestReceivable == nil &&
		in.AccruedInterestPending == nil
}

// Validate memeriksa sandi inline dan nominal yang dikirim. Sandi referensi
// (kabupaten) tidak diperiksa di sini karena harus dibandingkan dengan tabel referensi.
func (in UpdateOJKPlacementCodesInput) Validate() error {
	if in.OJKHubunganBankCode != nil {
		if err := validateOJKInline(*in.OJKHubunganBankCode, ojkHubunganBankPenempatanAllowed, OJKHubunganBankField); err != nil {
			return err
		}
	}
	amounts := []struct {
		field string
		value *string
	}{
		{OJKBlockedAmountField, in.BlockedAmount},
		{OJKAccruedInterestReceivableField, in.AccruedInterestReceivable},
		{OJKAccruedInterestPendingField, in.AccruedInterestPending},
	}
	for _, item := range amounts {
		if item.value == nil {
			continue
		}
		if _, err := ParseOJKAmount(item.field, *item.value); err != nil {
			return err
		}
	}
	return nil
}

// OJKPlacementCodesView adalah proyeksi baca sandi OJK satu penempatan untuk UI.
// Sandi/teks nullable: kosong ditampilkan sebagai null, bukan "". Nominal juga null
// selama belum diisi (dibedakan dari nol).
type OJKPlacementCodesView struct {
	PlacementID               uuid.UUID        `json:"placement_id"`
	Label                     string           `json:"label"`
	OJKKabupatenCode          *string          `json:"ojk_kabupaten_code"`
	OJKHubunganBankCode       *string          `json:"ojk_hubungan_bank_code"`
	OJKAlasanDiblokirCode     *string          `json:"ojk_alasan_diblokir_code"`
	CounterpartyCIF           *string          `json:"counterparty_cif"`
	OJKKlasifikasiAsetCode    *string          `json:"ojk_klasifikasi_aset_code"`
	BlockedAmount             *decimal.Decimal `json:"blocked_amount"`
	AccruedInterestReceivable *decimal.Decimal `json:"accrued_interest_receivable"`
	AccruedInterestPending    *decimal.Decimal `json:"accrued_interest_pending"`
}

// NewOJKPlacementCodesView membentuk proyeksi baca dari satu penempatan tersimpan.
func NewOJKPlacementCodesView(p LPSPlacement) OJKPlacementCodesView {
	return OJKPlacementCodesView{
		PlacementID:               p.ID,
		Label:                     LPSPlacementLabel(p),
		OJKKabupatenCode:          nullableOJKString(p.OJKKabupatenCode),
		OJKHubunganBankCode:       nullableOJKString(p.OJKHubunganBankCode),
		OJKAlasanDiblokirCode:     nullableOJKString(p.OJKAlasanDiblokirCode),
		CounterpartyCIF:           nullableOJKString(p.CounterpartyCIF),
		OJKKlasifikasiAsetCode:    nullableOJKString(p.OJKKlasifikasiAsetCode),
		BlockedAmount:             p.BlockedAmount,
		AccruedInterestReceivable: p.AccruedInterestReceivable,
		AccruedInterestPending:    p.AccruedInterestPending,
	}
}

// OJKPlacementOption adalah pilihan penempatan untuk pemilih UI: id + label terbaca.
type OJKPlacementOption struct {
	ID    uuid.UUID `json:"id"`
	Label string    `json:"label"`
}

// LPSPlacementLabel menyusun deskripsi terbaca satu penempatan: nama bank lawan dan
// tanggal mulai bila ada. Bukan sandi OJK, hanya teks bantu UI.
func LPSPlacementLabel(p LPSPlacement) string {
	label := strings.TrimSpace(p.CounterpartyBank)
	if p.StartDate != nil {
		tanggal := p.StartDate.Format("2006-01-02")
		if label == "" {
			label = tanggal
		} else {
			label += " · " + tanggal
		}
	}
	if label == "" {
		label = p.ID.String()
	}
	return label
}

// nullableOJKString mengubah string kosong menjadi nil agar UI membedakan "belum
// diisi" dari string kosong.
func nullableOJKString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	v := strings.TrimSpace(s)
	return &v
}

// OJKPlacementCodesStore membaca dan menulis sandi OJK satu penempatan. Implementasinya
// adalah LPSPlacementRepository postgres yang sama; interface sempit ini menjaga
// layanan tidak bergantung pada seluruh domain.LPSPlacementRepository.
type OJKPlacementCodesStore interface {
	GetPlacementByID(ctx context.Context, id uuid.UUID) (*LPSPlacement, error)
	// UpdateOJKPlacementCodesTx menyimpan hanya sandi yang dikirim (pointer non-nil) di
	// dalam transaksi pemanggil. Pointer nil tidak menyentuh kolomnya.
	UpdateOJKPlacementCodesTx(ctx context.Context, tx any, id uuid.UUID, input UpdateOJKPlacementCodesInput) error
}

// OJKPlacementCodesService mengelola sandi referensi/inline OJK per penempatan:
// memvalidasi, menyimpan, dan mengaudit dalam satu transaksi.
type OJKPlacementCodesService interface {
	// Update mengubah sandi OJK satu penempatan dan mengembalikan penempatan yang
	// sudah diperbarui. Penempatan di luar cakupan unit aktor ditolak
	// ErrCrossBranchAccess.
	Update(ctx context.Context, placementID uuid.UUID, input UpdateOJKPlacementCodesInput, actor Actor) (*LPSPlacement, error)
	// Get mengembalikan nilai penempatan yang tersimpan sebagai proyeksi baca.
	Get(ctx context.Context, placementID uuid.UUID, actor Actor) (OJKPlacementCodesView, error)
}
