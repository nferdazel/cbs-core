package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Sentinel error modul CKPN. Dipakai agar pemanggil dapat membedakan kegagalan
// konfigurasi kebijakan dari kegagalan data/tak terduga.
var (
	ErrCKPNLoanNotFound     = errors.New("kredit untuk perhitungan CKPN tidak ditemukan")
	ErrCKPNParameterMissing = errors.New("parameter kebijakan CKPN belum diisi bank")
	// ErrCKPNParameterInvalid menandai parameter yang DIISI tetapi tidak sah: salah
	// format (mis. "0,5") atau di luar rentang 0..1 (mis. "14.23" untuk persen).
	// Dibedakan dari ErrCKPNParameterMissing agar pesan tidak berkata "belum diisi"
	// pada nilai yang sebenarnya diisi keliru.
	ErrCKPNParameterInvalid = errors.New("parameter kebijakan CKPN tidak valid")
	ErrCKPNExpenseNotFound  = errors.New("akun beban kerugian penurunan nilai tidak ditemukan")
	ErrCKPNReserveNotFound  = errors.New("akun CKPN tidak ditemukan")
	// ErrCKPNStalePPAP menandai perbandingan CKPN yang tidak boleh memakai PPKA dari
	// run PPAP tanggal bisnis lain. Bila dibiarkan, laporan membandingkan CKPN
	// tanggal berjalan dengan required_ppap kemarin dan tetap terlihat sah.
	ErrCKPNStalePPAP = errors.New("perbandingan CKPN menolak PPKA dari tanggal bisnis yang tidak sama")
)

// CKPNAsetBaikMaxDPDDefault adalah batas tunggakan hari agar aset keuangan memenuhi
// kriteria "aset baik" menurut SEOJK No. 21/SEOJK.03/2024 Bab XII butir 12.3.a.1.c:
// tidak memiliki tunggakan lebih dari 7 (tujuh) hari dan tidak pernah direstrukturisasi.
// Bila aset baik, BPR "dapat tidak membentuk CKPN" (butir 12.3.a.2.a).
//
// Dua kriteria lain di butir yang sama — (a) diterbitkan Pemerintah Pusat RI dan
// (b) dijamin LPS — TIDAK dinilai karena tidak ada datanya di sistem ini. Keduanya
// diabaikan, bukan dianggap tidak terpenuhi: kriteria yang dievaluasi hanyalah
// butir 12.3.a.1.c (tunggakan dan restrukturisasi), sehingga kredit yang lolos
// kriteria itu BISA bernilai aset baik = true. Bila kelak data penerbit/penjamin
// tersedia, keduanya wajib ditambahkan di sini agar tidak ada kredit yang lolos
// karena asumsi.
const CKPNAsetBaikMaxDPDDefault = 7

