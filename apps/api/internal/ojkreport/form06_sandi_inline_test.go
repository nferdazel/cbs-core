package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// TestForm06KolomSandiInlineDariFieldPenyimpanan membuktikan kolom VIII (Jenis
// Penggunaan), IX (Hubungan dengan Bank), XI (Periode Pembayaran), dan XXII (Lokasi
// Penggunaan) kini terisi dari field penyimpanan nullable, dan menulis "-" bila bank
// belum mengisinya. Keempatnya tidak lagi terdaftar sebagai kolom tidak tersedia.
func TestForm06KolomSandiInlineDariFieldPenyimpanan(t *testing.T) {
	rows := []LoanRow{
		{
			Status:                   "DISBURSED",
			LoanNumber:               "LN-ISI",
			OJKJenisPenggunaanCode:   "10",
			OJKHubunganBankCode:      "20",
			OJKPeriodePembayaranCode: "1",
			OJKKabupatenCode:         "0197",
		},
		{Status: "DISBURSED", LoanNumber: "LN-KOSONG"},
	}
	sec := buildForm06(rows)

	kasus := []struct {
		sandi, mau string
		nama       string
	}{
		{form06SandiPenggunaan, "10", "VIII Jenis Penggunaan"},
		{form06SandiHubungan, "20", "IX Hubungan dengan Bank"},
		{form06SandiPeriodeBayar, "1", "XI Periode Pembayaran"},
		{form06SandiLokasi, "0197", "XXII Lokasi Penggunaan"},
	}
	for _, c := range kasus {
		if got := findCell(t, sec, "LN-ISI", c.sandi).Value; got != c.mau {
			t.Errorf("%s terisi = %q, ingin %q", c.nama, got, c.mau)
		}
		if got := findCell(t, sec, "LN-KOSONG", c.sandi).Value; got != "-" {
			t.Errorf("%s kosong = %q, ingin -", c.nama, got)
		}
		for _, u := range sec.Unavailable {
			if u.Sandi == c.sandi {
				t.Errorf("%s masih terdaftar tidak tersedia: %s", c.nama, u.Reason)
			}
		}
	}
}

// TestListLoansForOJKMengisiSandiInlineForm06 menutup rantai adaptor: sandi inline per
// kredit (VIII/XI/XXII) dipetakan dari baris kredit, dan sandi hubungan dengan bank
// (IX) dipetakan dari nasabah lewat GetByIDs yang sama. Nasabah tanpa sandi dibiarkan
// kosong; form menulis "-".
func TestListLoansForOJKMengisiSandiInlineForm06(t *testing.T) {
	nasabah := uuid.New()
	repo := &loanAggStub{
		loans: []domain.Loan{{
			LoanNumber:               "LN-INLINE",
			Status:                   domain.LoanStatusDisbursed,
			CustomerID:               nasabah,
			OJKJenisPenggunaanCode:   "31",
			OJKPeriodePembayaranCode: "2",
			OJKKabupatenCode:         "0197",
		}},
	}
	customers := &customerBatchStub{records: map[uuid.UUID]*domain.CustomerRecord{
		nasabah: {ID: nasabah, OJKHubunganBankCode: "11"},
	}}
	src := RepoSource{Loans: repo, Customers: customers}

	rows, err := src.ListLoansForOJK(context.Background(), time.Now().UTC(), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("ListLoansForOJK: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1", len(rows))
	}
	if rows[0].OJKJenisPenggunaanCode != "31" || rows[0].OJKPeriodePembayaranCode != "2" || rows[0].OJKKabupatenCode != "0197" {
		t.Errorf("sandi inline kredit = %q/%q/%q, ingin 31/2/0197",
			rows[0].OJKJenisPenggunaanCode, rows[0].OJKPeriodePembayaranCode, rows[0].OJKKabupatenCode)
	}
	if rows[0].OJKHubunganBankCode != "11" {
		t.Errorf("sandi IX = %q, ingin 11", rows[0].OJKHubunganBankCode)
	}
}
