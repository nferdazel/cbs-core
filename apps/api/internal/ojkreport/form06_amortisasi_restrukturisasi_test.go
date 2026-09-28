package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// TestForm06AmortisasiRestrukturisasiDariKolomPenyimpanan membuktikan kolom XXIX–XXXII
// Form 06.00 terisi dari kolom penyimpanan nullable per kredit (migrasi 000111) dan
// ditulis "-" bila belum diisi. Nol yang benar-benar tersimpan tetap ditulis "0",
// berbeda dari NULL; keempat kolom tidak lagi terdaftar tidak tersedia dan TIDAK
// diturunkan dari rumus amortisasi karangan.
func TestForm06AmortisasiRestrukturisasiDariKolomPenyimpanan(t *testing.T) {
	rows := []LoanRow{
		{
			Status:                                   "DISBURSED",
			LoanNumber:                               "LN-ISI",
			OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "200000"),
			OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
			OJKPendapatanBungaDitangguhkanAmount:     decimalPtr(t, "300000"),
			OJKCadanganKerugianRestrukturisasiAmount: decimalPtr(t, "400000"),
		},
		{
			Status:                                   "DISBURSED",
			LoanNumber:                               "LN-NOL",
			OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "0"),
			OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "0"),
			OJKPendapatanBungaDitangguhkanAmount:     decimalPtr(t, "0"),
			OJKCadanganKerugianRestrukturisasiAmount: decimalPtr(t, "0"),
		},
		{Status: "DISBURSED", LoanNumber: "LN-KOSONG"},
	}
	sec := buildForm06(rows, true)

	cases := []struct {
		sandi  string
		terisi string
	}{
		{form06SandiProvisi, "200000"},
		{form06SandiBiayaTrans, "50000"},
		{form06SandiBungaTangguh, "300000"},
		{form06SandiCadRestru, "400000"},
	}
	for _, c := range cases {
		if got := findCell(t, sec, "LN-ISI", c.sandi).Value; got != c.terisi {
			t.Errorf("kolom %s terisi = %q, ingin %q", c.sandi, got, c.terisi)
		}
		if got := findCell(t, sec, "LN-NOL", c.sandi).Value; got != "0" {
			t.Errorf("kolom %s nol = %q, ingin 0 (bukan -)", c.sandi, got)
		}
		if got := findCell(t, sec, "LN-KOSONG", c.sandi).Value; got != "-" {
			t.Errorf("kolom %s kosong = %q, ingin -", c.sandi, got)
		}
		for _, u := range sec.Unavailable {
			if u.Sandi == c.sandi {
				t.Errorf("kolom %s masih terdaftar tidak tersedia: %s", c.sandi, u.Reason)
			}
		}
	}
}

