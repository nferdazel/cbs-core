package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
)

// auditReader adalah kebutuhan handler ini saja: membaca audit log. Antarmuka sempit
// dipilih agar handler tidak ikut bergantung pada kontrak repositori yang lebih luas.
type auditReader interface {
	Query(ctx context.Context, filter domain.AuditLogFilter) ([]domain.AuditEvent, error)
}

type AuditHandler struct {
	repo auditReader
	// limits dipakai rute baca batas transaksi. Field ini dititipkan di handler sistem
	// yang sudah terdaftar karena router.go sedang dipegang perubahan lain; tidak ada
	// handler pengaturan khusus di pohon rute saat ini.
	limits domain.TransactionLimitService
}

func NewAuditHandler(repo auditReader, limits domain.TransactionLimitService) *AuditHandler {
	return &AuditHandler{repo: repo, limits: limits}
}

const (
	auditDefaultLimit = 50
	auditMaxLimit     = 200
	auditDateLayout   = "2006-01-02"
)

// RegisterRoutes memasang rute baca audit log. Wewenang audit_logs:read hanya dimiliki
// peran pengawas (Admin, Superadmin, Auditor): inilah cara auditor memverifikasi jejak
// aksi tanpa akun basis data. Audit log sendiri tetap append-only — rute ini hanya baca.
func (h *AuditHandler) RegisterRoutes(r chi.Router) {
	r.With(middleware.RequirePermission(domain.PermAuditLogsRead)).
		Get("/audit-logs", h.List)
	// Batas transaksi adalah konfigurasi sistem, bukan transaksi: izinnya system:config
	// (hanya Superadmin), bukan izin transaksi. Rute ditempelkan pada RegisterRoutes yang
	// sudah dipanggil router.go agar berkas itu tidak perlu disentuh.
	r.With(middleware.RequirePermission(domain.PermSystemConfig)).
		Get("/system/limits", h.ListTransactionLimits)
}

// TransactionLimitsResponse adalah bentuk respons rute GET /system/limits yang sudah
// ditetapkan. Jangan ubah nama field-nya tanpa menyepakati ulang dengan pemakai.
type TransactionLimitsResponse struct {
	Source string                        `json:"source"`
	Limits []domain.TransactionLimitView `json:"limits"`
}

// ListTransactionLimits handles GET /api/v1/system/limits.
// Mengembalikan nilai efektif per peran x jenis transaksi. configured=false berarti
// sebagian nilai masih bawaan aplikasi dan bank belum menetapkannya.
func (h *AuditHandler) ListTransactionLimits(w http.ResponseWriter, r *http.Request) {
	if h.limits == nil {
		Error(w, http.StatusServiceUnavailable, "layanan batas transaksi belum tersedia")
		return
	}
	views, err := h.limits.List(r.Context())
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, "batas transaksi efektif", TransactionLimitsResponse{
		Source: "config",
		Limits: views,
	})
}

// List handles GET /api/v1/audit-logs
//
// Filter opsional: resource_type, resource_id, actor, action, from, to (YYYY-MM-DD atau
// RFC3339), limit (bawaan 50, maksimum 200), offset. Tanpa filter, hasilnya adalah aksi
// terbaru lebih dulu — menjawab "apa yang baru terjadi" bagi pengawas.
func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	from, err := parseAuditTime(q.Get("from"), false)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	to, err := parseAuditTime(q.Get("to"), true)
	if err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}
	if from != nil && to != nil && !to.After(*from) {
		Error(w, http.StatusBadRequest, "rentang waktu tidak sah: 'to' harus setelah 'from'")
		return
	}

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 {
		limit = auditDefaultLimit
	}
	if limit > auditMaxLimit {
		limit = auditMaxLimit
	}
	offset, _ := strconv.Atoi(q.Get("offset"))
	if offset < 0 {
		offset = 0
	}

	filter := domain.AuditLogFilter{
		ResourceType: strings.TrimSpace(q.Get("resource_type")),
		ResourceID:   strings.TrimSpace(q.Get("resource_id")),
		Actor:        strings.TrimSpace(q.Get("actor")),
		Action:       strings.TrimSpace(q.Get("action")),
		From:         from,
		To:           to,
		Limit:        limit,
		Offset:       offset,
	}

	events, err := h.repo.Query(r.Context(), filter)
	if err != nil {
		InternalError(w, r, err)
		return
	}

	SuccessWithMeta(w, http.StatusOK, "audit log", events, map[string]any{
		"limit":  limit,
		"offset": offset,
		"count":  len(events),
	})
}

// parseAuditTime membaca batas waktu dari query. Nilai kosong berarti tanpa batas.
//
// Tanggal tanpa jam diperlakukan sebagai batas atas yang inklusif: to=2026-09-20 berarti
// "sampai dengan 20 September", bukan "sampai tengah malam 20 September" yang akan
// membuang seluruh aksi pada hari itu.
func parseAuditTime(value string, endOfDay bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}

	if t, err := time.Parse(auditDateLayout, value); err == nil {
		if endOfDay {
			t = t.AddDate(0, 0, 1)
		}
		return &t, nil
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("format waktu %q tidak dikenal; gunakan YYYY-MM-DD atau RFC3339", value)
	}
	return &t, nil
}
