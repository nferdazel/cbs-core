package domain

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Perlakuan akuntansi kerugian restrukturisasi kredit.
//
// Dasar dokumen (bukan ingatan):
//   - POJK No. 1 Tahun 2024 Pasal 32: "BPR wajib menerapkan perlakuan akuntansi
//     Restrukturisasi Kredit sesuai dengan standar akuntansi keuangan dan pedoman
//     akuntansi bagi BPR."
//   - SEOJK No. 21/SEOJK.03/2024 (Panduan Akuntansi Perbankan BPR, PA BPR) Bab 5.2
//     hlm. 60: "Selisih kurang antara perubahan estimasi arus kas atas Restrukturisasi
//     Kredit dibandingkan dengan nilai tercatat diperhitungkan sebagai kerugian kredit."
//     Nilai kini dihitung dengan tingkat diskonto suku bunga efektif orisinal
//     (mengacu SAK EP paragraf 11.20).
//   - PA BPR hlm. 60-61 (ilustrasi jurnal): Db. Beban kerugian penurunan nilai;
//     Kr. Kredit yang diberikan.
//   - PA BPR hlm. 144-145 (pos beban operasional butir 2): "beban kerugian
//     restrukturisasi kredit" disajikan tersendiri.
//
// Angka kerugian TIDAK boleh dihitung dengan suku bunga kontraktual: metode suku bunga
// efektif memakai arus kas nyata (termasuk provisi/biaya transaksi bila ada) dan tetap
// dipakai sebagai tingkat diskonto setelah restrukturisasi (PA BPR hlm. 41-42).

// Sentinel error modul kerugian restrukturisasi. Dipisah agar pemanggil dapat
// membedakan kegagalan perhitungan dari kegagalan konfigurasi/data.
var (
	// ErrIRRNotConverged menandai arus kas yang tidak menghasilkan suku bunga efektif:
	// seluruh arus kas sejenis (tidak ada perubahan tanda) atau akar tidak ditemukan.
	// Nilai tidak boleh diam-diam dijatuhkan menjadi nol atau suku bunga kontraktual.
	ErrIRRNotConverged = errors.New("suku bunga efektif tidak dapat dihitung: arus kas tidak konvergen")
	// ErrEIRMissing menandai kredit yang belum menyimpan suku bunga efektif orisinal.
	// Perhitungan kerugian menolak berjalan daripada memakai suku bunga kontraktual.
	ErrEIRMissing = errors.New("suku bunga efektif orisinal kredit belum tersimpan")
	// ErrRestructureLossParamInvalid menandai konfigurasi yang DIISI tetapi tidak sah.
	ErrRestructureLossParamInvalid = errors.New("konfigurasi kerugian restrukturisasi tidak valid")
	// ErrRestructureLossExpenseNotFound / ErrRestructureLossLoanAccountNotFound menandai
	// COA yang tidak dapat diresolusi; jurnal tidak boleh diposting sebagian.
	ErrRestructureLossExpenseNotFound     = errors.New("akun beban kerugian restrukturisasi tidak ditemukan")
	ErrRestructureLossLoanAccountNotFound = errors.New("akun kredit yang diberikan tidak ditemukan")
	// ErrRestructureLossIncomeAccountNotFound menandai akun pendapatan bunga untuk
	// amortisasi tidak dapat diresolusi; jurnal tidak boleh diposting sebagian.
	ErrRestructureLossIncomeAccountNotFound = errors.New("akun pendapatan bunga amortisasi saldo kerugian tidak ditemukan")
)

// RestructureLossEIRMethod adalah metode yang dicatat pada dasar audit EIR: suku bunga
// efektif dihitung dari arus kas aktual kredit (IRR periodik bulanan).
const RestructureLossEIRMethod = "IRR_ACTUAL_CASHFLOW"

// LoanCashFlow adalah satu arus kas pada periode bulan ke-N. Period 0 adalah saat
// pencairan/penilaian; Amount positif = penerimaan, negatif = pengeluaran.
type LoanCashFlow struct {
	Period int
	Amount decimal.Decimal
}

