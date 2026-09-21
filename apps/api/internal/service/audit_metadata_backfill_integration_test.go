package service_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"cbs-core/apps/core-api/internal/repository/postgres"
	"github.com/google/uuid"
)

// Uji integrasi backfill metadata alasan penolakan kredit (migrasi 000064) terhadap
// PostgreSQL sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// loan_number_reject_integration_test.go.
//
//	TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiBackfillMetadata -v
//
// Uji ini meniru keadaan produksi pra-000061: baris audit penolakan dengan
// metadata NULL tetapi changes memuat reason, ditambah satu baris aksi lain yang juga
// punya reason (tidak boleh ikut) dan satu baris yang metadata-nya sudah terisi
// (tidak boleh ditimpa). Migrasi dijalankan langsung dari berkasnya dua kali untuk
// membuktikan idempotensi.

const w3BackfillMigration = "000064_audit_metadata_backfill.up.sql"

// w3JalankanMigrasi menjalankan berkas migrasi apa adanya. Tanpa argumen, driver pgx
// memakai protokol sederhana sehingga beberapa pernyataan (DISABLE TRIGGER, UPDATE,
// ENABLE TRIGGER) berjalan sebagai satu transaksi implisit.
func w3JalankanMigrasi(t *testing.T, e *moneyEnv) {
	t.Helper()
	path := filepath.Join(configSeedRepoRoot(t), "packages", "db-migrations", w3BackfillMigration)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("membaca migrasi %s: %v", w3BackfillMigration, err)
	}
	if _, err := e.db.ExecContext(e.ctx, string(data)); err != nil {
		t.Fatalf("menjalankan migrasi %s: %v", w3BackfillMigration, err)
	}
}

// w3TulisAudit menyisipkan baris audit mentah dengan metadata yang bisa NULL.
func w3TulisAudit(t *testing.T, e *moneyEnv, action, resourceID, changes string, metadata any) {
	t.Helper()
	_, err := e.db.ExecContext(e.ctx, `
		INSERT INTO audit_logs (actor_id, actor_role, action, resource_type, resource_id, changes, metadata, created_at)
		VALUES ($1, $2, $3, 'loan', $4, $5::jsonb, $6, NOW())`,
		e.actor.Username, string(e.actor.Role), action, resourceID, changes, metadata)
	if err != nil {
		t.Fatalf("menyisipkan audit %s %s: %v", action, resourceID, err)
	}
}

// w3Keadaan membaca (metadata->>'reason', metadata IS NULL) satu baris audit.
func w3Keadaan(t *testing.T, e *moneyEnv, resourceID string) (alasan sql.NullString, kosong bool) {
	t.Helper()
	if err := e.db.QueryRowContext(e.ctx,
		`SELECT metadata->>'reason', metadata IS NULL FROM audit_logs WHERE resource_id = $1`,
		resourceID).Scan(&alasan, &kosong); err != nil {
		t.Fatalf("membaca metadata audit %s: %v", resourceID, err)
	}
	return alasan, kosong
}

func TestIntegrasiBackfillMetadataPenolakanLama(t *testing.T) {
	e := newMoneyEnv(t)

	// id unik agar tidak bentrok dengan baris uji lain pada database yang sama.
	penolakanLama := "w3-lama-" + uuid.NewString()
	aksiLain := "w3-lain-" + uuid.NewString()
	sudahTerisi := "w3-terisi-" + uuid.NewString()

	alasanLama := "agunan tidak memenuhi syarat (pra-migrasi)"
	alasanLain := "hapus buku tanpa alasan penolakan"
	alasanChanges := "alasan baru di changes"
	alasanSudahAda := "alasan sudah ada di metadata"

	// 1. penolakan pra-migrasi: metadata NULL, changes.reason ada.
	w3TulisAudit(t, e, "REJECT_LOAN", penolakanLama,
		`{"status":"REJECTED","amount":"3000000","reason":"`+alasanLama+`"}`, nil)
	// 2. aksi lain (bukan penolakan kredit) dengan changes.reason: tidak boleh ikut.
	w3TulisAudit(t, e, "WRITE_OFF_LOAN", aksiLain,
		`{"reason":"`+alasanLain+`"}`, nil)
	// 3. penolakan yang metadata-nya sudah terisi: tidak boleh ditimpa.
	w3TulisAudit(t, e, "REJECT_LOAN", sudahTerisi,
		`{"status":"REJECTED","reason":"`+alasanChanges+`"}`,
		`{"reason":"`+alasanSudahAda+`"}`)

	assertKeadaan := func(tahap string) {
		t.Helper()
		alasan, kosong := w3Keadaan(t, e, penolakanLama)
		if kosong || !alasan.Valid || alasan.String != alasanLama {
			t.Fatalf("%s: penolakan lama metadata=%v (kosong=%v), ingin reason %q", tahap, alasan, kosong, alasanLama)
		}
		alasan, kosong = w3Keadaan(t, e, aksiLain)
		if !kosong {
			t.Fatalf("%s: aksi non-penolakan ikut ter-backfill menjadi %v, ingin tetap NULL", tahap, alasan)
		}
		alasan, kosong = w3Keadaan(t, e, sudahTerisi)
		if kosong || !alasan.Valid || alasan.String != alasanSudahAda {
			t.Fatalf("%s: metadata yang sudah terisi berubah menjadi %v (kosong=%v), ingin %q", tahap, alasan, kosong, alasanSudahAda)
		}
	}

	w3JalankanMigrasi(t, e)
	assertKeadaan("setelah jalan pertama")

	// Jalan kedua harus tidak mengubah apa pun, termasuk baris yang sudah terisi.
	w3JalankanMigrasi(t, e)
	assertKeadaan("setelah jalan kedua")

	// Pembaca audit (repo) harus kini melihat alasan penolakan lama di metadata.
	events, err := postgres.NewAuditRepository(e.db).List(e.ctx, "loan", penolakanLama, 10)
	if err != nil {
		t.Fatalf("membaca audit lewat repositori: %v", err)
	}
	ditemukan := false
	for _, ev := range events {
		if ev.Action != "REJECT_LOAN" {
			continue
		}
		ditemukan = true
		if got, _ := ev.Metadata["reason"].(string); got != alasanLama {
			t.Fatalf("alasan penolakan lama dari metadata repo %q, ingin %q", got, alasanLama)
		}
	}
	if !ditemukan {
		t.Fatal("audit REJECT_LOAN lama tidak ditemukan lewat repositori audit")
	}
}
