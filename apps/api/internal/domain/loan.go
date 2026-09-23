package domain

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var (
	ErrLoanNotFound         = NewLocalizedError("loan_not_found", "pengajuan kredit tidak ditemukan")
	ErrLoanAlreadyApproved  = NewLocalizedError("loan_already_approved", "kredit sudah disetujui atau ditolak")
	ErrLoanNotApproved      = errors.New("kredit harus berstatus APPROVED sebelum dicairkan")
	ErrLoanAlreadyDisbursed = errors.New("kredit sudah dicairkan")
	// ErrLoanNotCancellable dipakai saat pembatalan pencairan diminta tetapi kredit tidak
	// dalam keadaan bisa dibatalkan (mis. sudah lunas, macet, atau dihapusbukukan).
	ErrLoanNotCancellable = errors.New("hanya kredit berstatus DISBURSED yang dapat dibatalkan pencairannya")
	// ErrLoanHasInstallmentPayments dipakai saat sudah ada angsuran yang dibayar. Dalam
	// keadaan itu pencairan tidak boleh dibatalkan — yang benar adalah koreksi nominal,
	// karena uang dan jadwal angsuran sudah berjalan.
	ErrLoanHasInstallmentPayments = errors.New("kredit sudah memiliki angsuran dibayar, pembatalan pencairan tidak dapat dilakukan")
	// ErrLoanHasAccrualPostings menolak pembatalan pencairan yang sudah memiliki akrual
	// terposting (piutang bunga, piutang denda, atau cadangan PPAP). Pembatalan hanya
	// menghapus jadwal dan menolkan tagihan; ia tidak membalik jurnal-jurnal akrual itu,
	// sehingga piutang dan cadangan akan menggantung tanpa kredit yang menopangnya.
	// Akrualnya harus diselesaikan atau dibalik lebih dulu.
	ErrLoanHasAccrualPostings = errors.New("kredit sudah memiliki akrual bunga/denda/PPAP terposting; selesaikan atau balik akrualnya lebih dulu sebelum membatalkan pencairan")
	ErrInvalidLoanAmount      = errors.New("nominal pokok harus positif")
	ErrInvalidLoanTerm        = errors.New("jangka waktu minimal 1 bulan")
	// Penolakan kredit wajib punya alasan yang bisa dipertanggungjawabkan. Alasan
	// kosong atau kepanjangan ditolak dengan pesan yang jelas, bukan disimpan apa adanya.
	ErrLoanRejectionReasonRequired = errors.New("alasan penolakan kredit wajib diisi")
	ErrLoanRejectionReasonTooLong  = errors.New("alasan penolakan kredit maksimal 500 karakter")
	// MaxLoanRejectionReasonLen membatasi panjang alasan; constraint database
	// (migrasi 000055) memakai angka yang sama.
	MaxLoanRejectionReasonLen = 500
	// ErrLoanAmountUnchanged menolak koreksi nominal yang tidak mengubah apa pun:
	// tanpa perubahan, jurnal selisih nol dan penulisan ulang jadwal hanya menghapus
	// jejak tanpa manfaat.
	ErrLoanAmountUnchanged = errors.New("nominal baru sama dengan nominal lama")

	// Syarat hapus buku menurut POJK 1/2024 Pasal 42-43 (BPR) dan POJK 24/2024
	// Pasal 50-51 (BPRS): hanya atas aset macet yang cadangannya sudah 100%, tidak
	// boleh sebagian, dan wajib didahului upaya penagihan yang terdokumentasi.
	// Tiap syarat punya sentinel sendiri supaya penolakan menyebut syarat mana yang
	// belum terpenuhi, bukan galat umum yang tidak bisa ditindaklanjuti.
	ErrWriteOffNotMacet                  = NewLocalizedError("write_off_not_macet", "hapus buku hanya dapat dilakukan atas kredit berkualitas Macet (kolektibilitas 5)")
	ErrWriteOffReserveIncomplete         = NewLocalizedError("write_off_reserve_incomplete", "hapus buku memerlukan cadangan/penyisihan 100% atas kredit")
	ErrWriteOffPartial                   = NewLocalizedError("write_off_partial", "hapus buku sebagian dilarang; hapus buku harus atas seluruh eksposur kredit")
	ErrWriteOffReasonRequired            = NewLocalizedError("write_off_reason_required", "dasar pertimbangan hapus buku wajib diisi")
	ErrWriteOffCollectionEffortsRequired = NewLocalizedError("write_off_collection_efforts_required", "upaya penagihan terdokumentasi wajib diisi sebelum hapus buku")
	// Nilai hapus buku disimpan di baris kredit (written_off_amount) supaya pemulihan
	// setelahnya dapat dibatasi pada nominal yang benar-benar pernah dihapus. Tanpa
	// simpanan ini, akumulasi pemulihan hanya berbatas kejujuran operator dan dapat
	// melebihi nilai yang pernah dilepas (lubang nyata: recovery 10x lipat diterima).
	ErrRecoveryExceedsWriteOff = NewLocalizedError("recovery_exceeds_write_off", "akumulasi pemulihan melebihi nilai hapus buku kredit")
	// ErrWriteOffAmountUnavailable menolak pemulihan atas kredit hapus buku yang nilai
	// hapus bukunya belum tersimpan dan tidak dapat dibackfill. Menolak lebih aman
	// daripada membiarkan pemulihan tanpa batas atas data yang tidak pasti.
	ErrWriteOffAmountUnavailable = NewLocalizedError("write_off_amount_unavailable", "nilai hapus buku kredit belum tersimpan, pemulihan tidak dapat dibatasi")
)

