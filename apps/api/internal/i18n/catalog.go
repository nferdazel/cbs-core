package i18n

// Code adalah kode pesan API. Nilainya internal dan stabil; handler memakai
// konstanta di bawah ini, bukan literal, sehingga pesan tidak lagi berserak.
type Code string

// Kode pesan API. Nama disusun dari makna pesan, bukan dari bahasa.
const (
	MsgAccountOpened                  Code = "account_opened"
	MsgAccountRetrieved               Code = "account_retrieved"
	MsgAccountReactivated             Code = "account_reactivated"
	MsgAccountsListed                 Code = "accounts_listed"
	MsgLoginSuccessful                Code = "login_successful"
	MsgTokenRefreshed                 Code = "token_refreshed"
	MsgLoggedOut                      Code = "logged_out"
	MsgCurrentUser                    Code = "current_user"
	MsgAppInfo                        Code = "app_info"
	MsgLoanApplicationSubmitted       Code = "loan_application_submitted"
	MsgLoansRetrieved                 Code = "loans_retrieved"
	MsgLoanRetrieved                  Code = "loan_retrieved"
	MsgLoanApproved                   Code = "loan_approved"
	MsgLoanRejected                   Code = "loan_rejected"
	MsgLoanDisbursed                  Code = "loan_disbursed"
	MsgInstallmentRecorded            Code = "installment_recorded"
	MsgLoanRestructured               Code = "loan_restructured"
	MsgLoanWrittenOff                 Code = "loan_written_off"
	MsgLoanRecoveryRecorded           Code = "loan_recovery_recorded"
	MsgLoanDisbursementCancelled      Code = "loan_disbursement_cancelled"
	MsgLoanAmountCorrected            Code = "loan_amount_corrected"
	MsgTransactionPendingApproval     Code = "transaction_pending_approval"
	MsgTransactionCancelled           Code = "transaction_cancelled"
	MsgDepositProcessed               Code = "deposit_processed"
	MsgWithdrawalProcessed            Code = "withdrawal_processed"
	MsgTransferExecuted               Code = "transfer_executed"
	MsgJournalEntryRetrieved          Code = "journal_entry_retrieved"
	MsgJournalsListed                 Code = "journals_listed"
	MsgAccountStatementListed         Code = "account_statement_listed"
	MsgChartOfAccountsListed          Code = "chart_of_accounts_listed"
	MsgDepositPlaced                  Code = "deposit_placed"
	MsgDepositPreviewCalculated       Code = "deposit_preview_calculated"
	MsgDepositAccrued                 Code = "deposit_accrued"
	MsgDepositsRolledOver             Code = "deposits_rolled_over"
	MsgDepositWithdrawn               Code = "deposit_withdrawn"
	MsgDepositRetrieved               Code = "deposit_retrieved"
	MsgDepositsListed                 Code = "deposits_listed"
	MsgCurrentBusinessDate            Code = "current_business_date"
	MsgEODExecuted                    Code = "e_o_d_executed"
	MsgEOMExecuted                    Code = "e_o_m_executed"
	MsgEOYExecuted                    Code = "e_o_y_executed"
	MsgEODDefinitions                 Code = "e_o_d_definitions"
	MsgEODDefinitionsUpdated          Code = "e_o_d_definitions_updated"
	MsgEODRunHistory                  Code = "e_o_d_run_history"
	MsgBranchList                     Code = "branch_list"
	MsgBranchCreated                  Code = "branch_created"
	MsgOrgUnitList                    Code = "org_unit_list"
	MsgOrgUnitCreated                 Code = "org_unit_created"
	MsgOrgUnitParentUpdated           Code = "org_unit_parent_updated"
	MsgOrgUnitListFailed              Code = "org_unit_list_failed"
	MsgProductList                    Code = "product_list"
	MsgProductDetail                  Code = "product_detail"
	MsgProductParamsUpdated           Code = "product_params_updated"
	MsgCollateralRecorded             Code = "collateral_recorded"
	MsgCollateralList                 Code = "collateral_list"
	MsgActiveCollateralSummary        Code = "active_collateral_summary"
	MsgCollateral                     Code = "collateral"
	MsgPPAPCalculated                 Code = "p_p_a_p_calculated"
	MsgPPAPPreview                    Code = "p_p_a_p_preview"
	MsgCKPNPPKAComparison             Code = "c_k_p_n_p_p_k_a_comparison"
	MsgCKPNCalculated                 Code = "c_k_p_n_calculated"
	MsgPPKAPlacementCalculated        Code = "p_p_k_a_placement_calculated"
	MsgReportTrialBalanceGenerated    Code = "report_trial_balance_generated"
	MsgReportBalanceSheetGenerated    Code = "report_balance_sheet_generated"
	MsgReportIncomeStatementGenerated Code = "report_income_statement_generated"
	MsgReportCashFlowGenerated        Code = "report_cash_flow_generated"
	MsgDueObligationsListed           Code = "due_obligations_listed"
	MsgOJKReportDefinitions           Code = "o_j_k_report_definitions"
	MsgOJKCOAMapping                  Code = "o_j_k_c_o_a_mapping"
	MsgOJKMappingDecisionSaved        Code = "o_j_k_mapping_decision_saved"
	MsgOJKSLIKCheckCompleted          Code = "o_j_k_s_l_i_k_check_completed"
	MsgDukcapilVerificationCompleted  Code = "dukcapil_verification_completed"
	MsgAuditLog                       Code = "audit_log"
	MsgTransactionLimitsEffective     Code = "transaction_limits_effective"
	MsgPermissionCatalog              Code = "permission_catalog"
	MsgPermissionChangeSubmitted      Code = "permission_change_submitted"
	MsgPendingMakerChecker            Code = "pending_maker_checker"
	MsgRequestApproved                Code = "request_approved"
	MsgRequestRejected                Code = "request_rejected"
	MsgCollectionProcessed            Code = "collection_processed"
	MsgCustomerRegistered             Code = "customer_registered"
	MsgCustomerRetrieved              Code = "customer_retrieved"
	MsgCustomersListed                Code = "customers_listed"
	MsgStaffCreated                   Code = "staff_created"
	MsgStaffRetrieved                 Code = "staff_retrieved"
	MsgStaffUpdated                   Code = "staff_updated"
	MsgStaffListed                    Code = "staff_listed"
	MsgPasswordChanged                Code = "password_changed"
	MsgPasswordReset                  Code = "password_reset"
	MsgBankProfile                    Code = "bank_profile"
	MsgBankProfileUpdated             Code = "bank_profile_updated"
	MsgHealthOK                       Code = "health_o_k"
	MsgAuthenticationRequired         Code = "authentication_required"
	MsgAccessTokenMissing             Code = "access_token_missing"
	MsgAccessTokenExpired             Code = "access_token_expired"
	MsgInvalidAccessToken             Code = "invalid_access_token"
	MsgForbiddenInsufficientRole      Code = "forbidden_insufficient_role"
	MsgCSRFMissing                    Code = "c_s_r_f_missing"
	MsgCSRFInvalid                    Code = "c_s_r_f_invalid"
	MsgTooManyLoginAttempts           Code = "too_many_login_attempts"
	MsgInternalError                  Code = "internal_error"
	MsgLoginFailed                    Code = "login_failed"
	MsgTokenRefreshFailed             Code = "token_refresh_failed"
	MsgRefreshTokenRequired           Code = "refresh_token_required"
	MsgUsernamePasswordRequired       Code = "username_password_required"
	MsgInvalidRequestBody             Code = "invalid_request_body"
	MsgInvalidRequestBodyWithErr      Code = "invalid_request_body_with_err"
	MsgInvalidLoanID                  Code = "invalid_loan_i_d"
	MsgInvalidCustomerID              Code = "invalid_customer_i_d"
	MsgInvalidStaffUserID             Code = "invalid_staff_user_i_d"
	MsgInvalidRequestID               Code = "invalid_request_i_d"
	MsgInvalidDepositID               Code = "invalid_deposit_i_d"
	MsgInvalidProductID               Code = "invalid_product_i_d"
	MsgInvalidCollateralID            Code = "invalid_collateral_i_d"
	MsgInvalidCashCollateralAccountID Code = "invalid_cash_collateral_account_i_d"
	MsgCustomerIDRequired             Code = "customer_i_d_required"
	MsgProductIDRequired              Code = "product_i_d_required"
	MsgAccountNumberRequired          Code = "account_number_required"
	MsgAccountNumberAmountRequired    Code = "account_number_amount_required"
	MsgTransferFieldsRequired         Code = "transfer_fields_required"
	MsgReferenceRequired              Code = "reference_required"
	MsgReferenceNumberRequired        Code = "reference_number_required"
	MsgTransactionReferenceRequired   Code = "transaction_reference_required"
	MsgReceiptNumberRequired          Code = "receipt_number_required"
	MsgFullNameIDCardRequired         Code = "full_name_i_d_card_required"
	MsgStaffFieldsRequired            Code = "staff_fields_required"
	MsgNIKRequired                    Code = "n_i_k_required"
	MsgInstallmentNoRequired          Code = "installment_no_required"
	MsgNewPasswordRequired            Code = "new_password_required"
	MsgMethodInvalid                  Code = "method_invalid"
	MsgCancelReasonRequired           Code = "cancel_reason_required"
	MsgCorrectionReasonRequired       Code = "correction_reason_required"
	MsgNewAmountMustBePositive        Code = "new_amount_must_be_positive"
	MsgProductCodeRequired            Code = "product_code_required"
	MsgDaysInvalid                    Code = "days_invalid"
	MsgDateFormatInvalid              Code = "date_format_invalid"
	MsgNJOPDateInvalid                Code = "n_j_o_p_date_invalid"
	MsgInvalidTimeRange               Code = "invalid_time_range"
	MsgBranchListFailed               Code = "branch_list_failed"
	MsgTransactionLimitsUnavailable   Code = "transaction_limits_unavailable"
	MsgProductParamsPayloadInvalid    Code = "product_params_payload_invalid"
	MsgBankProfilePayloadInvalid      Code = "bank_profile_payload_invalid"
	MsgForbiddenRolePermission        Code = "forbidden_role_permission"
)

