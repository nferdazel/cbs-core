package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Pelaku lintas cabang berkode 'HO' yang tidak terdaftar harus diatribusikan ke
// kantor pusat (kode nyata), bukan menghasilkan branch_id NULL pada agunan.
func TestCollateralCreate_PelakuLintasCabangDiatribusikanKeKantorPusat(t *testing.T) {
	repo := &collateralRepoStub{}
	svc := &collateralService{
		repo:       repo,
		config:     &collateralConfigStub{},
		auditRepo:  &reversalAuditRepo{},
		branchRepo: collateralBranchRepo(),
	}
	actor := domain.Actor{UserID: uuid.New(), Username: "auditor.bank", Role: domain.RoleAuditor, BranchCode: "HO"}

	if _, err := svc.Create(context.Background(), collateralInput(), actor); err != nil {
		t.Fatalf("auditor lintas cabang harus dapat mencatat agunan: %v", err)
	}
	if len(repo.branchCodes) != 1 || repo.branchCodes[0] != "001" {
		t.Fatalf("cabang agunan %v, ingin kantor pusat 001 (bukan NULL)", repo.branchCodes)
	}
}

// Penyelesaian cabang pengajuan maker-checker memakai satu sumber yang sama dengan
// resolveActorBranch: kode 'HO' jatuh ke kantor pusat, kode cabang terdaftar dipakai
// apa adanya.
func TestMakerCheckerResolveRequestBranchCode(t *testing.T) {
	svc := &makerCheckerService{branchRepo: actorBranchFixture()}

	code, err := svc.resolveRequestBranchCode(context.Background(), domain.Actor{
		Role: domain.RoleSuperAdmin, BranchCode: "HO",
	})
	if err != nil {
		t.Fatalf("superadmin lintas cabang: %v", err)
	}
	if code != "001" {
		t.Fatalf("cabang pengajuan %q, ingin kantor pusat 001 (bukan 'HO' yang menghasilkan NULL)", code)
	}

	code, err = svc.resolveRequestBranchCode(context.Background(), domain.Actor{
		Role: domain.RoleTeller, BranchCode: "001",
	})
	if err != nil {
		t.Fatalf("teller cabang terdaftar: %v", err)
	}
	if code != "001" {
		t.Fatalf("cabang pengajuan teller %q, ingin 001", code)
	}
}
