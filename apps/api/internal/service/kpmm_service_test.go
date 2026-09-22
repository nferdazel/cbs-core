package service

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// kpmmD mempermudah penulisan angka desimal pada uji ini.
func kpmmD(s string) decimal.Decimal {
	v, err := decimal.NewFromString(s)
	if err != nil {
		panic(err)
	}
	return v
}

// --- Stub ---

// kpmmReportStub menyediakan laporan posisi keuangan untuk KPMM; method lain
// diwarisi nil dari interface.
type kpmmReportStub struct {
	domain.ReportService
	bs *domain.BalanceSheet
}

func (s kpmmReportStub) GetBalanceSheet(context.Context, time.Time, string) (*domain.BalanceSheet, error) {
	return s.bs, nil
}

// kpmmCKPNStub menyediakan ringkasan perbandingan PPKA-CKPN.
type kpmmCKPNStub struct {
	domain.CKPNService
	summary domain.CKPNComparisonSummary
}

func (s kpmmCKPNStub) Compare(context.Context, time.Time, domain.Actor) (domain.CKPNComparisonSummary, error) {
	return s.summary, nil
}

// kpmmConfig mengimplementasikan SystemConfigService sekaligus ConfigRawValueReader
// supaya kunci yang kosong dapat dibedakan dari kunci berangka.
type kpmmConfig struct {
	values map[string]string
}

func newKPMMConfig() *kpmmConfig {
	return &kpmmConfig{values: map[string]string{
		"kpmm.min_frac":                 "0.12",
		"kpmm.modal_inti_min_frac":      "0.08",
		"kpmm.modal_inti_min_amount":    "6000000000",
		"kpmm.modal_pelengkap_max_frac": "1.00",
		"kpmm.ppka_umum_rwa_max_frac":   "0.0125",
		"kpmm.rwa_frac.kas":             "0.00",
		"kpmm.rwa_frac.antar_bank":      "0.20",
		"kpmm.rwa_frac.kredit":          "1.00",
		"kpmm.rwa_frac.ayda":            "1.00",
		"kpmm.rwa_frac.aset_tetap":      "1.00",
		"kpmm.rwa_frac.antar_kantor":    "1.00",
		"kpmm.rwa_frac.lainnya":         "1.00",
	}}
}

func (c *kpmmConfig) RawValue(_ context.Context, key string) (string, bool) {
	v, ok := c.values[key]
	return v, ok
}

