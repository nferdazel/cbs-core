package ojkreport

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// Form 01.01 harus memakai nama pos resmi untuk sandi yang tercantum di PDF #page 110,
// memakai uraian bank untuk sandi tak dikenal, dan menandai kolom Sandi Kantor belum
// tersedia (register bersifat bank-wide).
func TestBuildForm01_01NamaPosDanKolomBelumTersedia(t *testing.T) {
	rows := []domain.OffBalanceAggregate{
		{Category: domain.OffBalanceCategoryKomitmen, PositionCode: "6102010000",
			TotalAmount: decimal.NewFromInt(1500000), ItemCount: 1},
		{Category: domain.OffBalanceCategoryLainnya, PositionCode: "",
			Description: "Pos manual bank", TotalAmount: decimal.NewFromInt(250000), ItemCount: 1},
	}

	sec := BuildForm01_01(rows)
	if sec.Form != "01.01" {
		t.Fatalf("form = %q, ingin 01.01", sec.Form)
	}
	if !punyaUnavailable(sec.Unavailable, "I") {
		t.Fatal("kolom Sandi Kantor tidak ditandai belum tersedia")
	}
	if len(sec.Rows) != 2 {
		t.Fatalf("jumlah baris = %d, ingin 2", len(sec.Rows))
	}

	// Baris bersandi resmi: nama resmi diambil dari peta, sandi tampil apa adanya.
	if got := cellValue(sec.Rows[0], "II"); got != "Fasilitas Kredit kepada Nasabah yang Belum Ditarik" {
		t.Errorf("nama pos bersandi = %q", got)
	}
	if got := cellValue(sec.Rows[0], "III"); got != "6102010000" {
		t.Errorf("sandi pos = %q", got)
	}
	// Baris tanpa sandi: nama dari uraian bank, sandi ditulis "-".
	if got := cellValue(sec.Rows[1], "II"); got != "Pos manual bank" {
		t.Errorf("nama pos tanpa sandi = %q", got)
	}
	if got := cellValue(sec.Rows[1], "III"); got != "-" {
		t.Errorf("sandi pos kosong = %q, ingin -", got)
	}
}

// cellValue mengambil nilai satu sel menurut sandi kolomnya.
func cellValue(row TableRow, sandi string) string {
	for _, c := range row.Cells {
		if c.Sandi == sandi {
			return c.Value
		}
	}
	return ""
}
