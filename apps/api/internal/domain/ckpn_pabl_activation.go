package domain

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ckpn_pabl_activation.go menyediakan pintu masuk PENGATURAN enam kunci ckpn.pabl.*
// (migrasi 000099/000100) tanpa SQL. Jalur aktivasi utama (CKPNActivationKeys) sengaja
// TIDAK menyentuh kunci ini; rutenya terpisah agar setelan lanjutan tidak tertimpa.
//
// Ini BUKAN mesin CKPN per penempatan. Berkas ini hanya memindahkan pengisian kunci yang
// sebelumnya mengharuskan SQL ke jalur berizin + teraudit, dengan validasi yang sama
// seperti mesin kolektif PABL (validatePABLCKPNFraction) dan kesiapan CKPN utama.
//
// Prinsip keselamatan:
//   - Menyalakan ckpn.pabl.enabled HANYA boleh bila keempat fraksi PABL terisi sah DAN
//     tidak ada penahan CKPN utama. Form 05.00 kolom XII/XXI tidak boleh membawa nilai
//     yang berasal dari parameter SEMENTARA (kebijakan §1.5 di docs/KEPUTUSAN-OJK.md).
//   - Fraksi nol DITOLAK: nol berarti "belum diisi" dan mesin kolektif PABL menolaknya.
//   - Menonaktifkan (enabled=false) selalu boleh.

// ErrCKPNPABLActivationEmpty menolak PUT tanpa satu bidang pun, agar aksi kosong tidak
// tercatat sebagai perubahan.
var ErrCKPNPABLActivationEmpty = NewLocalizedError("ckpn_pabl_activation_empty", "tidak ada bidang pengaturan CKPN PABL yang dikirim")

// ErrCKPNPABLActivationNotReady menolak penyalakan ckpn.pabl.enabled selama masih ada
// penahan: fraksi PABL belum lengkap atau kesiapan CKPN utama belum terpenuhi.
var ErrCKPNPABLActivationNotReady = NewLocalizedError("ckpn_pabl_activation_not_ready", "CKPN PABL belum boleh dinyalakan: masih ada penahan yang harus diselesaikan")

// ckpnPABLFractionKeys mengembalikan empat kunci fraksi PABL dalam urutan stabil, dipakai
// validasi maupun daftar penahan.
func ckpnPABLFractionKeys() []string {
	return []string{
		CKPNPABLPDFracGol1Key, CKPNPABLPDFracGol3Key, CKPNPABLPDFracGol5Key, CKPNPABLLGDFracKey,
	}
}

// CKPNPABLActivationKeys adalah seluruh kunci yang dikelola pintu pengaturan ini,
// berurutan agar keluaran stabil.
func CKPNPABLActivationKeys() []string {
	return []string{
		CKPNPABLPDFracGol1Key,
		CKPNPABLPDFracGol3Key,
		CKPNPABLPDFracGol5Key,
		CKPNPABLLGDFracKey,
		CKPNPABLAsetBaikBentukCKPNKey,
		CKPNPABLEnabledKey,
	}
}

// CKPNPABLActivation adalah snapshot baca-saja pengaturan CKPN PABL: nilai tiap kunci
// yang dikelola beserta kesiapan penyalakan ckpn.pabl.enabled. Nilai kosong berarti
// "belum diisi", bukan nol.
type CKPNPABLActivation struct {
	// Values memetakan kunci konfigurasi ke nilainya (string apa adanya).
	Values map[string]string `json:"values"`
	// EnablementReady true berarti tidak ada penahan penyalakan yang dapat diperiksa
	// mesin; menyalakan ckpn.pabl.enabled akan diterima.
	EnablementReady bool `json:"enablement_ready"`
	// EnablementGaps menyebut penahan penyalakan yang tersisa (kosong = siap).
	EnablementGaps []string `json:"enablement_gaps,omitempty"`
}

// UpdateCKPNPABLActivationInput adalah isi PUT pengaturan CKPN PABL. Pointer nil berarti
// "jangan ubah"; string kosong berarti "kosongkan". Semua bidang opsional, sehingga klien
// web dapat mengirim satu bidang pada satu waktu.
type UpdateCKPNPABLActivationInput struct {
	PDFracGol1         *string `json:"pd_frac_gol_1"`
	PDFracGol3         *string `json:"pd_frac_gol_3"`
	PDFracGol5         *string `json:"pd_frac_gol_5"`
	LGDFrac            *string `json:"lgd_frac"`
	AsetBaikBentukCKPN *bool   `json:"aset_baik_bentuk_ckpn"`
	Enabled            *bool   `json:"enabled"`
}

// IsEmpty melaporkan apakah tidak ada satu bidang pun yang dikirim.
func (in UpdateCKPNPABLActivationInput) IsEmpty() bool {
	return in.PDFracGol1 == nil && in.PDFracGol3 == nil && in.PDFracGol5 == nil &&
		in.LGDFrac == nil && in.AsetBaikBentukCKPN == nil && in.Enabled == nil
}