// CKPNPolicy adalah parameter kebijakan bank untuk CKPN kolektif. Nilai-nilainya
// TIDAK boleh ditanam di kode: SAK EP melalui SEOJK No. 21/SEOJK.03/2024 Bab XII
// mewajibkan bank menetapkan sendiri PD (butir 12.6) dan LGD (butir 12.7) dari data
// historisnya (periode observasi minimal 3 tahun, butir 12.4.g.2.c), dengan
// judgment/diskresi manajemen (butir 12.4.c.2). Karena itu struct ini hanya wadah;
// pengisiannya berasal dari system_config.
type CKPNPolicy struct {
	// Enabled adalah saklar utama. false berarti CKPN tidak dihitung sama sekali dan
	// tidak ada query tambahan (pola yang sama dengan ppap.collateral.enabled).
	Enabled bool
	// ShadowMode adalah saklar mode bayangan. Bila Enabled masih false sementara
	// ShadowMode true, CKPN dihitung dan dibandingkan dengan PPKA TANPA menjurnal dan
	// TANPA mengubah state, lalu dilaporkan sebagai angka BAYANGAN (asumsi sementara,
	// belum disetujui bank, pengurangan modal inti belum dilakukan). Bila Enabled true,
	// mode resmi yang berlaku dan nilai ini diabaikan demi perilaku lama.
	ShadowMode bool
	// PD adalah probability of default per golongan kolektibilitas. Kunci yang tidak
	// ada berarti bank belum mengisi golongan itu, dan perhitungannya menolak berjalan
	// untuk kredit tersebut — bukan diam-diam memakai nol.
	PD map[Collectibility]decimal.Decimal
	// PDErrors mencatat golongan yang parameter PD-nya DIISI tetapi tidak sah (salah
	// format atau di luar 0..1). Golongan yang tidak ada di peta ini maupun di PD
	// berarti belum diisi. Pemisahan ini menjaga pesan "belum diisi" tidak tertukar
	// dengan "salah isi".
	PDErrors map[Collectibility]error
	// LGD adalah loss given default. Dokumen memperbolehkan LGD "all account" bila data
	// tidak mendukung pengelompokan per kategori kredit (butir 12.7.a).
	LGD decimal.Decimal
	// LGDIsSet menandai LGD benar-benar diisi. Angka nol yang diisi sengaja (mis.
	// portofolio tanpa kerugian) harus dibedakan dari LGD yang belum diisi.
	LGDIsSet bool
	// LGDError mencatat LGD yang diisi tetapi tidak sah. Terpisah dari LGDIsSet
	// dengan alasan yang sama seperti PDErrors.
	LGDError error
	// AsetBaikMaxDPD adalah batas tunggakan hari kriteria aset baik; diisi dari
	// konfigurasi dengan bawaan CKPNAsetBaikMaxDPDDefault.
	AsetBaikMaxDPD int
}

// CKPNIsAsetBaik menilai kriteria aset baik butir 12.3.a.1.c SEOJK 21/2024. Kredit
// yang pernah direstrukturisasi tidak pernah dianggap aset baik, sekalipun sedang
// tidak menunggak, karena unsur pertama kriteria itu adalah "tidak pernah dilakukan
// restrukturisasi". maxDPD biasanya CKPNAsetBaikMaxDPDDefault (7).
func CKPNIsAsetBaik(dpd int, isRestructured bool, maxDPD int) bool {
	if isRestructured {
		return false
	}
	return dpd <= maxDPD
}

// CKPNLoanSnapshot adalah data kredit aktif yang dibutuhkan perhitungan CKPN.
// PPKA yang dibandingkan adalah hasil yang SUDAH dihitung jalur PPAP
// (loans.required_ppap), bukan hitungan ulang di sini: modul CKPN tidak boleh
// menduplikasi logika PPKA.
type CKPNLoanSnapshot struct {
	LoanID     uuid.UUID
	LoanNumber string
	ProductID  *uuid.UUID
	// BranchCode adalah cabang KREDIT, bukan cabang aktor yang menjalankan. Jurnal
	// CKPN harus diatribusikan ke cabang kredit agar buku cabang tidak salah.
	BranchCode string
	// Status menentukan apakah kredit masih punya eksposur. Hanya DISBURSED dan
	// DEFAULTED yang dihitung; status lain (PAID_OFF, WRITTEN_OFF, CANCELLED) hanya
	// boleh melepas required_ckpn yang tersisa dengan target nol.
	Status         LoanStatus
	Outstanding    decimal.Decimal
	Collectibility Collectibility
	DPD            int
	IsRestructured bool
	// RequiredPPAP adalah PPKA per kredit yang sudah diakui (target terakhir jalur PPAP).
	RequiredPPAP decimal.Decimal
	// RestructureLoss adalah saldo kerugian restrukturisasi yang belum diamortisasi.
	// EAD CKPN memakai nilai tercatat setelah dikurangi saldo ini agar basisnya
	// KONSISTEN dengan PPKA yang juga dihitung atas saldo setelah kerugian (Pasal 32
	// POJK 1/2024 jo. PA BPR Bab 5.2). Tanpa ini, perbandingan PPKA vs CKPN akan
	// membandingkan dua basis yang berbeda.
	RestructureLoss decimal.Decimal
	// RequiredCKPN adalah target CKPN yang terakhir diakui untuk kredit ini.
	RequiredCKPN decimal.Decimal
}

