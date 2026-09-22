package service_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Uji integrasi fase persiapan bobot risiko agunan Lampiran II SEOJK No.
// 2/SEOJK.03/2025 terhadap PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN
// diisi, mengikuti pola collateral_njop_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiAgunanLampiranII -v
//
// Yang dibuktikan: (1) baris lama tetap valid tanpa backfill; (2) informasi Lampiran
// II yang baru benar-benar tersimpan round-trip; (3) bobot yang dipakai SEMUA masih
// 100% dan saklarnya mati, sedangkan angka resmi tersimpan terpisah supaya pengisian
// kelak tidak menebak; (4) migrasi idempotent.

const lampiranIIMigration = "000087_collateral_lampiran_ii_prep.up.sql"

// addCollateralLampiranII mencatat agunan uji lengkap kolom lama dan baru, lalu
// membacanya kembali dari database agar round-trip benar-benar teruji.
func (e *moneyEnv) addCollateralLampiranII(t *testing.T, c *domain.LoanCollateral) *domain.LoanCollateral {
	t.Helper()
	c.Description = "agunan uji Lampiran II"
	c.DocumentNumber = fmt.Sprintf("DOCL2-%d", time.Now().UnixNano())
	c.OwnerName = "Pemilik Uji Lampiran II"
	if c.AppraisalDate.IsZero() {
		c.AppraisalDate = tanggalSaja(time.Now().UTC().AddDate(0, 0, -5))
	}
	c.HaircutPercent = decimal.Zero
	c.Status = domain.CollateralActive
	c.ExistsKnown = true
	c.Executable = true
	if err := e.collateralRepo.Create(e.ctx, c, "001"); err != nil {
		t.Fatalf("mencatat agunan uji Lampiran II: %v", err)
	}
	got, err := e.collateralRepo.GetByID(e.ctx, c.ID)
	if err != nil {
		t.Fatalf("membaca ulang agunan uji Lampiran II: %v", err)
	}
	return got
}

