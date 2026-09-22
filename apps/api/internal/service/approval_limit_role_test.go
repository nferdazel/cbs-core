package service_test

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// stubApprovalRoles adalah ApprovalLimitRoleResolver in-memory: memetakan pengguna
// ke peran matriks limit. Tanpa pemetaan, peran pengguna sendiri yang dikembalikan,
// meniru seed migrasi 000079 (approval_limit_role = peran grup).
type stubApprovalRoles struct {
	byUser map[uuid.UUID]domain.StaffRole
	err    error
}

func (s stubApprovalRoles) ResolveApprovalLimitRole(_ context.Context, userID uuid.UUID, role domain.StaffRole) (domain.StaffRole, error) {
	if s.err != nil {
		return "", s.err
	}
	if r, ok := s.byUser[userID]; ok {
		return r, nil
	}
	return role, nil
}

var _ domain.ApprovalLimitRoleResolver = stubApprovalRoles{}

// limitConfigForRoles membangun konfigurasi limit dengan nilai berbeda per peran
// agar pemilihan baris matriks dapat dibedakan dari hasilnya.
func limitConfigForRoles() *stubLimitConfig {
	return &stubLimitConfig{values: map[string]decimal.Decimal{
		"limit.teller.deposit.per_transaction":     decimal.NewFromInt(50_000_000),
		"limit.teller.deposit.daily":               decimal.NewFromInt(500_000_000),
		"limit.teller.deposit.approval_above":      decimal.NewFromInt(100_000_000),
		"limit.supervisor.deposit.per_transaction": decimal.NewFromInt(250_000_000),
		"limit.supervisor.deposit.daily":           decimal.NewFromInt(1_000_000_000),
		"limit.supervisor.deposit.approval_above":  decimal.NewFromInt(500_000_000),
	}}
}

// Tanpa kaitan grup, peran pengguna sendiri yang dipakai.
func TestApprovalLimitTanpaGrupTetapPeranSendiri(t *testing.T) {
	svc := service.NewTransactionLimitService(limitConfigForRoles(), &stubDailyDebit{}, businessDateStub())
	actor := domain.Actor{UserID: uuid.New(), Username: "teller01", Role: domain.RoleTeller}
	limit, err := svc.ForActor(context.Background(), actor, "DEPOSIT")
	if err != nil {
		t.Fatalf("ForActor: %v", err)
	}
	if !limit.PerTransaction.Equal(decimal.NewFromInt(50_000_000)) ||
		!limit.DailyAmount.Equal(decimal.NewFromInt(500_000_000)) ||
		!limit.RequiresApprovalAbove.Equal(decimal.NewFromInt(100_000_000)) {
		t.Fatalf("batas teller %+v, ingin baris matriks teller", limit)
	}
}

// Grup yang menunjuk peran lain memilih baris matriks peran itu, bukan membuat
// matriks kedua.
func TestApprovalLimitGrupMenunjukPeranLain(t *testing.T) {
	userID := uuid.New()
	resolver := stubApprovalRoles{byUser: map[uuid.UUID]domain.StaffRole{userID: domain.RoleSupervisor}}
	svc := service.NewTransactionLimitService(limitConfigForRoles(), &stubDailyDebit{}, businessDateStub(), resolver)
	actor := domain.Actor{UserID: userID, Username: "teller01", Role: domain.RoleTeller}

	limit, err := svc.ForActor(context.Background(), actor, "DEPOSIT")
	if err != nil {
		t.Fatalf("ForActor: %v", err)
	}
	if !limit.PerTransaction.Equal(decimal.NewFromInt(250_000_000)) ||
		!limit.DailyAmount.Equal(decimal.NewFromInt(1_000_000_000)) ||
		!limit.RequiresApprovalAbove.Equal(decimal.NewFromInt(500_000_000)) {
		t.Fatalf("batas grup %+v, ingin baris matriks SUPERVISOR", limit)
	}
}

// Kegagalan resolusi grup TIDAK mengunci/mengubah batas: peran pengguna yang dipakai.
func TestApprovalLimitResolverGagalTetapPeranSendiri(t *testing.T) {
	userID := uuid.New()
	resolver := stubApprovalRoles{byUser: map[uuid.UUID]domain.StaffRole{userID: domain.RoleSupervisor}, err: context.DeadlineExceeded}
	svc := service.NewTransactionLimitService(limitConfigForRoles(), &stubDailyDebit{}, businessDateStub(), resolver)
	actor := domain.Actor{UserID: userID, Username: "teller01", Role: domain.RoleTeller}

	limit, err := svc.ForActor(context.Background(), actor, "DEPOSIT")
	if err != nil {
		t.Fatalf("ForActor: %v", err)
	}
	if !limit.RequiresApprovalAbove.Equal(decimal.NewFromInt(100_000_000)) {
		t.Fatalf("ambang %s, ingin 100000000 (peran sendiri saat resolusi gagal)", limit.RequiresApprovalAbove)
	}
}

// Kesetaraan seed: untuk tiap peran, resolusi grup mengembalikan peran itu sendiri
// (seperti seed 000079), sehingga ambang efektif sebelum vs sesudah kaitan identik.
func TestApprovalLimitSeedSetaraDenganSebelumKaitan(t *testing.T) {
	config := seedBackedLimitConfig(t)
	resolver := stubApprovalRoles{}
	sebelum := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub())
	sesudah := service.NewTransactionLimitService(config, &stubDailyDebit{}, businessDateStub(), resolver)

	for _, role := range service.TransactionLimitRoles() {
		for _, txType := range service.TransactionLimitTypes() {
			actor := domain.Actor{UserID: uuid.New(), Role: role}
			a, err := sebelum.ForActor(context.Background(), actor, txType)
			if err != nil {
				t.Fatalf("sebelum %s/%s: %v", role, txType, err)
			}
			b, err := sesudah.ForActor(context.Background(), actor, txType)
			if err != nil {
				t.Fatalf("sesudah %s/%s: %v", role, txType, err)
			}
			if !a.PerTransaction.Equal(b.PerTransaction) ||
				!a.DailyAmount.Equal(b.DailyAmount) ||
				!a.RequiresApprovalAbove.Equal(b.RequiresApprovalAbove) {
				t.Fatalf("%s/%s berubah: sebelum %+v, sesudah %+v", role, txType, a, b)
			}
			if txType == "DEPOSIT" {
				t.Logf("%-11s per_transaksi sebelum=%s sesudah=%s | ambang sebelum=%s sesudah=%s (keduanya sama)",
					role, a.PerTransaction, b.PerTransaction, a.RequiresApprovalAbove, b.RequiresApprovalAbove)
			}
		}
	}
}
