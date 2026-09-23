package domain

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Gerbang aktivasi bobot risiko agunan — keputusan panel butir 3.
//
// SEOJK No. 2/SEOJK.03/2025 Lampiran II membolehkan bobot risiko kredit di bawah 100%
// bila agunan memenuhi syarat. Panel menetapkan NINE syarat (C1-C9) plus tiga gerbang
// tambahan (mode bayangan minimal 2 bulan bisnis, cakupan nilai agunan >= 90% lolos
// C1-C4, dan tanggung jawab Direksi lewat maker-checker). Berkas ini adalah satu tempat
// yang memeriksa semuanya SEBELUM sebuah kategori boleh dinyalakan, sehingga pengaktifan
// tidak dapat "lolos" karena satu syarat terlewat.
//
// Semua penamaan bobot memakai FRAKSI 0..1 (bukan persen), konsisten dengan kolom
// official_weight_frac/applied_weight_frac pada collateral_lampiran_ii_weights.

// Kode syarat yang dikembalikan pada Failures. Dipisah sebagai konstanta agar pesan
// dapat menyebut syarat mana yang gagal dan pengujian tidak bergantung string bebas.
const (
	CollateralWeightC1 = "C1" // binding_type terisi
	CollateralWeightC2 = "C2" // asuransi terisi dan belum kedaluwarsa
	CollateralWeightC3 = "C3" // taksasi belum kedaluwarsa
	CollateralWeightC4 = "C4" // tidak sengketa
	CollateralWeightC5 = "C5" // kategori ada dan bobot resmi terisi
	CollateralWeightC6 = "C6" // bagian tak dijamin agunan tetap 100%
	CollateralWeightC7 = "C7" // applied = official untuk semua baris
	CollateralWeightC8 = "C8" // kebijakan Direksi bertanggal
	CollateralWeightC9 = "C9" // izin system:config + maker-checker dua orang
	// Gerbang tambahan non-C.
	CollateralWeightShadow   = "SHADOW"   // mode bayangan minimal 2 bulan bisnis
	CollateralWeightCoverage = "COVERAGE" // cakupan nilai agunan lolos C1-C4 >= ambang
)

// ErrCollateralWeightActivationRejected menandai aktivasi kategori bobot agunan yang
// ditolak gerbang. Pesannya menyebut syarat yang gagal; pemanggil tidak boleh menebak.
// Dibuat LocalizedError agar pesan dasarnya mengikuti bahasa instalasi, sementara
// daftar syarat yang gagal ditambahkan pemanggil sebagai akhiran tetap.
var ErrCollateralWeightActivationRejected = NewLocalizedError("collateral_weight_activation_rejected", "aktivasi bobot agunan ditolak")

// CollateralWeightCategory adalah satu baris kategori pada
// collateral_lampiran_ii_weights beserta penanda persetujuannya.
type CollateralWeightCategory struct {
	CategoryCode       string
	LampiranIIItem     int
	OfficialWeightFrac decimal.Decimal
	AppliedWeightFrac  decimal.Decimal
	Enabled            bool
	// CollateralType/BindingType menentukan cakupan agunan kategori; nil berarti
	// kategori tidak dibatasi jenis/ikatan (mis. UMK, jatuh tempo/macet).
	CollateralType *CollateralType
	BindingType    *CollateralBinding
	// Penanda C8 (kebijakan Direksi) dan C9 (maker/checker) disimpan pada baris.
	DireksiPolicyNumber string
	DireksiPolicyDate   *time.Time
	ShadowStartedAt     *time.Time
	// ActivatedMaker/ActivatedBy adalah pembuat dan penyetuju aktivasi (C9). Keduanya
	// HANYA diisi dari aktor terautentikasi, tidak pernah dari body/kueri.
	ActivatedMaker string
	ActivatedBy    string
	ActivatedAt    *time.Time
}

// CollateralWeightActivateAction adalah jenis aksi maker-checker aktivasi kategori
// bobot agunan. Aktivasi selalu melewati antrean maker-checker: pengaju (maker) dan
// penyetuju (checker) adalah dua aktor terautentikasi yang berbeda (C9), sehingga
// identitas tidak mungkin diklaim dari body/kueri.
const CollateralWeightActivateAction = "COLLATERAL_WEIGHT_ACTIVATE"

