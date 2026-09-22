package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// lossLoanRepoStub hanya mengimplementasikan method yang dipanggil jalur kerugian
// restrukturisasi; sisanya memakai interface kosong agar tidak perlu menulis seluruh
// kontrak domain.LoanRepository di sini.
type lossLoanRepoStub struct {
	domain.LoanRepository
	// loan adalah snapshot yang dikembalikan GetByID.
	loan *domain.Loan
	// locked adalah baris yang dikembalikan LockLoanTx. Sengaja terpisah dari loan
	// agar uji dapat membuktikan perhitungan memakai baris hasil kunci, bukan
	// snapshot. Bila nil, salinan loan yang dipakai.
	locked    *domain.Loan
	schedules []domain.LoanSchedule

	plainRestructureCalled bool
	txRestructureCalled    bool
	eirStored              bool
	storedEIR              decimal.Decimal
	storedEIRMethod        string
}

func (s *lossLoanRepoStub) GetByID(context.Context, uuid.UUID) (*domain.Loan, error) {
	return s.loan, nil
}

func (s *lossLoanRepoStub) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	src := s.locked
	if src == nil {
		src = s.loan
	}
	if src == nil {
		return nil, domain.ErrLoanNotFound
	}
	// Salinan baru: pointer hasil kunci tidak boleh sama dengan objek snapshot,
	// supaya uji benar-benar membuktikan nilai mana yang dipakai.
	cp := *src
	return &cp, nil
}

func (s *lossLoanRepoStub) GetSchedulesTx(context.Context, any, uuid.UUID) ([]domain.LoanSchedule, error) {
	return s.schedules, nil
}

func (s *lossLoanRepoStub) UpdateRestructure(_ context.Context, l *domain.Loan, schedules []domain.LoanSchedule) error {
	s.plainRestructureCalled = true
	s.loan = l
	s.locked = l
	s.schedules = schedules
	return nil
}

func (s *lossLoanRepoStub) UpdateRestructureTx(_ context.Context, _ any, l *domain.Loan, schedules []domain.LoanSchedule) error {
	s.txRestructureCalled = true
	s.loan = l
	s.locked = l
	s.schedules = schedules
	return nil
}

func (s *lossLoanRepoStub) UpdateOriginalEIRTx(_ context.Context, _ any, _ uuid.UUID, monthly decimal.Decimal, method string, _ []byte, _ time.Time) error {
	s.eirStored = true
	s.storedEIR = monthly
	s.storedEIRMethod = method
	return nil
}

var _ domain.LoanRepository = (*lossLoanRepoStub)(nil)
var _ domain.RestructureTxWriter = (*lossLoanRepoStub)(nil)
var _ domain.LoanEIRWriter = (*lossLoanRepoStub)(nil)

// lossActor adalah aktor lintas cabang: cabang aktor (001) sengaja berbeda dari cabang
// kredit (002) agar atribusi cabang jurnal dapat diuji.
func lossActor() domain.Actor {
	return domain.Actor{Username: "uji", Role: domain.RoleSuperAdmin, BranchCode: "001"}
}

func newLossService(repo *lossLoanRepoStub, products *stubProductRepo, posting *stubPosting, cfg *ckpnConfigStub) *loanService {
	return &loanService{
		loanRepo:    repo,
		productRepo: products,
		resolver:    stubResolver{},
		poster:      NewProductPoster(products, stubResolver{}, posting),
		posting:     posting,
		config:      cfg,
		txRunner:    stubTxRunner{},
	}
}

func lossLoan() (*domain.Loan, *domain.BankingProduct, uuid.UUID) {
	productID := uuid.New()
	loan := &domain.Loan{
		ID:                   uuid.New(),
		LoanNumber:           "KRD-RESTRUCT-1",
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		BranchCode:           "002",
		Collectibility:       domain.CollectibilityKol1,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
		OriginalEIRMonthly:   decimal.NewFromFloat(0.01),
	}
	product := &domain.BankingProduct{
		ID:             productID,
		Code:           "KRD-FLAT",
		ProfitScheme:   domain.SchemeInterest,
		ScheduleMethod: domain.ScheduleFlat,
		RateAnnual:     decimal.NewFromInt(12),
	}
	return loan, product, productID
}

