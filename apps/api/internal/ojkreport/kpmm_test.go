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

// TestBatasModalPelengkap membuktikan sub-batas POJK No. 5/POJK.03/2015 dipotong
// dengan angka: komponen ber-instrumen paling tinggi 50% modal inti (Pasal 10 ayat
// (2)), PPKA umum paling tinggi 1,25% ATMR (Pasal 10 ayat (1) huruf c), dan total
// modal pelengkap paling tinggi 100% modal inti (Pasal 3 ayat (2)).
func TestBatasModalPelengkap(t *testing.T) {
	cases := []struct {
		name                                      string
		instrumen, surplus, ppka, modalInti, atmr string
		pelengkapMax, instrumenMax, ppkaMax       string
		wantInstrumen, wantSurplus, wantPPKA      string
		wantTotal, wantPotonganInstrumen          string
		wantPotonganPPKA, wantPotonganTotal       string
	}{
		{
			// Instrumen 100jt, modal inti 100jt: batas 50% = 50jt, jadi dipotong 50jt;
			// total 50jt masih di bawah batas 100% (100jt).
			name:      "instrumen dipotong 50 persen modal inti",
			instrumen: "100000000", surplus: "0", ppka: "0",
			modalInti: "100000000", atmr: "1000000000",
			pelengkapMax: "1.00", instrumenMax: "0.50", ppkaMax: "0.0125",
			wantInstrumen: "50000000", wantSurplus: "0", wantPPKA: "0",
			wantTotal: "50000000", wantPotonganInstrumen: "50000000",
			wantPotonganPPKA: "0", wantPotonganTotal: "0",
		},
		{
			// Instrumen (dibatasi 50jt) + surplus 70jt = 120jt melebihi batas total
			// 100% modal inti (100jt), jadi total dipotong 20jt.
			name:      "total pelengkap dipotong 100 persen modal inti",
			instrumen: "100000000", surplus: "70000000", ppka: "0",
			modalInti: "100000000", atmr: "1000000000",
			pelengkapMax: "1.00", instrumenMax: "0.50", ppkaMax: "0.0125",
			wantInstrumen: "50000000", wantSurplus: "70000000", wantPPKA: "0",
			wantTotal: "100000000", wantPotonganInstrumen: "50000000",
			wantPotonganPPKA: "0", wantPotonganTotal: "20000000",
		},
		{
			// PPKA umum 20jt terhadap ATMR 1M: batas 1,25% = 12,5jt, dipotong 7,5jt.
			name:      "ppka umum dipotong 1,25 persen atmr",
			instrumen: "0", surplus: "0", ppka: "20000000",
			modalInti: "100000000", atmr: "1000000000",
			pelengkapMax: "1.00", instrumenMax: "0.50", ppkaMax: "0.0125",
			wantInstrumen: "0", wantSurplus: "0", wantPPKA: "12500000",
			wantTotal: "12500000", wantPotonganInstrumen: "0",
			wantPotonganPPKA: "7500000", wantPotonganTotal: "0",
		},
		{
			// Tidak ada komponen: semua nol, tanpa potongan.
			name:      "tanpa komponen",
			instrumen: "0", surplus: "0", ppka: "0",
			modalInti: "100000000", atmr: "1000000000",
			pelengkapMax: "1.00", instrumenMax: "0.50", ppkaMax: "0.0125",
			wantInstrumen: "0", wantSurplus: "0", wantPPKA: "0",
			wantTotal: "0", wantPotonganInstrumen: "0",
			wantPotonganPPKA: "0", wantPotonganTotal: "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := BatasModalPelengkap(
				d(tc.instrumen), d(tc.surplus), d(tc.ppka), d(tc.modalInti), d(tc.atmr),
				d(tc.pelengkapMax), d(tc.instrumenMax), d(tc.ppkaMax),
			)
			if !h.Instrumen.Equal(d(tc.wantInstrumen)) || !h.SurplusRevaluasi.Equal(d(tc.wantSurplus)) || !h.PPKAUmum.Equal(d(tc.wantPPKA)) {
				t.Fatalf("komponen = instrumen %s surplus %s ppka %s, mau %s/%s/%s",
					h.Instrumen, h.SurplusRevaluasi, h.PPKAUmum, tc.wantInstrumen, tc.wantSurplus, tc.wantPPKA)
			}
			if !h.TotalEfektif.Equal(d(tc.wantTotal)) {
				t.Fatalf("total efektif = %s, mau %s", h.TotalEfektif, tc.wantTotal)
			}
			if !h.PotonganInstrumen.Equal(d(tc.wantPotonganInstrumen)) ||
				!h.PotonganPPKAUmum.Equal(d(tc.wantPotonganPPKA)) ||
				!h.PotonganTotal.Equal(d(tc.wantPotonganTotal)) {
				t.Fatalf("potongan = instrumen %s ppka %s total %s, mau %s/%s/%s",
					h.PotonganInstrumen, h.PotonganPPKAUmum, h.PotonganTotal,
					tc.wantPotonganInstrumen, tc.wantPotonganPPKA, tc.wantPotonganTotal)
			}
		})
	}
}

// TestKlasifikasiModalCOATidakMenghasilkanModalFiktif membuktikan kode COA yang
// kelas modalnya belum pasti (di luar pemetaan atau kelasnya kosong) TIDAK
// menghasilkan entri modal sama sekali, bukan nol. Hanya saldo akun berkelas pasti
// yang diperhitungkan.
func TestKlasifikasiModalCOATidakMenghasilkanModalFiktif(t *testing.T) {
	rows := []domain.ReportRow{
		{AccountCode: "30100", Amount: d("500000000")},  // Modal Disetor -> INTI_UTAMA
		{AccountCode: "29999", Amount: d("9999999999")}, // tak terpetakan: bukan modal
		{AccountCode: "10500", Amount: d("10000000")},   // AYDA: kelas belum pasti -> kosong
		{AccountCode: "20100", Amount: d("12345")},      // kewajiban: bukan modal
	}
	kelas := KlasifikasiModalCOA(rows)
	if len(kelas) != 1 {
		t.Fatalf("kelas = %v, mau hanya INTI_UTAMA", kelas)
	}
	if !kelas[ModalClassIntiUtama].Equal(d("500000000")) {
		t.Fatalf("INTI_UTAMA = %s, mau 500000000", kelas[ModalClassIntiUtama])
	}
	if got := ModalClassCOA("10500"); got != "" {
		t.Fatalf("ModalClassCOA(10500) = %q, mau kosong (umur AYDA tidak tersedia)", got)
	}
	if got := ModalClassCOA("29999"); got != "" {
		t.Fatalf("ModalClassCOA(29999) = %q, mau kosong", got)
	}
}
