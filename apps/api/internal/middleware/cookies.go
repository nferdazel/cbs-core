package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
)

// CookieConfig menentukan nama cookie sesi/CSRF dan atribut Secure. Secure dapat
// dipaksa true (APP_ENV=production) atau ditentukan per-request dari TLS /
// X-Forwarded-Proto. Domain opsional untuk deployment lintas subdomain.
type CookieConfig struct {
	AccessName  string
	RefreshName string
	CSRFName    string
	CSRFHeader  string
	Domain      string
	Secure      bool
}

// isSecure memutuskan atribut Secure: dipaksa di production, atau mengikuti
// koneksi HTTPS (langsung maupun di belakang proxy Caddy).
func (c CookieConfig) isSecure(r *http.Request) bool {
	return c.Secure || r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// SetSessionCookies menulis access & refresh token sebagai httpOnly cookie.
// maxAge dalam detik.
func (c CookieConfig) SetSessionCookies(w http.ResponseWriter, r *http.Request, accessToken, refreshToken string, accessMaxAge, refreshMaxAge int) {
	secure := c.isSecure(r)
	http.SetCookie(w, c.newCookie(c.AccessName, accessToken, accessMaxAge, secure, true))
	http.SetCookie(w, c.newCookie(c.RefreshName, refreshToken, refreshMaxAge, secure, true))
}

// SetCSRFCookie menulis token CSRF non-httpOnly agar JavaScript dapat membacanya
// dan mengirimnya kembali lewat header X-CSRF-Token (pola double-submit).
func (c CookieConfig) SetCSRFCookie(w http.ResponseWriter, r *http.Request, token string, maxAge int) {
	http.SetCookie(w, c.newCookie(c.CSRFName, token, maxAge, c.isSecure(r), false))
}

// ClearSessionCookies menghapus cookie sesi dan CSRF. Atribut Secure dihitung
// sama seperti saat menulis agar browser mencocokkan lalu benar-benar menghapus.
func (c CookieConfig) ClearSessionCookies(w http.ResponseWriter, r *http.Request) {
	secure := c.isSecure(r)
	for _, name := range []string{c.AccessName, c.RefreshName, c.CSRFName} {
		http.SetCookie(w, c.newCookie(name, "", -1, secure, name != c.CSRFName))
	}
}

func (c CookieConfig) newCookie(name, value string, maxAge int, secure, httpOnly bool) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Domain:   c.Domain,
		MaxAge:   maxAge,
		Secure:   secure,
		HttpOnly: httpOnly,
		SameSite: http.SameSiteStrictMode,
	}
}

// AccessTokenFromRequest mengambil access token dari cookie terlebih dahulu,
// lalu jatuh ke header Authorization: Bearer untuk kompatibilitas integrasi.
func (c CookieConfig) AccessTokenFromRequest(r *http.Request) string {
	if ck, err := r.Cookie(c.AccessName); err == nil && ck.Value != "" {
		return ck.Value
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
	}
	return ""
}

// RefreshTokenFromRequest mengambil refresh token dari cookie.
func (c CookieConfig) RefreshTokenFromRequest(r *http.Request) string {
	if ck, err := r.Cookie(c.RefreshName); err == nil && ck.Value != "" {
		return ck.Value
	}
	return ""
}

// GenerateCSRFToken menghasilkan token acak kriptografis 32 byte (base64 URL).
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
