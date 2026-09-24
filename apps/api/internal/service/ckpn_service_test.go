package service

import (
	"context"
	"errors"
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

func (s *ckpnRepoStub) UpdateIndividualTarget(_ context.Context, _ any, loanID uuid.UUID, target decimal.Decimal) error {
	// Stub unit: jejak individual tidak dibedakan di sini; uji perilakunya ada pada
	// uji integrasi T4 dengan basis data nyata.
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
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
	cfg.values["ckpn.pd_frac.gol_3"] = "0.20"
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
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

// P1: pengurang modal inti dihitung PER KREDIT, bukan dari selisih agregat. Portofolio
// campuran (satu kredit PPKA>CKPN, satu kredit CKPN>PPKA) harus menghasilkan
// Σ max(PPKA_i - CKPN_i, 0). Angka di bawah sengaja dibuat agar keduanya BERBEDA:
// selisih agregat 0 (SAMA) padahal potensi pengurang modal inti 500.000.
func TestCKPN_PengurangModalIntiPerKreditBukanAgregat(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.shadow_mode.enabled": "true", // murni pelaporan; tak ada jurnal
		"ckpn.parameters.status":   "FINAL",
		"ckpn.floor.ppka_enabled":  "false",
		"ckpn.pd_frac.gol_3":       "0.10",
		"ckpn.lgd_frac":            "0.50",
	}}

	// Kredit A: PPKA 1.000.000 > CKPN 500.000 -> pengurang 500.000.
	a := ckpnLoan()
	a.LoanNumber = "KRD-2026-0001"
	a.RequiredPPAP = decimal.NewFromInt(1_000_000)
	// Kredit B: PPKA 0 < CKPN 500.000 -> TIDAK boleh mengurangi kelebihan A.
	b := ckpnLoan()
	b.LoanID = uuid.New()
	b.LoanNumber = "KRD-2026-0002"
	b.RequiredPPAP = decimal.Zero

	svc, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{a, b}}, cfg)
	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Processed != 2 {
		t.Fatalf("processed=%d, mau 2 (%+v)", summary.Processed, summary.Failures)
	}
	// Σ per kredit = 500.000 + 0 = 500.000.
	if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("pengurang modal inti %s, mau 500000 (Σ max per kredit)", summary.ModalIntiDeduction)
	}
	// Agregat = 1.000.000 - 1.000.000 = 0; inilah bukti kedua angka berbeda.
	if !summary.Difference.IsZero() || summary.Higher != domain.CKPNLargerSame {
		t.Fatalf("selisih agregat %s higher %q, mau 0/SAMA", summary.Difference, summary.Higher)
	}
}

