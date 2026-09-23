package http_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cbs-core/apps/core-api/internal/domain"
	httpHandler "cbs-core/apps/core-api/internal/handler/http"
	"cbs-core/apps/core-api/internal/i18n"
)

// Temuan: pesan galat dinamis yang menggabungkan teks Indonesia dengan data tetap
// berbahasa Indonesia saat CBS_LANGUAGE=en. Contoh yang paling sering dilihat pengguna
// adalah batas minimum produk pada pengajuan kredit/penempatan deposito. Basis pesan
// kini berkode katalog, sedangkan kode produk dan nominal tetap sebagai akhiran
// dinamis, sehingga bahasa dasar mengikuti instalasi tanpa membongkar katalog.
func TestFailPesanBatasMinimumProdukIkutBahasaInstalasi(t *testing.T) {
	err := fmt.Errorf("%w %s (%s)", domain.ErrProductAmountBelowMin, "KRD-FLAT", "1000.00")

	t.Setenv("CBS_LANGUAGE", "id")
	idRec := httptest.NewRecorder()
	httpHandler.Fail(idRec, httptest.NewRequest(http.MethodPost, "/loans", nil),
		http.StatusUnprocessableEntity, err)
	id := decodeError(t, idRec)
	if !strings.Contains(id, "nominal di bawah minimum produk") || !strings.Contains(id, "KRD-FLAT (1000.00)") {
		t.Fatalf("pesan ID = %q, mau makna lama utuh", id)
	}

	t.Setenv("CBS_LANGUAGE", "en")
	enRec := httptest.NewRecorder()
	httpHandler.Fail(enRec, httptest.NewRequest(http.MethodPost, "/loans", nil),
		http.StatusUnprocessableEntity, err)
	en := decodeError(t, enRec)
	if !strings.Contains(en, i18n.Text(i18n.MsgProductAmountBelowMin)) {
		t.Fatalf("pesan EN tidak memakai katalog: %q", en)
	}
	if strings.Contains(en, "nominal di bawah minimum produk") {
		t.Fatalf("pesan EN masih berbahasa Indonesia: %q", en)
	}
	if !strings.Contains(en, "KRD-FLAT (1000.00)") {
		t.Fatalf("detail dinamis (kode produk + nominal) hilang: %q", en)
	}
}
