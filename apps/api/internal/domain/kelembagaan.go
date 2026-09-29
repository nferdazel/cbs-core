package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// kelembagaan.go memuat fondasi data LAPORAN_KELEMBAGAAN (SEOJK No. 16/SEOJK.03/2024):
// jaringan kantor, anggota direksi/dewan komisaris, dan pejabat eksekutif. Ini BUKAN
// perhitungan akuntansi: hanya penyimpanan data bank yang membuat laporan dapat
// dibangun.
//
// Prinsip yang dipegang berkas ini:
//   - TIDAK mengarang daftar/angka. Tipe kantor, jabatan, dan sandi jabatan OJK
//     disimpan sebagai nilai yang BANK isi. Kategori orang (DIREKSI/KOMISARIS/
//     PEJABAT_EKSEKUTIF) dan status baku karena maknanya baku, bukan sandi regulasi.
//   - NIK SENGAJA tidak dimodelkan: keputusan panel menetapkan NIK tidak dipaparkan
//     sebagai keluaran laporan (docs/KEPUTUSAN-OJK.md §4). Data pribadi lain di luar
//     kolom yang benar-benar dilaporkan juga tidak disimpan.
//
// Sumber struktur kolom (PDF resmi 528 hlm., di luar repo):
//   - Form 00.04 "Data Kantor BPR"     : PDF #page 88-95, hlm. 36-43.
//   - Form 00.02 "Data Direksi/Komisaris": PDF #page 75-82, hlm. 23-30.
//   - Form 00.03 "Data Pejabat Eksekutif": PDF #page 83-87, hlm. 31-35.

// Kategori orang pada bank_management. Nilainya baku karena menentukan bagian laporan
// (Form 00.02 vs 00.03), bukan sandi regulasi yang dikarang.
const (
	KelembagaanKategoriDireksi          = "DIREKSI"
	KelembagaanKategoriKomisaris        = "KOMISARIS"
	KelembagaanKategoriPejabatEksekutif = "PEJABAT_EKSEKUTIF"
)

// Status baku kantor dan orang.
const (
	KelembagaanKantorAktif = "AKTIF"
	KelembagaanKantorTutup = "TUTUP"

	KelembagaanOrangAktif    = "AKTIF"
	KelembagaanOrangNonaktif = "NONAKTIF"
)

var (
	// ErrKelembagaanBankWide menandai permintaan laporan kelembagaan oleh aktor yang
	// tidak berwenang atas seluruh bank. Laporan ini bank-wide.
	ErrKelembagaanBankWide = NewLocalizedError("kelembagaan_bank_wide",
		"laporan kelembagaan bersifat bank-wide dan hanya dapat dibaca peran lintas cabang")
	// ErrKelembagaanInputInvalid menandai data kelembagaan yang tidak sah (kategori/
	// status/tipe kosong, tanggal salah, dst.).
	ErrKelembagaanInputInvalid = NewLocalizedError("kelembagaan_input_invalid",
		"data kelembagaan tidak valid")
	// ErrKelembagaanNotFound menandai baris yang hendak dihapus tidak ada.
	ErrKelembagaanNotFound = NewLocalizedError("kelembagaan_not_found",
		"data kelembagaan tidak ditemukan")
)

