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
	// listActor menyimpan aktor yang diteruskan ke repo agar filter cabang dapat diuji.
	listActor domain.Actor
}

func (s *ckpnRepoStub) ListActiveLoans(_ context.Context, actor domain.Actor) ([]domain.CKPNLoanSnapshot, error) {
	s.listCalled = true
	s.listActor = actor
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
	// loans memetakan id ke baris kredit segar yang dikembalikan LockLoanTx. Test yang
	// mensimulasikan perubahan antar baca-dan-kunci mengubah isi peta ini.
	loans map[uuid.UUID]*domain.Loan
}

func (s *ckpnLockerStub) LockLoanTx(_ context.Context, _ any, id uuid.UUID) (*domain.Loan, error) {
	if s.err != nil {
		return nil, s.err
	}
	s.locked = append(s.locked, id)
	if l, ok := s.loans[id]; ok {
		return l, nil
	}
	return &domain.Loan{ID: id}, nil
}

var _ ckpnLoanLocker = (*ckpnLockerStub)(nil)

// loanFromSnapshot menyusun baris kredit segar dari snapshot agar stub kunci
// mengembalikan data yang konsisten dengan yang dibaca repo. Status kosong dianggap
// DISBURSED, karena produksi selalu mengisi status dari database.
func loanFromSnapshot(s domain.CKPNLoanSnapshot) *domain.Loan {
	status := s.Status
	if status == "" {
		status = domain.LoanStatusDisbursed
	}
	return &domain.Loan{
		ID:                   s.LoanID,
		LoanNumber:           s.LoanNumber,
		ProductID:            s.ProductID,
		BranchCode:           s.BranchCode,
		Status:               status,
		OutstandingPrincipal: s.Outstanding,
		Collectibility:       s.Collectibility.OJKCode(),
		DPD:                  s.DPD,
		IsRestructured:       s.IsRestructured,
		RequiredPPAP:         s.RequiredPPAP,
		RequiredCKPN:         s.RequiredCKPN,
	}
}

func newTestCKPNService(repo *ckpnRepoStub, cfg *ckpnConfigStub) (*ckpnService, *stubPosting, *ckpnLockerStub) {
	products := &stubProductRepo{}
	posting := &stubPosting{}
	locker := &ckpnLockerStub{loans: map[uuid.UUID]*domain.Loan{}}
	for _, snap := range repo.snapshots {
		locker.loans[snap.LoanID] = loanFromSnapshot(snap)
	}
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
		Status:         domain.LoanStatusDisbursed,
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
	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
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
	summary2, err := svc2.Compare(context.Background(), asOf, domain.Actor{})
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
	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if !summary.Items[0].IsAsetBaik || !summary.Items[0].CKPN.IsZero() {
		t.Fatalf("DPD 3 dengan ambang 7 harus aset baik tanpa CKPN, dapat %+v", summary.Items[0])
	}

	// Ambang 0 hari: DPD 3 tidak lagi aset baik; tanpa PD/LGD harus gagal jelas.
	cfg.values["ckpn.aset_baik.max_dpd"] = "0"
	svc2, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, cfg)
	summary2, err := svc2.Compare(context.Background(), asOf, domain.Actor{})
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

			summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
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
	svc, posting, _ := newTestCKPNService(repo, cfg)
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
	// Jurnal pemulihan WAJIB lahir: target nol saja tidak cukup, karena saldo GL CKPN
	// harus ikut turun. Arah: debit CKPN (10950), kredit beban (50301).
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal pemulihan %d, mau 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.Lines[0].AccountNumber != fallbackCKPNReserveConventional || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("pemulihan aset baik harus mendebit CKPN %s, dapat %+v", fallbackCKPNReserveConventional, req.Lines[0])
	}
	if req.Lines[1].AccountNumber != fallbackCKPNExpenseConventional || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("pemulihan aset baik harus mengkredit beban %s, dapat %+v", fallbackCKPNExpenseConventional, req.Lines[1])
	}
	if !req.Lines[0].Amount.Equal(decimal.NewFromInt(250_000)) {
		t.Fatalf("nominal pemulihan %s, mau 250000", req.Lines[0].Amount)
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
	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
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

// C1: snapshot di luar transaksi bisa basi. Bila kredit dibatalkan setelah daftar
// dibaca tetapi sebelum kunci diperoleh, angka basi TIDAK boleh dipakai: baris segar
// hasil kunci yang berlaku, dan kredit yang keluar dari daftar aktif tidak boleh
// mendapat jurnal pembentukan.
func TestCKPN_SnapshotBasiTidakDipakaiSetelahKunci(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan() // DISBURSED, sisa pokok 10jt, tanpa CKPN tersimpan
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, locker := newTestCKPNService(repo, cfg)

	// Antara baca dan kunci, kredit dibatalkan: sisa pokok dan required_ckpn nol.
	locker.loans[snap.LoanID].Status = domain.LoanStatusCancelled
	locker.loans[snap.LoanID].OutstandingPrincipal = decimal.Zero
	locker.loans[snap.LoanID].RequiredCKPN = decimal.Zero

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("kredit yang sudah dibatalkan tidak boleh dijurnal CKPN, dapat %d", len(posting.requests))
	}
	if len(repo.updates) != 0 {
		t.Fatalf("kredit yang sudah dibatalkan tidak boleh menyimpan target, dapat %d", len(repo.updates))
	}
	if len(summary.Items) != 1 || !summary.Items[0].CKPN.IsZero() {
		t.Fatalf("target kredit batal harus nol, dapat %+v", summary.Items)
	}
}