// CollateralWeightCollateral adalah satu agunan aktif dalam cakupan kategori. Nilai
// pengurang memakai BoundAmount (database generated dari taksasi x (1-haircut)),
// sedangkan Coverage memakai AppraisalValue (nilai agunan), sesuai keputusan panel
// butir 3.3: cakupan dihitung atas NILAI AGUNAN.
type CollateralWeightCollateral struct {
	LoanID              uuid.UUID
	CollateralType      CollateralType
	BindingType         *CollateralBinding
	Disputed            bool
	InsuranceExpiryDate *time.Time
	AppraisalDate       time.Time
	AppraisalValidUntil *time.Time
	AppraisalValue      decimal.Decimal
	BoundAmount         decimal.Decimal
	Status              CollateralStatus
}

// CollateralWeightActivationRequest adalah seluruh masukan gerbang. Approval/aktor
// dibawa terpisah dari data agunan supaya pengujian dapat memvariasikan tiap syarat
// tanpa database.
type CollateralWeightActivationRequest struct {
	Category CollateralWeightCategory
	// CandidateAppliedFrac adalah bobot yang AKAN dipakai bila diaktifkan. Syarat C7
	// mewajibkan nilainya sama dengan OfficialWeightFrac; nilai lain ditolak.
	CandidateAppliedFrac decimal.Decimal
	Collaterals          []CollateralWeightCollateral
	// LoanExposure adalah sisa pokok per kredit; dipakai C6 (bagian tak dijamin tetap
	// 100%): total pengurang agunan layak per kredit tidak boleh melebihi eksposurnya.
	LoanExposure map[uuid.UUID]decimal.Decimal

	AsOf           time.Time
	ValidityMonths int

	// Kebijakan aktivasi (dari konfigurasi; bawaan aman).
	ShadowMonthsRequired int
	CoverageMinFrac      decimal.Decimal

	// Persetujuan dan tanggung jawab.
	DireksiPolicyNumber string
	DireksiPolicyDate   *time.Time
	Maker               string
	Checker             string
	HasConfigPermission bool
}

// CollateralWeightActivationResult adalah hasil gerbang. Allowed true berarti kategori
// boleh dinyalakan; Failures berisi kode syarat yang gagal (kosong bila lolos).
// Field status dipakai endpoint baca-saja agar syarat, cakupan, dan umur mode bayangan
// dapat diaudit tanpa menyalakan kategori.
type CollateralWeightActivationResult struct {
	Allowed       bool
	Failures      []string
	CoverageFrac  decimal.Decimal
	EligibleValue decimal.Decimal
	TotalValue    decimal.Decimal

	// CategoryCode/CategoryEnabled menyalin keadaan kategori saat dinilai.
	CategoryCode    string
	CategoryEnabled bool
	// CoverageMinFrac adalah ambang cakupan yang dipakai gerbang.
	CoverageMinFrac decimal.Decimal
	// ShadowStartedAt nol berarti mode bayangan belum dimulai. ShadowMonthsElapsed
	// adalah jumlah bulan penuh mode bayangan berjalan per AsOf (-1 bila belum mulai).
	ShadowStartedAt      *time.Time
	ShadowMonthsElapsed  int
	ShadowMonthsRequired int
}

// CollateralWeightActivationApproval adalah permintaan aktivasi dari pemanggil.
// Dibawa terpisah dari data agunan agar gerbang dapat menilai syarat C8/C9 tanpa
// menyentuh database.
type CollateralWeightActivationApproval struct {
	DireksiPolicyNumber string
	DireksiPolicyDate   *time.Time
	// Maker adalah pengaju, Checker adalah penyetuju. Gerbang menuntut keduanya terisi
	// dan berbeda; pengaju tidak boleh menyetujui pengajuannya sendiri.
	Maker string
	// Checker diisi handler dari identitas aktor yang benar-benar memanggil aktivasi.
	Checker string
	// AppliedWeightFrac nil berarti memakai bobot resmi (C7 lolos); diisi nilai lain
	// akan ditolak gerbang.
	AppliedWeightFrac   *decimal.Decimal
	HasConfigPermission bool
}

