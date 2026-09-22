package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// GetByID melengkapi stub cabang untuk jalur pemindahan unit (W14).
func (s *stubBranchRepo) GetByID(_ context.Context, id uuid.UUID) (*domain.Branch, error) {
	for _, b := range s.branches {
		if b.ID == id {
			return b, nil
		}
	}
	return nil, domain.ErrBranchNotFound
}

func (s *stubBranchRepo) SetParentTx(_ context.Context, _ any, id uuid.UUID, parentID *uuid.UUID) error {
	for _, b := range s.branches {
		if b.ID == id {
			b.ParentID = parentID
			return nil
		}
	}
	return domain.ErrBranchNotFound
}

// scopeImpact membuat stub dapat mensimulasikan dampak cakupan tanpa database.
func (s *stubBranchRepo) ScopeImpactUsers(_ context.Context, _ uuid.UUID, _, _ *uuid.UUID) (int, int, error) {
	return s.scopeLosing, s.scopeGaining, nil
}

func orgUnitFixture() (domain.BranchService, *stubBranchRepo, *stubAuditRepo) {
	areaID := uuid.New()
	repo := &stubBranchRepo{branches: map[string]*domain.Branch{
		"001": {ID: uuid.New(), Code: "001", Name: "Kantor Pusat", IsHeadOffice: true, IsActive: true, UnitLevel: domain.UnitLevelBranch},
		"810": {ID: areaID, Code: "810", Name: "Area Barat", IsActive: true, UnitLevel: domain.UnitLevelArea},
		"811": {ID: uuid.New(), Code: "811", Name: "Cabang 811", IsActive: true, UnitLevel: domain.UnitLevelBranch, ParentID: &areaID},
	}}
	audit := &stubAuditRepo{}
	return &branchService{repo: repo, auditRepo: audit, txRunner: stubTxRunner{}}, repo, audit
}

// Jenjang di luar CABANG/AREA/WILAYAH ditolak sebelum menyentuh database.
func TestCreateOrgUnit_JenjangTidakDikenal(t *testing.T) {
	svc, _, _ := orgUnitFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}
	if _, err := svc.CreateOrgUnit(context.Background(), domain.CreateOrgUnitInput{
		Code: "810", Name: "X", Level: domain.OrgUnitLevel("DIVISI"),
	}, actor); !errors.Is(err, domain.ErrOrgUnitLevelInvalid) {
		t.Fatalf("err = %v, ingin ErrOrgUnitLevelInvalid", err)
	}
}

// Kode area/wilayah tidak boleh mengandung pemisah cakupan (koma) atau spasi,
// karena kode digabung ke cakupan aktor dan harus tetap dapat dipisah kembali.
func TestCreateOrgUnit_KodeAreaAman(t *testing.T) {
	svc, _, _ := orgUnitFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}
	for _, code := range []string{"", " ", "AREA,1", "AREA 1", "AREA/1", "AREA-1-YANG-SANGAT-PANJANG"} {
		if _, err := svc.CreateOrgUnit(context.Background(), domain.CreateOrgUnitInput{
			Code: code, Name: "Area Uji", Level: domain.UnitLevelArea,
		}, actor); !errors.Is(err, domain.ErrInvalidBranchCode) {
			t.Fatalf("kode %q: err = %v, ingin ErrInvalidBranchCode", code, err)
		}
	}
}

// Atasan wajib berjenjang lebih tinggi: cabang tidak boleh membawahi area, dan
// area tidak boleh membawahi wilayah.
func TestCreateOrgUnit_AtasanHarusLebihTinggi(t *testing.T) {
	svc, _, _ := orgUnitFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}
	if _, err := svc.CreateOrgUnit(context.Background(), domain.CreateOrgUnitInput{
		Code: "820", Name: "Area Baru", Level: domain.UnitLevelArea, ParentCode: "811",
	}, actor); !errors.Is(err, domain.ErrOrgUnitParentInvalid) {
		t.Fatalf("area di bawah cabang: err = %v, ingin ErrOrgUnitParentInvalid", err)
	}
}

// Area tanpa atasan sah (bank boleh memakai hanya satu jenjang) dan tercatat di audit.
func TestCreateOrgUnit_AreaTanpaAtasanSukses(t *testing.T) {
	svc, repo, audit := orgUnitFixture()
	actor := domain.Actor{Username: "super.uji", Role: domain.RoleSuperAdmin}
	if _, err := svc.CreateOrgUnit(context.Background(), domain.CreateOrgUnitInput{
		Code: " wilayah, dilarang ", Name: "Wilayah Timur", Level: domain.UnitLevelRegion,
	}, actor); err == nil {
		t.Fatal("kode dengan koma harus ditolak")
	}
	unit, err := svc.CreateOrgUnit(context.Background(), domain.CreateOrgUnitInput{
		Code: " wilayah-1 ", Name: "Wilayah Timur", Level: domain.UnitLevelRegion,
	}, actor)
	if err != nil {
		t.Fatalf("CreateOrgUnit: %v", err)
	}
	if unit.Code != "WILAYAH-1" || unit.UnitLevel != domain.UnitLevelRegion || unit.ParentID != nil {
		t.Fatalf("unit tersimpan %+v", unit)
	}
	if _, ok := repo.branches["WILAYAH-1"]; !ok {
		t.Fatal("unit tidak tersimpan di repository")
	}
	if len(audit.events) != 1 || audit.events[0].Action != "CREATE_ORG_UNIT" {
		t.Fatalf("audit = %+v, ingin satu event CREATE_ORG_UNIT", audit.events)
	}
}

