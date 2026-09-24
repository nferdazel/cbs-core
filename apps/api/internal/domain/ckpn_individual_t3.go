package domain

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// CKPN individual TAHAP T3 — pintu masuk jalur individual (keputusan panel butir 5).
//
// T3 menjawab pertanyaan "kredit mana yang WAJIB dinilai individual" tanpa menghitung
// target apa pun. Dua pintu masuk menurut rancangan (PA BPR 12.3.b-12.3.c):
//
//  1. Pemicu WAJIB apa pun besarnya nominal: macet, pernah direstrukturisasi, DPD
//     melewati ambang, penurunan agunan, atau bukti objektif yang ditandai pengelola.
//  2. Signifikansi (PRAKTIK, bukan aturan tertulis): outstanding ≥ ambang Rp per
//     debitur, atau termasuk N debitur terbesar.
//
// Hasilnya adalah USULAN: penandaan pada kredit (loans.ckpn_method dsb.) tetap lewat
// alur persetujuan pengelola kredit, bukan otomatis.

// CKPNIndividualMethodExcludedAsetBaik menandai kredit yang bank keluarkan dari jalur
// individual karena gugur kriteria penurunan nilai (aset baik, tidak terpapar). Nilai
// ini disimpan pada loans.ckpn_method sehingga keputusan bank terdokumentasi dan dapat
// diaudit; kredit berpenanda ini tidak dinilai individual apa pun nominalnya.
const CKPNIndividualMethodExcludedAsetBaik = CKPNIndividualMethod("EXCLUDED_ASET_BAIK")

// CKPNEntryTrigger menyebut satu alasan kredit masuk jalur individual.
type CKPNEntryTrigger struct {
	Code   string `json:"code"`
	Reason string `json:"reason"`
	// Mandatory bernilai true untuk pemicu wajib (PA BPR 12.3.c) dan false untuk
	// kebijakan signifikansi (praktik bank).
	Mandatory bool `json:"mandatory"`
}

// CKPNIndividualEntryCheck adalah masukan penilaian pintu masuk untuk satu kredit.
// Struktur murni: seluruh keadaan kredit dibawa pemanggil, tanpa akses basis data.
type CKPNIndividualEntryCheck struct {
	// Outstanding adalah sisa pokok kredit saat ini.
	Outstanding decimal.Decimal
	// Rank adalah peringkat kredit di antara seluruh kredit aktif menurut sisa pokok
	// (1 = terbesar). Nol berarti peringkat tidak dihitung; aturan TopN dilewati.
	Rank int
	// Keadaan kredit: kolektibilitas, hari tunggakan, dan penanda restrukturisasi.
	Collectibility OJKCollectibility
	DPD            int
	IsRestructured bool
	// HasCollateral menandai kredit memiliki agunan AKTIF; dipakai hanya untuk
	// menyarankan metode, bukan untuk menentukan masuknya jalur.
	HasCollateral bool
	// ObjectiveEvidence ditandai pengelola kredit: bukti objektif penurunan nilai
	// lain di luar pemicu otomatis (mis. pabrik debitur terbakar).
	ObjectiveEvidence bool
	// CollateralDrop ditandai perhitungan agunan: taksasi turun signifikan dibanding
	// penilaian sebelumnya. Domain tidak menebak apa yang "signifikan".
	CollateralDrop bool
	// ExcludedAsetBaik menandai kredit yang dikeluarkan bank dari jalur individual
	// karena gugur kriteria penurunan nilai (aset baik, tidak terpapar).
	ExcludedAsetBaik bool
}

