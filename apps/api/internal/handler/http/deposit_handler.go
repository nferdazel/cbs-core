package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type DepositHandler struct {
	service domain.DepositService
}

func NewDepositHandler(service domain.DepositService) *DepositHandler {
	return &DepositHandler{service: service}
}

// RegisterRoutes mendaftarkan endpoint deposito berjangka. Permission dipilih dari
// domain/staff.go sesuai operasinya:
//   - pembukaan deposito (membuka rekening + jurnal penempatan): PermAccountsOpen
//   - akrual imbal hasil (menyentuh beban/pendapatan): PermTransactionsDeposit
//   - pencairan (kas keluar): PermTransactionsWithdraw
//   - baca kontrak: PermAccountsRead
//
// Parameter perms adalah middleware otorisasi tambahan yang dibungkus pada seluruh
// route (mis. middleware.RequirePermission dari pemanggil); boleh kosong.
func (h *DepositHandler) RegisterRoutes(r chi.Router, perms ...func(http.Handler) http.Handler) {
	r.Route("/deposits", func(r chi.Router) {
		if len(perms) > 0 {
			r.Use(perms...)
		}
		r.With(middleware.RequirePermission(domain.PermAccountsOpen)).
			Post("/place", h.Place)
		r.With(middleware.RequirePermission(domain.PermTransactionsDeposit)).
			Post("/{id}/accrue", h.Accrue)
		r.With(middleware.RequirePermission(domain.PermTransactionsWithdraw)).
			Post("/{id}/withdraw", h.Withdraw)
		r.With(middleware.RequirePermission(domain.PermAccountsRead)).
			Get("/{id}", h.GetByID)
		r.With(middleware.RequirePermission(domain.PermAccountsRead)).
			Get("/", h.List)
	})
}

func (h *DepositHandler) Place(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.PlaceDepositInput
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
	if input.BranchCode == "" {
		input.BranchCode = claims.BranchCode
	}
	if idem := r.Header.Get("Idempotency-Key"); idem != "" && input.IdempotencyKey == "" {
		input.IdempotencyKey = idem
	}

	deposit, err := h.service.Place(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusCreated, "deposit placed successfully", deposit)
}

func (h *DepositHandler) Accrue(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	depositID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "id deposito tidak valid")
		return
	}

	var body domain.AccrueDepositInput
	_ = json.NewDecoder(r.Body).Decode(&body)
	asOf := body.AsOf
	if asOf.IsZero() {
		asOf = time.Now().UTC()
	}

	deposit, err := h.service.Accrue(r.Context(), depositID, asOf, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusOK, "deposit accrued successfully", deposit)
}

func (h *DepositHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	rawID := chi.URLParam(r, "id")
	var body domain.WithdrawDepositInput
	_ = json.NewDecoder(r.Body).Decode(&body)
	if rawID == "" && body.DepositID != uuid.Nil {
		rawID = body.DepositID.String()
	}
	depositID, err := uuid.Parse(rawID)
	if err != nil {
		Error(w, http.StatusBadRequest, "id deposito tidak valid")
		return
	}

	deposit, err := h.service.MatureOrWithdraw(r.Context(), depositID, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}
	Success(w, http.StatusOK, "deposit withdrawn successfully", deposit)
}

func (h *DepositHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	depositID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "id deposito tidak valid")
		return
	}

	deposit, err := h.service.GetByID(r.Context(), depositID)
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}
	Success(w, http.StatusOK, "deposit retrieved", deposit)
}

func (h *DepositHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	deposits, total, err := h.service.List(r.Context(), page, pageSize)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize
	SuccessWithMeta(w, http.StatusOK, "deposits listed", deposits, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}
