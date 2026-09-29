package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// form00_11_kantor.go memuat jalur isi Form 00.11 "Data Kantor selain Kantor Pusat dan
// Kantor Cabang dan Terminal Perbankan Elektronik (TPE)".
//
// Struktur dari PDF resmi SEOJK No. 16/SEOJK.03/2024: PDF #page 270 (hlm. 218) susunan,
// sandi #271-272 (hlm. 219-220), penjelasan #273-275 (hlm. 221-223). Empat belas kolom;
// sandi I Jenis 02-08/99; sandi X Keterangan 1-7; XIV Lokasi Lampiran 03.
//
// Jalur ini hanya menyentuh kolom Form 00.11 pada bank_offices (migrasi 000126). Ia
// TERPISAH dari upsert kelembagaan Form 00.04: kolom yang tumpang tindih (code, name,
// address, ojk_kabupaten_code) tidak diubah di sini agar Form 00.04 tidak bergeser.
// Baris dipilih oleh sandi Jenis (kolom I) karena kantor pusat/cabang tidak punya sandi
// itu — penggolongan tidak ditebak dari teks bebas office_type.

// Sandi baku kolom I Form 00.11 (PDF #271).
const (
	KantorKindKantorKas       = "02"
	KantorKindKasKeliling     = "03"
	KantorKindTitikPembayaran = "04"
	KantorKindATM             = "05"
	KantorKindEDC             = "06"
	KantorKindKantorWilayah   = "07"
	KantorKindSentraKeuangan  = "08"
	KantorKindLainnya         = "99"
)

// UpdateOfficeForm00_11Input adalah isi kolom Form 00.11 satu kantor. Hanya kolom Form
// 00.11; identitas kantor Form 00.04 tidak disentuh dari sini.
type UpdateOfficeForm00_11Input struct {
	OJKOfficeKindCode  string `json:"ojk_office_kind_code"`
	ParentOfficeCode   string `json:"parent_office_code"`
	PreviousOfficeCode string `json:"previous_office_code"`
	Coordinates        string `json:"coordinates"`
	HeadName           string `json:"head_name"`
	PhoneNumber        string `json:"phone_number"`
	OJKChangeCode      string `json:"ojk_change_code"`
	// Tanggal memakai format YYYY-MM-DD; kosong = belum diisi.
	ImplementationDate string `json:"implementation_date"`
	ControlOfficeCode  string `json:"control_office_code"`
	OJKApprovalDate    string `json:"ojk_approval_date"`
}

// BuildOfficeForm00_11 memvalidasi masukan Form 00.11. Sandi Jenis wajib (menentukan
// baris masuk form); sandi Keterangan boleh kosong; tanggal diurai YYYY-MM-DD.
func BuildOfficeForm00_11(in UpdateOfficeForm00_11Input) (UpdateOfficeForm00_11Input, error) {
	out := UpdateOfficeForm00_11Input{
		OJKOfficeKindCode:  strings.TrimSpace(in.OJKOfficeKindCode),
		ParentOfficeCode:   strings.TrimSpace(in.ParentOfficeCode),
		PreviousOfficeCode: strings.TrimSpace(in.PreviousOfficeCode),
		Coordinates:        strings.TrimSpace(in.Coordinates),
		HeadName:           strings.TrimSpace(in.HeadName),
		PhoneNumber:        strings.TrimSpace(in.PhoneNumber),
		OJKChangeCode:      strings.TrimSpace(in.OJKChangeCode),
		ImplementationDate: strings.TrimSpace(in.ImplementationDate),
		ControlOfficeCode:  strings.TrimSpace(in.ControlOfficeCode),
		OJKApprovalDate:    strings.TrimSpace(in.OJKApprovalDate),
	}
	switch out.OJKOfficeKindCode {
	case KantorKindKantorKas, KantorKindKasKeliling, KantorKindTitikPembayaran,
		KantorKindATM, KantorKindEDC, KantorKindKantorWilayah,
		KantorKindSentraKeuangan, KantorKindLainnya:
	case "":
		return out, fmt.Errorf("%w: sandi Jenis Form 00.11 wajib diisi (form hanya memuat kantor yang ditandai jenisnya)",
			ErrKelembagaanInputInvalid)
	default:
		return out, fmt.Errorf("%w: sandi Jenis Form 00.11 hanya 02-08 atau 99", ErrKelembagaanInputInvalid)
	}
	if out.OJKChangeCode != "" {
		switch out.OJKChangeCode {
		case "1", "2", "3", "4", "5", "6", "7":
		default:
			return out, fmt.Errorf("%w: sandi Keterangan Form 00.11 hanya 1-7", ErrKelembagaanInputInvalid)
		}
	}
	if len(out.ParentOfficeCode) > 16 || len(out.PreviousOfficeCode) > 16 ||
		len(out.ControlOfficeCode) > 16 {
		return out, fmt.Errorf("%w: sandi kantor maksimal 16 karakter", ErrKelembagaanInputInvalid)
	}
	if len(out.Coordinates) > 64 {
		return out, fmt.Errorf("%w: koordinat maksimal 64 karakter", ErrKelembagaanInputInvalid)
	}
	if len(out.HeadName) > 128 {
		return out, fmt.Errorf("%w: nama pimpinan maksimal 128 karakter", ErrKelembagaanInputInvalid)
	}
	if len(out.PhoneNumber) > 32 {
		return out, fmt.Errorf("%w: no. telepon maksimal 32 karakter", ErrKelembagaanInputInvalid)
	}
	var err error
	if out.ImplementationDate, err = normalizeKelembagaanDate(out.ImplementationDate, "tanggal pelaksanaan"); err != nil {
		return out, err
	}
	if out.OJKApprovalDate, err = normalizeKelembagaanDate(out.OJKApprovalDate, "tanggal persetujuan"); err != nil {
		return out, err
	}
	return out, nil
}

// normalizeKelembagaanDate mengembalikan string YYYY-MM-DD yang sah, atau "" bila kosong.
func normalizeKelembagaanDate(raw, field string) (string, error) {
	if raw == "" {
		return "", nil
	}
	if _, err := time.Parse("2006-01-02", raw); err != nil {
		return "", fmt.Errorf("%w: %s harus format YYYY-MM-DD", ErrKelembagaanInputInvalid, field)
	}
	return raw, nil
}

// OfficeForm00_11Repository menyimpan kolom Form 00.11 satu kantor.
type OfficeForm00_11Repository interface {
	// UpdateForm00_11Tx menyimpan kolom Form 00.11 satu kantor. found=false bila kantor
	// tidak ada.
	UpdateForm00_11Tx(ctx context.Context, tx any, id uuid.UUID, in UpdateOfficeForm00_11Input) (bool, error)
}

// OfficeForm00_11Service melayani pengisian kolom Form 00.11.
type OfficeForm00_11Service interface {
	UpdateOfficeForm00_11(ctx context.Context, id uuid.UUID, in UpdateOfficeForm00_11Input, actor Actor) error
}