// BankOffice adalah satu kantor pada jaringan kantor bank (sumber Form 00.04).
// Hanya kolom yang benar-benar dipetakan yang ada; kolom form lain (koordinat, jumlah
// pegawai per jenjang, EDC/ATM, dsb.) belum punya sumber dan ditandai belum tersedia
// oleh perakit laporan, bukan diisi nol.
type BankOffice struct {
	ID         uuid.UUID `json:"id"`
	OfficeType string    `json:"office_type"`
	// Code adalah sandi kantor 3 angka yang bank pakai; kosong = belum diisi.
	Code             string     `json:"code,omitempty"`
	Name             string     `json:"name"`
	Address          string     `json:"address,omitempty"`
	City             string     `json:"city,omitempty"`
	OJKKabupatenCode string     `json:"ojk_kabupaten_code,omitempty"`
	OpenedAt         *time.Time `json:"opened_at,omitempty"`
	ClosedAt         *time.Time `json:"closed_at,omitempty"`
	Status           string     `json:"status"`
	Note             string     `json:"note,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	// Kolom Form 00.11 (migrasi 000126). OfficeType tetap teks bebas kebijakan bank untuk
	// Form 00.04; OJKOfficeKindCode adalah sandi baku Form 00.11 kolom I yang terpisah.
	OJKOfficeKindCode  string     `json:"ojk_office_kind_code,omitempty"`
	ParentOfficeCode   string     `json:"parent_office_code,omitempty"`
	PreviousOfficeCode string     `json:"previous_office_code,omitempty"`
	Coordinates        string     `json:"coordinates,omitempty"`
	HeadName           string     `json:"head_name,omitempty"`
	PhoneNumber        string     `json:"phone_number,omitempty"`
	OJKChangeCode      string     `json:"ojk_change_code,omitempty"`
	ImplementationDate *time.Time `json:"implementation_date,omitempty"`
	ControlOfficeCode  string     `json:"control_office_code,omitempty"`
	OJKApprovalDate    *time.Time `json:"ojk_approval_date,omitempty"`
}

// BankManagement adalah satu orang pada direksi/dewan komisaris/pejabat eksekutif
// (sumber Form 00.02 dan 00.03). NIK tidak disimpan (keputusan privasi).
type BankManagement struct {
	ID       uuid.UUID `json:"id"`
	Category string    `json:"category"`
	Name     string    `json:"name"`
	Position string    `json:"position,omitempty"`
	// OJKPositionCode adalah sandi jabatan resmi OJK yang bank isi; kosong = "-".
	OJKPositionCode string     `json:"ojk_position_code,omitempty"`
	LicenseNumber   string     `json:"license_number,omitempty"`
	LicenseDate     *time.Time `json:"license_date,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	Status          string     `json:"status"`
	Note            string     `json:"note,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// KelembagaanReport adalah keluaran baca-saja LAPORAN_KELEMBAGAAN untuk satu posisi.
type KelembagaanReport struct {
	AsOf       time.Time        `json:"as_of"`
	Offices    []BankOffice     `json:"offices"`
	Management []BankManagement `json:"management"`
	// Warnings mencatat batas laporan yang perlu diketahui pembaca (mis. bagian form
	// yang belum punya sumber), bukan menutupi kekurangan data.
	Warnings []string `json:"warnings,omitempty"`
}

// UpdateBankOfficeInput adalah isi upsert satu kantor. ID kosong = buat baru (id
// dibangkitkan server); ID terisi = perbarui baris itu.
type UpdateBankOfficeInput struct {
	ID               string `json:"id"`
	OfficeType       string `json:"office_type"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	Address          string `json:"address"`
	City             string `json:"city"`
	OJKKabupatenCode string `json:"ojk_kabupaten_code"`
	// OpenedAt/ClosedAt memakai format YYYY-MM-DD; kosong = belum diisi.
	OpenedAt string `json:"opened_at"`
	ClosedAt string `json:"closed_at"`
	// Status kosong berarti AKTIF.
	Status string `json:"status"`
	Note   string `json:"note"`
}

// UpdateBankManagementInput adalah isi upsert satu orang. ID kosong = buat baru.
type UpdateBankManagementInput struct {
	ID              string `json:"id"`
	Category        string `json:"category"`
	Name            string `json:"name"`
	Position        string `json:"position"`
	OJKPositionCode string `json:"ojk_position_code"`
	LicenseNumber   string `json:"license_number"`
	// Tanggal memakai format YYYY-MM-DD; kosong = belum diisi.
	LicenseDate string `json:"license_date"`
	StartedAt   string `json:"started_at"`
	EndedAt     string `json:"ended_at"`
	// Status kosong berarti AKTIF.
	Status string `json:"status"`
	Note   string `json:"note"`
}

