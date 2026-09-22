package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
)

// eodDefinitionService mengelola definisi langkah tutup hari (urutan, saklar,
// prasyarat) dan membaca riwayatnya. Definisi adalah konfigurasi keuangan: perubahan
// hanya boleh dari izin system:config dan WAJIB teraudit (keputusan pemilik sistem).
type eodDefinitionService struct {
	repo      domain.EODStepRepository
	auditRepo domain.AuditRepository
	txRunner  ppapTxRunner
}

// NewEODDefinitionService merangkai layanan definisi EOD. auditRepo boleh nil pada
// lingkungan uji; pada produksi ia terisi sehingga setiap perubahan tercatat.
func NewEODDefinitionService(db *sql.DB, repo domain.EODStepRepository, auditRepo domain.AuditRepository) domain.EODDefinitionService {
	return &eodDefinitionService{
		repo:      repo,
		auditRepo: auditRepo,
		txRunner:  sqlPPAPTxRunner{db: db},
	}
}

func (s *eodDefinitionService) ListDefinitions(ctx context.Context) ([]domain.EODStepDefinition, error) {
	defs, err := s.repo.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	return domain.SortEODStepDefinitions(defs), nil
}

// UpdateDefinitions menyimpan seluruh definisi dalam satu transaksi beserta audit.
//
// Pengaman yang ditegakkan di sini:
//   - daftar kode yang dikirim harus SAMA dengan yang ada: tidak boleh menambah
//     langkah tanpa pelaksana, dan tidak boleh menghapus langkah (termasuk langkah
//     inti). Kolom core tidak dapat diubah lewat jalur ini;
//   - domain.ValidateEODStepDefinitions menolak urutan ganda/kosong, prasyarat tak
//     dikenal/bersiklus, langkah aktif yang bergantung pada langkah nonaktif, dan
//     langkah inti yang hilang/nonaktif.
//
// Bila validasi gagal, tidak ada baris yang berubah dan tidak ada audit yang ditulis.
func (s *eodDefinitionService) UpdateDefinitions(ctx context.Context, defs []domain.EODStepDefinition, actor domain.Actor) ([]domain.EODStepDefinition, error) {
	existing, err := s.repo.ListDefinitions(ctx)
	if err != nil {
		return nil, err
	}

	existingCodes := make(map[string]struct{}, len(existing))
	for _, def := range existing {
		existingCodes[def.Code] = struct{}{}
	}
	if len(defs) != len(existing) {
		return nil, fmt.Errorf("%w: jumlah langkah %d, seharusnya %d (langkah tidak dapat ditambah atau dihapus lewat API)",
			domain.ErrEODDefinitionsUnavailable, len(defs), len(existing))
	}
	for _, def := range defs {
		if _, ok := existingCodes[def.Code]; !ok {
			return nil, fmt.Errorf("%w: langkah %q tidak ada; langkah baru tidak dapat ditambah lewat API",
				domain.ErrEODDefinitionsUnavailable, def.Code)
		}
	}

	if err := domain.ValidateEODStepDefinitions(defs); err != nil {
		return nil, err
	}

	err = s.txRunner.Run(ctx, func(tx any) error {
		if err := s.repo.SaveDefinitions(ctx, tx, defs, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.auditRepo, tx, actor, "UPDATE_EOD_STEP_DEFINITIONS", "eod_step_definitions", "all", map[string]any{
			"definitions": defs,
		})
	})
	if err != nil {
		return nil, err
	}
	return domain.SortEODStepDefinitions(defs), nil
}

func (s *eodDefinitionService) ListStepRuns(ctx context.Context, businessDate time.Time) ([]domain.EODStepRunRecord, error) {
	return s.repo.ListStepRuns(ctx, businessDate)
}

var _ domain.EODDefinitionService = (*eodDefinitionService)(nil)
