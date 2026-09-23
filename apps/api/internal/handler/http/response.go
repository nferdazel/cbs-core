package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
	"cbs-core/apps/core-api/internal/i18n"
	"cbs-core/apps/core-api/internal/observability"
)

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Meta    any    `json:"meta,omitempty"`
}

type PaginationMeta struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

func JSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

// Success membalas sukses dengan pesan dari katalog i18n. Pesan diterjemahkan di
// sini, bukan di handler, sehingga hanya kode yang berserak dan bahasa respons
// tetap satu sumber. Isi pesan lama (perilaku dan maknanya) tidak berubah.
func Success(w http.ResponseWriter, statusCode int, code i18n.Code, data any) {
	JSON(w, statusCode, APIResponse{
		Success: true,
		Message: i18n.Text(code),
		Data:    normalizeListData(data),
	})
}

func SuccessWithMeta(w http.ResponseWriter, statusCode int, code i18n.Code, data any, meta any) {
	JSON(w, statusCode, APIResponse{
		Success: true,
		Message: i18n.Text(code),
		Data:    normalizeListData(data),
		Meta:    meta,
	})
}

// normalizeListData mengubah slice/peta nil menjadi bentuk kosong non-nil, sehingga
// daftar kosong diserialkan sebagai [] / {} alih-alih null. Klien tidak perlu lagi
// menangani dua bentuk (null dan array) untuk kontrak daftar yang sama. Nilai yang
// bukan slice/peta, termasuk interface nil (data memang tidak ada), dibiarkan apa
// adanya.
func normalizeListData(data any) any {
	if data == nil {
		return nil
	}
	v := reflect.ValueOf(data)
	switch v.Kind() {
	case reflect.Slice:
		if v.IsNil() {
			return reflect.MakeSlice(v.Type(), 0, 0).Interface()
		}
	case reflect.Map:
		if v.IsNil() {
			return reflect.MakeMap(v.Type()).Interface()
		}
	}
	return data
}

// domainErrorCodes memetakan sentinel galat domain yang tampil ke pengguna ke kode
// katalog i18n. Pemetaan di lapisan handler menjaga tipe error domain tetap apa
// adanya, sementara pesan tetap satu sumber dan tersedia ID + EN. Hanya kecocokan
// PERSIS (errors.Is terhadap sentinel tak terbungkus) yang diterjemahkan; galat yang
// sudah dibungkus tetap menampilkan detail tambahannya agar makna tidak hilang.
var domainErrorCodes = []struct {
	err  error
	code i18n.Code
}{
	{domain.ErrInvalidCredentials, i18n.MsgInvalidCredentials},
	{domain.ErrInsufficientFunds, i18n.MsgInsufficientFunds},
	{domain.ErrSessionExpired, i18n.MsgSessionExpired},
}

// translatedDomainError mengembalikan pesan katalog untuk sentinel domain yang
// dikenali, atau ok=false bila tidak ada.
func translatedDomainError(err error) (string, bool) {
	for _, m := range domainErrorCodes {
		if err == m.err {
			return i18n.Text(m.code), true
		}
	}
	return "", false
}

// Error mengirim pesan yang sudah aman ditampilkan ke pengguna. Jangan lewatkan
// error internal mentah ke sini; pakai InternalError untuk itu. Untuk pesan tetap
// dari katalog pakai ErrorCode; fungsi ini khusus pesan dinamis (mis. err.Error()).
func Error(w http.ResponseWriter, statusCode int, errMessage string) {
	JSON(w, statusCode, APIResponse{
		Success: false,
		Error:   errMessage,
	})
}

// ErrorCode membalas galat dengan pesan tetap dari katalog i18n.
func ErrorCode(w http.ResponseWriter, statusCode int, code i18n.Code) {
	Error(w, statusCode, i18n.Text(code))
}

// ErrorCodef membalas galat dengan pesan berkode yang menyertakan detail dinamis.
func ErrorCodef(w http.ResponseWriter, statusCode int, code i18n.Code, args ...any) {
	Error(w, statusCode, i18n.Textf(code, args...))
}

