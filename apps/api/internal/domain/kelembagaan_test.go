package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

// Kantor tanpa tipe/nama harus ditolak; nilai yang sah dinormalkan (status bawaan
// AKTIF) dan tanggal diurai.
func TestBuildBankOffice(t *testing.T) {
	if _, err := BuildBankOffice(UpdateBankOfficeInput{}); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("input kosong: err=%v, ingin ErrKelembagaanInputInvalid", err)
	}

	o, err := BuildBankOffice(UpdateBankOfficeInput{
		OfficeType: " KANTOR_CABANG ",
		Name:       " Purworejo ",
		City:       "Purworejo",
		OpenedAt:   "2024-01-02",
	})
	if err != nil {
		t.Fatalf("kantor sah: %v", err)
	}
	if o.OfficeType != "KANTOR_CABANG" || o.Name != "Purworejo" {
		t.Fatalf("nilai tidak dinormalkan: %+v", o)
	}
	if o.Status != KelembagaanKantorAktif {
		t.Fatalf("status bawaan = %q, ingin %q", o.Status, KelembagaanKantorAktif)
	}
	if o.OpenedAt == nil {
		t.Fatal("tanggal buka tidak diurai")
	}
	if o.ID == uuid.Nil {
		t.Fatal("id kosong tidak dibangkitkan")
	}

	if _, err := BuildBankOffice(UpdateBankOfficeInput{OfficeType: "X", Name: "Y", Status: "HAPUS"}); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("status tak dikenal: err=%v, ingin ErrKelembagaanInputInvalid", err)
	}
	if _, err := BuildBankOffice(UpdateBankOfficeInput{OfficeType: "X", Name: "Y", OpenedAt: "02-01-2024"}); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("tanggal salah format: err=%v, ingin ErrKelembagaanInputInvalid", err)
	}
}

// Kategori di luar tiga nilai baku harus ditolak; status bawaan AKTIF.
func TestBuildBankManagement(t *testing.T) {
	m, err := BuildBankManagement(UpdateBankManagementInput{
		Category: "direksi", Name: "Budi", Position: "Direktur Utama", OJKPositionCode: "110",
	})
	if err != nil {
		t.Fatalf("pengurus sah: %v", err)
	}
	if m.Category != KelembagaanKategoriDireksi {
		t.Fatalf("kategori = %q, ingin %q", m.Category, KelembagaanKategoriDireksi)
	}
	if m.Status != KelembagaanOrangAktif {
		t.Fatalf("status bawaan = %q, ingin %q", m.Status, KelembagaanOrangAktif)
	}

	if _, err := BuildBankManagement(UpdateBankManagementInput{Category: "PEMILIK", Name: "X"}); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("kategori tak dikenal: err=%v, ingin ErrKelembagaanInputInvalid", err)
	}
	if _, err := BuildBankManagement(UpdateBankManagementInput{Category: KelembagaanKategoriKomisaris, Name: ""}); !errors.Is(err, ErrKelembagaanInputInvalid) {
		t.Fatalf("nama kosong: err=%v, ingin ErrKelembagaanInputInvalid", err)
	}
}
