package ojkreport

import (
	"context"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// form08_00.go membangun Form 08.00 "DAFTAR ASET TETAP, INVENTARIS, DAN ASET TIDAK
// BERWUJUD" dari register aset_tetap_register (migrasi 000119).
//
// Dasar: Lampiran II SEOJK No. 16/SEOJK.03/2024.
//   - Form 08.00 "DAFTAR ASET TETAP, INVENTARIS, DAN ASET TIDAK BERWUJUD": PDF #page
//     182 (hlm. tercetak 130). Sembilan kolom: I Sandi Kantor, II Jenis Aset,
//     III Sumber Perolehan, IV Status Aset, V Biaya Perolehan,
//     VI Akumulasi Penyusutan/Amortisasi, VII Akumulasi Kerugian Penurunan Nilai,
//     VIII Nilai Tercatat, IX Metode Pengukuran. Ada baris JUMLAH.
//   - Sandi: PDF #page 183 (hlm. 131). II 101/102/103/104/199 dan 201/202/299;
//     III 01/02/03/99; IV 1/2; IX 1/2.
//   - Penjelasan: PDF #page 184-186 (hlm. 132-134). Baris per kombinasi jenis aset /
//     sumber perolehan / status / metode pengukuran; aset dengan kesamaan sandi rincian
//     dan angka kolom I s.d. VI dapat digabung. Status Aset dikosongkan untuk aset
//     tidak berwujud (PDF #185). Biaya Perolehan memakai nilai setelah revaluasi.
//     Nilai Tercatat = biaya perolehan - akumulasi penyusutan atau amortisasi -
//     kerugian penurunan nilai.
//
// BATAS SUMBER — jangan diisi tebakan:
//   - SELURUH angka V/VI/VII dan sandi adalah isian bank dari register. Sistem tidak
//     menghitung biaya perolehan maupun akumulasi.
//   - Register disimpan PER ASET; builder ini MENGELOMPOKKAN baris per kombinasi
//     II x III x IV x IX dan menjumlahkan kolom nominal, sesuai aturan penggabungan
//     PDF #page 184.
//   - Kolom VIII "Nilai Tercatat" adalah turunan: V - VI - VII. Bila hasil agregat
//     negatif, kolom VIII ditulis "-" beserta alasannya, bukan angka negatif.
//   - Kolom IV Status Aset dikosongkan untuk aset tidak berwujud. Untuk aset berwujud
//     yang statusnya belum diisi, kolom ditulis "-" beserta alasannya, bukan ditebak.
//   - Baris JUMLAH hanya diisi bila SELURUH baris lengkap; bila ada yang tidak, JUMLAH
//     ditulis "-" karena total sebagian akan menyesatkan (pola Form 07.00/14.00).
//   - Kolom I "Sandi Kantor" diambil dari kantor pelapor tunggal bank_offices (migrasi
//     000112) lewat ReportingOffice. Register sendiri bank-wide, tidak per kantor.
//   - Register adalah RINCIAN per aset; pos Aset Tetap/Inventaris pada laporan posisi
//     keuangan (1202010000/1202020000) berasal dari saldo COA 10600/10700 bank-wide.
//     Keduanya dapat berbeda bila register belum lengkap; laporan ini TIDAK memaksa tie
//     (tidak ada tie otomatis).

// AsetTetapRegisterSource menyediakan baris register aset bank-wide untuk Form 08.00.
// Kontraknya opsional pada perakitan RepoSource: tanpa sumber ini Form 08.00 dinyatakan
// belum tersedia, bukan ditulis kosong.
type AsetTetapRegisterSource interface {
	ListAsetTetapForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.AsetTetapItem, error)
}

// form08_00TotalKey adalah kunci baris kaki tabel JUMLAH; bukan sandi pos OJK.
const form08_00TotalKey = "JUMLAH"

// asetTetapGroup adalah agregat baris register untuk satu kombinasi
// II x III x IV x IX.
type asetTetapGroup struct {
	jenis  string
	sumber string
	// status adalah status efektif yang dipakai mengelompokkan: "" untuk aset tidak
	// berwujud (dikosongkan form) atau aset berwujud yang belum diisi.
	status string
	metode string
	biaya  decimal.Decimal
	akum   decimal.Decimal
	rugi   decimal.Decimal
}

