package service_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// Uji integrasi identitas Form 00.00 yang disimpan sebagai kunci system_config
// ojk.* terhadap PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiOJKProfil -v
//
// Membuktikan: (1) identitas dapat disimpan dan dibaca kembali lewat layanan, bukan
// SQL; (2) mengosongkan bidang SAH; (3) bentuk salah ditolak dengan sentinel;
// (4) field Form 00.00 yang belum diisi tetap dilaporkan "belum tersedia" beserta
// kunci yang harus diisi; (5) migrasi 000096 idempotent dan tidak menimpa nilai bank.

func TestIntegrasiOJKProfilSimpanBacaValidasi(t *testing.T) {
	db, ctx := newAppInfoDB(t)
	repo := postgres.NewOJKProfileRepository(db)
	svc := service.NewOJKProfileService(db, repo, postgres.NewAuditRepository(db))

	// Pulihkan seluruh kunci ojk.* ke keadaan semula supaya tidak mengganggu uji lain.
	original, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("membaca profil OJK awal: %v", err)
	}
	t.Cleanup(func() {
		for key, value := range original.KeyValues() {
			if _, err := db.ExecContext(ctx, `
				INSERT INTO system_config (key, value, updated_at) VALUES ($1, $2, NOW())
				ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`,
				key, value); err != nil {
				t.Errorf("memulihkan %s: %v", key, err)
			}
		}
	})

	// Aktor nyata seperti jalur API: system_config.updated_by FK ke staff_users.
	var staffID uuid.UUID
	if err := db.QueryRowContext(ctx, `SELECT id FROM staff_users ORDER BY created_at LIMIT 1`).Scan(&staffID); err != nil {
		t.Fatalf("membaca staf yang di-seed: %v", err)
	}
	actor := domain.Actor{UserID: staffID, Username: "superadmin.uji", Role: domain.RoleSuperAdmin}

	// 1. Simpan lewat layanan (jalur API), bukan SQL.
	updated, err := svc.Update(ctx, domain.UpdateOJKProfileInput{
		BankEmail:    strPtrValue("bpr@uji.integrasi"),
		BankWebsite:  strPtrValue("https://bpr.uji.integrasi"),
		BankCityCode: strPtrValue("3171"),
		PICName:      strPtrValue("Budi Uji"),
		AuditInfo:    strPtrValue("KAP Uji, opini WTP"),
	}, actor)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.BankEmail != "bpr@uji.integrasi" || updated.AuditInfo != "KAP Uji, opini WTP" {
		t.Fatalf("hasil = %+v", updated)
	}

	// 2. Baca kembali lewat repositori: nilai tersimpan benar-benar di database.
	got, err := repo.Get(ctx)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.BankWebsite != "https://bpr.uji.integrasi" || got.PICName != "Budi Uji" {
		t.Fatalf("baca kembali = %+v", got)
	}

	// 3. Mengosongkan bidang SAH: kosong berarti belum tersedia, bukan galat.
	if _, err := svc.Update(ctx, domain.UpdateOJKProfileInput{PICName: strPtrValue("")}, actor); err != nil {
		t.Fatalf("mengosongkan PIC harus boleh: %v", err)
	}
	if reread, _ := repo.Get(ctx); reread.PICName != "" {
		t.Fatalf("PICName tidak dikosongkan: %q", reread.PICName)
	}

	// 4. Bentuk salah ditolak dan tidak mengubah nilai tersimpan.
	if _, err := svc.Update(ctx, domain.UpdateOJKProfileInput{BankEmail: strPtrValue("bukan-surel")}, actor); !errors.Is(err, domain.ErrOJKProfileEmailInvalid) {
		t.Fatalf("err = %v, ingin ErrOJKProfileEmailInvalid", err)
	}
	if reread, _ := repo.Get(ctx); reread.BankEmail != "bpr@uji.integrasi" {
		t.Fatalf("nilai tersimpan berubah setelah input salah: %q", reread.BankEmail)
	}
}