// Changes mengembalikan peta kunci yang HARUS berubah beserta nilai barunya. Bidang nil
// dilewati; bidang yang nilainya sama dengan keadaan sekarang juga dilewati supaya audit
// tidak mencatat perubahan semu.
func (in UpdateCKPNPABLActivationInput) Changes(current map[string]string) map[string]string {
	out := map[string]string{}
	set := func(key string, v *string) {
		if v != nil && !strings.EqualFold(strings.TrimSpace(*v), current[key]) {
			out[key] = strings.TrimSpace(*v)
		}
	}
	set(CKPNPABLPDFracGol1Key, in.PDFracGol1)
	set(CKPNPABLPDFracGol3Key, in.PDFracGol3)
	set(CKPNPABLPDFracGol5Key, in.PDFracGol5)
	set(CKPNPABLLGDFracKey, in.LGDFrac)
	setBool := func(key string, v *bool) {
		if v != nil {
			raw := strconv.FormatBool(*v)
			if raw != current[key] {
				out[key] = raw
			}
		}
	}
	setBool(CKPNPABLAsetBaikBentukCKPNKey, in.AsetBaikBentukCKPN)
	setBool(CKPNPABLEnabledKey, in.Enabled)
	return out
}

// ValidateCKPNPABLActivation memeriksa hasil AKHIR (nilai lama yang tidak diubah +
// perubahan yang dikirim) sebelum disimpan. candidate adalah peta lengkap seluruh kunci
// yang dikelola (kunci tak ada = kosong). main adalah konfigurasi CKPN utama yang dipakai
// menilai kesiapan penyalakan; nil diperlakukan belum siap (gagal-aman).
//
// Urutan pemeriksaan penting: bentuk (fraksi) diperiksa sebelum konsekuensinya (nyala),
// sehingga pengguna melihat kesalahan ketik lebih dulu.
func ValidateCKPNPABLActivation(ctx context.Context, candidate map[string]string, main SystemConfigService) error {
	for _, key := range ckpnPABLFractionKeys() {
		if err := validateCKPNPABLFractionValue(key, candidate[key]); err != nil {
			return err
		}
	}

	// Penyalakan dinilai TANPA memandang saklar saat ini: kalau ckpn.pabl.enabled sudah
	// true di keadaan sekarang, penahan tetap harus terlihat agar perubahan lain (mis.
	// mengosongkan PD) tidak lolos begitu saja saat PABL menyala.
	if parseBoolOrZero(candidate[CKPNPABLEnabledKey]) {
		if gaps := CKPNPABLEnablementGaps(ctx, candidate, main); len(gaps) > 0 {
			return fmt.Errorf("%w: %s", ErrCKPNPABLActivationNotReady, strings.Join(gaps, "; "))
		}
	}
	return nil
}

// CKPNPABLEnablementGaps mengembalikan penahan penyalakan ckpn.pabl.enabled atas SUATU
// konfigurasi kandidat: keempat fraksi PABL yang belum diisi/tidak sah, ditambah penahan
// CKPN utama (parameter FINAL, ratifikasi, PD/LGD kredit, akun syariah) lewat fungsi yang
// sama dengan jalur aktivasi utama. Daftarnya dapat diperiksa mesin dan menyebut kunci
// yang harus diselesaikan, bukan menebak nilainya.
func CKPNPABLEnablementGaps(ctx context.Context, candidate map[string]string, main SystemConfigService) []string {
	var gaps []string
	for _, key := range ckpnPABLFractionKeys() {
		if !validCKPNPABLFraction(candidate[key]) {
			gaps = append(gaps, key+" belum diisi atau bukan fraksi (0,1] (satuan FRAKSI, bukan persen)")
		}
	}
	gaps = append(gaps, CKPNEnablementGapsForCandidate(ctx, main)...)
	return gaps
}

// validateCKPNPABLFractionValue menerima nilai kosong (belum diisi/dikosongkan) atau
// desimal dalam rentang (0,1]. Nol ditolak karena nol berarti "belum diisi" dan mesin
// kolektif PABL menolaknya. Pesan galat menyebut nama kunci.
func validateCKPNPABLFractionValue(key, raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	d, err := decimal.NewFromString(raw)
	if err != nil {
		return fmt.Errorf("%w: %s bukan angka desimal (satuan FRAKSI, bukan persen)", ErrPABLCKPNParameterInvalid, key)
	}
	return validatePABLCKPNFraction(key, d)
}

// validCKPNPABLFraction melaporkan apakah nilai terisi dan berada pada rentang (0,1].
func validCKPNPABLFraction(raw string) bool {
	d, err := decimal.NewFromString(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return d.IsPositive() && !d.GreaterThan(decimal.NewFromInt(1))
}

// CKPNPABLActivationStore membaca dan menulis kunci pengaturan CKPN PABL. Seluruh
// penulisan terjadi di dalam transaksi yang sama dengan audit pemanggil.
type CKPNPABLActivationStore interface {
	// Get membaca nilai kunci yang dikelola (kunci tak ada = kosong).
	Get(ctx context.Context) (map[string]string, error)
	// GetTx membaca di dalam transaksi tulis agar nilai "sebelum" pada audit benar.
	GetTx(ctx context.Context, tx any) (map[string]string, error)
	// SaveTx menyimpan perubahan kunci (upsert) di dalam transaksi tulis.
	SaveTx(ctx context.Context, tx any, changes map[string]string, updatedBy uuid.UUID) error
}

// CKPNPABLActivationService mengelola pengaturan CKPN PABL.
type CKPNPABLActivationService interface {
	// Get mengembalikan nilai terkini beserta penahan penyalakan.
	Get(ctx context.Context) (*CKPNPABLActivation, error)
	// Update memvalidasi, menyimpan, dan mengaudit perubahan dalam satu transaksi.
	Update(ctx context.Context, input UpdateCKPNPABLActivationInput, actor Actor) (*CKPNPABLActivation, error)
}
