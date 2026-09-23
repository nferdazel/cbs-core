package service_test

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Rangka uji unit untuk pengelolaan izin: pengaman pencabutan, eksekusi saat
// disetujui, dan jejak audit sebelum->sesudah. Tanpa database.

type fakePermissionRepo struct {
	exists     map[string]bool
	perms      map[string]map[domain.Permission]bool
	members    map[string]map[uuid.UUID]bool
	userExists map[uuid.UUID]bool
	affected   int
	granted    []string
	revoked    []string
}

func newFakePermissionRepo() *fakePermissionRepo {
	return &fakePermissionRepo{
		exists:     map[string]bool{},
		perms:      map[string]map[domain.Permission]bool{},
		members:    map[string]map[uuid.UUID]bool{},
		userExists: map[uuid.UUID]bool{},
	}
}

func (f *fakePermissionRepo) ListGroups(context.Context) ([]domain.UserGroup, error) { return nil, nil }
func (f *fakePermissionRepo) ListMenus(context.Context) ([]domain.MenuDefinition, error) {
	return nil, nil
}
func (f *fakePermissionRepo) GroupExists(_ context.Context, code string) (bool, error) {
	return f.exists[code], nil
}
func (f *fakePermissionRepo) GroupHasPermission(_ context.Context, code string, p domain.Permission) (bool, error) {
	return f.perms[code][p], nil
}
func (f *fakePermissionRepo) GrantPermissionTx(_ context.Context, _ any, code string, p domain.Permission) error {
	f.granted = append(f.granted, code+"/"+string(p))
	if f.perms[code] == nil {
		f.perms[code] = map[domain.Permission]bool{}
	}
	f.perms[code][p] = true
	return nil
}
func (f *fakePermissionRepo) RevokePermissionTx(_ context.Context, _ any, code string, p domain.Permission) error {
	f.revoked = append(f.revoked, code+"/"+string(p))
	delete(f.perms[code], p)
	return nil
}
func (f *fakePermissionRepo) CountUsersLosingPermission(context.Context, string, domain.Permission) (int, error) {
	return f.affected, nil
}

func (f *fakePermissionRepo) UserExists(_ context.Context, userID uuid.UUID) (bool, error) {
	return f.userExists[userID], nil
}

func (f *fakePermissionRepo) UserInGroup(_ context.Context, groupCode string, userID uuid.UUID) (bool, error) {
	return f.members[groupCode][userID], nil
}

func (f *fakePermissionRepo) AddMemberTx(_ context.Context, _ any, groupCode string, userID uuid.UUID) error {
	if f.members[groupCode] == nil {
		f.members[groupCode] = map[uuid.UUID]bool{}
	}
	f.members[groupCode][userID] = true
	return nil
}

func (f *fakePermissionRepo) RemoveMemberTx(_ context.Context, _ any, groupCode string, userID uuid.UUID) error {
	delete(f.members[groupCode], userID)
	return nil
}

type fakeMakerChecker struct {
	created *domain.CreateMakerCheckerInput
}

func (f *fakeMakerChecker) CreateRequest(_ context.Context, in domain.CreateMakerCheckerInput, _ domain.Actor) (*domain.MakerCheckerRequest, error) {
	f.created = &in
	return &domain.MakerCheckerRequest{ID: uuid.New(), ActionType: in.ActionType, Status: domain.MakerCheckerPending}, nil
}
func (f *fakeMakerChecker) Approve(context.Context, uuid.UUID, domain.Actor, string) error {
	return nil
}
func (f *fakeMakerChecker) Reject(context.Context, uuid.UUID, domain.Actor, string) error { return nil }
func (f *fakeMakerChecker) ListPending(context.Context, domain.Actor) ([]domain.MakerCheckerRequest, error) {
	return nil, nil
}
func (f *fakeMakerChecker) Threshold(context.Context, string) decimal.Decimal {
	return decimal.Zero
}

type fakeAuditRepo struct {
	events []domain.AuditEvent
}

func (f *fakeAuditRepo) Write(_ context.Context, _ any, event domain.AuditEvent) error {
	f.events = append(f.events, event)
	return nil
}
func (f *fakeAuditRepo) List(context.Context, string, string, int) ([]domain.AuditEvent, error) {
	return nil, nil
}

func newPermissionSvc(repo *fakePermissionRepo, audit *fakeAuditRepo, mc *fakeMakerChecker) domain.PermissionService {
	return service.NewPermissionService(repo, audit, mc)
}

func actorAdmin() domain.Actor {
	return domain.Actor{UserID: uuid.New(), Username: "admin-uji", Role: domain.RoleAdmin}
}

