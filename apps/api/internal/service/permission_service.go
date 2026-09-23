package service

import (
	"context"
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ActionPermissionChange adalah jenis aksi maker-checker untuk mengubah pemetaan
// izin. Perubahan TIDAK diterapkan saat pengajuan, melainkan saat disetujui
// pemeriksa lain lewat alur maker-checker yang sudah ada.
const ActionPermissionChange = "PERMISSION_CHANGE"

type permissionService struct {
	repo      domain.PermissionRepository
	auditRepo domain.AuditRepository
	mc        domain.MakerCheckerService
}

func NewPermissionService(repo domain.PermissionRepository, auditRepo domain.AuditRepository, mc domain.MakerCheckerService) domain.PermissionService {
	return &permissionService{repo: repo, auditRepo: auditRepo, mc: mc}
}

func (s *permissionService) ListGroups(ctx context.Context) ([]domain.UserGroup, error) {
	return s.repo.ListGroups(ctx)
}

func (s *permissionService) ListMenus(ctx context.Context) ([]domain.MenuDefinition, error) {
	return s.repo.ListMenus(ctx)
}

// RequestChange memvalidasi pengajuan lalu menitipkannya ke maker-checker. Perubahan
// belum terjadi di sini. Pencabutan yang akan membuat pengguna kehilangan akses
// DITOLAK kecuali ConfirmAccessLoss disetel: dengan begitu pencabutan selalu
// keputusan sadar, bukan efek samping tak terlihat.
func (s *permissionService) RequestChange(ctx context.Context, input domain.PermissionChangeInput, actor domain.Actor) (*domain.MakerCheckerRequest, error) {
	groupCode := strings.TrimSpace(input.GroupCode)
	if groupCode == "" {
		return nil, domain.ErrGroupNotFound
	}
	exists, err := s.repo.GroupExists(ctx, groupCode)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrGroupNotFound
	}

	// Operasi keanggotaan mengubah siapa yang tergabung di grup; operasi izin
	// mengubah pemetaan grup->izin. Keduanya lewat maker-checker yang sama.
	if input.Operation.IsMemberOperation() {
		return s.requestMemberChange(ctx, groupCode, input, actor)
	}

	if !domain.KnownPermission(input.Permission) {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnknownPermission, input.Permission)
	}

	if input.Operation == domain.PermissionChangeRevoke {
		if err := s.guardAccessLoss(ctx, groupCode, input.Permission, input.ConfirmAccessLoss); err != nil {
			return nil, err
		}
	}

	return s.mc.CreateRequest(ctx, domain.CreateMakerCheckerInput{
		ActionType: ActionPermissionChange,
		Amount:     decimal.Zero,
		Payload: map[string]any{
			"group_code": groupCode,
			"permission": string(input.Permission),
			"operation":  string(input.Operation),
			// Disimpan sebagai bukti apa yang dikonfirmasi pembuat; eksekusi tetap
			// memeriksa keadaan terkini agar tidak buta terhadap perubahan antara.
			"confirm_access_loss": input.ConfirmAccessLoss,
		},
		Notes: input.Notes,
	}, actor)
}