func (c *kpmmConfig) GetString(_ context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func (c *kpmmConfig) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := c.values[key]; ok {
		if parsed, err := decimal.NewFromString(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func (c *kpmmConfig) GetInt(_ context.Context, _ string, fallback int) int { return fallback }
func (c *kpmmConfig) GetBool(_ context.Context, _ string, fallback bool) bool {
	return fallback
}
func (c *kpmmConfig) Invalidate(string) {}

var _ domain.ConfigRawValueReader = (*kpmmConfig)(nil)

func kpmmTestBalanceSheet() *domain.BalanceSheet {
	return &domain.BalanceSheet{
		TotalEquity: kpmmD("650000000"),
		Rows: []domain.ReportRow{
			{AccountCode: "10100", Amount: kpmmD("100000000")},
			{AccountCode: "10200", Amount: kpmmD("50000000")},
			{AccountCode: "10300", Amount: kpmmD("1000000000")},
			{AccountCode: "10900", Amount: kpmmD("-20000000")},
			{AccountCode: "10500", Amount: kpmmD("10000000")},
			{AccountCode: "10600", Amount: kpmmD("200000000")},
			{AccountCode: "10700", Amount: kpmmD("-50000000")},
			{AccountCode: "10800", Amount: kpmmD("5000000")},
			{AccountCode: "10999", Amount: kpmmD("1000000")},
			{AccountCode: "20100", Amount: kpmmD("646000000")},
		},
	}
}

func kpmmSummary(deductionPerKredit, difference decimal.Decimal) domain.CKPNComparisonSummary {
	return domain.CKPNComparisonSummary{
		Enabled:            true,
		Processed:          3,
		ModalIntiDeduction: deductionPerKredit,
		Difference:         difference,
	}
}

// TestKPMMHitungManual memverifikasi angka laporan terhadap hitungan tangan:
// ATMR = 1.156.000.000; modal inti = 650.000.000 - 20.000.000 = 630.000.000;
// rasio = 630.000.000 / 1.156.000.000 x 100.
func TestKPMMHitungManual(t *testing.T) {
	svc := NewKPMMService(
		kpmmReportStub{bs: kpmmTestBalanceSheet()},
		kpmmCKPNStub{summary: kpmmSummary(kpmmD("20000000"), kpmmD("15000000"))},
		newKPMMConfig(),
	)
	report, err := svc.Hitung(context.Background(), time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC), "CONVENTIONAL", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}
	if !report.ATMR.Tersedia || !report.ATMR.Nilai.Equal(kpmmD("1156000000")) {
		t.Fatalf("ATMR = %s (tersedia=%v), mau 1156000000", report.ATMR.Nilai, report.ATMR.Tersedia)
	}
	if !report.ModalInti.Tersedia || !report.ModalInti.Nilai.Equal(kpmmD("630000000")) {
		t.Fatalf("modal inti = %s (tersedia=%v), mau 630000000", report.ModalInti.Nilai, report.ModalInti.Tersedia)
	}
	if report.DeductionBasis != "per_kredit" {
		t.Fatalf("basis = %q, mau per_kredit", report.DeductionBasis)
	}
	mau := kpmmD("630000000").Div(kpmmD("1156000000")).Mul(decimal.NewFromInt(100))
	if !report.RasioKPMM.Tersedia || !report.RasioKPMM.Nilai.Equal(mau) {
		t.Fatalf("rasio KPMM = %s (tersedia=%v), mau %s", report.RasioKPMM.Nilai, report.RasioKPMM.Tersedia, mau)
	}
	if !report.ModalPelengkap.Tersedia && report.ModalPelengkap.Alasan == "" {
		t.Fatalf("modal pelengkap harus ditandai tidak tersedia beserta alasannya")
	}
}

// TestKPMMPengurangModalIntiBerpengaruh membuktikan pengurang modal inti PPKA-CKPN
// mengubah modal dan rasio: sebelum (tanpa kelebihan PPKA) vs sesudah (PPKA > CKPN).
func TestKPMMPengurangModalIntiBerpengaruh(t *testing.T) {
	tanpaPengurang := NewKPMMService(
		kpmmReportStub{bs: kpmmTestBalanceSheet()},
		kpmmCKPNStub{summary: kpmmSummary(kpmmD("0"), kpmmD("0"))},
		newKPMMConfig(),
	)
	sebelum, err := tanpaPengurang.Hitung(context.Background(), time.Now(), "CONVENTIONAL", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung sebelum: %v", err)
	}
	denganPengurang := NewKPMMService(
		kpmmReportStub{bs: kpmmTestBalanceSheet()},
		kpmmCKPNStub{summary: kpmmSummary(kpmmD("20000000"), kpmmD("15000000"))},
		newKPMMConfig(),
	)
	sesudah, err := denganPengurang.Hitung(context.Background(), time.Now(), "CONVENTIONAL", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung sesudah: %v", err)
	}
	if sebelum.ModalInti.Nilai.Equal(sesudah.ModalInti.Nilai) {
		t.Fatalf("modal inti tidak berubah: %s", sebelum.ModalInti.Nilai)
	}
	if !sebelum.ModalInti.Nilai.Sub(sesudah.ModalInti.Nilai).Equal(kpmmD("20000000")) {
		t.Fatalf("selisih modal inti = %s, mau 20000000", sebelum.ModalInti.Nilai.Sub(sesudah.ModalInti.Nilai))
	}
	if sebelum.RasioKPMM.Nilai.LessThanOrEqual(sesudah.RasioKPMM.Nilai) {
		t.Fatalf("rasio harus turun setelah pengurang: sebelum=%s sesudah=%s", sebelum.RasioKPMM.Nilai, sesudah.RasioKPMM.Nilai)
	}
}

// TestKPMMBasisAgregat memeriksa setelan per_kredit vs agregat memakai angka yang
// berbeda sehingga pilihannya terlihat pada modal inti.
func TestKPMMBasisAgregat(t *testing.T) {
	cfg := newKPMMConfig()
	cfg.values["kpmm.deduction_basis"] = "agregat"
	svc := NewKPMMService(
		kpmmReportStub{bs: kpmmTestBalanceSheet()},
		kpmmCKPNStub{summary: kpmmSummary(kpmmD("20000000"), kpmmD("15000000"))},
		cfg,
	)
	report, err := svc.Hitung(context.Background(), time.Now(), "CONVENTIONAL", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}
	if report.DeductionBasis != "agregat" {
		t.Fatalf("basis = %q, mau agregat", report.DeductionBasis)
	}
	if !report.ModalInti.Nilai.Equal(kpmmD("635000000")) {
		t.Fatalf("modal inti agregat = %s, mau 635000000", report.ModalInti.Nilai)
	}
}

// TestKPMMCKPNBelumDihitung menandai pengurang modal inti sebagai tidak tersedia
// ketika kedua saklar CKPN mati, dan TIDAK menampilkan rasio yang menyesatkan.
func TestKPMMCKPNBelumDihitung(t *testing.T) {
	svc := NewKPMMService(
		kpmmReportStub{bs: kpmmTestBalanceSheet()},
		kpmmCKPNStub{summary: domain.CKPNComparisonSummary{}},
		newKPMMConfig(),
	)
	report, err := svc.Hitung(context.Background(), time.Now(), "CONVENTIONAL", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}
	if report.PengurangModalInti.Tersedia || report.PengurangModalInti.Alasan == "" {
		t.Fatalf("pengurang harus tidak tersedia beserta alasan: %+v", report.PengurangModalInti)
	}
	if report.ModalInti.Tersedia {
		t.Fatalf("modal inti harus ditandai belum final")
	}
	if report.RasioKPMM.Tersedia || report.RasioKPMM.Alasan == "" {
		t.Fatalf("rasio harus tidak tersedia beserta alasan: %+v", report.RasioKPMM)
	}
}

// TestKPMMATMRBelumLengkap memastikan pos tanpa pemetaan OJK menandai ATMR (dan
// rasio) belum lengkap, bukan diabaikan.
func TestKPMMATMRBelumLengkap(t *testing.T) {
	bs := kpmmTestBalanceSheet()
	bs.Rows = append(bs.Rows, domain.ReportRow{AccountCode: "10888", Amount: kpmmD("1000")})
	svc := NewKPMMService(
		kpmmReportStub{bs: bs},
		kpmmCKPNStub{summary: kpmmSummary(kpmmD("20000000"), kpmmD("15000000"))},
		newKPMMConfig(),
	)
	report, err := svc.Hitung(context.Background(), time.Now(), "CONVENTIONAL", domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}
	if report.ATMR.Tersedia {
		t.Fatalf("ATMR harus ditandai belum lengkap")
	}
	if report.RasioKPMM.Tersedia {
		t.Fatalf("rasio harus tidak tersedia bila ATMR belum lengkap")
	}
}
