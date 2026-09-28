package ojkreport

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// loanRowPosisiForm06 adalah satu kredit dengan komponen kolom materiil Form 06.00
// yang lengkap, dipakai membedakan perilaku posisi bulan berjalan dan periode lampau.
func loanRowPosisiForm06(t *testing.T) LoanRow {
	t.Helper()
	return LoanRow{
		Status:                                   "DISBURSED",
		LoanNumber:                               "LN-POSISI",
		Outstanding:                              decimal.NewFromInt(5_000_000),
		RequiredCKPN:                             decimal.NewFromInt(250_000),
		AccruedProfit:                            decimal.NewFromInt(75_000),
		OverdueUnpaid:                            decimal.NewFromInt(1_250_000),
		OJKProvisiBelumDiamortisasiAmount:        decimalPtr(t, "200000"),
		OJKBiayaTransaksiBelumDiamortisasiAmount: decimalPtr(t, "50000"),
	}
}

// Periode bulan berjalan: kolom XIII angsuran tetap tersedia dan kolom materiil
// XVII/XXVIII/XXXIII/XXXIV/XXXV memuat angka keadaan kini.
func TestBuildForm06PosisiBulanBerjalanMengisiKolomMateriil(t *testing.T) {
	sec := buildForm06([]LoanRow{loanRowPosisiForm06(t)}, true)

	cases := []struct{ sandi, want string }{
		{form06SandiNominalTungg, "1250000"},
		{form06SandiBakiDebet, "5000000"},
		{form06SandiBakiNeto, "4850000"}, // 5.000.000 - 200.000 + 50.000
		{form06SandiCKPN, "250000"},
		{form06SandiBungaAkanTer, "75000"},
	}
	for _, c := range cases {
		if got := findCell(t, sec, "LN-POSISI", c.sandi).Value; got != c.want {
			t.Errorf("kolom %s = %q, ingin %q", c.sandi, got, c.want)
		}
		for _, u := range sec.Unavailable {
			if u.Sandi == c.sandi {
				t.Errorf("kolom %s seharusnya tersedia pada bulan berjalan: %s", c.sandi, u.Reason)
			}
		}
	}
}

// Periode lampau: kelima kolom materiil dinyatakan tidak tersedia beserta alasannya,
// bukan diisi angka keadaan kini.
func TestBuildForm06PosisiPeriodeLampauMenulisKolomMateriilTidakTersedia(t *testing.T) {
	sec := buildForm06([]LoanRow{loanRowPosisiForm06(t)}, false)

	// Baris tetap dilaporkan walau kolom materiil tidak tersedia.
	_ = rowByKey(t, sec, "LN-POSISI")

	for _, sandi := range []string{
		form06SandiNominalTungg, form06SandiBakiDebet, form06SandiBakiNeto,
		form06SandiCKPN, form06SandiBungaAkanTer,
	} {
		u := unavailableColumn(t, sec, sandi)
		if u.Reason != form06AlasanPosisiPeriodeLampau {
			t.Errorf("alasan kolom %s = %q, ingin %q", sandi, u.Reason, form06AlasanPosisiPeriodeLampau)
		}
		for _, col := range sec.Columns {
			if col.Sandi == sandi {
				t.Errorf("kolom %s tidak boleh disajikan pada periode lampau", sandi)
			}
		}
	}
}

// Kolom XIV Kualitas dan XVI DPD selalu tidak tersedia beserta alasannya, baik pada
// posisi bulan berjalan maupun periode lampau.
func TestForm06KualitasDanDPDSelaluTidakTersedia(t *testing.T) {
	for _, posisiKini := range []bool{true, false} {
		sec := buildForm06([]LoanRow{
			{Status: "DISBURSED", LoanNumber: "LN-POSISI", DPD: 45, Collectibility: "3_KURANG_LANCAR"},
		}, posisiKini)

		for _, sandi := range []string{form06SandiKualitas, form06SandiHariTunggakan} {
			u := unavailableColumn(t, sec, sandi)
			if u.Reason != form06AlasanRiwayatKualitasDPD {
				t.Errorf("posisiKini=%v: alasan kolom %s = %q, ingin %q", posisiKini, sandi, u.Reason, form06AlasanRiwayatKualitasDPD)
			}
		}
	}
}

// Form 06.00 tetap terbangun pada periode lampau: barisnya muncul, kolom materiil
// tidak tersedia, dan form tidak dicatat sebagai form yang dilewati.
func TestGenerateMonthlyPeriodeLampauForm06TetapTerbangun(t *testing.T) {
	src := &ojkStubSource{
		stubSource: newStubSource(),
		loans:      []LoanRow{loanRowPosisiForm06(t)},
	}
	builder := NewBuilder(src)
	builder.now = func() time.Time { return time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC) }

	b, err := builder.GenerateMonthly(context.Background(), time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "")
	if err != nil {
		t.Fatalf("GenerateMonthly: %v", err)
	}

	form06 := tableByForm(t, b, "06.00")
	if len(form06.Rows) == 0 {
		t.Fatal("Form 06.00 pada periode lampau tetap harus memuat baris")
	}
	if u := unavailableColumn(t, form06, form06SandiBakiDebet); u.Reason != form06AlasanPosisiPeriodeLampau {
		t.Errorf("alasan baki debet = %q, ingin %q", u.Reason, form06AlasanPosisiPeriodeLampau)
	}
	for _, f := range b.SkippedForms {
		if f.Form == "06.00" {
			t.Fatalf("Form 06.00 tidak boleh dicatat sebagai form yang dilewati: %s", f.UnavailableReason)
		}
	}
}
