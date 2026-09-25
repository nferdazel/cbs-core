package http

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// allowedOrigins membaca CORS_ALLOWED_ORIGINS (dipisah koma). Default pengembangan
// hanya localhost, tidak ada wildcard: go-chi/cors mencocokkan origin secara literal,
// sehingga "https://*.domain" tidak pernah cocok dan hanya menyesatkan.
func allowedOrigins() []string {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = "http://localhost:3000,http://localhost:3001"
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" && p != "*" {
			// Wildcard ditolak: dengan AllowCredentials=true, origin harus eksplisit
			// agar browser tidak mengirim cookie ke origin sembarang.
			origins = append(origins, p)
		}
	}
	return origins
}

type RouterParams struct {
	CustomerHandler     *CustomerHandler
	AccountHandler      *AccountHandler
	BranchHandler       *BranchHandler
	ProductHandler      *ProductHandler
	LedgerHandler       *LedgerHandler
	AuthHandler         *AuthHandler
	StaffHandler        *StaffHandler
	LoanHandler         *LoanHandler
	MakerCheckerHandler *MakerCheckerHandler
	ReportHandler       *ReportHandler
	CollectionHandler   *CollectionHandler
	IntegrationHandler  *IntegrationHandler
	OJKReportHandler    *OJKReportHandler
	// KPMMHandler menyajikan laporan KPMM/ATMR BPR (baca saja, bank-wide).
	KPMMHandler         *KPMMHandler
	BatchProcessHandler *BatchProcessHandler
	// EODDefinitionHandler mengelola definisi urutan langkah EOD dan riwayatnya.
	EODDefinitionHandler *EODDefinitionHandler
	DocumentHandler      *DocumentHandler
	DepositHandler       *DepositHandler
	PPAPHandler          *PPAPHandler
	CKPNHandler          *CKPNHandler
	LPSPlacementHandler  *LPSPlacementHandler
	AuditHandler         *AuditHandler
	CollateralHandler    *CollateralHandler
	// CollateralWeightHandler menyajikan gerbang aktivasi bobot agunan (baca-saja +
	// aktivasi). Bobot tetap mati bawaan; rutenya tidak dipasang bila nil.
	CollateralWeightHandler *CollateralWeightHandler
	// AppInfoHandler melayani identitas aplikasi publik untuk halaman login/web.
	AppInfoHandler *AppInfoHandler
	// BankProfileHandler mengelola identitas bank tingkat instalasi (baca & ubah).
	BankProfileHandler *BankProfileHandler
	// OJKProfileHandler mengelola identitas Form 00.00 yang disimpan sebagai kunci
	// system_config ojk.* (baca & ubah), melengkapi BankProfileHandler.
	OJKProfileHandler *OJKProfileHandler
	// CKPNActivationHandler mengelola pengaturan aktivasi CKPN (parameter PD/LGD, akun
	// syariah, status SEMENTARA/FINAL, bukti ratifikasi, saklar ckpn.enabled) agar bank
	// mengisinya tanpa SQL. Menyalakan CKPN tetap keputusan manusia.
	CKPNActivationHandler *CKPNActivationHandler
	// PermissionHandler melayani katalog grup/izin/menu dan pengajuan perubahan
	// pemetaan izin (lewat maker-checker, teraudit).
	PermissionHandler *PermissionHandler
	// MonitoringHandler menyajikan temuan kesehatan operasional (tanggal bisnis,
	// run EOD, PPAP, dan parameter CKPN). Baca-saja, rutenya tidak dipasang bila nil.
	MonitoringHandler *MonitoringHandler
	AuthService       domain.AuthService
	// ConfigService membaca cakupan buku tingkat instalasi (institution.book_scope)
	// yang diisi ke klaim setiap permintaan oleh BookScopeMiddleware. Bila nil,
	// seluruh aktor berperilaku DUAL seperti sebelum setelan ini ada.
	ConfigService domain.SystemConfigService
	// BranchScopeResolver meresolusi cakupan unit organisasi (cabang/area/wilayah)
	// ke klaim setiap permintaan lewat BranchScopeMiddleware. Bila nil, aktor
	// berperilaku seperti sebelum hierarki organisasi ada (hanya kode cabangnya).
	BranchScopeResolver domain.BranchScopeResolver
	// Cookies menentukan nama/atribut cookie sesi & CSRF.
	Cookies middleware.CookieConfig
	// Logger dipakai untuk access log dan panic recovery. Bila nil, logger default.
	Logger *slog.Logger
	// LoginRateLimiter membatasi percobaan login per akun dan per IP. Bila nil,
	// rute login dibiarkan tanpa pembatasan (mis. pada test).
	LoginRateLimiter *middleware.LoginRateLimiter
}

