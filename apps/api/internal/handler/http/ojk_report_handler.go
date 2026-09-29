package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"cbs-core/apps/core-api/internal/ojkreport"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// OJKCOALister membaca bagan akun untuk melengkapi peninjauan pemetaan dengan nama akun.
// Kontraknya sengaja sempit agar handler tidak bergantung pada layanan ledger penuh.
type OJKCOALister interface {
	// GetCOAList menerima aktor karena bagan akun disaring buku. Peninjauan pemetaan
	// bersifat bank-wide, jadi pemanggil memakai aktor lintas buku (RoleSystem).
	GetCOAList(ctx context.Context, actor domain.Actor) ([]domain.ChartOfAccount, error)
}

// OJKPlacementLister menyediakan daftar penempatan pada bank lain untuk pemilih UI
// pengisian sandi OJK. Penyaringan cakupan cabang dilakukan pemanggil di repositori.
type OJKPlacementLister interface {
	ListPlacements(ctx context.Context, asOf time.Time, actor domain.Actor) ([]domain.LPSPlacement, error)
}

// OJKReportHandler melayani fondasi ekspor laporan OJK (APOLO). Angka diambil
// dari laporan journal-based yang sudah ada; handler hanya menyajikan.
type OJKReportHandler struct {
	builder *ojkreport.Builder
	// source disimpan untuk menghitung kode COA yang belum terpetakan pada peninjauan.
	source ojkreport.Source
	// coa dan reviews dipakai endpoint peninjauan pemetaan; boleh nil pada uji lama.
	coa     OJKCOALister
	reviews ojkreport.MappingReviewRepository
	// config dipakai menolak ekspor selama parameter CKPN SEMENTARA (butir 1.5).
	config domain.SystemConfigService
	// bmpk menyediakan laporan BMPK untuk endpoint ekspor /ojk/bmpk; nil berarti
	// sumber BMPK tidak dirangkai pada handler ini.
	bmpk ojkreport.BMPKSource
	// bmpkAdmin menyediakan pembacaan master dan jalur tulis pihak terkait/batas BMPK
	// (izin system:config). Boleh nil pada uji; tanpa itu rute pengaturan BMPK tetap
	// terpasang tetapi gagal saat dipanggil.
	bmpkAdmin domain.BMPKService
	// loans menyediakan baris kredit untuk endpoint ekspor Laporan Perbedaan Kualitas
	// Aset Produktif; nil berarti sumber kredit tidak dirangkai pada handler ini.
	loans ojkreport.LoanDataSource
	// kelembagaan menyediakan data jaringan kantor/direksi/komisaris/pejabat eksekutif
	// untuk laporan LAPORAN_KELEMBAGAAN sekaligus operasi upsert/hapusnya. Boleh nil
	// pada uji; tanpa itu rute kelembagaan tetap terpasang tetapi gagal saat dipanggil.
	kelembagaan domain.KelembagaanService
	// offBalance menyediakan register rekening administratif (Form 01.01) sekaligus
	// operasi upsert/hapusnya. Boleh nil pada uji; tanpa itu rute rekening administratif
	// tetap terpasang tetapi gagal saat dipanggil.
	offBalance domain.OffBalanceService
	// ayda menyediakan register AYDA (Form 07.00) sekaligus operasi upsert/hapusnya.
	// Boleh nil pada uji; tanpa itu rute register AYDA tetap terpasang tetapi gagal
	// saat dipanggil.
	ayda domain.AYDARegisterService
	// kepemilikan menyediakan register pemegang saham (Form 00.01) sekaligus operasi
	// upsert/hapusnya. Boleh nil pada uji; tanpa itu rute register kepemilikan tetap
	// terpasang tetapi gagal saat dipanggil.
	kepemilikan domain.KepemilikanRegisterService
	// pinjaman menyediakan register pinjaman yang diterima (Form 00.07) sekaligus
	// operasi upsert/hapusnya. Boleh nil pada uji; tanpa itu rute register pinjaman
	// tetap terpasang tetapi gagal saat dipanggil.
	pinjaman domain.PinjamanRegisterService
	// placements menyediakan daftar penempatan pada bank lain untuk pemilih UI sandi
	// OJK (Form 05.00). Boleh nil pada uji; tanpa itu rute daftar tetap terpasang
	// tetapi gagal saat dipanggil.
	placements OJKPlacementLister
}

// NewOJKReportHandler menyusun handler ekspor sekaligus peninjauan pemetaan. coa dan
// reviews boleh nil, tetapi endpoint peninjauan membutuhkannya. config boleh nil pada
// uji lama; tanpa itu ekspor diperlakukan gagal-aman (parameter dianggap SEMENTARA).
// Sumber BMPK diambil dari source bila memenuhi kontraknya (RepoSource). kelembagaan
// boleh nil; tanpa itu endpoint kelembagaan gagal saat dipanggil, bukan saat didaftarkan.
// offBalance boleh nil dengan perilaku yang sama. ayda menyediakan register AYDA
// (Form 07.00) dan boleh nil dengan perilaku yang sama. kepemilikan menyediakan
// register pemegang saham (Form 00.01) dan boleh nil dengan perilaku yang sama.
// pinjaman menyediakan register pinjaman yang diterima (Form 00.07) dan boleh nil
// dengan perilaku yang sama. bmpkAdmin menyediakan pengaturan pihak terkait/batas
// BMPK; boleh nil dengan perilaku yang sama. placements menyediakan daftar penempatan
// untuk pemilih UI sandi OJK; boleh nil dengan perilaku yang sama.
func NewOJKReportHandler(source ojkreport.Source, coa OJKCOALister, reviews ojkreport.MappingReviewRepository, config domain.SystemConfigService, kelembagaan domain.KelembagaanService, offBalance domain.OffBalanceService, ayda domain.AYDARegisterService, kepemilikan domain.KepemilikanRegisterService, pinjaman domain.PinjamanRegisterService, bmpkAdmin domain.BMPKService, placements OJKPlacementLister) *OJKReportHandler {
	h := &OJKReportHandler{
		builder:     ojkreport.NewBuilder(source),
		source:      source,
		coa:         coa,
		reviews:     reviews,
		config:      config,
		kelembagaan: kelembagaan,
		offBalance:  offBalance,
		ayda:        ayda,
		kepemilikan: kepemilikan,
		pinjaman:    pinjaman,
		bmpkAdmin:   bmpkAdmin,
		placements:  placements,
	}
	if bs, ok := source.(ojkreport.BMPKSource); ok {
		h.bmpk = bs
	}
	if ls, ok := source.(ojkreport.LoanDataSource); ok {
		h.loans = ls
	}
	return h
}