// BuildBankOffice memvalidasi masukan dan membentuk baris yang siap disimpan. ID kosong
// dibangkitkan server; tanggal diurai dengan format YYYY-MM-DD. Waktu tidak disetel di
// sini (diisi repositori saat menyimpan).
func BuildBankOffice(in UpdateBankOfficeInput) (BankOffice, error) {
	out := BankOffice{
		OfficeType:       strings.TrimSpace(in.OfficeType),
		Code:             strings.TrimSpace(in.Code),
		Name:             strings.TrimSpace(in.Name),
		Address:          strings.TrimSpace(in.Address),
		City:             strings.TrimSpace(in.City),
		OJKKabupatenCode: strings.TrimSpace(in.OJKKabupatenCode),
		Status:           strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:             strings.TrimSpace(in.Note),
	}
	if out.OfficeType == "" {
		return out, fmt.Errorf("%w: office_type wajib diisi", ErrKelembagaanInputInvalid)
	}
	if len(out.OfficeType) > 32 {
		return out, fmt.Errorf("%w: office_type maksimal 32 karakter", ErrKelembagaanInputInvalid)
	}
	if out.Name == "" {
		return out, fmt.Errorf("%w: nama kantor wajib diisi", ErrKelembagaanInputInvalid)
	}
	if len(out.Name) > 255 {
		return out, fmt.Errorf("%w: nama kantor maksimal 255 karakter", ErrKelembagaanInputInvalid)
	}
	if out.Status == "" {
		out.Status = KelembagaanKantorAktif
	}
	if out.Status != KelembagaanKantorAktif && out.Status != KelembagaanKantorTutup {
		return out, fmt.Errorf("%w: status kantor hanya AKTIF atau TUTUP", ErrKelembagaanInputInvalid)
	}
	id, err := parseKelembagaanID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	if out.OpenedAt, err = parseKelembagaanDate(in.OpenedAt, "tanggal buka"); err != nil {
		return out, err
	}
	if out.ClosedAt, err = parseKelembagaanDate(in.ClosedAt, "tanggal tutup"); err != nil {
		return out, err
	}
	return out, nil
}

// BuildBankManagement memvalidasi masukan dan membentuk baris yang siap disimpan.
func BuildBankManagement(in UpdateBankManagementInput) (BankManagement, error) {
	out := BankManagement{
		Category:        strings.ToUpper(strings.TrimSpace(in.Category)),
		Name:            strings.TrimSpace(in.Name),
		Position:        strings.TrimSpace(in.Position),
		OJKPositionCode: strings.TrimSpace(in.OJKPositionCode),
		LicenseNumber:   strings.TrimSpace(in.LicenseNumber),
		Status:          strings.ToUpper(strings.TrimSpace(in.Status)),
		Note:            strings.TrimSpace(in.Note),
	}
	switch out.Category {
	case KelembagaanKategoriDireksi, KelembagaanKategoriKomisaris, KelembagaanKategoriPejabatEksekutif:
	default:
		return out, fmt.Errorf("%w: category hanya DIREKSI, KOMISARIS, atau PEJABAT_EKSEKUTIF",
			ErrKelembagaanInputInvalid)
	}
	if out.Name == "" {
		return out, fmt.Errorf("%w: nama wajib diisi", ErrKelembagaanInputInvalid)
	}
	if len(out.Name) > 255 {
		return out, fmt.Errorf("%w: nama maksimal 255 karakter", ErrKelembagaanInputInvalid)
	}
	if len(out.Position) > 128 {
		return out, fmt.Errorf("%w: jabatan maksimal 128 karakter", ErrKelembagaanInputInvalid)
	}
	if len(out.OJKPositionCode) > 16 {
		return out, fmt.Errorf("%w: sandi jabatan maksimal 16 karakter", ErrKelembagaanInputInvalid)
	}
	if out.Status == "" {
		out.Status = KelembagaanOrangAktif
	}
	if out.Status != KelembagaanOrangAktif && out.Status != KelembagaanOrangNonaktif {
		return out, fmt.Errorf("%w: status hanya AKTIF atau NONAKTIF", ErrKelembagaanInputInvalid)
	}
	id, err := parseKelembagaanID(in.ID)
	if err != nil {
		return out, err
	}
	out.ID = id
	if out.LicenseDate, err = parseKelembagaanDate(in.LicenseDate, "tanggal surat"); err != nil {
		return out, err
	}
	if out.StartedAt, err = parseKelembagaanDate(in.StartedAt, "tanggal mulai menjabat"); err != nil {
		return out, err
	}
	if out.EndedAt, err = parseKelembagaanDate(in.EndedAt, "tanggal selesai menjabat"); err != nil {
		return out, err
	}
	return out, nil
}

