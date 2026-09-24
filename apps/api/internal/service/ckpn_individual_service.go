package service

import (
	"context"
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
	collateralTarget, finalTarget := domain.CKPNIndividualCollateralTarget(carrying, target, totalNRV)

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
