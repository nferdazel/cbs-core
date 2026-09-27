package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Form 06.00 kolom XXXVII terisi status BMPK pihak terkait; baris yang nasabahnya
// belum ditandai pihak terkait ditulis "-" dan kolom tetap tersedia.
func TestBuildForm06MengisiStatusBMPK(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-BMPK", BMPKStatus: domain.BMPKStatusMelampauiBatas},
		{Status: "DISBURSED", LoanNumber: "LN-BIASA"},
	}
	sec := buildForm06(rows)

	if got := findCell(t, sec, "LN-BMPK", form06SandiBMPK).Value; got != domain.BMPKStatusMelampauiBatas {
		t.Errorf("kolom XXXVII = %q, ingin %s", got, domain.BMPKStatusMelampauiBatas)
	}
	if got := findCell(t, sec, "LN-BIASA", form06SandiBMPK).Value; got != "-" {
		t.Errorf("baris tanpa pihak terkait = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiBMPK {
			t.Errorf("kolom XXXVII seharusnya tersedia saat ada status BMPK: %s", u.Reason)
		}
	}
}

// Tanpa status BMPK sama sekali, kolom XXXVII dinyatakan belum tersedia beserta
// alasannya, bukan diisi nol.
func TestBuildForm06StatusBMPKBelumTersediaTanpaData(t *testing.T) {
	sec := buildForm06([]LoanRow{{Status: "DISBURSED", LoanNumber: "LN-001"}})
	if u := unavailableColumn(t, sec, form06SandiBMPK); u.Reason == "" {
		t.Fatal("kolom XXXVII tanpa data harus punya alasan")
	}
}

// Form 05.00 kolom XV terisi status BMPK pihak terkait yang ditautkan ke penempatan.
func TestBuildForm05MengisiStatusBMPK(t *testing.T) {
	rows := []PlacementRow{
		{CounterpartyBank: "Bank Terkait", PlacementType: "DEPOSITO", Collectibility: "LANCAR",
			Outstanding: decimal.NewFromInt(1_000_000), BMPKStatus: domain.BMPKStatusDalamBatas},
		{CounterpartyBank: "Bank Belum Ditautkan", PlacementType: "GIRO", Collectibility: "LANCAR",
			Outstanding: decimal.NewFromInt(500_000)},
	}
	sec := buildForm05(rows)

	if got := findCell(t, sec, "Bank Terkait", form05SandiBMPK).Value; got != domain.BMPKStatusDalamBatas {
		t.Errorf("kolom XV = %q, ingin %s", got, domain.BMPKStatusDalamBatas)
	}
	if got := findCell(t, sec, "Bank Belum Ditautkan", form05SandiBMPK).Value; got != "-" {
		t.Errorf("penempatan belum ditautkan = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form05SandiBMPK {
			t.Errorf("kolom XV seharusnya tersedia saat ada status BMPK: %s", u.Reason)
		}
	}
}

// bmpkBuilderStubSource menambahkan kontrak kredit, penempatan, dan BMPK supaya
// perakit dapat diuji mengisi kolom Status BMPK dari satu pemuatan agregat.
type bmpkBuilderStubSource struct {
	*stubSource
	loans      []LoanRow
	placements []PlacementRow
	report     domain.BMPKReport
	bmpkErr    error
	bmpkCalls  int
}

func (s *bmpkBuilderStubSource) ListLoansForOJK(context.Context, time.Time, domain.Actor) ([]LoanRow, error) {
	return s.loans, nil
}

func (s *bmpkBuilderStubSource) ListPlacementsForOJK(context.Context, time.Time, domain.Actor) ([]PlacementRow, error) {
	return s.placements, nil
}

func (s *bmpkBuilderStubSource) BMPKReport(context.Context, time.Time, domain.Actor) (domain.BMPKReport, error) {
	s.bmpkCalls++
	return s.report, s.bmpkErr
}