type LoanStatus string

const (
	LoanStatusPendingApproval LoanStatus = "PENDING_APPROVAL"
	LoanStatusApproved        LoanStatus = "APPROVED"
	LoanStatusDisbursed       LoanStatus = "DISBURSED"
	LoanStatusRejected        LoanStatus = "REJECTED"
	LoanStatusPaidOff         LoanStatus = "PAID_OFF"
	LoanStatusDefaulted       LoanStatus = "DEFAULTED"
	LoanStatusWrittenOff      LoanStatus = "WRITTEN_OFF"
	// LoanStatusCancelled menandai pencairan yang dibatalkan sebelum ada angsuran dibayar.
	// Berbeda dari PAID_OFF (selesai dibayar), DEFAULTED (gagal bayar), dan WRITTEN_OFF
	// (dihapusbukukan): kredit yang batal tidak pernah ada sebagai tagihan.
	LoanStatusCancelled LoanStatus = "CANCELLED"
)

// IsCKPNActive melaporkan apakah kredit masih punya eksposur berjalan yang harus
// dibentuk CKPN. Hanya DISBURSED dan DEFAULTED; status lain (PENDING_APPROVAL,
// APPROVED, REJECTED, PAID_OFF, WRITTEN_OFF, CANCELLED) tidak punya eksposur dan
// hanya boleh melepas required_ckpn yang tersisa.
func (s LoanStatus) IsCKPNActive() bool {
	return s == LoanStatusDisbursed || s == LoanStatusDefaulted
}

// IsPPAPActive melaporkan apakah kredit masih punya eksposur berjalan yang harus
// dicadangkan PPAP. Hanya DISBURSED dan DEFAULTED; status lain (PAID_OFF, WRITTEN_OFF,
// CANCELLED) tidak punya eksposur dan hanya boleh melepas required_ppap yang tersisa
// dengan target nol, supaya cadangan tidak menggantung selamanya.
func (s LoanStatus) IsPPAPActive() bool {
	return s == LoanStatusDisbursed || s == LoanStatusDefaulted
}

// LoanType mengikuti enum loan_type di database dan tidak punya nilai default,
// sehingga wajib diisi setiap kali kredit dibuat.
type LoanType string

const (
	LoanTypeConventionalFlat    LoanType = "CONVENTIONAL_FLAT"
	LoanTypeConventionalAnnuity LoanType = "CONVENTIONAL_ANNUITY"
	LoanTypeSyariahMurabahah    LoanType = "SYARIAH_MURABAHAH"
	LoanTypeSyariahMudharabah   LoanType = "SYARIAH_MUDHARABAH"
)

// LoanTypeFor memetakan produk ke jenis kredit. Enum database belum punya nilai
// untuk musyarakah, ijarah, dan skema syariah lain, jadi semuanya masuk ke
// SYARIAH_MUDHARABAH (sama-sama bagi hasil) sampai enum diperluas lewat migrasi.
func LoanTypeFor(product *BankingProduct) LoanType {
	switch product.ProfitScheme {
	case SchemeMurabahah:
		return LoanTypeSyariahMurabahah
	case SchemeMudharabah, SchemeMusyarakah, SchemeIjarah:
		return LoanTypeSyariahMudharabah
	}
	if product.ScheduleMethod == ScheduleAnnuity {
		return LoanTypeConventionalAnnuity
	}
	return LoanTypeConventionalFlat
}

// LoanNumberPrefix menentukan prefix nomor kredit menurut buku produk: KRD untuk
// konvensional, PMB untuk pembiayaan syariah. Buku kosong (produk lama) diperlakukan
// konvensional, sama seperti perilaku jalur kredit sebelum nomor kredit dipisahkan
// dari referensi transaksi.
func LoanNumberPrefix(product *BankingProduct) string {
	if product != nil && product.Book == BookSyariah {
		return "PMB"
	}
	return "KRD"
}

type InstallmentStatus string

const (
	InstallmentStatusPending InstallmentStatus = "PENDING"
	InstallmentStatusPaid    InstallmentStatus = "PAID"
	InstallmentStatusOverdue InstallmentStatus = "OVERDUE"
	InstallmentStatusPartial InstallmentStatus = "PARTIAL"
)

