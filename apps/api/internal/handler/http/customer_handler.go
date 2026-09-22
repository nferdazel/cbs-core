package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
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
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var input domain.CreateCustomerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		ErrorCodef(w, http.StatusBadRequest, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}

	// Email opsional: nasabah tanpa email tetap boleh didaftarkan. Bila diisi,
	// formatnya diperiksa di sini sebelum masuk ke service.
	if input.FullName == "" || input.IDCardNumber == "" {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgFullNameIDCardRequired)
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

	Success(w, http.StatusCreated, i18n.MsgCustomerRegistered, cust)
}

func (h *CustomerHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgInvalidCustomerID)
		return
	}

	cust, err := h.service.GetCustomer(r.Context(), id, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgCustomerRetrieved, cust)
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
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

	// q mencari nomor CIF (awalan), NIK (persis, lewat blind index), atau potongan
	// nama (tiap kata dicocokkan lewat token nama).
	search := r.URL.Query().Get("q")

	customers, total, err := h.service.ListCustomers(r.Context(), page, pageSize, search, claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, err)
		return
	}

	totalPages := (total + pageSize - 1) / pageSize

	SuccessWithMeta(w, http.StatusOK, i18n.MsgCustomersListed, customers, PaginationMeta{
		Page:       page,
		PageSize:   pageSize,
		TotalItems: total,
		TotalPages: totalPages,
	})
}