// Parameter yang belum diisi harus menghasilkan kegagalan yang jelas, bukan CKPN nol.
func TestCKPN_ParameterBelumDiisiGagalJelas(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	t.Run("PD golongan belum diisi", func(t *testing.T) {
		cfg := &ckpnConfigStub{values: map[string]string{
			"ckpn.enabled":  "true",
			"ckpn.lgd_frac": "0.50", // PD golongan 3 sengaja kosong
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
		if !strings.Contains(msg, "ckpn.pd_frac.gol_3") || !strings.Contains(msg, domain.ErrCKPNParameterMissing.Error()) {
			t.Fatalf("pesan gagal %q harus menyebut parameter yang belum diisi", msg)
		}
		if len(posting.requests) != 0 || len(repo.updates) != 0 {
			t.Fatal("parameter belum diisi tidak boleh memposting atau menulis state")
		}
	})

	t.Run("LGD belum diisi", func(t *testing.T) {
		cfg := &ckpnConfigStub{values: map[string]string{
			"ckpn.enabled":       "true",
			"ckpn.pd_frac.gol_3": "0.10", // LGD sengaja kosong
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
		if !strings.Contains(summary.Failures[0].Error, "ckpn.lgd_frac") {
			t.Fatalf("pesan gagal %q harus menyebut ckpn.lgd_frac", summary.Failures[0].Error)
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

// Saklar ckpn.aset_baik.bentuk_ckpn: bawaan (false) mempertahankan perilaku
// sekarang — aset baik dikecualikan sehingga CKPN nol; bila dinyalakan, CKPN
// tahap-1 tetap dibentuk dengan EAD x PD x LGD golongan lancar.
func TestCKPN_SaklarAsetBaikBentukCKPN(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.DPD = 0
	snap.Collectibility = domain.KolLancar

	// Bawaan: dikecualikan, CKPN nol walau PD/LGD sama sekali tidak diisi.
	bawaan := &ckpnConfigStub{values: map[string]string{"ckpn.enabled": "true"}}
	svcBawaan, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, bawaan)
	summaryBawaan, err := svcBawaan.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare bawaan: %v", err)
	}
	if !summaryBawaan.Items[0].IsAsetBaik || !summaryBawaan.Items[0].CKPN.IsZero() {
		t.Fatalf("bawaan harus mengecualikan aset baik tanpa CKPN, dapat %+v", summaryBawaan.Items[0])
	}

	// Saklar menyala: aset baik tetap dihitung. 10.000.000 x 2% x 50% = 100.000.
	menyala := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled":               "true",
		"ckpn.parameters.status":     "FINAL",
		"ckpn.floor.ppka_enabled":    "false",
		"ckpn.aset_baik.bentuk_ckpn": "true",
		"ckpn.pd_frac.gol_1":         "0.02",
		"ckpn.lgd_frac":              "0.50",
	}}
	svcMenyala, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, menyala)
	summaryMenyala, err := svcMenyala.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare saklar menyala: %v", err)
	}
	if len(summaryMenyala.Items) != 1 {
		t.Fatalf("item saklar menyala %d, mau 1 (failure: %+v)", len(summaryMenyala.Items), summaryMenyala.Failures)
	}
	if summaryMenyala.Items[0].IsAsetBaik {
		t.Fatal("saklar menyala tidak boleh lagi menandai kredit sebagai aset baik yang dikecualikan")
	}
	if !summaryMenyala.Items[0].CKPN.Equal(decimal.NewFromInt(100_000)) {
		t.Fatalf("CKPN saklar menyala %s, mau 100000", summaryMenyala.Items[0].CKPN)
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
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
		{"PD negatif", map[string]string{"ckpn.enabled": "true", "ckpn.pd_frac.gol_3": "-0.1", "ckpn.lgd_frac": "0.50"}, "di luar rentang"},
		{"PD di atas 1", map[string]string{"ckpn.enabled": "true", "ckpn.pd_frac.gol_3": "1.5", "ckpn.lgd_frac": "0.50"}, "di luar rentang"},
		{"PD salah format koma", map[string]string{"ckpn.enabled": "true", "ckpn.pd_frac.gol_3": "0,5", "ckpn.lgd_frac": "0.50"}, "bukan angka desimal"},
		{"LGD negatif", map[string]string{"ckpn.enabled": "true", "ckpn.pd_frac.gol_3": "0.10", "ckpn.lgd_frac": "-0.10"}, "di luar rentang"},
		{"LGD format persen", map[string]string{"ckpn.enabled": "true", "ckpn.pd_frac.gol_3": "0.10", "ckpn.lgd_frac": "14.23"}, "di luar rentang"},
		{"LGD huruf", map[string]string{"ckpn.enabled": "true", "ckpn.pd_frac.gol_3": "0.10", "ckpn.lgd_frac": "abc"}, "bukan angka desimal"},
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
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0",
		"ckpn.lgd_frac":           "0",
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

// ckpnRunMarkerStub menyimulasikan penanda tanggal bisnis run PPAP terakhir.
type ckpnRunMarkerStub struct {
	last time.Time
	ok   bool
	err  error
}

func (ckpnRunMarkerStub) RecordRun(context.Context, time.Time, uuid.UUID) error { return nil }

func (s ckpnRunMarkerStub) LastRunBusinessDate(context.Context) (time.Time, bool, error) {
	return s.last, s.ok, s.err
}

var _ domain.PPAPRunMarker = ckpnRunMarkerStub{}

// Perbandingan CKPN tidak boleh memakai required_ppap run PPAP tanggal bisnis lain,
// termasuk saat dipanggil manual di luar tutup hari. Tanpa gerbang ini, angka kemarin
// disajikan seolah angka tanggal bisnis berjalan.
func TestCKPN_MenolakPPAPBasi(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled":            "true",
		"ckpn.parameters.status":  "FINAL",
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
	svc, _, _ := newTestCKPNService(repo, cfg)

	// Belum pernah ada run PPAP yang berhasil.
	svc.runMarker = ckpnRunMarkerStub{}
	if _, err := svc.Compare(context.Background(), asOf, domain.Actor{}); !errors.Is(err, domain.ErrCKPNStalePPAP) {
		t.Fatalf("Compare tanpa run PPAP: %v, mau ErrCKPNStalePPAP", err)
	}
	if repo.listCalled {
		t.Fatal("kredit tidak boleh dibaca saat PPAP belum berjalan")
	}

	// Run PPAP terakhir tanggal bisnis lain: ditolak, dan pesan menyebut tanggal itu.
	lastDay := asOf.AddDate(0, 0, -1)
	svc.runMarker = ckpnRunMarkerStub{last: lastDay, ok: true}
	_, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if !errors.Is(err, domain.ErrCKPNStalePPAP) {
		t.Fatalf("Compare PPAP basi: %v, mau ErrCKPNStalePPAP", err)
	}
	if !strings.Contains(err.Error(), lastDay.Format("2006-01-02")) {
		t.Fatalf("pesan %q harus menyebut tanggal bisnis PPAP terakhir %s", err, lastDay.Format("2006-01-02"))
	}
	// Run juga ditolak: summary perbandingannya akan memakai PPKA basi.
	if _, err := svc.Run(context.Background(), asOf, domain.Actor{}); !errors.Is(err, domain.ErrCKPNStalePPAP) {
		t.Fatalf("Run PPAP basi: %v, mau ErrCKPNStalePPAP", err)
	}

	// Tanggal bisnis PPAP sama dengan tanggal perbandingan: gerbang lolos.
	svc.runMarker = ckpnRunMarkerStub{last: asOf, ok: true}
	if _, err := svc.Compare(context.Background(), asOf, domain.Actor{}); err != nil {
		t.Fatalf("Compare dengan PPAP tanggal sama: %v", err)
	}
	if !repo.listCalled {
		t.Fatal("kredit harus dibaca setelah gerbang tanggal terpenuhi")
	}
}

// --- Mode bayangan CKPN ---

// Saklar bayangan menyala sementara ckpn.enabled mati: CKPN dihitung dan dilaporkan
// lengkap dengan asumsi serta peran PPKA sebagai lantai, TETAPI tidak ada jurnal dan
// tidak ada state yang ditulis. Ini inti pemisahan "hitung & tampilkan" dari
// "jurnal & regulasi".
func TestCKPN_ModeBayanganMenghitungTanpaMenjurnal(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		// ckpn.enabled sengaja TIDAK ada -> false; yang menyala hanya mode bayangan.
		"ckpn.shadow_mode.enabled": "true",
		"ckpn.parameters.status":   "FINAL",
		"ckpn.floor.ppka_enabled":  "false",
		"ckpn.pd_frac.gol_1":       "0.005",
		"ckpn.pd_frac.gol_2":       "0.05",
		"ckpn.pd_frac.gol_3":       "0.10",
		"ckpn.pd_frac.gol_4":       "0.30",
		"ckpn.pd_frac.gol_5":       "0.50",
		"ckpn.lgd_frac":            "0.50",
	}}
	snap := ckpnLoan() // PPKA 1.000.000, target CKPN 10.000.000 x 10% x 50% = 500.000
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, locker := newTestCKPNService(repo, cfg)

	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Enabled {
		t.Fatal("ckpn.enabled mati: ringkasan tidak boleh menandai mode resmi")
	}
	if !summary.ShadowMode {
		t.Fatal("ringkasan harus menandai mode bayangan")
	}
	if summary.Processed != 1 || summary.Failed != 0 {
		t.Fatalf("processed=%d failed=%d, mau 1/0 (%+v)", summary.Processed, summary.Failed, summary.Failures)
	}
	if !summary.TotalCKPN.Equal(decimal.NewFromInt(500_000)) || !summary.TotalPPKA.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("total CKPN/PPKA = %s/%s, mau 500000/1000000", summary.TotalCKPN, summary.TotalPPKA)
	}
	// PPKA lebih tinggi: PPKA adalah lantai, selisih positifnya 500.000.
	if summary.Higher != domain.CKPNLargerPPKA {
		t.Fatalf("pihak lebih tinggi %q, mau PPKA", summary.Higher)
	}
	if !summary.Difference.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("selisih %s, mau 500000", summary.Difference)
	}
	if len(summary.Assumptions) == 0 {
		t.Fatal("mode bayangan harus melaporkan asumsi yang dipakai")
	}
	if len(summary.ParameterGaps) != 0 {
		t.Fatalf("parameter lengkap, gap harus kosong, dapat %v", summary.ParameterGaps)
	}

	// Run pada mode bayangan juga tidak boleh memposting/mengubah state: post dipaksa
	// mati selama ckpn.enabled masih false.
	runSummary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run bayangan: %v", err)
	}
	if !runSummary.Preview {
		t.Fatal("Run mode bayangan harus tetap Preview (tanpa posting)")
	}
	if len(posting.requests) != 0 {
		t.Fatalf("mode bayangan tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
	if len(repo.updates) != 0 {
		t.Fatalf("mode bayangan tidak boleh menulis state, dapat %d", len(repo.updates))
	}
	if len(locker.locked) != 0 {
		t.Fatalf("mode bayangan tidak boleh mengunci kredit, dapat %d", len(locker.locked))
	}
}

// Parameter PD/LGD kosong di mode bayangan: CKPN tidak dapat dihitung. Yang dilaporkan
// adalah daftar kunci yang harus diisi (bukan angka karangan), tetap tanpa jurnal dan
// tanpa menggagalkan pemanggil.
func TestCKPN_ModeBayanganParameterKosongDilaporkan(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.shadow_mode.enabled": "true", // ckpn.enabled mati, PD/LGD kosong
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}}
	svc, posting, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("parameter kosong tidak boleh menggagalkan pemanggil: %v", err)
	}
	if summary.Processed != 0 || summary.Failed != 1 {
		t.Fatalf("processed=%d failed=%d, mau 0/1", summary.Processed, summary.Failed)
	}
	joined := strings.Join(summary.ParameterGaps, " ")
	if !strings.Contains(joined, "ckpn.pd_frac.gol_3") || !strings.Contains(joined, "ckpn.lgd_frac") {
		t.Fatalf("gap parameter %v harus menyebut ckpn.pd_frac.gol_3 dan ckpn.lgd_frac", summary.ParameterGaps)
	}
	if len(posting.requests) != 0 || len(repo.updates) != 0 {
		t.Fatal("parameter kosong tidak boleh memposting atau menulis state")
	}
}

