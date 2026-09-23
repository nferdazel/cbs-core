package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CKPN individual TAHAP T1: perhitungan nilai kini arus kas (DCF) dengan EIR orisinal,
// dalam MODE BAYANGAN BACA-SAJA. Fungsi di berkas ini TIDAK menulis required_ckpn dan
// tidak menjurnal: target yang dihitung hanya dilaporkan (keputusan panel butir 4.3).
//
// Masukan arus kas adalah DATA OPERASIONAL BANK (proyeksi per debitur), bukan keputusan
// panel. Bentuk paling sederhana yang tetap benar adalah daftar (periode, nominal) per
// kredit yang diisi manual dan divalidasi; tabelnya sudah ada sejak T0
// (loan_cashflow_projections, migrasi 000094).
//
// Rumus mengikuti PA BPR 12.4.g.1.a dan rancangan §2.4:
//
//	carrying  = PPAPCarryingAmount(outstanding, restructure_loss)   # domain/ppap.go
//	PV_DCF    = Σ_t CF_t / (1 + eir_monthly)^t
//	CKPN_DCF  = max(0, RoundToRupiah(carrying − PV_DCF))
//
// Kredit tanpa EIR orisinal dan tanpa override GAGAL dengan ErrEIRMissing — bukan nol
// dan bukan suku bunga kontraktual (preseden restructure_loss_service).

// ErrCKPNProjectionInvalid menandai proyeksi arus kas manual yang tidak sah
// (periode kosong/duplikat/nol, nominal negatif, atau daftar kosong).
var ErrCKPNProjectionInvalid = errors.New("proyeksi arus kas CKPN individual tidak valid")

// CKPNCashflowProjection adalah satu arus kas proyeksi: nominal pada periode ke-n
// (bulan). Nominal non-negatif; daftar minimal satu baris dan periode tidak duplikat.
type CKPNCashflowProjection struct {
	Period int             `json:"period"`
	Amount decimal.Decimal `json:"amount"`
}

// CKPNIndividualAssessment adalah hasil penilaian individual satu kredit (T1). Seluruh
// bidang BACA-SAJA: tidak ada penulisan required_ckpn di T1.
type CKPNIndividualAssessment struct {
	LoanID         uuid.UUID       `json:"loan_id"`
	LoanNumber     string          `json:"loan_number"`
	AsOf           time.Time       `json:"as_of"`
	CarryingAmount decimal.Decimal `json:"carrying_amount"`
	PresentValue   decimal.Decimal `json:"present_value"`
	// Target adalah CKPN individual DCF = max(0, carrying − present value). Angka ini
	// BAYANGAN pada T1: belum menjadi required_ckpn resmi.
	Target decimal.Decimal `json:"target"`
	// EIRMonthly adalah tingkat diskonto yang benar-benar dipakai (fraksi per bulan).
	EIRMonthly decimal.Decimal `json:"eir_monthly"`
	// EIRSource menjelaskan asal tingkat diskonto: EIR_ORISINAL atau OVERRIDE_KONFIG.
	EIRSource string `json:"eir_source"`
	// Method adalah metode yang dipakai bank (bawaan MAX); pada T1 hanya DCF tersedia.
	Method CKPNIndividualMethod `json:"method"`
	// MandatoryTrigger menyebut pemicu individual wajib, bila ada (macet,
	// restrukturisasi, atau DPD melewati ambang). Kosong berarti kredit masuk jalur
	// individual karena signifikansi nominal (belum dihitung T1) atau penilaian manual.
	MandatoryTrigger string `json:"mandatory_trigger,omitempty"`
	// Projections adalah arus kas yang dipakai, agar hasil dapat diaudit.
	Projections []CKPNCashflowProjection `json:"projections"`
}

