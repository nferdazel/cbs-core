package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi ekspor OJK yang diperluas (Form 00.00/05.00/06.00 dan rasio NPL)
// terhadap PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti
// pola moneyflow_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55440/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiOJK -v

// ojkStubAccounting adalah Source journal-based kosong: uji ini memverifikasi angka
// kredit/penempatan/profil dari database, bukan angka jurnal.
type ojkStubAccounting struct{}

func (ojkStubAccounting) GetBalanceSheet(_ context.Context, _ time.Time, _ string) (*domain.BalanceSheet, error) {
	return &domain.BalanceSheet{}, nil
}

func (ojkStubAccounting) GetIncomeStatement(_ context.Context, _, _ time.Time, _ string) (*domain.IncomeStatement, error) {
	return &domain.IncomeStatement{}, nil
}

// setKreditOJK memaksa kualitas, CKPN, dan baki debet kredit untuk uji (jalur
// produksi tidak memberi masukan kualitas manual).
func setKreditOJK(t *testing.T, e *moneyEnv, loanID uuid.UUID, collectibility string, dpd int, outstanding, ckpn int64) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE loans
		SET collectibility=$1, dpd=$2, outstanding_principal=$3, required_ckpn=$4
		WHERE id=$5`,
		collectibility, dpd, decimal.NewFromInt(outstanding), decimal.NewFromInt(ckpn), loanID); err != nil {
		t.Fatalf("menyetel data OJK kredit: %v", err)
	}
}

func ojkFindLine(b *ojkreport.Bundle, form, sandi string) ojkreport.Line {
	for _, s := range b.Sections {
		if s.Form != form {
			continue
		}
		for _, l := range s.Lines {
			if l.Sandi == sandi {
				return l
			}
		}
	}
	return ojkreport.Line{}
}

func ojkFindTable(b *ojkreport.Bundle, form string) ojkreport.TableSection {
	for _, s := range b.Tables {
		if s.Form == form {
			return s
		}
	}
	return ojkreport.TableSection{}
}

func ojkFindRow(t *testing.T, sec ojkreport.TableSection, key string) ojkreport.TableRow {
	t.Helper()
	for _, r := range sec.Rows {
		if r.Key == key {
			return r
		}
	}
	t.Fatalf("baris %s pada form %s tidak ditemukan", key, sec.Form)
	return ojkreport.TableRow{}
}

func ojkCell(t *testing.T, row ojkreport.TableRow, sandi string) string {
	t.Helper()
	for _, c := range row.Cells {
		if c.Sandi == sandi {
			return c.Value
		}
	}
	t.Fatalf("kolom %s pada baris %s tidak ditemukan", sandi, row.Key)
	return ""
}

// insertOJKeCustomer menyisipkan nasabah langsung tanpa layanan produksi: uji ini
// memverifikasi angka kredit/penempatan, bukan jalur pendaftaran nasabah.
func insertOJKeCustomer(t *testing.T, e *moneyEnv, label string) uuid.UUID {
	t.Helper()
	cif := "OJK-" + uuid.New().String()[:12]
	id := uuid.New()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO customers (id, cif_number, full_name, id_card_number, branch_id, index_key_version)
		VALUES ($1, $2, $3, $2, (SELECT id FROM branches WHERE code='001'), 'k1')`,
		id, cif, label); err != nil {
		t.Fatalf("menyisipkan nasabah uji: %v", err)
	}
	return id
}