// RegisterRoutes memasang rute di bawah /reports/ojk. Izin reports:export dipakai
// karena seluruh endpoint menghasilkan data laporan untuk diekspor.
//
// Menandai keputusan bank bukan tindakan membaca laporan, melainkan pengaturan pedoman
// konversi bagan akun. Karena itu endpoint keputusan memakai coa:manage yang menurut
// domain.RolePermissions hanya dipegang SUPERADMIN dan ADMIN; AUDITOR tetap dapat
// membaca pemetaan untuk memeriksanya tanpa dapat mengubah keputusan bank.
func (h *OJKReportHandler) RegisterRoutes(r chi.Router) {
	r.Route("/ojk", func(r chi.Router) {
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/definitions", h.Definitions)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/monthly", h.ExportMonthly)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/bmpk", h.ExportBMPK)
		// Pengaturan BMPK (pihak terkait + batas) yang sebelumnya hanya bisa diisi
		// SQL/seed: baca master dan jalur tulis memakai system:config, izin yang sama
		// dengan setelan OJK lain, dan teraudit di service.
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Get("/bmpk/master", h.BMPKMaster)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/bmpk/related-parties", h.UpsertBMPKRelatedParty)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/bmpk/related-parties/{customerId}", h.DeleteBMPKRelatedParty)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/bmpk/limits", h.UpsertBMPKLimit)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/bmpk/limits/{customerId}", h.DeleteBMPKLimit)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/perbedaan-kualitas", h.ExportPerbedaanKualitas)
		// Laporan kelembagaan (jaringan kantor + direksi/komisaris/pejabat eksekutif):
		// baca cukup reports:export; pengisian data memakai system:config, izin yang
		// sama dengan setelan OJK lain, dan teraudit di service. Hanya lima rute agar
		// tidak menumpuk detail.
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/kelembagaan", h.ExportKelembagaan)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/kelembagaan/offices", h.UpsertKelembagaanOffice)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/kelembagaan/offices/{id}", h.DeleteKelembagaanOffice)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/kelembagaan/management", h.UpsertKelembagaanManagement)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/kelembagaan/management/{id}", h.DeleteKelembagaanManagement)
		// Register rekening administratif (Form 01.01): baca cukup reports:export;
		// pengisian dan daftar mentah UI memakai system:config dan teraudit di service.
		// Empat rute saja.
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/off-balance", h.ExportOffBalance)
		// Daftar baris mentah register rekening administratif untuk UI edit memakai
		// system:config, sama seperti jalur tulisnya.
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Get("/off-balance/items", h.ListOffBalanceItems)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/off-balance/items", h.UpsertOffBalanceItem)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/off-balance/items/{id}", h.DeleteOffBalanceItem)
		// Register AYDA (Form 07.00): baca cukup reports:export; pengisian dan daftar
		// mentah UI memakai system:config dan teraudit di service. Empat rute saja.
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/ayda", h.ExportAYDA)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Get("/ayda/items", h.ListAYDAItems)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/ayda/items", h.UpsertAYDAItem)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/ayda/items/{id}", h.DeleteAYDAItem)
		// Register pemegang saham (Form 00.01): baca cukup reports:export; pengisian
		// dan daftar mentah UI memakai system:config dan teraudit di service. Empat
		// rute saja.
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/kepemilikan", h.ExportKepemilikan)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Get("/kepemilikan/items", h.ListKepemilikanItems)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/kepemilikan/items", h.UpsertKepemilikanItem)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/kepemilikan/items/{id}", h.DeleteKepemilikanItem)
		// Register pinjaman yang diterima (Form 00.07): baca cukup reports:export;
		// pengisian dan daftar mentah UI memakai system:config dan teraudit di service.
		// Empat rute saja.
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/pinjaman", h.ExportPinjaman)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Get("/pinjaman/items", h.ListPinjamanItems)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Put("/pinjaman/items", h.UpsertPinjamanItem)
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Delete("/pinjaman/items/{id}", h.DeletePinjamanItem)
		// Pemilih penempatan pada bank lain untuk pengisian sandi OJK (Form 05.00):
		// daftar id + label memakai system:config, sama seperti jalur tulisnya.
		r.With(middleware.RequirePermission(domain.PermSystemConfig)).
			Get("/placements", h.ListPlacements)
		r.With(middleware.RequirePermission(domain.PermReportsExport)).
			Get("/mapping", h.Mapping)
		r.With(middleware.RequirePermission(domain.PermCOAManage)).
			Post("/mapping/decision", h.DecideMapping)
	})
}

// Definitions mengembalikan definisi laporan, periodisitas/tenggat, daftar form
// bulanan, dan status pemetaan. Laporan yang belum dapat dibangun ikut didaftarkan.
func (h *OJKReportHandler) Definitions(w http.ResponseWriter, r *http.Request) {
	Success(w, http.StatusOK, i18n.MsgOJKReportDefinitions, map[string]any{
		"reports":         ojkreport.OJKReportDefinitions,
		"monthly_forms":   ojkreport.OJKBulananForms,
		"mapping_status":  ojkreport.MappingStatus,
		"ckpn_parameters": domain.CKPNParametersStatusFromConfig(r.Context(), h.config, time.Now()),
	})
}