// Saklar mati berarti jalur lama dipakai persis: tidak ada jurnal, tidak ada penulisan
// transaksional, dan tidak ada EIR yang dihitung.
func TestRestructureLoss_SaklarMatiTidakMengubahApaPun(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{}} // loan.restructure.loss.enabled tidak ada -> false
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if !repo.plainRestructureCalled {
		t.Fatal("saklar mati harus memakai jalur UpdateRestructure lama")
	}
	if repo.txRestructureCalled || repo.eirStored {
		t.Fatal("saklar mati tidak boleh menulis transaksional atau menyimpan EIR")
	}
	if len(posting.requests) != 0 {
		t.Fatalf("saklar mati tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
}

// Parameter diskonto yang DIISI tetapi di luar rentang / salah format ditolak dengan
// pesan yang menyebut kuncinya, bukan dijatuhkan menjadi nol.
func TestRestructureLoss_ParameterSalahDitolak(t *testing.T) {
	cases := map[string]string{
		"nol":         "0",
		"lebih 100":   "100.5",
		"bukan angka": "dua belas",
	}
	for name, value := range cases {
		t.Run(name, func(t *testing.T) {
			loan, product, _ := lossLoan()
			repo := &lossLoanRepoStub{loan: loan}
			cfg := &ckpnConfigStub{values: map[string]string{
				"loan.restructure.loss.enabled":                  "true",
				"loan.restructure.loss.discount_rate_annual_pct": value,
			}}
			svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

			_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
				LoanID:        loan.ID,
				NewTermMonths: 12,
			}, lossActor())
			if !errors.Is(err, domain.ErrRestructureLossParamInvalid) {
				t.Fatalf("mau ErrRestructureLossParamInvalid, dapat %v", err)
			}
			if !strings.Contains(err.Error(), "loan.restructure.loss.discount_rate_annual_pct") {
				t.Fatalf("pesan harus menyebut kunci konfigurasi, dapat %q", err.Error())
			}
		})
	}
}

// Kredit yang belum menyimpan EIR orisinal ditolak, bukan dihitung dengan suku bunga
// kontraktual.
func TestRestructureLoss_EIRHilangDitolak(t *testing.T) {
	loan, product, _ := lossLoan()
	loan.OriginalEIRMonthly = decimal.Zero
	repo := &lossLoanRepoStub{loan: loan}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

	_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:        loan.ID,
		NewTermMonths: 12,
	}, lossActor())
	if !errors.Is(err, domain.ErrEIRMissing) {
		t.Fatalf("mau ErrEIRMissing, dapat %v", err)
	}
}

// Pesan penolakan EIR harus TEPAT membedakan dua sebab: kredit lama yang memang belum
// pernah dihitung, dan kredit yang perhitungan EIR-nya sudah dicoba tetapi gagal
// konvergen (alasan tersimpan pada basis audit). Keduanya menunjuk jalan keluar.
func TestRestructureLoss_PesanEIRHilangTepat(t *testing.T) {
	reason := domain.ErrIRRNotConverged.Error() + ": arus kas tidak berubah tanda"
	unavailableBasis, err := json.Marshal(domain.EIRBasis{
		Method: domain.RestructureLossEIRUnavailableMethod,
		Reason: reason,
	})
	if err != nil {
		t.Fatalf("menyusun basis uji: %v", err)
	}

	cases := []struct {
		name    string
		setup   func(*domain.Loan)
		substrs []string
	}{
		{
			name: "kredit lama tanpa EIR",
			setup: func(l *domain.Loan) {
				l.OriginalEIRMonthly = decimal.Zero
				l.OriginalEIRMethod = ""
				l.OriginalEIRBasis = ""
			},
			substrs: []string{"EIR tidak tersedia", "belum menyimpan EIR", cfgRestructureLossDiscountRate},
		},
		{
			name: "perhitungan EIR gagal konvergen",
			setup: func(l *domain.Loan) {
				l.OriginalEIRMonthly = decimal.Zero
				l.OriginalEIRMethod = domain.RestructureLossEIRUnavailableMethod
				l.OriginalEIRBasis = string(unavailableBasis)
			},
			substrs: []string{"EIR tidak tersedia", "gagal", "tidak berubah tanda", cfgRestructureLossDiscountRate},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loan, product, _ := lossLoan()
			tc.setup(loan)
			repo := &lossLoanRepoStub{loan: loan}
			cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
			svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

			_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
				LoanID:        loan.ID,
				NewTermMonths: 12,
			}, lossActor())
			if !errors.Is(err, domain.ErrEIRMissing) {
				t.Fatalf("mau ErrEIRMissing, dapat %v", err)
			}
			for _, sub := range tc.substrs {
				if !strings.Contains(err.Error(), sub) {
					t.Fatalf("pesan %q tidak memuat %q", err.Error(), sub)
				}
			}
		})
	}
}

