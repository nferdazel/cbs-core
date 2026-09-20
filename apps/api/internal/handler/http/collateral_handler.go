package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
)

type CollateralHandler struct {
	svc domain.CollateralService
}

func NewCollateralHandler(svc domain.CollateralService) *CollateralHandler {
	return &CollateralHandler{svc: svc}
}

// RegisterRoutes memasang rute agunan kredit. Pencatatan hanya untuk pegawai kredit
// (wewenang collateral:manage), pembacaan untuk yang berwenang membaca agunan.
func (h *CollateralHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermCollateralManage)).
		Post("/loans/{loanId}/collaterals", h.Create)
	r.With(middleware.RequirePermission(domain.PermCollateralRead)).
		Get("/loans/{loanId}/collaterals", h.ListByLoan)
	r.With(middleware.RequirePermission(domain.PermCollateralRead)).
		Get("/collaterals/{id}", h.GetByID)
}

// collateralRequest adalah bentuk permintaan pencatatan agunan. Nilai taksasi dan haircut
// diterima sebagai string desimal agar tidak kehilangan presisi seperti pada angka pecahan.
type collateralRequest struct {
	CollateralType string           `json:"collateral_type"`
	Description    string           `json:"description"`
	DocumentNumber string           `json:"document_number"`
	OwnerName      string           `json:"owner_name"`
	AppraisalValue decimal.Decimal  `json:"appraisal_value"`
	AppraisalDate  string           `json:"appraisal_date"`
	Appraiser      string           `json:"appraiser"`
	HaircutPercent *decimal.Decimal `json:"haircut_percent,omitempty"`
	Notes          string           `json:"notes"`
}

// Create handles POST /api/v1/loans/{loanId}/collaterals
func (h *CollateralHandler) Create(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	loanID, err := uuid.Parse(chi.URLParam(r, "loanId"))
	if err != nil {
		Error(w, http.StatusBadRequest, "id kredit tidak valid")
		return
	}

	var req collateralRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	appraisalDate, err := parseDateOrRFC3339(strings.TrimSpace(req.AppraisalDate))
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	collateral, err := h.svc.Create(r.Context(), domain.CollateralInput{
		LoanID:         loanID,
		CollateralType: domain.CollateralType(strings.ToUpper(strings.TrimSpace(req.CollateralType))),
		Description:    req.Description,
		DocumentNumber: req.DocumentNumber,
		OwnerName:      req.OwnerName,
		AppraisalValue: req.AppraisalValue,
		AppraisalDate:  appraisalDate,
		Appraiser:      req.Appraiser,
		HaircutPercent: req.HaircutPercent,
		Notes:          req.Notes,
	}, actor)
	if err != nil {
		writeCollateralError(w, err)
		return
	}

	Success(w, http.StatusCreated, "agunan tercatat", collateral)
}

// ListByLoan handles GET /api/v1/loans/{loanId}/collaterals
func (h *CollateralHandler) ListByLoan(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	loanID, err := uuid.Parse(chi.URLParam(r, "loanId"))
	if err != nil {
		Error(w, http.StatusBadRequest, "id kredit tidak valid")
		return
	}

	list, err := h.svc.ListByLoan(r.Context(), loanID, actor)
	if err != nil {
		writeCollateralError(w, err)
		return
	}
	Success(w, http.StatusOK, "daftar agunan kredit", list)
}

// GetByID handles GET /api/v1/collaterals/{id}
func (h *CollateralHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "id agunan tidak valid")
		return
	}

	collateral, err := h.svc.GetByID(r.Context(), id, actor)
	if err != nil {
		writeCollateralError(w, err)
		return
	}
	Success(w, http.StatusOK, "agunan", collateral)
}

// writeCollateralError memetakan kesalahan agunan ke status HTTP. Agunan yang tidak ada
// dan agunan cabang lain sama-sama 404 — membedakannya memberi tahu pemanggil bahwa
// agunan dengan id tertentu memang ada di bank ini.
func writeCollateralError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrCollateralNotFound):
		Error(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrCollateralDocumentRequired),
		errors.Is(err, domain.ErrCollateralOwnerRequired),
		errors.Is(err, domain.ErrCollateralAppraisalInvalid),
		errors.Is(err, domain.ErrCollateralAppraisalDateInvalid),
		errors.Is(err, domain.ErrCollateralHaircutInvalid):
		Error(w, http.StatusUnprocessableEntity, err.Error())
	default:
		// Kesalahan lain (mis. aturan domain yang belum punya sentinel) tetap ditampilkan
		// sebagai permintaan tidak dapat diproses, bukan kegagalan server.
		Error(w, http.StatusUnprocessableEntity, err.Error())
	}
}

// parseDateOrRFC3339 menerima tanggal (YYYY-MM-DD) maupun waktu RFC3339.
func parseDateOrRFC3339(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, errBadDate("tanggal taksasi wajib diisi")
	}
	if t, err := time.Parse(auditDateLayout, value); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	return time.Time{}, errBadDate("format tanggal taksasi tidak dikenal; gunakan YYYY-MM-DD")
}

type errBadDate string

func (e errBadDate) Error() string { return string(e) }