// Mapping menyajikan pemetaan COA DRAF sebagai data untuk ditinjau bank: setiap baris
// memuat kode COA, nama akun, sandi dan nama pos OJK, status verifikasi draf, serta
// keputusan bank bila sudah ada. Disertakan pula kode COA pada laporan sumber yang belum
// terpetakan (penyebab ekspor ditolak) dan pos OJK yang belum punya sumber COA.
//
// Query period (YYYY-MM, default bulan lalu) dan book menentukan laporan sumber yang
// dipakai mendeteksi celah pemetaan; tanpa angka jurnal, celah tidak dapat dikarang.
func (h *OJKReportHandler) Mapping(w http.ResponseWriter, r *http.Request) {
	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	book := r.URL.Query().Get("book")

	entries := ojkreport.COAMappingDraft

	coaNames := make(map[string]string)
	if h.coa != nil {
		list, err := h.coa.GetCOAList(r.Context(), domain.Actor{Role: domain.RoleSystem})
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca bagan akun: %w", err))
			return
		}
		for _, c := range list {
			coaNames[c.Code] = c.Name
		}
	}

	var reviews []ojkreport.MappingReview
	if h.reviews != nil {
		reviews, err = h.reviews.ListReviews(r.Context())
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca keputusan pemetaan: %w", err))
			return
		}
	}

	// Celah dihitung dari laporan sumber periode ini, sama seperti ekspor. Bila sumber
	// tidak tersedia, celah dibiarkan kosong alih-alih menuduh pemetaan bolong.
	var unmappedCOA []ojkreport.UnmappedCOA
	unbalanced := false
	if h.source != nil {
		periodEnd := ojkreport.MonthEnd(period)
		yearStart := time.Date(period.Year(), time.January, 1, 0, 0, 0, 0, time.UTC)
		bs, err := h.source.GetBalanceSheet(r.Context(), periodEnd, book)
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca posisi keuangan: %w", err))
			return
		}
		is, err := h.source.GetIncomeStatement(r.Context(), yearStart, periodEnd, book)
		if err != nil {
			InternalError(w, r, fmt.Errorf("membaca laba rugi: %w", err))
			return
		}
		unmappedCOA = ojkreport.UnmappedCOACodes(bs, is, entries)
		unbalanced = ojkreport.HasImbalance(bs, is)
	}
	if unmappedCOA == nil {
		unmappedCOA = []ojkreport.UnmappedCOA{}
	}

	Success(w, http.StatusOK, i18n.MsgOJKCOAMapping, map[string]any{
		"mapping_status":     ojkreport.MappingStatus,
		"period":             period,
		"book":               book,
		"rows":               ojkreport.BuildMappingReviewRows(entries, coaNames, reviews),
		"unmapped_coa":       unmappedCOA,
		"unmapped_positions": ojkreport.UnmappedPositions(entries),
		"duplicate_coa":      ojkreport.DuplicateCOACodes(entries),
		"source_unbalanced":  unbalanced,
	})
}

// mappingDecisionRequest adalah keputusan bank atas satu baris pemetaan.
type mappingDecisionRequest struct {
	Form     string `json:"form"`
	COACode  string `json:"coa_code"`
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// DecideMapping menyimpan persetujuan bank atau catatan keberatannya atas satu baris
// pemetaan. Baris diidentifikasi (form, coa_code) dan hanya baris yang benar-benar ada
// pada pemetaan draf yang diterima. Keputusan disimpan idempoten di basis data beserta
// pelaku dan waktunya; status pemetaan global tidak berubah.
func (h *OJKReportHandler) DecideMapping(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	if h.reviews == nil {
		InternalError(w, r, errors.New("penyimpan keputusan pemetaan belum dikonfigurasi"))
		return
	}

	var req mappingDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		Fail(w, r, http.StatusBadRequest, errors.New("body JSON tidak valid"))
		return
	}
	req.Form = strings.TrimSpace(req.Form)
	req.COACode = strings.TrimSpace(req.COACode)
	decision := ojkreport.ReviewDecision(strings.TrimSpace(req.Decision))

	if !decision.Valid() {
		Fail(w, r, http.StatusBadRequest, errors.New("decision harus DISETUJUI atau DICATAT"))
		return
	}
	if !ojkreport.KnownMappingLine(req.Form, req.COACode) {
		Fail(w, r, http.StatusBadRequest, errors.New("baris pemetaan (form, coa_code) tidak dikenal"))
		return
	}
	note := strings.TrimSpace(req.Note)
	if decision == ojkreport.ReviewNoted && note == "" {
		Fail(w, r, http.StatusBadRequest, errors.New("catatan wajib diisi bila mencatat keberatan"))
		return
	}

	review := ojkreport.MappingReview{
		Form:      req.Form,
		COACode:   req.COACode,
		Decision:  decision,
		Note:      note,
		DecidedBy: claims.Username,
		DecidedAt: time.Now().UTC(),
	}
	if claims.UserID != uuid.Nil {
		review.DecidedByID = claims.UserID.String()
	}
	if err := h.reviews.UpsertReview(r.Context(), review); err != nil {
		InternalError(w, r, fmt.Errorf("menyimpan keputusan pemetaan: %w", err))
		return
	}

	Success(w, http.StatusOK, i18n.MsgOJKMappingDecisionSaved, review)
}