// EIRBasis menyimpan masukan perhitungan EIR agar hasilnya dapat diaudit: jumlah yang
// benar-benar cair, biaya yang dipotong (bila ada), jadwal angsuran yang dipakai, dan
// waktu perhitungan. Disimpan sebagai JSONB pada loans.original_eir_basis.
type EIRBasis struct {
	Method       string            `json:"method"`
	NetProceeds  decimal.Decimal   `json:"net_proceeds"`
	FeesDeducted decimal.Decimal   `json:"fees_deducted"`
	Installments []decimal.Decimal `json:"installments"`
	CalculatedAt time.Time         `json:"calculated_at"`
	// Note menjelaskan batas yang disadari, mis. sistem belum memotong biaya saat
	// pencairan sehingga EIR hanya berasal dari jadwal angsuran.
	Note string `json:"note,omitempty"`
}

// EffectiveMonthlyRate menghitung suku bunga efektif periodik bulanan (IRR) dari arus
// kas, dengan Newton-Raphson lalu jatuh ke bagi dua (bisection) bila Newton divergen.
// Arus kas harus memuat minimal satu nilai negatif dan satu positif; bila tidak,
// hasilnya ErrIRRNotConverged — bukan nol.
//
// Hasil dibulatkan ke 10 desimal agar sama dengan presisi kolom numeric(18,10)
// loans.original_eir_monthly; pemakaian nilai yang tersimpan tidak kehilangan presisi.
func EffectiveMonthlyRate(flows []LoanCashFlow) (decimal.Decimal, error) {
	if len(flows) < 2 {
		return decimal.Zero, fmt.Errorf("%w: arus kas kurang dari dua periode", ErrIRRNotConverged)
	}
	amounts := make([]float64, 0, len(flows))
	hasNegative, hasPositive := false, false
	for _, f := range flows {
		if f.Period < 0 {
			return decimal.Zero, fmt.Errorf("%w: periode arus kas %d negatif", ErrIRRNotConverged, f.Period)
		}
		// Period tidak diwajibkan unik/berurutan: IRR dihitung dari pasangan
		// (periode, nominal), sehingga urutan input tidak mengubah hasil.
		v := f.Amount.InexactFloat64()
		if v < 0 {
			hasNegative = true
		}
		if v > 0 {
			hasPositive = true
		}
		amounts = append(amounts, v)
	}
	if !hasNegative || !hasPositive {
		return decimal.Zero, fmt.Errorf("%w: arus kas tidak berubah tanda (semua %s)",
			ErrIRRNotConverged, signWord(hasPositive))
	}

	periods := make([]int, 0, len(flows))
	for _, f := range flows {
		periods = append(periods, f.Period)
	}

	rate, ok := irrNewton(amounts, periods)
	if !ok {
		rate, ok = irrBisect(amounts, periods)
	}
	if !ok || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return decimal.Zero, fmt.Errorf("%w: akar tidak ditemukan pada rentang suku bunga yang wajar", ErrIRRNotConverged)
	}
	return decimal.NewFromFloat(rate).Round(10), nil
}

func signWord(positive bool) string {
	if positive {
		return "positif"
	}
	return "negatif"
}

// npvFloat menghitung nilai kini bersih pada suku bunga periodik rate.
func npvFloat(rate float64, amounts []float64, periods []int) float64 {
	total := 0.0
	base := 1 + rate
	for i, a := range amounts {
		total += a / math.Pow(base, float64(periods[i]))
	}
	return total
}

// dnpvFloat adalah turunan pertama npvFloat terhadap rate, untuk Newton-Raphson.
func dnpvFloat(rate float64, amounts []float64, periods []int) float64 {
	total := 0.0
	base := 1 + rate
	for i, a := range amounts {
		k := float64(periods[i])
		total += -k * a / math.Pow(base, k+1)
	}
	return total
}

func irrNewton(amounts []float64, periods []int) (float64, bool) {
	rate := 0.05 // tebakan awal: 5% per bulan
	for i := 0; i < 100; i++ {
		f := npvFloat(rate, amounts, periods)
		df := dnpvFloat(rate, amounts, periods)
		if df == 0 || math.IsNaN(df) || math.IsInf(df, 0) {
			return rate, false
		}
		next := rate - f/df
		if math.IsNaN(next) || math.IsInf(next, 0) || next <= -0.999999 {
			return rate, false
		}
		if math.Abs(next-rate) < 1e-13 {
			return next, true
		}
		rate = next
	}
	return rate, false
}

