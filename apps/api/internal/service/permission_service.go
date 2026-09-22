package service

import (
	"context"
	"fmt"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
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
	if !domain.KnownPermission(input.Permission) {
		return nil, fmt.Errorf("%w: %s", domain.ErrUnknownPermission, input.Permission)
	}
	exists, err := s.repo.GroupExists(ctx, groupCode)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, domain.ErrGroupNotFound
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
	permission := domain.Permission(strings.TrimSpace(payloadString(payload, "permission")))
	operation, err := domain.NormalizePermissionOperation(payloadString(payload, "operation"))
	if err != nil {
		return err
	}
	if groupCode == "" {
		return domain.ErrGroupNotFound
	}
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
