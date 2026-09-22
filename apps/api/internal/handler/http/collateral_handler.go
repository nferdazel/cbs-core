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
		Get("/collateral/summary", h.Summary)
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
	// Penanda kepatuhan Pasal 20/21 POJK No. 1 Tahun 2024. Boolean biasa berarti "tidak"
	// bila tidak dikirim (mis. belum ber-hak tanggungan = tidak boleh jadi pengurang),
	// sedangkan ExistsKnown/Executable berupa pointer karena keadaan normalnya "ya".
	AppraiserIndependent bool            `json:"appraiser_independent,omitempty"`
	Certified            bool            `json:"certified,omitempty"`
	Mortgaged            bool            `json:"mortgaged,omitempty"`
	MortgageValue        decimal.Decimal `json:"mortgage_value,omitempty"`
	ExistsKnown          *bool           `json:"exists_known,omitempty"`
	Executable           *bool           `json:"executable,omitempty"`
	ThirdPartyOwner      bool            `json:"third_party_owner,omitempty"`
	OwnerConsent         bool            `json:"owner_consent,omitempty"`
	// Agunan tunai Pasal 17 dan rekening tempat dananya diblokir. Penanda ini memindahkan
	// agunan dari pengurang PPKA khusus ke pengecualian PPKA umum (Pasal 19(4)(b)).
	IsCash        bool   `json:"is_cash,omitempty"`
	CashAccountID string `json:"cash_account_id,omitempty"`
	// Tanggal penilaian resi gudang untuk pita umur Pasal 20(1) huruf c/h/j; kosong
	// berarti memakai tanggal taksasi agunan.
	WarehouseReceiptValuedAt string `json:"warehouse_receipt_valued_at,omitempty"`
	// Dasar pengurang Pasal 20(1) huruf d/e: NJOP (atau nilai penilai independen) beserta
	// tanggal dan asalnya. Nilai dan asal harus dikirim berpasangan.
	NJOPValue  decimal.Decimal `json:"njop_value,omitempty"`
	NJOPDate   string          `json:"njop_date,omitempty"`
	NJOPSource string          `json:"njop_source,omitempty"`
	// Penanda kriteria penjamin BUMN/BUMD Pasal 20(1) huruf i beserta buktinya.
	BumnBumdCriteriaMet bool   `json:"bumn_bumd_criteria_met,omitempty"`
	BumnBumdEvidence    string `json:"bumn_bumd_evidence,omitempty"`
	Notes               string `json:"notes"`
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

	// Rekening agunan tunai wajib dapat diurai bila dikirim; agunan tunai tanpa rekening
	// ditolak domain karena pemblokirannya tidak dapat diaudit (Pasal 17 ayat (3)).
	var cashAccountID *uuid.UUID
	if raw := strings.TrimSpace(req.CashAccountID); raw != "" {
		id, err := uuid.Parse(raw)
		if err != nil {
			Error(w, http.StatusBadRequest, "id rekening agunan tunai tidak valid")
			return
		}
		cashAccountID = &id
	}

	var warehouseReceiptValuedAt *time.Time
	if raw := strings.TrimSpace(req.WarehouseReceiptValuedAt); raw != "" {
		t, err := parseDateOrRFC3339(raw)
		if err != nil {
			Error(w, http.StatusBadRequest, err.Error())
			return
		}
		warehouseReceiptValuedAt = &t
	}

	// Tanggal NJOP boleh kosong; bila diisi, formatnya harus dikenal. Pesannya khusus NJOP
	// agar operator tidak mengira tanggal taksasi yang salah.
	var njopDate *time.Time
	if raw := strings.TrimSpace(req.NJOPDate); raw != "" {
		t, err := parseDateOrRFC3339(raw)
		if err != nil {
			Error(w, http.StatusBadRequest, "format tanggal NJOP tidak dikenal; gunakan YYYY-MM-DD")
			return
		}
		njopDate = &t
	}

	// Asal NJOP harus salah satu nilai yang dikenal. Nilai tak dikenal ditolak, bukan
	// diperlakukan sebagai NJOP (Pasal 20 ayat (1) huruf d/e).
	var njopSource *domain.NJOPSource
	if raw := strings.TrimSpace(req.NJOPSource); raw != "" {
		s := domain.NJOPSource(strings.ToUpper(raw))
		if !s.Valid() {
			Error(w, http.StatusBadRequest, domain.ErrCollateralNJOPSourceInvalid.Error())
			return
		}
		njopSource = &s
	}

	collateral, err := h.svc.Create(r.Context(), domain.CollateralInput{
		LoanID:                   loanID,
		CollateralType:           domain.CollateralType(strings.ToUpper(strings.TrimSpace(req.CollateralType))),
		Description:              req.Description,
		DocumentNumber:           req.DocumentNumber,
		OwnerName:                req.OwnerName,
		AppraisalValue:           req.AppraisalValue,
		AppraisalDate:            appraisalDate,
		Appraiser:                req.Appraiser,
		HaircutPercent:           req.HaircutPercent,
		AppraiserIndependent:     req.AppraiserIndependent,
		Certified:                req.Certified,
		Mortgaged:                req.Mortgaged,
		MortgageValue:            req.MortgageValue,
		ExistsKnown:              req.ExistsKnown,
		Executable:               req.Executable,
		ThirdPartyOwner:          req.ThirdPartyOwner,
		OwnerConsent:             req.OwnerConsent,
		IsCash:                   req.IsCash,
		CashAccountID:            cashAccountID,
		WarehouseReceiptValuedAt: warehouseReceiptValuedAt,
		NJOPValue:                req.NJOPValue,
		NJOPDate:                 njopDate,
		NJOPSource:               njopSource,
		BumnBumdCriteriaMet:      req.BumnBumdCriteriaMet,
		BumnBumdEvidence:         req.BumnBumdEvidence,
		Notes:                    req.Notes,
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

// Summary handles GET /api/v1/collateral/summary
// Rekap agunan AKTIF per jenis untuk cabang aktor (seluruh bank bila aktor lintas cabang).
func (h *CollateralHandler) Summary(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}

	list, err := h.svc.Summary(r.Context(), actor)
	if err != nil {
		writeCollateralError(w, err)
		return
	}
	Success(w, http.StatusOK, "rekap agunan aktif", list)
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
	case errors.Is(err, domain.ErrCrossBookAccess), errors.Is(err, domain.ErrCrossBranchAccess):
		Error(w, http.StatusForbidden, err.Error())
	case errors.Is(err, domain.ErrCollateralDocumentRequired),
		errors.Is(err, domain.ErrCollateralOwnerRequired),
		errors.Is(err, domain.ErrCollateralAppraisalInvalid),
		errors.Is(err, domain.ErrCollateralAppraisalDateInvalid),
		errors.Is(err, domain.ErrCollateralHaircutInvalid),
		errors.Is(err, domain.ErrCollateralCashAccountRequired),
		errors.Is(err, domain.ErrCollateralNJOPRequired),
		errors.Is(err, domain.ErrCollateralNJOPInvalid),
		errors.Is(err, domain.ErrCollateralNJOPDateInvalid),
		errors.Is(err, domain.ErrCollateralNJOPSourceInvalid),
		errors.Is(err, domain.ErrCollateralBumnEvidenceRequired):
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
