package service_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// TestIntegrasiUbahParameterProduk menjalankan jalur tulis parameter produk pada
// database sungguhan: UPDATE parameter, audit sebelum->sesudah, 404 produk tak
// ditemukan, dan jaring pengaman constraint rentang. Di-skip kecuali
// CBS_TEST_DB_DSN diisi, mengikuti pola uji integrasi lain.
func TestIntegrasiUbahParameterProduk(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi parameter produk dilewati")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("membuka database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("database tidak dapat dihubungi: %v", err)
	}

	repo := postgres.NewProductRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	svc := service.NewProductService(db, repo, auditRepo)

	const code = "PMB-MUDHARABAH"
	before, err := repo.GetByCode(ctx, code)
	if err != nil {
		t.Fatalf("membaca produk uji %s: %v", code, err)
	}
	// Kembalikan parameter produk ke keadaan semula agar database uji tidak berubah.
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, `
			UPDATE banking_products SET
				rate_annual = $1, profit_sharing_ratio = $2, projected_revenue_rate_annual = $3,
				min_amount = $4, max_amount = $5, min_term_months = $6, max_term_months = $7,
				admin_fee = $8, tax_rate = $9, early_withdrawal_penalty_rate = $10, updated_at = NOW()
			WHERE code = $11`,
			before.RateAnnual, before.ProfitSharingRatio, before.ProjectedRevenueRateAnnual,
			before.MinAmount, before.MaxAmount, before.MinTermMonths, before.MaxTermMonths,
			before.AdminFee, before.TaxRate, before.EarlyWithdrawalPenaltyRate, code); err != nil {
			t.Logf("mengembalikan parameter produk: %v", err)
		}
	})

	projected := decimal.NewFromInt(12)
	ratio := decimal.RequireFromString("0.45")

	// audit_logs.staff_user_id ber-FK ke staff_users, jadi pelaku harus ada.
	// Nama unik per jalan agar aman bila uji diulang pada database yang sama.
	// Baris staf sengaja tidak dihapus: audit_logs bersifat append-only dan FK-nya
	// akan menabrak trigger penghapusan.
	staffID := uuid.New()
	username := "admin.integrasi." + staffID.String()[:8]
	if _, err := db.ExecContext(ctx, `
		INSERT INTO staff_users (id, employee_id, username, full_name, email, password_hash, role, branch_code)
		VALUES ($1, $2, $3, 'Admin Integrasi', $4, 'x', 'ADMIN', 'HO')`,
		staffID, "EMP-UJI-"+staffID.String()[:8], username, username+"@uji.local"); err != nil {
		t.Fatalf("menyiapkan staf uji: %v", err)
	}

	updated, err := svc.UpdateParams(ctx, code, domain.UpdateProductParamsInput{
		ProjectedRevenueRateAnnual: &projected,
		ProfitSharingRatio:         &ratio,
	}, domain.Actor{UserID: staffID, Username: username, Role: domain.RoleAdmin})
	if err != nil {
		t.Fatalf("UpdateParams: %v", err)
	}
	if !updated.ProjectedRevenueRateAnnual.Equal(projected) || !updated.ProfitSharingRatio.Equal(ratio) {
		t.Fatalf("hasil update = proyeksi %s nisbah %s", updated.ProjectedRevenueRateAnnual, updated.ProfitSharingRatio)
	}

	stored, err := repo.GetByCode(ctx, code)
	if err != nil {
		t.Fatalf("membaca ulang produk: %v", err)
	}
	if !stored.ProjectedRevenueRateAnnual.Equal(projected) || !stored.ProfitSharingRatio.Equal(ratio) {
		t.Fatalf("nilai tersimpan = proyeksi %s nisbah %s", stored.ProjectedRevenueRateAnnual, stored.ProfitSharingRatio)
	}

	// Audit harus memuat nilai sebelum -> sesudah pada resource produk ini.
	events, err := auditRepo.Query(ctx, domain.AuditLogFilter{
		ResourceType: "product",
		ResourceID:   before.ID.String(),
		Action:       "UPDATE_PRODUCT_PARAMS",
		Limit:        5,
	})
	if err != nil {
		t.Fatalf("membaca audit: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("tidak ada audit UPDATE_PRODUCT_PARAMS")
	}
	event := events[0]
	pair, ok := event.Changes["projected_revenue_rate_annual"].(map[string]any)
	if !ok {
		t.Fatalf("audit proyeksi = %#v, ingin pasangan before/after", event.Changes["projected_revenue_rate_annual"])
	}
	if pair["before"] != "0" || pair["after"] != "12" {
		t.Fatalf("audit proyeksi before/after = %v/%v, ingin 0/12", pair["before"], pair["after"])
	}
	if event.ActorID != username {
		t.Fatalf("pelaku audit = %q, ingin %q", event.ActorID, username)
	}

	// Produk tidak ditemukan -> sentinel 404, bukan galat SQL.
	ten := decimal.NewFromInt(10)
	if _, err := svc.UpdateParams(ctx, "TIDAK-ADA", domain.UpdateProductParamsInput{RateAnnual: &ten},
		domain.Actor{Username: "admin.integrasi", Role: domain.RoleAdmin}); !errors.Is(err, domain.ErrProductNotFound) {
		t.Fatalf("produk tak ada: err = %v, ingin ErrProductNotFound", err)
	}

	// Jaring pengaman database: nilai di luar rentang harus ditolak constraint,
	// meski service dilewati (migrasi 000060/000063).
	_, err = db.ExecContext(ctx, `UPDATE banking_products SET projected_revenue_rate_annual = 1200 WHERE code = $1`, code)
	if err == nil {
		t.Fatal("proyeksi 1200 lolos ke database; constraint rentang tidak aktif")
	}
	if !strings.Contains(err.Error(), "chk_projected_revenue_range") {
		t.Fatalf("galat constraint = %v, ingin chk_projected_revenue_range", err)
	}

	// Batas atas parameter lain juga dijaga database (migrasi 000062): nilai 101
	// untuk tarif persen harus ditolak constraint, bukan hanya oleh service.
	_, err = db.ExecContext(ctx, `UPDATE banking_products SET tax_rate = 101 WHERE code = $1`, code)
	if err == nil {
		t.Fatal("tax_rate 101 lolos ke database; constraint batas atas tidak aktif")
	}
	if !strings.Contains(err.Error(), "chk_tax_rate_range") {
		t.Fatalf("galat constraint = %v, ingin chk_tax_rate_range", err)
	}
}