// Cabang lewat jalur unit tetap wajib 3 digit (dipakai awalan nomor rekening).
func TestCreateOrgUnit_CabangTetapTigaDigit(t *testing.T) {
	svc, _, _ := orgUnitFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}
	if _, err := svc.CreateOrgUnit(context.Background(), domain.CreateOrgUnitInput{
		Code: "CABANG-BARAT", Name: "Cabang", Level: domain.UnitLevelBranch,
	}, actor); !errors.Is(err, domain.ErrInvalidBranchCode) {
		t.Fatalf("err = %v, ingin ErrInvalidBranchCode", err)
	}
}

// Unit tidak boleh menjadi atasannya sendiri. Karena jenjang mengikat menurun,
// inilah bentuk siklus yang mungkin; pindah ke diri sendiri ditolak.
func TestSetOrgUnitParent_MenolakDiriSendiri(t *testing.T) {
	svc, _, _ := orgUnitFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}
	if _, err := svc.SetOrgUnitParent(context.Background(), "810", domain.SetOrgUnitParentInput{ParentCode: "810"}, actor); !errors.Is(err, domain.ErrOrgUnitParentInvalid) {
		t.Fatalf("err = %v, ingin ErrOrgUnitParentInvalid", err)
	}
}

// Pemindahan yang sah mengubah parent_id dan tercatat di audit.
func TestSetOrgUnitParent_Sukses(t *testing.T) {
	svc, repo, audit := orgUnitFixture()
	actor := domain.Actor{Username: "super.uji", Role: domain.RoleSuperAdmin}
	// 811 (cabang) semula di bawah area 810: lepas menjadi puncak.
	unit, err := svc.SetOrgUnitParent(context.Background(), "811", domain.SetOrgUnitParentInput{}, actor)
	if err != nil {
		t.Fatalf("SetOrgUnitParent: %v", err)
	}
	if unit.ParentID != nil {
		t.Fatalf("parent_id = %v, ingin nil setelah dilepas ke puncak", unit.ParentID)
	}
	if repo.branches["811"].ParentID != nil {
		t.Fatal("repository belum memperbarui parent_id")
	}
	if len(audit.events) != 1 || audit.events[0].Action != "SET_ORG_UNIT_PARENT" {
		t.Fatalf("audit = %+v, ingin satu event SET_ORG_UNIT_PARENT", audit.events)
	}
}

// Pemindahan yang mengubah cakupan pengguna aktif ditolak tanpa konfirmasi, dan
// diterima setelah dikonfirmasi. Unit target sendiri tidak pernah kehilangan
// cakupan, sehingga pengaman ini tidak mengunci pengguna.
func TestSetOrgUnitParent_KonfirmasiDampakCakupan(t *testing.T) {
	svc, repo, audit := orgUnitFixture()
	repo.scopeLosing = 2
	repo.scopeGaining = 1
	actor := domain.Actor{Username: "super.uji", Role: domain.RoleSuperAdmin}

	_, err := svc.SetOrgUnitParent(context.Background(), "811", domain.SetOrgUnitParentInput{}, actor)
	var scope *domain.ScopeChangeError
	if !errors.As(err, &scope) {
		t.Fatalf("err = %v, ingin ScopeChangeError", err)
	}
	if scope.Losing != 2 || scope.Gaining != 1 {
		t.Fatalf("dampak = kehilangan %d/mendapat %d, ingin 2/1", scope.Losing, scope.Gaining)
	}
	if repo.branches["811"].ParentID == nil {
		t.Fatal("pemindahan ditolak tetapi parent sudah berubah")
	}
	if len(audit.events) != 0 {
		t.Fatalf("tidak boleh ada audit saat ditolak: %+v", audit.events)
	}

	unit, err := svc.SetOrgUnitParent(context.Background(), "811", domain.SetOrgUnitParentInput{ConfirmScopeChange: true}, actor)
	if err != nil {
		t.Fatalf("dengan konfirmasi seharusnya diterima: %v", err)
	}
	if unit.ParentID != nil {
		t.Fatalf("parent_id = %v, ingin nil", unit.ParentID)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "SET_ORG_UNIT_PARENT" {
		t.Fatalf("audit = %+v, ingin satu SET_ORG_UNIT_PARENT", audit.events)
	}
}
