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

// asetKeuanganRepoStub mengimplementasikan domain.AsetKeuanganRegisterRepository di
// memori untuk uji layanan tanpa basis data.
type asetKeuanganRepoStub struct {
	rows        []domain.AsetKeuanganItem
	forOJK      []domain.AsetKeuanganItem
	listErr     error
	upsertErr   error
	upserted    []domain.AsetKeuanganItem
	softDeleted []uuid.UUID
	deleteFound bool
}

func (s *asetKeuanganRepoStub) ListItems(context.Context) ([]domain.AsetKeuanganItem, error) {
	return s.rows, s.listErr
}

func (s *asetKeuanganRepoStub) ListAsetKeuanganForOJK(context.Context, time.Time) ([]domain.AsetKeuanganItem, error) {
	return s.forOJK, s.listErr
}

func (s *asetKeuanganRepoStub) UpsertItemTx(_ context.Context, _ any, item domain.AsetKeuanganItem, _ uuid.UUID) error {
	if s.upsertErr != nil {
		return s.upsertErr
	}
	s.upserted = append(s.upserted, item)
	return nil
}

func (s *asetKeuanganRepoStub) SoftDeleteItemTx(_ context.Context, _ any, id uuid.UUID, _ uuid.UUID) (bool, error) {
	s.softDeleted = append(s.softDeleted, id)
	return s.deleteFound, nil
}

var _ domain.AsetKeuanganRegisterRepository = (*asetKeuanganRepoStub)(nil)

func newAsetKeuanganService(repo domain.AsetKeuanganRegisterRepository) *asetKeuanganRegisterService {
	return &asetKeuanganRegisterService{repo: repo, runner: aydaRunnerStub{}}
}

func asetKeuanganInputSah() domain.UpdateAsetKeuanganItemInput {
	return domain.UpdateAsetKeuanganItemInput{
		NoRekening:                  "AKL-001",
		CounterpartyID:              "PL-001",
		JenisCode:                   domain.AsetKeuanganJenisTagihanFraud,
		TanggalMulai:                "2025-06-01",
		TanggalJatuhTempo:           "2026-06-01",
		SukuBunga:                   decimal.NewFromInt(7),
		Nominal:                     decimal.NewFromInt(1_000_000_000),
		NilaiAgunanDiperhitungkan:   decimal.NewFromInt(500_000_000),
		CKPN:                        decimal.NewFromInt(10_000_000),
		CKPNAsetBaik:                decimal.NewFromInt(5_000_000),
		CKPNAsetKurangBaik:          decimal.NewFromInt(3_000_000),
		CKPNAsetTidakBaik:           decimal.NewFromInt(2_000_000),
		KlasifikasiAsetKeuanganCode: domain.AsetKeuanganKlasifikasiBiayaPerolehanDiamortasi,
		JenisCKPNCode:               domain.AsetKeuanganJenisCKPNKolektif,
		AsOf:                        "2026-03-31",
	}
}

// UpsertItem yang sah tersimpan lewat repositori; masukan tidak sah ditolak sebelum
// menyentuh repositori.
func TestAsetKeuanganServiceUpsertValidDanInvalid(t *testing.T) {
	repo := &asetKeuanganRepoStub{}
	svc := newAsetKeuanganService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	item, err := svc.UpsertItem(context.Background(), asetKeuanganInputSah(), actor)
	if err != nil {
		t.Fatalf("upsert sah: %v", err)
	}
	if len(repo.upserted) != 1 || repo.upserted[0].ID != item.ID {
		t.Fatalf("repositori tidak menerima baris sah: %+v", repo.upserted)
	}

	invalid := asetKeuanganInputSah()
	invalid.JenisCode = "11"
	if _, err := svc.UpsertItem(context.Background(), invalid, actor); !errors.Is(err, domain.ErrAsetKeuanganInputInvalid) {
		t.Fatalf("upsert tidak sah: err=%v, ingin ErrAsetKeuanganInputInvalid", err)
	}
	if len(repo.upserted) != 1 {
		t.Fatalf("baris tidak sah tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// No. Rekening yang sudah pernah dipakai (termasuk baris NONAKTIF) ditolak repositori
// dengan ErrAsetKeuanganNoRekeningUsed; layanan meneruskannya dan tidak menyimpan baris.
func TestAsetKeuanganServiceNoRekeningTidakBolehDipakaiUlang(t *testing.T) {
	repo := &asetKeuanganRepoStub{upsertErr: domain.ErrAsetKeuanganNoRekeningUsed}
	svc := newAsetKeuanganService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}

	if _, err := svc.UpsertItem(context.Background(), asetKeuanganInputSah(), actor); !errors.Is(err, domain.ErrAsetKeuanganNoRekeningUsed) {
		t.Fatalf("nomor rekening terpakai: err=%v, ingin ErrAsetKeuanganNoRekeningUsed", err)
	}
	if len(repo.upserted) != 0 {
		t.Fatalf("baris dengan nomor terpakai tidak boleh tersimpan: %+v", repo.upserted)
	}
}

// DeleteItem memanggil soft-delete (bukan DELETE fisik) lewat repositori; baris yang
// tidak ada ditolak ErrAsetKeuanganNotFound.
func TestAsetKeuanganServiceDelete(t *testing.T) {
	repo := &asetKeuanganRepoStub{deleteFound: true}
	svc := newAsetKeuanganService(repo)
	actor := domain.Actor{Role: domain.RoleAdmin}
	id := uuid.New()

	if err := svc.DeleteItem(context.Background(), id, actor); err != nil {
		t.Fatalf("hapus baris ada: %v", err)
	}
	if len(repo.softDeleted) != 1 || repo.softDeleted[0] != id {
		t.Fatalf("id tidak diteruskan ke soft-delete: %+v", repo.softDeleted)
	}

	repo.deleteFound = false
	if err := svc.DeleteItem(context.Background(), uuid.New(), actor); !errors.Is(err, domain.ErrAsetKeuanganNotFound) {
		t.Fatalf("hapus baris absen: err=%v, ingin ErrAsetKeuanganNotFound", err)
	}
}

// AsetKeuanganReport bersifat bank-wide: aktor non-lintas cabang ditolak; aktor lintas
// cabang menerima baris register tanpa mengubah data.
func TestAsetKeuanganServiceReportBankWide(t *testing.T) {
	repo := &asetKeuanganRepoStub{forOJK: []domain.AsetKeuanganItem{{ID: uuid.New(), NoRekening: "AKL-001"}}}
	svc := newAsetKeuanganService(repo)

	if _, err := svc.AsetKeuanganReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrAsetKeuanganBankWide) {
		t.Fatalf("aktor non-lintas cabang: err=%v, ingin ErrAsetKeuanganBankWide", err)
	}
	report, err := svc.AsetKeuanganReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("laporan lintas cabang: %v", err)
	}
	if len(report.Items) != 1 {
		t.Fatalf("baris laporan = %d, ingin 1", len(report.Items))
	}
}

// ListItems mengembalikan daftar kosong (bukan nil) agar API selalu berbentuk array.
func TestAsetKeuanganServiceListItemsKosong(t *testing.T) {
	svc := newAsetKeuanganService(&asetKeuanganRepoStub{})
	items, err := svc.ListItems(context.Background())
	if err != nil {
		t.Fatalf("ListItems: %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, ingin slice kosong", items)
	}
}
