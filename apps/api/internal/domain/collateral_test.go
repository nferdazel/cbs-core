package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func agunanValid() *domain.LoanCollateral {
	return &domain.LoanCollateral{
		LoanID:         uuid.New(),
		CollateralType: domain.CollateralTanahBangunan,
		Description:    "SHM No. 123, Cibaduyut",
		DocumentNumber: "SKMHT-2026-001",
		OwnerName:      "Budi Santoso",
		AppraisalValue: decimal.NewFromInt(500000000),
		AppraisalDate:  time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		HaircutPercent: domain.DefaultHaircutPercent(),
		Status:         domain.CollateralActive,
	}
}

func TestCollateral_ValidateMenolakAgunanYangTidakDapatDihitung(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name    string
		ubah    func(*domain.LoanCollateral)
		wantErr error
	}{
		{
			name:    "tanpa nomor bukti ikatan",
			ubah:    func(c *domain.LoanCollateral) { c.DocumentNumber = "   " },
			wantErr: domain.ErrCollateralDocumentRequired,
		},
		{
			name:    "tanpa nama pemilik",
			ubah:    func(c *domain.LoanCollateral) { c.OwnerName = "" },
			wantErr: domain.ErrCollateralOwnerRequired,
		},
		{
			name:    "nilai taksasi nol",
			ubah:    func(c *domain.LoanCollateral) { c.AppraisalValue = decimal.Zero },
			wantErr: domain.ErrCollateralAppraisalInvalid,
		},
		{
			name:    "nilai taksasi negatif",
			ubah:    func(c *domain.LoanCollateral) { c.AppraisalValue = decimal.NewFromInt(-1) },
			wantErr: domain.ErrCollateralAppraisalInvalid,
		},
		{
			name:    "tanggal taksasi di masa depan",
			ubah:    func(c *domain.LoanCollateral) { c.AppraisalDate = now.AddDate(0, 0, 1) },
			wantErr: domain.ErrCollateralAppraisalDateInvalid,
		},
		{
			name:    "haircut di atas seratus persen",
			ubah:    func(c *domain.LoanCollateral) { c.HaircutPercent = decimal.NewFromInt(101) },
			wantErr: domain.ErrCollateralHaircutInvalid,
		},
		{
			name:    "haircut negatif",
			ubah:    func(c *domain.LoanCollateral) { c.HaircutPercent = decimal.NewFromInt(-1) },
			wantErr: domain.ErrCollateralHaircutInvalid,
		},
		{
			name: "tanpa kredit induk",
			ubah: func(c *domain.LoanCollateral) { c.LoanID = uuid.Nil },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanValid()
			tc.ubah(c)

			err := c.Validate(now)
			if tc.wantErr == nil {
				if err == nil {
					t.Fatal("agunan tanpa kredit induk seharusnya ditolak")
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("kesalahan %v, ingin %v", err, tc.wantErr)
			}
		})
	}
}

func TestCollateral_ValidateMenerimaAgunanSahTermasukTaksasiHariIni(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	c := agunanValid()
	c.AppraisalDate = now // jam taksasi apa pun pada hari berjalan tetap sah
	if err := c.Validate(now); err != nil {
		t.Fatalf("agunan sah ditolak: %v", err)
	}
}

// Haircut bawaan harus 100 (tanpa pengurangan): modul agunan tidak boleh mengurangi
// eksposur PPAP sebelum bank menetapkan kebijakannya sendiri.
func TestCollateral_HaircutBawaanTidakMengurangiEksposur(t *testing.T) {
	if got := domain.DefaultHaircutPercent(); !got.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("haircut bawaan %s, mau 100", got)
	}
}

func TestCollateral_HaircutConfigKeyPerJenis(t *testing.T) {
	cases := map[domain.CollateralType]string{
		domain.CollateralTanahBangunan:  "collateral.haircut.tanah_bangunan_pct",
		domain.CollateralKendaraan:      "collateral.haircut.kendaraan_pct",
		domain.CollateralDeposit:        "collateral.haircut.deposit_pct",
		domain.CollateralMesinPeralatan: "collateral.haircut.mesin_peralatan_pct",
		domain.CollateralLainnya:        "collateral.haircut.lainnya_pct",
		// Jenis tak dikenal tidak boleh diam-diam memakai kebijakan jenis lain.
		domain.CollateralType("TIDAK_DIKENAL"): "collateral.haircut.lainnya_pct",
	}

	for jenis, want := range cases {
		if got := domain.HaircutConfigKey(jenis); got != want {
			t.Fatalf("kunci haircut %s = %q, mau %q", jenis, got, want)
		}
	}
}

