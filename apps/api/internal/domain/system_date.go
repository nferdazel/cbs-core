package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrEODAlreadyRunForDate = errors.New("end of day (EOD) process has already been executed for this business date")
	ErrInvalidBusinessDate  = errors.New("business date cannot be set to a past date")
	// ErrEODInProgress menandai tutup hari lain yang sedang berjalan. Berbeda dari
	// ErrEODAlreadyRunForDate yang berarti tanggalnya memang sudah ditutup.
	ErrEODInProgress = NewLocalizedError("eod_in_progress", "tutup hari sedang berjalan; tunggu sampai selesai")
)

type BusinessDateStatus string

const (
	BusinessDateStatusOpen   BusinessDateStatus = "OPEN"
	BusinessDateStatusEOD    BusinessDateStatus = "IN_EOD_PROCESSING"
	BusinessDateStatusClosed BusinessDateStatus = "CLOSED"
)

type SystemBusinessDate struct {
	CurrentDate time.Time          `json:"current_date"` // YYYY-MM-DD
	Status      BusinessDateStatus `json:"status"`
	UpdatedBy   *uuid.UUID         `json:"updated_by,omitempty"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

// EODStepStatus menandai hasil satu langkah tutup hari.
type EODStepStatus string

const (
	// EODStepRan berarti langkah dijalankan dan selesai tanpa galat.
	EODStepRan EODStepStatus = "RAN"
	// EODStepFailed berarti langkah dijalankan tetapi gagal.
	EODStepFailed EODStepStatus = "FAILED"
	// EODStepSkipped berarti langkah tidak dijalankan: prasyaratnya gagal/tidak
	// berjalan pada tanggal bisnis yang sama, atau layanannya tidak dikonfigurasi.
	EODStepSkipped EODStepStatus = "SKIPPED"
)

// EODStepResult menyatakan apa yang terjadi pada satu langkah tutup hari. Tanpa
// daftar ini, langkah yang menolak berjalan karena prasyaratnya gagal tidak
// terlihat, dan ringkasan bisa tampak sah padahal dasarnya hasil run sebelumnya.
type EODStepResult struct {
	Name   string        `json:"name"`
	Status EODStepStatus `json:"status"`
	// Prerequisites adalah nama langkah yang harus RAN lebih dulu.
	Prerequisites []string `json:"prerequisites,omitempty"`
	// Reason diisi pada status FAILED/SKIPPED agar operator tahu mengapa.
	Reason string `json:"reason,omitempty"`
}

type EODSummaryResult struct {
	ExecutedDate             time.Time `json:"executed_date"`
	NextBusinessDate         time.Time `json:"next_business_date"`
	TotalPostedJournalsToday int       `json:"total_posted_journals_today"`
	// TotalDepositAmountToday adalah setoran tunai teller (transaction_type =
	// DEPOSIT) saja, agar tidak tercampur penempatan deposito berjangka.
	//
	// Keterbatasan: jurnal penempatan yang dibuat sebelum jenis DEPOSIT_PLACEMENT ada
	// tetap bertipe DEPOSIT sehingga ikut terhitung di sini. Jumlah dan nominal
	// penempatan berjenis baru dilaporkan pada dua bidang berikut.
	TotalDepositAmountToday decimal.Decimal `json:"total_deposit_amount_today"`
	// TotalDepositPlacementsToday/TotalDepositPlacementAmountToday melaporkan
	// penempatan deposito berjangka (DEPOSIT_PLACEMENT) secara terpisah. Bidang
	// aditif: klien lama tetap membaca TotalDepositAmountToday seperti sebelumnya.
	TotalDepositPlacementsToday int             `json:"total_deposit_placements_today"`
	TotalDepositPlacementAmount decimal.Decimal `json:"total_deposit_placement_amount_today"`
	// TotalDepositAmountTodayLabel menjelaskan cakupan angka TotalDepositAmountToday
	// beserta keterbatasan data lamanya, supaya pembaca laporan tidak salah tafsir.
	TotalDepositAmountTodayLabel string          `json:"total_deposit_amount_today_label,omitempty"`
	TotalWithdrawalAmountToday   decimal.Decimal `json:"total_withdrawal_amount_today"`
	// SocialFundBalance adalah saldo akun 12500 "Dana Kebajikan" saat tutup hari:
	// akumulasi denda pembiayaan syariah (ta'zir) yang BUKAN pendapatan bank. Dana ini
	// menunggu keputusan penyaluran oleh Dewan Pengawas Syariah; EOD hanya melaporkan
	// saldo, tidak menulis jurnal dan tidak mengubah akrual.
	SocialFundBalance decimal.Decimal `json:"social_fund_balance"`
	// SocialFundNote adalah catatan singkat yang menjelaskan status dana kebajikan.
	SocialFundNote string `json:"social_fund_note,omitempty"`
	// Pekerjaan harian berikut bersifat best-effort: kegagalannya tidak
	// menggagalkan tutup hari, tetapi selalu tampil di Warnings agar tidak
	// terlihat sukses padahal tidak berjalan.
	DepositsRolledOver int `json:"deposits_rolled_over"`
	// PPAPProcessed adalah jumlah kredit yang dievaluasi; PPAPAdjusted adalah jumlah
	// kredit yang PPAP-nya benar-benar berubah sehingga menulis jurnal penyesuaian.
	// Tanpa pembedaan ini, "ppap_processed: 5" dengan 1 jurnal menyesatkan pembaca.
	PPAPProcessed        int             `json:"ppap_processed"`
	PPAPAdjusted         int             `json:"ppap_adjusted"`
	LoanPenaltiesAccrued int             `json:"loan_penalties_accrued"`
	LoanPenaltyAmount    decimal.Decimal `json:"loan_penalty_amount"`
	// Plafon denda per kredit (persen dari pokok tunggakan). LoanPenaltiesCapped
	// adalah jumlah kredit yang akrualnya dibatasi/dihentikan karena plafon; ia wajib
	// terlihat agar denda tidak berhenti bertambah diam-diam. Persentase nol berarti
	// plafon dimatikan.
	LoanPenaltiesCapped   int             `json:"loan_penalties_capped"`
	LoanPenaltyCapPercent decimal.Decimal `json:"loan_penalty_cap_percent"`
	// Denda pembiayaan syariah yang diposting ke Dana Kebajikan, bukan pendapatan.
	// Sikap sementara menunggu keputusan DPS.
	LoanPenaltiesSyariahSocialFund int `json:"loan_penalties_syariah_social_fund"`
	// Akrual pendapatan bunga kredit berbasis jadwal angsuran (peristiwa EOD kelima).
	LoanInterestAccrued       int             `json:"loan_interest_accrued"`
	LoanInterestAccruedAmount decimal.Decimal `json:"loan_interest_accrued_amount"`
	// Amortisasi saldo kerugian restrukturisasi ke pendapatan bunga (peristiwa EOD
	// keenam, di balik loan.restructure.loss.enabled).
	LoanLossAmortized       int             `json:"loan_loss_amortized"`
	LoanLossAmortizedAmount decimal.Decimal `json:"loan_loss_amortized_amount"`
	AccountsMarkedDormant   int             `json:"accounts_marked_dormant"`
	// Langkah mencatat status setiap pekerjaan harian: RAN, FAILED, atau SKIPPED.
	// Langkah yang bergantung pada langkah lain (mis. perbandingan CKPN terhadap
	// required_ppap hasil PPAP) menolak berjalan bila prasyaratnya tidak RAN pada
	// tanggal bisnis yang sama, dan penolakan itu tampil di sini.
	Steps []EODStepResult `json:"steps,omitempty"`
	// Perbandingan CKPN vs PPKA tanggal bisnis ini. Baca-saja, dihitung hanya bila
	// langkah PPAP berhasil; nilainya nol bila dilewati.
	CKPNCompared           int             `json:"ckpn_compared"`
	CKPNFailed             int             `json:"ckpn_failed"`
	CKPNTotalPPKA          decimal.Decimal `json:"ckpn_total_ppka"`
	CKPNTotalCKPN          decimal.Decimal `json:"ckpn_total_ckpn"`
	CKPNModalIntiDeduction decimal.Decimal `json:"ckpn_modal_inti_deduction"`
	// Bidang ADITIF mode bayangan CKPN (ckpn.shadow_mode.enabled). Terisi hanya saat
	// saklar bayangan menyala dan saklar resmi ckpn.enabled mati; perilaku jalur resmi
	// tidak berubah. Angka-angka ini BUKAN kewajiban akuntansi: tidak ada jurnal yang
	// ditulis, tidak ada state yang diubah, dan pengurangan modal inti BELUM dilakukan.
	// Asumsinya sementara sampai bank menyetujui parameter dan menyalakan ckpn.enabled.
	CKPNShadowMode      bool            `json:"ckpn_shadow_mode"`
	CKPNShadowProcessed int             `json:"ckpn_shadow_processed"`
	CKPNShadowFailed    int             `json:"ckpn_shadow_failed"`
	CKPNShadowTotalPPKA decimal.Decimal `json:"ckpn_shadow_total_ppka"`
	CKPNShadowTotalCKPN decimal.Decimal `json:"ckpn_shadow_total_ckpn"`
	// CKPNShadowDifference = total PPKA - total CKPN pada tingkat AGREGAT portofolio.
	// Ia hanya membandingkan kedua total; ia BUKAN dasar pengurang modal inti. Pada
	// portofolio campuran (satu kredit PPKA>CKPN, kredit lain CKPN>PPKA) angka ini bisa
	// 0 atau negatif walaupun tiap kredit memiliki kelebihan PPKA. Dasar yang benar ada
	// di CKPNShadowModalIntiDeduction.
	CKPNShadowDifference decimal.Decimal `json:"ckpn_shadow_difference"`
	// CKPNShadowModalIntiDeduction = Σ max(PPKA_i - CKPN_i, 0) PER KREDIT, yaitu
	// potensi pengurang modal inti menurut SEOJK No. 21/SEOJK.03/2024 butir 1.1.6.
	// Berbeda dari CKPNShadowDifference yang agregat: pengurang modal dihitung per
	// kredit, sehingga kredit dengan CKPN>PPKA TIDAK boleh mengurangi kelebihan kredit
	// lain. Pengurangannya BELUM dijalankan pada mode bayangan; angka ini hanya potensi.
	CKPNShadowModalIntiDeduction decimal.Decimal `json:"ckpn_shadow_modal_inti_deduction"`
	// CKPNShadowHigher menamai pihak yang lebih tinggi pada tingkat agregat: "PPKA",
	// "CKPN", atau "SAMA". Ia TIDAK dipakai sebagai dasar pengurang modal inti.
	CKPNShadowHigher string `json:"ckpn_shadow_higher"`
	// CKPNShadowAssumptions adalah asumsi yang dipakai (PD per golongan, LGD, perlakuan
	// agunan, aset baik, dasar EAD) supaya pembaca tahu ini hitungan sementara
	// beralasan, bukan kebijakan final.
	CKPNShadowAssumptions []string `json:"ckpn_shadow_assumptions,omitempty"`
	// CKPNShadowAsetBaik adalah jumlah kredit yang target bayangannya nol karena
	// dikecualikan sebagai aset baik (butir 12.3.a.2.a); CKPNShadowAsetBaikOutstanding
	// adalah jumlah sisa pokoknya. Keduanya menjelaskan total CKPN nol sebagai hasil
	// perhitungan, bukan tanda model belum dijalankan.
	CKPNShadowAsetBaik            int             `json:"ckpn_shadow_aset_baik"`
	CKPNShadowAsetBaikOutstanding decimal.Decimal `json:"ckpn_shadow_aset_baik_outstanding"`
	// Bidang basis KEDUA "setara PPKA" (bidang CKPNShadow* di atas adalah basis
	// pertama "sesuai kebijakan" yang mengecualikan aset baik). Basis kedua menilai
	// aset baik dengan model yang sama (EAD x PD x LGD) supaya sebanding dengan PPKA
	// yang dihitung atas seluruh kredit. Keduanya hanya DILAPORKAN: tidak ada jurnal
	// dan required_ckpn tidak ditulis. Bila CKPNShadowSetaraPPKAFailed > 0, totalnya
	// hanya mencakup kredit yang dapat dihitung.
	CKPNShadowSetaraPPKAProcessed          int             `json:"ckpn_shadow_setara_ppka_processed"`
	CKPNShadowSetaraPPKAFailed             int             `json:"ckpn_shadow_setara_ppka_failed"`
	CKPNShadowSetaraPPKATotalPPKA          decimal.Decimal `json:"ckpn_shadow_setara_ppka_total_ppka"`
	CKPNShadowSetaraPPKATotalCKPN          decimal.Decimal `json:"ckpn_shadow_setara_ppka_total_ckpn"`
	CKPNShadowSetaraPPKADifference         decimal.Decimal `json:"ckpn_shadow_setara_ppka_difference"`
	CKPNShadowSetaraPPKAModalIntiDeduction decimal.Decimal `json:"ckpn_shadow_setara_ppka_modal_inti_deduction"`
	CKPNShadowSetaraPPKAHigher             string          `json:"ckpn_shadow_setara_ppka_higher"`
	// CKPNShadowBasisNote menjelaskan mengapa dua basis bisa berbeda dan menegaskan
	// keduanya belum menjadi kebijakan bank.
	CKPNShadowBasisNote string `json:"ckpn_shadow_basis_note,omitempty"`
	// CKPNShadowParameterGaps adalah kunci parameter PD/LGD yang belum diisi atau diisi
	// dengan satuan salah (persen alih-alih fraksi). CKPN tidak dapat dihitung untuk
	// kredit yang membutuhkannya sampai kunci ini diperbaiki.
	CKPNShadowParameterGaps []string `json:"ckpn_shadow_parameter_gaps,omitempty"`
	// CKPNShadowNote menjelaskan status mode bayangan, termasuk bila parameter belum
	// lengkap sehingga CKPN tidak dapat dihitung dan apa yang harus diisi.
	CKPNShadowNote string    `json:"ckpn_shadow_note,omitempty"`
	Warnings       []string  `json:"warnings,omitempty"`
	ExecutedBy     uuid.UUID `json:"executed_by"`
	CompletedAt    time.Time `json:"completed_at"`
}

type EOMSummaryResult struct {
	ExecutedMonth          string          `json:"executed_month"` // YYYY-MM
	TotalAdminFeesDeducted decimal.Decimal `json:"total_admin_fees_deducted"`
	TotalInterestPaid      decimal.Decimal `json:"total_interest_paid"`
	ProcessedAccounts      int             `json:"processed_accounts"`
	FailedAccounts         int             `json:"failed_accounts"`
	CompletedAt            time.Time       `json:"completed_at"`
}

type EOYSummaryResult struct {
	FiscalYear          int             `json:"fiscal_year"`
	TotalRevenueClosed  decimal.Decimal `json:"total_revenue_closed"`
	TotalExpenseClosed  decimal.Decimal `json:"total_expense_closed"`
	NetRetainedEarnings decimal.Decimal `json:"net_retained_earnings"`
	ClosingJournalRef   string          `json:"closing_journal_ref"`
	Books               []EOYBookResult `json:"books"`
	CompletedAt         time.Time       `json:"completed_at"`
}

// --- Interfaces ---

type BusinessDateRepository interface {
	GetCurrentDate(ctx context.Context) (*SystemBusinessDate, error)
	AdvanceDate(ctx context.Context, nextDate time.Time, updatedBy uuid.UUID) error
	// ClaimEOD memindahkan status tanggal bisnis ke EOD dalam SATU statement, dan
	// menolak bila tanggal sudah CLOSED. Hasil false berarti tutup hari tidak boleh
	// dijalankan. Ini menggantikan pola baca-lalu-tulis yang membuat dua permintaan
	// bersamaan dapat sama-sama lolos dan menjalankan pekerjaan harian dua kali.
	ClaimEOD(ctx context.Context) (bool, error)
	// TryEODLock mengambil kunci eksklusif tutup hari pada sesi database. Selama
	// kunci dipegang, permintaan tutup hari lain ditolak dengan ErrEODInProgress.
	// Klaim status saja tidak cukup: status EOD berarti "sedang berjalan" sekaligus
	// "pernah berhenti di tengah", dan keduanya harus dibedakan. Kunci dilepas
	// otomatis bila koneksi berakhir, sehingga proses yang mati di tengah tidak
	// meninggalkan kunci permanen; fungsi release yang dikembalikan melepasnya lebih
	// awal.
	TryEODLock(ctx context.Context) (func() error, error)
}

// OperationalActivityReader menunjukkan apakah instalasi sudah pernah beroperasi
// (ada jurnal tersimpan dan/atau tanggal bisnis sudah disetel). Dipakai peringatan
// kesiapan CKPN saat start: CKPN yang masih mati pada instalasi yang sudah berjalan
// berarti bank sudah beroperasi tanpa membentuk CKPN. Sengaja interface sempit dan
// terpisah dari BusinessDateRepository agar tidak memaksa setiap peniru repositori
// tanggal bisnis menyediakan metode ini.
type OperationalActivityReader interface {
	// HasOperationalActivity mengembalikan true bila instalasi sudah punya jurnal
	// dan/atau tanggal bisnis tersimpan. Kegagalan membaca dikembalikan apa adanya
	// agar pemanggil tidak menyimpulkan "belum beroperasi" dari error.
	HasOperationalActivity(ctx context.Context) (bool, error)
}

type BatchProcessService interface {
	GetCurrentBusinessDate(ctx context.Context) (*SystemBusinessDate, error)
	RunEOD(ctx context.Context, executedBy uuid.UUID) (*EODSummaryResult, error)
	// RunScheduledEOD menjalankan tutup hari dari pemicu terjadwal (tabel
	// eod_triggers). Jalur sambung bagi penjadwal yang belum dibangun; menolak bila
	// tidak ada pemicu terjadwal yang jatuh tempo.
	RunScheduledEOD(ctx context.Context, executedBy uuid.UUID) (*EODSummaryResult, error)
	RunEOM(ctx context.Context, executedBy uuid.UUID) (*EOMSummaryResult, error)
	// RunEOY menerima actor (bukan hanya id) karena parameter `book` dari body harus
	// dibatasi pada buku aktor: aktor satu buku tidak boleh menutup buku lain.
	RunEOY(ctx context.Context, book string, actor Actor) (*EOYSummaryResult, error)
}