// ProfitType menamai sifat imbal hasil pada jadwal angsuran. Nilainya mengikuti
// skema produk, dan menentukan label serta jurnal yang dipakai.
type ProfitType string

const (
	ProfitTypeInterest  ProfitType = "INTEREST"   // bunga (konvensional)
	ProfitTypeMargin    ProfitType = "MARGIN"     // margin murabahah
	ProfitTypeBagiHasil ProfitType = "BAGI_HASIL" // nisbah mudharabah/musyarakah
)

type OJKCollectibility string

const (
	CollectibilityKol1 OJKCollectibility = "1_LANCAR"
	CollectibilityKol2 OJKCollectibility = "2_DPK"
	CollectibilityKol3 OJKCollectibility = "3_KURANG_LANCAR"
	CollectibilityKol4 OJKCollectibility = "4_DIRAGUKAN"
	CollectibilityKol5 OJKCollectibility = "5_MACET"
)

type AccrualStatus string

const (
	AccrualStatusAccrual AccrualStatus = "ACCRUAL_PERFORMING"
	AccrualStatusCash    AccrualStatus = "CASH_BASIS_NPL"
)

// Aturan kolektibilitas hanya boleh ada satu: CollectibilityFromDPD di ppap.go
// dengan ambang dari system_config. Jangan menambah fungsi kolektibilitas lain.

