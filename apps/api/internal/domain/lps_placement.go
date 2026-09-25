package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Modul Pasal 23 POJK No. 1 Tahun 2024 tentang Kualitas Aset Bank Perekonomian Rakyat:
//
//	"Bagian Penempatan pada Bank Lain yang memenuhi persyaratan kriteria penjaminan
//	 Lembaga Penjamin Simpanan dapat dijadikan sebagai faktor pengurang dalam
//	 pembentukan perhitungan PPKA umum dan khusus."
//
// Penjelasan pasal memberi contoh: penempatan Rp10.000.000.000 pada satu bank dengan
// jaminan LPS paling tinggi Rp2.000.000.000 (per nasabah pada satu bank) menghasilkan
// PPKA umum 0,5% x (10 miliar - 2 miliar) dan PPKA khusus dengan rumus yang sama.
//
// Sistem ini hanya punya COA 10200 (konvensional) dan 11200 (syariah) untuk PABL; tidak
// ada instrumen penempatan, bank lawan, maupun kualitas per penempatan. Karena itu nilai
// yang dijamin LPS TIDAK boleh disimpulkan dari nama akun dengan pencocokan teks. Tabel
// lps_placements (migrasi 000045) menyimpannya eksplisit, dan modul ini menghitung
// pengurangnya. Saklar LPSPlacementEnabledKey bawaan false: selama mati tidak ada query
// tambahan dan tidak ada angka PPKA yang berubah.

// Kunci konfigurasi modul Pasal 23.
const (
	// LPSPlacementEnabledKey adalah saklar utama pengurang PPKA untuk penempatan yang
	// dijamin LPS. Bawaan false; mengikuti pola ppap.collateral.enabled dan ckpn.enabled.
	LPSPlacementEnabledKey = "ppap.lps.enabled"
	// LPSGuaranteeCapKey adalah plafon penjaminan LPS per nasabah pada satu bank
	// (dalam rupiah penuh). Nilai yang diisi tetapi salah format atau tidak positif
	// DITOLAK dengan ErrLPSParameterInvalid, tidak dijatuhkan menjadi nol.
	LPSGuaranteeCapKey = "ppap.lps.guarantee_cap"
)

// LPSGuaranteeCapDefault adalah plafon penjaminan LPS per nasabah pada satu bank
// menurut ketentuan peraturan perundang-undangan mengenai penjaminan oleh LPS yang
// dirujuk Penjelasan Pasal 23 (Rp2.000.000.000). Dipakai hanya bila kunci konfigurasi
// tidak diisi; plafon ini dapat berubah, karena itu dapat ditimpa konfigurasi.
func LPSGuaranteeCapDefault() decimal.Decimal {
	return decimal.NewFromInt(2_000_000_000)
}

// Sentinel error modul Pasal 23. Dipisah agar pemanggil dapat membedakan kegagalan
// parameter konfigurasi dari kegagalan data penempatan.
var (
	// ErrLPSParameterInvalid menandai parameter konfigurasi yang DIISI tetapi tidak sah
	// (salah format atau di luar rentang), berbeda dari "tidak diisi" yang memakai bawaan.
	ErrLPSParameterInvalid = errors.New("parameter pengurang PPKA penempatan LPS tidak valid")
	// ErrLPSPlacementInvalid menandai baris penempatan yang tidak dapat dihitung.
	ErrLPSPlacementInvalid = errors.New("penempatan pada bank lain tidak valid")
	// ErrLPSGuaranteeOverCap menandai jumlah penempatan yang dijamin LPS pada satu bank
	// melebihi plafon penjaminan. Data seperti ini tidak boleh dipotong diam-diam: lebih
	// baik ditolak agar bank memperbaiki penandaannya.
	ErrLPSGuaranteeOverCap = errors.New("nilai penempatan yang dijamin LPS melebihi plafon penjaminan per bank")
)

// LPSPlacementType adalah bentuk Penempatan pada Bank Lain menurut Pasal 1 angka 4
// POJK 1/2024: giro, tabungan, deposito, sertifikat deposito, kredit yang diberikan,
// dan penempatan dana lain yang sejenis.
type LPSPlacementType string

