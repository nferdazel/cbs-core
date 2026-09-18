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
	if got := decodeError(t, rec); !strings.Contains(got, domain.ErrInsufficientFunds.Error()) {
		t.Fatalf("pesan bisnis harus ditampilkan, dapat %q", got)
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
