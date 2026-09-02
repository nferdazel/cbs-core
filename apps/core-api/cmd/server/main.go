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
		log.Printf("⚠️ Database connection failed (%v). Continuing in standalone mode...", err)
	} else {
		defer db.Close()
		log.Println("✅ PostgreSQL connected successfully")
	}

	// 2. Repositories
	customerRepo := postgres.NewCustomerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)

	// 3. Services
	customerSvc := service.NewCustomerService(customerRepo)
	accountSvc := service.NewAccountService(accountRepo, customerRepo)
	ledgerSvc := service.NewLedgerService(ledgerRepo, accountRepo, db)

	// 4. HTTP Handlers
	custHandler := httpHandler.NewCustomerHandler(customerSvc)
	accHandler := httpHandler.NewAccountHandler(accountSvc)
	ledHandler := httpHandler.NewLedgerHandler(ledgerSvc)

	// 5. Router
	router := httpHandler.NewRouter(httpHandler.RouterParams{
		CustomerHandler: custHandler,
		AccountHandler:  accHandler,
		LedgerHandler:   ledHandler,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("📡 HTTP Server running on http://localhost:%s\n", cfg.Port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}
}
