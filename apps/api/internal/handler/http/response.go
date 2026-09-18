package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"cbs-core/apps/core-api/internal/domain"
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

func Success(w http.ResponseWriter, statusCode int, message string, data any) {
	JSON(w, statusCode, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}

func SuccessWithMeta(w http.ResponseWriter, statusCode int, message string, data any, meta any) {
	JSON(w, statusCode, APIResponse{
		Success: true,
		Message: message,
		Data:    data,
		Meta:    meta,
	})
}

// Error mengirim pesan yang sudah aman ditampilkan ke pengguna. Jangan lewatkan
// error internal mentah ke sini; pakai InternalError untuk itu.
func Error(w http.ResponseWriter, statusCode int, errMessage string) {
	JSON(w, statusCode, APIResponse{
		Success: false,
		Error:   errMessage,
	})
}

// InternalError membalas pesan generik untuk kegagalan tak terduga, sementara detail
// aslinya hanya dicatat ke log dengan request id. Ini mencegah nama tabel, query SQL,
// atau path internal bocor ke klien.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	observability.FromContext(r.Context()).Error("kegagalan internal saat menangani permintaan",
		"method", r.Method, "path", r.URL.Path, "error", err)
	Error(w, http.StatusInternalServerError, "terjadi kesalahan internal, silakan coba lagi")
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
	domain.ErrInsufficientFunds,
	domain.ErrInvalidAmount,
	domain.ErrDuplicateIdempotencyKey,
	domain.ErrLedgerUnbalanced,
	domain.ErrProductNotFound,
	domain.ErrBranchNotFound,
	domain.ErrLoanNotFound,
	domain.ErrLoanAlreadyApproved,
	domain.ErrInvalidBusinessDate,
	domain.ErrEODAlreadyRunForDate,
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
	// Penolakan lintas cabang selalu 403, apa pun status default pemanggil.
	// Dipusatkan di sini agar seluruh handler konsisten tanpa memetakan sendiri.
	if errors.Is(err, domain.ErrCrossBranchAccess) {
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