// BuildForm08_00 menyusun tabel Form 08.00 dari baris register aset dan kantor pelapor
// kolom I. Fungsi ini murni sehingga dapat diuji tanpa basis data. Baris register
// dikelompokkan per kombinasi II x III x IV x IX; kolom nominal dijumlahkan. Bila tidak
// ada baris, baris JUMLAH tetap dibawa dengan sel "-".
func BuildForm08_00(rows []domain.AsetTetapItem, kantor ReportingOffice) TableSection {
	sec := TableSection{
		Form:     "08.00",
		Name:     formName("08.00"),
		KeyLabel: "Jenis Aset|Sumber Perolehan|Status Aset|Metode Pengukuran",
		Columns: []TableColumn{
			{Sandi: "II", Nama: "Jenis Aset"},
			{Sandi: "III", Nama: "Sumber Perolehan"},
			{Sandi: "IV", Nama: "Status Aset"},
			{Sandi: "V", Nama: "Biaya Perolehan"},
			{Sandi: "VI", Nama: "Akumulasi Penyusutan/Amortisasi"},
			{Sandi: "VII", Nama: "Akumulasi Kerugian Penurunan Nilai"},
			{Sandi: "VIII", Nama: "Nilai Tercatat"},
			{Sandi: "IX", Nama: "Metode Pengukuran"},
		},
		Notes: []string{
			"Baris per kombinasi Jenis Aset (II) x Sumber Perolehan (III) x Status Aset (IV) x Metode Pengukuran (IX) dari register aset_tetap_register yang bank isi; bank boleh menggabungkan aset dengan kesamaan sandi rincian dan angka kolom I s.d. VI (Form 08.00, PDF #page 184).",
			"Sandi II baku PDF #page 183: 101 Tanah, 102 Bangunan, 103 Peralatan dan perlengkapan, 104 Kendaraan, 199 Lainnya (aset tetap dan inventaris); 201 Program aplikasi (software), 202 Goodwill, 299 Lainnya (aset tidak berwujud). III: 01 Sewa Pembiayaan, 02 Modal Disetor, 03 Modal Sumbangan, 99 Sumber Perolehan Lainnya. IX: 1 Model Biaya, 2 Model Revaluasi.",
			"Status Aset (IV) dikosongkan untuk aset tidak berwujud (PDF #page 185). Untuk aset berwujud yang statusnya belum diisi bank, kolom IV ditulis \"-\" beserta alasannya, bukan ditebak.",
			"Biaya Perolehan (V) memakai nilai aset setelah revaluasi bila aset telah direvaluasi (PDF #page 185). Seluruh angka V/VI/VII adalah isian bank; sistem tidak menghitungnya.",
			"Nilai Tercatat (VIII) dihitung sebagai Biaya Perolehan (V) dikurangi Akumulasi Penyusutan/Amortisasi (VI) dan Akumulasi Kerugian Penurunan Nilai (VII) (PDF #page 185). Bila hasilnya negatif, kolom VIII ditulis \"-\" beserta alasannya, bukan angka negatif.",
			"Baris JUMLAH hanya diisi bila seluruh baris lengkap; bila ada baris yang kolomnya tidak tersedia, JUMLAH ditulis \"-\" karena total sebagian akan menyesatkan.",
			"Register adalah rincian per aset, sedangkan pos Aset Tetap dan Inventaris pada laporan posisi keuangan (1202010000/1202020000 Form 01.00) berasal dari saldo COA 10600/10700 bank-wide. Keduanya dapat berbeda bila register belum lengkap; laporan ini TIDAK memaksa total register sama dengan saldo COA (tidak ada tie otomatis).",
			"Kolom I Sandi Kantor diambil dari kantor pelapor tunggal pada bank_offices (migrasi 000112), bukan dari register; bila jumlah kantor aktif ber-sandi bukan tepat satu, kolom dinyatakan tidak tersedia, bukan dikarang.",
		},
	}

	// Kelompokkan baris per kombinasi, mempertahankan urutan kemunculan pertama agar
	// keluaran deterministik mengikuti urutan sumber.
	var urutan []string
	grup := make(map[string]*asetTetapGroup)
	for _, item := range rows {
		status := form08_00StatusEfektif(item)
		key := form08_00GroupKey(item.JenisAsetCode, item.SumberPerolehanCode, status, item.MetodePengukuranCode)
		g := grup[key]
		if g == nil {
			g = &asetTetapGroup{
				jenis:  strings.TrimSpace(item.JenisAsetCode),
				sumber: strings.TrimSpace(item.SumberPerolehanCode),
				status: status,
				metode: strings.TrimSpace(item.MetodePengukuranCode),
			}
			grup[key] = g
			urutan = append(urutan, key)
		}
		g.biaya = g.biaya.Add(item.BiayaPerolehan)
		g.akum = g.akum.Add(item.AkumulasiPenyusutanAmortisasi)
		g.rugi = g.rugi.Add(item.AkumulasiKerugianPenurunanNilai)
	}

	totalBiaya := decimal.Zero
	totalAkum := decimal.Zero
	totalRugi := decimal.Zero
	totalNilai := decimal.Zero
	lengkap := true

	for _, key := range urutan {
		g := grup[key]
		row := TableRow{Key: key}
		nilai, ok := asetTetapNilaiTercatat(g.biaya, g.akum, g.rugi)
		if !ok {
			lengkap = false
			row.Reason = "akumulasi penyusutan/amortisasi dan/atau akumulasi kerugian penurunan nilai melebihi biaya perolehan; kolom VIII Nilai Tercatat tidak dapat dihitung dan tidak diisi angka negatif"
		}
		if g.status == "" && !domain.AsetJenisTidakBerwujud(g.jenis) {
			lengkap = false
			if row.Reason != "" {
				row.Reason += "; "
			}
			row.Reason += "Status Aset (IV) belum diisi bank untuk aset berwujud; kolom ditulis \"-\" dan tidak ditebak"
		}
		row.Cells = []TableCell{
			{Sandi: "II", Nama: "Jenis Aset", Value: dashIfEmpty(g.jenis)},
			{Sandi: "III", Nama: "Sumber Perolehan", Value: dashIfEmpty(g.sumber)},
			{Sandi: "IV", Nama: "Status Aset", Value: form08_00StatusCell(g)},
			{Sandi: "V", Nama: "Biaya Perolehan", Value: FormatRupiah(g.biaya)},
			{Sandi: "VI", Nama: "Akumulasi Penyusutan/Amortisasi", Value: FormatRupiah(g.akum)},
			{Sandi: "VII", Nama: "Akumulasi Kerugian Penurunan Nilai", Value: FormatRupiah(g.rugi)},
			{Sandi: "VIII", Nama: "Nilai Tercatat", Value: asetTetapNilaiCell(nilai, ok)},
			{Sandi: "IX", Nama: "Metode Pengukuran", Value: dashIfEmpty(g.metode)},
		}
		totalBiaya = totalBiaya.Add(g.biaya)
		totalAkum = totalAkum.Add(g.akum)
		totalRugi = totalRugi.Add(g.rugi)
		totalNilai = totalNilai.Add(nilai)
		sec.Rows = append(sec.Rows, row)
	}

	totalRow := TableRow{
		Key: form08_00TotalKey,
		Cells: []TableCell{
			{Sandi: "II", Nama: "Jenis Aset", Value: "JUMLAH"},
			{Sandi: "III", Nama: "Sumber Perolehan", Value: "-"},
			{Sandi: "IV", Nama: "Status Aset", Value: "-"},
			{Sandi: "V", Nama: "Biaya Perolehan", Value: "-"},
			{Sandi: "VI", Nama: "Akumulasi Penyusutan/Amortisasi", Value: "-"},
			{Sandi: "VII", Nama: "Akumulasi Kerugian Penurunan Nilai", Value: "-"},
			{Sandi: "VIII", Nama: "Nilai Tercatat", Value: "-"},
			{Sandi: "IX", Nama: "Metode Pengukuran", Value: "-"},
		},
	}
	if lengkap && len(rows) > 0 {
		totalRow.Cells[3].Value = FormatRupiah(totalBiaya)
		totalRow.Cells[4].Value = FormatRupiah(totalAkum)
		totalRow.Cells[5].Value = FormatRupiah(totalRugi)
		totalRow.Cells[6].Value = FormatRupiah(totalNilai)
	} else if len(rows) > 0 {
		totalRow.Reason = "JUMLAH belum dapat dihitung karena ada baris yang kolom IV/VIII-nya tidak tersedia; angka sebagian akan menyesatkan"
	}
	sec.Rows = append(sec.Rows, totalRow)

	pasangKolomSandiKantor(&sec, kantor)
	return sec
}

