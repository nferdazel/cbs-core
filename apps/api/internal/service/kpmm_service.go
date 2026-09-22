package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/ojkreport"
	"github.com/shopspring/decimal"
)

// kpmm_service.go merakit laporan KPMM dari sumber yang sudah ada tanpa menghitung
// ulang rumus akuntansi: laporan posisi keuangan (journal-based), perbandingan PPKA
// vs CKPN (modul CKPN), dan parameter kebijakan di system_config.
//
// Batas yang disengaja (jangan diisi nol diam-diam):
//   - Modal pelengkap: komponen yang memerlukan persetujuan OJK, surplus revaluasi
//     aset tetap, dan PPKA umum belum dipisah dari data yang ada, sehingga
//     ModalPelengkap ditandai Tersedia=false.
//   - Pengurang modal inti lain (AYDA/properti terbengkalai >1 tahun, pajak
//     tangguhan, goodwill, disagio) tidak dapat dihitung dari data agregat yang
//     tersedia; disebut pada Catatan.
//   - ATMR kredit memakai satu bobot agregat karena jenis agunan per kredit belum
//     tersedia pada perhitungan ATMR; disebut pada Catatan.

// Kunci konfigurasi KPMM. Dibaca sebagai literali agar uji invarian seed menangkap
// kunci yang belum di-seed (lihat config_seed_invariant_test.go).
const (
	kpmmMinFracKey              = "kpmm.min_frac"
	kpmmModalIntiMinFracKey     = "kpmm.modal_inti_min_frac"
	kpmmModalIntiMinAmountKey   = "kpmm.modal_inti_min_amount"
	kpmmModalPelengkapMaxKey    = "kpmm.modal_pelengkap_max_frac"
	kpmmPPKAUmumRWAMaxKey       = "kpmm.ppka_umum_rwa_max_frac"
	kpmmDeductionBasisKey       = "kpmm.deduction_basis"
	kpmmBobotKeyPrefix          = "kpmm.rwa_frac."
	basisPengurangPerKredit     = "per_kredit"
	basisPengurangAgregat       = "agregat"
	kpmmDefaultBobotKonservatif = 1
)

type kpmmService struct {
	reports domain.ReportService
	ckpn    domain.CKPNService
	config  domain.SystemConfigService
}

// NewKPMMService menyusun penghitung KPMM baca-saja.
func NewKPMMService(reports domain.ReportService, ckpn domain.CKPNService, config domain.SystemConfigService) domain.KPMMService {
	return &kpmmService{reports: reports, ckpn: ckpn, config: config}
}