const (
	LPSPlacementGiro               LPSPlacementType = "GIRO"
	LPSPlacementTabungan           LPSPlacementType = "TABUNGAN"
	LPSPlacementDeposito           LPSPlacementType = "DEPOSITO"
	LPSPlacementSertifikatDeposito LPSPlacementType = "SERTIFIKAT_DEPOSITO"
	LPSPlacementKredit             LPSPlacementType = "KREDIT"
	LPSPlacementLainnya            LPSPlacementType = "LAINNYA"
)

func (t LPSPlacementType) Valid() bool {
	switch t {
	case LPSPlacementGiro, LPSPlacementTabungan, LPSPlacementDeposito,
		LPSPlacementSertifikatDeposito, LPSPlacementKredit, LPSPlacementLainnya:
		return true
	default:
		return false
	}
}

// LPSPlacementCollectibility adalah kualitas Penempatan pada Bank Lain. Pasal 15 ayat (1)
// POJK 1/2024 hanya mengenal tiga golongan, berbeda dari Kredit yang punya lima.
type LPSPlacementCollectibility string

const (
	LPSLancar       LPSPlacementCollectibility = "LANCAR"
	LPSKurangLancar LPSPlacementCollectibility = "KURANG_LANCAR"
	LPSMacet        LPSPlacementCollectibility = "MACET"
)

func (k LPSPlacementCollectibility) Valid() bool {
	switch k {
	case LPSLancar, LPSKurangLancar, LPSMacet:
		return true
	default:
		return false
	}
}

// ToCollectibility memetakan kualitas PABL ke golongan mesin PPAP. Lancar memakai PPKA
// umum (0,5%), Kurang Lancar dan Macet memakai PPKA khusus (Pasal 19 ayat (2) dan (3)
// POJK 1/2024 jo. Pasal 15).
func (k LPSPlacementCollectibility) ToCollectibility() Collectibility {
	switch k {
	case LPSKurangLancar:
		return KolKurangLancar
	case LPSMacet:
		return KolMacet
	default:
		return KolLancar
	}
}

// LPSPlacement adalah satu penempatan pada bank lain beserta bagiannya yang memenuhi
// kriteria penjaminan LPS. LPSGuaranteed adalah penanda eksplisit nilai yang dijamin;
// nilainya tidak diturunkan dari nama akun atau tipe penempatan.
type LPSPlacement struct {
	ID               uuid.UUID
	COACode          string
	CounterpartyBank string
	PlacementType    LPSPlacementType
	Outstanding      decimal.Decimal
	// LPSGuaranteed adalah bagian penempatan yang memenuhi persyaratan kriteria
	// penjaminan LPS (mis. tingkat suku bunga) dan berada dalam plafon per bank lawan.
	LPSGuaranteed  decimal.Decimal
	Collectibility LPSPlacementCollectibility
	AsOf           time.Time
	// StartDate dan MaturityDate adalah tanggal mulai dan jatuh tempo penempatan,
	// sumber kolom VI (Jangka Waktu) Form 05.00. StartDate nil berarti belum diisi
	// (ditulis "-"); MaturityDate nil berarti penempatan tanpa jatuh tempo sehingga
	// kolom VI hanya memuat tanggal mulai.
	StartDate    *time.Time
	MaturityDate *time.Time
	// InterestRateAnnual adalah suku bunga TAHUNAN penempatan dalam persen (mis. 5.5
	// berarti 5,5%), sumber kolom VIII (Suku Bunga) Form 05.00, sampai 2 digit desimal.
	InterestRateAnnual decimal.Decimal
	BranchID           *uuid.UUID
	BranchCode         string
	// CKPN adalah asesmen CKPN terakhir penempatan ini; nil berarti belum pernah
	// diasesmen (kolom ckpn_assessed_at NULL). Form 05.00 kolom XII/XXI hanya tersedia
	// bila minimal satu baris punya CKPN.
	CKPN *LPSPlacementCKPN
}

