package http_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/i18n"
)

func decodeError(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("respons bukan JSON valid: %v (%s)", err, rec.Body.String())
	}
	return body.Error
}

func TestFailShowsBusinessError(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/loans", nil)

	httpHandler.Fail(rec, r, http.StatusUnprocessableEntity, domain.ErrInsufficientFunds)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, mau 422", rec.Code)
	}
	// Sungguhan domain kini dipetakan ke katalog i18n, sehingga pesan yang tampil
	// adalah terjemahan (default ID), bukan literal domain berbahasa Inggris.
	if got := decodeError(t, rec); got != i18n.Text(i18n.MsgInsufficientFunds) {
		t.Fatalf("pesan bisnis harus ditampilkan dari katalog, dapat %q", got)
	}
}

// Daftar kosong harus dikirim sebagai [] dan peta kosong sebagai {}, bukan null,
// supaya klien hanya menangani satu bentuk kontrak daftar. Dengan serialisasi lama,
// data bernilai null dan uji ini gagal.
func TestSuccessListKosongArray(t *testing.T) {
	for _, tc := range []struct {
		name string
		data any
		want string
	}{
		{"slice nil", []string(nil), "[]"},
		{"slice kosong", []string{}, "[]"},
		{"map nil", map[string]string(nil), "{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpHandler.Success(rec, http.StatusOK, i18n.MsgAccountsListed, tc.data)

			var body struct {
				Data json.RawMessage `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("respons bukan JSON: %v", err)
			}
			if string(body.Data) != tc.want {
				t.Fatalf("data = %s, ingin %s", body.Data, tc.want)
			}
		})
	}
}

// Sentinel dormant harus dikenali sebagai aturan bisnis sehingga menjadi 422 dengan
// pesan yang jelas, bukan 500 generik.
func TestFailShowsDormantBusinessErrors(t *testing.T) {
	for _, sentinel := range []error{domain.ErrAccountDormant, domain.ErrAccountNotDormant} {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/transactions/withdraw", nil)

		httpHandler.Fail(rec, r, http.StatusUnprocessableEntity, sentinel)

		if rec.Code != http.StatusUnprocessableEntity {
			t.Fatalf("%v: status = %d, mau 422", sentinel, rec.Code)
		}
		if got := decodeError(t, rec); got != sentinel.Error() {
			t.Fatalf("%v: pesan = %q, mau %q", sentinel, got, sentinel.Error())
		}
	}
}

func TestFailShowsCleanValidationMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/loans", nil)

	// Pesan validasi buatan service, tanpa sentinel dan tanpa jejak internal.
	httpHandler.Fail(rec, r, http.StatusUnprocessableEntity,
		fmt.Errorf("nominal di atas maksimum produk KRD-FLAT"))

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, mau 422", rec.Code)
	}
	if got := decodeError(t, rec); !strings.Contains(got, "maksimum") {
		t.Fatalf("pesan validasi harus ditampilkan, dapat %q", got)
	}
}

// Inilah yang dicegah: detail database tidak boleh sampai ke klien, sekalipun
// statusnya 4xx.
func TestFailHidesInternalError(t *testing.T) {
	leaky := []error{
		errors.New(`pq: duplicate key value violates unique constraint "customers_id_card_index_key"`),
		errors.New("sql: no rows in result set"),
		errors.New("gagal insert: ERROR: relation \"accounts\" does not exist (SQLSTATE 42P01)"),
	}

	for _, err := range leaky {
		rec := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/v1/accounts", nil)

		httpHandler.Fail(rec, r, http.StatusUnprocessableEntity, err)

		got := decodeError(t, rec)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("error internal harus jadi 500, dapat %d (%q)", rec.Code, got)
		}
		for _, marker := range []string{"pq:", "sql:", "constraint", "SQLSTATE", "relation"} {
			if strings.Contains(got, marker) {
				t.Fatalf("pesan membocorkan %q: %q", marker, got)
			}
		}
	}
}

func TestFailNeverShowsInternalOnServerError(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/reports/trial-balance", nil)

	httpHandler.Fail(rec, r, http.StatusInternalServerError, fmt.Errorf("koneksi database putus"))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, mau 500", rec.Code)
	}
	if got := decodeError(t, rec); strings.Contains(got, "koneksi database") {
		t.Fatalf("detail internal tidak boleh tampil di 500, dapat %q", got)
	}
}

// Kontrak respons tidak boleh berubah karena pesan dipindah ke katalog: bentuk
// {"success","message","data"} untuk sukses dan {"success","error"} untuk galat,
// dengan kode HTTP tetap ditentukan pemanggil. Uji ini mengunci bentuk itu.
func TestEnvelopeResponsTetapDenganKatalog(t *testing.T) {
	t.Setenv("CBS_LANGUAGE", "id")

	rec := httptest.NewRecorder()
	httpHandler.Success(rec, http.StatusCreated, i18n.MsgLoginSuccessful, map[string]any{"id": "u1"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status sukses = %d, mau 201", rec.Code)
	}
	var sukses struct {
		Success bool           `json:"success"`
		Message string         `json:"message"`
		Data    map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sukses); err != nil {
		t.Fatalf("respons sukses bukan JSON: %v", err)
	}
	if !sukses.Success || sukses.Message != "login berhasil" || sukses.Data["id"] != "u1" {
		t.Fatalf("envelope sukses berubah: %+v", sukses)
	}

	rec = httptest.NewRecorder()
	httpHandler.ErrorCode(rec, http.StatusForbidden, i18n.MsgAuthenticationRequired)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status galat = %d, mau 403", rec.Code)
	}
	var galat struct {
		Success bool   `json:"success"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &galat); err != nil {
		t.Fatalf("respons galat bukan JSON: %v", err)
	}
	if galat.Success || galat.Error != "autentikasi diperlukan" {
		t.Fatalf("envelope galat berubah: %+v", galat)
	}
}
