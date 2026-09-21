package domain_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// pasal20AsOf adalah tanggal acuan uji kepatuhan Pasal 20/21 POJK No. 1 Tahun 2024.
var pasal20AsOf = time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

// agunanTanah membuat agunan tanah/bangunan bersertifikat ber-hak tanggungan yang laik
// jadi pengurang, sebagai titik awal tiap kasus uji.
func agunanTanah() domain.LoanCollateral {
	return domain.LoanCollateral{
		ID:             uuid.New(),
		LoanID:         uuid.New(),
		CollateralType: domain.CollateralTanahBangunan,
		AppraisalValue: decimal.NewFromInt(1_000_000_000),
		AppraisalDate:  pasal20AsOf.AddDate(0, 0, -30),
		HaircutPercent: decimal.Zero,
		Status:         domain.CollateralActive,
		Certified:      true,
		Mortgaged:      true,
		ExistsKnown:    true,
		Executable:     true,
	}
}

func TestPasal20Rate_MemetaTarifPerJenis(t *testing.T) {
	cases := []struct {
		name      string
		ubah      func(*domain.LoanCollateral)
		want      string
		wantDapat bool
	}{
		{name: "tanah bersertifikat ber-hak tanggungan 80%", want: "0.8", wantDapat: true},
		{
			name:      "tanah bersertifikat tanpa hak tanggungan 60%",
			ubah:      func(c *domain.LoanCollateral) { c.Mortgaged = false },
			want:      "0.6",
			wantDapat: true,
		},
		{
			name: "tanah belum bersertifikat di luar daftar",
			ubah: func(c *domain.LoanCollateral) { c.Certified = false },
		},
		{
			name: "kendaraan ber-hipotek 50%",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralKendaraan
			},
			want:      "0.5",
			wantDapat: true,
		},
		{
			name: "mesin/peralatan ber-hipotek 50%",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralMesinPeralatan
			},
			want:      "0.5",
			wantDapat: true,
		},
		{
			name: "kendaraan tanpa hipotek di luar daftar",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralKendaraan
				c.Mortgaged = false
			},
		},
		{
			name: "agunan lain dinilai penilai independen 20%",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralLainnya
				c.Certified = false
				c.Mortgaged = false
				c.AppraiserIndependent = true
			},
			want:      "0.2",
			wantDapat: true,
		},
		{
			name: "agunan lain tanpa penilai independen di luar daftar",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralLainnya
				c.Certified = false
				c.Mortgaged = false
			},
		},
		{
			name: "agunan lain dengan penilaian kedaluwarsa di luar daftar",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralLainnya
				c.Certified = false
				c.Mortgaged = false
				c.AppraiserIndependent = true
				c.AppraisalDate = pasal20AsOf.AddDate(-2, 0, 0)
			},
		},
		{
			name: "deposito tidak tercantum pada ayat (1)",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralDeposit
			},
		},
		{
			name: "jenis tak dikenal tidak tercantum pada ayat (1)",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralType("TIDAK_DIKENAL")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			if tc.ubah != nil {
				tc.ubah(&c)
			}
			rate, ok := domain.Pasal20Rate(&c, pasal20AsOf)
			if ok != tc.wantDapat {
				t.Fatalf("dapat=%v, mau %v", ok, tc.wantDapat)
			}
			if !tc.wantDapat {
				return
			}
			want, _ := decimal.NewFromString(tc.want)
			if !rate.Equal(want) {
				t.Fatalf("tarif %s, mau %s", rate, want)
			}
		})
	}
}

