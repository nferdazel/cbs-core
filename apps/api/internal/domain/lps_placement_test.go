package domain_test

import (
	"errors"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// lpsPlacement membangun penempatan uji dengan nilai dan kualitas yang diberikan.
func lpsPlacement(outstanding, guaranteed int64, k domain.LPSPlacementCollectibility) domain.LPSPlacement {
	return domain.LPSPlacement{
		COACode:          "10200",
		CounterpartyBank: "Bank Y",
		PlacementType:    domain.LPSPlacementDeposito,
		Outstanding:      decimal.NewFromInt(outstanding),
		LPSGuaranteed:    decimal.NewFromInt(guaranteed),
		Collectibility:   k,
	}
}

// Contoh 1 Penjelasan Pasal 23 POJK No. 1 Tahun 2024: penempatan Rp10 miliar pada satu
// bank, dijamin LPS Rp2 miliar, kualitas Lancar -> PPKA umum = 0,5% x 8 miliar = 40 juta.
// Uji ini sekaligus membuktikan pengurang diterapkan pada DASAR PPKA umum.
func TestLPSPlacementPPKA_PengurangDiterapkanPadaPPKAUmum(t *testing.T) {
	calc, err := domain.CalculateLPSPlacementPPKA(
		lpsPlacement(10_000_000_000, 2_000_000_000, domain.LPSLancar), domain.DefaultPPAPRates())
	if err != nil {
		t.Fatalf("hitung PPKA penempatan: %v", err)
	}
	if !calc.Deduction.Equal(decimal.NewFromInt(2_000_000_000)) {
		t.Fatalf("pengurang LPS %s, mau 2000000000", calc.Deduction)
	}
	if !calc.Base.Equal(decimal.NewFromInt(8_000_000_000)) {
		t.Fatalf("dasar PPKA %s, mau 8000000000", calc.Base)
	}
	if !calc.GeneralPPKA.Equal(decimal.NewFromInt(40_000_000)) {
		t.Fatalf("PPKA umum %s, mau 40000000", calc.GeneralPPKA)
	}
	if calc.AppliesTo != "UMUM" || !calc.PKKATarget.Equal(decimal.NewFromInt(40_000_000)) {
		t.Fatalf("PPKA berlaku %s senilai %s, mau UMUM 40000000", calc.AppliesTo, calc.PKKATarget)
	}
}

// Contoh 2 Penjelasan Pasal 23: data yang sama dengan kualitas Kurang Lancar menghasilkan
// PPKA khusus = 10% x 8 miliar = 800 juta. Untuk Macet, tarif 100% x 8 miliar = 8 miliar.
// Uji ini membuktikan pengurang diterapkan pada DASAR PPKA khusus.
func TestLPSPlacementPPKA_PengurangDiterapkanPadaPPKAKhusus(t *testing.T) {
	khusus, err := domain.CalculateLPSPlacementPPKA(
		lpsPlacement(10_000_000_000, 2_000_000_000, domain.LPSKurangLancar), domain.DefaultPPAPRates())
	if err != nil {
		t.Fatalf("hitung PPKA penempatan kurang lancar: %v", err)
	}
	if !khusus.Base.Equal(decimal.NewFromInt(8_000_000_000)) {
		t.Fatalf("dasar PPKA khusus %s, mau 8000000000", khusus.Base)
	}
	if !khusus.SpecificPPKA.Equal(decimal.NewFromInt(800_000_000)) {
		t.Fatalf("PPKA khusus %s, mau 800000000", khusus.SpecificPPKA)
	}
	if khusus.AppliesTo != "KHUSUS" || !khusus.PKKATarget.Equal(decimal.NewFromInt(800_000_000)) {
		t.Fatalf("PPKA berlaku %s senilai %s, mau KHUSUS 800000000", khusus.AppliesTo, khusus.PKKATarget)
	}

	macet, err := domain.CalculateLPSPlacementPPKA(
		lpsPlacement(10_000_000_000, 2_000_000_000, domain.LPSMacet), domain.DefaultPPAPRates())
	if err != nil {
		t.Fatalf("hitung PPKA penempatan macet: %v", err)
	}
	if !macet.PKKATarget.Equal(decimal.NewFromInt(8_000_000_000)) {
		t.Fatalf("PPKA khusus macet %s, mau 8000000000", macet.PKKATarget)
	}
}

// Pengurang tidak boleh melebihi nilai penempatannya dan tidak boleh menghasilkan dasar
// (karenanya cadangan) negatif. Nilai jaminan yang melebihi penempatan dijepit ke penuh.
func TestLPSPlacementDeduction_TidakMelebihiNilaiPenempatan(t *testing.T) {
	if got := domain.LPSPlacementDeduction(decimal.NewFromInt(5_000_000), decimal.NewFromInt(8_000_000)); !got.Equal(decimal.NewFromInt(5_000_000)) {
		t.Fatalf("pengurang %s, mau dijepit ke 5000000", got)
	}
	if got := domain.LPSPlacementDeduction(decimal.NewFromInt(5_000_000), decimal.NewFromInt(-1)); !got.IsZero() {
		t.Fatalf("pengurang negatif %s, mau nol", got)
	}
	if got := domain.LPSPlacementDeduction(decimal.NewFromInt(-1), decimal.NewFromInt(2)); !got.IsZero() {
		t.Fatalf("penempatan negatif %s, mau nol", got)
	}
}

// Jaminan yang sama besar dengan penempatan menghasilkan dasar nol (bukan negatif), dan
// penempatan tanpa jaminan tidak berkurang sama sekali.
func TestLPSPlacementPPKA_TidakMenghasilkanCadanganNegatif(t *testing.T) {
	penuh, err := domain.CalculateLPSPlacementPPKA(
		lpsPlacement(1_000_000, 1_000_000, domain.LPSLancar), domain.DefaultPPAPRates())
	if err != nil {
		t.Fatalf("hitung PPKA penempatan dijamin penuh: %v", err)
	}
	if !penuh.Base.IsZero() || !penuh.PKKATarget.IsZero() {
		t.Fatalf("dasar %s target %s, mau nol", penuh.Base, penuh.PKKATarget)
	}

	tanpaJaminan, err := domain.CalculateLPSPlacementPPKA(
		lpsPlacement(1_000_000, 0, domain.LPSLancar), domain.DefaultPPAPRates())
	if err != nil {
		t.Fatalf("hitung PPKA penempatan tanpa jaminan: %v", err)
	}
	if !tanpaJaminan.Deduction.IsZero() {
		t.Fatalf("pengurang %s, mau nol", tanpaJaminan.Deduction)
	}
	if !tanpaJaminan.Base.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("dasar %s, mau penuh 1000000", tanpaJaminan.Base)
	}
	if !tanpaJaminan.PKKATarget.Equal(decimal.NewFromInt(5_000)) {
		t.Fatalf("PPKA target %s, mau 5000", tanpaJaminan.PKKATarget)
	}
}

