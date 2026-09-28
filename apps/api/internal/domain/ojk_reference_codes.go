package domain

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ojk_reference_codes.go membuka pengisian sandi referensi/inline OJK lewat API
// untuk kolom yang sebelumnya hanya dapat diisi lewat SQL/seed (migrasi 000104 dan
// 000105):
//
//   - customers.ojk_pihak_lawan_code    (Form 06.00 XVIII, FK ojk_pihak_lawan)
//   - customers.ojk_sektor_ekonomi_code (Form 06.00 XX, FK ojk_sektor_ekonomi)
//   - customers.ojk_hubungan_bank_code  (Form 06.00 IX, inline 11/12/20)
//   - loans.ojk_jenis_penggunaan_code   (Form 06.00 VIII, inline 10/20/31/32/35/39)
//   - loans.ojk_periode_pembayaran_code (Form 06.00 XI, inline 1..8)
//   - loans.ojk_kabupaten_code          (Form 06.00 XXII, FK ojk_kabupaten)
//
// Sandi inline divalidasi terhadap himpunan yang diizinkan di bawah ini (konstanta
// Go, bukan literal yang tersebar). Sandi referensi wajib ada di tabel ojk_* dan
// diperiksa service lewat foreign key serta pemeriksaan eksplisit. Tidak ada nilai
// yang dikarang: kosong berarti "belum diisi" dan laporan menulis "-".

// OJKReferenceTable adalah nama tabel referensi sandi OJK (Lampiran 02/03/05 SEOJK
// 16/2024). Nilainya dipakai memilih query dan pesan galat; bukan masukan pengguna.
type OJKReferenceTable string

const (
	OJKRefPihakLawan    OJKReferenceTable = "ojk_pihak_lawan"
	OJKRefSektorEkonomi OJKReferenceTable = "ojk_sektor_ekonomi"
	OJKRefKabupaten     OJKReferenceTable = "ojk_kabupaten"
)

// Nama kolom sandi OJK dipakai pada pesan galat agar pengguna tahu sandi mana yang
// bermasalah. Nilai ini sama dengan nama kolom di database, bukan teks tampilan.
const (
	OJKJenisPenggunaanField   = "ojk_jenis_penggunaan_code"
	OJKPeriodePembayaranField = "ojk_periode_pembayaran_code"
	OJKKabupatenField         = "ojk_kabupaten_code"
	OJKPihakLawanField        = "ojk_pihak_lawan_code"
	OJKSektorEkonomiField     = "ojk_sektor_ekonomi_code"
	OJKHubunganBankField      = "ojk_hubungan_bank_code"
	// Kolom K1/K2 Form 06.00 (migrasi 000107-000111) yang kini dapat diisi lewat API.
	OJKKelompokKreditField                        = "ojk_kelompok_kredit_code"
	OJKSumberDanaField                            = "ojk_sumber_dana_code"
	OJKKategoriUsahaField                         = "ojk_kategori_usaha_code"
	OJKSifatKreditField                           = "ojk_sifat_kredit_code"
	OJKPenjaminField                              = "ojk_penjamin_code"
	OJKPenjaminBagianPctField                     = "ojk_penjamin_bagian_pct"
	OJKTanggalMulaiMacetField                     = "ojk_tanggal_mulai_macet"
	OJKKlasifikasiAsetField                       = "ojk_klasifikasi_aset_code"
	OJKAgunanPPKAAmountField                      = "ojk_agunan_ppka_amount"
	OJKKelonggaranTarikAmountField                = "ojk_kelonggaran_tarik_amount"
	OJKProvisiBelumDiamortisasiAmountField        = "ojk_provisi_belum_diamortisasi_amount"
	OJKBiayaTransaksiBelumDiamortisasiAmountField = "ojk_biaya_transaksi_belum_diamortisasi_amount"
	OJKPendapatanBungaDitangguhkanAmountField     = "ojk_pendapatan_bunga_ditangguhkan_amount"
	OJKCadanganKerugianRestrukturisasiAmountField = "ojk_cadangan_kerugian_restrukturisasi_amount"
)

// Himpunan sandi inline Form 06.00-2 (Lampiran II SEOJK 16/2024). Peta membuat
// pemeriksaan O(1) dan menghindari literal yang tersebar di validasi.
var (
	ojkJenisPenggunaanAllowed = map[string]struct{}{
		"10": {}, "20": {}, "31": {}, "32": {}, "35": {}, "39": {},
	}
	ojkPeriodePembayaranAllowed = map[string]struct{}{
		"1": {}, "2": {}, "3": {}, "4": {}, "5": {}, "6": {}, "7": {}, "8": {},
	}
	ojkHubunganBankAllowed = map[string]struct{}{
		"11": {}, "12": {}, "20": {},
	}
	// Sandi inline K1/K2 Form 06.00 (Lampiran II SEOJK 16/2024, migrasi 000107).
	ojkSumberDanaAllowed = map[string]struct{}{
		"10": {}, "21": {}, "22": {}, "31": {}, "32": {},
	}
	ojkKategoriUsahaAllowed = map[string]struct{}{
		"1": {}, "2": {}, "3": {}, "4": {},
	}
	ojkSifatKreditAllowed = map[string]struct{}{
		"2": {}, "9": {},
	}
	// Hubungan dengan Bank khusus penempatan pada bank lain (Form 05.00-2): 12 terkait,
	// 20 tidak terkait. Berbeda dari nasabah (11/12/20) sehingga dipisah.
	ojkHubunganBankPenempatanAllowed = map[string]struct{}{
		"12": {}, "20": {},
	}
)