// ExportMonthly menulis berkas teks Laporan Bulanan BPR: form 01.00, 02.00, 00.08,
// dan — bila sumbernya tersedia — form daftar 00.00, 05.00, dan 06.00 untuk periode
// YYYY-MM. Query book opsional (mis. CONVENTIONAL/SYARIAH).
func (h *OJKReportHandler) ExportMonthly(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	// Laporan OJK bersifat bank-wide: membatasinya pada satu cabang menghasilkan
	// laporan yang salah, sementara memberikannya ke aktor yang hanya berwenang
	// atas cabangnya membuka data lintas cabang. Karena itu hanya aktor lintas
	// cabang (pengawas) yang boleh mengekspor.
	if !actor.IsCrossBranch() {
		Fail(w, r, http.StatusForbidden, fmt.Errorf(
			"%w: laporan OJK bersifat bank-wide dan hanya dapat diekspor peran lintas cabang",
			domain.ErrCrossBranchAccess))
		return
	}

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}

	// Selama parameter CKPN SEMENTARA, angka CKPN belum diratifikasi dan DILARANG
	// menjadi dasar laporan OJK/APOLO (butir 1.5 keputusan panel). Berkas bulanan
	// adalah kendaraan pengiriman, jadi bentuk yang dipilih adalah MENOLAK KERAS, bukan
	// menandai sebagian: berkas bertanda masih dapat terkirim dan angka sementaranya
	// ikut. Pesannya menyebut langkah perbaikan, bukan sekadar menolak.
	status := domain.CKPNParametersStatusFromConfig(r.Context(), h.config, time.Now())
	if status.OJKExportBlocked {
		Fail(w, r, http.StatusUnprocessableEntity, errors.New(status.OJKExportBlockReason))
		return
	}

	bundle, err := h.builder.GenerateMonthlyForActor(r.Context(), period, r.URL.Query().Get("book"), actor)
	if err != nil {
		if errors.Is(err, ojkreport.ErrOJKBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		if errors.Is(err, ojkreport.ErrIncompleteMapping) ||
			errors.Is(err, ojkreport.ErrInvalidMapping) ||
			errors.Is(err, ojkreport.ErrUnbalancedSource) {
			Fail(w, r, http.StatusUnprocessableEntity, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	filename := "ojk-bulanan-" + period.Format("2006-01") + ".txt"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	// Header sudah terkirim; kegagalan menulis hanya dapat dicatat ke log.
	if err := ojkreport.WriteText(w, bundle); err != nil {
		observability.FromContext(r.Context()).Error("gagal menulis berkas ekspor OJK", "error", err)
	}
}

// ExportBMPK menulis berkas teks Laporan BMPK (Batas Maksimum Pemberian Kredit) untuk
// periode YYYY-MM lewat ojkreport.GenerateBMPK + WriteText. Laporan bersifat bank-wide;
// aktor non-lintas cabang ditolak 403. Seperti ExportMonthly, ekspor DITUNDA selama
// parameter CKPN SEMENTARA karena berkas yang terkirim akan membawa angka sementara itu.
func (h *OJKReportHandler) ExportBMPK(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	if !actor.IsCrossBranch() {
		Fail(w, r, http.StatusForbidden, fmt.Errorf(
			"%w: laporan BMPK bersifat bank-wide dan hanya dapat diekspor peran lintas cabang",
			domain.ErrBMPKBankWide))
		return
	}

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}

	status := domain.CKPNParametersStatusFromConfig(r.Context(), h.config, time.Now())
	if status.OJKExportBlocked {
		Fail(w, r, http.StatusUnprocessableEntity, errors.New(status.OJKExportBlockReason))
		return
	}

	bundle, err := ojkreport.GenerateBMPK(r.Context(), h.bmpk, period, actor)
	if err != nil {
		if errors.Is(err, domain.ErrBMPKBankWide) || errors.Is(err, ojkreport.ErrOJKBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	filename := "ojk-bmpk-" + period.Format("2006-01") + ".txt"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	// Header sudah terkirim; kegagalan menulis hanya dapat dicatat ke log.
	if err := ojkreport.WriteText(w, bundle); err != nil {
		observability.FromContext(r.Context()).Error("gagal menulis berkas ekspor BMPK", "error", err)
	}
}

// BMPKMaster menyajikan data mentah pengaturan BMPK: seluruh pihak terkait dan batas
// yang bank isi. Izin system:config; tidak ada penegakan bank-wide di sini karena ini
// pengaturan, bukan laporan.
func (h *OJKReportHandler) BMPKMaster(w http.ResponseWriter, r *http.Request) {
	if h.bmpkAdmin == nil {
		InternalError(w, r, errors.New("modul BMPK belum dikonfigurasi"))
		return
	}
	master, err := h.bmpkAdmin.ListMaster(r.Context())
	if err != nil {
		InternalError(w, r, fmt.Errorf("membaca master BMPK: %w", err))
		return
	}
	Success(w, http.StatusOK, i18n.MsgBMPKMasterListed, map[string]any{
		"related_parties": master.RelatedParties,
		"limits":          master.Limits,
	})
}

// UpsertBMPKRelatedParty membuat/memperbarui satu penandaan pihak terkait
// (PUT /reports/ojk/bmpk/related-parties). Decoder menolak bidang tak dikenal agar
// payload di luar kontrak ditolak 422. Izin system:config; teraudit di service.
func (h *OJKReportHandler) UpsertBMPKRelatedParty(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateBMPKRelatedPartyInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.bmpkAdmin == nil {
		InternalError(w, r, errors.New("modul BMPK belum dikonfigurasi"))
		return
	}
	party, err := h.bmpkAdmin.UpsertRelatedParty(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeBMPKError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgBMPKRelatedPartySaved, party)
}

// DeleteBMPKRelatedParty menghapus penandaan satu nasabah menurut customerId.
func (h *OJKReportHandler) DeleteBMPKRelatedParty(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	customerID, err := uuid.Parse(chi.URLParam(r, "customerId"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgBMPKCustomerIDInvalid)
		return
	}
	if h.bmpkAdmin == nil {
		InternalError(w, r, errors.New("modul BMPK belum dikonfigurasi"))
		return
	}
	if err := h.bmpkAdmin.DeleteRelatedParty(r.Context(), customerID,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeBMPKError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgBMPKRelatedPartyDeleted, map[string]any{"customer_id": customerID.String()})
}

// UpsertBMPKLimit membuat/memperbarui satu batas per nasabah
// (PUT /reports/ojk/bmpk/limits). Izin system:config; teraudit di service.
func (h *OJKReportHandler) UpsertBMPKLimit(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateBMPKLimitInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.bmpkAdmin == nil {
		InternalError(w, r, errors.New("modul BMPK belum dikonfigurasi"))
		return
	}
	limit, err := h.bmpkAdmin.UpsertLimit(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeBMPKError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgBMPKLimitSaved, limit)
}

// DeleteBMPKLimit menghapus batas satu nasabah menurut customerId.
func (h *OJKReportHandler) DeleteBMPKLimit(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	customerID, err := uuid.Parse(chi.URLParam(r, "customerId"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgBMPKCustomerIDInvalid)
		return
	}
	if h.bmpkAdmin == nil {
		InternalError(w, r, errors.New("modul BMPK belum dikonfigurasi"))
		return
	}
	if err := h.bmpkAdmin.DeleteLimit(r.Context(), customerID,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeBMPKError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgBMPKLimitDeleted, map[string]any{"customer_id": customerID.String()})
}

// writeBMPKError memetakan galat pengaturan BMPK: baris/nasabah tak ada 404,
// pelanggaran bank-wide 403, sisanya 422 lewat Fail (validasi/pesan bisnis; galat
// berjejak internal otomatis disembunyikan Fail).
func writeBMPKError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrBMPKNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrBMPKBankWide):
		Fail(w, r, http.StatusForbidden, err)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

// ExportPerbedaanKualitas menulis berkas teks Laporan Perbedaan Kualitas Aset
// Produktif (Form 19.00) untuk periode YYYY-MM lewat
// ojkreport.GeneratePerbedaanKualitas + WriteText. Laporan bersifat bank-wide; aktor
// non-lintas cabang ditolak 403. Seperti ExportBMPK, ekspor DITUNDA selama parameter
// CKPN SEMENTARA karena berkas yang terkirim akan membawa angka laporan yang belum
// diratifikasi.
func (h *OJKReportHandler) ExportPerbedaanKualitas(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	if !actor.IsCrossBranch() {
		Fail(w, r, http.StatusForbidden, fmt.Errorf(
			"%w: laporan perbedaan kualitas aset produktif bersifat bank-wide dan hanya dapat diekspor peran lintas cabang",
			ojkreport.ErrOJKBankWide))
		return
	}

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}

	status := domain.CKPNParametersStatusFromConfig(r.Context(), h.config, time.Now())
	if status.OJKExportBlocked {
		Fail(w, r, http.StatusUnprocessableEntity, errors.New(status.OJKExportBlockReason))
		return
	}

	bundle, err := ojkreport.GeneratePerbedaanKualitas(r.Context(), h.loans, period, actor)
	if err != nil {
		if errors.Is(err, ojkreport.ErrOJKBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	filename := "ojk-perbedaan-kualitas-" + period.Format("2006-01") + ".txt"
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.WriteHeader(http.StatusOK)
	// Header sudah terkirim; kegagalan menulis hanya dapat dicatat ke log.
	if err := ojkreport.WriteText(w, bundle); err != nil {
		observability.FromContext(r.Context()).Error("gagal menulis berkas ekspor perbedaan kualitas", "error", err)
	}
}

// ExportKelembagaan menyajikan LAPORAN_KELEMBAGAAN sebagai JSON untuk periode
// YYYY-MM: data jaringan kantor, direksi/komisaris, dan pejabat eksekutif, beserta
// daftar bagian tabel dan kolom yang belum tersedia. Laporan bank-wide; aktor
// non-lintas cabang ditolak 403 oleh layanan.
func (h *OJKReportHandler) ExportKelembagaan(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	if h.kelembagaan == nil {
		InternalError(w, r, ojkreport.ErrKelembagaanSourceUnavailable)
		return
	}

	report, err := h.kelembagaan.KelembagaanReport(r.Context(), period, actor)
	if err != nil {
		if errors.Is(err, domain.ErrKelembagaanBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgKelembagaanReport, map[string]any{
		"report": report,
		"tables": ojkreport.BuildKelembagaanTables(report),
	})
}

// UpsertKelembagaanOffice membuat/memperbarui satu kantor (PUT /reports/ojk/kelembagaan/
// offices). Decoder menolak bidang tak dikenal agar payload di luar kontrak ditolak
// 422, bukan diam-diam diabaikan. Izin system:config; teraudit di service.
func (h *OJKReportHandler) UpsertKelembagaanOffice(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateBankOfficeInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.kelembagaan == nil {
		InternalError(w, r, ojkreport.ErrKelembagaanSourceUnavailable)
		return
	}
	office, err := h.kelembagaan.UpsertOffice(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeKelembagaanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKelembagaanSaved, office)
}

// DeleteKelembagaanOffice menghapus satu kantor menurut id.
func (h *OJKReportHandler) DeleteKelembagaanOffice(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgKelembagaanIDInvalid)
		return
	}
	if h.kelembagaan == nil {
		InternalError(w, r, ojkreport.ErrKelembagaanSourceUnavailable)
		return
	}
	if err := h.kelembagaan.DeleteOffice(r.Context(), id,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeKelembagaanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKelembagaanDeleted, map[string]any{"id": id.String()})
}

// UpsertKelembagaanManagement membuat/memperbarui satu direksi/komisaris/pejabat.
func (h *OJKReportHandler) UpsertKelembagaanManagement(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateBankManagementInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.kelembagaan == nil {
		InternalError(w, r, ojkreport.ErrKelembagaanSourceUnavailable)
		return
	}
	m, err := h.kelembagaan.UpsertManagement(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeKelembagaanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKelembagaanSaved, m)
}

// DeleteKelembagaanManagement menghapus satu direksi/komisaris/pejabat menurut id.
func (h *OJKReportHandler) DeleteKelembagaanManagement(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgKelembagaanIDInvalid)
		return
	}
	if h.kelembagaan == nil {
		InternalError(w, r, ojkreport.ErrKelembagaanSourceUnavailable)
		return
	}
	if err := h.kelembagaan.DeleteManagement(r.Context(), id,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeKelembagaanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKelembagaanDeleted, map[string]any{"id": id.String()})
}

// ExportOffBalance menyajikan register rekening administratif (Form 01.01) sebagai
// JSON untuk periode YYYY-MM: daftar pos komitmen/kontinjensi off-balance beserta
// tabel agregat yang dipakai bundel bulanan dan kolom yang belum tersedia. Register
// bank-wide; aktor non-lintas cabang ditolak 403 oleh layanan.
func (h *OJKReportHandler) ExportOffBalance(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	if h.offBalance == nil {
		InternalError(w, r, errors.New("sumber register rekening administratif belum dikonfigurasi"))
		return
	}

	report, err := h.offBalance.OffBalanceReport(r.Context(), period, actor)
	if err != nil {
		if errors.Is(err, domain.ErrOffBalanceBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	// Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bila sumbernya
	// tersedia (RepoSource); tanpa itu tetap dinyatakan tidak tersedia, bukan "-"
	// tanpa alasan.
	kantor := ojkreport.ReportingOffice{Reason: "register rekening administratif dicatat bank-wide (tidak menyimpan kantor); kolom Sandi Kantor belum punya sumber dan tidak dikarang"}
	if ros, ok := h.source.(ojkreport.ReportingOfficeSource); ok {
		k, err := ros.ReportingOffice(r.Context())
		if err != nil {
			InternalError(w, r, err)
			return
		}
		kantor = k
	}

	Success(w, http.StatusOK, i18n.MsgOffBalanceReport, map[string]any{
		"report": report,
		"tables": []ojkreport.TableSection{ojkreport.BuildForm01_01(report.Aggregates, kantor)},
	})
}

// ListOffBalanceItems menyajikan baris mentah register rekening administratif untuk
// UI edit (GET /reports/ojk/off-balance/items). Urutan deterministik dari repositori.
// Izin system:config; tidak ada penegakan bank-wide karena ini pengaturan.
func (h *OJKReportHandler) ListOffBalanceItems(w http.ResponseWriter, r *http.Request) {
	if h.offBalance == nil {
		InternalError(w, r, errors.New("sumber register rekening administratif belum dikonfigurasi"))
		return
	}
	items, err := h.offBalance.ListItems(r.Context())
	if err != nil {
		InternalError(w, r, fmt.Errorf("membaca register rekening administratif: %w", err))
		return
	}
	Success(w, http.StatusOK, i18n.MsgOffBalanceItemsListed, map[string]any{"items": items})
}

// ListPlacements menyajikan pemilih penempatan pada bank lain untuk pengisian sandi OJK
// (GET /reports/ojk/placements): id + label terbaca. Penyaringan cakupan cabang di
// repositori memakai aktor dari klaim; izin system:config.
func (h *OJKReportHandler) ListPlacements(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	if h.placements == nil {
		InternalError(w, r, errors.New("sumber penempatan belum dikonfigurasi"))
		return
	}
	items, err := h.placements.ListPlacements(r.Context(), time.Now().UTC(),
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		InternalError(w, r, fmt.Errorf("membaca penempatan pada bank lain: %w", err))
		return
	}
	options := make([]domain.OJKPlacementOption, 0, len(items))
	for _, p := range items {
		options = append(options, domain.OJKPlacementOption{ID: p.ID, Label: domain.LPSPlacementLabel(p)})
	}
	Success(w, http.StatusOK, i18n.MsgOJKPlacementsListed, map[string]any{"placements": options})
}

// UpsertOffBalanceItem membuat/memperbarui satu pos rekening administratif
// (PUT /reports/ojk/off-balance/items). Decoder menolak bidang tak dikenal agar
// payload di luar kontrak ditolak 422, bukan diam-diam diabaikan. Izin system:config;
// teraudit di service.
func (h *OJKReportHandler) UpsertOffBalanceItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateOffBalanceItemInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.offBalance == nil {
		InternalError(w, r, errors.New("sumber register rekening administratif belum dikonfigurasi"))
		return
	}
	item, err := h.offBalance.UpsertItem(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeOffBalanceError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOffBalanceSaved, item)
}

// DeleteOffBalanceItem menghapus satu pos rekening administratif menurut id.
func (h *OJKReportHandler) DeleteOffBalanceItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgOffBalanceIDInvalid)
		return
	}
	if h.offBalance == nil {
		InternalError(w, r, errors.New("sumber register rekening administratif belum dikonfigurasi"))
		return
	}
	if err := h.offBalance.DeleteItem(r.Context(), id,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeOffBalanceError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgOffBalanceDeleted, map[string]any{"id": id.String()})
}

