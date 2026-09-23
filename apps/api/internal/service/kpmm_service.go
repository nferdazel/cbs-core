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
	kpmmMinFracKey                 = "kpmm.min_frac"
	kpmmModalIntiMinFracKey        = "kpmm.modal_inti_min_frac"
	kpmmModalIntiMinAmountKey      = "kpmm.modal_inti_min_amount"
	kpmmModalPelengkapMaxKey       = "kpmm.modal_pelengkap_max_frac"
	kpmmModalPelengkapInstrumenKey = "kpmm.modal_pelengkap_instrumen_max_frac"
	kpmmPPKAUmumRWAMaxKey          = "kpmm.ppka_umum_rwa_max_frac"
	kpmmDeductionBasisKey          = "kpmm.deduction_basis"
	kpmmBobotKeyPrefix             = "kpmm.rwa_frac."
	basisPengurangPerKredit        = "per_kredit"
	basisPengurangAgregat          = "agregat"
	kpmmDefaultBobotKonservatif    = 1
)

type kpmmService struct {
	reports domain.ReportService
	ckpn    domain.CKPNService
	config  domain.SystemConfigService
	// ppkaUmum menghitung PPKA umum (0,5% aset produktif lancar) dari kredit dan
	// penempatan. Opsional: bila nil, komponen PPKAUmum ditandai belum tersedia
	// seperti perilaku lama, sehingga uji unit tanpa modul ini tidak berubah.
	ppkaUmum domain.PPKAUmumService
}

// ppapRunDateProvider adalah kontrak opsional modul CKPN yang memberi tanggal bisnis
// run PPAP terakhir. Bila implementasi tidak memenuhinya (mis. stub uji), laporan
// memakai as_of apa adanya seperti sebelumnya sehingga tidak ada perilaku baru.
type ppapRunDateProvider interface {
	LastPPAPBusinessDate(ctx context.Context) (time.Time, bool, error)
}

// sameDateUTC membandingkan komponen tanggal (bukan jam/zona) dua waktu.
func sameDateUTC(a, b time.Time) bool {
	ay, am, ad := a.UTC().Date()
	by, bm, bd := b.UTC().Date()
	return ay == by && am == bm && ad == bd
}