func NewRouter(p RouterParams) *chi.Mux {
	r := chi.NewRouter()

	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Middleware global. RequestID milik kita menangani korelasi; Recoverer dan
	// AccessLog memakai logger terstruktur agar data sensitif tersaring.
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestID)
	// ProxyIP menggantikan chiMiddleware.RealIP: RealIP memakai entri X-Forwarded-For
	// paling kiri yang dapat dipalsukan klien, sedangkan jejak audit dan pembatasan
	// login harus memakai alamat yang dicatat proxy tepercaya.
	r.Use(middleware.ProxyIP)
	r.Use(middleware.LimitBodySize(middleware.MaxBodyBytes))
	r.Use(middleware.AccessLog(logger))
	r.Use(middleware.Recoverer(logger))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key", "X-Request-ID"},
		ExposedHeaders:   []string{"Link", "Idempotency-Key", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check — public, no auth
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		Success(w, http.StatusOK, i18n.MsgHealthOK, map[string]string{"status": "UP"})
	})

	r.Route("/api/v1", func(r chi.Router) {

		// ── Public: identitas aplikasi (no JWT required) ──
		// Dipakai halaman login dan metadata/judul tab web. Sengaja di luar grup
		// ber-AuthMiddleware/BookScopeMiddleware: cakupan buku mengatur DATA, bukan
		// identitas instalasi, sehingga endpoint ini tetap sama pada cakupan apa pun.
		if p.AppInfoHandler != nil {
			r.Get("/app-info", p.AppInfoHandler.Get)
		}

		// ── Public: Auth endpoints (no JWT required) ──
		r.Route("/auth", func(r chi.Router) {
			// Login & refresh tidak memakai CSRF: keduanya pintu masuk sesi,
			// belum ada sesi terautentikasi yang bisa disalahgunakan. Login tetap
			// dibatasi percobaannya untuk menahan brute force.
			if p.LoginRateLimiter != nil {
				r.With(p.LoginRateLimiter.Middleware).Post("/login", p.AuthHandler.Login)
			} else {
				r.Post("/login", p.AuthHandler.Login)
			}
			r.Post("/refresh", p.AuthHandler.Refresh)

			// Logout di luar AuthMiddleware agar cookie tetap terhapus walau
			// access token kedaluwarsa, tetapi tetap wajib lolos CSRF.
			r.With(middleware.CSRFMiddleware(p.Cookies)).
				Post("/logout", p.AuthHandler.Logout)

			// Protected auth routes (require valid token)
			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(p.AuthService, p.Cookies))
				// Cakupan instalasi ikut dibaca agar /auth/me dapat memberi tahu web
				// lini usaha mana yang aktif, tanpa web menebak.
				r.Use(middleware.BookScopeMiddleware(p.ConfigService))
				r.Use(middleware.CSRFMiddleware(p.Cookies))
				r.Get("/me", p.AuthHandler.Me)
			})
		})

		// ── All routes below require authentication ──
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(p.AuthService, p.Cookies))
			// Setelah identitas terverifikasi, cakupan buku instalasi diisi dari
			// konfigurasi sebelum handler membangun Actor. Semua endpoint di bawah
			// ini (baca maupun tulis) mewarisi batas lini usaha tersebut.
			r.Use(middleware.BookScopeMiddleware(p.ConfigService))
			// Cakupan unit organisasi (cabang + turunan area/wilayah) diisi dari
			// tabel branches sebelum handler membangun Actor, sehingga baca dan
			// tulis memakai sumbu cakupan unit yang sama. Bank tanpa area/wilayah
			// menghasilkan himpunan satu kode, identik dengan perilaku lama.
			r.Use(middleware.BranchScopeMiddleware(p.BranchScopeResolver))
			// Token berpenanda kata sandi kedaluwarsa hanya boleh mengganti kata
			// sandi. Didaftarkan di sini (setelah AuthMiddleware) agar rute /auth/me
			// dan logout tetap bebas, sedangkan seluruh rute bisnis lain ditolak 403.
			r.Use(middleware.RequirePasswordChange)
			r.Use(middleware.CSRFMiddleware(p.Cookies))

			// ── Staff Management (Admin & SuperAdmin only) ──
			r.Route("/staff", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermUsersRead)).
					Get("/", p.StaffHandler.List)
				r.With(middleware.RequirePermission(domain.PermUsersCreate)).
					Post("/", p.StaffHandler.Create)
				r.With(middleware.RequirePermission(domain.PermUsersRead)).
					Get("/{id}", p.StaffHandler.GetByID)
				r.With(middleware.RequirePermission(domain.PermUsersUpdate)).
					Put("/{id}", p.StaffHandler.Update)
				r.With(middleware.RequirePermission(domain.PermUsersUpdate)).
					Post("/{id}/reset-password", p.StaffHandler.ResetPassword)

				// Any authenticated user can change their own password
				r.Post("/me/change-password", p.StaffHandler.ChangePassword)
			})

			// ── Customer / CIF ──
			r.Route("/customers", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermCustomersCreate)).
					Post("/", p.CustomerHandler.Register)
				r.With(middleware.RequirePermission(domain.PermCustomersRead)).
					Get("/", p.CustomerHandler.List)
				r.With(middleware.RequirePermission(domain.PermCustomersRead)).
					Get("/{id}", p.CustomerHandler.GetByID)
				// Perubahan data nasabah memakai izin customers:update yang sudah ada
				// di matriks peran; rute ini menutup izin yang sebelumnya tanpa jalur.
				r.With(middleware.RequirePermission(domain.PermCustomersUpdate)).
					Put("/{id}", p.CustomerHandler.Update)
			})

			// ── Master data: cabang & produk ──
			r.Route("/branches", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermUsersRead)).
					Get("/", p.BranchHandler.List)
				r.With(middleware.RequirePermission(domain.PermBranchesCreate)).
					Post("/", p.BranchHandler.Create)
				// Susunan hierarki organisasi (W14): baca unit semua jenjang dengan
				// users:read, ubah dengan branches:create (fungsi administratif yang
				// sama seperti pembuatan cabang).
				r.With(middleware.RequirePermission(domain.PermUsersRead)).
					Get("/org-units", p.BranchHandler.ListOrgUnits)
				r.With(middleware.RequirePermission(domain.PermBranchesCreate)).
					Post("/org-units", p.BranchHandler.CreateOrgUnit)
				r.With(middleware.RequirePermission(domain.PermBranchesCreate)).
					Put("/org-units/{code}/parent", p.BranchHandler.SetOrgUnitParent)
			})
			r.Route("/products", func(r chi.Router) {
				// Data referensi produk dipakai layar rekening, deposito, dan kredit.
				// Sebelumnya dijaga loans:read sehingga TELLER tidak dapat memuat daftar
				// produk, padahal ia berwenang membuka rekening.
				r.With(middleware.RequirePermission(domain.PermProductsRead)).
					Get("/", p.ProductHandler.List)
				r.With(middleware.RequirePermission(domain.PermProductsRead)).
					Get("/{id}", p.ProductHandler.GetByID)
				// Perubahan parameter produk (suku bunga, nisbah, proyeksi pendapatan,
				// batas plafon/tenor, biaya) adalah fungsi administratif. Dijaga izin
				// pengelolaan data master yang hanya dipegang ADMIN dan SUPERADMIN;
				// identitas produk tidak diubah lewat rute ini.
				r.With(middleware.RequirePermission(domain.PermCOAManage)).
					Put("/{code}", p.ProductHandler.UpdateParams)
			})

			// ── Accounts ──
			r.Route("/accounts", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermAccountsOpen)).
					Post("/open", p.AccountHandler.Open)
				r.With(middleware.RequirePermission(domain.PermAccountsRead)).
					Get("/", p.AccountHandler.List)
				r.With(middleware.RequirePermission(domain.PermAccountsRead)).
					Get("/{accountNumber}", p.AccountHandler.GetByNumber)
				// Kewenangan membekukan (PermAccountsFreeze) sudah mencakup kewenangan
				// memulihkan; tidak perlu permission baru untuk reaktivasi.
				r.With(middleware.RequirePermission(domain.PermAccountsFreeze)).
					Post("/{accountNumber}/reactivate", p.AccountHandler.Reactivate)
				// Pembekuan rekening (keputusan panel: ADMIN + SUPERVISOR). Hanya
				// rekening ACTIVE yang boleh dibekukan; pelaksana dicatat agar
				// unfreeze oleh orang yang sama ditolak.
				r.With(middleware.RequirePermission(domain.PermAccountsFreeze)).
					Post("/{accountNumber}/freeze", p.AccountHandler.Freeze)
				// Unfreeze adalah kebalikan operasi pembekuan (FROZEN -> ACTIVE).
				// Izin yang sama (accounts:freeze) sudah cocok: dengan begitu peran
				// yang boleh membekukan tetap boleh membatalkannya, tidak ada yang
				// kehilangan akses; yang dilarang adalah ORANG yang sama melakukannya
				// dua kali (lihat UnfreezeAccount).
				r.With(middleware.RequirePermission(domain.PermAccountsFreeze)).
					Post("/{accountNumber}/unfreeze", p.AccountHandler.Unfreeze)
				// Penutupan rekening adalah wewenang tersendiri (accounts:close). Izin
				// sudah ada di matriks peran; rute ini menutup izin mati tersebut.
				r.With(middleware.RequirePermission(domain.PermAccountsClose)).
					Post("/{accountNumber}/close", p.AccountHandler.Close)
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/{accountNumber}/statements", p.LedgerHandler.GetStatement)
			})

			// ── Transactions / Ledger ──
			r.Route("/transactions", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermTransactionsDeposit)).
					Post("/deposit", p.LedgerHandler.Deposit)
				r.With(middleware.RequirePermission(domain.PermTransactionsWithdraw)).
					Post("/withdraw", p.LedgerHandler.Withdraw)
				r.With(middleware.RequirePermission(domain.PermTransactionsTransfer)).
					Post("/transfer", p.LedgerHandler.Transfer)
				// Pembatalan transaksi: hanya peran pengawas (Supervisor ke atas), dan
				// pencatat transaksi asal tidak boleh membatalkannya sendiri.
				r.With(middleware.RequirePermission(domain.PermTransactionsReverse)).
					Post("/{reference}/reverse", p.LedgerHandler.Reverse)
				// Jurnal majemuk manual menyentuh buku besar di luar alur domain,
				// jadi dijaga izin pengelolaan data master akuntansi (coa:manage,
				// ADMIN/SUPERADMIN) — sejajar dengan perubahan bagan akun dan
				// parameter produk.
				r.With(middleware.RequirePermission(domain.PermCOAManage)).
					Post("/journals", p.LedgerHandler.PostCompoundJournal)
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/journals", p.LedgerHandler.ListJournals)
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/journals/{reference}", p.LedgerHandler.GetJournalByRef)
			})

			// ── Loans & Financing (Kredit BPR / Pembiayaan BMT) ──
			r.Route("/loans", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermLoansApply)).
					Post("/apply", p.LoanHandler.Apply)
				r.With(middleware.RequirePermission(domain.PermLoansRead)).
					Get("/", p.LoanHandler.List)
				r.With(middleware.RequirePermission(domain.PermLoansRead)).
					Get("/{id}", p.LoanHandler.GetByID)
				r.With(middleware.RequirePermission(domain.PermLoansApprove)).
					Post("/{id}/approve", p.LoanHandler.Approve)
				r.With(middleware.RequirePermission(domain.PermLoansApprove)).
					Post("/{id}/reject", p.LoanHandler.Reject)
				r.With(middleware.RequirePermission(domain.PermLoansApprove)).
					Post("/{id}/disburse", p.LoanHandler.Disburse)
				// Pembatalan pencairan lebih sensitif daripada pencairan: hanya peran
				// tertinggi (Superadmin/Admin) yang memegang loans:cancel.
				r.With(middleware.RequirePermission(domain.PermLoansCancel)).
					Post("/{id}/cancel-disbursement", p.LoanHandler.CancelDisbursement)
				// Koreksi nominal mengubah tagihan yang sudah berjalan: sama sensitifnya
				// dengan pembatalan, jadi hanya peran tertinggi (loans:correct).
				r.With(middleware.RequirePermission(domain.PermLoansCorrect)).
					Post("/{id}/correct-amount", p.LoanHandler.CorrectAmount)
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Post("/{id}/pay-installment", p.LoanHandler.PayInstallment)
				r.With(middleware.RequirePermission(domain.PermLoansApprove)).
					Post("/{id}/restructure", p.LoanHandler.Restructure)
				r.With(middleware.RequirePermission(domain.PermLoansWriteOff)).
					Post("/{id}/write-off", p.LoanHandler.WriteOff)
				r.With(middleware.RequirePermission(domain.PermLoansRecover)).
					Post("/{id}/recover", p.LoanHandler.Recover)
			})

			// ── Mobile Field Collections (Jemput Bola) ──
			r.Route("/collections", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Post("/mobile-collect", p.CollectionHandler.ProcessMobileCollection)
			})

			// ── Financial Statement Reports (dihitung dari jurnal) ──
			// Keempat laporan keuangan bersifat bank-wide, jadi dijaga izin
			// tersendiri (reports:financial:read) yang tidak dipegang peran cabang.
			// ledger:read tetap cukup untuk membaca mutasi/statement rekening.
			r.Route("/reports", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermReportsFinancialRead)).
					Get("/trial-balance", p.ReportHandler.TrialBalance)
				r.With(middleware.RequirePermission(domain.PermReportsFinancialRead)).
					Get("/balance-sheet", p.ReportHandler.BalanceSheet)
				r.With(middleware.RequirePermission(domain.PermReportsFinancialRead)).
					Get("/income-statement", p.ReportHandler.IncomeStatement)
				r.With(middleware.RequirePermission(domain.PermReportsFinancialRead)).
					Get("/cash-flow", p.ReportHandler.CashFlow)
				// Daftar jatuh tempo operasional untuk teller: angsuran kredit dan
				// deposito berjangka. Baca saja, cabang dibatasi oleh service. Ini
				// bukan laporan keuangan bank-wide, jadi tetap cukup ledger:read
				// agar TELLER/CS/AO tidak kehilangan pekerjaannya.
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/due-obligations", p.ReportHandler.DueObligations)
				// Fondasi ekspor laporan OJK (APOLO): definisi/tenggat dan berkas
				// teks bulanan. Rute pemiliknya didaftarkan handler agar router tidak
				// menumpuk detail form di sini.
				if p.OJKReportHandler != nil {
					p.OJKReportHandler.RegisterRoutes(r)
				}
				// Laporan KPMM/ATMR BPR: bank-wide dan bersifat keuangan, jadi dijaga
				// reports:financial:read (bukan laporan operasional).
				if p.KPMMHandler != nil {
					r.With(middleware.RequirePermission(domain.PermReportsFinancialRead)).
						Get("/kpmm", p.KPMMHandler.Report)
				}
			})

			// ── Third-Party Integration Gateway (OJK SLIK / CBAS & Dukcapil) ──
			r.Route("/integrations", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermLoansRead)).
					Post("/slik/check", p.IntegrationHandler.CheckSLIK)
				r.With(middleware.RequirePermission(domain.PermCustomersRead)).
					Post("/dukcapil/verify", p.IntegrationHandler.VerifyDukcapil)
			})

			// ── Banking Business Date & EOD / EOM / EOY Batch Processes ──
			r.Get("/system/business-date", p.BatchProcessHandler.GetBusinessDate)

			// ── Pemantauan kesehatan operasional (baca-saja) ──
			// Izin baca konfigurasi/status instalasi yang sudah ada (system:config:read,
			// dipakai juga oleh bank-profile dan riwayat EOD); tidak ada izin baru.
			if p.MonitoringHandler != nil {
				r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
					Get("/system/monitoring", p.MonitoringHandler.Get)
			}

			// ── Definisi & riwayat langkah EOD (dikelola di database) ──
			// Definisi urutan adalah konfigurasi keuangan: baca cukup
			// system:config:read, ubah WAJIB system:config penuh (bukan yang read) dan
			// teraudit di service.
			if p.EODDefinitionHandler != nil {
				r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
					Get("/system/eod-definitions", p.EODDefinitionHandler.List)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Put("/system/eod-definitions", p.EODDefinitionHandler.Update)
				r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
					Get("/system/eod-runs", p.EODDefinitionHandler.History)
			}

			// ── Identitas bank tingkat instalasi (profil bank) ──
			// Bank boleh mengisi identitasnya sendiri tanpa SQL. Baca dijaga
			// system:config:read (Superadmin/Admin/Supervisor/Auditor) dan ubah
			// dijaga system:config, izin yang sama dengan setelan instalasi lain
			// (EOD/state sistem) — tidak ada izin baru yang dibuat. Profil ini
			// identitas instalasi, bukan data lini usaha, sehingga tidak mengikuti
			// cakupan buku.
			if p.BankProfileHandler != nil {
				r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
					Get("/system/bank-profile", p.BankProfileHandler.Get)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Put("/system/bank-profile", p.BankProfileHandler.Update)
			}
			// ── Identitas Form 00.00 yang tidak muat di bank_profile ──
			// Kunci ojk.* (surel, situs web, sandi kota/wilayah, penanggung jawab
			// laporan) sebelumnya hanya dapat diisi lewat SQL. Rute ini membuat bank
			// mengisinya sendiri, dengan izin dan audit yang sama dengan bank-profile.
			if p.OJKProfileHandler != nil {
				r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
					Get("/system/ojk-profile", p.OJKProfileHandler.Get)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Put("/system/ojk-profile", p.OJKProfileHandler.Update)
			}
			// ── Pengaturan aktivasi CKPN (parameter & ratifikasi) ──
			// Menggantikan pengisian SQL untuk jalur CKPN. Baca dijaga
			// system:config:read (auditor), ubah dijaga system:config, dengan validasi
			// yang menolak penyalakan prematur dan audit satu transaksi.
			if p.CKPNActivationHandler != nil {
				p.CKPNActivationHandler.RegisterRoutes(r)
			}
			r.Route("/batch", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Post("/eod", p.BatchProcessHandler.RunEOD)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Post("/eom", p.BatchProcessHandler.RunEOM)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Post("/eoy", p.BatchProcessHandler.RunEOY)
			})

			// ── Grup pengguna, izin, dan menu (dikelola di database) ──
			// Baca katalog cukup system:config:read (pengawas dapat meninjau).
			// MENGUBAH pemetaan izin hanya lewat pengajuan permissions:manage, dan
			// baru berlaku setelah disetujui pemeriksa lain (maker-checker).
			if p.PermissionHandler != nil {
				r.With(middleware.RequirePermission(domain.PermSystemConfigRead)).
					Get("/permissions/catalog", p.PermissionHandler.Catalog)
				r.With(middleware.RequirePermission(domain.PermPermissionsManage)).
					Post("/permissions/requests", p.PermissionHandler.RequestChange)
			}

			// ── Maker-Checker Workflow Queue ──
			r.Route("/maker-checker", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermMakerCheckerApprove)).
					Get("/pending", p.MakerCheckerHandler.ListPending)
				r.With(middleware.RequirePermission(domain.PermMakerCheckerApprove)).
					Post("/{id}/approve", p.MakerCheckerHandler.Approve)
				r.With(middleware.RequirePermission(domain.PermMakerCheckerReject)).
					Post("/{id}/reject", p.MakerCheckerHandler.Reject)
			})

			// ── Document & PDF Printable Generator (Slips, Loan Agreements, Passbooks) ──
			r.Route("/documents", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermTransactionsDeposit)).
					Get("/deposit-slip/{refNo}", p.DocumentHandler.DepositSlip)
				r.With(middleware.RequirePermission(domain.PermTransactionsDeposit)).
					Get("/withdrawal-slip/{refNo}", p.DocumentHandler.WithdrawalSlip)
				r.With(middleware.RequirePermission(domain.PermLoansRead)).
					Get("/loan-agreement/{loanId}", p.DocumentHandler.LoanAgreement)
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Get("/thermal-receipt/{receiptNo}", p.DocumentHandler.ThermalReceipt)
			})

			// ── Deposito berjangka (penempatan, akrual, pencairan) ──
			if p.DepositHandler != nil {
				p.DepositHandler.RegisterRoutes(r)
			}

			// ── PPAP & kolektibilitas harian ──
			if p.PPAPHandler != nil {
				p.PPAPHandler.RegisterRoutes(r)
			}

			// ── CKPN (SAK EP) dan perbandingannya dengan PPKA ──
			if p.CKPNHandler != nil {
				p.CKPNHandler.RegisterRoutes(r)
			}

			// ── PPKA penempatan yang dijamin LPS (Pasal 23 POJK 1/2024) ──
			if p.LPSPlacementHandler != nil {
				p.LPSPlacementHandler.RegisterRoutes(r)
			}

			// ── Audit log (baca saja, untuk pengawas) ──
			if p.AuditHandler != nil {
				p.AuditHandler.RegisterRoutes(r)
			}

			// ── Agunan kredit ──
			if p.CollateralHandler != nil {
				p.CollateralHandler.RegisterRoutes(r)
			}

			// ── Gerbang aktivasi bobot agunan (Lampiran II SEOJK 2/2025) ──
			if p.CollateralWeightHandler != nil {
				p.CollateralWeightHandler.RegisterRoutes(r)
			}

			// ── Chart of Accounts (Admin & above) ──
			r.With(middleware.RequirePermission(domain.PermCOAManage)).
				Get("/chart-of-accounts", p.LedgerHandler.ListCOA)
		})
	})

	return r
}
