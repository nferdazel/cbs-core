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

// Galat manajemen staf sebelumnya BERBAUR: sebagian pesannya berbahasa Inggris
// ("password must be at least 8 characters") padahal ditampilkan ke pengguna, sebagian
// lagi Indonesia. Uji ini mengunci keseragaman: pesan ID memakai bahasa Indonesia dan
// pesan EN memakai katalog, untuk kedua kelompok itu.
func TestPesanStaffIkutBahasaInstalasi(t *testing.T) {
	kasus := []struct {
		nama    string
		pesanID string
		kode    i18n.Code
		err     error
	}{
		{
			nama:    "kata sandi terlalu pendek",
			pesanID: "kata sandi minimal 8 karakter",
			kode:    i18n.MsgStaffPasswordTooShort,
			err:     domain.ErrStaffPasswordTooShort,
		},
		{
			nama:    "kata sandi lemah",
			pesanID: "kata sandi harus memuat huruf besar, huruf kecil, angka, dan karakter khusus",
			kode:    i18n.MsgStaffPasswordWeak,
			err:     domain.ErrStaffPasswordWeak,
		},
		{
			nama:    "akun sendiri tidak dapat dinonaktifkan",
			pesanID: "akun sendiri tidak dapat dinonaktifkan",
			kode:    i18n.MsgStaffSelfDeactivate,
			err:     domain.ErrStaffSelfDeactivate,
		},
		{
			nama:    "kata sandi saat ini salah",
			pesanID: "kata sandi saat ini salah",
			kode:    i18n.MsgStaffCurrentPassword,
			err:     domain.ErrStaffCurrentPassword,
		},
		{
			nama:    "peran istimewa lewat pembaruan",
			pesanID: "peran SUPERADMIN atau SYSTEM tidak dapat diberikan lewat pembaruan",
			kode:    i18n.MsgStaffPrivilegedRole,
			err:     domain.ErrStaffPrivilegedRole,
		},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			t.Setenv("CBS_LANGUAGE", "id")
			idRec := httptest.NewRecorder()
			httpHandler.Fail(idRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			if got := decodeError(t, idRec); got != k.pesanID {
				t.Fatalf("pesan ID tidak sesuai:\n  dapat %q\n  mau   %q", got, k.pesanID)
			}

			t.Setenv("CBS_LANGUAGE", "en")
			enRec := httptest.NewRecorder()
			httpHandler.Fail(enRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			if en := decodeError(t, enRec); en != i18n.T(i18n.EN, k.kode) {
				t.Fatalf("pesan EN tidak sesuai katalog:\n  dapat %q\n  mau   %q", en, i18n.T(i18n.EN, k.kode))
			}
			// Pesan EN tidak boleh lagi berbahasa Indonesia (regresi kelompok ini).
			if strings.Contains(decodeError(t, enRec), "kata sandi") ||
				strings.Contains(decodeError(t, enRec), "akun sendiri") {
				t.Fatalf("pesan EN masih berbahasa Indonesia: %q", decodeError(t, enRec))
			}
		})
	}
}

