package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
)

type LedgerHandler struct {
	service domain.LedgerService
}

func NewLedgerHandler(service domain.LedgerService) *LedgerHandler {
	return &LedgerHandler{service: service}
}

func (h *LedgerHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	var req domain.DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.AccountNumber == "" || req.Amount.IsZero() {
		Error(w, http.StatusBadRequest, "account_number and amount are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Description == "" {
		req.Description = "Cash Deposit"
	}
	if req.CreatedBy == "" {
		req.CreatedBy = "TELLER"
	}

	// Read idempotency key from header if available
	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.Deposit(r.Context(), req)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusCreated, "deposit processed successfully", entry)
}

func (h *LedgerHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	var req domain.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.AccountNumber == "" || req.Amount.IsZero() {
		Error(w, http.StatusBadRequest, "account_number and amount are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Description == "" {
		req.Description = "Cash Withdrawal"
	}
	if req.CreatedBy == "" {
		req.CreatedBy = "TELLER"
	}

	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.Withdraw(r.Context(), req)
	if err != nil {
		if err == domain.ErrInsufficientFunds {
			Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusCreated, "withdrawal processed successfully", entry)
}

func (h *LedgerHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	var req domain.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.SourceAccountNumber == "" || req.DestinationAccountNumber == "" || req.Amount.IsZero() {
		Error(w, http.StatusBadRequest, "source_account_number, destination_account_number, and amount are required")
		return
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Description == "" {
		req.Description = "Internal Transfer"
	}
	if req.CreatedBy == "" {
		req.CreatedBy = "CUSTOMER"
	}

	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.TransferInternal(r.Context(), req)
	if err != nil {
		if err == domain.ErrInsufficientFunds {
			Error(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusCreated, "transfer executed successfully", entry)
}

func (h *LedgerHandler) GetJournalByRef(w http.ResponseWriter, r *http.Request) {
	ref := chi.URLParam(r, "reference")
	if ref == "" {
		Error(w, http.StatusBadRequest, "reference is required")
		return
	}

	entry, err := h.service.GetJournalByReference(r.Context(), ref)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	Success(w, http.StatusOK, "journal entry retrieved", entry)
}

func (h *LedgerHandler) ListJournals(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	journals, total, err := h.service.ListJournals(r.Context(), page, pageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, "journals listed", journals, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}

func (h *LedgerHandler) GetStatement(w http.ResponseWriter, r *http.Request) {
	accNum := chi.URLParam(r, "accountNumber")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	lines, total, err := h.service.GetAccountStatement(r.Context(), accNum, page, pageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, "account statement listed", lines, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}

func (h *LedgerHandler) ListCOA(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.GetChartOfAccounts(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusOK, "chart of accounts listed", list)
}
