package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// Kepatuhan Pasal 20 dan Pasal 21 POJK No. 1 Tahun 2024 tentang Kualitas Aset Bank
// Perekonomian Rakyat. Seluruh fungsi di berkas ini adalah satu sumber kebenaran untuk
// menentukan nilai agunan yang boleh diperhitungkan sebagai pengurang PPKA.
//
// Rujukan pasal yang ditegakkan di sini:
//   - Pasal 20 ayat (1): batas atas nilai agunan yang boleh jadi pengurang per jenis.
//   - Pasal 20 ayat (2): agunan di luar daftar ayat (1) TIDAK diperhitungkan sama sekali.
//   - Pasal 20 ayat (3): untuk kualitas Macet, pengurang agunan huruf b, d, e, f paling
//     tinggi 50% setelah 2-4 tahun dan tidak diperhitungkan setelah 4 tahun.
//   - Pasal 20 ayat (4): pengecualian ayat (3) untuk tanah/bangunan bersertifikat
//     ber-hak tanggungan yang dinilai penilai independen <= 1 tahun dan nilai hak
//     tanggungannya menutup seluruh kewajiban debitur.
//   - Pasal 20 ayat (5): untuk kualitas Macet, pengurang agunan huruf g (kendaraan
//     bermotor/alat berat/mesin) 50% setelah 1-2 tahun dan tidak diperhitungkan setelah
//     2 tahun.
//   - Pasal 21 ayat (2): agunan bukan pengurang bila belum dinilai, tidak diketahui
//     keberadaannya, tidak dapat dieksekusi, atau milik pihak lain tanpa persetujuan
//     pemiliknya.
//   - Pasal 17 jo. Pasal 19 ayat (4) huruf b: bagian Aset Produktif yang dijamin agunan
//     tunai dikecualikan dari PPKA umum. Agunan tunai yang sama BUKAN pengurang PPKA
//     khusus karena tidak tercantum pada Pasal 20 ayat (1).

// Pasal20Rate adalah batas atas fraksi nilai taksasi yang boleh diperhitungkan menurut
// Pasal 20 ayat (1). ok=false berarti jenis agunan tidak tercantum pada daftar ayat (1),
// sehingga menurut ayat (2) nilainya tidak diperhitungkan sama sekali.
//
// Pemetaan ke jenis agunan yang ada di sistem (jenis di luar peta ini -> nol):
//   - EMAS_PERHIASAN                                            -> 85% (huruf a)
//   - TANAH_BANGUNAN bersertifikat ber-hak tanggungan/fidusia   -> 80% (huruf b)
//   - RESI_GUDANG dinilai <= 12 bulan                           -> 70% (huruf c)
//   - TANAH_BANGUNAN bersertifikat tanpa hak tanggungan         -> 60% (huruf d, dari NJOP)
//   - TANAH_ADAT (surat pengakuan tanah adat)                   -> 50% (huruf e, dari NJOP)
//   - TEMPAT_USAHA dengan bukti kepemilikan/izin pakai + kuasa
//     menjual                                                   -> 50% (huruf f)
//   - KENDARAAN / MESIN_PERALATAN ber-hipotek                    -> 50% (huruf g)
//   - RESI_GUDANG dinilai > 12 s/d 18 bulan                      -> 50% (huruf h)
//   - JAMINAN_BUMN_BUMD penjamin yang memenuhi kriteria KPMM     -> 50% (huruf i)
//   - RESI_GUDANG dinilai > 18 s/d 24 bulan                      -> 30% (huruf j)
//   - LAINNYA dinilai penilai independen <= 1 tahun              -> 20% (huruf k)
//
// Agunan tunai (IsCash) TIDAK pernah masuk peta ini: perannya adalah membuat bagian yang
// dijamin berkualitas Lancar (Pasal 17 ayat (1)) sehingga dikecualikan dari PPKA umum
// (Pasal 19 ayat (4) huruf b), bukan menjadi pengurang PPKA khusus. Menghitungnya di sini
// juga akan menghitung satu jaminan dua kali.
//
// NJOP pada huruf d dan e belum tersedia sebagai kolom tersendiri; nilai taksasi dipakai
// sebagai pendekatannya sampai kolom NJOP ditambahkan.
func Pasal20Rate(c *LoanCollateral, asOf time.Time) (decimal.Decimal, bool) {
	if c == nil {
		return decimal.Zero, false
	}
	// Agunan tunai dikecualikan dari pengurang PPKA khusus (Pasal 20 ayat (2)); lihat
	// catatan di atas.
	if c.IsCash {
		return decimal.Zero, false
	}
	switch c.CollateralType {
	case CollateralEmasPerhiasan:
		// Huruf a: paling tinggi 85% dari nilai pasar.
		return decimal.NewFromFloat(0.85), true
	case CollateralTanahBangunan:
		if c.Certified && c.Mortgaged {
			return decimal.NewFromFloat(0.80), true
		}
		if c.Certified {
			return decimal.NewFromFloat(0.60), true
		}
		return decimal.Zero, false
	case CollateralTanahAdat:
		// Huruf e: paling tinggi 50% dari NJOP/nilai pasar tanah dengan kepemilikan
		// surat pengakuan tanah adat (girik, petok D, letter C, dan sejenisnya).
		return decimal.NewFromFloat(0.50), true
	case CollateralTempatUsaha:
		// Huruf f: paling tinggi 50% untuk tempat usaha yang disertai bukti
		// kepemilikan/izin pemakaian/hak pakai dan surat kuasa menjual.
		return decimal.NewFromFloat(0.50), true
	case CollateralKendaraan, CollateralMesinPeralatan:
		if c.Mortgaged {
			return decimal.NewFromFloat(0.50), true
		}
		return decimal.Zero, false
	case CollateralResiGudang:
		// Huruf c, h, j: tarif menurut umur penilaian terakhir.
		return pasal20ResiGudangRate(c, asOf)
	case CollateralJaminanBumnBumd:
		// Huruf i: paling tinggi 50% untuk bagian Kredit yang dijamin BUMN/BUMD yang
		// berusaha sebagai penjamin dan memenuhi kriteria KPMM. Pemenuhan kriteria
		// diperiksa operator; jenis ini menyatakan kriterianya sudah dipenuhi.
		return decimal.NewFromFloat(0.50), true
	case CollateralLainnya:
		if c.AppraiserIndependent && dinilaiDalamTahunTerakhir(c.AppraisalDate, asOf) {
			return decimal.NewFromFloat(0.20), true
		}
		return decimal.Zero, false
	default:
		// DEPOSIT dan jenis tak dikenal tidak tercantum pada Pasal 20 ayat (1).
		return decimal.Zero, false
	}
}