type Loan struct {
	ID                    uuid.UUID  `json:"id"`
	LoanNumber            string     `json:"loan_number"`
	CustomerID            uuid.UUID  `json:"customer_id"`
	ProductID             *uuid.UUID `json:"product_id,omitempty"`
	BranchID              *uuid.UUID `json:"branch_id,omitempty"`
	BranchCode            string     `json:"branch_code,omitempty"` // kode cabang kredit; kosong = data lama
	DisbursementAccountID uuid.UUID  `json:"disbursement_account_id"`
	LoanType              LoanType   `json:"loan_type"`
	Status                LoanStatus `json:"status"`

	Collectibility OJKCollectibility `json:"collectibility"`
	DPD            int               `json:"dpd"`
	AccrualStatus  AccrualStatus     `json:"accrual_status"`
	RequiredPPAP   decimal.Decimal   `json:"required_ppap"`
	// RequiredCKPN adalah target CKPN terakhir yang diakui untuk kredit ini. Dibaca
	// jalur CKPN sebagai nilai otoritatif dari baris yang sudah dikunci.
	RequiredCKPN decimal.Decimal `json:"required_ckpn"`

	// OriginalEIRMonthly adalah suku bunga efektif orisinal per bulan (fraksi, mis.
	// 0.01713), dihitung saat pencairan dari arus kas nyata dan dipakai sebagai tingkat
	// diskonto nilai kini arus kas hasil restrukturisasi (PA BPR Bab 5.2, SAK EP 11.20).
	// Nol berarti belum tersimpan; perhitungan kerugian menolak memakai suku bunga
	// kontraktual sebagai gantinya.
	OriginalEIRMonthly decimal.Decimal `json:"original_eir_monthly,omitempty"`
	OriginalEIRMethod  string          `json:"original_eir_method,omitempty"`
	// OriginalEIRBasis adalah dasar audit perhitungan EIR (metode, masukan, waktu) dalam
	// bentuk JSON; disimpan sebagai JSONB pada loans.original_eir_basis.
	OriginalEIRBasis        string     `json:"original_eir_basis,omitempty"`
	OriginalEIRCalculatedAt *time.Time `json:"original_eir_calculated_at,omitempty"`
	// RestructureLossBalance adalah saldo kerugian restrukturisasi yang belum
	// diamortisasi (pengurang nilai tercatat). PPKA dihitung atas nilai tercatat
	// setelah dikurangi saldo ini (domain.PPAPCarryingAmount).
	//
	// JALUR LUNAS SUDAH AMAN: amortisasi saldo ini ADA (service
	// amortizeRestructureLossForLoan, di balik saklar loan.restructure.loss.enabled).
	// Setiap angsuran jatuh tempo memulihkan sebagian saldo lewat jurnal
	// Db. Kredit yang diberikan / Kr. Pendapatan bunga sehingga akun piutang kredit
	// tidak berakhir negatif saat pokok penuh dikredit; saat kredit lunas sisa saldo
	// ditutup tepat nol (bukti angka: uji integrasi
	// TestIntegrasiAmortisasiSaldoKerugianLunasNol vs pembanding
	// TestIntegrasiAmortisasiSaklarMatiPiutangNegatif). Jadi saldo ini bukan lagi
	// "sekadar belum diamortisasi" pada jalur pelunasan.
	//
	// SISA KETERBATASAN (butuh keputusan pemilik, belum dipaksa di sini): hapus buku
	// (writeOffTx) melepas POKOK BRUTO (OutstandingPrincipal) sebesar P, padahal nilai
	// tercatat saat itu P - saldo ini. Selisihnya membuat akun piutang kredit sempat
	// negatif, lalu penutupan amortisasi batch mengkredit PENDAPATAN BUNGA atas kredit
	// macet yang dihapusbukukan — perlakuan yang secara akuntansi perlu ditinjau
	// (seharusnya beban, bukan pendapatan). Perbaikannya menyentuh nominal jurnal hapus
	// buku dan harus diputuskan bank; jangan menyebut saldo ini tanpa menyebut akibat
	// saldo negatif pada jalur hapus buku.
	RestructureLossBalance decimal.Decimal `json:"restructure_loss_balance"`

	IsRestructured      bool       `json:"is_restructured"`
	RestructuredCount   int        `json:"restructured_count"`
	RestructuredAt      *time.Time `json:"restructured_at,omitempty"`
	RestructuringReason string     `json:"restructuring_reason,omitempty"`

	// PreRestructureCollectibility adalah kualitas Kredit sesaat sebelum
	// restrukturisasi terakhir; dasar penerapan batas Pasal 31 POJK 1/2024.
	// Catatan: rujukan lama "Pasal 23" berasal dari POJK 33/2018 yang sudah dicabut.
	PreRestructureCollectibility OJKCollectibility `json:"pre_restructure_collectibility,omitempty"`

	// FinalDueDate adalah jatuh tempo Kredit (angsuran terakhir), dipakai menilai
	// dimensi "Kredit telah jatuh tempo" POJK 1/2024 Lampiran II.
	FinalDueDate *time.Time `json:"final_due_date,omitempty"`

	PrincipalAmount      decimal.Decimal `json:"principal_amount"`
	AcquisitionCost      decimal.Decimal `json:"acquisition_cost"`
	DeferredMargin       decimal.Decimal `json:"deferred_margin"`
	InterestRateAnnual   decimal.Decimal `json:"interest_rate_annual"`
	MarginAmount         decimal.Decimal `json:"margin_amount"`
	ProfitSharingRatio   decimal.Decimal `json:"profit_sharing_ratio"`
	TotalPayable         decimal.Decimal `json:"total_payable"`
	TermMonths           int             `json:"term_months"`
	MonthlyInstallment   decimal.Decimal `json:"monthly_installment"`
	OutstandingPrincipal decimal.Decimal `json:"outstanding_principal"`
	PenaltyAccrued       decimal.Decimal `json:"penalty_accrued"`
	// WrittenOffAmount adalah nilai yang dilepas dari neraca saat hapus buku (pokok +
	// piutang bunga + denda). Disimpan saat eksekusi agar pemulihan setelahnya dapat
	// dibatasi pada nominal yang benar-benar pernah dihapus. Nol berarti belum/tidak
	// diketahui (data lama yang tidak dapat dibackfill).
	WrittenOffAmount decimal.Decimal `json:"written_off_amount"`

	AkadNumber string     `json:"akad_number,omitempty"`
	AkadDate   *time.Time `json:"akad_date,omitempty"`
	Purpose    string     `json:"purpose,omitempty"`
	// RejectionReason diisi saat kredit ditolak; ikut tampil di daftar dan detail
	// kredit agar keputusan penolakan dapat diperiksa tanpa membuka audit log.
	RejectionReason string `json:"rejection_reason,omitempty"`

	AOID        *uuid.UUID `json:"ao_id,omitempty"`
	ApprovedBy  *uuid.UUID `json:"approved_by,omitempty"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	DisbursedAt *time.Time `json:"disbursed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`

	Schedules []LoanSchedule `json:"schedules,omitempty"`
}

type LoanSchedule struct {
	ID                   uuid.UUID         `json:"id"`
	LoanID               uuid.UUID         `json:"loan_id"`
	InstallmentNo        int               `json:"installment_no"`
	DueDate              time.Time         `json:"due_date"`
	PrincipalAmount      decimal.Decimal   `json:"principal_amount"`
	ProfitAmount         decimal.Decimal   `json:"profit_amount"`
	TotalInstallment     decimal.Decimal   `json:"total_installment"`
	PaidPrincipal        decimal.Decimal   `json:"paid_principal"`
	PaidProfit           decimal.Decimal   `json:"paid_profit"`
	ProfitType           ProfitType        `json:"profit_type"`
	OutstandingPrincipal decimal.Decimal   `json:"outstanding_principal"`
	Status               InstallmentStatus `json:"status"`
	PaidAt               *time.Time        `json:"paid_at,omitempty"`
	CreatedAt            time.Time         `json:"created_at"`

	// ProfitAccruedAt menandai angsuran ini sudah diakru bunganya (basis akrual).
	// ProfitAccruedAmount adalah SISA akruan yang belum diselesaikan pembayaran;
	// nilainya berkurang saat angsuran dibayar agar piutang bunga (10400) tidak
	// pernah negatif dan pendapatan diakui maksimal sebesar porsi bunga jadwal.
	ProfitAccruedAt     *time.Time      `json:"profit_accrued_at,omitempty"`
	ProfitAccruedAmount decimal.Decimal `json:"profit_accrued_amount"`

	// RestructureLossAmortizedAt menandai angsuran ini sudah diamortisasi saldo
	// kerugian restrukturisasinya; RestructureLossAmortizedAmount menyimpan nominal
	// kumulatif yang diamortisasi (audit). Penanda ini hanya agar batch berikutnya
	// tidak memproses ulang angsuran yang sama; idempotensi sesungguhnya dijaga
	// keberadaan jurnal dengan kunci LOSSAMORT-<nomor kredit>-<angsuran>.
	RestructureLossAmortizedAt     *time.Time      `json:"restructure_loss_amortized_at,omitempty"`
	RestructureLossAmortizedAmount decimal.Decimal `json:"restructure_loss_amortized_amount"`
}

