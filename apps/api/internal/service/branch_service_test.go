package service

import (
	"context"
	"errors"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
)

// stubBranchRepo menyimpan cabang dari peta kode dan meniru penolakan unique
// constraint database agar jalur penerjemahan error ke sentinel bisnis teruji.
type stubBranchRepo struct {
	domain.BranchRepository
	branches map[string]*domain.Branch
	lastTx   any
}

func (s *stubBranchRepo) GetByCode(_ context.Context, code string) (*domain.Branch, error) {
	if b, ok := s.branches[code]; ok {
		return b, nil
	}
	return nil, domain.ErrBranchNotFound
}

func (s *stubBranchRepo) Create(_ context.Context, b *domain.Branch) error {
	return s.CreateTx(context.Background(), nil, b)
}

func (s *stubBranchRepo) CreateTx(_ context.Context, tx any, b *domain.Branch) error {
	s.lastTx = tx
	if _, ok := s.branches[b.Code]; ok {
		return errors.New("ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)")
	}
	s.branches[b.Code] = b
	return nil
}

func branchFixture() (domain.BranchService, *stubBranchRepo, *stubAuditRepo) {
	repo := &stubBranchRepo{branches: map[string]*domain.Branch{
		"001": {Code: "001", Name: "Kantor Pusat", IsHeadOffice: true, IsActive: true},
	}}
	audit := &stubAuditRepo{}
	return &branchService{repo: repo, auditRepo: audit, txRunner: stubTxRunner{}}, repo, audit
}

// Kode cabang dipakai sebagai 3 digit pertama nomor rekening, jadi formatnya
// ditegakkan: tepat 3 angka. Kode non-angka seperti 'HO' bukan cabang operasional
// dan harus ditolak sebelum menyentuh database.
func TestBranchServiceCreateBranch_ValidasiKode(t *testing.T) {
	svc, _, _ := branchFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin, BranchCode: "HO"}

	for _, code := range []string{"HO", "12", "1234", "12A", ""} {
		if _, err := svc.CreateBranch(context.Background(), domain.CreateBranchInput{Code: code, Name: "Cabang X"}, actor); !errors.Is(err, domain.ErrInvalidBranchCode) {
			t.Fatalf("kode %q: err = %v, ingin ErrInvalidBranchCode", code, err)
		}
	}
}

// Nama wajib diisi: cabang tanpa nama tidak dapat dikenali pada laporan maupun
// surat konfirmasi.
func TestBranchServiceCreateBranch_NamaWajib(t *testing.T) {
	svc, _, _ := branchFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}

	if _, err := svc.CreateBranch(context.Background(), domain.CreateBranchInput{Code: "002", Name: "   "}, actor); !errors.Is(err, domain.ErrBranchNameRequired) {
		t.Fatalf("err = %v, ingin ErrBranchNameRequired", err)
	}
}

// Kode unik: kode yang sudah dipakai ditolak dengan sentinel yang dapat dipetakan
// ke HTTP 409, bukan diteruskan sebagai galat internal.
func TestBranchServiceCreateBranch_KodeUnik(t *testing.T) {
	svc, _, _ := branchFixture()
	actor := domain.Actor{Role: domain.RoleSuperAdmin}

	if _, err := svc.CreateBranch(context.Background(), domain.CreateBranchInput{Code: "001", Name: "Duplikat"}, actor); !errors.Is(err, domain.ErrBranchCodeExists) {
		t.Fatalf("err = %v, ingin ErrBranchCodeExists", err)
	}
}

// Pembuatan yang sah menyimpan cabang aktif dan meninggalkan jejak audit aksi
// administratif tersebut.
func TestBranchServiceCreateBranch_SuksesDanAudit(t *testing.T) {
	svc, repo, audit := branchFixture()
	actor := domain.Actor{Username: "super.uji", Role: domain.RoleSuperAdmin}

	branch, err := svc.CreateBranch(context.Background(), domain.CreateBranchInput{
		Code: " 002 ", Name: " Cabang Kedua ", Address: "Jl. Uji 1", Phone: "021-000",
	}, actor)
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if branch.Code != "002" || branch.Name != "Cabang Kedua" {
		t.Fatalf("cabang tersimpan %q/%q, ingin 002/Cabang Kedua", branch.Code, branch.Name)
	}
	if !branch.IsActive {
		t.Fatal("cabang baru harus aktif")
	}
	if branch.IsHeadOffice {
		t.Fatal("cabang biasa yang dibuat lewat API tidak boleh mengaku kantor pusat")
	}
	if _, ok := repo.branches["002"]; !ok {
		t.Fatal("cabang tidak tersimpan di repository")
	}
	if len(audit.events) != 1 || audit.events[0].Action != "CREATE_BRANCH" {
		t.Fatalf("audit = %+v, ingin satu event CREATE_BRANCH", audit.events)
	}
}

