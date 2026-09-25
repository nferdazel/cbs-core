package domain

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ckpn_activation.go menyediakan pintu masuk PENGATURAN jalur CKPN tanpa SQL: parameter
// PD/LGD, pemetaan akun syariah, status parameter SEMENTARA/FINAL, bukti ratifikasi, mode
// bayangan, dan saklar ckpn.enabled.
//
// Ini BUKAN mesin CKPN. Berkas ini hanya memindahkan pengisian kunci yang sebelumnya
// mengharuskan SQL ke jalur berizin + teraudit, dengan validasi yang sama seperti yang
// sudah dipakai mesin CKPN (CKPNParametersStatusFromConfig/CKPNRatificationReadiness).
// Tidak ada nilai yang dikarang: seluruh isian adalah data bank.
//
// Prinsip keselamatan:
//   - Menyalakan ckpn.enabled HANYA boleh bila tidak ada penahan yang dapat diperiksa
//     mesin (parameter FINAL, PD/LGD terisi sah, akun syariah bila relevan). Ini
//     mencegah CKPN menyala dengan parameter sementara atau setengah terisi.
//   - Status FINAL HANYA boleh disetel bila bukti ratifikasi lengkap. Sistem tidak
//     dapat meratifikasi atas nama bank; ia hanya memeriksa kelengkapan.
//   - Tanggal tidak boleh di masa depan dan wajib format YYYY-MM-DD.

const (
	// CKPNLGDFracKey adalah kunci fraksi LGD. PD per golongan memakai CKPNPDFracKey.
	CKPNLGDFracKey = "ckpn.lgd_frac"
	// CKPNCOAExpenseSyariahKey / CKPNCOAReserveSyariahKey memetakan akun jurnal CKPN
	// untuk buku syariah; kosong membuat jurnal syariah jatuh ke akun konvensional.
	CKPNCOAExpenseSyariahKey = "ckpn.coa.expense.syariah"
	CKPNCOAReserveSyariahKey = "ckpn.coa.reserve.syariah"
	// CKPNShadowModeEnabledKey menyalakan mode bayangan (hitung tanpa jurnal).
	CKPNShadowModeEnabledKey = "ckpn.shadow_mode.enabled"
)

// CKPNPDFracKey membangun kunci fraksi PD untuk golongan kualitas 1..5.
func CKPNPDFracKey(golongan int) string {
	return fmt.Sprintf("ckpn.pd_frac.gol_%d", golongan)
}

// CKPNActivationKeys adalah seluruh kunci yang dikelola pintu pengaturan ini, berurutan
// agar keluaran stabil. Kunci di luar daftar ini (mis. ckpn.pabl.*, ckpn.individual.*)
// TIDAK disentuh: jalur aktivasi dasar tidak boleh menimpa setelan lanjutan.
func CKPNActivationKeys() []string {
	keys := []string{
		CKPNPDFracKey(1), CKPNPDFracKey(2), CKPNPDFracKey(3), CKPNPDFracKey(4), CKPNPDFracKey(5),
		CKPNLGDFracKey,
		CKPNCOAExpenseSyariahKey, CKPNCOAReserveSyariahKey,
		ConfigKeyCKPNParametersStatus,
		ConfigKeyCKPNParametersSince,
		ConfigKeyCKPNRatificationBANumber,
		ConfigKeyCKPNRatificationBADate,
		ConfigKeyCKPNRatificationApprovedBy,
		ConfigKeyCKPNRatificationPDLGDBasis,
		ConfigKeyCKPNRatificationPDLGDFromBank,
		CKPNShadowModeEnabledKey,
		ConfigKeyCKPNEnabled,
	}
	return keys
}

// ErrCKPNActivationEmpty menolak PUT tanpa satu bidang pun, agar aksi kosong tidak
// tercatat sebagai perubahan.
var ErrCKPNActivationEmpty = NewLocalizedError("ckpn_activation_empty", "tidak ada bidang aktivasi CKPN yang dikirim")

// ErrCKPNActivationFractionInvalid menolak fraksi PD/LGD yang bukan angka atau di luar 0..1.
var ErrCKPNActivationFractionInvalid = NewLocalizedError("ckpn_activation_fraction_invalid", "fraksi PD/LGD harus angka 0..1 (satuan fraksi, bukan persen)")

// ErrCKPNActivationStatusInvalid menolak status parameter selain SEMENTARA/FINAL.
var ErrCKPNActivationStatusInvalid = NewLocalizedError("ckpn_activation_status_invalid", "status parameter CKPN hanya boleh SEMENTARA atau FINAL")