// Kedua saklar menyala: mode resmi (ckpn.enabled) yang berlaku dan penjurnalan tetap
// seperti perilaku lama; mode bayangan tidak menghalangi posting.
func TestCKPN_KeduaSaklarMenyalaModeResmiBerlaku(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled":             "true",
		"ckpn.shadow_mode.enabled": "true",
		"ckpn.parameters.status":   "FINAL",
		"ckpn.floor.ppka_enabled":  "false",
		"ckpn.pd_frac.gol_3":       "0.10",
		"ckpn.lgd_frac":            "0.50",
	}}
	snap := ckpnLoan()
	snap.RequiredCKPN = decimal.NewFromInt(100_000) // selisih 400.000 -> 1 jurnal
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, posting, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Run(context.Background(), asOf, domain.Actor{Username: "tester"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !summary.Enabled {
		t.Fatal("ckpn.enabled menyala: jalur resmi yang berlaku")
	}
	// K3: bayangan diabaikan, jadi ShadowMode yang dilaporkan TIDAK boleh true; kalau
	// true, respons HTTP/EOD dapat dibaca seolah bayangan yang dipakai.
	if summary.ShadowMode {
		t.Fatal("kedua saklar menyala: bayangan diabaikan, ShadowMode yang dilaporkan harus false")
	}
	if len(summary.Assumptions) != 0 || len(summary.ParameterGaps) != 0 {
		t.Fatalf("jalur resmi tidak boleh membawa asumsi/gap bayangan: asumsi=%v gap=%v", summary.Assumptions, summary.ParameterGaps)
	}
	if summary.Preview {
		t.Fatal("jalur resmi harus benar-benar memposting (Preview=false)")
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jalur resmi harus memposting 1 jurnal, dapat %d", len(posting.requests))
	}
	if len(repo.updates) != 1 || !repo.updates[0].target.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("jalur resmi harus menyimpan target 500000, dapat %+v", repo.updates)
	}
}