// LPSPlacementCKPN adalah asesmen CKPN tersimpan satu penempatan. RequiredCKPN adalah
// target yang berlaku; IndividualTarget hanya bermakna untuk metode INDIVIDUAL_*.
type LPSPlacementCKPN struct {
	Method            PABLCKPNMethod
	Significant       bool
	ObjectiveEvidence bool
	RequiredCKPN      decimal.Decimal
	IndividualTarget  decimal.Decimal
	AssessedAt        *time.Time
}

// Validate menegakkan syarat yang tidak dapat dijaga skema: bank lawan terisi, jenis dan
// kualitas dikenal, nilai tidak negatif, serta jaminan tidak melebihi penempatannya.
// Baris yang tidak lolos DITOLAK, bukan dihitung dengan nilai nol.
func (p LPSPlacement) Validate() error {
	if strings.TrimSpace(p.CounterpartyBank) == "" {
		return fmt.Errorf("%w: nama bank lawan wajib diisi", ErrLPSPlacementInvalid)
	}
	if !p.PlacementType.Valid() {
		return fmt.Errorf("%w: jenis penempatan %q tidak dikenal (Pasal 1 angka 4 POJK 1/2024)",
			ErrLPSPlacementInvalid, p.PlacementType)
	}
	if !p.Collectibility.Valid() {
		return fmt.Errorf("%w: kualitas %q tidak dikenal (Pasal 15 ayat (1) POJK 1/2024 hanya Lancar, Kurang Lancar, atau Macet)",
			ErrLPSPlacementInvalid, p.Collectibility)
	}
	if p.Outstanding.IsNegative() {
		return fmt.Errorf("%w: nilai penempatan %s tidak boleh negatif", ErrLPSPlacementInvalid, p.Outstanding)
	}
	if p.LPSGuaranteed.IsNegative() {
		return fmt.Errorf("%w: nilai yang dijamin LPS %s tidak boleh negatif", ErrLPSPlacementInvalid, p.LPSGuaranteed)
	}
	if p.LPSGuaranteed.GreaterThan(p.Outstanding) {
		return fmt.Errorf("%w: jaminan LPS %s melebihi nilai penempatan %s",
			ErrLPSPlacementInvalid, p.LPSGuaranteed, p.Outstanding)
	}
	return nil
}

// LPSPlacementDeduction menghitung faktor pengurang Pasal 23: bagian yang dijamin LPS,
// dibatasi pada [0, outstanding] agar pengurang tidak pernah melebihi nilai penempatannya.
func LPSPlacementDeduction(outstanding, guaranteed decimal.Decimal) decimal.Decimal {
	if outstanding.IsNegative() {
		outstanding = decimal.Zero
	}
	if guaranteed.IsNegative() {
		guaranteed = decimal.Zero
	}
	if guaranteed.GreaterThan(outstanding) {
		return outstanding
	}
	return guaranteed
}

// LPSPlacementPPKACalculation adalah hasil perhitungan PPKA satu penempatan. Base adalah
// penempatan dikurangi bagian yang dijamin LPS; dari dasar yang SAMA itulah PPKA umum
// (Lancar) atau PPKA khusus (Kurang Lancar/Macet) dihitung. Keduanya memakai pengurang
// Pasal 23, tetapi hanya satu yang berlaku menurut kualitas penempatan.
type LPSPlacementPPKACalculation struct {
	Outstanding    decimal.Decimal
	Deduction      decimal.Decimal
	Base           decimal.Decimal
	Collectibility Collectibility
	// GeneralPPKA adalah PPKA umum 0,5% atas base (Pasal 19 ayat (2)); hanya terisi
	// bila kualitasnya Lancar.
	GeneralPPKA decimal.Decimal
	// SpecificPPKA adalah PPKA khusus atas base menurut kualitas (Pasal 19 ayat (3));
	// hanya terisi bila kualitasnya Kurang Lancar atau Macet.
	SpecificPPKA decimal.Decimal
	// PKKATarget adalah PPKA yang berlaku: GeneralPPKA untuk Lancar, SpecificPPKA untuk
	// Kurang Lancar/Macet.
	PKKATarget decimal.Decimal
	// AppliesTo bernilai "UMUM" atau "KHUSUS".
	AppliesTo string
}