// Target yang sama dengan yang tersimpan tidak boleh menghasilkan jurnal maupun
// penulisan baris: tidak ada perubahan, jadi tidak ada yang perlu ditulis.
func TestCKPN_TargetSamaTidakAdaPostingDanTidakAdaUpdate(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	// Target = 10.000.000 x 10% x 50% = 500.000, sama dengan yang tersimpan.
	snap.RequiredCKPN = decimal.NewFromInt(500_000)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0.10",
		"ckpn.lgd":     "0.50",
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || summary.Processed != 1 {
		t.Fatalf("hasil: processed=%d failed=%d (%+v)", summary.Processed, summary.Failed, summary.Failures)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("target sama tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
	if len(repo.updates) != 0 {
		t.Fatalf("target sama tidak boleh menulis state, dapat %d", len(repo.updates))
	}
}

// K2: kredit yang tidak lagi aktif tetapi masih menyimpan required_ckpn harus
// melepas cadangannya dengan target nol, tanpa memerlukan PD/LGD.
func TestCKPN_KreditTidakAktifMelepasCadangan(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.Status = domain.LoanStatusPaidOff
	snap.Outstanding = decimal.Zero
	snap.RequiredCKPN = decimal.NewFromInt(500_000)
	// Sengaja tanpa PD/LGD: pelepasan cadangan tidak boleh butuh parameter kebijakan.
	cfg := &ckpnConfigStub{values: map[string]string{"ckpn.enabled": "true"}}

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, _ := newTestCKPNService(repo, cfg)
	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || len(summary.Items) != 1 {
		t.Fatalf("pelepasan cadangan harus berhasil: failed=%d items=%d (%+v)", summary.Failed, len(summary.Items), summary.Failures)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("pelepasan cadangan harus memposting 1 jurnal, dapat %d", len(posting.requests))
	}
	req := posting.requests[0]
	if req.Lines[0].AccountNumber != fallbackCKPNReserveConventional || req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("pelepasan harus mendebit CKPN, dapat %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != fallbackCKPNExpenseConventional || req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("pelepasan harus mengkredit beban, dapat %+v", req.Lines[1])
	}
	if !req.Lines[0].Amount.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("nominal pelepasan %s, mau 500000", req.Lines[0].Amount)
	}
	if len(repo.updates) != 1 || !repo.updates[0].target.IsZero() {
		t.Fatalf("target kredit tidak aktif harus nol, dapat %+v", repo.updates)
	}
}

// C2: PD/LGD yang diisi tetapi salah (negatif, di atas 1, atau bukan angka desimal)
// harus ditolak dengan ErrCKPNParameterInvalid, bukan diperlakukan sebagai belum
// diisi maupun dijatuhkan menjadi nol.
func TestCKPN_ParameterSalahDitolak(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		values  map[string]string
		wantMsg string
	}{
		{"PD negatif", map[string]string{"ckpn.enabled": "true", "ckpn.pd.3": "-0.1", "ckpn.lgd": "0.50"}, "di luar rentang"},
		{"PD di atas 1", map[string]string{"ckpn.enabled": "true", "ckpn.pd.3": "1.5", "ckpn.lgd": "0.50"}, "di luar rentang"},
		{"PD salah format koma", map[string]string{"ckpn.enabled": "true", "ckpn.pd.3": "0,5", "ckpn.lgd": "0.50"}, "bukan angka desimal"},
		{"LGD negatif", map[string]string{"ckpn.enabled": "true", "ckpn.pd.3": "0.10", "ckpn.lgd": "-0.10"}, "di luar rentang"},
		{"LGD format persen", map[string]string{"ckpn.enabled": "true", "ckpn.pd.3": "0.10", "ckpn.lgd": "14.23"}, "di luar rentang"},
		{"LGD huruf", map[string]string{"ckpn.enabled": "true", "ckpn.pd.3": "0.10", "ckpn.lgd": "abc"}, "bukan angka desimal"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
			svc, posting, _ := newTestCKPNService(repo, &ckpnConfigStub{values: tc.values})

			summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if summary.Failed != 1 || len(summary.Items) != 0 {
				t.Fatalf("parameter salah harus gagal 1 tanpa item, dapat failed=%d items=%d", summary.Failed, len(summary.Items))
			}
			msg := summary.Failures[0].Error
			if !strings.Contains(msg, domain.ErrCKPNParameterInvalid.Error()) {
				t.Fatalf("pesan %q harus memakai ErrCKPNParameterInvalid", msg)
			}
			if !strings.Contains(msg, tc.wantMsg) {
				t.Fatalf("pesan %q harus memuat %q", msg, tc.wantMsg)
			}
			if strings.Contains(msg, domain.ErrCKPNParameterMissing.Error()) {
				t.Fatalf("parameter yang salah diisi tidak boleh disebut belum diisi: %q", msg)
			}
			if len(posting.requests) != 0 || len(repo.updates) != 0 {
				t.Fatalf("parameter salah tidak boleh memposting atau menulis state (jurnal=%d update=%d)",
					len(posting.requests), len(repo.updates))
			}
		})
	}
}