// ErrCKPNActivationRatificationIncomplete menolak penyetelan FINAL tanpa bukti ratifikasi.
var ErrCKPNActivationRatificationIncomplete = NewLocalizedError("ckpn_activation_ratification_incomplete", "status FINAL belum boleh disetel: bukti ratifikasi parameter CKPN belum lengkap")

// ErrCKPNActivationNotReady menolak penyalakan ckpn.enabled selama masih ada penahan.
var ErrCKPNActivationNotReady = NewLocalizedError("ckpn_activation_not_ready", "CKPN belum boleh dinyalakan: masih ada penahan yang harus diselesaikan")

// ErrCKPNActivationDateInvalid menolak tanggal yang bukan YYYY-MM-DD atau di masa depan.
var ErrCKPNActivationDateInvalid = NewLocalizedError("ckpn_activation_date_invalid", "tanggal harus format YYYY-MM-DD dan tidak boleh di masa depan")

// CKPNActivation adalah snapshot baca-saja pengaturan aktivasi CKPN: nilai tiap kunci
// yang dikelola beserta status parameter dan kesiapan penyalakan. Nilai kosong berarti
// "belum diisi", bukan nol.
type CKPNActivation struct {
	// Values memetakan kunci konfigurasi ke nilainya (string apa adanya).
	Values map[string]string `json:"values"`
	// Status adalah ringkasan parameter (SEMENTARA/FINAL), batas ratifikasi, blokir
	// ekspor OJK, dan peringatan yang sama dengan yang muncul saat start/EOD.
	Status CKPNParametersStatus `json:"status"`
	// EnablementReady true berarti tidak ada penahan penyalakan yang dapat diperiksa
	// mesin; menyalakan ckpn.enabled akan diterima.
	EnablementReady bool `json:"enablement_ready"`
	// EnablementGaps menyebut penahan penyalakan yang tersisa (kosong = siap).
	EnablementGaps []string `json:"enablement_gaps,omitempty"`
}

// UpdateCKPNActivationInput adalah isi PUT pengaturan aktivasi. Pointer nil berarti
// "jangan ubah"; string kosong berarti "kosongkan". Semua bidang opsional, sehingga
// klien web dapat mengirim satu bidang pada satu waktu.
type UpdateCKPNActivationInput struct {
	PDFracGol1                *string `json:"pd_frac_gol_1"`
	PDFracGol2                *string `json:"pd_frac_gol_2"`
	PDFracGol3                *string `json:"pd_frac_gol_3"`
	PDFracGol4                *string `json:"pd_frac_gol_4"`
	PDFracGol5                *string `json:"pd_frac_gol_5"`
	LGDFrac                   *string `json:"lgd_frac"`
	COAExpenseSyariah         *string `json:"coa_expense_syariah"`
	COAReserveSyariah         *string `json:"coa_reserve_syariah"`
	ParametersStatus          *string `json:"parameters_status"`
	ParametersTemporarySince  *string `json:"parameters_temporary_since"`
	RatificationBANumber      *string `json:"ratification_ba_number"`
	RatificationBADate        *string `json:"ratification_ba_date"`
	RatificationApprovedBy    *string `json:"ratification_approved_by"`
	RatificationPDLGDBasis    *string `json:"ratification_pd_lgd_basis"`
	RatificationPDLGDFromBank *bool   `json:"ratification_pd_lgd_from_bank"`
	ShadowModeEnabled         *bool   `json:"shadow_mode_enabled"`
	CKPNEnabled               *bool   `json:"ckpn_enabled"`
}

// IsEmpty melaporkan apakah tidak ada satu bidang pun yang dikirim.
func (in UpdateCKPNActivationInput) IsEmpty() bool {
	return in.PDFracGol1 == nil && in.PDFracGol2 == nil && in.PDFracGol3 == nil &&
		in.PDFracGol4 == nil && in.PDFracGol5 == nil && in.LGDFrac == nil &&
		in.COAExpenseSyariah == nil && in.COAReserveSyariah == nil &&
		in.ParametersStatus == nil && in.ParametersTemporarySince == nil &&
		in.RatificationBANumber == nil && in.RatificationBADate == nil &&
		in.RatificationApprovedBy == nil && in.RatificationPDLGDBasis == nil &&
		in.RatificationPDLGDFromBank == nil && in.ShadowModeEnabled == nil &&
		in.CKPNEnabled == nil
}

