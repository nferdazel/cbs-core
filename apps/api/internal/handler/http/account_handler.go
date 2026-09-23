package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
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
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.OpenAccountInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	if input.CustomerID == uuid.Nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgCustomerIDRequired)
		return
	}
	if input.ProductID == uuid.Nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgProductIDRequired)
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

	Success(w, http.StatusCreated, i18n.MsgAccountOpened, account)
}

func (h *AccountHandler) GetByNumber(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	accNum := chi.URLParam(r, "accountNumber")
	if accNum == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgAccountNumberRequired)
		return
	}

	acc, err := h.service.GetAccountByNumber(r.Context(), accNum, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgAccountRetrieved, acc)
}

// Reactivate memulihkan rekening dormant ke ACTIVE. Body bersifat opsional dan hanya
// membawa catatan operator; identitas pelaku selalu dari JWT.
func (h *AccountHandler) Reactivate(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	accNum := chi.URLParam(r, "accountNumber")
	if accNum == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgAccountNumberRequired)
		return
	}

	var body struct {
		Notes string `json:"notes"`
	}
	// Body kosong bukan alasan menolak: reaktivasi tanpa catatan tetap sah.
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	acc, err := h.service.ReactivateAccount(r.Context(), accNum, body.Notes, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// Fail memetakan lintas cabang ke 403 dan sentinel bisnis lain ke 422.
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgAccountReactivated, acc)
}

// Close menutup rekening yang sudah bersih. Body opsional hanya membawa catatan
// operator; identitas pelaku selalu dari JWT. Rekening bersaldo ditolak 422.
func (h *AccountHandler) Close(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	accNum := chi.URLParam(r, "accountNumber")
	if accNum == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgAccountNumberRequired)
		return
	}

	var body struct {
		Notes string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	acc, err := h.service.CloseAccount(r.Context(), accNum, body.Notes, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// Fail memetakan lintas cabang ke 403 dan sentinel bisnis lain ke 422.
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgAccountClosed, acc)
}

func (h *AccountHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	// q mencari nomor rekening (awalan) agar teller tidak perlu menghafal nomor penuh.
	search := r.URL.Query().Get("q")

	accounts, total, err := h.service.ListAccounts(r.Context(), page, pageSize, search, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, i18n.MsgAccountsListed, accounts, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}
