package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"cbs-core/apps/core-api/internal/domain"
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
	var input domain.CreateCustomerInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		Error(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if input.FullName == "" || input.IDCardNumber == "" || input.Email == "" {
		Error(w, http.StatusBadRequest, "full_name, id_card_number, and email are required")
		return
	}

	cust, err := h.service.RegisterCustomer(r.Context(), input)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusCreated, "customer registered successfully", cust)
}

func (h *CustomerHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		Error(w, http.StatusBadRequest, "invalid customer id")
		return
	}

	cust, err := h.service.GetCustomer(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	Success(w, http.StatusOK, "customer retrieved", cust)
}

func (h *CustomerHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	customers, total, err := h.service.ListCustomers(r.Context(), page, pageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
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
