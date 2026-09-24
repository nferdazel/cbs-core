package domain

// Galat validasi dinamis yang paling sering dilihat pengguna pada alur harian:
// pengajuan/pencairan kredit, pembayaran angsuran, penempatan/penarikan deposito,
// pembukaan rekening, dan pembukaan/registrasi nasabah. Basis pesannya berkode
// katalog i18n (ID+EN) sedangkan data (kode produk, nominal, tenor) tetap
// ditambahkan pemanggil lewat %w sehingga makna pesan Indonesia lama tidak berubah.
var (
	// Batas produk: data (kode produk + nominal/tenor) ditambahkan pemanggil sebagai
	// akhiran, sama seperti ErrProductAmountBelowMin.
	ErrProductAmountAboveMax = NewLocalizedError("product_amount_above_max", "nominal di atas maksimum produk")
	ErrProductTermOutOfRange = NewLocalizedError("product_term_out_of_range", "jangka waktu di luar rentang produk")

	ErrDisbursementAccountNotFound = NewLocalizedError("disbursement_account_not_found", "rekening pencairan tidak ditemukan")
	ErrDisbursementAccountNotOwned = NewLocalizedError("disbursement_account_not_owned", "rekening pencairan bukan milik nasabah yang mengajukan")
	ErrLoanAccountCOAMissing       = NewLocalizedError("loan_account_coa_missing", "rekening nasabah tidak punya kode COA; jurnal tidak dapat dipetakan")

	ErrLoanNotActive          = NewLocalizedError("loan_not_active", "kredit tidak dalam status aktif")
	ErrInstallmentNotFound    = NewLocalizedError("installment_not_found", "jadwal angsuran tidak ditemukan")
	ErrInstallmentAlreadyPaid = NewLocalizedError("installment_already_paid", "angsuran ini sudah dibayar penuh")

	ErrDisbursementCancelReasonRequired = NewLocalizedError("disbursement_cancel_reason_required", "alasan pembatalan pencairan wajib diisi")
	ErrLoanCorrectionReasonRequired     = NewLocalizedError("loan_correction_reason_required", "alasan koreksi nominal wajib diisi")

	ErrRestructureOnlyActiveLoan  = NewLocalizedError("restructure_only_active_loan", "hanya kredit aktif yang dapat direstrukturisasi")
	ErrRestructureNewTermPositive = NewLocalizedError("restructure_new_term_positive", "jangka waktu baru harus positif")
	ErrRecoveryAmountPositive     = NewLocalizedError("recovery_amount_positive", "nominal recovery harus positif")

	ErrCustomerNameRequired = NewLocalizedError("customer_name_required", "nama lengkap wajib diisi")
	ErrSameAccountTransfer  = NewLocalizedError("same_account_transfer", "rekening asal dan tujuan tidak boleh sama")
	ErrAccountNumberTaken   = NewLocalizedError("account_number_taken", "nomor rekening sudah terpakai, silakan coba lagi")
)