// pasal20ResiGudangRate menerapkan pita umur penilaian resi gudang Pasal 20 ayat (1)
// huruf c, h, dan j: sampai dengan 12 bulan 70%, lebih dari 12 sampai dengan 18 bulan
// 50%, lebih dari 18 sampai dengan 24 bulan 30%. Penilaian yang lebih tua dari 24 bulan
// tidak lagi tercantum pada daftar ayat (1), sehingga menurut ayat (2) tidak diperhitungkan.
//
// Tanggal yang dipakai adalah WarehouseReceiptValuedAt bila diisi operator; bila kosong,
// AppraisalDate agunan yang dipakai sebagai penilaian terakhirnya.
func pasal20ResiGudangRate(c *LoanCollateral, asOf time.Time) (decimal.Decimal, bool) {
	t := c.AppraisalDate
	if c.WarehouseReceiptValuedAt != nil && !c.WarehouseReceiptValuedAt.IsZero() {
		t = *c.WarehouseReceiptValuedAt
	}
	if t.IsZero() {
		return decimal.Zero, false
	}
	ref := tanggalSaja(asOf)
	penilaian := tanggalSaja(t)
	switch {
	case !penilaian.Before(ref.AddDate(0, -12, 0)):
		return decimal.NewFromFloat(0.70), true
	case !penilaian.Before(ref.AddDate(0, -18, 0)):
		return decimal.NewFromFloat(0.50), true
	case !penilaian.Before(ref.AddDate(0, -24, 0)):
		return decimal.NewFromFloat(0.30), true
	default:
		return decimal.Zero, false
	}
}

// Pasal21Eligible menegakkan Pasal 21 ayat (2): agunan hanya boleh jadi pengurang bila
// sudah dinilai, keberadaannya diketahui, dapat dieksekusi, dan — bila milik pihak lain —
// ada persetujuan pemiliknya. Status di luar ACTIVE juga dikeluarkan karena agunan yang
// sudah dilepas/dieksekusi tidak lagi menjamin kredit.
func (c *LoanCollateral) Pasal21Eligible() bool {
	if c == nil || !c.IsActive() {
		return false
	}
	// Belum dinilai: taksasi tidak ada/tidak positif atau tanggal taksasi kosong.
	if c.AppraisalValue.LessThanOrEqual(decimal.Zero) || c.AppraisalDate.IsZero() {
		return false
	}
	if !c.ExistsKnown {
		return false
	}
	if !c.Executable {
		return false
	}
	if c.ThirdPartyOwner && !c.OwnerConsent {
		return false
	}
	return true
}

