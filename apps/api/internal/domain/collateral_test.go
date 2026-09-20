package domain_test

import (
	"errors"
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
		domain.CollateralTanahBangunan:  "collateral.haircut.tanah_bangunan",
		domain.CollateralKendaraan:      "collateral.haircut.kendaraan",
		domain.CollateralDeposit:        "collateral.haircut.deposit",
		domain.CollateralMesinPeralatan: "collateral.haircut.mesin_peralatan",
		domain.CollateralLainnya:        "collateral.haircut.lainnya",
		// Jenis tak dikenal tidak boleh diam-diam memakai kebijakan jenis lain.
		domain.CollateralType("TIDAK_DIKENAL"): "collateral.haircut.lainnya",
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
