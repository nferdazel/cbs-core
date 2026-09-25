package i18n

// Code adalah kode pesan API. Nilainya internal dan stabil; handler memakai
// konstanta di bawah ini, bukan literal, sehingga pesan tidak lagi berserak.
type Code string

// Kode pesan API. Nama disusun dari makna pesan, bukan dari bahasa.
const (
	MsgAccountOpened                  Code = "account_opened"
	MsgAccountRetrieved               Code = "account_retrieved"
	MsgAccountReactivated             Code = "account_reactivated"
	MsgAccountFrozen                  Code = "account_frozen"
	MsgAccountUnfrozen                Code = "account_unfrozen"
	MsgAccountClosed                  Code = "account_closed"
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
	MsgCompoundJournalPosted          Code = "compound_journal_posted"
	MsgCompoundJournalLinesRequired   Code = "compound_journal_lines_required"
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
	MsgPPKAUmumCalculated             Code = "ppka_umum_calculated"
	MsgCKPNPPKAComparison             Code = "c_k_p_n_p_p_k_a_comparison"
	MsgCKPNCalculated                 Code = "c_k_p_n_calculated"
	MsgCKPNParametersStatus           Code = "c_k_p_n_parameters_status"
	MsgCKPNIndividualAssessment       Code = "ckpn_individual_assessment"
	MsgCKPNIndividualProjectionsSaved Code = "ckpn_individual_projections_saved"
	MsgPPKAPlacementCalculated        Code = "p_p_k_a_placement_calculated"
	MsgReportTrialBalanceGenerated    Code = "report_trial_balance_generated"
	MsgReportBalanceSheetGenerated    Code = "report_balance_sheet_generated"
	MsgReportIncomeStatementGenerated Code = "report_income_statement_generated"
	MsgReportCashFlowGenerated        Code = "report_cash_flow_generated"
	MsgDueObligationsListed           Code = "due_obligations_listed"
	MsgOJKReportDefinitions           Code = "o_j_k_report_definitions"
	MsgKPMMReport                     Code = "k_p_m_m_report"
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
	MsgCustomerUpdated                Code = "customer_updated"
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
	MsgMonitoringSnapshot             Code = "monitoring_snapshot"
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
	// Pesan galat domain yang sebelumnya masih berbahasa Inggris. Maknanya tetap
	// sama, hanya dipindahkan ke katalog agar kedua bahasa tersedia.
	MsgInvalidCredentials Code = "invalid_credentials"
	MsgInsufficientFunds  Code = "insufficient_funds"
	MsgSessionExpired     Code = "session_expired"

	// Kode pesan galat domain (domain.LocalizedError). Setiap sentinel yang tampil ke
	// pengguna menyatakan kode ini di dekat definisinya; handler menerjemahkan lewat
	// satu jalur, sehingga tidak ada daftar pemetaan kedua yang rapuh.
	MsgAccountNotFound                    Code = "account_not_found"
	MsgAccountDormant                     Code = "account_dormant"
	MsgAccountNotDormant                  Code = "account_not_dormant"
	MsgAccountNotFreezable                Code = "account_not_freezable"
	MsgAccountNotFrozen                   Code = "account_not_frozen"
	MsgAccountUnfreezeSameActor           Code = "account_unfreeze_same_actor"
	MsgAccountCloseBalance                Code = "account_close_balance"
	MsgAccountNotClosable                 Code = "account_not_closable"
	MsgInvalidBranchCode                  Code = "invalid_branch_code"
	MsgCrossBranchAccess                  Code = "cross_branch_access"
	MsgBankProfileNameRequired            Code = "bank_profile_name_required"
	MsgBankProfileNameTooShort            Code = "bank_profile_name_too_short"
	MsgBankProfileEmpty                   Code = "bank_profile_empty"
	MsgBankProfileFieldTooLong            Code = "bank_profile_field_too_long"
	MsgBankProfileNPWPInvalid             Code = "bank_profile_npwp_invalid"
	MsgBankProfilePhoneInvalid            Code = "bank_profile_phone_invalid"
	MsgOJKProfile                         Code = "ojk_profile"
	MsgOJKProfileUpdated                  Code = "ojk_profile_updated"
	MsgOJKProfilePayloadInvalid           Code = "ojk_profile_payload_invalid"
	MsgOJKProfileEmpty                    Code = "ojk_profile_empty"
	MsgOJKProfileFieldTooLong             Code = "ojk_profile_field_too_long"
	MsgOJKProfileEmailInvalid             Code = "ojk_profile_email_invalid"
	MsgOJKProfileWebsiteInvalid           Code = "ojk_profile_website_invalid"
	MsgOJKProfilePhoneInvalid             Code = "ojk_profile_phone_invalid"
	MsgOJKProfileAgentCountInvalid        Code = "ojk_profile_agent_count_invalid"
	MsgBranchNotFound                     Code = "branch_not_found"
	MsgBranchCodeExists                   Code = "branch_code_exists"
	MsgBranchNameRequired                 Code = "branch_name_required"
	MsgBranchHeadOfficeNotAllowed         Code = "branch_head_office_not_allowed"
	MsgOrgUnitCodeTooLong                 Code = "org_unit_code_too_long"
	MsgCustomerNotFound                   Code = "customer_not_found"
	MsgDuplicateIDCard                    Code = "duplicate_id_card"
	MsgDuplicateEmail                     Code = "duplicate_email"
	MsgCipherNotConfigured                Code = "cipher_not_configured"
	MsgLoanNotFound                       Code = "loan_not_found"
	MsgLoanAlreadyApprovedDomain          Code = "loan_already_approved"
	MsgWriteOffNotMacet                   Code = "write_off_not_macet"
	MsgWriteOffReserveIncomplete          Code = "write_off_reserve_incomplete"
	MsgWriteOffPartial                    Code = "write_off_partial"
	MsgWriteOffReasonRequired             Code = "write_off_reason_required"
	MsgWriteOffCollectionEffortsNeeded    Code = "write_off_collection_efforts_required"
	MsgRecoveryExceedsWriteOff            Code = "recovery_exceeds_write_off"
	MsgWriteOffAmountUnavailable          Code = "write_off_amount_unavailable"
	MsgMakerCheckerNotFound               Code = "maker_checker_not_found"
	MsgMakerCheckerNotPending             Code = "maker_checker_not_pending"
	MsgCannotSelfApprove                  Code = "cannot_self_approve"
	MsgNoExecutorForAction                Code = "no_executor_for_action"
	MsgProductNotFound                    Code = "product_not_found"
	MsgBagiHasilNisbahMissing             Code = "bagi_hasil_nisbah_missing"
	MsgBagiHasilProjectionMissing         Code = "bagi_hasil_projection_missing"
	MsgBagiHasilNisbahOutOfRange          Code = "bagi_hasil_nisbah_out_of_range"
	MsgBagiHasilProjectionOutOfRange      Code = "bagi_hasil_projection_out_of_range"
	MsgProductParamsEmpty                 Code = "product_params_empty"
	MsgProductRateNegative                Code = "product_rate_negative"
	MsgProductAdminFeeNegative            Code = "product_admin_fee_negative"
	MsgProductTaxRateNegative             Code = "product_tax_rate_negative"
	MsgProductPenaltyRateNegative         Code = "product_penalty_rate_negative"
	MsgProductMinAmountNegative           Code = "product_min_amount_negative"
	MsgProductMaxAmountNegative           Code = "product_max_amount_negative"
	MsgProductAmountRange                 Code = "product_amount_range"
	MsgProductAmountBelowMin              Code = "product_amount_below_min"
	MsgProductTermNegative                Code = "product_term_negative"
	MsgProductTermRange                   Code = "product_term_range"
	MsgPasswordExpired                    Code = "password_expired"
	MsgStaffRoleNotManageable             Code = "staff_role_not_manageable"
	MsgStaffAlreadyExists                 Code = "staff_already_exists"
	MsgStaffBranchRequired                Code = "staff_branch_required"
	MsgStaffBranchUnknown                 Code = "staff_branch_unknown"
	MsgLimitPerTransaction                Code = "limit_per_transaction"
	MsgLimitDaily                         Code = "limit_daily"
	MsgRequiresApproval                   Code = "requires_approval"
	MsgCKPNStalePPAP                      Code = "ckpn_stale_ppap"
	MsgCollateralWeightActivationRejected Code = "collateral_weight_activation_rejected"
	MsgCollateralWeightAssessment         Code = "collateral_weight_assessment"
	MsgCollateralWeightActivated          Code = "collateral_weight_activated"
	// MsgCollateralWeightCategories menyertai daftar kategori bobot agunan yang dibaca
	// dari basis data (bukan dari daftar di kode klien).
	MsgCollateralWeightCategories       Code = "collateral_weight_categories"
	MsgEODInProgress                    Code = "eod_in_progress"
	MsgProductAmountAboveMax            Code = "product_amount_above_max"
	MsgProductTermOutOfRange            Code = "product_term_out_of_range"
	MsgDisbursementAccountNotFound      Code = "disbursement_account_not_found"
	MsgDisbursementAccountNotOwned      Code = "disbursement_account_not_owned"
	MsgLoanAccountCOAMissing            Code = "loan_account_coa_missing"
	MsgLoanNotActive                    Code = "loan_not_active"
	MsgInstallmentNotFound              Code = "installment_not_found"
	MsgInstallmentAlreadyPaid           Code = "installment_already_paid"
	MsgDisbursementCancelReasonRequired Code = "disbursement_cancel_reason_required"
	MsgLoanCorrectionReasonRequired     Code = "loan_correction_reason_required"
	MsgRestructureOnlyActiveLoan        Code = "restructure_only_active_loan"
	MsgRestructureNewTermPositive       Code = "restructure_new_term_positive"
	MsgRecoveryAmountPositive           Code = "recovery_amount_positive"
	MsgCustomerNameRequired             Code = "customer_name_required"
	MsgSameAccountTransfer              Code = "same_account_transfer"
	MsgAccountNumberTaken               Code = "account_number_taken"

	// Galat validasi dinamis lanjutan (putaran i18n berikutnya): produk/rekening yang
	// tidak cocok, hapus buku, dan koreksi nominal.
	MsgNotLoanProduct               Code = "not_loan_product"
	MsgLoanProductMissing           Code = "loan_product_missing"
	MsgPaymentMethodUnknown         Code = "payment_method_unknown"
	MsgInstallmentAccountNotFound   Code = "installment_account_missing"
	MsgRecoveryAccountNotFound      Code = "recovery_account_missing"
	MsgWriteOffOnlyActive           Code = "write_off_only_active"
	MsgWriteOffNoPrincipal          Code = "write_off_no_principal"
	MsgWriteOffNotWrittenOff        Code = "write_off_not_written_off"
	MsgWriteOffMappingMissing       Code = "write_off_mapping_missing"
	MsgCorrectionBelowPaidPrincipal Code = "correction_below_paid_principal"
	MsgCorrectionBelowScheduled     Code = "correction_below_scheduled"
	MsgProductNotForSavings         Code = "product_not_for_savings"
	MsgProductInactive              Code = "product_inactive"
	MsgBranchInactive               Code = "branch_inactive"
	MsgCOAAccountNotFound           Code = "coa_account_not_found"
	MsgAROInstructionUnknown        Code = "aro_instruction_unknown"

	// Manajemen staf & kata sandi (sebelumnya berbaur Inggris/Indonesia).
	MsgStaffPasswordTooShort   Code = "staff_password_too_short"
	MsgStaffPasswordWeak       Code = "staff_password_weak"
	MsgStaffPrivilegedCreate   Code = "staff_privileged_create"
	MsgStaffPrivilegedRole     Code = "staff_privileged_role"
	MsgStaffCurrentPassword    Code = "staff_current_password_wrong"
	MsgStaffPasswordUnchanged  Code = "staff_password_unchanged"
	MsgStaffUseOwnPasswordFlow Code = "staff_use_own_password_flow"
	MsgStaffSelfDeactivate     Code = "staff_self_deactivate"
	MsgStaffPasswordOnlySelf   Code = "staff_password_only_self"

	// Validasi masukan operator pada alur yang diisi manusia.
	MsgNoUnpaidInstallment Code = "no_unpaid_installment"
	MsgCKPNValueNotDecimal Code = "ckpn_value_not_decimal"

	// Validasi NIK dan konfigurasi batas transaksi.
	MsgNIKTooShort        Code = "nik_too_short"
	MsgLimitConfigMissing Code = "limit_config_missing"
	MsgLimitConfigInvalid Code = "limit_config_invalid"

	// Validasi penagihan/collection mobile.
	MsgCKPNIndividualDisposalCostSaved Code = "ckpn_individual_disposal_cost_saved"
	MsgCKPNIndividualScan              Code = "ckpn_individual_scan"
	MsgCKPNIndividualEntryMarked       Code = "ckpn_individual_entry_marked"
	MsgCollectionTypeUnknown           Code = "collection_type_unknown"
	MsgCollectionLoanRefs              Code = "collection_loan_refs_required"
)

