package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ActionSetOrgUnitParent adalah jenis aksi maker-checker untuk pemindahan unit
// organisasi. Hierarki = cakupan data, grup = kewenangan; karena pemindahan unit
// mengubah siapa yang melihat data siapa, perubahan yang berdampak pada pengguna
// aktif wajib disetujui pejabat kedua lewat alur maker-checker yang sudah ada.
const ActionSetOrgUnitParent = "SET_ORG_UNIT_PARENT"

type branchService struct {
	repo      domain.BranchRepository
	auditRepo domain.AuditRepository
	// approvals menahan pemindahan unit yang mengubah cakupan pengguna. Boleh nil
	// (mis. pada uji unit): saat nil, pengaman lama (konfirmasi) dipakai agar
	// perubahan tidak lolos diam-diam; produksi selalu memasang maker-checker.
	approvals domain.MakerCheckerService
	// txRunner membuka satu transaksi untuk penulisan cabang dan auditnya. Lewat
	// interface agar dapat diganti stub pada test unit.
	txRunner ppapTxRunner
}

func NewBranchService(db *sql.DB, repo domain.BranchRepository, approvals domain.MakerCheckerService, auditSinks ...domain.AuditRepository) domain.BranchService {
	var auditRepo domain.AuditRepository
	if len(auditSinks) > 0 {
		auditRepo = auditSinks[0]
	}
	return &branchService{repo: repo, auditRepo: auditRepo, approvals: approvals, txRunner: sqlPPAPTxRunner{db: db}}
}

func (s *branchService) ListBranches(ctx context.Context) ([]domain.Branch, error) {
	return s.repo.List(ctx)
}

// CreateBranch membuat cabang baru. Kode dinormalisasi lalu divalidasi formatnya
// (3 digit, dipakai sebagai awalan nomor rekening), nama wajib diisi, dan kode
// harus unik. Kantor pusat ditolak: ia data fondasi yang hanya boleh satu dan
// dibuat lewat migrasi, karena pemilihan kantor pusat memakai is_head_office
// sehingga cabang HO kedua akan menggeser atribusi pelaku lintas cabang.
//
// Penulisan cabang dan jejak auditnya dibungkus SATU transaksi: keberhasilan
// dicatat ke audit log, dan bila audit gagal cabang tidak pernah ada. auditRepo
// boleh nil (mis. pada test yang tidak menyiapkan audit).
func (s *branchService) CreateBranch(ctx context.Context, input domain.CreateBranchInput, actor domain.Actor) (*domain.Branch, error) {
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if err := domain.ValidateBranchCode(code); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.ErrBranchNameRequired
	}
	if input.IsHeadOffice {
		return nil, domain.ErrBranchHeadOfficeNotAllowed
	}

	// Deteksi kode ganda lebih dulu agar pesannya jelas dan tidak bergantung pada
	// pesan unique violation database.
	if existing, err := s.repo.GetByCode(ctx, code); err == nil && existing != nil {
		return nil, domain.ErrBranchCodeExists
	} else if err != nil && !errors.Is(err, domain.ErrBranchNotFound) {
		return nil, err
	}

	branch := &domain.Branch{
		ID:           uuid.New(),
		Code:         code,
		Name:         name,
		Address:      strings.TrimSpace(input.Address),
		Phone:        strings.TrimSpace(input.Phone),
		IsHeadOffice: false,
		IsActive:     true,
		UnitLevel:    domain.UnitLevelBranch,
	}
	err := s.txRunner.Run(ctx, func(tx any) error {
		if err := s.repo.CreateTx(ctx, tx, branch); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrBranchCodeExists
			}
			return err
		}
		return writeAudit(ctx, s.auditRepo, tx, actor, "CREATE_BRANCH", "branch", branch.ID.String(), map[string]any{
			"code":           branch.Code,
			"name":           branch.Name,
			"is_head_office": branch.IsHeadOffice,
		})
	})
	if err != nil {
		return nil, err
	}
	return branch, nil
}

// ListOrgUnits mengembalikan seluruh unit organisasi (CABANG/AREA/WILAYAH) untuk
// pengelolaan susunan. Bila bank tidak memakai area/wilayah, isinya sama dengan
// daftar cabang biasa sehingga tidak ada perilaku baru.
func (s *branchService) ListOrgUnits(ctx context.Context) ([]domain.Branch, error) {
	return s.repo.List(ctx)
}

// orgUnitCodeMaxLength adalah lebar kolom branches.code (VARCHAR(32), migrasi
// 000088 memperlebar dari VARCHAR(8)). Kode area/wilayah yang lebih panjang tidak
// dapat disimpan, jadi ditolak di sini dengan pesan yang menyebut batasnya (422),
// bukan dibiarkan menjadi 500. Batas ini harus sama dengan lebar kolom skema;
// dua angka yang berbeda akan membuat validasi dan database tidak sepakat.
const orgUnitCodeMaxLength = 32