type ApplyLoanInput struct {
	CustomerID            uuid.UUID       `json:"customer_id"`
	ProductID             uuid.UUID       `json:"product_id"`
	DisbursementAccountID uuid.UUID       `json:"disbursement_account_id"`
	PrincipalAmount       decimal.Decimal `json:"principal_amount"`
	TermMonths            int             `json:"term_months"`
	MarginAmount          decimal.Decimal `json:"margin_amount"`
	Purpose               string          `json:"purpose"`
}

// LoanPaymentMethod menentukan dari mana angsuran dibayar. Bank menerima angsuran
// tunai di kas teller maupun lewat rekening nasabah, dan keduanya menghasilkan jurnal
// yang berbeda: yang pertama menyentuh kas, yang kedua menyentuh rekening nasabah.
type LoanPaymentMethod string

const (
	// LoanPaymentAccount mendebit rekening nasabah. Ini perilaku bawaan bila metode
	// tidak disebutkan, sehingga pemanggil lama tidak berubah.
	LoanPaymentAccount LoanPaymentMethod = "ACCOUNT"
	// LoanPaymentCash mencatat penerimaan tunai di kas teller tanpa menyentuh
	// rekening nasabah.
	LoanPaymentCash LoanPaymentMethod = "CASH"
)

type PayInstallmentInput struct {
	LoanID        uuid.UUID `json:"loan_id"`
	InstallmentNo int       `json:"installment_no"`
	// Amount kosong berarti pelunasan penuh angsuran ini beserta dendanya; nominal
	// yang lebih kecil dicatat sebagai angsuran sebagian.
	Amount decimal.Decimal `json:"amount"`
	// Method kosong diperlakukan sebagai LoanPaymentAccount.
	Method LoanPaymentMethod `json:"method"`
}

type RestructureLoanInput struct {
	LoanID                uuid.UUID       `json:"loan_id"`
	NewTermMonths         int             `json:"new_term_months"`
	NewInterestRateAnnual decimal.Decimal `json:"new_interest_rate_annual"`
	NewMarginAmount       decimal.Decimal `json:"new_margin_amount"`
	Reason                string          `json:"reason"`
}

type WriteOffLoanInput struct {
	LoanID uuid.UUID `json:"loan_id"`
	Reason string    `json:"reason"`
	// CollectionEfforts adalah ringkasan upaya penagihan yang sudah dilakukan; wajib
	// diisi karena POJK 1/2024 Pasal 43 mewajibkan upaya memperoleh kembali
	// didokumentasikan sebelum hapus buku. Nilainya ikut tersimpan pada permintaan
	// maker-checker dan audit log sebagai bukti syarat terpenuhi saat keputusan dibuat.
	CollectionEfforts string `json:"collection_efforts"`
	// Amount boleh dikosongkan untuk menghapus buku seluruh eksposur. Bila diisi,
	// nilainya WAJIB sama dengan seluruh sisa pokok: nominal yang lebih kecil ditolak
	// sebagai hapus buku sebagian (POJK 1/2024 Pasal 42 ayat (2)).
	Amount decimal.Decimal `json:"amount,omitempty"`
}

