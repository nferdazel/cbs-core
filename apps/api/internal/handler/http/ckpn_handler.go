package http

import (
	"errors"
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
)

type CKPNHandler struct {
	ckpnSvc domain.CKPNService
}

func NewCKPNHandler(ckpnSvc domain.CKPNService) *CKPNHandler {
	return &CKPNHandler{ckpnSvc: ckpnSvc}
}

// Compare handles GET /api/v1/ckpn/comparison.
// Menyajikan perbandingan CKPN vs PPKA per kredit dan total tanpa menulis apa pun.
// Tanggal proses opsional lewat query `as_of` (lihat parseAsOf). Aktor diambil dari
// claims agar perbandingan hanya memuat kredit cabangnya; aktor lintas cabang
// (SUPERADMIN/AUDITOR) melihat seluruh bank.
func (h *CKPNHandler) Compare(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	summary, err := h.ckpnSvc.Compare(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		// PPKA yang basi (tanggal bisnis PPAP berbeda) adalah kondisi yang dapat
		// diperbaiki operator, bukan kegagalan server: balas 409 dengan pesan yang
		// menyebut tanggal bisnisnya, bukan 500 generik.
		if errors.Is(err, domain.ErrCKPNStalePPAP) {
			Fail(w, r, http.StatusConflict, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgCKPNPPKAComparison, summary)
}

// Run handles POST /api/v1/ckpn/run.
// Menghitung CKPN, memposting selisihnya, dan menyimpan target per kredit. Tanggal
// proses opsional lewat query `as_of`.
func (h *CKPNHandler) Run(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}

	summary, err := h.ckpnSvc.Run(r.Context(), parseAsOf(r),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		if errors.Is(err, domain.ErrCKPNStalePPAP) {
			Fail(w, r, http.StatusConflict, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgCKPNCalculated, summary)
}

// RegisterRoutes memasang rute CKPN pada router yang sudah berada di dalam grup
// terautentikasi (AuthMiddleware). Menjalankan perhitungan (yang memposting jurnal)
// butuh wewenang approve kredit; perbandingan cukup wewenang baca.
//
// KEPUTUSAN EKSPLISIT cakupan lintas cabang: Run diizinkan bank-wide, tetapi hanya
// lewat aktor yang memang berwenang lintas cabang (SUPERADMIN/AUDITOR/SYSTEM, lihat
// Actor.IsCrossBranch). Aktor cabang otomatis terbatas pada cabangnya karena
// service membaca kredit memakai filter branchReadClause atas actor. Satu run
// bank-wide diperlukan agar tutup hari tidak harus dijalankan per cabang; jurnalnya
// tetap diatribusikan ke cabang KREDIT, bukan cabang aktor, sehingga buku cabang
// tidak tercampur. Tidak ada jalur yang membiarkan aktor cabang memproses cabang lain.
func (h *CKPNHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ckpn", func(r chi.Router) {
		r.With(middleware.RequirePermission(domain.PermLoansApprove)).
			Post("/run", h.Run)
		r.With(middleware.RequirePermission(domain.PermLoansRead)).
			Get("/comparison", h.Compare)
	})
}
