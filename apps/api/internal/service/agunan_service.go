package service

import (
	"context"
	"database/sql"
	"fmt"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// agunanService menyediakan jalur baca dan isi kolom Form 06.01 "Daftar Agunan" pada
// loan_collaterals (migrasi 000125). Ini BUKAN modul agunan operasional: hanya kolom
// pelaporan OJK yang disentuh, dan hanya untuk agunan AKTIF.
//
// Pengisian dijaga izin system:config di rute dan diaudit dalam transaksi yang sama.
type agunanService struct {
	repo   domain.AgunanRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewAgunanService merakit layanan agunan Form 06.01. auditSinks variadik mengikuti pola
// layanan lain: nil berarti audit dilewati. db dipakai membuka transaksi tulis.
func NewAgunanService(db *sql.DB, repo domain.AgunanRepository, auditSinks ...domain.AuditRepository) domain.AgunanService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &agunanService{repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// ListAgunan membaca agunan AKTIF beserta kolom Form 06.01 untuk UI pengisian dan ekspor,
// urutan deterministik dari repositori. Bank-wide: aktor yang tidak berwenang atas seluruh
// bank ditolak agar ekspor Form 06.01 tidak memuat sebagian cabang.
func (s *agunanService) ListAgunan(ctx context.Context, actor domain.Actor) ([]domain.AgunanRow, error) {
	if !actor.IsCrossBranch() {
		return nil, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrAgunanBankWide, actor.Role)
	}
	if s.repo == nil {
		return nil, fmt.Errorf("modul agunan Form 06.01: repositori tidak tersedia")
	}
	rows, err := s.repo.ListAgunanForOJK(ctx)
	if err != nil {
		return nil, fmt.Errorf("membaca agunan Form 06.01: %w", err)
	}
	if rows == nil {
		rows = []domain.AgunanRow{}
	}
	return rows, nil
}

// UpdateAgunan menyimpan kolom Form 06.01 satu agunan dan menulis audit dalam satu
// transaksi. Kode register yang sudah dipakai agunan lain ditolak ErrAgunanRegisterUsed;
// agunan yang tidak ada/tidak AKTIF ditolak ErrAgunanNotFound.
func (s *agunanService) UpdateAgunan(ctx context.Context, id uuid.UUID, in domain.UpdateAgunanInput, actor domain.Actor) error {
	value, err := domain.BuildAgunanUpdate(in)
	if err != nil {
		return err
	}
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.UpdateOjkColumnsTx(ctx, tx, id, value)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrAgunanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPDATE_AGUNAN_FORM06_01", "loan_collateral",
			id.String(), agunanAuditChanges(value))
	})
}

// agunanAuditChanges membangun catatan audit ringkas kolom Form 06.01 yang disimpan.
func agunanAuditChanges(in domain.UpdateAgunanInput) map[string]any {
	return map[string]any{
		"kode_register":     in.KodeRegister,
		"jenis_agunan_code": in.JenisAgunanCode,
		"nilai_diagunkan":   in.NilaiDiagunkan.String(),
		"nilai_agunan":      in.NilaiAgunan.String(),
		"penilai_code":      in.PenilaiCode,
		"tanggal_penilaian": in.TanggalPenilaian,
		"ppka_amount":       in.PPKAAmount.String(),
	}
}

var _ domain.AgunanService = (*agunanService)(nil)