// HaircutBoundAmount menghitung nilai pengurang menurut kebijakan haircut bank. Rumusnya
// sama dengan kolom generated bound_amount di database, sehingga nilai yang dipakai
// perhitungan sama dengan yang tersimpan dan dapat diaudit.
func (c *LoanCollateral) HaircutBoundAmount() decimal.Decimal {
	return c.AppraisalValue.Mul(decimal.NewFromInt(1).Sub(c.HaircutPercent.Div(decimal.NewFromInt(100))))
}

// Pasal20TimeFactor adalah faktor penurunan pengurang menurut Pasal 20 ayat (3) dan (5)
// untuk kualitas Macet. Faktor 1 berarti tidak ada penurunan.
//
// Kredit tanpa penanda tanggal macet (macetAt nil) dianggap baru digolongkan Macet pada
// perhitungan berjalan, sehingga pengurangnya masih penuh. Penanda itu diisi saat kredit
// pertama kali digolongkan Macet.
func Pasal20TimeFactor(c *LoanCollateral, macetAt *time.Time, asOf time.Time) decimal.Decimal {
	if c == nil || macetAt == nil || macetAt.IsZero() {
		return decimal.NewFromInt(1)
	}
	macet := tanggalSaja(*macetAt)
	ref := tanggalSaja(asOf)
	switch c.CollateralType {
	case CollateralTanahBangunan, CollateralTanahAdat, CollateralTempatUsaha:
		// Huruf b, d, e, dan f: penuh sampai 2 tahun, 50% pada 2-4 tahun, nol setelah
		// 4 tahun.
		switch {
		case !macet.Before(ref.AddDate(-2, 0, 0)):
			return decimal.NewFromInt(1)
		case !macet.Before(ref.AddDate(-4, 0, 0)):
			return decimal.NewFromFloat(0.5)
		default:
			return decimal.Zero
		}
	case CollateralKendaraan, CollateralMesinPeralatan:
		// Huruf g: penuh sampai 1 tahun, 50% pada 1-2 tahun, nol setelah 2 tahun.
		switch {
		case !macet.Before(ref.AddDate(-1, 0, 0)):
			return decimal.NewFromInt(1)
		case !macet.Before(ref.AddDate(-2, 0, 0)):
			return decimal.NewFromFloat(0.5)
		default:
			return decimal.Zero
		}
	default:
		// Huruf a, c, h, i, j, k dan jenis di luar ayat (3)/(5) tidak diturunkan
		// menurut waktu.
		return decimal.NewFromInt(1)
	}
}

// Pasal20Ayat4Exception menentukan apakah penurunan Pasal 20 ayat (3) tidak berlaku bagi
// agunan ini. Syaratnya kumulatif dan hanya untuk tanah/bangunan: bersertifikat,
// ber-hak tanggungan/fidusia, dinilai penilai independen dalam 1 tahun terakhir, dan nilai
// hak tanggungan paling sedikit menutup seluruh kewajiban debitur (outstanding).
func Pasal20Ayat4Exception(c *LoanCollateral, outstanding decimal.Decimal, asOf time.Time) bool {
	if c == nil || c.CollateralType != CollateralTanahBangunan {
		return false
	}
	if !c.Certified || !c.Mortgaged {
		return false
	}
	if !c.AppraiserIndependent || !dinilaiDalamTahunTerakhir(c.AppraisalDate, asOf) {
		return false
	}
	return c.MortgageValue.GreaterThanOrEqual(outstanding)
}

// PPAPPasal20Context membawa keadaan kredit yang dibutuhkan saat menilai pengurang agunan:
// tanggal perhitungan, kualitasnya (penurunan waktu hanya untuk Macet), tanggal macet, dan
// baki debet sebagai pembanding nilai hak tanggungan pada Pasal 20 ayat (4).
type PPAPPasal20Context struct {
	AsOf           time.Time
	Collectibility Collectibility
	MacetAt        *time.Time
	Outstanding    decimal.Decimal
}

