package service

import (
	"context"
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
}

func NewTransactionLimitService(config domain.SystemConfigService, daily domain.DailyDebitSumReader) domain.TransactionLimitService {
	return &transactionLimitService{config: config, daily: daily}
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
	if limit.PerTransaction.IsPositive() && amount.GreaterThan(limit.PerTransaction) {
		return domain.ErrLimitPerTransaction
	}

	if limit.DailyAmount.IsPositive() && s.daily != nil {
		existing, err := s.daily.SumDebitByCreatedByAndDate(ctx, actor.DisplayName(), time.Now().UTC())
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
	return nil
}
