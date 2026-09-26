package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TestBMPKTotalAgregasiKreditPenempatan memastikan agregasi paparan per pihak
// menjumlahkan kredit dan penempatan, bukan mengambil salah satunya saja.
func TestBMPKTotalAgregasiKreditPenempatan(t *testing.T) {
	e := BMPKPartyExposure{
		LoanExposure:      decimal.NewFromInt(100_000_000),
		PlacementExposure: decimal.NewFromInt(25_000_000),
	}
	if got, want := e.Total(), decimal.NewFromInt(125_000_000); !got.Equal(want) {
		t.Fatalf("total paparan = %s, ingin %s", got, want)
	}
}

func TestCheckBMPKPartyStatus(t *testing.T) {
	limit := func(amount int64) *decimal.Decimal {
		d := decimal.NewFromInt(amount)
		return &d
	}
	cases := []struct {
		nama       string
		credit     int64
		placement  int64
		limit      *decimal.Decimal
		wantStatus string
		wantExcess int64
	}{
		{
			nama:       "batas belum diset meski ada paparan",
			credit:     100,
			placement:  0,
			limit:      nil,
			wantStatus: BMPKStatusBatasBelumDiset,
			wantExcess: 0,
		},
		{
			nama:       "paparan tepat sama dengan batas",
			credit:     100,
			placement:  0,
			limit:      limit(100),
			wantStatus: BMPKStatusDalamBatas,
			wantExcess: 0,
		},
		{
			nama:       "paparan di bawah batas",
			credit:     40,
			placement:  10,
			limit:      limit(100),
			wantStatus: BMPKStatusDalamBatas,
			wantExcess: 0,
		},
		{
			nama:       "kredit saja melampaui batas",
			credit:     150,
			placement:  0,
			limit:      limit(100),
			wantStatus: BMPKStatusMelampauiBatas,
			wantExcess: 50,
		},
		{
			nama:       "kredit dan penempatan bersama melampaui batas",
			credit:     80,
			placement:  45,
			limit:      limit(100),
			wantStatus: BMPKStatusMelampauiBatas,
			wantExcess: 25,
		},
		{
			nama:       "batas nol sah dan setiap paparan positif melampaui",
			credit:     1,
			placement:  0,
			limit:      limit(0),
			wantStatus: BMPKStatusMelampauiBatas,
			wantExcess: 1,
		},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			e := BMPKPartyExposure{
				LoanExposure:      decimal.NewFromInt(c.credit),
				PlacementExposure: decimal.NewFromInt(c.placement),
			}
			if c.limit != nil {
				e.HasLimit = true
				e.LimitAmount = *c.limit
			}
			got := CheckBMPKParty(e)
			if got.Status != c.wantStatus {
				t.Fatalf("status = %s, ingin %s (alasan: %s)", got.Status, c.wantStatus, got.StatusReason)
			}
			if !got.Excess.Equal(decimal.NewFromInt(c.wantExcess)) {
				t.Fatalf("kelebihan = %s, ingin %d", got.Excess, c.wantExcess)
			}
			if !got.TotalExposure.Equal(decimal.NewFromInt(c.credit + c.placement)) {
				t.Fatalf("total = %s, ingin %d", got.TotalExposure, c.credit+c.placement)
			}
			if c.wantStatus == BMPKStatusMelampauiBatas && got.StatusReason == "" {
				t.Fatal("status melampaui harus menyertakan alasan")
			}
		})
	}
}

func TestBMPKRelatedPartyValidate(t *testing.T) {
	valid := BMPKRelatedParty{CustomerID: uuid.New(), RelationshipType: "PEMILIK"}
	if err := valid.Validate(); err != nil {
		t.Fatalf("data sah ditolak: %v", err)
	}
	if err := (BMPKRelatedParty{CustomerID: uuid.New()}).Validate(); err == nil {
		t.Fatal("jenis hubungan kosong harus ditolak")
	}
	if err := (BMPKRelatedParty{RelationshipType: "PEMILIK"}).Validate(); err == nil {
		t.Fatal("customer_id kosong harus ditolak")
	}
}
