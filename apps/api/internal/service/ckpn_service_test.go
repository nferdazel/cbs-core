package service

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// --- Stub ---

type ckpnUpdate struct {
	loanID uuid.UUID
	target decimal.Decimal
}

type ckpnRepoStub struct {
	snapshots  []domain.CKPNLoanSnapshot
	updates    []ckpnUpdate
	listCalled bool
	listErr    error
	updateErr  error
}

func (s *ckpnRepoStub) ListActiveLoans(context.Context) ([]domain.CKPNLoanSnapshot, error) {
	s.listCalled = true
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.snapshots, nil
}

func (s *ckpnRepoStub) UpdateRequiredCKPN(_ context.Context, _ any, loanID uuid.UUID, target decimal.Decimal) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.updates = append(s.updates, ckpnUpdate{loanID: loanID, target: target})
	return nil
}

var _ domain.CKPNRepository = (*ckpnRepoStub)(nil)

// ckpnConfigStub membaca parameter dari peta; method yang tidak diisi memakai fallback.
// Berbeda dari collateralConfigStub, GetBool/GetInt di sini benar-benar membaca nilai
// karena saklar ckpn.enabled dan ambang aset baik diuji di sini.
type ckpnConfigStub struct {
	domain.SystemConfigService
	values map[string]string
}

func (c *ckpnConfigStub) GetString(_ context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func (c *ckpnConfigStub) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	if v, ok := c.values[key]; ok {
		if parsed, err := decimal.NewFromString(v); err == nil {
			return parsed
		}
	}
	return fallback
}

func (c *ckpnConfigStub) GetInt(_ context.Context, key string, fallback int) int {
	if v, ok := c.values[key]; ok {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func (c *ckpnConfigStub) GetBool(_ context.Context, key string, fallback bool) bool {
	if v, ok := c.values[key]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func (c *ckpnConfigStub) Invalidate(string) {}

var _ domain.SystemConfigService = (*ckpnConfigStub)(nil)

type ckpnLockerStub struct {
	locked []uuid.UUID
	err    error
}

func (s *ckpnLockerStub) LockLoanTx(_ context.Context, _ any, id uuid.UUID) (*domain.Loan, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.locked = append(s.locked, id)
	return &domain.Loan{ID: id}, nil
}

var _ ckpnLoanLocker = (*ckpnLockerStub)(nil)

func newTestCKPNService(repo *ckpnRepoStub, cfg *ckpnConfigStub) (*ckpnService, *stubPosting, *ckpnLockerStub) {
	products := &stubProductRepo{}
	posting := &stubPosting{}
	locker := &ckpnLockerStub{}
	return &ckpnService{
		txRunner:    stubTxRunner{},
		repo:        repo,
		productRepo: products,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      cfg,
		locker:      locker,
	}, posting, locker
}

func ckpnLoan() domain.CKPNLoanSnapshot {
	return domain.CKPNLoanSnapshot{
		LoanID:         uuid.New(),
		LoanNumber:     "KRD-2026-0001",
		Outstanding:    decimal.NewFromInt(10_000_000),
		Collectibility: domain.KolKurangLancar,
		DPD:            100,
		RequiredPPAP:   decimal.NewFromInt(1_000_000),
	}
}

// Parameter PD/LGD harus dibaca dari konfigurasi. Mengubah nilai konfigurasi harus
// mengubah hasil tanpa menyentuh kode.
func TestCKPN_ParameterKebijakanDibacaDariKonfigurasi(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}

	// 10.000.000 x 10% x 50% = 500.000
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
	svc, _, _ := newTestCKPNService(repo, cfg)
	summary, err := svc.Compare(context.Background(), asOf)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("item %d, mau 1 (failure: %+v)", len(summary.Items), summary.Failures)
	}
	if !summary.Items[0].CKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("CKPN %s, mau 500000", summary.Items[0].CKPN)
	}

	// PD dinaikkan menjadi 20% lewat konfigurasi: hasil harus 1.000.000.
	cfg.values["ckpn.pd.3"] = "0.20"
	svc2, _, _ := newTestCKPNService(repo, cfg)
	summary2, err := svc2.Compare(context.Background(), asOf)
	if err != nil {
		t.Fatalf("Compare kedua: %v", err)
	}
	if !summary2.Items[0].CKPN.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("CKPN %s, mau 1000000 setelah konfigurasi berubah", summary2.Items[0].CKPN)
	}
}

