package service_test

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi asesmen CKPN per penempatan pada bank lain (kolom XII/XXI Form 05.00)
// terhadap PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// moneyflow_integration_test.go sehingga `go test ./...` tetap hijau tanpa database.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55440/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiPABLCKPN -v

// pablCKPNInputUji adalah masukan asesmen sah untuk uji integrasi.
func pablCKPNInputUji() domain.PABLCKPNInput {
	return domain.PABLCKPNInput{
		Method:            domain.PABLCKPNMethodIndividualDCF,
		Significant:       true,
		ObjectiveEvidence: true,
		RequiredCKPN:      decimal.NewFromInt(250_000),
		IndividualTarget:  decimal.NewFromInt(250_000),
		AsOf:              time.Now().UTC(),
	}
}

// insertPABLPlacementUji menyisipkan penempatan unik pada satu cabang dan
// mengembalikan id beserta nama bank lawannya.
func insertPABLPlacementUji(t *testing.T, e *moneyEnv) (uuid.UUID, string) {
	t.Helper()
	branchID := e.ensureBranch(t, ("CK" + uuid.New().String())[:8], "Cabang Uji CKPN PABL")
	counterparty := "Bank Uji CKPN PABL " + uuid.New().String()
	id := insertLPSPlacement(t, e, "10200", counterparty, "DEPOSITO", "LANCAR", 10_000_000, 0, branchID)
	return id, counterparty
}

// TestIntegrasiPABLCKPNMengisiForm05 memverifikasi end-to-end: asesmen disimpan, lalu
// Form 05.00 kolom XII (CKPN) dan XXI (Jenis CKPN) terisi dari data itu.
func TestIntegrasiPABLCKPNMengisiForm05(t *testing.T) {
	e := newMoneyEnv(t)
	setLPSConfig(t, e, domain.CKPNPABLEnabledKey, "true")

	placementID, counterparty := insertPABLPlacementUji(t, e)
	svc := newLPSSvcForTest(e)

	got, err := svc.AssessCKPN(e.ctx, placementID, pablCKPNInputUji(), e.actor)
	if err != nil {
		t.Fatalf("AssessCKPN: %v", err)
	}
	if got.CKPN == nil || !got.CKPN.RequiredCKPN.Equal(decimal.NewFromInt(250_000)) {
		t.Fatalf("CKPN hasil %+v, mau 250000", got.CKPN)
	}

	// Bukti tersimpan: kolom CKPN terisi dan waktu asesmen tidak NULL.
	var required decimal.Decimal
	var assessed sql.NullTime
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT required_ckpn, ckpn_assessed_at FROM lps_placements WHERE id = $1`,
		placementID).Scan(&required, &assessed); err != nil {
		t.Fatalf("membaca penempatan setelah asesmen: %v", err)
	}
	if !required.Equal(decimal.NewFromInt(250_000)) || !assessed.Valid {
		t.Fatalf("required_ckpn=%s assessed=%v, mau 250000 dan terisi", required, assessed.Valid)
	}

	// Form 05.00 memakai sumber riil yang sama dengan produksi.
	src := ojkreport.RepoSource{
		Source:     ojkStubAccounting{},
		Loans:      e.loanRepo,
		Profile:    postgres.NewBankProfileRepository(e.db),
		Config:     postgres.NewSystemConfigRepository(e.db),
		Placements: postgres.NewLPSPlacementRepository(e.db),
	}
	build, err := ojkreport.NewBuilder(src).GenerateMonthlyForActor(
		e.ctx, time.Now().UTC(), "", e.actor)
	if err != nil {
		t.Fatalf("GenerateMonthlyForActor: %v", err)
	}
	form05 := ojkFindTable(build, "05.00")
	row := ojkFindRow(t, form05, counterparty)
	if cell := ojkCell(t, row, "XII"); cell != "250000" {
		t.Fatalf("Form 05.00 kolom XII = %s, ingin 250000", cell)
	}
	if cell := ojkCell(t, row, "XXI"); cell != "1" {
		t.Fatalf("Form 05.00 kolom XXI = %s, ingin 1 (individual)", cell)
	}
}

// Saklar mati menolak asesmen dan tidak menyimpan apa pun.
func TestIntegrasiPABLCKPNSaklarMatiMenolak(t *testing.T) {
	e := newMoneyEnv(t)
	setLPSConfig(t, e, domain.CKPNPABLEnabledKey, "false")

	placementID, _ := insertPABLPlacementUji(t, e)
	svc := newLPSSvcForTest(e)

	if _, err := svc.AssessCKPN(e.ctx, placementID, pablCKPNInputUji(), e.actor); !errors.Is(err, domain.ErrCKPNPABLDisabled) {
		t.Fatalf("error %v, mau ErrCKPNPABLDisabled", err)
	}

	var assessed sql.NullTime
	if err := e.db.QueryRowContext(e.ctx, `
		SELECT ckpn_assessed_at FROM lps_placements WHERE id = $1`,
		placementID).Scan(&assessed); err != nil {
		t.Fatalf("membaca penempatan: %v", err)
	}
	if assessed.Valid {
		t.Fatal("saklar mati tidak boleh menyimpan asesmen")
	}
}