// Changes mengembalikan peta kunci yang HARUS berubah beserta nilai barunya. Bidang nil
// dilewati; bidang yang nilainya sama dengan keadaan sekarang juga dilewati supaya audit
// tidak mencatat perubahan semu.
func (in UpdateCKPNActivationInput) Changes(current map[string]string) map[string]string {
	out := map[string]string{}
	set := func(key string, v *string) {
		if v != nil && !strings.EqualFold(strings.TrimSpace(*v), current[key]) {
			out[key] = strings.TrimSpace(*v)
		}
	}
	set(CKPNPDFracKey(1), in.PDFracGol1)
	set(CKPNPDFracKey(2), in.PDFracGol2)
	set(CKPNPDFracKey(3), in.PDFracGol3)
	set(CKPNPDFracKey(4), in.PDFracGol4)
	set(CKPNPDFracKey(5), in.PDFracGol5)
	set(CKPNLGDFracKey, in.LGDFrac)
	set(CKPNCOAExpenseSyariahKey, in.COAExpenseSyariah)
	set(CKPNCOAReserveSyariahKey, in.COAReserveSyariah)
	// Status dinormalkan ke huruf besar supaya "final" tidak tersimpan sebagai nilai
	// tak dikenal yang diperlakukan SEMENTARA.
	if in.ParametersStatus != nil {
		v := strings.ToUpper(strings.TrimSpace(*in.ParametersStatus))
		if v != current[ConfigKeyCKPNParametersStatus] {
			out[ConfigKeyCKPNParametersStatus] = v
		}
	}
	set(ConfigKeyCKPNParametersSince, in.ParametersTemporarySince)
	set(ConfigKeyCKPNRatificationBANumber, in.RatificationBANumber)
	set(ConfigKeyCKPNRatificationBADate, in.RatificationBADate)
	set(ConfigKeyCKPNRatificationApprovedBy, in.RatificationApprovedBy)
	set(ConfigKeyCKPNRatificationPDLGDBasis, in.RatificationPDLGDBasis)
	setBool := func(key string, v *bool) {
		if v != nil {
			raw := strconv.FormatBool(*v)
			if raw != current[key] {
				out[key] = raw
			}
		}
	}
	setBool(ConfigKeyCKPNRatificationPDLGDFromBank, in.RatificationPDLGDFromBank)
	setBool(CKPNShadowModeEnabledKey, in.ShadowModeEnabled)
	setBool(ConfigKeyCKPNEnabled, in.CKPNEnabled)
	return out
}

// ValidateCKPNActivation memeriksa hasil AKHIR (nilai lama yang tidak diubah + perubahan
// yang dikirim) sebelum disimpan. candidate adalah peta lengkap seluruh kunci yang
// dikelola (kunci tak ada = kosong). now dipakai menolak tanggal di masa depan.
//
// Urutan pemeriksaan penting: bentuk (fraksi/tanggal/status) diperiksa sebelum
// konsekuensinya (FINAL/nyala), sehingga pengguna melihat kesalahan ketik lebih dulu.
func ValidateCKPNActivation(candidate map[string]string, now time.Time) error {
	for i := 1; i <= 5; i++ {
		if err := validateOptionalFraction(candidate[CKPNPDFracKey(i)]); err != nil {
			return fmt.Errorf("%w: %s", ErrCKPNActivationFractionInvalid, CKPNPDFracKey(i))
		}
	}
	if err := validateOptionalFraction(candidate[CKPNLGDFracKey]); err != nil {
		return fmt.Errorf("%w: %s", ErrCKPNActivationFractionInvalid, CKPNLGDFracKey)
	}

	if err := validateOptionalPastDate(candidate[ConfigKeyCKPNParametersSince], now); err != nil {
		return fmt.Errorf("%w: %s", ErrCKPNActivationDateInvalid, ConfigKeyCKPNParametersSince)
	}
	if err := validateOptionalPastDate(candidate[ConfigKeyCKPNRatificationBADate], now); err != nil {
		return fmt.Errorf("%w: %s", ErrCKPNActivationDateInvalid, ConfigKeyCKPNRatificationBADate)
	}

	status := strings.ToUpper(strings.TrimSpace(candidate[ConfigKeyCKPNParametersStatus]))
	switch status {
	case "", CKPNParameterStatusSementara, CKPNParameterStatusFinal:
	default:
		return fmt.Errorf("%w: %q", ErrCKPNActivationStatusInvalid, candidate[ConfigKeyCKPNParametersStatus])
	}

	snapshot := NewConfigSnapshot(candidate)
	if status == CKPNParameterStatusFinal {
		if ready, missing := CKPNRatificationReadiness(context.Background(), snapshot, now); !ready {
			return fmt.Errorf("%w: %s", ErrCKPNActivationRatificationIncomplete, strings.Join(missing, "; "))
		}
	}

	// Penyalakan dinilai TANPA memandang saklar saat ini: kalau ckpn.enabled sudah true
	// di keadaan sekarang, penahan tetap harus terlihat agar perubahan lain (mis.
	// mengosongkan PD) tidak lolos begitu saja saat CKPN menyala.
	if parseBoolOrZero(candidate[ConfigKeyCKPNEnabled]) {
		if gaps := CKPNEnablementGapsForCandidate(context.Background(), snapshot); len(gaps) > 0 {
			return fmt.Errorf("%w: %s", ErrCKPNActivationNotReady, strings.Join(gaps, "; "))
		}
	}
	return nil
}