// Jalur aktif: kerugian dihitung dari EIR orisinal, dijurnal Db 50401 / Kr 10301 dengan
// cabang KREDIT, dan saldo kerugian disimpan pada baris kredit.
func TestRestructureLoss_MempostingJurnalDanMenyimpanSaldo(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	// Turunkan suku bunga kontraktual: nilai kini arus kas baru < nilai tercatat.
	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if !repo.txRestructureCalled {
		t.Fatal("jalur aktif harus menyimpan lewat transaksi pemanggil")
	}
	// Cabang jurnal = cabang kredit (002), bukan cabang aktor (001).
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(posting.requests))
	}
	req := posting.requests[0]
	if req.BranchCode != "002" {
		t.Fatalf("cabang jurnal %q, mau cabang kredit 002", req.BranchCode)
	}
	if len(req.Lines) != 2 {
		t.Fatalf("jurnal %d baris, ingin 2", len(req.Lines))
	}
	if req.Lines[0].AccountNumber != fallbackRestructureLossExpenseConventional ||
		req.Lines[0].Direction != domain.DirectionDebit {
		t.Fatalf("baris debit salah: %+v", req.Lines[0])
	}
	if req.Lines[1].AccountNumber != fallbackRestructureLossLoanConventional ||
		req.Lines[1].Direction != domain.DirectionCredit {
		t.Fatalf("baris kredit salah: %+v", req.Lines[1])
	}
	if !req.Lines[0].Amount.IsPositive() || !req.Lines[0].Amount.Equal(req.Lines[1].Amount) {
		t.Fatalf("nominal jurnal tidak seimbang: %+v", req.Lines)
	}
	// Saldo kerugian tersimpan sama dengan nominal yang dijurnal.
	if !repo.loan.RestructureLossBalance.Equal(req.Lines[0].Amount) {
		t.Fatalf("saldo kerugian %s, mau %s", repo.loan.RestructureLossBalance, req.Lines[0].Amount)
	}
	// Bentuk kunci idempotensi lengkap: <nomor kredit>-<nomor urut restrukturisasi>.
	wantKey := fmt.Sprintf("RESTRUCT-LOSS-%s-%d", loan.LoanNumber, 1)
	if req.IdempotencyKey != wantKey {
		t.Fatalf("kunci idempotensi %q, mau %q", req.IdempotencyKey, wantKey)
	}
}

// Bila pemetaan produk untuk LOAN_RESTRUCTURE_LOSS belum ada, COA konfigurasi dipakai.
func TestRestructureLoss_CoaKonfigurasiDipakai(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{
		"loan.restructure.loss.enabled":     "true",
		"loan.restructure.loss.coa.expense": "59999",
		"loan.restructure.loss.coa.loan":    "10999",
	}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal diposting %d kali, ingin 1", len(posting.requests))
	}
	lines := posting.requests[0].Lines
	if lines[0].AccountNumber != "59999" || lines[1].AccountNumber != "10999" {
		t.Fatalf("COA konfigurasi tidak dipakai: %+v", lines)
	}
}