// CalculateLPSPlacementPPKA menghitung PPKA umum dan khusus atas penempatan setelah
// dikurangi bagian yang dijamin LPS. Dasar memakai PPAPExposure yang berlantai nol agar
// jaminan yang lebih besar dari penempatan tidak pernah menghasilkan cadangan negatif.
func CalculateLPSPlacementPPKA(p LPSPlacement, rates PPAPRates) (LPSPlacementPPKACalculation, error) {
	if err := p.Validate(); err != nil {
		return LPSPlacementPPKACalculation{}, err
	}
	deduction := LPSPlacementDeduction(p.Outstanding, p.LPSGuaranteed)
	base := PPAPExposure(p.Outstanding, deduction)
	col := p.Collectibility.ToCollectibility()

	calc := LPSPlacementPPKACalculation{
		Outstanding:    p.Outstanding,
		Deduction:      deduction,
		Base:           base,
		Collectibility: col,
	}
	if col == KolLancar {
		calc.GeneralPPKA = PPAPAmount(base, KolLancar, rates)
		calc.AppliesTo = "UMUM"
		calc.PKKATarget = calc.GeneralPPKA
	} else {
		calc.SpecificPPKA = PPAPAmount(base, col, rates)
		calc.AppliesTo = "KHUSUS"
		calc.PKKATarget = calc.SpecificPPKA
	}
	return calc, nil
}

// LPSPlacementItem adalah hasil perhitungan satu penempatan untuk laporan; angka-angka
// antara disertakan agar selisih antar periode dapat ditelusuri.
type LPSPlacementItem struct {
	PlacementID      uuid.UUID        `json:"id"`
	COACode          string           `json:"coa_code"`
	CounterpartyBank string           `json:"counterparty_bank"`
	PlacementType    LPSPlacementType `json:"placement_type"`
	Collectibility   Collectibility   `json:"collectibility"`
	AsOf             time.Time        `json:"as_of"`
	Outstanding      decimal.Decimal  `json:"outstanding"`
	Deduction        decimal.Decimal  `json:"lps_deduction"`
	Base             decimal.Decimal  `json:"ppka_base"`
	GeneralPPKA      decimal.Decimal  `json:"general_ppka"`
	SpecificPPKA     decimal.Decimal  `json:"specific_ppka"`
	PPKA             decimal.Decimal  `json:"ppka"`
	AppliesTo        string           `json:"applies_to"`
}

// LPSPlacementFailure mencatat penempatan yang gagal dihitung tanpa menggagalkan laporan.
type LPSPlacementFailure struct {
	PlacementID      uuid.UUID `json:"id"`
	CounterpartyBank string    `json:"counterparty_bank"`
	Error            string    `json:"error"`
}

// LPSPlacementSummary adalah ringkasan perhitungan PPKA penempatan pada bank lain.
type LPSPlacementSummary struct {
	// Enabled false berarti saklar ppap.lps.enabled mati: tidak ada penempatan yang
	// dibaca dan tidak ada angka yang dihitung.
	Enabled           bool                  `json:"enabled"`
	AsOf              time.Time             `json:"as_of"`
	GuaranteeCap      decimal.Decimal       `json:"guarantee_cap"`
	Total             int                   `json:"total"`
	Processed         int                   `json:"processed"`
	Failed            int                   `json:"failed"`
	Items             []LPSPlacementItem    `json:"items"`
	Failures          []LPSPlacementFailure `json:"failures"`
	TotalOutstanding  decimal.Decimal       `json:"total_outstanding"`
	TotalDeduction    decimal.Decimal       `json:"total_lps_deduction"`
	TotalGeneralPPKA  decimal.Decimal       `json:"total_general_ppka"`
	TotalSpecificPPKA decimal.Decimal       `json:"total_specific_ppka"`
	TotalPPKA         decimal.Decimal       `json:"total_ppka"`
}

