package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Kunci COA CKPN per unit usaha (migrasi 000071). Kredit berbook SYARIAH boleh
// memakai akun beban/cadangan sendiri; bila kuncinya kosong, perilakunya harus
// sama persis dengan sebelum kunci syariah ada.

func TestCKPNResolveCKPNCOAUrutanSyariahLaluGlobal(t *testing.T) {
	cases := []struct {
		name    string
		book    domain.COABook
		syariah string
		global  string
		values  map[string]string
		want    string
	}{
		{
			name:    "konvensional memakai kunci global",
			book:    domain.BookConventional,
			syariah: "ckpn.coa.expense.syariah",
			global:  "ckpn.coa.expense",
			values:  map[string]string{"ckpn.coa.expense": "50301", "ckpn.coa.expense.syariah": "15901"},
			want:    "50301",
		},
		{
			name:    "syariah memakai kunci syariah bila terisi",
			book:    domain.BookSyariah,
			syariah: "ckpn.coa.expense.syariah",
			global:  "ckpn.coa.expense",
			values:  map[string]string{"ckpn.coa.expense": "50301", "ckpn.coa.expense.syariah": "15901"},
			want:    "15901",
		},
		{
			name:    "syariah jatuh ke kunci global bila kunci syariah kosong",
			book:    domain.BookSyariah,
			syariah: "ckpn.coa.expense.syariah",
			global:  "ckpn.coa.expense",
			values:  map[string]string{"ckpn.coa.expense": "50301", "ckpn.coa.expense.syariah": ""},
			want:    "50301",
		},
		{
			name:    "syariah jatuh ke kunci global bila kunci syariah hanya spasi",
			book:    domain.BookSyariah,
			syariah: "ckpn.coa.expense.syariah",
			global:  "ckpn.coa.expense",
			values:  map[string]string{"ckpn.coa.expense": "50301", "ckpn.coa.expense.syariah": "   "},
			want:    "50301",
		},
		{
			name:    "syariah tanpa kedua kunci memakai fallback syariah",
			book:    domain.BookSyariah,
			syariah: "ckpn.coa.expense.syariah",
			global:  "ckpn.coa.expense",
			values:  map[string]string{},
			want:    fallbackCKPNExpenseSyariah,
		},
		{
			name:    "konvensional tanpa kedua kunci memakai fallback konvensional",
			book:    domain.BookConventional,
			syariah: "ckpn.coa.expense.syariah",
			global:  "ckpn.coa.expense",
			values:  map[string]string{},
			want:    fallbackCKPNExpenseConventional,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			local := &ckpnConfigStub{values: tc.values}
			got := resolveCKPNCOA(context.Background(), local, tc.book, tc.syariah, tc.global,
				fallbackCKPNExpenseSyariah, fallbackCKPNExpenseConventional)
			if got != tc.want {
				t.Fatalf("resolveCKPNCOA = %q, mau %q", got, tc.want)
			}
		})
	}
}

// newCKPNSyariahSvc menyiapkan service dengan kredit berproduk SYARIAH tanpa pemetaan
// jurnal CKPN, sehingga jalur fallback COA yang diuji.
func newCKPNSyariahSvc(snap domain.CKPNLoanSnapshot, cfg *ckpnConfigStub) (*ckpnService, *stubPosting) {
	productID := uuid.New()
	snap.ProductID = &productID
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, _ := newTestCKPNService(repo, cfg)
	svc.productRepo = &stubProductRepo{
		product: &domain.BankingProduct{ID: productID, Code: "PMB-MURABAHAH", Book: domain.BookSyariah},
	}
	return svc, posting
}

// ckpnSyariahConfig adalah parameter kebijakan minimum agar satu kredit dapat diposting.
func ckpnSyariahConfig(extra map[string]string) *ckpnConfigStub {
	values := map[string]string{
		"ckpn.enabled":       "true",
		"ckpn.pd_frac.gol_3": "0.10",
		"ckpn.lgd_frac":      "0.50",
	}
	for k, v := range extra {
		values[k] = v
	}
	return &ckpnConfigStub{values: values}
}

