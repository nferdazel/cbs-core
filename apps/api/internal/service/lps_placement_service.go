package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// lpsPlacementService menghitung PPKA penempatan pada bank lain yang dijamin LPS menurut
// Pasal 23 POJK No. 1 Tahun 2024. Baca-saja: tidak memposting jurnal dan tidak menulis
// state, karena bank belum memutuskan bagaimana PPKA penempatan dicatat di GL.
type lpsPlacementService struct {
	repo   domain.LPSPlacementRepository
	config domain.SystemConfigService
}

func NewLPSPlacementService(repo domain.LPSPlacementRepository, config domain.SystemConfigService) domain.LPSPlacementService {
	return &lpsPlacementService{repo: repo, config: config}
}

// Calculate menghitung PPKA umum dan khusus atas penempatan setelah dikurangi bagian
// yang dijamin LPS. Selama saklar mati tidak ada query dan tidak ada angka yang berubah.
func (s *lpsPlacementService) Calculate(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.LPSPlacementSummary, error) {
	asOf = asOf.UTC()
	summary := domain.LPSPlacementSummary{AsOf: asOf}

	// Tanpa config, saklar tidak dapat dibaca; perlakukan sebagai mati agar perilaku
	// pemanggil lama/uji tetap sama seperti sebelum modul ini ada.
	if s.config == nil {
		return summary, nil
	}
	if !s.config.GetBool(ctx, domain.LPSPlacementEnabledKey, false) {
		return summary, nil
	}
	summary.Enabled = true

	// Parameter salah format/rentang DITOLAK, tidak dijatuhkan menjadi nol.
	cap, err := lpsGuaranteeCap(ctx, s.config)
	if err != nil {
		return summary, err
	}
	summary.GuaranteeCap = cap

	// Saklar hidup tanpa repositori adalah konfigurasi yang salah, bukan alasan untuk
	// diam-diam mengembalikan nol. Tersambungnya repo adalah tanggung jawab perakitan.
	if s.repo == nil {
		return summary, fmt.Errorf("modul pengurang PPKA penempatan LPS aktif tetapi repositori tidak tersedia")
	}

	placements, err := s.repo.ListPlacements(ctx, asOf, actor)
	if err != nil {
		// Galat baca tidak boleh menjadi laporan kosong yang tampak sah.
		return summary, fmt.Errorf("mengambil penempatan pada bank lain: %w", err)
	}
	summary.Total = len(placements)

	rates := collectibilityRates(ctx, s.config)

	// Validasi dan hitung lebih dulu, sekaligus mengumpulkan pengurang per bank lawan
	// untuk menegakkan plafon LPS per nasabah pada satu bank.
	type counted struct {
		placement domain.LPSPlacement
		calc      domain.LPSPlacementPPKACalculation
	}
	valid := make([]counted, 0, len(placements))
	deductionByBank := make(map[string]decimal.Decimal)

	for _, p := range placements {
		if err := p.Validate(); err != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, domain.LPSPlacementFailure{
				PlacementID:      p.ID,
				CounterpartyBank: p.CounterpartyBank,
				Error:            err.Error(),
			})
			continue
		}
		calc, err := domain.CalculateLPSPlacementPPKA(p, rates)
		if err != nil {
			summary.Failed++
			summary.Failures = append(summary.Failures, domain.LPSPlacementFailure{
				PlacementID:      p.ID,
				CounterpartyBank: p.CounterpartyBank,
				Error:            err.Error(),
			})
			continue
		}
		deductionByBank[p.CounterpartyBank] = deductionByBank[p.CounterpartyBank].Add(calc.Deduction)
		valid = append(valid, counted{placement: p, calc: calc})
	}

	// Plafon diuji atas JUMLAH jaminan per bank lawan, sesuai Penjelasan Pasal 23
	// ("untuk setiap nasabah pada satu bank"). Kelebihan ditolak agar bank memperbaiki
	// penandaannya, bukan dipotong diam-diam sehingga angka PPKA terlihat sah.
	for bank, total := range deductionByBank {
		if total.GreaterThan(cap) {
			return summary, fmt.Errorf("%w: bank %s dijamin %s, plafon %s",
				domain.ErrLPSGuaranteeOverCap, bank, total, cap)
		}
	}

	for _, c := range valid {
		item := domain.LPSPlacementItem{
			PlacementID:      c.placement.ID,
			COACode:          c.placement.COACode,
			CounterpartyBank: c.placement.CounterpartyBank,
			PlacementType:    c.placement.PlacementType,
			Collectibility:   c.calc.Collectibility,
			AsOf:             c.placement.AsOf,
			Outstanding:      c.calc.Outstanding,
			Deduction:        c.calc.Deduction,
			Base:             c.calc.Base,
			GeneralPPKA:      c.calc.GeneralPPKA,
			SpecificPPKA:     c.calc.SpecificPPKA,
			PPKA:             c.calc.PKKATarget,
			AppliesTo:        c.calc.AppliesTo,
		}
		summary.Items = append(summary.Items, item)
		summary.Processed++
		summary.TotalOutstanding = summary.TotalOutstanding.Add(item.Outstanding)
		summary.TotalDeduction = summary.TotalDeduction.Add(item.Deduction)
		summary.TotalGeneralPPKA = summary.TotalGeneralPPKA.Add(item.GeneralPPKA)
		summary.TotalSpecificPPKA = summary.TotalSpecificPPKA.Add(item.SpecificPPKA)
		summary.TotalPPKA = summary.TotalPPKA.Add(item.PPKA)
	}
	return summary, nil
}

// lpsGuaranteeCap membaca plafon penjaminan LPS per nasabah pada satu bank. Tiga keadaan
// dibedakan: tidak diisi (memakai bawaan POJK), nilai sah, dan nilai ada tetapi salah.
// Nilai salah TIDAK boleh menjadi nol diam-diam.
func lpsGuaranteeCap(ctx context.Context, config domain.SystemConfigService) (decimal.Decimal, error) {
	raw := strings.TrimSpace(config.GetString(ctx, domain.LPSGuaranteeCapKey, ""))
	if raw == "" {
		return domain.LPSGuaranteeCapDefault(), nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%w: plafon penjaminan LPS (kunci %s): nilai %q bukan angka desimal yang sah",
			domain.ErrLPSParameterInvalid, domain.LPSGuaranteeCapKey, raw)
	}
	if !d.IsPositive() {
		return decimal.Zero, fmt.Errorf("%w: plafon penjaminan LPS (kunci %s) harus lebih besar dari nol: %s",
			domain.ErrLPSParameterInvalid, domain.LPSGuaranteeCapKey, d)
	}
	return d, nil
}

var _ domain.LPSPlacementService = (*lpsPlacementService)(nil)
