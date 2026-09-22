package http

import (
	"encoding/json"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
)

// EODDefinitionHandler melayani definisi langkah tutup hari dan riwayatnya. Otorisasi
// ada di rute: baca memakai system:config:read, ubah memakai system:config (bukan
// yang read). Perubahan definisi menyentuh urutan akuntansi, jadi ia diperlakukan
// seperti konfigurasi sistem yang sensitif dan teraudit.
type EODDefinitionHandler struct {
	svc domain.EODDefinitionService
}

func NewEODDefinitionHandler(svc domain.EODDefinitionService) *EODDefinitionHandler {
	return &EODDefinitionHandler{svc: svc}
}

// List handles GET /api/v1/system/eod-definitions
func (h *EODDefinitionHandler) List(w http.ResponseWriter, r *http.Request) {
	defs, err := h.svc.ListDefinitions(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgEODDefinitions, defs)
}

// Update handles PUT /api/v1/system/eod-definitions
// Body: {"definitions":[{code,sequence,enabled,category,prerequisites,description}, ...]}
// Kolom core dikelola migrasi, bukan body: operator tidak dapat melucuti penanda inti.
func (h *EODDefinitionHandler) Update(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var body struct {
		Definitions []domain.EODStepDefinition `json:"definitions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	defs, err := h.svc.UpdateDefinitions(r.Context(), body.Definitions,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgEODDefinitionsUpdated, defs)
}

// History handles GET /api/v1/system/eod-runs?date=YYYY-MM-DD
// Tanpa date, run terakhir yang dikembalikan.
func (h *EODDefinitionHandler) History(w http.ResponseWriter, r *http.Request) {
	var businessDate time.Time
	if raw := r.URL.Query().Get("date"); raw != "" {
		parsed, err := time.Parse("2006-01-02", raw)
		if err != nil {
			ErrorCode(w, http.StatusBadRequest, i18n.MsgDateFormatInvalid)
			return
		}
		businessDate = parsed
	}

	runs, err := h.svc.ListStepRuns(r.Context(), businessDate)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgEODRunHistory, runs)
}