// parseKelembagaanID mengurai id yang dikirim klien; kosong berarti buat baru.
func parseKelembagaanID(raw string) (uuid.UUID, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return uuid.New(), nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("%w: id bukan UUID yang sah", ErrKelembagaanInputInvalid)
	}
	return id, nil
}

// parseKelembagaanDate menerima tanggal kosong (belum diisi) atau YYYY-MM-DD.
func parseKelembagaanDate(raw, label string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrKelembagaanInputInvalid, label)
	}
	return &t, nil
}

// KelembagaanRepository menyimpan dan membaca data kelembagaan. Seluruh pembacaan
// bank-wide; penulisan menerima transaksi dari layanan agar audit berada pada
// transaksi yang sama.
type KelembagaanRepository interface {
	ListOffices(ctx context.Context) ([]BankOffice, error)
	ListManagement(ctx context.Context) ([]BankManagement, error)
	// UpsertOfficeTx menyimpan (INSERT ... ON CONFLICT id DO UPDATE) satu kantor.
	UpsertOfficeTx(ctx context.Context, tx any, office BankOffice, actorID uuid.UUID) error
	// DeleteOfficeTx menghapus satu kantor; found=false bila baris tidak ada.
	DeleteOfficeTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
	UpsertManagementTx(ctx context.Context, tx any, m BankManagement, actorID uuid.UUID) error
	DeleteManagementTx(ctx context.Context, tx any, id uuid.UUID) (bool, error)
	// UpdateForm00_11Tx menyimpan kolom Form 00.11 satu kantor; found=false bila tidak ada.
	UpdateForm00_11Tx(ctx context.Context, tx any, id uuid.UUID, in UpdateOfficeForm00_11Input) (bool, error)
}

// KelembagaanService merakit LAPORAN_KELEMBAGAAN dan melayani pengisian berizin.
type KelembagaanService interface {
	// KelembagaanReport menyusun laporan untuk satu posisi. Bank-wide.
	KelembagaanReport(ctx context.Context, asOf time.Time, actor Actor) (KelembagaanReport, error)
	// UpsertOffice menyimpan satu kantor (id kosong = buat baru), teraudit.
	UpsertOffice(ctx context.Context, input UpdateBankOfficeInput, actor Actor) (*BankOffice, error)
	// DeleteOffice menghapus satu kantor, teraudit.
	DeleteOffice(ctx context.Context, id uuid.UUID, actor Actor) error
	// UpsertManagement menyimpan satu orang direksi/komisaris/pejabat, teraudit.
	UpsertManagement(ctx context.Context, input UpdateBankManagementInput, actor Actor) (*BankManagement, error)
	// DeleteManagement menghapus satu orang, teraudit.
	DeleteManagement(ctx context.Context, id uuid.UUID, actor Actor) error
	// UpdateOfficeForm00_11 menyimpan kolom Form 00.11 satu kantor, teraudit.
	UpdateOfficeForm00_11(ctx context.Context, id uuid.UUID, in UpdateOfficeForm00_11Input, actor Actor) error
}