// PPAPCollateralDeduction menghitung nilai pengurang satu agunan setelah seluruh syarat
// Pasal 20 dan Pasal 21 diterapkan. Agunan yang tidak lolos salah satu syarat menghasilkan
// nol, bukan nilai bawaan: itulah yang diminta Pasal 20 ayat (2).
func PPAPCollateralDeduction(c *LoanCollateral, ctx PPAPPasal20Context) decimal.Decimal {
	if !c.Pasal21Eligible() {
		return decimal.Zero
	}
	rate, ok := Pasal20Rate(c, ctx.AsOf)
	if !ok {
		return decimal.Zero
	}

	// Pasal 20 ayat (1) adalah batas atas; kebijakan haircut bank boleh lebih konservatif,
	// jadi yang dipakai adalah yang lebih kecil dari keduanya.
	nilai := c.AppraisalValue.Mul(rate)
	if bound := c.HaircutBoundAmount(); bound.LessThan(nilai) {
		nilai = bound
	}
	if nilai.IsNegative() {
		nilai = decimal.Zero
	}

	// Penurunan menurut waktu hanya berlaku untuk kualitas Macet dan ditahan oleh
	// pengecualian Pasal 20 ayat (4).
	if ctx.Collectibility == KolMacet && !Pasal20Ayat4Exception(c, ctx.Outstanding, ctx.AsOf) {
		nilai = nilai.Mul(Pasal20TimeFactor(c, ctx.MacetAt, ctx.AsOf))
	}
	return nilai
}

// PPAPCollateralDeductionTotal menjumlahkan pengurang seluruh agunan satu kredit.
func PPAPCollateralDeductionTotal(collaterals []LoanCollateral, ctx PPAPPasal20Context) decimal.Decimal {
	total := decimal.Zero
	for i := range collaterals {
		total = total.Add(PPAPCollateralDeduction(&collaterals[i], ctx))
	}
	return total
}

// --- Agunan tunai: Pasal 17 jo. Pasal 19 ayat (4) huruf b ---

// CashCollateralValue adalah nilai agunan tunai yang mengecualikan bagian Aset Produktif
// dari PPKA umum. Hanya agunan aktif dan yang ditandai IsCash yang dihitung. Pemenuhan
// syarat Pasal 17 ayat (3) (diblokir, ada surat kuasa pencairan, jangka waktu pemblokiran,
// pengikatan hukum, dan bukti kepemilikan) menjadi tanggung jawab operator yang menandainya;
// domain tidak boleh menebak syarat yang tidak tersimpan.
func (c *LoanCollateral) CashCollateralValue() decimal.Decimal {
	if c == nil || !c.IsCash || !c.IsActive() {
		return decimal.Zero
	}
	return c.AppraisalValue
}

// PPAPCashCollateralTotal menjumlahkan nilai agunan tunai seluruh agunan satu kredit.
// Nilai ini TIDAK dipakai mengurangi PPKA khusus (agunan tunai bukan daftar Pasal 20 ayat
// (1)); yang memakainya adalah dasar PPKA umum.
func PPAPCashCollateralTotal(collaterals []LoanCollateral) decimal.Decimal {
	total := decimal.Zero
	for i := range collaterals {
		total = total.Add(collaterals[i].CashCollateralValue())
	}
	return total
}

// PPAPGeneralBase menghitung dasar PPKA umum (Pasal 19 ayat (2)) setelah mengecualikan
// bagian Aset Produktif yang dijamin agunan tunai (Pasal 19 ayat (4) huruf b jo. Pasal 17).
// Lantai nol dipakai karena jaminan yang melebihi baki debet tidak boleh membuat dasar
// pengenaan negatif.
func PPAPGeneralBase(outstanding, cashCollateral decimal.Decimal) decimal.Decimal {
	if cashCollateral.LessThanOrEqual(decimal.Zero) {
		return outstanding
	}
	base := outstanding.Sub(cashCollateral)
	if base.IsNegative() {
		return decimal.Zero
	}
	return base
}

// dinilaiDalamTahunTerakhir menilai apakah tanggal taksasi berada dalam 1 (satu) tahun
// terakhir sebelum asOf. Perbandingan dilakukan per tanggal agar jam pada tanggal yang
// sama tidak mengubah hasil.
func dinilaiDalamTahunTerakhir(t, asOf time.Time) bool {
	if t.IsZero() {
		return false
	}
	batas := tanggalSaja(asOf).AddDate(-1, 0, 0)
	return !tanggalSaja(t).Before(batas)
}

// tanggalSaja membuang komponen jam dari sebuah waktu.
func tanggalSaja(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