// CKPNIndividualEntry adalah hasil penilaian pintu masuk satu kredit.
type CKPNIndividualEntry struct {
	// Individual bernilai true bila kredit WAJIB dinilai individual.
	Individual bool `json:"individual"`
	// Significance bernilai true bila kredit memenuhi kriteria signifikansi. Dipisah
	// dari pemicu wajib supaya keduanya tidak tertukar dalam laporan.
	Significance bool `json:"significance"`
	// Triggers memuat seluruh alasan yang terpenuhi.
	Triggers []CKPNEntryTrigger `json:"triggers"`
	// SuggestedMethod menuntun pengelola: MAX bila kredit beragunan (konservatif dan
	// memenuhi lantai CKPN terbentuk sebelumnya), DCF bila tanpa agunan. Tetap boleh
	// ditimpa saat penandaan.
	SuggestedMethod CKPNIndividualMethod `json:"suggested_method"`
	// ExcludedAsetBaik diteruskan apa adanya: kredit aset baik tidak dinilai individual
	// meskipun nominalnya besar, karena kewajiban individual lahir dari bukti objektif.
	ExcludedAsetBaik bool `json:"excluded_aset_baik"`
}

// EvaluateCKPNIndividualEntry menilai pintu masuk individual satu kredit. MURNI: tidak
// menyentuh basis data dan tidak mengubah penandaan apa pun.
func EvaluateCKPNIndividualEntry(check CKPNIndividualEntryCheck, policy CKPNIndividualPolicy) CKPNIndividualEntry {
	entry := CKPNIndividualEntry{
		SuggestedMethod:  CKPNIndividualMethodDCF,
		ExcludedAsetBaik: check.ExcludedAsetBaik,
	}

	// Kredit aset baik yang dikeluarkan tidak dinilai individual apa pun keadaannya;
	// keputusan itu tetap dilaporkan agar terdokumentasi (12.4.c.1).
	if check.ExcludedAsetBaik {
		return entry
	}

	add := func(code, reason string, mandatory bool) {
		entry.Triggers = append(entry.Triggers, CKPNEntryTrigger{Code: code, Reason: reason, Mandatory: mandatory})
	}

	// Pintu 1: pemicu WAJIB, tanpa memandang nominal.
	if policy.MandatoryOnMacet && check.Collectibility == CollectibilityKol5 {
		add("MACET", "kolektibilitas macet (golongan 5)", true)
	}
	if policy.MandatoryOnRestructured && check.IsRestructured {
		add("RESTRUCTURED", "kredit pernah direstrukturisasi; konsesi adalah bukti objektif", true)
	}
	if policy.MandatoryDPDDays > 0 && check.DPD > policy.MandatoryDPDDays {
		add("DPD", fmt.Sprintf("hari tunggakan %d melewati ambang %d hari", check.DPD, policy.MandatoryDPDDays), true)
	}
	if policy.MandatoryOnCollateralDrop && check.CollateralDrop {
		add("COLLATERAL_DROP", "nilai agunan turun signifikan", true)
	}
	if policy.MandatoryOnObjectiveEvidence && check.ObjectiveEvidence {
		add("OBJECTIVE_EVIDENCE", "bukti objektif penurunan nilai ditandai pengelola kredit", true)
	}

	// Pintu 2: signifikansi (praktik bank; angka dari konfigurasi, bukan kode).
	significant := false
	if policy.SignificanceAmount.IsPositive() && check.Outstanding.GreaterThanOrEqual(policy.SignificanceAmount) {
		significant = true
		add("SIGNIFICANT", fmt.Sprintf("sisa pokok %s mencapai ambang signifikansi %s", check.Outstanding, policy.SignificanceAmount), false)
	}
	if policy.SignificanceTopN > 0 && check.Rank > 0 && check.Rank <= policy.SignificanceTopN {
		significant = true
		add("TOP_N", fmt.Sprintf("peringkat eksposur %d masuk %d terbesar", check.Rank, policy.SignificanceTopN), false)
	}
	entry.Significance = significant

	entry.Individual = len(entry.Triggers) > 0
	if entry.Individual && check.HasCollateral {
		entry.SuggestedMethod = CKPNIndividualMethodMax
	}
	return entry
}

