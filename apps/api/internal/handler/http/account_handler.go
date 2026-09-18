package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type AccountHandler struct {
	service domain.AccountService
}

func NewAccountHandler(service domain.AccountService) *AccountHandler {
	return &AccountHandler{service: service}
}

func (h *AccountHandler) Open(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.OpenAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.CustomerID == uuid.Nil {
		Error(w, http.StatusBadRequest, "customer_id is required")
		return
	}
	if input.ProductID == uuid.Nil {
		Error(w, http.StatusBadRequest, "product_id is required")
		return
	}
	if input.Currency == "" {
		input.Currency = "IDR"
	}
	if input.BranchCode == "" {
		input.BranchCode = claims.BranchCode
	}

	account, err := h.service.OpenAccount(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusCreated, "account opened successfully", account)
}

func (h *AccountHandler) GetByNumber(w http.ResponseWriter, r *http.Request) {
	accNum := chi.URLParam(r, "accountNumber")
	if accNum == "" {
		Error(w, http.StatusBadRequest, "account number is required")
		return
	}

	acc, err := h.service.GetAccountByNumber(r.Context(), accNum)
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	Success(w, http.StatusOK, "account retrieved", acc)
}

func (h *AccountHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	accounts, total, err := h.service.ListAccounts(r.Context(), page, pageSize)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, "accounts listed", accounts, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}
