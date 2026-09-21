package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Default limit dipakai bila key system_config belum diisi. Ini tempat sementara:
// nilai produksi wajib diisi di tabel system_config saat provisioning, karena limit
// per role dan per jenis transaksi adalah kebijakan bank, bukan konstanta kode.
var (
	defaultPerTransactionLimit = decimal.NewFromInt(50_000_000)
	defaultDailyLimit          = decimal.NewFromInt(500_000_000)
	defaultApprovalAbove       = decimal.NewFromInt(10_000_000)
)

type transactionLimitService struct {
	config domain.SystemConfigService
	daily  domain.DailyDebitSumReader
	// dates adalah sumber tanggal bisnis bank (WIB). Akumulasi harian WAJIB disaring
	// dengan tanggal bisnis, bukan tanggal kalender UTC: jurnal membawa entry_date dari
	// tanggal bisnis, sehingga saat tutup hari tertinggal, menyaring hari kalender UTC
	// membuat jurnal hari bisnis berjalan tidak terhitung (dan batas harian lolos).
	dates domain.BusinessDateProvider
}

func NewTransactionLimitService(config domain.SystemConfigService, daily domain.DailyDebitSumReader, dates domain.BusinessDateProvider) domain.TransactionLimitService {
	return &transactionLimitService{config: config, daily: daily, dates: dates}
}

// currentBusinessDate membaca tanggal bisnis berjalan dari repositori tanggal, bukan
// dari kalender. Tanggal yang tidak tersedia menolak evaluasi batas alih-alih menebak
// hari kalender: penilaian harian yang salah lebih berbahaya daripada penolakan jelas.
func (s *transactionLimitService) currentBusinessDate(ctx context.Context) (time.Time, error) {
	if s.dates == nil {
		return time.Time{}, errors.New("batas harian: sumber tanggal bisnis belum terpasang")
	}
	date, err := s.dates.CurrentBusinessDate(ctx)
	if err != nil {
		return time.Time{}, err
	}
	if date.IsZero() {
		return time.Time{}, errors.New("batas harian: tanggal bisnis tidak tersedia")
	}
	return date, nil
}

// limitKey membangun key konfigurasi: limit.<role>.<txtype>.<suffix> dengan role dan
// jenis transaksi huruf kecil, mis. limit.teller.deposit.per_transaction.
func limitKey(actor domain.Actor, txType, suffix string) string {
	return fmt.Sprintf("limit.%s.%s.%s",
		strings.ToLower(string(actor.Role)),
		strings.ToLower(strings.TrimSpace(txType)),
		suffix,
	)
}

func (s *transactionLimitService) ForActor(ctx context.Context, actor domain.Actor, txType string) (domain.TransactionLimit, error) {
	return domain.TransactionLimit{
		PerTransaction:        s.config.GetDecimal(ctx, limitKey(actor, txType, "per_transaction"), defaultPerTransactionLimit),
		DailyAmount:           s.config.GetDecimal(ctx, limitKey(actor, txType, "daily"), defaultDailyLimit),
		RequiresApprovalAbove: s.config.GetDecimal(ctx, limitKey(actor, txType, "approval_above"), defaultApprovalAbove),
	}, nil
}

// TransactionLimitRoles mengembalikan seluruh peran yang batas transaksinya dibaca
// penjaga batas. Urutannya sengaja tetap agar keluaran endpoint baca stabil dan test
// invarian dapat membandingkannya dengan seed migrasi.
func TransactionLimitRoles() []domain.StaffRole {
	return []domain.StaffRole{
		domain.RoleSuperAdmin,
		domain.RoleAdmin,
		domain.RoleSupervisor,
		domain.RoleTeller,
		domain.RoleCS,
		domain.RoleAO,
		domain.RoleAuditor,
	}
}

// TransactionLimitTypes mengembalikan jenis transaksi yang benar-benar dijaga penjaga
// batas, yaitu tiga panggilan guardLimit di ledger_service.go (deposit, withdrawal,
// transfer). Ditulis huruf besar untuk tampilan; limitKey menurunkannya ke huruf kecil
// sehingga sama dengan kunci yang di-seed.
func TransactionLimitTypes() []string {
	return []string{"DEPOSIT", "WITHDRAWAL", "TRANSFER"}
}

