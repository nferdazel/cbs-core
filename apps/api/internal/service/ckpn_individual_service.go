package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"cbs-core/apps/core-api/internal/domain"
)

// CKPN individual TAHAP T1+T2 — mode bayangan baca-saja (keputusan panel butir 4.3).
//
// T1 menghitung nilai kini arus kas (DCF) satu kredit memakai EIR orisinal dan
// proyeksi arus kas MANUAL yang diisi bank. T2 menambah nilai realisasi bersih agunan
// (NRV = max(0, bound_amount − biaya pelepasan)) dan aturan metode MAX. Keduanya TIDAK
// menulis loans.required_ckpn dan tidak menjurnal: ckpn.individual.enabled tetap mati,
// dan hasilnya hanya dilaporkan. Data proyeksi dan biaya pelepasan adalah data
// operasional bank, bukan objek keputusan panel.
type ckpnIndividualService struct {
	loanRepo domain.LoanRepository
	repo     domain.CKPNIndividualRepository
	config   domain.SystemConfigService
}

func NewCKPNIndividualService(loanRepo domain.LoanRepository, repo domain.CKPNIndividualRepository, config domain.SystemConfigService) domain.CKPNIndividualService {
	return &ckpnIndividualService{loanRepo: loanRepo, repo: repo, config: config}
}

// Evaluate menghitung penilaian DCF satu kredit. Baca-saja: tidak menyimpan apa pun.
// Kredit yang tidak boleh diakses aktor ditolak ErrCrossBranchAccess. Kredit tanpa EIR
// orisinal dan tanpa override GAGAL dengan ErrEIRMissing — bukan dihitung nol.
func (s *ckpnIndividualService) Evaluate(ctx context.Context, loanNumber string, asOf time.Time, actor domain.Actor) (domain.CKPNIndividualAssessment, error) {
	loan, err := s.loadScopedLoan(ctx, loanNumber, actor)
	if err != nil {
		return domain.CKPNIndividualAssessment{}, err
	}
	return s.evaluateLoan(ctx, loan, asOf)
}

// EvaluateForLoan menilai kredit yang sudah dikunci pemanggil (T4). Kebijakan tetap
// dibaca dari konfigurasi; baris kredit disediakan pemanggil karena ia memegang kunci.
func (s *ckpnIndividualService) EvaluateForLoan(ctx context.Context, loan *domain.Loan, asOf time.Time) (domain.CKPNIndividualAssessment, error) {
	return s.evaluateLoan(ctx, loan, asOf)
}

func (s *ckpnIndividualService) evaluateLoan(ctx context.Context, loan *domain.Loan, asOf time.Time) (domain.CKPNIndividualAssessment, error) {
	policy := ckpnIndividualPolicy(ctx, s.config)
	asOf = asOf.UTC()

	carrying := domain.PPAPCarryingAmount(loan.OutstandingPrincipal, loan.RestructureLossBalance)
	projs, err := s.repo.ListCashflowProjections(ctx, loan.ID, asOf)
	if err != nil {
		return domain.CKPNIndividualAssessment{}, err
	}
	eir, err := domain.CKPNIndividualDiscountRateMonthly(loan.OriginalEIRMonthly, policy.DiscountRateAnnualPct)
	if err != nil {
		return domain.CKPNIndividualAssessment{}, err
	}
	pv, target, err := domain.CKPNIndividualDCF(carrying, eir, projs)
	if err != nil {
		return domain.CKPNIndividualAssessment{}, err
	}

	// T2: nilai realisasi bersih agunan aktif dan aturan metode MAX. Kredit tanpa
	// agunan tetap bekerja persis seperti T1 (FinalTarget == Target).
	cols, err := s.repo.ListActiveCollaterals(ctx, loan.ID)
	if err != nil {
		return domain.CKPNIndividualAssessment{}, err
	}
	totalNRV := decimal.Zero
	missingCost := 0
	for i := range cols {
		nrv, missing := domain.CKPNIndividualNRV(cols[i].BoundAmount, cols[i].DisposalCostAmount)
		cols[i].NRV = nrv
		cols[i].MissingDisposalCost = missing
		totalNRV = totalNRV.Add(nrv)
		if missing {
			missingCost++
		}
	}
	// Aturan MAX (12.4.g.1.c) hanya bila kredit benar-benar beragunan aktif: tanpa
	// agunan, target agunan = carrying penuh (Σ NRV = 0) dan memalsukan "yang lebih
	// konservatif". Kredit tanpa agunan memakai DCF apa adanya.
	collateralTarget := decimal.Zero
	finalTarget := target
	if len(cols) > 0 {
		collateralTarget, finalTarget = domain.CKPNIndividualCollateralTarget(carrying, target, totalNRV)
	}

	eirSource := "EIR_ORISINAL"
	if !loan.OriginalEIRMonthly.IsPositive() {
		eirSource = "OVERRIDE_KONFIG"
	}
	return domain.CKPNIndividualAssessment{
		LoanID:                   loan.ID,
		LoanNumber:               loan.LoanNumber,
		AsOf:                     asOf,
		CarryingAmount:           carrying,
		PresentValue:             pv,
		Target:                   target,
		EIRMonthly:               eir,
		EIRSource:                eirSource,
		Method:                   policy.Method,
		MandatoryTrigger:         ckpnMandatoryTrigger(loan, policy),
		Projections:              projs,
		Collaterals:              cols,
		TotalNRV:                 totalNRV,
		CollateralTarget:         collateralTarget,
		FinalTarget:              finalTarget,
		MissingDisposalCostCount: missingCost,
	}, nil
}

