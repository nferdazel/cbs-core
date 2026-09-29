package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// form00_05_pihak_terkait.go memuat fondasi data Form 00.05 "DATA PIHAK TERKAIT LAINNYA",
// Lampiran II SEOJK No. 16/SEOJK.03/2024.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Form 00.05 "DATA PIHAK TERKAIT LAINNYA": PDF #page 96 (hlm. tercetak 44) susunan,
//     sandi PDF #page 97 (hlm. 45), penjelasan PDF #page 98 (hlm. 46). Lima kolom:
//     I Nama Pihak Terkait, II No. Identitas, III Alamat Pihak Terkait,
//     IV Jenis Pihak Terkait, V Hubungan Pihak Terkait. TIDAK ada baris JUMLAH.
//   - Sandi IV: 01 Perorangan, 02 Perusahaan/Badan, 03 Pemerintah Daerah/Pusat.
//     Sandi V: 01-06 (definisi rinci di PDF #98).
//
// MENGAPA REGISTER SENDIRI, bukan bmpk_related_parties: tabel itu menandai NASABAH sebagai
// pihak terkait BMPK (kunci customer_id NOT NULL) dan dipakai bersama bmpk_limits untuk
// batas paparan. Form 00.05 memuat pihak terkait "selain pemegang saham/direksi/komisaris/
// pejabat eksekutif" yang bisa BUKAN nasabah (mis. pemerintah daerah, perusahaan
// pengendali). Memaksakannya ke bmpk_related_parties menuntut baris customers palsu.
//
// Kolom II No. Identitas (NIK/NPWP) sengaja tidak disimpan (keputusan privasi,
// docs/KEPUTUSAN-OJK.md §4 dan §7 butir 6); laporan menulis "-" beralasan.

// Sandi baku Form 00.05 kolom IV (PDF #97).
const (
	PihakTerkaitJenisPerorangan = "01"
	PihakTerkaitJenisBadan      = "02"
	PihakTerkaitJenisPemerintah = "03"
)

// Sandi baku Form 00.05 kolom V (PDF #97). Definisi rinci tiap sandi ada di PDF #98 dan
// tidak disalin ke kode.
const (
	PihakTerkaitHubunganPengendaliKeluarga  = "01"
	PihakTerkaitHubunganPerusahaanBukanBank = "02"
	PihakTerkaitHubunganBPRLainDimiliki     = "03"
	PihakTerkaitHubunganBPRRangkapKomisaris = "04"
	PihakTerkaitHubunganPerusahaanRangkap   = "05"
	PihakTerkaitHubunganPeminjamDijamin     = "06"
)

var (
	// ErrPihakTerkaitInputInvalid menandai data register pihak terkait tidak sah.
	ErrPihakTerkaitInputInvalid = NewLocalizedError("pihak_terkait_input_invalid",
		"data pihak terkait Form 00.05 tidak valid")
	// ErrPihakTerkaitNotFound menandai baris yang hendak dihapus tidak ada.
	ErrPihakTerkaitNotFound = NewLocalizedError("pihak_terkait_not_found",
		"data pihak terkait tidak ditemukan")
	// ErrPihakTerkaitBankWide menandai permintaan register pihak terkait oleh peran yang
	// tidak berwenang atas seluruh bank.
	ErrPihakTerkaitBankWide = NewLocalizedError("pihak_terkait_bank_wide",
		"register pihak terkait bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
)

// PihakTerkaitItem adalah satu pihak terkait (satu baris Form 00.05). Kolom II No.
// Identitas tidak ada di sini karena sengaja tidak disimpan (keputusan privasi).
type PihakTerkaitItem struct {
	ID uuid.UUID `json:"id"`
	// Nama kolom I.
	Nama string `json:"nama"`
	// Alamat kolom III.
	Alamat string `json:"alamat"`
	// JenisCode kolom IV: 01/02/03.
	JenisCode string `json:"jenis_code"`
	// HubunganCode kolom V: 01-06.
	HubunganCode string    `json:"hubungan_code"`
	Note         string    `json:"note,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UpdatePihakTerkaitItemInput adalah isi upsert satu baris. ID kosong = buat baru.
type UpdatePihakTerkaitItemInput struct {
	ID           string `json:"id"`
	Nama         string `json:"nama"`
	Alamat       string `json:"alamat"`
	JenisCode    string `json:"jenis_code"`
	HubunganCode string `json:"hubungan_code"`
	Note         string `json:"note"`
}

// BuildPihakTerkaitItem memvalidasi masukan dan membentuk baris yang siap disimpan.
func BuildPihakTerkaitItem(in UpdatePihakTerkaitItemInput) (PihakTerkaitItem, error) {
	out := PihakTerkaitItem{
		Nama:         strings.TrimSpace(in.Nama),
		Alamat:       strings.TrimSpace(in.Alamat),
		JenisCode:    strings.TrimSpace(in.JenisCode),
		HubunganCode: strings.TrimSpace(in.HubunganCode),
		Note:         strings.TrimSpace(in.Note),
	}
	if out.Nama == "" {
		return out, fmt.Errorf("%w: Nama Pihak Terkait wajib diisi", ErrPihakTerkaitInputInvalid)
	}
	if len(out.Nama) > 255 {
		return out, fmt.Errorf("%w: Nama Pihak Terkait maksimal 255 karakter", ErrPihakTerkaitInputInvalid)
	}
	switch out.JenisCode {
	case PihakTerkaitJenisPerorangan, PihakTerkaitJenisBadan, PihakTerkaitJenisPemerintah:
	default:
		return out, fmt.Errorf("%w: Jenis Pihak Terkait hanya 01, 02, atau 03", ErrPihakTerkaitInputInvalid)
	}
	switch out.HubunganCode {
	case PihakTerkaitHubunganPengendaliKeluarga, PihakTerkaitHubunganPerusahaanBukanBank,
		PihakTerkaitHubunganBPRLainDimiliki, PihakTerkaitHubunganBPRRangkapKomisaris,
		PihakTerkaitHubunganPerusahaanRangkap, PihakTerkaitHubunganPeminjamDijamin:
	default:
		return out, fmt.Errorf("%w: Hubungan Pihak Terkait hanya 01 sampai 06", ErrPihakTerkaitInputInvalid)
	}
	id, err := parsePihakTerkaitID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	return out, nil
}

// parsePihakTerkaitID mengurai id; kosong berarti buat baru.
func parsePihakTerkaitID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrPihakTerkaitInputInvalid)
	}
	return id, nil
}

// PihakTerkaitReport adalah keluaran baca-saja register pihak terkait.
type PihakTerkaitReport struct {
	Items []PihakTerkaitItem `json:"items"`
}

// PihakTerkaitRepository menyimpan dan membaca register pihak terkait lainnya.
type PihakTerkaitRepository interface {
	ListItems(ctx context.Context) ([]PihakTerkaitItem, error)
	// ListPihakTerkaitForOJK membaca baris register untuk Form 00.05, urutan deterministik.
	ListPihakTerkaitForOJK(ctx context.Context) ([]PihakTerkaitItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu baris.
	UpsertItemTx(ctx context.Context, tx any, item PihakTerkaitItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu baris secara fisik; found=false bila tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// PihakTerkaitService melayani baca dan pengisian register pihak terkait.
type PihakTerkaitService interface {
	ListPihakTerkait(ctx context.Context, actor Actor) (PihakTerkaitReport, error)
	ListItems(ctx context.Context) ([]PihakTerkaitItem, error)
	UpsertItem(ctx context.Context, in UpdatePihakTerkaitItemInput, actor Actor) (*PihakTerkaitItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