// Data penempatan yang tidak sah harus ditolak dengan pesan yang menyebut alasannya,
// bukan dihitung dengan nilai nol.
func TestLPSPlacementValidate_MenolakDataTidakSah(t *testing.T) {
	valid := lpsPlacement(1_000_000, 500_000, domain.LPSLancar)
	if err := valid.Validate(); err != nil {
		t.Fatalf("data yang sah ditolak: %v", err)
	}

	kasus := []struct {
		nama  string
		ubah  func(*domain.LPSPlacement)
		pesan string
	}{
		{"bank lawan kosong", func(p *domain.LPSPlacement) { p.CounterpartyBank = "  " }, "bank lawan"},
		{"jenis tak dikenal", func(p *domain.LPSPlacement) { p.PlacementType = "SAHAM" }, "jenis penempatan"},
		{"kualitas tak dikenal", func(p *domain.LPSPlacement) { p.Collectibility = "DPK" }, "kualitas"},
		{"nilai negatif", func(p *domain.LPSPlacement) { p.Outstanding = decimal.NewFromInt(-1) }, "negatif"},
		{"jaminan negatif", func(p *domain.LPSPlacement) { p.LPSGuaranteed = decimal.NewFromInt(-1) }, "negatif"},
		{"jaminan melebihi penempatan", func(p *domain.LPSPlacement) { p.LPSGuaranteed = decimal.NewFromInt(2_000_000) }, "melebihi"},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			p := valid
			k.ubah(&p)
			err := p.Validate()
			if !errors.Is(err, domain.ErrLPSPlacementInvalid) {
				t.Fatalf("error %v, mau ErrLPSPlacementInvalid", err)
			}
			if err == nil || !strings.Contains(err.Error(), k.pesan) {
				t.Fatalf("pesan %q tidak memuat %q", err, k.pesan)
			}
		})
	}
}

// Kualitas PABL hanya tiga golongan (Pasal 15 ayat (1)); pemetaannya ke mesin PPAP harus
// tepat agar tarif yang dikenakan benar.
func TestLPSPlacementCollectibility_ToCollectibility(t *testing.T) {
	if got := domain.LPSLancar.ToCollectibility(); got != domain.KolLancar {
		t.Fatalf("Lancar -> %v, mau KolLancar", got)
	}
	if got := domain.LPSKurangLancar.ToCollectibility(); got != domain.KolKurangLancar {
		t.Fatalf("Kurang Lancar -> %v, mau KolKurangLancar", got)
	}
	if got := domain.LPSMacet.ToCollectibility(); got != domain.KolMacet {
		t.Fatalf("Macet -> %v, mau KolMacet", got)
	}
}