// Ambang aset baik juga parameter kebijakan: menurunkannya ke 0 membuat kredit dengan
// tunggakan 3 hari tidak lagi menjadi aset baik, sehingga membutuhkan PD/LGD.
func TestCKPN_AmbangAsetBaikDibacaDariKonfigurasi(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.DPD = 3
	snap.Collectibility = domain.KolLancar

	// Bawaan 7 hari: DPD 3 masih aset baik.
	cfg := &ckpnConfigStub{values: map[string]string{"ckpn.enabled": "true"}}
	svc, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, cfg)
	summary, err := svc.Compare(context.Background(), asOf)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !summary.Items[0].IsAsetBaik || !summary.Items[0].CKPN.IsZero() {
		t.Fatalf("DPD 3 dengan ambang 7 harus aset baik tanpa CKPN, dapat %+v", summary.Items[0])
	}

	// Ambang 0 hari: DPD 3 tidak lagi aset baik; tanpa PD/LGD harus gagal jelas.
	cfg.values["ckpn.aset_baik.max_dpd"] = "0"
	svc2, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, cfg)
	summary2, err := svc2.Compare(context.Background(), asOf)
	if err != nil {
		t.Fatalf("Compare kedua: %v", err)
	}
	if summary2.Failed != 1 || len(summary2.Items) != 0 {
		t.Fatalf("DPD 3 dengan ambang 0 harus gagal parameter, dapat processed=%d failed=%d", summary2.Processed, summary2.Failed)
	}
}

// Saklar mati berarti tidak ada query, tidak ada posting, dan tidak ada state yang
// berubah — perilaku identik dengan sebelum modul CKPN ada.
func TestCKPN_SaklarMatiTidakAdaQueryDanTidakMengubahApaPun(t *testing.T) {
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
	cfg := &ckpnConfigStub{values: map[string]string{}} // ckpn.enabled tidak ada -> false
	svc, posting, locker := newTestCKPNService(repo, cfg)

	summary, err := svc.Run(context.Background(), time.Now().UTC(), domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Enabled {
		t.Fatal("ringkasan harus menandai saklar mati")
	}
	if repo.listCalled {
		t.Fatal("saklar mati tidak boleh membaca kredit")
	}
	if len(summary.Items) != 0 || len(summary.Failures) != 0 {
		t.Fatalf("saklar mati tidak boleh menghasilkan item/failure: %+v", summary)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("saklar mati tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
	if len(repo.updates) != 0 {
		t.Fatalf("saklar mati tidak boleh menulis state, dapat %d", len(repo.updates))
	}
	if len(locker.locked) != 0 {
		t.Fatalf("saklar mati tidak boleh mengunci kredit, dapat %d", len(locker.locked))
	}
}

// Perbandingan harus benar saat PPKA lebih besar, CKPN lebih besar, dan keduanya sama.
func TestCKPN_PerbandinganDenganPPKA(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}
	// Target CKPN = 10.000.000 x 10% x 50% = 500.000.
	cases := []struct {
		name           string
		ppka           int64
		wantLarger     domain.CKPNLarger
		wantDifference int64
		wantDeduction  int64
	}{
		{"PPKA lebih besar", 1_000_000, domain.CKPNLargerPPKA, 500_000, 500_000},
		{"CKPN lebih besar", 200_000, domain.CKPNLargerCKPN, -300_000, 0},
		{"keduanya sama", 500_000, domain.CKPNLargerSame, 0, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := ckpnLoan()
			snap.RequiredPPAP = decimal.NewFromInt(tc.ppka)
			svc, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, cfg)

			summary, err := svc.Compare(context.Background(), asOf)
			if err != nil {
				t.Fatalf("Compare: %v", err)
			}
			item := summary.Items[0]
			if item.Larger != tc.wantLarger {
				t.Fatalf("pihak yang lebih besar %q, mau %q", item.Larger, tc.wantLarger)
			}
			if !item.Difference.Equal(decimal.NewFromInt(tc.wantDifference)) {
				t.Fatalf("selisih %s, mau %d", item.Difference, tc.wantDifference)
			}
			if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(tc.wantDeduction)) {
				t.Fatalf("pengurang modal inti %s, mau %d", summary.ModalIntiDeduction, tc.wantDeduction)
			}
		})
	}
}

// Parameter yang belum diisi harus menghasilkan kegagalan yang jelas, bukan CKPN nol.
func TestCKPN_ParameterBelumDiisiGagalJelas(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	t.Run("PD golongan belum diisi", func(t *testing.T) {
		cfg := &ckpnConfigStub{values: map[string]string{
			"ckpn.enabled": "true",
			"ckpn.lgd":     "0.50", // PD golongan 3 sengaja kosong
		}}
		repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
		svc, posting, _ := newTestCKPNService(repo, cfg)

		summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if summary.Failed != 1 || len(summary.Items) != 0 {
			t.Fatalf("mau 1 gagal tanpa item, dapat failed=%d items=%d", summary.Failed, len(summary.Items))
		}
		msg := summary.Failures[0].Error
		if !strings.Contains(msg, "ckpn.pd.3") || !strings.Contains(msg, domain.ErrCKPNParameterMissing.Error()) {
			t.Fatalf("pesan gagal %q harus menyebut parameter yang belum diisi", msg)
		}
		if len(posting.requests) != 0 || len(repo.updates) != 0 {
			t.Fatal("parameter belum diisi tidak boleh memposting atau menulis state")
		}
	})

	t.Run("LGD belum diisi", func(t *testing.T) {
		cfg := &ckpnConfigStub{values: map[string]string{
			"ckpn.enabled": "true",
			"ckpn.pd.3":    "0.10", // LGD sengaja kosong
		}}
		repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
		svc, _, _ := newTestCKPNService(repo, cfg)

		summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if summary.Failed != 1 {
			t.Fatalf("mau 1 gagal, dapat %d", summary.Failed)
		}
		if !strings.Contains(summary.Failures[0].Error, "ckpn.lgd") {
			t.Fatalf("pesan gagal %q harus menyebut ckpn.lgd", summary.Failures[0].Error)
		}
	})
}

