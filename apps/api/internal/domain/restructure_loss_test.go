package domain

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func monthlyFlows(pairs ...any) []LoanCashFlow {
	flows := make([]LoanCashFlow, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		flows = append(flows, LoanCashFlow{
			Period: pairs[i].(int),
			Amount: decimal.NewFromInt(int64(pairs[i+1].(int))),
		})
	}
	return flows
}

// EIR dari jadwal anuitas 12% per tahun harus mendekati 1% per bulan: tidak ada
// provisi/biaya yang dipotong, jadi satu-satunya sumber perbedaan adalah pembulatan
// jadwal.
func TestEffectiveMonthlyRate_AnuitasMendekatiTarifKontraktual(t *testing.T) {
	schedules, _, _ := BuildSchedule(uuid.New(), ScheduleParams{
		Principal:  decimal.NewFromInt(12_000_000),
		AnnualRate: decimal.NewFromInt(12),
		TermMonths: 12,
		StartDate:  time.Now(),
		Method:     ScheduleAnnuity,
		ProfitType: ProfitTypeInterest,
	})
	rate, err := EffectiveMonthlyRate(DisbursementCashFlows(decimal.NewFromInt(12_000_000), schedules))
	if err != nil {
		t.Fatalf("EffectiveMonthlyRate: %v", err)
	}
	if diff := rate.Sub(decimal.NewFromFloat(0.01)).Abs(); diff.GreaterThan(decimal.NewFromFloat(0.0005)) {
		t.Fatalf("EIR anuitas %s, mau dekat 0.01 (selisih %s)", rate, diff)
	}
}

// Jadwal flat (bunga atas pokok penuh) memang menghasilkan EIR lebih tinggi daripada
// tarif nominal: ini fakta matematis, bukan kesalahan. Uji ini menjaga perbedaan itu
// tidak diam-diam disamakan dengan suku bunga kontraktual.
func TestEffectiveMonthlyRate_JadwalFlatLebihTinggiDariNominal(t *testing.T) {
	schedules, _, _ := BuildSchedule(uuid.New(), ScheduleParams{
		Principal:  decimal.NewFromInt(12_000_000),
		AnnualRate: decimal.NewFromInt(12),
		TermMonths: 12,
		StartDate:  time.Now(),
		Method:     ScheduleFlat,
		ProfitType: ProfitTypeInterest,
	})
	rate, err := EffectiveMonthlyRate(DisbursementCashFlows(decimal.NewFromInt(12_000_000), schedules))
	if err != nil {
		t.Fatalf("EffectiveMonthlyRate: %v", err)
	}
	if !rate.GreaterThan(decimal.NewFromFloat(0.015)) {
		t.Fatalf("EIR flat %s harus > 1,5%% per bulan (efektif > nominal 1%%)", rate)
	}
}

// Arus kas tidak seragam (ada periode tanpa penerimaan) tetap dihitung benar:
// 1000 = 1100/(1+r)^2 -> r = sqrt(1.1)-1.
func TestEffectiveMonthlyRate_ArusKasTidakSeragam(t *testing.T) {
	flows := []LoanCashFlow{
		{Period: 0, Amount: decimal.NewFromInt(-1000)},
		{Period: 1, Amount: decimal.Zero},
		{Period: 2, Amount: decimal.NewFromInt(1100)},
	}
	rate, err := EffectiveMonthlyRate(flows)
	if err != nil {
		t.Fatalf("EffectiveMonthlyRate: %v", err)
	}
	want := math.Sqrt(1.1) - 1
	if diff := math.Abs(rate.InexactFloat64() - want); diff > 1e-9 {
		t.Fatalf("EIR %s, mau %v (selisih %v)", rate, want, diff)
	}
}

// Arus kas yang tidak berubah tanda tidak punya akar dan HARUS gagal dengan error yang
// jelas, bukan mengembalikan nol diam-diam.
func TestEffectiveMonthlyRate_TidakKonvergenDitolak(t *testing.T) {
	cases := map[string][]LoanCashFlow{
		"semua positif":  monthlyFlows(0, 100, 1, 100),
		"semua negatif":  monthlyFlows(0, -100, 1, -100),
		"kurang dua":     monthlyFlows(0, -100),
		"period negatif": {{Period: -1, Amount: decimal.NewFromInt(10)}, {Period: 0, Amount: decimal.NewFromInt(-10)}},
	}
	for name, flows := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := EffectiveMonthlyRate(flows); !errors.Is(err, ErrIRRNotConverged) {
				t.Fatalf("mau ErrIRRNotConverged, dapat %v", err)
			}
		})
	}
}

// Kerugian = nilai tercatat - nilai kini arus kas baru, keduanya ke rupiah penuh.
// 10.000.000 - (5.000.000/1,01 + 5.000.000/1,01^2) = 148.025.
func TestCalculateRestructureLoss_SelisihNilaiTercatatDanNilaiKini(t *testing.T) {
	flows := []LoanCashFlow{
		{Period: 1, Amount: decimal.NewFromInt(5_000_000)},
		{Period: 2, Amount: decimal.NewFromInt(5_000_000)},
	}
	res, err := CalculateRestructureLoss(decimal.NewFromInt(10_000_000), flows, decimal.NewFromFloat(0.01))
	if err != nil {
		t.Fatalf("CalculateRestructureLoss: %v", err)
	}
	if !res.PresentValue.Equal(decimal.NewFromInt(9_851_975)) {
		t.Fatalf("nilai kini %s, mau 9851975", res.PresentValue)
	}
	if !res.Loss.Equal(decimal.NewFromInt(148_025)) {
		t.Fatalf("kerugian %s, mau 148025", res.Loss)
	}
}

