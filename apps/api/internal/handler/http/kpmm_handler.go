package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
	"cbs-core/apps/core-api/internal/ojkreport"
)

// kpmm_handler.go menyajikan laporan KPMM/ATMR BPR (baca saja). Angka diambil dari
// laporan posisi keuangan journal-based, perbandingan PPKA-CKPN, dan konfigurasi
// database; tidak ada rumus akuntansi yang dihitung ulang di handler.
type KPMMHandler struct {
	svc domain.KPMMService
}

// NewKPMMHandler menyusun handler laporan KPMM.
func NewKPMMHandler(svc domain.KPMMService) *KPMMHandler {
	return &KPMMHandler{svc: svc}
}

// Report menghitung dan menyajikan laporan KPMM untuk periode YYYY-MM (default
// bulan lalu). Laporan bersifat bank-wide sehingga hanya peran lintas cabang yang
// boleh membacanya.
func (h *KPMMHandler) Report(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))
	if !actor.IsCrossBranch() {
		Fail(w, r, http.StatusForbidden, fmt.Errorf(
			"%w: laporan KPMM bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
			domain.ErrCrossBranchAccess))
		return
	}
	if h.svc == nil {
		InternalError(w, r, errors.New("layanan KPMM belum dikonfigurasi"))
		return
	}

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}

	report, err := h.svc.Hitung(r.Context(), ojkreport.MonthEnd(period), r.URL.Query().Get("book"), actor)
	if err != nil {
		InternalError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKPMMReport, report)
}
