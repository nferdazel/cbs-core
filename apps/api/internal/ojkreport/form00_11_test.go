package ojkreport

import (
	"strings"
	"testing"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// kantor0011 membangun satu kantor Form 00.11 untuk uji perakit.
func kantor0011(sandiJenis, kode string) domain.BankOffice {
	return domain.BankOffice{
		ID:                 uuid.New(),
		OfficeType:         "KANTOR_KAS",
		Code:               kode,
		Name:               "Kantor Kas Melati",
		Address:            "Jl. Melati 2",
		OJKKabupatenCode:   "1101",
		Status:             "AKTIF",
		OJKOfficeKindCode:  sandiJenis,
		ParentOfficeCode:   "001",
		PreviousOfficeCode: "",
		Coordinates:        "-6.2,106.8",
		HeadName:           "Budi",
		PhoneNumber:        "021-555",
		OJKChangeCode:      "1",
	}
}

// TestBuildForm00_11HanyaKantorTersandiJenis memastikan hanya kantor yang bank beri sandi
// Jenis (kolom I) yang masuk; kantor pusat/cabang tanpa sandi itu tidak masuk.
func TestBuildForm00_11HanyaKantorTersandiJenis(t *testing.T) {
	pusat := domain.BankOffice{ID: uuid.New(), OfficeType: "KANTOR_PUSAT", Name: "Pusat", Status: "AKTIF"}
	sec := BuildForm00_11([]domain.BankOffice{pusat, kantor0011("02", "101")})
	if sec.Form != "00.11" {
		t.Fatalf("form = %q, ingin 00.11", sec.Form)
	}
	if len(sec.Rows) != 1 {
		t.Fatalf("ingin 1 baris (hanya kantor tersandi), dapat %d", len(sec.Rows))
	}
	// Kolom I index 0: label baku 02 - Kantor Kas.
	if got := sec.Rows[0].Cells[0].Value; got != "02 - Kantor Kas" {
		t.Fatalf("kolom I = %q, ingin '02 - Kantor Kas'", got)
	}
}

// TestBuildForm00_11TidakAdaBarisJumlah memastikan form ini tidak punya baris JUMLAH.
func TestBuildForm00_11TidakAdaBarisJumlah(t *testing.T) {
	sec := BuildForm00_11([]domain.BankOffice{kantor0011("02", "101"), kantor0011("07", "201")})
	if len(sec.Rows) != 2 {
		t.Fatalf("ingin 2 baris, dapat %d", len(sec.Rows))
	}
	for _, r := range sec.Rows {
		if strings.EqualFold(r.Key, "JUMLAH") {
			t.Fatal("Form 00.11 tidak boleh memiliki baris JUMLAH")
		}
	}
}

// TestBuildForm00_11KendaliHanyaWilayahDanSKK memastikan kolom XII hanya diisi untuk
// Kantor Wilayah (07) dan Sentra Keuangan Khusus (08); jenis lain ditulis "-" (PDF #274).
func TestBuildForm00_11KendaliHanyaWilayahDanSKK(t *testing.T) {
	kas := kantor0011("02", "101")
	kas.ControlOfficeCode = "999"
	wilayah := kantor0011("07", "201")
	wilayah.ControlOfficeCode = "888"
	sec := BuildForm00_11([]domain.BankOffice{kas, wilayah})
	// Kolom XII index 11 (I..XI = 0..10).
	if got := sec.Rows[0].Cells[11].Value; got != "-" {
		t.Fatalf("kolom XII untuk kantor kas harus '-', dapat %q", got)
	}
	if got := sec.Rows[1].Cells[11].Value; got != "888" {
		t.Fatalf("kolom XII untuk kantor wilayah harus '888', dapat %q", got)
	}
}

// TestBuildForm00_11JenisLainnyaDanTanggal memastikan sandi 99 punya label dan tanggal nil
// ditulis "-".
func TestBuildForm00_11JenisLainnyaDanTanggal(t *testing.T) {
	o := kantor0011("99", "301")
	impl := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	o.ImplementationDate = &impl
	sec := BuildForm00_11([]domain.BankOffice{o})
	if got := sec.Rows[0].Cells[0].Value; got != "99 - Lainnya" {
		t.Fatalf("kolom I = %q, ingin '99 - Lainnya'", got)
	}
	// Kolom XI index 10.
	if got := sec.Rows[0].Cells[10].Value; got != "2026-07-01" {
		t.Fatalf("kolom XI = %q, ingin 2026-07-01", got)
	}
	// Kolom XIII index 12 (tanggal persetujuan kosong).
	if got := sec.Rows[0].Cells[12].Value; got != "-" {
		t.Fatalf("kolom XIII kosong harus '-', dapat %q", got)
	}
}

// TestBuildForm00_11KosongTanpaKantorTersandi memastikan tanpa kantor tersandi, Rows kosong
// (builder mencatatnya pada SkippedForms).
func TestBuildForm00_11KosongTanpaKantorTersandi(t *testing.T) {
	sec := BuildForm00_11(nil)
	if len(sec.Rows) != 0 {
		t.Fatalf("ingin 0 baris, dapat %d", len(sec.Rows))
	}
}