// codeList memuat seluruh kode. Uji katalog memastikan setiap konstanta punya
// terjemahan ID dan EN sehingga tidak ada kode yang jatuh ke teks fallback.
var codeList = []Code{
	MsgAccountOpened,
	MsgAccountRetrieved,
	MsgAccountReactivated,
	MsgAccountsListed,
	MsgLoginSuccessful,
	MsgTokenRefreshed,
	MsgLoggedOut,
	MsgCurrentUser,
	MsgAppInfo,
	MsgLoanApplicationSubmitted,
	MsgLoansRetrieved,
	MsgLoanRetrieved,
	MsgLoanApproved,
	MsgLoanRejected,
	MsgLoanDisbursed,
	MsgInstallmentRecorded,
	MsgLoanRestructured,
	MsgLoanWrittenOff,
	MsgLoanRecoveryRecorded,
	MsgLoanDisbursementCancelled,
	MsgLoanAmountCorrected,
	MsgTransactionPendingApproval,
	MsgTransactionCancelled,
	MsgDepositProcessed,
	MsgWithdrawalProcessed,
	MsgTransferExecuted,
	MsgJournalEntryRetrieved,
	MsgJournalsListed,
	MsgAccountStatementListed,
	MsgChartOfAccountsListed,
	MsgDepositPlaced,
	MsgDepositPreviewCalculated,
	MsgDepositAccrued,
	MsgDepositsRolledOver,
	MsgDepositWithdrawn,
	MsgDepositRetrieved,
	MsgDepositsListed,
	MsgCurrentBusinessDate,
	MsgEODExecuted,
	MsgEOMExecuted,
	MsgEOYExecuted,
	MsgEODDefinitions,
	MsgEODDefinitionsUpdated,
	MsgEODRunHistory,
	MsgBranchList,
	MsgBranchCreated,
	MsgOrgUnitList,
	MsgOrgUnitCreated,
	MsgOrgUnitParentUpdated,
	MsgOrgUnitListFailed,
	MsgProductList,
	MsgProductDetail,
	MsgProductParamsUpdated,
	MsgCollateralRecorded,
	MsgCollateralList,
	MsgActiveCollateralSummary,
	MsgCollateral,
	MsgPPAPCalculated,
	MsgPPAPPreview,
	MsgCKPNPPKAComparison,
	MsgCKPNCalculated,
	MsgPPKAPlacementCalculated,
	MsgReportTrialBalanceGenerated,
	MsgReportBalanceSheetGenerated,
	MsgReportIncomeStatementGenerated,
	MsgReportCashFlowGenerated,
	MsgDueObligationsListed,
	MsgOJKReportDefinitions,
	MsgOJKCOAMapping,
	MsgOJKMappingDecisionSaved,
	MsgOJKSLIKCheckCompleted,
	MsgDukcapilVerificationCompleted,
	MsgAuditLog,
	MsgTransactionLimitsEffective,
	MsgPermissionCatalog,
	MsgPermissionChangeSubmitted,
	MsgPendingMakerChecker,
	MsgRequestApproved,
	MsgRequestRejected,
	MsgCollectionProcessed,
	MsgCustomerRegistered,
	MsgCustomerRetrieved,
	MsgCustomersListed,
	MsgStaffCreated,
	MsgStaffRetrieved,
	MsgStaffUpdated,
	MsgStaffListed,
	MsgPasswordChanged,
	MsgPasswordReset,
	MsgBankProfile,
	MsgBankProfileUpdated,
	MsgHealthOK,
	MsgAuthenticationRequired,
	MsgAccessTokenMissing,
	MsgAccessTokenExpired,
	MsgInvalidAccessToken,
	MsgForbiddenInsufficientRole,
	MsgCSRFMissing,
	MsgCSRFInvalid,
	MsgTooManyLoginAttempts,
	MsgInternalError,
	MsgLoginFailed,
	MsgTokenRefreshFailed,
	MsgRefreshTokenRequired,
	MsgUsernamePasswordRequired,
	MsgInvalidRequestBody,
	MsgInvalidRequestBodyWithErr,
	MsgInvalidLoanID,
	MsgInvalidCustomerID,
	MsgInvalidStaffUserID,
	MsgInvalidRequestID,
	MsgInvalidDepositID,
	MsgInvalidProductID,
	MsgInvalidCollateralID,
	MsgInvalidCashCollateralAccountID,
	MsgCustomerIDRequired,
	MsgProductIDRequired,
	MsgAccountNumberRequired,
	MsgAccountNumberAmountRequired,
	MsgTransferFieldsRequired,
	MsgReferenceRequired,
	MsgReferenceNumberRequired,
	MsgTransactionReferenceRequired,
	MsgReceiptNumberRequired,
	MsgFullNameIDCardRequired,
	MsgStaffFieldsRequired,
	MsgNIKRequired,
	MsgInstallmentNoRequired,
	MsgNewPasswordRequired,
	MsgMethodInvalid,
	MsgCancelReasonRequired,
	MsgCorrectionReasonRequired,
	MsgNewAmountMustBePositive,
	MsgProductCodeRequired,
	MsgDaysInvalid,
	MsgDateFormatInvalid,
	MsgNJOPDateInvalid,
	MsgInvalidTimeRange,
	MsgBranchListFailed,
	MsgTransactionLimitsUnavailable,
	MsgProductParamsPayloadInvalid,
	MsgBankProfilePayloadInvalid,
	MsgForbiddenRolePermission,
}