// List mengembalikan batas efektif setiap peran x jenis transaksi. Configured bernilai
// true hanya bila ketiga kunci baris itu ada di system_config; bila tidak, sebagian
// nilainya masih bawaan aplikasi dan bank belum menetapkannya.
func (s *transactionLimitService) List(ctx context.Context) ([]domain.TransactionLimitView, error) {
	exister, canCheck := s.config.(domain.ConfigKeyExister)
	roles := TransactionLimitRoles()
	txTypes := TransactionLimitTypes()
	views := make([]domain.TransactionLimitView, 0, len(roles)*len(txTypes))

	for _, role := range roles {
		actor := domain.Actor{Role: role}
		for _, txType := range txTypes {
			limit, err := s.ForActor(ctx, actor, txType)
			if err != nil {
				return nil, err
			}
			configured := false
			if canCheck {
				configured = exister.Exists(ctx, limitKey(actor, txType, "per_transaction")) &&
					exister.Exists(ctx, limitKey(actor, txType, "daily")) &&
					exister.Exists(ctx, limitKey(actor, txType, "approval_above"))
			}
			views = append(views, domain.TransactionLimitView{
				Role:            role,
				TransactionType: txType,
				PerTransaction:  limit.PerTransaction,
				DailyLimit:      limit.DailyAmount,
				ApprovalAbove:   limit.RequiresApprovalAbove,
				Configured:      configured,
			})
		}
	}
	return views, nil
}

func (s *transactionLimitService) Check(ctx context.Context, actor domain.Actor, txType string, amount decimal.Decimal) error {
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.ErrInvalidAmount
	}

	limit, err := s.ForActor(ctx, actor, txType)
	if err != nil {
		return err
	}

	// 0 berarti tanpa batas, mengikuti konvensi kunci limit.* di system_config.
	//
	// URUTAN EVALUASI ini disengaja dan pernah menjadi bug produksi:
	//
	//   1. Akumulasi harian diperiksa LEBIH DULU karena ia batas KERAS yang mengikat
	//      siapa pun; persetujuan pejabat TIDAK boleh menjadi jalan melampauinya.
	//      Akumulasi itu memuat jurnal terposting DAN pengajuan yang masih PENDING,
	//      sehingga antrean persetujuan tidak dapat dipakai menumpuk nominal.
	//   2. Ambang persetujuan diperiksa SEBELUM batas per transaksi.
	//   3. Batas per transaksi diperiksa terakhir.
	//
	// ALASAN urutan 2-3: konfigurasi yang lazim menaruh per_transaction DI BAWAH
	// approval_above (mis. teller deposit 50 juta < 100 juta). Bila per transaksi
	// diperiksa lebih dulu, ambang persetujuan tidak pernah tercapai: setoran 60 juta
	// DAN 150 juta dua-duanya ditolak 422, sehingga jalur persetujuan atasan praktis
	// mati. Nominal di atas approval_above harus dialihkan ke maker-checker (202),
	// sedangkan nominal di atas per_transaction tetapi tidak melewati approval_above
	// tetap ditolak 422.
	//
	// Peran tanpa batas (per_transaction = 0, mis. ADMIN) tidak berubah perilakunya:
	// pemeriksaan per transaksi dilewati, dan nominal yang tidak melewati ambang
	// persetujuan tetap langsung lolos.
	if limit.DailyAmount.IsPositive() && s.daily != nil {
		existing, err := s.accumulatedDaily(ctx, actor, txType)
		if err != nil {
			return err
		}
		if existing.Add(amount).GreaterThan(limit.DailyAmount) {
			return domain.ErrLimitDaily
		}
	}

	if limit.RequiresApprovalAbove.IsPositive() && amount.GreaterThan(limit.RequiresApprovalAbove) {
		return domain.ErrRequiresApproval
	}

	if limit.PerTransaction.IsPositive() && amount.GreaterThan(limit.PerTransaction) {
		return domain.ErrLimitPerTransaction
	}
	return nil
}

// pendingActionTypes menerjemahkan jenis transaksi batas (huruf kecil, sama dengan
// sufiks limit.<peran>.<jenis>) ke jenis aksi maker-checker yang menyumbang ke
// akumulasi harian jenis itu. Penempatan deposito (PLACE_DEPOSIT) memakai batas
// limit.<peran>.deposit.* yang sama dengan setoran tunai (DEPOSIT), jadi keduanya
// harus ikut dihitung.
func pendingActionTypes(txType string) []string {
	switch strings.ToLower(strings.TrimSpace(txType)) {
	case "deposit":
		return []string{ActionDeposit, ActionPlaceDeposit}
	case "withdrawal":
		return []string{ActionWithdraw}
	case "transfer":
		return []string{ActionTransfer}
	default:
		return nil
	}
}

