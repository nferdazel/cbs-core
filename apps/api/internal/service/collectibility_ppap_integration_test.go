package service_test

import (
	"fmt"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Uji integrasi terhadap PostgreSQL sungguhan: membuktikan kualitas aset (kolektibilitas)
// MENGGUNAKAN mekanisme yang sudah ada, bukan dibiarkan basi. Jalur EOD -> langkah PPAP
// (eod_step_definitions kode "ppap") memanggil PPAP RunDaily; di dalamnya DPD dihitung
// ulang dari jadwal angsuran dan golongan diturunkan dari pita DPD, lalu kedua nilai itu
// disimpan kembali ke loans (collectibility + dpd + required_ppap).
//
// Pita yang berlaku adalah POJK No. 1 Tahun 2024 tentang Kualitas Aset BPR, Lampiran II,
// baris "Kredit dengan angsuran 1 (satu) bulan atau lebih": golongan Lancar mencakup
// "tunggakan angsuran pokok dan/atau bunga tidak lebih dari 30 (tiga puluh) hari ...
// dan Kredit belum jatuh tempo". Karena itu DPD 21 WAJIB tetap Lancar (bukan cacat basi),
// sedangkan DPD 100 WAJIB turun ke Kurang Lancar. Uji ini gagal bila mekanisme pembaruan
// tidak berjalan atau pitanya tidak sesuai Lampiran II.
func TestIntegrasiKolektibilitasKreditMenunggakSelarasPPAP(t *testing.T) {
	e := newMoneyEnv(t)
	asOf := ckpnTestAsOf()
	e.setCollateralEnabled(t, false) // dasar PPAP tanpa pengurang agunan agar angkanya eksak

	branchCode := ckpnTestBranchCode("K")
	branchID := e.ensureBranch(t, branchCode, "Cabang Uji Kolektibilitas DPD")
	actor := domain.Actor{UserID: e.actor.UserID, Username: "admin.ujikol", Role: domain.RoleAdmin, BranchCode: branchCode}

	kasus := []struct {
		nama     string
		dpd      int
		wantKol  string
		wantRate decimal.Decimal
	}{
		{"tunggakan 21 hari masih Lancar", 21, string(domain.CollectibilityKol1), decimal.NewFromFloat(0.005)},
		{"tunggakan 100 hari wajib Kurang Lancar", 100, string(domain.CollectibilityKol3), decimal.NewFromFloat(0.10)},
	}

	for i, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			cust := e.newCustomer(t, "Nasabah "+k.nama, fmt.Sprintf("kol-%d-%d@uji.local", i, time.Now().UnixNano()))
			acc := e.newAccountInBranch(t, cust.ID, branchID)
			loan := e.disburseAs(t, actor, cust.ID, acc, idr(10_000_000), 6)

			// Keadaan awal: kualitas dari pencairan masih Lancar dan DPD 0 — inilah
			// "kualitas basi" yang dikeluhkan sebelum batch berjalan.
			if got := e.loanString(t, loan.ID, "collectibility"); got != string(domain.CollectibilityKol1) {
				t.Fatalf("kualitas sebelum PPAP %q, mau %q", got, domain.CollectibilityKol1)
			}
			if got := e.loanInt(t, loan.ID, "dpd"); got != 0 {
				t.Fatalf("DPD sebelum PPAP %d, mau 0", got)
			}

			e.setOldestDueDate(t, loan.ID, asOf.AddDate(0, 0, -k.dpd))

			if _, err := e.ppapSvc.RunDaily(e.ctx, asOf, actor); err != nil {
				t.Fatalf("RunDaily PPAP: %v", err)
			}

			if got := e.loanInt(t, loan.ID, "dpd"); got != k.dpd {
				t.Fatalf("DPD sesudah PPAP %d, mau %d: mekanisme tidak memperbarui DPD", got, k.dpd)
			}
			if got := e.loanString(t, loan.ID, "collectibility"); got != k.wantKol {
				t.Fatalf("kualitas sesudah PPAP %q, mau %q (pita POJK 1/2024 Lampiran II)", got, k.wantKol)
			}
			outstanding := e.loanDecimal(t, loan.ID, "outstanding_principal")
			wantPPAP := domain.RoundToRupiah(outstanding.Mul(k.wantRate))
			if got := e.loanDecimal(t, loan.ID, "required_ppap"); !got.Equal(wantPPAP) {
				t.Fatalf("required_ppap %s, mau %s (tarif %s x pokok %s)", got, wantPPAP, k.wantRate, outstanding)
			}
		})
	}
}