// Aset baik boleh tidak membentuk CKPN; kredit itu tidak boleh dianggap gagal hanya
// karena PD/LGD belum diisi.
func TestCKPN_AsetBaikTidakButuhParameter(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.DPD = 0
	snap.Collectibility = domain.KolLancar
	snap.RequiredCKPN = decimal.NewFromInt(250_000) // CKPN lama harus dipulihkan
	cfg := &ckpnConfigStub{values: map[string]string{"ckpn.enabled": "true"}}

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, _, _ := newTestCKPNService(repo, cfg)
	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || len(summary.Items) != 1 {
		t.Fatalf("aset baik harus berhasil tanpa parameter: failed=%d items=%d", summary.Failed, len(summary.Items))
	}
	if !summary.Items[0].CKPN.IsZero() {
		t.Fatalf("CKPN aset baik %s, mau 0", summary.Items[0].CKPN)
	}
	// Pemulihan CKPN lama harus tersimpan sebagai target nol.
	if len(repo.updates) != 1 || !repo.updates[0].target.IsZero() {
		t.Fatalf("target tersimpan %+v, mau nol", repo.updates)
	}
}

// Kredit yang pernah direstrukturisasi tidak pernah dianggap aset baik (butir
// 12.3.a.1.c), walau sedang tidak menunggak.
func TestCKPN_KreditRestrukturisasiBukanAsetBaik(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.DPD = 0
	snap.Collectibility = domain.KolLancar
	snap.IsRestructured = true
	cfg := &ckpnConfigStub{values: map[string]string{"ckpn.enabled": "true"}} // tanpa PD/LGD

	svc, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, cfg)
	summary, err := svc.Compare(context.Background(), asOf)
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Failed != 1 {
		t.Fatalf("kredit restrukturisasi tanpa parameter harus gagal, dapat failed=%d", summary.Failed)
	}
}

// Run harus memposting selisih (bukan target penuh), mengunci kredit, dan menyimpan
// target. Jurnal memakai COA konfigurasi karena produk belum punya pemetaan CKPN.
func TestCKPN_RunMempostingSelisihDanMenyimpanTarget(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.RequiredCKPN = decimal.NewFromInt(100_000) // target 500.000 -> selisih 400.000
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, locker := newTestCKPNService(repo, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester", BranchCode: "001"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || summary.Processed != 1 {
		t.Fatalf("hasil: processed=%d failed=%d (%+v)", summary.Processed, summary.Failed, summary.Failures)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, mau 1", len(posting.requests))
	}
	req := posting.requests[0]
	if len(req.Lines) != 2 {
		t.Fatalf("baris jurnal %d, mau 2", len(req.Lines))
	}
	if req.Lines[0].AccountNumber != fallbackCKPNExpenseConventional || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("baris debit %+v, mau beban %s", req.Lines[0], fallbackCKPNExpenseConventional)
	}
	if req.Lines[1].AccountNumber != fallbackCKPNReserveConventional || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("baris kredit %+v, mau CKPN %s", req.Lines[1], fallbackCKPNReserveConventional)
	}
	if !req.Lines[0].Amount.Equal(decimal.NewFromInt(400_000)) {
		t.Fatalf("nominal jurnal %s, mau 400000", req.Lines[0].Amount)
	}
	if len(locker.locked) != 1 || locker.locked[0] != snap.LoanID {
		t.Fatalf("kredit terkunci %v, mau %s", locker.locked, snap.LoanID)
	}
	if len(repo.updates) != 1 || !repo.updates[0].target.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("target tersimpan %+v, mau 500000", repo.updates)
	}
}

// Saat CKPN turun (mis. perbaikan kualitas), selisih negatif harus diposting sebagai
// pemulihan: debit CKPN, kredit beban.
func TestCKPN_RunPemulihanMembalikArahJurnal(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.RequiredCKPN = decimal.NewFromInt(900_000) // target 500.000 -> selisih -400.000
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, _ := newTestCKPNService(repo, cfg)

	if _, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, mau 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.Lines[0].AccountNumber != fallbackCKPNReserveConventional || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("pemulihan harus mendebit CKPN, dapat %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != fallbackCKPNExpenseConventional || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("pemulihan harus mengkredit beban, dapat %+v", req.Lines[1])
	}
}
