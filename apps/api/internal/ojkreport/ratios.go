package ojkreport

import (
	"time"

	"github.com/shopspring/decimal"
)

// ratios.go menghitung Form 00.08 RASIO KEUANGAN TRIWULANAN.
//
// Dasar rumus: SEOJK No. 16/SEOJK.03/2024, Lampiran II, Form 00.08 – 2 PENJELASAN
// RASIO KEUANGAN TRIWULANAN (hlm. 204-205 pada penomoran dokumen; PDF hlm. 255).
// Lampiran II hanya memberi definisi operasional. Untuk KPMM, ROA, BOPO, NIM, LDR,
// dan Cash Ratio, Lampiran II merujuk ke SEOJK lain (KPMM BPR, penilaian tingkat
// kesehatan BPR/BPRS, dan manajemen risiko BPR) yang belum ditelusuri di repositori
// ini. Karena itu rasio yang komponennya belum tersedia dinyatakan TIDAK TERSEDIA
// beserta alasannya, bukan diisi nol: nol dan tidak tersedia berbeda bagi regulator.
//
// Form hanya diisi untuk posisi Maret, Juni, September, dan Desember. Posisi bulan
// lain dikosongkan (Lampiran II hlm. 204, kalimat pertama).

// Sandi rasio Form 00.08 (Lampiran II hlm. 203).
const (
	sandiKPMM         = "0101"
	sandiCadanganPPKA = "0202"
	sandiNPLNeto      = "0203"
	sandiNPLGross     = "0204"
	sandiROA          = "0401"
	sandiBOPO         = "0402"
	sandiNIM          = "0403"
	sandiLDR          = "0501"
	sandiCashRatio    = "0502"
)

// kreditDiberikanSandi adalah pos "Kredit yang Diberikan (Baki Debet)" Form 01.00.
// LDR memakai nilai bruto (sebelum CKPN dan pos kontra) sesuai penyebut "total
// kredit yang diberikan" pada Lampiran II hlm. 204 butir 8.
var kreditDiberikanSandi = []string{"1104010100"}

// danaPihakKetigaSandi adalah Simpanan a. Tabungan dan b. Deposito beserta pos
// kontra biaya transaksi belum diamortisasi. "Simpanan dari Bank Lain" tidak
// termasuk karena penyebutnya adalah dana pihak ketiga BUKAN bank
// (Lampiran II hlm. 204 butir 8).
var danaPihakKetigaSandi = []string{"2102010100", "2102010200", "2102020100", "2102020200"}

// alasanDikosongkan dipakai untuk posisi bulan selain Maret/Juni/September/Desember.
const alasanDikosongkan = "hanya diisi untuk posisi Maret, Juni, September, dan Desember; posisi bulan ini dikosongkan (Lampiran II hlm. 204)"

// RasioHasil adalah satu baris Form 00.08 yang siap ditulis.
type RasioHasil struct {
	Sandi string
	Nama  string
	// NilaiPersen hanya bermakna bila Tersedia benar.
	NilaiPersen decimal.Decimal
	Tersedia    bool
	// Alasan wajib terisi bila Tersedia=false, menjelaskan komponen yang belum ada.
	Alasan string
	// Dasar menyebut dasar rumus beserta letaknya di Lampiran II.
	Dasar string
}

// rasioDefinition menetapkan urutan, sandi, nama, dan dasar rumus baris Form 00.08
// persis seperti Lampiran II hlm. 203. AlasanKosong menjelaskan mengapa rasio ini
// belum dapat dihitung dari laporan yang ada; kosong berarti dihitung.
type rasioDefinition struct {
	Sandi        string
	Nama         string
	Dasar        string
	AlasanKosong string
}

