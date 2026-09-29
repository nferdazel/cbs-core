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

// kepemilikanRepoStub mengimplementasikan domain.KepemilikanRegisterRepository di
// memori untuk uji layanan tanpa basis data.
type kepemilikanRepoStub struct {
	rows        []domain.KepemilikanItem
	forOJK      []domain.KepemilikanItem
	listErr     error
	upserted    []domain.KepemilikanItem
	deleted     []uuid.UUID
	deleteFound bool
}

func (s *kepemilikanRepoStub) ListItems(context.Context) ([]domain.KepemilikanItem, error) {
	return s.rows, s.listErr
}

func (s *kepemilikanRepoStub) ListKepemilikanForOJK(context.Context, time.Time) ([]domain.KepemilikanItem, error) {
	return s.forOJK, s.listErr
}

func (s *kepemilikanRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.KepemilikanItem, _ uuid.UUID) error {
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *kepemilikanRepoStub) DeleteItemTx(_ context.Context, _ any, id uuid.UUID) (bool, error) {
	s.deleted = append(s.deleted, id)
	return s.deleteFound, nil
}

var _ domain.KepemilikanRegisterRepository = (*kepemilikanRepoStub)(nil)

func newKepemilikanService(repo domain.KepemilikanRegisterRepository) *kepemilikanRegisterService {
	return &kepemilikanRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func kepemilikanInputSah() domain.UpdateKepemilikanItemInput {
	return domain.UpdateKepemilikanItemInput{
		ShareholderName:       "Budi Santoso",
		ShareholderAddress:    "Jl. Merdeka 1",
		ShareholderTypeCode:   "01",
		ShareholderStatusCode: "01",
		NominalAmount:         decimal.NewFromInt(50_000_000),
		OwnershipPercentage:   decimal.NewFromInt(60),
		ChangeStatusCode:      "2",
		AsOf:                  "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestKepemilikanServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &kepemilikanRepoStub{}
	svc := newKepemilikanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), kepemilikanInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := kepemilikanInputSah()
	invalid.ShareholderTypeCode = "05"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrKepemilikanInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrKepemilikanInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem meneruskan id ke repositori; baris yang tidak ada ditolak
// ErrKepemilikanNotFound.
func TestKepemilikanServiceDelete(t *testing.T) {
	repo := &kepemilikanRepoStub{deleteFound: true}
	svc := newKepemilikanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("id tidak diteruskan: %+v", repo.deleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrKepemilikanNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrKepemilikanNotFound", err)
	}
}

// KepemilikanReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestKepemilikanServiceReportBankWide(t *testing.T) {
	repo := &kepemilikanRepoStub{forOJK: []domain.KepemilikanItem{{ID: uuid.New(), ShareholderName: "A"}}}
	svc := newKepemilikanService(repo)

	if _, err := svc.KepemilikanReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrKepemilikanBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrKepemilikanBankWide", err)
	}
	report, err := svc.KepemilikanReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestKepemilikanServiceListItemsKosong(t *testing.T) {
	svc := newKepemilikanService(&kepemilikanRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
