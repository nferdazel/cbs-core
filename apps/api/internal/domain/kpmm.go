package domain

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// kpmm.go memuat komponen laporan KPMM (Kewajiban Penyediaan Modal Minimum) BPR:
// modal inti, modal pelengkap, ATMR, dan rasionya.
//
// Dasar hukum:
//   - POJK No. 5/POJK.03/2015 (KPMM & Modal Inti Minimum BPR), dilanjutkan POJK
//     No. 7 Tahun 2026: rasio KPMM minimum 12%, modal inti minimum 8% dan
//     Rp6.000.000.000, modal pelengkap paling tinggi 100% modal inti.
//   - SEOJK No. 2/SEOJK.03/2025 (KPMM BPR) Lampiran II: rincian bobot risiko ATMR.
//   - SEOJK No. 21/SEOJK.03/2024 (PA BPR) butir 1.1.6: selisih PPKA > CKPN
//     menjadi faktor pengurang modal inti.
//
// Prinsip penyajian: komponen yang tidak dapat dihitung dari data yang ada
// ditandai Tersedia=false beserta Alasannya, BUKAN diisi nol. Nol dan tidak
// tersedia adalah dua hal berbeda bagi regulator.

// KPMMKomponen adalah satu komponen angka laporan yang boleh jadi belum tersedia.
type KPMMKomponen struct {
	Nilai    decimal.Decimal `json:"nilai"`
	Tersedia bool            `json:"tersedia"`
	// Alasan wajib terisi bila Tersedia=false; menyebut data/kunci yang kurang.
	Alasan string `json:"alasan,omitempty"`
}

// KPMMModalKelasBaris merinci saldo baris laporan menurut kelas modal pada pemetaan
// bagan akun (coa_mapping.go). Hanya kelas yang PASTI dijumlahkan; kode tanpa kelas
// tidak muncul (bukan diisi nol) supaya klasifikasi yang belum pasti tidak menjadi
// modal fiktif.
type KPMMModalKelasBaris struct {
	Kelas string          `json:"kelas"`
	Nilai decimal.Decimal `json:"nilai"`
}

// KPMMATMRBaris adalah satu kelompok pos aset ATMR beserta bobot risikonya.
type KPMMATMRBaris struct {
	Kategori string          `json:"kategori"`
	Dasar    decimal.Decimal `json:"dasar"`
	Bobot    decimal.Decimal `json:"bobot"`
	Nilai    decimal.Decimal `json:"nilai"`
}

// KPMMReport adalah hasil perhitungan KPMM satu posisi.
type KPMMReport struct {
	AsOf time.Time `json:"as_of"`
	Book string    `json:"book"`
	// ATMR adalah jumlah aset tertimbang menurut risiko.
	ATMR KPMMKomponen `json:"atmr"`
	// ATMRBaris merinci dasar dan bobot tiap kategori agar dapat diperiksa.
	ATMRBaris []KPMMATMRBaris `json:"atmr_baris"`
	// ATMRTidakTerkategori menyebut pos neraca tanpa pemetaan pos OJK sehingga
	// tidak ikut ATMR; bila ada, angkanya ditandai belum lengkap.
	ATMRTidakTerkategori []string `json:"atmr_tidak_terkategori,omitempty"`
	// ModalIntiUtama = ekuitas + laba/rugi tahun berjalan (sesuai identitas
	// aset = kewajiban + ekuitas + laba/rugi berjalan pada laporan sumber).
	ModalIntiUtama KPMMKomponen `json:"modal_inti_utama"`
	// PengurangModalInti dari selisih PPKA > CKPN (SEOJK 21/2024 butir 1.1.6).
	PengurangModalInti KPMMKomponen `json:"pengurang_modal_inti"`
	// ModalInti = ModalIntiUtama - PengurangModalInti.
	ModalInti KPMMKomponen `json:"modal_inti"`
	// ModalPelengkap: hanya sejauh data mendukung (PPKA umum & surplus revaluasi
	// belum dipisah dari data yang ada).
	ModalPelengkap KPMMKomponen `json:"modal_pelengkap"`
	// ModalPelengkapInstrumen, SurplusRevaluasi, dan PPKAUmum merinci komponen modal
	// pelengkap (POJK 5/2015 Pasal 10 ayat (1)). Masing-masing Tersedia=false beserta
	// alasannya bila datanya belum ada — BUKAN diisi nol, karena nol dan tidak
	// tersedia adalah dua hal berbeda bagi regulator.
	ModalPelengkapInstrumen KPMMKomponen `json:"modal_pelengkap_instrumen"`
	SurplusRevaluasi        KPMMKomponen `json:"surplus_revaluasi"`
	PPKAUmum                KPMMKomponen `json:"ppka_umum"`
	// TotalModal = ModalInti + ModalPelengkap (setelah batas 100% modal inti).
	TotalModal KPMMKomponen `json:"total_modal"`
	// RasioKPMM dalam PERSEN. Tersedia hanya bila ATMR dan TotalModal tersedia.
	RasioKPMM KPMMKomponen `json:"rasio_kpmm"`
	// RasioModalInti dalam PERSEN.
	RasioModalInti KPMMKomponen `json:"rasio_modal_inti"`
	// Ambang yang dipakai, diambil dari konfigurasi.
	KPMMMinFrac        decimal.Decimal `json:"kpmm_min_frac"`
	ModalIntiMinFrac   decimal.Decimal `json:"modal_inti_min_frac"`
	ModalIntiMinAmount decimal.Decimal `json:"modal_inti_min_amount"`
	// Ambang lain yang berlaku (dibaca dari konfigurasi) supaya pembaca dapat
	// memeriksa batas yang dipakai walau komponennya belum dapat dihitung.
	ModalPelengkapMaxFrac decimal.Decimal `json:"modal_pelengkap_max_frac"`
	// ModalPelengkapInstrumenMaxFrac adalah sub-batas komponen pelengkap ber-instrumen
	// terhadap modal inti (POJK 5/2015 Pasal 10 ayat (2)).
	ModalPelengkapInstrumenMaxFrac decimal.Decimal `json:"modal_pelengkap_instrumen_max_frac"`
	PPKAUmumRWAMaxFrac             decimal.Decimal `json:"ppka_umum_rwa_max_frac"`
	// ModalKelasCOA merinci saldo baris laporan menurut kelas modal pemetaan COA.
	// Kode yang kelasnya belum pasti tidak ikut, sehingga tidak menjadi modal fiktif.
	ModalKelasCOA []KPMMModalKelasBaris `json:"modal_kelas_coa,omitempty"`
	// DeductionBasis mencatat pilihan per_kredit/agregat yang dipakai.
	DeductionBasis string `json:"deduction_basis"`
	// ParameterGaps menyebut kunci konfigurasi yang belum diisi.
	ParameterGaps []string `json:"parameter_gaps,omitempty"`
	// Catatan menjelaskan batas perhitungan yang harus diketahui pembaca.
	Catatan []string `json:"catatan,omitempty"`
}

// KPMMService menghitung laporan KPMM BPR secara baca-saja.
type KPMMService interface {
	// Hitung menyusun laporan KPMM untuk posisi asOf (bank-wide). actor dipakai
	// membatasi pembacaan data kredit pada cabangnya.
	Hitung(ctx context.Context, asOf time.Time, book string, actor Actor) (KPMMReport, error)
}
