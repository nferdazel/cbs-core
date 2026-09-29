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

// Uji integrasi aturan no reuse/no recycle No. Register Form 16.00 terhadap PostgreSQL
// sungguhan. Di-skip kecuali CBS_TEST_DB_DSN diisi, mengikuti pola
// properti_no_reuse_integration_test.go.
//
//	CBS_TEST_DB_DSN='postgres://...' go test ./internal/service/ -run IntegrasiPenyertaanNoReuse -v
//
// Membuktikan: (1) penghapusan register adalah soft-delete (baris tetap ada dengan
// status NONAKTIF, bukan DELETE fisik); (2) setelah "dihapus", nomor register itu TIDAK
// dapat dipakai lagi untuk baris baru, baik lewat layanan maupun lewat INSERT langsung
// (UNIQUE penuh); (3) baris NONAKTIF tidak muncul sebagai sumber Form 16.00.

func TestIntegrasiPenyertaanNoReuseNomorRegister(t *testing.T) {
	dsn := os.Getenv("CBS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("CBS_TEST_DB_DSN tidak diisi: uji integrasi penyertaan modal dilewati")
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

	svc := service.NewPenyertaanRegisterService(db, postgres.NewPenyertaanRegisterRepository(db))
	actor := domain.Actor{Role: domain.RoleSuperAdmin}

	// Nomor unik per run agar tidak bentrok dengan sisa data pada database bersama.
	noRegister := "PM-UJI-" + uuid.NewString()[:8]
	asOf := time.Date(2026, time.March, 31, 0, 0, 0, 0, time.UTC)
	input := domain.UpdatePenyertaanItemInput{
		NoRegister:           noRegister,
		CounterpartyID:       "PL-UJI",
		MetodePenyertaanCode: domain.PenyertaanMetodeBiayaPerolehan,
		KualitasCode:         domain.PenyertaanKualitasLancar,
		TujuanPenyertaanCode: domain.PenyertaanTujuanLembagaPenunjang,
		TanggalMulai:         "2025-06-01",
		PersentasePenyertaan: decimal.NewFromInt(25),
		Nominal:              decimal.NewFromInt(1_000),
		JumlahBulanLaporan:   decimal.NewFromInt(900),
		CKPN:                 decimal.NewFromInt(10),
		CKPNAsetBaik:         decimal.NewFromInt(5),
		CKPNAsetKurangBaik:   decimal.NewFromInt(3),
		CKPNAsetTidakBaik:    decimal.NewFromInt(2),
		JenisCKPNCode:        domain.PenyertaanJenisCKPNKolektif,
		AsOf:                 asOf.Format("2006-01-02"),
	}

	item, err := svc.UpsertItem(ctx, input, actor)
	if err != nil {
		t.Fatalf("upsert awal: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecContext(ctx,
			`DELETE FROM penyertaan_modal_register WHERE no_register = $1`, noRegister); err != nil {
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
		`SELECT status, COUNT(*) OVER () FROM penyertaan_modal_register WHERE no_register = $1`,
		noRegister).Scan(&status, &jumlahBaris); err != nil {
		t.Fatalf("membaca baris setelah hapus: %v", err)
	}
	if status != domain.PenyertaanStatusNonaktif {
		t.Fatalf("status setelah hapus = %q, ingin NONAKTIF (soft-delete)", status)
	}
	if jumlahBaris != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1 (tidak dihapus fisik)", jumlahBaris)
	}

	// Baris NONAKTIF tidak menjadi sumber Form 16.00.
	report, err := svc.PenyertaanReport(ctx, asOf, actor)
	if err != nil {
		t.Fatalf("laporan setelah hapus: %v", err)
	}
	for _, r := range report.Items {
		if r.NoRegister == noRegister {
			t.Fatalf("baris NONAKTIF %s tidak boleh muncul di laporan Form 16.00", noRegister)
		}
	}

	// Nomor tidak boleh dipakai ulang lewat layanan (id baru).
	ulang := input
	ulang.CounterpartyID = "PL-UJI-2"
	if _, err := svc.UpsertItem(ctx, ulang, actor); !errors.Is(err, domain.ErrPenyertaanNoRegisterUsed) {
		t.Fatalf("nomor dipakai ulang lewat layanan: err=%v, ingin ErrPenyertaanNoRegisterUsed", err)
	}

	// Nomor juga tidak boleh dipakai ulang lewat INSERT langsung: UNIQUE penuh menjaga
	// nomor tetap terpakai walau barisnya NONAKTIF.
	if _, err := db.ExecContext(ctx, `
		INSERT INTO penyertaan_modal_register (
			id, no_register, counterparty_id, metode_penyertaan_code, kualitas_code,
			tujuan_penyertaan_code, tanggal_mulai, jenis_ckpn_code, as_of
		) VALUES ($1, $2, 'PL-UJI-3', '1', '1', '1', '2025-06-01', '1', $3)`,
		uuid.New(), noRegister, asOf.Format("2006-01-02")); err == nil {
		t.Fatal("INSERT langsung dengan nomor terpakai harus ditolak unique constraint")
	}
}
