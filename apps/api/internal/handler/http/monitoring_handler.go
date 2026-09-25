package http

import (
	"context"
	"net/http"

	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/monitoring"
)

// MonitoringService menyajikan potret kesehatan operasional. Implementasi produksi
// adalah *monitoring.Collector.
type MonitoringService interface {
	Collect(ctx context.Context) monitoring.Snapshot
}

// MonitoringHandler melayani GET /api/v1/system/monitoring. Otorisasi ada di rute
// (system:config:read — izin baca konfigurasi/status instalasi yang sudah dipakai
// bank-profile dan riwayat EOD), sehingga tidak ada izin baru yang dibuat. Isinya
// baca-saja: handler tidak mengubah state apa pun.
type MonitoringHandler struct {
	svc MonitoringService
}

func NewMonitoringHandler(svc MonitoringService) *MonitoringHandler {
	return &MonitoringHandler{svc: svc}
}

// Get handles GET /api/v1/system/monitoring
// Mengembalikan temuan ber-severity beserta cacahnya. Endpoint selalu 200 selama
// permintaan terautentikasi: kegagalan membaca satu sumber data adalah temuan,
// bukan kegagalan endpoint, agar pemantauan tetap memberi gambaran.
func (h *MonitoringHandler) Get(w http.ResponseWriter, r *http.Request) {
	Success(w, http.StatusOK, i18n.MsgMonitoringSnapshot, h.svc.Collect(r.Context()))
}
