package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
)

type IntegrationHandler struct {
	slikGateway     domain.SLIKGateway
	dukcapilGateway domain.DukcapilGateway
}

func NewIntegrationHandler(slikGateway domain.SLIKGateway, dukcapilGateway domain.DukcapilGateway) *IntegrationHandler {
	return &IntegrationHandler{
		slikGateway:     slikGateway,
		dukcapilGateway: dukcapilGateway,
	}
}

// CheckSLIK handles POST /api/v1/integrations/slik/check (AO / Supervisor)
func (h *IntegrationHandler) CheckSLIK(w http.ResponseWriter, r *http.Request) {
	var body struct {
		NIK string `json:"nik"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.NIK == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgNIKRequired)
		return
	}

	result, err := h.slikGateway.CheckDebtor(r.Context(), body.NIK)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgOJKSLIKCheckCompleted, result)
}

// VerifyDukcapil handles POST /api/v1/integrations/dukcapil/verify (CS / AO)
func (h *IntegrationHandler) VerifyDukcapil(w http.ResponseWriter, r *http.Request) {
	var input domain.DukcapilVerifyInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil || input.NIK == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgNIKRequired)
		return
	}

	result, err := h.dukcapilGateway.VerifyIdentity(r.Context(), input)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgDukcapilVerificationCompleted, result)
}