// Aktor tetap wajib diteruskan ke repo pada mode bayangan agar filter cabang diterapkan
// di query; mode bayangan tidak boleh membuka kebocoran lintas cabang.
func TestCKPN_ModeBayanganMeneruskanAktorUntukFilterCabang(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	actor := domain.Actor{Username: "teller", Role: domain.RoleTeller, BranchCode: "001"}
	repo := &ckpnRepoStub{}
	cfg := &ckpnConfigStub{values: map[string]string{"ckpn.shadow_mode.enabled": "true"}}
	svc, _, _ := newTestCKPNService(repo, cfg)

	if _, err := svc.Compare(context.Background(), asOf, actor); err != nil {
		t.Fatalf("Compare mode bayangan: %v", err)
	}
	if repo.listActor != actor {
		t.Fatalf("aktor diteruskan ke repo %+v, mau %+v", repo.listActor, actor)
	}
}

// Parameter yang diisi dalam PERSEN (mis. ckpn.lgd_frac = 45) harus ditolak sebagai nilai
// tidak sah dan dilaporkan sebagai kekurangan parameter dengan satuan yang benar
// (fraksi 0..1), bukan diam-diam dijatuhkan menjadi nol.
func TestCKPN_ModeBayanganParameterPersenDilaporkanSatuanFraksi(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.shadow_mode.enabled": "true", // ckpn.enabled mati
		"ckpn.pd_frac.gol_1":       "1",    // 1% dimaksud bank, dibaca 100% oleh mesin fraksi
		"ckpn.pd_frac.gol_3":       "0.10",
		"ckpn.lgd_frac":            "45", // persen, di luar fraksi 0..1
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{ckpnLoan()}} // golongan 3
	svc, posting, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Failed != 1 || summary.Processed != 0 {
		t.Fatalf("LGD persen harus menolak perhitungan: processed=%d failed=%d", summary.Processed, summary.Failed)
	}
	joined := strings.Join(summary.ParameterGaps, " ")
	if !strings.Contains(joined, "ckpn.lgd_frac") || !strings.Contains(joined, "FRAKSI") {
		t.Fatalf("gap harus menyebut ckpn.lgd_frac dan satuan FRAKSI, dapat %v", summary.ParameterGaps)
	}
	if len(posting.requests) != 0 || len(repo.updates) != 0 {
		t.Fatal("mode bayangan tidak boleh memposting atau menulis state")
	}
}