// Batas rentang 0..1 tetap sah: nol yang diisi sengaja bukan "belum diisi" dan tidak
// boleh ditolak.
func TestCKPN_ParameterBatasNolSah(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		"ckpn.pd.3":    "0",
		"ckpn.lgd":     "0",
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
	svc, posting, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if summary.Failed != 0 || len(summary.Items) != 1 {
		t.Fatalf("nol yang diisi sengaja harus dihitung, dapat failed=%d items=%d (%+v)", summary.Failed, len(summary.Items), summary.Failures)
	}
	if !summary.Items[0].CKPN.IsZero() {
		t.Fatalf("PD/LGD nol menghasilkan target nol, dapat %s", summary.Items[0].CKPN)
	}
	if len(posting.requests) != 0 || len(repo.updates) != 0 {
		t.Fatal("target nol tanpa cadangan tersisa tidak boleh memposting atau menulis state")
	}
}

// C3: aktor wajib diteruskan ke repo agar filter cabang diterapkan di query.
func TestCKPN_AktorDiteruskanKeRepo(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	actor := domain.Actor{Username: "teller", Role: domain.RoleTeller, BranchCode: "001"}
	repo := &ckpnRepoStub{} // saklar mati tetap tidak membaca; pakai enabled agar terbaca
	cfg := &ckpnConfigStub{values: map[string]string{"ckpn.enabled": "true"}}
	svc, _, _ := newTestCKPNService(repo, cfg)

	if _, err := svc.Compare(context.Background(), asOf, actor); err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if repo.listActor != actor {
		t.Fatalf("aktor diteruskan ke repo %+v, mau %+v", repo.listActor, actor)
	}
}