// accumulatedDaily menjumlahkan akumulasi harian satu pembuat: jurnal yang SUDAH
// terposting DITAMBAH pengajuan yang masih menunggu persetujuan. Pengajuan PENDING
// harus ikut dihitung; tanpa itu batas harian dapat dilewati lewat antrean persetujuan
// (N pengajuan masing-masing di bawah batas harian dapat disetujui semua).
//
// Tanggal acuan adalah TANGGAL BISNIS berjalan (WIB), bukan hari kalender UTC: jurnal
// bertanggal entry_date = tanggal bisnis, dan pengajuan PENDING membawa business_date
// saat dibuat. Keduanya harus jatuh pada hari bisnis yang sama agar akumulasi berarti.
func (s *transactionLimitService) accumulatedDaily(ctx context.Context, maker domain.Actor, txType string) (decimal.Decimal, error) {
	date, err := s.currentBusinessDate(ctx)
	if err != nil {
		return decimal.Zero, err
	}
	total, err := s.daily.SumDebitByCreatedByAndDate(ctx, maker.DisplayName(), date)
	if err != nil {
		return decimal.Zero, err
	}
	for _, action := range pendingActionTypes(txType) {
		pending, err := s.daily.SumPendingDebitByMakerAndAction(ctx, maker.DisplayName(), action, date)
		if err != nil {
			return decimal.Zero, err
		}
		total = total.Add(pending)
	}
	return total, nil
}

// CheckDailyAtExecution mengevaluasi ULANG hanya batas harian saat pengajuan yang
// sudah disetujui dieksekusi. Persetujuan pejabat melegalkan nominal di atas ambang
// dan batas per transaksi, tetapi bukan pelanggaran batas harian; karena itu ambang
// persetujuan dan batas per transaksi sengaja tidak diperiksa di sini.
//
// Yang dihitung hanya jurnal yang SUDAH terposting, bukan pengajuan PENDING lain:
// pengajuan yang sedang dieksekusi belum terposting, dan menghitung PENDING lain akan
// membuat seluruh antrean saling memblokir. Bila sudah melampaui batas, eksekusi gagal
// (pengajuan tetap PENDING dan dapat ditolak/ditinjau), bukan diam-diam jalan.
//
// Evaluasi diserialkan terhadap eksekusi lain untuk pembuat/jenis/tanggal bisnis yang
// sama lewat kunci advisory di dalam transaksi tx (lihat LockDailyEvaluation). Tanpa
// itu dua persetujuan bersamaan sama-sama membaca "masih di bawah batas" dari snapshot
// yang sama, lalu keduanya memposting. Peran tanpa batas harian keluar sebelum kunci
// diambil, sehingga tidak pernah terkena serialisasi.
func (s *transactionLimitService) CheckDailyAtExecution(ctx context.Context, tx any, maker domain.Actor, txType string, amount decimal.Decimal) error {
	if s.daily == nil {
		return nil
	}
	limit, err := s.ForActor(ctx, maker, txType)
	if err != nil {
		return err
	}
	// 0 berarti tanpa batas: keluar sebelum mengambil kunci agar peran tanpa batas
	// tidak pernah menunggu evaluasi peran lain.
	if !limit.DailyAmount.IsPositive() {
		return nil
	}
	date, err := s.currentBusinessDate(ctx)
	if err != nil {
		return err
	}
	// Kunci diambil SEBELUM membaca akumulasi dan SEBELUM jurnal ditulis. Bila evaluasi
	// gagal, transaksi pemanggil rollback sehingga kunci terlepas tanpa jurnal tertinggal.
	if err := s.daily.LockDailyEvaluation(ctx, tx, maker.DisplayName(), txType, date); err != nil {
		return err
	}
	posted, err := s.daily.SumDebitByCreatedByAndDate(ctx, maker.DisplayName(), date)
	if err != nil {
		return err
	}
	if posted.Add(amount).GreaterThan(limit.DailyAmount) {
		return domain.ErrLimitDaily
	}
	return nil
}