// PABLCKPNRunSummary adalah ringkasan satu kali jalan mesin kolektif CKPN PABL.
// Processed adalah penempatan yang targetnya dihitung dan disimpan (termasuk
// EXCLUDED_ASET_BAIK yang targetnya nol), Skipped adalah penempatan individual yang
// asesmen manualnya dipertahankan, dan Failed adalah penempatan yang gagal diproses
// (masuk Failures tanpa menggagalkan seluruh run).
type PABLCKPNRunSummary struct {
	AsOf        time.Time             `json:"as_of"`
	Total       int                   `json:"total"`
	Processed   int                   `json:"processed"`
	Skipped     int                   `json:"skipped"`
	Failed      int                   `json:"failed"`
	TotalTarget decimal.Decimal       `json:"total_target"`
	Failures    []LPSPlacementFailure `json:"failures"`
}

// LPSPlacementRepository membaca penempatan pada bank lain. Filter cabang diterapkan di
// query memakai actor; aktor lintas cabang menerima seluruh bank.
type LPSPlacementRepository interface {
	// ListPlacements mengambil penempatan yang tanggal penilaiannya tidak melewati asOf.
	ListPlacements(ctx context.Context, asOf time.Time, actor Actor) ([]LPSPlacement, error)
	// LockPlacementTx membaca satu penempatan dengan SELECT ... FOR UPDATE di dalam
	// transaksi pemanggil, agar asesmen CKPN tidak berlomba dengan penulisan lain.
	LockPlacementTx(ctx context.Context, tx any, id uuid.UUID) (*LPSPlacement, error)
	// RecordCKPNTx menyimpan asesmen CKPN penempatan: memperbarui kolom CKPN pada
	// lps_placements dan menulis jejaknya ke pabl_ckpn_assessments.
	RecordCKPNTx(ctx context.Context, tx any, placementID uuid.UUID, input PABLCKPNInput, carrying decimal.Decimal, actorName string) error
	// RecordCollectiveCKPNTx menyimpan hasil mesin kolektif satu penempatan: hanya
	// required_ckpn dan ckpn_assessed_at yang diperbarui (ckpn_method TIDAK diubah),
	// lalu jejaknya ditulis ke pabl_ckpn_assessments memakai metode tersimpan dan
	// carrying = outstanding baris terkunci.
	RecordCollectiveCKPNTx(ctx context.Context, tx any, placementID uuid.UUID, asOf time.Time, target decimal.Decimal, basis []byte, actorName string) error
}

// LPSPlacementService menghitung PPKA penempatan yang dijamin LPS. Calculate baca-saja:
// belum ada keputusan bank tentang bagaimana PPKA penempatan dicatat, jadi modul ini
// tidak memposting jurnal. AssessCKPN menulis asesmen CKPN (kolom XII/XXI Form 05.00)
// dan TIDAK memposting jurnal.
type LPSPlacementService interface {
	Calculate(ctx context.Context, asOf time.Time, actor Actor) (LPSPlacementSummary, error)
	// AssessCKPN menyimpan asesmen CKPN satu penempatan. Gagal dengan
	// ErrCKPNPABLDisabled bila saklar ckpn.pabl.enabled mati, dan dengan
	// ErrCrossBranchAccess bila penempatan berada di luar cakupan cabang aktor.
	AssessCKPN(ctx context.Context, placementID uuid.UUID, input PABLCKPNInput, actor Actor) (LPSPlacement, error)
	// RunCollectiveCKPN menjalankan mesin kolektif PD/LGD untuk penempatan
	// ber-metode COLLECTIVE dan menyimpan required_ckpn-nya. Asesmen INDIVIDUAL_*
	// dilewati agar tidak ditimpa; EXCLUDED_ASET_BAIK disimpan dengan target nol.
	// Gagal dengan ErrCKPNPABLDisabled bila saklar mati dan
	// ErrPABLCKPNParameterInvalid bila parameter PD/LGD belum diisi/tidak sah.
	RunCollectiveCKPN(ctx context.Context, asOf time.Time, actor Actor) (PABLCKPNRunSummary, error)
}
