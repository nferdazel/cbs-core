package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DocumentHandler struct {
	docSvc domain.DocumentService
}

func NewDocumentHandler(docSvc domain.DocumentService) *DocumentHandler {
	return &DocumentHandler{docSvc: docSvc}
}

// DepositSlip handles GET /api/v1/documents/deposit-slip/{refNo}
func (h *DocumentHandler) DepositSlip(w http.ResponseWriter, r *http.Request) {
	refNo := chi.URLParam(r, "refNo")
	if refNo == "" {
		Error(w, http.StatusBadRequest, "reference number is required")
		return
	}

	html, err := h.docSvc.GenerateDepositSlipHTML(r.Context(), refNo)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// WithdrawalSlip handles GET /api/v1/documents/withdrawal-slip/{refNo}
func (h *DocumentHandler) WithdrawalSlip(w http.ResponseWriter, r *http.Request) {
	refNo := chi.URLParam(r, "refNo")
	if refNo == "" {
		Error(w, http.StatusBadRequest, "reference number is required")
		return
	}

	html, err := h.docSvc.GenerateWithdrawalSlipHTML(r.Context(), refNo)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// LoanAgreement handles GET /api/v1/documents/loan-agreement/{loanId}
func (h *DocumentHandler) LoanAgreement(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "loanId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		id = uuid.Nil
	}

	html, err := h.docSvc.GenerateLoanAgreementHTML(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// ThermalReceipt handles GET /api/v1/documents/thermal-receipt/{receiptNo}
func (h *DocumentHandler) ThermalReceipt(w http.ResponseWriter, r *http.Request) {
	receiptNo := chi.URLParam(r, "receiptNo")
	if receiptNo == "" {
		Error(w, http.StatusBadRequest, "receipt number is required")
		return
	}

	text, err := h.docSvc.GenerateThermalReceiptText(r.Context(), receiptNo)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}
