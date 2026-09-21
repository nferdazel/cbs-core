package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CustomerHandler struct {
	service domain.CustomerService
}

func NewCustomerHandler(service domain.CustomerService) *CustomerHandler {
	return &CustomerHandler{service: service}
}

func (h *CustomerHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var input domain.CreateCustomerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Email opsional: nasabah tanpa email tetap boleh didaftarkan. Bila diisi,
	// formatnya diperiksa di sini sebelum masuk ke service.
	if input.FullName == "" || input.IDCardNumber == "" {
		Error(w, http.StatusBadRequest, "full_name and id_card_number are required")
		return
	}
	if err := domain.ValidateEmail(input.Email); err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}

	cust, err := h.service.RegisterCustomer(r.Context(), input, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, domain.ErrDuplicateIDCard) || errors.Is(err, domain.ErrDuplicateEmail) {
			status = http.StatusConflict
		}
		Error(w, status, err.Error())
		return
	}

	Success(w, http.StatusCreated, "customer registered successfully", cust)
}

func (h *CustomerHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	cust, err := h.service.GetCustomer(r.Context(), id, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	Success(w, http.StatusOK, "customer retrieved", cust)
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
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

	// q mencari nomor CIF (awalan) atau NIK (persis, lewat blind index).
	search := r.URL.Query().Get("q")

	customers, total, err := h.service.ListCustomers(r.Context(), page, pageSize, search, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, "customers listed", customers, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}
