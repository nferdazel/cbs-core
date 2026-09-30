package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// form00_06_modal.go memuat fondasi data Form 00.06 "DAFTAR MODAL DISETOR, MODAL
// SUMBANGAN, DAN DANA SETORAN MODAL - EKUITAS", Lampiran II SEOJK No. 16/SEOJK.03/2024.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Susunan PDF #page 245 (hlm. tercetak 193), sandi PDF #page 246 (hlm. 194),
//     penjelasan PDF #page 247 (hlm. 195). Empat kolom: I Jenis, II Tanggal Persetujuan
//     Otoritas, III Jenis Modal, IV Jumlah. ADA baris JUMLAH.
//   - Sandi I: 01 Dana, 02 Tanah/bangunan (modal inti), 03 Tanah/bangunan (bukan modal
//     inti). Sandi III: 01 Modal Disetor, 02 Modal Sumbangan, 03 Dana Setoran Modal -
//     Ekuitas.
//
// MENGAPA REGISTER SENDIRI, bukan menurunkan dari saldo COA: saldo bagan akun (30100/
// 13100) tidak menyimpan bentuk setoran (dana vs tanah/bangunan), tanggal persetujuan
// otoritas, atau pemisahan Modal Sumbangan / Dana Setoran Modal. Menurunkannya dari saldo
// tunggal akan mengarang dimensi yang tidak ada di data.
//
// Kolom IV Jumlah adalah isian bank (rupiah penuh); nol sah. Kolom II boleh kosong
// (modal yang belum disetujui otoritas).

// Sandi baku Form 00.06 kolom I (PDF #246).
const (
	ModalJenisDana                 = "01"
	ModalJenisTanahBangunanInti    = "02"
	ModalJenisTanahBangunanNonInti = "03"
)

// Sandi baku Form 00.06 kolom III (PDF #246).
const (
	ModalJenisModalDisetor       = "01"
	ModalJenisModalSumbangan     = "02"
	ModalJenisDanaSetoranEkuitas = "03"
)

var (
	// ErrModalInputInvalid menandai data register modal tidak sah.
	ErrModalInputInvalid = NewLocalizedError("modal_input_invalid",
		"data modal Form 00.06 tidak valid")
	// ErrModalNotFound menandai baris yang hendak dihapus tidak ada.
	ErrModalNotFound = NewLocalizedError("modal_not_found",
		"data modal tidak ditemukan")
	// ErrModalBankWide menandai permintaan register modal oleh peran yang tidak
	// berwenang atas seluruh bank.
	ErrModalBankWide = NewLocalizedError("modal_bank_wide",
		"register modal bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
)

// ModalItem adalah satu peristiwa modal (satu baris Form 00.06).
type ModalItem struct {
	ID uuid.UUID `json:"id"`
	// JenisCode kolom I: 01/02/03.
	JenisCode string `json:"jenis_code"`
	// TanggalPersetujuan kolom II; nil = belum dicatat.
	TanggalPersetujuan *time.Time `json:"tanggal_persetujuan"`
	// JenisModalCode kolom III: 01/02/03.
	JenisModalCode string `json:"jenis_modal_code"`
	// Jumlah kolom IV (rupiah penuh); nol sah.
	Jumlah    decimal.Decimal `json:"jumlah"`
	Note      string          `json:"note,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// UpdateModalItemInput adalah isi upsert satu baris. ID kosong = buat baru.
type UpdateModalItemInput struct {
	ID                 string `json:"id"`
	JenisCode          string `json:"jenis_code"`
	TanggalPersetujuan string `json:"tanggal_persetujuan"`
	JenisModalCode     string `json:"jenis_modal_code"`
	Jumlah             string `json:"jumlah"`
	Note               string `json:"note"`
}

// BuildModalItem memvalidasi masukan dan membentuk baris yang siap disimpan.
func BuildModalItem(in UpdateModalItemInput) (ModalItem, error) {
	out := ModalItem{
		JenisCode:      strings.TrimSpace(in.JenisCode),
		JenisModalCode: strings.TrimSpace(in.JenisModalCode),
		Note:           strings.TrimSpace(in.Note),
	}
	switch out.JenisCode {
	case ModalJenisDana, ModalJenisTanahBangunanInti, ModalJenisTanahBangunanNonInti:
	default:
		return out, fmt.Errorf("%w: Jenis hanya 01, 02, atau 03", ErrModalInputInvalid)
	}
	switch out.JenisModalCode {
	case ModalJenisModalDisetor, ModalJenisModalSumbangan, ModalJenisDanaSetoranEkuitas:
	default:
		return out, fmt.Errorf("%w: Jenis Modal hanya 01, 02, atau 03", ErrModalInputInvalid)
	}
	// Kolom II opsional: kosong berarti belum dicatat, bukan tanggal nol.
	if raw := strings.TrimSpace(in.TanggalPersetujuan); raw != "" {
		t, err := time.Parse("2006-01-02", raw)
		if err != nil {
			return out, fmt.Errorf("%w: Tanggal Persetujuan Otoritas harus format YYYY-MM-DD", ErrModalInputInvalid)
		}
		out.TanggalPersetujuan = &t
	}
	// Kolom IV: rupiah penuh, wajib, tidak boleh negatif. Nol sah.
	jumlah, err := parseModalJumlah(in.Jumlah)
	if err != nil {
		return out, err
	}
	out.Jumlah = jumlah
	id, err := parseModalID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	return out, nil
}

// parseModalJumlah mengurai nominal rupiah. Kosong dianggap belum diisi (galat), bukan nol.
func parseModalJumlah(raw string) (decimal.Decimal, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return decimal.Zero, fmt.Errorf("%w: Jumlah wajib diisi", ErrModalInputInvalid)
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: Jumlah bukan angka yang sah", ErrModalInputInvalid)
	}
	if d.IsNegative() {
		return decimal.Zero, fmt.Errorf("%w: Jumlah tidak boleh negatif", ErrModalInputInvalid)
	}
	return d, nil
}

// parseModalID mengurai id; kosong berarti buat baru.
func parseModalID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrModalInputInvalid)
	}
	return id, nil
}

// ModalReport adalah keluaran baca-saja register modal.
type ModalReport struct {
	Items []ModalItem `json:"items"`
}

// ModalRepository menyimpan dan membaca register modal.
type ModalRepository interface {
	ListItems(ctx context.Context) ([]ModalItem, error)
	// ListModalForOJK membaca baris register untuk Form 00.06, urutan deterministik.
	ListModalForOJK(ctx context.Context) ([]ModalItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu baris.
	UpsertItemTx(ctx context.Context, tx any, item ModalItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu baris secara fisik; found=false bila tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// ModalService melayani baca dan pengisian register modal.
type ModalService interface {
	ListModal(ctx context.Context, actor Actor) (ModalReport, error)
	ListItems(ctx context.Context) ([]ModalItem, error)
	UpsertItem(ctx context.Context, in UpdateModalItemInput, actor Actor) (*ModalItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