func irrBisect(amounts []float64, periods []int) (float64, bool) {
	lo, hi := -0.9999, 1e6
	flo := npvFloat(lo, amounts, periods)
	fhi := npvFloat(hi, amounts, periods)
	if math.IsNaN(flo) || math.IsNaN(fhi) {
		return 0, false
	}
	if flo == 0 {
		return lo, true
	}
	if fhi == 0 {
		return hi, true
	}
	if flo*fhi > 0 {
		return 0, false
	}
	for i := 0; i < 300; i++ {
		mid := (lo + hi) / 2
		fm := npvFloat(mid, amounts, periods)
		if math.Abs(fm) < 1e-9 || (hi-lo) < 1e-15 {
			return mid, true
		}
		if flo*fm < 0 {
			hi = mid
			fhi = fm
		} else {
			lo = mid
			flo = fm
		}
	}
	return (lo + hi) / 2, true
}

// PresentValue menghitung nilai kini arus kas pada suku bunga efektif periodik
// bulanan. Nilai yang dikembalikan belum dibulatkan ke rupiah; pemanggil membulatkan.
func PresentValue(monthlyRate decimal.Decimal, flows []LoanCashFlow) (decimal.Decimal, error) {
	rate, _ := monthlyRate.Float64()
	if rate <= -1 {
		return decimal.Zero, fmt.Errorf("%w: tingkat diskonto %s <= -100%%",
			ErrRestructureLossParamInvalid, monthlyRate)
	}
	total := 0.0
	for _, f := range flows {
		if f.Period < 0 {
			return decimal.Zero, fmt.Errorf("%w: periode arus kas %d negatif",
				ErrRestructureLossParamInvalid, f.Period)
		}
		total += f.Amount.InexactFloat64() / math.Pow(1+rate, float64(f.Period))
	}
	return decimal.NewFromFloat(total), nil
}

// RestructureLossResult adalah hasil perhitungan kerugian restrukturisasi satu kredit.
type RestructureLossResult struct {
	// CarryingAmount adalah nilai tercatat kredit sebelum kerugian baru: sisa pokok
	// dikurangi saldo kerugian restrukturisasi yang belum diamortisasi.
	CarryingAmount decimal.Decimal
	// PresentValue adalah nilai kini arus kas baru setelah restrukturisasi, dibulatkan
	// ke rupiah.
	PresentValue decimal.Decimal
	// Loss = CarryingAmount - PresentValue. Positif berarti kerugian (jurnal beban),
	// negatif berarti keuntungan modifikasi.
	Loss decimal.Decimal
	// DiscountRateMonthly adalah tingkat diskonto yang benar-benar dipakai (fraksi).
	DiscountRateMonthly decimal.Decimal
}

// CalculateRestructureLoss menghitung kerugian restrukturisasi menurut PA BPR Bab 5.2
// hlm. 60: nilai tercatat dikurangi nilai kini arus kas baru yang didiskonto pada suku
// bunga efektif orisinal. newFlows adalah arus kas SETELAH restrukturisasi (periode
// 1..n), tanpa pencairan baru.
func CalculateRestructureLoss(carryingAmount decimal.Decimal, newFlows []LoanCashFlow, monthlyDiscount decimal.Decimal) (RestructureLossResult, error) {
	if carryingAmount.IsNegative() {
		return RestructureLossResult{}, fmt.Errorf("%w: nilai tercatat negatif %s",
			ErrRestructureLossParamInvalid, carryingAmount)
	}
	if len(newFlows) == 0 {
		return RestructureLossResult{}, fmt.Errorf("%w: arus kas baru kosong",
			ErrRestructureLossParamInvalid)
	}
	if monthlyDiscount.IsNegative() {
		return RestructureLossResult{}, fmt.Errorf("%w: tingkat diskonto negatif %s",
			ErrRestructureLossParamInvalid, monthlyDiscount)
	}
	pv, err := PresentValue(monthlyDiscount, newFlows)
	if err != nil {
		return RestructureLossResult{}, err
	}
	pvRupiah := RoundToRupiah(pv)
	return RestructureLossResult{
		CarryingAmount:      carryingAmount,
		PresentValue:        pvRupiah,
		Loss:                RoundToRupiah(carryingAmount.Sub(pvRupiah)),
		DiscountRateMonthly: monthlyDiscount,
	}, nil
}

