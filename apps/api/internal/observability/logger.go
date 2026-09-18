// Package observability menyediakan logger terstruktur untuk seluruh layanan.
//
// Dua aturan yang mengikat seluruh paket ini:
//
//  1. Data pribadi nasabah tidak pernah masuk log. Redaksi dilakukan oleh helper di
//     paket ini, bukan diserahkan pada disiplin pemanggil.
//  2. Log teknis (paket ini) terpisah dari audit log bisnis. Audit log adalah catatan
//     transaksi yang disimpan di database; paket ini menulis ke stdout saja.
package observability

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// contextKey adalah tipe privat untuk kunci context, mencegah tabrakan.
type contextKey struct{ name string }

var (
	requestIDKey = contextKey{"request_id"}
	loggerKey    = contextKey{"logger"}
)

// NewLogger membangun logger sesuai lingkungan. Produksi memakai JSON level Info
// agar bisa diindeks oleh pengumpul log; selain itu teks level Debug untuk diagnosis.
func NewLogger(env string) *slog.Logger {
	var handler slog.Handler
	if strings.EqualFold(env, "production") {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	}
	return slog.New(handler)
}

// WithRequest melampirkan identitas permintaan ke logger. Nilai yang kosong dilewati
// agar log tidak penuh atribut hampa.
func WithRequest(l *slog.Logger, requestID, userID, role, branch string) *slog.Logger {
	if l == nil {
		l = slog.Default()
	}
	attrs := make([]any, 0, 8)
	if requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	if userID != "" {
		attrs = append(attrs, "user_id", userID)
	}
	if role != "" {
		attrs = append(attrs, "role", role)
	}
	if branch != "" {
		attrs = append(attrs, "branch", branch)
	}
	return l.With(attrs...)
}

// ContextWithLogger menyimpan logger ke context agar lapisan bawah memakai logger
// yang sudah membawa request_id, tanpa harus meneruskannya lewat parameter.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext mengambil logger dari context. Bila tidak ada, logger default dipakai
// sehingga pemanggil tidak perlu memeriksa nil.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(loggerKey).(*slog.Logger); ok && l != nil {
		return l
	}
	return slog.Default()
}

// ContextWithRequestID menyimpan id permintaan ke context.
func ContextWithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

// RequestIDFromContext mengambil id permintaan dari context.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey).(string); ok {
		return id
	}
	return ""
}

// sensitiveKeys adalah nama field yang nilainya tidak boleh tercatat. Pencocokan
// memakai substring dan case-insensitive, sehingga "customer_email" dan "EMAIL"
// keduanya tertangkap.
var sensitiveKeys = []string{
	"password", "passwd", "secret", "token", "authorization", "cookie",
	"nik", "id_card", "ktp", "npwp",
	"account_number", "norek", "rekening",
	"phone", "telepon", "hp", "whatsapp",
	"email", "address", "alamat",
	"pin", "otp", "cvv",
}

// IsSensitiveKey melaporkan apakah nama field termasuk data yang harus diredaksi.
func IsSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range sensitiveKeys {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// Redact menyamarkan nilai sensitif. Nomor rekening menyisakan 4 digit terakhir agar
// masih bisa dikorelasikan; nilai lain diganti seluruhnya.
func Redact(key, value string) string {
	if !IsSensitiveKey(key) {
		return value
	}
	if strings.Contains(strings.ToLower(key), "account") || strings.Contains(strings.ToLower(key), "rekening") {
		if len(value) >= 8 {
			return strings.Repeat("*", len(value)-4) + value[len(value)-4:]
		}
	}
	return "***"
}
