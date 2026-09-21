package service_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi pengurang PPKA Pasal 23 POJK No. 1 Tahun 2024 terhadap PostgreSQL
// sungguhan. Mengikuti pola moneyflow_integration_test.go: di-skip kecuali
// CBS_TEST_DB_DSN diisi, sehingga `go test ./...` tetap hijau tanpa database.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55432/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiLPS -v

// setLPSConfig menyetel satu kunci konfigurasi LPS sambil membuang cache-nya.
func setLPSConfig(t *testing.T, e *moneyEnv, key, value string) {
	t.Helper()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi Pasal 23 LPS')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
	e.configSvc.Invalidate(key)
}

// insertLPSPlacement menyisipkan satu penempatan uji pada cabang tertentu dan
// mengembalikan id-nya.
func insertLPSPlacement(t *testing.T, e *moneyEnv, coa, counterparty, placementType, collectibility string, outstanding, guaranteed int64, branchID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := e.db.ExecContext(e.ctx, `
		INSERT INTO lps_placements (id, coa_code, counterparty_bank, placement_type, outstanding, lps_guaranteed, collectibility, as_of, branch_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_DATE, $8)`,
		id, coa, counterparty, placementType, decimal.NewFromInt(outstanding),
		decimal.NewFromInt(guaranteed), collectibility, branchID); err != nil {
		t.Fatalf("menyisipkan penempatan uji: %v", err)
	}
	return id
}

func newLPSSvcForTest(e *moneyEnv) domain.LPSPlacementService {
	return service.NewLPSPlacementService(postgres.NewLPSPlacementRepository(e.db), e.configSvc)
}

// TestIntegrasiLPSPlacementPengurangPasal23 memverifikasi angka di database: pengurang
// Pasal 23 diterapkan pada PPKA umum dan khusus, dan filter cabang pada pembacaan
// penempatan menghormati aktor.
func TestIntegrasiLPSPlacementPengurangPasal23(t *testing.T) {
	e := newMoneyEnv(t)

	setLPSConfig(t, e, domain.LPSPlacementEnabledKey, "true")
	setLPSConfig(t, e, domain.LPSGuaranteeCapKey, "2000000000")

	// Cabang uji sendiri agar penempatan dari uji lain tidak ikut terbaca.
	branchA := e.ensureBranch(t, ("LA" + uuid.New().String())[:8], "Cabang Uji LPS A")
	branchB := e.ensureBranch(t, ("LB" + uuid.New().String())[:8], "Cabang Uji LPS B")
	branchACode := branchCodeOf(t, e, branchA)
	actorA := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujilps", Role: domain.RoleAdmin, BranchCode: branchACode}

	// Cabang A: contoh 1 Penjelasan Pasal 23 -> PPKA umum 0,5% x (10 miliar - 2 miliar).
	placeA := insertLPSPlacement(t, e, "10200", "Bank Uji A", "DEPOSITO", "LANCAR", 10_000_000_000, 2_000_000_000, branchA)
	// Cabang B: contoh 2 -> PPKA khusus 10% x (10 miliar - 2 miliar).
	insertLPSPlacement(t, e, "10200", "Bank Uji B", "GIRO", "KURANG_LANCAR", 10_000_000_000, 2_000_000_000, branchB)

	svc := newLPSSvcForTest(e)

	// Aktor cabang A hanya melihat penempatan cabang A.
	summaryA, err := svc.Calculate(e.ctx, time.Now().UTC(), actorA)
	if err != nil {
		t.Fatalf("Calculate cabang A: %v", err)
	}
	if summaryA.Total != 1 || summaryA.Processed != 1 {
		t.Fatalf("filter cabang gagal: total=%d processed=%d (harus 1)", summaryA.Total, summaryA.Processed)
	}
	itemA := lpsItem(t, summaryA, placeA)
	if !itemA.Deduction.Equal(decimal.NewFromInt(2_000_000_000)) {
		t.Fatalf("pengurang %s, mau 2000000000", itemA.Deduction)
	}
	if !itemA.Base.Equal(decimal.NewFromInt(8_000_000_000)) {
		t.Fatalf("dasar %s, mau 8000000000", itemA.Base)
	}
	if !itemA.PPKA.Equal(decimal.NewFromInt(40_000_000)) || itemA.AppliesTo != "UMUM" {
		t.Fatalf("PPKA umum %s (%s), mau 40000000 (UMUM)", itemA.PPKA, itemA.AppliesTo)
	}

	// Aktor lintas cabang melihat seluruh bank, termasuk contoh 2.
	summaryAll, err := svc.Calculate(e.ctx, time.Now().UTC(), e.actor)
	if err != nil {
		t.Fatalf("Calculate lintas cabang: %v", err)
	}
	if summaryAll.Total != 2 || summaryAll.Processed != 2 {
		t.Fatalf("lintas cabang total=%d processed=%d, mau 2", summaryAll.Total, summaryAll.Processed)
	}
	if !summaryAll.TotalDeduction.Equal(decimal.NewFromInt(4_000_000_000)) {
		t.Fatalf("total pengurang %s, mau 4000000000", summaryAll.TotalDeduction)
	}
}

// Saklar mati diuji pada database: tabel sudah terisi, tetapi Calculate tidak membaca
// apa pun dan tidak mengubah angka (total nol).
func TestIntegrasiLPSPlacementSaklarMati(t *testing.T) {
	e := newMoneyEnv(t)

	setLPSConfig(t, e, domain.LPSPlacementEnabledKey, "false")
	branchID := e.ensureBranch(t, ("LC" + uuid.New().String())[:8], "Cabang Uji LPS Mati")
	insertLPSPlacement(t, e, "10200", "Bank Uji Mati", "DEPOSITO", "LANCAR", 1_000_000_000, 500_000_000, branchID)

	svc := newLPSSvcForTest(e)
	summary, err := svc.Calculate(e.ctx, time.Now().UTC(), e.actor)
	if err != nil {
		t.Fatalf("Calculate: %v", err)
	}
	if summary.Enabled {
		t.Fatal("saklar mati tetapi ringkasan mengaku aktif")
	}
	if summary.Total != 0 || len(summary.Items) != 0 {
		t.Fatalf("saklar mati tidak boleh membaca: total=%d items=%d", summary.Total, len(summary.Items))
	}
}

// branchCodeOf membaca kode cabang dari id, karena actor memakai kode, bukan id.
func branchCodeOf(t *testing.T, e *moneyEnv, branchID uuid.UUID) string {
	t.Helper()
	var code string
	if err := e.db.QueryRowContext(e.ctx, `SELECT code FROM branches WHERE id=$1`, branchID).Scan(&code); err != nil {
		t.Fatalf("membaca kode cabang: %v", err)
	}
	return code
}

// lpsItem mencari satu item dalam ringkasan berdasarkan id penempatan.
func lpsItem(t *testing.T, summary domain.LPSPlacementSummary, placementID uuid.UUID) domain.LPSPlacementItem {
	t.Helper()
	for _, it := range summary.Items {
		if it.PlacementID == placementID {
			return it
		}
	}
	t.Fatalf("penempatan %s tidak ada di ringkasan (failures: %+v)", placementID, summary.Failures)
	return domain.LPSPlacementItem{}
}