// CKPNCalculation adalah hasil perhitungan CKPN satu kredit.
type CKPNCalculation struct {
	Outstanding    decimal.Decimal
	Collectibility Collectibility
	PD             decimal.Decimal
	LGD            decimal.Decimal
	// IsAsetBaik true berarti kredit dikecualikan dari pembentukan CKPN (butir
	// 12.3.a.2.a); targetnya nol tanpa memerlukan PD/LGD.
	IsAsetBaik bool
	Target     decimal.Decimal
	Existing   decimal.Decimal
	Adjustment decimal.Decimal
}

// CalculateCKPN menghitung target CKPN satu kredit dari data kredit dan parameter
// kebijakan bank. Rumusnya adalah pendekatan kolektif SAK EP sebagaimana
// dicontohkan SEOJK No. 21/SEOJK.03/2024 Bab XII butir 12.6–12.9, yaitu
// CKPN = EAD x PD x LGD (contoh tabel butir 12.9: baki debet x PD x LGD). EAD di sini
// adalah sisa pokok terutang.
//
// Batas yang disengaja:
//   - Pendekatan INDIVIDUAL (discounted cash flow atau nilai realisasi agunan, butir
//     12.4.g.1) TIDAK diimplementasikan: ia memerlukan estimasi arus kas per debitur
//     dan suku bunga efektif awal yang tidak tersedia andal di data sistem ini.
//   - PD/LGD tidak dihitung dari data historis oleh fungsi ini; bank menghitungnya
//     (mis. dengan Excel PD/LGD dari OJK) lalu mengisinya sebagai parameter kebijakan.
//
// Parameter yang belum diisi menghasilkan ErrCKPNParameterMissing, bukan nol.
func CalculateCKPN(snap CKPNLoanSnapshot, policy CKPNPolicy) (CKPNCalculation, error) {
	out := CKPNCalculation{
		Outstanding:    snap.Outstanding,
		Collectibility: snap.Collectibility,
		Existing:       snap.RequiredCKPN,
	}

	// Kredit yang sudah tidak aktif (PAID_OFF, WRITTEN_OFF, CANCELLED) tidak lagi
	// punya eksposur, sehingga tidak boleh membentuk CKPN baru. Tetapi required_ckpn
	// yang masih tersisa WAJIB dilepas lewat pemulihan: membiarkannya berarti cadangan
	// lebih besar daripada risikonya (salah saji). Karena itu targetnya nol dan
	// selisihnya sebesar cadangan yang tersisa. Jalur ini sengaja ditempatkan sebelum
	// pemeriksaan PD/LGD: melepas cadangan tidak memerlukan parameter kebijakan.
	if !snap.Status.IsCKPNActive() {
		out.Adjustment = RoundToRupiah(decimal.Zero.Sub(out.Existing))
		return out, nil
	}

	if CKPNIsAsetBaik(snap.DPD, snap.IsRestructured, policy.AsetBaikMaxDPD) {
		// Aset baik boleh tidak membentuk CKPN. Bila ada CKPN lama, selisihnya dipulihkan
		// (butir 12.5.b), paling tinggi sebesar yang pernah dibentuk — diwakili Existing.
		out.IsAsetBaik = true
		out.Adjustment = RoundToRupiah(decimal.Zero.Sub(out.Existing))
		return out, nil
	}

	// Parameter yang DIISI tetapi tidak sah ditolak dengan pesan tersendiri, bukan
	// diperlakukan sebagai "belum diisi" maupun dijatuhkan menjadi nol.
	if err, invalid := policy.PDErrors[snap.Collectibility]; invalid {
		return out, err
	}
	pd, ok := policy.PD[snap.Collectibility]
	if !ok {
		return out, fmt.Errorf("%w: probability of default golongan %s (kunci ckpn.pd.%d) belum diisi",
			ErrCKPNParameterMissing, snap.Collectibility.Label(), int(snap.Collectibility))
	}
	if policy.LGDError != nil {
		return out, policy.LGDError
	}
	if !policy.LGDIsSet {
		return out, fmt.Errorf("%w: loss given default (kunci ckpn.lgd) belum diisi", ErrCKPNParameterMissing)
	}

	out.PD = pd
	out.LGD = policy.LGD
	// EAD adalah nilai tercatat setelah kerugian restrukturisasi, bukan pokok bruto:
	// dasar ini sama dengan dasar PPKA sehingga perbandingan CKPN vs PPKA konsisten
	// (Pasal 32 POJK 1/2024 jo. PA BPR Bab 5.2).
	ead := PPAPCarryingAmount(snap.Outstanding, snap.RestructureLoss)
	out.Target = RoundToRupiah(ead.Mul(pd).Mul(policy.LGD))
	out.Adjustment = RoundToRupiah(out.Target.Sub(out.Existing))
	return out, nil
}

