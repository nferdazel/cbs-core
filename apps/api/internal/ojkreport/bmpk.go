package ojkreport

import (
	"context"
	"errors"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// bmpk.go merakit Laporan BMPK (Batas Maksimum Pemberian Kredit) dari keluaran modul
// BMPK (domain.BMPKService): paparan per pihak terkait beserta uji batasnya.
//
// BATAS SUMBER:
//   - Nomor/sandi kolom resmi Laporan BMPK (Lampiran SEOJK) belum tersedia di repo ini,
//     sehingga baris memakai penamaan kolom INTERNAL yang eksplisit. Pemetaan ke sandi
//     resmi dicatat sebagai kolom belum tersedia, bukan dikarang.
//   - Batas BMPK sebagai persentase modal belum diputuskan; karena itu laporan memakai
//     batas nominal per pihak (bmpk_limits) yang diisi bank. Kolom persentase modal
//     dinyatakan belum tersedia.
//
// Tidak ada angka yang ditebak: pihak tanpa batas ditulis "-" pada kolom batas dan
// berstatus BATAS_BELUM_DISET.

const (
	// bmpkSandiID menamai kolom kunci baris (CIF internal), bukan sandi OJK.
	bmpkSandiID         = "CIF"
	bmpkSandiNama       = "NAMA"
	bmpkSandiHubungan   = "HUBUNGAN"
	bmpkSandiKredit     = "KREDIT"
	bmpkSandiPenempatan = "PENEMPATAN"
	bmpkSandiTotal      = "TOTAL"
	bmpkSandiBatas      = "BATAS"
	bmpkSandiStatus     = "STATUS"
	bmpkSandiAlasan     = "ALASAN"
)

// BMPKSource menyediakan laporan BMPK terhitung. Kontraknya opsional pada perakitan
// ekspor: tanpa sumber ini laporan BMPK belum dapat dibangun.
type BMPKSource interface {
	BMPKReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.BMPKReport, error)
}

// GenerateBMPK menyusun Bundle berisi satu tabel Laporan BMPK untuk posisi asOf. Bundle
// dapat ditulis dengan WriteText seperti laporan lain. Sumber wajib tersedia; tanpa itu
// ekspor ditolak, bukan menghasilkan berkas kosong yang tampak sah.
func GenerateBMPK(ctx context.Context, source BMPKSource, asOf time.Time, actor domain.Actor) (*Bundle, error) {
	if source == nil {
		return nil, errors.New("sumber laporan BMPK belum dikonfigurasi")
	}
	report, err := source.BMPKReport(ctx, asOf, actor)
	if err != nil {
		return nil, err
	}
	def := reportByCode("LAPORAN_BMPK")
	periodStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	return &Bundle{
		Period:             periodStart,
		PeriodEnd:          MonthEnd(asOf),
		GeneratedAt:        time.Now().UTC(),
		Deadline:           def.Deadline(periodStart),
		CorrectionDeadline: def.CorrectionDeadline(periodStart),
		MappingStatus:      MappingStatus,
		Tables:             []TableSection{BuildBMPKTable(report)},
	}, nil
}