type RecoverWrittenOffLoanInput struct {
	LoanID         uuid.UUID       `json:"loan_id"`
	RecoveryAmount decimal.Decimal `json:"recovery_amount"`
	// IdempotencyKey membedakan SETIAP penerimaan kas yang sah atas kredit yang sama.
	// Pemanggil mengisinya dari header Idempotency-Key (atau body); pengulangan
	// permintaan dengan kunci yang sama tidak menggandakan jurnal. Kosong berarti
	// kunci deterministik dari nomor kredit + nominal + tanggal bisnis dipakai, bukan
	// time.Now(): pengulangan pada hari bisnis yang sama tetap idempoten.
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// CancelLoanInput membatalkan pencairan kredit yang belum pernah menerima
// angsuran. Reason wajib diisi: pembatalan yang membalik jurnal dan menghapus
// seluruh jadwal tagihan harus dapat dipertanggungjawabkan.
type CancelLoanInput struct {
	LoanID uuid.UUID `json:"loan_id"`
	Reason string    `json:"reason"`
}

// CorrectLoanAmountInput mengoreksi nominal pokok kredit yang sudah dicairkan tanpa
// membatalkan pencairan. Berbeda dari pembatalan, angsuran yang sudah dibayar tetap
// apa adanya: uang yang sudah masuk bukan objek koreksi ini. Reason wajib diisi
// karena perubahannya mengubah tagihan nasabah yang sedang berjalan.
type CorrectLoanAmountInput struct {
	LoanID    uuid.UUID       `json:"loan_id"`
	NewAmount decimal.Decimal `json:"new_amount"`
	Reason    string          `json:"reason"`
}

// ProfitSchemeLabel menurunkan label skema imbal hasil untuk keperluan dokumen.
func (l *Loan) ProfitSchemeLabel() string {
	if l.ProfitSharingRatio.IsPositive() {
		return "Bagi Hasil"
	}
	if l.MarginAmount.IsPositive() {
		return "Murabahah (Margin)"
	}
	return "Bunga"
}

type LoanRepository interface {
	Create(ctx context.Context, loan *Loan, schedules []LoanSchedule) error
	GetByID(ctx context.Context, id uuid.UUID) (*Loan, error)
	GetByNumber(ctx context.Context, loanNumber string) (*Loan, error)
	// List mengembalikan daftar kredit yang boleh dibaca aktor. Filter cabang
	// diterapkan di query agar pagination dan total tetap benar.
	List(ctx context.Context, limit, offset int, actor Actor) ([]Loan, int, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status LoanStatus, approvedBy *uuid.UUID) error
	UpdateStatusTx(ctx context.Context, tx any, id uuid.UUID, status LoanStatus, approvedBy *uuid.UUID) error
	// RejectLoanTx menandai kredit REJECTED sekaligus menyimpan alasan penolakan di
	// dalam transaksi pemanggil, agar status dan alasan tidak pernah terpisah. Hanya
	// baris berstatus PENDING_APPROVAL yang berubah: penolakan yang membaca status basi
	// tidak boleh menimpa kredit yang sudah disetujui/dicairkan.
	RejectLoanTx(ctx context.Context, tx any, id uuid.UUID, reason string) error
	MarkDisbursed(ctx context.Context, id uuid.UUID, outstanding decimal.Decimal) error
	MarkDisbursedTx(ctx context.Context, tx any, id uuid.UUID, outstanding decimal.Decimal) error
	GetSchedules(ctx context.Context, loanID uuid.UUID) ([]LoanSchedule, error)
	// UpdateSchedulePayment mencatat pembayaran angsuran. settleAccrued adalah porsi
	// bunga yang diselesaikan dari akruan yang sudah terbentuk; kolom
	// profit_accrued_amount dikurangi sebesar itu (tidak pernah negatif) sehingga
	// piutang bunga 10400 nol setelah seluruh angsuran dibayar.
	UpdateSchedulePayment(ctx context.Context, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status InstallmentStatus) error
	// UpdateSchedulePaymentTx mencatat pembayaran angsuran di dalam transaksi pemanggil
	// agar jurnal dan perubahan jadwal tidak pernah terpisah.
	UpdateSchedulePaymentTx(ctx context.Context, tx any, scheduleID uuid.UUID, paidPrincipal, paidProfit, settleAccrued decimal.Decimal, status InstallmentStatus) error
	UpdateRestructure(ctx context.Context, loan *Loan, schedules []LoanSchedule) error
	UpdateCollectibility(ctx context.Context, id uuid.UUID, col OJKCollectibility, dpd int, accrual AccrualStatus, ppap decimal.Decimal) error
	UpdateOutstanding(ctx context.Context, id uuid.UUID, outstanding, penalty decimal.Decimal) error
	// UpdateOutstandingTx menyimpan sisa pokok dan denda di dalam transaksi pemanggil.
	UpdateOutstandingTx(ctx context.Context, tx any, id uuid.UUID, outstanding, penalty decimal.Decimal) error
	// SetWrittenOffAmountTx menyimpan nilai hapus buku di dalam transaksi pemanggil,
	// satu transaksi dengan jurnal pelepasan dan perubahan statusnya. Nilai inilah yang
	// menjadi batas akumulasi pemulihan setelah kredit dihapus buku.
	SetWrittenOffAmountTx(ctx context.Context, tx any, id uuid.UUID, amount decimal.Decimal) error
	// SumRecoveredAmountTx menjumlahkan nominal pemulihan yang sudah tercatat untuk satu
	// nomor kredit dari jurnal RECOV-, MENGECUALIKAN kunci idempotensi yang sedang
	// diproses. Pengecualian itu penting: pengulangan permintaan recovery yang sama harus
	// tetap lolos pemeriksaan batas (idempoten), bukan ditolak karena jurnalnya sendiri
	// sudah terhitung.
	SumRecoveredAmountTx(ctx context.Context, tx any, loanNumber, excludeIdempotencyKey string) (decimal.Decimal, error)
	// ListPenaltyCandidates mengambil kredit aktif beserta pokok angsuran yang lewat
	// jatuh tempo pada asOf dan jatuh tempo angsuran tertua. actor membatasi hasil
	// pada buku yang aktif di instalasi/aktor agar batch denda tidak memproses lini
	// usaha yang tidak dilayani.
	ListPenaltyCandidates(ctx context.Context, asOf time.Time, actor Actor) ([]LoanPenaltyCandidate, error)
	// AddPenaltyAccruedTx menambah penalty_accrued dan memajukan
	// penalty_last_accrued_on ke accruedOn di dalam transaksi pemanggil. Penambahan
	// hanya terjadi bila jurnal denda dengan idempotencyKey tersebut belum ada,
	// sehingga akrual tanggal yang sama tidak pernah dihitung dua kali. Nilai kembali
	// false berarti denda tanggal itu sudah pernah diakru (replay idempoten).
	AddPenaltyAccruedTx(ctx context.Context, tx any, loanID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedOn time.Time) (bool, error)
	// ListInterestAccrualCandidates mengambil angsuran konvensional yang sudah jatuh
	// tempo, belum dibayar, dan belum diakru bunganya pada asOf. actor membatasi hasil
	// pada buku yang aktif di instalasi/aktor agar batch akrual tidak memproses lini
	// usaha yang tidak dilayani.
	ListInterestAccrualCandidates(ctx context.Context, asOf time.Time, actor Actor) ([]LoanInterestAccrualCandidate, error)
	// AddScheduleProfitAccruedTx menambah profit_accrued_amount dan mengisi
	// profit_accrued_at satu angsuran di dalam transaksi pemanggil. Penambahan hanya
	// terjadi bila jurnal akrual dengan idempotencyKey tersebut belum ada, sehingga
	// satu angsuran tidak pernah diakru dua kali. Nilai kembali false berarti angsuran
	// itu sudah pernah diakru (replay idempoten).
	AddScheduleProfitAccruedTx(ctx context.Context, tx any, scheduleID uuid.UUID, amount decimal.Decimal, idempotencyKey string, accruedAt time.Time) (bool, error)
	// HasInstallmentPaymentTx melaporkan apakah sudah ada angsuran yang dibayar atau
	// tidak lagi PENDING. Selama masih seluruhnya PENDING, pencairan belum menyentuh
	// uang dan jadwal, sehingga aman dibatalkan penuh.
	HasInstallmentPaymentTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error)
	// LockLoanTx mengunci baris kredit (SELECT ... FOR UPDATE) dan mengembalikan
	// keadaannya yang segar di dalam transaksi pemanggil. Seluruh jalur yang mengubah
	// jadwal angsuran, nominal, atau status kredit harus mengambil kunci ini lebih dulu
	// agar tidak saling menyalip: tanpa kunci, pembayaran yang commit di antara baca dan
	// hapus jadwal akan terhapus, dan dua koreksi bersamaan menerbitkan jurnal ganda.
	LockLoanTx(ctx context.Context, tx any, id uuid.UUID) (*Loan, error)
	// HasAccrualPostingsTx melaporkan apakah kredit sudah punya akrual terposting:
	// piutang bunga tersimpan di jadwal, piutang denda atau cadangan PPAP tersimpan di
	// baris kredit. Pembatalan pencairan menolak kredit semacam itu karena ia tidak
	// membalik jurnal akrualnya.
	HasAccrualPostingsTx(ctx context.Context, tx any, loanID uuid.UUID) (bool, error)
	// DeleteSchedulesTx menghapus seluruh jadwal angsuran satu kredit di dalam
	// transaksi pemanggil. Kredit yang dibatalkan tidak pernah ada sebagai tagihan,
	// jadi jadwalnya tidak boleh tertinggal menggantung.
	DeleteSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) error
	// GetDisbursementJournalRefTx mengambil nomor referensi jurnal pencairan kredit.
	// Jurnal pencairan tidak menyimpan nomor kredit, jadi pencariannya lewat
	// idempotency_key yang dibentuk DisburseLoan ("DISB-"+nomor kredit).
	GetDisbursementJournalRefTx(ctx context.Context, tx any, loanNumber string) (string, error)
	// GetSchedulesTx membaca jadwal angsuran di dalam transaksi pemanggil. Koreksi
	// nominal harus menghitung dari jadwal yang sama dengan yang akan ditulis. Jaminan
	// bahwa tidak ada pembayaran yang menyelinap di antara baca dan tulis datang dari
	// LockLoanTx yang harus dipanggil lebih dulu, bukan dari transaksi itu sendiri.
	GetSchedulesTx(ctx context.Context, tx any, loanID uuid.UUID) ([]LoanSchedule, error)
	// CorrectLoanAmountTx memperbarui nominal, total tagihan, angsuran bulanan, dan
	// sisa pokok lalu mengganti seluruh jadwal angsuran di dalam transaksi pemanggil.
	// Pola ganti jadwalnya sama dengan UpdateRestructure.
	CorrectLoanAmountTx(ctx context.Context, tx any, loan *Loan, schedules []LoanSchedule) error
	// NextCorrectionCountTx menaikkan penghitung koreksi nominal kredit dan
	// mengembalikan nilai barunya di dalam transaksi pemanggil. Kenaikan harus satu
	// transaksi dengan jurnalnya agar transaksi yang gagal tidak meninggalkan nomor
	// koreksi yang terpakai; nilainya menjadi bagian kunci idempotensi jurnal koreksi.
	NextCorrectionCountTx(ctx context.Context, tx any, loanID uuid.UUID) (int, error)
}

