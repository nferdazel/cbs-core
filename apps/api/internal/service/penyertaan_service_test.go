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

// penyertaanRepoStub mengimplementasikan domain.PenyertaanRegisterRepository di memori
// untuk uji layanan tanpa basis data.
type penyertaanRepoStub struct {
	rows        []domain.PenyertaanItem
	forOJK      []domain.PenyertaanItem
	listErr     error
	upsertErr   error
	upserted    []domain.PenyertaanItem
	softDeleted []uuid.UUID
	deleteFound bool
}

func (s *penyertaanRepoStub) ListItems(context.Context) ([]domain.PenyertaanItem, error) {
	return s.rows, s.listErr
}

func (s *penyertaanRepoStub) ListPenyertaanForOJK(context.Context, time.Time) ([]domain.PenyertaanItem, error) {
	return s.forOJK, s.listErr
}

func (s *penyertaanRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.PenyertaanItem, _ uuid.UUID) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *penyertaanRepoStub) SoftDeleteItemTx(_ context.Context, _ any, id uuid.UUID, _ uuid.UUID) (bool, error) {
	s.softDeleted = append(s.softDeleted, id)
	return s.deleteFound, nil
}

var _ domain.PenyertaanRegisterRepository = (*penyertaanRepoStub)(nil)

func newPenyertaanService(repo domain.PenyertaanRegisterRepository) *penyertaanRegisterService {
	return &penyertaanRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func penyertaanInputSah() domain.UpdatePenyertaanItemInput {
	return domain.UpdatePenyertaanItemInput{
		NoRegister:           "PM-001",
		CounterpartyID:       "PL-001",
		MetodePenyertaanCode: domain.PenyertaanMetodeBiayaPerolehan,
		KualitasCode:         domain.PenyertaanKualitasLancar,
		TujuanPenyertaanCode: domain.PenyertaanTujuanLembagaPenunjang,
		TanggalMulai:         "2025-06-01",
		PersentasePenyertaan: decimal.NewFromInt(25),
		Nominal:              decimal.NewFromInt(1_000_000_000),
		JumlahBulanLaporan:   decimal.NewFromInt(900_000_000),
		CKPN:                 decimal.NewFromInt(10_000_000),
		CKPNAsetBaik:         decimal.NewFromInt(5_000_000),
		CKPNAsetKurangBaik:   decimal.NewFromInt(3_000_000),
		CKPNAsetTidakBaik:    decimal.NewFromInt(2_000_000),
		JenisCKPNCode:        domain.PenyertaanJenisCKPNKolektif,
		AsOf:                 "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestPenyertaanServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &penyertaanRepoStub{}
	svc := newPenyertaanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), penyertaanInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := penyertaanInputSah()
	invalid.KualitasCode = "2"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrPenyertaanInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrPenyertaanInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// No. Register yang sudah pernah dipakai (termasuk baris NONAKTIF) ditolak repositori
// dengan ErrPenyertaanNoRegisterUsed; layanan meneruskannya dan tidak menyimpan baris.
func TestPenyertaanServiceNoRegisterTidakBolehDipakaiUlang(t *testing.T) {
	repo := &penyertaanRepoStub{upsertErr: domain.ErrPenyertaanNoRegisterUsed}
	svc := newPenyertaanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	if _, err := svc.UpsertItem(context.Background(), penyertaanInputSah(), actor); !errors.Is(err, domain.ErrPenyertaanNoRegisterUsed) {
		t.Fatalf("nomor register terpakai: err=%v, ingin ErrPenyertaanNoRegisterUsed", err)
	}
	if len(repo.upserted) != 0 {
		t.Fatalf("baris dengan nomor terpakai tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem memanggil soft-delete (bukan DELETE fisik) lewat repositori; baris yang
// tidak ada ditolak ErrPenyertaanNotFound.
func TestPenyertaanServiceDelete(t *testing.T) {
	repo := &penyertaanRepoStub{deleteFound: true}
	svc := newPenyertaanService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.softDeleted) != 1 || repo.softDeleted[0] != id {
		t.Fatalf("id tidak diteruskan ke soft-delete: %+v", repo.softDeleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrPenyertaanNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrPenyertaanNotFound", err)
	}
}

// PenyertaanReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestPenyertaanServiceReportBankWide(t *testing.T) {
	repo := &penyertaanRepoStub{forOJK: []domain.PenyertaanItem{{ID: uuid.New(), NoRegister: "PM-001"}}}
	svc := newPenyertaanService(repo)

	if _, err := svc.PenyertaanReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrPenyertaanBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrPenyertaanBankWide", err)
	}
	report, err := svc.PenyertaanReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestPenyertaanServiceListItemsKosong(t *testing.T) {
	svc := newPenyertaanService(&penyertaanRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