// Keuntungan modifikasi (nilai kini > nilai tercatat) tidak diposting sebagai kerugian
// dan tidak menambah saldo kerugian.
func TestRestructureLoss_KeuntunganTidakDijurnal(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         24,
		NewInterestRateAnnual: decimal.NewFromInt(100),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("keuntungan tidak boleh dijurnal, dapat %d jurnal", len(posting.requests))
	}
	if !repo.loan.RestructureLossBalance.IsZero() {
		t.Fatalf("saldo kerugian %s, mau tetap 0", repo.loan.RestructureLossBalance)
	}
}

// Kredit cabang lain tetap ditolak pada jalur aktif, dan pemeriksaan memakai data hasil
// kunci (LockLoanTx).
func TestRestructureLoss_TolakCabangLain(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

	_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:        loan.ID,
		NewTermMonths: 12,
	}, domain.Actor{Role: domain.RoleTeller, BranchCode: "001"})
	if !errors.Is(err, domain.ErrCrossBranchAccess) {
		t.Fatalf("mau ErrCrossBranchAccess, dapat %v", err)
	}
}

// Baris hasil kunci (LockLoanTx) sengaja berbeda dari snapshot GetByID. Perhitungan
// harus memakai sisa pokok baris hasil kunci, bukan angka snapshot di luar transaksi.
func TestRestructureLoss_MemakaiBarisHasilKunci(t *testing.T) {
	snapshot, product, _ := lossLoan()
	snapshot.OutstandingPrincipal = decimal.NewFromInt(10_000_000)
	locked := *snapshot
	locked.OutstandingPrincipal = decimal.NewFromInt(8_000_000)

	repo := &lossLoanRepoStub{loan: snapshot, locked: &locked}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	returned, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                locked.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor())
	if err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}

	// Jadwal baru dibangun dari pokok baris hasil kunci (8jt), bukan snapshot (10jt).
	totalPrincipal := decimal.Zero
	for _, sc := range repo.schedules {
		totalPrincipal = totalPrincipal.Add(sc.PrincipalAmount)
	}
	if !totalPrincipal.Equal(locked.OutstandingPrincipal) {
		t.Fatalf("jadwal dibangun dari pokok %s, mau pokok hasil kunci %s (snapshot %s)",
			totalPrincipal, locked.OutstandingPrincipal, snapshot.OutstandingPrincipal)
	}
	if !returned.OutstandingPrincipal.Equal(locked.OutstandingPrincipal) {
		t.Fatalf("kredit hasil %s, mau pokok hasil kunci %s",
			returned.OutstandingPrincipal, locked.OutstandingPrincipal)
	}
	// Nilai tercatat yang dilaporkan ke audit pun harus nilai hasil kunci.
	if len(posting.requests) != 1 {
		t.Fatalf("jurnal %d, ingin 1", len(posting.requests))
	}
}

// Status yang menentukan boleh/tidaknya restrukturisasi juga dibaca dari baris hasil
// kunci. Snapshot DISBURSED tidak boleh meloloskan kredit yang sudah tidak aktif.
func TestRestructureLoss_StatusDariBarisHasilKunci(t *testing.T) {
	snapshot, product, _ := lossLoan()
	locked := *snapshot
	locked.Status = domain.LoanStatusPaidOff

	repo := &lossLoanRepoStub{loan: snapshot, locked: &locked}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, &stubPosting{}, cfg)

	_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:        locked.ID,
		NewTermMonths: 12,
	}, lossActor())
	if err == nil {
		t.Fatal("status baris hasil kunci bukan aktif harus ditolak, snapshot tidak boleh dipakai")
	}
	if repo.txRestructureCalled {
		t.Fatal("kredit tidak aktif tidak boleh disimpan")
	}
}

