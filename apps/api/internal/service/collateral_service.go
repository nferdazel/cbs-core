package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// collateralService mengelola pencatatan agunan kredit.
//
// Fase ini sengaja terbatas: agunan dapat dicatat dan dibaca, tetapi belum mengurangi
// eksposur PPAP karena saklar ppap.collateral.enabled masih false dan pasal POJK yang
// mengatur agunan pengurang belum diverifikasi. Mencatat agunan lebih dulu tidak
// berbahaya — tidak ada angka laporan yang bergerak sampai pengurangannya dinyalakan.
type collateralService struct {
	repo      domain.CollateralRepository
	config    domain.SystemConfigService
	auditRepo domain.AuditRepository
	// branchRepo menyelesaikan cabang aktor lewat satu sumber (resolveActorBranch)
	// agar agunan pelaku lintas cabang tidak tersimpan tanpa branch_id.
	branchRepo domain.BranchRepository
}

func NewCollateralService(
	repo domain.CollateralRepository,
	config domain.SystemConfigService,
	branchRepo domain.BranchRepository,
	auditSinks ...domain.AuditRepository,
) domain.CollateralService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &collateralService{repo: repo, config: config, auditRepo: auditRepo, branchRepo: branchRepo}
}

// haircutFor memilih kebijakan haircut: permintaan operator bila diisi, kalau tidak
// kebijakan bank untuk jenis agunan tersebut dari konfigurasi.
//
// Nilai bawaan konfigurasi adalah 100 (tanpa pengurangan). Bila operator mengisi nilai
// yang tidak sah, permintaan ditolak — bukan dibulatkan diam-diam, karena haircut yang
// salah langsung mengubah besaran penyisihan bank.
func (s *collateralService) haircutFor(ctx context.Context, input domain.CollateralInput) (decimal.Decimal, error) {
	if input.HaircutPercent != nil {
		haircut := *input.HaircutPercent
		if haircut.IsNegative() || haircut.GreaterThan(decimal.NewFromInt(100)) {
			return decimal.Zero, domain.ErrCollateralHaircutInvalid
		}
		return haircut, nil
	}

	haircut := domain.DefaultHaircutPercent()
	if s.config != nil {
		raw := strings.TrimSpace(s.config.GetString(ctx, domain.HaircutConfigKey(input.CollateralType), ""))
		if raw != "" {
			parsed, err := decimal.NewFromString(raw)
			if err != nil {
				return decimal.Zero, fmt.Errorf("kebijakan haircut agunan %q tidak dapat dibaca: %w", raw, err)
			}
			if parsed.IsNegative() || parsed.GreaterThan(decimal.NewFromInt(100)) {
				return decimal.Zero, domain.ErrCollateralHaircutInvalid
			}
			haircut = parsed
		}
	}
	return haircut, nil
}

