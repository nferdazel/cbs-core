package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type ProductHandler struct {
	service domain.ProductService
}

func NewProductHandler(service domain.ProductService) *ProductHandler {
	return &ProductHandler{service: service}
}

func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	products, err := h.service.ListProducts(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, "gagal memuat daftar produk")
		return
	}
	Success(w, http.StatusOK, "daftar produk", products)
}

func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		Error(w, http.StatusBadRequest, "id produk tidak valid")
		return
	}

	product, err := h.service.GetProduct(r.Context(), id)
	if err != nil {
		Fail(w, r, http.StatusNotFound, err)
		return
	}
	Success(w, http.StatusOK, "detail produk", product)
}
