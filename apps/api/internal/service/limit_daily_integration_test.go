package service_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
)

// Uji integrasi dua celah batas harian terhadap PostgreSQL sungguhan. Mengikuti pola
// deposit_approval_integration_test.go: di-skip kecuali CBS_TEST_DB_DSN diisi.
//
//	CBS_TEST_DB_DSN='postgres://qouver@127.0.0.1:55474/cbs?sslmode=disable' \
//	  go test ./internal/service/ -run IntegrasiBatasHarian -v
//
// Celah 1: akumulasi harian harus memakai TANGGAL BISNIS (entry_date jurnal dan
// business_date pengajuan), bukan kalender UTC created_at. Celah 2: dua persetujuan
// bersamaan untuk pembuat/jenis/tanggal bisnis yang sama harus terserialkan sehingga
// tidak ada kombinasi yang melewati batas harian.

// setBusinessDate menulis tanggal bisnis ke system_config; dibaca BusinessDateRepository
// setiap kali tanpa cache, jadi perubahan langsung berlaku.
func setBusinessDate(t *testing.T, e *depositApprovalEnv, date time.Time) {
	t.Helper()
	if _, err := e.money.db.ExecContext(e.money.ctx, `
		INSERT INTO system_config (key, value, description)
		VALUES ('system.business_date', $1, 'tanggal bisnis untuk uji batas harian')
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, date.Format("2006-01-02")); err != nil {
		t.Fatalf("menyetel tanggal bisnis: %v", err)
	}
}

func tellerDepositInput(customerID, productID uuid.UUID, amount int64) domain.PlaceDepositInput {
	return domain.PlaceDepositInput{
		CustomerID:      customerID,
		ProductID:       productID,
		PlacementAmount: idr(amount),
		TermMonths:      3,
		Currency:        "IDR",
		BranchCode:      "001",
	}
}

// Celah 1: penempatan pada tanggal bisnis kemarin tidak boleh dihitung untuk hari ini
// (kalender UTC), tetapi HARUS dihitung untuk tanggal bisnis itu. Asersi terakhir
// membuktikan penjaga batas memakai tanggal bisnis: dengan batas harian 5 juta dan
// akumulasi tanggal bisnis 5 juta, nominal 1 rupiah pun ditolak. Bila memakai kalender
// UTC, akumulasi nol dan nominal itu lolos.
func TestIntegrasiBatasHarianPakaiTanggalBisnis(t *testing.T) {
	e := newDepositApprovalEnv(t)
	// Nama pembuat unik per uji (dan per kali jalan) agar akumulasi tidak tercampur
	// uji lain atau sisa jalan sebelumnya di database yang sama; akumulasi dihitung
	// per created_by.
	e.teller.Username = fmt.Sprintf("teller.batas.tanggal.%d", time.Now().UnixNano())
	ctx := e.money.ctx

	// Penempatan langsung (tanpa ambang, tanpa batas per transaksi) agar jurnal
	// benar-benar terposting pada tanggal bisnis yang diuji.
	setLimitConfig(t, e.money, "limit.teller.deposit.per_transaction", "0")
	setLimitConfig(t, e.money, "limit.teller.deposit.approval_above", "0")
	t.Cleanup(func() {
		setLimitConfig(t, e.money, "limit.teller.deposit.per_transaction", "50000000")
		setLimitConfig(t, e.money, "limit.teller.deposit.approval_above", "100000000")
	})

	businessDate := time.Now().UTC().AddDate(0, 0, -1)
	setBusinessDate(t, e, businessDate)
	t.Cleanup(func() { setBusinessDate(t, e, time.Now().UTC()) })

	cust := e.money.newCustomer(t, "Nasabah Tanggal Bisnis", "")
	product, err := e.money.productRepo.GetByCode(ctx, "DEP-CONV")
	if err != nil {
		t.Fatalf("membaca produk deposito: %v", err)
	}
	if _, err := e.svc.Place(ctx, tellerDepositInput(cust.ID, product.ID, 5_000_000), e.teller); err != nil {
		t.Fatalf("penempatan pada tanggal bisnis kemarin: %v", err)
	}

	ledgerRepo := postgres.NewLedgerRepository(e.money.db)
	onTodayUTC, err := ledgerRepo.SumDebitByCreatedByAndDate(ctx, e.teller.Username, time.Now().UTC())
	if err != nil {
		t.Fatalf("membaca akumulasi hari UTC: %v", err)
	}
	if !onTodayUTC.IsZero() {
		t.Fatalf("akumulasi dengan kalender UTC hari ini %s, mau 0: jurnal bertanggal bisnis kemarin tidak boleh terhitung hari ini", onTodayUTC)
	}

	onBusinessDate, err := ledgerRepo.SumDebitByCreatedByAndDate(ctx, e.teller.Username, businessDate)
	if err != nil {
		t.Fatalf("membaca akumulasi tanggal bisnis: %v", err)
	}
	if !onBusinessDate.Equal(idr(5_000_000)) {
		t.Fatalf("akumulasi tanggal bisnis %s, mau 5000000", onBusinessDate)
	}

	setLimitConfig(t, e.money, "limit.teller.deposit.daily", "5000000")
	limitSvc := service.NewTransactionLimitService(e.money.configSvc, ledgerRepo, postgres.NewBusinessDateRepository(e.money.db))
	if err := limitSvc.Check(ctx, e.teller, "deposit", idr(1)); !errors.Is(err, domain.ErrLimitDaily) {
		t.Fatalf("cek batas harian pada tanggal bisnis: dapat %v, mau ErrLimitDaily", err)
	}
}

// Celah 2: dua persetujuan bersamaan masing-masing 150 juta dengan batas harian 250
// juta. Sebelum perbaikan keduanya lolos (total 300 juta). Sesudah perbaikan tepat satu
// sukses dan satu gagal ErrLimitDaily tanpa menulis jurnal, pada 15 percobaan berturut.
func TestIntegrasiBatasHarianDuaPersetujuanBersamaan(t *testing.T) {
	e := newDepositApprovalEnv(t)
	// Nama pembuat unik per uji (dan per kali jalan) agar akumulasi tidak tercampur
	// uji lain atau sisa jalan sebelumnya.
	e.teller.Username = fmt.Sprintf("teller.batas.balapan.%d", time.Now().UnixNano())
	ctx := e.money.ctx

	product, err := e.money.productRepo.GetByCode(ctx, "DEP-CONV")
	if err != nil {
		t.Fatalf("membaca produk deposito: %v", err)
	}
	cust := e.money.newCustomer(t, "Nasabah Balapan Batas", "")

	const attempts = 15
	const dailyLimit = 250_000_000
	const amount = 150_000_000
	base := time.Now().UTC().AddDate(0, 0, -1)
	t.Cleanup(func() { setBusinessDate(t, e, time.Now().UTC()) })

	ledgerRepo := postgres.NewLedgerRepository(e.money.db)

	for i := 0; i < attempts; i++ {
		// Tanggal bisnis berbeda tiap percobaan agar akumulasi tiap percobaan bersih
		// dan hasilnya tidak dipengaruhi percobaan sebelumnya.
		businessDate := base.AddDate(0, 0, -i)
		setBusinessDate(t, e, businessDate)
		// Saat pengajuan dibuat batas harian dilonggarkan agar DUA pengajuan 150 juta
		// boleh sama-sama mengantre; batas 250 juta baru berlaku saat eksekusi.
		setLimitConfig(t, e.money, "limit.teller.deposit.daily", "0")

		ids := make([]uuid.UUID, 2)
		for k := range ids {
			_, err := e.svc.Place(ctx, tellerDepositInput(cust.ID, product.ID, amount), e.teller)
			var pending *domain.PendingApprovalError
			if !errors.As(err, &pending) {
				t.Fatalf("percobaan %d pengajuan %d: dapat %v, mau menunggu persetujuan", i, k, err)
			}
			ids[k] = pending.RequestID
		}

		setLimitConfig(t, e.money, "limit.teller.deposit.daily", "250000000")

		var wg sync.WaitGroup
		errs := make([]error, 2)
		for k, id := range ids {
			wg.Add(1)
			go func(k int, id uuid.UUID) {
				defer wg.Done()
				errs[k] = e.mcSvc.Approve(ctx, id, e.checker, "persetujuan bersamaan")
			}(k, id)
		}
		wg.Wait()

		sukses, kalah := 0, 0
		for k, err := range errs {
			switch {
			case err == nil:
				sukses++
			case errors.Is(err, domain.ErrLimitDaily):
				kalah++
			default:
				t.Fatalf("percobaan %d persetujuan %d gagal tak terduga: %v", i, k, err)
			}
		}
		if sukses != 1 || kalah != 1 {
			t.Fatalf("percobaan %d: sukses=%d kalah=%d, mau tepat satu sukses dan satu ErrLimitDaily", i, sukses, kalah)
		}

		total, err := ledgerRepo.SumDebitByCreatedByAndDate(ctx, e.teller.Username, businessDate)
		if err != nil {
			t.Fatalf("percobaan %d membaca akumulasi: %v", i, err)
		}
		if total.GreaterThan(idr(dailyLimit)) {
			t.Fatalf("percobaan %d: akumulasi %s melewati batas harian %d", i, total, dailyLimit)
		}
		if !total.Equal(idr(amount)) {
			t.Fatalf("percobaan %d: akumulasi %s, mau %d (permintaan yang kalah tidak boleh menulis jurnal)", i, total, amount)
		}

		// Permintaan yang kalah tetap PENDING dan dapat ditinjau ulang.
		var pendingLeft int
		if err := e.money.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM maker_checker_requests
			WHERE id = ANY($1) AND status = 'PENDING'`, ids).Scan(&pendingLeft); err != nil {
			t.Fatalf("percobaan %d menghitung pengajuan tersisa: %v", i, err)
		}
		if pendingLeft > 1 {
			t.Fatalf("percobaan %d: %d pengajuan masih PENDING, mau paling banyak satu yang kalah", i, pendingLeft)
		}
	}
}
