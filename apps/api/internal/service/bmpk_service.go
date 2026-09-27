package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// bmpkService merakit laporan BMPK: membaca paparan per pihak terkait (satu query
// agregat di repositori), melengkapi nama nasabah secara batch, lalu menguji batas.
//
// Layanan ini juga melayani PENGATURAN pihak terkait dan batas (pihak terkait + batas
// kini dapat diisi lewat API berizin system:config, bukan SQL/seed). Penulisan bukan
// perubahan data operasional yang menyentuh uang: hanya pengaturan pedoman BMPK, dan
// diaudit dalam transaksi yang sama seperti kelembagaan/OJK loan codes.
type bmpkService struct {
	db     *sql.DB
	repo   domain.BMPKRepository
	config domain.SystemConfigService
	names  domain.BMPKCustomerNamer
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewBMPKService merakit layanan BMPK. names boleh nil; tanpa itu nama pihak terkait
// dibiarkan kosong (laporan menuliskannya sebagai belum tersedia), bukan ditebak.
// db dipakai memeriksa nasabah sebelum menyimpan (404 bila tidak ada); nil melewati
// pemeriksaan itu (foreign key tetap penjaga terakhir). auditSinks variadik mengikuti
// pola layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi.
func NewBMPKService(db *sql.DB, repo domain.BMPKRepository, config domain.SystemConfigService, names domain.BMPKCustomerNamer, auditSinks ...domain.AuditRepository) domain.BMPKService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &bmpkService{db: db, repo: repo, config: config, names: names, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
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

// ListMaster mengembalikan data pengaturan mentah untuk UI: seluruh pihak terkait dan
// batas yang bank isi. Daftar kosong dikembalikan sebagai slice non-nil agar
// serialisasinya [] bukan null.
func (s *bmpkService) ListMaster(ctx context.Context) (domain.BMPKMaster, error) {
	out := domain.BMPKMaster{RelatedParties: []domain.BMPKRelatedParty{}, Limits: []domain.BMPKLimit{}}
	if s.repo == nil {
		return out, fmt.Errorf("modul BMPK: repositori tidak tersedia")
	}
	parties, err := s.repo.ListRelatedParties(ctx)
	if err != nil {
		return out, fmt.Errorf("membaca pihak terkait BMPK: %w", err)
	}
	limits, err := s.repo.ListLimits(ctx)
	if err != nil {
		return out, fmt.Errorf("membaca batas BMPK: %w", err)
	}
	if parties != nil {
		out.RelatedParties = parties
	}
	if limits != nil {
		out.Limits = limits
	}
	return out, nil
}

// UpsertRelatedParty menyimpan satu penandaan pihak terkait dan menulis audit dalam
// satu transaksi. Nasabah yang tidak ada ditolak ErrBMPKNotFound.
func (s *bmpkService) UpsertRelatedParty(ctx context.Context, input domain.UpdateBMPKRelatedPartyInput, actor domain.Actor) (*domain.BMPKRelatedParty, error) {
	party, err := domain.BuildBMPKRelatedParty(input)
	if err != nil {
		return nil, err
	}
	if err := s.periksaNasabah(ctx, party.CustomerID); err != nil {
		return nil, err
	}
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertRelatedPartyTx(ctx, tx, party); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_BMPK_RELATED_PARTY", "bmpk_related_party",
			party.CustomerID.String(), bmpkRelatedPartyAuditChanges(party))
	}); err != nil {
		return nil, err
	}
	return &party, nil
}

// DeleteRelatedParty menghapus penandaan satu nasabah dan menulis audit. Baris yang
// tidak ada ditolak ErrBMPKNotFound agar penghapusan tidak tampak berhasil.
func (s *bmpkService) DeleteRelatedParty(ctx context.Context, customerID uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteRelatedPartyTx(ctx, tx, customerID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrBMPKNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_BMPK_RELATED_PARTY", "bmpk_related_party",
			customerID.String(), map[string]any{"customer_id": customerID.String()})
	})
}

// UpsertLimit menyimpan satu batas per nasabah dan menulis audit dalam satu transaksi.
// Nasabah yang tidak ada ditolak ErrBMPKNotFound.
func (s *bmpkService) UpsertLimit(ctx context.Context, input domain.UpdateBMPKLimitInput, actor domain.Actor) (*domain.BMPKLimit, error) {
	limit, err := domain.BuildBMPKLimit(input)
	if err != nil {
		return nil, err
	}
	if err := s.periksaNasabah(ctx, limit.CustomerID); err != nil {
		return nil, err
	}
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertLimitTx(ctx, tx, limit); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_BMPK_LIMIT", "bmpk_limit",
			limit.CustomerID.String(), bmpkLimitAuditChanges(limit))
	}); err != nil {
		return nil, err
	}
	return &limit, nil
}

// DeleteLimit menghapus batas satu nasabah dan menulis audit. Baris yang tidak ada
// ditolak ErrBMPKNotFound.
func (s *bmpkService) DeleteLimit(ctx context.Context, customerID uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteLimitTx(ctx, tx, customerID)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrBMPKNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_BMPK_LIMIT", "bmpk_limit",
			customerID.String(), map[string]any{"customer_id": customerID.String()})
	})
}

// periksaNasabah menolak customer_id yang tidak ada dengan ErrBMPKNotFound. db nil
// melewati pemeriksaan (foreign key tetap penjaga terakhir).
func (s *bmpkService) periksaNasabah(ctx context.Context, id uuid.UUID) error {
	if s.db == nil {
		return nil
	}
	var exists bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM customers WHERE id = $1)`, id).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("%w: customer_id tidak ditemukan", domain.ErrBMPKNotFound)
	}
	return nil
}

// bmpkRelatedPartyAuditChanges membangun catatan audit ringkas penandaan pihak terkait.
func bmpkRelatedPartyAuditChanges(p domain.BMPKRelatedParty) map[string]any {
	return map[string]any{
		"customer_id":       p.CustomerID.String(),
		"relationship_type": p.RelationshipType,
		"note":              p.Note,
	}
}

// bmpkLimitAuditChanges membangun catatan audit ringkas batas per nasabah.
func bmpkLimitAuditChanges(l domain.BMPKLimit) map[string]any {
	changes := map[string]any{
		"customer_id": l.CustomerID.String(),
		"max_amount":  l.MaxAmount.String(),
		"note":        l.Note,
	}
	if l.EffectiveDate != nil {
		changes["effective_date"] = l.EffectiveDate.Format("2006-01-02")
	}
	return changes
}