// rasioKeuanganDefinisi adalah susunan baris Form 00.08 (Lampiran II hlm. 203).
var rasioKeuanganDefinisi = []rasioDefinition{
	{
		Sandi: sandiKPMM, Nama: "Kewajiban Penyediaan Modal Minimum (KPMM)",
		Dasar: "Lampiran II hlm. 204 butir 1: modal dibagi ATMR sesuai POJK KPMM BPR",
		AlasanKosong: "komponen modal dan ATMR sesuai POJK KPMM BPR belum dihitung " +
			"di repositori; hanya total ekuitas yang tersedia",
	},
	{
		Sandi: sandiCadanganPPKA, Nama: "Rasio Cadangan terhadap PPKA",
		Dasar: "Lampiran II hlm. 204 butir 2: pencadangan dibentuk dibagi PPKA",
		AlasanKosong: "PPKA (pencadangan wajib menurut POJK Kualitas Aset) belum " +
			"dihitung; yang tersedia baru saldo CKPN pada Form 01.00",
	},
	{
		Sandi: sandiNPLNeto, Nama: "Non Performing Loan (NPL) Neto",
		Dasar: "Lampiran II hlm. 204 butir 3: (kurang lancar + diragukan + macet " +
			"- CKPN) dibagi total kredit",
		AlasanKosong: "rincian kredit menurut kualitas kurang lancar/diragukan/macet " +
			"belum tersedia pada laporan journal-based; yang ada baru total baki debet",
	},
	{
		Sandi: sandiNPLGross, Nama: "Non Performing Loan (NPL) Gross",
		Dasar: "Lampiran II hlm. 204 butir 4: (kurang lancar + diragukan + macet) " +
			"dibagi total kredit",
		AlasanKosong: "rincian kredit menurut kualitas kurang lancar/diragukan/macet " +
			"belum tersedia pada laporan journal-based; yang ada baru total baki debet",
	},
	{
		Sandi: sandiROA, Nama: "Return on Assets (ROA)",
		Dasar: "Lampiran II hlm. 204 butir 5: laba sebelum pajak dibagi rata-rata " +
			"total aset (metode SEOJK penilaian tingkat kesehatan BPR/BPRS)",
		AlasanKosong: "penyebut rata-rata total aset belum dihitung; metode rata-rata " +
			"mengikuti SEOJK penilaian tingkat kesehatan BPR/BPRS yang belum diacu",
	},
	{
		Sandi: sandiBOPO, Nama: "Biaya Operasional terhadap Pendapatan Operasional (BOPO)",
		Dasar: "Lampiran II hlm. 204 butir 6: beban operasional dibagi pendapatan operasional",
	},
	{
		Sandi: sandiNIM, Nama: "Net Interest Margin (NIM)",
		Dasar: "Lampiran II hlm. 204 butir 7: pendapatan bunga bersih dibagi rata-rata " +
			"aset produktif (metode SEOJK penilaian tingkat kesehatan BPR/BPRS)",
		AlasanKosong: "penyebut rata-rata aset produktif belum tersedia; metode " +
			"mengikuti SEOJK penilaian tingkat kesehatan BPR/BPRS yang belum diacu",
	},
	{
		Sandi: sandiLDR, Nama: "Loan to Deposit Ratio (LDR)",
		Dasar: "Lampiran II hlm. 204 butir 8: total kredit dibagi total dana pihak " +
			"ketiga bukan bank",
	},
	{
		Sandi: sandiCashRatio, Nama: "Cash Ratio",
		Dasar: "Lampiran II hlm. 204-205 butir 9: aset likuid dibagi utang lancar " +
			"menurut SEOJK manajemen risiko BPR",
		AlasanKosong: "komposisi aset likuid dan utang lancar mengikuti SEOJK manajemen " +
			"risiko BPR yang belum dirinci pada Lampiran II",
	},
}

// RasioPersen menghitung (pembilang / penyebut) x 100. ok=false bila penyebut nol
// sehingga tidak pernah menghasilkan NaN/Inf; nilai yang dikembalikan saat itu nol.
func RasioPersen(pembilang, penyebut decimal.Decimal) (decimal.Decimal, bool) {
	if penyebut.IsZero() {
		return decimal.Zero, false
	}
	return pembilang.Div(penyebut).Mul(decimal.NewFromInt(100)), true
}