// NewKPMMService menyusun penghitung KPMM baca-saja. ppkaUmumOpsional boleh kosong
// (mis. uji unit lama): tanpa itu komponen PPKA umum tetap belum tersedia.
func NewKPMMService(reports domain.ReportService, ckpn domain.CKPNService, config domain.SystemConfigService, ppkaUmumOpsional ...domain.PPKAUmumService) domain.KPMMService {
	var ppkaUmum domain.PPKAUmumService
	if len(ppkaUmumOpsional) > 0 {
		ppkaUmum = ppkaUmumOpsional[0]
	}
	return &kpmmService{reports: reports, ckpn: ckpn, config: config, ppkaUmum: ppkaUmum}
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

	// Ambang dibaca LEBIH DAHULU supaya batas modal pelengkap sudah tersedia saat
	// menyusun total modal. Sebelumnya urutannya terbalik: ModalPelengkapMaxFrac
	// masih nol saat batas diterapkan (tidak terlihat karena pelengkap belum pernah
	// terisi).
	report.KPMMMinFrac, _ = s.fracAtauGapFallback(ctx, kpmmMinFracKey, decimal.NewFromFloat(0.12))
	report.ModalIntiMinFrac, _ = s.fracAtauGapFallback(ctx, kpmmModalIntiMinFracKey, decimal.NewFromFloat(0.08))
	report.ModalIntiMinAmount, _ = s.fracAtauGapFallback(ctx, kpmmModalIntiMinAmountKey, decimal.NewFromInt(6000000000))
	report.ModalPelengkapMaxFrac, _ = s.fracAtauGapFallback(ctx, kpmmModalPelengkapMaxKey, decimal.NewFromInt(1))
	report.ModalPelengkapInstrumenMaxFrac, _ = s.fracAtauGapFallback(ctx, kpmmModalPelengkapInstrumenKey, decimal.NewFromFloat(0.50))
	report.PPKAUmumRWAMaxFrac, _ = s.fracAtauGapFallback(ctx, kpmmPPKAUmumRWAMaxKey, decimal.NewFromFloat(0.0125))

	// Rincian kelas modal dari pemetaan COA. Kode yang kelasnya belum pasti TIDAK ikut
	// (lihat ojkreport.KlasifikasiModalCOA) supaya tidak menjadi modal fiktif.
	report.ModalKelasCOA = kpmmModalKelasBaris(ojkreport.KlasifikasiModalCOA(bs.Rows))

	// Modal inti utama = ekuitas + laba/rugi tahun berjalan. Ini identitas laporan
	// sumber: aset = kewajiban + ekuitas + laba/rugi berjalan.
	modalIntiUtama := bs.TotalEquity.Add(bs.NetIncome)
	report.ModalIntiUtama = domain.KPMMKomponen{Nilai: modalIntiUtama, Tersedia: true}

	// Perbandingan PPKA-CKPN hanya sah pada tanggal bisnis run PPAP yang tersimpan.
	// Bila run terakhir berada SEBELUM akhir periode (mis. EOD PPAP jatuh di hari
	// sebelum tutup buku), memakai as_of akhir periode akan ditolak sebagai PPKA
	// basi sehingga baris KPMM Form 00.08 kosong. Modul CKPN memberi tanggal run
	// terakhir lewat kontrak opsional; bila tersedia dan tidak melewati as_of,
	// periode itulah yang dipakai dan dicatat pada laporan.
	asOfPPAP := asOf
	if p, ok := s.ckpn.(ppapRunDateProvider); ok {
		if last, ok2, err := p.LastPPAPBusinessDate(ctx); err == nil && ok2 && !last.After(asOf) {
			asOfPPAP = last
			if !sameDateUTC(last, asOf) {
				report.PPAPBusinessDate = last.Format(layoutTanggalBisnis)
			}
		}
	}

	report.PengurangModalInti, report.DeductionBasis = s.pengurangModalInti(ctx, asOfPPAP, actor, &gaps)

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

	// Komponen modal pelengkap (POJK 5/2015 Pasal 10). Data pembilangnya belum
	// dipisah dari bagan akun yang tersedia, sehingga tiap komponen ditandai TIDAK
	// TERSEDIA (bukan nol) beserta apa yang dibutuhkan bank.
	report.ModalPelengkapInstrumen = domain.KPMMKomponen{
		Alasan: "komponen modal pelengkap ber-instrumen (persetujuan OJK) tidak dapat dipisah dari bagan akun; daftar instrumen manual per bank belum tersedia",
	}
	report.SurplusRevaluasi = domain.KPMMKomponen{
		Alasan: "surplus revaluasi aset tetap belum berakun tersendiri pada bagan akun sehingga tidak dapat dipisah",
	}
	report.PPKAUmum = s.ppkaUmumKomponen(ctx, asOf, actor)
	report.ModalPelengkap = domain.KPMMKomponen{
		Alasan: "komponen modal pelengkap (instrumen dengan persetujuan OJK, surplus revaluasi aset tetap, " +
			"dan PPKA umum) belum dipisah dari data yang tersimpan, sehingga tidak dihitung",
	}

	// Total modal = modal inti + modal pelengkap SETELAH sub-batas: komponen
	// ber-instrumen <= 50% modal inti (Pasal 10 ayat (2)), PPKA umum <= 1,25% ATMR
	// (Pasal 10 ayat (1) huruf c), dan total <= 100% modal inti (Pasal 3 ayat (2)).
	// Komponen yang tidak tersedia tidak diwakili nol: field laporannya tetap
	// Tersedia=false beserta alasan, sedangkan nilaiTersedia hanya menyiapkan angka
	// untuk perhitungan batas.
	modalPelengkapDiperhitungkan := decimal.Zero
	if report.ModalInti.Tersedia {
		batas := ojkreport.BatasModalPelengkap(
			nilaiTersedia(report.ModalPelengkapInstrumen),
			nilaiTersedia(report.SurplusRevaluasi),
			nilaiTersedia(report.PPKAUmum),
			report.ModalInti.Nilai,
			report.ATMR.Nilai,
			report.ModalPelengkapMaxFrac,
			report.ModalPelengkapInstrumenMaxFrac,
			report.PPKAUmumRWAMaxFrac,
		)
		modalPelengkapDiperhitungkan = batas.TotalEfektif
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
	report.Lengkap, report.AlasanTidakLengkap = kpmmKelengkapan(report)
	return report, nil
}

// kpmmKelengkapan menandai apakah angka laporan boleh dibaca sebagai final. Ini
// memisahkan "sementara" dari "lengkap" tanpa mengubah angka apa pun: komponen
// modal yang belum tersedia (mis. modal pelengkap) atau ATMR yang belum
// terkategori membuat rasio hanya batas bawah, jadi laporan ditandai belum
// lengkap beserta alasannya.
func kpmmKelengkapan(r domain.KPMMReport) (bool, []string) {
	var alasan []string
	if !r.ModalPelengkap.Tersedia {
		alasan = append(alasan, "modal pelengkap belum tersedia")
	}
	if !r.PPKAUmum.Tersedia {
		alasan = append(alasan, "PPKA umum belum tersedia")
	}
	if !r.ModalPelengkapInstrumen.Tersedia {
		alasan = append(alasan, "komponen modal pelengkap ber-instrumen belum tersedia")
	}
	if !r.SurplusRevaluasi.Tersedia {
		alasan = append(alasan, "surplus revaluasi aset tetap belum tersedia")
	}
	if !r.ModalInti.Tersedia {
		alasan = append(alasan, "modal inti belum final: "+r.ModalInti.Alasan)
	}
	if len(r.ATMRTidakTerkategori) > 0 {
		alasan = append(alasan, "ATMR belum lengkap: pos neraca belum terkategori: "+
			strings.Join(r.ATMRTidakTerkategori, ", "))
	}
	if !r.ATMR.Tersedia {
		alasan = append(alasan, "ATMR belum dapat dihitung")
	}
	if len(r.ParameterGaps) > 0 {
		alasan = append(alasan, "parameter konfigurasi belum diisi: "+strings.Join(r.ParameterGaps, ", "))
	}
	if len(alasan) == 0 {
		return true, nil
	}
	return false, dedupSorted(alasan)
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

// nilaiTersedia mengembalikan nilai komponen bila tersedia, selain itu nol. Hanya
// dipakai untuk perhitungan batas; pelaporan tetap memakai field Tersedia/Alasan.
func nilaiTersedia(k domain.KPMMKomponen) decimal.Decimal {
	if !k.Tersedia {
		return decimal.Zero
	}
	return k.Nilai
}

// kpmmModalKelasBaris menyusun rincian kelas modal terurut agar laporan stabil.
func kpmmModalKelasBaris(perKelas map[string]decimal.Decimal) []domain.KPMMModalKelasBaris {
	if len(perKelas) == 0 {
		return nil
	}
	kelas := make([]string, 0, len(perKelas))
	for k := range perKelas {
		kelas = append(kelas, k)
	}
	sort.Strings(kelas)
	out := make([]domain.KPMMModalKelasBaris, 0, len(kelas))
	for _, k := range kelas {
		out = append(out, domain.KPMMModalKelasBaris{Kelas: k, Nilai: perKelas[k]})
	}
	return out
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

// ppkaUmumKomponen menghitung PPKA umum dari modul khusus bila tersambung. Tanpa
// modul, komponen ditandai belum tersedia seperti perilaku lama. Sumber kredit
// diwajibkan tersedia agar angka tidak tampak final padahal hanya sebagian.
func (s *kpmmService) ppkaUmumKomponen(ctx context.Context, asOf time.Time, actor domain.Actor) domain.KPMMKomponen {
	if s.ppkaUmum == nil {
		return domain.KPMMKomponen{
			Alasan: "modul PPKA umum tidak tersambung; PPKA umum minimum 0,5% aset produktif lancar " +
				"(POJK 1/2024 Pasal 19 ayat (2)) belum dihitung",
		}
	}
	summary, err := s.ppkaUmum.Hitung(ctx, asOf, actor)
	if err != nil {
		return domain.KPMMKomponen{Alasan: "PPKA umum gagal dihitung: " + err.Error()}
	}
	if !summary.SumberKredit {
		return domain.KPMMKomponen{Alasan: "kredit lancar belum dapat dihitung sehingga PPKA umum belum tersedia"}
	}
	komponen := domain.KPMMKomponen{Nilai: summary.TotalPPKA, Tersedia: true}
	if !summary.Lengkap {
		komponen.Alasan = "batas bawah; " + strings.Join(summary.AlasanTidakLengkap, "; ")
	}
	return komponen
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
	catatan = append(catatan,
		"Sub-batas modal pelengkap yang berlaku: komponen ber-instrumen <= 50% modal inti (POJK 5/2015 Pasal 10 ayat (2)); PPKA umum <= 1,25% ATMR (Pasal 10 ayat (1) huruf c); total modal pelengkap <= 100% modal inti (Pasal 3 ayat (2)). Batas diterapkan hanya pada komponen yang tersedia; komponen tidak tersedia tidak diwakili nol.")
	if !r.PPKAUmum.Tersedia {
		catatan = append(catatan, "PPKA umum belum dihitung: perlu pemetaan bagan akun aset produktif yang terverifikasi dan kualitas aset produktif per pos (POJK No. 1/2024 Pasal 19 ayat (2)).")
	}
	if r.DeductionBasis == basisPengurangAgregat {
		catatan = append(catatan, "Pengurang modal inti memakai selisih AGREGAT PPKA-CKPN, bukan per kredit.")
	} else {
		catatan = append(catatan, "Pengurang modal inti memakai jumlah selisih PER KREDIT max(PPKA-CKPN,0) (konservatif).")
	}
	if r.PPAPBusinessDate != "" {
		catatan = append(catatan, "Perbandingan PPKA-CKPN memakai tanggal bisnis run PPAP terakhir "+r.PPAPBusinessDate+
			", bukan akhir periode; PPKA yang tersimpan memang milik tanggal tersebut.")
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
