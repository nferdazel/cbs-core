package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Sentinel error modul PPAP. Dipakai agar pemanggil bisa membedakan kegagalan
// konfigurasi/data dari kegagalan tak terduga.
var (
	ErrPPAPLoanNotFound    = errors.New("kredit untuk perhitungan PPAP tidak ditemukan")
	ErrPPAPReserveNotFound = errors.New("akun cadangan PPAP tidak ditemukan")
	ErrPPAPExpenseNotFound = errors.New("akun beban PPAP tidak ditemukan")
)

// Collectibility adalah kualitas aset versi numerik: 1 Lancar, 2 Dalam Perhatian
// Khusus, 3 Kurang Lancar, 4 Diragukan, 5 Macet. Nilai ini yang dipakai mesin PPAP;
// kolom database tetap menyimpan bentuk lama (mis. "2_DPK") demi kompatibilitas.
type Collectibility int

const (
	KolLancar       Collectibility = 1
	KolDPK          Collectibility = 2
	KolKurangLancar Collectibility = 3
	KolDiragukan    Collectibility = 4
	KolMacet        Collectibility = 5
)

func (c Collectibility) Valid() bool {
	return c >= KolLancar && c <= KolMacet
}

func (c Collectibility) Label() string {
	switch c {
	case KolLancar:
		return "Lancar"
	case KolDPK:
		return "Dalam Perhatian Khusus"
	case KolKurangLancar:
		return "Kurang Lancar"
	case KolDiragukan:
		return "Diragukan"
	case KolMacet:
		return "Macet"
	default:
		return "Tidak Dikenal"
	}
}

// IsNPL menandai kredit tidak lancar. Mulai kolektibilitas 3 (Kurang Lancar),
// pendapatan bunga/margin tidak boleh lagi diakui secara akrual (cash basis).
func (c Collectibility) IsNPL() bool {
	return c >= KolKurangLancar
}

// OJKCode memetakan nilai numerik ke kode lama yang tersimpan di kolom loans.collectibility.
func (c Collectibility) OJKCode() OJKCollectibility {
	switch c {
	case KolLancar:
		return CollectibilityKol1
	case KolDPK:
		return CollectibilityKol2
	case KolKurangLancar:
		return CollectibilityKol3
	case KolDiragukan:
		return CollectibilityKol4
	case KolMacet:
		return CollectibilityKol5
	default:
		return CollectibilityKol1
	}
}

// CollectibilityFromOJK mengubah kode kolektibilitas tersimpan menjadi nilai numerik.
// Nilai tak dikenal dianggap Lancar agar kredit lama tetap ikut terpantau.
func CollectibilityFromOJK(o OJKCollectibility) Collectibility {
	switch o {
	case CollectibilityKol2:
		return KolDPK
	case CollectibilityKol3:
		return KolKurangLancar
	case CollectibilityKol4:
		return KolDiragukan
	case CollectibilityKol5:
		return KolMacet
	default:
		return KolLancar
	}
}

// CollectibilityThresholds adalah batas atas jumlah hari tunggakan angsuran (DPD)
// per golongan. Regulasi menyambung dimensi tunggakan dengan dimensi jatuh tempo
// memakai "dan/atau", sehingga golongan akhir adalah yang TERBURUK di antara
// keduanya - lihat CollectibilityFromPosition.
type CollectibilityThresholds struct {
	// Lancar adalah batas atas tunggakan yang masih digolongkan Lancar selama
	// Kredit belum jatuh tempo. Ini bagian dari definisi Lancar di Lampiran II,
	// bukan kelonggaran opsional.
	Lancar       int
	DPK          int
	KurangLancar int
	Diragukan    int
}

// DefaultCollectibilityThresholds adalah pita DPD untuk Kredit dengan angsuran
// 1 (satu) bulan atau lebih menurut POJK No. 1 Tahun 2024 tentang Kualitas Aset
// Bank Perekonomian Rakyat, Lampiran II:
//
//	Lancar       : tanpa tunggakan, atau tunggakan <= 30 hari dan Kredit belum jatuh tempo
//	DPK          : tunggakan > 30 s/d 90 hari, dan/atau telah jatuh tempo <= 15 hari
//	Kurang Lancar: tunggakan > 90 s/d 180 hari, dan/atau telah jatuh tempo > 15 s/d 30 hari
//	Diragukan    : tunggakan > 180 s/d 360 hari, dan/atau telah jatuh tempo > 30 s/d 60 hari
//	Macet        : tunggakan > 360 hari, dan/atau telah jatuh tempo > 60 hari, dan/atau
//	               diserahkan kepada DJKN, dan/atau diajukan klaim asuransi Kredit
//
// POJK 33/POJK.03/2018 yang dulu dirujuk sudah dicabut oleh POJK 1/2024. Pitanya
// kebetulan sama, tetapi dasar hukum yang berlaku sekarang adalah POJK 1/2024.
func DefaultCollectibilityThresholds() CollectibilityThresholds {
	return CollectibilityThresholds{Lancar: 30, DPK: 90, KurangLancar: 180, Diragukan: 360}
}

