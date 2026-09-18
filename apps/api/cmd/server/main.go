package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"cbs-core/apps/core-api/internal/config"
	"cbs-core/apps/core-api/internal/crypto"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/observability"
	"cbs-core/apps/core-api/internal/repository/postgres"
	"cbs-core/apps/core-api/internal/service"
)

func main() {
	cfg := config.Load()
	logger := observability.NewLogger(cfg.Environment)
	slog.SetDefault(logger)

	logger.Info("memulai Core Banking System (CBS) Core API",
		"environment", cfg.Environment, "port", cfg.Port)

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
		logger.Error("koneksi database gagal; server berjalan tanpa database", "error", err)
	} else {
		defer db.Close()
		logger.Info("postgresql terhubung")
	}

	// 2. Enkripsi data pribadi (envelope encryption, master key dari environment)
	var cipher *crypto.Cipher
	if cfg.EncryptionMasterKey != "" {
		cipher, err = crypto.NewCipher(cfg.EncryptionKeyID, cfg.EncryptionMasterKey, cfg.EncryptionPreviousKey)
		if err != nil {
			logger.Error("konfigurasi enkripsi tidak valid", "error", err)
			os.Exit(1)
		}
	}

	// 3. Repositories & Third-Party Gateways
	customerRepo := postgres.NewCustomerRepository(db)
	accountRepo := postgres.NewAccountRepository(db)
	ledgerRepo := postgres.NewLedgerRepository(db)
	productRepo := postgres.NewProductRepository(db)
	branchRepo := postgres.NewBranchRepository(db)
	numberingRepo := postgres.NewNumberingRepository(db)
	staffRepo := postgres.NewStaffRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	configRepo := postgres.NewSystemConfigRepository(db)
	auditRepo := postgres.NewAuditRepository(db)
	loanRepo := postgres.NewLoanRepository(db)
	reportRepo := postgres.NewReportRepository(db)
	dateRepo := postgres.NewBusinessDateRepository(db)
	referenceGen := postgres.NewReferenceGenerator(db)
	depositRepo := postgres.NewDepositRepository(db)
	ppapRepo := postgres.NewPPAPRepository(db)
	batchRepo := postgres.NewBatchActivityRepository(db)
	savingsRepo := postgres.NewSavingsInterestRepository(db)
	yearEndRepo := postgres.NewYearEndRepository(db)

	slikGateway := service.NewMockSLIKGateway()
	dukcapilGateway := service.NewMockDukcapilGateway()

	// 4. Core services
	configSvc := service.NewSystemConfigService(configRepo)
	postingSvc := service.NewPostingService(db, ledgerRepo, accountRepo, ledgerRepo, referenceGen)
	poster := service.NewProductPoster(productRepo, ledgerRepo, postingSvc)

	// Registry memutus siklus ledger <-> maker-checker: ledger mengajukan persetujuan,
	// maker-checker mengeksekusi lewat registry, bukan memegang ledger secara langsung.
	executors := service.NewExecutorRegistry()
	mcRepo := postgres.NewMakerCheckerRepository(db)
	mcSvc := service.NewMakerCheckerService(db, mcRepo, auditRepo, configSvc, executors)
	limitSvc := service.NewTransactionLimitService(configSvc, ledgerRepo)

	customerSvc := service.NewCustomerService(db, customerRepo, cipher, referenceGen, auditRepo)
	accountSvc := service.NewAccountService(db, accountRepo, customerRepo, customerSvc, productRepo, branchRepo, numberingRepo, auditRepo)
	branchSvc := service.NewBranchService(branchRepo)
	productSvc := service.NewProductService(productRepo)
	ledgerSvc := service.NewLedgerService(db, ledgerRepo, accountRepo, productRepo, ledgerRepo, postingSvc, configSvc, limitSvc, mcSvc)

	// Ledger service adalah eksekutor untuk transaksi rekening yang disetujui.
	executors.Register(service.ActionDeposit, ledgerSvc)
	executors.Register(service.ActionWithdraw, ledgerSvc)
	executors.Register(service.ActionTransfer, ledgerSvc)
	authSvc := service.NewAuthService(staffRepo, sessionRepo, configRepo, cfg.JWTSecret)
	staffSvc := service.NewStaffService(staffRepo)
	loanSvc := service.NewLoanService(db, loanRepo, productRepo, accountRepo, poster, referenceGen, configSvc, auditRepo)
	reportSvc := service.NewReportService(reportRepo)
	collectionSvc := service.NewCollectionService(ledgerSvc, loanSvc)
	savingsSvc := service.NewSavingsInterestService(db, savingsRepo, accountRepo, productRepo, poster, postingSvc, ledgerRepo, configSvc)
	batchSvc := service.NewBatchProcessService(dateRepo, batchRepo, savingsSvc, yearEndRepo, postingSvc, ledgerRepo, configSvc, db)
	depositSvc := service.NewDepositService(db, depositRepo, productRepo, accountRepo, ledgerRepo, customerRepo, branchRepo, numberingRepo, poster, postingSvc, ledgerRepo, configSvc, auditRepo)
	ppapSvc := service.NewPPAPService(db, ppapRepo, productRepo, ledgerRepo, poster, postingSvc, configSvc)
	docSvc := service.NewDocumentService(ledgerRepo, accountRepo, loanRepo, customerRepo, cipher)

	// 5. HTTP Handlers
	custHandler := httpHandler.NewCustomerHandler(customerSvc)
	accHandler := httpHandler.NewAccountHandler(accountSvc)
	branchHandler := httpHandler.NewBranchHandler(branchSvc)
	productHandler := httpHandler.NewProductHandler(productSvc)
	ledHandler := httpHandler.NewLedgerHandler(ledgerSvc)
	authHandler := httpHandler.NewAuthHandler(authSvc)
	staffHandler := httpHandler.NewStaffHandler(staffSvc)
	loanHandler := httpHandler.NewLoanHandler(loanSvc)
	mcHandler := httpHandler.NewMakerCheckerHandler(mcSvc)
	reportHandler := httpHandler.NewReportHandler(reportSvc)
	collectionHandler := httpHandler.NewCollectionHandler(collectionSvc)
	integrationHandler := httpHandler.NewIntegrationHandler(slikGateway, dukcapilGateway)
	batchHandler := httpHandler.NewBatchProcessHandler(batchSvc)
	docHandler := httpHandler.NewDocumentHandler(docSvc)
	depositHandler := httpHandler.NewDepositHandler(depositSvc)
	ppapHandler := httpHandler.NewPPAPHandler(ppapSvc)

	// 6. Router
	router := httpHandler.NewRouter(httpHandler.RouterParams{
		CustomerHandler:     custHandler,
		AccountHandler:      accHandler,
		BranchHandler:       branchHandler,
		ProductHandler:      productHandler,
		LedgerHandler:       ledHandler,
		AuthHandler:         authHandler,
		StaffHandler:        staffHandler,
		LoanHandler:         loanHandler,
		MakerCheckerHandler: mcHandler,
		ReportHandler:       reportHandler,
		CollectionHandler:   collectionHandler,
		IntegrationHandler:  integrationHandler,
		BatchProcessHandler: batchHandler,
		DocumentHandler:     docHandler,
		DepositHandler:      depositHandler,
		PPAPHandler:         ppapHandler,
		AuthService:         authSvc,
		Logger:              logger,
	})

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	logger.Info("server HTTP siap menerima permintaan",
		"addr", server.Addr,
		"environment", cfg.Environment,
		"enkripsi_nasabah", cfg.EncryptionMasterKey != "",
	)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server berhenti dengan error", "error", err)
		os.Exit(1)
	}
}
