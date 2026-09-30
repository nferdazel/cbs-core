package ojkreport

import (
	"context"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// HapusBukuOJKSource menyediakan baris register hapus buku untuk Form 15.00. Kontraknya
// opsional pada perakitan RepoSource: tanpa sumber ini Form 15.00 dinyatakan belum
// tersedia, bukan ditulis kosong.
type HapusBukuOJKSource interface {
	ListHapusBukuForOJK(ctx context.Context) ([]domain.HapusBukuItem, error)
}

// form15_00.go membangun Form 15.00 "DAFTAR ASET PRODUKTIF YANG DIHAPUS BUKU" dari
// register hapus_buku_register (migrasi 000129).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Susunan PDF #page 218 (hlm. tercetak 166) kolom I-VI, PDF #page 219 (hlm. 167)
//     kolom VII-X + baris JUMLAH, sandi PDF #page 220 (hlm. 168), penjelasan PDF
//     #page 221-222 (hlm. 169-170).
//   - Sepuluh kolom logis; VIII Saldo Pokok (3 sub) dan IX Tunggakan Bunga (4 sub);
//     X Agunan (3 sub). Sandi IV 10/20; VI 11/12/20; X.a Lampiran 01 (299 = tanpa agunan).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH kolom angka adalah ISIAN BANK; form tidak memberi rumus pengikat, sehingga
//     laporan hanya menjumlahkan baris untuk kaki JUMLAH, tidak menurunkan nilai.
//   - Form memuat kredit MAUPUN penempatan pada bank lain; keduanya baris register yang
//     bank isi, bukan hasil turunan dari loans (yang tidak menyimpan dimensi ini).
//   - Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice, bukan dari register.
//   - Tanpa baris register, form dinyatakan belum tersedia, bukan ditulis kosong.

// form15_00TotalKey adalah kunci baris kaki tabel JUMLAH.
const form15_00TotalKey = "JUMLAH"

// BuildForm15_00 menyusun tabel Form 15.00 dari baris register hapus buku dan kantor
// pelapor kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Form ini punya
// baris JUMLAH yang menjumlahkan kolom nominal (VIII, IX, X Nilai).
func BuildForm15_00(rows []domain.HapusBukuItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "15.00",
		Name:     formName("15.00"),
		KeyLabel: "No. Rekening / ID Pihak Lawan",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "ID Pihak Lawan/Sandi Bank"},
			{Sandi: "III", Nama: "No. Rekening"},
			{Sandi: "IV", Nama: "Jenis Aset"},
			{Sandi: "V", Nama: "Jenis Debitur"},
			{Sandi: "VI", Nama: "Hubungan dengan Bank"},
			{Sandi: "VII", Nama: "Tanggal Hapus Buku"},
			{Sandi: "VIII", Nama: "Saldo Pokok Saat Hapus Buku"},
			{Sandi: "VIII", Nama: "Saldo Pokok Akumulasi Tertagih"},
			{Sandi: "VIII", Nama: "Saldo Pokok Per Posisi Laporan"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Saat Hapus Buku"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Akumulasi Tertagih"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Akumulasi Tambahan Bunga Berjalan"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Per Posisi Laporan"},
			{Sandi: "X", Nama: "Jenis Agunan"},
			{Sandi: "X", Nama: "Alamat Agunan"},
			{Sandi: "X", Nama: "Nilai Agunan"},
		},
		Notes: []string{
			"Baris per aset produktif yang dihapus buku dari register hapus_buku_register yang bank isi (Form 15.00, PDF #page 218-222); dengan baris JUMLAH.",
			"Memuat kredit yang diberikan (Jenis Aset 10) maupun penempatan pada bank lain (20) yang telah dihapus buku, TIDAK termasuk yang sudah lunas atau dihapus tagih (PDF #page 221).",
			"Jenis Aset (IV) memakai sandi baku PDF #page 220: 10 Kredit yang Diberikan, 20 Penempatan pada Bank Lain.",
			"Jenis Debitur (V) hanya bermakna untuk kredit (jenis aset 10) dan mengacu Lampiran 02; untuk penempatan dibiarkan kosong dan ditulis \"-\".",
			"Hubungan dengan Bank (VI) memakai sandi baku PDF #page 220: 11 Terkait Dalam Rangka Kesejahteraan, 12 Terkait Lainnya, 20 Tidak Terkait.",
			"Saldo Pokok (VIII) dan Tunggakan Bunga (IX) adalah ISIAN BANK: nilai saat hapus buku, akumulasi tertagih, tambahan bunga berjalan, dan posisi laporan. Laporan hanya menjumlahkan untuk kaki JUMLAH, tidak menurunkan nilainya.",
			"Agunan (X): jenis mengacu Lampiran 01 dan tanpa agunan diserahkan diisi 299 dengan Nilai 0, serta alamat tanda hubung \"-\" (PDF #page 222). Nilai agunan adalah nominal nilai pasar hasil penilaian terakhir.",
			"Baris JUMLAH menjumlahkan kolom VIII.a/b/c, IX.a/b/c/d, dan X Nilai; kolom sandi/tanggal/teks pada baris JUMLAH ditulis \"-\" karena penjumlahan sandi tidak bermakna.",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	var (
		totalPokokHapus, totalPokokTertagih, totalPokokPosisi                     decimal.Decimal
		totalBungaHapus, totalBungaTertagih, totalBungaTambahan, totalBungaPosisi decimal.Decimal
		totalAgunan                                                               decimal.Decimal
	)
	for _, item := range rows {
		row := TableRow{Key: form15_00RowKey(item)}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "ID Pihak Lawan/Sandi Bank", Value: dashIfEmpty(item.PihakLawanID)},
			{Sandi: "III", Nama: "No. Rekening", Value: dashIfEmpty(item.NomorRekening)},
			{Sandi: "IV", Nama: "Jenis Aset", Value: form15_00JenisAsetLabel(item.JenisAsetCode)},
			{Sandi: "V", Nama: "Jenis Debitur", Value: dashIfEmpty(item.JenisDebiturCode)},
			{Sandi: "VI", Nama: "Hubungan dengan Bank", Value: form15_00HubunganLabel(item.HubunganBankCode)},
			{Sandi: "VII", Nama: "Tanggal Hapus Buku", Value: item.TanggalHapusBuku.Format("2006-01-02")},
			{Sandi: "VIII", Nama: "Saldo Pokok Saat Hapus Buku", Value: FormatRupiah(item.SaldoPokokSaatHapus)},
			{Sandi: "VIII", Nama: "Saldo Pokok Akumulasi Tertagih", Value: FormatRupiah(item.SaldoPokokAkumTertagih)},
			{Sandi: "VIII", Nama: "Saldo Pokok Per Posisi Laporan", Value: FormatRupiah(item.SaldoPokokPerPosisi)},
			{Sandi: "IX", Nama: "Tunggakan Bunga Saat Hapus Buku", Value: FormatRupiah(item.BungaSaatHapus)},
			{Sandi: "IX", Nama: "Tunggakan Bunga Akumulasi Tertagih", Value: FormatRupiah(item.BungaAkumTertagih)},
			{Sandi: "IX", Nama: "Tunggakan Bunga Akumulasi Tambahan Bunga Berjalan", Value: FormatRupiah(item.BungaAkumTambahan)},
			{Sandi: "IX", Nama: "Tunggakan Bunga Per Posisi Laporan", Value: FormatRupiah(item.BungaPerPosisi)},
			{Sandi: "X", Nama: "Jenis Agunan", Value: dashIfEmpty(item.AgunanJenisCode)},
			{Sandi: "X", Nama: "Alamat Agunan", Value: dashIfEmpty(item.AgunanAlamat)},
			{Sandi: "X", Nama: "Nilai Agunan", Value: FormatRupiah(item.AgunanNilai)},
		}
		totalPokokHapus = totalPokokHapus.Add(item.SaldoPokokSaatHapus)
		totalPokokTertagih = totalPokokTertagih.Add(item.SaldoPokokAkumTertagih)
		totalPokokPosisi = totalPokokPosisi.Add(item.SaldoPokokPerPosisi)
		totalBungaHapus = totalBungaHapus.Add(item.BungaSaatHapus)
		totalBungaTertagih = totalBungaTertagih.Add(item.BungaAkumTertagih)
		totalBungaTambahan = totalBungaTambahan.Add(item.BungaAkumTambahan)
		totalBungaPosisi = totalBungaPosisi.Add(item.BungaPerPosisi)
		totalAgunan = totalAgunan.Add(item.AgunanNilai)
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form15_00TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "ID Pihak Lawan/Sandi Bank", Value: "-"},
			{Sandi: "III", Nama: "No. Rekening", Value: "JUMLAH"},
			{Sandi: "IV", Nama: "Jenis Aset", Value: "-"},
			{Sandi: "V", Nama: "Jenis Debitur", Value: "-"},
			{Sandi: "VI", Nama: "Hubungan dengan Bank", Value: "-"},
			{Sandi: "VII", Nama: "Tanggal Hapus Buku", Value: "-"},
			{Sandi: "VIII", Nama: "Saldo Pokok Saat Hapus Buku", Value: "-"},
			{Sandi: "VIII", Nama: "Saldo Pokok Akumulasi Tertagih", Value: "-"},
			{Sandi: "VIII", Nama: "Saldo Pokok Per Posisi Laporan", Value: "-"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Saat Hapus Buku", Value: "-"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Akumulasi Tertagih", Value: "-"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Akumulasi Tambahan Bunga Berjalan", Value: "-"},
			{Sandi: "IX", Nama: "Tunggakan Bunga Per Posisi Laporan", Value: "-"},
			{Sandi: "X", Nama: "Jenis Agunan", Value: "-"},
			{Sandi: "X", Nama: "Alamat Agunan", Value: "-"},
			{Sandi: "X", Nama: "Nilai Agunan", Value: "-"},
		},
	}
	if len(rows) > 0 {
		totalRow.Cells[6].Value = FormatRupiah(totalPokokHapus)
		totalRow.Cells[7].Value = FormatRupiah(totalPokokTertagih)
		totalRow.Cells[8].Value = FormatRupiah(totalPokokPosisi)
		totalRow.Cells[9].Value = FormatRupiah(totalBungaHapus)
		totalRow.Cells[10].Value = FormatRupiah(totalBungaTertagih)
		totalRow.Cells[11].Value = FormatRupiah(totalBungaTambahan)
		totalRow.Cells[12].Value = FormatRupiah(totalBungaPosisi)
		totalRow.Cells[15].Value = FormatRupiah(totalAgunan)
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form15_00RowKey mengidentifikasi satu baris Form 15.00 secara terbaca.
func form15_00RowKey(item domain.HapusBukuItem) string {
	if item.NomorRekening != "" {
		return item.NomorRekening
	}
	if item.PihakLawanID != "" {
		return item.PihakLawanID
	}
	return "hapus-buku:" + item.ID.String()
}

// form15_00JenisAsetLabel menulis sandi IV sebagai label baku (PDF #220).
func form15_00JenisAsetLabel(code string) string {
	switch code {
	case domain.HapusBukuJenisAsetKredit:
		return "10 - Kredit yang Diberikan"
	case domain.HapusBukuJenisAsetPenempatan:
		return "20 - Penempatan pada Bank Lain"
	default:
		return dashIfEmpty(code)
	}
}

// form15_00HubunganLabel menulis sandi VI sebagai label baku (PDF #220).
func form15_00HubunganLabel(code string) string {
	switch code {
	case domain.HapusBukuHubunganKesejahteraan:
		return "11 - Terkait Dalam Rangka Kesejahteraan"
	case domain.HapusBukuHubunganTerkaitLain:
		return "12 - Terkait Lainnya"
	case domain.HapusBukuHubunganTidakTerkait:
		return "20 - Tidak Terkait"
	default:
		return dashIfEmpty(code)
	}
}