// TestIntegrasiOJKForm00MelaporkanFieldKosong membangun Form 00.00 lewat jalur
// produksi dan memeriksa bahwa field kosong menyebut kunci system_config yang harus
// diisi, sedangkan field yang sudah diisi tampil apa adanya.
func TestIntegrasiOJKForm00MelaporkanFieldKosong(t *testing.T) {
	e := newMoneyEnv(t)
	ctx := e.ctx

	// Simpan dan pulihkan nama bank + kunci audit agar uji lain tidak terganggu.
	var origName string
	if err := e.db.QueryRowContext(ctx, `SELECT bank_name FROM bank_profile WHERE id=1`).Scan(&origName); err != nil {
		t.Fatalf("membaca nama bank: %v", err)
	}
	var origAudit string
	if err := e.db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key=$1`, domain.OJKAuditInfoKey).Scan(&origAudit); err != nil {
		t.Fatalf("kunci %s belum di-seed migrasi 000096: %v", domain.OJKAuditInfoKey, err)
	}
	var origEmail string
	if err := e.db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key=$1`, domain.OJKBankEmailKey).Scan(&origEmail); err != nil {
		t.Fatalf("kunci %s belum di-seed migrasi 000046: %v", domain.OJKBankEmailKey, err)
	}
	t.Cleanup(func() {
		_, _ = e.db.ExecContext(ctx, `UPDATE bank_profile SET bank_name=$1 WHERE id=1`, origName)
		_, _ = e.db.ExecContext(ctx, `UPDATE system_config SET value=$1 WHERE key=$2`, origAudit, domain.OJKAuditInfoKey)
		_, _ = e.db.ExecContext(ctx, `UPDATE system_config SET value=$1 WHERE key=$2`, origEmail, domain.OJKBankEmailKey)
	})

	if _, err := e.db.ExecContext(ctx, `UPDATE bank_profile SET bank_name='BPR Uji Integrasi OJK' WHERE id=1`); err != nil {
		t.Fatalf("menyetel nama bank: %v", err)
	}
	// Pastikan surel kosong untuk uji ini: uji lain di database yang sama dapat
	// meninggalkan nilai. Nilai dipulihkan lewat t.Cleanup di atas.
	if _, err := e.db.ExecContext(ctx, `UPDATE system_config SET value='' WHERE key=$1`, domain.OJKBankEmailKey); err != nil {
		t.Fatalf("mengosongkan surel: %v", err)
	}
	svc := service.NewOJKProfileService(e.db, postgres.NewOJKProfileRepository(e.db), postgres.NewAuditRepository(e.db))
	if _, err := svc.Update(ctx, domain.UpdateOJKProfileInput{
		AuditInfo: strPtrValue("KAP Uji, opini WTP"),
	}, e.actor); err != nil {
		t.Fatalf("mengisi audit_info: %v", err)
	}

	src := ojkreport.RepoSource{
		Source:  ojkStubAccounting{},
		Profile: postgres.NewBankProfileRepository(e.db),
		Config:  postgres.NewSystemConfigRepository(e.db),
	}
	b, err := ojkreport.NewBuilder(src).GenerateMonthlyForActor(ctx, time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC), "", e.actor)
	if err != nil {
		t.Fatalf("GenerateMonthlyForActor: %v", err)
	}
	form00 := ojkFindTable(b, "00.00")
	if form00.Form == "" {
		t.Fatal("Form 00.00 tidak ada di bundle")
	}

	// Field yang sudah diisi tampil apa adanya.
	auditRow := ojkFindRow(t, form00, "12. Informasi Audit Laporan Keuangan Tahunan (KAP/AP)")
	if got := ojkCell(t, auditRow, "NILAI"); got != "KAP Uji, opini WTP" {
		t.Errorf("Form 00.00 butir 12 = %q, ingin %q", got, "KAP Uji, opini WTP")
	}

	// Field yang belum diisi (email) tetap "belum tersedia" dan menyebut kuncinya.
	emailRow := ojkFindRow(t, form00, "6. E-mail")
	if emailRow.Reason == "" {
		t.Fatal("e-mail kosong harus dinyatakan belum tersedia")
	}
	if !strings.Contains(emailRow.Reason, domain.OJKBankEmailKey) {
		t.Errorf("alasan e-mail = %q, harus menyebut %s", emailRow.Reason, domain.OJKBankEmailKey)
	}
	// Keluaran nyata agar perilaku "belum tersedia" terlihat saat uji dijalankan -v.
	t.Logf("Form 00.00 butir 12 terisi: %q", ojkCell(t, auditRow, "NILAI"))
	t.Logf("Form 00.00 butir 6 kosong -> reason=%q", emailRow.Reason)
	t.Logf("Form 00.00 butir 21 kosong -> reason=%q", ojkFindRow(t, form00, "21. Nama Ultimate Shareholders").Reason)
}

// TestIntegrasiMigrasi000096IdempotentDanTidakMenimpa memastikan migrasi dapat
// dijalankan ulang tanpa galat dan tanpa menimpa nilai yang sudah diisi bank.
func TestIntegrasiMigrasi000096IdempotentDanTidakMenimpa(t *testing.T) {
	db, ctx := newAppInfoDB(t)

	const key = domain.OJKDividendsPaidKey
	var orig string
	if err := db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key=$1`, key).Scan(&orig); err != nil {
		t.Fatalf("kunci %s belum di-seed migrasi 000096: %v", key, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecContext(ctx, `UPDATE system_config SET value=$1 WHERE key=$2`, orig, key)
	})

	const nilaiBank = "Rp 0 (tidak membagikan dividen)"
	if _, err := db.ExecContext(ctx, `UPDATE system_config SET value=$1 WHERE key=$2`, nilaiBank, key); err != nil {
		t.Fatalf("menyetel nilai bank: %v", err)
	}

	// Jalankan ulang berkas migrasi 000096 persis seperti migrate.sh.
	path := filepath.Join("..", "..", "..", "..", "packages", "db-migrations", "000096_ojk_profile_form00_fields.up.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi: %v", err)
	}
	if _, err := db.ExecContext(ctx, string(data)); err != nil {
		t.Fatalf("menjalankan ulang migrasi 000096: %v", err)
	}

	var got string
	if err := db.QueryRowContext(ctx, `SELECT value FROM system_config WHERE key=$1`, key).Scan(&got); err != nil {
		t.Fatalf("membaca ulang: %v", err)
	}
	if got != nilaiBank {
		t.Fatalf("migrasi menimpa nilai bank: %q, ingin %q", got, nilaiBank)
	}
}
