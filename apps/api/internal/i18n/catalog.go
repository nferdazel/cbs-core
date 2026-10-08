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
	MsgCKPNPABLAssessed               Code = "ckpn_pabl_assessed"
	MsgPABLCKPNRunCompleted           Code = "pabl_ckpn_run_completed"
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
	MsgServiceNotReady                Code = "service_not_ready"
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
	MsgInvalidPlacementID             Code = "invalid_placement_id"
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
	MsgAccountNotFound                      Code = "account_not_found"
	MsgAccountDormant                       Code = "account_dormant"
	MsgAccountNotDormant                    Code = "account_not_dormant"
	MsgAccountNotFreezable                  Code = "account_not_freezable"
	MsgAccountNotFrozen                     Code = "account_not_frozen"
	MsgAccountUnfreezeSameActor             Code = "account_unfreeze_same_actor"
	MsgAccountCloseBalance                  Code = "account_close_balance"
	MsgAccountNotClosable                   Code = "account_not_closable"
	MsgInvalidBranchCode                    Code = "invalid_branch_code"
	MsgCrossBranchAccess                    Code = "cross_branch_access"
	MsgBMPKBankWide                         Code = "bmpk_bank_wide"
	MsgBMPKInputInvalid                     Code = "bmpk_input_invalid"
	MsgBMPKNotFound                         Code = "bmpk_not_found"
	MsgBMPKCustomerIDInvalid                Code = "bmpk_customer_id_invalid"
	MsgBMPKMasterListed                     Code = "bmpk_master_listed"
	MsgBMPKRelatedPartySaved                Code = "bmpk_related_party_saved"
	MsgBMPKRelatedPartyDeleted              Code = "bmpk_related_party_deleted"
	MsgBMPKLimitSaved                       Code = "bmpk_limit_saved"
	MsgBMPKLimitDeleted                     Code = "bmpk_limit_deleted"
	MsgBankProfileNameRequired              Code = "bank_profile_name_required"
	MsgBankProfileNameTooShort              Code = "bank_profile_name_too_short"
	MsgBankProfileEmpty                     Code = "bank_profile_empty"
	MsgBankProfileFieldTooLong              Code = "bank_profile_field_too_long"
	MsgBankProfileNPWPInvalid               Code = "bank_profile_npwp_invalid"
	MsgBankProfilePhoneInvalid              Code = "bank_profile_phone_invalid"
	MsgOJKProfile                           Code = "ojk_profile"
	MsgOJKProfileUpdated                    Code = "ojk_profile_updated"
	MsgOJKProfilePayloadInvalid             Code = "ojk_profile_payload_invalid"
	MsgOJKProfileEmpty                      Code = "ojk_profile_empty"
	MsgOJKProfileFieldTooLong               Code = "ojk_profile_field_too_long"
	MsgOJKProfileEmailInvalid               Code = "ojk_profile_email_invalid"
	MsgOJKProfileWebsiteInvalid             Code = "ojk_profile_website_invalid"
	MsgOJKProfilePhoneInvalid               Code = "ojk_profile_phone_invalid"
	MsgOJKProfileAgentCountInvalid          Code = "ojk_profile_agent_count_invalid"
	MsgCKPNActivation                       Code = "ckpn_activation"
	MsgCKPNActivationUpdated                Code = "ckpn_activation_updated"
	MsgCKPNActivationEmpty                  Code = "ckpn_activation_empty"
	MsgCKPNActivationFractionInvalid        Code = "ckpn_activation_fraction_invalid"
	MsgCKPNActivationStatusInvalid          Code = "ckpn_activation_status_invalid"
	MsgCKPNActivationRatificationIncomplete Code = "ckpn_activation_ratification_incomplete"
	MsgCKPNActivationNotReady               Code = "ckpn_activation_not_ready"
	MsgCKPNActivationDateInvalid            Code = "ckpn_activation_date_invalid"
	// Pengaturan CKPN per penempatan pada bank lain (Form 05.00 kolom XII/XXI):
	// enam kunci ckpn.pabl.* sebelumnya hanya bisa diisi lewat SQL.
	MsgCKPNPABLActivation         Code = "ckpn_pabl_activation"
	MsgCKPNPABLActivationUpdated  Code = "ckpn_pabl_activation_updated"
	MsgCKPNPABLActivationEmpty    Code = "ckpn_pabl_activation_empty"
	MsgCKPNPABLActivationNotReady Code = "ckpn_pabl_activation_not_ready"
	// Sandi referensi/inline OJK (Form 06.00) yang sebelumnya hanya bisa diisi lewat
	// SQL/seed. Galat inline/referensi memuat nama kolom sebagai placeholder.
	MsgOJKLoanCodesUpdated     Code = "ojk_loan_codes_updated"
	MsgOJKLoanCodesEmpty       Code = "ojk_loan_codes_empty"
	MsgOJKInlineCodeInvalid    Code = "ojk_inline_code_invalid"
	MsgOJKReferenceCodeInvalid Code = "ojk_reference_code_invalid"
	// Sandi referensi/inline OJK per penempatan pada bank lain (Form 05.00) dan
	// validasi nominal/persentase/tanggal yang menyertainya.
	MsgOJKPlacementCodesUpdated Code = "ojk_placement_codes_updated"
	MsgOJKPlacementCodesEmpty   Code = "ojk_placement_codes_empty"
	MsgOJKPlacementCodesRead    Code = "ojk_placement_codes_read"
	MsgOJKPlacementsListed      Code = "ojk_placements_listed"
	// Daftar sandi referensi OJK Lampiran 02/03 untuk pemilih UI (baca-saja).
	MsgOJKCreditorGroupsListed Code = "ojk_creditor_groups_listed"
	MsgOJKRegenciesListed      Code = "ojk_regencies_listed"
	MsgOJKAmountInvalid        Code = "ojk_amount_invalid"
	MsgOJKPercentageInvalid    Code = "ojk_percentage_invalid"
	MsgOJKDateInvalid          Code = "ojk_date_invalid"
	// Data kelembagaan (jaringan kantor, direksi/komisaris, pejabat eksekutif) untuk
	// LAPORAN_KELEMBAGAAN. Galat berkode harus selaras dengan pesan domain (uji
	// errors_localized_test menegakkan ID/EN-nya).
	MsgKelembagaanReport        Code = "kelembagaan_report"
	MsgKelembagaanSaved         Code = "kelembagaan_saved"
	MsgKelembagaanDeleted       Code = "kelembagaan_deleted"
	MsgKelembagaanForm0011Saved Code = "kelembagaan_form00_11_saved"
	MsgKelembagaanIDInvalid     Code = "kelembagaan_id_invalid"
	MsgKelembagaanBankWide      Code = "kelembagaan_bank_wide"
	MsgKelembagaanInputInvalid  Code = "kelembagaan_input_invalid"
	MsgKelembagaanNotFound      Code = "kelembagaan_not_found"
	// Register rekening administratif (Form 01.01) pos komitmen/kontinjensi
	// off-balance. Galat berkode harus selaras dengan pesan domain (uji
	// errors_localized_test menegakkan ID/EN-nya).
	MsgOffBalanceReport       Code = "off_balance_report"
	MsgOffBalanceSaved        Code = "off_balance_saved"
	MsgOffBalanceDeleted      Code = "off_balance_deleted"
	MsgOffBalanceIDInvalid    Code = "off_balance_id_invalid"
	MsgOffBalanceBankWide     Code = "off_balance_bank_wide"
	MsgOffBalanceInputInvalid Code = "off_balance_input_invalid"
	MsgOffBalanceNotFound     Code = "off_balance_not_found"
	MsgOffBalanceItemsListed  Code = "off_balance_items_listed"
	// Register AYDA (Form 07.00) agunan yang diambil alih. Galat berkode harus selaras
	// dengan pesan domain (uji errors_localized_test menegakkan ID/EN-nya).
	MsgAYDAReport       Code = "ayda_report"
	MsgAYDASaved        Code = "ayda_saved"
	MsgAYDADeleted      Code = "ayda_deleted"
	MsgAYDAIDInvalid    Code = "ayda_id_invalid"
	MsgAYDABankWide     Code = "ayda_bank_wide"
	MsgAYDAInputInvalid Code = "ayda_input_invalid"
	MsgAYDANotFound     Code = "ayda_not_found"
	MsgAYDAItemsListed  Code = "ayda_items_listed"
	// Register pemegang saham BPR (Form 00.01) data kepemilikan. Galat berkode harus
	// selaras dengan pesan domain (uji errors_localized_test menegakkan ID/EN-nya).
	MsgKepemilikanReport       Code = "kepemilikan_report"
	MsgKepemilikanSaved        Code = "kepemilikan_saved"
	MsgKepemilikanDeleted      Code = "kepemilikan_deleted"
	MsgKepemilikanIDInvalid    Code = "kepemilikan_id_invalid"
	MsgKepemilikanBankWide     Code = "kepemilikan_bank_wide"
	MsgKepemilikanInputInvalid Code = "kepemilikan_input_invalid"
	MsgKepemilikanNotFound     Code = "kepemilikan_not_found"
	MsgKepemilikanItemsListed  Code = "kepemilikan_items_listed"
	// Register pinjaman yang diterima (Form 00.07). Galat berkode harus selaras dengan
	// pesan domain (uji errors_localized_test menegakkan ID/EN-nya).
	MsgPinjamanReport       Code = "pinjaman_report"
	MsgPinjamanSaved        Code = "pinjaman_saved"
	MsgPinjamanDeleted      Code = "pinjaman_deleted"
	MsgPinjamanIDInvalid    Code = "pinjaman_id_invalid"
	MsgPinjamanBankWide     Code = "pinjaman_bank_wide"
	MsgPinjamanInputInvalid Code = "pinjaman_input_invalid"
	MsgPinjamanNotFound     Code = "pinjaman_not_found"
	MsgPinjamanItemsListed  Code = "pinjaman_items_listed"
	// Register properti terbengkalai (Form 17.00). Galat berkode harus selaras dengan
	// pesan domain (uji errors_localized_test menegakkan ID/EN-nya). Nomor register
	// bersifat no reuse/no recycle sehingga penghapusan adalah soft-delete.
	MsgPropertiReport         Code = "properti_report"
	MsgPropertiSaved          Code = "properti_saved"
	MsgPropertiDeleted        Code = "properti_deleted"
	MsgPropertiIDInvalid      Code = "properti_id_invalid"
	MsgPropertiBankWide       Code = "properti_bank_wide"
	MsgPropertiInputInvalid   Code = "properti_input_invalid"
	MsgPropertiNotFound       Code = "properti_not_found"
	MsgPropertiNoRegisterUsed Code = "properti_no_register_used"
	MsgPropertiItemsListed    Code = "properti_items_listed"
	// Register aset tetap, inventaris, dan aset tidak berwujud (Form 08.00). Galat
	// berkode harus selaras dengan pesan domain (uji errors_localized_test menegakkan
	// ID/EN-nya). Form ini tidak mengatur no reuse/no recycle sehingga penghapusan
	// adalah DELETE fisik.
	MsgAsetTetapReport       Code = "aset_tetap_report"
	MsgAsetTetapSaved        Code = "aset_tetap_saved"
	MsgAsetTetapDeleted      Code = "aset_tetap_deleted"
	MsgAsetTetapIDInvalid    Code = "aset_tetap_id_invalid"
	MsgAsetTetapBankWide     Code = "aset_tetap_bank_wide"
	MsgAsetTetapInputInvalid Code = "aset_tetap_input_invalid"
	MsgAsetTetapNotFound     Code = "aset_tetap_not_found"
	MsgAsetTetapItemsListed  Code = "aset_tetap_items_listed"
	// Register penyertaan modal (Form 16.00). Galat berkode harus selaras dengan pesan
	// domain (uji errors_localized_test menegakkan ID/EN-nya). Nomor register bersifat
	// no reuse/no recycle sehingga penghapusan adalah soft-delete.
	MsgPenyertaanReport         Code = "penyertaan_report"
	MsgPenyertaanSaved          Code = "penyertaan_saved"
	MsgPenyertaanDeleted        Code = "penyertaan_deleted"
	MsgPenyertaanIDInvalid      Code = "penyertaan_id_invalid"
	MsgPenyertaanBankWide       Code = "penyertaan_bank_wide"
	MsgPenyertaanInputInvalid   Code = "penyertaan_input_invalid"
	MsgPenyertaanNotFound       Code = "penyertaan_not_found"
	MsgPenyertaanNoRegisterUsed Code = "penyertaan_no_register_used"
	MsgPenyertaanItemsListed    Code = "penyertaan_items_listed"

	// Register aset keuangan lainnya (Form 18.00). Galat berkode harus selaras dengan
	// pemetaan HTTP di ojk_report_handler.go.
	MsgAsetKeuanganReport         Code = "aset_keuangan_report"
	MsgAsetKeuanganSaved          Code = "aset_keuangan_saved"
	MsgAsetKeuanganDeleted        Code = "aset_keuangan_deleted"
	MsgAsetKeuanganIDInvalid      Code = "aset_keuangan_id_invalid"
	MsgAsetKeuanganBankWide       Code = "aset_keuangan_bank_wide"
	MsgAsetKeuanganInputInvalid   Code = "aset_keuangan_input_invalid"
	MsgAsetKeuanganNotFound       Code = "aset_keuangan_not_found"
	MsgAsetKeuanganNoRekeningUsed Code = "aset_keuangan_no_rekening_used"
	MsgAsetKeuanganItemsListed    Code = "aset_keuangan_items_listed"

	// Register surat berharga (Form 04.00). Galat berkode harus selaras dengan pemetaan
	// HTTP di ojk_report_handler.go.
	MsgSuratBerhargaReport       Code = "surat_berharga_report"
	MsgSuratBerhargaSaved        Code = "surat_berharga_saved"
	MsgSuratBerhargaDeleted      Code = "surat_berharga_deleted"
	MsgSuratBerhargaIDInvalid    Code = "surat_berharga_id_invalid"
	MsgSuratBerhargaBankWide     Code = "surat_berharga_bank_wide"
	MsgSuratBerhargaInputInvalid Code = "surat_berharga_input_invalid"
	MsgSuratBerhargaNotFound     Code = "surat_berharga_not_found"
	MsgSuratBerhargaItemsListed  Code = "surat_berharga_items_listed"

	// Register kas valuta asing (Form 03.00). Galat berkode harus selaras dengan pemetaan
	// HTTP di ojk_report_handler.go.
	MsgKasValasReport                     Code = "kas_valas_report"
	MsgKasValasSaved                      Code = "kas_valas_saved"
	MsgKasValasDeleted                    Code = "kas_valas_deleted"
	MsgKasValasIDInvalid                  Code = "kas_valas_id_invalid"
	MsgKasValasBankWide                   Code = "kas_valas_bank_wide"
	MsgKasValasInputInvalid               Code = "kas_valas_input_invalid"
	MsgKasValasNotFound                   Code = "kas_valas_not_found"
	MsgKasValasItemsListed                Code = "kas_valas_items_listed"
	MsgKreditSindikasiReport              Code = "kredit_sindikasi_report"
	MsgKreditSindikasiSaved               Code = "kredit_sindikasi_saved"
	MsgKreditSindikasiDeleted             Code = "kredit_sindikasi_deleted"
	MsgKreditSindikasiIDInvalid           Code = "kredit_sindikasi_id_invalid"
	MsgKreditSindikasiBankWide            Code = "kredit_sindikasi_bank_wide"
	MsgKreditSindikasiInputInvalid        Code = "kredit_sindikasi_input_invalid"
	MsgKreditSindikasiNotFound            Code = "kredit_sindikasi_not_found"
	MsgKreditSindikasiNoRekeningUsed      Code = "kredit_sindikasi_no_rekening_used"
	MsgAgunanInputInvalid                 Code = "agunan_input_invalid"
	MsgAgunanNotFound                     Code = "agunan_not_found"
	MsgAgunanRegisterUsed                 Code = "agunan_register_used"
	MsgAgunanBankWide                     Code = "agunan_bank_wide"
	MsgKreditSindikasiItemsListed         Code = "kredit_sindikasi_items_listed"
	MsgAgunanReport                       Code = "agunan_report"
	MsgAgunanItemsListed                  Code = "agunan_items_listed"
	MsgAgunanSaved                        Code = "agunan_saved"
	MsgAgunanIDInvalid                    Code = "agunan_id_invalid"
	MsgPihakTerkaitInputInvalid           Code = "pihak_terkait_input_invalid"
	MsgPihakTerkaitNotFound               Code = "pihak_terkait_not_found"
	MsgPihakTerkaitBankWide               Code = "pihak_terkait_bank_wide"
	MsgPihakTerkaitReport                 Code = "pihak_terkait_report"
	MsgPihakTerkaitItemsListed            Code = "pihak_terkait_items_listed"
	MsgPihakTerkaitSaved                  Code = "pihak_terkait_saved"
	MsgPihakTerkaitDeleted                Code = "pihak_terkait_deleted"
	MsgPihakTerkaitIDInvalid              Code = "pihak_terkait_id_invalid"
	MsgModalReport                        Code = "modal_report"
	MsgModalItemsListed                   Code = "modal_items_listed"
	MsgModalSaved                         Code = "modal_saved"
	MsgModalDeleted                       Code = "modal_deleted"
	MsgModalIDInvalid                     Code = "modal_id_invalid"
	MsgModalInputInvalid                  Code = "modal_input_invalid"
	MsgModalNotFound                      Code = "modal_not_found"
	MsgModalBankWide                      Code = "modal_bank_wide"
	MsgHapusBukuReport                    Code = "hapus_buku_report"
	MsgHapusBukuItemsListed               Code = "hapus_buku_items_listed"
	MsgHapusBukuSaved                     Code = "hapus_buku_saved"
	MsgHapusBukuDeleted                   Code = "hapus_buku_deleted"
	MsgHapusBukuIDInvalid                 Code = "hapus_buku_id_invalid"
	MsgHapusBukuInputInvalid              Code = "hapus_buku_input_invalid"
	MsgHapusBukuNotFound                  Code = "hapus_buku_not_found"
	MsgHapusBukuBankWide                  Code = "hapus_buku_bank_wide"
	MsgPihakLawanReport                   Code = "pihak_lawan_report"
	MsgPihakLawanItemsListed              Code = "pihak_lawan_items_listed"
	MsgPihakLawanSaved                    Code = "pihak_lawan_saved"
	MsgPihakLawanDeleted                  Code = "pihak_lawan_deleted"
	MsgPihakLawanIDInvalid                Code = "pihak_lawan_id_invalid"
	MsgPihakLawanInputInvalid             Code = "pihak_lawan_input_invalid"
	MsgPihakLawanNotFound                 Code = "pihak_lawan_not_found"
	MsgPihakLawanBankWide                 Code = "pihak_lawan_bank_wide"
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
	MsgStaffBookInvalid                   Code = "staff_book_invalid"
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
	MsgCKPNPABLAssessed,
	MsgPABLCKPNRunCompleted,
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
	MsgServiceNotReady,
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
	MsgInvalidPlacementID,
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
	MsgBMPKBankWide,
	MsgBMPKInputInvalid,
	MsgBMPKNotFound,
	MsgBMPKCustomerIDInvalid,
	MsgBMPKMasterListed,
	MsgBMPKRelatedPartySaved,
	MsgBMPKRelatedPartyDeleted,
	MsgBMPKLimitSaved,
	MsgBMPKLimitDeleted,
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
	MsgCKPNActivation,
	MsgCKPNActivationUpdated,
	MsgCKPNActivationEmpty,
	MsgCKPNActivationFractionInvalid,
	MsgCKPNActivationStatusInvalid,
	MsgCKPNActivationRatificationIncomplete,
	MsgCKPNActivationNotReady,
	MsgCKPNActivationDateInvalid,
	MsgCKPNPABLActivation,
	MsgCKPNPABLActivationUpdated,
	MsgCKPNPABLActivationEmpty,
	MsgCKPNPABLActivationNotReady,
	MsgOJKLoanCodesUpdated,
	MsgOJKLoanCodesEmpty,
	MsgOJKInlineCodeInvalid,
	MsgOJKReferenceCodeInvalid,
	MsgOJKPlacementCodesUpdated,
	MsgOJKPlacementCodesEmpty,
	MsgOJKPlacementCodesRead,
	MsgOJKPlacementsListed,
	MsgOJKCreditorGroupsListed,
	MsgOJKRegenciesListed,
	MsgOJKAmountInvalid,
	MsgOJKPercentageInvalid,
	MsgOJKDateInvalid,
	MsgKelembagaanReport,
	MsgKelembagaanSaved,
	MsgKelembagaanDeleted,
	MsgKelembagaanForm0011Saved,
	MsgKelembagaanIDInvalid,
	MsgKelembagaanBankWide,
	MsgKelembagaanInputInvalid,
	MsgKelembagaanNotFound,
	MsgOffBalanceReport,
	MsgOffBalanceSaved,
	MsgOffBalanceDeleted,
	MsgOffBalanceIDInvalid,
	MsgOffBalanceBankWide,
	MsgOffBalanceInputInvalid,
	MsgOffBalanceNotFound,
	MsgOffBalanceItemsListed,
	MsgAYDAReport,
	MsgAYDASaved,
	MsgAYDADeleted,
	MsgAYDAIDInvalid,
	MsgAYDABankWide,
	MsgAYDAInputInvalid,
	MsgAYDANotFound,
	MsgAYDAItemsListed,
	MsgKepemilikanReport,
	MsgKepemilikanSaved,
	MsgKepemilikanDeleted,
	MsgKepemilikanIDInvalid,
	MsgKepemilikanBankWide,
	MsgKepemilikanInputInvalid,
	MsgKepemilikanNotFound,
	MsgKepemilikanItemsListed,
	MsgPinjamanReport,
	MsgPinjamanSaved,
	MsgPinjamanDeleted,
	MsgPinjamanIDInvalid,
	MsgPinjamanBankWide,
	MsgPinjamanInputInvalid,
	MsgPinjamanNotFound,
	MsgPinjamanItemsListed,
	MsgPropertiReport,
	MsgPropertiSaved,
	MsgPropertiDeleted,
	MsgPropertiIDInvalid,
	MsgPropertiBankWide,
	MsgPropertiInputInvalid,
	MsgPropertiNotFound,
	MsgPropertiNoRegisterUsed,
	MsgPropertiItemsListed,
	MsgAsetTetapReport,
	MsgAsetTetapSaved,
	MsgAsetTetapDeleted,
	MsgAsetTetapIDInvalid,
	MsgAsetTetapBankWide,
	MsgAsetTetapInputInvalid,
	MsgAsetTetapNotFound,
	MsgAsetTetapItemsListed,
	MsgPenyertaanReport,
	MsgPenyertaanSaved,
	MsgPenyertaanDeleted,
	MsgPenyertaanIDInvalid,
	MsgPenyertaanBankWide,
	MsgPenyertaanInputInvalid,
	MsgPenyertaanNotFound,
	MsgPenyertaanNoRegisterUsed,
	MsgPenyertaanItemsListed,
	MsgAsetKeuanganReport,
	MsgAsetKeuanganSaved,
	MsgAsetKeuanganDeleted,
	MsgAsetKeuanganIDInvalid,
	MsgAsetKeuanganBankWide,
	MsgAsetKeuanganInputInvalid,
	MsgAsetKeuanganNotFound,
	MsgAsetKeuanganNoRekeningUsed,
	MsgAsetKeuanganItemsListed,
	MsgSuratBerhargaReport,
	MsgSuratBerhargaSaved,
	MsgSuratBerhargaDeleted,
	MsgSuratBerhargaIDInvalid,
	MsgSuratBerhargaBankWide,
	MsgSuratBerhargaInputInvalid,
	MsgSuratBerhargaNotFound,
	MsgSuratBerhargaItemsListed,
	MsgKasValasReport,
	MsgKasValasSaved,
	MsgKasValasDeleted,
	MsgKasValasIDInvalid,
	MsgKasValasBankWide,
	MsgKasValasInputInvalid,
	MsgKasValasNotFound,
	MsgKasValasItemsListed,
	MsgKreditSindikasiReport,
	MsgKreditSindikasiSaved,
	MsgKreditSindikasiDeleted,
	MsgKreditSindikasiIDInvalid,
	MsgKreditSindikasiBankWide,
	MsgKreditSindikasiInputInvalid,
	MsgKreditSindikasiNotFound,
	MsgKreditSindikasiNoRekeningUsed,
	MsgAgunanInputInvalid,
	MsgAgunanNotFound,
	MsgAgunanRegisterUsed,
	MsgAgunanBankWide,
	MsgKreditSindikasiItemsListed,
	MsgAgunanReport,
	MsgAgunanItemsListed,
	MsgAgunanSaved,
	MsgAgunanIDInvalid,
	MsgPihakTerkaitInputInvalid,
	MsgPihakTerkaitNotFound,
	MsgPihakTerkaitBankWide,
	MsgPihakTerkaitReport,
	MsgPihakTerkaitItemsListed,
	MsgPihakTerkaitSaved,
	MsgPihakTerkaitDeleted,
	MsgPihakTerkaitIDInvalid,
	MsgModalReport,
	MsgModalItemsListed,
	MsgModalSaved,
	MsgModalDeleted,
	MsgModalIDInvalid,
	MsgModalInputInvalid,
	MsgModalNotFound,
	MsgModalBankWide,
	MsgHapusBukuReport,
	MsgHapusBukuItemsListed,
	MsgHapusBukuSaved,
	MsgHapusBukuDeleted,
	MsgHapusBukuIDInvalid,
	MsgHapusBukuInputInvalid,
	MsgHapusBukuNotFound,
	MsgHapusBukuBankWide,
	MsgPihakLawanReport,
	MsgPihakLawanItemsListed,
	MsgPihakLawanSaved,
	MsgPihakLawanDeleted,
	MsgPihakLawanIDInvalid,
	MsgPihakLawanInputInvalid,
	MsgPihakLawanNotFound,
	MsgPihakLawanBankWide,
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
	MsgStaffBookInvalid,
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
	MsgCKPNPABLAssessed: {
		ID: "asesmen CKPN per penempatan pada bank lain tersimpan",
		EN: "CKPN assessment per placement at other banks saved",
	},
	MsgPABLCKPNRunCompleted: {
		ID: "mesin kolektif CKPN per penempatan pada bank lain selesai",
		EN: "collective CKPN run for placements at other banks completed",
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
	MsgServiceNotReady: {
		ID: "Core Banking API belum siap melayani",
		EN: "Core Banking API is not ready to serve",
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
	MsgInvalidPlacementID: {
		ID: "id penempatan tidak valid",
		EN: "invalid placement id",
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
	MsgBMPKBankWide: {
		ID: "laporan BMPK bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the BMPK report is bank-wide and can only be read by cross-branch roles",
	},
	MsgBMPKInputInvalid: {
		ID: "data pihak terkait/batas BMPK tidak valid",
		EN: "BMPK related party/limit data is invalid",
	},
	MsgBMPKNotFound: {
		ID: "data pihak terkait/batas BMPK tidak ditemukan",
		EN: "BMPK related party/limit data was not found",
	},
	MsgBMPKCustomerIDInvalid: {
		ID: "customer_id BMPK tidak valid",
		EN: "BMPK customer_id is invalid",
	},
	MsgBMPKMasterListed: {
		ID: "daftar pengaturan BMPK",
		EN: "BMPK master data listed",
	},
	MsgBMPKRelatedPartySaved: {
		ID: "pihak terkait BMPK berhasil disimpan",
		EN: "BMPK related party saved successfully",
	},
	MsgBMPKRelatedPartyDeleted: {
		ID: "pihak terkait BMPK berhasil dihapus",
		EN: "BMPK related party deleted successfully",
	},
	MsgBMPKLimitSaved: {
		ID: "batas BMPK berhasil disimpan",
		EN: "BMPK limit saved successfully",
	},
	MsgBMPKLimitDeleted: {
		ID: "batas BMPK berhasil dihapus",
		EN: "BMPK limit deleted successfully",
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
	MsgCKPNActivation: {
		ID: "pengaturan aktivasi CKPN",
		EN: "CKPN activation settings",
	},
	MsgCKPNActivationUpdated: {
		ID: "pengaturan aktivasi CKPN diperbarui",
		EN: "CKPN activation settings updated",
	},
	MsgCKPNActivationEmpty: {
		ID: "tidak ada bidang aktivasi CKPN yang dikirim",
		EN: "no CKPN activation field was submitted",
	},
	MsgCKPNActivationFractionInvalid: {
		ID: "fraksi PD/LGD harus angka 0..1 (satuan fraksi, bukan persen)",
		EN: "PD/LGD fraction must be a number 0..1 (fraction, not percent)",
	},
	MsgCKPNActivationStatusInvalid: {
		ID: "status parameter CKPN hanya boleh SEMENTARA atau FINAL",
		EN: "CKPN parameter status must be SEMENTARA or FINAL",
	},
	MsgCKPNActivationRatificationIncomplete: {
		ID: "status FINAL belum boleh disetel: bukti ratifikasi parameter CKPN belum lengkap",
		EN: "status FINAL cannot be set yet: CKPN parameter ratification evidence is incomplete",
	},
	MsgCKPNActivationNotReady: {
		ID: "CKPN belum boleh dinyalakan: masih ada penahan yang harus diselesaikan",
		EN: "CKPN cannot be enabled yet: blocking items remain",
	},
	MsgCKPNActivationDateInvalid: {
		ID: "tanggal harus format YYYY-MM-DD dan tidak boleh di masa depan",
		EN: "date must be YYYY-MM-DD and must not be in the future",
	},
	MsgCKPNPABLActivation: {
		ID: "pengaturan CKPN PABL",
		EN: "CKPN PABL settings",
	},
	MsgCKPNPABLActivationUpdated: {
		ID: "pengaturan CKPN PABL diperbarui",
		EN: "CKPN PABL settings updated",
	},
	MsgCKPNPABLActivationEmpty: {
		ID: "tidak ada bidang pengaturan CKPN PABL yang dikirim",
		EN: "no CKPN PABL settings field was submitted",
	},
	MsgCKPNPABLActivationNotReady: {
		ID: "CKPN PABL belum boleh dinyalakan: masih ada penahan yang harus diselesaikan",
		EN: "CKPN PABL cannot be enabled yet: blocking items remain",
	},
	MsgOJKLoanCodesUpdated: {
		ID: "sandi OJK kredit diperbarui",
		EN: "OJK loan codes updated",
	},
	MsgOJKLoanCodesEmpty: {
		ID: "tidak ada sandi OJK kredit yang dikirim",
		EN: "no OJK loan code was submitted",
	},
	MsgOJKInlineCodeInvalid: {
		ID: "sandi inline %s tidak termasuk daftar yang diizinkan",
		EN: "inline code %s is not in the allowed set",
	},
	MsgOJKReferenceCodeInvalid: {
		ID: "sandi referensi %s tidak ditemukan pada tabel referensi",
		EN: "reference code %s was not found in the reference table",
	},
	MsgOJKPlacementCodesUpdated: {
		ID: "sandi OJK penempatan diperbarui",
		EN: "OJK placement codes updated",
	},
	MsgOJKPlacementCodesEmpty: {
		ID: "tidak ada sandi OJK penempatan yang dikirim",
		EN: "no OJK placement code was submitted",
	},
	MsgOJKPlacementCodesRead: {
		ID: "sandi OJK penempatan",
		EN: "OJK placement codes",
	},
	MsgOJKPlacementsListed: {
		ID: "daftar penempatan OJK",
		EN: "OJK placements listed",
	},
	MsgOJKCreditorGroupsListed: {
		ID: "daftar sandi pihak lawan OJK (Lampiran 02)",
		EN: "OJK counterparty code list (Appendix 02)",
	},
	MsgOJKRegenciesListed: {
		ID: "daftar sandi kabupaten/kota OJK (Lampiran 03)",
		EN: "OJK regency/municipality code list (Appendix 03)",
	},
	MsgOJKAmountInvalid: {
		ID: "nominal %s tidak sah: harus angka dan tidak negatif",
		EN: "amount %s is invalid: must be a number and not negative",
	},
	MsgOJKPercentageInvalid: {
		ID: "persentase %s tidak sah: harus 0-100 dengan paling banyak 2 desimal",
		EN: "percentage %s is invalid: must be 0-100 with at most 2 decimals",
	},
	MsgOJKDateInvalid: {
		ID: "tanggal %s tidak sah: harus format YYYY-MM-DD",
		EN: "date %s is invalid: must be YYYY-MM-DD",
	},
	MsgKelembagaanReport: {
		ID: "laporan kelembagaan",
		EN: "institutional report",
	},
	MsgKelembagaanSaved: {
		ID: "data kelembagaan disimpan",
		EN: "institutional data saved",
	},
	MsgKelembagaanDeleted: {
		ID: "data kelembagaan dihapus",
		EN: "institutional data deleted",
	},

	MsgKelembagaanForm0011Saved: {
		ID: "kolom Form 00.11 kantor disimpan",
		EN: "office Form 00.11 columns saved",
	}, MsgKelembagaanIDInvalid: {
		ID: "id kelembagaan bukan UUID yang sah",
		EN: "institutional id is not a valid UUID",
	},
	MsgKelembagaanBankWide: {
		ID: "laporan kelembagaan bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the institutional report is bank-wide and can only be read by cross-branch roles",
	},
	MsgKelembagaanInputInvalid: {
		ID: "data kelembagaan tidak valid",
		EN: "institutional data is invalid",
	},
	MsgKelembagaanNotFound: {
		ID: "data kelembagaan tidak ditemukan",
		EN: "institutional data was not found",
	},
	MsgOffBalanceReport: {
		ID: "register rekening administratif",
		EN: "administrative accounts register",
	},
	MsgOffBalanceSaved: {
		ID: "data rekening administratif disimpan",
		EN: "administrative account data saved",
	},
	MsgOffBalanceDeleted: {
		ID: "data rekening administratif dihapus",
		EN: "administrative account data deleted",
	},
	MsgOffBalanceIDInvalid: {
		ID: "id rekening administratif bukan UUID yang sah",
		EN: "administrative account id is not a valid UUID",
	},
	MsgOffBalanceBankWide: {
		ID: "register rekening administratif bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the administrative accounts register is bank-wide and can only be read by cross-branch roles",
	},
	MsgOffBalanceInputInvalid: {
		ID: "data rekening administratif tidak valid",
		EN: "administrative account data is invalid",
	},
	MsgOffBalanceNotFound: {
		ID: "data rekening administratif tidak ditemukan",
		EN: "administrative account data was not found",
	},
	MsgOffBalanceItemsListed: {
		ID: "daftar pos rekening administratif",
		EN: "off-balance items listed",
	},
	MsgAYDAReport: {
		ID: "register AYDA",
		EN: "AYDA register",
	},
	MsgAYDASaved: {
		ID: "data register AYDA disimpan",
		EN: "AYDA register data saved",
	},
	MsgAYDADeleted: {
		ID: "data register AYDA dihapus",
		EN: "AYDA register data deleted",
	},
	MsgAYDAIDInvalid: {
		ID: "id register AYDA bukan UUID yang sah",
		EN: "AYDA register id is not a valid UUID",
	},
	MsgAYDABankWide: {
		ID: "register AYDA bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the AYDA register is bank-wide and can only be read by cross-branch roles",
	},
	MsgAYDAInputInvalid: {
		ID: "data register AYDA tidak valid",
		EN: "AYDA register data is invalid",
	},
	MsgAYDANotFound: {
		ID: "data register AYDA tidak ditemukan",
		EN: "AYDA register data was not found",
	},
	MsgAYDAItemsListed: {
		ID: "daftar baris register AYDA",
		EN: "AYDA register items listed",
	},
	MsgKepemilikanReport: {
		ID: "register kepemilikan BPR",
		EN: "BPR ownership register",
	},
	MsgKepemilikanSaved: {
		ID: "data register kepemilikan BPR disimpan",
		EN: "BPR ownership register data saved",
	},
	MsgKepemilikanDeleted: {
		ID: "data register kepemilikan BPR dihapus",
		EN: "BPR ownership register data deleted",
	},
	MsgKepemilikanIDInvalid: {
		ID: "id register kepemilikan BPR bukan UUID yang sah",
		EN: "BPR ownership register id is not a valid UUID",
	},
	MsgKepemilikanBankWide: {
		ID: "register kepemilikan BPR bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the BPR ownership register is bank-wide and can only be read by cross-branch roles",
	},
	MsgKepemilikanInputInvalid: {
		ID: "data register kepemilikan BPR tidak valid",
		EN: "BPR ownership register data is invalid",
	},
	MsgKepemilikanNotFound: {
		ID: "data register kepemilikan BPR tidak ditemukan",
		EN: "BPR ownership register data was not found",
	},
	MsgKepemilikanItemsListed: {
		ID: "daftar baris register kepemilikan BPR",
		EN: "BPR ownership register items listed",
	},
	MsgPinjamanReport: {
		ID: "register pinjaman yang diterima",
		EN: "received loan register",
	},
	MsgPinjamanSaved: {
		ID: "data register pinjaman yang diterima disimpan",
		EN: "received loan register data saved",
	},
	MsgPinjamanDeleted: {
		ID: "data register pinjaman yang diterima dihapus",
		EN: "received loan register data deleted",
	},
	MsgPinjamanIDInvalid: {
		ID: "id register pinjaman yang diterima bukan UUID yang sah",
		EN: "received loan register id is not a valid UUID",
	},
	MsgPinjamanBankWide: {
		ID: "register pinjaman yang diterima bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the received loan register is bank-wide and can only be read by cross-branch roles",
	},
	MsgPinjamanInputInvalid: {
		ID: "data register pinjaman yang diterima tidak valid",
		EN: "received loan register data is invalid",
	},
	MsgPinjamanNotFound: {
		ID: "data register pinjaman yang diterima tidak ditemukan",
		EN: "received loan register data was not found",
	},
	MsgPinjamanItemsListed: {
		ID: "daftar baris register pinjaman yang diterima",
		EN: "received loan register items listed",
	},
	MsgPropertiReport: {
		ID: "register properti terbengkalai",
		EN: "abandoned property register",
	},
	MsgPropertiSaved: {
		ID: "data register properti terbengkalai disimpan",
		EN: "abandoned property register data saved",
	},
	MsgPropertiDeleted: {
		ID: "data register properti terbengkalai dinonaktifkan",
		EN: "abandoned property register data deactivated",
	},
	MsgPropertiIDInvalid: {
		ID: "id register properti terbengkalai bukan UUID yang sah",
		EN: "abandoned property register id is not a valid UUID",
	},
	MsgPropertiBankWide: {
		ID: "register properti terbengkalai bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the abandoned property register is bank-wide and can only be read by cross-branch roles",
	},
	MsgPropertiInputInvalid: {
		ID: "data register properti terbengkalai tidak valid",
		EN: "abandoned property register data is invalid",
	},
	MsgPropertiNotFound: {
		ID: "data register properti terbengkalai tidak ditemukan",
		EN: "abandoned property register data was not found",
	},
	MsgPropertiNoRegisterUsed: {
		ID: "nomor register properti terbengkalai sudah pernah dipakai dan tidak boleh dipakai ulang",
		EN: "the abandoned property register number has already been used and cannot be reused",
	},
	MsgPropertiItemsListed: {
		ID: "daftar baris register properti terbengkalai",
		EN: "abandoned property register items listed",
	},
	MsgAsetTetapReport: {
		ID: "register aset tetap",
		EN: "fixed asset register",
	},
	MsgAsetTetapSaved: {
		ID: "data register aset tetap disimpan",
		EN: "fixed asset register data saved",
	},
	MsgAsetTetapDeleted: {
		ID: "data register aset tetap dihapus",
		EN: "fixed asset register data deleted",
	},
	MsgAsetTetapIDInvalid: {
		ID: "id register aset tetap bukan UUID yang sah",
		EN: "fixed asset register id is not a valid UUID",
	},
	MsgAsetTetapBankWide: {
		ID: "register aset tetap bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the fixed asset register is bank-wide and can only be read by cross-branch roles",
	},
	MsgAsetTetapInputInvalid: {
		ID: "data register aset tetap tidak valid",
		EN: "fixed asset register data is invalid",
	},
	MsgAsetTetapNotFound: {
		ID: "data register aset tetap tidak ditemukan",
		EN: "fixed asset register data was not found",
	},
	MsgAsetTetapItemsListed: {
		ID: "daftar baris register aset tetap",
		EN: "fixed asset register items listed",
	},
	MsgPenyertaanReport: {
		ID: "register penyertaan modal",
		EN: "equity participation register",
	},
	MsgPenyertaanSaved: {
		ID: "data register penyertaan modal disimpan",
		EN: "equity participation register data saved",
	},
	MsgPenyertaanDeleted: {
		ID: "data register penyertaan modal dihapus",
		EN: "equity participation register data deleted",
	},
	MsgPenyertaanIDInvalid: {
		ID: "id register penyertaan modal bukan UUID yang sah",
		EN: "equity participation register id is not a valid UUID",
	},
	MsgPenyertaanBankWide: {
		ID: "register penyertaan modal bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the equity participation register is bank-wide and can only be read by cross-branch roles",
	},
	MsgPenyertaanInputInvalid: {
		ID: "data register penyertaan modal tidak valid",
		EN: "equity participation register data is invalid",
	},
	MsgPenyertaanNotFound: {
		ID: "data register penyertaan modal tidak ditemukan",
		EN: "equity participation register data was not found",
	},
	MsgPenyertaanNoRegisterUsed: {
		ID: "nomor register penyertaan modal sudah pernah dipakai dan tidak boleh dipakai ulang",
		EN: "the equity participation register number has already been used and cannot be reused",
	},
	MsgPenyertaanItemsListed: {
		ID: "daftar baris register penyertaan modal",
		EN: "equity participation register items listed",
	},
	MsgAsetKeuanganReport: {
		ID: "register aset keuangan lainnya",
		EN: "other financial assets register",
	},
	MsgAsetKeuanganSaved: {
		ID: "data register aset keuangan lainnya disimpan",
		EN: "other financial assets register data saved",
	},
	MsgAsetKeuanganDeleted: {
		ID: "data register aset keuangan lainnya dihapus",
		EN: "other financial assets register data deleted",
	},
	MsgAsetKeuanganIDInvalid: {
		ID: "id register aset keuangan lainnya bukan UUID yang sah",
		EN: "other financial assets register id is not a valid UUID",
	},
	MsgAsetKeuanganBankWide: {
		ID: "register aset keuangan lainnya bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the other financial assets register is bank-wide and can only be read by cross-branch roles",
	},
	MsgAsetKeuanganInputInvalid: {
		ID: "data register aset keuangan lainnya tidak valid",
		EN: "other financial assets register data is invalid",
	},
	MsgAsetKeuanganNotFound: {
		ID: "data register aset keuangan lainnya tidak ditemukan",
		EN: "other financial assets register data was not found",
	},
	MsgAsetKeuanganNoRekeningUsed: {
		ID: "nomor rekening aset keuangan lainnya sudah pernah dipakai dan tidak boleh dipakai ulang",
		EN: "the other financial assets account number has already been used and cannot be reused",
	},
	MsgAsetKeuanganItemsListed: {
		ID: "daftar baris register aset keuangan lainnya",
		EN: "other financial assets register items listed",
	},
	MsgSuratBerhargaReport: {
		ID: "register surat berharga",
		EN: "marketable securities register",
	},
	MsgSuratBerhargaSaved: {
		ID: "data register surat berharga disimpan",
		EN: "marketable securities register data saved",
	},
	MsgSuratBerhargaDeleted: {
		ID: "data register surat berharga dihapus",
		EN: "marketable securities register data deleted",
	},
	MsgSuratBerhargaIDInvalid: {
		ID: "id register surat berharga bukan UUID yang sah",
		EN: "marketable securities register id is not a valid UUID",
	},
	MsgSuratBerhargaBankWide: {
		ID: "register surat berharga bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the marketable securities register is bank-wide and can only be read by cross-branch roles",
	},
	MsgSuratBerhargaInputInvalid: {
		ID: "data register surat berharga tidak valid",
		EN: "marketable securities register data is invalid",
	},
	MsgSuratBerhargaNotFound: {
		ID: "data register surat berharga tidak ditemukan",
		EN: "marketable securities register data was not found",
	},
	MsgSuratBerhargaItemsListed: {
		ID: "daftar baris register surat berharga",
		EN: "marketable securities register items listed",
	},
	MsgKasValasReport: {
		ID: "register kas valuta asing",
		EN: "foreign currency cash register",
	},
	MsgKasValasSaved: {
		ID: "data register kas valuta asing disimpan",
		EN: "foreign currency cash register data saved",
	},
	MsgKasValasDeleted: {
		ID: "data register kas valuta asing dihapus",
		EN: "foreign currency cash register data deleted",
	},
	MsgKasValasIDInvalid: {
		ID: "id register kas valuta asing bukan UUID yang sah",
		EN: "foreign currency cash register id is not a valid UUID",
	},
	MsgKasValasBankWide: {
		ID: "register kas valuta asing bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the foreign currency cash register is bank-wide and can only be read by cross-branch roles",
	},
	MsgKasValasInputInvalid: {
		ID: "data register kas valuta asing tidak valid",
		EN: "foreign currency cash register data is invalid",
	},
	MsgKasValasNotFound: {
		ID: "data register kas valuta asing tidak ditemukan",
		EN: "foreign currency cash register data was not found",
	},
	MsgKasValasItemsListed: {
		ID: "daftar baris register kas valuta asing",
		EN: "foreign currency cash register items listed",
	},
	MsgKreditSindikasiReport: {
		ID: "register kredit sindikasi",
		EN: "syndicated loan register",
	},
	MsgKreditSindikasiSaved: {
		ID: "data register kredit sindikasi disimpan",
		EN: "syndicated loan register data saved",
	},
	MsgKreditSindikasiDeleted: {
		ID: "data register kredit sindikasi dinonaktifkan",
		EN: "syndicated loan register data deactivated",
	},
	MsgKreditSindikasiIDInvalid: {
		ID: "id register kredit sindikasi bukan UUID yang sah",
		EN: "syndicated loan register id is not a valid UUID",
	},
	MsgKreditSindikasiBankWide: {
		ID: "register kredit sindikasi bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the syndicated loan register is bank-wide and can only be read by cross-branch roles",
	},
	MsgKreditSindikasiInputInvalid: {
		ID: "data register kredit sindikasi tidak valid",
		EN: "syndicated loan register data is invalid",
	},
	MsgKreditSindikasiNotFound: {
		ID: "data register kredit sindikasi tidak ditemukan",
		EN: "syndicated loan register data was not found",
	},
	MsgKreditSindikasiNoRekeningUsed: {
		ID: "nomor rekening sindikasi sudah pernah dipakai dan tidak boleh dipakai ulang",
		EN: "the syndicated loan account number has been used and cannot be reused",
	},
	MsgAgunanInputInvalid: {
		ID: "data agunan Form 06.01 tidak valid",
		EN: "collateral Form 06.01 data is invalid",
	},
	MsgAgunanNotFound: {
		ID: "data agunan tidak ditemukan",
		EN: "collateral data was not found",
	},
	MsgAgunanRegisterUsed: {
		ID: "kode register/nomor agunan sudah dipakai agunan lain dan tidak boleh dipakai ulang",
		EN: "the collateral register number has been used and cannot be reused",
	},
	MsgAgunanBankWide: {
		ID: "daftar agunan bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the collateral list is bank-wide and can only be read by cross-branch roles",
	},
	MsgKreditSindikasiItemsListed: {
		ID: "daftar baris register kredit sindikasi",
		EN: "syndicated loan register items listed",
	},
	MsgAgunanReport: {
		ID: "daftar agunan Form 06.01",
		EN: "collateral list Form 06.01",
	},
	MsgAgunanItemsListed: {
		ID: "daftar agunan untuk pengisian Form 06.01",
		EN: "collateral list for Form 06.01 entry",
	},
	MsgAgunanSaved: {
		ID: "kolom Form 06.01 agunan disimpan",
		EN: "collateral Form 06.01 columns saved",
	},
	MsgAgunanIDInvalid: {
		ID: "id agunan bukan UUID yang sah",
		EN: "collateral id is not a valid UUID",
	},
	MsgPihakTerkaitInputInvalid: {
		ID: "data pihak terkait Form 00.05 tidak valid",
		EN: "related party Form 00.05 data is invalid",
	},
	MsgPihakTerkaitNotFound: {
		ID: "data pihak terkait tidak ditemukan",
		EN: "related party data was not found",
	},
	MsgPihakTerkaitBankWide: {
		ID: "register pihak terkait bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the related party register is bank-wide and can only be read by cross-branch roles",
	},
	MsgPihakTerkaitReport: {
		ID: "daftar pihak terkait Form 00.05",
		EN: "related party list Form 00.05",
	},
	MsgPihakTerkaitItemsListed: {
		ID: "daftar pihak terkait untuk pengisian Form 00.05",
		EN: "related party list for Form 00.05 entry",
	},
	MsgPihakTerkaitSaved: {
		ID: "data pihak terkait Form 00.05 disimpan",
		EN: "related party Form 00.05 data saved",
	},
	MsgPihakTerkaitDeleted: {
		ID: "data pihak terkait Form 00.05 dihapus",
		EN: "related party Form 00.05 data deleted",
	},
	MsgPihakTerkaitIDInvalid: {
		ID: "id pihak terkait bukan UUID yang sah",
		EN: "related party id is not a valid UUID",
	},
	MsgModalReport: {
		ID: "daftar modal Form 00.06",
		EN: "capital list Form 00.06",
	},
	MsgModalItemsListed: {
		ID: "daftar modal untuk pengisian Form 00.06",
		EN: "capital list for Form 00.06 entry",
	},
	MsgModalSaved: {
		ID: "data modal Form 00.06 disimpan",
		EN: "capital Form 00.06 data saved",
	},
	MsgModalDeleted: {
		ID: "data modal Form 00.06 dihapus",
		EN: "capital Form 00.06 data deleted",
	},
	MsgModalIDInvalid: {
		ID: "id modal bukan UUID yang sah",
		EN: "capital id is not a valid UUID",
	},
	MsgModalInputInvalid: {
		ID: "data modal Form 00.06 tidak valid",
		EN: "capital Form 00.06 data is invalid",
	},
	MsgModalNotFound: {
		ID: "data modal tidak ditemukan",
		EN: "capital data was not found",
	},
	MsgModalBankWide: {
		ID: "register modal bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the capital register is bank-wide and can only be read by cross-branch roles",
	},
	MsgHapusBukuReport: {
		ID: "daftar aset produktif yang dihapus buku Form 15.00",
		EN: "written-off productive asset list Form 15.00",
	},
	MsgHapusBukuItemsListed: {
		ID: "daftar hapus buku untuk pengisian Form 15.00",
		EN: "write-off list for Form 15.00 entry",
	},
	MsgHapusBukuSaved: {
		ID: "data hapus buku Form 15.00 disimpan",
		EN: "write-off Form 15.00 data saved",
	},
	MsgHapusBukuDeleted: {
		ID: "data hapus buku Form 15.00 dihapus",
		EN: "write-off Form 15.00 data deleted",
	},
	MsgHapusBukuIDInvalid: {
		ID: "id hapus buku bukan UUID yang sah",
		EN: "write-off id is not a valid UUID",
	},
	MsgHapusBukuInputInvalid: {
		ID: "data hapus buku Form 15.00 tidak valid",
		EN: "write-off Form 15.00 data is invalid",
	},
	MsgHapusBukuNotFound: {
		ID: "data hapus buku tidak ditemukan",
		EN: "write-off data was not found",
	},
	MsgHapusBukuBankWide: {
		ID: "register hapus buku bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the write-off register is bank-wide and can only be read by cross-branch roles",
	},
	MsgPihakLawanReport: {
		ID: "daftar pihak lawan Form 00.16",
		EN: "counterparty list Form 00.16",
	},
	MsgPihakLawanItemsListed: {
		ID: "daftar pihak lawan untuk pengisian Form 00.16",
		EN: "counterparty list for Form 00.16 entry",
	},
	MsgPihakLawanSaved: {
		ID: "data pihak lawan Form 00.16 disimpan",
		EN: "counterparty Form 00.16 data saved",
	},
	MsgPihakLawanDeleted: {
		ID: "data pihak lawan Form 00.16 dihapus",
		EN: "counterparty Form 00.16 data deleted",
	},
	MsgPihakLawanIDInvalid: {
		ID: "id pihak lawan bukan UUID yang sah",
		EN: "counterparty id is not a valid UUID",
	},
	MsgPihakLawanInputInvalid: {
		ID: "data pihak lawan Form 00.16 tidak valid",
		EN: "counterparty Form 00.16 data is invalid",
	},
	MsgPihakLawanNotFound: {
		ID: "data pihak lawan tidak ditemukan",
		EN: "counterparty data was not found",
	},
	MsgPihakLawanBankWide: {
		ID: "register pihak lawan bersifat bank-wide dan hanya dapat dibaca peran lintas cabang",
		EN: "the counterparty register is bank-wide and can only be read by cross-branch roles",
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
	MsgStaffBookInvalid: {
		ID: "buku staf harus CONVENTIONAL atau SYARIAH dan sesuai cakupan buku instalasi serta buku pengelola",
		EN: "staff book must be CONVENTIONAL or SYARIAH and match the installation book scope and the manager's book",
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