// InternalError membalas pesan generik untuk kegagalan tak terduga, sementara detail
// aslinya hanya dicatat ke log dengan request id. Ini mencegah nama tabel, query SQL,
// atau path internal bocor ke klien.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	observability.FromContext(r.Context()).Error("kegagalan internal saat menangani permintaan",
		"method", r.Method, "path", r.URL.Path, "error", err)
	Error(w, http.StatusInternalServerError, i18n.Text(i18n.MsgInternalError))
}

// businessErrors adalah error yang aman ditampilkan ke pengguna karena maknanya
// jelas bagi mereka: kesalahan input, aturan bisnis, atau otorisasi. Error di luar
// daftar ini dianggap kegagalan internal dan pesannya disembunyikan.
var businessErrors = []error{
	domain.ErrInvalidCredentials,
	domain.ErrAccountLocked,
	domain.ErrAccountInactiveUser,
	domain.ErrSessionExpired,
	domain.ErrSessionRevoked,
	domain.ErrInvalidToken,
	domain.ErrForbidden,
	domain.ErrPasswordExpired,
	domain.ErrCustomerNotFound,
	domain.ErrDuplicateIDCard,
	domain.ErrDuplicateEmail,
	domain.ErrAccountNotFound,
	domain.ErrAccountInactive,
	domain.ErrAccountDormant,
	domain.ErrAccountNotDormant,
	// Unfreeze adalah aturan bisnis: pesannya menyebut status saat ini agar operator
	// tahu rekening tidak sedang dibekukan.
	domain.ErrAccountNotFrozen,
	// Penutupan rekening adalah aturan bisnis: pesannya menyebut syarat yang belum
	// terpenuhi (saldo/status) agar operator dapat menindaklanjuti.
	domain.ErrAccountCloseBalance,
	domain.ErrAccountNotClosable,
	domain.ErrInsufficientFunds,
	domain.ErrInvalidAmount,
	domain.ErrDuplicateIdempotencyKey,
	domain.ErrLedgerUnbalanced,
	domain.ErrProductNotFound,
	// Parameter produk bagi hasil yang belum diisi adalah aturan bisnis: pesannya
	// menyebut apa yang harus dilengkapi bank, bukan detail internal.
	domain.ErrBagiHasilNisbahMissing,
	domain.ErrBagiHasilProjectionMissing,
	domain.ErrBagiHasilNisbahOutOfRange,
	domain.ErrBagiHasilProjectionOutOfRange,
	// Perubahan parameter produk: pesan validasi menyebut bidang dan satuannya,
	// sehingga aman ditampilkan dan menolong operator memperbaiki masukan.
	domain.ErrProductParamsEmpty,
	domain.ErrProductRateNegative,
	domain.ErrProductAdminFeeNegative,
	domain.ErrProductTaxRateNegative,
	domain.ErrProductPenaltyRateNegative,
	domain.ErrProductMinAmountNegative,
	domain.ErrProductMaxAmountNegative,
	domain.ErrProductAmountRange,
	domain.ErrProductTermNegative,
	domain.ErrProductTermRange,
	domain.ErrBranchNotFound,
	domain.ErrBranchCodeExists,
	domain.ErrBranchNameRequired,
	domain.ErrBranchHeadOfficeNotAllowed,
	domain.ErrInvalidBranchCode,
	// Validasi profil bank adalah aturan bisnis: pesannya menyebut bidang yang
	// harus diperbaiki agar operator dapat mengisinya sendiri tanpa SQL.
	domain.ErrBankProfileNameRequired,
	domain.ErrBankProfileNameTooShort,
	domain.ErrBankProfileEmpty,
	domain.ErrBankProfileFieldTooLong,
	domain.ErrBankProfileNPWPInvalid,
	domain.ErrBankProfilePhoneInvalid,
	domain.ErrLoanNotFound,
	domain.ErrLoanAlreadyApproved,
	// Syarat hapus buku adalah aturan bisnis/regulasi: pesannya harus menyebut syarat
	// mana yang belum terpenuhi supaya operator dapat menindaklanjutinya.
	domain.ErrWriteOffNotMacet,
	domain.ErrWriteOffReserveIncomplete,
	domain.ErrWriteOffPartial,
	domain.ErrWriteOffReasonRequired,
	domain.ErrWriteOffCollectionEffortsRequired,
	// Batas pemulihan hapus buku adalah aturan uang: pesannya harus menyebut nilai
	// hapus buku vs akumulasi pemulihan agar operator dapat menindaklanjuti.
	domain.ErrRecoveryExceedsWriteOff,
	domain.ErrWriteOffAmountUnavailable,
	domain.ErrInvalidBusinessDate,
	domain.ErrEODAlreadyRunForDate,
	domain.ErrEODInProgress,
	domain.ErrCipherNotConfigured,
	// Batas transaksi dan maker-checker adalah aturan bisnis: pesannya harus terlihat
	// pengguna agar mereka tahu mengapa transaksi ditolak atau dialihkan.
	domain.ErrLimitPerTransaction,
	domain.ErrLimitDaily,
	domain.ErrRequiresApproval,
	domain.ErrMakerCheckerNotFound,
	domain.ErrMakerCheckerNotPending,
	domain.ErrCannotSelfApprove,
	domain.ErrNoExecutorForAction,
	// Penolakan lintas cabang adalah aturan otorisasi: pesannya harus terlihat
	// pengguna agar mereka tahu mengapa operasi ditolak.
	domain.ErrCrossBranchAccess,
	// PPKA basi menandai perbandingan CKPN/PPAP yang memakai tanggal bisnis
	// berbeda; ini kondisi operator, bukan kegagalan server.
	domain.ErrCKPNStalePPAP,
}

