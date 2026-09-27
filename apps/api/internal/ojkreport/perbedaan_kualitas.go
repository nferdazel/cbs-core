package ojkreport

import (
	"context"
	"errors"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// perbedaan_kualitas.go merakit Laporan Perbedaan Kualitas Aset Produktif
// (Form 19.00 SEOJK No. 16/SEOJK.03/2024) dari baris kredit.
//
// BATAS SUMBER (jangan diisi dengan tebakan):
//   - Kolom I-X "Pada BPR Bersangkutan" dapat dibangun dari baris kredit: kantor, ID
//     pihak lawan (CIF), nomor rekening, jenis penggunaan, plafon, baki debet, tanggal
//     mulai/jatuh tempo, dan golongan kualitas tersimpan.
//   - Kolom XI-XIX "Pada BPR Lain" TIDAK dapat dibangun: laporan resmi membandingkan
//     aset produktif debitur yang sama pada BPR pelapor dengan BPR lain, sedangkan data
//     BPR lain tidak ada di sistem (hanya diterima lewat APOLO/SPOJK). Bagian ini
//     didaftarkan sebagai kolom belum tersedia, bukan dikosongkan diam-diam.
//   - Perbandingan kualitas komersial vs PPKA per debitur juga belum punya dasar
//     tersimpan: modul PPAP menghitung golongan dari DPD/maturitas lalu menimpanya ke
//     loans.collectibility (sumber kolom X), sehingga tidak ada golongan PPKA terpisah
//     yang dapat dibandingkan. Yang tersimpan dari PPAP adalah nominal target cadangan
//     (loans.required_ppap), dan itu bukan golongan kualitas.
//
// Jenis aset produktif lain (surat berharga, penempatan pada bank lain, penyertaan
// modal) belum dimodelkan pada laporan ini.

const (
	perbedaanSandiKantor        = "I"
	perbedaanSandiIDPihakLawan  = "II"
	perbedaanSandiJenisAset     = "III"
	perbedaanSandiNoRekening    = "IV"
	perbedaanSandiJenisGuna     = "V"
	perbedaanSandiPlafon        = "VI"
	perbedaanSandiBakiDebet     = "VII"
	perbedaanSandiTanggalMulai  = "VIII"
	perbedaanSandiTanggalJatuh  = "IX"
	perbedaanSandiKualitas      = "X"
	perbedaanSandiPPKA          = "PPKA"
	perbedaanSandiBPRLain       = "XI-XIX"
	perbedaanSandiKualitasPPKA  = "KUALITAS_PPKA"
	perbedaanJenisAsetKredit    = "3"
	perbedaanKodeAturanPPKANote = "modul PPAP menghitung golongan kualitas dari DPD/maturitas lalu menimpanya ke loans.collectibility (sumber kolom X); tidak ada golongan PPKA tersimpan terpisah per kredit, sehingga perbandingan kualitas komersial vs PPKA tidak dapat dibentuk. Yang tersimpan dari PPAP hanya nominal target cadangan (loans.required_ppap), dan itu bukan golongan kualitas"
)

// GeneratePerbedaanKualitas menyusun Bundle berisi satu tabel Laporan Perbedaan
// Kualitas Aset Produktif untuk posisi asOf. Sumber kredit wajib tersedia; tanpa itu
// ekspor ditolak, bukan menghasilkan berkas kosong yang tampak sah.
func GeneratePerbedaanKualitas(ctx context.Context, source LoanDataSource, asOf time.Time, actor domain.Actor) (*Bundle, error) {
	if source == nil {
		return nil, errors.New("sumber data kredit belum dikonfigurasi untuk Laporan Perbedaan Kualitas")
	}
	rows, err := source.ListLoansForOJK(ctx, asOf, actor)
	if err != nil {
		return nil, err
	}
	def := reportByCode("LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF")
	periodStart := time.Date(asOf.Year(), asOf.Month(), 1, 0, 0, 0, 0, time.UTC)
	return &Bundle{
		Period:             periodStart,
		PeriodEnd:          MonthEnd(asOf),
		GeneratedAt:        time.Now().UTC(),
		Deadline:           def.Deadline(periodStart),
		CorrectionDeadline: def.CorrectionDeadline(periodStart),
		MappingStatus:      MappingStatus,
		Tables:             []TableSection{BuildPerbedaanKualitasTable(rows)},
	}, nil
}

// BuildPerbedaanKualitasTable menyusun tabel Form 19.00 dari baris kredit. Fungsi ini
// murni sehingga dapat diuji tanpa basis data.
func BuildPerbedaanKualitasTable(rows []LoanRow) TableSection {
	sec := TableSection{
		Form:     "LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF",
		Name:     reportByCode("LAPORAN_PERBEDAAN_KUALITAS_ASET_PRODUKTIF").Name,
		KeyLabel: "No. Rekening",
		Columns: []TableColumn{
			{Sandi: perbedaanSandiKantor, Nama: "Sandi Kantor"},
			{Sandi: perbedaanSandiIDPihakLawan, Nama: "ID Pihak Lawan"},
			{Sandi: perbedaanSandiJenisAset, Nama: "Jenis Aset Produktif"},
			{Sandi: perbedaanSandiNoRekening, Nama: "Nomor Rekening"},
			{Sandi: perbedaanSandiJenisGuna, Nama: "Jenis Penggunaan"},
			{Sandi: perbedaanSandiPlafon, Nama: "Plafon/Nominal"},
			{Sandi: perbedaanSandiBakiDebet, Nama: "Baki Debet"},
			{Sandi: perbedaanSandiTanggalMulai, Nama: "Tanggal Mulai"},
			{Sandi: perbedaanSandiTanggalJatuh, Nama: "Tanggal Jatuh Tempo"},
			{Sandi: perbedaanSandiKualitas, Nama: "Kualitas"},
			{Sandi: perbedaanSandiPPKA, Nama: "PPKA yang Telah Dibentuk (keluaran PPAP)"},
		},
		Unavailable: []ColumnUnavailable{
			{Sandi: perbedaanSandiBPRLain, Nama: "Aset Produktif pada BPR Lain (Jenis Aset, Sandi BPR Lain, Jenis Penggunaan, Plafon, Baki Debet, Tanggal Mulai, Tanggal Jatuh Tempo, Kualitas, Keterangan)",
				Reason: "laporan resmi membandingkan aset produktif debitur yang sama pada BPR pelapor dengan BPR lain; data BPR lain tidak ada di sistem dan hanya diterima lewat APOLO/SPOJK"},
			{Sandi: perbedaanSandiKualitasPPKA, Nama: "Kualitas menurut PPKA/regulasi (pembanding)",
				Reason: perbedaanKodeAturanPPKANote},
		},
		Notes: []string{
			"Kolom I-X \"Pada BPR Bersangkutan\" hanya memuat aset produktif berupa Kredit (kolom III diisi sandi 3). Surat berharga, penempatan pada bank lain, dan penyertaan modal belum dimodelkan pada laporan ini.",
			"Kolom X Kualitas adalah golongan yang tersimpan pada kredit (loans.collectibility), sandi 1 Lancar sampai 5 Macet.",
			"Kolom tanggal jatuh tempo (IX) diisi tanggal mulai bila kredit tidak punya tanggal jatuh tempo, sesuai penjelasan Form 19.00-3.",
			"Akumulasi kualitas & nominal dibaca dari keadaan kredit saat ekspor dijalankan; sistem belum menyimpan riwayat posisi kredit per akhir bulan.",
		},
	}

	for _, r := range rows {
		if !aktifUntukOJK(r.Status) {
			continue
		}
		row := TableRow{Key: dashIfEmpty(r.LoanNumber)}
		row.Cells = append(row.Cells,
			TableCell{Sandi: perbedaanSandiKantor, Nama: "Sandi Kantor", Value: dashIfEmpty(r.BranchCode)},
			TableCell{Sandi: perbedaanSandiIDPihakLawan, Nama: "ID Pihak Lawan", Value: dashIfEmpty(r.IDPihakLawan)},
			TableCell{Sandi: perbedaanSandiJenisAset, Nama: "Jenis Aset Produktif", Value: perbedaanJenisAsetKredit},
			TableCell{Sandi: perbedaanSandiNoRekening, Nama: "Nomor Rekening", Value: dashIfEmpty(r.LoanNumber)},
			TableCell{Sandi: perbedaanSandiJenisGuna, Nama: "Jenis Penggunaan", Value: dashIfEmpty(r.OJKJenisPenggunaanCode)},
			TableCell{Sandi: perbedaanSandiPlafon, Nama: "Plafon/Nominal", Value: FormatRupiah(r.PrincipalAmount)},
			TableCell{Sandi: perbedaanSandiBakiDebet, Nama: "Baki Debet", Value: FormatRupiah(r.Outstanding)},
			TableCell{Sandi: perbedaanSandiTanggalMulai, Nama: "Tanggal Mulai", Value: formatTanggalAtauDash(r.AkadDate)},
			TableCell{Sandi: perbedaanSandiTanggalJatuh, Nama: "Tanggal Jatuh Tempo", Value: formatTanggalPerbedaanKualitas(r)},
			TableCell{Sandi: perbedaanSandiKualitas, Nama: "Kualitas", Value: sandiKualitasKredit(r.Collectibility)},
			TableCell{Sandi: perbedaanSandiPPKA, Nama: "PPKA yang Telah Dibentuk (keluaran PPAP)", Value: FormatRupiah(r.RequiredPPAP)},
		)
		sec.Rows = append(sec.Rows, row)
	}
	return sec
}

// formatTanggalPerbedaanKualitas mengikuti penjelasan Form 19.00-3 kolom IX: tanggal
// jatuh tempo diisi tanggal mulai bila aset produktif tidak memiliki tanggal jatuh
// tempo.
func formatTanggalPerbedaanKualitas(r LoanRow) string {
	if r.FinalDueDate != nil {
		return r.FinalDueDate.Format("2006-01-02")
	}
	return formatTanggalAtauDash(r.AkadDate)
}

// formatTanggalAtauDash menulis tanggal ISO atau "-" bila tidak tersedia.
func formatTanggalAtauDash(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Format("2006-01-02")
}