// Validasi masukan operator: koreksi jadwal angsuran dan parameter CKPN yang bukan
// angka. Keduanya diisi manusia lewat layar, jadi harus ikut bahasa instalasi.
func TestPesanValidasiOperatorIkutBahasaInstalasi(t *testing.T) {
	kasus := []struct {
		nama    string
		pesanID string
		kode    i18n.Code
		err     error
	}{
		{
			nama:    "tidak ada angsuran belum dibayar",
			pesanID: "tidak ada angsuran belum dibayar yang dapat disesuaikan",
			kode:    i18n.MsgNoUnpaidInstallment,
			err:     domain.ErrNoUnpaidInstallment,
		},
		{
			nama:    "parameter CKPN bukan angka desimal",
			pesanID: `nilai "abc" bukan angka desimal yang sah`,
			kode:    i18n.MsgCKPNValueNotDecimal,
			err:     domain.CKPNValueNotDecimal("abc"),
		},
		{
			// Sebelumnya BERBAHASA INGGRIS ("invalid collection_type").
			nama:    "jenis penagihan tidak dikenal",
			pesanID: "jenis penagihan tidak dikenal",
			kode:    i18n.MsgCollectionTypeUnknown,
			err:     domain.ErrCollectionTypeUnknown,
		},
		{
			// Sebelumnya BERBAHASA INGGRIS ("loan_id and installment_no are required...").
			nama:    "referensi kredit wajib untuk penagihan angsuran",
			pesanID: "loan_id dan installment_no wajib diisi untuk penagihan angsuran kredit",
			kode:    i18n.MsgCollectionLoanRefs,
			err:     domain.ErrCollectionLoanRefs,
		},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			t.Setenv("CBS_LANGUAGE", "id")
			idRec := httptest.NewRecorder()
			httpHandler.Fail(idRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			if got := decodeError(t, idRec); got != k.pesanID {
				t.Fatalf("pesan ID tidak sesuai:\n  dapat %q\n  mau   %q", got, k.pesanID)
			}

			t.Setenv("CBS_LANGUAGE", "en")
			enRec := httptest.NewRecorder()
			httpHandler.Fail(enRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
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
			en := decodeError(t, enRec)
			if en != mauEN {
				t.Fatalf("pesan EN tidak sesuai katalog:\n  dapat %q\n  mau   %q", en, mauEN)
			}
			// Penjaga yang tidak bergantung katalog itu sendiri: teks EN tidak boleh
			// memuat kata khas Indonesia. Tanpa ini, entri EN yang keliru diisi bahasa
			// Indonesia akan lolos karena pembandingnya katalog yang sama.
			for _, kata := range []string{
				"tidak ada", "angsuran", "nilai", "bukan angka",
				"tidak dikenal", "wajib diisi", "penagihan", "belum dibayar",
			} {
				if strings.Contains(en, kata) {
					t.Fatalf("pesan EN masih memuat kata Indonesia %q: %q", kata, en)
				}
			}
		})
	}
}

// Validasi NIK (masukan pengguna) dan konfigurasi batas transaksi (dibaca operator dari
// penolakan transaksi). Keduanya harus terbaca dalam bahasa instalasi; penjaga kata
// Indonesia dipakai supaya entri EN yang keliru tidak lolos membandingkan katalog
// dengan dirinya sendiri.
func TestPesanNIKDanBatasIkutBahasaInstalasi(t *testing.T) {
	kasus := []struct {
		nama    string
		pesanID string
		kode    i18n.Code
		err     error
		kataID  []string
	}{
		{
			nama:    "NIK kurang dari 16 digit",
			pesanID: "NIK harus 16 digit",
			kode:    i18n.MsgNIKTooShort,
			err:     domain.ErrNIKTooShort,
			// "NIK" dan "digit" sengaja TIDAK masuk daftar: keduanya juga kata Inggris
			// sah ("national ID number (NIK)", "16 digits"), jadi menandainya sebagai
			// kebocoran bahasa akan menghasilkan kegagalan palsu.
			kataID: []string{"harus", "wajib"},
		},
		{
			nama:    "konfigurasi batas belum diisi",
			pesanID: "konfigurasi batas limit.teller harian belum diisi; batas transaksi tidak boleh memakai angka bawaan",
			kode:    i18n.MsgLimitConfigMissing,
			err:     domain.LimitConfigMissing("limit.teller harian"),
			kataID:  []string{"konfigurasi", "batas", "belum diisi"},
		},
		{
			nama:    "konfigurasi batas bukan angka",
			pesanID: `konfigurasi batas limit.teller harian bernilai "abc", bukan angka; perbaiki nilainya sebelum bertransaksi`,
			kode:    i18n.MsgLimitConfigInvalid,
			err:     domain.LimitConfigInvalid("limit.teller harian", "abc"),
			kataID:  []string{"konfigurasi", "batas", "bukan angka"},
		},
	}
	for _, k := range kasus {
		t.Run(k.nama, func(t *testing.T) {
			t.Setenv("CBS_LANGUAGE", "id")
			idRec := httptest.NewRecorder()
			httpHandler.Fail(idRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			if got := decodeError(t, idRec); got != k.pesanID {
				t.Fatalf("pesan ID tidak sesuai:\n  dapat %q\n  mau   %q", got, k.pesanID)
			}

			t.Setenv("CBS_LANGUAGE", "en")
			enRec := httptest.NewRecorder()
			httpHandler.Fail(enRec, httptest.NewRequest(http.MethodPost, "/", nil),
				http.StatusUnprocessableEntity, k.err)
			en := decodeError(t, enRec)
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
			for _, kata := range k.kataID {
				if strings.Contains(en, kata) {
					t.Fatalf("pesan EN masih memuat kata Indonesia %q: %q", kata, en)
				}
			}
		})
	}
}
