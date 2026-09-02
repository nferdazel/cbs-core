package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
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
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.ApplyLoanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	loan, err := h.loanSvc.ApplyLoan(r.Context(), input, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusCreated, "loan application submitted successfully", loan)
}

// List handles GET /api/v1/loans
func (h *LoanHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	loans, total, err := h.loanSvc.ListLoans(r.Context(), page, pageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	SuccessWithMeta(w, http.StatusOK, "loans retrieved", loans, PaginationMeta{
		Page: page, PageSize: pageSize, TotalItems: total, TotalPages: totalPages,
	})
}

// GetByID handles GET /api/v1/loans/{id}
func (h *LoanHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	loan, err := h.loanSvc.GetLoan(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	Success(w, http.StatusOK, "loan retrieved", loan)
}

// Approve handles POST /api/v1/loans/{id}/approve (Supervisor / Admin)
func (h *LoanHandler) Approve(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	loan, err := h.loanSvc.ApproveLoan(r.Context(), id, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "loan application approved", loan)
}

// Reject handles POST /api/v1/loans/{id}/reject (Supervisor / Admin)
func (h *LoanHandler) Reject(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	loan, err := h.loanSvc.RejectLoan(r.Context(), id, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "loan application rejected", loan)
}

// Disburse handles POST /api/v1/loans/{id}/disburse (Supervisor / Teller)
func (h *LoanHandler) Disburse(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	loan, err := h.loanSvc.DisburseLoan(r.Context(), id, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "loan disbursed to customer account successfully", loan)
}

// PayInstallment handles POST /api/v1/loans/{id}/pay-installment (Teller / AO Collector)
func (h *LoanHandler) PayInstallment(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	var body struct {
		InstallmentNo int `json:"installment_no"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.InstallmentNo <= 0 {
		Error(w, http.StatusBadRequest, "valid installment_no is required")
		return
	}

	input := domain.PayInstallmentInput{
		LoanID:        id,
		InstallmentNo: body.InstallmentNo,
	}

	schedule, err := h.loanSvc.PayInstallment(r.Context(), input, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "installment payment recorded successfully", schedule)
}

// Restructure handles POST /api/v1/loans/{id}/restructure (Supervisor / Admin)
func (h *LoanHandler) Restructure(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	var input domain.RestructureLoanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	input.LoanID = id

	loan, err := h.loanSvc.RestructureLoan(r.Context(), input, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "loan restructured successfully according to OJK rules", loan)
}

// WriteOff handles POST /api/v1/loans/{id}/write-off (Supervisor / Admin)
func (h *LoanHandler) WriteOff(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	var body struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	input := domain.WriteOffLoanInput{
		LoanID: id,
		Reason: body.Reason,
	}

	loan, err := h.loanSvc.WriteOffLoan(r.Context(), input, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "loan written off (hapus buku) successfully", loan)
}

// Recover handles POST /api/v1/loans/{id}/recover (Teller / Supervisor)
func (h *LoanHandler) Recover(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid loan id")
		return
	}

	var input domain.RecoverWrittenOffLoanInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	input.LoanID = id

	loan, err := h.loanSvc.RecoverWrittenOffLoan(r.Context(), input, claims.UserID)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	Success(w, http.StatusOK, "written-off loan recovery payment recorded successfully", loan)
}
