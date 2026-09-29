package ojkreport

import (
	"context"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form00_07.go membangun Form 00.07 "DAFTAR PINJAMAN YANG DITERIMA" dari register
// pinjaman_diterima_register (migrasi 000117).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 00.07 - 1 "DAFTAR PINJAMAN YANG DITERIMA": PDF #page 248-249 (hlm.
//     tercetak 196-197). Lima belas kolom: I ID Pihak Lawan, II Gol. Kreditur,
//     III Sandi Bank, IV Lokasi Kreditur, V Jenis, VI Hubungan dengan Bank,
//     VII Jangka Waktu (Tanggal Mulai + Tanggal Jatuh Tempo), VIII Suku Bunga
//     (Persentase + Cara Perhitungan), IX Plafon, X Jenis Agunan yang Dijaminkan,
//     XI Nominal Agunan yang Dijaminkan, XII Baki Debet,
//     XIII Biaya Transaksi Belum Diamortisasi, XIV Diskonto Belum Diamortisasi,
//     XV Baki Debet Neto. Ada baris JUMLAH.
//   - Sandi: PDF #page 250 (hlm. 198). V 10/20/31/32/41/42/99; VI 12/20;
//     VIII 11/12/21/22.
//   - Penjelasan: PDF #page 252-254 (hlm. 200-202). X tanpa agunan = 299;
//     XI tanpa agunan = 0; XV = baki debet - (diskonto + biaya transaksi belum
//     diamortisasi).
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH angka I-XIV adalah isian bank dari register. Sistem tidak menghitung
//     plafon maupun baki debet.
//   - Kolom XV "Baki Debet Neto" adalah turunan: baki debet (XII) dikurangi diskonto
//     (XIV) dan biaya transaksi belum diamortisasi (XIII), mengikuti
//     domain.PinjamanItem.BakiDebetNeto. Bila hasilnya negatif, kolom XV ditulis "-"
//     beserta alasannya, bukan angka negatif.
//   - Baris JUMLAH kaki tabel hanya diisi bila SELURUH baris punya nilai kolom XV;
//     bila ada yang tidak, JUMLAH ditulis "-" dengan alasan karena total sebagian akan
//     menyesatkan (pola Form 07.00/14.00).
//   - Form ini TIDAK punya kolom Sandi Kantor: kolom I-nya ID Pihak Lawan, sehingga
//     tidak ada kantor pelapor yang perlu dipilih. Kolom III Sandi Bank adalah sandi
//     bank kreditur (6 digit SPOJK), bukan kantor pelapor.
//   - Kolom II Gol. Kreditur dan IV Lokasi Kreditur memakai sandi Lampiran 02/03 yang
//     diverifikasi basis data lewat FK (ojk_pihak_lawan/ojk_kabupaten, migrasi 000102).
//     Sandi X Jenis Agunan (Lampiran 01) belum punya tabel referensi; laporan menulis
//     sandi bank apa adanya tanpa mengarang daftar.

// PinjamanRegisterSource menyediakan baris register pinjaman yang diterima bank-wide
// untuk Form 00.07. Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini
// Form 00.07 dinyatakan belum tersedia, bukan ditulis kosong.
type PinjamanRegisterSource interface {
	ListPinjamanForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.PinjamanItem, error)
}

// form00_07TotalKey adalah kunci baris kaki tabel JUMLAH; bukan sandi pos OJK.
const form00_07TotalKey = "JUMLAH"

