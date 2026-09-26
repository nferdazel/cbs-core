package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// bmpkConfigStub membaca system_config dari peta; GetBool benar-benar membaca nilai
// agar jalur saklar hidup dan mati dapat diuji.
type bmpkConfigStub struct {
	values map[string]string
}

func (c *bmpkConfigStub) GetString(_ context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func (c *bmpkConfigStub) GetDecimal(_ context.Context, _ string, fallback decimal.Decimal) decimal.Decimal {
	return fallback
}

func (c *bmpkConfigStub) GetInt(_ context.Context, _ string, fallback int) int { return fallback }

func (c *bmpkConfigStub) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := c.values[key]; ok {
		return strings.EqualFold(strings.TrimSpace(v), "true")
	}
	return fallback
}

func (c *bmpkConfigStub) Invalidate(string) {}

// bmpkRepoStub mengembalikan paparan yang sudah disiapkan dan merekam pemanggilan.
type bmpkRepoStub struct {
	rows   []domain.BMPKPartyExposure
	called bool
	err    error
}

func (r *bmpkRepoStub) ListPartyExposures(_ context.Context) ([]domain.BMPKPartyExposure, error) {
	r.called = true
	if r.err != nil {
		return nil, r.err
	}
	return r.rows, nil
}

// bmpkNamerStub melengkapi nama nasabah secara batch.
type bmpkNamerStub struct {
	names map[uuid.UUID]string
	err   error
}

func (n *bmpkNamerStub) NamesByIDs(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	if n.err != nil {
		return nil, n.err
	}
	out := make(map[uuid.UUID]string)
	for _, id := range ids {
		if name, ok := n.names[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

func TestBMPKServiceMenolakAktorBukanLintasCabang(t *testing.T) {
	repo := &bmpkRepoStub{}
	svc := NewBMPKService(repo, &bmpkConfigStub{}, nil)
	_, err := svc.BMPKReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
	if !errors.Is(err, domain.ErrBMPKBankWide) {
		t.Fatalf("error = %v, ingin ErrBMPKBankWide", err)
	}
	if repo.called {
		t.Fatal("repositori tidak boleh dibaca untuk aktor bukan lintas cabang")
	}
}

func TestBMPKServiceMelengkapiNamaDanStatus(t *testing.T) {
	terkait := uuid.New()
	tanpaBatas := uuid.New()
	repo := &bmpkRepoStub{rows: []domain.BMPKPartyExposure{
		{
			CustomerID:        terkait,
			RelationshipType:  "PEMILIK",
			LoanExposure:      decimal.NewFromInt(80),
			PlacementExposure: decimal.NewFromInt(45),
			LimitAmount:       decimal.NewFromInt(100),
			HasLimit:          true,
		},
		{
			CustomerID:       tanpaBatas,
			RelationshipType: "KELUARGA",
			LoanExposure:     decimal.NewFromInt(10),
		},
	}}
	namer := &bmpkNamerStub{names: map[uuid.UUID]string{
		terkait:    "Budi",
		tanpaBatas: "Siti",
	}}
	svc := NewBMPKService(repo, &bmpkConfigStub{values: map[string]string{domain.BMPKEnabledKey: "false"}}, namer)

	report, err := svc.BMPKReport(context.Background(), time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC), domain.Actor{Role: domain.RoleAuditor})
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	if len(report.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(report.Rows))
	}
	// Urutan deterministik: paparan terbesar lebih dulu.
	if report.Rows[0].CustomerID != terkait {
		t.Fatalf("baris pertama = %s, ingin pihak paparan terbesar", report.Rows[0].CustomerID)
	}
	if report.Rows[0].Status != domain.BMPKStatusMelampauiBatas {
		t.Fatalf("status pihak melampaui = %s", report.Rows[0].Status)
	}
	if !report.Rows[0].Excess.Equal(decimal.NewFromInt(25)) {
		t.Fatalf("kelebihan = %s, ingin 25", report.Rows[0].Excess)
	}
	if report.Rows[0].CustomerName != "Budi" {
		t.Fatalf("nama pihak = %q, ingin Budi", report.Rows[0].CustomerName)
	}
	if report.Rows[1].Status != domain.BMPKStatusBatasBelumDiset {
		t.Fatalf("status pihak tanpa batas = %s", report.Rows[1].Status)
	}
	if report.Rows[1].CustomerName != "Siti" {
		t.Fatalf("nama pihak tanpa batas = %q, ingin Siti", report.Rows[1].CustomerName)
	}
	if report.EnforcementEnabled {
		t.Fatal("saklar mati seharusnya EnforcementEnabled=false")
	}
	// Peringatan harus menyebut saklar mati dan batas yang belum diset.
	var adaSaklar, adaBelumDiset bool
	for _, w := range report.Warnings {
		if strings.Contains(w, domain.BMPKEnabledKey) {
			adaSaklar = true
		}
		if strings.Contains(w, "BATAS_BELUM_DISET") {
			adaBelumDiset = true
		}
	}
	if !adaSaklar || !adaBelumDiset {
		t.Fatalf("peringatan = %v, harus menyebut saklar dan batas belum diset", report.Warnings)
	}
}

func TestBMPKServiceSaklarMenyala(t *testing.T) {
	svc := NewBMPKService(&bmpkRepoStub{}, &bmpkConfigStub{values: map[string]string{domain.BMPKEnabledKey: "true"}}, nil)
	report, err := svc.BMPKReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	if !report.EnforcementEnabled {
		t.Fatal("saklar menyala seharusnya EnforcementEnabled=true")
	}
	for _, w := range report.Warnings {
		if strings.Contains(w, "belum menyala") {
			t.Fatalf("tidak boleh ada peringatan saklar mati: %v", report.Warnings)
		}
	}
}

func TestBMPKServiceTanpaRepoMenolak(t *testing.T) {
	svc := NewBMPKService(nil, &bmpkConfigStub{}, nil)
	if _, err := svc.BMPKReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin}); err == nil {
		t.Fatal("repo nil harus ditolak, bukan laporan kosong yang tampak sah")
	}
}

func TestBMPKServiceRepoErrorDiteruskan(t *testing.T) {
	repo := &bmpkRepoStub{err: errors.New("db mati")}
	svc := NewBMPKService(repo, &bmpkConfigStub{}, nil)
	_, err := svc.BMPKReport(context.Background(), time.Now(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err == nil || !strings.Contains(err.Error(), "db mati") {
		t.Fatalf("error = %v, ingin membungkus galat repositori", err)
	}
}
