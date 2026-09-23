package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// ppka_umum.go memuat hasil perhitungan PPKA umum (Penyisihan Penghapusan Aset
// Produktif umum): minimum 0,5% atas aset produktif yang digolongkan Lancar,
// sesuai POJK No. 1 Tahun 2024 Pasal 19 ayat (2). Angka ini disajikan bersama
// laporan PPAP dan KPMM dari SATU sumber, bukan dihitung ulang di masing-masing
// laporan.
//
// Sumber datanya adalah data operasional yang sudah ada: kredit lancar (evaluasi
// PPAP) dan penempatan pada bank lain (modul Pasal 23) — keduanya memakai tarif
// ppap.rate_frac.gol_1. Bila salah satu sumber belum tersedia, komponennya
// ditandai belum lengkap, BUKAN diisi nol dan dianggap final.

// PPKAUmumSummary adalah hasil perhitungan PPKA umum satu posisi.
type PPKAUmumSummary struct {
	AsOf time.Time `json:"as_of"`
	// RateFrac adalah tarif PPKA umum yang dipakai (fraksi; 0,005 = 0,5%) dari
	// ppap.rate_frac.gol_1.
	RateFrac decimal.Decimal `json:"rate_frac"`
	// KreditLancarDasar adalah jumlah eksposur kredit bergolongan Lancar (setelah
	// pengecualian agunan tunai menurut Pasal 19 ayat (4) huruf b) yang menjadi
	// dasar PPKA umum.
	KreditLancarDasar decimal.Decimal `json:"kredit_lancar_dasar"`
	KreditLancarPPKA  decimal.Decimal `json:"kredit_lancar_ppka"`
	// PenempatanLancarDasar/PPKA berasal dari penempatan pada bank lain bergolongan
	// Lancar setelah pengurang jaminan LPS (Pasal 23); nol bila modul/saklar mati.
	PenempatanLancarDasar decimal.Decimal `json:"penempatan_lancar_dasar"`
	PenempatanLancarPPKA  decimal.Decimal `json:"penempatan_lancar_ppka"`
	// TotalDasar dan TotalPPKA adalah jumlah kedua sumber di atas.
	TotalDasar decimal.Decimal `json:"total_dasar"`
	TotalPPKA  decimal.Decimal `json:"total_ppka"`
	// SumberKredit/SumberPenempatan menandai sumber yang benar-benar ikut dihitung.
	// False berarti komponen itu belum tersedia dan angka adalah batas bawah.
	SumberKredit     bool `json:"sumber_kredit"`
	SumberPenempatan bool `json:"sumber_penempatan"`
	// Lengkap false berarti angka masih batas bawah; AlasanTidakLengkap menyebut
	// sumber yang belum tersedia. Angka tidak diubah oleh penanda ini.
	Lengkap            bool     `json:"lengkap"`
	AlasanTidakLengkap []string `json:"alasan_tidak_lengkap,omitempty"`
}

// PPKAUmumService menghitung PPKA umum dari data operasional kredit dan penempatan.
// Baca-saja: tidak memposting jurnal dan tidak mengubah state.
type PPKAUmumService interface {
	Hitung(ctx context.Context, asOf time.Time, actor Actor) (PPKAUmumSummary, error)
}