func TestPasal20TimeFactor_PenurunanMenurutWaktu(t *testing.T) {
	tahunLalu := func(n int) *time.Time {
		t := pasal20AsOf.AddDate(-n, 0, 0)
		return &t
	}
	bulanLalu := func(n int) *time.Time {
		t := pasal20AsOf.AddDate(0, -n, 0)
		return &t
	}

	cases := []struct {
		name       string
		jenis      domain.CollateralType
		macetAt    *time.Time
		wantFaktor string
	}{
		{name: "tanah 1 tahun masih penuh", jenis: domain.CollateralTanahBangunan, macetAt: tahunLalu(1), wantFaktor: "1"},
		{name: "tanah 3 tahun turun 50%", jenis: domain.CollateralTanahBangunan, macetAt: tahunLalu(3), wantFaktor: "0.5"},
		{name: "tanah 5 tahun tidak diperhitungkan", jenis: domain.CollateralTanahBangunan, macetAt: tahunLalu(5), wantFaktor: "0"},
		{name: "kendaraan 6 bulan masih penuh", jenis: domain.CollateralKendaraan, macetAt: bulanLalu(6), wantFaktor: "1"},
		{name: "kendaraan 18 bulan turun 50%", jenis: domain.CollateralKendaraan, macetAt: bulanLalu(18), wantFaktor: "0.5"},
		{name: "kendaraan 3 tahun tidak diperhitungkan", jenis: domain.CollateralKendaraan, macetAt: tahunLalu(3), wantFaktor: "0"},
		{name: "mesin 18 bulan turun 50%", jenis: domain.CollateralMesinPeralatan, macetAt: bulanLalu(18), wantFaktor: "0.5"},
		{name: "tanpa penanda macet masih penuh", jenis: domain.CollateralTanahBangunan, macetAt: nil, wantFaktor: "1"},
		{name: "deposito tidak diturunkan", jenis: domain.CollateralDeposit, macetAt: tahunLalu(5), wantFaktor: "1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			c.CollateralType = tc.jenis
			got := domain.Pasal20TimeFactor(&c, tc.macetAt, pasal20AsOf)
			want, _ := decimal.NewFromString(tc.wantFaktor)
			if !got.Equal(want) {
				t.Fatalf("faktor %s, mau %s", got, want)
			}
		})
	}
}

func TestPasal20Ayat4Exception_MenerapkanSyaratKumulatif(t *testing.T) {
	outstanding := decimal.NewFromInt(500_000_000)
	cases := []struct {
		name string
		ubah func(*domain.LoanCollateral)
		want bool
	}{
		{
			name: "lolos semua syarat",
			ubah: func(c *domain.LoanCollateral) {
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
			want: true,
		},
		{
			name: "nilai hak tanggungan kurang dari kewajiban",
			ubah: func(c *domain.LoanCollateral) {
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(400_000_000)
			},
		},
		{
			name: "bukan penilai independen",
			ubah: func(c *domain.LoanCollateral) {
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
		},
		{
			name: "penilaian lebih dari 1 tahun",
			ubah: func(c *domain.LoanCollateral) {
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(600_000_000)
				c.AppraisalDate = pasal20AsOf.AddDate(-2, 0, 0)
			},
		},
		{
			name: "tidak ber-hak tanggungan",
			ubah: func(c *domain.LoanCollateral) {
				c.Mortgaged = false
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
		},
		{
			name: "bukan tanah/bangunan",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralKendaraan
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			tc.ubah(&c)
			if got := domain.Pasal20Ayat4Exception(&c, outstanding, pasal20AsOf); got != tc.want {
				t.Fatalf("pengecualian=%v, mau %v", got, tc.want)
			}
		})
	}
}

func TestPPAPCollateralDeduction_MenerapkanTarifPasal20(t *testing.T) {
	ctx := domain.PPAPPasal20Context{AsOf: pasal20AsOf, Collectibility: domain.KolLancar}

	cases := []struct {
		name string
		ubah func(*domain.LoanCollateral)
		want string
	}{
		{name: "tanah 80%", want: "800000000"},
		{
			// Huruf d: dasar pengurangnya NJOP, bukan nilai taksasi. NJOP 500 juta
			// menghasilkan 60% x 500 juta = 300 juta, walaupun taksasinya 1 miliar.
			name: "tanah tanpa hak tanggungan 60% dari NJOP",
			ubah: func(c *domain.LoanCollateral) {
				c.Mortgaged = false
				c.NJOPValue = decimal.NewFromInt(500_000_000)
				s := domain.NJOPSourceTax
				c.NJOPSource = &s
			},
			want: "300000000",
		},
		{
			// Tanpa NJOP, huruf d TIDAK memakai nilai taksasi sebagai gantinya: pengurang
			// nol (konservatif), bukan 600 juta.
			name: "tanah tanpa hak tanggungan tanpa NJOP nol",
			ubah: func(c *domain.LoanCollateral) { c.Mortgaged = false },
			want: "0",
		},
		{
			name: "kendaraan 50%",
			ubah: func(c *domain.LoanCollateral) { c.CollateralType = domain.CollateralKendaraan },
			want: "500000000",
		},
		{
			name: "agunan lain 20%",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralLainnya
				c.Certified = false
				c.Mortgaged = false
				c.AppraiserIndependent = true
			},
			want: "200000000",
		},
		{
			name: "deposito nol (ayat 2)",
			ubah: func(c *domain.LoanCollateral) { c.CollateralType = domain.CollateralDeposit },
			want: "0",
		},
		{
			name: "jenis tak dikenal nol (ayat 2)",
			ubah: func(c *domain.LoanCollateral) { c.CollateralType = domain.CollateralType("XX") },
			want: "0",
		},
		{
			name: "haircut bank lebih konservatif dipakai",
			ubah: func(c *domain.LoanCollateral) { c.HaircutPercent = decimal.NewFromInt(50) },
			want: "500000000",
		},
		{
			name: "haircut bank di bawah batas atas tidak menurunkan",
			ubah: func(c *domain.LoanCollateral) { c.HaircutPercent = decimal.NewFromInt(10) },
			want: "800000000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			if tc.ubah != nil {
				tc.ubah(&c)
			}
			got := domain.PPAPCollateralDeduction(&c, ctx)
			want, _ := decimal.NewFromString(tc.want)
			if !got.Equal(want) {
				t.Fatalf("pengurang %s, mau %s", got, want)
			}
		})
	}
}