// SetDisposalCost menyimpan estimasi biaya pelepasan satu agunan (T2). Agunan harus
// milik kredit yang boleh diakses aktor; nilai boleh nol, tidak boleh negatif, dan
// mengosongkannya (nil) berarti NRV memakai bound_amount tanpa pengurangan.
func (s *ckpnIndividualService) SetDisposalCost(ctx context.Context, loanNumber string, collateralID uuid.UUID, cost *decimal.Decimal, actor domain.Actor) error {
	if cost != nil && cost.IsNegative() {
		return fmt.Errorf("%w: biaya pelepasan tidak boleh negatif (%s)", domain.ErrCKPNProjectionInvalid, cost)
	}
	loan, err := s.loadScopedLoan(ctx, loanNumber, actor)
	if err != nil {
		return err
	}
	// Agunan harus benar-benar milik kredit ini: menyimpan biaya untuk agunan kredit
	// lain lewat nomor kredit mana pun adalah penulisan silang yang tidak boleh terjadi.
	cols, err := s.repo.ListActiveCollaterals(ctx, loan.ID)
	if err != nil {
		return err
	}
	milik := false
	for _, c := range cols {
		if c.ID == collateralID {
			milik = true
			break
		}
	}
	if !milik {
		return fmt.Errorf("%w: agunan %s bukan milik kredit %s", domain.ErrLoanNotFound, collateralID, loanNumber)
	}
	updatedBy := actor.Username
	if updatedBy == "" {
		updatedBy = actor.UserID.String()
	}
	return s.repo.SetDisposalCost(ctx, collateralID, cost, updatedBy)
}

// ReplaceProjections memvalidasi lalu menyimpan proyeksi arus kas kredit. Validasi
// dijalankan lebih dulu sehingga masukan yang salah TIDAK tersimpan diam-diam.
func (s *ckpnIndividualService) ReplaceProjections(ctx context.Context, loanNumber string, asOf time.Time, projs []domain.CKPNCashflowProjection, actor domain.Actor) error {
	if err := domain.ValidateCKPNCashflowProjections(projs); err != nil {
		return err
	}
	loan, err := s.loadScopedLoan(ctx, loanNumber, actor)
	if err != nil {
		return err
	}
	createdBy := actor.Username
	if createdBy == "" {
		createdBy = actor.UserID.String()
	}
	return s.repo.ReplaceCashflowProjections(ctx, loan.ID, asOf.UTC(), projs, createdBy)
}

// loadScopedLoan membaca kredit dan menegakkan cakupan cabang aktor. Kredit yang tidak
// boleh diakses aktor ditolak, bukan disamarkan.
func (s *ckpnIndividualService) loadScopedLoan(ctx context.Context, loanNumber string, actor domain.Actor) (*domain.Loan, error) {
	loan, err := s.loanRepo.GetByNumber(ctx, loanNumber)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessBranch(loan.BranchCode) {
		return nil, domain.ErrCrossBranchAccess
	}
	return loan, nil
}

// ckpnMandatoryTrigger menyebut pemicu individual wajib tanpa memandang nominal
// (keputusan panel butir 4.2). Kosong berarti tidak ada pemicu wajib; kredit dapat
// tetap masuk jalur individual lewat kebijakan signifikansi bank (belum dihitung T1).
func ckpnMandatoryTrigger(loan *domain.Loan, policy domain.CKPNIndividualPolicy) string {
	switch {
	case policy.MandatoryOnMacet && loan.Collectibility == domain.CollectibilityKol5:
		return "MACET"
	case policy.MandatoryOnRestructured && loan.IsRestructured:
		return "RESTRUCTURED"
	case policy.MandatoryDPDDays > 0 && loan.DPD >= policy.MandatoryDPDDays:
		return "DPD"
	default:
		return ""
	}
}

var _ domain.CKPNIndividualService = (*ckpnIndividualService)(nil)

// ScanEntries memindai portofolio yang boleh diakses aktor (T3) dan melaporkan kredit
// yang memenuhi jalur individual beserta seluruh alasannya. Hasilnya USULAN: tidak ada
// penandaan yang berubah oleh pemindaian. Kredit aset baik yang dikeluarkan bank
// dilaporkan terpisah agar keputusan itu tetap terdokumentasi.
func (s *ckpnIndividualService) ScanEntries(ctx context.Context, actor domain.Actor) (domain.CKPNIndividualScanResult, error) {
	policy := ckpnIndividualPolicy(ctx, s.config)
	rows, err := s.repo.ListEntryScanCandidates(ctx, actor)
	if err != nil {
		return domain.CKPNIndividualScanResult{}, err
	}
	// Kredit yang sudah ditandai pengecualian (ckpn_method EXCLUDED_ASET_BAIK) sudah
	// terbawa pada baris pemindaian dan dilaporkan sebagai pengecualian.
	return domain.ScanIndividualEntries(rows, nil, policy), nil
}