// form08_00StatusEfektif menentukan status aset yang dipakai form: aset tidak berwujud
// selalu dikosongkan (PDF #page 185), aset berwujud memakai isian bank (boleh kosong).
func form08_00StatusEfektif(item domain.AsetTetapItem) string {
	if domain.AsetJenisTidakBerwujud(item.JenisAsetCode) {
		return ""
	}
	return strings.TrimSpace(item.StatusAsetCode)
}

// form08_00StatusCell menulis kolom IV: sandi status apa adanya, atau "" untuk aset
// tidak berwujud (dikosongkan form), atau "-" untuk aset berwujud yang belum diisi.
func form08_00StatusCell(g *asetTetapGroup) string {
	if g.status != "" {
		return g.status
	}
	if domain.AsetJenisTidakBerwujud(g.jenis) {
		return ""
	}
	return "-"
}

// form08_00GroupKey menyusun kunci kombinasi II x III x IV x IX secara terbaca dan
// stabil. Status kosong menjadi segmen kosong, tetap unik terhadap status terisi.
func form08_00GroupKey(jenis, sumber, status, metode string) string {
	return strings.Join([]string{
		strings.TrimSpace(jenis),
		strings.TrimSpace(sumber),
		status,
		strings.TrimSpace(metode),
	}, "|")
}

// asetTetapNilaiTercatat menghitung kolom VIII untuk agregat satu kombinasi:
// biaya (V) - akumulasi penyusutan/amortisasi (VI) - akumulasi kerugian penurunan nilai
// (VII). ok=false bila hasilnya negatif.
func asetTetapNilaiTercatat(biaya, akum, rugi decimal.Decimal) (decimal.Decimal, bool) {
	nilai := biaya.Sub(akum).Sub(rugi)
	if nilai.IsNegative() {
		return decimal.Zero, false
	}
	return nilai, true
}

// asetTetapNilaiCell menulis kolom VIII: angka nilai tercatat bila dapat dihitung, "-"
// bila tidak. Baris yang "-" sudah membawa alasannya lewat TableRow.Reason.
func asetTetapNilaiCell(nilai decimal.Decimal, ok bool) string {
	if !ok {
		return "-"
	}
	return FormatRupiah(nilai)
}
