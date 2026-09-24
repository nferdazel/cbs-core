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
	// Rincian agunan (T2): NRV per agunan dan target agunan. Kosong bila kredit tidak
	// beragunan. FinalTarget = max(target DCF, target agunan) sesuai metode MAX;
	// Target tetap menyimpan angka DCF agar kedua komponen dapat diaudit terpisah.
	Collaterals []CKPNIndividualCollateral `json:"collaterals,omitempty"`
	// TotalNRV adalah jumlah NRV seluruh agunan aktif; nol bila tidak beragunan.
	TotalNRV decimal.Decimal `json:"total_nrv"`
	// CollateralTarget = max(0, carrying − TotalNRV). Bila lebih besar dari Target
	// (DCF), maka FinalTarget mengikuti agunan sesuai aturan MAX.
	CollateralTarget decimal.Decimal `json:"collateral_target"`
	// FinalTarget adalah target individual akhir menurut metode MAX. Pada T1
	// (= tanpa agunan) FinalTarget == Target.
	FinalTarget decimal.Decimal `json:"final_target"`
	// MissingDisposalCost menghitung agunan yang biaya pelepasannya belum diisi bank:
	// NRV agunan itu memakai bound_amount penuh (konservatif) dan angkanya wajib
	// dibaca dengan tanda itu, bukan dianggap final.
	MissingDisposalCostCount int `json:"missing_disposal_cost_count"`
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

// CKPNIndividualCollateral adalah satu agunan aktif pada penilaian individual T2.
// Seluruh angka BACA-SAJA; penilaian tidak pernah mengubah agunan.
type CKPNIndividualCollateral struct {
	ID             uuid.UUID      `json:"id"`
	CollateralType CollateralType `json:"collateral_type"`
	Description    string         `json:"description"`
	// AppraisalValue adalah nilai taksasi penuh; HaircutPercent adalah kebijakan
	// pengurang PPKA (bukan biaya penjualan). BoundAmount = taksasi × (1−haircut),
	// dihitung basis data agar selalu konsisten dengan keduanya.
	AppraisalValue decimal.Decimal `json:"appraisal_value"`
	HaircutPercent decimal.Decimal `json:"haircut_percent"`
	BoundAmount    decimal.Decimal `json:"bound_amount"`
	// DisposalCostAmount adalah estimasi biaya pelepasan (penjualan/lelang, pajak,
	// notaris). KOSONG berarti bank belum mengisinya: NRV memakai bound_amount tanpa
	// pengurangan dan penilaian menandai hal itu, bukan menebak.
	DisposalCostAmount *decimal.Decimal `json:"disposal_cost_amount,omitempty"`
	// NRV (net realisable value) agunan ini = max(0, bound_amount − disposal_cost).
	// Dibatasi nol agar biaya yang lebih besar daripada nilai tidak menghasilkan
	// nilai negatif (agunan tidak pernah "menambah" kerugian kredit).
	NRV decimal.Decimal `json:"nrv"`
	// MissingDisposalCost bernilai true bila bank belum mengisi biaya pelepasan:
	// NRV dihitung tanpa pengurangan, dan itu harus terbuka pada penilaian agar
	// pengguna tahu angkanya konservatif karena data belum lengkap.
	MissingDisposalCost bool `json:"missing_disposal_cost"`
}

// CKPNIndividualNRV menghitung nilai realisasi bersih satu agunan menurut PA BPR
// 12.4.g.1.b: NRV = max(0, bound_amount − biaya pelepasan). BoundAmount dipakai apa
// adanya (sudah = taksasi × (1−haircut), dihitung basis data), dan biaya pelepasan
// kosong berarti tanpa pengurangan — bukan nol yang dikarang.
func CKPNIndividualNRV(boundAmount decimal.Decimal, disposalCost *decimal.Decimal) (nrv decimal.Decimal, missingCost bool) {
	if disposalCost == nil {
		missingCost = true
	} else {
		boundAmount = boundAmount.Sub(*disposalCost)
	}
	if boundAmount.IsNegative() {
		boundAmount = decimal.Zero
	}
	return boundAmount, missingCost
}

// CKPNIndividualCollateralTarget menerapkan aturan metode MAX (PA BPR 12.4.g.1.c):
// CKPN individual = max(target DCF, target agunan). Target agunan =
// max(0, carrying − Σ NRV). Fungsi ini MURNI: tidak menyentuh basis data.
func CKPNIndividualCollateralTarget(carrying, dcfTarget decimal.Decimal, totalNRV decimal.Decimal) (collateralTarget, finalTarget decimal.Decimal) {
	collateralTarget = carrying.Sub(totalNRV)
	if collateralTarget.IsNegative() {
		collateralTarget = decimal.Zero
	}
	finalTarget = dcfTarget
	if collateralTarget.GreaterThan(finalTarget) {
		finalTarget = collateralTarget
	}
	return collateralTarget, finalTarget
}

