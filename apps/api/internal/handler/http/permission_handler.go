package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
)

// PermissionHandler melayani katalog grup/izin/menu dan pengajuan perubahan izin.
// Perubahan TIDAK diterapkan di sini: pengajuan masuk antrean maker-checker dan
// baru berlaku setelah disetujui pemeriksa lain.
type PermissionHandler struct {
	svc domain.PermissionService
}

func NewPermissionHandler(svc domain.PermissionService) *PermissionHandler {
	return &PermissionHandler{svc: svc}
}

type permissionGroupResponse struct {
	Code              string   `json:"code"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	IsSystem          bool     `json:"is_system"`
	ApprovalLimitRole string   `json:"approval_limit_role,omitempty"`
	Permissions       []string `json:"permissions"`
}

type permissionMenuResponse struct {
	MenuKey     string   `json:"menu_key"`
	Permissions []string `json:"permissions"`
}

// Catalog handles GET /api/v1/permissions/catalog. Menampilkan pemetaan yang
// berlaku agar bank dapat meninjau tanpa membaca database langsung.
func (h *PermissionHandler) Catalog(w http.ResponseWriter, r *http.Request) {
	groups, err := h.svc.ListGroups(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	menus, err := h.svc.ListMenus(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}

	groupList := make([]permissionGroupResponse, 0, len(groups))
	for _, g := range groups {
		groupList = append(groupList, permissionGroupResponse{
			Code:              g.Code,
			Name:              g.Name,
			Description:       g.Description,
			IsSystem:          g.IsSystem,
			ApprovalLimitRole: string(g.ApprovalLimitRole),
			Permissions:       permissionStrings(g.Permissions),
		})
	}
	menuList := make([]permissionMenuResponse, 0, len(menus))
	for _, m := range menus {
		menuList = append(menuList, permissionMenuResponse{
			MenuKey:     m.MenuKey,
			Permissions: permissionStrings(m.Permissions),
		})
	}

	Success(w, http.StatusOK, i18n.MsgPermissionCatalog, map[string]any{
		"groups": groupList,
		"menus":  menuList,
	})
}

// RequestChange handles POST /api/v1/permissions/requests.
func (h *PermissionHandler) RequestChange(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var body struct {
		GroupCode         string `json:"group_code"`
		Permission        string `json:"permission"`
		Operation         string `json:"operation"`
		ConfirmAccessLoss bool   `json:"confirm_access_loss"`
		Notes             string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		return
	}
	operation, err := domain.NormalizePermissionOperation(body.Operation)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	req, err := h.svc.RequestChange(r.Context(), domain.PermissionChangeInput{
		GroupCode:         body.GroupCode,
		Permission:        domain.Permission(body.Permission),
		Operation:         operation,
		ConfirmAccessLoss: body.ConfirmAccessLoss,
		Notes:             body.Notes,
	}, actor)
	if err != nil {
		writePermissionError(w, r, err)
		return
	}
	Success(w, http.StatusAccepted, i18n.MsgPermissionChangeSubmitted, map[string]any{
		"request_id":  req.ID,
		"action_type": req.ActionType,
		"status":      req.Status,
	})
}

func writePermissionError(w http.ResponseWriter, r *http.Request, err error) {
	var loss *domain.AccessLossError
	switch {
	case errors.As(err, &loss):
		Error(w, http.StatusUnprocessableEntity, loss.Error())
	case errors.Is(err, domain.ErrGroupNotFound):
		Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrUnknownPermission):
		Error(w, http.StatusUnprocessableEntity, err.Error())
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

func permissionStrings(perms []domain.Permission) []string {
	out := make([]string, 0, len(perms))
	for _, p := range perms {
		out = append(out, string(p))
	}
	return out
}