// MarkLoanEntry mencatat keputusan pintu masuk pengelola untuk satu kredit (T3):
// metode individual, penanda signifikansi, penanda bukti objektif, atau penanda
// pengecualian aset baik. required_ckpn TIDAK disentuh (itu wewenang langkah CKPN EOD),
// dan keputusan ditulis ke jejak audit agar dapat dinilai ulang.
func (s *ckpnIndividualService) MarkLoanEntry(ctx context.Context, loanNumber string, method domain.CKPNIndividualMethod, significant, objectiveEvidence, excludedAsetBaik bool, actor domain.Actor) (domain.CKPNIndividualEntry, error) {
	if excludedAsetBaik {
		// Penandaan pengecualian memakai metode khusus; argumen lain diabaikan agar
		// keadaan tersimpan tidak ambigu.
		method = domain.CKPNIndividualMethodExcludedAsetBaik
		significant, objectiveEvidence = false, false
	} else if !method.Valid() || method == domain.CKPNIndividualMethodExcludedAsetBaik {
		return domain.CKPNIndividualEntry{}, fmt.Errorf("%w: metode %q tidak sah untuk penandaan individual", domain.ErrCKPNParameterInvalid, method)
	}
	loan, err := s.loadScopedLoan(ctx, loanNumber, actor)
	if err != nil {
		return domain.CKPNIndividualEntry{}, err
	}
	policy := ckpnIndividualPolicy(ctx, s.config)

	// Sinyal agunan dibaca dari data, bukan diasumsikan: saran metode menentukan
	// apakah pengelola diberi pilihan MAX atau hanya DCF.
	cols, err := s.repo.ListActiveCollaterals(ctx, loan.ID)
	if err != nil {
		return domain.CKPNIndividualEntry{}, err
	}

	// Keputusan pengelola ditinjau ulang terhadap kebijakan: penanda WAJIB mencerminkan
	// keadaan kredit sekarang, bukan klaim yang tidak diverifikasi. Bukti objektif
	// adalah penanda manual pengelola; pemicu otomatis dihitung ulang di sini agar
	// jejak audit menyebut alasan yang benar.
	check := domain.CKPNIndividualEntryCheck{
		Outstanding:       loan.OutstandingPrincipal,
		Collectibility:    loan.Collectibility,
		DPD:               loan.DPD,
		IsRestructured:    loan.IsRestructured,
		HasCollateral:     len(cols) > 0,
		ObjectiveEvidence: objectiveEvidence,
		ExcludedAsetBaik:  excludedAsetBaik,
	}
	entry := domain.EvaluateCKPNIndividualEntry(check, policy)
	if !excludedAsetBaik {
		entry.SuggestedMethod = method // keputusan pengelola menimpa saran.
		if !entry.Individual && !significant {
			return domain.CKPNIndividualEntry{}, fmt.Errorf("%w: kredit %s tidak memenuhi pemicu wajib maupun signifikansi; gunakan penanda pengecualian aset baik bila memang tidak dinilai individual", domain.ErrCKPNParameterInvalid, loanNumber)
		}
	}

	updatedBy := actor.Username
	if updatedBy == "" {
		updatedBy = actor.UserID.String()
	}
	if err := s.repo.MarkEntry(ctx, loan.ID, method, significant, objectiveEvidence, updatedBy); err != nil {
		return domain.CKPNIndividualEntry{}, err
	}
	basis, _ := json.Marshal(map[string]any{
		"keputusan":              "pintu_masuk_t3",
		"metode":                 string(method),
		"signifikan":             significant,
		"bukti_objektif":         objectiveEvidence,
		"pengecualian_aset_baik": excludedAsetBaik,
		"pemicu":                 entry.Triggers,
		"sisa_pokok":             loan.OutstandingPrincipal,
	})
	if err := s.repo.RecordAssessmentTrail(ctx, loan.ID, time.Now().UTC(), method,
		loan.OutstandingPrincipal, decimal.Zero, decimal.Zero, decimal.Zero, string(basis), updatedBy); err != nil {
		return domain.CKPNIndividualEntry{}, err
	}
	return entry, nil
}

// RecordEODTrail menulis jejak penilaian individual pada langkah CKPN EOD (T4).
// Delegasi ke repo; ada di service agar pemanggil (mesin CKPN) tidak menyentuh repo.
func (s *ckpnIndividualService) RecordEODTrail(ctx context.Context, assessment domain.CKPNIndividualAssessment, target decimal.Decimal, asOf time.Time, decidedBy string) error {
	return s.repo.RecordEODTrail(ctx, assessment, target, asOf, decidedBy)
}
