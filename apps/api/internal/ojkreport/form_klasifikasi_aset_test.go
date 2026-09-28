package ojkreport

import (
	"context"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// TestForm06KlasifikasiAsetDariKolomPenyimpanan membuktikan kolom XLVII (Klasifikasi
// Aset Keuangan) kini terisi dari kolom sandi opsional per kredit (migrasi 000108) dan
// menulis "-" bila bank belum mengisinya. Kolom ini tidak lagi terdaftar tidak
// tersedia, dan nilainya TIDAK diturunkan dari kolektibilitas.
func TestForm06KlasifikasiAsetDariKolomPenyimpanan(t *testing.T) {
	rows := []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-ISI", OJKKlasifikasiAsetCode: "AMORTIZED_COST"},
		{Status: "DISBURSED", LoanNumber: "LN-KOSONG"},
	}
	sec := buildForm06(rows, true)

	if got := findCell(t, sec, "LN-ISI", form06SandiKlasifikasi).Value; got != "AMORTIZED_COST" {
		t.Errorf("kolom XLVII terisi = %q, ingin AMORTIZED_COST", got)
	}
	if got := findCell(t, sec, "LN-KOSONG", form06SandiKlasifikasi).Value; got != "-" {
		t.Errorf("kolom XLVII kosong = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiKlasifikasi {
			t.Errorf("kolom XLVII masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}

// TestForm05KlasifikasiAsetDariKolomPenyimpanan membuktikan kolom XX (Klasifikasi
// Aset Keuangan) Form 05.00 terisi dari lps_placements.ojk_klasifikasi_aset_code dan
// menulis "-" bila belum diisi.
func TestForm05KlasifikasiAsetDariKolomPenyimpanan(t *testing.T) {
	rows := []PlacementRow{
		{
			CounterpartyBank:       "Bank Berisi",
			PlacementType:          "DEPOSITO",
			Collectibility:         "LANCAR",
			Outstanding:            decimal.NewFromInt(1_000_000),
			OJKKlasifikasiAsetCode: "FAIR_VALUE",
		},
		{
			CounterpartyBank: "Bank Kosong",
			PlacementType:    "GIRO",
			Collectibility:   "LANCAR",
			Outstanding:      decimal.NewFromInt(500_000),
		},
	}
	sec := buildForm05(rows)

	if got := findCell(t, sec, "Bank Berisi", form05SandiKlasifikasi).Value; got != "FAIR_VALUE" {
		t.Errorf("kolom XX terisi = %q, ingin FAIR_VALUE", got)
	}
	if got := findCell(t, sec, "Bank Kosong", form05SandiKlasifikasi).Value; got != "-" {
		t.Errorf("kolom XX kosong = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form05SandiKlasifikasi {
			t.Errorf("kolom XX masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}

// TestForm06KolomKBPakaiAlasanKebijakan membuktikan empat kolom kondisional bank
// (VI, XXXIX, XL, XLIII) menyatakan KEBIJAKAN secara eksplisit: bank bukan peserta
// secara bawaan, partisipasi dikonfirmasi saat onboarding, dan kolom ditulis "-"
// selama bukan peserta. Tidak ada saklar config yang dirujuk karena modul KUR/LPBBTI
// belum dibangun (kunci config tanpa pembaca hanya menjadi sampah).
func TestForm06KolomKBPakaiAlasanKebijakan(t *testing.T) {
	sec := buildForm06([]LoanRow{{Status: "DISBURSED", LoanNumber: "LN-1"}}, true)

	for _, sandi := range []string{form06SandiJenis, form06SandiProgram, form06SandiSektorKUR, form06SandiLPBBTI} {
		u := unavailableColumn(t, sec, sandi)
		if !strings.Contains(u.Reason, "onboarding") {
			t.Errorf("alasan kolom %s = %q, harus menyebut onboarding", sandi, u.Reason)
		}
		if !strings.Contains(u.Reason, "bukan peserta") {
			t.Errorf("alasan kolom %s = %q, harus menyatakan bank bukan peserta secara bawaan", sandi, u.Reason)
		}
	}
}

// TestListLoansForOJKMengisiKlasifikasiAset menutup rantai adaptor kredit: sandi
// klasifikasi aset per kredit dipetakan dari baris kredit tanpa query tambahan.
func TestListLoansForOJKMengisiKlasifikasiAset(t *testing.T) {
	repo := &loanAggStub{
		loans: []domain.Loan{{
			LoanNumber:             "LN-KLAS",
			Status:                 domain.LoanStatusDisbursed,
			OJKKlasifikasiAsetCode: "FVTPL",
		}},
	}
	src := RepoSource{Loans: repo}

	rows, err := src.ListLoansForOJK(context.Background(), time.Now().UTC(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListLoansForOJK: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(rows))
	}
	if rows[0].OJKKlasifikasiAsetCode != "FVTPL" {
		t.Errorf("klasifikasi aset kredit = %q, ingin FVTPL", rows[0].OJKKlasifikasiAsetCode)
	}
}

// placementListStub menyediakan penempatan bank-wide untuk uji adaptor tanpa basis data.
type placementListStub struct {
	domain.LPSPlacementRepository
	items []domain.LPSPlacement
}

func (s placementListStub) ListPlacements(context.Context, time.Time, domain.Actor) ([]domain.LPSPlacement, error) {
	return s.items, nil
}

// TestListPlacementsForOJKMemetakanKlasifikasiAset menutup rantai adaptor penempatan:
// sandi klasifikasi aset per penempatan dipetakan dari baris lps_placements.
func TestListPlacementsForOJKMemetakanKlasifikasiAset(t *testing.T) {
	src := RepoSource{Placements: placementListStub{items: []domain.LPSPlacement{{
		CounterpartyBank:       "Bank Lawan",
		PlacementType:          domain.LPSPlacementDeposito,
		Collectibility:         domain.LPSLancar,
		OJKKlasifikasiAsetCode: "AMORTIZED_COST",
	}}}}

	rows, err := src.ListPlacementsForOJK(context.Background(), time.Now().UTC(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListPlacementsForOJK: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(rows))
	}
	if rows[0].OJKKlasifikasiAsetCode != "AMORTIZED_COST" {
		t.Errorf("klasifikasi aset penempatan = %q, ingin AMORTIZED_COST", rows[0].OJKKlasifikasiAsetCode)
	}
}
