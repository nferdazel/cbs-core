package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
)

type LedgerHandler struct {
	service domain.LedgerService
}

func NewLedgerHandler(service domain.LedgerService) *LedgerHandler {
	return &LedgerHandler{service: service}
}

// requireActor mengambil identitas pelaku dari JWT. Jalur transaksi tidak pernah
// menerima identitas dari body request.
func requireActor(w http.ResponseWriter, r *http.Request) (domain.Actor, bool) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return domain.Actor{}, false
	}
	return claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())), true
}

// writeTransactionError menerjemahkan error transaksi ke respons HTTP. Transaksi yang
// melewati ambang persetujuan bukan kegagalan: ia diterima untuk direview, jadi
// dibalas 202 beserta id permintaannya.
func writeTransactionError(w http.ResponseWriter, r *http.Request, err error) {
	var pending *domain.PendingApprovalError
	if errors.As(err, &pending) {
		Success(w, http.StatusAccepted, i18n.MsgTransactionPendingApproval, map[string]any{
			"request_id":  pending.RequestID,
			"action_type": pending.ActionType,
			"status":      "PENDING_APPROVAL",
		})
		return
	}
	Fail(w, r, http.StatusUnprocessableEntity, err)
}

// Reverse handles POST /api/v1/transactions/{reference}/reverse
//
// Hanya untuk transaksi yang tidak menyalakan state domain (setoran, penarikan, transfer,
// biaya). Nominal diambil dari jurnal asal, sehingga yang perlu dikirim hanya alasannya.
func (h *LedgerHandler) Reverse(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	reference := chi.URLParam(r, "reference")
	if reference == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgTransactionReferenceRequired)
		return
	}

	var req domain.ReversalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgCancelReasonRequired)
		return
	}
	req.Reference = reference
	req.Actor = actor

	entry, err := h.service.Reverse(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrJournalNotFound):
			Error(w, http.StatusNotFound, err.Error())
		case errors.Is(err, domain.ErrJournalAlreadyReversed), errors.Is(err, domain.ErrJournalNotPosted),
			errors.Is(err, domain.ErrReversalNotAllowed), errors.Is(err, domain.ErrSelfReversal):
			Error(w, http.StatusConflict, err.Error())
		default:
			writeTransactionError(w, r, err)
		}
		return
	}

	Success(w, http.StatusCreated, i18n.MsgTransactionCancelled, entry)
}

func (h *LedgerHandler) Deposit(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var req domain.DepositRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	if req.AccountNumber == "" || req.Amount.IsZero() {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgAccountNumberAmountRequired)
		return
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Description == "" {
		req.Description = "Setoran tunai"
	}
	req.Actor = actor

	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.Deposit(r.Context(), req)
	if err != nil {
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, i18n.MsgDepositProcessed, entry)
}

func (h *LedgerHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var req domain.WithdrawRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	if req.AccountNumber == "" || req.Amount.IsZero() {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgAccountNumberAmountRequired)
		return
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Description == "" {
		req.Description = "Penarikan tunai"
	}
	req.Actor = actor

	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.Withdraw(r.Context(), req)
	if err != nil {
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, i18n.MsgWithdrawalProcessed, entry)
}

func (h *LedgerHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var req domain.TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	if req.SourceAccountNumber == "" || req.DestinationAccountNumber == "" || req.Amount.IsZero() {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgTransferFieldsRequired)
		return
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}
	if req.Description == "" {
		req.Description = "Transfer antar rekening"
	}
	req.Actor = actor

	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.TransferInternal(r.Context(), req)
	if err != nil {
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, i18n.MsgTransferExecuted, entry)
}

// PostCompoundJournal handles POST /api/v1/transactions/journals
//
// Jurnal majemuk disusun operator, bukan hasil alur domain (setoran, kredit, EOD),
// sehingga menulis langsung ke buku besar. Minimal dua baris dan setiap nominal harus
// positif; keseimbangan debit-kredit tetap ditegakkan mesin posting. Identitas pencatat
// selalu diambil dari JWT agar jurnal tidak dapat dibuat atas nama orang lain.
func (h *LedgerHandler) PostCompoundJournal(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	var req domain.CustomJournalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if len(req.Lines) < 2 {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgCompoundJournalLinesRequired)
		return
	}
	req.CreatedBy = actor.DisplayName()

	if idem := r.Header.Get("Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	entry, err := h.service.PostCompoundJournal(r.Context(), req)
	if err != nil {
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, i18n.MsgCompoundJournalPosted, entry)
}

func (h *LedgerHandler) GetJournalByRef(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	ref := chi.URLParam(r, "reference")
	if ref == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgReferenceRequired)
		return
	}

	entry, err := h.service.GetJournalByReference(r.Context(), ref)
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	// Pemeriksaan cabang di sini, bukan di service: referensi jurnal juga dipakai
	// document_service yang tidak punya actor. Tanpa ini, pegawai yang mengetahui
	// nomor referensi dapat membaca jurnal cabang lain.
	if !actor.CanAccessBranch(entry.BranchCode) {
		Fail(w, r, http.StatusForbidden, domain.ErrCrossBranchAccess)
		return
	}
	// Buku jurnal diturunkan dari akun barisnya (entry.Book/entry.BookMixed). Jurnal
	// tanpa buku (mis. jurnal sistem) tetap boleh dibaca, sedangkan jurnal lintas buku
	// ditolak, mengikuti semantik CanAccessJournal.
	if !actor.CanAccessJournal(entry.Book, entry.BookMixed) {
		Fail(w, r, http.StatusForbidden, domain.ErrCrossBookAccess)
		return
	}

	Success(w, http.StatusOK, i18n.MsgJournalEntryRetrieved, entry)
}

func (h *LedgerHandler) ListJournals(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
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

	journals, total, err := h.service.ListJournals(r.Context(), page, pageSize, actor)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, i18n.MsgJournalsListed, journals, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}

func (h *LedgerHandler) GetStatement(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	accNum := chi.URLParam(r, "accountNumber")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	lines, total, err := h.service.GetAccountStatement(r.Context(), accNum, page, pageSize, actor)
	if err != nil {
		// Fail, bukan InternalError: Fail memaksa batas keamanan (lintas cabang/buku)
		// menjadi 403; argumen 500 hanya status default yang tidak pernah terpakai
		// untuk penolakan itu.
		Fail(w, r, http.StatusInternalServerError, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, i18n.MsgAccountStatementListed, lines, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}

func (h *LedgerHandler) ListCOA(w http.ResponseWriter, r *http.Request) {
	list, err := h.service.GetChartOfAccounts(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgChartOfAccountsListed, list)
}