// validateOJKInline menerima nilai kosong (belum diisi) atau sandi yang ada pada
// himpunan yang diizinkan. Spasi tepi dibuang lebih dulu.
func validateOJKInline(raw string, allowed map[string]struct{}, field string) error {
	code := strings.TrimSpace(raw)
	if code == "" {
		return nil
	}
	if _, ok := allowed[code]; !ok {
		return OJKInlineCodeInvalid(field)
	}
	return nil
}

// ValidateOJKHubunganBankCode memeriksa sandi inline Hubungan dengan Bank nasabah
// (Form 06.00 kolom IX: 11/12/20).
func ValidateOJKHubunganBankCode(raw string) error {
	return validateOJKInline(raw, ojkHubunganBankAllowed, OJKHubunganBankField)
}

// ParseOJKAmount mengubah masukan nominal bertipe string menjadi nilai nullable.
// String kosong berarti "kosongkan" (NULL); selain itu wajib angka dan tidak negatif.
// Satu sumber aturan dipakai validasi maupun penyimpanan.
func ParseOJKAmount(field, raw string) (decimal.NullDecimal, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return decimal.NullDecimal{}, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil || d.IsNegative() {
		return decimal.NullDecimal{}, OJKAmountInvalid(field)
	}
	return decimal.NullDecimal{Decimal: d, Valid: true}, nil
}

// ParseOJKPercentage mengubah masukan persentase bertipe string menjadi nilai
// nullable. Rentang 0-100 inklusif, paling banyak 2 digit desimal.
func ParseOJKPercentage(field, raw string) (decimal.NullDecimal, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return decimal.NullDecimal{}, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil || d.IsNegative() || d.GreaterThan(decimal.NewFromInt(100)) || !d.Equal(d.Round(2)) {
		return decimal.NullDecimal{}, OJKPercentageInvalid(field)
	}
	return decimal.NullDecimal{Decimal: d, Valid: true}, nil
}

// ParseOJKDate mengubah masukan tanggal bertipe string (YYYY-MM-DD) menjadi nilai
// nullable. String kosong berarti "kosongkan" (NULL).
func ParseOJKDate(field, raw string) (*time.Time, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, OJKDateInvalid(field)
	}
	return &t, nil
}

// UpdateOJKLoanCodesInput adalah isi PUT sandi OJK per kredit. Semua bidang bertipe
// pointer-string nullable dengan semantik seragam: absen/null berarti "jangan ubah";
// string kosong berarti "kosongkan" (NULL); selain itu divalidasi lalu disimpan.
// Nominal, persentase, dan tanggal juga dikirim sebagai string agar semantiknya sama.
type UpdateOJKLoanCodesInput struct {
	OJKJenisPenggunaanCode                   *string `json:"ojk_jenis_penggunaan_code,omitempty"`
	OJKPeriodePembayaranCode                 *string `json:"ojk_periode_pembayaran_code,omitempty"`
	OJKKabupatenCode                         *string `json:"ojk_kabupaten_code,omitempty"`
	OJKKelompokKreditCode                    *string `json:"ojk_kelompok_kredit_code,omitempty"`
	OJKSumberDanaCode                        *string `json:"ojk_sumber_dana_code,omitempty"`
	OJKKategoriUsahaCode                     *string `json:"ojk_kategori_usaha_code,omitempty"`
	OJKSifatKreditCode                       *string `json:"ojk_sifat_kredit_code,omitempty"`
	OJKPenjaminCode                          *string `json:"ojk_penjamin_code,omitempty"`
	OJKPenjaminBagianPct                     *string `json:"ojk_penjamin_bagian_pct,omitempty"`
	OJKTanggalMulaiMacet                     *string `json:"ojk_tanggal_mulai_macet,omitempty"`
	OJKAgunanPPKAAmount                      *string `json:"ojk_agunan_ppka_amount,omitempty"`
	OJKKelonggaranTarikAmount                *string `json:"ojk_kelonggaran_tarik_amount,omitempty"`
	OJKProvisiBelumDiamortisasiAmount        *string `json:"ojk_provisi_belum_diamortisasi_amount,omitempty"`
	OJKBiayaTransaksiBelumDiamortisasiAmount *string `json:"ojk_biaya_transaksi_belum_diamortisasi_amount,omitempty"`
	OJKPendapatanBungaDitangguhkanAmount     *string `json:"ojk_pendapatan_bunga_ditangguhkan_amount,omitempty"`
	OJKCadanganKerugianRestrukturisasiAmount *string `json:"ojk_cadangan_kerugian_restrukturisasi_amount,omitempty"`
	OJKKlasifikasiAsetCode                   *string `json:"ojk_klasifikasi_aset_code,omitempty"`
}