// SubMonthlyCollectibilityThresholds adalah pita DPD untuk Kredit dengan angsuran
// kurang dari 1 (satu) bulan (POJK 1/2024 Lampiran II): Lancar <= 15 hari,
// DPK > 15 s/d 30, Kurang Lancar > 30 s/d 90, Diragukan > 90 s/d 180, Macet > 180.
// Batas jatuh tempo (15/30/60 hari) sama dengan tabel bulanan.
//
// Belum dipakai jalur produksi: seluruh jadwal angsuran sistem ini masih bulanan
// (loans.monthly_installment), sehingga pita bulanan yang berlaku. Nilai ini
// disiapkan dan diuji lebih dulu agar aturannya sudah benar saat produk dengan
// angsuran mingguan/harian diperkenalkan.
func SubMonthlyCollectibilityThresholds() CollectibilityThresholds {
	return CollectibilityThresholds{Lancar: 15, DPK: 30, KurangLancar: 90, Diragukan: 180}
}

// PPAPExposure menghitung eksposur yang dikenai tarif PPAP setelah dikurangi nilai agunan
// pengurang, dengan lantai nol: agunan yang nilainya melebihi baki debet tidak menghasilkan
// eksposur negatif, karena penyisihan negatif tidak punya arti.
//
// PASAL DAN CAKUPAN RESMINYA BELUM DIVERIFIKASI. Catatan lama proyek menyebut Pasal 20,
// komentar migrasi 000026 menyebut Pasal 17; salah satu salah. Fungsi ini disediakan agar
// perhitungannya siap, tetapi baru dipakai bila ppap.collateral.enabled diaktifkan setelah
// teks POJK No. 1 Tahun 2024 diverifikasi.
func PPAPExposure(outstanding, collateralValue decimal.Decimal) decimal.Decimal {
	if collateralValue.IsNegative() {
		collateralValue = decimal.Zero
	}
	exposure := outstanding.Sub(collateralValue)
	if exposure.IsNegative() {
		return decimal.Zero
	}
	return exposure
}

// Batas umur jatuh tempo Kredit dalam hari (POJK 1/2024 Lampiran II, kolom
// kemampuan membayar). Berlaku sama untuk kedua tabel frekuensi angsuran.
const (
	maturityDPKDays          = 15
	maturityKurangLancarDays = 30
	maturityDiragukanDays    = 60
)

// CollectibilityFromMaturity menggolongkan Kredit dari umurnya sejak tanggal jatuh
// tempo. daysPastMaturity <= 0 berarti Kredit belum jatuh tempo.
func CollectibilityFromMaturity(daysPastMaturity int) Collectibility {
	switch {
	case daysPastMaturity <= 0:
		return KolLancar
	case daysPastMaturity <= maturityDPKDays:
		return KolDPK
	case daysPastMaturity <= maturityKurangLancarDays:
		return KolKurangLancar
	case daysPastMaturity <= maturityDiragukanDays:
		return KolDiragukan
	default:
		return KolMacet
	}
}

// CollectibilityFromPosition menentukan golongan dari dua dimensi sekaligus:
// tunggakan angsuran (dpd) dan umur jatuh tempo Kredit (daysPastMaturity, <= 0
// berarti belum jatuh tempo). Regulasi menyambung keduanya dengan "dan/atau",
// sehingga golongan yang berlaku adalah yang TERBURUK dari kedua perhitungan.
func CollectibilityFromPosition(dpd, daysPastMaturity int, t CollectibilityThresholds) Collectibility {
	byArrears := collectibilityFromArrears(dpd, sanitizeThresholds(t))
	if byMaturity := CollectibilityFromMaturity(daysPastMaturity); byMaturity > byArrears {
		return byMaturity
	}
	return byArrears
}