// TestForm06BakiDebetNeto membuktikan kolom XXXIII dihitung menurut definisi resmi
// (baki debet - provisi belum diamortisasi + biaya transaksi belum diamortisasi -
// pendapatan bunga ditangguhkan restrukturisasi - cadangan kerugian restrukturisasi),
// dan ditulis "-" bila komponen yang wajib belum tersimpan — bukan dianggap nol.
func TestForm06BakiDebetNeto(t *testing.T) {
	rows := []LoanRow{
		{
			// Tidak direstrukturisasi: komponen restrukturisasi memang nol.
			Status:                                   "DISBURSED",
			LoanNumber:                               "LN-NONRESTRUK",
			Outstanding:                              decimal.RequireFromString("10000000"),
			OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "200000"),
			OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
		},
		{
			// Direstrukturisasi: semua komponen tersimpan.
			Status:                                   "DISBURSED",
			LoanNumber:                               "LN-RESTRUK",
			IsRestructured:                           true,
			Outstanding:                              decimal.RequireFromString("10000000"),
			OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "200000"),
			OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
			OJKPendapatanBungaDitangguhkanAmount:     decimalPtr(t, "300000"),
			OJKCadanganKerugianRestrukturisasiAmount: decimalPtr(t, "400000"),
		},
		{
			// Direstrukturisasi tetapi cadangan restrukturisasi belum diisi: "-".
			Status:                                   "DISBURSED",
			LoanNumber:                               "LN-RESTRUK-MENTAH",
			IsRestructured:                           true,
			Outstanding:                              decimal.RequireFromString("10000000"),
			OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "200000"),
			OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
			OJKPendapatanBungaDitangguhkanAmount:     decimalPtr(t, "0"),
		},
		{
			// Provisi belum diisi: "-" walau komponen lain ada.
			Status:                                   "DISBURSED",
			LoanNumber:                               "LN-TANPA-PROVISI",
			Outstanding:                              decimal.RequireFromString("10000000"),
			OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
		},
	}
	sec := buildForm06(rows, true)

	if got := findCell(t, sec, "LN-NONRESTRUK", form06SandiBakiNeto).Value; got != "9850000" {
		t.Errorf("baki debet neto non-restrukturisasi = %q, ingin 9850000", got)
	}
	if got := findCell(t, sec, "LN-RESTRUK", form06SandiBakiNeto).Value; got != "9150000" {
		t.Errorf("baki debet neto restrukturisasi = %q, ingin 9150000", got)
	}
	if got := findCell(t, sec, "LN-RESTRUK-MENTAH", form06SandiBakiNeto).Value; got != "-" {
		t.Errorf("baki debet neto komponen belum lengkap = %q, ingin -", got)
	}
	if got := findCell(t, sec, "LN-TANPA-PROVISI", form06SandiBakiNeto).Value; got != "-" {
		t.Errorf("baki debet neto tanpa provisi = %q, ingin -", got)
	}
	for _, u := range sec.Unavailable {
		if u.Sandi == form06SandiBakiNeto {
			t.Errorf("kolom XXXIII masih terdaftar tidak tersedia: %s", u.Reason)
		}
	}
}

// TestListLoansForOJKMengisiAmortisasiRestrukturisasi menutup rantai adaptor kredit:
// keempat komponen amortisasi/restrukturisasi per kredit dipetakan dari baris kredit
// tanpa query tambahan (ikut kolom kredit yang sudah ada).
func TestListLoansForOJKMengisiAmortisasiRestrukturisasi(t *testing.T) {
	repo := &loanAggStub{loans: []domain.Loan{{
		LoanNumber:                               "LN-AMORT",
		Status:                                   domain.LoanStatusDisbursed,
		OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "200000"),
		OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
		OJKPendapatanBungaDitangguhkanAmount:     decimalPtr(t, "300000"),
		OJKCadanganKerugianRestrukturisasiAmount: decimalPtr(t, "400000"),
	}}}
	src := RepoSource{Loans: repo}

	rows, err := src.ListLoansForOJK(context.Background(), time.Now().UTC(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListLoansForOJK: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(rows))
	}
	if rows[0].OJKProvisiBelumDiamortisasiAmount == nil || !rows[0].OJKProvisiBelumDiamortisasiAmount.Equal(decimal.NewFromInt(200_000)) {
		t.Errorf("provisi belum diamortisasi = %v, ingin 200000", rows[0].OJKProvisiBelumDiamortisasiAmount)
	}
	if rows[0].OJKBiayaTransaksiBelumDiamortisasiAmount == nil || !rows[0].OJKBiayaTransaksiBelumDiamortisasiAmount.Equal(decimal.NewFromInt(50_000)) {
		t.Errorf("biaya transaksi belum diamortisasi = %v, ingin 50000", rows[0].OJKBiayaTransaksiBelumDiamortisasiAmount)
	}
	if rows[0].OJKPendapatanBungaDitangguhkanAmount == nil || !rows[0].OJKPendapatanBungaDitangguhkanAmount.Equal(decimal.NewFromInt(300_000)) {
		t.Errorf("pendapatan ditangguhkan = %v, ingin 300000", rows[0].OJKPendapatanBungaDitangguhkanAmount)
	}
	if rows[0].OJKCadanganKerugianRestrukturisasiAmount == nil || !rows[0].OJKCadanganKerugianRestrukturisasiAmount.Equal(decimal.NewFromInt(400_000)) {
		t.Errorf("cadangan kerugian restrukturisasi = %v, ingin 400000", rows[0].OJKCadanganKerugianRestrukturisasiAmount)
	}
}
