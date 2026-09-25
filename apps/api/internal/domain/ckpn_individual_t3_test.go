package domain

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
)

// Uji pintu masuk jalur CKPN individual T3. Fungsi murni: tanpa basis data.
// Setiap kasus menyatakan perilaku yang dijamin PA BPR 12.3.b-12.3.c dan keputusan
// panel: pemicu wajib tak memandang nominal, signifikansi praktik, pengecualian aset
// baik menutup seluruh pintu.

func policyT3() CKPNIndividualPolicy {
	return CKPNIndividualPolicy{
		Enabled:                      false, // pemindaian bayangan tetap boleh berjalan
		SignificanceAmount:           decimal.RequireFromString("1000000000"),
		SignificanceTopN:             20,
		MandatoryOnMacet:             true,
		MandatoryOnRestructured:      true,
		MandatoryDPDDays:             90,
		MandatoryOnCollateralDrop:    true,
		MandatoryOnObjectiveEvidence: true,
	}
}

func TestIntegrasiT3PemicuWajibApaPunNominal(t *testing.T) {
	p := policyT3()
	kecil := decimal.RequireFromString("50000000") // jauh di bawah ambang signifikansi

	kasus := []struct {
		nama  string
		check CKPNIndividualEntryCheck
		kode  string
	}{
		{"macet", CKPNIndividualEntryCheck{Outstanding: kecil, Collectibility: CollectibilityKol5}, "MACET"},
		{"restrukturisasi", CKPNIndividualEntryCheck{Outstanding: kecil, IsRestructured: true}, "RESTRUCTURED"},
		{"dpd", CKPNIndividualEntryCheck{Outstanding: kecil, DPD: 91}, "DPD"},
		{"agunan_turun", CKPNIndividualEntryCheck{Outstanding: kecil, CollateralDrop: true}, "COLLATERAL_DROP"},
		{"bukti_objektif", CKPNIndividualEntryCheck{Outstanding: kecil, ObjectiveEvidence: true}, "OBJECTIVE_EVIDENCE"},
	}
	for _, k := range kasus {
		entry := EvaluateCKPNIndividualEntry(k.check, p)
		if !entry.Individual {
			t.Fatalf("%s: kredit wajib dinilai individual", k.nama)
		}
		if len(entry.Triggers) != 1 || entry.Triggers[0].Code != k.kode || !entry.Triggers[0].Mandatory {
			t.Fatalf("%s: pemicu salah: %+v", k.nama, entry.Triggers)
		}
	}
}

func TestIntegrasiT3DPDPersisAmbangTidakMemicu(t *testing.T) {
	// PA BPR 12.3.c: "lebih dari 90 hari". Persis 90 belum memicu.
	entry := EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: decimal.RequireFromString("1000000"), DPD: 90,
	}, policyT3())
	if entry.Individual {
		t.Fatalf("DPD persis 90 tidak boleh memicu jalur individual: %+v", entry.Triggers)
	}
}

func TestIntegrasiT3SignifikansiAmbangDanTopN(t *testing.T) {
	p := policyT3()
	// Melewati ambang nominal.
	entry := EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: decimal.RequireFromString("1000000000"),
	}, p)
	if !entry.Individual || !entry.Significance {
		t.Fatalf("sisa pokok = ambang harus signifikan: %+v", entry)
	}
	// Di bawah ambang tetapi masuk TopN.
	entry = EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: decimal.RequireFromString("900000000"), Rank: 20,
	}, p)
	if !entry.Individual || !entry.Significance {
		t.Fatalf("peringkat 20 harus masuk TopN: %+v", entry)
	}
	// Di luar TopN dan di bawah ambang: tidak individual.
	entry = EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: decimal.RequireFromString("900000000"), Rank: 21,
	}, p)
	if entry.Individual {
		t.Fatalf("di luar TopN dan di bawah ambang tidak boleh masuk: %+v", entry.Triggers)
	}
}

func TestIntegrasiT3PengecualianAsetBaikMenutupSemuaPintu(t *testing.T) {
	p := policyT3()
	// Kredit Rp5 miliar, macet, direstrukturisasi — tetapi bank menandai pengecualian.
	entry := EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding:      decimal.RequireFromString("5000000000"),
		Collectibility:   CollectibilityKol5,
		IsRestructured:   true,
		ExcludedAsetBaik: true,
	}, p)
	if entry.Individual || len(entry.Triggers) != 0 {
		t.Fatalf("pengecualian aset baik harus menutup seluruh pintu: %+v", entry)
	}
	if !entry.ExcludedAsetBaik {
		t.Fatalf("pengecualian harus dilaporkan apa adanya")
	}
}