// Aset baik menghasilkan target nol tanpa memakai PD/LGD. Jumlah dan sisa pokoknya harus
// dilaporkan supaya TotalCKPN nol dapat dibaca sebagai hasil perhitungan, bukan model
// yang belum dijalankan.
func TestCKPN_ModeBayanganMenghitungAsetBaik(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	snap := ckpnLoan()
	snap.DPD = 0
	snap.Collectibility = domain.KolLancar
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.shadow_mode.enabled": "true",
		"ckpn.pd_frac.gol_1":       "0.005",
		"ckpn.lgd_frac":            "0.45",
	}}
	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, _, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Processed != 1 || summary.Failed != 0 {
		t.Fatalf("aset baik harus terhitung: processed=%d failed=%d", summary.Processed, summary.Failed)
	}
	if !summary.TotalCKPN.IsZero() {
		t.Fatalf("CKPN aset baik %s, mau 0", summary.TotalCKPN)
	}
	if summary.AsetBaikCount != 1 {
		t.Fatalf("aset baik terhitung %d, mau 1", summary.AsetBaikCount)
	}
	if !summary.AsetBaikOutstanding.Equal(snap.Outstanding) {
		t.Fatalf("sisa pokok aset baik %s, mau %s", summary.AsetBaikOutstanding, snap.Outstanding)
	}
}

// Dua basis perbandingan CKPN: basis pertama "sesuai kebijakan" mengecualikan aset
// baik (CKPN nol), basis kedua "setara PPKA" tetap menilai aset baik EAD x PD x LGD
// supaya sebanding dengan PPKA. Portofolio campuran (satu kredit tidak lancar, satu
// aset baik) membuktikan kedua basis menghasilkan angka BERBEDA dan keduanya benar
// manual, tanpa jurnal dan tanpa menulis required_ckpn.
func TestCKPN_DuaBasisSetaraPPKAPortofolioCampuran(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.shadow_mode.enabled": "true", // ckpn.enabled mati: murni pelaporan
		"ckpn.parameters.status":   "FINAL",
		"ckpn.floor.ppka_enabled":  "false",
		"ckpn.pd_frac.gol_1":       "0.005",
		"ckpn.pd_frac.gol_3":       "0.10",
		"ckpn.lgd_frac":            "0.50",
	}}

	// A: tidak lancar (bukan aset baik). 10.000.000 x 10% x 50% = 500.000; PPKA 1.000.000.
	loanA := ckpnLoan()
	loanA.Outstanding = decimal.NewFromInt(10_000_000)
	loanA.RequiredPPAP = decimal.NewFromInt(1_000_000)
	// C: aset baik. dasar CKPN basis-1 nol; basis-2 = 5.000.000 x 0,5% x 50% = 12.500.
	loanC := ckpnLoan()
	loanC.LoanNumber = "KRD-2026-0002"
	loanC.Collectibility = domain.KolLancar
	loanC.DPD = 0
	loanC.Outstanding = decimal.NewFromInt(5_000_000)
	loanC.RequiredPPAP = decimal.NewFromInt(200_000)

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{loanA, loanC}}
	svc, posting, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Processed != 2 || summary.Failed != 0 {
		t.Fatalf("processed=%d failed=%d, mau 2/0 (%+v)", summary.Processed, summary.Failed, summary.Failures)
	}

	// Basis 1 — sesuai kebijakan (aset baik dikecualikan).
	if !summary.TotalCKPN.Equal(decimal.NewFromInt(500_000)) {
		t.Fatalf("basis kebijakan total CKPN %s, mau 500000", summary.TotalCKPN)
	}
	if !summary.TotalPPKA.Equal(decimal.NewFromInt(1_200_000)) {
		t.Fatalf("basis kebijakan total PPKA %s, mau 1200000", summary.TotalPPKA)
	}
	if !summary.ModalIntiDeduction.Equal(decimal.NewFromInt(700_000)) {
		t.Fatalf("basis kebijakan pengurang modal inti %s, mau 700000", summary.ModalIntiDeduction)
	}
	if summary.AsetBaikCount != 1 || !summary.AsetBaikOutstanding.Equal(decimal.NewFromInt(5_000_000)) {
		t.Fatalf("aset baik %d/%s, mau 1/5000000", summary.AsetBaikCount, summary.AsetBaikOutstanding)
	}

	// Basis 2 — setara PPKA (aset baik tetap dinilai). 500.000 + 12.500 = 512.500.
	if summary.SetaraPPKAProcessed != 2 || summary.SetaraPPKAFailed != 0 {
		t.Fatalf("basis setara processed=%d failed=%d, mau 2/0", summary.SetaraPPKAProcessed, summary.SetaraPPKAFailed)
	}
	if !summary.SetaraPPKATotalPPKA.Equal(decimal.NewFromInt(1_200_000)) {
		t.Fatalf("basis setara total PPKA %s, mau 1200000 (sama dengan basis kebijakan)", summary.SetaraPPKATotalPPKA)
	}
	if !summary.SetaraPPKATotalCKPN.Equal(decimal.NewFromInt(512_500)) {
		t.Fatalf("basis setara total CKPN %s, mau 512500", summary.SetaraPPKATotalCKPN)
	}
	if !summary.SetaraPPKADifference.Equal(decimal.NewFromInt(687_500)) || summary.SetaraPPKAHigher != domain.CKPNLargerPPKA {
		t.Fatalf("basis setara difference=%s higher=%s, mau 687500/PPKA", summary.SetaraPPKADifference, summary.SetaraPPKAHigher)
	}
	if !summary.SetaraPPKAModalIntiDeduction.Equal(decimal.NewFromInt(687_500)) {
		t.Fatalf("basis setara pengurang modal inti %s, mau 687500", summary.SetaraPPKAModalIntiDeduction)
	}
	// Kedua basis memang BERBEDA, inti laporan A1.
	if summary.SetaraPPKATotalCKPN.Equal(summary.TotalCKPN) {
		t.Fatalf("dua basis harus berbeda, keduanya %s", summary.TotalCKPN)
	}
	if !strings.Contains(summary.BasisNote, "DUA BASIS") || !strings.Contains(summary.BasisNote, "BUKAN kebijakan bank") {
		t.Fatalf("basis note harus menjelaskan dua basis dan statusnya, dapat %q", summary.BasisNote)
	}

	// Murni pelaporan: tidak ada jurnal, tidak ada state, tidak ada kunci.
	if len(posting.requests) != 0 {
		t.Fatalf("dua basis tidak boleh menjurnal, dapat %d", len(posting.requests))
	}
	if len(repo.updates) != 0 {
		t.Fatalf("dua basis tidak boleh menulis required_ckpn, dapat %d", len(repo.updates))
	}
}