func TestPPAPCollateralDeduction_MenerapkanPasal21(t *testing.T) {
	ctx := domain.PPAPPasal20Context{AsOf: pasal20AsOf, Collectibility: domain.KolLancar}

	cases := []struct {
		name string
		ubah func(*domain.LoanCollateral)
		want string
	}{
		{name: "agunan aktif tetap dihitung", want: "800000000"},
		{
			name: "sudah dilepas nol",
			ubah: func(c *domain.LoanCollateral) { c.Status = domain.CollateralReleased },
			want: "0",
		},
		{
			name: "sudah dieksekusi nol",
			ubah: func(c *domain.LoanCollateral) { c.Status = domain.CollateralExecuted },
			want: "0",
		},
		{
			name: "belum dinilai (taksasi nol) nol",
			ubah: func(c *domain.LoanCollateral) { c.AppraisalValue = decimal.Zero },
			want: "0",
		},
		{
			name: "belum ada tanggal taksasi nol",
			ubah: func(c *domain.LoanCollateral) { c.AppraisalDate = time.Time{} },
			want: "0",
		},
		{
			name: "keberadaan tidak diketahui nol",
			ubah: func(c *domain.LoanCollateral) { c.ExistsKnown = false },
			want: "0",
		},
		{
			name: "tidak dapat dieksekusi nol",
			ubah: func(c *domain.LoanCollateral) { c.Executable = false },
			want: "0",
		},
		{
			name: "milik pihak lain tanpa persetujuan nol",
			ubah: func(c *domain.LoanCollateral) {
				c.ThirdPartyOwner = true
				c.OwnerConsent = false
			},
			want: "0",
		},
		{
			name: "milik pihak lain dengan persetujuan dihitung",
			ubah: func(c *domain.LoanCollateral) {
				c.ThirdPartyOwner = true
				c.OwnerConsent = true
			},
			want: "800000000",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			if tc.ubah != nil {
				tc.ubah(&c)
			}
			got := domain.PPAPCollateralDeduction(&c, ctx)
			want, _ := decimal.NewFromString(tc.want)
			if !got.Equal(want) {
				t.Fatalf("pengurang %s, mau %s", got, want)
			}
		})
	}
}

func TestPPAPCollateralDeduction_PenurunanWaktuSaatMacet(t *testing.T) {
	macetTahun := func(n int) *time.Time {
		t := pasal20AsOf.AddDate(-n, 0, 0)
		return &t
	}
	macetBulan := func(n int) *time.Time {
		t := pasal20AsOf.AddDate(0, -n, 0)
		return &t
	}

	cases := []struct {
		name           string
		jenis          domain.CollateralType
		macetAt        *time.Time
		kolektibilitas domain.Collectibility
		want           string
	}{
		{name: "macet 3 tahun tanah turun 50%", jenis: domain.CollateralTanahBangunan, macetAt: macetTahun(3), kolektibilitas: domain.KolMacet, want: "400000000"},
		{name: "macet 5 tahun tanah nol", jenis: domain.CollateralTanahBangunan, macetAt: macetTahun(5), kolektibilitas: domain.KolMacet, want: "0"},
		{name: "macet 18 bulan kendaraan turun 50%", jenis: domain.CollateralKendaraan, macetAt: macetBulan(18), kolektibilitas: domain.KolMacet, want: "250000000"},
		{name: "macet 3 tahun kendaraan nol", jenis: domain.CollateralKendaraan, macetAt: macetTahun(3), kolektibilitas: domain.KolMacet, want: "0"},
		{name: "diragukan 5 tahun tidak diturunkan", jenis: domain.CollateralTanahBangunan, macetAt: macetTahun(5), kolektibilitas: domain.KolDiragukan, want: "800000000"},
		{name: "macet tanpa penanda masih penuh", jenis: domain.CollateralTanahBangunan, macetAt: nil, kolektibilitas: domain.KolMacet, want: "800000000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			c.CollateralType = tc.jenis
			ctx := domain.PPAPPasal20Context{
				AsOf:           pasal20AsOf,
				Collectibility: tc.kolektibilitas,
				MacetAt:        tc.macetAt,
			}
			got := domain.PPAPCollateralDeduction(&c, ctx)
			want, _ := decimal.NewFromString(tc.want)
			if !got.Equal(want) {
				t.Fatalf("pengurang %s, mau %s", got, want)
			}
		})
	}
}

