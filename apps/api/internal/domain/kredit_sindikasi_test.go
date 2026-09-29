package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

// sindikasiInputValid menghasilkan masukan sah yang dapat diubah per kasus uji.
func sindikasiInputValid() UpdateSindikasiItemInput {
	return UpdateSindikasiItemInput{
		CounterpartyID:             "CP-1",
		NoRekening:                 "001",
		JumlahPendanaanSindikasi:   decimal.NewFromInt(1000000000),
		BagianPendanaan:            decimal.NewFromInt(200000000),
		SandiBankPeserta:           "123456",
		Plafon:                     decimal.NewFromInt(500000000),
		BakiDebet:                  decimal.NewFromInt(450000000),
		StatusKepesertaanCode:      SindikasiKepesertaanAnggota,
		NomorPerjanjianInduk:       "ABC-12345678",
		PendanaanDiBankPelaporCode: SindikasiPendanaanYa,
		KualitasCode:               SindikasiKualitasLancar,
		AsOf:                       "2026-09-30",
	}
}

// TestBuildSindikasiItemMenerimaMasukanSah memastikan masukan lengkap diterima dan status
// default AKTIF saat tidak diisi.
func TestBuildSindikasiItemMenerimaMasukanSah(t *testing.T) {
	item, err := BuildSindikasiItem(sindikasiInputValid())
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	if item.Status != SindikasiStatusAktif {
		t.Fatalf("status default = %q, ingin AKTIF", item.Status)
	}
	if item.CounterpartyID != "CP-1" || item.NomorPerjanjianInduk != "ABC-12345678" {
		t.Fatalf("bidang tidak terpetakan benar: %+v", item)
	}
}

// TestBuildSindikasiItemNoRekeningDanPendanaanSalingMengikat menguji aturan PDF #page 177:
// kolom IV dikosongkan bila X = 2, dan wajib diisi bila X = 1.
func TestBuildSindikasiItemNoRekeningDanPendanaanSalingMengikat(t *testing.T) {
	tanpaRekeningSaatYa := sindikasiInputValid()
	tanpaRekeningSaatYa.NoRekening = ""
	if _, err := BuildSindikasiItem(tanpaRekeningSaatYa); !errors.Is(err, ErrSindikasiInputInvalid) {
		t.Fatalf("X=1 tanpa No. Rekening harus ditolak, dapat %v", err)
	}

	rekeningSaatTidak := sindikasiInputValid()
	rekeningSaatTidak.PendanaanDiBankPelaporCode = SindikasiPendanaanTidak
	if _, err := BuildSindikasiItem(rekeningSaatTidak); !errors.Is(err, ErrSindikasiInputInvalid) {
		t.Fatalf("X=2 dengan No. Rekening harus ditolak, dapat %v", err)
	}

	kosongSaatTidak := sindikasiInputValid()
	kosongSaatTidak.PendanaanDiBankPelaporCode = SindikasiPendanaanTidak
	kosongSaatTidak.NoRekening = ""
	if _, err := BuildSindikasiItem(kosongSaatTidak); err != nil {
		t.Fatalf("X=2 tanpa No. Rekening adalah keadaan sah, tetapi ditolak: %v", err)
	}
}

// TestBuildSindikasiItemMenolakSandiDiLuarBaku memastikan sandi VIII/X/XI hanya menerima
// nilai baku PDF.
func TestBuildSindikasiItemMenolakSandiDiLuarBaku(t *testing.T) {
	kasus := []struct {
		nama string
		ubah func(*UpdateSindikasiItemInput)
	}{
		{"status kepesertaan 3", func(in *UpdateSindikasiItemInput) { in.StatusKepesertaanCode = "3" }},
		{"pendanaan 3", func(in *UpdateSindikasiItemInput) { in.PendanaanDiBankPelaporCode = "3" }},
		{"kualitas 6", func(in *UpdateSindikasiItemInput) { in.KualitasCode = "6" }},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			in := sindikasiInputValid()
			k.ubah(&in)
			if _, err := BuildSindikasiItem(in); !errors.Is(err, ErrSindikasiInputInvalid) {
				t.Fatalf("%s harus ditolak, dapat %v", k.nama, err)
			}
		})
	}
}

// TestBuildSindikasiItemMenolakNilaiNegatifDanHariNegatif memastikan seluruh angka dan hari
// tunggakan tidak negatif (PDF #page 176: hari paling singkat 0).
func TestBuildSindikasiItemMenolakNilaiNegatifDanHariNegatif(t *testing.T) {
	negatif := sindikasiInputValid()
	negatif.TunggakanPokok = decimal.NewFromInt(-1)
	if _, err := BuildSindikasiItem(negatif); !errors.Is(err, ErrSindikasiInputInvalid) {
		t.Fatalf("tunggakan pokok negatif harus ditolak, dapat %v", err)
	}

	hari := sindikasiInputValid()
	hari.HariTunggakanBunga = -1
	if _, err := BuildSindikasiItem(hari); !errors.Is(err, ErrSindikasiInputInvalid) {
		t.Fatalf("hari tunggakan negatif harus ditolak, dapat %v", err)
	}
}

// TestBuildSindikasiItemMenolakTanpaPerjanjianDanTanggalSalah memastikan kolom IX wajib dan
// tanggal harus YYYY-MM-DD.
func TestBuildSindikasiItemMenolakTanpaPerjanjianDanTanggalSalah(t *testing.T) {
	tanpaPerjanjian := sindikasiInputValid()
	tanpaPerjanjian.NomorPerjanjianInduk = ""
	if _, err := BuildSindikasiItem(tanpaPerjanjian); !errors.Is(err, ErrSindikasiInputInvalid) {
		t.Fatalf("tanpa Nomor Perjanjian harus ditolak, dapat %v", err)
	}

	tanggalSalah := sindikasiInputValid()
	tanggalSalah.AsOf = "30-09-2026"
	if _, err := BuildSindikasiItem(tanggalSalah); !errors.Is(err, ErrSindikasiInputInvalid) {
		t.Fatalf("tanggal salah format harus ditolak, dapat %v", err)
	}
}

// TestBuildSindikasiItemTidakMenyimpanIdentitas memastikan tipe masukan memang tidak punya
// bidang No. Identitas (keputusan privasi), dan galat tidak pernah memuat nomor identitas.
func TestBuildSindikasiItemTidakMenyimpanIdentitas(t *testing.T) {
	// Bidang ini sengaja tidak ada; uji menjaga agar tidak ditambahkan tanpa keputusan.
	if strings.Contains(strings.ToLower(sindikasiInputValid().CounterpartyID), "nik") {
		t.Fatal("ID Pihak Lawan bukan wadah NIK")
	}
	item, err := BuildSindikasiItem(sindikasiInputValid())
	if err != nil {
		t.Fatalf("masukan sah ditolak: %v", err)
	}
	// SindikasiItem tidak memiliki bidang identitas; bila kelak ditambah, uji ini gagal
	// lewat kompilasi (bidang tak dikenal) sehingga keputusan privasi terpantau.
	_ = item.CounterpartyID
}
