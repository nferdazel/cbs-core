package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// lpsTxRunner membuka satu transaksi untuk operasi tulis modul ini. Service memegang
// runner (bukan *sql.DB langsung) mengikuti pola ckpnTxRunner/ppapTxRunner di paket ini.
type lpsTxRunner interface {
	Run(ctx context.Context, fn func(tx any) error) error
}

type sqlLPSTxRunner struct{ db *sql.DB }

func (r sqlLPSTxRunner) Run(ctx context.Context, fn func(tx any) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// lpsPlacementService menghitung PPKA penempatan pada bank lain yang dijamin LPS menurut
// Pasal 23 POJK No. 1 Tahun 2024. Calculate baca-saja: tidak memposting jurnal dan tidak
// menulis state, karena bank belum memutuskan bagaimana PPKA penempatan dicatat di GL.
// AssessCKPN menulis asesmen CKPN per penempatan (kolom XII/XXI Form 05.00) dan juga
// TIDAK memposting jurnal.
type lpsPlacementService struct {
	txRunner lpsTxRunner
	repo     domain.LPSPlacementRepository
	config   domain.SystemConfigService
}

func NewLPSPlacementService(db *sql.DB, repo domain.LPSPlacementRepository, config domain.SystemConfigService) domain.LPSPlacementService {
	return &lpsPlacementService{txRunner: sqlLPSTxRunner{db: db}, repo: repo, config: config}
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

// AssessCKPN menyimpan asesmen CKPN satu penempatan pada bank lain. Saklar
// ckpn.pabl.enabled diperiksa lebih dulu: selama mati permintaan DITOLAK dengan
// ErrCKPNPABLDisabled, bukan diam-diam sukses tanpa menyimpan apa pun. Asesmen berjalan
// dalam satu transaksi (kunci baris lalu tulis kolom + jejak) dan tidak memposting jurnal.
func (s *lpsPlacementService) AssessCKPN(ctx context.Context, placementID uuid.UUID, input domain.PABLCKPNInput, actor domain.Actor) (domain.LPSPlacement, error) {
	if s.config == nil || !s.config.GetBool(ctx, domain.CKPNPABLEnabledKey, false) {
		return domain.LPSPlacement{}, domain.ErrCKPNPABLDisabled
	}
	if err := input.Validate(); err != nil {
		return domain.LPSPlacement{}, err
	}
	// Saklar hidup tanpa transaction runner adalah perakitan yang salah; jangan
	// diam-diam mengembalikan sukses.
	if s.txRunner == nil {
		return domain.LPSPlacement{}, fmt.Errorf("modul CKPN penempatan aktif tetapi transaction runner tidak dikonfigurasi")
	}

	var result domain.LPSPlacement
	err := s.txRunner.Run(ctx, func(tx any) error {
		placement, err := s.repo.LockPlacementTx(ctx, tx, placementID)
		if err != nil {
			return err
		}
		// Cakupan cabang ditegakkan SETELAH baris dikunci, seperti jalur tulis lain:
		// penempatan di cabang lain ditolak ErrCrossBranchAccess.
		if !actor.CanAccessBranch(placement.BranchCode) {
			return domain.ErrCrossBranchAccess
		}
		actorName := actor.Username
		if actorName == "" {
			actorName = actor.UserID.String()
		}
		// Carrying asesmen adalah outstanding penempatan saat ini; target adalah CKPN
		// yang dipilih bank.
		if err := s.repo.RecordCKPNTx(ctx, tx, placement.ID, input, placement.Outstanding, actorName); err != nil {
			return err
		}
		assessedAt := time.Now().UTC()
		result = *placement
		result.CKPN = &domain.LPSPlacementCKPN{
			Method:            input.Method,
			Significant:       input.Significant,
			ObjectiveEvidence: input.ObjectiveEvidence,
			RequiredCKPN:      input.RequiredCKPN,
			IndividualTarget:  input.IndividualTarget,
			AssessedAt:        &assessedAt,
		}
		return nil
	})
	if err != nil {
		return domain.LPSPlacement{}, err
	}
	return result, nil
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
