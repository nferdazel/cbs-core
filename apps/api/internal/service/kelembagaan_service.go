package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

// kelembagaanService menyediakan jalur baca dan tulis DATA KELEMBAGAAN (jaringan
// kantor, direksi/komisaris, pejabat eksekutif) untuk LAPORAN_KELEMBAGAAN.
//
// Ini BUKAN perubahan data operasional yang menyentuh uang: hanya atribut laporan.
// Laporan bersifat bank-wide (aktor non-lintas cabang ditolak); penulisan dijaga izin
// system:config di rute dan diaudit dalam transaksi yang sama, mengikuti pola
// OJK loan codes/CKPN activation.
type kelembagaanService struct {
	db     *sql.DB
	repo   domain.KelembagaanRepository
	audit  domain.AuditRepository
	runner ppapTxRunner
}

// NewKelembagaanService merakit layanan kelembagaan. auditRepo variadik mengikuti pola
// layanan lain: nil berarti audit dilewati tanpa menggagalkan aksi. db dipakai untuk
// memeriksa sandi kabupaten OJK sebelum menyimpan; nil melewati pemeriksaan itu
// (foreign key tetap penjaga terakhir).
func NewKelembagaanService(db *sql.DB, repo domain.KelembagaanRepository, auditSinks ...domain.AuditRepository) domain.KelembagaanService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &kelembagaanService{db: db, repo: repo, audit: auditRepo, runner: sqlPPAPTxRunner{db: db}}
}

// KelembagaanReport menyusun laporan kelembagaan untuk satu posisi. Bank-wide: aktor
// yang tidak berwenang atas seluruh bank ditolak.
func (s *kelembagaanService) KelembagaanReport(ctx context.Context, asOf time.Time, actor domain.Actor) (domain.KelembagaanReport, error) {
	report := domain.KelembagaanReport{AsOf: asOf.UTC(), Offices: []domain.BankOffice{}, Management: []domain.BankManagement{}}
	if !actor.IsCrossBranch() {
		return report, fmt.Errorf("%w: peran %s tidak berwenang", domain.ErrKelembagaanBankWide, actor.Role)
	}
	if s.repo == nil {
		return report, fmt.Errorf("modul kelembagaan: repositori tidak tersedia")
	}
	offices, err := s.repo.ListOffices(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca jaringan kantor: %w", err)
	}
	management, err := s.repo.ListManagement(ctx)
	if err != nil {
		return report, fmt.Errorf("membaca direksi/komisaris/pejabat eksekutif: %w", err)
	}
	if offices != nil {
		report.Offices = offices
	}
	if management != nil {
		report.Management = management
	}
	return report, nil
}

// UpsertOffice menyimpan satu kantor (id kosong = buat baru) dan menulis audit dalam
// satu transaksi.
func (s *kelembagaanService) UpsertOffice(ctx context.Context, input domain.UpdateBankOfficeInput, actor domain.Actor) (*domain.BankOffice, error) {
	office, err := domain.BuildBankOffice(input)
	if err != nil {
		return nil, err
	}
	if err := s.periksaKabupaten(ctx, office.OJKKabupatenCode); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	office.CreatedAt, office.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertOfficeTx(ctx, tx, office, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_BANK_OFFICE", "bank_office",
			office.ID.String(), bankOfficeAuditChanges(office))
	}); err != nil {
		return nil, err
	}
	return &office, nil
}

// UpdateOfficeForm00_11 menyimpan kolom Form 00.11 (migrasi 000126) satu kantor dan
// menulis audit dalam satu transaksi. Kantor yang tidak ada ditolak ErrKelembagaanNotFound.
// Kolom Form 00.04 yang tumpang tindih tidak disentuh.
func (s *kelembagaanService) UpdateOfficeForm00_11(ctx context.Context, id uuid.UUID, input domain.UpdateOfficeForm00_11Input, actor domain.Actor) error {
	value, err := domain.BuildOfficeForm00_11(input)
	if err != nil {
		return err
	}
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.UpdateForm00_11Tx(ctx, tx, id, value)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrKelembagaanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPDATE_OFFICE_FORM00_11", "bank_office",
			id.String(), officeForm00_11AuditChanges(value))
	})
}