// codeList memuat seluruh kode. Uji katalog memastikan setiap konstanta punya
// terjemahan ID dan EN sehingga tidak ada kode yang jatuh ke teks fallback.
var codeList = []Code{
	MsgAccountOpened,
	MsgAccountRetrieved,
	MsgAccountReactivated,
	MsgAccountFrozen,
	MsgAccountUnfrozen,
	MsgAccountClosed,
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
	MsgCompoundJournalPosted,
	MsgCompoundJournalLinesRequired,
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
	MsgPPKAUmumCalculated,
	MsgCKPNPPKAComparison,
	MsgCKPNCalculated,
	MsgCKPNParametersStatus,
	MsgCKPNIndividualAssessment,
	MsgCKPNIndividualProjectionsSaved,
	MsgPPKAPlacementCalculated,
	MsgReportTrialBalanceGenerated,
	MsgReportBalanceSheetGenerated,
	MsgReportIncomeStatementGenerated,
	MsgReportCashFlowGenerated,
	MsgDueObligationsListed,
	MsgOJKReportDefinitions,
	MsgKPMMReport,
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
	MsgCustomerUpdated,
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
	MsgMonitoringSnapshot,
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
	MsgInvalidCredentials,
	MsgInsufficientFunds,
	MsgSessionExpired,
	MsgAccountNotFound,
	MsgAccountDormant,
	MsgAccountNotDormant,
	MsgAccountNotFreezable,
	MsgAccountNotFrozen,
	MsgAccountUnfreezeSameActor,
	MsgAccountCloseBalance,
	MsgAccountNotClosable,
	MsgInvalidBranchCode,
	MsgCrossBranchAccess,
	MsgBankProfileNameRequired,
	MsgBankProfileNameTooShort,
	MsgBankProfileEmpty,
	MsgBankProfileFieldTooLong,
	MsgBankProfileNPWPInvalid,
	MsgBankProfilePhoneInvalid,
	MsgOJKProfile,
	MsgOJKProfileUpdated,
	MsgOJKProfilePayloadInvalid,
	MsgOJKProfileEmpty,
	MsgOJKProfileFieldTooLong,
	MsgOJKProfileEmailInvalid,
	MsgOJKProfileWebsiteInvalid,
	MsgOJKProfilePhoneInvalid,
	MsgOJKProfileAgentCountInvalid,
	MsgBranchNotFound,
	MsgBranchCodeExists,
	MsgBranchNameRequired,
	MsgBranchHeadOfficeNotAllowed,
	MsgOrgUnitCodeTooLong,
	MsgCustomerNotFound,
	MsgDuplicateIDCard,
	MsgDuplicateEmail,
	MsgCipherNotConfigured,
	MsgLoanNotFound,
	MsgLoanAlreadyApprovedDomain,
	MsgWriteOffNotMacet,
	MsgWriteOffReserveIncomplete,
	MsgWriteOffPartial,
	MsgWriteOffReasonRequired,
	MsgWriteOffCollectionEffortsNeeded,
	MsgRecoveryExceedsWriteOff,
	MsgWriteOffAmountUnavailable,
	MsgMakerCheckerNotFound,
	MsgMakerCheckerNotPending,
	MsgCannotSelfApprove,
	MsgNoExecutorForAction,
	MsgProductNotFound,
	MsgBagiHasilNisbahMissing,
	MsgBagiHasilProjectionMissing,
	MsgBagiHasilNisbahOutOfRange,
	MsgBagiHasilProjectionOutOfRange,
	MsgProductParamsEmpty,
	MsgProductRateNegative,
	MsgProductAdminFeeNegative,
	MsgProductTaxRateNegative,
	MsgProductPenaltyRateNegative,
	MsgProductMinAmountNegative,
	MsgProductMaxAmountNegative,
	MsgProductAmountRange,
	MsgProductAmountBelowMin,
	MsgProductTermNegative,
	MsgProductTermRange,
	MsgPasswordExpired,
	MsgStaffRoleNotManageable,
	MsgStaffAlreadyExists,
	MsgStaffBranchRequired,
	MsgStaffBranchUnknown,
	MsgLimitPerTransaction,
	MsgLimitDaily,
	MsgRequiresApproval,
	MsgCKPNStalePPAP,
	MsgCollateralWeightActivationRejected,
	MsgCollateralWeightAssessment,
	MsgCollateralWeightActivated,
	MsgCollateralWeightCategories,
	MsgEODInProgress,
	MsgProductAmountAboveMax,
	MsgProductTermOutOfRange,
	MsgDisbursementAccountNotFound,
	MsgDisbursementAccountNotOwned,
	MsgLoanAccountCOAMissing,
	MsgLoanNotActive,
	MsgInstallmentNotFound,
	MsgInstallmentAlreadyPaid,
	MsgDisbursementCancelReasonRequired,
	MsgLoanCorrectionReasonRequired,
	MsgRestructureOnlyActiveLoan,
	MsgRestructureNewTermPositive,
	MsgRecoveryAmountPositive,
	MsgCustomerNameRequired,
	MsgSameAccountTransfer,
	MsgAccountNumberTaken,
	MsgNotLoanProduct,
	MsgLoanProductMissing,
	MsgPaymentMethodUnknown,
	MsgInstallmentAccountNotFound,
	MsgRecoveryAccountNotFound,
	MsgWriteOffOnlyActive,
	MsgWriteOffNoPrincipal,
	MsgWriteOffNotWrittenOff,
	MsgWriteOffMappingMissing,
	MsgCorrectionBelowPaidPrincipal,
	MsgCorrectionBelowScheduled,
	MsgProductNotForSavings,
	MsgProductInactive,
	MsgBranchInactive,
	MsgCOAAccountNotFound,
	MsgAROInstructionUnknown,
	MsgStaffPasswordTooShort,
	MsgStaffPasswordWeak,
	MsgStaffPrivilegedCreate,
	MsgStaffPrivilegedRole,
	MsgStaffCurrentPassword,
	MsgStaffPasswordUnchanged,
	MsgStaffUseOwnPasswordFlow,
	MsgStaffSelfDeactivate,
	MsgStaffPasswordOnlySelf,
	MsgNoUnpaidInstallment,
	MsgCKPNValueNotDecimal,
	MsgNIKTooShort,
	MsgLimitConfigMissing,
	MsgLimitConfigInvalid,
	MsgCKPNIndividualDisposalCostSaved,
	MsgCKPNIndividualScan,
	MsgCKPNIndividualEntryMarked,
	MsgCollectionTypeUnknown,
	MsgCollectionLoanRefs,
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
	MsgAccountFrozen: {
		ID: "pembekuan rekening berhasil",
		EN: "account frozen successfully",
	},
	MsgAccountUnfrozen: {
		ID: "pembekuan rekening berhasil dibatalkan",
		EN: "account unfrozen successfully",
	},
	MsgAccountClosed: {
		ID: "rekening berhasil ditutup",
		EN: "account closed successfully",
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
	MsgCompoundJournalPosted: {
		ID: "jurnal majemuk berhasil diposting",
		EN: "compound journal posted successfully",
	},
	MsgCompoundJournalLinesRequired: {
		ID: "jurnal majemuk memerlukan minimal dua baris",
		EN: "compound journal requires at least two lines",
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
	MsgPPKAUmumCalculated: {
		ID: "perhitungan PPKA umum",
		EN: "general PPKA calculation",
	},
	MsgCKPNPPKAComparison: {
		ID: "perbandingan CKPN dan PPKA",
		EN: "CKPN and PPKA comparison",
	},
	MsgCKPNCalculated: {
		ID: "perhitungan CKPN selesai",
		EN: "CKPN calculation completed",
	},
	MsgCKPNParametersStatus: {
		ID: "status parameter CKPN",
		EN: "CKPN parameter status",
	},
	MsgCKPNIndividualAssessment: {
		ID: "penilaian CKPN individual (DCF, mode bayangan)",
		EN: "individual CKPN assessment (DCF, shadow mode)",
	},
	MsgCKPNIndividualProjectionsSaved: {
		ID: "proyeksi arus kas CKPN individual tersimpan",
		EN: "individual CKPN cash flow projections saved",
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
	MsgKPMMReport: {
		ID: "laporan KPMM",
		EN: "KPMM report",
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
	MsgCustomerUpdated: {
		ID: "data nasabah berhasil diperbarui",
		EN: "customer data updated successfully",
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
	MsgMonitoringSnapshot: {
		ID: "potret kesehatan operasional",
		EN: "operational health snapshot",
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
	MsgInvalidCredentials: {
		ID: "nama pengguna atau kata sandi salah",
		EN: "invalid username or password",
	},
	MsgInsufficientFunds: {
		ID: "saldo tersedia tidak mencukupi",
		EN: "insufficient available balance",
	},
	MsgSessionExpired: {
		ID: "sesi telah berakhir",
		EN: "session has expired",
	},
	MsgAccountNotFound: {
		ID: "rekening tidak ditemukan",
		EN: "account not found",
	},
	MsgAccountDormant: {
		ID: "rekening dormant: nasabah harus melakukan reaktivasi di cabang",
		EN: "account is dormant: the customer must reactivate it at the branch",
	},
	MsgAccountNotDormant: {
		ID: "rekening tidak berstatus dormant dan tidak dapat direaktivasi",
		EN: "account is not dormant and cannot be reactivated",
	},
	MsgAccountNotFrozen: {
		ID: "rekening tidak berstatus dibekukan dan tidak dapat dibatalkan pembekuannya",
		EN: "account is not frozen and cannot be unfrozen",
	},
	MsgAccountNotFreezable: {
		ID: "rekening tidak berstatus aktif dan tidak dapat dibekukan",
		EN: "account is not active and cannot be frozen",
	},
	MsgAccountUnfreezeSameActor: {
		ID: "pembekuan tidak dapat dibatalkan oleh pelaksana pembekuan yang sama",
		EN: "the freeze cannot be reversed by the same actor who froze the account",
	},
	MsgAccountCloseBalance: {
		ID: "rekening tidak dapat ditutup: masih ada saldo, saldo tersedia, atau dana tertahan",
		EN: "account cannot be closed: balance, available balance, or held funds remain",
	},
	MsgAccountNotClosable: {
		ID: "rekening tidak dapat ditutup pada status saat ini",
		EN: "account cannot be closed in its current status",
	},
	MsgInvalidBranchCode: {
		ID: "kode cabang harus 3 digit angka",
		EN: "branch code must be 3 digits",
	},
	MsgCrossBranchAccess: {
		ID: "akses lintas cabang ditolak: data berada di cabang lain",
		EN: "cross-branch access denied: the data belongs to another branch",
	},
	MsgBankProfileNameRequired: {
		ID: "nama bank wajib diisi",
		EN: "bank name is required",
	},
	MsgBankProfileNameTooShort: {
		ID: "nama bank terlalu pendek (minimal 3 karakter)",
		EN: "bank name is too short (minimum 3 characters)",
	},
	MsgBankProfileEmpty: {
		ID: "tidak ada bidang profil bank yang dikirim",
		EN: "no bank profile field was submitted",
	},
	MsgBankProfileFieldTooLong: {
		ID: "nilai identitas bank terlalu panjang",
		EN: "bank identity value is too long",
	},
	MsgBankProfileNPWPInvalid: {
		ID: "NPWP harus 15 atau 16 digit angka (titik/tanda hubung/spasi sebagai pemisah diperbolehkan)",
		EN: "NPWP must be 15 or 16 digits (dots, hyphens, or spaces as separators are allowed)",
	},
	MsgBankProfilePhoneInvalid: {
		ID: "nomor telepon hanya boleh berisi angka, spasi, dan tanda + - ( ) .",
		EN: "phone number may only contain digits, spaces, and the + - ( ) . characters",
	},
	MsgOJKProfile: {
		ID: "profil OJK",
		EN: "OJK profile",
	},
	MsgOJKProfileUpdated: {
		ID: "profil OJK diperbarui",
		EN: "OJK profile updated",
	},
	MsgOJKProfilePayloadInvalid: {
		ID: "isi permintaan profil OJK tidak valid: %s",
		EN: "invalid OJK profile request body: %s",
	},
	MsgOJKProfileEmpty: {
		ID: "tidak ada bidang profil OJK yang dikirim",
		EN: "no OJK profile field was submitted",
	},
	MsgOJKProfileFieldTooLong: {
		ID: "nilai profil OJK terlalu panjang",
		EN: "OJK profile value is too long",
	},
	MsgOJKProfileEmailInvalid: {
		ID: "alamat surel harus memuat tanda @",
		EN: "email address must contain the @ sign",
	},
	MsgOJKProfileWebsiteInvalid: {
		ID: "situs web harus diawali http:// atau https://",
		EN: "website must start with http:// or https://",
	},
	MsgOJKProfilePhoneInvalid: {
		ID: "nomor telepon hanya boleh berisi angka, spasi, dan tanda + - ( ) .",
		EN: "phone number may only contain digits, spaces, and the + - ( ) . characters",
	},
	MsgOJKProfileAgentCountInvalid: {
		ID: "jumlah agen Laku Pandai harus berupa angka",
		EN: "Laku Pandai agent count must be a number",
	},
	MsgBranchNotFound: {
		ID: "cabang tidak ditemukan",
		EN: "branch not found",
	},
	MsgBranchCodeExists: {
		ID: "kode cabang sudah terpakai",
		EN: "branch code is already used",
	},
	MsgBranchNameRequired: {
		ID: "nama cabang wajib diisi",
		EN: "branch name is required",
	},
	MsgBranchHeadOfficeNotAllowed: {
		ID: "kantor pusat tidak dapat dibuat lewat API; is_head_office harus false",
		EN: "head office cannot be created through the API; is_head_office must be false",
	},
	MsgOrgUnitCodeTooLong: {
		ID: "kode unit organisasi terlalu panjang: maksimal 32 karakter",
		EN: "organization unit code is too long: at most 32 characters",
	},
	MsgCustomerNotFound: {
		ID: "nasabah tidak ditemukan",
		EN: "customer not found",
	},
	MsgDuplicateIDCard: {
		ID: "NIK sudah terdaftar",
		EN: "ID card number is already registered",
	},
	MsgDuplicateEmail: {
		ID: "email sudah terdaftar",
		EN: "email is already registered",
	},
	MsgCipherNotConfigured: {
		ID: "kunci enkripsi data nasabah belum dikonfigurasi",
		EN: "customer data encryption key is not configured",
	},
	MsgLoanNotFound: {
		ID: "pengajuan kredit tidak ditemukan",
		EN: "loan application not found",
	},
	MsgLoanAlreadyApprovedDomain: {
		ID: "kredit sudah disetujui atau ditolak",
		EN: "loan has already been approved or rejected",
	},
	MsgWriteOffNotMacet: {
		ID: "hapus buku hanya dapat dilakukan atas kredit berkualitas Macet (kolektibilitas 5)",
		EN: "write-off may only be applied to loans classified as Loss (collectibility 5)",
	},
	MsgWriteOffReserveIncomplete: {
		ID: "hapus buku memerlukan cadangan/penyisihan 100% atas kredit",
		EN: "write-off requires a 100% reserve/provision for the loan",
	},
	MsgWriteOffPartial: {
		ID: "hapus buku sebagian dilarang; hapus buku harus atas seluruh eksposur kredit",
		EN: "partial write-off is prohibited; write-off must cover the entire loan exposure",
	},
	MsgWriteOffReasonRequired: {
		ID: "dasar pertimbangan hapus buku wajib diisi",
		EN: "the rationale for the write-off is required",
	},
	MsgWriteOffCollectionEffortsNeeded: {
		ID: "upaya penagihan terdokumentasi wajib diisi sebelum hapus buku",
		EN: "documented collection efforts are required before write-off",
	},
	MsgRecoveryExceedsWriteOff: {
		ID: "akumulasi pemulihan melebihi nilai hapus buku kredit",
		EN: "accumulated recovery exceeds the written-off amount of the loan",
	},
	MsgWriteOffAmountUnavailable: {
		ID: "nilai hapus buku kredit belum tersimpan, pemulihan tidak dapat dibatasi",
		EN: "the loan's written-off amount is not stored; recovery cannot be limited",
	},
	MsgMakerCheckerNotFound: {
		ID: "permintaan maker-checker tidak ditemukan",
		EN: "maker-checker request not found",
	},
	MsgMakerCheckerNotPending: {
		ID: "permintaan maker-checker sudah diproses",
		EN: "maker-checker request has already been processed",
	},
	MsgCannotSelfApprove: {
		ID: "pembuat permintaan tidak boleh menyetujui permintaannya sendiri",
		EN: "the requester may not approve their own request",
	},
	MsgNoExecutorForAction: {
		ID: "tidak ada eksekutor untuk jenis aksi maker-checker ini",
		EN: "no executor exists for this maker-checker action type",
	},
	MsgProductNotFound: {
		ID: "produk tidak ditemukan",
		EN: "product not found",
	},
	MsgBagiHasilNisbahMissing: {
		ID: "nisbah bagi hasil produk (profit_sharing_ratio) belum diisi; bank harus mengisi nisbah bagi hasil produk terlebih dahulu",
		EN: "the product's profit-sharing ratio (profit_sharing_ratio) has not been set; the bank must set it first",
	},
	MsgBagiHasilProjectionMissing: {
		ID: "proyeksi pendapatan usaha produk (projected_revenue_rate_annual) belum diisi; bank harus mengisi proyeksi pendapatan tahunan pembiayaan bagi hasil terlebih dahulu",
		EN: "the product's projected business revenue (projected_revenue_rate_annual) has not been set; the bank must set the annual projected revenue for profit-sharing financing first",
	},
	MsgBagiHasilNisbahOutOfRange: {
		ID: "nisbah bagi hasil produk (profit_sharing_ratio) harus lebih dari 0 dan maksimal 1; nisbah dinyatakan sebagai pecahan, mis. 0,4 untuk 40%",
		EN: "the product's profit-sharing ratio (profit_sharing_ratio) must be greater than 0 and at most 1; express it as a fraction, e.g. 0.4 for 40%",
	},
	MsgBagiHasilProjectionOutOfRange: {
		ID: "proyeksi pendapatan usaha produk (projected_revenue_rate_annual) harus lebih dari 0 dan maksimal 100; nilai dinyatakan dalam persen per tahun, mis. 12 untuk 12%",
		EN: "the product's projected business revenue (projected_revenue_rate_annual) must be greater than 0 and at most 100; the value is a percentage per year, e.g. 12 for 12%",
	},
	MsgProductParamsEmpty: {
		ID: "tidak ada parameter produk yang diubah; sertakan minimal satu parameter",
		EN: "no product parameter was changed; provide at least one parameter",
	},
	MsgProductRateNegative: {
		ID: "suku bunga/margin tahunan (rate_annual) tidak boleh negatif; nilai dinyatakan dalam persen per tahun",
		EN: "the annual interest rate/margin (rate_annual) must not be negative; the value is a percentage per year",
	},
	MsgProductAdminFeeNegative: {
		ID: "biaya administrasi (admin_fee) tidak boleh negatif; nilai dinyatakan dalam rupiah",
		EN: "the administration fee (admin_fee) must not be negative; the value is in rupiah",
	},
	MsgProductTaxRateNegative: {
		ID: "tarif pajak (tax_rate) tidak boleh negatif; nilai dinyatakan dalam persen",
		EN: "the tax rate (tax_rate) must not be negative; the value is a percentage",
	},
	MsgProductPenaltyRateNegative: {
		ID: "tarif penalti penarikan dini (early_withdrawal_penalty_rate) tidak boleh negatif; nilai dinyatakan dalam persen per tahun",
		EN: "the early withdrawal penalty rate must not be negative; the value is a percentage per year",
	},
	MsgProductMinAmountNegative: {
		ID: "batas plafon minimum (min_amount) tidak boleh negatif; nilai dinyatakan dalam rupiah",
		EN: "the minimum ceiling (min_amount) must not be negative; the value is in rupiah",
	},
	MsgProductMaxAmountNegative: {
		ID: "batas plafon maksimum (max_amount) tidak boleh negatif; nilai dinyatakan dalam rupiah",
		EN: "the maximum ceiling (max_amount) must not be negative; the value is in rupiah",
	},
	MsgProductAmountRange: {
		ID: "batas plafon maksimum (max_amount) harus 0 (tanpa batas) atau lebih besar/sama dengan minimum (min_amount); nilai dalam rupiah",
		EN: "the maximum ceiling (max_amount) must be 0 (no limit) or greater than/equal to the minimum (min_amount); the value is in rupiah",
	},
	MsgProductAmountBelowMin: {
		ID: "nominal di bawah minimum produk",
		EN: "amount is below the product minimum",
	},
	MsgProductTermNegative: {
		ID: "tenor minimum/maksimum (min_term_months/max_term_months) tidak boleh negatif; nilai dinyatakan dalam bulan",
		EN: "the minimum/maximum tenor (min_term_months/max_term_months) must not be negative; the value is in months",
	},
	MsgProductTermRange: {
		ID: "tenor minimum (min_term_months) tidak boleh melebihi tenor maksimum (max_term_months); nilai dinyatakan dalam bulan",
		EN: "the minimum tenor (min_term_months) must not exceed the maximum tenor (max_term_months); the value is in months",
	},
	MsgPasswordExpired: {
		ID: "password kedaluwarsa, ganti password terlebih dahulu",
		EN: "password has expired; change it first",
	},
	MsgStaffRoleNotManageable: {
		ID: "peran Anda tidak berwenang mengubah akun ini",
		EN: "your role is not authorized to modify this account",
	},
	MsgStaffAlreadyExists: {
		ID: "username, email, atau nomor pegawai staf sudah terpakai",
		EN: "the staff username, email, or employee number is already used",
	},
	MsgStaffBranchRequired: {
		ID: "kode cabang wajib diisi untuk staf operasional",
		EN: "a branch code is required for operational staff",
	},
	MsgStaffBranchUnknown: {
		ID: "kode cabang staf tidak terdaftar; pilih unit organisasi yang ada",
		EN: "the staff branch code is not registered; choose an existing organization unit",
	},
	MsgLimitPerTransaction: {
		ID: "nominal melebihi batas per transaksi",
		EN: "amount exceeds the per-transaction limit",
	},
	MsgLimitDaily: {
		ID: "akumulasi transaksi harian melebihi batas",
		EN: "accumulated daily transactions exceed the limit",
	},
	MsgRequiresApproval: {
		ID: "nominal melebihi ambang yang memerlukan persetujuan",
		EN: "amount exceeds the threshold requiring approval",
	},
	MsgCKPNStalePPAP: {
		ID: "perbandingan CKPN menolak PPKA dari tanggal bisnis yang tidak sama",
		EN: "CKPN comparison rejects PPKA from a different business date",
	},
	MsgCollateralWeightActivationRejected: {
		ID: "aktivasi bobot agunan ditolak",
		EN: "collateral weight activation rejected",
	},
	MsgCollateralWeightAssessment: {
		ID: "penilaian gerbang bobot agunan",
		EN: "collateral weight gate assessment",
	},
	MsgCollateralWeightActivated: {
		ID: "kategori bobot agunan diaktifkan",
		EN: "collateral weight category activated",
	},
	MsgCollateralWeightCategories: {
		ID: "daftar kategori bobot agunan",
		EN: "collateral weight category list",
	},
	MsgEODInProgress: {
		ID: "tutup hari sedang berjalan; tunggu sampai selesai",
		EN: "end of day is in progress; wait until it finishes",
	},
	MsgProductAmountAboveMax: {
		ID: "nominal di atas maksimum produk",
		EN: "amount is above the product maximum",
	},
	MsgProductTermOutOfRange: {
		ID: "jangka waktu di luar rentang produk",
		EN: "tenor is outside the product range",
	},
	MsgDisbursementAccountNotFound: {
		ID: "rekening pencairan tidak ditemukan",
		EN: "disbursement account not found",
	},
	MsgDisbursementAccountNotOwned: {
		ID: "rekening pencairan bukan milik nasabah yang mengajukan",
		EN: "the disbursement account does not belong to the applying customer",
	},
	MsgLoanAccountCOAMissing: {
		ID: "rekening nasabah tidak punya kode COA; jurnal tidak dapat dipetakan",
		EN: "the customer account has no COA code; the journal entry cannot be mapped",
	},
	MsgLoanNotActive: {
		ID: "kredit tidak dalam status aktif",
		EN: "the loan is not in active status",
	},
	MsgInstallmentNotFound: {
		ID: "jadwal angsuran tidak ditemukan",
		EN: "installment schedule not found",
	},
	MsgInstallmentAlreadyPaid: {
		ID: "angsuran ini sudah dibayar penuh",
		EN: "this installment has already been paid in full",
	},
	MsgDisbursementCancelReasonRequired: {
		ID: "alasan pembatalan pencairan wajib diisi",
		EN: "a reason for cancelling the disbursement is required",
	},
	MsgLoanCorrectionReasonRequired: {
		ID: "alasan koreksi nominal wajib diisi",
		EN: "a reason for the amount correction is required",
	},
	MsgRestructureOnlyActiveLoan: {
		ID: "hanya kredit aktif yang dapat direstrukturisasi",
		EN: "only active loans can be restructured",
	},
	MsgRestructureNewTermPositive: {
		ID: "jangka waktu baru harus positif",
		EN: "the new tenor must be positive",
	},
	MsgRecoveryAmountPositive: {
		ID: "nominal recovery harus positif",
		EN: "the recovery amount must be positive",
	},
	MsgCustomerNameRequired: {
		ID: "nama lengkap wajib diisi",
		EN: "full name is required",
	},
	MsgSameAccountTransfer: {
		ID: "rekening asal dan tujuan tidak boleh sama",
		EN: "the source and destination accounts must not be the same",
	},
	MsgAccountNumberTaken: {
		ID: "nomor rekening sudah terpakai, silakan coba lagi",
		EN: "the account number is already taken; please try again",
	},
	MsgNotLoanProduct: {
		ID: "produk %s bukan produk kredit/pembiayaan",
		EN: "product %s is not a loan/financing product",
	},
	MsgLoanProductMissing: {
		ID: "kredit tidak terhubung ke produk",
		EN: "the loan is not linked to a product",
	},
	MsgPaymentMethodUnknown: {
		ID: "metode pembayaran %q tidak dikenal",
		EN: "payment method %q is not recognised",
	},
	MsgInstallmentAccountNotFound: {
		ID: "rekening pembayaran angsuran tidak ditemukan",
		EN: "the instalment payment account was not found",
	},
	MsgRecoveryAccountNotFound: {
		ID: "rekening recovery tidak ditemukan",
		EN: "the recovery account was not found",
	},
	MsgWriteOffOnlyActive: {
		ID: "hanya kredit aktif yang dapat dihapus buku",
		EN: "only active loans can be written off",
	},
	MsgWriteOffNoPrincipal: {
		ID: "kredit tidak memiliki sisa pokok yang dapat dihapus buku",
		EN: "the loan has no outstanding principal to write off",
	},
	MsgWriteOffNotWrittenOff: {
		ID: "kredit tidak berstatus hapus buku",
		EN: "the loan is not in written-off status",
	},
	MsgWriteOffMappingMissing: {
		ID: "pemetaan hapus buku belum lengkap",
		EN: "the write-off product mapping is incomplete",
	},
	MsgCorrectionBelowPaidPrincipal: {
		ID: "nominal baru lebih kecil daripada pokok yang sudah dibayar",
		EN: "the new amount is lower than the principal already paid",
	},
	MsgCorrectionBelowScheduled: {
		ID: "nominal baru lebih kecil daripada pokok jadwal yang sudah dibayar",
		EN: "the new amount is lower than the scheduled principal already paid",
	},
	MsgProductNotForSavings: {
		ID: "produk %s tidak untuk pembukaan rekening simpanan",
		EN: "product %s is not for opening a savings account",
	},
	MsgProductInactive: {
		ID: "produk %s sedang tidak aktif",
		EN: "product %s is not active",
	},
	MsgBranchInactive: {
		ID: "cabang %s sedang tidak aktif",
		EN: "branch %s is not active",
	},
	MsgCOAAccountNotFound: {
		// Data (kode akun) berada di TENGAH pesan, jadi kedua bahasa memakai
		// placeholder dan diisi lewat Textf dengan argumen asli galat.
		ID: "akun COA %s tidak ditemukan",
		EN: "chart-of-accounts account %s was not found",
	},
	MsgAROInstructionUnknown: {
		ID: "instruksi ARO tidak dikenal",
		EN: "unknown ARO instruction",
	},
	MsgStaffPasswordTooShort: {
		ID: "kata sandi minimal 8 karakter",
		EN: "password must be at least 8 characters",
	},
	MsgStaffPasswordWeak: {
		ID: "kata sandi harus memuat huruf besar, huruf kecil, angka, dan karakter khusus",
		EN: "password must contain uppercase, lowercase, number, and special character",
	},
	MsgStaffPrivilegedCreate: {
		ID: "SUPERADMIN atau SYSTEM tidak dapat dibuat lewat endpoint ini",
		EN: "SUPERADMIN or SYSTEM cannot be created through this endpoint",
	},
	MsgStaffPrivilegedRole: {
		ID: "peran SUPERADMIN atau SYSTEM tidak dapat diberikan lewat pembaruan",
		EN: "the SUPERADMIN or SYSTEM role cannot be assigned via update",
	},
	MsgStaffCurrentPassword: {
		ID: "kata sandi saat ini salah",
		EN: "the current password is incorrect",
	},
	MsgStaffPasswordUnchanged: {
		ID: "kata sandi baru harus berbeda dari kata sandi saat ini",
		EN: "the new password must be different from the current password",
	},
	MsgStaffUseOwnPasswordFlow: {
		ID: "gunakan ubah kata sandi untuk akun sendiri",
		EN: "use change password for your own account",
	},
	MsgStaffSelfDeactivate: {
		ID: "akun sendiri tidak dapat dinonaktifkan",
		EN: "your own account cannot be deactivated",
	},
	MsgStaffPasswordOnlySelf: {
		ID: "ubah kata sandi hanya berlaku untuk akun sendiri",
		EN: "changing the password only applies to your own account",
	},
	MsgNoUnpaidInstallment: {
		ID: "tidak ada angsuran belum dibayar yang dapat disesuaikan",
		EN: "there is no unpaid instalment to adjust",
	},
	MsgCKPNValueNotDecimal: {
		// Nilai yang salah dikutip di tengah pesan, jadi memakai placeholder.
		ID: "nilai %q bukan angka desimal yang sah",
		EN: "value %q is not a valid decimal number",
	},
	MsgNIKTooShort: {
		ID: "NIK harus 16 digit",
		EN: "the national ID number (NIK) must be 16 digits",
	},
	MsgLimitConfigMissing: {
		ID: "konfigurasi batas %s belum diisi; batas transaksi tidak boleh memakai angka bawaan",
		EN: "the %s limit setting has not been filled in; transaction limits must not fall back to a default value",
	},
	MsgLimitConfigInvalid: {
		ID: "konfigurasi batas %s bernilai %q, bukan angka; perbaiki nilainya sebelum bertransaksi",
		EN: "the %s limit setting is %q, which is not a number; fix it before running transactions",
	},
	MsgCKPNIndividualDisposalCostSaved: {
		ID: "biaya pelepasan agunan tersimpan",
		EN: "collateral disposal cost saved",
	},
	MsgCKPNIndividualScan: {
		ID: "hasil pemindaian pintu masuk CKPN individual (usulan, belum menandai kredit)",
		EN: "individual CKPN entry scan result (proposal, loans not yet marked)",
	},
	MsgCKPNIndividualEntryMarked: {
		ID: "keputusan pintu masuk CKPN individual tercatat; required_ckpn tidak berubah",
		EN: "individual CKPN entry decision recorded; required_ckpn unchanged",
	},
	MsgCollectionTypeUnknown: {
		ID: "jenis penagihan tidak dikenal",
		EN: "unknown collection type",
	},
	MsgCollectionLoanRefs: {
		ID: "loan_id dan installment_no wajib diisi untuk penagihan angsuran kredit",
		EN: "loan_id and installment_no are required for loan instalment collection",
	},
}
