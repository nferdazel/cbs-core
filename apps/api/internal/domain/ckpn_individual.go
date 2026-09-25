package domain

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// Kerangka CKPN individual TAHAP T0 (keputusan panel butir 4).
//
// T0 sengaja TIDAK menghitung apa pun: yang disiapkan hanya konfigurasi dan primitif
// yang akan dipakai tahap berikutnya. Karena itu fungsi di berkas ini hanya pembaca
// nilai/pemilih tingkat diskonto; tidak ada satu pun yang mengubah required_ckpn.

// CKPNIndividualMethod adalah metode perhitungan target individual yang dipilih bank.
// MAX adalah bawaan: memakai yang lebih konservatif antara DCF dan agunan, sekaligus
// memenuhi aturan pemilihan PA BPR 12.4.g.1.c (CKPN agunan minimal sama dengan CKPN
// yang telah dibentuk sebelumnya).
type CKPNIndividualMethod string

const (
	CKPNIndividualMethodDCF        CKPNIndividualMethod = "DCF"
	CKPNIndividualMethodCollateral CKPNIndividualMethod = "COLLATERAL"
	CKPNIndividualMethodMax        CKPNIndividualMethod = "MAX"
)

// Valid menandai metode yang dikenal. Nilai tak dikenal ditolak, bukan diperlakukan
// sebagai MAX diam-diam: metode menentukan angka cadangan.
func (m CKPNIndividualMethod) Valid() bool {
	switch m {
	case CKPNIndividualMethodDCF, CKPNIndividualMethodCollateral, CKPNIndividualMethodMax,
		CKPNIndividualMethodExcludedAsetBaik:
		return true
	}
	return false
}

// CKPNIndividualPolicy adalah wadah konfigurasi T0 CKPN individual. Semuanya berasal
// dari system_config; tidak ada ambang yang ditanam di kode. Selama Enabled=false,
// tidak ada kredit yang dihitung individual.
type CKPNIndividualPolicy struct {
	Enabled            bool                 `json:"enabled"`
	SignificanceAmount decimal.Decimal      `json:"significance_amount"`
	SignificanceTopN   int                  `json:"significance_top_n"`
	Method             CKPNIndividualMethod `json:"method"`
	// DiscountRateAnnualPct adalah override tingkat diskonto (% per tahun) HANYA bila
	// EIR orisinal kredit belum tersimpan. Kosong berarti wajib EIR orisinal.
	DiscountRateAnnualPct string `json:"discount_rate_annual_pct"`
	// Pemicu non-nominal wajib (PA BPR 12.2.b/12.3.c), tanpa memandang nominal.
	MandatoryOnMacet             bool `json:"mandatory_on_macet"`
	MandatoryOnRestructured      bool `json:"mandatory_on_restructured"`
	MandatoryDPDDays             int  `json:"mandatory_dpd_days"`
	MandatoryOnCollateralDrop    bool `json:"mandatory_on_collateral_drop"`
	MandatoryOnObjectiveEvidence bool `json:"mandatory_on_objective_evidence"`
}

// CKPNIndividualDiscountRateMonthly memilih tingkat diskonto bulanan untuk perhitungan
// individual. Urutan: EIR orisinal tersimpan; lalu override tahunan dari konfigurasi
// dibagi 12; bila keduanya tidak ada, perhitungan GAGAL dengan ErrEIRMissing.
//
// Kredit tanpa EIR TIDAK boleh diperlakukan sebagai nol dan TIDAK boleh memakai suku
// bunga kontraktual: PA BPR 12.4.g.1.a mewajibkan suku bunga efektif orisinal, dan
// preseden jalur kerugian restrukturisasi sudah menolak dengan ErrEIRMissing.
func CKPNIndividualDiscountRateMonthly(originalEIRMonthly decimal.Decimal, overrideAnnualPct string) (decimal.Decimal, error) {
	if originalEIRMonthly.IsPositive() {
		return originalEIRMonthly, nil
	}
	if raw := strings.TrimSpace(overrideAnnualPct); raw != "" {
		annual, err := decimal.NewFromString(raw)
		if err != nil {
			return decimal.Zero, fmt.Errorf(
				"override tingkat diskonto CKPN individual %q bukan angka persen yang sah: %w",
				raw, ErrCKPNParameterInvalid)
		}
		if !annual.IsPositive() {
			return decimal.Zero, fmt.Errorf(
				"override tingkat diskonto CKPN individual %s harus lebih besar dari nol: %w",
				annual, ErrEIRMissing)
		}
		return annual.Div(decimal.NewFromInt(12)), nil
	}
	return decimal.Zero, fmt.Errorf(
		"%w: EIR orisinal bulanan kredit belum tersimpan dan override tingkat diskonto CKPN individual belum diisi; isi salah satunya, jangan hitung nol atau memakai suku bunga kontraktual",
		ErrEIRMissing)
}
