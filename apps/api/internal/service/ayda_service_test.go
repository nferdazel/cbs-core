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

// aydaRepoStub mengimplementasikan domain.AYDARegisterRepository di memori untuk uji
// layanan tanpa basis data.
type aydaRepoStub struct {
	rows        []domain.AYDAItem
	forOJK      []domain.AYDAItem
	listErr     error
	upserted    []domain.AYDAItem
	deleted     []uuid.UUID
	deleteFound bool
}

func (s *aydaRepoStub) ListItems(context.Context) ([]domain.AYDAItem, error) {
	return s.rows, s.listErr
}

func (s *aydaRepoStub) ListAYDAForOJK(context.Context, time.Time) ([]domain.AYDAItem, error) {
	return s.forOJK, s.listErr
}

func (s *aydaRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.AYDAItem, _ uuid.UUID) error {
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *aydaRepoStub) DeleteItemTx(_ context.Context, _ any, id uuid.UUID) (bool, error) {
	s.deleted = append(s.deleted, id)
	return s.deleteFound, nil
}

var _ domain.AYDARegisterRepository = (*aydaRepoStub)(nil)

// aydaRunnerStub menjalankan fn tanpa transaksi sungguhan.
type aydaRunnerStub struct{}

func (aydaRunnerStub) Run(_ context.Context, fn func(tx any) error) error { return fn(nil) }

func newAYDAService(repo domain.AYDARegisterRepository) *aydaRegisterService {
	return &aydaRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func aydaInputSah() domain.UpdateAYDAItemInput {
	return domain.UpdateAYDAItemInput{
		CollateralTypeCode:      "02",
		CollateralAddress:       "Jl. Merdeka 1",
		AcquisitionDate:         "2025-06-30",
		InitialRecognitionValue: decimal.NewFromInt(1_000_000),
		AccumulatedImpairment:   decimal.NewFromInt(100_000),
		NetRealizableValue:      decimal.NewFromInt(900_000),
		AsOf:                    "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestAYDAServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &aydaRepoStub{}
	svc := newAYDAService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), aydaInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := aydaInputSah()
	invalid.CollateralTypeCode = "07"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrAYDAInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrAYDAInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem meneruskan id ke repositori; baris yang tidak ada ditolak ErrAYDANotFound.
func TestAYDAServiceDelete(t *testing.T) {
	repo := &aydaRepoStub{deleteFound: true}
	svc := newAYDAService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("id tidak diteruskan: %+v", repo.deleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrAYDANotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrAYDANotFound", err)
	}
}

// AYDAReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas cabang
// menerima baris register tanpa mengubah data.
func TestAYDAServiceReportBankWide(t *testing.T) {
	repo := &aydaRepoStub{forOJK: []domain.AYDAItem{{ID: uuid.New(), CollateralAddress: "A"}}}
	svc := newAYDAService(repo)

	if _, err := svc.AYDAReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrAYDABankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrAYDABankWide", err)
	}
	report, err := svc.AYDAReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestAYDAServiceListItemsKosong(t *testing.T) {
	svc := newAYDAService(&aydaRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