func TestPPAPCollateralDeduction_PengecualianPasal20Ayat4(t *testing.T) {
	macetLimaTahun := pasal20AsOf.AddDate(-5, 0, 0)

	cases := []struct {
		name        string
		outstanding string
		ubah        func(*domain.LoanCollateral)
		want        string
	}{
		{
			name:        "hak tanggungan menutup kewajiban menahan penurunan",
			outstanding: "500000000",
			ubah: func(c *domain.LoanCollateral) {
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
			want: "800000000",
		},
		{
			name:        "hak tanggungan kurang dari kewajiban tetap diturunkan sampai nol",
			outstanding: "700000000",
			ubah: func(c *domain.LoanCollateral) {
				c.AppraiserIndependent = true
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
			want: "0",
		},
		{
			name:        "bukan penilai independen tetap diturunkan sampai nol",
			outstanding: "500000000",
			ubah: func(c *domain.LoanCollateral) {
				c.MortgageValue = decimal.NewFromInt(600_000_000)
			},
			want: "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			tc.ubah(&c)
			outstanding, _ := decimal.NewFromString(tc.outstanding)
			ctx := domain.PPAPPasal20Context{
				AsOf:           pasal20AsOf,
				Collectibility: domain.KolMacet,
				MacetAt:        &macetLimaTahun,
				Outstanding:    outstanding,
			}
			got := domain.PPAPCollateralDeduction(&c, ctx)
			want, _ := decimal.NewFromString(tc.want)
			if !got.Equal(want) {
				t.Fatalf("pengurang %s, mau %s", got, want)
			}
		})
	}
}

func TestPPAPCollateralDeductionTotal_MenjumlahkanYangLolos(t *testing.T) {
	tanah := agunanTanah()
	kendaraan := agunanTanah()
	kendaraan.CollateralType = domain.CollateralKendaraan
	deposito := agunanTanah()
	deposito.CollateralType = domain.CollateralDeposit

	ctx := domain.PPAPPasal20Context{AsOf: pasal20AsOf, Collectibility: domain.KolLancar}
	got := domain.PPAPCollateralDeductionTotal([]domain.LoanCollateral{tanah, kendaraan, deposito}, ctx)
	// 80% + 50% dari 1 miliar; deposito nol karena di luar daftar Pasal 20(1).
	want := decimal.NewFromInt(1_300_000_000)
	if !got.Equal(want) {
		t.Fatalf("total pengurang %s, mau %s", got, want)
	}
}

// agunanTunai adalah agunan tunai Pasal 17 yang sah: berstatus aktif, ditandai, dan
// dikaitkan ke rekening tempat dananya diblokir.
func agunanTunai(jenis domain.CollateralType, nilai int64) domain.LoanCollateral {
	c := agunanTanah()
	c.CollateralType = jenis
	c.IsCash = true
	akun := uuid.New()
	c.CashAccountID = &akun
	c.AppraisalValue = decimal.NewFromInt(nilai)
	return c
}

// Setiap jenis baru Pasal 20 ayat (1) harus memetakan tarif sesuai hurufnya. Sebelum
// ditambahkan, semuanya jatuh ke nol sehingga penyisihan lebih besar dari kewajiban.
func TestPasal20Rate_JenisBaruSesuaiHuruf(t *testing.T) {
	cases := []struct {
		name  string
		jenis domain.CollateralType
		want  string
		ubah  func(*domain.LoanCollateral)
	}{
		{name: "emas perhiasan 85% (huruf a)", jenis: domain.CollateralEmasPerhiasan, want: "0.85"},
		{name: "tanah adat 50% (huruf e)", jenis: domain.CollateralTanahAdat, want: "0.5"},
		{name: "tempat usaha 50% (huruf f)", jenis: domain.CollateralTempatUsaha, want: "0.5"},
		{
			name:  "dijamin BUMN/BUMD 50% (huruf i)",
			jenis: domain.CollateralJaminanBumnBumd,
			want:  "0.5",
			ubah: func(c *domain.LoanCollateral) {
				// Kriteria penjamin tidak dapat dinilai sistem; hanya penanda operator
				// beserta buktinya yang membuat tarif huruf i berlaku.
				c.BumnBumdCriteriaMet = true
				c.BumnBumdEvidence = "Surat jaminan penjamin BUMN/BUMD"
			},
		},
		{name: "resi gudang dinilai 1 bulan lalu 70% (huruf c)", jenis: domain.CollateralResiGudang, want: "0.7"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			c.CollateralType = tc.jenis
			if tc.ubah != nil {
				tc.ubah(&c)
			}
			rate, ok := domain.Pasal20Rate(&c, pasal20AsOf)
			if !ok {
				t.Fatalf("jenis %s seharusnya tercantum pada Pasal 20 ayat (1)", tc.jenis)
			}
			want := decimal.RequireFromString(tc.want)
			if !rate.Equal(want) {
				t.Fatalf("tarif %s, mau %s", rate, want)
			}
		})
	}
}

