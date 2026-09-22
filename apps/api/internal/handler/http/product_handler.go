package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/observability"
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
		InternalError(w, r, err)
		return
	}
	// Produk lini usaha yang tidak aktif di instalasi (atau bukan buku aktor) tidak
	// disajikan, sehingga web tidak menawarkan produk yang akan ditolak jalur tulis.
	if claims, ok := domain.ClaimsFromContext(r.Context()); ok && claims != nil {
		actor := claims.ToActor("", "")
		visible := make([]domain.BankingProduct, 0, len(products))
		for _, p := range products {
			if actor.CanAccessBook(p.Book) {
				visible = append(visible, p)
			}
		}
		products = visible
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
	if claims, ok := domain.ClaimsFromContext(r.Context()); ok && claims != nil {
		if !claims.ToActor("", "").CanAccessBook(product.Book) {
			Fail(w, r, http.StatusForbidden, domain.ErrCrossBookAccess)
			return
		}
	}
	Success(w, http.StatusOK, "detail produk", product)
}

// UpdateParams mengubah parameter produk (tarif, nisbah, batas, biaya) berdasarkan
// kode produk. Otorisasi ada di rute; handler hanya merapikan masukan dan memetakan
// error bisnis ke status HTTP.
//
// Decoder menolak bidang tak dikenal, sehingga payload yang mencoba mengubah
// identitas produk (mis. code, family, book, profit_scheme) ditolak 422 alih-alih
// diam-diam diabaikan.
func (h *ProductHandler) UpdateParams(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		Error(w, http.StatusUnauthorized, "authentication required")
		return
	}

	code := strings.TrimSpace(chi.URLParam(r, "code"))
	if code == "" {
		Error(w, http.StatusUnprocessableEntity, "kode produk wajib diisi")
		return
	}

	var input domain.UpdateProductParamsInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		Error(w, http.StatusUnprocessableEntity, "payload parameter produk tidak valid: "+err.Error())
		return
	}

	product, err := h.service.UpdateParams(r.Context(), code, input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		status := http.StatusUnprocessableEntity
		if errors.Is(err, domain.ErrProductNotFound) {
			status = http.StatusNotFound
		}
		if errors.Is(err, domain.ErrCrossBookAccess) {
			status = http.StatusForbidden
		}
		Fail(w, r, status, err)
		return
	}
	Success(w, http.StatusOK, "parameter produk diperbarui", product)
}
