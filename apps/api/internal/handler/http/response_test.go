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

// N2: galat domain berkode diterjemahkan lewat SATU jalur (kode dimiliki galatnya
// sendiri), termasuk galat yang dibungkus %w; detail dinamis di belakang pesan dasar
// tetap dipertahankan agar maknanya tidak hilang.
func TestFailTerjemahkanGalatDomainBerkode(t *testing.T) {
	t.Setenv("CBS_LANGUAGE", "en")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/accounts/1/close", nil)

	rec := httptest.NewRecorder()
	httpHandler.Fail(rec, r, http.StatusUnprocessableEntity, domain.ErrAccountCloseBalance)
	if got, want := decodeError(t, rec), i18n.T(i18n.EN, i18n.MsgAccountCloseBalance); got != want {
		t.Fatalf("pesan EN = %q, ingin %q", got, want)
	}

	// Sentinel yang dulu tidak diterjemahkan (temuan E2E) kini ikut EN.
	rec = httptest.NewRecorder()
	httpHandler.Fail(rec, r, http.StatusUnprocessableEntity, domain.ErrOrgUnitCodeTooLong)
	if got, want := decodeError(t, rec), i18n.T(i18n.EN, i18n.MsgOrgUnitCodeTooLong); got != want {
		t.Fatalf("pesan kode terlalu panjang = %q, ingin %q", got, want)
	}

	rec = httptest.NewRecorder()
	httpHandler.Fail(rec, r, http.StatusUnprocessableEntity,
		fmt.Errorf("%w: saldo 1.000", domain.ErrAccountCloseBalance))
	got := decodeError(t, rec)
	if !strings.HasPrefix(got, i18n.T(i18n.EN, i18n.MsgAccountCloseBalance)) || !strings.Contains(got, "saldo 1.000") {
		t.Fatalf("detail galat terbungkus hilang: %q", got)
	}
}

// Temuan E2E: override 403 batas keamanan sempat tertelan jalur terjemahan katalog.
// Sejak translatedDomainError didahulukan, penolakan TULIS lintas cabang keluar
// dengan status default pemanggil (mis. 422), bukan 403 seperti yang dinyatakan
// komentar. Uji ini mengunci: batas keamanan SELALU 403, apa pun status defaultnya,
// dengan pesan tetap dari katalog i18n.
func TestFailPenolakanKeamananSelaluForbidden(t *testing.T) {
	t.Setenv("CBS_LANGUAGE", "id")
	r := httptest.NewRequest(http.MethodPost, "/api/v1/loans", nil)

	kasus := []struct {
		nama   string
		status int
		err    error
		pesan  string
	}{
		{"lintas cabang dari 422", http.StatusUnprocessableEntity, domain.ErrCrossBranchAccess, i18n.Text(i18n.MsgCrossBranchAccess)},
		{"lintas cabang dari 400", http.StatusBadRequest, domain.ErrCrossBranchAccess, i18n.Text(i18n.MsgCrossBranchAccess)},
		{"lintas buku dari 422", http.StatusUnprocessableEntity, domain.ErrCrossBookAccess, domain.ErrCrossBookAccess.Error()},
		{"kewenangan staf dari 422", http.StatusUnprocessableEntity, domain.ErrStaffRoleNotManageable, i18n.Text(i18n.MsgStaffRoleNotManageable)},
	}
	for _, tc := range kasus {
		t.Run(tc.nama, func(t *testing.T) {
			rec := httptest.NewRecorder()
			httpHandler.Fail(rec, r, tc.status, tc.err)
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, ingin 403 (default %d tidak boleh menang)", rec.Code, tc.status)
			}
			if got := decodeError(t, rec); got != tc.pesan {
				t.Fatalf("pesan = %q, ingin %q", got, tc.pesan)
			}
		})
	}

	// Galat berkode yang dibungkus %w tetap 403 dan detail dinamisnya dipertahankan.
	rec := httptest.NewRecorder()
	httpHandler.Fail(rec, r, http.StatusUnprocessableEntity,
		fmt.Errorf("%w: nasabah cabang 821", domain.ErrCrossBranchAccess))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status terbungkus = %d, ingin 403", rec.Code)
	}
	got := decodeError(t, rec)
	if !strings.HasPrefix(got, i18n.Text(i18n.MsgCrossBranchAccess)) || !strings.Contains(got, "nasabah cabang 821") {
		t.Fatalf("pesan terbungkus = %q, ingin katalog + detail", got)
	}
}

// Pembacaan lintas cabang/buku yang disamarkan pemanggil sebagai 404 TETAP 404:
// status itu sengaja dipilih supaya keberadaan data tidak bocor. Hanya penulisan
// (status validasi) yang dipaksa 403.
func TestFailPembacaanLintasCabangTetap404(t *testing.T) {
	t.Setenv("CBS_LANGUAGE", "id")
	r := httptest.NewRequest(http.MethodGet, "/api/v1/customers/1", nil)

	rec := httptest.NewRecorder()
	httpHandler.Fail(rec, r, http.StatusNotFound, domain.ErrCrossBranchAccess)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("baca lintas cabang = %d, ingin tetap 404", rec.Code)
	}
	if got, want := decodeError(t, rec), i18n.Text(i18n.MsgCrossBranchAccess); got != want {
		t.Fatalf("pesan baca lintas cabang = %q, ingin %q", got, want)
	}

	rec = httptest.NewRecorder()
	httpHandler.Fail(rec, r, http.StatusNotFound, domain.ErrCrossBookAccess)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("baca lintas buku = %d, ingin tetap 404", rec.Code)
	}
	if got, want := decodeError(t, rec), domain.ErrCrossBookAccess.Error(); got != want {
		t.Fatalf("pesan baca lintas buku = %q, ingin %q", got, want)
	}
}