// officeForm00_11AuditChanges membangun catatan audit ringkas kolom Form 00.11.
func officeForm00_11AuditChanges(in domain.UpdateOfficeForm00_11Input) map[string]any {
	return map[string]any{
		"ojk_office_kind_code": in.OJKOfficeKindCode,
		"parent_office_code":   in.ParentOfficeCode,
		"ojk_change_code":      in.OJKChangeCode,
		"control_office_code":  in.ControlOfficeCode,
	}
}

// DeleteOffice menghapus satu kantor dan menulis audit. Baris yang tidak ada ditolak
// ErrKelembagaanNotFound agar penghapusan tidak tampak berhasil.
func (s *kelembagaanService) DeleteOffice(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteOfficeTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrKelembagaanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_BANK_OFFICE", "bank_office",
			id.String(), map[string]any{"id": id.String()})
	})
}

// UpsertManagement menyimpan satu orang direksi/komisaris/pejabat eksekutif (id kosong
// = buat baru) dan menulis audit dalam satu transaksi.
func (s *kelembagaanService) UpsertManagement(ctx context.Context, input domain.UpdateBankManagementInput, actor domain.Actor) (*domain.BankManagement, error) {
	m, err := domain.BuildBankManagement(input)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	m.CreatedAt, m.UpdatedAt = now, now
	if err := s.runner.Run(ctx, func(tx any) error {
		if err := s.repo.UpsertManagementTx(ctx, tx, m, actor.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, s.audit, tx, actor, "UPSERT_BANK_MANAGEMENT", "bank_management",
			m.ID.String(), bankManagementAuditChanges(m))
	}); err != nil {
		return nil, err
	}
	return &m, nil
}

// DeleteManagement menghapus satu orang dan menulis audit.
func (s *kelembagaanService) DeleteManagement(ctx context.Context, id uuid.UUID, actor domain.Actor) error {
	return s.runner.Run(ctx, func(tx any) error {
		found, err := s.repo.DeleteManagementTx(ctx, tx, id)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrKelembagaanNotFound
		}
		return writeAudit(ctx, s.audit, tx, actor, "DELETE_BANK_MANAGEMENT", "bank_management",
			id.String(), map[string]any{"id": id.String()})
	})
}

// periksaKabupaten menolak sandi kabupaten OJK yang tidak ada di tabel referensi
// (Lampiran 03) dengan pesan yang menyebut kolomnya. Sandi kosong lolos; db nil
// melewati pemeriksaan (foreign key tetap penjaga terakhir).
func (s *kelembagaanService) periksaKabupaten(ctx context.Context, code string) error {
	if s.db == nil {
		return nil
	}
	return requireOJKReference(ctx, s.db, domain.OJKRefKabupaten, code, domain.OJKKabupatenField)
}

// bankOfficeAuditChanges membangun catatan audit ringkas kantor yang disimpan.
func bankOfficeAuditChanges(o domain.BankOffice) map[string]any {
	return map[string]any{
		"office_type":        o.OfficeType,
		"code":               o.Code,
		"name":               o.Name,
		"status":             o.Status,
		"ojk_kabupaten_code": o.OJKKabupatenCode,
	}
}

// bankManagementAuditChanges membangun catatan audit ringkas orang yang disimpan.
// NIK tidak ada karena memang tidak dimodelkan (keputusan privasi).
func bankManagementAuditChanges(m domain.BankManagement) map[string]any {
	return map[string]any{
		"category":          m.Category,
		"name":              m.Name,
		"position":          m.Position,
		"ojk_position_code": m.OJKPositionCode,
		"license_number":    m.LicenseNumber,
		"status":            m.Status,
	}
}

var _ domain.KelembagaanService = (*kelembagaanService)(nil)
