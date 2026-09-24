package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CKPN individual TAHAP T4 — integrasi ke langkah CKPN EOD (rancangan §6.2, §5.3-5.4).
//
// Prinsip yang ditegakkan tahap ini:
//   - ANTI-DOUBLE-COUNT (§5.3): kredit yang tersegel individual (loans.ckpn_method
//     INDIVIDUAL_*) TIDAK boleh dihitung rumus kolektif EAD×PD×LGD; kredit tersegel
//     EXCLUDED_ASET_BAIK dikecualikan seperti aset baik. Penentuan ini membaca SEGEL
//     hasil keputusan T3, bukan menilai ulang dari nol — penilaian ulang mengaburkan
//     keputusan pengelola yang sudah diaudit.
//   - SATU SUMBER KEBENARAN (§5.4): target individual ditulis ke loans.required_ckpn
//     lewat mekanisme apply/postAdjustment yang sama dengan kolektif; yang berbeda
//     hanya cara memperoleh target. ckpn_individual_target hanyalah jejak.
//   - LANTAI 12.4.g.1.c: target individual tidak boleh turun di bawah CKPN individual
//     yang sudah dibentuk sebelumnya (required_ckpn yang pernah diakui saat metode
//     masih individual).

// CKPNIndividualSealed memutuskan perlakuan EOD satu kredit dari segel metodenya:
// "" atau COLLECTIVE berarti jalur kolektif biasa; INDIVIDUAL_* berarti jalur
// individual; EXCLUDED_ASET_BAIK berarti dikecualikan (target nol, seperti aset baik).
// Kredit bersegel tak dikenal GAGAL dengan galat, bukan diam-diam masuk kolektif:
// segel menentukan angka cadangan dan tidak boleh diabaikan.
func CKPNIndividualSealed(method string) (individual bool, excluded bool, err error) {
	switch method {
	case "", "COLLECTIVE":
		return false, false, nil
	case string(CKPNIndividualMethodDCF), string(CKPNIndividualMethodCollateral), string(CKPNIndividualMethodMax):
		return true, false, nil
	case string(CKPNIndividualMethodExcludedAsetBaik):
		return false, true, nil
	default:
		return false, false, fmt.Errorf("%w: segel ckpn_method %q tidak dikenal", ErrCKPNParameterInvalid, method)
	}
}

// CKPNIndividualEODInput adalah masukan perhitungan target individual pada EOD: hasil
// penilaian T1/T2 kredit ini dan lantai dari target yang sudah dibentuk sebelumnya.
type CKPNIndividualEODInput struct {
	Assessment CKPNIndividualAssessment
	// PreviousIndividualTarget adalah required_ckpn yang pernah diakui saat kredit
	// masih tersegel individual. Lantai 12.4.g.1.c memakai nilai ini, BUKAN
	// required_ckpn hasil kolektif: lantai melindungi cadangan individual yang sudah
	// terbentuk, bukan menggabungkan dua basis angka.
	PreviousIndividualTarget decimal.Decimal
}

// CKPNIndividualTargetForEOD menetapkan target individual resmi satu kredit:
// metode kredit (segel T3) memilih komponen penilaian; lantai 12.4.g.1.c menjaga
// target tidak turun di bawah CKPN individual sebelumnya. Fungsi murni.
func CKPNIndividualTargetForEOD(in CKPNIndividualEODInput, method CKPNIndividualMethod) (decimal.Decimal, error) {
	a := in.Assessment
	switch method {
	case CKPNIndividualMethodDCF:
		// DCF murni: proyeksi wajib ada dan EIR wajib tersedia; kegagalan data adalah
		// galat nyata, bukan alasan jatuh ke nol atau ke rumus lain.
		if len(a.Projections) == 0 {
			return decimal.Zero, fmt.Errorf("%w: kredit %s individual DCF tanpa proyeksi arus kas; isi lewat PUT /ckpn/individual/%s/projections",
				ErrCKPNProjectionInvalid, a.LoanNumber, a.LoanNumber)
		}
	case CKPNIndividualMethodCollateral:
		// 12.4.g.1.b: pendekatan agunan. DCF boleh tidak ada (proyeksi kosong), tetapi
		// kredit tanpa agunan aktif tidak punya dasar pendekatan ini.
		if len(a.Collaterals) == 0 {
			return decimal.Zero, fmt.Errorf("%w: kredit %s individual agunan tanpa agunan aktif",
				ErrCKPNProjectionInvalid, a.LoanNumber)
		}
	case CKPNIndividualMethodMax:
		// Bila keduanya dapat dihitung, ambil yang lebih konservatif. DCF tanpa
		// proyeksi BUKAN sah untuk dimaksimalisasikan: proyeksi kosong menghasilkan
		// target 0 yang akan memalsukan aturan MAX.
		if len(a.Projections) == 0 {
			return decimal.Zero, fmt.Errorf("%w: kredit %s individual MAX tanpa proyeksi arus kas; isi proyeksi atau ubah metode",
				ErrCKPNProjectionInvalid, a.LoanNumber)
		}
	default:
		return decimal.Zero, fmt.Errorf("%w: metode individual %q tidak sah untuk EOD", ErrCKPNParameterInvalid, method)
	}

	target := a.FinalTarget
	// Lantai 12.4.g.1.c: tidak boleh turun di bawah CKPN individual yang sudah
	// dibentuk sebelumnya.
	if in.PreviousIndividualTarget.GreaterThan(target) {
		target = in.PreviousIndividualTarget
	}
	return target, nil
}

// CKPNIndividualEODTrail adalah jejak audit penilaian individual pada EOD: seluruh
// angka yang dipakai menetapkan target resmi, agar dapat direkonstruksi.
type CKPNIndividualEODTrail struct {
	LoanID         uuid.UUID            `json:"loan_id"`
	AsOf           time.Time            `json:"as_of"`
	Method         CKPNIndividualMethod `json:"method"`
	CarryingAmount decimal.Decimal      `json:"carrying_amount"`
	PresentValue   decimal.Decimal      `json:"present_value"`
	CollateralNRV  decimal.Decimal      `json:"collateral_nrv"`
	Target         decimal.Decimal      `json:"target"`
	FloorApplied   bool                 `json:"floor_applied"`
	MissingCost    int                  `json:"missing_disposal_cost_count"`
}
