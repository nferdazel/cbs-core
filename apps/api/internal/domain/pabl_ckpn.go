package domain

import (
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// CKPN per penempatan pada bank lain (PABL).
//
// Form 05.00 kolom XII (CKPN) dan XXI (Jenis CKPN) menuntut cadangan per penempatan,
// sedangkan lps_placements (migrasi 000045) hanya menyimpan penempatan dan bagian yang
// dijamin LPS. Modul ini menambahkan asesmen eksplisit: bank memutuskan metode, target,
// dan tanggal asesmen; kode TIDAK menurunkan CKPN dari nama akun atau jenis penempatan.
//
// Saklar ckpn.pabl.enabled bawaan FALSE sehingga perilaku default tidak berubah: Form
// 05.00 tetap menyatakan kolom XII/XXI belum tersedia sampai minimal satu penempatan
// diasesmen. Metode INDIVIDUAL_* memakai sandi "1", COLLECTIVE dan EXCLUDED_ASET_BAIK
// memakai sandi "2" — cermin sandiJenisCKPN Form 06.00.

// CKPNPABLEnabledKey adalah saklar utama CKPN per penempatan pada bank lain. Bawaan
// false: selama mati tidak ada asesmen yang dapat disimpan lewat API.
const CKPNPABLEnabledKey = "ckpn.pabl.enabled"

// PABLCKPNMethod adalah metode perhitungan CKPN satu penempatan yang dipilih bank.
// COLLECTIVE adalah bawaan: penempatan diperlakukan kolektif seperti sebelum modul ini
// ada. INDIVIDUAL_* menandai perhitungan individual (DCF, agunan, atau yang lebih
// konservatif). EXCLUDED_ASET_BAIK menandai penempatan yang bank keluarkan dari jalur
// individual.
type PABLCKPNMethod string

const (
	PABLCKPNMethodCollective           PABLCKPNMethod = "COLLECTIVE"
	PABLCKPNMethodIndividualDCF        PABLCKPNMethod = "INDIVIDUAL_DCF"
	PABLCKPNMethodIndividualCollateral PABLCKPNMethod = "INDIVIDUAL_COLLATERAL"
	PABLCKPNMethodIndividualMax        PABLCKPNMethod = "INDIVIDUAL_MAX"
	PABLCKPNMethodExcludedAsetBaik     PABLCKPNMethod = "EXCLUDED_ASET_BAIK"
)

// Valid menandai metode yang dikenal. Nilai tak dikenal ditolak, bukan diperlakukan
// sebagai COLLECTIVE diam-diam: metode menentukan sandi dan angka cadangan.
func (m PABLCKPNMethod) Valid() bool {
	switch m {
	case PABLCKPNMethodCollective, PABLCKPNMethodIndividualDCF,
		PABLCKPNMethodIndividualCollateral, PABLCKPNMethodIndividualMax,
		PABLCKPNMethodExcludedAsetBaik:
		return true
	default:
		return false
	}
}

// SandiJenisCKPNPABL memetakan metode asesmen ke sandi kolom XXI Form 05.00:
// sandi 1 = CKPN individual, sandi 2 = CKPN kolektif. Metode kosong dan tak dikenal
// dilaporkan kolektif (bawaan kebijakan), bukan dikosongkan, agar baris tetap dilaporkan.
func SandiJenisCKPNPABL(method PABLCKPNMethod) string {
	switch method {
	case PABLCKPNMethodIndividualDCF, PABLCKPNMethodIndividualCollateral,
		PABLCKPNMethodIndividualMax:
		return "1"
	default:
		return "2"
	}
}

// Sentinel error modul CKPN PABL. Dipisah agar pemanggil dapat membedakan saklar mati,
// masukan tidak sah, dan kegagalan data.
var (
	// ErrCKPNPABLDisabled menandai percobaan asesmen saat saklar ckpn.pabl.enabled mati.
	// Permintaan TIDAK boleh diam-diam sukses tanpa menyimpan apa pun.
	ErrCKPNPABLDisabled = errors.New("penilaian CKPN per penempatan pada bank lain belum diaktifkan (saklar ckpn.pabl.enabled)")
	// ErrCKPNPABLInputInvalid menandai masukan asesmen yang tidak sah.
	ErrCKPNPABLInputInvalid = errors.New("masukan penilaian CKPN per penempatan pada bank lain tidak valid")
	// ErrLPSPlacementNotFound menandai penempatan yang tidak ditemukan saat asesmen.
	ErrLPSPlacementNotFound = errors.New("penempatan pada bank lain tidak ditemukan")
)

// PABLCKPNInput adalah masukan asesmen CKPN satu penempatan. RequiredCKPN adalah target
// cadangan yang berlaku; IndividualTarget adalah target perhitungan individual (hanya
// bermakna untuk metode INDIVIDUAL_*). AsOf adalah tanggal asesmen.
type PABLCKPNInput struct {
	Method            PABLCKPNMethod  `json:"method"`
	Significant       bool            `json:"significant"`
	ObjectiveEvidence bool            `json:"objective_evidence"`
	RequiredCKPN      decimal.Decimal `json:"required_ckpn"`
	IndividualTarget  decimal.Decimal `json:"individual_target"`
	AsOf              time.Time       `json:"as_of"`
}

// Validate menegakkan syarat yang tidak dapat dijaga skema: metode dikenal, nilai tidak
// negatif, dan tanggal asesmen terisi. Target individual boleh nol (perhitungan dapat
// menghasilkan nol), jadi nol TIDAK ditolak.
func (in PABLCKPNInput) Validate() error {
	if !in.Method.Valid() {
		return fmt.Errorf("%w: metode %q tidak dikenal", ErrCKPNPABLInputInvalid, in.Method)
	}
	if in.RequiredCKPN.IsNegative() {
		return fmt.Errorf("%w: CKPN %s tidak boleh negatif", ErrCKPNPABLInputInvalid, in.RequiredCKPN)
	}
	if in.IndividualTarget.IsNegative() {
		return fmt.Errorf("%w: target individual %s tidak boleh negatif", ErrCKPNPABLInputInvalid, in.IndividualTarget)
	}
	if in.AsOf.IsZero() {
		return fmt.Errorf("%w: tanggal asesmen wajib diisi", ErrCKPNPABLInputInvalid)
	}
	return nil
}
