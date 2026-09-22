package service_test

import (
	"database/sql"
	"encoding/json"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// Uji integrasi manajemen profil bank (W16) terhadap PostgreSQL sungguhan.
// Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola app_info_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiProfilBank -v
//
// Membuktikan: (1) profil kosong tetap membuat /app-info berjalan; (2) setelah
// profil diisi lewat layanan (API), nama PT tampil di /app-info DAN laporan OJK;
// (3) perubahan meninggalkan audit; (4) nilai awal dipulihkan agar uji lain aman.

func TestIntegrasiProfilBankKosongLaluDiisiLewatAPI(t *testing.T) {
	db, ctx := newAppInfoDB(t)
	repo := postgres.NewBankProfileRepository(db)

	// Simpan profil awal dan pulihkan setelah uji supaya tidak mengganggu uji lain
	// pada database yang sama.
	original, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("membaca profil awal: %v", err)
	}
	if original == nil {
		t.Fatal("baris bank_profile belum ada; migrasi 000025 tidak dijalankan")
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx, `
			UPDATE bank_profile
			SET bank_name = $1, address = $2, city = $3, phone = $4, npwp = $5, updated_at = NOW()
			WHERE id = 1`,
			original.Name, original.Address, original.City, original.Phone, original.NPWP); err != nil {
			t.Errorf("memulihkan bank_profile: %v", err)
		}
	})

	// 1. Profil kosong: identitas tetap terbaca (nama PT kosong) tanpa error/panik.
	if _, err := db.ExecContext(ctx, `
		UPDATE bank_profile SET bank_name = '', address = '', city = '', phone = '', npwp = '', updated_at = NOW()
		WHERE id = 1`); err != nil {
		t.Fatalf("mengosongkan bank_profile: %v", err)
	}
	if got := newAppInfoService(t, db).Get(ctx); got.CompanyName != "" {
		t.Fatalf("company_name profil kosong = %q, mau kosong (endpoint tetap berjalan)", got.CompanyName)
	}

	// 2. Isi profil lewat layanan yang dipakai API — bukan SQL.
	var staffID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM staff_users ORDER BY created_at LIMIT 1`).Scan(&staffID); err != nil {
		t.Fatalf("membaca staf yang di-seed: %v", err)
	}
	actor := domain.Actor{UserID: staffID, Username: "superadmin.uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}
	svc := service.NewBankProfileService(db, repo, postgres.NewAuditRepository(db))

	// Hitung audit SEBELUM perubahan: audit_logs append-only, jadi baris dari uji
	// sebelumnya bisa membuat asersi "ada audit" lulus palsu. Kita menuntut baris
	// BARU, bukan sekadar keberadaan baris lama.
	var auditBefore int
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_logs WHERE action = 'UPDATE_BANK_PROFILE' AND resource_id = '1'`).Scan(&auditBefore); err != nil {
		t.Fatalf("menghitung audit awal: %v", err)
	}

	const newName = "PT Bank Integrasi W16"
	updated, err := svc.Update(ctx, domain.UpdateBankProfileInput{
		Name:    strPtrValue(newName),
		Address: strPtrValue("Jl. Uji Integrasi No. 1"),
		City:    strPtrValue("Jakarta"),
	}, actor)
	if err != nil {
		t.Fatalf("Update profil bank: %v", err)
	}
	if updated.Name != newName || updated.City != "Jakarta" {
		t.Fatalf("profil hasil ubah = %+v", updated)
	}

	// 3. Nama PT tampil di /app-info (instance baru = setelah perubahan).
	if got := newAppInfoService(t, db).Get(ctx); got.CompanyName != newName {
		t.Fatalf("company_name setelah diisi = %q, mau %q", got.CompanyName, newName)
	}

	// 4. Departemen yang memakai profil (laporan OJK Form 00.00) juga melihatnya.
	cfg, err := ojkreport.RepoSource{
		Profile: repo,
		Config:  postgres.NewSystemConfigRepository(db),
	}.GetBankProfileConfig(ctx)
	if err != nil {
		t.Fatalf("GetBankProfileConfig: %v", err)
	}
	if cfg.Name != newName || !cfg.Configured {
		t.Fatalf("profil laporan OJK = %+v, mau name=%q configured", cfg, newName)
	}

	// 5. Perubahan teraudit: BARIS BARU dengan nilai sebelum -> sesudah tersimpan.
	var auditAfter int
	var changesRaw string
	if err := db.QueryRowContext(ctx,
		`SELECT count(*) FROM audit_logs WHERE action = 'UPDATE_BANK_PROFILE' AND resource_id = '1'`).Scan(&auditAfter); err != nil {
		t.Fatalf("menghitung audit sesudah: %v", err)
	}
	if auditAfter <= auditBefore {
		t.Fatalf("audit tidak bertambah (sebelum=%d, sesudah=%d); perubahan tidak teraudit", auditBefore, auditAfter)
	}
	if err := db.QueryRowContext(ctx, `
		SELECT changes::text FROM audit_logs
		WHERE action = 'UPDATE_BANK_PROFILE' AND resource_type = 'bank_profile' AND resource_id = '1'
		ORDER BY created_at DESC LIMIT 1`).Scan(&changesRaw); err != nil {
		if err == sql.ErrNoRows {
			t.Fatal("tidak ada audit UPDATE_BANK_PROFILE; perubahan tidak teraudit")
		}
		t.Fatalf("membaca audit: %v", err)
	}
	if !strings.Contains(changesRaw, newName) {
		t.Fatalf("audit tidak memuat nilai sesudah %q: %s", newName, changesRaw)
	}
	// Nilai "sebelum" harus mencerminkan keadaan yang benar-benar ditimpa, yaitu
	// nama kosong yang kita set tepat sebelum Update — bukan potret dari luar.
	var changes map[string]struct {
		Before string `json:"before"`
		After  string `json:"after"`
	}
	if err := json.Unmarshal([]byte(changesRaw), &changes); err != nil {
		t.Fatalf("audit changes bukan JSON objek: %v (%s)", err, changesRaw)
	}
	if name, ok := changes["name"]; !ok || name.Before != "" || name.After != newName {
		t.Fatalf("audit name before/after tidak sesuai: %s", changesRaw)
	}
}

func strPtrValue(v string) *string { return &v }