// validOrgUnitCode menegakkan kode unit yang aman untuk disimpan dan digabung ke
// cakupan aktor: tanpa pemisah cakupan (koma) dan tanpa spasi, panjang sesuai lebar
// kolom. Cabang tetap wajib 3 digit karena dipakai mengawali nomor rekening.
func validOrgUnitCode(code string, level domain.OrgUnitLevel) error {
	if level == domain.UnitLevelBranch {
		return domain.ValidateBranchCode(code)
	}
	if err := domain.ValidateOrgUnitCode(code); err != nil {
		return err
	}
	if len(code) > orgUnitCodeMaxLength {
		return domain.ErrOrgUnitCodeTooLong
	}
	for _, r := range code {
		switch {
		case r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
		default:
			return domain.ErrInvalidBranchCode
		}
	}
	return nil
}

// CreateOrgUnit membuat unit organisasi (cabang/area/wilayah) beserta atasannya.
// Kode unik, jenjang dikenal, dan atasan (bila ada) wajib berjenjang lebih tinggi.
// Area/wilayah tanpa atasan diizinkan karena bank boleh memakai hanya salah satu
// jenjang (keputusan pemilik: area dan wilayah keduanya opsional).
//
// Penulisan unit dan audit dibungkus satu transaksi, sama seperti CreateBranch.
func (s *branchService) CreateOrgUnit(ctx context.Context, input domain.CreateOrgUnitInput, actor domain.Actor) (*domain.Branch, error) {
	level := domain.OrgUnitLevel(strings.ToUpper(strings.TrimSpace(string(input.Level))))
	if !level.Valid() {
		return nil, domain.ErrOrgUnitLevelInvalid
	}
	code := strings.ToUpper(strings.TrimSpace(input.Code))
	if err := validOrgUnitCode(code, level); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, domain.ErrBranchNameRequired
	}

	if existing, err := s.repo.GetByCode(ctx, code); err == nil && existing != nil {
		return nil, domain.ErrBranchCodeExists
	} else if err != nil && !errors.Is(err, domain.ErrBranchNotFound) {
		return nil, err
	}

	var parentID *uuid.UUID
	parentCode := strings.ToUpper(strings.TrimSpace(input.ParentCode))
	if parentCode != "" {
		parent, err := s.repo.GetByCode(ctx, parentCode)
		if err != nil {
			return nil, err
		}
		if !parent.UnitLevel.CanBeParentOf(level) {
			return nil, domain.ErrOrgUnitParentInvalid
		}
		parentID = &parent.ID
	}

	unit := &domain.Branch{
		ID:        uuid.New(),
		Code:      code,
		Name:      name,
		Address:   strings.TrimSpace(input.Address),
		Phone:     strings.TrimSpace(input.Phone),
		IsActive:  true,
		UnitLevel: level,
		ParentID:  parentID,
	}
	err := s.txRunner.Run(ctx, func(tx any) error {
		if err := s.repo.CreateTx(ctx, tx, unit); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrBranchCodeExists
			}
			return err
		}
		return writeAudit(ctx, s.auditRepo, tx, actor, "CREATE_ORG_UNIT", "branch", unit.ID.String(), map[string]any{
			"code":        unit.Code,
			"name":        unit.Name,
			"unit_level":  string(unit.UnitLevel),
			"parent_code": parentCode,
		})
	})
	if err != nil {
		return nil, err
	}
	return unit, nil
}