// Parameter aset baik yang belum diisi membuat basis setara PPKA tidak dapat dihitung
// pada kredit aset baik, sementara basis kebijakan tetap berhasil (tidak butuh PD).
// Kegagalan itu harus dilaporkan terpisah, tidak menambah Failed basis pertama.
func TestCKPN_DuaBasisSetaraPPKAParameterAsetBaikKosong(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.shadow_mode.enabled": "true",
		"ckpn.pd_frac.gol_3":       "0.10",
		"ckpn.lgd_frac":            "0.50",
		// ckpn.pd_frac.gol_1 sengaja kosong: hanya dibutuhkan basis setara untuk aset baik.
	}}
	snap := ckpnLoan()
	snap.Collectibility = domain.KolLancar
	snap.DPD = 0

	repo := &ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}
	svc, _, _ := newTestCKPNService(repo, cfg)

	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if summary.Processed != 1 || summary.Failed != 0 {
		t.Fatalf("basis kebijakan processed=%d failed=%d, mau 1/0", summary.Processed, summary.Failed)
	}
	if summary.SetaraPPKAProcessed != 0 || summary.SetaraPPKAFailed != 1 {
		t.Fatalf("basis setara processed=%d failed=%d, mau 0/1", summary.SetaraPPKAProcessed, summary.SetaraPPKAFailed)
	}
	if !summary.SetaraPPKATotalCKPN.IsZero() {
		t.Fatalf("basis setara total CKPN %s, mau nol saat tidak dapat dihitung", summary.SetaraPPKATotalCKPN)
	}
	if !strings.Contains(summary.BasisNote, "belum dapat menghitung") {
		t.Fatalf("basis note harus menyebut kredit yang belum dapat dihitung, dapat %q", summary.BasisNote)
	}
}

// --- Pengaman parameter SEMENTARA (keputusan panel butir 1) ---

// Status parameter bawaan (tanpa kunci) harus SEMENTARA: angka produksi belum
// diratifikasi dan lantai PPKA ditegakkan. Ini gagal-aman — salah membaca SEMENTARA
// sebagai FINAL berarti angka sementara dikirim ke OJK.
//
// Blokir ekspor OJK bergantung pada ckpn.enabled: selama CKPN resmi masih mati, laporan
// OJK memuat angka PPKA (bukan CKPN dari PD/LGD sementara), jadi memblokirnya hanya
// menutup laporan yang sehat. Begitu CKPN menyala tanpa ratifikasi, ekspor wajib ditutup.
func TestCKPNParametersStatusBawaanSementaraDanBlokirOJK(t *testing.T) {
	now := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	// CKPN resmi masih mati: SEMENTARA tetap diperingatkan, tetapi ekspor belum ditutup.
	mati := domain.CKPNParametersStatusFromConfig(context.Background(), &ckpnConfigStub{values: map[string]string{}}, now)
	if mati.Status != domain.CKPNParameterStatusSementara || !mati.Sementara {
		t.Fatalf("status bawaan %q sementara=%v, mau SEMENTARA/true", mati.Status, mati.Sementara)
	}
	if !mati.FloorPPKAEnforced {
		t.Fatal("selama SEMENTARA lantai PPKA wajib ditegakkan")
	}
	if mati.OJKExportBlocked {
		t.Fatal("CKPN resmi masih mati: laporan OJK memuat PPKA, tidak boleh diblokir")
	}
	if len(mati.Warnings) == 0 {
		t.Fatal("peringatan SEMENTARA wajib ada walau ekspor belum diblokir")
	}
	joined := strings.Join(mati.Warnings, " ")
	if !strings.Contains(joined, "SEMENTARA") || !strings.Contains(joined, "OJK") {
		t.Fatalf("peringatan harus menyebut SEMENTARA dan larangan OJK: %v", mati.Warnings)
	}

	// CKPN resmi menyala tanpa ratifikasi: angka sementara mengalir ke laporan -> tutup.
	nyala := domain.CKPNParametersStatusFromConfig(context.Background(), &ckpnConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNEnabled: "true",
	}}, now)
	if !nyala.Sementara || !nyala.OJKExportBlocked || nyala.OJKExportBlockReason == "" {
		t.Fatalf("CKPN menyala + SEMENTARA harus memblokir ekspor beralasan: %+v", nyala)
	}
}