// Hitung menyusun laporan KPMM untuk posisi asOf. Bila komponen kunci tidak dapat
// dihitung, hasilnya mengembalikan komponen Tersedia=false beserta alasannya dan
// TIDAK menampilkan rasio yang menyesatkan.
func (s *kpmmService) Hitung(ctx context.Context, asOf time.Time, book string, actor domain.Actor) (domain.KPMMReport, error) {
	report := domain.KPMMReport{
		AsOf: asOf.UTC(),
		Book: book,
	}

	if s.reports == nil {
		return report, fmt.Errorf("layanan laporan belum dikonfigurasi")
	}

	bs, err := s.reports.GetBalanceSheet(ctx, asOf, book)
	if err != nil {
		return report, fmt.Errorf("membaca laporan posisi keuangan untuk KPMM: %w", err)
	}
	if bs == nil {
		return report, fmt.Errorf("laporan posisi keuangan kosong")
	}

	var gaps []string
	bobot := s.bobotRisiko(ctx, &gaps)
	hasilATMR := ojkreport.HitungATMR(bs.Rows, bobot)
	report.ATMRBaris = hasilATMR.Baris
	report.ATMRTidakTerkategori = dedupSorted(hasilATMR.TidakTerkategori)
	report.ATMR = domain.KPMMKomponen{Nilai: hasilATMR.Total, Tersedia: true}
	if len(hasilATMR.Baris) == 0 {
		report.ATMR.Tersedia = false
		report.ATMR.Alasan = "tidak ada pos aset pada laporan posisi keuangan"
	} else if len(report.ATMRTidakTerkategori) > 0 {
		report.ATMR.Tersedia = false
		report.ATMR.Alasan = "ada pos neraca yang belum dipetakan ke pos OJK sehingga ATMR belum lengkap: " +
			strings.Join(report.ATMRTidakTerkategori, ", ")
	}

	// Modal inti utama = ekuitas + laba/rugi tahun berjalan. Ini identitas laporan
	// sumber: aset = kewajiban + ekuitas + laba/rugi berjalan.
	modalIntiUtama := bs.TotalEquity.Add(bs.NetIncome)
	report.ModalIntiUtama = domain.KPMMKomponen{Nilai: modalIntiUtama, Tersedia: true}

	report.PengurangModalInti, report.DeductionBasis = s.pengurangModalInti(ctx, asOf, actor, &gaps)

	if report.PengurangModalInti.Tersedia {
		report.ModalInti = domain.KPMMKomponen{
			Nilai:    modalIntiUtama.Sub(report.PengurangModalInti.Nilai),
			Tersedia: true,
		}
	} else {
		report.ModalInti = domain.KPMMKomponen{
			Nilai:  modalIntiUtama,
			Alasan: "pengurang modal inti (selisih PPKA-CKPN) belum dapat dihitung: " + report.PengurangModalInti.Alasan,
		}
	}

	// Modal pelengkap belum dapat dipisah dari data yang ada.
	report.ModalPelengkap = domain.KPMMKomponen{
		Alasan: "komponen modal pelengkap (instrumen dengan persetujuan OJK, surplus revaluasi aset tetap, " +
			"dan PPKA umum) belum dipisah dari data yang tersimpan, sehingga tidak dihitung",
	}

	// Total modal = modal inti + modal pelengkap. Batas modal pelengkap 100% modal
	// inti diterapkan lebih dulu bila kelak pelengkap terisi.
	modalPelengkapDiperhitungkan := decimal.Zero
	if report.ModalPelengkap.Tersedia && report.ModalInti.Tersedia {
		modalPelengkapDiperhitungkan = report.ModalPelengkap.Nilai
		if maks := report.ModalInti.Nilai.Mul(report.ModalPelengkapMaxFrac); modalPelengkapDiperhitungkan.GreaterThan(maks) {
			modalPelengkapDiperhitungkan = maks
		}
	}
	if report.ModalInti.Tersedia {
		report.TotalModal = domain.KPMMKomponen{
			Nilai:    report.ModalInti.Nilai.Add(modalPelengkapDiperhitungkan),
			Tersedia: true,
		}
		if !report.ModalPelengkap.Tersedia {
			report.TotalModal.Alasan = "tidak termasuk modal pelengkap yang belum dapat dihitung (batas bawah modal)"
		}
	} else {
		report.TotalModal = domain.KPMMKomponen{
			Nilai:  report.ModalInti.Nilai,
			Alasan: "modal inti belum final: " + report.ModalInti.Alasan,
		}
	}

	report.KPMMMinFrac, _ = s.fracAtauGapFallback(ctx, kpmmMinFracKey, decimal.NewFromFloat(0.12))
	report.ModalIntiMinFrac, _ = s.fracAtauGapFallback(ctx, kpmmModalIntiMinFracKey, decimal.NewFromFloat(0.08))
	report.ModalIntiMinAmount, _ = s.fracAtauGapFallback(ctx, kpmmModalIntiMinAmountKey, decimal.NewFromInt(6000000000))
	report.ModalPelengkapMaxFrac, _ = s.fracAtauGapFallback(ctx, kpmmModalPelengkapMaxKey, decimal.NewFromInt(1))
	report.PPKAUmumRWAMaxFrac, _ = s.fracAtauGapFallback(ctx, kpmmPPKAUmumRWAMaxKey, decimal.NewFromFloat(0.0125))

	if report.ATMR.Tersedia && report.TotalModal.Tersedia {
		if nilai, ok := ojkreport.RumusKPMM(report.TotalModal.Nilai, report.ATMR.Nilai); ok {
			report.RasioKPMM = domain.KPMMKomponen{Nilai: nilai, Tersedia: true}
		} else {
			report.RasioKPMM = domain.KPMMKomponen{Alasan: "ATMR nol sehingga rasio KPMM tidak dapat dihitung"}
		}
	} else {
		report.RasioKPMM = domain.KPMMKomponen{Alasan: alasanRasio(report)}
	}
	if report.ATMR.Tersedia && report.ModalInti.Tersedia {
		if nilai, ok := ojkreport.RumusKPMM(report.ModalInti.Nilai, report.ATMR.Nilai); ok {
			report.RasioModalInti = domain.KPMMKomponen{Nilai: nilai, Tersedia: true}
		} else {
			report.RasioModalInti = domain.KPMMKomponen{Alasan: "ATMR nol sehingga rasio modal inti tidak dapat dihitung"}
		}
	} else {
		report.RasioModalInti = domain.KPMMKomponen{Alasan: alasanRasio(report)}
	}

	report.Catatan = kpmmCatatan(report)
	report.ParameterGaps = dedupSorted(gaps)
	return report, nil
}

// alasanRasio merangkum komponen yang membuat rasio belum dapat dihitung.
func alasanRasio(r domain.KPMMReport) string {
	var sebab []string
	if !r.ATMR.Tersedia {
		sebab = append(sebab, "ATMR belum lengkap")
	}
	if !r.ModalInti.Tersedia {
		sebab = append(sebab, "pengurang modal inti belum dapat dihitung")
	}
	if len(sebab) == 0 {
		return "komponen rasio belum tersedia"
	}
	return "rasio belum dapat dihitung: " + strings.Join(sebab, " dan ")
}