// RumusKPMM = modal / ATMR x 100 (Lampiran II hlm. 204 butir 1).
func RumusKPMM(modal, atmr decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(modal, atmr)
}

// RumusCadanganPPKA = pencadangan dibentuk / PPKA x 100 (Lampiran II hlm. 204 butir 2).
func RumusCadanganPPKA(cadanganDibentuk, ppka decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(cadanganDibentuk, ppka)
}

// RumusNPLNeto = (kurang lancar + diragukan + macet - CKPN) / total kredit x 100
// (Lampiran II hlm. 204 butir 3).
func RumusNPLNeto(kurangLancar, diragukan, macet, ckpn, totalKredit decimal.Decimal) (decimal.Decimal, bool) {
	npl := kurangLancar.Add(diragukan).Add(macet).Sub(ckpn)
	return RasioPersen(npl, totalKredit)
}

// RumusNPLGross = (kurang lancar + diragukan + macet) / total kredit x 100
// (Lampiran II hlm. 204 butir 4).
func RumusNPLGross(kurangLancar, diragukan, macet, totalKredit decimal.Decimal) (decimal.Decimal, bool) {
	npl := kurangLancar.Add(diragukan).Add(macet)
	return RasioPersen(npl, totalKredit)
}

// RumusROA = laba sebelum pajak / rata-rata total aset x 100 (Lampiran II hlm. 204 butir 5).
func RumusROA(labaSebelumPajak, rataRataTotalAset decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(labaSebelumPajak, rataRataTotalAset)
}

// RumusBOPO = beban operasional / pendapatan operasional x 100 (Lampiran II hlm. 204 butir 6).
func RumusBOPO(bebanOperasional, pendapatanOperasional decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(bebanOperasional, pendapatanOperasional)
}

// RumusNIM = pendapatan bunga bersih / rata-rata aset produktif x 100
// (Lampiran II hlm. 204 butir 7).
func RumusNIM(pendapatanBungaBersih, rataRataAsetProduktif decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(pendapatanBungaBersih, rataRataAsetProduktif)
}

// RumusLDR = total kredit / total dana pihak ketiga bukan bank x 100
// (Lampiran II hlm. 204 butir 8).
func RumusLDR(kreditDiberikan, danaPihakKetiga decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(kreditDiberikan, danaPihakKetiga)
}

// RumusCashRatio = aset likuid / utang lancar x 100 (Lampiran II hlm. 204-205 butir 9).
func RumusCashRatio(asetLikuid, kewajibanLancar decimal.Decimal) (decimal.Decimal, bool) {
	return RasioPersen(asetLikuid, kewajibanLancar)
}

// IsQuarterMonth melaporkan apakah periode adalah posisi triwulanan Form 00.08.
func IsQuarterMonth(t time.Time) bool {
	switch t.Month() {
	case time.March, time.June, time.September, time.December:
		return true
	default:
		return false
	}
}

// KomponenRasio membawa komponen yang tidak berasal dari Form 01.00/02.00: kualitas
// kredit (untuk NPL) dan rata-rata total aset (untuk ROA). Zero-value berarti
// komponen belum tersedia; Tersedia menegaskan hal itu secara eksplisit.
type KomponenRasio struct {
	Kredit            KomponenKreditNPL
	RataRataTotalAset decimal.Decimal
	// RataRataTotalAsetTersedia false berarti rata-rata belum dapat diturunkan.
	RataRataTotalAsetTersedia bool
}