// CKPNLarger menamai pihak yang nilainya lebih besar pada perbandingan CKPN vs PPKA.
type CKPNLarger string

const (
	// CKPNLargerPPKA dipakai saat PPKA > CKPN. Menurut SEOJK No. 21/SEOJK.03/2024
	// butir 1.1.6, selisih itulah yang menjadi pengurang modal inti (ATMR/pengurang
	// modal) — bukan seluruh PPKA maupun seluruh CKPN.
	CKPNLargerPPKA CKPNLarger = "PPKA"
	// CKPNLargerCKPN dipakai saat CKPN > PPKA. Tidak ada pengurang modal inti.
	CKPNLargerCKPN CKPNLarger = "CKPN"
	// CKPNLargerSame dipakai saat keduanya sama.
	CKPNLargerSame CKPNLarger = "SAMA"
)

// CKPNComparisonItem adalah perbandingan CKPN dan PPKA satu kredit.
type CKPNComparisonItem struct {
	LoanID         uuid.UUID       `json:"loan_id"`
	LoanNumber     string          `json:"loan_number"`
	Outstanding    decimal.Decimal `json:"outstanding"`
	Collectibility Collectibility  `json:"collectibility"`
	IsAsetBaik     bool            `json:"aset_baik"`
	CKPN           decimal.Decimal `json:"ckpn"`
	PPKA           decimal.Decimal `json:"ppka"`
	// Difference adalah PPKA - CKPN. Positif berarti PPKA lebih besar dan menurut
	// butir 1.1.6 menjadi pengurang modal inti.
	Difference decimal.Decimal `json:"difference"`
	Larger     CKPNLarger      `json:"larger"`
}

// CKPNRunFailure mencatat kredit yang gagal dihitung tanpa menggagalkan seluruh proses.
type CKPNRunFailure struct {
	LoanID     uuid.UUID `json:"loan_id"`
	LoanNumber string    `json:"loan_number"`
	Error      string    `json:"error"`
}