// Pita umur penilaian resi gudang: huruf c (<=12 bulan) 70%, huruf h (>12 s/d 18) 50%,
// huruf j (>18 s/d 24) 30%, dan lebih tua dari 24 bulan di luar daftar ayat (1).
func TestPasal20Rate_ResiGudangMenurutUmurPenilaian(t *testing.T) {
	bulanLalu := func(n int) time.Time { return pasal20AsOf.AddDate(0, -n, 0) }

	cases := []struct {
		name      string
		ubah      func(*domain.LoanCollateral)
		want      string
		wantDapat bool
	}{
		{
			name:      "dinilai 6 bulan lalu 70% (huruf c)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(6) },
			want:      "0.7",
			wantDapat: true,
		},
		{
			name:      "tepat 12 bulan 70% (huruf c)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(12) },
			want:      "0.7",
			wantDapat: true,
		},
		{
			name:      "15 bulan 50% (huruf h)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(15) },
			want:      "0.5",
			wantDapat: true,
		},
		{
			name:      "tepat 18 bulan 50% (huruf h)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(18) },
			want:      "0.5",
			wantDapat: true,
		},
		{
			name:      "21 bulan 30% (huruf j)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(21) },
			want:      "0.3",
			wantDapat: true,
		},
		{
			name:      "tepat 24 bulan 30% (huruf j)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(24) },
			want:      "0.3",
			wantDapat: true,
		},
		{
			name:      "25 bulan di luar daftar ayat (1)",
			ubah:      func(c *domain.LoanCollateral) { c.AppraisalDate = bulanLalu(25) },
			wantDapat: false,
		},
		{
			name: "tanggal penilaian resi gudang menimpa taksasi lama",
			ubah: func(c *domain.LoanCollateral) {
				c.AppraisalDate = bulanLalu(30)
				tgl := bulanLalu(3)
				c.WarehouseReceiptValuedAt = &tgl
			},
			want:      "0.7",
			wantDapat: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			c.CollateralType = domain.CollateralResiGudang
			tc.ubah(&c)
			rate, ok := domain.Pasal20Rate(&c, pasal20AsOf)
			if ok != tc.wantDapat {
				t.Fatalf("dapat=%v, mau %v", ok, tc.wantDapat)
			}
			if !tc.wantDapat {
				return
			}
			want := decimal.RequireFromString(tc.want)
			if !rate.Equal(want) {
				t.Fatalf("tarif %s, mau %s", rate, want)
			}
		})
	}
}

// Tanah adat (huruf e) dan tempat usaha (huruf f) termasuk kelompok yang diturunkan
// Pasal 20 ayat (3) saat Macet; emas perhiasan dan resi gudang tidak diturunkan.
func TestPasal20TimeFactor_HurufEDanFBukanHurufACDanI(t *testing.T) {
	tahunLalu := func(n int) *time.Time {
		t := pasal20AsOf.AddDate(-n, 0, 0)
		return &t
	}

	cases := []struct {
		name       string
		jenis      domain.CollateralType
		macetAt    *time.Time
		wantFaktor string
	}{
		{name: "tanah adat 1 tahun penuh", jenis: domain.CollateralTanahAdat, macetAt: tahunLalu(1), wantFaktor: "1"},
		{name: "tanah adat 3 tahun turun 50%", jenis: domain.CollateralTanahAdat, macetAt: tahunLalu(3), wantFaktor: "0.5"},
		{name: "tanah adat 5 tahun nol", jenis: domain.CollateralTanahAdat, macetAt: tahunLalu(5), wantFaktor: "0"},
		{name: "tempat usaha 3 tahun turun 50%", jenis: domain.CollateralTempatUsaha, macetAt: tahunLalu(3), wantFaktor: "0.5"},
		{name: "tempat usaha 5 tahun nol", jenis: domain.CollateralTempatUsaha, macetAt: tahunLalu(5), wantFaktor: "0"},
		{name: "emas perhiasan tidak diturunkan", jenis: domain.CollateralEmasPerhiasan, macetAt: tahunLalu(5), wantFaktor: "1"},
		{name: "resi gudang tidak diturunkan", jenis: domain.CollateralResiGudang, macetAt: tahunLalu(5), wantFaktor: "1"},
		{name: "jaminan BUMN/BUMD tidak diturunkan", jenis: domain.CollateralJaminanBumnBumd, macetAt: tahunLalu(5), wantFaktor: "1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			c.CollateralType = tc.jenis
			got := domain.Pasal20TimeFactor(&c, tc.macetAt, pasal20AsOf)
			want := decimal.RequireFromString(tc.wantFaktor)
			if !got.Equal(want) {
				t.Fatalf("faktor %s, mau %s", got, want)
			}
		})
	}
}