// Kunci idempotensi jurnal kerugian harus berbentuk lengkap
// RESTRUCT-LOSS-<nomor kredit>-<nomor urut> dan berbeda antar restrukturisasi.
func TestRestructureLoss_KunciIdempotensiUnikAntarRestrukturisasi(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	// Suku bunga diturunkan bertahap agar restrukturisasi kedua tetap menghasilkan
	// kerugian (bila tarifnya sama, nilai tercatat sudah setara nilai kini -> nol).
	rates := []decimal.Decimal{decimal.NewFromInt(6), decimal.NewFromInt(3)}
	for i, rate := range rates {
		if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
			LoanID:                loan.ID,
			NewTermMonths:         12,
			NewInterestRateAnnual: rate,
		}, lossActor()); err != nil {
			t.Fatalf("RestructureLoan ke-%d: %v", i+1, err)
		}
	}
	if len(posting.requests) != 2 {
		t.Fatalf("jurnal %d, ingin 2", len(posting.requests))
	}
	for i := range posting.requests {
		want := fmt.Sprintf("RESTRUCT-LOSS-%s-%d", loan.LoanNumber, i+1)
		if got := posting.requests[i].IdempotencyKey; got != want {
			t.Fatalf("kunci ke-%d %q, mau %q", i+1, got, want)
		}
	}
	if posting.requests[0].IdempotencyKey == posting.requests[1].IdempotencyKey {
		t.Fatalf("kunci idempotensi dua restrukturisasi harus berbeda, keduanya %q",
			posting.requests[0].IdempotencyKey)
	}
}

// Galat pembacaan pemetaan jurnal BUKAN "produk belum dipetakan": transaksi harus
// gagal, bukan diam-diam jatuh ke COA fallback.
func TestRestructureLoss_GalatPemetaanMenggagalkanTransaksi(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	products := &stubProductRepo{product: product, mappingErr: errors.New("koneksi database terputus")}
	svc := newLossService(repo, products, posting, cfg)

	_, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor())
	if err == nil {
		t.Fatal("galat pembacaan pemetaan harus menggagalkan transaksi")
	}
	if !strings.Contains(err.Error(), "pemetaan") {
		t.Fatalf("pesan galat harus menyebut pemetaan, dapat %q", err.Error())
	}
	if len(posting.requests) != 0 {
		t.Fatalf("tidak boleh ada jurnal fallback saat pemetaan gagal dibaca, dapat %d", len(posting.requests))
	}
}

// sentinelTxRunner menyalurkan penanda transaksi non-nil ke callback, meniru runner
// produksi yang memberikan *sql.Tx.
type sentinelTxRunner struct{ tx any }

func (r sentinelTxRunner) Run(_ context.Context, fn func(tx any) error) error { return fn(r.tx) }

// recordingAuditRepo merekam tx yang dipakai writeAudit.
type recordingAuditRepo struct {
	domain.AuditRepository
	writeCalled bool
	writeTx     any
}

func (a *recordingAuditRepo) Write(_ context.Context, tx any, _ domain.AuditEvent) error {
	a.writeCalled = true
	a.writeTx = tx
	return nil
}

// Audit restrukturisasi harus ditulis DI DALAM transaksi jurnal kerugian. Bila ditulis
// di luar (tx=nil) lalu gagal, pemanggil mencoba lagi padahal jurnal sudah commit dan
// menerbitkan jurnal kerugian kedua.
func TestRestructureLoss_AuditDitulisDalamTransaksi(t *testing.T) {
	loan, product, _ := lossLoan()
	repo := &lossLoanRepoStub{loan: loan}
	posting := &stubPosting{}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newLossService(repo, &stubProductRepo{product: product}, posting, cfg)

	sentinel := &struct{ nama string }{nama: "tx-restrukturisasi"}
	svc.txRunner = sentinelTxRunner{tx: sentinel}
	audit := &recordingAuditRepo{}
	svc.auditRepo = audit

	if _, err := svc.RestructureLoan(context.Background(), domain.RestructureLoanInput{
		LoanID:                loan.ID,
		NewTermMonths:         12,
		NewInterestRateAnnual: decimal.NewFromInt(6),
	}, lossActor()); err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if !audit.writeCalled {
		t.Fatal("audit restrukturisasi tidak ditulis")
	}
	if audit.writeTx != sentinel {
		t.Fatalf("audit ditulis dengan tx %v, mau tx transaksi yang sama (bukan nil)", audit.writeTx)
	}
}