// Setelah FINAL: tidak ada peringatan, ekspor boleh, dan lantai menjadi pilihan bank.
func TestCKPNParametersStatusFinalTidakMemblokir(t *testing.T) {
	now := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus: "FINAL",
		domain.ConfigKeyCKPNFloorPPKA:        "false",
	}}
	st := domain.CKPNParametersStatusFromConfig(context.Background(), cfg, now)
	if st.Sementara || st.Status != domain.CKPNParameterStatusFinal {
		t.Fatalf("status %q sementara=%v, mau FINAL/false", st.Status, st.Sementara)
	}
	if st.OJKExportBlocked {
		t.Fatal("parameter FINAL tidak boleh memblokir ekspor OJK")
	}
	if st.FloorPPKAEnforced {
		t.Fatal("setelah FINAL lantai boleh dimatikan bank lewat kunci")
	}
	if len(st.Warnings) != 0 {
		t.Fatalf("parameter FINAL tidak boleh memberi peringatan: %v", st.Warnings)
	}
}

// Umur parameter melewati batas 12 bulan wajib memunculkan peringatan tingkat tinggi.
func TestCKPNParametersStatusLewatBatasTingkatTinggi(t *testing.T) {
	now := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	since := now.AddDate(0, -13, 0).Format("2006-01-02")
	cfg := &ckpnConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus: domain.CKPNParameterStatusSementara,
		domain.ConfigKeyCKPNParametersSince:  since,
		domain.ConfigKeyCKPNParametersMonths: "12",
	}}
	st := domain.CKPNParametersStatusFromConfig(context.Background(), cfg, now)
	if !st.DeadlinePassed || st.Deadline == "" {
		t.Fatalf("lewat 13 bulan harus DeadlinePassed; deadline=%q passed=%v", st.Deadline, st.DeadlinePassed)
	}
	joined := strings.Join(st.Warnings, " ")
	if !strings.Contains(joined, "TINGKAT TINGGI") {
		t.Fatalf("peringatan lewat batas harus tingkat tinggi: %v", st.Warnings)
	}
}

// Nilai status asing (mis. RATIFIED alih-alih FINAL) TIDAK boleh dianggap final:
// diperlakukan SEMENTARA dan dilaporkan, bukan ditebak sah. Blokir ekspor tetap
// mengikuti aturan yang sama seperti SEMENTARA lain: berlaku saat CKPN resmi menyala,
// karena di situlah angka sementara benar-benar masuk laporan OJK.
func TestCKPNParametersStatusNilaiAsingGagalAman(t *testing.T) {
	now := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)

	// Tanpa ckpn.enabled: SEMENTARA + dilaporkan, ekspor belum ditutup.
	cfg := &ckpnConfigStub{values: map[string]string{domain.ConfigKeyCKPNParametersStatus: "RATIFIED"}}
	st := domain.CKPNParametersStatusFromConfig(context.Background(), cfg, now)
	if !st.Sementara {
		t.Fatalf("status asing harus diperlakukan SEMENTARA: %+v", st)
	}
	if st.OJKExportBlocked {
		t.Fatalf("CKPN mati: status asing belum menutup ekspor: %+v", st)
	}
	if !strings.Contains(strings.Join(st.Warnings, " "), "tidak dikenal") {
		t.Fatalf("nilai asing harus dilaporkan: %v", st.Warnings)
	}

	// Dengan ckpn.enabled: status asing menutup ekspor (gagal-aman).
	cfg = &ckpnConfigStub{values: map[string]string{
		domain.ConfigKeyCKPNParametersStatus: "RATIFIED",
		domain.ConfigKeyCKPNEnabled:          "true",
	}}
	st = domain.CKPNParametersStatusFromConfig(context.Background(), cfg, now)
	if !st.Sementara || !st.OJKExportBlocked || st.OJKExportBlockReason == "" {
		t.Fatalf("CKPN menyala + status asing harus SEMENTARA dan menutup ekspor: %+v", st)
	}
}