type LoanService interface {
	ApplyLoan(ctx context.Context, input ApplyLoanInput, actor Actor) (*Loan, error)
	ApproveLoan(ctx context.Context, loanID uuid.UUID, actor Actor) (*Loan, error)
	// RejectLoan menolak pengajuan dan menyimpan alasan yang diberikan pejabat;
	// alasan kosong atau terlalu panjang ditolak dengan error yang jelas.
	RejectLoan(ctx context.Context, loanID uuid.UUID, reason string, actor Actor) (*Loan, error)
	DisburseLoan(ctx context.Context, loanID uuid.UUID, actor Actor) (*Loan, error)
	// GetLoan menolak kredit cabang lain dengan ErrCrossBranchAccess.
	GetLoan(ctx context.Context, id uuid.UUID, actor Actor) (*Loan, error)
	ListLoans(ctx context.Context, page, pageSize int, actor Actor) ([]Loan, int, error)
	PayInstallment(ctx context.Context, input PayInstallmentInput, actor Actor) (*LoanSchedule, error)
	RestructureLoan(ctx context.Context, input RestructureLoanInput, actor Actor) (*Loan, error)
	// WriteOffLoan dan RecoverWrittenOffLoan tidak langsung berefek bila melewati
	// ambang persetujuan: keduanya mengembalikan PendingApprovalError dan efeknya
	// baru terjadi saat ExecuteApproved dipanggil maker-checker.
	WriteOffLoan(ctx context.Context, input WriteOffLoanInput, actor Actor) (*Loan, error)
	RecoverWrittenOffLoan(ctx context.Context, input RecoverWrittenOffLoanInput, actor Actor) (*Loan, error)
	// CancelDisbursementLoan membatalkan pencairan kredit yang belum memiliki
	// angsuran dibayar: jurnal pencairan dibalik, jadwal dihapus, status CANCELLED,
	// dan sisa pokok nol. Seluruh efeknya berada dalam satu transaksi.
	CancelDisbursementLoan(ctx context.Context, input CancelLoanInput, actor Actor) (*Loan, error)
	// CorrectLoanAmount mengoreksi nominal pokok kredit DISBURSED tanpa membatalkan
	// pencairan: jadwal yang belum dibayar dihitung ulang, jurnal selisih diposting,
	// dan audit ditulis. Wajib lewat persetujuan pejabat kedua.
	CorrectLoanAmount(ctx context.Context, input CorrectLoanAmountInput, actor Actor) (*Loan, error)
	// ExecuteApproved menjalankan hapus buku atau recovery yang sudah disetujui
	// maker-checker, di dalam transaksi milik pemanggil.
	ExecuteApproved(ctx context.Context, tx any, actionType string, payload map[string]any, actor Actor) error
	AccruePenalties(ctx context.Context, asOf time.Time, actor Actor) (LoanPenaltySummary, error)
	// AccrueInterest mengakru pendapatan bunga kredit konvensional berbasis jadwal
	// angsuran; kredit tidak lancar dan produk syariah dilewati.
	AccrueInterest(ctx context.Context, asOf time.Time, actor Actor) (LoanInterestAccrualSummary, error)
	// AmortizeRestructureLoss memulihkan saldo kerugian restrukturisasi ke pendapatan
	// bunga memakai EIR orisinal (PA BPR Bab 5.2 hlm. 61). Berada di balik saklar
	// loan.restructure.loss.enabled; saklar mati berarti tidak ada perubahan apa pun.
	AmortizeRestructureLoss(ctx context.Context, asOf time.Time, actor Actor) (RestructureLossAmortizationSummary, error)
}