func TestCollateral_IsActiveHanyaUntukStatusAktif(t *testing.T) {
	c := agunanValid()
	if !c.IsActive() {
		t.Fatal("agunan baru harus aktif")
	}
	c.Status = domain.CollateralReleased
	if c.IsActive() {
		t.Fatal("agunan yang sudah dilepas tidak boleh dihitung sebagai pengurang")
	}
}

// Agunan tunai tanpa rekening tidak dapat dibuktikan diblokir (Pasal 17 ayat (3)),
// sehingga pengecualian PPKA umumnya tidak boleh diakui.
func TestCollateral_ValidateAgunanTunaiWajibRekening(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	c := agunanValid()
	c.CollateralType = domain.CollateralDeposit
	c.IsCash = true
	if err := c.Validate(now); !errors.Is(err, domain.ErrCollateralCashAccountRequired) {
		t.Fatalf("kesalahan %v, ingin %v", err, domain.ErrCollateralCashAccountRequired)
	}

	akun := uuid.New()
	c.CashAccountID = &akun
	if err := c.Validate(now); err != nil {
		t.Fatalf("agunan tunai ber-rekening ditolak: %v", err)
	}
}

// Pasal 20 ayat (1) huruf d/e dan huruf i: nilai NJOP harus berpasangan dengan asalnya,
// dan penanda kriteria penjamin BUMN/BUMD harus disertai bukti. Parameter yang salah
// ditolak dengan sentinel yang jelas, bukan dilonggarkan diam-diam.
func TestCollateral_ValidateNJOPDanPenandaBumnBumd(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	sumberNJOP := domain.NJOPSourceTax
	njop := decimal.NewFromInt(500_000_000)
	besok := now.AddDate(0, 0, 1)

	cases := []struct {
		name    string
		ubah    func(*domain.LoanCollateral)
		wantErr error
	}{
		{
			name: "NJOP dengan asal diterima",
			ubah: func(c *domain.LoanCollateral) {
				c.NJOPValue = njop
				c.NJOPSource = &sumberNJOP
			},
		},
		{
			name:    "nilai NJOP tanpa asal ditolak",
			ubah:    func(c *domain.LoanCollateral) { c.NJOPValue = njop },
			wantErr: domain.ErrCollateralNJOPRequired,
		},
		{
			name:    "asal NJOP tanpa nilai ditolak",
			ubah:    func(c *domain.LoanCollateral) { c.NJOPSource = &sumberNJOP },
			wantErr: domain.ErrCollateralNJOPRequired,
		},
		{
			name: "NJOP negatif ditolak",
			ubah: func(c *domain.LoanCollateral) {
				c.NJOPValue = decimal.NewFromInt(-1)
				c.NJOPSource = &sumberNJOP
			},
			wantErr: domain.ErrCollateralNJOPInvalid,
		},
		{
			name: "asal NJOP tak dikenal ditolak",
			ubah: func(c *domain.LoanCollateral) {
				s := domain.NJOPSource("KIRA_KIRA")
				c.NJOPValue = njop
				c.NJOPSource = &s
			},
			wantErr: domain.ErrCollateralNJOPSourceInvalid,
		},
		{
			name: "tanggal NJOP di masa depan ditolak",
			ubah: func(c *domain.LoanCollateral) {
				c.NJOPValue = njop
				c.NJOPSource = &sumberNJOP
				c.NJOPDate = &besok
			},
			wantErr: domain.ErrCollateralNJOPDateInvalid,
		},
		{
			name: "penanda BUMN/BUMD tanpa bukti ditolak",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralJaminanBumnBumd
				c.BumnBumdCriteriaMet = true
			},
			wantErr: domain.ErrCollateralBumnEvidenceRequired,
		},
		{
			name: "penanda BUMN/BUMD dengan bukti diterima",
			ubah: func(c *domain.LoanCollateral) {
				c.CollateralType = domain.CollateralJaminanBumnBumd
				c.BumnBumdCriteriaMet = true
				c.BumnBumdEvidence = "Surat jaminan penjamin BUMN"
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanValid()
			tc.ubah(c)
			err := c.Validate(now)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("agunan sah ditolak: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("kesalahan %v, ingin %v", err, tc.wantErr)
			}
		})
	}
}