// SetOrgUnitParent memindahkan unit ke bawah atasan lain, atau melepasnya menjadi
// puncak bila parentCode kosong. Atasan wajib berjenjang lebih tinggi; karena
// jenjang mengikat dan menurun, siklus tidak mungkin terbentuk (pindah ke diri
// sendiri pun ditolak sebagai atasan setingkat).
//
// Pengaman cakupan (W14): pemindahan mengubah siapa yang melihat cabang mana. Bila ada
// pengguna aktif yang akan kehilangan atau mendapat cakupan, perubahan WAJIB lewat
// maker-checker (bukan sekadar konfirmasi) sehingga tidak ada satu orang pun yang dapat
// mengubah akses orang lain sendiri. Bila dampaknya nol, perubahan berjalan langsung.
// Pengaman ini tidak mengunci siapa pun: staf di unit target selalu mempertahankan
// cakupannya (cakupan dimulai dari unit sendiri).
func (s *branchService) SetOrgUnitParent(ctx context.Context, code string, input domain.SetOrgUnitParentInput, actor domain.Actor) (*domain.Branch, error) {
	targetCode := strings.ToUpper(strings.TrimSpace(code))
	target, err := s.repo.GetByCode(ctx, targetCode)
	if err != nil {
		return nil, err
	}

	var parentID *uuid.UUID
	parentCode := strings.ToUpper(strings.TrimSpace(input.ParentCode))
	if parentCode != "" {
		parent, err := s.repo.GetByCode(ctx, parentCode)
		if err != nil {
			return nil, err
		}
		if !parent.UnitLevel.CanBeParentOf(target.UnitLevel) {
			return nil, domain.ErrOrgUnitParentInvalid
		}
		parentID = &parent.ID
	}

	losing, gaining, err := s.repo.ScopeImpactUsers(ctx, target.ID, target.ParentID, parentID)
	if err != nil {
		return nil, err
	}
	if losing > 0 || gaining > 0 {
		// Dampak > 0 pengguna: perubahan susunan organisasi mengubah cakupan data
		// orang lain, sehingga WAJIB lewat maker-checker (keputusan pemilik sistem:
		// hierarki = cakupan data, grup = kewenangan). Pembuat tidak dapat menyetujui
		// pengajuannya sendiri; pemeriksaan itu ada di maker-checker service.
		if s.approvals != nil {
			req, err := s.approvals.CreateRequest(ctx, domain.CreateMakerCheckerInput{
				ActionType: ActionSetOrgUnitParent,
				// Hierarki tidak bertalian dengan nominal: Amount nol, bukan angka yang
				// dipakai untuk memutuskan (keputusan "perlu persetujuan" dibuat di sini).
				Amount: decimal.Zero,
				Payload: map[string]any{
					"code":             target.Code,
					"parent_code":      parentCode,
					"affected_losing":  losing,
					"affected_gaining": gaining,
				},
				Notes: "Pemindahan unit mengubah cakupan pengguna aktif",
			}, actor)
			if err != nil {
				return nil, err
			}
			return nil, &domain.PendingApprovalError{RequestID: req.ID, ActionType: ActionSetOrgUnitParent}
		}
		// Tanpa layanan maker-checker (mis. uji unit), pengaman lama dipakai: tolak
		// kecuali dikonfirmasi. Produksi selalu memasang maker-checker, sehingga jalur
		// konfirmasi ini tidak pernah menjadi cara melewati persetujuan.
		if !input.ConfirmScopeChange {
			return nil, &domain.ScopeChangeError{Losing: losing, Gaining: gaining}
		}
	}

	err = s.txRunner.Run(ctx, func(tx any) error {
		if err := s.repo.SetParentTx(ctx, tx, target.ID, parentID); err != nil {
			return err
		}
		return writeAudit(ctx, s.auditRepo, tx, actor, "SET_ORG_UNIT_PARENT", "branch", target.ID.String(), map[string]any{
			"code":             target.Code,
			"parent_code":      parentCode,
			"affected_losing":  losing,
			"affected_gaining": gaining,
		})
	})
	if err != nil {
		return nil, err
	}
	updated, err := s.repo.GetByID(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ExecuteApproved menjalankan pemindahan unit yang sudah disetujui maker-checker, di
// dalam transaksi milik maker-checker service sehingga keputusan dan perubahannya
// commit bersama. Validasi jenjang diulang dari keadaan segar: payload bisa berumur,
// dan hierarki tidak boleh berubah karena permintaan yang syaratnya sudah tidak sah.
func (s *branchService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	if normalizeAction(actionType) != ActionSetOrgUnitParent {
		return fmt.Errorf("%w: %s", domain.ErrNoExecutorForAction, actionType)
	}
	code, _ := payload["code"].(string)
	rawParent, _ := payload["parent_code"].(string)
	parentCode := strings.ToUpper(strings.TrimSpace(rawParent))
	target, err := s.repo.GetByCode(ctx, code)
	if err != nil {
		return err
	}
	var parentID *uuid.UUID
	if parentCode != "" {
		parent, err := s.repo.GetByCode(ctx, parentCode)
		if err != nil {
			return err
		}
		if !parent.UnitLevel.CanBeParentOf(target.UnitLevel) {
			return domain.ErrOrgUnitParentInvalid
		}
		parentID = &parent.ID
	}
	if err := s.repo.SetParentTx(ctx, tx, target.ID, parentID); err != nil {
		return err
	}
	return writeAudit(ctx, s.auditRepo, tx, actor, "SET_ORG_UNIT_PARENT", "branch", target.ID.String(), map[string]any{
		"code":             target.Code,
		"parent_code":      parentCode,
		"affected_losing":  payload["affected_losing"],
		"affected_gaining": payload["affected_gaining"],
	})
}
