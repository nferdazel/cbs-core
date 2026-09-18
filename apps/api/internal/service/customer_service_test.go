package service

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// canReadRecord memutuskan akses baca satu nasabah. CustomerRecord hanya menyimpan
// branch_id, sehingga pemetaan kode cabang aktor ke id memerlukan database; tanpa
// database, kode cabang tidak pernah cocok dan arah penolakannya yang diuji.
func TestCustomerCanReadRecord(t *testing.T) {
	branchID := uuid.New()
	record := &domain.CustomerRecord{ID: uuid.New(), BranchID: &branchID}
	svc := &customerService{}

	t.Run("teller cabang lain ditolak", func(t *testing.T) {
		allowed, err := svc.canReadRecord(context.Background(), domain.Actor{Role: domain.RoleTeller, BranchCode: "001"}, record)
		if err != nil {
			t.Fatalf("canReadRecord: %v", err)
		}
		if allowed {
			t.Fatal("teller cabang lain seharusnya ditolak")
		}
	})

	t.Run("auditor lintas cabang boleh", func(t *testing.T) {
		allowed, err := svc.canReadRecord(context.Background(), domain.Actor{Role: domain.RoleAuditor}, record)
		if err != nil || !allowed {
			t.Fatalf("auditor seharusnya boleh: allowed=%v err=%v", allowed, err)
		}
	})

	t.Run("nasabah bercabang NULL tetap terlihat", func(t *testing.T) {
		allowed, err := svc.canReadRecord(context.Background(), domain.Actor{Role: domain.RoleTeller, BranchCode: "001"}, &domain.CustomerRecord{ID: uuid.New()})
		if err != nil || !allowed {
			t.Fatalf("data pra-migrasi tanpa cabang tidak boleh ditolak: allowed=%v err=%v", allowed, err)
		}
	})

	t.Run("aktor tanpa kode cabang ditolak atas data bercabang", func(t *testing.T) {
		allowed, err := svc.canReadRecord(context.Background(), domain.Actor{Role: domain.RoleTeller}, record)
		if err != nil {
			t.Fatalf("canReadRecord: %v", err)
		}
		if allowed {
			t.Fatal("aktor tanpa kode cabang seharusnya ditolak atas data bercabang")
		}
	})
}
