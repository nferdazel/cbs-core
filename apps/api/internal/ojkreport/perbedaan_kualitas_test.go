package ojkreport

import (
	"context"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

func perbedaanCell(t *testing.T, row TableRow, sandi string) string {
	t.Helper()
	for _, c := range row.Cells {
		if c.Sandi == sandi {
			return c.Value
		}
	}
	t.Fatalf("sel %s tidak ditemukan pada baris %s", sandi, row.Key)
	return ""
}

// Kolom I-X terisi dari baris kredit; kolom PPKA diisi nominal required_ppap. Kredit
// di luar status berjalan dilewati.
func TestBuildPerbedaanKualitasTableBarisDanKualitas(t *testing.T) {
	akad := time.Date(2025, time.January, 5, 0, 0, 0, 0, time.UTC)
	jatuh := time.Date(2027, time.January, 5, 0, 0, 0, 0, time.UTC)
	rows := []LoanRow{
		{
			Status:                 "DISBURSED",
			LoanNumber:             "LN-001",
			BranchCode:             "001",
			IDPihakLawan:           "CIF-1",
			Collectibility:         "2_DPK",
			OJKJenisPenggunaanCode: "10",
			PrincipalAmount:        decimal.NewFromInt(50_000_000),
			Outstanding:            decimal.NewFromInt(40_000_000),
			RequiredPPAP:           decimal.NewFromInt(1_200_000),
			AkadDate:               &akad,
			FinalDueDate:           &jatuh,
		},
		{Status: "PAID_OFF", LoanNumber: "LN-002"},
	}

	sec := BuildPerbedaanKualitasTable(rows)
	if sec.Form != "LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF" {
		t.Fatalf("form = %s", sec.Form)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("jumlah baris = %d, ingin 1 (hanya kredit berjalan)", len(sec.Rows))
	}
	row := sec.Rows[0]
	if got := perbedaanCell(t, row, perbedaanSandiNoRekening); got != "LN-001" {
		t.Fatalf("no rekening = %q", got)
	}
	if got := perbedaanCell(t, row, perbedaanSandiJenisAset); got != "3" {
		t.Fatalf("jenis aset = %q, ingin sandi kredit 3", got)
	}
	if got := perbedaanCell(t, row, perbedaanSandiKualitas); got != "2" {
		t.Fatalf("kualitas = %q, ingin 2 (DPK)", got)
	}
	if got := perbedaanCell(t, row, perbedaanSandiPPKA); got != "1200000" {
		t.Fatalf("PPKA = %q, ingin 1200000", got)
	}
	if got := perbedaanCell(t, row, perbedaanSandiTanggalJatuh); got != "2027-01-05" {
		t.Fatalf("tanggal jatuh tempo = %q", got)
	}

	// Bagian pembanding belum tersedia dan harus didaftarkan dengan alasan, bukan dikosongkan.
	unavailableColumn(t, sec, perbedaanSandiBPRLain)
	unavailableColumn(t, sec, perbedaanSandiKualitasPPKA)
}

// Tanggal jatuh tempo tanpa jadwal diisi tanggal mulai, mengikuti penjelasan Form
// 19.00-3 kolom IX.
func TestBuildPerbedaanKualitasTableJatuhTempoIkutTanggalMulai(t *testing.T) {
	akad := time.Date(2025, time.March, 1, 0, 0, 0, 0, time.UTC)
	sec := BuildPerbedaanKualitasTable([]LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-003", Collectibility: "1_LANCAR", AkadDate: &akad},
	})
	if got := perbedaanCell(t, sec.Rows[0], perbedaanSandiTanggalJatuh); got != "2025-03-01" {
		t.Fatalf("tanggal jatuh tempo = %q, ingin tanggal mulai", got)
	}
}

// perbedaanStubSource memenuhi LoanDataSource untuk menguji GeneratePerbedaanKualitas.
type perbedaanStubSource struct {
	rows []LoanRow
	err  error
}

func (s *perbedaanStubSource) ListLoansForOJK(_ context.Context, _ time.Time, _ domain.Actor) ([]LoanRow, error) {
	return s.rows, s.err
}

func TestGeneratePerbedaanKualitasMenolakTanpaSumber(t *testing.T) {
	if _, err := GeneratePerbedaanKualitas(context.Background(), nil, time.Now(), domain.Actor{Role: domain.RoleSuperAdmin}); err == nil {
		t.Fatal("sumber nil harus ditolak")
	}
}

func TestGeneratePerbedaanKualitasMenghasilkanBundle(t *testing.T) {
	src := &perbedaanStubSource{rows: []LoanRow{
		{Status: "DISBURSED", LoanNumber: "LN-001", Collectibility: "1_LANCAR"},
	}}
	bundle, err := GeneratePerbedaanKualitas(context.Background(), src,
		time.Date(2026, time.September, 15, 0, 0, 0, 0, time.UTC), domain.Actor{Role: domain.RoleSuperAdmin})
	if err != nil {
		t.Fatalf("error tak terduga: %v", err)
	}
	if len(bundle.Tables) != 1 || bundle.Tables[0].Form != "LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF" {
		t.Fatalf("bundle harus memuat satu tabel perbedaan kualitas: %+v", bundle.Tables)
	}
	if want := time.Date(2026, time.October, 10, 0, 0, 0, 0, time.UTC); !bundle.Deadline.Equal(want) {
		t.Fatalf("tenggat = %s, ingin %s", bundle.Deadline, want)
	}
}