// writeOffBalanceError memetakan galat rekening administratif: baris tak ada 404,
// pelanggaran bank-wide 403, sisanya 422 lewat Fail (validasi/pesan bisnis; galat
// berjejak internal otomatis disembunyikan Fail).
func writeOffBalanceError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrOffBalanceNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrOffBalanceBankWide):
		Fail(w, r, http.StatusForbidden, err)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

// ExportAYDA menyajikan register AYDA (Form 07.00) sebagai JSON untuk periode
// YYYY-MM: daftar agunan yang diambil alih beserta tabel Form 07.00 dan kolom yang
// belum tersedia. Register bank-wide; aktor non-lintas cabang ditolak 403 oleh layanan.
func (h *OJKReportHandler) ExportAYDA(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	if h.ayda == nil {
		InternalError(w, r, errors.New("sumber register AYDA belum dikonfigurasi"))
		return
	}

	report, err := h.ayda.AYDAReport(r.Context(), period, actor)
	if err != nil {
		if errors.Is(err, domain.ErrAYDABankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	// Kolom I Sandi Kantor diambil dari kantor pelapor tunggal bila sumbernya tersedia
	// (RepoSource); tanpa itu tetap dinyatakan tidak tersedia, bukan "-" tanpa alasan.
	kantor := ojkreport.ReportingOffice{Reason: "register AYDA dicatat bank-wide (tidak menyimpan kantor); kolom Sandi Kantor belum punya sumber dan tidak dikarang"}
	if ros, ok := h.source.(ojkreport.ReportingOfficeSource); ok {
		k, err := ros.ReportingOffice(r.Context())
		if err != nil {
			InternalError(w, r, err)
			return
		}
		kantor = k
	}

	Success(w, http.StatusOK, i18n.MsgAYDAReport, map[string]any{
		"report": report,
		"tables": []ojkreport.TableSection{ojkreport.BuildForm07(report.Items, kantor)},
	})
}

// ListAYDAItems menyajikan baris mentah register AYDA untuk UI edit
// (GET /reports/ojk/ayda/items). Urutan deterministik dari repositori. Izin
// system:config; tidak ada penegakan bank-wide karena ini pengaturan.
func (h *OJKReportHandler) ListAYDAItems(w http.ResponseWriter, r *http.Request) {
	if h.ayda == nil {
		InternalError(w, r, errors.New("sumber register AYDA belum dikonfigurasi"))
		return
	}
	items, err := h.ayda.ListItems(r.Context())
	if err != nil {
		InternalError(w, r, fmt.Errorf("membaca register AYDA: %w", err))
		return
	}
	Success(w, http.StatusOK, i18n.MsgAYDAItemsListed, map[string]any{"items": items})
}

// UpsertAYDAItem membuat/memperbarui satu AYDA (PUT /reports/ojk/ayda/items). Decoder
// menolak bidang tak dikenal agar payload di luar kontrak ditolak 422, bukan diam-diam
// diabaikan. Izin system:config; teraudit di service.
func (h *OJKReportHandler) UpsertAYDAItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateAYDAItemInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.ayda == nil {
		InternalError(w, r, errors.New("sumber register AYDA belum dikonfigurasi"))
		return
	}
	item, err := h.ayda.UpsertItem(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeAYDAError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgAYDASaved, item)
}

// DeleteAYDAItem menghapus satu AYDA menurut id.
func (h *OJKReportHandler) DeleteAYDAItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgAYDAIDInvalid)
		return
	}
	if h.ayda == nil {
		InternalError(w, r, errors.New("sumber register AYDA belum dikonfigurasi"))
		return
	}
	if err := h.ayda.DeleteItem(r.Context(), id,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeAYDAError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgAYDADeleted, map[string]any{"id": id.String()})
}

