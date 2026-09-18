package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// stubMakerCheckerRepo mengembalikan satu permintaan tetap tanpa database.
// Dipakai untuk menguji gerbang cabang pada Approve/Reject: begitu pemeriksaan
// cabang lolos, jalur process menyentuh *sql.DB sehingga tidak dapat diuji di
// sini. Karena itu test hanya membuktikan penolakan cabang dan aturan yang
// berhenti SEBELUM process (status, self-approve).
type stubMakerCheckerRepo struct {
	req *domain.MakerCheckerRequest
	err error
}

func (s *stubMakerCheckerRepo) CreateTx(ctx context.Context, tx any, req *domain.MakerCheckerRequest) error {
	return nil
}

func (s *stubMakerCheckerRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.MakerCheckerRequest, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.req, nil
}

func (s *stubMakerCheckerRepo) UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status domain.MakerCheckerStatus, checkerID, notes string) error {
	return nil
}

func (s *stubMakerCheckerRepo) ListPending(ctx context.Context, actor domain.Actor) ([]domain.MakerCheckerRequest, error) {
	return nil, nil
}

// pemeriksa cabang lain ditolak sebelum proses apa pun dijalankan.
func TestApproveRejectsCrossBranch(t *testing.T) {
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		Status:     domain.MakerCheckerPending,
		MakerID:    uuid.New().String(),
		BranchCode: "002",
	}
	svc := &makerCheckerService{repo: &stubMakerCheckerRepo{req: req}}

	actor := domain.Actor{UserID: uuid.New(), Role: domain.RoleSupervisor, BranchCode: "001"}
	err := svc.Approve(context.Background(), req.ID, actor, "")
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("harus ditolak dengan ErrCrossBranchAccess, dapat %v", err)
	}
}

func TestRejectRejectsCrossBranch(t *testing.T) {
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		Status:     domain.MakerCheckerPending,
		MakerID:    uuid.New().String(),
		BranchCode: "002",
	}
	svc := &makerCheckerService{repo: &stubMakerCheckerRepo{req: req}}

	actor := domain.Actor{UserID: uuid.New(), Role: domain.RoleSupervisor, BranchCode: "001"}
	err := svc.Reject(context.Background(), req.ID, actor, "")
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("harus ditolak dengan ErrCrossBranchAccess, dapat %v", err)
	}
}

// Aktor cabang sendiri lolos gerbang cabang. Permintaan sengaja tidak PENDING
// agar fungsi berhenti di pemeriksaan status, bukan menyentuh database.
func TestApproveSameBranchPassesBranchGate(t *testing.T) {
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		Status:     domain.MakerCheckerApproved,
		MakerID:    uuid.New().String(),
		BranchCode: "001",
	}
	svc := &makerCheckerService{repo: &stubMakerCheckerRepo{req: req}}

	actor := domain.Actor{UserID: uuid.New(), Role: domain.RoleSupervisor, BranchCode: "001"}
	err := svc.Approve(context.Background(), req.ID, actor, "")
	if !errors.Is(err, domain.ErrMakerCheckerNotPending) {
		t.Fatalf("harus lanjut ke pemeriksaan status, dapat %v", err)
	}
}

// Aktor lintas cabang juga lolos gerbang cabang.
func TestApproveCrossBranchRolePassesBranchGate(t *testing.T) {
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		Status:     domain.MakerCheckerApproved,
		MakerID:    uuid.New().String(),
		BranchCode: "002",
	}
	svc := &makerCheckerService{repo: &stubMakerCheckerRepo{req: req}}

	actor := domain.Actor{UserID: uuid.New(), Role: domain.RoleSuperAdmin, BranchCode: "999"}
	err := svc.Approve(context.Background(), req.ID, actor, "")
	if !errors.Is(err, domain.ErrMakerCheckerNotPending) {
		t.Fatalf("aktor lintas cabang harus lolos gerbang cabang, dapat %v", err)
	}
}

// Aturan lama tetap terjaga: pembuat tidak boleh menyetujui pengajuannya sendiri.
func TestApproveStillRejectsSelfApproval(t *testing.T) {
	makerID := uuid.New()
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		Status:     domain.MakerCheckerPending,
		MakerID:    makerID.String(),
		BranchCode: "001",
	}
	svc := &makerCheckerService{repo: &stubMakerCheckerRepo{req: req}}

	actor := domain.Actor{UserID: makerID, Role: domain.RoleSupervisor, BranchCode: "001"}
	err := svc.Approve(context.Background(), req.ID, actor, "")
	if !errors.Is(err, domain.ErrCannotSelfApprove) {
		t.Fatalf("harus ditolak dengan ErrCannotSelfApprove, dapat %v", err)
	}
}

// Pengajuan lama tanpa cabang (BranchCode kosong) tetap boleh diputuskan oleh
// aktor cabang mana pun, mengikuti semantik Actor.CanAccessBranch.
func TestApproveNullBranchPassesBranchGate(t *testing.T) {
	req := &domain.MakerCheckerRequest{
		ID:         uuid.New(),
		Status:     domain.MakerCheckerApproved,
		MakerID:    uuid.New().String(),
		BranchCode: "",
	}
	svc := &makerCheckerService{repo: &stubMakerCheckerRepo{req: req}}

	actor := domain.Actor{UserID: uuid.New(), Role: domain.RoleSupervisor, BranchCode: "001"}
	err := svc.Approve(context.Background(), req.ID, actor, "")
	if !errors.Is(err, domain.ErrMakerCheckerNotPending) {
		t.Fatalf("baris tanpa cabang harus lolos gerbang cabang, dapat %v", err)
	}
}