// CollateralWeightService adalah gerbang aktivasi bobot agunan. Assess menilai syarat
// tanpa mengubah apa pun (baca-saja). Aktivasi tidak lagi diekspos sebagai satu operasi
// yang menerima identitas: ia dieksekusi lewat antrean maker-checker (MakerCheckerExecutor),
// sehingga pembuat dan penyetuju selalu berasal dari aktor terautentikasi yang berbeda.
type CollateralWeightService interface {
	MakerCheckerExecutor
	Assess(ctx context.Context, categoryCode string, approval CollateralWeightActivationApproval, actor Actor) (CollateralWeightActivationResult, error)
}

// EvaluateCollateralWeightActivation menjalankan seluruh syarat C1-C9 dan gerbang
// tambahan. Fungsi ini murni: tidak menyentuh database dan tidak mengaktifkan apa pun.
func EvaluateCollateralWeightActivation(req CollateralWeightActivationRequest) CollateralWeightActivationResult {
	res := CollateralWeightActivationResult{}
	var failures []string
	fail := func(kode, pesan string) {
		failures = append(failures, kode+": "+pesan)
	}

	// C5: kategori ada dan bobot resmi terisi.
	if strings.TrimSpace(req.Category.CategoryCode) == "" {
		fail(CollateralWeightC5, "kode kategori kosong; kategori (collateral_type, binding_type) harus ada dengan official_weight_frac terisi")
	} else if !req.Category.OfficialWeightFrac.IsPositive() {
		fail(CollateralWeightC5, fmt.Sprintf("kategori %s tidak punya bobot resmi (official_weight_frac) yang terisi", req.Category.CategoryCode))
	}

	// C7: bobot yang akan dipakai harus sama dengan bobot resmi.
	if !req.CandidateAppliedFrac.Equal(req.Category.OfficialWeightFrac) {
		fail(CollateralWeightC7, fmt.Sprintf("applied_weight_frac %s harus sama dengan official_weight_frac %s; nilai lain ditolak",
			req.CandidateAppliedFrac, req.Category.OfficialWeightFrac))
	}

	// C8: kebijakan Direksi bertanggal.
	if strings.TrimSpace(req.DireksiPolicyNumber) == "" || req.DireksiPolicyDate == nil || req.DireksiPolicyDate.IsZero() {
		fail(CollateralWeightC8, "kebijakan Direksi (nomor dan tanggal surat) yang menyetujui bobot belum ada")
	} else if tanggalSaja(*req.DireksiPolicyDate).After(tanggalSaja(req.AsOf)) {
		fail(CollateralWeightC8, "tanggal kebijakan Direksi tidak boleh di masa depan")
	}

	// C9: izin system:config + maker-checker dua orang berbeda.
	switch {
	case !req.HasConfigPermission:
		fail(CollateralWeightC9, "aktivasi hanya lewat izin system:config")
	case strings.TrimSpace(req.Maker) == "" || strings.TrimSpace(req.Checker) == "":
		fail(CollateralWeightC9, "pembuat (maker) dan penyetuju (checker) wajib tercatat")
	case strings.EqualFold(strings.TrimSpace(req.Maker), strings.TrimSpace(req.Checker)):
		fail(CollateralWeightC9, "penyetuju (checker) harus berbeda dari pembuat (maker)")
	}

	// Gerbang mode bayangan: minimal ShadowMonthsRequired bulan bisnis sejak mulai.
	// Nol berarti gerbang ini belum diisi; diperlakukan sebagai belum terpenuhi agar
	// aktivasi tidak lolos karena konfigurasi kosong.
	if req.Category.ShadowStartedAt == nil || req.Category.ShadowStartedAt.IsZero() {
		fail(CollateralWeightShadow, "mode bayangan belum dimulai; aktivasi baru boleh setelah lama minimum berlalu")
	} else {
		minMulai := tanggalSaja(req.AsOf).AddDate(0, -req.ShadowMonthsRequired, 0)
		if tanggalSaja(*req.Category.ShadowStartedAt).After(minMulai) {
			fail(CollateralWeightShadow, fmt.Sprintf("mode bayangan baru berjalan sejak %s; minimal %d bulan bisnis sebelum aktivasi",
				tanggalSaja(*req.Category.ShadowStartedAt).Format("2006-01-02"), req.ShadowMonthsRequired))
		}
	}

	// C1-C4 per agunan + cakupan nilai agunan (hanya agunan yang lolos C1-C4 dihitung).
	// Cakupan "yang relevan" = agunan yang JENISNYA sama dengan jenis kategori (bila
	// kategori dibatasi jenis); ikatan yang berbeda adalah kategori lain dan tidak
	// boleh menaikkan cakupan kategori ini.
	var eligibleValue, totalValue decimal.Decimal
	// eligibleBoundPerLoan untuk C6: total pengurang agunan layak per kredit.
	eligibleBoundPerLoan := map[uuid.UUID]decimal.Decimal{}
	for i := range req.Collaterals {
		c := &req.Collaterals[i]
		// Hanya agunan AKTIF yang relevan; agunan lepas/eksekusi tidak menjamin apa pun.
		if c.Status != "" && c.Status != CollateralActive {
			continue
		}
		if req.Category.CollateralType != nil && c.CollateralType != *req.Category.CollateralType {
			continue
		}
		totalValue = totalValue.Add(c.AppraisalValue)

		if c.BindingType == nil {
			fail(CollateralWeightC1, fmt.Sprintf("agunan pada kredit %s belum punya binding_type", c.LoanID))
			continue
		}
		// Ikatan yang tidak cocok bukan kegagalan C1-C4, tetapi bukan bagian kategori
		// ini: tidak dihitung layak dan tidak menaikkan cakupan.
		if req.Category.BindingType != nil && *c.BindingType != *req.Category.BindingType {
			continue
		}
		if c.InsuranceExpiryDate == nil || c.InsuranceExpiryDate.IsZero() {
			fail(CollateralWeightC2, fmt.Sprintf("agunan pada kredit %s belum punya insurance_expiry_date", c.LoanID))
			continue
		}
		if tanggalSaja(req.AsOf).After(tanggalSaja(*c.InsuranceExpiryDate)) {
			fail(CollateralWeightC2, fmt.Sprintf("polis asuransi agunan pada kredit %s sudah kedaluwarsa (%s)", c.LoanID, tanggalSaja(*c.InsuranceExpiryDate).Format("2006-01-02")))
			continue
		}
		expired := AppraisalExpiredFor(c.AppraisalDate, c.AppraisalValidUntil, req.AsOf, req.ValidityMonths)
		if expired {
			fail(CollateralWeightC3, fmt.Sprintf("taksasi agunan pada kredit %s sudah kedaluwarsa", c.LoanID))
			continue
		}
		if c.Disputed {
			fail(CollateralWeightC4, fmt.Sprintf("agunan pada kredit %s terbukti sengketa; wajib 100%%", c.LoanID))
			continue
		}

		eligibleValue = eligibleValue.Add(c.AppraisalValue)
		eligibleBoundPerLoan[c.LoanID] = eligibleBoundPerLoan[c.LoanID].Add(c.BoundAmount)
	}

	// C6: bagian kredit yang TIDAK dijamin agunan tetap 100%. Mesin ATMR per agunan
	// belum ada, jadi yang dapat diperiksa mesin adalah syarat matematis: total
	// pengurang agunan layak sebuah kredit tidak boleh melebihi eksposurnya, sehingga
	// sisa (bagian tak dijamin) tidak pernah negatif dan tidak ikut diturunkan.
	for loanID, bound := range eligibleBoundPerLoan {
		if exposure, ok := req.LoanExposure[loanID]; ok && exposure.IsPositive() && bound.GreaterThan(exposure) {
			fail(CollateralWeightC6, fmt.Sprintf(
				"pengurang agunan layak kredit %s (%s) melebihi eksposurnya (%s): bagian tak dijamin harus tetap 100%%",
				loanID, bound, exposure))
		}
	}

	// Gerbang cakupan: >= CoverageMinFrac dari NILAI agunan aktif harus lolos C1-C4.
	res.EligibleValue = eligibleValue
	res.TotalValue = totalValue
	if totalValue.IsPositive() {
		res.CoverageFrac = eligibleValue.Div(totalValue)
	}
	if !res.CoverageFrac.GreaterThanOrEqual(req.CoverageMinFrac) {
		fail(CollateralWeightCoverage, fmt.Sprintf(
			"cakupan nilai agunan lolos C1-C4 %s dari %s (fraksi %s) di bawah minimum %s",
			eligibleValue, totalValue, res.CoverageFrac, req.CoverageMinFrac))
	}

	res.Failures = failures
	res.Allowed = len(failures) == 0
	res.CategoryCode = req.Category.CategoryCode
	res.CategoryEnabled = req.Category.Enabled
	res.CoverageMinFrac = req.CoverageMinFrac
	res.ShadowMonthsRequired = req.ShadowMonthsRequired
	res.ShadowStartedAt = req.Category.ShadowStartedAt
	if req.Category.ShadowStartedAt != nil && !req.Category.ShadowStartedAt.IsZero() {
		res.ShadowMonthsElapsed = businessMonthsElapsed(*req.Category.ShadowStartedAt, req.AsOf)
	} else {
		res.ShadowMonthsElapsed = -1
	}
	return res
}

