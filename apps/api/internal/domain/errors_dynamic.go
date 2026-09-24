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

	// Produk/rekening yang tidak cocok dengan operasi yang diminta.
	ErrLoanProductMissing         = NewLocalizedError("loan_product_missing", "kredit tidak terhubung ke produk")
	ErrInstallmentAccountNotFound = NewLocalizedError("installment_account_missing", "rekening pembayaran angsuran tidak ditemukan")
	ErrRecoveryAccountNotFound    = NewLocalizedError("recovery_account_missing", "rekening recovery tidak ditemukan")

	// Hapus buku: penolakan yang dibaca operator kredit.
	ErrWriteOffOnlyActive     = NewLocalizedError("write_off_only_active", "hanya kredit aktif yang dapat dihapus buku")
	ErrWriteOffNoPrincipal    = NewLocalizedError("write_off_no_principal", "kredit tidak memiliki sisa pokok yang dapat dihapus buku")
	ErrWriteOffNotWrittenOff  = NewLocalizedError("write_off_not_written_off", "kredit tidak berstatus hapus buku")
	ErrWriteOffMappingMissing = NewLocalizedError("write_off_mapping_missing", "pemetaan hapus buku belum lengkap")
	// Dua kondisi berbeda dengan bunyi pesan yang berbeda pula: nominal baru bisa
	// lebih kecil daripada POKOK YANG SUDAH DIBAYAR, atau lebih kecil daripada POKOK
	// JADWAL yang sudah dibekukan. Digabung menjadi satu sentinel akan mengubah bunyi
	// pesan Indonesia yang lama, jadi keduanya dipisah.
	ErrCorrectionBelowPaidPrincipal = NewLocalizedError("correction_below_paid_principal", "nominal baru lebih kecil daripada pokok yang sudah dibayar")
	ErrCorrectionBelowScheduled     = NewLocalizedError("correction_below_scheduled", "nominal baru lebih kecil daripada pokok jadwal yang sudah dibayar")

	// Produk dan cabang yang tidak sah untuk operasi yang diminta. Data (kode produk
	// atau kode cabang) berada di TENGAH pesan Indonesia, jadi semuanya dibuat lewat
	// konstruktor berplaceholder di bawah supaya bunyi pesan lama TIDAK berubah.
	ErrAROInstructionUnknown = NewLocalizedError("aro_instruction_unknown", "instruksi ARO tidak dikenal")
)

// Konstruktor pesan galat dengan data di TENGAH pesan. Masing-masing mempertahankan
// bunyi pesan Indonesia lama persis ("produk ABC bukan produk kredit/pembiayaan"),
// sementara katalog EN menyusun katanya sendiri dengan placeholder yang sama.
// Dipakai karena menerjemahkan pesan berkode lalu menempelkan data di AKHIR akan
// mengubah bunyi pesan Indonesia - hal yang dilarang agar bank dapat membandingkan
// berkas lama.
func NotLoanProduct(code string) *LocalizedError {
	return NewLocalizedErrorf("not_loan_product", "produk %s bukan produk kredit/pembiayaan", code)
}

func PaymentMethodUnknown(method string) *LocalizedError {
	return NewLocalizedErrorf("payment_method_unknown", "metode pembayaran %q tidak dikenal", method)
}

func ProductNotForSavings(code string) *LocalizedError {
	return NewLocalizedErrorf("product_not_for_savings", "produk %s tidak untuk pembukaan rekening simpanan", code)
}

func ProductInactive(code string) *LocalizedError {
	return NewLocalizedErrorf("product_inactive", "produk %s sedang tidak aktif", code)
}

func BranchInactive(code string) *LocalizedError {
	return NewLocalizedErrorf("branch_inactive", "cabang %s sedang tidak aktif", code)
}

func COAAccountNotFound(code string) *LocalizedError {
	return NewLocalizedErrorf("coa_account_not_found", "akun COA %s tidak ditemukan", code)
}