// validateOptionalFraction menerima nilai kosong (belum diisi) atau desimal 0..1.
func validateOptionalFraction(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return err
	}
	if d.IsNegative() || d.GreaterThan(decimal.NewFromInt(1)) {
		return fmt.Errorf("nilai %s di luar 0..1", raw)
	}
	return nil
}

// validateOptionalPastDate menerima nilai kosong atau tanggal YYYY-MM-DD yang tidak
// berada di masa depan.
func validateOptionalPastDate(raw string, now time.Time) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return err
	}
	if t.After(tanggalSaja(now)) {
		return fmt.Errorf("tanggal %s di masa depan", raw)
	}
	return nil
}

// parseBoolOrZero membaca boolean dengan gagal-aman: nilai tak dikenal dianggap false.
func parseBoolOrZero(raw string) bool {
	b, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return b
}

// CKPNActivationStore membaca dan menulis kunci pengaturan aktivasi CKPN. Seluruh
// penulisan terjadi di dalam transaksi yang sama dengan audit pemanggil.
type CKPNActivationStore interface {
	// Get membaca nilai kunci yang dikelola (kunci tak ada = kosong).
	Get(ctx context.Context) (map[string]string, error)
	// GetTx membaca di dalam transaksi tulis agar nilai "sebelum" pada audit benar.
	GetTx(ctx context.Context, tx any) (map[string]string, error)
	// SaveTx menyimpan perubahan kunci (upsert) di dalam transaksi tulis.
	SaveTx(ctx context.Context, tx any, changes map[string]string, updatedBy uuid.UUID) error
}

// CKPNActivationService mengelola pengaturan aktivasi CKPN.
type CKPNActivationService interface {
	// Get mengembalikan nilai terkini beserta status dan penahan penyalakan.
	Get(ctx context.Context) (*CKPNActivation, error)
	// Update memvalidasi, menyimpan, dan mengaudit perubahan dalam satu transaksi.
	Update(ctx context.Context, input UpdateCKPNActivationInput, actor Actor) (*CKPNActivation, error)
}

// ConfigSnapshot adalah SystemConfigService in-memory dari sekumpulan nilai. Dipakai
// menilai KANDIDAT konfigurasi sebelum disimpan (validasi pengaturan) dan pada uji.
// Parsing mengikuti config_service: nilai kosong/tak sah jatuh ke fallback, gagal-aman.
type ConfigSnapshot struct {
	values map[string]string
}

// NewConfigSnapshot membungkus peta nilai menjadi SystemConfigService baca-saja.
func NewConfigSnapshot(values map[string]string) SystemConfigService {
	copied := make(map[string]string, len(values))
	for k, v := range values {
		copied[k] = v
	}
	return ConfigSnapshot{values: copied}
}

func (c ConfigSnapshot) GetString(_ context.Context, key, fallback string) string {
	if v, ok := c.values[key]; ok {
		return v
	}
	return fallback
}

func (c ConfigSnapshot) GetDecimal(_ context.Context, key string, fallback decimal.Decimal) decimal.Decimal {
	raw, ok := c.values[key]
	if !ok {
		return fallback
	}
	d, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return d
}

func (c ConfigSnapshot) GetInt(_ context.Context, key string, fallback int) int {
	raw, ok := c.values[key]
	if !ok {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return n
}

func (c ConfigSnapshot) GetBool(_ context.Context, key string, fallback bool) bool {
	raw, ok := c.values[key]
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return b
}

func (c ConfigSnapshot) Invalidate(string) {}
