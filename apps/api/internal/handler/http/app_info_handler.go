package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
)

// AppInfoHandler melayani identitas aplikasi publik (GET /api/v1/app-info).
// Endpoint ini sengaja tanpa autentikasi karena dipakai halaman login dan metadata
// web sebelum sesi ada; isinya hanya identitas publik, bukan data internal.
type AppInfoHandler struct {
	svc domain.AppInfoService
}

func NewAppInfoHandler(svc domain.AppInfoService) *AppInfoHandler {
	return &AppInfoHandler{svc: svc}
}

// Get mengirim identitas aplikasi. Cache-Control dipasang agar browser/proxy tidak
// menanyakan ulang tiap muat halaman; service juga menyimpan hasilnya sebentar.
func (h *AppInfoHandler) Get(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "public, max-age=60")
	Success(w, http.StatusOK, "identitas aplikasi", h.svc.Get(r.Context()))
}