// CKPNComparisonSummary adalah ringkasan perbandingan CKPN vs PPKA satu kali proses.
type CKPNComparisonSummary struct {
	// Enabled false berarti saklar ckpn.enabled mati: tidak ada kredit yang dibaca.
	Enabled   bool                 `json:"enabled"`
	AsOf      time.Time            `json:"as_of"`
	Total     int                  `json:"total"`
	Processed int                  `json:"processed"`
	Failed    int                  `json:"failed"`
	Items     []CKPNComparisonItem `json:"items"`
	Failures  []CKPNRunFailure     `json:"failures"`
	// TotalCKPN dan TotalPPKA adalah jumlah target, bukan saldo GL.
	TotalCKPN decimal.Decimal `json:"total_ckpn"`
	TotalPPKA decimal.Decimal `json:"total_ppka"`
	// AsetBaikCount adalah jumlah kredit yang target CKPN-nya nol karena dikecualikan
	// sebagai aset baik (butir 12.3.a.2.a), bukan karena parameter PD/LGD belum diisi
	// maupun model belum dijalankan. AsetBaikOutstanding adalah jumlah sisa pokok
	// kredit-kredit itu, sebagai ukuran eksposur yang tidak dicadangkan. Keduanya diisi
	// pada jalur hitung resmi maupun bayangan; laporan bayangan memakainya untuk
	// menjelaskan TotalCKPN nol.
	AsetBaikCount       int             `json:"aset_baik_count"`
	AsetBaikOutstanding decimal.Decimal `json:"aset_baik_outstanding"`
	// ModalIntiDeduction adalah jumlah selisih positif (PPKA > CKPN) seluruh kredit,
	// yaitu pengurang modal inti menurut SEOJK No. 21/SEOJK.03/2024 butir 1.1.6.
	ModalIntiDeduction decimal.Decimal `json:"modal_inti_deduction"`
	// Preview true berarti hanya simulasi, tanpa posting dan tanpa tulis state.
	Preview bool `json:"preview"`
	// ShadowMode true berarti ringkasan berasal dari mode bayangan: Enabled harus
	// false. Angka-angka di bawah BUKAN kewajiban akuntansi; tidak ada jurnal yang
	// ditulis dan pengurangan modal inti belum dilakukan. Asumsinya sementara sampai
	// bank menyetujui parameter dan menyalakan ckpn.enabled.
	ShadowMode bool `json:"shadow_mode"`
	// Difference adalah TotalPPKA - TotalCKPN pada tingkat AGREGAT portofolio. Ia
	// hanya menamai pihak yang lebih tinggi; ia BUKAN dasar pengurang modal inti.
	// Pada portofolio campuran (sebagian kredit PPKA>CKPN, sebagian CKPN>PPKA) angka
	// ini bisa 0 atau negatif walaupun ada kelebihan PPKA per kredit, sehingga pembaca
	// tidak boleh memakainya sebagai dasar pengurang modal. Dasar yang benar adalah
	// ModalIntiDeduction.
	Difference decimal.Decimal `json:"difference"`
	// Higher menamai pihak yang lebih tinggi pada tingkat agregat: PPKA, CKPN, atau
	// SAMA. Ia hanya penjelas Difference dan TIDAK menentukan pengurang modal inti.
	Higher CKPNLarger `json:"higher"`
	// Assumptions adalah asumsi parameter yang benar-benar dipakai perhitungan (PD per
	// golongan, LGD, perlakuan agunan, aset baik, dasar EAD) supaya pembaca tahu ini
	// hitungan sementara beralasan, bukan kebijakan final. Hanya diisi mode bayangan.
	Assumptions []string `json:"assumptions,omitempty"`
	// ParameterGaps adalah kunci parameter kebijakan yang belum diisi atau diisi tetapi
	// tidak sah; CKPN tidak dapat dihitung sepenuhnya sampai bank mengisinya. Daftarnya
	// menyebutkan kunci yang harus diisi, bukan menebak nilainya.
	ParameterGaps []string `json:"parameter_gaps,omitempty"`
}

// CKPNRepository adalah akses data proses CKPN. Seluruh penulisan jurnal tetap lewat
// posting engine, bukan di sini.
type CKPNRepository interface {
	// ListActiveLoans mengambil kredit yang perlu diproses CKPN: kredit aktif, ditambah
	// kredit tidak aktif yang masih menyimpan required_ckpn bukan nol (agar cadangannya
	// dapat dilepas). Filter cabang diterapkan di query memakai actor, dan aktor
	// lintas cabang menerima seluruh bank.
	ListActiveLoans(ctx context.Context, actor Actor) ([]CKPNLoanSnapshot, error)
	// UpdateRequiredCKPN menyimpan target CKPN per kredit dalam transaksi pemanggil.
	UpdateRequiredCKPN(ctx context.Context, tx any, loanID uuid.UUID, target decimal.Decimal) error
}

// CKPNService menghitung CKPN dan membandingkannya dengan PPKA. Compare bersifat
// baca-saja; Run memposting selisih dan menyimpan target.
type CKPNService interface {
	// Compare hanya menghitung perbandingan. actor dipakai untuk membatasi kredit yang
	// dibaca pada cabangnya (aktor lintas cabang membaca seluruh bank).
	Compare(ctx context.Context, asOf time.Time, actor Actor) (CKPNComparisonSummary, error)
	Run(ctx context.Context, asOf time.Time, actor Actor) (CKPNComparisonSummary, error)
}
