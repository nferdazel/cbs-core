package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ---------- perhitungan murni (domain) ----------

// Beberapa periode berturut-turut: amortisasi = bunga efektif atas nilai tercatat
// dikurangi bunga kontraktual, saldo turun monoton, dan jumlah seluruh amortisasi
// kembali tepat sebesar saldo kerugian awal.
func TestCalculateRestructureLossAmortization_BeberapaPeriode(t *testing.T) {
	// Pokok 3jt dibagi rata 3 angsuran; bunga kontraktual 10rb/angsuran; saldo
	// kerugian awal 30rb; EIR bulanan 1%.
	loss := decimal.NewFromInt(30_000)
	eir := decimal.NewFromFloat(0.01)
	principal := []decimal.Decimal{
		decimal.NewFromInt(3_000_000),
		decimal.NewFromInt(2_000_000),
		decimal.NewFromInt(1_000_000),
	}
	profit := decimal.NewFromInt(10_000)

	wantAmort := []decimal.Decimal{
		decimal.NewFromInt(19_700), // 2.970.000 x 1% = 29.700 - 10.000
		decimal.NewFromInt(9_897),  // 1.989.700 x 1% = 19.897 - 10.000
		decimal.NewFromInt(403),    // periode terakhir: seluruh sisa 30.000-29.597
	}
	total := decimal.Zero
	for i, p := range principal {
		final := i == len(principal)-1
		got, err := domain.CalculateRestructureLossAmortization(domain.RestructureLossAmortizationInput{
			PrincipalBefore:   p,
			LossBalanceBefore: loss,
			EIRMonthly:        eir,
			ContractualProfit: profit,
			FinalPeriod:       final,
		})
		if err != nil {
			t.Fatalf("periode %d: %v", i+1, err)
		}
		if !got.Amortization.Equal(wantAmort[i]) {
			t.Fatalf("amortisasi periode %d = %s, mau %s", i+1, got.Amortization, wantAmort[i])
		}
		if !got.LossBalanceAfter.Equal(loss.Sub(got.Amortization)) {
			t.Fatalf("saldo periode %d = %s, mau %s", i+1, got.LossBalanceAfter, loss.Sub(got.Amortization))
		}
		total = total.Add(got.Amortization)
		loss = got.LossBalanceAfter
	}
	if !loss.IsZero() {
		t.Fatalf("saldo akhir %s, mau tepat nol", loss)
	}
	if !total.Equal(decimal.NewFromInt(30_000)) {
		t.Fatalf("jumlah amortisasi %s, mau sama saldo awal 30.000", total)
	}
}

// Saldo tidak pernah negatif: selisih yang melebihi saldo dipotong tepat sebesar saldo.
func TestCalculateRestructureLossAmortization_SaldoTidakNegatif(t *testing.T) {
	got, err := domain.CalculateRestructureLossAmortization(domain.RestructureLossAmortizationInput{
		PrincipalBefore:   decimal.NewFromInt(1_000_000),
		LossBalanceBefore: decimal.NewFromInt(100),
		EIRMonthly:        decimal.NewFromFloat(0.05), // eirInterest 49.995 >> 100
		ContractualProfit: decimal.Zero,
		FinalPeriod:       false,
	})
	if err != nil {
		t.Fatalf("hitung: %v", err)
	}
	if !got.Amortization.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("amortisasi %s, mau dipotong penuh 100 (tidak boleh melebihi saldo)", got.Amortization)
	}
	if got.LossBalanceAfter.IsNegative() {
		t.Fatalf("saldo menjadi negatif %s", got.LossBalanceAfter)
	}
}

// Bunga kontraktual yang menutupi bunga efektif tidak menghasilkan amortisasi negatif.
func TestCalculateRestructureLossAmortization_TidakAdaAmortisasiNegatif(t *testing.T) {
	got, err := domain.CalculateRestructureLossAmortization(domain.RestructureLossAmortizationInput{
		PrincipalBefore:   decimal.NewFromInt(1_000_000),
		LossBalanceBefore: decimal.NewFromInt(5_000),
		EIRMonthly:        decimal.NewFromFloat(0.001), // 1.000
		ContractualProfit: decimal.NewFromInt(2_000),
		FinalPeriod:       false,
	})
	if err != nil {
		t.Fatalf("hitung: %v", err)
	}
	if !got.Amortization.IsZero() {
		t.Fatalf("amortisasi %s, mau 0 (bunga kontraktual sudah menutupi)", got.Amortization)
	}
	if !got.LossBalanceAfter.Equal(decimal.NewFromInt(5_000)) {
		t.Fatalf("saldo %s, mau tetap 5.000", got.LossBalanceAfter)
	}
}