func TestPPAPGeneralBase_MengecualikanAgunanTunai(t *testing.T) {
	cases := []struct {
		name        string
		outstanding string
		tunai       string
		want        string
	}{
		{name: "tanpa agunan tunai penuh", outstanding: "1000000", tunai: "0", want: "1000000"},
		{name: "sebagian dijamin tunai", outstanding: "1000000", tunai: "400000", want: "600000"},
		{name: "agunan tunai menutup seluruh baki", outstanding: "1000000", tunai: "1000000", want: "0"},
		{name: "agunan tunai melebihi baki tidak negatif", outstanding: "1000000", tunai: "1500000", want: "0"},
		{name: "nilai tunai negatif diabaikan", outstanding: "1000000", tunai: "-500", want: "1000000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.PPAPGeneralBase(
				decimal.RequireFromString(tc.outstanding),
				decimal.RequireFromString(tc.tunai),
			)
			if !got.Equal(decimal.RequireFromString(tc.want)) {
				t.Fatalf("dasar PPKA umum %s, mau %s", got, tc.want)
			}
		})
	}
}

func TestPPAPCashCollateralTotal_HanyaAktifDanDitandaiTunai(t *testing.T) {
	aktif := agunanTunai(domain.CollateralDeposit, 200_000_000)
	dilepas := agunanTunai(domain.CollateralDeposit, 500_000_000)
	dilepas.Status = domain.CollateralReleased
	bukanTunai := agunanTanah()
	bukanTunai.AppraisalValue = decimal.NewFromInt(900_000_000)

	got := domain.PPAPCashCollateralTotal([]domain.LoanCollateral{aktif, dilepas, bukanTunai})
	want := decimal.NewFromInt(200_000_000)
	if !got.Equal(want) {
		t.Fatalf("total agunan tunai %s, mau %s", got, want)
	}
}

// Agunan tunai TIDAK boleh mengurangi PPKA khusus: ia tidak tercantum pada daftar
// Pasal 20 ayat (1), sehingga menurut ayat (2) bukan pengurang. Ini yang membedakannya
// dari pengecualian PPKA umum, yang justru memakainya.
func TestPPAPCollateralDeduction_AgunanTunaiTidakMengurangiPPKAKhusus(t *testing.T) {
	ctx := domain.PPAPPasal20Context{
		AsOf:           pasal20AsOf,
		Collectibility: domain.KolMacet,
		Outstanding:    decimal.NewFromInt(1_000_000_000),
	}

	cases := []struct {
		name string
		c    domain.LoanCollateral
	}{
		{name: "deposito ditandai agunan tunai", c: agunanTunai(domain.CollateralDeposit, 500_000_000)},
		{name: "emas perhiasan ditandai agunan tunai", c: agunanTunai(domain.CollateralEmasPerhiasan, 500_000_000)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.PPAPCollateralDeduction(&tc.c, ctx); !got.IsZero() {
				t.Fatalf("pengurang PPKA khusus %s, mau nol", got)
			}
		})
	}

	// Pembanding: emas perhiasan yang BUKAN agunan tunai tetap menjadi pengurang 85%.
	emas := agunanTanah()
	emas.CollateralType = domain.CollateralEmasPerhiasan
	want := decimal.NewFromInt(850_000_000)
	if got := domain.PPAPCollateralDeduction(&emas, ctx); !got.Equal(want) {
		t.Fatalf("pengurang emas perhiasan %s, mau %s", got, want)
	}
}

// ---------- Item 6: NJOP huruf d/e, porsi agunan tunai, penanda BUMN/BUMD ----------