// Perakit memuat laporan BMPK SATU kali dan memetakan statusnya ke baris Form 06.00
// (lewat UUID nasabah) dan Form 05.00 (lewat UUID pihak terkait penempatan).
func TestGenerateMonthlyMemetakanStatusBMPKKeForm(t *testing.T) {
	kreditTerkait := uuid.New()
	kreditBiasa := uuid.New()
	penempatanTerkait := uuid.New()

	src := &bmpkBuilderStubSource{
		stubSource: newStubSource(),
		loans: []LoanRow{
			{Status: "DISBURSED", LoanNumber: "LN-TERKAIT", CustomerID: kreditTerkait.String()},
			{Status: "DISBURSED", LoanNumber: "LN-BIASA", CustomerID: kreditBiasa.String()},
		},
		placements: []PlacementRow{
			{CounterpartyBank: "Bank Terkait", PlacementType: "DEPOSITO", Collectibility: "LANCAR",
				Outstanding: decimal.NewFromInt(1_000_000), CustomerID: penempatanTerkait.String()},
		},
		report: domain.BMPKReport{
			EnforcementEnabled: true,
			Rows: []domain.BMPKPartyCheck{
				domain.CheckBMPKParty(domain.BMPKPartyExposure{
					CustomerID: kreditTerkait, LoanExposure: decimal.NewFromInt(5),
				}),
				domain.CheckBMPKParty(domain.BMPKPartyExposure{
					CustomerID: penempatanTerkait, PlacementExposure: decimal.NewFromInt(1),
				}),
			},
		},
	}

	b, err := NewBuilder(src).GenerateMonthly(context.Background(),
		time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	if src.bmpkCalls != 1 {
		t.Fatalf("laporan BMPK dimuat %d kali, ingin 1 (hindari N+1)", src.bmpkCalls)
	}

	// Kedua pihak belum punya batas, jadi statusnya BATAS_BELUM_DISET.
	form06 := tableByForm(t, b, "06.00")
	if got := findCell(t, form06, "LN-TERKAIT", form06SandiBMPK).Value; got != domain.BMPKStatusBatasBelumDiset {
		t.Errorf("Form 06 kolom XXXVII = %q, ingin %s", got, domain.BMPKStatusBatasBelumDiset)
	}
	if got := findCell(t, form06, "LN-BIASA", form06SandiBMPK).Value; got != "-" {
		t.Errorf("Form 06 baris biasa = %q, ingin -", got)
	}

	form05 := tableByForm(t, b, "05.00")
	if got := findCell(t, form05, "Bank Terkait", form05SandiBMPK).Value; got != domain.BMPKStatusBatasBelumDiset {
		t.Errorf("Form 05 kolom XV = %q, ingin %s", got, domain.BMPKStatusBatasBelumDiset)
	}
}

// Sumber BMPK yang belum dirangkai bukan kegagalan ekspor bulanan: kolom BMPK
// dinyatakan belum tersedia, tetapi form lain tetap dibangun.
func TestGenerateMonthlySumberBMPKBelumDirangkai(t *testing.T) {
	src := &bmpkBuilderStubSource{
		stubSource: newStubSource(),
		loans:      []LoanRow{{Status: "DISBURSED", LoanNumber: "LN-001", CustomerID: uuid.New().String()}},
		bmpkErr:    ErrBMPKSourceUnavailable,
	}
	b, err := NewBuilder(src).GenerateMonthly(context.Background(),
		time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "CONVENTIONAL")
	if err != nil {
		t.Fatalf("sumber BMPK yang belum dirangkai tidak boleh menggagalkan ekspor: %v", err)
	}
	form06 := tableByForm(t, b, "06.00")
	if u := unavailableColumn(t, form06, form06SandiBMPK); u.Reason == "" {
		t.Fatal("kolom XXXVII harus dinyatakan belum tersedia beserta alasan")
	}
}
