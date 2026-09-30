package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// form00_16_pihak_lawan.go memuat fondasi data Form 00.16 "DAFTAR PIHAK LAWAN",
// Lampiran II SEOJK No. 16/SEOJK.03/2024.
//
// Sumber struktur form (PDF resmi, di luar repo):
//   - Susunan PDF #page 286 (hlm. tercetak 234) kolom I-IX, PDF #page 287 (hlm. 235)
//     kolom X-XX, sandi PDF #page 288-289 (hlm. 236-237), penjelasan PDF #page 290-291
//     (hlm. 238-239). Dua puluh kolom, TANPA baris JUMLAH.
//   - Isi: seluruh pihak lawan baik bank maupun bukan bank yang bertransaksi dengan BPR.
//   - Sandi II: 1 KTP, 2 Paspor, 3 KITAS/KITAP, 4 Kartu Keluarga. IV: 1 Laki-laki,
//     2 Perempuan. IX: 1 Konvensional, 2 Syariah. X: 12 Pihak Terkait, 20 Tidak Terkait.
//     XII tanpa peringkat = 9; XIII tanpa peringkat = 99.
//
// MENGAPA REGISTER SENDIRI: form memuat seluruh pihak lawan termasuk BANK dan bukan bank
// yang bukan nasabah BPR; tabel customers hanya menutup sebagian kolom untuk nasabah.
// Memaksa bank lawan masuk customers akan menuntut baris nasabah palsu.
//
// KEPUTUSAN PRIVASI: kolom III Nomor Identitas (NIK/NPWP) dan VI NPWP sengaja TIDAK
// DISIMPAN (docs/KEPUTUSAN-OJK.md §4 butir III dan §7 butir 6, pola Form 00.01/06.02).
// Laporan menulis keduanya "-" beralasan.

// Sandi baku Form 00.16 kolom II (PDF #288).
const (
	PihakLawanIdentitasKTP    = "1"
	PihakLawanIdentitasPaspor = "2"
	PihakLawanIdentitasKITAS  = "3"
	PihakLawanIdentitasKK     = "4"
)

// Sandi baku Form 00.16 kolom IV (PDF #288).
const (
	PihakLawanKelaminLakiLaki  = "1"
	PihakLawanKelaminPerempuan = "2"
)

// Sandi baku Form 00.16 kolom IX (PDF #288).
const (
	PihakLawanUsahaKonvensional = "1"
	PihakLawanUsahaSyariah      = "2"
)

// Sandi baku Form 00.16 kolom X (PDF #288).
const (
	PihakLawanHubunganTerkait      = "12"
	PihakLawanHubunganTidakTerkait = "20"
)

// Sentinel baku Form 00.16 kolom XII/XIII (PDF #291).
const (
	PihakLawanPemeringkatTanpa = "9"
	PihakLawanPeringkatTanpa   = "99"
)

