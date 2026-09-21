package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type branchService struct {
	repo      domain.BranchRepository
	auditRepo domain.AuditRepository
	// txRunner membuka satu transaksi untuk penulisan cabang dan auditnya. Lewat
	// interface agar dapat diganti stub pada test unit.
	txRunner ppapTxRunner
}

func NewBranchService(db *sql.DB, repo domain.BranchRepository, auditSinks ...domain.AuditRepository) domain.BranchService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &branchService{repo: repo, auditRepo: auditRepo, txRunner: sqlPPAPTxRunner{db: db}}
}

func (s *branchService) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	return s.repo.List(ctx)
}

// CreateBranch membuat cabang baru. Kode dinormalisasi lalu divalidasi formatnya
// (3 digit, dipakai sebagai awalan nomor rekening), nama wajib diisi, dan kode
// harus unik. Kantor pusat ditolak: ia data fondasi yang hanya boleh satu dan
// dibuat lewat migrasi, karena pemilihan kantor pusat memakai is_head_office
// sehingga cabang HO kedua akan menggeser atribusi pelaku lintas cabang.
//
// Penulisan cabang dan jejak auditnya dibungkus SATU transaksi: keberhasilan
// dicatat ke audit log, dan bila audit gagal cabang tidak pernah ada. auditRepo
// boleh nil (mis. pada test yang tidak menyiapkan audit).
func (s *branchService) CreateBranch(ctx context.Context, input domain.CreateBranchInput, actor domain.Actor) (*domain.Branch, error) {
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if err := domain.ValidateBranchCode(code); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.ErrBranchNameRequired
	}
	if input.IsHeadOffice {
		return nil, domain.ErrBranchHeadOfficeNotAllowed
	}

	// Deteksi kode ganda lebih dulu agar pesannya jelas dan tidak bergantung pada
	// pesan unique violation database.
	if existing, err := s.repo.GetByCode(ctx, code); err == nil && existing != nil {
		return nil, domain.ErrBranchCodeExists
	} else if err != nil && !errors.Is(err, domain.ErrBranchNotFound) {
		return nil, err
	}

	branch := &domain.Branch{
		ID:           uuid.New(),
		Code:         code,
		Name:         name,
		Address:      strings.TrimSpace(input.Address),
		Phone:        strings.TrimSpace(input.Phone),
		IsHeadOffice: false,
		IsActive:     true,
	}
	err := s.txRunner.Run(ctx, func(tx any) error {
		if err := s.repo.CreateTx(ctx, tx, branch); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrBranchCodeExists
			}
			return err
		}
		return writeAudit(ctx, s.auditRepo, tx, actor, "CREATE_BRANCH", "branch", branch.ID.String(), map[string]any{
			"code":           branch.Code,
			"name":           branch.Name,
			"is_head_office": branch.IsHeadOffice,
		})
	})
	if err != nil {
		return nil, err
	}
	return branch, nil
}