// CKPNIndividualCandidate adalah satu kredit yang memenuhi jalur individual beserta
// alasannya; dipakai daftar pemindaian portofolio.
type CKPNIndividualCandidate struct {
	LoanID      uuid.UUID           `json:"loan_id"`
	LoanNumber  string              `json:"loan_number"`
	BranchCode  string              `json:"branch_code,omitempty"`
	Outstanding decimal.Decimal     `json:"outstanding"`
	Rank        int                 `json:"rank"`
	Entry       CKPNIndividualEntry `json:"entry"`
}

// CKPNIndividualScanRow adalah satu baris kredit aktif untuk pemindaian pintu masuk.
// Kolomnya persis yang dibutuhkan EvaluateCKPNIndividualEntry, tanpa lebih.
type CKPNIndividualScanRow struct {
	LoanID            uuid.UUID         `json:"loan_id"`
	LoanNumber        string            `json:"loan_number"`
	BranchCode        string            `json:"branch_code,omitempty"`
	Outstanding       decimal.Decimal   `json:"outstanding"`
	Collectibility    OJKCollectibility `json:"collectibility"`
	DPD               int               `json:"dpd"`
	IsRestructured    bool              `json:"is_restructured"`
	ObjectiveEvidence bool              `json:"objective_evidence"`
	// Method adalah loans.ckpn_method hasil keputusan pengelola sebelumnya; nilai
	// EXCLUDED_ASET_BAIK membuat kredit dilaporkan sebagai pengecualian, bukan kandidat.
	Method        CKPNIndividualMethod `json:"method,omitempty"`
	HasCollateral bool                 `json:"has_collateral"`
}

// CKPNIndividualScanResult adalah hasil pemindaian portofolio: seluruh kredit yang
// memenuhi jalur individual, terurut sisa pokok terbesar dahulu, beserta kebijakan
// yang dipakai agar hasil dapat direproduksi.
type CKPNIndividualScanResult struct {
	// Individual memuat kredit yang WAJIB dinilai individual beserta alasannya.
	Individual []CKPNIndividualCandidate `json:"individual"`
	// ExcludedAsetBaik memuat kredit yang dikeluarkan bank dari jalur individual.
	ExcludedAsetBaik []string `json:"excluded_aset_baik,omitempty"`
	// Scanned adalah jumlah kredit yang dinilai pintu masuknya.
	Scanned int `json:"scanned"`
	// Policy adalah kebijakan yang dipakai pemindaian (untuk jejak audit).
	Policy CKPNIndividualPolicy `json:"policy"`
}

// ScanIndividualEntries menilai pintu masuk seluruh baris pemindaian. Peringkat
// dihitung dari urutan baris (harus terurut sisa pokok terbesar dahulu). Fungsi murni.
func ScanIndividualEntries(rows []CKPNIndividualScanRow, excluded map[uuid.UUID]bool, policy CKPNIndividualPolicy) CKPNIndividualScanResult {
	res := CKPNIndividualScanResult{Individual: []CKPNIndividualCandidate{}, Scanned: len(rows), Policy: policy}
	for i, row := range rows {
		check := CKPNIndividualEntryCheck{
			Outstanding:       row.Outstanding,
			Rank:              i + 1,
			Collectibility:    row.Collectibility,
			DPD:               row.DPD,
			IsRestructured:    row.IsRestructured,
			HasCollateral:     row.HasCollateral,
			ObjectiveEvidence: row.ObjectiveEvidence,
			ExcludedAsetBaik:  excluded[row.LoanID] || row.Method == CKPNIndividualMethodExcludedAsetBaik,
		}
		entry := EvaluateCKPNIndividualEntry(check, policy)
		if entry.ExcludedAsetBaik {
			res.ExcludedAsetBaik = append(res.ExcludedAsetBaik, row.LoanNumber)
			continue
		}
		if !entry.Individual {
			continue
		}
		res.Individual = append(res.Individual, CKPNIndividualCandidate{
			LoanID:      row.LoanID,
			LoanNumber:  row.LoanNumber,
			BranchCode:  row.BranchCode,
			Outstanding: row.Outstanding,
			Rank:        i + 1,
			Entry:       entry,
		})
	}
	return res
}