func TestRequestChangeRejectsUnknownPermission(t *testing.T) {
	repo := newFakePermissionRepo()
	repo.exists["ROLE_TELLER"] = true
	svc := newPermissionSvc(repo, &fakeAuditRepo{}, &fakeMakerChecker{})

	_, err := svc.RequestChange(context.Background(), domain.PermissionChangeInput{
		GroupCode:  "ROLE_TELLER",
		Permission: domain.Permission("tidak:ada"),
		Operation:  domain.PermissionChangeGrant,
	}, actorAdmin())
	if !errors.Is(err, domain.ErrUnknownPermission) {
		t.Fatalf("err = %v, mau ErrUnknownPermission", err)
	}
}

func TestRequestChangeRevokeNeedsConfirmation(t *testing.T) {
	repo := newFakePermissionRepo()
	repo.exists["ROLE_TELLER"] = true
	repo.affected = 3
	mc := &fakeMakerChecker{}
	svc := newPermissionSvc(repo, &fakeAuditRepo{}, mc)

	_, err := svc.RequestChange(context.Background(), domain.PermissionChangeInput{
		GroupCode:  "ROLE_TELLER",
		Permission: domain.PermLedgerRead,
		Operation:  domain.PermissionChangeRevoke,
	}, actorAdmin())
	var loss *domain.AccessLossError
	if !errors.As(err, &loss) {
		t.Fatalf("err = %v, mau AccessLossError", err)
	}
	if loss.AffectedUsers != 3 {
		t.Fatalf("affected = %d, mau 3", loss.AffectedUsers)
	}

	// Dengan konfirmasi eksplisit, pengajuan diterima (belum diterapkan).
	if _, err := svc.RequestChange(context.Background(), domain.PermissionChangeInput{
		GroupCode:         "ROLE_TELLER",
		Permission:        domain.PermLedgerRead,
		Operation:         domain.PermissionChangeRevoke,
		ConfirmAccessLoss: true,
	}, actorAdmin()); err != nil {
		t.Fatalf("dengan konfirmasi seharusnya diterima: %v", err)
	}
	if mc.created == nil || mc.created.Payload["confirm_access_loss"] != true {
		t.Fatalf("payload pengajuan tidak memuat konfirmasi: %+v", mc.created)
	}
}

func TestExecuteApprovedGrantWritesAuditBeforeAfter(t *testing.T) {
	repo := newFakePermissionRepo()
	repo.exists["ROLE_TELLER"] = true
	audit := &fakeAuditRepo{}
	svc := newPermissionSvc(repo, audit, &fakeMakerChecker{})

	err := svc.ExecuteApproved(context.Background(), nil, service.ActionPermissionChange, map[string]any{
		"group_code": "ROLE_TELLER",
		"permission": string(domain.PermReportsExport),
		"operation":  string(domain.PermissionChangeGrant),
	}, actorAdmin())
	if err != nil {
		t.Fatalf("grant gagal: %v", err)
	}
	if len(repo.granted) != 1 {
		t.Fatalf("grant tercatat %v, mau 1", repo.granted)
	}
	if len(audit.events) != 1 {
		t.Fatalf("audit tercatat %d, mau 1", len(audit.events))
	}
	if audit.events[0].Changes["before"] != false || audit.events[0].Changes["after"] != true {
		t.Fatalf("jejak sebelum->sesudah salah: %+v", audit.events[0].Changes)
	}
}

func TestExecuteApprovedRevokeGuardRechecked(t *testing.T) {
	repo := newFakePermissionRepo()
	repo.exists["ROLE_TELLER"] = true
	repo.perms["ROLE_TELLER"] = map[domain.Permission]bool{domain.PermLedgerRead: true}
	repo.affected = 2 // keadaan berubah setelah pengajuan: kini ada yang terdampak
	svc := newPermissionSvc(repo, &fakeAuditRepo{}, &fakeMakerChecker{})

	err := svc.ExecuteApproved(context.Background(), nil, service.ActionPermissionChange, map[string]any{
		"group_code":          "ROLE_TELLER",
		"permission":          string(domain.PermLedgerRead),
		"operation":           string(domain.PermissionChangeRevoke),
		"confirm_access_loss": false,
	}, actorAdmin())
	var loss *domain.AccessLossError
	if !errors.As(err, &loss) {
		t.Fatalf("err = %v, mau AccessLossError (penjaga diperiksa ulang)", err)
	}
	if len(repo.revoked) != 0 {
		t.Fatalf("revoke tetap dijalankan padahal seharusnya ditolak: %v", repo.revoked)
	}
}