// ValidateCKPNCashflowProjections memeriksa proyeksi manual. Periode harus >= 1 dan
// tidak duplikat, nominal tidak negatif, dan minimal satu baris. Validasi ini menjaga
// masukan operator sebelum disimpan; nilai yang gagal TIDAK disimpan diam-diam.
func ValidateCKPNCashflowProjections(projs []CKPNCashflowProjection) error {
	if len(projs) == 0 {
		return fmt.Errorf("%w: proyeksi arus kas kosong; minimal satu baris (periode, nominal)", ErrCKPNProjectionInvalid)
	}
	seen := make(map[int]bool, len(projs))
	for _, p := range projs {
		if p.Period < 1 {
			return fmt.Errorf("%w: periode %d harus >= 1 (bulan ke-n)", ErrCKPNProjectionInvalid, p.Period)
		}
		if seen[p.Period] {
			return fmt.Errorf("%w: periode %d duplikat", ErrCKPNProjectionInvalid, p.Period)
		}
		seen[p.Period] = true
		if p.Amount.IsNegative() {
			return fmt.Errorf("%w: nominal periode %d negatif (%s)", ErrCKPNProjectionInvalid, p.Period, p.Amount)
		}
	}
	return nil
}

// CKPNIndividualDCF menghitung nilai kini arus kas dan target CKPN individual (DCF)
// menurut PA BPR 12.4.g.1.a. eirMonthly adalah fraksi per bulan (bukan persen) dan
// wajib positif. Proyeksi divalidasi lebih dulu.
func CKPNIndividualDCF(carrying, eirMonthly decimal.Decimal, projs []CKPNCashflowProjection) (presentValue, target decimal.Decimal, err error) {
	if err := ValidateCKPNCashflowProjections(projs); err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	if !eirMonthly.IsPositive() {
		return decimal.Zero, decimal.Zero, fmt.Errorf(
			"%w: tingkat diskonto bulanan %s tidak positif; isi EIR orisinal atau override konfigurasi ckpn.individual.discount_rate_annual_pct",
			ErrEIRMissing, eirMonthly)
	}
	flows := make([]LoanCashFlow, 0, len(projs))
	for _, p := range projs {
		flows = append(flows, LoanCashFlow{Period: p.Period, Amount: p.Amount})
	}
	pv, err := PresentValue(eirMonthly, flows)
	if err != nil {
		return decimal.Zero, decimal.Zero, err
	}
	pvRupiah := RoundToRupiah(pv)
	ckpn := RoundToRupiah(carrying.Sub(pvRupiah))
	if ckpn.IsNegative() {
		ckpn = decimal.Zero
	}
	return pvRupiah, ckpn, nil
}

// CKPNIndividualRepository menyimpan/membaca arus kas proyeksi manual per kredit (T1).
// Ia tidak menyentuh required_ckpn: T1 hanya membaca dan mencatat data operasional.
type CKPNIndividualRepository interface {
	// ListCashflowProjections mengambil proyeksi pada tanggal asOf, terurut periode.
	ListCashflowProjections(ctx context.Context, loanID uuid.UUID, asOf time.Time) ([]CKPNCashflowProjection, error)
	// ReplaceCashflowProjections mengganti seluruh proyeksi kredit pada tanggal asOf
	// dalam satu transaksi, sehingga tidak ada campuran versi lama dan baru.
	ReplaceCashflowProjections(ctx context.Context, loanID uuid.UUID, asOf time.Time, projs []CKPNCashflowProjection, createdBy string) error
}

// CKPNIndividualService adalah T1: menilai satu kredit dengan DCF memakai proyeksi
// manual, dan menyimpan proyeksi itu. Seluruhnya BAYANGAN: tidak menulis required_ckpn.
type CKPNIndividualService interface {
	// Evaluate menghitung penilaian DCF satu kredit. Baca-saja.
	Evaluate(ctx context.Context, loanNumber string, asOf time.Time, actor Actor) (CKPNIndividualAssessment, error)
	// ReplaceProjections memvalidasi lalu menyimpan proyeksi arus kas kredit.
	ReplaceProjections(ctx context.Context, loanNumber string, asOf time.Time, projs []CKPNCashflowProjection, actor Actor) error
}