// isBusinessError melaporkan apakah error termasuk yang aman ditampilkan ke pengguna.
// Pemeriksaan juga memakai pesan karena banyak service membungkus error dengan %w.
func isBusinessError(err error) bool {
	for _, sentinel := range businessErrors {
		if errors.Is(err, sentinel) {
			return true
		}
	}
	return false
}

// internalLeakMarkers adalah penanda bahwa sebuah pesan error berasal dari lapisan
// bawah (database, driver, filesystem) dan tidak boleh terlihat pengguna.
var internalLeakMarkers = []string{
	"sql:", "pq:", "pgx", "sqlstate", "constraint", "relation ", "column ",
	"table ", "syntax error", "foreign key", "unique violation",
	".go:", "goroutine", "runtime error", "nil pointer",
	"/srv/", "/home/", "/Users/", "postgres://",
}

// looksLikeInternalError melaporkan apakah pesan error membawa jejak implementasi.
func looksLikeInternalError(msg string) bool {
	lower := strings.ToLower(msg)
	for _, marker := range internalLeakMarkers {
		if strings.Contains(lower, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// Fail membalas error dengan satu aturan: error bisnis yang dikenali ditampilkan
// apa adanya, sisanya dicatat ke log dan diganti pesan generik. Ini mencegah detail
// internal (SQL, nama tabel, path) bocor lewat status 4xx sekalipun.
func Fail(w http.ResponseWriter, r *http.Request, status int, err error) {
	if err == nil {
		InternalError(w, r, errors.New("Fail dipanggil tanpa error"))
		return
	}
	// Sentinel galat domain yang tampil ke pengguna diterjemahkan dari katalog
	// supaya pesannya konsisten dengan pesan API lain (ID + EN).
	if msg, ok := translatedDomainError(err); ok {
		Error(w, status, msg)
		return
	}
	// Penolakan lintas cabang selalu 403, apa pun status default pemanggil.
	// Dipusatkan di sini agar seluruh handler konsisten tanpa memetakan sendiri.
	if errors.Is(err, domain.ErrCrossBranchAccess) {
		Error(w, http.StatusForbidden, err.Error())
		return
	}
	// Penolakan kewenangan atas akun staf juga 403: klien perlu membedakannya dari
	// kesalahan validasi masukan, bukan dari status yang dipilih pemanggil.
	if errors.Is(err, domain.ErrStaffRoleNotManageable) {
		Error(w, http.StatusForbidden, err.Error())
		return
	}
	if isBusinessError(err) {
		Error(w, status, err.Error())
		return
	}

	// Error tanpa sentinel yang pesannya bersih dianggap pesan validasi/aturan bisnis
	// yang memang ditujukan untuk pengguna. Yang membawa jejak internal disembunyikan.
	if msg := err.Error(); msg != "" && !looksLikeInternalError(msg) {
		if status >= 400 && status < 500 {
			Error(w, status, msg)
			return
		}
	}
	InternalError(w, r, err)
}
