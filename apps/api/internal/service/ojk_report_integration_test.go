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

	// Dua kredit pada cabang yang sama: satu lancar, satu kurang lancar.
	custA := insertOJKeCustomer(t, e, "Debitur OJK Lancar")
	acctA := e.newAccount(t, custA)
	loanA := e.disburse(t, custA, acctA, decimal.NewFromInt(1_000_000), 12)
	setKreditOJK(t, e, loanA.ID, "1_LANCAR", 0, 600_000, 0)

	custB := insertOJKeCustomer(t, e, "Debitur OJK Kurang Lancar")
	acctB := e.newAccount(t, custB)
	loanB := e.disburse(t, custB, acctB, decimal.NewFromInt(1_000_000), 12)
	setKreditOJK(t, e, loanB.ID, "3_KURANG_LANCAR", 45, 400_000, 40_000)

	// Profil bank (Form 00.00) dan kunci konfigurasi identitas.
	if _, err := e.db.ExecContext(e.ctx, `
		UPDATE bank_profile SET bank_name='BPR Uji Integrasi OJK', address='Jl. Uji 1',
			city='Bandung', phone='022-000', npwp='01.234.567.8-901.000' WHERE id=1`); err != nil {
		t.Fatalf("menyetel profil bank: %v", err)
	}
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

	// Form 00.08: NPL gross = 400.000/1.000.000 = 40%; neto = (400.000-40.000)/1.000.000 = 36%.
	gross := ojkFindLine(b, "00.08", "0204")
	if gross.UnavailableReason != "" {
		t.Fatalf("NPL gross belum tersedia: %s", gross.UnavailableReason)
	}
	if !gross.Amount.Equal(decimal.NewFromInt(40)) {
		t.Errorf("NPL gross = %s, ingin 40", gross.Amount)
	}
	neto := ojkFindLine(b, "00.08", "0203")
	if !neto.Amount.Equal(decimal.NewFromInt(36)) {
		t.Errorf("NPL neto = %s, ingin 36", neto.Amount)
	}
}
