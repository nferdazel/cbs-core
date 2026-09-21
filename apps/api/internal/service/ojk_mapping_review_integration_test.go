package service_test

import (
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/ojkreport"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
)

// Uji integrasi penyimpanan keputusan bank atas pemetaan COA (migrasi 000050).
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola moneyflow_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://qouver:...@127.0.0.1:55452/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiOJKMappingReview -v
//
// Repositori tidak memvalidasi baris (validasi ada di handler), jadi kode COA sintetis
// dipakai agar uji tidak bertabrakan dengan baris pemetaan lain dan mudah dibersihkan.
func TestIntegrasiOJKMappingReviewIdempoten(t *testing.T) {
	e := newMoneyEnv(t)
	repo := postgres.NewOJKMappingReviewRepository(e.db)

	coaCode := "IT-" + uuid.New().String()[:8]
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(e.ctx,
			`DELETE FROM ojk_mapping_reviews WHERE form = $1 AND coa_code = $2`, "01.00", coaCode)
	})

	countForLine := func() int {
		t.Helper()
		var n int
		if err := e.db.QueryRowContext(e.ctx,
			`SELECT COUNT(*) FROM ojk_mapping_reviews WHERE form = $1 AND coa_code = $2`,
			"01.00", coaCode).Scan(&n); err != nil {
			t.Fatalf("menghitung baris keputusan: %v", err)
		}
		return n
	}
	findForLine := func() (ojkreport.MappingReview, bool) {
		t.Helper()
		rows, err := repo.ListReviews(e.ctx)
		if err != nil {
			t.Fatalf("ListReviews: %v", err)
		}
		for _, r := range rows {
			if r.Form == "01.00" && r.COACode == coaCode {
				return r, true
			}
		}
		return ojkreport.MappingReview{}, false
	}

	firstAt := time.Date(2026, time.March, 2, 1, 0, 0, 0, time.UTC)
	first := ojkreport.MappingReview{
		Form: "01.00", COACode: coaCode, Decision: ojkreport.ReviewNoted, Note: "perlu konfirmasi",
		DecidedBy: e.actor.Username, DecidedByID: e.actor.UserID.String(), DecidedAt: firstAt,
	}
	if err := repo.UpsertReview(e.ctx, first); err != nil {
		t.Fatalf("UpsertReview pertama: %v", err)
	}
	if n := countForLine(); n != 1 {
		t.Fatalf("setelah upsert pertama jumlah baris = %d, ingin 1", n)
	}
	saved, ok := findForLine()
	if !ok {
		t.Fatal("keputusan pertama tidak ditemukan")
	}
	if saved.DecidedBy != e.actor.Username {
		t.Errorf("decided_by = %q, ingin %q", saved.DecidedBy, e.actor.Username)
	}
	if saved.DecidedByID != e.actor.UserID.String() {
		t.Errorf("decided_by_id = %q, ingin %q", saved.DecidedByID, e.actor.UserID.String())
	}
	if !saved.DecidedAt.Equal(firstAt) {
		t.Errorf("decided_at = %s, ingin %s", saved.DecidedAt, firstAt)
	}

	// Pengiriman ulang identik tidak boleh menggandakan baris.
	if err := repo.UpsertReview(e.ctx, first); err != nil {
		t.Fatalf("UpsertReview ulang identik: %v", err)
	}
	if n := countForLine(); n != 1 {
		t.Fatalf("setelah upsert identik jumlah baris = %d, ingin tetap 1", n)
	}

	// Keputusan bank boleh berubah; yang tersisa harus keputusan terakhir beserta waktunya.
	secondAt := firstAt.Add(time.Hour)
	second := ojkreport.MappingReview{
		Form: "01.00", COACode: coaCode, Decision: ojkreport.ReviewApproved, Note: "disetujui rapat",
		DecidedBy: "pejabat.web", DecidedByID: "", DecidedAt: secondAt,
	}
	if err := repo.UpsertReview(e.ctx, second); err != nil {
		t.Fatalf("UpsertReview kedua: %v", err)
	}
	if n := countForLine(); n != 1 {
		t.Fatalf("setelah upsert kedua jumlah baris = %d, ingin tetap 1", n)
	}
	updated, ok := findForLine()
	if !ok {
		t.Fatal("keputusan terbaru tidak ditemukan")
	}
	if updated.Decision != ojkreport.ReviewApproved || updated.Note != "disetujui rapat" {
		t.Errorf("keputusan terbaru = %+v", updated)
	}
	if updated.DecidedBy != "pejabat.web" {
		t.Errorf("decided_by terbaru = %q, ingin pejabat.web", updated.DecidedBy)
	}
	if !updated.DecidedAt.Equal(secondAt) {
		t.Errorf("decided_at terbaru = %s, ingin %s", updated.DecidedAt, secondAt)
	}
}