type CKPNIndividualRepository interface {
	// ListActiveCollaterals mengembalikan agunan AKTIF kredit (T2) beserta taksasi,
	// haircut, bound_amount, dan biaya pelepasan. Hanya ACTIVE: agunan lepas/eksekusi
	// tidak menjamin apa pun untuk NRV.
	ListActiveCollaterals(ctx context.Context, loanID uuid.UUID) ([]CKPNIndividualCollateral, error)
	// SetDisposalCost menyimpan estimasi biaya pelepasan satu agunan (T2). Nilai boleh
	// nol (tanpa biaya) tetapi tidak boleh negatif; mengubah data operasional bank.
	SetDisposalCost(ctx context.Context, collateralID uuid.UUID, cost *decimal.Decimal, updatedBy string) error
	// ListCashflowProjections mengambil proyeksi pada tanggal asOf, terurut periode.
	ListCashflowProjections(ctx context.Context, loanID uuid.UUID, asOf time.Time) ([]CKPNCashflowProjection, error)
	// ReplaceCashflowProjections mengganti seluruh proyeksi kredit pada tanggal asOf
	// dalam satu transaksi, sehingga tidak ada campuran versi lama dan baru.
	ReplaceCashflowProjections(ctx context.Context, loanID uuid.UUID, asOf time.Time, projs []CKPNCashflowProjection, createdBy string) error

	// T3: pintu masuk jalur individual.
	// ListEntryScanCandidates membaca kredit aktif yang boleh diakses aktor untuk
	// pemindaian pintu masuk, terurut sisa pokok terbesar dahulu.
	ListEntryScanCandidates(ctx context.Context, actor Actor) ([]CKPNIndividualScanRow, error)
	// MarkEntry menandai keputusan pintu masuk pada satu kredit; required_ckpn tidak
	// disentuh. RecordAssessmentTrail menulis jejak audit penilaian.
	MarkEntry(ctx context.Context, loanID uuid.UUID, method CKPNIndividualMethod, significant, objectiveEvidence bool, updatedBy string) error
	RecordAssessmentTrail(ctx context.Context, loanID uuid.UUID, asOf time.Time, method CKPNIndividualMethod, carrying, pv, nrv, target decimal.Decimal, basis, decidedBy string) error
	// RecordEODTrail menulis jejak penilaian individual pada langkah CKPN EOD (T4).
	RecordEODTrail(ctx context.Context, assessment CKPNIndividualAssessment, target decimal.Decimal, asOf time.Time, decidedBy string) error
}

// CKPNIndividualService adalah T1: menilai satu kredit dengan DCF memakai proyeksi
// manual, dan menyimpan proyeksi itu. Seluruhnya BAYANGAN: tidak menulis required_ckpn.
type CKPNIndividualService interface {
	// Evaluate menghitung penilaian DCF satu kredit. Baca-saja.
	Evaluate(ctx context.Context, loanNumber string, asOf time.Time, actor Actor) (CKPNIndividualAssessment, error)
	// EvaluateForLoan sama dengan Evaluate tetapi menerima baris kredit yang SUDAH
	// dikunci pemanggil (T4: langkah CKPN EOD mengunci kredit sebelum menilai).
	// Melanjutkan kredit yang basi pada jalur resmi akan menghasilkan angka salah.
	EvaluateForLoan(ctx context.Context, loan *Loan, asOf time.Time) (CKPNIndividualAssessment, error)
	// ReplaceProjections memvalidasi lalu menyimpan proyeksi arus kas kredit.
	ReplaceProjections(ctx context.Context, loanNumber string, asOf time.Time, projs []CKPNCashflowProjection, actor Actor) error
	// SetDisposalCost menyimpan estimasi biaya pelepasan satu agunan milik kredit
	// (T2). Nilai nol sah; nil berarti belum diisi (NRV tanpa pengurangan).
	SetDisposalCost(ctx context.Context, loanNumber string, collateralID uuid.UUID, cost *decimal.Decimal, actor Actor) error

	// T3: ScanEntries memindai portofolio yang boleh diakses aktor dan melaporkan
	// kredit yang memenuhi jalur individual beserta alasannya (usulan, bukan
	// penandaan). MarkLoanEntry mencatat keputusan pengelola pada satu kredit dan
	// menulis jejak auditnya; required_ckpn tidak pernah disentuh.
	ScanEntries(ctx context.Context, actor Actor) (CKPNIndividualScanResult, error)
	// RecordEODTrail menulis jejak penilaian individual pada langkah CKPN EOD (T4)
	// setelah target resmi ditetapkan. Gagal menulis jejak menggagalkan run: jejak
	// audit bukan pelengkap yang boleh hilang.
	RecordEODTrail(ctx context.Context, assessment CKPNIndividualAssessment, target decimal.Decimal, asOf time.Time, decidedBy string) error
	MarkLoanEntry(ctx context.Context, loanNumber string, method CKPNIndividualMethod, significant, objectiveEvidence, excludedAsetBaik bool, actor Actor) (CKPNIndividualEntry, error)
}

// CKPNIndividualScanResult dipakai lintas lapisan (repo → service → handler); didefinisikan
// ulang sebagai alias agar deklarasi di ckpn_individual_t3.go tetap sumber kebenaran.