// CollectibilityFromDPD hanya memakai dimensi tunggakan; dipakai pemanggil yang
// tidak punya informasi tanggal jatuh tempo Kredit.
func CollectibilityFromDPD(dpd int, t CollectibilityThresholds) Collectibility {
	return collectibilityFromArrears(dpd, sanitizeThresholds(t))
}

func collectibilityFromArrears(dpd int, t CollectibilityThresholds) Collectibility {
	switch {
	case dpd <= t.Lancar: // mencakup dpd <= 0; sanitize menjamin Lancar > 0
		return KolLancar
	case dpd <= t.DPK:
		return KolDPK
	case dpd <= t.KurangLancar:
		return KolKurangLancar
	case dpd <= t.Diragukan:
		return KolDiragukan
	default:
		return KolMacet
	}
}

// sanitizeThresholds menjaga ambang tetap menaik dan tidak pernah lebih longgar
// dari default POJK: POJK menetapkan standar MINIMUM, sehingga bank boleh
// menggolongkan Kredit lebih buruk dari standar, tetapi tidak boleh lebih baik.
// Ambang yang lebih longgar dari default dipulihkan ke default; urutan yang
// terbalik dipersempit dari batas yang lebih ringan, bukan dilonggarkan dari yang
// lebih berat, agar hasilnya tidak pernah lebih ringan dari default.
func sanitizeThresholds(t CollectibilityThresholds) CollectibilityThresholds {
	def := DefaultCollectibilityThresholds()
	out := CollectibilityThresholds{
		Diragukan:    tighten(t.Diragukan, def.Diragukan),
		KurangLancar: tighten(t.KurangLancar, def.KurangLancar),
		DPK:          tighten(t.DPK, def.DPK),
		Lancar:       tighten(t.Lancar, def.Lancar),
	}
	if out.KurangLancar > out.Diragukan {
		out.KurangLancar = out.Diragukan
	}
	if out.DPK > out.KurangLancar {
		out.DPK = out.KurangLancar
	}
	if out.Lancar >= out.DPK {
		out.Lancar = out.DPK - 1
	}
	if out.Lancar < 0 {
		out.Lancar = 0
	}
	return out
}

// tighten memakai ambang konfigurasi hanya bila lebih ketat dari default. Nilai
// kosong (tidak dikonfigurasi) atau lebih longgar dari default berarti default.
func tighten(configured, def int) int {
	if configured <= 0 || configured > def {
		return def
	}
	return configured
}

// RestructureCleanPeriodsForLancar adalah jumlah periode pembayaran bersih
// berturut-turut yang membebaskan Kredit restrukturisasi dari batas kualitasnya
// (POJK No. 1 Tahun 2024 Pasal 23 ayat (2) huruf a).
const RestructureCleanPeriodsForLancar = 3

// RestructureCollectibility menerapkan Pasal 23 POJK No. 1 Tahun 2024:
//
//   - Kredit yang sebelum restrukturisasi tergolong Diragukan atau Macet paling tinggi
//     Kurang Lancar (ayat (1) huruf a).
//   - Kredit yang sebelum restrukturisasi tergolong Lancar, DPK, atau Kurang Lancar
//     tidak boleh membaik (ayat (1) huruf b).
//   - Batas itu lepas setelah RestructureCleanPeriodsForLancar kali periode pembayaran
//     bersih berturut-turut (ayat (2) huruf a). Sebelum itu Kredit tetap boleh
//     memburuk, karena POJK adalah standar minimum, bukan plafon.
//
// before adalah kualitas sesaat sebelum restrukturisasi terakhir, computed adalah
// kualitas dari penilaian biasa, dan cleanPeriods adalah jumlah angsuran yang dibayar
// tepat waktu berturut-turut sejak restrukturisasi terakhir.
func RestructureCollectibility(before, computed Collectibility, cleanPeriods int) Collectibility {
	if cleanPeriods >= RestructureCleanPeriodsForLancar {
		return computed
	}
	floor := before
	if before >= KolDiragukan {
		floor = KolKurangLancar
	}
	if computed > floor {
		return computed
	}
	return floor
}

// PPAPRate adalah tarif penyisihan minimum atas pokok terutang, dalam fraksi
// (0.005 = 0,5%). Disimpan sebagai decimal agar tidak ada galat pembulatan biner.
type PPAPRate decimal.Decimal

// PPAPRates memetakan kolektibilitas ke tarif minimumnya.
type PPAPRates map[Collectibility]PPAPRate