// Pasal20BaseValue menentukan nilai yang dikalikan tarif: nilai taksasi untuk sebagian
// besar huruf, NJOP untuk huruf d/e. NJOP yang tidak diisi harus menghasilkan ok=false,
// bukan diam-diam memakai nilai taksasi.
func TestPasal20BaseValue_NJOPHanyaUntukHurufDDanE(t *testing.T) {
	njop := decimal.NewFromInt(500_000_000)
	sumberNJOP := domain.NJOPSourceTax
	sumberPenilai := domain.NJOPSourceAppraiser
	sumberTakDikenal := domain.NJOPSource("KIRA_KIRA")

	cases := []struct {
		name      string
		ubah      func(*domain.LoanCollateral)
		want      string
		wantDapat bool
	}{
		{
			name: "huruf d memakai NJOP",
			ubah: func(c *domain.LoanCollateral) {
				c.Mortgaged = false
				c.NJOPValue = njop
				c.NJOPSource = &sumberNJOP
			},
			want:      "500000000",
			wantDapat: true,
		},
		{
			name: "huruf e memakai NJOP",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralTanahAdat
				c.NJOPValue = njop
				c.NJOPSource = &sumberNJOP
			},
			want:      "500000000",
			wantDapat: true,
		},
		{
			name: "huruf e nilai penilai independen dipakai bila eksplisit",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralTanahAdat
				c.NJOPValue = decimal.NewFromInt(350_000_000)
				c.NJOPSource = &sumberPenilai
			},
			want:      "350000000",
			wantDapat: true,
		},
		{
			name: "huruf d tanpa NJOP tidak memakai taksasi",
			ubah: func(c *domain.LoanCollateral) { c.Mortgaged = false },
		},
		{
			name: "huruf d nilai tanpa asal tidak dipakai",
			ubah: func(c *domain.LoanCollateral) {
				c.Mortgaged = false
				c.NJOPValue = njop
			},
		},
		{
			name: "huruf e asal tak dikenal tidak dipakai",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralTanahAdat
				c.NJOPValue = njop
				c.NJOPSource = &sumberTakDikenal
			},
		},
		{
			name: "huruf b tetap memakai taksasi walau NJOP diisi",
			ubah: func(c *domain.LoanCollateral) {
				c.NJOPValue = njop
				c.NJOPSource = &sumberNJOP
			},
			want:      "1000000000",
			wantDapat: true,
		},
		{
			name: "huruf f tetap memakai taksasi walau NJOP diisi",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralTempatUsaha
				c.NJOPValue = njop
				c.NJOPSource = &sumberNJOP
			},
			want:      "1000000000",
			wantDapat: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			tc.ubah(&c)
			got, ok := domain.Pasal20BaseValue(&c)
			if ok != tc.wantDapat {
				t.Fatalf("dapat=%v, mau %v", ok, tc.wantDapat)
			}
			if !tc.wantDapat {
				return
			}
			if want := decimal.RequireFromString(tc.want); !got.Equal(want) {
				t.Fatalf("dasar %s, mau %s", got, want)
			}
		})
	}
}

// Pengurang huruf d/e harus 60%/50% dari NJOP, bukan dari nilai taksasi. Inilah angka
// yang berubah bila NJOP kosong: nol, bukan nilai taksasi.
func TestPPAPCollateralDeduction_MemakaiNJOPHurufDDanE(t *testing.T) {
	ctx := domain.PPAPPasal20Context{AsOf: pasal20AsOf, Collectibility: domain.KolLancar}
	sumberPenilai := domain.NJOPSourceAppraiser

	cases := []struct {
		name string
		ubah func(*domain.LoanCollateral)
		want string
	}{
		{
			name: "huruf d 60% dari NJOP 500 juta",
			ubah: func(c *domain.LoanCollateral) {
				c.Mortgaged = false
				c.NJOPValue = decimal.NewFromInt(500_000_000)
				s := domain.NJOPSourceTax
				c.NJOPSource = &s
			},
			want: "300000000",
		},
		{
			name: "huruf e 50% dari nilai penilai independen eksplisit",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralTanahAdat
				c.NJOPValue = decimal.NewFromInt(400_000_000)
				c.NJOPSource = &sumberPenilai
			},
			want: "200000000",
		},
		{
			name: "huruf d tanpa NJOP nol, taksasi tidak dipakai",
			ubah: func(c *domain.LoanCollateral) { c.Mortgaged = false },
			want: "0",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanTanah()
			tc.ubah(&c)
			got := domain.PPAPCollateralDeduction(&c, ctx)
			if want := decimal.RequireFromString(tc.want); !got.Equal(want) {
				t.Fatalf("pengurang %s, mau %s", got, want)
			}
		})
	}
}

