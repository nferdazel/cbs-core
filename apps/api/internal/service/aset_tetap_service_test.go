package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// asetTetapRepoStub mengimplementasikan domain.AsetTetapRegisterRepository di memori
// untuk uji layanan tanpa basis data.
type asetTetapRepoStub struct {
	rows        []domain.AsetTetapItem
	forOJK      []domain.AsetTetapItem
	listErr     error
	upsertErr   error
	upserted    []domain.AsetTetapItem
	deleted     []uuid.UUID
	deleteFound bool
}

func (s *asetTetapRepoStub) ListItems(context.Context) ([]domain.AsetTetapItem, error) {
	return s.rows, s.listErr
}

func (s *asetTetapRepoStub) ListAsetTetapForOJK(context.Context, time.Time) ([]domain.AsetTetapItem, error) {
	return s.forOJK, s.listErr
}

func (s *asetTetapRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.AsetTetapItem, _ uuid.UUID) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *asetTetapRepoStub) DeleteItemTx(_ context.Context, _ any, id uuid.UUID) (bool, error) {
	s.deleted = append(s.deleted, id)
	return s.deleteFound, nil
}

var _ domain.AsetTetapRegisterRepository = (*asetTetapRepoStub)(nil)

func newAsetTetapService(repo domain.AsetTetapRegisterRepository) *asetTetapRegisterService {
	return &asetTetapRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func asetTetapInputSah() domain.UpdateAsetTetapItemInput {
	return domain.UpdateAsetTetapItemInput{
		JenisAsetCode:                   domain.AsetJenisBangunan,
		SumberPerolehanCode:             domain.AsetSumberModalDisetor,
		StatusAsetCode:                  domain.AsetStatusTidakDijaminkan,
		BiayaPerolehan:                  decimal.NewFromInt(2_000_000_000),
		AkumulasiPenyusutanAmortisasi:   decimal.NewFromInt(300_000_000),
		AkumulasiKerugianPenurunanNilai: decimal.NewFromInt(0),
		MetodePengukuranCode:            domain.AsetMetodeBiaya,
		AsOf:                            "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestAsetTetapServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &asetTetapRepoStub{}
	svc := newAsetTetapService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), asetTetapInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := asetTetapInputSah()
	invalid.JenisAsetCode = "999"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrAsetTetapInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrAsetTetapInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem memanggil hapus fisik lewat repositori; baris yang tidak ada ditolak
// ErrAsetTetapNotFound.
func TestAsetTetapServiceDelete(t *testing.T) {
	repo := &asetTetapRepoStub{deleteFound: true}
	svc := newAsetTetapService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("id tidak diteruskan ke hapus: %+v", repo.deleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrAsetTetapNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrAsetTetapNotFound", err)
	}
}

// AsetTetapReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestAsetTetapServiceReportBankWide(t *testing.T) {
	repo := &asetTetapRepoStub{forOJK: []domain.AsetTetapItem{{ID: uuid.New(), JenisAsetCode: domain.AsetJenisTanah}}}
	svc := newAsetTetapService(repo)

	if _, err := svc.AsetTetapReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrAsetTetapBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrAsetTetapBankWide", err)
	}
	report, err := svc.AsetTetapReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestAsetTetapServiceListItemsKosong(t *testing.T) {
	svc := newAsetTetapService(&asetTetapRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
