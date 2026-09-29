package service_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Uji integrasi aturan no reuse/no recycle No. Register Form 17.00 terhadap PostgreSQL
// sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// app_info_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiPropertiNoReuse -v
//
// Membuktikan: (1) penghapusan register adalah soft-delete (baris tetap ada dengan
// status NONAKTIF, bukan DELETE fisik); (2) setelah "dihapus", nomor register itu TIDAK
// dapat dipakai lagi untuk baris baru, baik lewat layanan maupun lewat INSERT langsung
// (UNIQUE penuh); (3) baris NONAKTIF tidak muncul sebagai sumber Form 17.00.

// newPropertiNoReuseDB membuka koneksi database uji tanpa memakai moneyEnv supaya tidak
// menyentuh berkas uji alur lain.
func newPropertiNoReuseDB(t *testing.T) (*sql.DB, context.Context) {
	t.Helper()
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi properti terbengkalai dilewati")
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

func TestIntegrasiPropertiNoReuseNomorRegister(t *testing.T) {
	db, ctx := newPropertiNoReuseDB(t)
	svc := service.NewPropertiRegisterService(db, postgres.NewPropertiRegisterRepository(db))
	actor := domain.Actor{Role: domain.RoleSuperAdmin}

	// Nomor unik per run agar tidak bentrok dengan sisa data pada database bersama.
	noRegister := "PRP-UJI-" + uuid.NewString()[:8]
	asOf := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	input := domain.UpdatePropertiItemInput{
		NoRegister:                    noRegister,
		JenisPropertiCode:             "1",
		AlamatProperti:                "Jl. Uji No. 1",
		Koordinat:                     "-6.2, 106.8",
		TanggalPenetapan:              "2025-06-01",
		BiayaPerolehanNilaiWajar:      decimal.NewFromInt(1_000),
		AkumulasiPenyusutanAmortisasi: decimal.NewFromInt(200),
		MetodePengukuranCode:          "1",
		AsOf:                          asOf.Format("2006-01-02"),
	}

	item, err := svc.UpsertItem(ctx, input, actor)
	if err != nil {
		t.Fatalf("upsert awal: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx,
			`DELETE FROM properti_terbengkalai_register WHERE no_register = $1`, noRegister); err != nil {
			t.Errorf("membersihkan data uji: %v", err)
		}
	})

	// Soft-delete: baris tetap ada, status NONAKTIF.
	if err := svc.DeleteItem(ctx, item.ID, actor); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}
	var status string
	var jumlahBaris int
	if err := db.QueryRowContext(ctx,
		`SELECT status, COUNT(*) OVER () FROM properti_terbengkalai_register WHERE no_register = $1`,
		noRegister).Scan(&status, &jumlahBaris); err != nil {
		t.Fatalf("membaca baris setelah hapus: %v", err)
	}
	if status != domain.PropertiStatusNonaktif {
		t.Fatalf("status setelah hapus = %q, ingin NONAKTIF (soft-delete)", status)
	}
	if jumlahBaris != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1 (tidak dihapus fisik)", jumlahBaris)
	}

	// Baris NONAKTIF tidak menjadi sumber Form 17.00.
	report, err := svc.PropertiReport(ctx, asOf, actor)
	if err != nil {
		t.Fatalf("laporan setelah hapus: %v", err)
	}
	for _, r := range report.Items {
		if r.NoRegister == noRegister {
			t.Fatalf("baris NONAKTIF %s tidak boleh muncul di laporan Form 17.00", noRegister)
		}
	}

	// Nomor tidak boleh dipakai ulang lewat layanan (id baru).
	ulang := input
	ulang.AlamatProperti = "Jl. Uji No. 2"
	if _, err := svc.UpsertItem(ctx, ulang, actor); !errors.Is(err, domain.ErrPropertiNoRegisterUsed) {
		t.Fatalf("nomor dipakai ulang lewat layanan: err=%v, ingin ErrPropertiNoRegisterUsed", err)
	}

	// Nomor juga tidak boleh dipakai ulang lewat INSERT langsung: UNIQUE penuh menjaga
	// nomor tetap terpakai walau barisnya NONAKTIF.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO properti_terbengkalai_register (
			id, no_register, jenis_properti_code, alamat_properti, koordinat,
			tanggal_penetapan, biaya_perolehan_atau_nilai_wajar,
			akumulasi_penyusutan_atau_amortisasi, metode_pengukuran_code, as_of
		) VALUES ($1, $2, '1', 'Jl. Uji No. 3', '', '2025-06-01', 0, 0, '1', $3)`,
		uuid.New(), noRegister, asOf.Format("2006-01-02")); err == nil {
		t.Fatal("INSERT langsung dengan nomor terpakai harus ditolak unique constraint")
	}
}