// IsEmpty melaporkan apakah tidak ada satu bidang pun yang dikirim.
func (in UpdateOJKLoanCodesInput) IsEmpty() bool {
	return in.OJKJenisPenggunaanCode == nil &&
		in.OJKPeriodePembayaranCode == nil &&
		in.OJKKabupatenCode == nil &&
		in.OJKKelompokKreditCode == nil &&
		in.OJKSumberDanaCode == nil &&
		in.OJKKategoriUsahaCode == nil &&
		in.OJKSifatKreditCode == nil &&
		in.OJKPenjaminCode == nil &&
		in.OJKPenjaminBagianPct == nil &&
		in.OJKTanggalMulaiMacet == nil &&
		in.OJKAgunanPPKAAmount == nil &&
		in.OJKKelonggaranTarikAmount == nil &&
		in.OJKProvisiBelumDiamortisasiAmount == nil &&
		in.OJKBiayaTransaksiBelumDiamortisasiAmount == nil &&
		in.OJKPendapatanBungaDitangguhkanAmount == nil &&
		in.OJKCadanganKerugianRestrukturisasiAmount == nil &&
		in.OJKKlasifikasiAsetCode == nil
}

// Validate memeriksa sandi inline serta nominal/persentase/tanggal yang dikirim.
// Sandi referensi (kabupaten/penjamin) tidak diperiksa di sini karena harus
// dibandingkan dengan tabel referensi lewat service.
func (in UpdateOJKLoanCodesInput) Validate() error {
	inline := []struct {
		field   string
		value   *string
		allowed map[string]struct{}
	}{
		{OJKJenisPenggunaanField, in.OJKJenisPenggunaanCode, ojkJenisPenggunaanAllowed},
		{OJKPeriodePembayaranField, in.OJKPeriodePembayaranCode, ojkPeriodePembayaranAllowed},
		{OJKSumberDanaField, in.OJKSumberDanaCode, ojkSumberDanaAllowed},
		{OJKKategoriUsahaField, in.OJKKategoriUsahaCode, ojkKategoriUsahaAllowed},
		{OJKSifatKreditField, in.OJKSifatKreditCode, ojkSifatKreditAllowed},
	}
	for _, item := range inline {
		if item.value == nil {
			continue
		}
		if err := validateOJKInline(*item.value, item.allowed, item.field); err != nil {
			return err
		}
	}

	if in.OJKPenjaminBagianPct != nil {
		if _, err := ParseOJKPercentage(OJKPenjaminBagianPctField, *in.OJKPenjaminBagianPct); err != nil {
			return err
		}
	}
	if in.OJKTanggalMulaiMacet != nil {
		if _, err := ParseOJKDate(OJKTanggalMulaiMacetField, *in.OJKTanggalMulaiMacet); err != nil {
			return err
		}
	}
	amounts := []struct {
		field string
		value *string
	}{
		{OJKAgunanPPKAAmountField, in.OJKAgunanPPKAAmount},
		{OJKKelonggaranTarikAmountField, in.OJKKelonggaranTarikAmount},
		{OJKProvisiBelumDiamortisasiAmountField, in.OJKProvisiBelumDiamortisasiAmount},
		{OJKBiayaTransaksiBelumDiamortisasiAmountField, in.OJKBiayaTransaksiBelumDiamortisasiAmount},
		{OJKPendapatanBungaDitangguhkanAmountField, in.OJKPendapatanBungaDitangguhkanAmount},
		{OJKCadanganKerugianRestrukturisasiAmountField, in.OJKCadanganKerugianRestrukturisasiAmount},
	}
	for _, item := range amounts {
		if item.value == nil {
			continue
		}
		if _, err := ParseOJKAmount(item.field, *item.value); err != nil {
			return err
		}
	}
	return nil
}

// OJKLoanCodesStore membaca kredit dan menulis sandi OJK-nya. Implementasinya adalah
// LoanRepository postgres yang sama; interface sempit ini menjaga agar layanan sandi
// tidak bergantung pada seluruh domain.LoanRepository.
type OJKLoanCodesStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (*Loan, error)
	// UpdateOJKLoanCodesTx menyimpan hanya sandi yang dikirim (pointer non-nil) di
	// dalam transaksi pemanggil. Pointer nil tidak menyentuh kolomnya.
	UpdateOJKLoanCodesTx(ctx context.Context, tx any, id uuid.UUID, input UpdateOJKLoanCodesInput) error
}

// OJKLoanCodesService mengelola sandi referensi/inline OJK per kredit: memvalidasi,
// menyimpan, dan mengaudit dalam satu transaksi.
type OJKLoanCodesService interface {
	// Update mengubah sandi OJK satu kredit dan mengembalikan kredit yang sudah
	// diperbarui. Kredit di luar cakupan unit aktor ditolak ErrCrossBranchAccess.
	Update(ctx context.Context, loanID uuid.UUID, input UpdateOJKLoanCodesInput, actor Actor) (*Loan, error)
}