// RasioKeuangan menghitung seluruh baris Form 00.08 dari angka Form 01.00
// (amounts01) dan Form 02.00 (amounts02) yang sudah dihitung builder, ditambah
// komponen opsional (kualitas kredit dan rata-rata total aset). Rasio yang
// komponennya belum tersedia tetap dikembalikan dengan Tersedia=false dan Alasan
// terisi, supaya tidak pernah tertukar dengan nilai nol.
func RasioKeuangan(period time.Time, amounts01, amounts02 map[string]decimal.Decimal, extra ...KomponenRasio) []RasioHasil {
	triwulanan := IsQuarterMonth(period)
	var komponen KomponenRasio
	if len(extra) > 0 {
		komponen = extra[0]
	}
	bebanOperasional := amounts02["5100000000"]
	pendapatanOperasional := amounts02["4100000000"]
	kredit := jumlahSandi(amounts01, kreditDiberikanSandi)
	danaPihakKetiga := jumlahSandi(amounts01, danaPihakKetigaSandi)
	labaSebelumPajak := amounts02["3104040300"]

	out := make([]RasioHasil, 0, len(rasioKeuanganDefinisi))
	for _, d := range rasioKeuanganDefinisi {
		h := RasioHasil{Sandi: d.Sandi, Nama: d.Nama, Dasar: d.Dasar}
		switch {
		case !triwulanan:
			h.Alasan = alasanDikosongkan
		case d.Sandi == sandiBOPO:
			h.NilaiPersen, h.Tersedia = RumusBOPO(bebanOperasional, pendapatanOperasional)
			if !h.Tersedia {
				h.Alasan = "pendapatan operasional nol atau tidak tersedia pada laporan laba rugi"
			}
		case d.Sandi == sandiLDR:
			h.NilaiPersen, h.Tersedia = RumusLDR(kredit, danaPihakKetiga)
			if !h.Tersedia {
				h.Alasan = "dana pihak ketiga bukan bank nol atau tidak tersedia pada laporan posisi keuangan"
			}
		case d.Sandi == sandiNPLGross:
			h.NilaiPersen, h.Tersedia, h.Alasan = hitungNPL(komponen.Kredit, false)
		case d.Sandi == sandiNPLNeto:
			h.NilaiPersen, h.Tersedia, h.Alasan = hitungNPL(komponen.Kredit, true)
		case d.Sandi == sandiROA:
			if !komponen.RataRataTotalAsetTersedia {
				h.Alasan = d.AlasanKosong
				break
			}
			h.NilaiPersen, h.Tersedia = RumusROA(labaSebelumPajak, komponen.RataRataTotalAset)
			if !h.Tersedia {
				h.Alasan = "rata-rata total aset nol sehingga ROA tidak dapat dihitung"
			}
		default:
			h.Alasan = d.AlasanKosong
		}
		out = append(out, h)
	}
	return out
}

// hitungNPL menghitung NPL gross/neto dari komponen kredit. Bila komponen kredit
// belum tersedia, hasilnya dinyatakan tidak tersedia beserta alasan spesifik, bukan
// nol. neto=true mengurangkan CKPN kredit tidak lancar (Lampiran II hlm. 204 butir 3).
func hitungNPL(k KomponenKreditNPL, neto bool) (decimal.Decimal, bool, string) {
	if !k.Tersedia {
		return decimal.Zero, false, "kualitas kredit per debitur belum tersedia; baki debet dan CKPN per kredit belum dibaca"
	}
	if k.TotalKredit.IsZero() {
		return decimal.Zero, false, "total kredit yang diberikan nol sehingga NPL tidak dapat dihitung"
	}
	if neto {
		nilai, ok := RumusNPLNeto(k.KurangLancar, k.Diragukan, k.Macet, k.CKPNNPL, k.TotalKredit)
		return nilai, ok, ""
	}
	nilai, ok := RumusNPLGross(k.KurangLancar, k.Diragukan, k.Macet, k.TotalKredit)
	return nilai, ok, ""
}

// jumlahSandi menjumlahkan nilai sejumlah sandi; sandi tanpa nilai dianggap nol.
func jumlahSandi(amounts map[string]decimal.Decimal, sandis []string) decimal.Decimal {
	total := decimal.Zero
	for _, s := range sandis {
		total = total.Add(amounts[s])
	}
	return total
}
