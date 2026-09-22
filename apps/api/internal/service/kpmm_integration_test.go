package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/shopspring/decimal"
)

// Uji integrasi KPMM terhadap PostgreSQL sungguhan: laporan posisi keuangan nyata
// (journal-based) dikalikan bobot risiko dari system_config (migrasi 000082), lalu
// pengurang modal inti dari selisih PPKA-CKPN diterapkan. Di-skip kecuali
// CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiKPMM -v
//
// CKPN di-stub karena menyalakan ckpn.enabled dilarang; perhitungan CKPN sendiri
// sudah diuji terpisah. Uji ini menanam portofolio jurnal terkendali agar angka
// tangan dapat dibandingkan dengan keluaran sistem.

type kpmmIntegCKPNStub struct {
	domain.CKPNService
	perKredit decimal.Decimal
	agregat   decimal.Decimal
}

func (s kpmmIntegCKPNStub) Compare(context.Context, time.Time, domain.Actor) (domain.CKPNComparisonSummary, error) {
	return domain.CKPNComparisonSummary{
		Enabled:            true,
		Processed:          2,
		ModalIntiDeduction: s.perKredit,
		Difference:         s.agregat,
	}, nil
}

// kpmmCleanPortfolio menghapus sisa jurnal uji KPMM agar baseline sebelum penanaman
// bersih dan kontribusi portofolio dapat diukur tepat walau uji dijalankan berulang.
func kpmmCleanPortfolio(t *testing.T, e *moneyEnv) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		DELETE FROM journal_lines WHERE journal_entry_id IN
			(SELECT id FROM journal_entries WHERE reference_number LIKE 'KPMM-IT-%')`); err != nil {
		t.Fatalf("membersihkan baris jurnal KPMM: %v", err)
	}
	if _, err := e.db.ExecContext(e.ctx, `
		DELETE FROM journal_entries WHERE reference_number LIKE 'KPMM-IT-%'`); err != nil {
		t.Fatalf("membersihkan jurnal KPMM: %v", err)
	}
}

// kpmmSeedPortfolio menanam portofolio jurnal terkendali (aset, kewajiban, ekuitas)
// dengan jurnal langsung agar angka laporan posisi keuangan pasti. Portofolio:
//
//	Kas 100jt, Penempatan 50jt, Kredit 1M, CKPN -20jt, AYDA 10jt, Aset tetap
//	200jt, Akum. penyusutan -50jt, Antarkantor 5jt, Aset lain 1jt, Tabungan
//	646jt, Modal disetor 500jt, Laba ditahan 100jt, Laba tahun berjalan 50jt.
//	Aset neto 1.296jt = kewajiban 646jt + ekuitas 650jt (neraca seimbang).
func kpmmSeedPortfolio(t *testing.T, e *moneyEnv, asOf time.Time) {
	t.Helper()

	type baris struct {
		coa       string
		direction string
		amount    decimal.Decimal
	}
	barisAset := []baris{
		{"10100", "DEBIT", idr(100_000_000)},
		{"10200", "DEBIT", idr(50_000_000)},
		{"10300", "DEBIT", idr(1_000_000_000)},
		{"10900", "CREDIT", idr(20_000_000)},
		{"10500", "DEBIT", idr(10_000_000)},
		{"10600", "DEBIT", idr(200_000_000)},
		{"10700", "CREDIT", idr(50_000_000)},
		{"10800", "DEBIT", idr(5_000_000)},
		{"10999", "DEBIT", idr(1_000_000)},
		{"20100", "CREDIT", idr(646_000_000)},
		{"30100", "CREDIT", idr(500_000_000)},
		{"30200", "CREDIT", idr(100_000_000)},
		{"30300", "CREDIT", idr(50_000_000)},
	}

	for i, b := range barisAset {
		accountNumber := "KPMM-IT-" + b.coa
		if _, err := e.db.ExecContext(e.ctx, `
			INSERT INTO accounts (account_number, coa_id, account_type, status, currency, branch_id)
			VALUES ($1, (SELECT id FROM chart_of_accounts WHERE code=$2), 'INTERNAL_GL', 'ACTIVE', 'IDR',
			        (SELECT id FROM branches WHERE code='001'))
			ON CONFLICT (account_number) DO NOTHING`, accountNumber, b.coa); err != nil {
			t.Fatalf("menyiapkan akun KPMM %s: %v", b.coa, err)
		}
		ref := "KPMM-IT-" + b.coa
		var entryID string
		if err := e.db.QueryRowContext(e.ctx, `
			INSERT INTO journal_entries (reference_number, transaction_type, description, entry_date, branch_id, created_by)
			VALUES ($1, 'ADJUSTMENT', 'portofolio uji KPMM', $2, (SELECT id FROM branches WHERE code='001'), 'SYSTEM')
			RETURNING id`, ref, asOf.Format("2006-01-02")).Scan(&entryID); err != nil {
			t.Fatalf("menyisipkan jurnal KPMM %s: %v", b.coa, err)
		}
		if _, err := e.db.ExecContext(e.ctx, `
			INSERT INTO journal_lines (journal_entry_id, account_id, direction, amount, balance_after, sequence, description)
			VALUES ($1, (SELECT id FROM accounts WHERE account_number=$2), $3, $4, 0, $5, 'KPMM IT')`,
			entryID, accountNumber, b.direction, b.amount, i+1); err != nil {
			t.Fatalf("menyisipkan baris jurnal KPMM %s: %v", b.coa, err)
		}
	}
}

// hitungATMRManual menghitung ATMR di luar service memakai bobot yang sama dibaca
// langsung dari konfigurasi, sehingga hasil service dapat dibandingkan dengan
// hitungan tangan atas portofolio (baris neraca) yang benar-benar ada.
func hitungATMRManual(t *testing.T, rows []domain.ReportRow, bobot map[string]decimal.Decimal) decimal.Decimal {
	t.Helper()
	total := decimal.Zero
	for _, r := range rows {
		kategori, ok := ojkreport.KategoriRisikoCOA(r.AccountCode)
		if !ok {
			continue
		}
		w, ada := bobot[kategori]
		if !ada {
			t.Fatalf("bobot kategori %q tidak tersedia", kategori)
		}
		total = total.Add(r.Amount.Mul(w))
	}
	return total
}

func TestIntegrasiKPMM(t *testing.T) {
	e := newMoneyEnv(t)
	t.Cleanup(func() { kpmmCleanPortfolio(t, e) })
	reportSvc := service.NewReportService(postgres.NewReportRepository(e.db))
	asOf := time.Now().UTC()

	kpmmCleanPortfolio(t, e)
	sebelum, err := reportSvc.GetBalanceSheet(e.ctx, asOf, "")
	if err != nil {
		t.Fatalf("GetBalanceSheet sebelum: %v", err)
	}
	kpmmSeedPortfolio(t, e, asOf)
	bs, err := reportSvc.GetBalanceSheet(e.ctx, asOf, "")
	if err != nil {
		t.Fatalf("GetBalanceSheet: %v", err)
	}

	bobot := map[string]decimal.Decimal{
		ojkreport.RisikoKas:         idr(0),
		ojkreport.RisikoAntarBank:   decimal.NewFromFloat(0.20),
		ojkreport.RisikoKredit:      idr(1),
		ojkreport.RisikoAYDA:        idr(1),
		ojkreport.RisikoAsetTetap:   idr(1),
		ojkreport.RisikoAntarKantor: idr(1),
		ojkreport.RisikoLainnya:     idr(1),
	}
	atmrManual := hitungATMRManual(t, bs.Rows, bobot)
	modalUtamaManual := bs.TotalEquity.Add(bs.NetIncome)
	pengurang := idr(25_000_000)
	modalIntiManual := modalUtamaManual.Sub(pengurang)
	rasioManual := modalIntiManual.Div(atmrManual).Mul(decimal.NewFromInt(100))

	// Kontribusi portofolio uji ini terhadap agregat: ATMR 1.156.000.000 dan modal
	// utama 650.000.000, dihitung dari selisih sebelum vs sesudah.
	atmrKontribusi := hitungATMRManual(t, bs.Rows, bobot).Sub(hitungATMRManual(t, sebelum.Rows, bobot))
	modalKontribusi := bs.TotalEquity.Add(bs.NetIncome).Sub(sebelum.TotalEquity.Add(sebelum.NetIncome))
	if !atmrKontribusi.Equal(idr(1_156_000_000)) {
		t.Fatalf("kontribusi ATMR portofolio uji %s, ingin 1156000000", atmrKontribusi)
	}
	if !modalKontribusi.Equal(idr(650_000_000)) {
		t.Fatalf("kontribusi modal inti utama portofolio uji %s, ingin 650000000", modalKontribusi)
	}

	svc := service.NewKPMMService(reportSvc, kpmmIntegCKPNStub{perKredit: pengurang}, e.configSvc)
	report, err := svc.Hitung(e.ctx, asOf, "", e.actor)
	if err != nil {
		t.Fatalf("Hitung: %v", err)
	}

	t.Logf("ATMR manual=%s sistem=%s (tersedia=%v)", atmrManual, report.ATMR.Nilai, report.ATMR.Tersedia)
	t.Logf("modal inti manual=%s sistem=%s", modalIntiManual, report.ModalInti.Nilai)
	t.Logf("rasio manual=%s sistem=%s", rasioManual, report.RasioKPMM.Nilai)

	if !report.ATMR.Nilai.Equal(atmrManual) {
		t.Fatalf("ATMR sistem %s tidak sama dengan hitungan manual %s", report.ATMR.Nilai, atmrManual)
	}
	if !report.ModalInti.Nilai.Equal(modalIntiManual) {
		t.Fatalf("modal inti sistem %s tidak sama dengan hitungan manual %s", report.ModalInti.Nilai, modalIntiManual)
	}
	if !report.RasioKPMM.Tersedia || !report.RasioKPMM.Nilai.Equal(rasioManual) {
		t.Fatalf("rasio KPMM sistem %s (tersedia=%v) tidak sama dengan manual %s",
			report.RasioKPMM.Nilai, report.RasioKPMM.Tersedia, rasioManual)
	}
	if !report.KPMMMinFrac.Equal(decimal.NewFromFloat(0.12)) {
		t.Fatalf("kpmm.min_frac terbaca %s, mau 0.12", report.KPMMMinFrac)
	}

	// Efek pengurang modal inti: tanpa pengurang, modal inti lebih tinggi tepat
	// sebesar pengurang dan rasio ikut naik.
	svcTanpa := service.NewKPMMService(reportSvc, kpmmIntegCKPNStub{}, e.configSvc)
	tanpa, err := svcTanpa.Hitung(e.ctx, asOf, "", e.actor)
	if err != nil {
		t.Fatalf("Hitung tanpa pengurang: %v", err)
	}
	if !tanpa.ModalInti.Nilai.Sub(report.ModalInti.Nilai).Equal(pengurang) {
		t.Fatalf("selisih modal inti %s, mau %s", tanpa.ModalInti.Nilai.Sub(report.ModalInti.Nilai), pengurang)
	}
	if !tanpa.RasioKPMM.Nilai.GreaterThan(report.RasioKPMM.Nilai) {
		t.Fatalf("rasio tanpa pengurang %s harus lebih besar dari %s", tanpa.RasioKPMM.Nilai, report.RasioKPMM.Nilai)
	}
}