// bobotRisiko membaca bobot tiap kategori dari konfigurasi. Kategori yang kuncinya
// kosong dicatat sebagai ParameterGap dan diberi bobot konservatif 100% agar ATMR
// tidak tampak lebih kecil dari kenyataan.
func (s *kpmmService) bobotRisiko(ctx context.Context, gaps *[]string) ojkreport.BobotRisikoATMR {
	out := ojkreport.BobotRisikoATMR{}
	for _, kategori := range ojkreport.KategoriRisikoATMR() {
		key := kpmmBobotKeyPrefix + kategori
		bobot, gap := s.fracAtauGapFallback(ctx, key, decimal.NewFromInt(kpmmDefaultBobotKonservatif))
		if gap {
			*gaps = append(*gaps, key)
		}
		out[kategori] = bobot
	}
	return out
}

// fracAtauGapFallback membaca nilai fraksi. Bila kunci benar-benar kosong/tidak ada
// (dapat dibedakan lewat ConfigRawValueReader) maka gap=true; pemanggil tetap
// memakai fallback konservatif. Bila pembaca tidak mendukung pembedaan itu, nilai
// dianggap ada dan fallback hanya jaring pengaman.
func (s *kpmmService) fracAtauGapFallback(ctx context.Context, key string, fallback decimal.Decimal) (decimal.Decimal, bool) {
	if s.config == nil {
		return fallback, true
	}
	if raw, ok := s.config.(domain.ConfigRawValueReader); ok {
		value, present := raw.RawValue(ctx, key)
		if !present || strings.TrimSpace(value) == "" {
			return fallback, true
		}
		return s.config.GetDecimal(ctx, key, fallback), false
	}
	return s.config.GetDecimal(ctx, key, fallback), false
}

// pengurangModalInti menghitung selisih PPKA > CKPN sebagai pengurang modal inti
// (SEOJK 21/SEOJK.03/2024 butir 1.1.6). Pilihan per_kredit/agregat dibaca dari
// konfigurasi; per_kredit adalah nilai awal karena konservatif.
func (s *kpmmService) pengurangModalInti(ctx context.Context, asOf time.Time, actor domain.Actor, gaps *[]string) (domain.KPMMKomponen, string) {
	basis := basisPengurangPerKredit
	if s.config != nil {
		if raw, ok := s.config.(domain.ConfigRawValueReader); ok {
			if v, present := raw.RawValue(ctx, kpmmDeductionBasisKey); present && strings.TrimSpace(v) != "" {
				basis = strings.TrimSpace(v)
			} else {
				*gaps = append(*gaps, kpmmDeductionBasisKey)
			}
		} else {
			basis = s.config.GetString(ctx, kpmmDeductionBasisKey, basisPengurangPerKredit)
		}
	}
	if basis != basisPengurangAgregat {
		basis = basisPengurangPerKredit
	}

	if s.ckpn == nil {
		return domain.KPMMKomponen{Alasan: "modul CKPN belum dikonfigurasi"}, basis
	}
	summary, err := s.ckpn.Compare(ctx, asOf, actor)
	if err != nil {
		return domain.KPMMKomponen{Alasan: "perbandingan PPKA-CKPN gagal: " + err.Error()}, basis
	}
	// Kedua saklar CKPN mati: tidak ada kredit yang dibaca.
	if !summary.Enabled && !summary.ShadowMode && summary.Processed == 0 {
		return domain.KPMMKomponen{
			Alasan: "CKPN belum dihitung (ckpn.enabled dan mode bayangan mati); selisih PPKA-CKPN tidak tersedia",
		}, basis
	}
	nilai, basisEfektif := ojkreport.PilihPengurangModalInti(basis, summary.ModalIntiDeduction, summary.Difference)
	komponen := domain.KPMMKomponen{Nilai: nilai, Tersedia: true}
	if summary.Failed > 0 {
		komponen.Tersedia = false
		komponen.Alasan = fmt.Sprintf("%d kredit gagal dihitung CKPN-nya sehingga selisih PPKA-CKPN belum lengkap", summary.Failed)
	}
	return komponen, basisEfektif
}

// kpmmCatatan merangkum batas perhitungan yang harus dibaca bersama angkanya.
func kpmmCatatan(r domain.KPMMReport) []string {
	catatan := []string{
		"ATMR BPR hanya memuat aset neraca berbobot risiko (POJK 5/POJK.03/2015); tidak ada beban ATMR operasional/pasar.",
		"Bobot risiko kredit memakai satu bobot agregat karena jenis agunan per kredit belum tersedia pada perhitungan ATMR.",
		"AYDA/properti terbengkalai yang melampaui 1 tahun, pajak tangguhan, goodwill, dan disagio belum dapat dihitung dari data agregat sehingga belum dikurangkan dari modal inti.",
	}
	if !r.ModalPelengkap.Tersedia {
		catatan = append(catatan, "Modal pelengkap belum dihitung; total modal dan rasio KPMM adalah batas bawah (konservatif).")
	}
	if r.DeductionBasis == basisPengurangAgregat {
		catatan = append(catatan, "Pengurang modal inti memakai selisih AGREGAT PPKA-CKPN, bukan per kredit.")
	} else {
		catatan = append(catatan, "Pengurang modal inti memakai jumlah selisih PER KREDIT max(PPKA-CKPN,0) (konservatif).")
	}
	return catatan
}

// dedupSorted mengembalikan salinan terurut tanpa duplikat.
func dedupSorted(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}
