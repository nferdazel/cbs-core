package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// defaultMakerCheckerThreshold dipakai bila ambang per jenis aksi belum diisi di
// system_config. Ini tempat sementara: nilai produksi wajib diisi saat provisioning.
var defaultMakerCheckerThreshold = decimal.NewFromInt(50_000_000)

type makerCheckerService struct {
	db        *sql.DB
	repo      domain.MakerCheckerRepository
	auditRepo domain.AuditRepository
	config    domain.SystemConfigService
	executors domain.MakerCheckerExecutor
	// dates adalah sumber tanggal bisnis bank (WIB). Dipakai menandai setiap pengajuan
	// dengan business_date agar akumulasi batas harian mengaitkannya ke hari bisnis
	// yang benar, bukan tanggal kalender UTC created_at.
	dates domain.BusinessDateProvider
	// branchRepo menyelesaikan kode cabang pembuat menjadi cabang nyata (satu sumber
	// dengan resolveActorBranch), supaya pengajuan oleh pelaku lintas cabang berkode
	// 'HO' tidak tersimpan dengan branch_id NULL.
	branchRepo domain.BranchRepository
}

func NewMakerCheckerService(
	db *sql.DB,
	repo domain.MakerCheckerRepository,
	auditRepo domain.AuditRepository,
	config domain.SystemConfigService,
	executors domain.MakerCheckerExecutor,
	dates domain.BusinessDateProvider,
	branchRepo domain.BranchRepository,
) domain.MakerCheckerService {
	return &makerCheckerService{db: db, repo: repo, auditRepo: auditRepo, config: config, executors: executors, dates: dates, branchRepo: branchRepo}
}

// resolveRequestBranchCode menyelesaikan cabang pelaku lewat resolveActorBranch
// sehingga kode 'HO' yang tidak terdaftar jatuh ke kantor pusat, bukan NULL. Kode
// cabang yang memang kosong tetap diteruskan apa adanya (data pra-migrasi / batch):
// memaksakan kantor pusat untuk pelaku tanpa cabang mengubah makna lama. Bila repo
// belum dipasang (test unit tanpa database), kode aktor diteruskan apa adanya.
func (s *makerCheckerService) resolveRequestBranchCode(ctx context.Context, actor domain.Actor) (string, error) {
	code := strings.TrimSpace(actor.BranchCode)
	if s.branchRepo == nil || code == "" {
		return actor.BranchCode, nil
	}
	branch, err := resolveActorBranch(ctx, s.branchRepo, actor)
	if err != nil {
		return "", err
	}
	if branch == nil {
		return actor.BranchCode, nil
	}
	return branch.Code, nil
}

// makerCheckerBook memulihkan buku COA pengajuan dari payload yang diisi server-side
// saat pengajuan dibuat (maker_book). Pengajuan lama atau aktor tanpa buku
// mengembalikan buku kosong, yang oleh CanAccessBook diizinkan agar data lama tidak
// terblokir.
func makerCheckerBook(req *domain.MakerCheckerRequest) domain.COABook {
	if req == nil || req.Payload == nil {
		return ""
	}
	raw, _ := req.Payload["maker_book"].(string)
	return domain.COABook(raw)
}

// Threshold membaca ambang persetujuan untuk satu jenis aksi dari system_config.
// Key: maker_checker.<action_type>.<threshold>, mis. maker_checker.deposit.threshold.
func (s *makerCheckerService) Threshold(ctx context.Context, actionType string) decimal.Decimal {
	key := "maker_checker." + strings.ToLower(strings.TrimSpace(actionType)) + ".threshold"
	return s.config.GetDecimal(ctx, key, defaultMakerCheckerThreshold)
}

