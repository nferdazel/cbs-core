package service

import (
	"context"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// ppka_umum_service.go merakit PPKA umum (POJK 1/2024 Pasal 19 ayat (2): minimum
// 0,5% aset produktif lancar) dari data operasional yang sudah ada:
//
//   - kredit lancar: evaluasi PPAP yang sama dengan laporan PPAP (agunan tunai
//     dikecualikan menurut Pasal 19 ayat (4) huruf b, tepat seperti PPAP harian);
//   - penempatan pada bank lain: modul Pasal 23 (jaminan LPS) bila saklarnya hidup.
//
// Keduanya memakai satu tarif, ppap.rate_frac.gol_1. Modul ini baca-saja: tidak
// menjurnal dan tidak mengubah state. Bila salah satu sumber belum tersedia,
// hasilnya ditandai Lengkap=false beserta alasan, bukan diisi nol lalu dianggap final.
type ppkaUmuServiceImpl struct {
	ppap       domain.PPAPService
	placements domain.LPSPlacementService
	config     domain.SystemConfigService
}

// NewPPKAUmumService menyusun penghitung PPKA umum. ppap dan placements boleh nil
// (lingkungan uji tanpa modul itu); sumber yang nil dihitung sebagai belum tersedia.
func NewPPKAUmumService(ppap domain.PPAPService, placements domain.LPSPlacementService, config domain.SystemConfigService) domain.PPKAUmumService {
	return &ppkaUmuServiceImpl{ppap: ppap, placements: placements, config: config}
}

// Hitung menghitung PPKA umum per posisi. Galat baca dari sumber yang ada
// dikembalikan, bukan diubah menjadi angka lebih kecil yang tampak sah.
func (s *ppkaUmuServiceImpl) Hitung(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.PPKAUmumSummary, error) {
	asOf = asOf.UTC()
	summary := domain.PPKAUmumSummary{AsOf: asOf}

	rate := decimal.Zero
	if s.config != nil {
		rates := collectibilityRates(ctx, s.config)
		rate = decimal.Decimal(rates[domain.KolLancar])
	}
	summary.RateFrac = rate

	var alasan []string

	// 1. Kredit lancar.
	if s.ppap != nil {
		run, err := s.ppap.Preview(ctx, asOf, actor)
		if err != nil {
			return summary, fmt.Errorf("menghitung PPKA umum dari kredit lancar: %w", err)
		}
		for _, item := range run.Items {
			if item.Collectibility == domain.KolLancar {
				summary.KreditLancarDasar = summary.KreditLancarDasar.Add(item.Exposure)
			}
		}
		summary.KreditLancarPPKA = domain.RoundToRupiah(summary.KreditLancarDasar.Mul(rate))
		summary.SumberKredit = true
		if run.Failed > 0 {
			alasan = append(alasan, fmt.Sprintf("%d kredit gagal dievaluasi sehingga PPKA umum kredit belum lengkap", run.Failed))
		}
	} else {
		alasan = append(alasan, "modul PPAP tidak tersedia sehingga kredit lancar belum dihitung")
	}

	// 2. Penempatan pada bank lain (Pasal 23). Saklar ppap.lps.enabled bawaan false;
	// saat mati tidak ada data penempatan dan komponen ini ditandai belum tersedia.
	if s.placements != nil {
		lps, err := s.placements.Calculate(ctx, asOf, actor)
		if err != nil {
			return summary, fmt.Errorf("menghitung PPKA umum dari penempatan pada bank lain: %w", err)
		}
		if lps.Enabled {
			for _, item := range lps.Items {
				if item.Collectibility == domain.KolLancar {
					summary.PenempatanLancarDasar = summary.PenempatanLancarDasar.Add(item.Base)
				}
			}
			summary.PenempatanLancarPPKA = lps.TotalGeneralPPKA
			summary.SumberPenempatan = true
			if lps.Failed > 0 {
				alasan = append(alasan, fmt.Sprintf("%d penempatan gagal dihitung sehingga PPKA umum penempatan belum lengkap", lps.Failed))
			}
		} else {
			alasan = append(alasan, "saklar ppap.lps.enabled mati sehingga penempatan pada bank lain belum dihitung")
		}
	} else {
		alasan = append(alasan, "modul penempatan pada bank lain tidak tersedia")
	}

	summary.TotalDasar = summary.KreditLancarDasar.Add(summary.PenempatanLancarDasar)
	summary.TotalPPKA = summary.KreditLancarPPKA.Add(summary.PenempatanLancarPPKA)
	summary.AlasanTidakLengkap = dedupSorted(alasan)
	summary.Lengkap = summary.SumberKredit && summary.SumberPenempatan && len(summary.AlasanTidakLengkap) == 0
	return summary, nil
}

var _ domain.PPKAUmumService = (*ppkaUmuServiceImpl)(nil)
