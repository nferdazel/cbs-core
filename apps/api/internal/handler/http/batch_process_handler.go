package http

import (
	"encoding/json"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
)

// batchWriteTimeout adalah batas waktu penulisan respons untuk batch tutup buku.
// Server memakai WriteTimeout 15 detik, sedangkan akrual seluruh rekening dapat
// berjalan menit-menitan. Tanpa perpanjangan ini koneksi ditutup sebelum hasilnya
// terkirim, dan operator mengira tutup buku gagal padahal jurnalnya sudah masuk —
// lalu menjalankannya ulang.
const batchWriteTimeout = 15 * time.Minute

// extendWriteDeadline memperpanjang batas tulis hanya untuk permintaan ini.
func extendWriteDeadline(w http.ResponseWriter) {
	// Kegagalan (mis. server atau pembungkus yang tidak mendukung deadline) dibiarkan:
	// batas global tetap berlaku, dan batch tetap berjalan sampai selesai.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(batchWriteTimeout))
}

type BatchProcessHandler struct {
	batchSvc domain.BatchProcessService
}

func NewBatchProcessHandler(batchSvc domain.BatchProcessService) *BatchProcessHandler {
	return &BatchProcessHandler{batchSvc: batchSvc}
}

// GetBusinessDate handles GET /api/v1/system/business-date
func (h *BatchProcessHandler) GetBusinessDate(w http.ResponseWriter, r *http.Request) {
	dateInfo, err := h.batchSvc.GetCurrentBusinessDate(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgCurrentBusinessDate, dateInfo)
}

// RunEOD handles POST /api/v1/batch/eod (Supervisor / Admin)
func (h *BatchProcessHandler) RunEOD(w http.ResponseWriter, r *http.Request) {
	extendWriteDeadline(w)
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	result, err := h.batchSvc.RunEOD(r.Context(), claims.UserID)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgEODExecuted, result)
}

// RunEOM handles POST /api/v1/batch/eom (Admin)
// Tarif bunga dan biaya admin tidak lagi diterima dari body: keduanya dibaca dari
// produk dan system_config agar batch tidak bisa dijalankan dengan tarif sembarang.
func (h *BatchProcessHandler) RunEOM(w http.ResponseWriter, r *http.Request) {
	extendWriteDeadline(w)
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	result, err := h.batchSvc.RunEOM(r.Context(), claims.UserID)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgEOMExecuted, result)
}

// RunEOY handles POST /api/v1/batch/eoy (Superadmin / Admin)
// Body opsional: {"book":"CONVENTIONAL"|"SYARIAH"}; kosong berarti kedua buku.
func (h *BatchProcessHandler) RunEOY(w http.ResponseWriter, r *http.Request) {
	extendWriteDeadline(w)
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	var body struct {
		Book string `json:"book"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	// Buku dari body dibatasi di service memakai ConstrainedBook aktor; identitas
	// aktor dibangun dari JWT, bukan dari body, sehingga klaim buku tidak dapat dipalsukan.
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	result, err := h.batchSvc.RunEOY(r.Context(), body.Book, actor)
	if err != nil {
		Fail(w, r, http.StatusUnprocessableEntity, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgEOYExecuted, result)
}