// Periode terakhir menutup tepat nol walaupun rumus periode itu menghitung selisih lebih
// besar, sehingga tidak ada rupiah yang menggantung akibat pembulatan.
func TestCalculateRestructureLossAmortization_PeriodeTerakhirMenutupNol(t *testing.T) {
	got, err := domain.CalculateRestructureLossAmortization(domain.RestructureLossAmortizationInput{
		PrincipalBefore:   decimal.NewFromInt(1_000_000),
		LossBalanceBefore: decimal.NewFromInt(403),
		EIRMonthly:        decimal.NewFromFloat(0.01),
		ContractualProfit: decimal.NewFromInt(10_000),
		FinalPeriod:       true,
	})
	if err != nil {
		t.Fatalf("hitung: %v", err)
	}
	if !got.Amortization.Equal(decimal.NewFromInt(403)) {
		t.Fatalf("amortisasi periode terakhir %s, mau 403 (seluruh sisa)", got.Amortization)
	}
	if !got.LossBalanceAfter.IsZero() {
		t.Fatalf("saldo akhir %s, mau tepat nol", got.LossBalanceAfter)
	}
}

// EIR yang belum tersimpan ditolak, bukan diam-diam menjadi nol.
func TestCalculateRestructureLossAmortization_EIRHilangDitolak(t *testing.T) {
	_, err := domain.CalculateRestructureLossAmortization(domain.RestructureLossAmortizationInput{
		PrincipalBefore:   decimal.NewFromInt(1_000_000),
		LossBalanceBefore: decimal.NewFromInt(10_000),
		EIRMonthly:        decimal.Zero,
		ContractualProfit: decimal.Zero,
	})
	if !errors.Is(err, domain.ErrEIRMissing) {
		t.Fatalf("mau ErrEIRMissing, dapat %v", err)
	}
}

// ---------- batch service dengan stub ----------

// amortLoanRepoStub adalah domain.LoanRepository minimal untuk amortisasi: kandidat
// dikembalikan apa adanya, kunci kredit mengembalikan baris terkunci, dan pengurangan
// saldo mengikuti kunci jurnal (idempoten).
type amortLoanRepoStub struct {
	domain.LoanRepository

	candidates []domain.LoanRestructureLossAmortizationCandidate
	loan       *domain.Loan
	// locked sengaja terpisah dari loan agar uji membuktikan perhitungan memakai baris
	// hasil kunci, bukan snapshot.
	locked    *domain.Loan
	schedules []domain.LoanSchedule

	listCalled bool
	applied    map[string]decimal.Decimal
}

func (r *amortLoanRepoStub) ListRestructureLossAmortizationCandidates(_ context.Context, _ time.Time, _ domain.Actor) ([]domain.LoanRestructureLossAmortizationCandidate, error) {
	r.listCalled = true
	return r.candidates, nil
}

// authoritative adalah baris yang dianggap tersimpan di database: baris terkunci bila
// ada, jika tidak snapshot. Lock dan Apply sama-sama memakainya, meniru produksi yang
// mengunci dan mengubah baris yang sama.
func (r *amortLoanRepoStub) authoritative() *domain.Loan {
	if r.locked != nil {
		return r.locked
	}
	return r.loan
}

func (r *amortLoanRepoStub) LockLoanTx(context.Context, any, uuid.UUID) (*domain.Loan, error) {
	cp := *r.authoritative()
	return &cp, nil
}

func (r *amortLoanRepoStub) GetSchedulesTx(context.Context, any, uuid.UUID) ([]domain.LoanSchedule, error) {
	return r.schedules, nil
}