// TestIntegrasiOJKFormDaftarDanNPL memverifikasi angka di database: Form 06.00
// menampilkan kredit per debitur, Form 05.00 penempatan, Form 00.00 profil bank,
// dan rasio NPL dihitung dari kualitas kredit serta CKPN yang tersimpan.
func TestIntegrasiOJKFormDaftarDanNPL(t *testing.T) {
	e := newMoneyEnv(t)

	// Rasio NPL Form 00.08 bersifat BANK-WIDE dan tidak dapat di-scope ke satu cabang
	// oleh penyaji. Karena database yang sama juga diisi uji integrasi lain, angka
	// eksaknya tidak boleh diasumsikan 40/36 dari data uji ini saja. Yang dilakukan:
	// ekspektasi bank-wide dihitung dari baris kredit yang benar-benar ada (jalur data
	// yang sama dengan builder), lalu rasio 40% gross / 36% neto dibuktikan sebagai
	// KONTRIBUSI data uji ini terhadap agregat (selisih sebelum vs sesudah), sehingga
	// asersi tetap bermakna walau ada kredit uji lain.
	base := ojkNPLBankWide(t, e)

	// Data uji ditempatkan pada cabang unik agar Form 06.00 per kredit dapat ditemukan
	// tanpa bentrok nomor kredit uji lain.
	branchID := e.ensureBranch(t, ckpnTestBranchCode("J"), "Cabang Uji OJK")

	// Dua kredit pada cabang yang sama: satu lancar, satu kurang lancar.
	custA := insertOJKeCustomer(t, e, "Debitur OJK Lancar")
	acctA := e.newAccountInBranch(t, custA, branchID)
	loanA := e.disburse(t, custA, acctA, decimal.NewFromInt(1_000_000), 12)
	setKreditOJK(t, e, loanA.ID, "1_LANCAR", 0, 600_000, 0)

	custB := insertOJKeCustomer(t, e, "Debitur OJK Kurang Lancar")
	acctB := e.newAccountInBranch(t, custB, branchID)
	loanB := e.disburse(t, custB, acctB, decimal.NewFromInt(1_000_000), 12)
	setKreditOJK(t, e, loanB.ID, "3_KURANG_LANCAR", 45, 400_000, 40_000)

	full := ojkNPLBankWide(t, e)

	// Profil bank (Form 00.00) dan kunci konfigurasi identitas.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE bank_profile SET bank_name='BPR Uji Integrasi OJK', address='Jl. Uji 1',
			city='Bandung', phone='022-000', npwp='01.234.567.8-901.000' WHERE id=1`); err != nil {
		t.Fatalf("menyetel profil bank: %v", err)
	}
	e.simpanPulihkanConfigKunci(t, "ojk.bank.email")
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description) VALUES ('ojk.bank.email','ojk@uji.local','uji')
		ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`); err != nil {
		t.Fatalf("menyetel konfigurasi: %v", err)
	}

	// Penempatan pada bank lain (Form 05.00) dengan as_of sebelum akhir periode uji.
	counterparty := "Bank Uji Integrasi " + time.Now().Format("150405.000")
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO lps_placements (coa_code, counterparty_bank, placement_type, outstanding, lps_guaranteed, collectibility, as_of, branch_id)
		VALUES ('10200', $1, 'DEPOSITO', 10000000, 5000000, 'LANCAR', DATE '2025-12-31',
		        (SELECT id FROM branches WHERE code='001'))`, counterparty); err != nil {
		t.Fatalf("menyisipkan penempatan: %v", err)
	}

	src := ojkreport.RepoSource{
		Source:     ojkStubAccounting{},
		Loans:      e.loanRepo,
		Profile:    postgres.NewBankProfileRepository(e.db),
		Config:     postgres.NewSystemConfigRepository(e.db),
		Placements: postgres.NewLPSPlacementRepository(e.db),
	}
	period := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	b, err := ojkreport.NewBuilder(src).GenerateMonthlyForActor(e.ctx, period, "", e.actor)
	if err != nil {
		t.Fatalf("GenerateMonthlyForActor: %v", err)
	}

	// Form 06.00: baki debet, kualitas, dan CKPN berasal dari database.
	form06 := ojkFindTable(b, "06.00")
	rowB := ojkFindRow(t, form06, loanB.LoanNumber)
	if got := ojkCell(t, rowB, "XXVIII"); got != "400000" {
		t.Errorf("Form 06.00 baki debet LN-B = %s, ingin 400000", got)
	}
	if got := ojkCell(t, rowB, "XIV"); got != "3" {
		t.Errorf("Form 06.00 kualitas LN-B = %s, ingin 3", got)
	}
	if got := ojkCell(t, rowB, "XXXIV"); got != "40000" {
		t.Errorf("Form 06.00 CKPN LN-B = %s, ingin 40000", got)
	}

	// Form 05.00: penempatan dari lps_placements.
	form05 := ojkFindTable(b, "05.00")
	rowP := ojkFindRow(t, form05, counterparty)
	if got := ojkCell(t, rowP, "IX"); got != "10000000" {
		t.Errorf("Form 05.00 jumlah = %s, ingin 10000000", got)
	}
	if got := ojkCell(t, rowP, "VII"); got != "1" {
		t.Errorf("Form 05.00 kualitas = %s, ingin 1", got)
	}

	// Form 00.00: profil bank dari konfigurasi.
	form00 := ojkFindTable(b, "00.00")
	rowName := ojkFindRow(t, form00, "1. Nama BPR")
	if got := ojkCell(t, rowName, "NILAI"); got != "BPR Uji Integrasi OJK" {
		t.Errorf("Form 00.00 nama = %s", got)
	}

	// Form 00.08: NPL dihitung bank-wide oleh builder. Ekspektasi dihitung dari baris
	// kredit yang sama dengan yang dibaca builder, bukan angka tetap: dengan begitu
	// asersi tetap sah ketika database juga memuat kredit uji lain.
	wantGross, ok := ojkreport.RumusNPLGross(full.KurangLancar, full.Diragukan, full.Macet, full.TotalKredit)
	if !ok {
		t.Fatal("total kredit bank-wide nol; ekspektasi NPL tidak dapat dihitung")
	}
	wantNet, ok := ojkreport.RumusNPLNeto(full.KurangLancar, full.Diragukan, full.Macet, full.CKPNNPL, full.TotalKredit)
	if !ok {
		t.Fatal("total kredit bank-wide nol; ekspektasi NPL neto tidak dapat dihitung")
	}
	gross := ojkFindLine(b, "00.08", "0204")
	if gross.UnavailableReason != "" {
		t.Fatalf("NPL gross belum tersedia: %s", gross.UnavailableReason)
	}
	if !gross.Amount.Equal(wantGross) {
		t.Errorf("NPL gross = %s, ingin %s (dihitung dari baris kredit bank-wide)", gross.Amount, wantGross)
	}
	neto := ojkFindLine(b, "00.08", "0203")
	if !neto.Amount.Equal(wantNet) {
		t.Errorf("NPL neto = %s, ingin %s (dihitung dari baris kredit bank-wide)", neto.Amount, wantNet)
	}

	// Tetap buktikan angka yang dimaksud: kontribusi data uji ini adalah kredit kurang
	// lancar 400.000 dari tambahan total kredit 1.000.000, yaitu NPL gross 40% dan neto
	// (400.000 - CKPN 40.000)/1.000.000 = 36%. Selisih dihitung dari data yang benar
	// ada, sehingga tidak bergantung pada isi database di luar data uji.
	deltaKurangLancar := full.KurangLancar.Sub(base.KurangLancar)
	deltaDiragukan := full.Diragukan.Sub(base.Diragukan)
	deltaMacet := full.Macet.Sub(base.Macet)
	deltaCKPN := full.CKPNNPL.Sub(base.CKPNNPL)
	deltaTotal := full.TotalKredit.Sub(base.TotalKredit)
	if !deltaTotal.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("kontribusi total kredit data uji %s, ingin 1000000", deltaTotal)
	}
	nplDelta := deltaKurangLancar.Add(deltaDiragukan).Add(deltaMacet)
	if !nplDelta.Equal(decimal.NewFromInt(400_000)) {
		t.Fatalf("kontribusi NPL data uji %s, ingin 400000", nplDelta)
	}
	if !deltaCKPN.Equal(decimal.NewFromInt(40_000)) {
		t.Fatalf("kontribusi CKPN NPL data uji %s, ingin 40000", deltaCKPN)
	}
	deltaGross, ok := ojkreport.RumusNPLGross(deltaKurangLancar, deltaDiragukan, deltaMacet, deltaTotal)
	if !ok || !deltaGross.Equal(decimal.NewFromInt(40)) {
		t.Fatalf("kontribusi NPL gross data uji %s, ingin 40", deltaGross)
	}
	deltaNet, ok := ojkreport.RumusNPLNeto(deltaKurangLancar, deltaDiragukan, deltaMacet, deltaCKPN, deltaTotal)
	if !ok || !deltaNet.Equal(decimal.NewFromInt(36)) {
		t.Fatalf("kontribusi NPL neto data uji %s, ingin 36", deltaNet)
	}
}

// ojkNPLBankWide membaca komponen NPL bank-wide lewat jalur data yang sama dengan
// builder (RepoSource.ListLoansForOJK), sehingga ekspektasi uji selalu mengikuti isi
// database yang sebenarnya. Aktor lintas cabang diperlukan kebijakan bank-wide.
func ojkNPLBankWide(t *testing.T, e *moneyEnv) ojkreport.KomponenKreditNPL {
	t.Helper()
	rows, err := (ojkreport.RepoSource{Loans: e.loanRepo}).ListLoansForOJK(e.ctx, time.Now().UTC(), e.actor)
	if err != nil {
		t.Fatalf("membaca baris kredit bank-wide untuk OJK: %v", err)
	}
	return ojkreport.NPLDariLoanRows(rows)
}