var (
	// ErrPihakLawanInputInvalid menandai data register pihak lawan tidak sah.
	ErrPihakLawanInputInvalid = NewLocalizedError("pihak_lawan_input_invalid",
		"data pihak lawan Form 00.16 tidak valid")
	// ErrPihakLawanNotFound menandai baris yang hendak dihapus tidak ada.
	ErrPihakLawanNotFound = NewLocalizedError("pihak_lawan_not_found",
		"data pihak lawan tidak ditemukan")
	// ErrPihakLawanBankWide menandai permintaan register pihak lawan oleh peran yang tidak
	// berwenang atas seluruh bank.
	ErrPihakLawanBankWide = NewLocalizedError("pihak_lawan_bank_wide",
		"register pihak lawan bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
)

// PihakLawanItem adalah satu pihak lawan (satu baris Form 00.16). Kolom III Nomor
// Identitas dan VI NPWP tidak ada di sini karena sengaja tidak disimpan (keputusan privasi).
type PihakLawanItem struct {
	ID uuid.UUID `json:"id"`
	// PihakLawanID kolom I.
	PihakLawanID string `json:"pihak_lawan_id"`
	// JenisIdentitasCode kolom II.
	JenisIdentitasCode string `json:"jenis_identitas_code"`
	// JenisKelaminCode kolom IV.
	JenisKelaminCode string `json:"jenis_kelamin_code"`
	// Nama kolom V.
	Nama string `json:"nama"`
	// KewarganegaraanCode kolom VII (Lampiran 10).
	KewarganegaraanCode string `json:"kewarganegaraan_code"`
	// NegaraCode kolom VIII (Lampiran 10).
	NegaraCode string `json:"negara_code"`
	// JenisUsahaCode kolom IX.
	JenisUsahaCode string `json:"jenis_usaha_code"`
	// HubunganBankCode kolom X.
	HubunganBankCode string `json:"hubungan_bank_code"`
	// GolonganCode kolom XI (Lampiran 02).
	GolonganCode string `json:"golongan_code"`
	// LembagaPemeringkatCode kolom XII (Lampiran 08).
	LembagaPemeringkatCode string `json:"lembaga_pemeringkat_code"`
	// PeringkatCode kolom XIII (Lampiran 09).
	PeringkatCode string `json:"peringkat_code"`
	// TanggalPemeringkatan kolom XIV; nil = tanpa peringkat.
	TanggalPemeringkatan *time.Time `json:"tanggal_pemeringkatan"`
	// TanggalLahir kolom XV; nil = bukan perorangan.
	TanggalLahir *time.Time `json:"tanggal_lahir"`
	// LokasiCode kolom XVI (Lampiran 03).
	LokasiCode string `json:"lokasi_code"`
	// GrupID kolom XVII; GrupNama kolom XVIII.
	GrupID   string `json:"grup_id"`
	GrupNama string `json:"grup_nama"`
	// Telepon kolom XIX; Alamat kolom XX.
	Telepon   string    `json:"telepon"`
	Alamat    string    `json:"alamat"`
	Note      string    `json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpdatePihakLawanItemInput adalah isi upsert satu baris. ID kosong = buat baru. Tanggal
// berupa teks YYYY-MM-DD; kosong = tidak dicatat.
type UpdatePihakLawanItemInput struct {
	ID                     string `json:"id"`
	PihakLawanID           string `json:"pihak_lawan_id"`
	JenisIdentitasCode     string `json:"jenis_identitas_code"`
	JenisKelaminCode       string `json:"jenis_kelamin_code"`
	Nama                   string `json:"nama"`
	KewarganegaraanCode    string `json:"kewarganegaraan_code"`
	NegaraCode             string `json:"negara_code"`
	JenisUsahaCode         string `json:"jenis_usaha_code"`
	HubunganBankCode       string `json:"hubungan_bank_code"`
	GolonganCode           string `json:"golongan_code"`
	LembagaPemeringkatCode string `json:"lembaga_pemeringkat_code"`
	PeringkatCode          string `json:"peringkat_code"`
	TanggalPemeringkatan   string `json:"tanggal_pemeringkatan"`
	TanggalLahir           string `json:"tanggal_lahir"`
	LokasiCode             string `json:"lokasi_code"`
	GrupID                 string `json:"grup_id"`
	GrupNama               string `json:"grup_nama"`
	Telepon                string `json:"telepon"`
	Alamat                 string `json:"alamat"`
	Note                   string `json:"note"`
}

// BuildPihakLawanItem memvalidasi masukan dan membentuk baris yang siap disimpan.
func BuildPihakLawanItem(in UpdatePihakLawanItemInput) (PihakLawanItem, error) {
	out := PihakLawanItem{
		PihakLawanID:           strings.TrimSpace(in.PihakLawanID),
		JenisIdentitasCode:     strings.TrimSpace(in.JenisIdentitasCode),
		JenisKelaminCode:       strings.TrimSpace(in.JenisKelaminCode),
		Nama:                   strings.TrimSpace(in.Nama),
		KewarganegaraanCode:    strings.TrimSpace(in.KewarganegaraanCode),
		NegaraCode:             strings.TrimSpace(in.NegaraCode),
		JenisUsahaCode:         strings.TrimSpace(in.JenisUsahaCode),
		HubunganBankCode:       strings.TrimSpace(in.HubunganBankCode),
		GolonganCode:           strings.TrimSpace(in.GolonganCode),
		LembagaPemeringkatCode: strings.TrimSpace(in.LembagaPemeringkatCode),
		PeringkatCode:          strings.TrimSpace(in.PeringkatCode),
		LokasiCode:             strings.TrimSpace(in.LokasiCode),
		GrupID:                 strings.TrimSpace(in.GrupID),
		GrupNama:               strings.TrimSpace(in.GrupNama),
		Telepon:                strings.TrimSpace(in.Telepon),
		Alamat:                 strings.TrimSpace(in.Alamat),
		Note:                   strings.TrimSpace(in.Note),
	}
	if out.PihakLawanID == "" {
		return out, fmt.Errorf("%w: ID Pihak Lawan wajib diisi", ErrPihakLawanInputInvalid)
	}
	if out.Nama == "" {
		return out, fmt.Errorf("%w: Nama Lengkap/Nama Badan Usaha wajib diisi", ErrPihakLawanInputInvalid)
	}
	if len(out.Nama) > 255 {
		return out, fmt.Errorf("%w: Nama maksimal 255 karakter", ErrPihakLawanInputInvalid)
	}
	if err := validateSandiPihakLawan(out); err != nil {
		return out, err
	}
	tanggalPemeringkatan, err := parseTanggalPihakLawan(in.TanggalPemeringkatan, "Tanggal Pemeringkatan")
	if err != nil {
		return out, err
	}
	out.TanggalPemeringkatan = tanggalPemeringkatan
	tanggalLahir, err := parseTanggalPihakLawan(in.TanggalLahir, "Tanggal Lahir")
	if err != nil {
		return out, err
	}
	out.TanggalLahir = tanggalLahir
	id, err := parsePihakLawanID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	return out, nil
}

// validateSandiPihakLawan menegakkan sandi baku yang boleh kosong (kolom kondisional).
func validateSandiPihakLawan(out PihakLawanItem) error {
	switch out.JenisIdentitasCode {
	case "", PihakLawanIdentitasKTP, PihakLawanIdentitasPaspor,
		PihakLawanIdentitasKITAS, PihakLawanIdentitasKK:
	default:
		return fmt.Errorf("%w: Jenis Identitas hanya 1, 2, 3, atau 4", ErrPihakLawanInputInvalid)
	}
	switch out.JenisKelaminCode {
	case "", PihakLawanKelaminLakiLaki, PihakLawanKelaminPerempuan:
	default:
		return fmt.Errorf("%w: Jenis Kelamin hanya 1 atau 2", ErrPihakLawanInputInvalid)
	}
	switch out.JenisUsahaCode {
	case "", PihakLawanUsahaKonvensional, PihakLawanUsahaSyariah:
	default:
		return fmt.Errorf("%w: Jenis Kegiatan Usaha hanya 1 atau 2", ErrPihakLawanInputInvalid)
	}
	switch out.HubunganBankCode {
	case "", PihakLawanHubunganTerkait, PihakLawanHubunganTidakTerkait:
	default:
		return fmt.Errorf("%w: Hubungan dengan Bank hanya 12 atau 20", ErrPihakLawanInputInvalid)
	}
	return nil
}

// parseTanggalPihakLawan mengurai tanggal opsional; kosong berarti tidak dicatat.
func parseTanggalPihakLawan(raw, name string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrPihakLawanInputInvalid, name)
	}
	return &t, nil
}

// parsePihakLawanID mengurai id; kosong berarti buat baru.
func parsePihakLawanID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrPihakLawanInputInvalid)
	}
	return id, nil
}

// PihakLawanReport adalah keluaran baca-saja register pihak lawan.
type PihakLawanReport struct {
	Items []PihakLawanItem `json:"items"`
}

// PihakLawanRepository menyimpan dan membaca register pihak lawan.
type PihakLawanRepository interface {
	ListItems(ctx context.Context) ([]PihakLawanItem, error)
	// ListPihakLawanForOJK membaca baris register untuk Form 00.16, urutan deterministik.
	ListPihakLawanForOJK(ctx context.Context) ([]PihakLawanItem, error)
	// UpsertItemTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu baris.
	UpsertItemTx(ctx context.Context, tx any, item PihakLawanItem, actorID uuid.UUID) error
	// DeleteItemTx menghapus satu baris secara fisik; found=false bila tidak ada.
	DeleteItemTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
}

// PihakLawanService melayani baca dan pengisian register pihak lawan.
type PihakLawanService interface {
	ListPihakLawan(ctx context.Context, actor Actor) (PihakLawanReport, error)
	ListItems(ctx context.Context) ([]PihakLawanItem, error)
	UpsertItem(ctx context.Context, in UpdatePihakLawanItemInput, actor Actor) (*PihakLawanItem, error)
	DeleteItem(ctx context.Context, id uuid.UUID, actor Actor) error
}