func (r *amortLoanRepoStub) ApplyRestructureLossAmortizationTx(_ context.Context, _ any, _ uuid.UUID, scheduleID *uuid.UUID, amount decimal.Decimal, idempotencyKey string, amortizedAt time.Time) (bool, error) {
	if r.applied == nil {
		r.applied = map[string]decimal.Decimal{}
	}
	if _, exists := r.applied[idempotencyKey]; exists {
		return false, nil
	}
	stored := r.authoritative()
	if stored.RestructureLossBalance.LessThan(amount) {
		return false, nil
	}
	r.applied[idempotencyKey] = amount
	stored.RestructureLossBalance = stored.RestructureLossBalance.Sub(amount)
	if scheduleID != nil {
		for i := range r.schedules {
			if r.schedules[i].ID == *scheduleID {
				at := amortizedAt
				r.schedules[i].RestructureLossAmortizedAt = &at
				r.schedules[i].RestructureLossAmortizedAmount = r.schedules[i].RestructureLossAmortizedAmount.Add(amount)
			}
		}
	}
	return true, nil
}

var _ domain.LoanRepository = (*amortLoanRepoStub)(nil)
var _ domain.RestructureLossAmortizationRepository = (*amortLoanRepoStub)(nil)

// amortFixture merangkai satu kredit bersaldo kerugian 30rb dengan jadwal 3 angsuran.
func amortFixture() (*amortLoanRepoStub, *stubPosting, *stubProductRepo, *domain.Loan) {
	productID := uuid.New()
	loan := &domain.Loan{
		ID:                     uuid.New(),
		LoanNumber:             "KRD-AMORT-1",
		Status:                 domain.LoanStatusDisbursed,
		ProductID:              &productID,
		BranchCode:             "002",
		OriginalEIRMonthly:     decimal.NewFromFloat(0.01),
		RestructureLossBalance: decimal.NewFromInt(30_000),
	}
	due := time.Now().UTC().AddDate(0, 0, -1)
	schedules := []domain.LoanSchedule{
		{ID: uuid.New(), LoanID: loan.ID, InstallmentNo: 1, DueDate: due, PrincipalAmount: decimal.NewFromInt(1_000_000), ProfitAmount: decimal.NewFromInt(10_000)},
		{ID: uuid.New(), LoanID: loan.ID, InstallmentNo: 2, DueDate: due, PrincipalAmount: decimal.NewFromInt(1_000_000), ProfitAmount: decimal.NewFromInt(10_000)},
		{ID: uuid.New(), LoanID: loan.ID, InstallmentNo: 3, DueDate: due, PrincipalAmount: decimal.NewFromInt(1_000_000), ProfitAmount: decimal.NewFromInt(10_000)},
	}
	repo := &amortLoanRepoStub{
		candidates: []domain.LoanRestructureLossAmortizationCandidate{{LoanID: loan.ID, LoanNumber: loan.LoanNumber}},
		loan:       loan,
		schedules:  schedules,
	}
	products := &stubProductRepo{product: &domain.BankingProduct{ID: productID, Code: "KRD-FLAT", Book: domain.BookConventional}}
	return repo, &stubPosting{}, products, loan
}

func newAmortService(repo *amortLoanRepoStub, products *stubProductRepo, posting *stubPosting, cfg *ckpnConfigStub) *loanService {
	return &loanService{
		loanRepo:    repo,
		productRepo: products,
		resolver:    stubResolver{},
		posting:     posting,
		config:      cfg,
		txRunner:    stubTxRunner{},
	}
}

func amortActor() domain.Actor {
	return domain.Actor{Username: "uji.amort", Role: domain.RoleSuperAdmin, BranchCode: "001"}
}

