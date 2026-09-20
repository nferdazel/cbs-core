package http

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
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
	BatchProcessHandler *BatchProcessHandler
	DocumentHandler     *DocumentHandler
	DepositHandler      *DepositHandler
	PPAPHandler         *PPAPHandler
	AuditHandler        *AuditHandler
	CollateralHandler   *CollateralHandler
	AuthService         domain.AuthService
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
		Success(w, http.StatusOK, "Core Banking API is healthy", map[string]string{"status": "UP"})
	})

	r.Route("/api/v1", func(r chi.Router) {

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
				r.Use(middleware.CSRFMiddleware(p.Cookies))
				r.Get("/me", p.AuthHandler.Me)
			})
		})

		// ── All routes below require authentication ──
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(p.AuthService, p.Cookies))
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
			})

			// ── Master data: cabang & produk ──
			r.Route("/branches", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermUsersRead)).
					Get("/", p.BranchHandler.List)
			})
			r.Route("/products", func(r chi.Router) {
				// Data referensi produk dipakai layar rekening, deposito, dan kredit.
				// Sebelumnya dijaga loans:read sehingga TELLER tidak dapat memuat daftar
				// produk, padahal ia berwenang membuka rekening.
				r.With(middleware.RequirePermission(domain.PermProductsRead)).
					Get("/", p.ProductHandler.List)
				r.With(middleware.RequirePermission(domain.PermProductsRead)).
					Get("/{id}", p.ProductHandler.GetByID)
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
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Post("/{id}/pay-installment", p.LoanHandler.PayInstallment)
				r.With(middleware.RequirePermission(domain.PermLoansApprove)).
					Post("/{id}/restructure", p.LoanHandler.Restructure)
				r.With(middleware.RequirePermission(domain.PermLoansApprove)).
					Post("/{id}/write-off", p.LoanHandler.WriteOff)
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Post("/{id}/recover", p.LoanHandler.Recover)
			})

			// ── Mobile Field Collections (Jemput Bola) ──
			r.Route("/collections", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Post("/mobile-collect", p.CollectionHandler.ProcessMobileCollection)
			})

			// ── Financial Statement Reports (dihitung dari jurnal) ──
			r.Route("/reports", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/trial-balance", p.ReportHandler.TrialBalance)
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/balance-sheet", p.ReportHandler.BalanceSheet)
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/income-statement", p.ReportHandler.IncomeStatement)
				r.With(middleware.RequirePermission(domain.PermLedgerRead)).
					Get("/cash-flow", p.ReportHandler.CashFlow)
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
			r.Route("/batch", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Post("/eod", p.BatchProcessHandler.RunEOD)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Post("/eom", p.BatchProcessHandler.RunEOM)
				r.With(middleware.RequirePermission(domain.PermSystemConfig)).
					Post("/eoy", p.BatchProcessHandler.RunEOY)
			})

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

			// ── Audit log (baca saja, untuk pengawas) ──
			if p.AuditHandler != nil {
				p.AuditHandler.RegisterRoutes(r)
			}

			// ── Agunan kredit ──
			if p.CollateralHandler != nil {
				p.CollateralHandler.RegisterRoutes(r)
			}

			// ── Chart of Accounts (Admin & above) ──
			r.With(middleware.RequirePermission(domain.PermCOAManage)).
				Get("/chart-of-accounts", p.LedgerHandler.ListCOA)
		})
	})

	return r
}
