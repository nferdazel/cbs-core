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

// pinjamanRepoStub mengimplementasikan domain.PinjamanRegisterRepository di memori
// untuk uji layanan tanpa basis data.
type pinjamanRepoStub struct {
	rows        []domain.PinjamanItem
	forOJK      []domain.PinjamanItem
	listErr     error
	upserted    []domain.PinjamanItem
	deleted     []uuid.UUID
	deleteFound bool
}

func (s *pinjamanRepoStub) ListItems(context.Context) ([]domain.PinjamanItem, error) {
	return s.rows, s.listErr
}

func (s *pinjamanRepoStub) ListPinjamanForOJK(context.Context, time.Time) ([]domain.PinjamanItem, error) {
	return s.forOJK, s.listErr
}

func (s *pinjamanRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.PinjamanItem, _ uuid.UUID) error {
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *pinjamanRepoStub) DeleteItemTx(_ context.Context, _ any, id uuid.UUID) (bool, error) {
	s.deleted = append(s.deleted, id)
	return s.deleteFound, nil
}

var _ domain.PinjamanRegisterRepository = (*pinjamanRepoStub)(nil)

func newPinjamanService(repo domain.PinjamanRegisterRepository) *pinjamanRegisterService {
	return &pinjamanRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func pinjamanInputSah() domain.UpdatePinjamanItemInput {
	return domain.UpdatePinjamanItemInput{
		CounterpartyID:             "KRT-001",
		CreditorGroupCode:          "600",
		BankCode:                   "123456",
		LocationCode:               "1101",
		JenisCode:                  "10",
		RelationshipCode:           "12",
		StartDate:                  "2025-01-02",
		MaturityDate:               "2026-01-02",
		InterestRate:               decimal.NewFromInt(9),
		InterestCalcCode:           "11",
		Plafon:                     decimal.NewFromInt(1_000_000_000),
		CollateralTypeCode:         "02",
		CollateralAmount:           decimal.NewFromInt(1_500_000_000),
		BakiDebet:                  decimal.NewFromInt(800_000_000),
		UnamortizedTransactionCost: decimal.NewFromInt(5_000_000),
		UnamortizedDiscount:        decimal.NewFromInt(2_000_000),
		AsOf:                       "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestPinjamanServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &pinjamanRepoStub{}
	svc := newPinjamanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), pinjamanInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := pinjamanInputSah()
	invalid.JenisCode = "11"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrPinjamanInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrPinjamanInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem meneruskan id ke repositori; baris yang tidak ada ditolak
// ErrPinjamanNotFound.
func TestPinjamanServiceDelete(t *testing.T) {
	repo := &pinjamanRepoStub{deleteFound: true}
	svc := newPinjamanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("id tidak diteruskan: %+v", repo.deleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrPinjamanNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrPinjamanNotFound", err)
	}
}

// PinjamanReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestPinjamanServiceReportBankWide(t *testing.T) {
	repo := &pinjamanRepoStub{forOJK: []domain.PinjamanItem{{ID: uuid.New(), CounterpartyID: "KRT-001"}}}
	svc := newPinjamanService(repo)

	if _, err := svc.PinjamanReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrPinjamanBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrPinjamanBankWide", err)
	}
	report, err := svc.PinjamanReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestPinjamanServiceListItemsKosong(t *testing.T) {
	svc := newPinjamanService(&pinjamanRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