// Saklar mati tidak mengubah apa pun: kandidat tidak diambil dan tidak ada jurnal.
func TestAmortizeRestructureLoss_SaklarMatiTidakMenghitung(t *testing.T) {
	repo, posting, products, _ := amortFixture()
	cfg := &ckpnConfigStub{values: map[string]string{}} // enabled tidak ada -> false
	svc := newAmortService(repo, products, posting, cfg)

	summary, err := svc.AmortizeRestructureLoss(context.Background(), time.Now().UTC(), amortActor())
	if err != nil {
		t.Fatalf("AmortizeRestructureLoss: %v", err)
	}
	if repo.listCalled {
		t.Fatal("saklar mati tidak boleh mengambil kandidat")
	}
	if len(posting.requests) != 0 {
		t.Fatalf("saklar mati tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
	if summary.Processed != 0 || !summary.TotalAmortized.IsZero() {
		t.Fatalf("saklar mati harus tidak memproses apa pun: %+v", summary)
	}
}

// Parameter diskonto yang DIISI tetapi salah ditolak dengan pesan yang menyebut kuncinya.
func TestAmortizeRestructureLoss_ParameterSalahDitolak(t *testing.T) {
	repo, posting, products, _ := amortFixture()
	cfg := &ckpnConfigStub{values: map[string]string{
		"loan.restructure.loss.enabled":              "true",
		"loan.restructure.loss.discount_rate_annual": "bukan angka",
	}}
	svc := newAmortService(repo, products, posting, cfg)

	_, err := svc.AmortizeRestructureLoss(context.Background(), time.Now().UTC(), amortActor())
	if !errors.Is(err, domain.ErrRestructureLossParamInvalid) {
		t.Fatalf("mau ErrRestructureLossParamInvalid, dapat %v", err)
	}
	if !strings.Contains(err.Error(), "loan.restructure.loss.discount_rate_annual") {
		t.Fatalf("pesan harus menyebut kunci konfigurasi, dapat %q", err.Error())
	}
	if len(posting.requests) != 0 {
		t.Fatalf("parameter salah tidak boleh memposting jurnal, dapat %d", len(posting.requests))
	}
}

// Jalur aktif: satu jurnal per angsuran dengan Db 10301 / Kr 40100, nominal selisih,
// cabang KREDIT, tanggal bisnis, dan saldo akhir tepat nol.
func TestAmortizeRestructureLoss_MempostingDanMenutupNol(t *testing.T) {
	repo, posting, products, loan := amortFixture()
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newAmortService(repo, products, posting, cfg)

	day := time.Now().UTC()
	summary, err := svc.AmortizeRestructureLoss(context.Background(), day, amortActor())
	if err != nil {
		t.Fatalf("AmortizeRestructureLoss: %v", err)
	}
	if summary.Amortized != 3 {
		t.Fatalf("angsuran diamortisasi %d, mau 3 (items: %+v)", summary.Amortized, summary.Items)
	}
	if !repo.loan.RestructureLossBalance.IsZero() {
		t.Fatalf("saldo kerugian akhir %s, mau nol", repo.loan.RestructureLossBalance)
	}
	if !summary.TotalAmortized.Equal(decimal.NewFromInt(30_000)) {
		t.Fatalf("total amortisasi %s, mau 30.000", summary.TotalAmortized)
	}
	if len(posting.requests) != 3 {
		t.Fatalf("jurnal %d, mau 3", len(posting.requests))
	}
	// Penanda amortisasi periode terakhir terisi.
	if repo.schedules[2].RestructureLossAmortizedAt == nil {
		t.Fatal("penanda amortisasi periode terakhir tidak terisi")
	}
	for i, req := range posting.requests {
		wantKey := "LOSSAMORT-" + loan.LoanNumber + "-" + decimal.NewFromInt(int64(i+1)).String()
		if req.IdempotencyKey != wantKey {
			t.Fatalf("kunci jurnal ke-%d %q, mau %q", i+1, req.IdempotencyKey, wantKey)
		}
		if req.BranchCode != loan.BranchCode {
			t.Fatalf("cabang jurnal %q, mau cabang kredit %q (bukan cabang aktor %q)",
				req.BranchCode, loan.BranchCode, amortActor().BranchCode)
		}
		if !sameBusinessDay(req.EntryDate, day) {
			t.Fatalf("entry date %s, mau tanggal bisnis %s", req.EntryDate, day)
		}
		if len(req.Lines) != 2 {
			t.Fatalf("jurnal %d baris, mau 2", len(req.Lines))
		}
		if req.Lines[0].AccountNumber != fallbackRestructureLossLoanConventional ||
			req.Lines[0].Direction != domain.DirectionDebit {
			t.Fatalf("baris debit salah: %+v", req.Lines[0])
		}
		if req.Lines[1].AccountNumber != fallbackRestructureLossAmortIncomeConventional ||
			req.Lines[1].Direction != domain.DirectionCredit {
			t.Fatalf("baris kredit salah: %+v", req.Lines[1])
		}
		if !req.Lines[0].Amount.Equal(req.Lines[1].Amount) {
			t.Fatalf("jurnal tidak seimbang: %+v", req.Lines)
		}
	}
}

// Pengulangan pada tanggal bisnis yang sama tidak menggandakan jurnal maupun
// pengurangan saldo.
func TestAmortizeRestructureLoss_PengulanganTidakMenggandakan(t *testing.T) {
	repo, posting, products, loan := amortFixture()
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newAmortService(repo, products, posting, cfg)

	day := time.Now().UTC()
	if _, err := svc.AmortizeRestructureLoss(context.Background(), day, amortActor()); err != nil {
		t.Fatalf("eksekusi pertama: %v", err)
	}
	first := len(posting.requests)
	if first != 3 {
		t.Fatalf("jurnal pertama %d, mau 3", first)
	}

	// Pemulihan state: saldo dan penanda tetap seperti setelah eksekusi pertama.
	if _, err := svc.AmortizeRestructureLoss(context.Background(), day, amortActor()); err != nil {
		t.Fatalf("eksekusi kedua: %v", err)
	}
	if len(posting.requests) != first {
		t.Fatalf("jurnal bertambah menjadi %d, mau tetap %d (idempoten)", len(posting.requests), first)
	}
	if !loan.RestructureLossBalance.IsZero() {
		t.Fatalf("saldo kerugian %s, mau tetap nol", loan.RestructureLossBalance)
	}
}

// Perhitungan memakai baris hasil kunci (LockLoanTx), bukan snapshot di luar transaksi.
func TestAmortizeRestructureLoss_MemakaiBarisHasilKunci(t *testing.T) {
	repo, posting, products, loan := amortFixture()
	// Snapshot bersaldo nol; baris terkunci bersaldo 30rb. Tanpa pemakaian baris hasil
	// kunci, kredit ini akan dilewati dan tidak ada jurnal.
	snapshot := *loan
	snapshot.RestructureLossBalance = decimal.Zero
	repo.loan = &snapshot
	repo.locked = loan

	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newAmortService(repo, products, posting, cfg)

	summary, err := svc.AmortizeRestructureLoss(context.Background(), time.Now().UTC(), amortActor())
	if err != nil {
		t.Fatalf("AmortizeRestructureLoss: %v", err)
	}
	if summary.Amortized != 3 {
		t.Fatalf("angsuran diamortisasi %d, mau 3 dari baris hasil kunci", summary.Amortized)
	}
}

// Bila kunci jurnal sudah ada (Apply mengembalikan false), service TIDAK memposting
// ulang dan tidak mengubah saldo. Ini cabang idempotensi yang menjaga pengulangan.
func TestAmortizeRestructureLoss_KunciJurnalSudahAdaTidakMemposting(t *testing.T) {
	repo, posting, products, loan := amortFixture()
	// Seluruh kunci sudah ada di sisi repository: setiap Apply ditolak.
	repo.applied = map[string]decimal.Decimal{
		"LOSSAMORT-" + loan.LoanNumber + "-1": decimal.NewFromInt(19_700),
		"LOSSAMORT-" + loan.LoanNumber + "-2": decimal.NewFromInt(9_897),
		"LOSSAMORT-" + loan.LoanNumber + "-3": decimal.NewFromInt(403),
	}
	cfg := &ckpnConfigStub{values: map[string]string{"loan.restructure.loss.enabled": "true"}}
	svc := newAmortService(repo, products, posting, cfg)

	before := loan.RestructureLossBalance
	if _, err := svc.AmortizeRestructureLoss(context.Background(), time.Now().UTC(), amortActor()); err != nil {
		t.Fatalf("AmortizeRestructureLoss: %v", err)
	}
	if len(posting.requests) != 0 {
		t.Fatalf("kunci jurnal sudah ada tidak boleh memposting ulang, dapat %d jurnal", len(posting.requests))
	}
	if !loan.RestructureLossBalance.Equal(before) {
		t.Fatalf("saldo kerugian berubah %s -> %s padahal jurnal sudah ada",
			before, loan.RestructureLossBalance)
	}
}
