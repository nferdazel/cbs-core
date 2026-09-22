package ojkreport

import (
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// kpmm.go memuat perhitungan KPMM/ATMR yang murni (tanpa basis data) supaya dapat
// diuji terpisah. Orkestrasi pembacaan laporan, CKPN, dan konfigurasi ada di
// internal/service/kpmm_service.go.
//
// ATMR BPR menurut POJK No. 5/POJK.03/2015 dan SEOJK No. 2/SEOJK.03/2025 adalah
// aset neraca (laporan posisi keuangan) dikalikan bobot risiko; TIDAK ada beban
// ATMR operasional maupun pasar sebagaimana bank umum, sehingga tidak dihitung di
// sini. Bobot risiko diambil dari konfigurasi (bukan ditanam di kode), dengan
// rincian kategori pada Lampiran II SEOJK 2/2025.

// Kategori bobot risiko ATMR. Nama kategori sengaja stabil karena dipakai sebagai
// sufiks kunci konfigurasi kpmm.rwa_frac.<kategori>.
const (
	RisikoKas         = "kas"
	RisikoAntarBank   = "antar_bank"
	RisikoKredit      = "kredit"
	RisikoAYDA        = "ayda"
	RisikoAsetTetap   = "aset_tetap"
	RisikoAntarKantor = "antar_kantor"
	RisikoLainnya     = "lainnya"
)

// KategoriRisikoATMR mengembalikan seluruh kategori bobot risiko yang dibaca dari
// konfigurasi, terurut tetap agar pemeriksaan dapat diandalkan.
func KategoriRisikoATMR() []string {
	return []string{
		RisikoKas, RisikoAntarBank, RisikoKredit, RisikoAYDA,
		RisikoAsetTetap, RisikoAntarKantor, RisikoLainnya,
	}
}

// BobotRisikoATMR memetakan kategori ke bobot risiko (fraksi 0..1; 0,3 = 30%).
type BobotRisikoATMR map[string]decimal.Decimal

// KategoriRisikoSandi mengelompokkan pos aset Form 01.00 (sandi OJK) ke kategori
// bobot risiko. ok=false berarti sandi bukan pos aset (kewajiban/ekuitas), bukan
// galat. Sandi aset yang belum punya kategori khusus jatuh ke "lainnya" (100%,
// konservatif) alih-alih dilewatkan.
func KategoriRisikoSandi(sandi string) (string, bool) {
	if !strings.HasPrefix(sandi, "1") {
		return "", false
	}
	switch sandi {
	case "1101010000":
		return RisikoKas, true
	case "1103010000":
		return RisikoAntarBank, true
	case "1104010100", "1104010400", "1104020000":
		return RisikoKredit, true
	case "1201000000":
		return RisikoAYDA, true
	case "1202010000", "1202020000":
		return RisikoAsetTetap, true
	case "1204000000":
		return RisikoAntarKantor, true
	case "1299000000":
		return RisikoLainnya, true
	default:
		return RisikoLainnya, true
	}
}

// kategoriAsetTambahan memetakan pos aset yang BELUM ada pada pemetaan COA DRAF
// (coa_mapping.go) tetapi jelas aset neraca: akun cadangan CKPN 10950/11950 dari
// migrasi 000042 (pos lawan kredit, nilainya negatif pada laporan).
var kategoriAsetTambahan = map[string]string{
	"10950": RisikoKredit,
	"11950": RisikoKredit,
}

// KategoriRisikoCOA mengelompokkan kode COA ke kategori bobot risiko memakai
// pemetaan COA → pos OJK yang sudah ada (coa_mapping.go) ditambah pos aset yang
// diketahui di luar draf. ok=false berarti kode bukan pos aset yang dapat
// diklasifikasikan (kewajiban/ekuitas/pendapatan/beban atau kode tak dikenal).
func KategoriRisikoCOA(code string) (string, bool) {
	if kategori, ok := kategoriAsetTambahan[code]; ok {
		return kategori, true
	}
	e, ok := defaultMappingIndex[code]
	if !ok || e.Form != "01.00" {
		return "", false
	}
	return KategoriRisikoSandi(e.Sandi)
}

// ModalClassCOA mengembalikan kelas modal kode COA dari pemetaan bagan akun
// (coa_mapping.go). Kosong berarti kelasnya belum pasti dan tidak boleh
// diperhitungkan sebagai modal. Kode di luar pemetaan juga kosong.
func ModalClassCOA(code string) string {
	if e, ok := defaultMappingIndex[code]; ok {
		return e.ModalClass
	}
	return ""
}

// KlasifikasiModalCOA menjumlahkan saldo baris laporan per kelas modal. Hanya kode
// dengan kelas PASTI (INTI_UTAMA/INTI_TAMBAHAN/PELENGKAP/PENGURANG) yang dijumlahkan;
// kode tanpa kelas tidak menghasilkan entri sama sekali — bukan nol — supaya
// klasifikasi yang belum pasti tidak menjadi modal fiktif.
func KlasifikasiModalCOA(rows []domain.ReportRow) map[string]decimal.Decimal {
	out := map[string]decimal.Decimal{}
	for _, r := range rows {
		class := ModalClassCOA(r.AccountCode)
		if class == "" {
			continue
		}
		out[class] = out[class].Add(r.Amount)
	}
	return out
}

// looksLikeAsetCode menebak apakah kode COA yang TIDAK terpetakan adalah pos aset,
// memakai rentang bagan akun yang berlaku (aset 10xxx-11xxx). Pendapatan/beban
// 15xxx, kewajiban 12xxx/2xxxx, dan ekuitas 13xxx/3xxxx sengaja tidak dianggap aset.
func looksLikeAsetCode(code string) bool {
	return strings.HasPrefix(code, "10") || strings.HasPrefix(code, "11")
}

// ATMRHasil adalah hasil perhitungan ATMR.
type ATMRHasil struct {
	Total decimal.Decimal
	Baris []domain.KPMMATMRBaris
	// TidakTerkategori adalah kode COA bersaldo yang tidak pernah dipetakan ke
	// pos OJK mana pun, sehingga berdampak tidak dapat dihitung. Bila tidak
	// kosong, ATMR belum lengkap.
	TidakTerkategori []string
}

// HitungATMR mengalikan dasar tiap kategori aset dengan bobot risikonya. Baris
// kewajiban/ekuitas dan baris penyeimbang diabaikan. Kategori tanpa bobot yang
// dikonfigurasi diperlakukan bobot nol DAN dicatat sebagai tidak lengkap lewat
// Tersedia-nya pemanggil (lihat service).
func HitungATMR(rows []domain.ReportRow, bobot BobotRisikoATMR) ATMRHasil {
	hasil := ATMRHasil{Baris: make([]domain.KPMMATMRBaris, 0, len(KategoriRisikoATMR()))}
	if len(rows) == 0 {
		return hasil
	}

	dasar := map[string]decimal.Decimal{}
	for _, r := range rows {
		// Baris penyeimbang laporan sumber bukan aset/kewajiban riil.
		if r.AccountCode == "00000" || r.AccountCode == "-" {
			continue
		}
		kategori, ok := KategoriRisikoCOA(r.AccountCode)
		if ok {
			dasar[kategori] = dasar[kategori].Add(r.Amount)
			continue
		}
		// Tidak terklasifikasi: kewajiban/ekuitas/pendapatan/beban memang bukan dasar
		// ATMR. Hanya kode yang menyerupai pos aset yang dilaporkan agar tidak hilang
		// diam-diam (ATMR ditandai belum lengkap).
		if looksLikeAsetCode(r.AccountCode) {
			hasil.TidakTerkategori = append(hasil.TidakTerkategori, r.AccountCode)
		}
	}

	for _, kategori := range KategoriRisikoATMR() {
		nilaiDasar := dasar[kategori]
		w := bobot[kategori]
		hasil.Baris = append(hasil.Baris, domain.KPMMATMRBaris{
			Kategori: kategori,
			Dasar:    nilaiDasar,
			Bobot:    w,
			Nilai:    nilaiDasar.Mul(w),
		})
		hasil.Total = hasil.Total.Add(nilaiDasar.Mul(w))
	}
	return hasil
}

// PilihPengurangModalInti memilih dasar pengurang modal inti dari selisih
// PPKA–CKPN sesuai setelan. basis "agregat" memakai selisih tingkat portofolio
// (dinaikkan ke nol bila negatif); selain itu (termasuk kosong) memakai
// per-kredit Σ max(PPKA_i − CKPN_i, 0) yang secara umum lebih besar sehingga
// konservatif. basisEfektif dikembalikan untuk dicatat pada laporan.
func PilihPengurangModalInti(basis string, perKredit, agregat decimal.Decimal) (nilai decimal.Decimal, basisEfektif string) {
	if basis == "agregat" {
		if agregat.IsPositive() {
			return agregat, "agregat"
		}
		return decimal.Zero, "agregat"
	}
	return perKredit, "per_kredit"
}

// ModalPelengkapBatas adalah hasil penerapan sub-batas modal pelengkap menurut POJK
// No. 5/POJK.03/2015:
//   - komponen ber-instrumen <= instrumenMaxFrac x modal inti (Pasal 10 ayat (2));
//   - PPKA umum <= ppkaUmumMaxFrac x ATMR (Pasal 10 ayat (1) huruf c);
//   - total modal pelengkap <= pelengkapMaxFrac x modal inti (Pasal 3 ayat (2)).
//
// potongan* mencatat nilai yang tidak dapat diperhitungkan karena batas, supaya
// pembaca dapat memeriksa hitungan. Semua masukan dinegasikan lebih dulu ke nol:
// batas tidak boleh membalik tanda.
type ModalPelengkapBatas struct {
	Instrumen         decimal.Decimal
	SurplusRevaluasi  decimal.Decimal
	PPKAUmum          decimal.Decimal
	TotalEfektif      decimal.Decimal
	PotonganInstrumen decimal.Decimal
	PotonganPPKAUmum  decimal.Decimal
	PotonganTotal     decimal.Decimal
}

// BatasModalPelengkap menerapkan sub-batas modal pelengkap. Nilai yang tidak
// tersedia TIDAK diwakili nol di sini: pemanggil hanya memanggil fungsi ini untuk
// komponen yang benar-benar terisi, dan menandai komponen lain tidak tersedia.
func BatasModalPelengkap(instrumen, surplusRevaluasi, ppkaUmum, modalInti, atmr, pelengkapMaxFrac, instrumenMaxFrac, ppkaUmumMaxFrac decimal.Decimal) ModalPelengkapBatas {
	var out ModalPelengkapBatas

	kotorInstrumen := nonNegatif(instrumen)
	out.Instrumen = minDecimal(kotorInstrumen, nonNegatif(modalInti).Mul(instrumenMaxFrac))
	out.PotonganInstrumen = kotorInstrumen.Sub(out.Instrumen)

	kotorPPKA := nonNegatif(ppkaUmum)
	out.PPKAUmum = minDecimal(kotorPPKA, nonNegatif(atmr).Mul(ppkaUmumMaxFrac))
	out.PotonganPPKAUmum = kotorPPKA.Sub(out.PPKAUmum)

	out.SurplusRevaluasi = nonNegatif(surplusRevaluasi)

	kotor := out.Instrumen.Add(out.SurplusRevaluasi).Add(out.PPKAUmum)
	out.TotalEfektif = minDecimal(kotor, nonNegatif(modalInti).Mul(pelengkapMaxFrac))
	out.PotonganTotal = kotor.Sub(out.TotalEfektif)
	return out
}

func nonNegatif(d decimal.Decimal) decimal.Decimal {
	if d.IsNegative() {
		return decimal.Zero
	}
	return d
}

func minDecimal(a, b decimal.Decimal) decimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}