// tanggalSaja membuang jam dari waktu agar perbandingan dengan kolom DATE database
// (yang kembali tengah malam) tidak gagal karena selisih jam.
func tanggalSaja(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func TestIntegrasiAgunanLampiranIIRoundTripDanDataLama(t *testing.T) {
	e := newMoneyEnv(t)

	cust := e.newCustomer(t, "Lampiran II Round Trip", "")
	loan := e.disburse(t, cust.ID, e.newAccount(t, cust.ID), idr(10_000_000), 4)

	// 1. Baris gaya lama: hanya kolom lama. Harus tetap valid tanpa backfill, dan
	// informasi Lampiran II yang belum ada ditandai (bukan ditebak).
	lama := e.addCollateralLampiranII(t, &domain.LoanCollateral{
		LoanID:         loan.ID,
		CollateralType: domain.CollateralTanahBangunan,
		AppraisalValue: idr(10_000_000),
	})
	if lama.BindingType != nil {
		t.Fatalf("data lama tiba-tiba punya ikatan %v", *lama.BindingType)
	}
	if lama.Disputed || lama.InsuranceExpiryDate != nil || lama.AppraisalValidUntil != nil {
		t.Fatalf("data lama berubah tanpa backfill: %+v", lama)
	}
	if len(lama.MissingLampiranIIFields()) == 0 {
		t.Fatal("data lama tanpa informasi Lampiran II harus ditandai, bukan dipaksa")
	}

	// 2. Baris dengan informasi Lampiran II lengkap round-trip.
	ikatan := domain.BindingHakTanggungan
	akhirAsuransi := tanggalSaja(time.Now().UTC().AddDate(1, 0, 0))
	berlakuTaksasi := tanggalSaja(time.Now().UTC().AddDate(0, 3, 0))
	baru := e.addCollateralLampiranII(t, &domain.LoanCollateral{
		LoanID:                loan.ID,
		CollateralType:        domain.CollateralTanahBangunan,
		AppraisalValue:        idr(20_000_000),
		BindingType:           &ikatan,
		Disputed:              true,
		DisputeEvidence:       "sengketa waris",
		InsuranceExpiryDate:   &akhirAsuransi,
		InsurancePolicyNumber: "POL-L2-1",
		AppraisalValidUntil:   &berlakuTaksasi,
	})
	if baru.BindingType == nil || *baru.BindingType != ikatan {
		t.Fatalf("ikatan tersimpan %v, mau %s", baru.BindingType, ikatan)
	}
	if !baru.Disputed || baru.DisputeEvidence != "sengketa waris" {
		t.Fatalf("penanda sengketa tersimpan %v/%q", baru.Disputed, baru.DisputeEvidence)
	}
	if baru.InsuranceExpiryDate == nil || !baru.InsuranceExpiryDate.Equal(akhirAsuransi) {
		t.Fatalf("masa berlaku asuransi tersimpan %v, mau %v", baru.InsuranceExpiryDate, akhirAsuransi)
	}
	if baru.InsurancePolicyNumber != "POL-L2-1" {
		t.Fatalf("nomor polis tersimpan %q, mau POL-L2-1", baru.InsurancePolicyNumber)
	}
	if baru.AppraisalValidUntil == nil || !baru.AppraisalValidUntil.Equal(berlakuTaksasi) {
		t.Fatalf("masa berlaku taksasi tersimpan %v, mau %v", baru.AppraisalValidUntil, berlakuTaksasi)
	}
	if len(baru.MissingLampiranIIFields()) != 0 {
		t.Fatalf("data Lampiran II lengkap masih ditandai: %v", baru.MissingLampiranIIFields())
	}
}

func TestIntegrasiAgunanLampiranIIBobotMasihKonservatif(t *testing.T) {
	e := newMoneyEnv(t)

	// 1. Semua bobot yang AKAN dipakai masih 100% dan saklar aktivasi mati: perilaku
	// ATMR tidak berubah. Bila ada satu baris saja yang menyimpang, uji gagal.
	hitungPenyimpang := func() (int, int) {
		t.Helper()
		var total, menyimpang int
		if err := e.db.QueryRowContext(e.ctx, `
			SELECT count(*),
			       count(*) FILTER (WHERE applied_weight_frac <> 1.00000 OR enabled)
			FROM collateral_lampiran_ii_weights`).Scan(&total, &menyimpang); err != nil {
			t.Fatalf("membaca pemetaan Lampiran II: %v", err)
		}
		return total, menyimpang
	}

	total, menyimpang := hitungPenyimpang()
	if total == 0 {
		t.Fatal("tabel pemetaan Lampiran II kosong: fase persiapan tidak menyiapkan data")
	}
	if menyimpang != 0 {
		t.Fatalf("%d dari %d baris sudah berbobot bukan-100%%/aktif; ATMR belum boleh berubah",
			menyimpang, total)
	}

	// 2. Angka resmi Lampiran II tersimpan sebagai dokumentasi supaya pengisian kelak
	// cukup menyalinnya, tanpa menebak butirnya.
	for _, kasus := range []struct {
		item     int
		official string
	}{
		{8, "0.15000"},  // emas perhiasan 15%
		{12, "0.30000"}, // tanah/bangunan ber-HT/fidusia 30%
		{17, "0.50000"}, // tanah/bangunan bersertipikat tanpa beban 50%
		{18, "0.70000"}, // UMK 70%
		{19, "0.70000"}, // kendaraan ber-hipotek/fidusia 70%
		{21, "1.00000"}, // lainnya 100%
		{22, "1.00000"}, // jatuh tempo/macet 100%
	} {
		var n int
		if err := e.db.QueryRowContext(e.ctx, `
			SELECT count(*) FROM collateral_lampiran_ii_weights
			WHERE lampiran_ii_item = $1 AND official_weight_frac = $2::numeric`,
			kasus.item, kasus.official).Scan(&n); err != nil {
			t.Fatalf("membaca butir %d: %v", kasus.item, err)
		}
		if n == 0 {
			t.Fatalf("butir Lampiran II %d tanpa angka resmi %s: pengisian bobot kelak menebak",
				kasus.item, kasus.official)
		}
	}

	// 3. Bobot kredit agregat KPMM tetap 100%: bukti angka ATMR tidak bergeser.
	var nilai string
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT value FROM system_config WHERE key = 'kpmm.rwa_frac.kredit'`).Scan(&nilai); err != nil {
		t.Fatalf("membaca bobot kredit KPMM: %v", err)
	}
	if nilai != "1.00" {
		t.Fatalf("bobot kredit KPMM %q, mau 1.00 (ATMR tidak boleh berubah)", nilai)
	}

	// 4. Migrasi idempotent: dijalankan ulang tidak menambah baris atau mengubah bobot.
	path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", lampiranIIMigration)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", lampiranIIMigration, err)
	}
	if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
		t.Fatalf("menjalankan ulang migrasi %s: %v", lampiranIIMigration, err)
	}
	totalKedua, menyimpangKedua := hitungPenyimpang()
	if totalKedua != total {
		t.Fatalf("migrasi tidak idempotent: %d -> %d baris pemetaan", total, totalKedua)
	}
	if menyimpangKedua != 0 {
		t.Fatalf("jalan kedua memunculkan bobot bukan-100%%: %d baris", menyimpangKedua)
	}
}
