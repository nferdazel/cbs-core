package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/google/uuid"
)

type fakePermissionService struct {
	groups    []domain.UserGroup
	menus     []domain.MenuDefinition
	changeErr error
}

func (f *fakePermissionService) ListGroups(context.Context) ([]domain.UserGroup, error) {
	return f.groups, nil
}
func (f *fakePermissionService) ListMenus(context.Context) ([]domain.MenuDefinition, error) {
	return f.menus, nil
}
func (f *fakePermissionService) RequestChange(context.Context, domain.PermissionChangeInput, domain.Actor) (*domain.MakerCheckerRequest, error) {
	if f.changeErr != nil {
		return nil, f.changeErr
	}
	return &domain.MakerCheckerRequest{ID: uuid.New(), ActionType: "PERMISSION_CHANGE", Status: domain.MakerCheckerPending}, nil
}
func (f *fakePermissionService) ExecuteApproved(context.Context, any, string, map[string]any, domain.Actor) error {
	return nil
}

func withOperatorClaims(r *http.Request) *http.Request {
	claims := &domain.JWTClaims{UserID: uuid.New(), Username: "admin-uji", Role: domain.RoleAdmin}
	return r.WithContext(context.WithValue(r.Context(), domain.ContextKeyClaims, claims))
}

func TestPermissionHandlerCatalogMengembalikanGrupDanMenu(t *testing.T) {
	h := NewPermissionHandler(&fakePermissionService{
		groups: []domain.UserGroup{{Code: "ROLE_TELLER", Name: "Teller", Permissions: []domain.Permission{domain.PermLedgerRead}}},
		menus:  []domain.MenuDefinition{{MenuKey: "beranda", Permissions: nil}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/permissions/catalog", nil)
	rec := httptest.NewRecorder()

	h.Catalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Groups []permissionGroupResponse `json:"groups"`
			Menus  []permissionMenuResponse  `json:"menus"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON: %v", err)
	}
	if !body.Success || len(body.Data.Groups) != 1 || len(body.Data.Menus) != 1 {
		t.Fatalf("katalog tidak lengkap: %s", rec.Body.String())
	}
	if body.Data.Groups[0].Permissions[0] != string(domain.PermLedgerRead) {
		t.Fatalf("izin grup tidak ikut: %s", rec.Body.String())
	}
}

func TestPermissionHandlerRequestChangeMemetakanError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"pencabutan butuh konfirmasi", &domain.AccessLossError{AffectedUsers: 2}, http.StatusUnprocessableEntity},
		{"izin tak dikenal", domain.ErrUnknownPermission, http.StatusUnprocessableEntity},
		{"grup tidak ada", domain.ErrGroupNotFound, http.StatusNotFound},
		{"sukses", nil, http.StatusAccepted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := NewPermissionHandler(&fakePermissionService{changeErr: tc.err})
			body := `{"group_code":"ROLE_TELLER","permission":"ledger:read","operation":"REVOKE"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/permissions/requests", strings.NewReader(body))
			req = withOperatorClaims(req)
			rec := httptest.NewRecorder()

			h.RequestChange(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status %d, mau %d (body=%s)", rec.Code, tc.want, rec.Body.String())
			}
			if tc.err != nil && !strings.Contains(rec.Body.String(), "error") {
				t.Fatalf("respons error tanpa pesan: %s", rec.Body.String())
			}
		})
	}
}

// Katalog harus memuat anggota grup (agar bank melihat siapa yang terdampak) dan
// daftar seluruh izin yang ditegakkan kode (agar web tidak menyalin daftar izin).
func TestPermissionHandlerCatalogMemuatAnggotaDanDaftarIzin(t *testing.T) {
	userID := uuid.New()
	h := NewPermissionHandler(&fakePermissionService{
		groups: []domain.UserGroup{{
			Code:        "ROLE_TELLER",
			Name:        "Teller",
			Permissions: []domain.Permission{domain.PermLedgerRead},
			Members: []domain.GroupMember{{
				UserID: userID, Username: "teller01", FullName: "Teller Uji", Role: domain.RoleTeller, IsActive: true,
			}},
		}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/permissions/catalog", nil)
	rec := httptest.NewRecorder()
	h.Catalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d, mau 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var body struct {
		Data struct {
			Groups               []permissionGroupResponse `json:"groups"`
			AvailablePermissions []string                  `json:"available_permissions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON: %v", err)
	}
	if len(body.Data.Groups) != 1 || len(body.Data.Groups[0].Members) != 1 {
		t.Fatalf("anggota grup tidak ikut: %s", rec.Body.String())
	}
	if body.Data.Groups[0].Members[0].Username != "teller01" {
		t.Fatalf("anggota salah: %s", rec.Body.String())
	}
	found := false
	for _, p := range body.Data.AvailablePermissions {
		if p == string(domain.PermLedgerRead) {
			found = true
		}
	}
	if !found {
		t.Fatalf("daftar izin tersedia tidak memuat ledger:read: %s", rec.Body.String())
	}
}