// writeAYDAError memetakan galat register AYDA: baris tak ada 404, pelanggaran
// bank-wide 403, sisanya 422 lewat Fail (validasi/pesan bisnis; galat berjejak internal
// otomatis disembunyikan Fail).
func writeAYDAError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrAYDANotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrAYDABankWide):
		Fail(w, r, http.StatusForbidden, err)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

// ExportKepemilikan menyajikan register pemegang saham (Form 00.01) sebagai JSON
// untuk periode YYYY-MM: daftar pemegang saham beserta tabel Form 00.01 dan kolom
// yang belum tersedia. Register bank-wide; aktor non-lintas cabang ditolak 403 oleh
// layanan. Kolom IV No. Identitas tidak disimpan (keputusan privasi) dan selalu
// ditulis "-" beralasan.
func (h *OJKReportHandler) ExportKepemilikan(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	if h.kepemilikan == nil {
		InternalError(w, r, errors.New("sumber register kepemilikan BPR belum dikonfigurasi"))
		return
	}

	report, err := h.kepemilikan.KepemilikanReport(r.Context(), period, actor)
	if err != nil {
		if errors.Is(err, domain.ErrKepemilikanBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgKepemilikanReport, map[string]any{
		"report": report,
		"tables": []ojkreport.TableSection{ojkreport.BuildForm00_01(report.Items)},
	})
}

// ListKepemilikanItems menyajikan baris mentah register pemegang saham untuk UI edit
// (GET /reports/ojk/kepemilikan/items). Urutan deterministik dari repositori. Izin
// system:config; tidak ada penegakan bank-wide karena ini pengaturan.
func (h *OJKReportHandler) ListKepemilikanItems(w http.ResponseWriter, r *http.Request) {
	if h.kepemilikan == nil {
		InternalError(w, r, errors.New("sumber register kepemilikan BPR belum dikonfigurasi"))
		return
	}
	items, err := h.kepemilikan.ListItems(r.Context())
	if err != nil {
		InternalError(w, r, fmt.Errorf("membaca register kepemilikan BPR: %w", err))
		return
	}
	Success(w, http.StatusOK, i18n.MsgKepemilikanItemsListed, map[string]any{"items": items})
}

// UpsertKepemilikanItem membuat/memperbarui satu pemegang saham
// (PUT /reports/ojk/kepemilikan/items). Decoder menolak bidang tak dikenal agar
// payload di luar kontrak ditolak 422, bukan diam-diam diabaikan; kolom IV No.
// Identitas tidak ada di kontrak karena tidak disimpan. Izin system:config; teraudit
// di service.
func (h *OJKReportHandler) UpsertKepemilikanItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdateKepemilikanItemInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.kepemilikan == nil {
		InternalError(w, r, errors.New("sumber register kepemilikan BPR belum dikonfigurasi"))
		return
	}
	item, err := h.kepemilikan.UpsertItem(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writeKepemilikanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKepemilikanSaved, item)
}

