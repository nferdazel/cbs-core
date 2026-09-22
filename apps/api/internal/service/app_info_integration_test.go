package service_test

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

// Uji integrasi identitas aplikasi terhadap PostgreSQL sungguhan. Di-skip kecuali
// CBS_TEST_DB_DSN diisi, mengikuti pola moneyflow_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiIdentitasAplikasi -v
//
// Membuktikan: (1) nilai seed mereproduksi tampilan lama; (2) mengubah parameter
// bank_profile.bank_name dan branding.* mengubah identitas yang dibaca, tanpa build
// ulang; (3) identitas tidak bergantung pada cakupan buku instalasi.

// newAppInfoDB membuka koneksi database uji; helper ini tidak memakai moneyEnv
// supaya tidak menyentuh berkas uji alur lain.
func newAppInfoDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi identitas aplikasi dilewati")
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
	return db, ctx
}

func newAppInfoService(t *testing.T, db *sql.DB) domain.AppInfoService {
	t.Helper()
	return service.NewAppInfoService(
		postgres.NewBankProfileRepository(db),
		service.NewSystemConfigService(postgres.NewSystemConfigRepository(db)),
	)
}

func readConfigValue(t *testing.T, db *sql.DB, ctx context.Context, key string) string {
	t.Helper()
	var value string
	if err := db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key = $1`, key).Scan(&value); err != nil {
		t.Fatalf("membaca konfigurasi %s: %v", key, err)
	}
	return value
}

func setConfigValue(t *testing.T, db *sql.DB, ctx context.Context, key, value string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ($1, $2, 'uji integrasi identitas aplikasi')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, key, value); err != nil {
		t.Fatalf("menyetel konfigurasi %s: %v", key, err)
	}
}

// TestIntegrasiIdentitasAplikasiSeedReproduksiTampilanLama memastikan migrasi 000075
// mengisi branding dengan nilai yang sama seperti tampilan sebelum dinamis.
func TestIntegrasiIdentitasAplikasiSeedReproduksiTampilanLama(t *testing.T) {
	db, ctx := newAppInfoDB(t)

	cases := map[string]string{
		domain.ConfigKeyBrandingDisplayName: domain.DefaultBrandingDisplayName,
		domain.ConfigKeyBrandingShortName:   domain.DefaultBrandingShortName,
		domain.ConfigKeyBrandingDescription: domain.DefaultBrandingDescription,
		domain.ConfigKeyBrandingLogoURL:     "",
	}
	for key, want := range cases {
		if got := readConfigValue(t, db, ctx, key); got != want {
			t.Errorf("seed %s = %q, mau %q (mereproduksi tampilan lama)", key, got, want)
		}
	}

	var bankName string
	if err := db.QueryRowContext(ctx, `SELECT bank_name FROM bank_profile WHERE id = 1`).Scan(&bankName); err != nil {
		t.Fatalf("membaca bank_profile: %v", err)
	}

	got := newAppInfoService(t, db).Get(ctx)
	if got.DisplayName != domain.DefaultBrandingDisplayName ||
		got.ShortName != domain.DefaultBrandingShortName ||
		got.Description != domain.DefaultBrandingDescription {
		t.Fatalf("identitas dari seed = %+v, mau bawaan aplikasi", got)
	}
	if got.CompanyName != bankName {
		t.Fatalf("company_name = %q, mau sama dengan bank_profile.bank_name = %q", got.CompanyName, bankName)
	}
}

// TestIntegrasiIdentitasAplikasiMengikutiPerubahanParameter membuktikan identitas
// benar-benar dibaca dari parameter: mengubah bank_profile.bank_name dan kunci
// branding mengubah hasil, tanpa build ulang. Cakupan buku SYARIAH tidak mengubah
// identitas (cakupan mengatur data, bukan identitas instalasi).
func TestIntegrasiIdentitasAplikasiMengikutiPerubahanParameter(t *testing.T) {
	db, ctx := newAppInfoDB(t)

	// Simpan nilai awal agar tidak mengganggu uji lain pada DB yang sama.
	origName := readConfigValue(t, db, ctx, domain.ConfigKeyBrandingDisplayName)
	origScope := readConfigValue(t, db, ctx, domain.ConfigKeyInstitutionBookScope)
	var origBank string
	if err := db.QueryRowContext(ctx, `SELECT bank_name FROM bank_profile WHERE id = 1`).Scan(&origBank); err != nil {
		t.Fatalf("membaca bank_profile: %v", err)
	}
	t.Cleanup(func() {
		setConfigValue(t, db, ctx, domain.ConfigKeyBrandingDisplayName, origName)
		setConfigValue(t, db, ctx, domain.ConfigKeyInstitutionBookScope, origScope)
		if _, err := db.ExecContext(ctx, `UPDATE bank_profile SET bank_name = $1, updated_at = NOW() WHERE id = 1`, origBank); err != nil {
			t.Errorf("memulihkan bank_profile: %v", err)
		}
	})

	setConfigValue(t, db, ctx, domain.ConfigKeyBrandingDisplayName, "Aplikasi Uji Dinamis")
	setConfigValue(t, db, ctx, domain.ConfigKeyInstitutionBookScope, string(domain.ScopeSyariah))
	if _, err := db.ExecContext(ctx, `UPDATE bank_profile SET bank_name = 'PT Bank Uji Dinamis', updated_at = NOW() WHERE id = 1`); err != nil {
		t.Fatalf("mengubah bank_profile: %v", err)
	}

	got := newAppInfoService(t, db).Get(ctx)
	if got.DisplayName != "Aplikasi Uji Dinamis" {
		t.Fatalf("display_name = %q, mau mengikuti parameter 'Aplikasi Uji Dinamis'", got.DisplayName)
	}
	if got.CompanyName != "PT Bank Uji Dinamis" {
		t.Fatalf("company_name = %q, mau mengikuti bank_profile.bank_name", got.CompanyName)
	}

	// Kembalikan cakupan buku ke DUAL dan pastikan identitas tidak berubah.
	setConfigValue(t, db, ctx, domain.ConfigKeyInstitutionBookScope, string(domain.ScopeDual))
	dual := newAppInfoService(t, db).Get(ctx)
	if dual != got {
		t.Fatalf("identitas berubah karena cakupan buku: DUAL=%+v, SYARIAH=%+v", dual, got)
	}
}
