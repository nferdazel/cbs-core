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

// suratBerhargaRepoStub mengimplementasikan domain.SuratBerhargaRegisterRepository di
// memori untuk uji layanan tanpa basis data.
type suratBerhargaRepoStub struct {
	rows      []domain.SuratBerhargaItem
	forOJK    []domain.SuratBerhargaItem
	listErr   error
	upsertErr error
	upserted  []domain.SuratBerhargaItem
	deleted   []uuid.UUID
	found     bool
}

func (s *suratBerhargaRepoStub) ListItems(context.Context) ([]domain.SuratBerhargaItem, error) {
	return s.rows, s.listErr
}

func (s *suratBerhargaRepoStub) ListSuratBerhargaForOJK(context.Context, time.Time) ([]domain.SuratBerhargaItem, error) {
	return s.forOJK, s.listErr
}

func (s *suratBerhargaRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.SuratBerhargaItem, _ uuid.UUID) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *suratBerhargaRepoStub) DeleteItemTx(_ context.Context, _ any, id uuid.UUID) (bool, error) {
	s.deleted = append(s.deleted, id)
	return s.found, nil
}

var _ domain.SuratBerhargaRegisterRepository = (*suratBerhargaRepoStub)(nil)

func newSuratBerhargaService(repo domain.SuratBerhargaRegisterRepository) *suratBerhargaRegisterService {
	return &suratBerhargaRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func suratBerhargaInputSah() domain.UpdateSuratBerhargaItemInput {
	return domain.UpdateSuratBerhargaItemInput{
		KlasifikasiCode:             domain.SuratBerhargaKlasifikasiTersediaUntukDijual,
		SukuBunga:                   decimal.NewFromInt(7),
		TanggalMulai:                "2025-06-01",
		TanggalJatuhTempo:           "2026-06-01",
		Nominal:                     decimal.NewFromInt(1_000_000),
		BiayaPerolehan:              decimal.NewFromInt(990_000),
		BiayaPerolehanDiamortisasi:  decimal.NewFromInt(981_000),
		NomorSuratBerharga:          "ID0000000001",
		CounterpartyID:              "PL-001",
		JenisCode:                   domain.SuratBerhargaJenisPemerintah,
		KualitasCode:                domain.SuratBerhargaKualitasLancar,
		KlasifikasiAsetKeuanganCode: domain.SuratBerhargaKlasifikasiAsetBiayaPerolehan,
		JenisCKPNCode:               domain.SuratBerhargaJenisCKPNKolektif,
		TanggalPenerbitan:           "2025-05-01",
		AsOf:                        "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestSuratBerhargaServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &suratBerhargaRepoStub{}
	svc := newSuratBerhargaService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), suratBerhargaInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := suratBerhargaInputSah()
	invalid.JenisCode = "4"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrSuratBerhargaInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrSuratBerhargaInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem memanggil DELETE fisik lewat repositori; baris yang tidak ada ditolak
// ErrSuratBerhargaNotFound.
func TestSuratBerhargaServiceDelete(t *testing.T) {
	repo := &suratBerhargaRepoStub{found: true}
	svc := newSuratBerhargaService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("id tidak diteruskan ke hapus: %+v", repo.deleted)
	}

	repo.found = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrSuratBerhargaNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrSuratBerhargaNotFound", err)
	}
}

// SuratBerhargaReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestSuratBerhargaServiceReportBankWide(t *testing.T) {
	repo := &suratBerhargaRepoStub{forOJK: []domain.SuratBerhargaItem{{ID: uuid.New(), NomorSuratBerharga: "ID1"}}}
	svc := newSuratBerhargaService(repo)

	if _, err := svc.SuratBerhargaReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrSuratBerhargaBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrSuratBerhargaBankWide", err)
	}
	report, err := svc.SuratBerhargaReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestSuratBerhargaServiceListItemsKosong(t *testing.T) {
	svc := newSuratBerhargaService(&suratBerhargaRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