// DeleteKepemilikanItem menghapus satu pemegang saham menurut id.
func (h *OJKReportHandler) DeleteKepemilikanItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgKepemilikanIDInvalid)
		return
	}
	if h.kepemilikan == nil {
		InternalError(w, r, errors.New("sumber register kepemilikan BPR belum dikonfigurasi"))
		return
	}
	if err := h.kepemilikan.DeleteItem(r.Context(), id,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writeKepemilikanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgKepemilikanDeleted, map[string]any{"id": id.String()})
}

// writeKepemilikanError memetakan galat register pemegang saham: baris tak ada 404,
// pelanggaran bank-wide 403, sisanya 422 lewat Fail (validasi/pesan bisnis; galat
// berjejak internal otomatis disembunyikan Fail).
func writeKepemilikanError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrKepemilikanNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrKepemilikanBankWide):
		Fail(w, r, http.StatusForbidden, err)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

// ExportPinjaman menyajikan register pinjaman yang diterima (Form 00.07) sebagai JSON
// untuk periode YYYY-MM: daftar pinjaman dari bank/Bank Indonesia/pihak ketiga bukan
// bank beserta tabel Form 00.07. Register bank-wide; aktor non-lintas cabang ditolak
// 403 oleh layanan. Form ini tidak punya kolom Sandi Kantor. Kolom XV Baki Debet Neto
// dihitung dari XII - (XIII + XIV).
func (h *OJKReportHandler) ExportPinjaman(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	actor := claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))

	period, err := parseOJKPeriod(strings.TrimSpace(r.URL.Query().Get("period")))
	if err != nil {
		Fail(w, r, http.StatusBadRequest, err)
		return
	}
	if h.pinjaman == nil {
		InternalError(w, r, errors.New("sumber register pinjaman yang diterima belum dikonfigurasi"))
		return
	}

	report, err := h.pinjaman.PinjamanReport(r.Context(), period, actor)
	if err != nil {
		if errors.Is(err, domain.ErrPinjamanBankWide) {
			Fail(w, r, http.StatusForbidden, err)
			return
		}
		InternalError(w, r, err)
		return
	}

	Success(w, http.StatusOK, i18n.MsgPinjamanReport, map[string]any{
		"report": report,
		"tables": []ojkreport.TableSection{ojkreport.BuildForm00_07(report.Items)},
	})
}