func TestIntegrasiT3SaranMetodeAgunan(t *testing.T) {
	p := policyT3()
	kecil := decimal.RequireFromString("50000000")
	// Beragunan: saran MAX (konservatif, lantai CKPN terbentuk sebelumnya).
	entry := EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: kecil, Collectibility: CollectibilityKol5, HasCollateral: true,
	}, p)
	if entry.SuggestedMethod != CKPNIndividualMethodMax {
		t.Fatalf("kredit beragunan disarankan MAX, dapat %s", entry.SuggestedMethod)
	}
	// Tanpa agunan: DCF satu-satunya yang berlaku.
	entry = EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: kecil, Collectibility: CollectibilityKol5,
	}, p)
	if entry.SuggestedMethod != CKPNIndividualMethodDCF {
		t.Fatalf("kredit tanpa agunan disarankan DCF, dapat %s", entry.SuggestedMethod)
	}
}

func TestIntegrasiT3PemindaianPortofolio(t *testing.T) {
	// Portofolio 5 kredit: TopN diperingkat menjadi 2 agar jalur TopN dan jalur
	// pemicu wajib dapat dibedakan; dengan TopN 20 seluruh portofolio ini masuk.
	p := policyT3()
	p.SignificanceTopN = 2
	rows := []CKPNIndividualScanRow{
		{LoanNumber: "L-001", Outstanding: decimal.RequireFromString("3000000000"),
			Collectibility: CollectibilityKol1, HasCollateral: true}, // signifikan, peringkat 1
		{LoanNumber: "L-002", Outstanding: decimal.RequireFromString("50000000"),
			Collectibility: CollectibilityKol5}, // macet
		{LoanNumber: "L-003", Outstanding: decimal.RequireFromString("40000000"),
			Collectibility: CollectibilityKol1, Method: CKPNIndividualMethodExcludedAsetBaik}, // pengecualian
		{LoanNumber: "L-004", Outstanding: decimal.RequireFromString("30000000"),
			Collectibility: CollectibilityKol1, DPD: 100}, // DPD > 90
		{LoanNumber: "L-005", Outstanding: decimal.RequireFromString("20000000"),
			Collectibility: CollectibilityKol1}, // aset baik kecil: tidak masuk
	}
	res := ScanIndividualEntries(rows, nil, p)
	if res.Scanned != 5 {
		t.Fatalf("harus memindai 5 kredit, dapat %d", res.Scanned)
	}
	if len(res.Individual) != 3 {
		t.Fatalf("harus 3 kandidat, dapat %d: %+v", len(res.Individual), res.Individual)
	}
	if res.Individual[0].LoanNumber != "L-001" || !res.Individual[0].Entry.Significance {
		t.Fatalf("L-001 harus signifikan: %+v", res.Individual[0])
	}
	if res.Individual[0].Rank != 1 {
		t.Fatalf("L-001 peringkat 1, dapat %d", res.Individual[0].Rank)
	}
	if res.Individual[1].LoanNumber != "L-002" {
		t.Fatalf("L-002 harus kandidat kedua")
	}
	if res.Individual[2].LoanNumber != "L-004" {
		t.Fatalf("L-004 harus kandidat ketiga (DPD), dapat %s", res.Individual[2].LoanNumber)
	}
	if len(res.ExcludedAsetBaik) != 1 || res.ExcludedAsetBaik[0] != "L-003" {
		t.Fatalf("L-003 harus dilaporkan pengecualian: %+v", res.ExcludedAsetBaik)
	}
}

func TestIntegrasiT3KebijakanPemicuDimatikan(t *testing.T) {
	// Bank boleh mematikan pemicu tertentu lewat konfigurasi; domain harus patuh,
	// bukan memaksakan pemicu yang sudah dimatikan.
	p := policyT3()
	p.MandatoryOnCollateralDrop = false
	entry := EvaluateCKPNIndividualEntry(CKPNIndividualEntryCheck{
		Outstanding: decimal.RequireFromString("50000000"), CollateralDrop: true,
	}, p)
	if entry.Individual {
		t.Fatalf("pemicu agunan-turun sudah dimatikan tetapi tetap memicu: %+v", entry.Triggers)
	}
}

// Kontrak JSON kebijakan T3: seluruh kunci snake_case, sejalan dengan bidang lain pada
// respons scan. Uji ini mengunci kontrak agar konsumen (web) tidak diam-diam rusak bila
// tag json dihapus atau diganti.
func TestIntegrasiT3KebijakanSerialisasiSnakeCase(t *testing.T) {
	raw, err := json.Marshal(policyT3())
	if err != nil {
		t.Fatalf("marshal kebijakan: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal kebijakan: %v", err)
	}
	wajib := []string{
		"enabled", "significance_amount", "significance_top_n", "method",
		"discount_rate_annual_pct", "mandatory_on_macet", "mandatory_on_restructured",
		"mandatory_dpd_days", "mandatory_on_collateral_drop", "mandatory_on_objective_evidence",
	}
	for _, k := range wajib {
		if _, ok := m[k]; !ok {
			t.Fatalf("kunci snake_case %q tidak ada pada JSON kebijakan: %v", k, m)
		}
	}
	if _, adaPascal := m["SignificanceAmount"]; adaPascal {
		t.Fatalf("kebijakan masih memakai nama field PascalCase di JSON")
	}
}
