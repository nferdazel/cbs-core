package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type RouterParams struct {
	CustomerHandler *CustomerHandler
	AccountHandler  *AccountHandler
	LedgerHandler   *LedgerHandler
}

func NewRouter(p RouterParams) *chi.Mux {
	r := chi.NewRouter()

	// Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// CORS Configuration for Backoffice Next.js Frontend
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:3001", "https://cbs.qouver.com", "https://*.qouver.com", "*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Idempotency-Key"},
		ExposedHeaders:   []string{"Link", "Idempotency-Key"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		Success(w, http.StatusOK, "Core Banking API is healthy", map[string]string{"status": "UP"})
	})

	// API Routes
	r.Route("/api/v1", func(r chi.Router) {
		// Customers / CIF
		r.Route("/customers", func(r chi.Router) {
			r.Post("/", p.CustomerHandler.Register)
			r.Get("/", p.CustomerHandler.List)
			r.Get("/{id}", p.CustomerHandler.GetByID)
		})

		// Accounts
		r.Route("/accounts", func(r chi.Router) {
			r.Post("/open", p.AccountHandler.Open)
			r.Get("/", p.AccountHandler.List)
			r.Get("/{accountNumber}", p.AccountHandler.GetByNumber)
			r.Get("/{accountNumber}/statements", p.LedgerHandler.GetStatement)
		})

		// Ledger & Transactions (Double-Entry Engine)
		r.Route("/transactions", func(r chi.Router) {
			r.Post("/deposit", p.LedgerHandler.Deposit)
			r.Post("/withdraw", p.LedgerHandler.Withdraw)
			r.Post("/transfer", p.LedgerHandler.Transfer)
			r.Get("/journals", p.LedgerHandler.ListJournals)
			r.Get("/journals/{reference}", p.LedgerHandler.GetJournalByRef)
		})

		// Chart of Accounts
		r.Get("/chart-of-accounts", p.LedgerHandler.ListCOA)
	})

	return r
}