// Lantai wajib butir 1.3: target = max(EAD x PD x LGD, required_ppap). Selama
// SEMENTARA bank TIDAK boleh mematikannya — kunci floor=false diabaikan.
func TestCKPN_LantaiPPKAWajibSelamaSementara(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	cfg := &ckpnConfigStub{values: map[string]string{
		"ckpn.enabled": "true",
		// Bank mencoba mematikan lantai, tetapi status masih SEMENTARA.
		"ckpn.floor.ppka_enabled": "false",
		"ckpn.parameters.status":  "SEMENTARA",
		"ckpn.pd_frac.gol_3":      "0.10",
		"ckpn.lgd_frac":           "0.50",
	}}
	// Model 10.000.000 x 10% x 50% = 500.000, PPKA 1.000.000 -> lantai menang.
	snap := ckpnLoan()
	svc, posting, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, cfg)
	summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if len(summary.Items) != 1 {
		t.Fatalf("item %d, mau 1 (%+v)", len(summary.Items), summary.Failures)
	}
	item := summary.Items[0]
	if !item.CKPN.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("target %s, mau 1000000 (lantai PPKA)", item.CKPN)
	}
	if item.Larger != domain.CKPNLargerSame || !summary.ModalIntiDeduction.IsZero() {
		t.Fatalf("CKPN = PPKA -> tidak ada pengurang modal: larger=%s deduction=%s", item.Larger, summary.ModalIntiDeduction)
	}
	if !summary.ParameterSementara || !summary.OJKExportBlocked || summary.ParameterNote == "" {
		t.Fatalf("ringkasan harus membawa label SEMENTARA dan blokir OJK: %+v", summary)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("Compare tidak boleh menjurnal, dapat %d", len(posting.requests))
	}
}

// Setelah FINAL, lantai menjadi pilihan bank: mati -> model apa adanya; nyala -> max.
func TestCKPN_LantaiPPKASetelahFinalMenjadiPilihan(t *testing.T) {
	asOf := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	base := map[string]string{
		"ckpn.enabled":           "true",
		"ckpn.parameters.status": "FINAL",
		"ckpn.pd_frac.gol_3":     "0.10",
		"ckpn.lgd_frac":          "0.50",
	}
	want := func(floorKey, wantTarget string) {
		t.Helper()
		values := map[string]string{}
		for k, v := range base {
			values[k] = v
		}
		values["ckpn.floor.ppka_enabled"] = floorKey
		snap := ckpnLoan()
		svc, _, _ := newTestCKPNService(&ckpnRepoStub{snapshots: []domain.CKPNLoanSnapshot{snap}}, &ckpnConfigStub{values: values})
		summary, err := svc.Compare(context.Background(), asOf, domain.Actor{})
		if err != nil {
			t.Fatalf("Compare floor=%s: %v", floorKey, err)
		}
		if got := summary.Items[0].CKPN.String(); got != wantTarget {
			t.Fatalf("floor=%s target %s, mau %s", floorKey, got, wantTarget)
		}
	}
	want("false", "500000") // lantai mati: model apa adanya
	want("true", "1000000") // lantai nyala: PPKA menang
}

// Peringatan status dipakai ulang oleh start/EOD sehingga harus kosong saat FINAL.
func TestCKPNProvisionalWarningsHanyaSaatSementara(t *testing.T) {
	sementara := &ckpnConfigStub{values: map[string]string{}}
	if got := CKPNProvisionalWarnings(context.Background(), sementara); len(got) == 0 {
		t.Fatal("status SEMENTARA harus menghasilkan peringatan")
	}
	sementara.values[domain.ConfigKeyCKPNParametersStatus] = "FINAL"
	if got := CKPNProvisionalWarnings(context.Background(), sementara); len(got) != 0 {
		t.Fatalf("status FINAL tidak boleh menghasilkan peringatan: %v", got)
	}
}

// Ringkasan tutup hari (EOD) wajib membawa peringatan status parameter SEMENTARA
// (butir 1.4.4). Dipisah dari runEOD agar dapat diuji tanpa menjalankan EOD penuh.
func TestEODRingkasanMembawaPeringatanParameterSementara(t *testing.T) {
	ctx := context.Background()
	sementara := &ckpnConfigStub{values: map[string]string{}}
	summary := &domain.EODSummaryResult{}
	appendCKPNProvisionalWarnings(ctx, sementara, summary)
	if len(summary.Warnings) == 0 {
		t.Fatal("ringkasan EOD harus membawa peringatan SEMENTARA")
	}

	final := &ckpnConfigStub{values: map[string]string{domain.ConfigKeyCKPNParametersStatus: "FINAL"}}
	summaryFinal := &domain.EODSummaryResult{}
	appendCKPNProvisionalWarnings(ctx, final, summaryFinal)
	if len(summaryFinal.Warnings) != 0 {
		t.Fatalf("parameter FINAL tidak boleh menambah peringatan EOD: %v", summaryFinal.Warnings)
	}
}