// CashFlowsFromSchedules mengubah jadwal angsuran menjadi arus kas periode 1..n sesuai
// nomor angsuran. Hanya sisi penerimaan yang dimasukkan; ini arus kas SETELAH
// restrukturisasi untuk perhitungan nilai kini.
func CashFlowsFromSchedules(schedules []LoanSchedule) []LoanCashFlow {
	flows := make([]LoanCashFlow, 0, len(schedules))
	for _, sc := range schedules {
		flows = append(flows, LoanCashFlow{Period: sc.InstallmentNo, Amount: sc.TotalInstallment})
	}
	return flows
}

// DisbursementCashFlows menyusun arus kas untuk perhitungan EIR saat pencairan:
// periode 0 adalah kas bersih yang cair (negatif), periode 1..n adalah angsuran.
// Catatan: sistem ini belum memotong provisi/biaya apa pun pada DisburseLoan
// (lihat loan_service.DisburseLoan), sehingga netProceeds = pokok penuh dan EIR yang
// dihasilkan murni dari bentuk jadwal angsuran.
func DisbursementCashFlows(netProceeds decimal.Decimal, schedules []LoanSchedule) []LoanCashFlow {
	flows := make([]LoanCashFlow, 0, len(schedules)+1)
	flows = append(flows, LoanCashFlow{Period: 0, Amount: netProceeds.Neg()})
	return append(flows, CashFlowsFromSchedules(schedules)...)
}

// CalculateDisbursementEIR menghitung EIR orisinal dari arus kas pencairan nyata dan
// menyusun dasar auditnya. netProceeds adalah jumlah yang benar-benar cair setelah
// potongan; karena DisburseLoan belum memotong biaya, pemanggil mengirim pokok penuh.
func CalculateDisbursementEIR(netProceeds, feesDeducted decimal.Decimal, schedules []LoanSchedule) (decimal.Decimal, EIRBasis, error) {
	if len(schedules) == 0 {
		return decimal.Zero, EIRBasis{}, fmt.Errorf("%w: jadwal angsuran kosong saat pencairan", ErrIRRNotConverged)
	}
	rate, err := EffectiveMonthlyRate(DisbursementCashFlows(netProceeds, schedules))
	if err != nil {
		return decimal.Zero, EIRBasis{}, err
	}
	basis := EIRBasis{
		Method:       RestructureLossEIRMethod,
		NetProceeds:  netProceeds,
		FeesDeducted: feesDeducted,
		CalculatedAt: time.Now().UTC(),
	}
	for _, sc := range schedules {
		basis.Installments = append(basis.Installments, sc.TotalInstallment)
	}
	if feesDeducted.IsZero() {
		basis.Note = "DisburseLoan belum memotong provisi/biaya transaksi; EIR berasal dari jadwal angsuran (lihat PA BPR hlm. 42-43 soal biaya transaksi)."
	}
	return rate, basis, nil
}

// LoanEIRWriter adalah irisan opsional domain.LoanRepository untuk menyimpan EIR
// orisinal saat pencairan. Dipisah dari LoanRepository agar implementasi uji yang tidak
// membutuhkan EIR tidak wajib berubah; service memakai type assertion.
type LoanEIRWriter interface {
	UpdateOriginalEIRTx(ctx context.Context, tx any, loanID uuid.UUID, monthly decimal.Decimal, method string, basis []byte, calculatedAt time.Time) error
}

// RestructureTxWriter adalah irisan opsional domain.LoanRepository untuk menyimpan hasil
// restrukturisasi di dalam transaksi pemanggil (sehingga jurnal kerugian dan jadwal baru
// commit bersama). Pola ganti jadwal sama dengan UpdateRestructure.
type RestructureTxWriter interface {
	UpdateRestructureTx(ctx context.Context, tx any, l *Loan, schedules []LoanSchedule) error
}
