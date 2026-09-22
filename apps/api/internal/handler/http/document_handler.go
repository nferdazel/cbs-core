package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DocumentHandler struct {
	docSvc domain.DocumentService
}

func NewDocumentHandler(docSvc domain.DocumentService) *DocumentHandler {
	return &DocumentHandler{docSvc: docSvc}
}

// writeHTMLDocument mengirim dokumen cetak sebagai HTML. CSP diganti dengan kebijakan
// dokumen karena middleware memasang default-src 'none' yang memblokir gaya inline.
func writeHTMLDocument(w http.ResponseWriter, html string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", middleware.DocumentCSP)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(html))
}

// DepositSlip handles GET /api/v1/documents/deposit-slip/{refNo}
func (h *DocumentHandler) DepositSlip(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	refNo := chi.URLParam(r, "refNo")
	if refNo == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgReferenceNumberRequired)
		return
	}

	html, err := h.docSvc.GenerateDepositSlipHTML(r.Context(), refNo, actor)
	if err != nil {
		Fail(w, r, http.StatusInternalServerError, err)
		return
	}

	writeHTMLDocument(w, html)
}

// WithdrawalSlip handles GET /api/v1/documents/withdrawal-slip/{refNo}
func (h *DocumentHandler) WithdrawalSlip(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	refNo := chi.URLParam(r, "refNo")
	if refNo == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgReferenceNumberRequired)
		return
	}

	html, err := h.docSvc.GenerateWithdrawalSlipHTML(r.Context(), refNo, actor)
	if err != nil {
		Fail(w, r, http.StatusInternalServerError, err)
		return
	}

	writeHTMLDocument(w, html)
}

// LoanAgreement handles GET /api/v1/documents/loan-agreement/{loanId}
func (h *DocumentHandler) LoanAgreement(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	idStr := chi.URLParam(r, "loanId")
	id, err := uuid.Parse(idStr)
	if err != nil {
		id = uuid.Nil
	}

	html, err := h.docSvc.GenerateLoanAgreementHTML(r.Context(), id, actor)
	if err != nil {
		Fail(w, r, http.StatusInternalServerError, err)
		return
	}

	writeHTMLDocument(w, html)
}

// ThermalReceipt handles GET /api/v1/documents/thermal-receipt/{receiptNo}
func (h *DocumentHandler) ThermalReceipt(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	receiptNo := chi.URLParam(r, "receiptNo")
	if receiptNo == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgReceiptNumberRequired)
		return
	}

	text, err := h.docSvc.GenerateThermalReceiptText(r.Context(), receiptNo, actor)
	if err != nil {
		Fail(w, r, http.StatusInternalServerError, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(text))
}
