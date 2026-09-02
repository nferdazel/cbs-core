package http

import (
	"encoding/json"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
)

type CollectionHandler struct {
	collectionSvc domain.CollectionService
}

func NewCollectionHandler(collectionSvc domain.CollectionService) *CollectionHandler {
	return &CollectionHandler{collectionSvc: collectionSvc}
}

// ProcessMobileCollection handles POST /api/v1/collections/mobile-collect (AO Collector)
func (h *CollectionHandler) ProcessMobileCollection(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.MobileCollectionInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	input.CollectorID = claims.UserID
	if input.IdempotencyKey == "" {
		input.IdempotencyKey = "COLLECT-" + claims.UserID.String() + "-" + r.Header.Get("X-Request-ID")
	}

	result, err := h.collectionSvc.ProcessMobileCollection(r.Context(), input)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "mobile collection processed successfully", result)
}
