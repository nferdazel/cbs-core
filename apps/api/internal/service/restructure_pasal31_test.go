package service_test

import (
	"context"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/service"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Batas kualitas Kredit restrukturisasi diatur POJK No. 1 Tahun 2024 **Pasal 31**,
// bukan Pasal 23. Pasal 23 berbunyi: "Bagian Penempatan pada Bank Lain yang memenuhi
// persyaratan kriteria penjaminan Lembaga Penjamin Simpanan dapat dijadikan sebagai
// faktor pengurang dalam pembentukan perhitungan PPKA umum dan khusus." Rujukan
// "Pasal 23 = restrukturisasi" berasal dari POJK 33/POJK.03/2018 yang sudah dicabut.
//
// Pasal 31:
//
//	ayat (1) huruf a: Kredit yang sebelum direstrukturisasi diragukan atau macet
//	                  paling tinggi kurang lancar;
//	ayat (1) huruf b: Kredit yang sebelumnya lancar, DPK, atau kurang lancar tidak
//	                  berubah;
//	ayat (2) huruf a: dapat menjadi lancar setelah 3 kali periode pembayaran tanpa
//	                  tunggakan secara berturut-turut.
//
// Perlakuan akuntansi kerugian restrukturisasi tidak diatur nominalnya oleh pasal ini,
// melainkan didelegasikan Pasal 32 ke standar akuntansi keuangan dan pedoman akuntansi
// BPR ("antara lain pengakuan kerugian yang timbul akibat Restrukturisasi Kredit").
// Test di bawah memastikan kode RestructureLoan menegakkan batas Pasal 31 dan tidak
// mengarang jurnal kerugian di jalur ini.

// newRestructureService merangkai LoanService dengan stub yang sama seperti test
// restrukturisasi lain: tanpa database, tanpa ledger, konfigurasi ambang default POJK.
func newRestructureService(loan *domain.Loan, product *domain.BankingProduct) (domain.LoanService, *stubLoanRepo) {
	repo := &stubLoanRepo{loan: loan}
	products := &stubLoanProductRepo{product: product}
	config := &stubLimitConfig{values: map[string]decimal.Decimal{}}
	return service.NewLoanService(nil, repo, products, nil, nil, nil, nil, nil, config, nil, nil), repo
}

func restructureProduct(productID uuid.UUID) *domain.BankingProduct {
	return &domain.BankingProduct{
		ID:             productID,
		Code:           "KRD-FLAT",
		ProfitScheme:   domain.SchemeInterest,
		ScheduleMethod: domain.ScheduleFlat,
		RateAnnual:     decimal.NewFromInt(12),
	}
}

// Restrukturisasi berulang tidak boleh dipakai untuk melompat ke Lancar. Setiap
// restrukturisasi memperbarui kualitas pra-restrukturisasi dan tetap menahan golongan
// pada batas Pasal 31 ayat (1) selama 3 periode pembayaran bersih belum tercapai.
// Pasal 34 huruf c menyebut restrukturisasi berulang yang hanya mengejar perbaikan
// kualitas sebagai dasar koreksi OJK.
func TestRestructureLoan_BerulangTetapTerbatasPasal31(t *testing.T) {
	loanID := uuid.New()
	productID := uuid.New()
	loan := &domain.Loan{
		ID:                   loanID,
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		DPD:                  10, // tunggakan ringan; golongan tersimpan tetap Macet
		Collectibility:       domain.CollectibilityKol5,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
	}
	svc, repo := newRestructureService(loan, restructureProduct(productID))
	input := domain.RestructureLoanInput{LoanID: loanID, NewTermMonths: 12}

	first, err := svc.RestructureLoan(context.Background(), input, domain.Actor{})
	if err != nil {
		t.Fatalf("restrukturisasi pertama: %v", err)
	}
	if first.Collectibility != domain.CollectibilityKol3 {
		t.Fatalf("mantan Macet harus paling tinggi Kurang Lancar, dapat %s", first.Collectibility)
	}
	if first.RestructuredCount != 1 {
		t.Fatalf("restructured_count %d, ingin 1", first.RestructuredCount)
	}

	// Angsuran baru berjalan, DPD nol. Tanpa batas Pasal 31 kredit akan langsung Lancar.
	repo.loan.DPD = 0
	second, err := svc.RestructureLoan(context.Background(), input, domain.Actor{})
	if err != nil {
		t.Fatalf("restrukturisasi kedua: %v", err)
	}
	if second.Collectibility != domain.CollectibilityKol3 {
		t.Fatalf("restrukturisasi ulang tidak boleh naik ke Lancar, dapat %s", second.Collectibility)
	}
	if second.RestructuredCount != 2 {
		t.Fatalf("restructured_count %d, ingin 2", second.RestructuredCount)
	}
	if second.PreRestructureCollectibility != domain.CollectibilityKol3 {
		t.Fatalf("kualitas pra-restrukturisasi harus Kurang Lancar, dapat %s",
			second.PreRestructureCollectibility)
	}
}

// Pasal 31 ayat (1) huruf b: kredit yang sudah Lancar tidak boleh "membaik" (sudah
// di golongan terbaik); restrukturisasi tetap dicatat tetapi golongannya Lancar.
func TestRestructureLoan_LancarTidakMembaik(t *testing.T) {
	loanID := uuid.New()
	productID := uuid.New()
	loan := &domain.Loan{
		ID:                   loanID,
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		DPD:                  0,
		Collectibility:       domain.CollectibilityKol1,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
	}
	svc, _ := newRestructureService(loan, restructureProduct(productID))

	got, err := svc.RestructureLoan(context.Background(),
		domain.RestructureLoanInput{LoanID: loanID, NewTermMonths: 12}, domain.Actor{})
	if err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if got.Collectibility != domain.CollectibilityKol1 {
		t.Fatalf("kredit Lancar harus tetap Lancar, dapat %s", got.Collectibility)
	}
	if got.AccrualStatus != domain.AccrualStatusAccrual {
		t.Fatalf("kredit Lancar harus accrual basis, dapat %s", got.AccrualStatus)
	}
}

// Kasus batas "selisih nol": bila batas Pasal 31 tidak mengubah golongan (golongan
// sebelum sama dengan hasil penilaian biasa), RestructureLoan tidak menyentuh cadangan
// yang sudah dibukukan. Jalur ini sengaja tidak memposting jurnal kerugian: Pasal 32
// menyerahkan perlakuan akuntansinya ke SAK EP dan pedoman akuntansi BPR, sedangkan
// pembentukan/penyesuaian cadangan adalah milik batch PPAP harian.
func TestRestructureLoan_SelisihKualitasNol_TidakMengubahCadangan(t *testing.T) {
	loanID := uuid.New()
	productID := uuid.New()
	cadangan := decimal.NewFromInt(300_000)
	loan := &domain.Loan{
		ID:                   loanID,
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		DPD:                  60, // 31-90: DPK
		Collectibility:       domain.CollectibilityKol2,
		RequiredPPAP:         cadangan,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
	}
	svc, _ := newRestructureService(loan, restructureProduct(productID))

	got, err := svc.RestructureLoan(context.Background(),
		domain.RestructureLoanInput{LoanID: loanID, NewTermMonths: 12}, domain.Actor{})
	if err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if got.Collectibility != domain.CollectibilityKol2 {
		t.Fatalf("DPK dengan DPD 60 harus tetap DPK, dapat %s", got.Collectibility)
	}
	if !got.RequiredPPAP.Equal(cadangan) {
		t.Fatalf("cadangan tidak boleh berubah di jalur restrukturisasi: %s, ingin %s",
			got.RequiredPPAP, cadangan)
	}
	if !got.OutstandingPrincipal.Equal(decimal.NewFromInt(10_000_000)) {
		t.Fatalf("pokok tercatat tidak boleh berubah: %s", got.OutstandingPrincipal)
	}
}

// Batas Pasal 31 adalah PLAFON golongan, bukan lantai: POJK menetapkan standar
// minimum, sehingga kredit yang memburuk tetap boleh lebih buruk dari golongan
// sebelum restrukturisasi. Ini juga alasan cap tidak boleh dipakai untuk menutup
// penurunan kualitas yang wajar.
func TestRestructureLoan_BolehMemburuk_StandarMinimum(t *testing.T) {
	loanID := uuid.New()
	productID := uuid.New()
	loan := &domain.Loan{
		ID:                   loanID,
		Status:               domain.LoanStatusDisbursed,
		ProductID:            &productID,
		DPD:                  200, // > 180: Diragukan
		Collectibility:       domain.CollectibilityKol2,
		PrincipalAmount:      decimal.NewFromInt(10_000_000),
		OutstandingPrincipal: decimal.NewFromInt(10_000_000),
		TermMonths:           12,
		InterestRateAnnual:   decimal.NewFromInt(12),
	}
	svc, _ := newRestructureService(loan, restructureProduct(productID))

	got, err := svc.RestructureLoan(context.Background(),
		domain.RestructureLoanInput{LoanID: loanID, NewTermMonths: 12}, domain.Actor{})
	if err != nil {
		t.Fatalf("RestructureLoan: %v", err)
	}
	if got.Collectibility != domain.CollectibilityKol4 {
		t.Fatalf("kredit yang memburuk tidak boleh ditutup cap: %s, ingin Diragukan",
			got.Collectibility)
	}
}
