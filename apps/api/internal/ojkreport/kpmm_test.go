package ojkreport

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func d(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

// TestKategoriRisiko menegakkan pengelompokan pos aset Form 01.00 memakai pemetaan
// COA yang ada: kewajiban/ekuitas (termasuk kode syariah 12900/13100 yang berawalan
// angka 1) TIDAK boleh dianggap aset.
func TestKategoriRisiko(t *testing.T) {
	kasus := []struct {
		coa      string
		kategori string
		aset     bool
	}{
		{"10100", RisikoKas, true},
		{"10200", RisikoAntarBank, true},
		{"10300", RisikoKredit, true},
		{"10900", RisikoKredit, true},
		{"10500", RisikoAYDA, true},
		{"10600", RisikoAsetTetap, true},
		{"10700", RisikoAsetTetap, true},
		{"10800", RisikoAntarKantor, true},
		{"10999", RisikoLainnya, true},
		{"10950", RisikoKredit, true}, // CKPN kredit: aset lawan di luar draf pemetaan
		{"11950", RisikoKredit, true}, // CKPN pembiayaan syariah: idem
		{"12900", "", false},          // Kewajiban Lainnya Syariah: bukan aset walau berawalan 1
		{"30100", "", false},          // Modal Disetor: ekuitas
		{"13100", "", false},          // Modal Disetor Syariah: ekuitas
		{"20100", "", false},          // Tabungan: kewajiban
		{"00000", "", false},          // baris penyeimbang
		{"-", "", false},              // laba/rugi berjalan
	}
	for _, k := range kasus {
		kategori, aset := KategoriRisikoCOA(k.coa)
		if aset != k.aset {
			t.Fatalf("KategoriRisikoCOA(%q) aset=%v, mau %v", k.coa, aset, k.aset)
		}
		if aset && kategori != k.kategori {
			t.Fatalf("KategoriRisikoCOA(%q) kategori=%q, mau %q", k.coa, kategori, k.kategori)
		}
	}
}

// TestHitungATMRManual memeriksa aritmetika ATMR dengan angka nyata yang dihitung
// tangan. Portofolio:
//
//	Kas 100.000.000 (0%), Penempatan 50.000.000 (20%), Kredit 1.000.000.000 net
//	CKPN -20.000.000 (100%), AYDA 10.000.000 (100%), Aset tetap 200.000.000 - akum
//	penyusutan 50.000.000 (100%), Antarkantor 5.000.000 (100%), Aset lain 1.000.000
//	(100%). ATMR = 10.000.000 + 980.000.000 + 10.000.000 + 150.000.000 + 5.000.000
//	+ 1.000.000 = 1.156.000.000.
func TestHitungATMRManual(t *testing.T) {
	rows := []domain.ReportRow{
		{AccountCode: "10100", Amount: d("100000000")},
		{AccountCode: "10200", Amount: d("50000000")},
		{AccountCode: "10300", Amount: d("1000000000")},
		{AccountCode: "10900", Amount: d("-20000000")},
		{AccountCode: "10500", Amount: d("10000000")},
		{AccountCode: "10600", Amount: d("200000000")},
		{AccountCode: "10700", Amount: d("-50000000")},
		{AccountCode: "10800", Amount: d("5000000")},
		{AccountCode: "10999", Amount: d("1000000")},
		{AccountCode: "20100", Amount: d("646000000")}, // kewajiban: diabaikan
		{AccountCode: "-", Amount: d("0")},             // penyeimbang: diabaikan
	}
	bobot := BobotRisikoATMR{
		RisikoKas:         d("0"),
		RisikoAntarBank:   d("0.20"),
		RisikoKredit:      d("1.00"),
		RisikoAYDA:        d("1.00"),
		RisikoAsetTetap:   d("1.00"),
		RisikoAntarKantor: d("1.00"),
		RisikoLainnya:     d("1.00"),
	}
	hasil := HitungATMR(rows, bobot)
	if !hasil.Total.Equal(d("1156000000")) {
		t.Fatalf("ATMR = %s, mau 1156000000", hasil.Total)
	}
	if len(hasil.TidakTerkategori) != 0 {
		t.Fatalf("tidak terkategori = %v, mau kosong", hasil.TidakTerkategori)
	}
	// Kredit net CKPN harus ikut mengurangi dasar kategori kredit.
	var kredit domain.KPMMATMRBaris
	for _, b := range hasil.Baris {
		if b.Kategori == RisikoKredit {
			kredit = b
		}
	}
	if !kredit.Dasar.Equal(d("980000000")) {
		t.Fatalf("dasar kredit = %s, mau 980000000", kredit.Dasar)
	}
}

// TestHitungATMRLaporkanCOATakTerpetakan memastikan pos bersaldo yang menyerupai
// aset belum dipetakan tidak hilang diam-diam, sedangkan pos non-aset (mis. beban
// 15901) tidak salah dilaporkan sebagai ATMR yang hilang.
func TestHitungATMRLaporkanCOATakTerpetakan(t *testing.T) {
	rows := []domain.ReportRow{
		{AccountCode: "10100", Amount: d("1000000")},
		{AccountCode: "10888", Amount: d("500000")}, // menyerupai aset, tak terpetakan
		{AccountCode: "15901", Amount: d("700000")}, // beban syariah: bukan aset
		{AccountCode: "20100", Amount: d("200000")}, // kewajiban: bukan aset
	}
	hasil := HitungATMR(rows, BobotRisikoATMR{RisikoKas: d("0")})
	if len(hasil.TidakTerkategori) != 1 || hasil.TidakTerkategori[0] != "10888" {
		t.Fatalf("tidak terkategori = %v, mau [10888]", hasil.TidakTerkategori)
	}
}

// TestPilihPengurangModalInti memeriksa pilihan dasar pengurang: per_kredit adalah
// nilai awal (konservatif), agregat memakai selisih portofolio dan tidak pernah
// negatif.
func TestPilihPengurangModalInti(t *testing.T) {
	perKredit := d("30000000")
	agregat := d("25000000")
	if got, basis := PilihPengurangModalInti("", perKredit, agregat); !got.Equal(perKredit) || basis != "per_kredit" {
		t.Fatalf("default = %s/%s, mau per_kredit 30000000", got, basis)
	}
	if got, basis := PilihPengurangModalInti("agregat", perKredit, agregat); !got.Equal(agregat) || basis != "agregat" {
		t.Fatalf("agregat = %s/%s, mau agregat 25000000", got, basis)
	}
	if got, _ := PilihPengurangModalInti("agregat", perKredit, d("-5000000")); !got.IsZero() {
		t.Fatalf("agregat negatif = %s, mau 0", got)
	}
	// Setelan tak dikenal jatuh ke per_kredit, bukan diam-diam ke agregat.
	if got, basis := PilihPengurangModalInti("entah", perKredit, agregat); !got.Equal(perKredit) || basis != "per_kredit" {
		t.Fatalf("setelan tak dikenal = %s/%s, mau per_kredit", got, basis)
	}
}
