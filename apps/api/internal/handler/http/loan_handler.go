package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type LoanHandler struct {
	loanSvc domain.LoanService
}

func NewLoanHandler(loanSvc domain.LoanService) *LoanHandler {
	return &LoanHandler{loanSvc: loanSvc}
}

// Apply handles POST /api/v1/loans/apply (AO / Teller)
func (h *LoanHandler) Apply(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.ApplyLoanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	loan, err := h.loanSvc.ApplyLoan(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusCreated, i18n.MsgLoanApplicationSubmitted, loan)
}

// List handles GET /api/v1/loans
func (h *LoanHandler) List(w http.ResponseWriter, r *http.Request) {
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

	loans, total, err := h.loanSvc.ListLoans(r.Context(), page, pageSize, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	SuccessWithMeta(w, http.StatusOK, i18n.MsgLoansRetrieved, loans, PaginationMeta{
		Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages,
	})
}

// GetByID handles GET /api/v1/loans/{id}
func (h *LoanHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	loan, err := h.loanSvc.GetLoan(r.Context(), id, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanRetrieved, loan)
}

// Approve handles POST /api/v1/loans/{id}/approve (Supervisor / Admin)
func (h *LoanHandler) Approve(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	loan, err := h.loanSvc.ApproveLoan(r.Context(), id, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanApproved, loan)
}

// Reject handles POST /api/v1/loans/{id}/reject (Supervisor / Admin)
func (h *LoanHandler) Reject(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	// Body memuat alasan penolakan. Body kosong diteruskan sebagai alasan kosong agar
	// service menolaknya dengan pesan domain yang jelas, bukan galat parsing JSON.
	var input struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil && !errors.Is(err, io.EOF) {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	loan, err := h.loanSvc.RejectLoan(r.Context(), id, input.Reason, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanRejected, loan)
}

// Disburse handles POST /api/v1/loans/{id}/disburse (Supervisor / Teller)
func (h *LoanHandler) Disburse(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	loan, err := h.loanSvc.DisburseLoan(r.Context(), id, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanDisbursed, loan)
}

// PayInstallment handles POST /api/v1/loans/{id}/pay-installment (Teller / AO Collector)
func (h *LoanHandler) PayInstallment(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var body struct {
		InstallmentNo int             `json:"installment_no"`
		Amount        decimal.Decimal `json:"amount"`
		Method        string          `json:"method"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InstallmentNo <= 0 {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInstallmentNoRequired)
		return
	}

	method := domain.LoanPaymentMethod(strings.ToUpper(strings.TrimSpace(body.Method)))
	if method == "" {
		method = domain.LoanPaymentAccount
	}
	if method != domain.LoanPaymentAccount && method != domain.LoanPaymentCash {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgMethodInvalid)
		return
	}

	input := domain.PayInstallmentInput{
		LoanID:        id,
		InstallmentNo: body.InstallmentNo,
		Amount:        body.Amount,
		Method:        method,
	}

	schedule, err := h.loanSvc.PayInstallment(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgInstallmentRecorded, schedule)
}

// Restructure handles POST /api/v1/loans/{id}/restructure (Supervisor / Admin)
func (h *LoanHandler) Restructure(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var input domain.RestructureLoanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	input.LoanID = id

	loan, err := h.loanSvc.RestructureLoan(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanRestructured, loan)
}

// WriteOff handles POST /api/v1/loans/{id}/write-off (Supervisor / Admin)
func (h *LoanHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var body struct {
		Reason            string          `json:"reason"`
		CollectionEfforts string          `json:"collection_efforts"`
		Amount            decimal.Decimal `json:"amount"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	input := domain.WriteOffLoanInput{
		LoanID:            id,
		Reason:            body.Reason,
		CollectionEfforts: body.CollectionEfforts,
		Amount:            body.Amount,
	}

	loan, err := h.loanSvc.WriteOffLoan(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanWrittenOff, loan)
}

// Recover handles POST /api/v1/loans/{id}/recover (Teller / Supervisor)
func (h *LoanHandler) Recover(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var input domain.RecoverWrittenOffLoanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	input.LoanID = id
	// Header diutamakan bila body tidak memuat kunci, mengikuti pola yang sama dengan
	// transaksi setoran/penarikan/transfer.
	if idem := r.Header.Get("Idempotency-Key"); idem != "" && input.IdempotencyKey == "" {
		input.IdempotencyKey = idem
	}

	loan, err := h.loanSvc.RecoverWrittenOffLoan(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanRecoveryRecorded, loan)
}

// CancelDisbursement handles POST /api/v1/loans/{id}/cancel-disbursement
// (Admin / Superadmin). Membatalkan pencairan kredit yang belum memiliki angsuran
// dibayar: jurnal pencairan dibalik, jadwal angsuran dihapus, status CANCELLED.
func (h *LoanHandler) CancelDisbursement(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgCancelReasonRequired)
		return
	}

	loan, err := h.loanSvc.CancelDisbursementLoan(r.Context(), domain.CancelLoanInput{
		LoanID: id,
		Reason: body.Reason,
	}, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// Kredit tidak ditemukan (termasuk cabang lain yang disamarkan) adalah 404,
		// konsisten dengan endpoint baca kredit.
		if errors.Is(err, domain.ErrLoanNotFound) {
			Fail(w, r, http.StatusNotFound, err)
			return
		}
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanDisbursementCancelled, loan)
}

// CorrectAmount handles POST /api/v1/loans/{id}/correct-amount
// (Admin / Superadmin). Mengoreksi nominal pokok kredit yang sudah berjalan tanpa
// membatalkan pencairan; angsuran yang sudah dibayar tetap apa adanya.
func (h *LoanHandler) CorrectAmount(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidLoanID)
		return
	}

	var body struct {
		NewAmount decimal.Decimal `json:"new_amount"`
		Reason    string          `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgCorrectionReasonRequired)
		return
	}
	if body.NewAmount.LessThanOrEqual(decimal.Zero) {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgNewAmountMustBePositive)
		return
	}

	loan, err := h.loanSvc.CorrectLoanAmount(r.Context(), domain.CorrectLoanAmountInput{
		LoanID:    id,
		NewAmount: body.NewAmount,
		Reason:    body.Reason,
	}, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// Kredit tidak ditemukan (termasuk cabang lain yang disamarkan) adalah 404,
		// konsisten dengan endpoint baca kredit.
		if errors.Is(err, domain.ErrLoanNotFound) {
			Fail(w, r, http.StatusNotFound, err)
			return
		}
		writeTransactionError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgLoanAmountCorrected, loan)
}
