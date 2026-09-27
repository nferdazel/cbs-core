package ojkreport

import (
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// Baris direksi/komisaris harus masuk bagian 00.02 dan pejabat eksekutif ke bagian
// 00.03; kolom yang belum punya sumber didaftarkan belum tersedia.
func TestBuildKelembagaanTablesMemisahkanKategori(t *testing.T) {
	report := domain.KelembagaanReport{
		Offices: []domain.BankOffice{
			{ID: uuid.New(), OfficeType: "KANTOR_PUSAT", Name: "Kantor Pusat", Status: domain.KelembagaanKantorAktif},
		},
		Management: []domain.BankManagement{
			{ID: uuid.New(), Category: domain.KelembagaanKategoriDireksi, Name: "Budi", Position: "Direktur Utama", Status: domain.KelembagaanOrangAktif},
			{ID: uuid.New(), Category: domain.KelembagaanKategoriKomisaris, Name: "Ani", Position: "Komisaris", Status: domain.KelembagaanOrangAktif},
			{ID: uuid.New(), Category: domain.KelembagaanKategoriPejabatEksekutif, Name: "Citra", Position: "Kepala Satuan Kerja", Status: domain.KelembagaanOrangAktif},
		},
	}

	tables := BuildKelembagaanTables(report)
	if len(tables) != 3 {
		t.Fatalf("jumlah bagian = %d, ingin 3", len(tables))
	}
	if tables[0].Form != "00.04" || len(tables[0].Rows) != 1 {
		t.Fatalf("bagian kantor: form=%q rows=%d", tables[0].Form, len(tables[0].Rows))
	}
	if tables[1].Form != "00.02" || len(tables[1].Rows) != 2 {
		t.Fatalf("bagian direksi/komisaris: form=%q rows=%d, ingin 2", tables[1].Form, len(tables[1].Rows))
	}
	if tables[2].Form != "00.03" || len(tables[2].Rows) != 1 {
		t.Fatalf("bagian pejabat eksekutif: form=%q rows=%d, ingin 1", tables[2].Form, len(tables[2].Rows))
	}

	// NIK wajib ditandai belum tersedia (keputusan privasi), bukan dikarang.
	if !punyaUnavailable(tables[1].Unavailable, "III") {
		t.Fatal("bagian direksi/komisaris tidak menandai NIK belum tersedia")
	}
}

// punyaUnavailable melaporkan apakah sandi ada pada daftar kolom belum tersedia.
func punyaUnavailable(list []ColumnUnavailable, sandi string) bool {
	for _, c := range list {
		if c.Sandi == sandi {
			return true
		}
	}
	return false
}
