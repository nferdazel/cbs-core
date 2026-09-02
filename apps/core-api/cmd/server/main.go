package main

import (
	"log"
	"net/http"
	"time"

	"cbs-core/apps/core-api/internal/config"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

func main() {
	cfg := config.Load()

	log.Println("==================================================")
	log.Println("🚀 Starting Core Banking System (CBS) Core API...")
	log.Println("==================================================")

	// 1. Initialize Database
	db, err := postgres.NewDB(postgres.Config{
		Host:     cfg.DBHost,
		Port:     cfg.DBPort,
		User:     cfg.DBUser,
		Password: cfg.DBPassword,
		DBName:   cfg.DBName,
		SSLMode:  cfg.DBSSLMode,
	})
	if err != nil {
		log.Printf("⚠️  Database connection failed (%v). Continuing in standalone mode...", err)
	} else {
		defer db.Close()
		log.Println("✅ PostgreSQL connected successfully")
	}

	// 2. Repositories & Third-Party Gateways
	customerRepo := postgres.NewCustomerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	staffRepo := postgres.NewStaffRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	configRepo := postgres.NewSystemConfigRepository(db)
	loanRepo := postgres.NewLoanRepository(db)
	reportRepo := postgres.NewReportRepository(db)

	slikGateway := service.NewMockSLIKGateway()
	dukcapilGateway := service.NewMockDukcapilGateway()

	// 3. Services
	customerSvc := service.NewCustomerService(customerRepo)
	accountSvc := service.NewAccountService(accountRepo, customerRepo)
	ledgerSvc := service.NewLedgerService(ledgerRepo, accountRepo, db)
	authSvc := service.NewAuthService(staffRepo, sessionRepo, configRepo, cfg.JWTSecret)
	staffSvc := service.NewStaffService(staffRepo)
	loanSvc := service.NewLoanService(loanRepo, accountRepo, customerRepo, ledgerSvc)
	reportSvc := service.NewReportService(reportRepo)
	collectionSvc := service.NewCollectionService(ledgerSvc, loanSvc)

	// 4. HTTP Handlers
	custHandler := httpHandler.NewCustomerHandler(customerSvc)
	accHandler := httpHandler.NewAccountHandler(accountSvc)
	ledHandler := httpHandler.NewLedgerHandler(ledgerSvc)
	authHandler := httpHandler.NewAuthHandler(authSvc)
	staffHandler := httpHandler.NewStaffHandler(staffSvc)
	loanHandler := httpHandler.NewLoanHandler(loanSvc)
	mcHandler := httpHandler.NewMakerCheckerHandler(db)
	reportHandler := httpHandler.NewReportHandler(reportSvc)
	collectionHandler := httpHandler.NewCollectionHandler(collectionSvc)
	integrationHandler := httpHandler.NewIntegrationHandler(slikGateway, dukcapilGateway)

	// 5. Router
	router := httpHandler.NewRouter(httpHandler.RouterParams{
		CustomerHandler:     custHandler,
		AccountHandler:      accHandler,
		LedgerHandler:       ledHandler,
		AuthHandler:         authHandler,
		StaffHandler:        staffHandler,
		LoanHandler:         loanHandler,
		MakerCheckerHandler: mcHandler,
		ReportHandler:       reportHandler,
		CollectionHandler:   collectionHandler,
		IntegrationHandler:  integrationHandler,
		AuthService:         authSvc,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("📡 HTTP Server running on http://localhost:%s\n", cfg.Port)
	log.Printf("🔐 Auth: JWT %s access token | Session-based refresh\n", cfg.Environment)
	log.Printf("🔌 Integration Middleware: OJK SLIK / CBAS & Dukcapil Gateways active\n")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}
}