// requestMemberChange memvalidasi pengajuan keanggotaan grup. Pengguna harus ada;
// grup sudah dipastikan ada pemanggil. Efeknya baru berlaku setelah disetujui
// pemeriksa lain, sama seperti perubahan izin.
func (s *permissionService) requestMemberChange(ctx context.Context, groupCode string, input domain.PermissionChangeInput, actor domain.Actor) (*domain.MakerCheckerRequest, error) {
	userID, err := uuid.Parse(strings.TrimSpace(input.UserID))
	if err != nil {
		return nil, fmt.Errorf("user_id tidak valid: %w", err)
	}
	userExists, err := s.repo.UserExists(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !userExists {
		return nil, domain.ErrUserNotFound
	}
	// Penghapusan anggota dapat membuat pengguna kehilangan izin efektifnya, jadi
	// tuntut konfirmasi yang sama seperti pencabutan izin. Pemeriksaan diulang saat
	// persetujuan memakai keadaan terkini; yang disimpan di sini hanya bukti bahwa
	// pembuat sudah mengonfirmasi.
	if input.Operation == domain.PermissionChangeRemoveMember {
		affected, err := s.repo.CountAccessLossOnMemberRemoval(ctx, groupCode, userID)
		if err != nil {
			return nil, err
		}
		if affected > 0 && !input.ConfirmAccessLoss {
			return nil, &domain.AccessLossError{AffectedUsers: affected}
		}
	}
	return s.mc.CreateRequest(ctx, domain.CreateMakerCheckerInput{
		ActionType: ActionPermissionChange,
		Amount:     decimal.Zero,
		Payload: map[string]any{
			"group_code": groupCode,
			"operation":  string(input.Operation),
			"user_id":    userID.String(),
			// Disimpan sebagai bukti apa yang dikonfirmasi pembuat; eksekusi tetap
			// memeriksa keadaan terkini agar tidak buta terhadap perubahan antara.
			"confirm_access_loss": input.ConfirmAccessLoss,
		},
		Notes: input.Notes,
	}, actor)
}

func (s *permissionService) guardAccessLoss(ctx context.Context, groupCode string, p domain.Permission, confirmed bool) error {
	affected, err := s.repo.CountUsersLosingPermission(ctx, groupCode, p)
	if err != nil {
		return err
	}
	if affected > 0 && !confirmed {
		return &domain.AccessLossError{AffectedUsers: affected}
	}
	return nil
}

// ExecuteApproved menerapkan perubahan izin di dalam transaksi yang sama dengan
// keputusan maker-checker. Sebelum menulis, penjaga akses diperiksa ULANG memakai
// keadaan terkini, sehingga izin yang semula aman tetapi berubah di antara
// pengajuan dan persetujuan tetap tidak mencabut akses secara mendadak. Jejak
// sebelum->sesudah ditulis ke audit log.
func (s *permissionService) ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor domain.Actor) error {
	if ActionPermissionChange != strings.ToUpper(strings.TrimSpace(actionType)) {
		return fmt.Errorf("permission: jenis aksi tidak didukung: %s", actionType)
	}

	groupCode := strings.TrimSpace(payloadString(payload, "group_code"))
	operation, err := domain.NormalizePermissionOperation(payloadString(payload, "operation"))
	if err != nil {
		return err
	}
	if groupCode == "" {
		return domain.ErrGroupNotFound
	}
	// Keanggotaan grup dieksekusi terpisah: yang berubah adalah baris
	// user_group_members, bukan pemetaan grup->izin.
	if operation.IsMemberOperation() {
		return s.executeMemberChange(ctx, tx, groupCode, operation, payload, actor)
	}
	permission := domain.Permission(strings.TrimSpace(payloadString(payload, "permission")))
	if !domain.KnownPermission(permission) {
		return fmt.Errorf("%w: %s", domain.ErrUnknownPermission, permission)
	}

	before, err := s.repo.GroupHasPermission(ctx, groupCode, permission)
	if err != nil {
		return err
	}

	switch operation {
	case domain.PermissionChangeGrant:
		if !before {
			if err := s.repo.GrantPermissionTx(ctx, tx, groupCode, permission); err != nil {
				return err
			}
		}
	case domain.PermissionChangeRevoke:
		if before {
			if err := s.guardAccessLoss(ctx, groupCode, permission, payloadBool(payload, "confirm_access_loss")); err != nil {
				return err
			}
			if err := s.repo.RevokePermissionTx(ctx, tx, groupCode, permission); err != nil {
				return err
			}
		}
	}

	after := false
	switch operation {
	case domain.PermissionChangeGrant:
		after = true
	case domain.PermissionChangeRevoke:
		after = false
	}

	return writeAudit(ctx, s.auditRepo, tx, actor, "PERMISSION_MAPPING_CHANGE", "user_group", groupCode, map[string]any{
		"group_code": groupCode,
		"permission": string(permission),
		"operation":  string(operation),
		"before":     before,
		"after":      after,
	})
}

// executeMemberChange menambah/mengeluarkan anggota grup di dalam transaksi
// maker-checker. Keadaan terkini diperiksa ulang saat eksekusi agar keputusan
// idempoten walau keadaan berubah antara pengajuan dan persetujuan; jejak
// sebelum->sesudah ditulis ke audit log.
func (s *permissionService) executeMemberChange(ctx context.Context, tx any, groupCode string, operation domain.PermissionChangeOperation, payload map[string]any, actor domain.Actor) error {
	userID, err := uuid.Parse(strings.TrimSpace(payloadString(payload, "user_id")))
	if err != nil {
		return fmt.Errorf("user_id tidak valid: %w", err)
	}
	before, err := s.repo.UserInGroup(ctx, groupCode, userID)
	if err != nil {
		return err
	}
	switch operation {
	case domain.PermissionChangeAddMember:
		if !before {
			if err := s.repo.AddMemberTx(ctx, tx, groupCode, userID); err != nil {
				return err
			}
		}
	case domain.PermissionChangeRemoveMember:
		if before {
			// Diperiksa ULANG dengan keadaan terkini: keanggotaan lain yang semula
			// menutup kehilangan dapat sudah hilang antara pengajuan dan persetujuan.
			affected, err := s.repo.CountAccessLossOnMemberRemoval(ctx, groupCode, userID)
			if err != nil {
				return err
			}
			if affected > 0 && !payloadBool(payload, "confirm_access_loss") {
				return &domain.AccessLossError{AffectedUsers: affected}
			}
			if err := s.repo.RemoveMemberTx(ctx, tx, groupCode, userID); err != nil {
				return err
			}
		}
	}
	after := operation == domain.PermissionChangeAddMember
	return writeAudit(ctx, s.auditRepo, tx, actor, "GROUP_MEMBERSHIP_CHANGE", "user_group", groupCode, map[string]any{
		"group_code": groupCode,
		"user_id":    userID.String(),
		"operation":  string(operation),
		"before":     before,
		"after":      after,
	})
}

// payloadString dan payloadBool membaca payload yang bisa datang dari JSON
// (string/bool) atau dari pemanggil internal, tanpa panik pada tipe tak terduga.
func payloadString(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

func payloadBool(payload map[string]any, key string) bool {
	if v, ok := payload[key].(bool); ok {
		return v
	}
	return false
}
