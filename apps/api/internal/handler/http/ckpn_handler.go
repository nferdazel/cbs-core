package http

import (
	"errors"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"github.com/go-chi/chi/v5"
)

type CKPNHandler struct {
	ckpnSvc domain.CKPNService
	// config dipakai endpoint status parameter SEMENTARA/FINAL agar dapat diperiksa
	// tanpa menjalankan EOD. Boleh nil (uji lama): status diperlakukan SEMENTARA.
	config domain.SystemConfigService
	// Individual menyajikan CKPN individual tahap T1 (mode bayangan baca-saja) dan
	// input proyeksi arus kas manual. Boleh nil: rute individual tidak dipasang, sehingga
	// instalasi/uji yang belum menyiapkannya berperilaku sama seperti sebelumnya.
	Individual domain.CKPNIndividualService
}

func NewCKPNHandler(ckpnSvc domain.CKPNService, config domain.SystemConfigService) *CKPNHandler {
	return &CKPNHandler{ckpnSvc: ckpnSvc, config: config}
}

// Status handles GET /api/v1/ckpn/status.
// Menyajikan status parameter CKPN (SEMENTARA/FINAL), tanggal mulai, batas ratifikasi
// 12 bulan, apakah lantai PPKA ditegakkan, apakah laporan OJK diblokir beserta
// alasannya, dan peringatan yang sama dengan yang muncul saat start/EOD. Baca-saja;
// tidak menjalankan EOD dan tidak mengubah konfigurasi.
func (h *CKPNHandler) Status(w http.ResponseWriter, r *http.Request) {
	status := domain.CKPNParametersStatusFromConfig(r.Context(), h.config, time.Now())
	Success(w, http.StatusOK, i18n.MsgCKPNParametersStatus, status)
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
		// Status parameter dapat diperiksa tanpa menjalankan EOD: peringatan SEMENTARA,
		// batas ratifikasi, dan status blokir ekspor OJK. Cukup izin baca kredit.
		r.With(middleware.RequirePermission(domain.PermLoansRead)).
			Get("/status", h.Status)

		// CKPN individual T1 (mode bayangan baca-saja). Hanya dipasang bila layanan
		// disiapkan; menilai cukup izin baca kredit, mengisi proyeksi arus kas (data
		// operasional pengelola kredit) memakai izin approve kredit.
		if h.Individual != nil {
			r.With(middleware.RequirePermission(domain.PermLoansRead)).
				Get("/individual/{loanNumber}/assessment", h.IndividualAssessment)
			r.With(middleware.RequirePermission(domain.PermLoansApprove)).
				Put("/individual/{loanNumber}/projections", h.ReplaceIndividualProjections)
		}
	})
}