// DefaultPPAPRates mengikuti PPAP minimum BPR menurut POJK No. 1 Tahun 2024
// tentang Kualitas Aset Bank Perekonomian Rakyat, Pasal 19 ayat (2) dan (3)
// (rumusan sama dengan POJK 33/POJK.03/2018 Pasal 16 yang sudah dicabut):
// PPAP umum atas aset produktif lancar 0,5%, dan PPAP khusus atas pokok terutang
// 3% (Dalam Perhatian Khusus), 10% (Kurang Lancar), 50% (Diragukan), 100% (Macet).
//
// PENTING: PPAP khusus dihitung SETELAH dikurangi nilai agunan pengurang
// (POJK 1/2024 Pasal 20). Sistem ini belum punya modul agunan, sehingga tarif di
// bawah dikenakan atas pokok penuh dan hasilnya cenderung LEBIH BESAR daripada
// kewajiban untuk kredit beragunan. Angka ini belum boleh dipakai sebagai laporan
// kepatuhan final sampai pengurang agunan tersedia.
func DefaultPPAPRates() PPAPRates {
	return PPAPRates{
		KolLancar:       PPAPRate(decimal.NewFromFloat(0.005)),
		KolDPK:          PPAPRate(decimal.NewFromFloat(0.03)),
		KolKurangLancar: PPAPRate(decimal.NewFromFloat(0.10)),
		KolDiragukan:    PPAPRate(decimal.NewFromFloat(0.50)),
		KolMacet:        PPAPRate(decimal.NewFromFloat(1.00)),
	}
}

// PPAPCalculation adalah hasil perhitungan PPAP satu kredit.
type PPAPCalculation struct {
	Outstanding    decimal.Decimal
	Collectibility Collectibility
	Rate           PPAPRate
	Target         decimal.Decimal
	Existing       decimal.Decimal
	Adjustment     decimal.Decimal
}

// PPAPAmount menghitung target PPAP atas saldo pokok terutang (bukan total tagihan)
// dan membulatkannya ke rupiah penuh. Tarif yang tidak dikenal menghasilkan nol.
func PPAPAmount(outstanding decimal.Decimal, c Collectibility, rates PPAPRates) decimal.Decimal {
	rate, ok := rates[c]
	if !ok || outstanding.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	return RoundToRupiah(outstanding.Mul(decimal.Decimal(rate)))
}

// PPAPAdjustment adalah selisih yang harus diposting: positif berarti menambah
// cadangan (provisi), negatif berarti memulihkan cadangan (reversal), nol berarti
// cadangan sudah sesuai target.
func PPAPAdjustment(target, existing decimal.Decimal) decimal.Decimal {
	return RoundToRupiah(target.Sub(existing))
}

// CalculatePPAP merangkai perhitungan target dan selisih dari saldo cadangan yang
// sudah diakui untuk kredit tersebut.
func CalculatePPAP(outstanding decimal.Decimal, c Collectibility, existing decimal.Decimal, rates PPAPRates) PPAPCalculation {
	target := PPAPAmount(outstanding, c, rates)
	return PPAPCalculation{
		Outstanding:    outstanding,
		Collectibility: c,
		Rate:           rates[c],
		Target:         target,
		Existing:       existing,
		Adjustment:     PPAPAdjustment(target, existing),
	}
}

// PPAPLoanSnapshot adalah data kredit aktif yang dibutuhkan proses PPAP harian.
type PPAPLoanSnapshot struct {
	LoanID         uuid.UUID
	LoanNumber     string
	ProductID      *uuid.UUID
	Outstanding    decimal.Decimal
	Collectibility Collectibility // kolektibilitas tersimpan
	DPD            int            // DPD tersimpan
	AccrualStatus  AccrualStatus
	RequiredPPAP   decimal.Decimal // cadangan per kredit yang sudah diakui (target terakhir)
	// LastDueDate adalah jatuh tempo angsuran terlama yang belum dibayar; nil bila
	// seluruh angsuran sudah lunas.
	LastDueDate *time.Time
	// FinalDueDate adalah jatuh tempo Kredit (angsuran terakhir), dipakai menilai
	// dimensi "Kredit telah jatuh tempo" POJK 1/2024 Lampiran II.
	FinalDueDate *time.Time
	// IsRestructured menandai Kredit pernah direstrukturisasi. Bila true,
	// RestructureCollectibility membatasi golongannya (POJK 1/2024 Pasal 23).
	IsRestructured bool
	// PreRestructureCollectibility adalah kualitas sesaat sebelum restrukturisasi
	// terakhir; CleanPeriods adalah jumlah angsuran tepat waktu berturut-turut sejak
	// restrukturisasi terakhir. Keduanya masukan RestructureCollectibility.
	PreRestructureCollectibility Collectibility
	CleanPeriods                 int
}