// BuildBMPKTable menyusun bagian tabel Laporan BMPK dari keluaran modul BMPK. Fungsi ini
// murni sehingga dapat diuji tanpa basis data.
func BuildBMPKTable(report domain.BMPKReport) TableSection {
	sec := TableSection{
		Form:     "LAPORAN_BMPK",
		Name:     reportByCode("LAPORAN_BMPK").Name,
		KeyLabel: "Pihak Terkait",
		Columns: []TableColumn{
			{Sandi: bmpkSandiID, Nama: "ID Pihak Terkait (CIF internal)"},
			{Sandi: bmpkSandiNama, Nama: "Nama Pihak Terkait"},
			{Sandi: bmpkSandiHubungan, Nama: "Jenis Hubungan"},
			{Sandi: bmpkSandiKredit, Nama: "Paparan Kredit"},
			{Sandi: bmpkSandiPenempatan, Nama: "Paparan Penempatan pada Bank Lain"},
			{Sandi: bmpkSandiTotal, Nama: "Total Paparan"},
			{Sandi: bmpkSandiBatas, Nama: "Batas BMPK"},
			{Sandi: bmpkSandiStatus, Nama: "Status Batas"},
			{Sandi: bmpkSandiAlasan, Nama: "Alasan Status"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: "-", Nama: "Nomor/sandi kolom resmi Laporan BMPK",
				Reason: "format resmi kolom Laporan BMPK (Lampiran SEOJK) belum tersedia di repo ini; baris memakai penamaan kolom internal agar angka tetap dapat diperiksa"},
			{Sandi: "MODAL", Nama: "Persentase terhadap Modal",
				Reason: "batas BMPK sebagai persentase modal menunggu keputusan bank/OJK: basis modal (KPMM) dan rujukan persentasenya belum ada di repo"},
		},
		Notes: []string{
			"Paparan per pihak terkait = baki debet pokok kredit berstatus DISBURSED/DEFAULTED + outstanding penempatan pada bank lain yang ditautkan ke nasabah (lps_placements.customer_id, migrasi 000103). Penempatan tanpa tautan nasabah tidak ikut dihitung.",
			"Batas per pihak diambil dari bmpk_limits (nominal rupiah) yang diisi bank; bila barisnya belum ada, kolom Batas ditulis '-' dan statusnya BATAS_BELUM_DISET (bukan dianggap nol atau sesuai batas).",
			"Nama pihak terkait dibaca dari layanan nasabah dan ditulis '-' bila belum dapat dibaca; laporan tidak menebak nama dari data lain.",
		},
	}

	// Peringatan dari modul BMPK (mis. saklar bmpk.enabled mati, batas belum diset)
	// dibawa apa adanya ke catatan laporan.
	sec.Notes = append(sec.Notes, report.Warnings...)

	for _, row := range report.Rows {
		tableRow := TableRow{Key: bmpkRowKey(row)}
		tableRow.Cells = append(tableRow.Cells,
			TableCell{Sandi: bmpkSandiID, Nama: "ID Pihak Terkait (CIF internal)", Value: dashIfEmpty(row.CustomerID.String())},
			TableCell{Sandi: bmpkSandiNama, Nama: "Nama Pihak Terkait", Value: dashIfEmpty(row.CustomerName)},
			TableCell{Sandi: bmpkSandiHubungan, Nama: "Jenis Hubungan", Value: dashIfEmpty(row.RelationshipType)},
			TableCell{Sandi: bmpkSandiKredit, Nama: "Paparan Kredit", Value: FormatRupiah(row.LoanExposure)},
			TableCell{Sandi: bmpkSandiPenempatan, Nama: "Paparan Penempatan pada Bank Lain", Value: FormatRupiah(row.PlacementExposure)},
			TableCell{Sandi: bmpkSandiTotal, Nama: "Total Paparan", Value: FormatRupiah(row.TotalExposure)},
			TableCell{Sandi: bmpkSandiBatas, Nama: "Batas BMPK", Value: bmpkBatasValue(row)},
			TableCell{Sandi: bmpkSandiStatus, Nama: "Status Batas", Value: dashIfEmpty(row.Status)},
			TableCell{Sandi: bmpkSandiAlasan, Nama: "Alasan Status", Value: dashIfEmpty(row.StatusReason)},
		)
		sec.Rows = append(sec.Rows, tableRow)
	}
	return sec
}

// bmpkRowKey memakai nama pihak bila tersedia, jika tidak id-nya. Kunci tidak pernah
// kosong agar baris tetap dapat diacak ke pemiliknya.
func bmpkRowKey(row domain.BMPKPartyCheck) string {
	if row.CustomerName != "" {
		return row.CustomerName
	}
	return row.CustomerID.String()
}

// bmpkBatasValue menulis "-" bila batas belum diset, bukan 0: nol berarti batas nol
// (tidak boleh ada eksposur), sedangkan tidak tersedia berarti bank belum mengisi.
func bmpkBatasValue(row domain.BMPKPartyCheck) string {
	if !row.HasLimit {
		return "-"
	}
	return FormatRupiah(row.LimitAmount)
}
