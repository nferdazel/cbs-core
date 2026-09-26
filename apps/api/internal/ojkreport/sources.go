package ojkreport

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cbs-core/apps/core-api/internal/domain"
	"github.com/shopspring/decimal"
)

// sources.go memuat kontrak data tambahan yang dipakai Form 00.00, 05.00, dan 06.00
// beserta komponen rasio NPL/ROA. Kontrak ini SENGAJA terpisah dari Source (laporan
// journal-based): bila sumber data tidak tersedia, form terkait ditandai belum
// tersedia beserta alasannya, bukan diisi angka kosong.
//
// Seluruh kontrak data kredit/penempatan bersifat BANK-WIDE. Handler sudah menolak
// aktor non-lintas cabang sebelum builder dipanggil; implementasi RepoSource
// menegakkannya sekali lagi agar kebijakan tidak bergantung pada satu tempat.

// ErrOJKBankWide menandai permintaan data bank-wide oleh aktor yang tidak berwenang
// atas seluruh bank.
var ErrOJKBankWide = errors.New("laporan OJK bersifat bank-wide dan hanya dapat dibangun oleh peran lintas cabang")

// Kunci konfigurasi identitas bank Form 00.00 yang tidak muat di tabel bank_profile.
// Nilai kanoniknya dimiliki paket domain supaya penyimpanan (repository), layanan,
// dan pengekspor OJK tidak mendefinisikan kunci yang sama dua kali. Kunci 8 butir
// pertama di-seed migrasi 000046; 12 butir sisanya di-seed migrasi 000096.
const (
	OJKBankEmailKey     = domain.OJKBankEmailKey
	OJKBankWebsiteKey   = domain.OJKBankWebsiteKey
	OJKBankCityCodeKey  = domain.OJKBankCityCodeKey
	OJKBankOJKRegionKey = domain.OJKBankOJKRegionKey
	OJKPICNameKey       = domain.OJKPICNameKey
	OJKPICDivisionKey   = domain.OJKPICDivisionKey
	OJKPICPhoneKey      = domain.OJKPICPhoneKey
	OJKPICEmailKey      = domain.OJKPICEmailKey
)

// LoanRow adalah satu fasilitas kredit yang dibutuhkan Form 06.00 dan rasio NPL.
// Hanya field yang benar-benar ada di basis data yang dimuat; kolom Form 06.00 yang
// tidak punya sumber ditandai belum tersedia di form06.go, bukan diisi nol.
type LoanRow struct {
	// Status adalah status kredit internal (DISBURSED, DEFAULTED, dst.).
	Status         string
	BranchCode     string
	LoanNumber     string
	CustomerID     string
	Collectibility string
	DPD            int
	// Outstanding adalah baki debet pokok.
	Outstanding decimal.Decimal
	// RequiredCKPN adalah target CKPN yang terakhir diakui untuk kredit ini.
	RequiredCKPN decimal.Decimal
	// CKPNMethod adalah segel metode per kredit (loans.ckpn_method): sumber kolom
	// "Jenis CKPN" Form 06.00 (sandi XXI). Kosong berarti kredit lama sebelum T3;
	// dilaporkan kolektif (bawaan kebijakan) agar tidak tersangkut data lama.
	CKPNMethod         string
	IsRestructured     bool
	RestructuredCount  int
	InterestRateAnnual decimal.Decimal
	PrincipalAmount    decimal.Decimal
	AkadDate           *time.Time
	FinalDueDate       *time.Time
	// FirstInstallmentDate adalah jatuh tempo angsuran pertama (MIN due_date)
	// menurut jadwal kredit; sumber kolom XIII "Angsuran Pokok Pertama" Form 06.00.
	// Nil berarti kredit tidak punya jadwal angsuran.
	FirstInstallmentDate *time.Time
	// OverdueUnpaid adalah nominal tunggakan pokok dan bunga: jumlah sisa
	// (principal - paid_principal) + (profit - paid_profit) untuk angsuran yang
	// jatuh tempo sebelum asOf dan belum lunas. Sumber kolom XVII "Nominal
	// Tunggakan Pokok dan Bunga" Form 06.00. Nol berarti tidak menunggak.
	OverdueUnpaid decimal.Decimal
}

// LoanDataSource menyediakan kredit bank-wide. asOf dipakai implementasi untuk
// memilih posisi bila riwayat tersedia.
type LoanDataSource interface {
	ListLoansForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]LoanRow, error)
}

// BankProfileConfig adalah identitas bank untuk Form 00.00. Nilai kosong berarti
// field belum dikonfigurasi; Configured=false berarti identitas inti (nama) belum ada
// sehingga seluruh form dinyatakan belum tersedia.
type BankProfileConfig struct {
	Name          string
	Address       string
	City          string
	CityCode      string
	OJKRegionCode string
	Phone         string
	Email         string
	Website       string
	NPWP          string
	PICName       string
	PICDivision   string
	PICPhone      string
	PICEmail      string
	// Butir 10 s.d. 21 Form 00.00 (migrasi 000096). Kosong berarti belum diisi.
	DividendsPaid        string
	AnnualBonusTantiem   string
	AuditInfo            string
	ShareNominalValue    string
	PublicOfferingStatus string
	PVAStatus            string
	EBankingStatus       string
	ITProvider           string
	LakuPandaiProvider   string
	LakuPandaiAgentCount string
	RUPSOwnershipChange  string
	UltimateShareholders string
	Configured           bool
}

