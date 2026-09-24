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

// Lanjutan cakupan: penolakan produk/cabang, hapus buku, koreksi nominal, dan metode
// pembayaran. Semua pesan ID harus tetap SAMA seperti sebelum konversi (bank
// membandingkan berkas lama), sementara pesan EN tidak boleh mengandung teks Indonesia.
func TestPesanValidasiLanjutanIkutBahasaInstalasi(t *testing.T) {
	kasus := []struct {
		nama    string
		pesanID string
		kode    i18n.Code
		err     error
	}{
		{
			nama:    "produk bukan produk kredit",
			pesanID: "produk ABC bukan produk kredit/pembiayaan",
			kode:    i18n.MsgNotLoanProduct,
			err:     domain.NotLoanProduct("ABC"),
		},
		{
			nama:    "metode pembayaran tidak dikenal",
			pesanID: "metode pembayaran \"TRANSFER_PALSU\" tidak dikenal",
			kode:    i18n.MsgPaymentMethodUnknown,
			err:     domain.PaymentMethodUnknown("TRANSFER_PALSU"),
		},
		{
			nama:    "kredit tidak terhubung ke produk",
			pesanID: "kredit tidak terhubung ke produk",
			kode:    i18n.MsgLoanProductMissing,
			err:     domain.ErrLoanProductMissing,
		},
		{
			nama:    "hanya kredit aktif dapat dihapus buku",
			pesanID: "hanya kredit aktif yang dapat dihapus buku",
			kode:    i18n.MsgWriteOffOnlyActive,
			err:     domain.ErrWriteOffOnlyActive,
		},
		{
			nama:    "kredit tanpa sisa pokok",
			pesanID: "kredit tidak memiliki sisa pokok yang dapat dihapus buku",
			kode:    i18n.MsgWriteOffNoPrincipal,
			err:     domain.ErrWriteOffNoPrincipal,
		},
		{
			nama:    "koreksi di bawah pokok dibayar",
			pesanID: "nominal baru lebih kecil daripada pokok yang sudah dibayar",
			kode:    i18n.MsgCorrectionBelowPaidPrincipal,
			err:     domain.ErrCorrectionBelowPaidPrincipal,
		},
		{
			nama:    "koreksi di bawah pokok jadwal",
			pesanID: "nominal baru lebih kecil daripada pokok jadwal yang sudah dibayar",
			kode:    i18n.MsgCorrectionBelowScheduled,
			err:     domain.ErrCorrectionBelowScheduled,
		},
		{
			nama:    "produk tidak untuk rekening simpanan",
			pesanID: "produk SIMP-01 tidak untuk pembukaan rekening simpanan",
			kode:    i18n.MsgProductNotForSavings,
			err:     domain.ProductNotForSavings("SIMP-01"),
		},
		{
			nama:    "cabang sedang tidak aktif",
			pesanID: "cabang 002 sedang tidak aktif",
			kode:    i18n.MsgBranchInactive,
			err:     domain.BranchInactive("002"),
		},
		{
			nama:    "produk sedang tidak aktif",
			pesanID: "produk KRD-FLAT sedang tidak aktif",
			kode:    i18n.MsgProductInactive,
			err:     domain.ProductInactive("KRD-FLAT"),
		},
		{
			// Data di TENGAH pesan: memakai sentinel berplaceholder dengan argumen,
			// sehingga ID tetap "akun COA 10999 tidak ditemukan" dan EN menyusun
			// sendiri di sekitar kode akun.
			nama:    "akun COA tidak ditemukan",
			pesanID: "akun COA 10999 tidak ditemukan",
			kode:    i18n.MsgCOAAccountNotFound,
			err:     domain.COAAccountNotFound("10999"),
		},
		{
			nama:    "instruksi ARO tidak dikenal",
			pesanID: "instruksi ARO tidak dikenal",
			kode:    i18n.MsgAROInstructionUnknown,
			err:     domain.ErrAROInstructionUnknown,
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
			// Pesan EN harus memakai katalog. Untuk pesan berplaceholder, bandingkan
			// dengan katalog EN yang sudah diisi argumen yang sama.
			args := []any{}
			if ap, ok := k.err.(interface{ MessageArgs() []any }); ok {
				args = ap.MessageArgs()
			}
			var mauEN string
			if len(args) > 0 {
				mauEN = i18n.Textf(k.kode, args...)
			} else {
				mauEN = i18n.T(i18n.EN, k.kode)
			}
			if en != mauEN {
				t.Fatalf("pesan EN tidak sesuai katalog:\n  dapat %q\n  mau   %q", en, mauEN)
			}
			// Tidak boleh ada sisa teks Indonesia pada pesan EN (kecuali data dinamis
			// seperti kode kategori/produk yang memang bukan bahasa).
			if len(args) == 0 && strings.Contains(en, i18n.T(i18n.ID, k.kode)) {
				t.Fatalf("pesan EN masih berbahasa Indonesia: %q", en)
			}
		})
	}
}
