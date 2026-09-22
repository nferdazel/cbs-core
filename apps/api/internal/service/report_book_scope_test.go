package service_test

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
)

// capturingBookRepo mencatat buku yang benar-benar diteruskan service ke repository,
// supaya pembatasan dapat diuji tanpa database. Metode lain diwarisi stubReportRepo.
type capturingBookRepo struct {
	*stubReportRepo
	lastBook string
}

func (c *capturingBookRepo) TrialBalance(_ context.Context, _, _ time.Time, book string) ([]domain.TrialBalanceRow, error) {
	c.lastBook = book
	return nil, nil
}

func (c *capturingBookRepo) IncomeStatement(_ context.Context, _, _ time.Time, book string) (domain.IncomeStatement, error) {
	c.lastBook = book
	return domain.IncomeStatement{}, nil
}

func (c *capturingBookRepo) BalanceSheet(_ context.Context, _ time.Time, book string) (domain.BalanceSheet, error) {
	c.lastBook = book
	return domain.BalanceSheet{}, nil
}

func (c *capturingBookRepo) CashFlow(_ context.Context, _, _ time.Time, book string) (domain.CashFlow, error) {
	c.lastBook = book
	return domain.CashFlow{}, nil
}

// Laporan keuangan menerima query param book; aktor satu buku tidak boleh memakai
// param itu untuk membaca posisi buku lain.
func TestLaporanDibatasiBukuAktor(t *testing.T) {
	ctxWithClaims := func(role domain.StaffRole, book domain.COABook) context.Context {
		return context.WithValue(context.Background(), domain.ContextKeyClaims,
			&domain.JWTClaims{Role: role, Book: book})
	}

	cases := []struct {
		name      string
		ctx       context.Context
		requested string
		want      string
	}{
		{"aktor syariah dipaksa ke bukunya", ctxWithClaims(domain.RoleAO, domain.BookSyariah), "CONVENTIONAL", "SYARIAH"},
		{"aktor konvensional dipaksa ke bukunya", ctxWithClaims(domain.RoleAdmin, domain.BookConventional), "SYARIAH", "CONVENTIONAL"},
		{"auditor lintas buku menghormati pilihan", ctxWithClaims(domain.RoleAuditor, domain.BookConventional), "SYARIAH", "SYARIAH"},
		{"buku belum ditentukan menghormati pilihan", ctxWithClaims(domain.RoleAO, ""), "SYARIAH", "SYARIAH"},
		{"tanpa claims tidak dibatasi", context.Background(), "SYARIAH", "SYARIAH"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &capturingBookRepo{stubReportRepo: &stubReportRepo{}}
			svc := service.NewReportService(repo)
			if _, err := svc.GetTrialBalance(tc.ctx, time.Now(), time.Now(), tc.requested); err != nil {
				t.Fatalf("GetTrialBalance: %v", err)
			}
			if repo.lastBook != tc.want {
				t.Fatalf("buku diteruskan %q, ingin %q", repo.lastBook, tc.want)
			}
		})
	}
}
