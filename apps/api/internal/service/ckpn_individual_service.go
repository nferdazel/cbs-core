package service

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// CKPN individual TAHAP T1 — mode bayangan baca-saja (keputusan panel butir 4.3).
//
// T1 menghitung nilai kini arus kas (DCF) satu kredit memakai EIR orisinal dan
// proyeksi arus kas MANUAL yang diisi bank. Ia TIDAK menulis loans.required_ckpn dan
// tidak menjurnal: ckpn.individual.enabled tetap mati pada rilis T1, dan hasilnya hanya
// dilaporkan. Data proyeksi adalah data operasional bank, bukan objek keputusan panel.
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

	eirSource := "EIR_ORISINAL"
	if !loan.OriginalEIRMonthly.IsPositive() {
		eirSource = "OVERRIDE_KONFIG"
	}
	return domain.CKPNIndividualAssessment{
		LoanID:           loan.ID,
		LoanNumber:       loan.LoanNumber,
		AsOf:             asOf,
		CarryingAmount:   carrying,
		PresentValue:     pv,
		Target:           target,
		EIRMonthly:       eir,
		EIRSource:        eirSource,
		Method:           policy.Method,
		MandatoryTrigger: ckpnMandatoryTrigger(loan, policy),
		Projections:      projs,
	}, nil
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