// Pasal 20 ayat (1) huruf i: kriteria penjamin BUMN/BUMD tidak dapat dinilai sistem.
// Tanpa penanda operator, nilainya bukan pengurang — bukan tarif 50% yang disimpulkan.
func TestPasal20Rate_BumnBumdPerluPenandaEksplisit(t *testing.T) {
	c := agunanTanah()
	c.CollateralType = domain.CollateralJaminanBumnBumd

	if _, ok := domain.Pasal20Rate(&c, pasal20AsOf); ok {
		t.Fatal("tanpa penanda kriteria, BUMN/BUMD tidak boleh tercantum di ayat (1)")
	}
	if got := domain.PPAPCollateralDeduction(&c, domain.PPAPPasal20Context{AsOf: pasal20AsOf, Collectibility: domain.KolLancar}); !got.IsZero() {
		t.Fatalf("pengurang tanpa penanda %s, mau nol", got)
	}

	c.BumnBumdCriteriaMet = true
	c.BumnBumdEvidence = "Surat jaminan penjamin"
	rate, ok := domain.Pasal20Rate(&c, pasal20AsOf)
	if !ok || !rate.Equal(decimal.RequireFromString("0.5")) {
		t.Fatalf("tarif BUMN/BUMD %s dapat=%v, mau 0.5", rate, ok)
	}
	want := decimal.NewFromInt(500_000_000)
	if got := domain.PPAPCollateralDeduction(&c, domain.PPAPPasal20Context{AsOf: pasal20AsOf, Collectibility: domain.KolLancar}); !got.Equal(want) {
		t.Fatalf("pengurang berpenanda %s, mau %s", got, want)
	}
}

// Pasal 17 ayat (1) jo. Pasal 19 ayat (4) huruf b: porsi eksposur yang dijamin agunan
// tunai tidak boleh melebihi eksposur.
func TestPPAPCashGuaranteedPortion_TidakMelebihiEksposur(t *testing.T) {
	cases := []struct {
		name     string
		eksposur string
		tunai    string
		want     string
	}{
		{name: "tunai lebih kecil", eksposur: "1000000", tunai: "400000", want: "400000"},
		{name: "tunai sama dengan eksposur", eksposur: "1000000", tunai: "1000000", want: "1000000"},
		{name: "tunai melebihi eksposur dibatasi", eksposur: "1000000", tunai: "1500000", want: "1000000"},
		{name: "tunai negatif nol", eksposur: "1000000", tunai: "-500", want: "0"},
		{name: "eksposur nol", eksposur: "0", tunai: "500000", want: "0"},
		{name: "eksposur negatif nol", eksposur: "-1", tunai: "500000", want: "0"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.PPAPCashGuaranteedPortion(
				decimal.RequireFromString(tc.eksposur),
				decimal.RequireFromString(tc.tunai),
			)
			if want := decimal.RequireFromString(tc.want); !got.Equal(want) {
				t.Fatalf("porsi dijamin %s, mau %s", got, want)
			}
		})
	}
}

// Pemisahan porsi harus utuh: yang dijamin ditambah yang tidak dijamin sama dengan
// eksposur. Yang tidak dijamin tetap dikenai tarif umumnya (bukan ditiadakan).
func TestPPAPLancarPortions_MemisahkanPorsiSecaraUtuh(t *testing.T) {
	cases := []struct {
		name           string
		eksposur       string
		tunai          string
		wantDijamin    string
		wantTakDijamin string
	}{
		{name: "sebagian dijamin", eksposur: "1000000", tunai: "400000", wantDijamin: "400000", wantTakDijamin: "600000"},
		{name: "seluruhnya dijamin", eksposur: "1000000", tunai: "1000000", wantDijamin: "1000000", wantTakDijamin: "0"},
		{name: "jaminan melebihi eksposur", eksposur: "1000000", tunai: "1500000", wantDijamin: "1000000", wantTakDijamin: "0"},
		{name: "tanpa jaminan", eksposur: "1000000", tunai: "0", wantDijamin: "0", wantTakDijamin: "1000000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			eksposur := decimal.RequireFromString(tc.eksposur)
			tunai := decimal.RequireFromString(tc.tunai)
			dijamin, takDijamin := domain.PPAPLancarPortions(eksposur, tunai)
			if want := decimal.RequireFromString(tc.wantDijamin); !dijamin.Equal(want) {
				t.Fatalf("porsi dijamin %s, mau %s", dijamin, want)
			}
			if want := decimal.RequireFromString(tc.wantTakDijamin); !takDijamin.Equal(want) {
				t.Fatalf("porsi tidak dijamin %s, mau %s", takDijamin, want)
			}
			if sum := dijamin.Add(takDijamin); !sum.Equal(eksposur) {
				t.Fatalf("jumlah porsi %s, mau sama dengan eksposur %s", sum, eksposur)
			}
		})
	}
}