// Izin pembuatan cabang adalah administratif kantor pusat: SUPERADMIN saja.
// Peran lain, termasuk ADMIN yang bercabang, tidak boleh menambah cabang.
func TestPermBranchesCreateHanyaSuperadmin(t *testing.T) {
	if !domain.RoleSuperAdmin.HasPermission(domain.PermBranchesCreate) {
		t.Fatal("SUPERADMIN harus memegang branches:create")
	}
	for _, role := range []domain.StaffRole{domain.RoleAdmin, domain.RoleSupervisor, domain.RoleTeller, domain.RoleCS, domain.RoleAO, domain.RoleAuditor} {
		if role.HasPermission(domain.PermBranchesCreate) {
			t.Fatalf("peran %s tidak boleh memegang branches:create", role)
		}
	}
}

// recordingTxRunner menandai bahwa service membuka transaksi dan meneruskan handle
// transaksi yang sama ke seluruh penulisan.
type recordingTxRunner struct {
	tx    any
	calls int
}

func (r *recordingTxRunner) Run(_ context.Context, fn func(tx any) error) error {
	r.calls++
	return fn(r.tx)
}

type txAwareAuditRepo struct {
	stubAuditRepo
	lastTx any
}

func (a *txAwareAuditRepo) Write(_ context.Context, tx any, event domain.AuditEvent) error {
	a.lastTx = tx
	a.events = append(a.events, event)
	return nil
}

// Penulisan cabang dan audit harus berada pada SATU transaksi: bila audit gagal,
// cabang tidak boleh terlanjur tersimpan dan permintaan mengembalikan galat.
func TestBranchServiceCreateBranch_MemakaiSatuTransaksi(t *testing.T) {
	repo := &stubBranchRepo{branches: map[string]*domain.Branch{}}
	audit := &txAwareAuditRepo{}
	runner := &recordingTxRunner{tx: "TX-CABANG"}
	svc := &branchService{repo: repo, auditRepo: audit, txRunner: runner}
	actor := domain.Actor{Username: "super.uji", Role: domain.RoleSuperAdmin}

	if _, err := svc.CreateBranch(context.Background(), domain.CreateBranchInput{Code: "002", Name: "Cabang Kedua"}, actor); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if runner.calls != 1 {
		t.Fatalf("transaksi dibuka %d kali, ingin tepat 1", runner.calls)
	}
	if repo.lastTx != runner.tx {
		t.Fatalf("repo menulis di luar transaksi: %v", repo.lastTx)
	}
	if audit.lastTx != runner.tx {
		t.Fatalf("audit menulis di luar transaksi: %v", audit.lastTx)
	}
}

// Kantor pusat adalah data fondasi: pembuatan lewat API harus ditolak dan tidak
// boleh menyimpan baris apa pun, karena pemilihan kantor pusat memakai
// is_head_office sehingga HO kedua akan menggeser atribusi pelaku lintas cabang.
func TestBranchServiceCreateBranch_MenolakKantorPusat(t *testing.T) {
	svc, repo, audit := branchFixture()
	actor := domain.Actor{Username: "super.uji", Role: domain.RoleSuperAdmin}

	_, err := svc.CreateBranch(context.Background(), domain.CreateBranchInput{
		Code: "002", Name: "Kantor Pusat Kedua", IsHeadOffice: true,
	}, actor)
	if !errors.Is(err, domain.ErrBranchHeadOfficeNotAllowed) {
		t.Fatalf("err = %v, ingin ErrBranchHeadOfficeNotAllowed", err)
	}
	if _, ok := repo.branches["002"]; ok {
		t.Fatal("kantor pusat tidak boleh tersimpan")
	}
	if len(audit.events) != 0 {
		t.Fatalf("tidak boleh ada audit untuk pembuatan yang ditolak: %+v", audit.events)
	}
}