// BuildForm00_07 menyusun tabel Form 00.07 dari baris register pinjaman. Fungsi ini
// murni sehingga dapat diuji tanpa basis data. Bila tidak ada baris, Rows kosong
// tetapi daftar kolom tetap dibawa.
func BuildForm00_07(rows []domain.PinjamanItem) TableSection {
	sec := TableSection{
		Form:     "00.07",
		Name:     formName("00.07"),
		KeyLabel: "ID Pihak Lawan",
		Columns: []TableColumn{
			{Sandi: "I", Nama: "ID Pihak Lawan"},
			{Sandi: "II", Nama: "Gol. Kreditur"},
			{Sandi: "III", Nama: "Sandi Bank"},
			{Sandi: "IV", Nama: "Lokasi Kreditur"},
			{Sandi: "V", Nama: "Jenis"},
			{Sandi: "VI", Nama: "Hubungan dengan Bank"},
			{Sandi: "VII.1", Nama: "Tanggal Mulai"},
			{Sandi: "VII.2", Nama: "Tanggal Jatuh Tempo"},
			{Sandi: "VIII.1", Nama: "Suku Bunga (%)"},
			{Sandi: "VIII.2", Nama: "Cara Perhitungan"},
			{Sandi: "IX", Nama: "Plafon"},
			{Sandi: "X", Nama: "Jenis Agunan yang Dijaminkan"},
			{Sandi: "XI", Nama: "Nominal Agunan yang Dijaminkan"},
			{Sandi: "XII", Nama: "Baki Debet"},
			{Sandi: "XIII", Nama: "Biaya Transaksi Belum Diamortisasi"},
			{Sandi: "XIV", Nama: "Diskonto Belum Diamortisasi"},
			{Sandi: "XV", Nama: "Baki Debet Neto"},
		},
		Notes: []string{
			"Baris per pinjaman yang diterima dari bank, Bank Indonesia, dan/atau pihak ketiga bukan bank dengan kewajiban pembayaran kembali berdasarkan persyaratan perjanjian utang piutang (Form 00.07, PDF #page 248-249, 252); satu baris = satu pinjaman dan ada baris JUMLAH.",
			"Jenis (V) mengikuti sandi PDF #page 250: 10 Bilateral, 20 Sindikasi, 31 Khusus dari Lembaga Pengayom, 32 Khusus Linkage, 41 Tertentu modal inti tambahan, 42 Tertentu modal pelengkap, 99 Lainnya. Hubungan dengan Bank (VI): 12 Terkait, 20 Tidak Terkait. Cara Perhitungan (VIII): 11 flat tetap, 12 flat mengambang, 21 tidak flat tetap, 22 tidak flat mengambang.",
			"Baki Debet Neto (XV) dihitung sebagai Baki Debet (XII) dikurangi Biaya Transaksi Belum Diamortisasi (XIII) dan Diskonto Belum Diamortisasi (XIV) (PDF #page 254). Bila hasilnya negatif, kolom XV ditulis \"-\" beserta alasannya, bukan angka negatif.",
			"Pinjaman tanpa agunan mengikuti aturan PDF #page 253: jenis agunan (X) diisi sandi 299 dan nominal agunan (XI) diisi 0. Aturan itu ditegakkan saat pengisian register, bukan ditambal laporan.",
			"Gol. Kreditur (II) dan Lokasi Kreditur (IV) memakai sandi Lampiran 02/03 yang diverifikasi basis data lewat FK ke ojk_pihak_lawan/ojk_kabupaten (migrasi 000102). Sandi Jenis Agunan (X) Lampiran 01 belum punya tabel referensi; ditulis apa adanya tanpa daftar karangan.",
			"Baris JUMLAH hanya diisi bila seluruh baris punya nilai Baki Debet Neto (XV); bila tidak, JUMLAH ditulis \"-\" karena total sebagian akan menyesatkan.",
			"Form ini tidak memakai kolom Sandi Kantor: kolom I-nya ID Pihak Lawan. Sandi Bank (III) adalah sandi bank kreditur 6 digit SPOJK, bukan kantor pelapor.",
		},
	}

	totalPlafon := decimal.Zero
	totalAgunan := decimal.Zero
	totalBaki := decimal.Zero
	totalBiaya := decimal.Zero
	totalDiskonto := decimal.Zero
	totalNeto := decimal.Zero
	lengkap := true
	for _, item := range rows {
		row := TableRow{Key: form00_07RowKey(item)}
		neto, ok := item.BakiDebetNeto()
		if !ok {
			lengkap = false
			row.Reason = "biaya transaksi dan/atau diskonto belum diamortisasi melebihi baki debet; kolom XV Baki Debet Neto tidak dapat dihitung dan tidak diisi angka negatif"
		} else {
			totalPlafon = totalPlafon.Add(item.Plafon)
			totalAgunan = totalAgunan.Add(item.CollateralAmount)
			totalBaki = totalBaki.Add(item.BakiDebet)
			totalBiaya = totalBiaya.Add(item.UnamortizedTransactionCost)
			totalDiskonto = totalDiskonto.Add(item.UnamortizedDiscount)
			totalNeto = totalNeto.Add(neto)
		}
		row.Cells = []TableCell{
			{Sandi: "I", Nama: "ID Pihak Lawan", Value: dashIfEmpty(item.CounterpartyID)},
			{Sandi: "II", Nama: "Gol. Kreditur", Value: dashIfEmpty(item.CreditorGroupCode)},
			{Sandi: "III", Nama: "Sandi Bank", Value: dashIfEmpty(item.BankCode)},
			{Sandi: "IV", Nama: "Lokasi Kreditur", Value: dashIfEmpty(item.LocationCode)},
			{Sandi: "V", Nama: "Jenis", Value: dashIfEmpty(item.JenisCode)},
			{Sandi: "VI", Nama: "Hubungan dengan Bank", Value: dashIfEmpty(item.RelationshipCode)},
			{Sandi: "VII.1", Nama: "Tanggal Mulai", Value: formatTanggalAtauDash(&item.StartDate)},
			{Sandi: "VII.2", Nama: "Tanggal Jatuh Tempo", Value: formatTanggalAtauDash(&item.MaturityDate)},
			{Sandi: "VIII.1", Nama: "Suku Bunga (%)", Value: FormatPersen(item.InterestRate)},
			{Sandi: "VIII.2", Nama: "Cara Perhitungan", Value: dashIfEmpty(item.InterestCalcCode)},
			{Sandi: "IX", Nama: "Plafon", Value: FormatRupiah(item.Plafon)},
			{Sandi: "X", Nama: "Jenis Agunan yang Dijaminkan", Value: dashIfEmpty(item.CollateralTypeCode)},
			{Sandi: "XI", Nama: "Nominal Agunan yang Dijaminkan", Value: FormatRupiah(item.CollateralAmount)},
			{Sandi: "XII", Nama: "Baki Debet", Value: FormatRupiah(item.BakiDebet)},
			{Sandi: "XIII", Nama: "Biaya Transaksi Belum Diamortisasi", Value: FormatRupiah(item.UnamortizedTransactionCost)},
			{Sandi: "XIV", Nama: "Diskonto Belum Diamortisasi", Value: FormatRupiah(item.UnamortizedDiscount)},
			{Sandi: "XV", Nama: "Baki Debet Neto", Value: netoCell(neto, ok)},
		}
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form00_07TotalKey,
		Cells: []TableCell{
			{Sandi: "I", Nama: "ID Pihak Lawan", Value: "JUMLAH"},
			{Sandi: "II", Nama: "Gol. Kreditur", Value: "-"},
			{Sandi: "III", Nama: "Sandi Bank", Value: "-"},
			{Sandi: "IV", Nama: "Lokasi Kreditur", Value: "-"},
			{Sandi: "V", Nama: "Jenis", Value: "-"},
			{Sandi: "VI", Nama: "Hubungan dengan Bank", Value: "-"},
			{Sandi: "VII.1", Nama: "Tanggal Mulai", Value: "-"},
			{Sandi: "VII.2", Nama: "Tanggal Jatuh Tempo", Value: "-"},
			{Sandi: "VIII.1", Nama: "Suku Bunga (%)", Value: "-"},
			{Sandi: "VIII.2", Nama: "Cara Perhitungan", Value: "-"},
			{Sandi: "IX", Nama: "Plafon", Value: "-"},
			{Sandi: "X", Nama: "Jenis Agunan yang Dijaminkan", Value: "-"},
			{Sandi: "XI", Nama: "Nominal Agunan yang Dijaminkan", Value: "-"},
			{Sandi: "XII", Nama: "Baki Debet", Value: "-"},
			{Sandi: "XIII", Nama: "Biaya Transaksi Belum Diamortisasi", Value: "-"},
			{Sandi: "XIV", Nama: "Diskonto Belum Diamortisasi", Value: "-"},
			{Sandi: "XV", Nama: "Baki Debet Neto", Value: "-"},
		},
	}
	if lengkap && len(rows) > 0 {
		totalRow.Cells[10].Value = FormatRupiah(totalPlafon)
		totalRow.Cells[12].Value = FormatRupiah(totalAgunan)
		totalRow.Cells[13].Value = FormatRupiah(totalBaki)
		totalRow.Cells[14].Value = FormatRupiah(totalBiaya)
		totalRow.Cells[15].Value = FormatRupiah(totalDiskonto)
		totalRow.Cells[16].Value = FormatRupiah(totalNeto)
	} else if len(rows) > 0 {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena ada baris yang kolom XV Baki Debet Neto-nya tidak tersedia; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	return sec
}

// netoCell menulis kolom XV: angka neto bila dapat dihitung, "-" bila tidak (hasil
// negatif). Baris yang "-" sudah membawa alasannya lewat TableRow.Reason.
func netoCell(neto decimal.Decimal, ok bool) string {
	if !ok {
		return "-"
	}
	return FormatRupiah(neto)
}

// form00_07RowKey mengidentifikasi satu baris Form 00.07 secara terbaca: ID Pihak
// Lawan kreditur.
func form00_07RowKey(item domain.PinjamanItem) string {
	return item.CounterpartyID
}
