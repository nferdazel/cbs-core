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

// propertiRepoStub mengimplementasikan domain.PropertiRegisterRepository di memori
// untuk uji layanan tanpa basis data.
type propertiRepoStub struct {
	rows        []domain.PropertiItem
	forOJK      []domain.PropertiItem
	listErr     error
	upsertErr   error
	upserted    []domain.PropertiItem
	softDeleted []uuid.UUID
	deleteFound bool
}

func (s *propertiRepoStub) ListItems(context.Context) ([]domain.PropertiItem, error) {
	return s.rows, s.listErr
}

func (s *propertiRepoStub) ListPropertiForOJK(context.Context, time.Time) ([]domain.PropertiItem, error) {
	return s.forOJK, s.listErr
}

func (s *propertiRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.PropertiItem, _ uuid.UUID) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *propertiRepoStub) SoftDeleteItemTx(_ context.Context, _ any, id uuid.UUID, _ uuid.UUID) (bool, error) {
	s.softDeleted = append(s.softDeleted, id)
	return s.deleteFound, nil
}

var _ domain.PropertiRegisterRepository = (*propertiRepoStub)(nil)

func newPropertiService(repo domain.PropertiRegisterRepository) *propertiRegisterService {
	return &propertiRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func propertiInputSah() domain.UpdatePropertiItemInput {
	return domain.UpdatePropertiItemInput{
		NoRegister:                    "PRP-001",
		JenisPropertiCode:             "1",
		AlamatProperti:                "Jl. Contoh No. 1",
		Koordinat:                     "-6.2, 106.8",
		TanggalPenetapan:              "2025-06-01",
		BiayaPerolehanNilaiWajar:      decimal.NewFromInt(1_000_000_000),
		AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(200_000_000),
		MetodePengukuranCode:          "1",
		AsOf:                          "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestPropertiServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &propertiRepoStub{}
	svc := newPropertiService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), propertiInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := propertiInputSah()
	invalid.JenisPropertiCode = "9"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrPropertiInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrPropertiInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// No. Register yang sudah pernah dipakai (termasuk baris NONAKTIF) ditolak repositori
// dengan ErrPropertiNoRegisterUsed; layanan meneruskannya dan tidak menulis audit.
func TestPropertiServiceNoRegisterTidakBolehDipakaiUlang(t *testing.T) {
	repo := &propertiRepoStub{upsertErr: domain.ErrPropertiNoRegisterUsed}
	svc := newPropertiService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	if _, err := svc.UpsertItem(context.Background(), propertiInputSah(), actor); !errors.Is(err, domain.ErrPropertiNoRegisterUsed) {
		t.Fatalf("nomor register terpakai: err=%v, ingin ErrPropertiNoRegisterUsed", err)
	}
	if len(repo.upserted) != 0 {
		t.Fatalf("baris dengan nomor terpakai tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem memanggil soft-delete (bukan DELETE fisik) lewat repositori; baris yang
// tidak ada ditolak ErrPropertiNotFound.
func TestPropertiServiceDelete(t *testing.T) {
	repo := &propertiRepoStub{deleteFound: true}
	svc := newPropertiService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.softDeleted) != 1 || repo.softDeleted[0] != id {
		t.Fatalf("id tidak diteruskan ke soft-delete: %+v", repo.softDeleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrPropertiNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrPropertiNotFound", err)
	}
}

// PropertiReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestPropertiServiceReportBankWide(t *testing.T) {
	repo := &propertiRepoStub{forOJK: []domain.PropertiItem{{ID: uuid.New(), NoRegister: "PRP-001"}}}
	svc := newPropertiService(repo)

	if _, err := svc.PropertiReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrPropertiBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrPropertiBankWide", err)
	}
	report, err := svc.PropertiReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestPropertiServiceListItemsKosong(t *testing.T) {
	svc := newPropertiService(&propertiRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
