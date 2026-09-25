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

const (
	// CKPNPABLEnabledKey adalah saklar utama CKPN per penempatan pada bank lain.
	// Bawaan false: selama mati tidak ada asesmen yang dapat disimpan lewat API dan
	// mesin kolektif tidak berjalan.
	CKPNPABLEnabledKey = "ckpn.pabl.enabled"

	// Parameter mesin kolektif PD/LGD PABL (migrasi 000100). Nilai adalah FRAKSI 0..1,
	// sama seperti ckpn.pd_frac.* / ckpn.lgd_frac untuk kredit. PD hanya untuk tiga
	// golongan yang dikenal PABL (1 Lancar, 3 Kurang Lancar, 5 Macet).
	CKPNPABLPDFracGol1Key         = "ckpn.pabl.pd_frac.gol_1"
	CKPNPABLPDFracGol3Key         = "ckpn.pabl.pd_frac.gol_3"
	CKPNPABLPDFracGol5Key         = "ckpn.pabl.pd_frac.gol_5"
	CKPNPABLLGDFracKey            = "ckpn.pabl.lgd_frac"
	CKPNPABLAsetBaikBentukCKPNKey = "ckpn.pabl.aset_baik_bentuk_ckpn"
)

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
	// ErrPABLCKPNParameterInvalid menandai parameter mesin kolektif PD/LGD PABL yang
	// kosong, salah format, atau di luar rentang (0,1]. Parameter yang salah DITOLAK,
	// tidak pernah dijatuhkan menjadi nol diam-diam.
	ErrPABLCKPNParameterInvalid = errors.New("parameter mesin kolektif CKPN per penempatan pada bank lain tidak valid")
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

// PABLCKPNParams adalah parameter kebijakan bank untuk mesin kolektif PABL: PD per
// golongan kualitas yang dikenal PABL dan LGD. Semua nilai adalah FRAKSI 0..1, sama
// seperti ckpn.pd_frac.* / ckpn.lgd_frac kredit. AsetBaikBentukCKPN true berarti bank
// (atau auditor) memilih tetap membentuk CKPN atas outstanding penuh sekalipun
// penempatan memenuhi kriteria aset baik butir 12.3.a.1.b.
type PABLCKPNParams struct {
	PDFracGol1         decimal.Decimal
	PDFracGol3         decimal.Decimal
	PDFracGol5         decimal.Decimal
	LGDFrac            decimal.Decimal
	AsetBaikBentukCKPN bool
}

// pablPDKey menamai kunci konfigurasi PD menurut kualitas PABL; dipakai pesan galat.
func pablPDKey(k LPSPlacementCollectibility) string {
	switch k {
	case LPSKurangLancar:
		return CKPNPABLPDFracGol3Key
	case LPSMacet:
		return CKPNPABLPDFracGol5Key
	default:
		return CKPNPABLPDFracGol1Key
	}
}

// pablPD memilih PD menurut kualitas PABL: Lancar golongan 1, Kurang Lancar golongan 3,
// Macet golongan 5. Kualitas di luar ketiganya (termasuk kosong) ditolak, bukan
// dipetakan diam-diam ke golongan Lancar.
func (params PABLCKPNParams) pablPD(k LPSPlacementCollectibility) (decimal.Decimal, bool) {
	switch k {
	case LPSLancar:
		return params.PDFracGol1, true
	case LPSKurangLancar:
		return params.PDFracGol3, true
	case LPSMacet:
		return params.PDFracGol5, true
	default:
		return decimal.Zero, false
	}
}

// CalculatePABLCKPNCollective menghitung target CKPN kolektif satu penempatan pada bank
// lain: CKPN = EAD x PD x LGD. Rumus dan pembulatannya (RoundToRupiah) disamakan dengan
// CalculateCKPN kredit, sesuai contoh tabel PA BPR Bab XII butir 12.9.
//
// Dasar (EAD) bergantung params.AsetBaikBentukCKPN:
//   - false (bawaan): outstanding dikurangi bagian yang dijamin LPS (LPSPlacementDeduction
//     lalu PPAPExposure, berlantai nol). Bagian yang dijamin LPS memenuhi kriteria aset
//     baik butir 12.3.a.1.b sehingga boleh tidak membentuk CKPN (butir 12.3.a.2).
//   - true: outstanding penuh.
//
// Setiap fraksi yang DIPAKAI wajib > 0 dan <= 1; nilai kosong/nol/negatif/> 1 ditolak
// dengan ErrPABLCKPNParameterInvalid, tidak pernah dijadikan nol.
func CalculatePABLCKPNCollective(p LPSPlacement, params PABLCKPNParams) (decimal.Decimal, error) {
	if err := p.Validate(); err != nil {
		return decimal.Zero, err
	}
	pd, ok := params.pablPD(p.Collectibility)
	if !ok {
		return decimal.Zero, fmt.Errorf("%w: kualitas %q tidak dikenal", ErrPABLCKPNParameterInvalid, p.Collectibility)
	}
	if err := validatePABLCKPNFraction(pablPDKey(p.Collectibility), pd); err != nil {
		return decimal.Zero, err
	}
	if err := validatePABLCKPNFraction(CKPNPABLLGDFracKey, params.LGDFrac); err != nil {
		return decimal.Zero, err
	}

	base := p.Outstanding
	if !params.AsetBaikBentukCKPN {
		base = PPAPExposure(p.Outstanding, LPSPlacementDeduction(p.Outstanding, p.LPSGuaranteed))
	}
	return RoundToRupiah(base.Mul(pd).Mul(params.LGDFrac)), nil
}

// validatePABLCKPNFraction menegakkan rentang fraksi (0,1]. Dipakai PD maupun LGD agar
// pesan galat menyebut kunci parameter yang salah.
func validatePABLCKPNFraction(name string, v decimal.Decimal) error {
	if !v.IsPositive() || v.GreaterThan(decimal.NewFromInt(1)) {
		return fmt.Errorf("%w: %s harus lebih besar dari 0 dan paling tinggi 1 (nilai %s)",
			ErrPABLCKPNParameterInvalid, name, v)
	}
	return nil
}