// PPAPLoanUpdate membawa perubahan state kredit hasil proses PPAP.
type PPAPLoanUpdate struct {
	LoanID         uuid.UUID
	Collectibility Collectibility
	DPD            int
	AccrualStatus  AccrualStatus
	StopAccrual    bool
	RequiredPPAP   decimal.Decimal
}

// PPAPRunItem adalah hasil pemrosesan satu kredit.
type PPAPRunItem struct {
	LoanID         uuid.UUID      `json:"loan_id"`
	LoanNumber     string         `json:"loan_number"`
	DPD            int            `json:"dpd"`
	Collectibility Collectibility `json:"collectibility"`
	Outstanding    decimal.Decimal
	// CollateralValue adalah nilai agunan pengurang yang dipakai, dan Exposure adalah
	// baki debet setelah dikuranginya. Keduanya disimpan pada hasil agar selisih cadangan
	// antar hari dapat ditelusuri: tanpa ini, perubahan cadangan yang berasal dari agunan
	// baru tidak dapat dibedakan dari perubahan kolektibilitas.
	CollateralValue       decimal.Decimal
	Exposure              decimal.Decimal `json:"outstanding"`
	Target                decimal.Decimal `json:"target"`
	Existing              decimal.Decimal `json:"existing"`
	Adjustment            decimal.Decimal `json:"adjustment"`
	CollectibilityChanged bool            `json:"collectibility_changed"`
	StopAccrual           bool            `json:"stop_accrual"`
	Posted                bool            `json:"posted"`
}

// PPAPRunFailure mencatat kredit yang gagal diproses tanpa menggagalkan batch.
type PPAPRunFailure struct {
	LoanID     uuid.UUID `json:"loan_id"`
	LoanNumber string    `json:"loan_number"`
	Error      string    `json:"error"`
}

// PPAPRunSummary adalah ringkasan satu kali proses PPAP.
type PPAPRunSummary struct {
	AsOf            time.Time        `json:"as_of"`
	Total           int              `json:"total"`
	Processed       int              `json:"processed"`
	Failed          int              `json:"failed"`
	Skipped         int              `json:"skipped"` // tidak berubah, tidak ada posting
	TotalAdjustment decimal.Decimal  `json:"total_adjustment"`
	Items           []PPAPRunItem    `json:"items"`
	Failures        []PPAPRunFailure `json:"failures"`
	ReserveBefore   decimal.Decimal  `json:"reserve_before"` // saldo GL cadangan sebelum proses (rekonsiliasi)
	ReserveAfter    decimal.Decimal  `json:"reserve_after"`  // saldo GL cadangan setelah proses (rekonsiliasi)
	Preview         bool             `json:"preview"`        // true bila hanya simulasi, tanpa posting
}

// PPAPRepository adalah akses data proses PPAP. Seluruh SQL jurnal tetap lewat
// posting engine, bukan di sini.
type PPAPRepository interface {
	// ListDueLoans mengambil kredit aktif beserta outstanding dan jatuh tempo
	// angsuran terlama yang belum dibayar, per tanggal asOf.
	ListDueLoans(ctx context.Context, asOf time.Time) ([]PPAPLoanSnapshot, error)
	// UpdateCollectibility menyimpan perubahan kolektibilitas (beserta status akrual
	// turunannya) di dalam transaksi pemanggil.
	UpdateCollectibility(ctx context.Context, tx any, loanID uuid.UUID, c Collectibility) error
	// UpdateLoanState menyimpan seluruh state PPAP kredit dalam satu update.
	UpdateLoanState(ctx context.Context, tx any, update PPAPLoanUpdate) error
	// GetPPAPReserveBalance membaca saldo akun cadangan GL untuk kode COA tertentu.
	GetPPAPReserveBalance(ctx context.Context, tx any, coaCode string) (decimal.Decimal, error)
}

// PPAPService menjalankan penyisihan PPAP dan pembaruan kolektibilitas kredit.
type PPAPService interface {
	RunDaily(ctx context.Context, asOf time.Time, actor Actor) (PPAPRunSummary, error)
	Preview(ctx context.Context, asOf time.Time) (PPAPRunSummary, error)
}
