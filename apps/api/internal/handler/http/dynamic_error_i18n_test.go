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

// Uji bahasa untuk kelompok galat validasi dinamis yang paling sering dilihat pengguna
// pada alur harian (batas produk kredit/deposito). Basis pesan kini berkode katalog,
// sedangkan data (kode produk, nominal, tenor) tetap sebagai akhiran dinamis. Karena
// itu pesan ID harus sama persis dengan sebelumnya dan pesan EN tidak boleh lagi
// mengandung teks Indonesia.
func TestPesanValidasiProdukIkutBahasaInstalasi(t *testing.T) {
	kasus := []struct {
		nama    string
		pesanID string
		dinamis string
		kode    i18n.Code
		err     error
	}{
		{
			nama:    "nominal di atas maksimum produk",
			pesanID: "nominal di atas maksimum produk KRD-FLAT (50000000.00)",
			dinamis: "KRD-FLAT (50000000.00)",
			kode:    i18n.MsgProductAmountAboveMax,
			err:     fmt.Errorf("%w %s (%s)", domain.ErrProductAmountAboveMax, "KRD-FLAT", "50000000.00"),
		},
		{
			nama:    "jangka waktu di luar rentang produk",
			pesanID: "jangka waktu di luar rentang produk DEP-CONV (1-12 bulan)",
			dinamis: "DEP-CONV (1-12 bulan)",
			kode:    i18n.MsgProductTermOutOfRange,
			err:     fmt.Errorf("%w %s (%d-%d bulan)", domain.ErrProductTermOutOfRange, "DEP-CONV", 1, 12),
		},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			t.Setenv("CBS_LANGUAGE", "id")
			idRec := httptest.NewRecorder()
			httpHandler.Fail(idRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			if got := decodeError(t, idRec); got != k.pesanID {
				t.Fatalf("pesan ID berubah:\n  dapat %q\n  mau   %q", got, k.pesanID)
			}

			t.Setenv("CBS_LANGUAGE", "en")
			enRec := httptest.NewRecorder()
			httpHandler.Fail(enRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			en := decodeError(t, enRec)
			if en != i18n.T(i18n.EN, k.kode)+" "+k.dinamis {
				t.Fatalf("pesan EN tidak sesuai katalog:\n  dapat %q", en)
			}
			if !strings.Contains(en, k.dinamis) {
				t.Fatalf("detail dinamis %q hilang dari pesan EN: %q", k.dinamis, en)
			}
			// Bahasa Indonesia pada basis pesan harus benar-benar hilang.
			basisID := strings.TrimSuffix(strings.TrimSpace(k.pesanID), " "+k.dinamis)
			if strings.Contains(en, basisID) {
				t.Fatalf("pesan EN masih berbahasa Indonesia: %q", en)
			}
		})
	}
}
