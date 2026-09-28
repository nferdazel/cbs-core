package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// buildAYDAValid adalah masukan sah yang dipakai banyak kasus uji.
func buildAYDAValid() UpdateAYDAItemInput {
	return UpdateAYDAItemInput{
		CollateralTypeCode:      "02",
		CollateralAddress:       " Jl. Merdeka 1 ",
		AcquisitionDate:         "2025-06-30",
		InitialRecognitionValue: decimal.NewFromInt(1_000_000),
		AccumulatedImpairment:   decimal.NewFromInt(100_000),
		NetRealizableValue:      decimal.NewFromInt(900_000),
		AsOf:                    "2026-03-31",
	}
}

// BuildAYDAItem menerima masukan sah, menormalkan teks, mengurai tanggal, memberi id,
// dan mengisi status bawaan AKTIF.
func TestBuildAYDAItemSah(t *testing.T) {
	item, err := BuildAYDAItem(buildAYDAValid())
	if err != nil {
		t.Fatalf("masukan sah: %v", err)
	}
	if item.ID == [16]byte{} {
		t.Fatal("id kosong tidak dibangkitkan")
	}
	if item.CollateralAddress != "Jl. Merdeka 1" {
		t.Fatalf("alamat tidak dinormalkan: %q", item.CollateralAddress)
	}
	if item.Status != AYDAStatusAktif {
		t.Fatalf("status bawaan = %q, ingin %q", item.Status, AYDAStatusAktif)
	}
	if !item.AcquisitionDate.Equal(time.Date(2025, time.June, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("tanggal pengambilalihan = %s", item.AcquisitionDate)
	}
}

// Masukan tidak sah ditolak dengan ErrAYDAInputInvalid: sandi jenis agunan asing,
// alamat kosong, nominal negatif, status asing, dan tanggal salah format.
func TestBuildAYDAItemTidakSah(t *testing.T) {
	cases := []struct {
		nama string
		ubah func(*UpdateAYDAItemInput)
	}{
		{"jenis agunan asing", func(in *UpdateAYDAItemInput) { in.CollateralTypeCode = "07" }},
		{"alamat kosong", func(in *UpdateAYDAItemInput) { in.CollateralAddress = "  " }},
		{"nilai awal negatif", func(in *UpdateAYDAItemInput) { in.InitialRecognitionValue = decimal.NewFromInt(-1) }},
		{"akumulasi negatif", func(in *UpdateAYDAItemInput) { in.AccumulatedImpairment = decimal.NewFromInt(-1) }},
		{"nrv negatif", func(in *UpdateAYDAItemInput) { in.NetRealizableValue = decimal.NewFromInt(-1) }},
		{"status asing", func(in *UpdateAYDAItemInput) { in.Status = "HAPUS" }},
		{"acquisition_date salah format", func(in *UpdateAYDAItemInput) { in.AcquisitionDate = "30-06-2025" }},
		{"as_of kosong", func(in *UpdateAYDAItemInput) { in.AsOf = "" }},
		{"id bukan uuid", func(in *UpdateAYDAItemInput) { in.ID = "bukan-uuid" }},
	}
	for _, tc := range cases {
		t.Run(tc.nama, func(t *testing.T) {
			in := buildAYDAValid()
			tc.ubah(&in)
			if _, err := BuildAYDAItem(in); !errors.Is(err, ErrAYDAInputInvalid) {
				t.Fatalf("err = %v, ingin ErrAYDAInputInvalid", err)
			}
		})
	}
}

// Jumlah memakai nilai yang lebih rendah dari NRV atau nilai tercatat (V - VI), dan
// menolak menghitung bila nilai tercatat negatif.
func TestAYDAItemJumlah(t *testing.T) {
	// Nilai tercatat 900 = V 1000 - VI 100; NRV 950 lebih tinggi -> dipakai tercatat.
	item := AYDAItem{
		InitialRecognitionValue: decimal.NewFromInt(1000),
		AccumulatedImpairment:   decimal.NewFromInt(100),
		NetRealizableValue:      decimal.NewFromInt(950),
	}
	if got, ok := item.Jumlah(); !ok || !got.Equal(decimal.NewFromInt(900)) {
		t.Fatalf("jumlah = %s ok=%v, ingin 900", got, ok)
	}
	// NRV 800 lebih rendah dari tercatat 900 -> dipakai NRV.
	item.NetRealizableValue = decimal.NewFromInt(800)
	if got, ok := item.Jumlah(); !ok || !got.Equal(decimal.NewFromInt(800)) {
		t.Fatalf("jumlah = %s ok=%v, ingin 800", got, ok)
	}
	// Akumulasi melebihi nilai pengakuan awal -> tidak dapat dihitung.
	item.AccumulatedImpairment = decimal.NewFromInt(1100)
	if _, ok := item.Jumlah(); ok {
		t.Fatal("nilai tercatat negatif tidak boleh dihitung")
	}
}
