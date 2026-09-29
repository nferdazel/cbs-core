package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// buildKepemilikanValid adalah masukan sah yang dipakai banyak kasus uji.
func buildKepemilikanValid() UpdateKepemilikanItemInput {
	return UpdateKepemilikanItemInput{
		ShareholderName:       "  Budi Santoso ",
		ShareholderAddress:    " Jl. Merdeka 1 ",
		ShareholderTypeCode:   "01",
		ShareholderStatusCode: "01",
		NominalAmount:         decimal.NewFromInt(50_000_000),
		OwnershipPercentage:   decimal.NewFromInt(60),
		ChangeStatusCode:      "2",
		AsOf:                  "2026-03-31",
	}
}

// BuildKepemilikanItem menerima masukan sah, menormalkan teks, mengurai tanggal,
// memberi id, dan mengisi status bawaan AKTIF.
func TestBuildKepemilikanItemSah(t *testing.T) {
	item, err := BuildKepemilikanItem(buildKepemilikanValid())
	if err != nil {
		t.Fatalf("masukan sah: %v", err)
	}
	if item.ID == [16]byte{} {
		t.Fatal("id kosong tidak dibangkitkan")
	}
	if item.ShareholderName != "Budi Santoso" {
		t.Fatalf("nama tidak dinormalkan: %q", item.ShareholderName)
	}
	if item.ShareholderAddress != "Jl. Merdeka 1" {
		t.Fatalf("alamat tidak dinormalkan: %q", item.ShareholderAddress)
	}
	if item.Status != KepemilikanStatusAktif {
		t.Fatalf("status bawaan = %q, ingin %q", item.Status, KepemilikanStatusAktif)
	}
	if !item.AsOf.Equal(time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("as_of = %s", item.AsOf)
	}
}

// Masukan tidak sah ditolak dengan ErrKepemilikanInputInvalid: nama kosong, sandi
// asing, nominal negatif, persentase di luar 0-100, status asing, dan tanggal salah.
func TestBuildKepemilikanItemTidakSah(t *testing.T) {
	cases := []struct {
		nama string
		ubah func(*UpdateKepemilikanItemInput)
	}{
		{"nama kosong", func(in *UpdateKepemilikanItemInput) { in.ShareholderName = "  " }},
		{"jenis asing", func(in *UpdateKepemilikanItemInput) { in.ShareholderTypeCode = "05" }},
		{"status pemegang asing", func(in *UpdateKepemilikanItemInput) { in.ShareholderStatusCode = "03" }},
		{"nominal negatif", func(in *UpdateKepemilikanItemInput) { in.NominalAmount = decimal.NewFromInt(-1) }},
		{"persentase negatif", func(in *UpdateKepemilikanItemInput) { in.OwnershipPercentage = decimal.NewFromInt(-1) }},
		{"persentase melebihi 100", func(in *UpdateKepemilikanItemInput) { in.OwnershipPercentage = decimal.NewFromInt(101) }},
		{"status perubahan asing", func(in *UpdateKepemilikanItemInput) { in.ChangeStatusCode = "4" }},
		{"status asing", func(in *UpdateKepemilikanItemInput) { in.Status = "HAPUS" }},
		{"as_of kosong", func(in *UpdateKepemilikanItemInput) { in.AsOf = "" }},
		{"id bukan uuid", func(in *UpdateKepemilikanItemInput) { in.ID = "bukan-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			in := buildKepemilikanValid()
			tc.ubah(&in)
			if _, err := BuildKepemilikanItem(in); !errors.Is(err, ErrKepemilikanInputInvalid) {
				t.Fatalf("err = %v, ingin ErrKepemilikanInputInvalid", err)
			}
		})
	}
}

// Alamat boleh kosong (form mengizinkannya untuk kepemilikan <2%) dan sistem TIDAK
// memvalidasi bahwa seluruh persentase berjumlah 100%: dua baris 40% tetap sah.
func TestBuildKepemilikanItemAlamatKosongPersenTidakDivalidasi(t *testing.T) {
	kosong := buildKepemilikanValid()
	kosong.ShareholderAddress = "  "
	item, err := BuildKepemilikanItem(kosong)
	if err != nil {
		t.Fatalf("alamat kosong harus diterima: %v", err)
	}
	if item.ShareholderAddress != "" {
		t.Fatalf("alamat kosong = %q, ingin kosong", item.ShareholderAddress)
	}

	// Dua pemegang saham masing-masing 40% hanya berjumlah 80%: bukan urusan sistem.
	for i := 0; i < 2; i++ {
		in := buildKepemilikanValid()
		in.OwnershipPercentage = decimal.NewFromInt(40)
		if _, err := BuildKepemilikanItem(in); err != nil {
			t.Fatalf("persentase per baris 40%% harus diterima (Σ tidak divalidasi): %v", err)
		}
	}
}
