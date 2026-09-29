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

// kasValasRepoStub mengimplementasikan domain.KasValasRegisterRepository di memori untuk
// uji layanan tanpa basis data.
type kasValasRepoStub struct {
	rows      []domain.KasValasItem
	forOJK    []domain.KasValasItem
	listErr   error
	upsertErr error
	upserted  []domain.KasValasItem
	deleted   []uuid.UUID
	found     bool
}

func (s *kasValasRepoStub) ListItems(context.Context) ([]domain.KasValasItem, error) {
	return s.rows, s.listErr
}

func (s *kasValasRepoStub) ListKasValasForOJK(context.Context, time.Time) ([]domain.KasValasItem, error) {
	return s.forOJK, s.listErr
}

func (s *kasValasRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.KasValasItem, _ uuid.UUID) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *kasValasRepoStub) DeleteItemTx(_ context.Context, _ any, id uuid.UUID) (bool, error) {
	s.deleted = append(s.deleted, id)
	return s.found, nil
}

var _ domain.KasValasRegisterRepository = (*kasValasRepoStub)(nil)

func newKasValasService(repo domain.KasValasRegisterRepository) *kasValasRegisterService {
	return &kasValasRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func kasValasInputSah() domain.UpdateKasValasItemInput {
	return domain.UpdateKasValasItemInput{
		JenisValasCode: "USD",
		Nominal:        decimal.NewFromInt(1_000),
		KursTengah:     decimal.NewFromInt(16_000),
		AsOf:           "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestKasValasServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &kasValasRepoStub{}
	svc := newKasValasService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), kasValasInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := kasValasInputSah()
	invalid.JenisValasCode = ""
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrKasValasInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrKasValasInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem memanggil DELETE fisik lewat repositori; baris yang tidak ada ditolak
// ErrKasValasNotFound.
func TestKasValasServiceDelete(t *testing.T) {
	repo := &kasValasRepoStub{found: true}
	svc := newKasValasService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("id tidak diteruskan ke hapus: %+v", repo.deleted)
	}

	repo.found = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrKasValasNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrKasValasNotFound", err)
	}
}

// KasValasReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas cabang
// menerima baris register tanpa mengubah data.
func TestKasValasServiceReportBankWide(t *testing.T) {
	repo := &kasValasRepoStub{forOJK: []domain.KasValasItem{{ID: uuid.New(), JenisValasCode: "USD"}}}
	svc := newKasValasService(repo)

	if _, err := svc.KasValasReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrKasValasBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrKasValasBankWide", err)
	}
	report, err := svc.KasValasReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestKasValasServiceListItemsKosong(t *testing.T) {
	svc := newKasValasService(&kasValasRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