// businessMonthsElapsed menghitung jumlah bulan penuh dari start sampai asOf,
// memperhitungkan tanggal sehingga 15 Jan -> 14 Mar = 1 bulan, 15 Jan -> 15 Mar = 2.
// Dipakai melaporkan berapa bulan bisnis mode bayangan sudah berjalan.
func businessMonthsElapsed(start, asOf time.Time) int {
	start, asOf = tanggalSaja(start), tanggalSaja(asOf)
	if start.IsZero() || start.After(asOf) {
		return 0
	}
	months := (asOf.Year()-start.Year())*12 + int(asOf.Month()) - int(start.Month())
	if asOf.Day() < start.Day() {
		months--
	}
	if months < 0 {
		return 0
	}
	return months
}

// AppraisalExpiredFor menilai kedaluwarsa taksasi dari data mentah (tanpa struct
// LoanCollateral) agar gerbang dapat dipakai atas baris hasil query ringkas. Batas
// tegas AppraisalValidUntil menang; bila kosong umur dihitung dari AppraisalDate.
func AppraisalExpiredFor(appraisalDate time.Time, validUntil *time.Time, asOf time.Time, validityMonths int) bool {
	if validUntil != nil && !validUntil.IsZero() {
		return tanggalSaja(asOf).After(tanggalSaja(*validUntil))
	}
	if appraisalDate.IsZero() {
		return true
	}
	return tanggalSaja(asOf).After(tanggalSaja(appraisalDate).AddDate(0, NormalizeAppraisalValidityMonths(validityMonths), 0))
}