// (a) Kredit syariah + kunci syariah terisi: jurnal memakai akun syariah, bukan
// akun global konvensional.
func TestCKPN_KreditSyariahMemakaiAkunSyariah(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.RequiredCKPN = decimal.NewFromInt(100_000) // selisih 400.000 -> 1 jurnal
	cfg := ckpnSyariahConfig(map[string]string{
		"ckpn.coa.expense":         "50301",
		"ckpn.coa.reserve":         "10950",
		"ckpn.coa.expense.syariah": "15901",
		"ckpn.coa.reserve.syariah": "11950",
	})
	svc, posting := newCKPNSyariahSvc(snap, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || len(posting.requests) != 1 {
		t.Fatalf("harus 1 jurnal sukses, dapat failed=%d jurnal=%d (%+v)", summary.Failed, len(posting.requests), summary.Failures)
	}
	req := posting.requests[0]
	if req.Lines[0].AccountNumber != "15901" || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("debit harus beban syariah 15901, dapat %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != "11950" || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kredit harus cadangan syariah 11950, dapat %+v", req.Lines[1])
	}
}

// (b) Kredit syariah + kedua kunci syariah kosong: jurnal memakai akun global
// (50301/10950), identik dengan perilaku sebelum kunci syariah ada.
func TestCKPN_KreditSyariahTanpaKunciSyariahMemakaiAkunGlobal(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.RequiredCKPN = decimal.NewFromInt(100_000)
	cfg := ckpnSyariahConfig(map[string]string{
		"ckpn.coa.expense":         "50301",
		"ckpn.coa.reserve":         "10950",
		"ckpn.coa.expense.syariah": "",
		"ckpn.coa.reserve.syariah": "",
	})
	svc, posting := newCKPNSyariahSvc(snap, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || len(posting.requests) != 1 {
		t.Fatalf("harus 1 jurnal sukses, dapat failed=%d jurnal=%d (%+v)", summary.Failed, len(posting.requests), summary.Failures)
	}
	req := posting.requests[0]
	if req.Lines[0].AccountNumber != "50301" || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("debit harus beban global 50301, dapat %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != "10950" || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("kredit harus cadangan global 10950, dapat %+v", req.Lines[1])
	}
}

// ckpnFailingResolver meniru bagan akun yang tidak memuat kode COA tertentu.
type ckpnFailingResolver struct{ failCode string }

func (r ckpnFailingResolver) ResolveGLAccount(_ context.Context, _ any, coaCode string) (string, error) {
	if coaCode == r.failCode {
		return "", fmt.Errorf("akun kontrol GL untuk COA %s belum ada", coaCode)
	}
	return coaCode, nil
}

// (c) Kode akun hasil pemetaan tidak ada di bagan akun: kredit gagal dengan galat
// jelas dan TIDAK ada jurnal yang ditulis.
func TestCKPN_KodeAkunCKPNTidakAdaDitolakJelas(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		failCode string
		wantErr  error
	}{
		{"akun beban tidak ada", "15901", domain.ErrCKPNExpenseNotFound},
		{"akun cadangan tidak ada", "11950", domain.ErrCKPNReserveNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := ckpnLoan()
			snap.RequiredCKPN = decimal.NewFromInt(100_000)
			cfg := ckpnSyariahConfig(map[string]string{
				"ckpn.coa.expense":         "50301",
				"ckpn.coa.reserve":         "10950",
				"ckpn.coa.expense.syariah": "15901",
				"ckpn.coa.reserve.syariah": "11950",
			})
			svc, posting := newCKPNSyariahSvc(snap, cfg)
			svc.resolver = ckpnFailingResolver{failCode: tc.failCode}

			summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if summary.Failed != 1 || len(posting.requests) != 0 {
				t.Fatalf("kode akun tidak ada harus gagal tanpa jurnal, dapat failed=%d jurnal=%d", summary.Failed, len(posting.requests))
			}
			msg := summary.Failures[0].Error
			if !strings.Contains(msg, tc.wantErr.Error()) || !strings.Contains(msg, tc.failCode) {
				t.Fatalf("pesan %q harus memuat %q dan kode %s", msg, tc.wantErr.Error(), tc.failCode)
			}
		})
	}
}