// ListPinjamanItems menyajikan baris mentah register pinjaman yang diterima untuk UI
// edit (GET /reports/ojk/pinjaman/items). Urutan deterministik dari repositori. Izin
// system:config; tidak ada penegakan bank-wide karena ini pengaturan.
func (h *OJKReportHandler) ListPinjamanItems(w http.ResponseWriter, r *http.Request) {
	if h.pinjaman == nil {
		InternalError(w, r, errors.New("sumber register pinjaman yang diterima belum dikonfigurasi"))
		return
	}
	items, err := h.pinjaman.ListItems(r.Context())
	if err != nil {
		InternalError(w, r, fmt.Errorf("membaca register pinjaman yang diterima: %w", err))
		return
	}
	Success(w, http.StatusOK, i18n.MsgPinjamanItemsListed, map[string]any{"items": items})
}

// UpsertPinjamanItem membuat/memperbarui satu pinjaman (PUT /reports/ojk/pinjaman/items).
// Decoder menolak bidang tak dikenal agar payload di luar kontrak ditolak 422, bukan
// diam-diam diabaikan. Izin system:config; teraudit di service.
func (h *OJKReportHandler) UpsertPinjamanItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	var input domain.UpdatePinjamanItemInput
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&input); err != nil {
		ErrorCodef(w, http.StatusUnprocessableEntity, i18n.MsgInvalidRequestBodyWithErr, err.Error())
		return
	}
	if h.pinjaman == nil {
		InternalError(w, r, errors.New("sumber register pinjaman yang diterima belum dikonfigurasi"))
		return
	}
	item, err := h.pinjaman.UpsertItem(r.Context(), input,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context())))
	if err != nil {
		writePinjamanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgPinjamanSaved, item)
}

// DeletePinjamanItem menghapus satu pinjaman menurut id.
func (h *OJKReportHandler) DeletePinjamanItem(w http.ResponseWriter, r *http.Request) {
	claims, ok := domain.ClaimsFromContext(r.Context())
	if !ok {
		ErrorCode(w, http.StatusUnauthorized, i18n.MsgAuthenticationRequired)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		ErrorCode(w, http.StatusBadRequest, i18n.MsgPinjamanIDInvalid)
		return
	}
	if h.pinjaman == nil {
		InternalError(w, r, errors.New("sumber register pinjaman yang diterima belum dikonfigurasi"))
		return
	}
	if err := h.pinjaman.DeleteItem(r.Context(), id,
		claims.ToActor(r.RemoteAddr, observability.RequestIDFromContext(r.Context()))); err != nil {
		writePinjamanError(w, r, err)
		return
	}
	Success(w, http.StatusOK, i18n.MsgPinjamanDeleted, map[string]any{"id": id.String()})
}

// writePinjamanError memetakan galat register pinjaman: baris tak ada 404, pelanggaran
// bank-wide 403, sisanya 422 lewat Fail (validasi/pesan bisnis; galat berjejak internal
// otomatis disembunyikan Fail).
func writePinjamanError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrPinjamanNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrPinjamanBankWide):
		Fail(w, r, http.StatusForbidden, err)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

// writeKelembagaanError memetakan galat kelembagaan: baris tak ada 404, pelanggaran
// bank-wide 403, sisanya 422 lewat Fail (validasi/pesan bisnis; galat berjejak internal
// otomatis disembunyikan Fail).
func writeKelembagaanError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, domain.ErrKelembagaanNotFound):
		Fail(w, r, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrKelembagaanBankWide):
		Fail(w, r, http.StatusForbidden, err)
	default:
		Fail(w, r, http.StatusUnprocessableEntity, err)
	}
}

// parseOJKPeriod membaca periode YYYY-MM atau YYYY-MM-DD. Bila kosong, dipakai
// bulan sebelumnya karena laporan bulanan disampaikan pada bulan berikutnya.
func parseOJKPeriod(raw string) (time.Time, error) {
	if raw == "" {
		prev := time.Now().UTC().AddDate(0, -1, 0)
		return time.Date(prev.Year(), prev.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}
	for _, layout := range []string{"2006-01", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, errors.New("format period tidak valid: gunakan YYYY-MM")
}
