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

// Galat manajemen staf dan kata sandi. Pesan lama pada berkas ini BERBAUR: sebagian
// Inggris ("password must be at least 8 characters") dan sebagian Indonesia, padahal
// semuanya ditampilkan ke pengguna. Semua kini berkode katalog agar bahasa instalasi
// dipatuhi seragam; bunyi Indonesia yang sudah ada dipertahankan apa adanya.
var (
	ErrStaffPasswordTooShort   = NewLocalizedError("staff_password_too_short", "kata sandi minimal 8 karakter")
	ErrStaffPasswordWeak       = NewLocalizedError("staff_password_weak", "kata sandi harus memuat huruf besar, huruf kecil, angka, dan karakter khusus")
	ErrStaffPrivilegedCreate   = NewLocalizedError("staff_privileged_create", "SUPERADMIN atau SYSTEM tidak dapat dibuat lewat endpoint ini")
	ErrStaffPrivilegedRole     = NewLocalizedError("staff_privileged_role", "peran SUPERADMIN atau SYSTEM tidak dapat diberikan lewat pembaruan")
	ErrStaffCurrentPassword    = NewLocalizedError("staff_current_password_wrong", "kata sandi saat ini salah")
	ErrStaffPasswordUnchanged  = NewLocalizedError("staff_password_unchanged", "kata sandi baru harus berbeda dari kata sandi saat ini")
	ErrStaffUseOwnPasswordFlow = NewLocalizedError("staff_use_own_password_flow", "gunakan ubah kata sandi untuk akun sendiri")
	ErrStaffSelfDeactivate     = NewLocalizedError("staff_self_deactivate", "akun sendiri tidak dapat dinonaktifkan")
	ErrStaffPasswordOnlySelf   = NewLocalizedError("staff_password_only_self", "ubah kata sandi hanya berlaku untuk akun sendiri")
)

// Validasi masukan operator pada alur yang memang diisi manusia: koreksi jadwal
// angsuran dan pengisian parameter CKPN. Keduanya bukan galat invarian internal.
var (
	ErrNoUnpaidInstallment   = NewLocalizedError("no_unpaid_installment", "tidak ada angsuran belum dibayar yang dapat disesuaikan")
	ErrCKPNValueNotDecimalID = "ckpn_value_not_decimal"
)

// CKPNValueNotDecimal membentuk galat parameter CKPN yang bukan angka desimal, dengan
// nilai apa adanya di TENGAH pesan supaya bunyi Indonesia lama tidak berubah.
func CKPNValueNotDecimal(raw string) *LocalizedError {
	return NewLocalizedErrorf(ErrCKPNValueNotDecimalID, "nilai %q bukan angka desimal yang sah", raw)
}

// Validasi NIK dan konfigurasi batas transaksi. NIK adalah masukan pengguna (16 digit);
// pesan batas menjelaskan mengapa transaksi DITOLAK dan apa yang harus diperbaiki
// operator, jadi keduanya harus terbaca dalam bahasa instalasi.
var (
	ErrNIKTooShort          = NewLocalizedError("nik_too_short", "NIK harus 16 digit")
	ErrLimitConfigMissingID = "limit_config_missing"
	ErrLimitConfigInvalidID = "limit_config_invalid"
)

// LimitConfigMissing membentuk galat konfigurasi batas yang belum diisi. Kunci konfigurasi
// berada di TENGAH pesan, jadi memakai placeholder supaya bunyi Indonesia tidak berubah.
func LimitConfigMissing(key string) *LocalizedError {
	return NewLocalizedErrorf(ErrLimitConfigMissingID, "konfigurasi batas %s belum diisi; batas transaksi tidak boleh memakai angka bawaan", key)
}

// LimitConfigInvalid membentuk galat konfigurasi batas yang bukan angka, dengan kunci dan
// nilai apa adanya supaya operator tahu persis yang harus diperbaiki.
func LimitConfigInvalid(key, value string) *LocalizedError {
	return NewLocalizedErrorf(ErrLimitConfigInvalidID, "konfigurasi batas %s bernilai %q, bukan angka; perbaiki nilainya sebelum bertransaksi", key, value)
}