func (s *makerCheckerService) CreateRequest(ctx context.Context, input domain.CreateMakerCheckerInput, actor domain.Actor) (*domain.MakerCheckerRequest, error) {
	if strings.TrimSpace(input.ActionType) == "" {
		return nil, errors.New("jenis aksi maker-checker wajib diisi")
	}

	// Ambang persetujuan TIDAK dihitung ulang di sini. Keputusan "perlu persetujuan"
	// sudah dibuat pemanggil: untuk setoran/penarikan/transfer dan penempatan deposito
	// oleh penjaga batas transaksi yang membaca SATU sumber kebenaran
	// limit.<peran>.<jenis>.approval_above; untuk aksi kredit oleh guard khususnya.
	//
	// Menulis ambang kedua dari maker_checker.<aksi>.threshold ke payload membuat dua
	// angka untuk hal yang sama bisa saling bertentangan: pembaca API melihat angka
	// yang bukan angka yang dipakai memutuskan, dan selisihnya dapat meloloskan dana.
	// Payload dari pemanggil yang masih memuat "threshold"/"requires_approval" karena
	// itu DITERIMA tetapi DIABAIKAN (dihapus), sehingga klien lama tidak patah dan
	// angka lama tidak tersimpan sebagai kebenaran.
	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	delete(payload, "threshold")
	delete(payload, "requires_approval")
	payload["amount"] = input.Amount.String()
	// Identitas pembuat disimpan pada payload agar penilai ulang batas saat eksekusi
	// dapat merekonstruksi peran pembuat tanpa kueri tambahan. Tanpa peran, batas harian
	// pembuat tidak dapat dibaca dan evaluasi ulang akan jatuh ke peran pemeriksa.
	payload["maker_id"] = actor.UserID.String()
	payload["maker_username"] = actor.DisplayName()
	payload["maker_role"] = string(actor.Role)
	// Buku pembuat disimpan server-side dari JWT (bukan body) agar pemeriksa dan
	// daftar antrean dapat dibatasi pada buku yang sama. Nilai kosong berarti buku
	// pembuat belum ditentukan dan tetap lolos, mengikuti semantik CanAccessBook.
	payload["maker_book"] = string(actor.Book)

	// Cabang pembuat diselesaikan lewat satu sumber (resolveActorBranch): pelaku
	// lintas cabang berkode 'HO' diatribusikan ke kantor pusat, bukan NULL.
	branchCode, err := s.resolveRequestBranchCode(ctx, actor)
	if err != nil {
		return nil, err
	}
	payload["maker_branch"] = branchCode

	// Tanggal bisnis ditandai pada pengajuan agar akumulasi batas harian dapat
	// mengaitkannya ke hari bisnis yang benar tanpa menebak dari tanggal kalender.
	// Bila sumber tanggal bisnis tidak tersedia (mis. stub uji tanpa database),
	// business_date dibiarkan nol dan pembaca jatuh ke tanggal kalender WIB created_at.
	var businessDate time.Time
	if s.dates != nil {
		d, err := s.dates.CurrentBusinessDate(ctx)
		if err != nil {
			return nil, fmt.Errorf("membaca tanggal bisnis untuk pengajuan: %w", err)
		}
		businessDate = d
	}

	now := time.Now().UTC()
	req := &domain.MakerCheckerRequest{
		ID:           uuid.New(),
		ActionType:   input.ActionType,
		Payload:      payload,
		Status:       domain.MakerCheckerPending,
		MakerID:      actor.UserID.String(),
		MakerNotes:   input.Notes,
		BranchCode:   branchCode,
		BusinessDate: businessDate,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.repo.CreateTx(ctx, tx, req); err != nil {
		return nil, err
	}
	if err := writeAudit(ctx, s.auditRepo, tx, actor, "CREATE_MAKER_CHECKER", "maker_checker", req.ID.String(), map[string]any{
		"action_type": req.ActionType,
		"amount":      input.Amount.String(),
		"status":      string(req.Status),
	}); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *makerCheckerService) Approve(ctx context.Context, id uuid.UUID, actor domain.Actor, notes string) error {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// Penegakan cabang didahulukan atas status: pemeriksa cabang lain tidak boleh
	// mengetahui status internal pengajuan. Baris tanpa cabang (data pra-migrasi)
	// tetap boleh diproses, mengikuti semantik CanAccessBranch.
	if !actor.CanAccessBranch(req.BranchCode) {
		return domain.ErrCrossBranchAccess
	}
	// Buku pengajuan ditegakkan: pemeriksa satu buku tidak boleh menyetujui pengajuan
	// buku lain. Penting karena persetujuan mengeksekusi tulisan lintas buku.
	if !actor.CanAccessBook(makerCheckerBook(req)) {
		return domain.ErrCrossBookAccess
	}
	if req.Status != domain.MakerCheckerPending {
		return domain.ErrMakerCheckerNotPending
	}
	// Pemisahan tugas: pembuat tidak boleh menjadi pemeriksa permintaannya sendiri.
	if req.MakerID == actor.UserID.String() {
		return domain.ErrCannotSelfApprove
	}
	return s.process(ctx, req, actor, domain.MakerCheckerApproved, notes, "APPROVE_MAKER_CHECKER")
}

func (s *makerCheckerService) Reject(ctx context.Context, id uuid.UUID, actor domain.Actor, notes string) error {
	req, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	// Penolakan pun lintas cabang: pengawas cabang lain tidak boleh memutuskan
	// pengajuan yang bukan wewenangnya.
	if !actor.CanAccessBranch(req.BranchCode) {
		return domain.ErrCrossBranchAccess
	}
	// Buku pengajuan ditegakkan juga pada penolakan: pengawas buku lain tidak boleh
	// memutuskan pengajuan yang bukan wewenangnya.
	if !actor.CanAccessBook(makerCheckerBook(req)) {
		return domain.ErrCrossBookAccess
	}
	if req.Status != domain.MakerCheckerPending {
		return domain.ErrMakerCheckerNotPending
	}
	return s.process(ctx, req, actor, domain.MakerCheckerRejected, notes, "REJECT_MAKER_CHECKER")
}

// process mengeksekusi transaksi yang disetujui sekaligus mencatat keputusannya.
// Status, posting jurnal, dan audit berada dalam SATU transaksi: keputusan tidak
// pernah tercatat tanpa efeknya, dan efek tidak pernah terjadi tanpa jejak keputusan.
func (s *makerCheckerService) process(ctx context.Context, req *domain.MakerCheckerRequest, actor domain.Actor, status domain.MakerCheckerStatus, notes, action string) error {
	// Penjaga buku dipasang juga di sini, bukan hanya di Approve/Reject: jalur ini
	// yang benar-benar mengeksekusi tulisan, jadi batas buku tidak boleh bergantung
	// pada pemanggil.
	if !actor.CanAccessBook(makerCheckerBook(req)) {
		return domain.ErrCrossBookAccess
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Kunci permintaan agar dua pemeriksa tidak memproses bersamaan.
	if err := s.repo.UpdateStatusTx(ctx, tx, req.ID, status, actor.UserID.String(), notes); err != nil {
		return err
	}

	// Hanya persetujuan yang mengeksekusi transaksi; penolakan berhenti di status.
	if status == domain.MakerCheckerApproved {
		if s.executors == nil {
			return domain.ErrNoExecutorForAction
		}
		if err := s.executors.ExecuteApproved(ctx, tx, req.ActionType, req.Payload, actor); err != nil {
			return err
		}
	}

	if err := writeAudit(ctx, s.auditRepo, tx, actor, action, "maker_checker", req.ID.String(), map[string]any{
		"action_type": req.ActionType,
		"status":      string(status),
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *makerCheckerService) ListPending(ctx context.Context, actor domain.Actor) ([]domain.MakerCheckerRequest, error) {
	return s.repo.ListPending(ctx, actor)
}