// Ikatan hukum dan masa berlaku taksasi adalah informasi Lampiran II SEOJK No.
// 2/SEOJK.03/2025 yang dipersiapkan. Ikatan tak dikenal ditolak, bukan diperlakukan
// sebagai "tanpa beban" (yang akan menggeser bobot 30%/70% ke 50%/100%), dan masa
// berlaku taksasi tidak boleh mendahului tanggal taksasinya.
func TestCollateral_ValidateIkatanDanMasaBerlakuTaksasi(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	ikatanHT := domain.BindingHakTanggungan
	tanpaBeban := domain.BindingTanpaBeban
	takDikenal := domain.CollateralBinding("KIRA_KIRA")

	cases := []struct {
		name    string
		ubah    func(*domain.LoanCollateral)
		wantErr error
	}{
		{
			name: "ikatan hak tanggungan diterima",
			ubah: func(c *domain.LoanCollateral) { c.BindingType = &ikatanHT },
		},
		{
			name: "ikatan tanpa beban diterima",
			ubah: func(c *domain.LoanCollateral) { c.BindingType = &tanpaBeban },
		},
		{
			name:    "ikatan tak dikenal ditolak",
			ubah:    func(c *domain.LoanCollateral) { c.BindingType = &takDikenal },
			wantErr: domain.ErrCollateralBindingInvalid,
		},
		{
			name: "masa berlaku taksasi sebelum tanggal taksasi ditolak",
			ubah: func(c *domain.LoanCollateral) {
				v := c.AppraisalDate.AddDate(0, 0, -1)
				c.AppraisalValidUntil = &v
			},
			wantErr: domain.ErrCollateralAppraisalValidityInvalid,
		},
		{
			name: "masa berlaku taksasi setelah tanggal taksasi diterima",
			ubah: func(c *domain.LoanCollateral) {
				v := c.AppraisalDate.AddDate(0, 3, 0)
				c.AppraisalValidUntil = &v
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := agunanValid()
			tc.ubah(c)
			err := c.Validate(now)
			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("agunan sah ditolak: %v", err)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("kesalahan %v, ingin %v", err, tc.wantErr)
			}
		})
	}
}

// Ikatan yang terikat secara hukum menandai pemisah butir 12/19 Lampiran II dari
// butir 17/21. Nilai kosong tidak dianggap terikat.
func TestCollateralBinding_BoundHanyaUntukIkatanHukum(t *testing.T) {
	cases := map[domain.CollateralBinding]bool{
		domain.BindingHakTanggungan: true,
		domain.BindingFidusia:       true,
		domain.BindingHipotek:       true,
		domain.BindingTanpaBeban:    false,
		domain.BindingLainnya:       false,
	}
	for ikatan, want := range cases {
		if got := ikatan.Bound(); got != want {
			t.Fatalf("Bound(%s) = %v, mau %v", ikatan, got, want)
		}
		if !ikatan.Valid() {
			t.Fatalf("ikatan %s seharusnya dikenal domain", ikatan)
		}
	}
	if (domain.CollateralBinding("")).Valid() {
		t.Fatal("ikatan kosong tidak boleh dianggap dikenal")
	}
}

// MissingLampiranIIFields menandai data Lampiran II yang belum diisi, tanpa menolak
// agunan. Setelah bank melengkapi, penandanya kosong.
func TestCollateral_MissingLampiranIIFieldsMenandaiTanpaMenolak(t *testing.T) {
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)
	c := agunanValid()
	if err := c.Validate(now); err != nil {
		t.Fatalf("agunan tanpa data Lampiran II ditolak: %v", err)
	}
	want := []string{"binding_type", "insurance_expiry_date", "appraisal_valid_until"}
	if got := c.MissingLampiranIIFields(); !reflect.DeepEqual(got, want) {
		t.Fatalf("data Lampiran II yang ditandai %v, mau %v", got, want)
	}

	ikatan := domain.BindingFidusia
	akhirAsuransi := now.AddDate(0, 6, 0)
	berlakuTaksasi := c.AppraisalDate.AddDate(0, 6, 0)
	c.BindingType = &ikatan
	c.InsuranceExpiryDate = &akhirAsuransi
	c.AppraisalValidUntil = &berlakuTaksasi
	if got := c.MissingLampiranIIFields(); len(got) != 0 {
		t.Fatalf("setelah dilengkapi masih ada yang ditandai: %v", got)
	}
}
