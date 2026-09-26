package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// bmpkService merakit laporan BMPK: membaca paparan per pihak terkait (satu query
// agregat di repositori), melengkapi nama nasabah secara batch, lalu menguji batas.
//
// Layanan ini BACA-SAJA: pengisian pihak terkait dan batas dilakukan bank lewat
// SQL/seed (migrasi 000103), belum ada endpoint tulis. Tidak ada jurnal yang diposting
// dan tidak ada state yang diubah.
type bmpkService struct {
	repo   domain.BMPKRepository
	config domain.SystemConfigService
	names  domain.BMPKCustomerNamer
}

// NewBMPKService merakit layanan BMPK. names boleh nil; tanpa itu nama pihak terkait
// dibiarkan kosong (laporan menuliskannya sebagai belum tersedia), bukan ditebak.
func NewBMPKService(repo domain.BMPKRepository, config domain.SystemConfigService, names domain.BMPKCustomerNamer) domain.BMPKService {
	return &bmpkService{repo: repo, config: config, names: names}
}

// BMPKReport menyusun laporan BMPK untuk satu posisi. Laporan bersifat bank-wide:
// aktor yang tidak berwenang atas seluruh bank ditolak. Saklar bmpk.enabled dibaca
// untuk melaporkan apakah modul menegakkan; status batas tetap dihitung sebagai
// informasi meski saklar mati.
func (s *bmpkService) BMPKReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.BMPKReport, error) {
	report := domain.BMPKReport{AsOf: asOf.UTC()}

	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrBMPKBankWide, actor.Role)
	}
	if s.config != nil {
		report.EnforcementEnabled = s.config.GetBool(ctx, domain.BMPKEnabledKey, false)
	}
	// Repositori wajib tersedia; tanpa itu laporan kosong akan tampak sah.
	if s.repo == nil {
		return report, fmt.Errorf("modul BMPK: repositori tidak tersedia")
	}

	rows, err := s.repo.ListPartyExposures(ctx)
	if err != nil {
		return report, fmt.Errorf("mengambil paparan BMPK per pihak terkait: %w", err)
	}

	// Nama nasabah tersimpan terenkripsi dan tidak dapat dibaca query agregat;
	// dibaca sekali secara batch (bukan satu query per pihak).
	if s.names != nil && len(rows) > 0 {
		ids := make([]uuid.UUID, 0, len(rows))
		seen := make(map[uuid.UUID]struct{}, len(rows))
		for _, r := range rows {
			if r.CustomerID == uuid.Nil {
				continue
			}
			if _, ok := seen[r.CustomerID]; ok {
				continue
			}
			seen[r.CustomerID] = struct{}{}
			ids = append(ids, r.CustomerID)
		}
		names, err := s.names.NamesByIDs(ctx, ids)
		if err != nil {
			return report, fmt.Errorf("membaca nama pihak terkait: %w", err)
		}
		for i := range rows {
			if name, ok := names[rows[i].CustomerID]; ok {
				rows[i].CustomerName = name
			}
		}
	}

	checks := make([]domain.BMPKPartyCheck, 0, len(rows))
	belumDiset := 0
	for _, r := range rows {
		check := domain.CheckBMPKParty(r)
		if check.Status == domain.BMPKStatusBatasBelumDiset {
			belumDiset++
		}
		checks = append(checks, check)
	}
	// Urutan deterministik: paparan terbesar lebih dulu, lalu id sebagai pemecah seri.
	sort.SliceStable(checks, func(i, j int) bool {
		if !checks[i].TotalExposure.Equal(checks[j].TotalExposure) {
			return checks[i].TotalExposure.GreaterThan(checks[j].TotalExposure)
		}
		return checks[i].CustomerID.String() < checks[j].CustomerID.String()
	})
	report.Rows = checks

	if !report.EnforcementEnabled {
		report.Warnings = append(report.Warnings,
			"saklar "+domain.BMPKEnabledKey+" belum menyala: status batas hanya dilaporkan sebagai informasi dan belum menegakkan apa pun")
	}
	if belumDiset > 0 {
		report.Warnings = append(report.Warnings, fmt.Sprintf(
			"%d pihak terkait belum memiliki batas pada bmpk_limits; statusnya BATAS_BELUM_DISET (bukan dianggap sesuai batas)", belumDiset))
	}
	return report, nil
}

var _ domain.BMPKService = (*bmpkService)(nil)