// CollateralWeightRepository membaca/menyimpan data gerbang aktivasi. Dipisah dari
// CollateralRepository karena bobot adalah kebijakan tingkat bank, bukan agunan satu
// kredit. Aktivasi HANYA boleh lewat EnableCategory setelah gerbang lolos.
type CollateralWeightRepository interface {
	GetCategory(ctx context.Context, categoryCode string) (*CollateralWeightCategory, error)
	// ListActiveCollateralsWithExposure mengembalikan seluruh agunan AKTIF beserta sisa
	// pokok kreditnya. Penyaringan kategori dilakukan gerbang, bukan query, agar
	// cakupan dapat dihitung atas nilai agunan seluruh kategori.
	ListActiveCollateralsWithExposure(ctx context.Context) ([]CollateralWeightCollateral, map[uuid.UUID]decimal.Decimal, error)
	// EnableCategory menyalakan satu kategori. Pemanggil WAJIB memastikan gerbang
	// lolos lebih dulu; fungsi ini tidak menilai ulang syarat.
	EnableCategory(ctx context.Context, categoryCode string, approval CollateralWeightApproval) error
	// EnableCategoryTx sama dengan EnableCategory tetapi memakai transaksi pemanggil,
	// sehingga penulisan bukti aktivasi dan audit persetujuan commit bersama.
	EnableCategoryTx(ctx context.Context, tx any, categoryCode string, approval CollateralWeightApproval) error
}

// CollateralWeightApproval adalah bukti persetujuan yang disimpan saat aktivasi.
type CollateralWeightApproval struct {
	AppliedWeightFrac   decimal.Decimal
	DireksiPolicyNumber string
	DireksiPolicyDate   *time.Time
	Maker               string
	Checker             string
	ActivatedAt         time.Time
}

// CollateralWeightActivationRejection membungkus hasil gerbang yang ditolak menjadi
// galat yang menyebut syarat gagal, agar pemanggil tidak dapat menyalakan kategori
// tanpa menyelesaikannya.
func CollateralWeightActivationRejection(res CollateralWeightActivationResult) error {
	if res.Allowed {
		return nil
	}
	return fmt.Errorf("%w (%s)", ErrCollateralWeightActivationRejected, strings.Join(res.Failures, "; "))
}