// catalog memetakan kode ke terjemahan. ID adalah bahasa utama; EN wajib ada.
var catalog = map[Code]map[Lang]string{
	MsgAccountOpened: {
		ID: "rekening berhasil dibuka",
		EN: "account opened successfully",
	},
	MsgAccountRetrieved: {
		ID: "detail rekening",
		EN: "account retrieved",
	},
	MsgAccountReactivated: {
		ID: "rekening berhasil diaktifkan kembali",
		EN: "account reactivated successfully",
	},
	MsgAccountsListed: {
		ID: "daftar rekening",
		EN: "accounts listed",
	},
	MsgLoginSuccessful: {
		ID: "login berhasil",
		EN: "login successful",
	},
	MsgTokenRefreshed: {
		ID: "token diperbarui",
		EN: "token refreshed",
	},
	MsgLoggedOut: {
		ID: "berhasil keluar",
		EN: "logged out successfully",
	},
	MsgCurrentUser: {
		ID: "pengguna saat ini",
		EN: "current user",
	},
	MsgAppInfo: {
		ID: "identitas aplikasi",
		EN: "application identity",
	},
	MsgLoanApplicationSubmitted: {
		ID: "pengajuan kredit berhasil dikirim",
		EN: "loan application submitted successfully",
	},
	MsgLoansRetrieved: {
		ID: "daftar kredit",
		EN: "loans retrieved",
	},
	MsgLoanRetrieved: {
		ID: "detail kredit",
		EN: "loan retrieved",
	},
	MsgLoanApproved: {
		ID: "pengajuan kredit disetujui",
		EN: "loan application approved",
	},
	MsgLoanRejected: {
		ID: "pengajuan kredit ditolak",
		EN: "loan application rejected",
	},
	MsgLoanDisbursed: {
		ID: "kredit berhasil dicairkan ke rekening nasabah",
		EN: "loan disbursed to customer account successfully",
	},
	MsgInstallmentRecorded: {
		ID: "pembayaran angsuran berhasil dicatat",
		EN: "installment payment recorded successfully",
	},
	MsgLoanRestructured: {
		ID: "kredit berhasil direstrukturisasi sesuai aturan OJK",
		EN: "loan restructured successfully according to OJK rules",
	},
	MsgLoanWrittenOff: {
		ID: "kredit berhasil dihapus buku",
		EN: "loan written off (hapus buku) successfully",
	},
	MsgLoanRecoveryRecorded: {
		ID: "pembayaran pemulihan kredit hapus buku berhasil dicatat",
		EN: "written-off loan recovery payment recorded successfully",
	},
	MsgLoanDisbursementCancelled: {
		ID: "pencairan kredit berhasil dibatalkan",
		EN: "loan disbursement cancelled successfully",
	},
	MsgLoanAmountCorrected: {
		ID: "nominal kredit berhasil dikoreksi",
		EN: "loan amount corrected successfully",
	},
	MsgTransactionPendingApproval: {
		ID: "transaksi menunggu persetujuan pejabat berwenang",
		EN: "transaction pending approval by an authorized officer",
	},
	MsgTransactionCancelled: {
		ID: "transaksi dibatalkan",
		EN: "transaction cancelled",
	},
	MsgDepositProcessed: {
		ID: "setoran berhasil diproses",
		EN: "deposit processed successfully",
	},
	MsgWithdrawalProcessed: {
		ID: "penarikan berhasil diproses",
		EN: "withdrawal processed successfully",
	},
	MsgTransferExecuted: {
		ID: "transfer berhasil dijalankan",
		EN: "transfer executed successfully",
	},
	MsgJournalEntryRetrieved: {
		ID: "detail jurnal",
		EN: "journal entry retrieved",
	},
	MsgJournalsListed: {
		ID: "daftar jurnal",
		EN: "journals listed",
	},
	MsgAccountStatementListed: {
		ID: "mutasi rekening",
		EN: "account statement listed",
	},
	MsgChartOfAccountsListed: {
		ID: "daftar bagan akun",
		EN: "chart of accounts listed",
	},
	MsgDepositPlaced: {
		ID: "deposito berhasil ditempatkan",
		EN: "deposit placed successfully",
	},
	MsgDepositPreviewCalculated: {
		ID: "pratinjau deposito dihitung",
		EN: "deposit preview calculated",
	},
	MsgDepositAccrued: {
		ID: "bunga deposito berhasil diakru",
		EN: "deposit accrued successfully",
	},
	MsgDepositsRolledOver: {
		ID: "deposito diperpanjang otomatis (ARO)",
		EN: "deposits rolled over (ARO)",
	},
	MsgDepositWithdrawn: {
		ID: "deposito berhasil dicairkan",
		EN: "deposit withdrawn successfully",
	},
	MsgDepositRetrieved: {
		ID: "detail deposito",
		EN: "deposit retrieved",
	},
	MsgDepositsListed: {
		ID: "daftar deposito",
		EN: "deposits listed",
	},
	MsgCurrentBusinessDate: {
		ID: "tanggal buku sistem saat ini",
		EN: "current system business date retrieved",
	},
	MsgEODExecuted: {
		ID: "End of Day (EOD) berhasil dijalankan. Tanggal sistem dimajukan.",
		EN: "End of Day (EOD) executed successfully. System date advanced.",
	},
	MsgEOMExecuted: {
		ID: "Proses batch End of Month (EOM) berhasil dijalankan.",
		EN: "End of Month (EOM) batch process executed successfully.",
	},
	MsgEOYExecuted: {
		ID: "End of Year (EOY) Tutup Buku Akhir Tahun berhasil diselesaikan.",
		EN: "End of Year (EOY) Tutup Buku Akhir Tahun completed successfully.",
	},
	MsgEODDefinitions: {
		ID: "definisi langkah EOD",
		EN: "EOD step definitions",
	},
	MsgEODDefinitionsUpdated: {
		ID: "definisi langkah EOD diperbarui",
		EN: "EOD step definitions updated",
	},
	MsgEODRunHistory: {
		ID: "riwayat langkah EOD",
		EN: "EOD step run history",
	},
	MsgBranchList: {
		ID: "daftar cabang",
		EN: "list of branches",
	},
	MsgBranchCreated: {
		ID: "cabang dibuat",
		EN: "branch created",
	},
	MsgOrgUnitList: {
		ID: "daftar unit organisasi",
		EN: "list of organizational units",
	},
	MsgOrgUnitCreated: {
		ID: "unit organisasi dibuat",
		EN: "organizational unit created",
	},
	MsgOrgUnitParentUpdated: {
		ID: "atasan unit diperbarui",
		EN: "unit supervisor updated",
	},
	MsgOrgUnitListFailed: {
		ID: "gagal memuat unit organisasi",
		EN: "failed to load organizational units",
	},
	MsgProductList: {
		ID: "daftar produk",
		EN: "list of products",
	},
	MsgProductDetail: {
		ID: "detail produk",
		EN: "product detail",
	},
	MsgProductParamsUpdated: {
		ID: "parameter produk diperbarui",
		EN: "product parameters updated",
	},
	MsgCollateralRecorded: {
		ID: "agunan tercatat",
		EN: "collateral recorded",
	},
	MsgCollateralList: {
		ID: "daftar agunan kredit",
		EN: "list of loan collaterals",
	},
	MsgActiveCollateralSummary: {
		ID: "rekap agunan aktif",
		EN: "active collateral summary",
	},
	MsgCollateral: {
		ID: "agunan",
		EN: "collateral",
	},
	MsgPPAPCalculated: {
		ID: "perhitungan PPAP harian selesai",
		EN: "daily PPAP calculation completed",
	},
	MsgPPAPPreview: {
		ID: "pratinjau PPAP",
		EN: "PPAP preview",
	},
	MsgCKPNPPKAComparison: {
		ID: "perbandingan CKPN dan PPKA",
		EN: "CKPN and PPKA comparison",
	},
	MsgCKPNCalculated: {
		ID: "perhitungan CKPN selesai",
		EN: "CKPN calculation completed",
	},
	MsgPPKAPlacementCalculated: {
		ID: "perhitungan PPKA penempatan pada bank lain (Pasal 23 POJK 1/2024)",
		EN: "PPKA calculation for placements at other banks (Article 23 POJK 1/2024)",
	},
	MsgReportTrialBalanceGenerated: {
		ID: "laporan Neraca Saldo dihasilkan",
		EN: "Trial Balance report generated",
	},
	MsgReportBalanceSheetGenerated: {
		ID: "laporan Neraca dihasilkan",
		EN: "Balance Sheet report generated",
	},
	MsgReportIncomeStatementGenerated: {
		ID: "laporan Laba Rugi dihasilkan",
		EN: "Income Statement report generated",
	},
	MsgReportCashFlowGenerated: {
		ID: "laporan Arus Kas dihasilkan",
		EN: "Cash Flow report generated",
	},
	MsgDueObligationsListed: {
		ID: "daftar kewajiban jatuh tempo",
		EN: "due obligations listed",
	},
	MsgOJKReportDefinitions: {
		ID: "definisi laporan OJK",
		EN: "OJK report definitions",
	},
	MsgOJKCOAMapping: {
		ID: "pemetaan COA ke pos OJK",
		EN: "COA mapping to OJK line items",
	},
	MsgOJKMappingDecisionSaved: {
		ID: "keputusan pemetaan tersimpan",
		EN: "mapping decision saved",
	},
	MsgOJKSLIKCheckCompleted: {
		ID: "pemeriksaan debitur OJK SLIK selesai",
		EN: "OJK SLIK debtor check completed",
	},
	MsgDukcapilVerificationCompleted: {
		ID: "verifikasi NIK Dukcapil selesai",
		EN: "Dukcapil NIK verification completed",
	},
	MsgAuditLog: {
		ID: "log audit",
		EN: "audit log",
	},
	MsgTransactionLimitsEffective: {
		ID: "batas transaksi efektif",
		EN: "effective transaction limits",
	},
	MsgPermissionCatalog: {
		ID: "katalog izin",
		EN: "permission catalog",
	},
	MsgPermissionChangeSubmitted: {
		ID: "perubahan izin dikirim untuk persetujuan",
		EN: "permission change submitted for approval",
	},
	MsgPendingMakerChecker: {
		ID: "permintaan maker-checker yang menunggu",
		EN: "pending maker-checker requests",
	},
	MsgRequestApproved: {
		ID: "permintaan berhasil disetujui",
		EN: "request approved successfully",
	},
	MsgRequestRejected: {
		ID: "permintaan ditolak",
		EN: "request rejected",
	},
	MsgCollectionProcessed: {
		ID: "penagihan lapangan berhasil diproses",
		EN: "mobile collection processed successfully",
	},
	MsgCustomerRegistered: {
		ID: "nasabah berhasil didaftarkan",
		EN: "customer registered successfully",
	},
	MsgCustomerRetrieved: {
		ID: "detail nasabah",
		EN: "customer retrieved",
	},
	MsgCustomersListed: {
		ID: "daftar nasabah",
		EN: "customers listed",
	},
	MsgStaffCreated: {
		ID: "pengguna staf berhasil dibuat",
		EN: "staff user created successfully",
	},
	MsgStaffRetrieved: {
		ID: "detail pengguna staf",
		EN: "staff user retrieved",
	},
	MsgStaffUpdated: {
		ID: "pengguna staf diperbarui",
		EN: "staff user updated",
	},
	MsgStaffListed: {
		ID: "daftar pengguna staf",
		EN: "staff users listed",
	},
	MsgPasswordChanged: {
		ID: "kata sandi berhasil diubah",
		EN: "password changed successfully",
	},
	MsgPasswordReset: {
		ID: "kata sandi berhasil diatur ulang",
		EN: "password reset successfully",
	},
	MsgBankProfile: {
		ID: "profil bank",
		EN: "bank profile",
	},
	MsgBankProfileUpdated: {
		ID: "profil bank diperbarui",
		EN: "bank profile updated",
	},
	MsgHealthOK: {
		ID: "Core Banking API sehat",
		EN: "Core Banking API is healthy",
	},
	MsgAuthenticationRequired: {
		ID: "autentikasi diperlukan",
		EN: "authentication required",
	},
	MsgAccessTokenMissing: {
		ID: "token akses tidak ada atau tidak valid",
		EN: "missing or invalid access token",
	},
	MsgAccessTokenExpired: {
		ID: "token akses kedaluwarsa",
		EN: "access token expired",
	},
	MsgInvalidAccessToken: {
		ID: "token akses tidak valid",
		EN: "invalid access token",
	},
	MsgForbiddenInsufficientRole: {
		ID: "akses ditolak: peran tidak mencukupi",
		EN: "forbidden: insufficient role",
	},
	MsgCSRFMissing: {
		ID: "permintaan ditolak: token CSRF tidak ditemukan",
		EN: "request rejected: CSRF token not found",
	},
	MsgCSRFInvalid: {
		ID: "permintaan ditolak: token CSRF tidak valid",
		EN: "request rejected: invalid CSRF token",
	},
	MsgTooManyLoginAttempts: {
		ID: "terlalu banyak percobaan masuk, silakan coba lagi nanti",
		EN: "too many login attempts, please try again later",
	},
	MsgInternalError: {
		ID: "terjadi kesalahan internal, silakan coba lagi",
		EN: "an internal error occurred, please try again",
	},
	MsgLoginFailed: {
		ID: "login gagal",
		EN: "login failed",
	},
	MsgTokenRefreshFailed: {
		ID: "penyegaran token gagal",
		EN: "token refresh failed",
	},
	MsgRefreshTokenRequired: {
		ID: "refresh_token wajib diisi",
		EN: "refresh_token is required",
	},
	MsgUsernamePasswordRequired: {
		ID: "username dan password wajib diisi",
		EN: "username and password are required",
	},
	MsgInvalidRequestBody: {
		ID: "isi permintaan tidak valid",
		EN: "invalid request body",
	},
	MsgInvalidRequestBodyWithErr: {
		ID: "isi permintaan tidak valid: %s",
		EN: "invalid request body: %s",
	},
	MsgInvalidLoanID: {
		ID: "id kredit tidak valid",
		EN: "invalid loan id",
	},
	MsgInvalidCustomerID: {
		ID: "id nasabah tidak valid",
		EN: "invalid customer id",
	},
	MsgInvalidStaffUserID: {
		ID: "id pengguna staf tidak valid",
		EN: "invalid staff user id",
	},
	MsgInvalidRequestID: {
		ID: "id permintaan tidak valid",
		EN: "invalid request id",
	},
	MsgInvalidDepositID: {
		ID: "id deposito tidak valid",
		EN: "invalid deposit id",
	},
	MsgInvalidProductID: {
		ID: "id produk tidak valid",
		EN: "invalid product id",
	},
	MsgInvalidCollateralID: {
		ID: "id agunan tidak valid",
		EN: "invalid collateral id",
	},
	MsgInvalidCashCollateralAccountID: {
		ID: "id rekening agunan tunai tidak valid",
		EN: "invalid cash collateral account id",
	},
	MsgCustomerIDRequired: {
		ID: "customer_id wajib diisi",
		EN: "customer_id is required",
	},
	MsgProductIDRequired: {
		ID: "product_id wajib diisi",
		EN: "product_id is required",
	},
	MsgAccountNumberRequired: {
		ID: "nomor rekening wajib diisi",
		EN: "account number is required",
	},
	MsgAccountNumberAmountRequired: {
		ID: "account_number dan amount wajib diisi",
		EN: "account_number and amount are required",
	},
	MsgTransferFieldsRequired: {
		ID: "source_account_number, destination_account_number, dan amount wajib diisi",
		EN: "source_account_number, destination_account_number, and amount are required",
	},
	MsgReferenceRequired: {
		ID: "referensi wajib diisi",
		EN: "reference is required",
	},
	MsgReferenceNumberRequired: {
		ID: "nomor referensi wajib diisi",
		EN: "reference number is required",
	},
	MsgTransactionReferenceRequired: {
		ID: "reference transaksi wajib diisi",
		EN: "transaction reference is required",
	},
	MsgReceiptNumberRequired: {
		ID: "nomor kuitansi wajib diisi",
		EN: "receipt number is required",
	},
	MsgFullNameIDCardRequired: {
		ID: "full_name dan id_card_number wajib diisi",
		EN: "full_name and id_card_number are required",
	},
	MsgStaffFieldsRequired: {
		ID: "username, full_name, email, password, dan role wajib diisi",
		EN: "username, full_name, email, password, and role are required",
	},
	MsgNIKRequired: {
		ID: "NIK yang valid wajib diisi",
		EN: "valid nik is required",
	},
	MsgInstallmentNoRequired: {
		ID: "installment_no yang valid wajib diisi",
		EN: "valid installment_no is required",
	},
	MsgNewPasswordRequired: {
		ID: "new_password wajib diisi",
		EN: "new_password is required",
	},
	MsgMethodInvalid: {
		ID: "method harus ACCOUNT atau CASH",
		EN: "method must be ACCOUNT or CASH",
	},
	MsgCancelReasonRequired: {
		ID: "alasan pembatalan wajib diisi",
		EN: "cancellation reason is required",
	},
	MsgCorrectionReasonRequired: {
		ID: "alasan koreksi wajib diisi",
		EN: "correction reason is required",
	},
	MsgNewAmountMustBePositive: {
		ID: "nominal baru harus positif",
		EN: "new amount must be positive",
	},
	MsgProductCodeRequired: {
		ID: "kode produk wajib diisi",
		EN: "product code is required",
	},
	MsgDaysInvalid: {
		ID: "days tidak valid",
		EN: "days is invalid",
	},
	MsgDateFormatInvalid: {
		ID: "format tanggal harus YYYY-MM-DD",
		EN: "date format must be YYYY-MM-DD",
	},
	MsgNJOPDateInvalid: {
		ID: "format tanggal NJOP tidak dikenal; gunakan YYYY-MM-DD",
		EN: "unrecognized NJOP date format; use YYYY-MM-DD",
	},
	MsgInvalidTimeRange: {
		ID: "rentang waktu tidak sah: 'to' harus setelah 'from'",
		EN: "invalid time range: 'to' must be after 'from'",
	},
	MsgBranchListFailed: {
		ID: "gagal memuat daftar cabang",
		EN: "failed to load branch list",
	},
	MsgTransactionLimitsUnavailable: {
		ID: "layanan batas transaksi belum tersedia",
		EN: "transaction limit service is not available yet",
	},
	MsgProductParamsPayloadInvalid: {
		ID: "payload parameter produk tidak valid: %s",
		EN: "invalid product parameter payload: %s",
	},
	MsgBankProfilePayloadInvalid: {
		ID: "payload profil bank tidak valid: %s",
		EN: "invalid bank profile payload: %s",
	},
	MsgForbiddenRolePermission: {
		ID: "akses ditolak: peran Anda (%s) tidak memiliki izin '%s'",
		EN: "forbidden: your role (%s) does not have '%s' permission",
	},
}
