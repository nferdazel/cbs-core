package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cbs-core/apps/core-api/internal/config"
	"cbs-core/apps/core-api/internal/crypto"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/middleware"
	"cbs-core/apps/core-api/internal/observability"
	"cbs-core/apps/core-api/internal/ojkreport"
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
		// Kunci indeks terpisah opsional; bila kosong, indeks memakai master key
		// enkripsi seperti sebelumnya (nilai indeks lama tidak berubah).
		cipher, err = crypto.NewCipherWithIndexKey(
			cfg.EncryptionKeyID, cfg.EncryptionMasterKey, cfg.EncryptionPreviousKey,
			crypto.IndexKeyConfig{
				ActiveKeyID:  cfg.EncryptionIndexKeyID,
				ActiveKey:    cfg.EncryptionIndexKey,
				PreviousKeys: cfg.EncryptionPreviousIndexKeys,
			},
		)
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
	bankProfileRepo := postgres.NewBankProfileRepository(db)
	// Definisi & riwayat langkah EOD tinggal di database (keputusan pemilik sistem).
	eodStepRepo := postgres.NewEODStepRepository(db)

	slikGateway := service.NewMockSLIKGateway()
	dukcapilGateway := service.NewMockDukcapilGateway()

	// 4. Core services
	configSvc := service.NewSystemConfigService(configRepo)
	// Peringatan cakupan buku instalasi dicatat saat mulai agar ketidakcocokan
	// konfigurasi (mis. SYARIAH tanpa pemetaan akun CKPN syariah) terlihat sebelum
	// transaksi berjalan, bukan setelah jurnal jatuh ke akun konvensional.
	for _, warning := range service.InstallationValidationWarnings(context.Background(), configSvc) {
		logger.Warn("peringatan konfigurasi instalasi", "pesan", warning)
	}
	postingSvc := service.NewPostingService(db, ledgerRepo, accountRepo, ledgerRepo, referenceGen, dateRepo)
	poster := service.NewProductPoster(productRepo, ledgerRepo, postingSvc)

	// Registry memutus siklus ledger <-> maker-checker: ledger mengajukan persetujuan,
	// maker-checker mengeksekusi lewat registry, bukan memegang ledger secara langsung.
	executors := service.NewExecutorRegistry()
	mcRepo := postgres.NewMakerCheckerRepository(db)
	mcSvc := service.NewMakerCheckerService(db, mcRepo, auditRepo, configSvc, executors, dateRepo, branchRepo)
	// Grup pengguna & pemetaan izin tinggal di database (keputusan pemilik sistem).
	// Perubahan izin diajukan lewat maker-checker dan diterapkan eksekutor ini saat
	// disetujui; kode hanya menjadi seed awal migrasi 000079. Repositori yang sama
	// menjadi kait peran limit grup (user_groups.approval_limit_role) ke matriks
	// limit 000051, tanpa matriks kedua.
	permissionRepo := postgres.NewPermissionRepository(db)
	limitSvc := service.NewTransactionLimitService(configSvc, ledgerRepo, dateRepo, permissionRepo)

	permissionSvc := service.NewPermissionService(permissionRepo, auditRepo, mcSvc)
	executors.Register(service.ActionPermissionChange, permissionSvc)

	customerSvc := service.NewCustomerService(db, customerRepo, cipher, referenceGen, auditRepo)
	accountSvc := service.NewAccountService(db, accountRepo, customerRepo, customerSvc, productRepo, branchRepo, numberingRepo, configSvc, auditRepo)
	branchSvc := service.NewBranchService(db, branchRepo, auditRepo)
	productSvc := service.NewProductService(db, productRepo, auditRepo)
	ledgerSvc := service.NewLedgerService(db, ledgerRepo, accountRepo, productRepo, ledgerRepo, postingSvc, configSvc, limitSvc, mcSvc, dateRepo, auditRepo)

	// Ledger service adalah eksekutor untuk transaksi rekening yang disetujui.
	executors.Register(service.ActionDeposit, ledgerSvc)
	executors.Register(service.ActionWithdraw, ledgerSvc)
	executors.Register(service.ActionTransfer, ledgerSvc)
	// Pembatalan transaksi lintas hari dieksekusi setelah disetujui pejabat kedua.
	executors.Register(service.ActionReverse, ledgerSvc)
	authSvc := service.NewAuthService(staffRepo, sessionRepo, configRepo, cfg.JWTSecret)
	staffSvc := service.NewStaffService(staffRepo, auditRepo)
	loanSvc := service.NewLoanService(db, loanRepo, productRepo, accountRepo, ledgerRepo, poster, postingSvc, referenceGen, configSvc, mcSvc, dateRepo, auditRepo)
	// Hapus buku, recovery, dan koreksi nominal kredit dieksekusi setelah disetujui
	// pejabat kedua.
	executors.Register(service.ActionLoanWriteOff, loanSvc)
	executors.Register(service.ActionLoanRecovery, loanSvc)
	executors.Register(service.ActionLoanCorrection, loanSvc)
	reportSvc := service.NewReportService(reportRepo)
	collectionSvc := service.NewCollectionService(ledgerSvc, loanSvc)
	savingsSvc := service.NewSavingsInterestService(db, savingsRepo, accountRepo, productRepo, poster, postingSvc, ledgerRepo, configSvc)
	depositSvc := service.NewDepositService(db, depositRepo, productRepo, accountRepo, ledgerRepo, customerRepo, branchRepo, numberingRepo, poster, postingSvc, ledgerRepo, configSvc, limitSvc, mcSvc, auditRepo)
	// Penempatan deposito di atas ambang persetujuan dieksekusi setelah disetujui,
	// memakai penjaga batas dan alur maker-checker yang sama dengan setoran tunai.
	executors.Register(service.ActionPlaceDeposit, depositSvc)
	// Repositori agunan dipakai dua jalur: pencatatan agunan dan pengurangan eksposur PPAP.
	collateralRepo := postgres.NewCollateralRepository(db)
	// Penanda tanggal bisnis run PPAP terakhir dibagi ke PPAP (penulis) dan CKPN
	// (pembaca) agar perbandingan CKPN menolak required_ppap dari tanggal bisnis lain.
	ppapRunMarker := service.NewPPAPRunMarker(configRepo)
	ppapSvc := service.NewPPAPService(db, ppapRepo, productRepo, ledgerRepo, poster, postingSvc, configSvc, ppapRunMarker, collateralRepo)
	// CKPN (SAK EP, SEOJK 21/2024) adalah konsep terpisah dari PPKA. Modul ini membaca
	// target PPKA yang sudah disimpan (loans.required_ppap) untuk membandingkannya,
	// bukan menghitung ulang PPKA. Kredit dikunci lewat LoanRepository saat menulis.
	ckpnSvc := service.NewCKPNService(db, postgres.NewCKPNRepository(db), productRepo, ledgerRepo, poster, postingSvc, configSvc, loanRepo, ppapRunMarker)
	// Pasal 23 POJK No. 1 Tahun 2024: pengurang PPKA umum dan khusus untuk bagian
	// Penempatan pada Bank Lain yang dijamin LPS. Baca-saja; saklar ppap.lps.enabled
	// bawaan false sehingga belum mengubah angka PPKA mana pun.
	lpsPlacementSvc := service.NewLPSPlacementService(postgres.NewLPSPlacementRepository(db), configSvc)
	// Batch dibuat setelah layanan yang dijalankannya setiap tutup hari tersedia:
	// ARO deposito, PPAP harian, akrual denda kredit, akrual bunga kredit, dan
	// penandaan rekening dormant.
	batchSvc := service.NewBatchProcessService(dateRepo, batchRepo, savingsSvc, yearEndRepo, postingSvc, ledgerRepo, configSvc, db, depositSvc, ppapSvc, loanSvc, accountSvc, loanSvc, ckpnSvc, eodStepRepo)
	// Penjadwal EOD (W17): goroutine yang memeriksa pemicu terjadwal tiap menit dan
	// menjalankannya lewat jalur yang sama dengan pemicu manual. Ia MATI secara default
	// karena saklar eod.scheduler.enabled di-seed false (migrasi 000083); bank
	// menyalakannya dengan mengubah kunci itu (berlaku tanpa restart) dan mengaktifkan
	// pemicu terjadwal di eod_triggers. Tanpa database, penjadwal tidak dijalankan.
	if db != nil {
		eodScheduler := service.NewEODScheduler(batchSvc, eodStepRepo, configSvc, logger)
		go eodScheduler.Start(context.Background())
	}
	// Definisi langkah EOD (urutan/saklar/prasyarat) dikelola lewat API berizin
	// system:config dan teraudit; riwayat per langkah dibaca di sini.
	eodDefinitionSvc := service.NewEODDefinitionService(db, eodStepRepo, auditRepo)
	docSvc := service.NewDocumentService(ledgerRepo, accountRepo, loanRepo, customerRepo, bankProfileRepo, cipher)
	// Identitas aplikasi: nama PT dari bank_profile, branding dari system_config.
	// Dipakai endpoint publik /app-info (halaman login + metadata web).
	appInfoSvc := service.NewAppInfoService(bankProfileRepo, configSvc)
	// Profil bank: bank mengisi identitasnya lewat API/web, teraudit, tanpa SQL.
	bankProfileSvc := service.NewBankProfileService(db, bankProfileRepo, auditRepo)

	// 5. HTTP Handlers
	cookies := middleware.CookieConfig{
		AccessName:  cfg.AccessCookieName,
		RefreshName: cfg.RefreshCookieName,
		CSRFName:    cfg.CSRFCookieName,
		CSRFHeader:  cfg.CSRFHeaderName,
		Domain:      cfg.CookieDomain,
		Secure:      cfg.Environment == "production",
	}

	// Pembatasan percobaan login: brute force per akun dan credential spraying per IP.
	loginLimiter := middleware.NewLoginRateLimiter(middleware.LoginRateLimitConfig{
		AccountMax:    cfg.LoginRateLimitAccountMax,
		AccountWindow: cfg.LoginRateLimitAccountWindow,
		IPMax:         cfg.LoginRateLimitIPMax,
		IPWindow:      cfg.LoginRateLimitIPWindow,
	})
	defer loginLimiter.Close()
	custHandler := httpHandler.NewCustomerHandler(customerSvc)
	accHandler := httpHandler.NewAccountHandler(accountSvc)
	branchHandler := httpHandler.NewBranchHandler(branchSvc)
	productHandler := httpHandler.NewProductHandler(productSvc)
	ledHandler := httpHandler.NewLedgerHandler(ledgerSvc)
	authHandler := httpHandler.NewAuthHandler(authSvc, cookies)
	staffHandler := httpHandler.NewStaffHandler(staffSvc)
	loanHandler := httpHandler.NewLoanHandler(loanSvc)
	mcHandler := httpHandler.NewMakerCheckerHandler(mcSvc)
	reportHandler := httpHandler.NewReportHandler(reportSvc)
	// Ekspor OJK memakai laporan journal-based yang sama; hanya pemetaan pos OJK
	// yang ditambahkan, tanpa menghitung ulang rumus akuntansi. RepoSource menambah
	// sumber form daftar: kredit (Form 06.00/NPL), profil bank (Form 00.00), dan
	// penempatan pada bank lain (Form 05.00).
	//
	// Peninjauan pemetaan memakai bagan akun untuk menampilkan nama akun dan repositori
	// keputusan (migrasi 000050) supaya persetujuan bank bertahan dan dapat diaudit.
	ojkReportHandler := httpHandler.NewOJKReportHandler(ojkreport.RepoSource{
		Source:     reportSvc,
		Loans:      loanRepo,
		Profile:    bankProfileRepo,
		Config:     configRepo,
		Placements: postgres.NewLPSPlacementRepository(db),
	}, ledgerRepo, postgres.NewOJKMappingReviewRepository(db))
	collectionHandler := httpHandler.NewCollectionHandler(collectionSvc)
	integrationHandler := httpHandler.NewIntegrationHandler(slikGateway, dukcapilGateway)
	batchHandler := httpHandler.NewBatchProcessHandler(batchSvc)
	eodDefinitionHandler := httpHandler.NewEODDefinitionHandler(eodDefinitionSvc)
	docHandler := httpHandler.NewDocumentHandler(docSvc)
	appInfoHandler := httpHandler.NewAppInfoHandler(appInfoSvc)
	bankProfileHandler := httpHandler.NewBankProfileHandler(bankProfileSvc)
	depositHandler := httpHandler.NewDepositHandler(depositSvc)
	ppapHandler := httpHandler.NewPPAPHandler(ppapSvc)
	ckpnHandler := httpHandler.NewCKPNHandler(ckpnSvc)
	lpsPlacementHandler := httpHandler.NewLPSPlacementHandler(lpsPlacementSvc)
	permissionHandler := httpHandler.NewPermissionHandler(permissionSvc)
	collateralSvc := service.NewCollateralService(collateralRepo, configSvc, branchRepo, auditRepo)

	// Laporan KPMM/ATMR memakai laporan journal-based, perbandingan PPKA-CKPN
	// (baca-saja), dan parameter kpmm.* di system_config. Tidak menyentuh saklar
	// ckpn.enabled.
	kpmmSvc := service.NewKPMMService(reportSvc, ckpnSvc, configSvc)
	kpmmHandler := httpHandler.NewKPMMHandler(kpmmSvc)

	// 6. Router
	router := httpHandler.NewRouter(httpHandler.RouterParams{
		CustomerHandler:      custHandler,
		AccountHandler:       accHandler,
		BranchHandler:        branchHandler,
		ProductHandler:       productHandler,
		LedgerHandler:        ledHandler,
		AuthHandler:          authHandler,
		StaffHandler:         staffHandler,
		LoanHandler:          loanHandler,
		MakerCheckerHandler:  mcHandler,
		ReportHandler:        reportHandler,
		OJKReportHandler:     ojkReportHandler,
		KPMMHandler:          kpmmHandler,
		CollectionHandler:    collectionHandler,
		IntegrationHandler:   integrationHandler,
		BatchProcessHandler:  batchHandler,
		EODDefinitionHandler: eodDefinitionHandler,
		DocumentHandler:      docHandler,
		DepositHandler:       depositHandler,
		PPAPHandler:          ppapHandler,
		CKPNHandler:          ckpnHandler,
		LPSPlacementHandler:  lpsPlacementHandler,
		AuditHandler:         httpHandler.NewAuditHandler(auditRepo, limitSvc),
		CollateralHandler:    httpHandler.NewCollateralHandler(collateralSvc),
		AppInfoHandler:       appInfoHandler,
		BankProfileHandler:   bankProfileHandler,
		PermissionHandler:    permissionHandler,
		AuthService:          authSvc,
		ConfigService:        configSvc,
		// Cakupan unit organisasi (cabang/area/wilayah) diresolusi per permintaan
		// dari tabel branches, sama seperti cakupan buku dibaca dari konfigurasi.
		BranchScopeResolver: branchRepo,
		Cookies:             cookies,
		Logger:              logger,
		LoginRateLimiter:    loginLimiter,
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
		// Hanya status boolean; nilai kunci tidak pernah ditulis ke log.
		"indeks_terpisah", cfg.EncryptionIndexKey != "",
	)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server berhenti dengan error", "error", err)
		os.Exit(1)
	}
}