// Create mencatat satu agunan pada kredit. Cabang diambil dari aktor, bukan dari
// permintaan: agunan yang dicatat pegawai cabang tertentu harus melekat pada cabang itu,
// dan permintaan yang datang dengan cabang lain tidak dipercaya.
func (s *collateralService) Create(ctx context.Context, input domain.CollateralInput, actor domain.Actor) (*domain.LoanCollateral, error) {
	haircut, err := s.haircutFor(ctx, input)
	if err != nil {
		return nil, err
	}

	// Keberadaan dan dapat-dieksekusi adalah keadaan normal; hanya bila operator secara
	// eksplisit menyatakannya tidak, agunan kehilangan status pengurang (Pasal 21(2)).
	existsKnown := true
	if input.ExistsKnown != nil {
		existsKnown = *input.ExistsKnown
	}
	executable := true
	if input.Executable != nil {
		executable = *input.Executable
	}

	collateral := &domain.LoanCollateral{
		LoanID:               input.LoanID,
		CollateralType:       input.CollateralType,
		Description:          strings.TrimSpace(input.Description),
		DocumentNumber:       strings.TrimSpace(input.DocumentNumber),
		OwnerName:            strings.TrimSpace(input.OwnerName),
		AppraisalValue:       input.AppraisalValue,
		AppraisalDate:        input.AppraisalDate,
		Appraiser:            strings.TrimSpace(input.Appraiser),
		AppraiserIndependent: input.AppraiserIndependent,
		Certified:            input.Certified,
		Mortgaged:            input.Mortgaged,
		MortgageValue:        input.MortgageValue,
		ExistsKnown:          existsKnown,
		Executable:           executable,
		ThirdPartyOwner:      input.ThirdPartyOwner,
		OwnerConsent:         input.OwnerConsent,
		// Penanda agunan tunai dan rekeningnya disalin apa adanya: hanya operator yang
		// dapat menyatakan syarat Pasal 17 ayat (3) sudah dipenuhi. Bila agunan tunai
		// tidak dikaitkan ke rekening mana pun, penandaannya ditolak agar tidak ada
		// pengecualian PPKA umum yang tidak dapat diaudit sumber dananya.
		IsCash:                   input.IsCash,
		CashAccountID:            input.CashAccountID,
		WarehouseReceiptValuedAt: input.WarehouseReceiptValuedAt,
		// Dasar pengurang huruf d/e dan penanda kriteria penjamin BUMN/BUMD disalin apa
		// adanya: nilainya hanya boleh datang dari operator, bukan disimpulkan service.
		NJOPValue:           input.NJOPValue,
		NJOPDate:            input.NJOPDate,
		NJOPSource:          input.NJOPSource,
		BumnBumdCriteriaMet: input.BumnBumdCriteriaMet,
		BumnBumdEvidence:    strings.TrimSpace(input.BumnBumdEvidence),
		HaircutPercent:      haircut,
		Status:              domain.CollateralActive,
		Notes:               strings.TrimSpace(input.Notes),
		CreatedBy:           actor.DisplayName(),
	}
	if err := collateral.Validate(time.Now().UTC()); err != nil {
		return nil, err
	}
	// Agunan tanpa cabang tidak dapat diatribusikan ke laporan cabang mana pun.
	// Cabang diselesaikan lewat satu sumber (resolveActorBranch): pelaku lintas
	// cabang berkode 'HO' jatuh ke kantor pusat, bukan menghasilkan branch_id NULL.
	branch, err := resolveActorBranch(ctx, s.branchRepo, actor)
	if err != nil {
		return nil, fmt.Errorf("cabang aktor tidak valid: %w", err)
	}
	if branch == nil {
		return nil, fmt.Errorf("cabang aktor tidak diketahui, agunan tidak dapat diatribusikan")
	}

	if err := s.repo.Create(ctx, collateral, branch.Code); err != nil {
		return nil, err
	}

	// Audit ditulis setelah agunan tersimpan. Nilai pengurang ikut dicatat karena
	// kebijakan haircut dapat berubah: yang perlu dapat ditelusuri kemudian adalah nilai
	// yang berlaku saat agunan itu dicatat, bukan hanya persentasenya.
	if err := writeAudit(ctx, s.auditRepo, nil, actor, "CREATE_COLLATERAL", "COLLATERAL", collateral.ID.String(), map[string]any{
		"loan_id":         collateral.LoanID.String(),
		"collateral_type": string(collateral.CollateralType),
		"document_number": collateral.DocumentNumber,
		"appraisal_value": collateral.AppraisalValue.StringFixed(2),
		"haircut_percent": collateral.HaircutPercent.StringFixed(2),
		"bound_amount":    collateral.BoundAmount.StringFixed(2),
		// Dasar pengurang huruf d/e dan penanda BUMN/BUMD ikut terekam: inilah yang
		// menentukan pengurang, sehingga harus dapat ditelusuri tanpa menebak.
		"njop_value":             collateral.NJOPValue.StringFixed(2),
		"bumn_bumd_criteria_met": collateral.BumnBumdCriteriaMet,
	}); err != nil {
		return nil, err
	}
	return collateral, nil
}

// Summary merekap agunan aktif. Aktor lintas cabang menerima rekap seluruh bank; aktor
// cabang menerima rekap cabangnya saja, dan kode cabangnya diambil dari aktor — bukan dari
// parameter permintaan, supaya pegawai cabang tidak dapat meminta rekap cabang lain.
func (s *collateralService) Summary(ctx context.Context, actor domain.Actor) ([]domain.CollateralSummary, error) {
	branchCode := actor.BranchCode
	if actor.IsCrossBranch() {
		branchCode = ""
	}
	return s.repo.SummaryActive(ctx, branchCode)
}

// GetByID membaca satu agunan dengan pemeriksaan cabang.
func (s *collateralService) GetByID(ctx context.Context, id uuid.UUID, actor domain.Actor) (*domain.LoanCollateral, error) {
	collateral, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !actor.CanAccessBranch(collateral.BranchCode) {
		// Agunan cabang lain dilaporkan sebagai tidak ditemukan: membedakannya memberi tahu
		// pemanggil bahwa agunan dengan id tertentu memang ada di bank ini.
		return nil, domain.ErrCollateralNotFound
	}
	return collateral, nil
}

// ListByLoan membaca seluruh agunan satu kredit dengan pemeriksaan cabang.
func (s *collateralService) ListByLoan(ctx context.Context, loanID uuid.UUID, actor domain.Actor) ([]domain.LoanCollateral, error) {
	// Penyaringan dilakukan per baris: daftar agunan satu kredit dapat memuat agunan
	// lintas cabang, dan yang boleh terkirim hanya yang cabangnya dapat diakses aktor.
	// Hasil kosong bukan kesalahan — pemanggil sudah menyebut kreditnya sendiri.
	list, err := s.repo.ListByLoan(ctx, loanID)
	if err != nil {
		return nil, err
	}
	visible := make([]domain.LoanCollateral, 0, len(list))
	for _, item := range list {
		if actor.CanAccessBranch(item.BranchCode) {
			visible = append(visible, item)
		}
	}
	return visible, nil
}
