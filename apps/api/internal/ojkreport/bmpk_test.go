package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func bmpkCell(t *testing.T, row TableRow, sandi string) string {
	t.Helper()
	for _, c := range row.Cells {
		if c.Sandi == sandi {
			return c.Value
		}
	}
	t.Fatalf("sel %s tidak ditemukan pada baris %s", sandi, row.Key)
	return ""
}

func TestBuildBMPKTableBarisDanBatasBelumDiset(t *testing.T) {
	idDenganBatas := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	idTanpaBatas := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	report := domain.BMPKReport{
		AsOf:               time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC),
		EnforcementEnabled: false,
		Rows: []domain.BMPKPartyCheck{
			domain.CheckBMPKParty(domain.BMPKPartyExposure{
				CustomerID:        idDenganBatas,
				CustomerName:      "Budi",
				RelationshipType:  "PEMILIK",
				LoanExposure:      decimal.NewFromInt(80),
				PlacementExposure: decimal.NewFromInt(45),
				LimitAmount:       decimal.NewFromInt(100),
				HasLimit:          true,
			}),
			domain.CheckBMPKParty(domain.BMPKPartyExposure{
				CustomerID:       idTanpaBatas,
				RelationshipType: "KELUARGA",
				LoanExposure:     decimal.NewFromInt(10),
			}),
		},
		Warnings: []string{"saklar bmpk.enabled belum menyala"},
	}

	sec := BuildBMPKTable(report)
	if sec.Form != "LAPORAN_BMPK" {
		t.Fatalf("form = %s, ingin LAPORAN_BMPK", sec.Form)
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(sec.Rows))
	}
	if len(sec.Unavailable) == 0 {
		t.Fatal("kolom resmi yang belum tersedia harus didaftarkan, bukan disembunyikan")
	}

	baris := sec.Rows[0]
	if got := bmpkCell(t, baris, bmpkSandiTotal); got != "125" {
		t.Fatalf("total = %q, ingin 125", got)
	}
	if got := bmpkCell(t, baris, bmpkSandiBatas); got != "100" {
		t.Fatalf("batas = %q, ingin 100", got)
	}
	if got := bmpkCell(t, baris, bmpkSandiStatus); got != domain.BMPKStatusMelampauiBatas {
		t.Fatalf("status = %q, ingin %s", got, domain.BMPKStatusMelampauiBatas)
	}
	if got := bmpkCell(t, baris, bmpkSandiNama); got != "Budi" {
		t.Fatalf("nama = %q, ingin Budi", got)
	}

	tanpa := sec.Rows[1]
	if got := bmpkCell(t, tanpa, bmpkSandiBatas); got != "-" {
		t.Fatalf("batas tanpa baris bmpk_limits = %q, ingin '-' (bukan 0)", got)
	}
	if got := bmpkCell(t, tanpa, bmpkSandiStatus); got != domain.BMPKStatusBatasBelumDiset {
		t.Fatalf("status = %q, ingin %s", got, domain.BMPKStatusBatasBelumDiset)
	}
	if got := bmpkCell(t, tanpa, bmpkSandiNama); got != "-" {
		t.Fatalf("nama kosong = %q, ingin '-'", got)
	}

	adaCatatanSaklar := false
	for _, n := range sec.Notes {
		if strings.Contains(n, "bmpk.enabled") {
			adaCatatanSaklar = true
		}
	}
	if !adaCatatanSaklar {
		t.Fatalf("catatan harus menyebut saklar; notes = %v", sec.Notes)
	}
}

func TestBuildBMPKTableBatasNolTetapNol(t *testing.T) {
	report := domain.BMPKReport{
		Rows: []domain.BMPKPartyCheck{
			domain.CheckBMPKParty(domain.BMPKPartyExposure{
				CustomerID:   uuid.New(),
				LoanExposure: decimal.NewFromInt(5),
				LimitAmount:  decimal.Zero,
				HasLimit:     true,
			}),
		},
	}
	sec := BuildBMPKTable(report)
	if got := bmpkCell(t, sec.Rows[0], bmpkSandiBatas); got != "0" {
		t.Fatalf("batas nol sah = %q, ingin '0' (bukan '-')", got)
	}
	if got := bmpkCell(t, sec.Rows[0], bmpkSandiStatus); got != domain.BMPKStatusMelampauiBatas {
		t.Fatalf("status = %q, ingin %s", got, domain.BMPKStatusMelampauiBatas)
	}
}

// bmpkStubSource memenuhi BMPKSource untuk menguji GenerateBMPK.
type bmpkStubSource struct {
	report domain.BMPKReport
	err    error
}

func (s *bmpkStubSource) BMPKReport(_ context.Context, _ time.Time, _ domain.Actor) (domain.BMPKReport, error) {
	return s.report, s.err
}

func TestGenerateBMPKMenolakTanpaSumber(t *testing.T) {
	if _, err := GenerateBMPK(context.Background(), nil, time.Now(), domain.Actor{Role: domain.RoleSuperAdmin}); err == nil {
		t.Fatal("sumber nil harus ditolak")
	}
}

func TestGenerateBMPKMenghasilkanBundleDenganTabel(t *testing.T) {
	src := &bmpkStubSource{report: domain.BMPKReport{
		AsOf:               time.Date(2026, time.September, 30, 0, 0, 0, 0, time.UTC),
		EnforcementEnabled: true,
		Rows: []domain.BMPKPartyCheck{
			domain.CheckBMPKParty(domain.BMPKPartyExposure{CustomerID: uuid.New(), LoanExposure: decimal.NewFromInt(1)}),
		},
	}}
	bundle, err := GenerateBMPK(context.Background(), src, time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	if len(bundle.Tables) != 1 || bundle.Tables[0].Form != "LAPORAN_BMPK" {
		t.Fatalf("bundle harus memuat satu tabel LAPORAN_BMPK: %+v", bundle.Tables)
	}
	if want := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC); !bundle.Deadline.Equal(want) {
		t.Fatalf("tenggat = %s, ingin %s", bundle.Deadline, want)
	}
}