// Nilai kini lebih besar dari nilai tercatat berarti keuntungan modifikasi (Loss < 0),
// bukan kerugian; pemanggil yang memutuskan tidak mempostingnya.
func TestCalculateRestructureLoss_NilaiKiniLebihBesarMenghasilkanKeuntungan(t *testing.T) {
	flows := []LoanCashFlow{{Period: 1, Amount: decimal.NewFromInt(11_000_000)}}
	res, err := CalculateRestructureLoss(decimal.NewFromInt(10_000_000), flows, decimal.NewFromFloat(0.01))
	if err != nil {
		t.Fatalf("CalculateRestructureLoss: %v", err)
	}
	if !res.Loss.IsNegative() {
		t.Fatalf("mau keuntungan (negatif), dapat %s", res.Loss)
	}
}

// Dasar PPKA adalah pokok terutang dikurangi saldo kerugian; tidak boleh negatif.
func TestPPAPCarryingAmount_MengurangiDanBerlantaiNol(t *testing.T) {
	if got := PPAPCarryingAmount(decimal.NewFromInt(10_000_000), decimal.NewFromInt(1_500_000)); !got.Equal(decimal.NewFromInt(8_500_000)) {
		t.Fatalf("carrying %s, mau 8.500.000", got)
	}
	if got := PPAPCarryingAmount(decimal.NewFromInt(10_000_000), decimal.NewFromInt(12_000_000)); !got.IsZero() {
		t.Fatalf("carrying %s, mau 0 (berlantai nol)", got)
	}
	// Saldo negatif (data cacat) diabaikan, bukan menambah dasar.
	if got := PPAPCarryingAmount(decimal.NewFromInt(10_000_000), decimal.NewFromInt(-5)); !got.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("carrying %s, mau 10.000.000", got)
	}
}

// Saat belum ada biaya yang dipotong, jumlah cair = pokok penuh dan dasar auditnya
// mencatat FeesDeducted nol apa adanya.
func TestCalculateDisbursementEIR_MencatatDasarAuditTanpaBiaya(t *testing.T) {
	schedules, _, _ := BuildSchedule(uuid.New(), ScheduleParams{
		Principal:  decimal.NewFromInt(10_000_000),
		AnnualRate: decimal.NewFromInt(12),
		TermMonths: 12,
		StartDate:  time.Now(),
		Method:     ScheduleAnnuity,
		ProfitType: ProfitTypeInterest,
	})
	rate, basis, err := CalculateDisbursementEIR(decimal.NewFromInt(10_000_000), decimal.Zero, schedules)
	if err != nil {
		t.Fatalf("CalculateDisbursementEIR: %v", err)
	}
	if !rate.IsPositive() {
		t.Fatalf("EIR harus positif, dapat %s", rate)
	}
	if !basis.FeesDeducted.IsZero() {
		t.Fatalf("biaya dipotong %s, mau 0", basis.FeesDeducted)
	}
	if basis.Method != RestructureLossEIRMethod || len(basis.Installments) != 12 {
		t.Fatalf("dasar audit tidak lengkap: %+v", basis)
	}
	if basis.Note == "" {
		t.Fatal("catatan batas (tanpa potongan biaya) harus ada pada dasar audit")
	}
}

// CKPN harus memakai basis yang sama dengan PPKA: EAD = nilai tercatat setelah
// kerugian restrukturisasi, sehingga perbandingan CKPN vs PPKA tidak membandingkan
// dua dasar yang berbeda.
func TestCalculateCKPN_MemakaiSaldoSetelahKerugian(t *testing.T) {
	snap := CKPNLoanSnapshot{
		Status:          LoanStatusDisbursed,
		Outstanding:     decimal.NewFromInt(10_000_000),
		RestructureLoss: decimal.NewFromInt(2_000_000),
		Collectibility:  KolDPK,
		DPD:             60,
	}
	policy := CKPNPolicy{
		AsetBaikMaxDPD: CKPNAsetBaikMaxDPDDefault,
		PD:             map[Collectibility]decimal.Decimal{KolDPK: decimal.NewFromFloat(0.10)},
		PDErrors:       map[Collectibility]error{},
		LGD:            decimal.NewFromFloat(0.50),
		LGDIsSet:       true,
	}
	calc, err := CalculateCKPN(snap, policy)
	if err != nil {
		t.Fatalf("CalculateCKPN: %v", err)
	}
	// 8.000.000 x 10% x 50% = 400.000 (bukan 500.000 dari pokok bruto).
	if !calc.Target.Equal(decimal.NewFromInt(400_000)) {
		t.Fatalf("target CKPN %s, mau 400.000", calc.Target)
	}
}