// BankProfileSource membaca identitas bank dari konfigurasi. Implementasi tidak
// mengarang nilai: field yang tidak diisi dibiarkan kosong.
type BankProfileSource interface {
	GetBankProfileConfig(ctx context.Context) (*BankProfileConfig, error)
}

// PlacementRow adalah satu penempatan pada bank lain. Sumbernya adalah penanda
// lps_placements (migrasi 000045); kolom Form 05.00 yang tidak ditandai di sana
// dinyatakan belum tersedia di form05.go. CKPN nil berarti penempatan belum diasesmen.
type PlacementRow struct {
	BranchCode       string
	CounterpartyBank string
	PlacementType    string
	Outstanding      decimal.Decimal
	Collectibility   string
	AsOf             time.Time
	// StartDate dan MaturityDate adalah sumber kolom VI (Jangka Waktu) Form 05.00;
	// StartDate nil berarti belum diisi. InterestRateAnnual adalah suku bunga TAHUNAN
	// dalam persen, sumber kolom VIII (Suku Bunga).
	StartDate          *time.Time
	MaturityDate       *time.Time
	InterestRateAnnual decimal.Decimal
	CKPN               *PlacementCKPNRow
}

// PlacementCKPNRow adalah CKPN tersimpan satu penempatan: metode asesmen dan target yang
// berlaku. Nilainya dibaca dari lps_placements (migrasi 000099).
type PlacementCKPNRow struct {
	Method       string
	RequiredCKPN decimal.Decimal
}

// PlacementDataSource menyediakan penempatan pada bank lain bank-wide.
type PlacementDataSource interface {
	ListPlacementsForOJK(ctx context.Context, asOf time.Time, actor domain.Actor) ([]PlacementRow, error)
}

// aktifUntukOJK melaporkan apakah kredit masih punya eksposur berjalan menurut
// statusnya. Mengikuti domain.LoanStatus.IsCKPNActive: hanya DISBURSED dan DEFAULTED.
func aktifUntukOJK(status string) bool {
	switch domain.LoanStatus(status) {
	case domain.LoanStatusDisbursed, domain.LoanStatusDefaulted:
		return true
	default:
		return false
	}
}

// KomponenKreditNPL merangkum kualitas kredit untuk rasio NPL. Tersedia=false berarti
// data kredit belum ada; pemanggil tidak boleh menafsirkan nilai nol sebagai tersedia.
type KomponenKreditNPL struct {
	KurangLancar decimal.Decimal
	Diragukan    decimal.Decimal
	Macet        decimal.Decimal
	CKPNNPL      decimal.Decimal
	TotalKredit  decimal.Decimal
	Tersedia     bool
}

// NPLDariLoanRows menjumlahkan baki debet menurut kualitas dan CKPN kredit tidak lancar.
// Kredit dengan status di luar DISBURSED/DEFAULTED dilewati karena tidak punya eksposur
// berjalan (lihat domain.LoanStatus.IsCKPNActive).
func NPLDariLoanRows(rows []LoanRow) KomponenKreditNPL {
	var k KomponenKreditNPL
	k.Tersedia = true
	for _, r := range rows {
		if !aktifUntukOJK(r.Status) {
			continue
		}
		k.TotalKredit = k.TotalKredit.Add(r.Outstanding)
		switch domain.OJKCollectibility(r.Collectibility) {
		case domain.CollectibilityKol3:
			k.KurangLancar = k.KurangLancar.Add(r.Outstanding)
		case domain.CollectibilityKol4:
			k.Diragukan = k.Diragukan.Add(r.Outstanding)
		case domain.CollectibilityKol5:
			k.Macet = k.Macet.Add(r.Outstanding)
		default:
			continue
		}
		// CKPN yang dikurangkan pada NPL neto adalah CKPN kredit tidak lancar
		// (Lampiran II hlm. 204 butir 3).
		k.CKPNNPL = k.CKPNNPL.Add(r.RequiredCKPN)
	}
	return k
}

// RataRata menghitung rata-rata aritmetika sederhana. ok=false bila tidak ada nilai,
// sehingga pemanggil menyatakan komponen belum tersedia, bukan rata-rata nol.
func RataRata(values []decimal.Decimal) (decimal.Decimal, bool) {
	if len(values) == 0 {
		return decimal.Zero, false
	}
	total := decimal.Zero
	for _, v := range values {
		total = total.Add(v)
	}
	return total.Div(decimal.NewFromInt(int64(len(values)))), true
}

// pastikanLintasCabang menegakkan kebijakan bank-wide pada lapisan data.
func pastikanLintasCabang(actor domain.Actor) error {
	if !actor.IsCrossBranch() {
		return fmt.Errorf("%w: peran %s tidak berwenang", ErrOJKBankWide, actor.Role)
	}
	return nil
}
