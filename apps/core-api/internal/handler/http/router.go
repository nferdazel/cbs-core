package http

import (
	"net/http"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/middleware"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type RouterParams struct {
	CustomerHandler     *CustomerHandler
	AccountHandler      *AccountHandler
	LedgerHandler       *LedgerHandler
	AuthHandler         *AuthHandler
	StaffHandler        *StaffHandler
	LoanHandler         *LoanHandler
	MakerCheckerHandler *MakerCheckerHandler
	ReportHandler       *ReportHandler
	CollectionHandler   *CollectionHandler
	IntegrationHandler  *IntegrationHandler
	BatchProcessHandler *BatchProcessHandler
	AuthService         domain.AuthService
}

func NewRouter(p RouterParams) *chi.Mux {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Logger)
	r.Use(chiMiddleware.Recoverer)

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001", "https://cbs.qouver.com", "https://*.qouver.com"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "Idempotency-Key"},
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
			r.Post("/login", p.AuthHandler.Login)
			r.Post("/refresh", p.AuthHandler.Refresh)

			// Protected auth routes (require valid token)
			r.Group(func(r chi.Router) {
				r.Use(middleware.AuthMiddleware(p.AuthService))
				r.Post("/logout", p.AuthHandler.Logout)
				r.Get("/me", p.AuthHandler.Me)
			})
		})

		// ── All routes below require authentication ──
		r.Group(func(r chi.Router) {
			r.Use(middleware.AuthMiddleware(p.AuthService))

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

			// ── Accounts ──
			r.Route("/accounts", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermAccountsOpen)).
					Post("/open", p.AccountHandler.Open)
				r.With(middleware.RequirePermission(domain.PermAccountsRead)).
					Get("/", p.AccountHandler.List)
				r.With(middleware.RequirePermission(domain.PermAccountsRead)).
					Get("/{accountNumber}", p.AccountHandler.GetByNumber)
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
			})

			// ── Mobile Field Collections (Jemput Bola) ──
			r.Route("/collections", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermCollectionsInput)).
					Post("/mobile-collect", p.CollectionHandler.ProcessMobileCollection)
			})

			// ── Financial Statement Reports ──
			r.Route("/reports", func(r chi.Router) {
				r.With(middleware.RequirePermission(domain.PermReportsExport)).
					Get("/trial-balance", p.ReportHandler.GetTrialBalance)
				r.With(middleware.RequirePermission(domain.PermReportsExport)).
					Get("/balance-sheet", p.ReportHandler.GetBalanceSheet)
				r.With(middleware.RequirePermission(domain.PermReportsExport)).
					Get("/income-statement", p.ReportHandler.GetIncomeStatement)
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

			// ── Chart of Accounts (Admin & above) ──
			r.With(middleware.RequirePermission(domain.PermCOAManage)).
				Get("/chart-of-accounts", p.LedgerHandler.ListCOA)
		})
	})

	return r
}
